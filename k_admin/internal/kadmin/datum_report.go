package kadmin

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/gin-gonic/gin"
)

// Report module (A6) for both materials and bookmarks: users file reports
// against approved content, track their own reports and the per-report
// timeline; an admin-authenticated audit endpoint records the verdict and
// applies the linkage rules — result 属实 (1) demotes the target to
// status 3 (被举报/下架) and, for materials, auto-generates the type-4
// takedown notification for the uploader.

func (s *Store) registerDatumReportRoutes(datumGroup *gin.RouterGroup) {
	reports := datumGroup.Group("/report", s.requireDatumAuth())
	reports.GET("/*rest", s.datumReportGet)
	reports.POST("", s.datumReportCreate)
	reports.PUT("", s.datumReportUpdate)
	reports.DELETE("/:ids", s.datumReportDelete)

	// 审核走管理端 JWT（requireAuth），不能挂在 datum 认证组下
	datumGroup.Group("/report").POST("/audit/:reportId", s.requireAuth(), s.datumReportAudit)

	bookmarkReports := datumGroup.Group("/bookmarkReport", s.requireDatumAuth())
	bookmarkReports.GET("/*rest", s.datumBookmarkReportGet)
	bookmarkReports.POST("", s.datumBookmarkReportCreate)
	bookmarkReports.PUT("", s.datumBookmarkReportUpdate)
	bookmarkReports.DELETE("/:ids", s.datumBookmarkReportDelete)

	datumGroup.Group("/bookmarkReport").POST("/audit/:reportId", s.requireAuth(), s.datumBookmarkReportAudit)
}

// ---------------------------------------------------------------------------
// file reports
// ---------------------------------------------------------------------------

// datumReportGet dispatches GET /datum/report/{list|timeline/:id|:id}.
func (s *Store) datumReportGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "list":
		s.datumReportList(c)
	case strings.HasPrefix(rest, "timeline/"):
		s.datumReportTimeline(c, rest)
	default:
		if reportID, err := strconv.ParseInt(rest, 10, 64); err == nil && reportID > 0 {
			c.Params = append(c.Params, gin.Param{Key: "reportId", Value: rest})
			s.datumReportDetail(c)
			return
		}
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

func (s *Store) datumReportList(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	page, size := datumPageParams(c)
	repo := datum.NewReportRepo(s.conn)
	// 查询口径：fileId → 该文件的时间线集合；result → 审核队列；默认 → 本人举报列表
	if fileID := datumQueryInt(c, "fileId"); fileID > 0 {
		reports, err := repo.ListByFile(fileID)
		if err != nil {
			fail(c, http.StatusInternalServerError, "举报列表查询失败")
			return
		}
		respondDatumReportRows(c, reports, int64(len(reports)))
		return
	}
	if result := strings.TrimSpace(c.Query("result")); result != "" {
		resultPage, err := repo.ListByResult(result, page, size)
		if err != nil {
			fail(c, http.StatusInternalServerError, "举报列表查询失败")
			return
		}
		rows, _ := resultPage.Items.([]datum.Report)
		respondDatumReportRows(c, rows, resultPage.Total)
		return
	}
	if claimed := datumQueryInt(c, "userId"); claimed != 0 && claimed != userID {
		fail(c, http.StatusForbidden, "没有权限")
		return
	}
	result, err := repo.ListByReporter(userID, page, size)
	if err != nil {
		fail(c, http.StatusInternalServerError, "举报列表查询失败")
		return
	}
	items, _ := result.Items.([]datum.ReportedFile)
	rows := make([]gin.H, 0, len(items))
	for _, item := range items {
		rows = append(rows, gin.H{
			"reportId": item.ReportID, "fileId": item.FileID, "userId": item.UserID,
			"reason": item.Reason, "result": item.Result, "remark": item.Remark,
			"createTime": item.CreateTime, "updateTime": item.UpdateTime,
			"fileName": item.FileName, "fileSubject": item.FileSubject, "fileSchool": item.FileSchool,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "msg": "ok", "rows": rows, "total": result.Total})
}

func respondDatumReportRows(c *gin.Context, reports []datum.Report, total int64) {
	rows := make([]gin.H, 0, len(reports))
	for _, report := range reports {
		rows = append(rows, gin.H{
			"reportId": report.ReportID, "fileId": report.FileID, "userId": report.UserID,
			"reason": report.Reason, "result": report.Result, "remark": report.Remark,
			"createTime": report.CreateTime, "updateTime": report.UpdateTime,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "msg": "ok", "rows": rows, "total": total})
}

type datumReportCreateRequest struct {
	FileID int64  `json:"fileId"`
	UserID int64  `json:"userId"`
	Reason string `json:"reason"`
	Remark string `json:"remark"`
}

func (s *Store) datumReportCreate(c *gin.Context) {
	var req datumReportCreateRequest
	_ = c.ShouldBind(&req)
	userID, ok := datumSessionUser(c, req.UserID)
	if !ok {
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len([]rune(reason)) > 500 {
		fail(c, http.StatusBadRequest, "举报原因不能为空且不超过 500 字")
		return
	}
	repo := datum.NewReportRepo(s.conn)
	file, found, err := datum.NewFileRepo(s.conn).FindByID(req.FileID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "文件查询失败")
		return
	}
	if !found || (file.FileStatus != 1 && file.UserID != userID) {
		fail(c, http.StatusNotFound, "文件不存在或已删除")
		return
	}
	if file.UserID == userID {
		fail(c, http.StatusBadRequest, "不能举报自己的文件")
		return
	}
	if pending, found, _ := repo.FindByFileAndReporter(req.FileID, userID); found && pending.Result == "0" {
		fail(c, http.StatusConflict, "该文件已有待处理的举报，请等待审核")
		return
	}
	reportID, err := repo.Create(req.FileID, userID, reason, strings.TrimSpace(req.Remark))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "该文件已有待处理的举报，请等待审核")
			return
		}
		fail(c, http.StatusInternalServerError, "举报提交失败")
		return
	}
	success(c, gin.H{"reportId": reportID})
}

func (s *Store) datumReportDetail(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	reportID, err := strconv.ParseInt(c.Param("reportId"), 10, 64)
	if err != nil || reportID <= 0 {
		fail(c, http.StatusBadRequest, "举报 ID 不正确")
		return
	}
	report, found, err := datum.NewReportRepo(s.conn).FindByID(reportID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "举报查询失败")
		return
	}
	if !found || report.UserID != userID {
		fail(c, http.StatusNotFound, "举报不存在")
		return
	}
	success(c, gin.H{
		"reportId": report.ReportID, "fileId": report.FileID, "userId": report.UserID,
		"reason": report.Reason, "result": report.Result, "remark": report.Remark,
		"createTime": report.CreateTime, "updateTime": report.UpdateTime,
	})
}

type datumReportUpdateRequest struct {
	ReportID int64  `json:"reportId"`
	Reason   string `json:"reason"`
	Remark   string `json:"remark"`
}

func (s *Store) datumReportUpdate(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumReportUpdateRequest
	_ = c.ShouldBind(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len([]rune(reason)) > 500 {
		fail(c, http.StatusBadRequest, "举报原因不能为空且不超过 500 字")
		return
	}
	if err := datum.NewReportRepo(s.conn).UpdateOwn(req.ReportID, userID, reason, strings.TrimSpace(req.Remark)); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "举报不存在或已进入审核，无法修改")
			return
		}
		fail(c, http.StatusInternalServerError, "举报更新失败")
		return
	}
	success(c, true)
}

func (s *Store) datumReportDelete(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	repo := datum.NewReportRepo(s.conn)
	deleted := 0
	for _, raw := range strings.Split(c.Param("ids"), ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if err := repo.DeleteOwn(id, userID); err == nil {
			deleted++
		}
	}
	if deleted == 0 {
		fail(c, http.StatusNotFound, "举报不存在或无权删除")
		return
	}
	success(c, gin.H{"deleted": deleted})
}

// datumReportTimeline renders the lifecycle events of one report (提交 → 审核结论).
func (s *Store) datumReportTimeline(c *gin.Context, rest string) {
	reportID, err := strconv.ParseInt(strings.TrimPrefix(rest, "timeline/"), 10, 64)
	if err != nil || reportID <= 0 {
		fail(c, http.StatusBadRequest, "举报 ID 不正确")
		return
	}
	userID, _ := datumUserIDFrom(c)
	repo := datum.NewReportRepo(s.conn)
	report, found, err := repo.FindByID(reportID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "举报查询失败")
		return
	}
	if !found || report.UserID != userID {
		fail(c, http.StatusNotFound, "举报不存在")
		return
	}
	events := []gin.H{{
		"step":   "提交举报",
		"time":   report.CreateTime,
		"result": "0",
		"reason": report.Reason,
	}}
	if report.Result != "0" {
		conclusion := "举报不属实，资料维持原状"
		if report.Result == "1" {
			conclusion = "举报属实，资料已下架"
		}
		events = append(events, gin.H{
			"step":       "审核完成",
			"time":       report.UpdateTime,
			"result":     report.Result,
			"remark":     report.Remark,
			"conclusion": conclusion,
		})
	}
	success(c, events)
}

type datumReportAuditRequest struct {
	Result string `json:"result"`
	Remark string `json:"remark"`
}

// datumReportAudit (admin) records the verdict and applies the linkage:
// result=1 demotes the file to status 3 and creates the type-4 takedown
// notification for the uploader.
func (s *Store) datumReportAudit(c *gin.Context) {
	reportID, err := strconv.ParseInt(c.Param("reportId"), 10, 64)
	if err != nil || reportID <= 0 {
		fail(c, http.StatusBadRequest, "举报 ID 不正确")
		return
	}
	var req datumReportAuditRequest
	_ = c.ShouldBind(&req)
	result := strings.TrimSpace(req.Result)
	if result != "1" && result != "2" {
		fail(c, http.StatusBadRequest, "审核结果不正确：1-属实 2-不属实")
		return
	}
	reviewer := "admin"
	if vbenUserID := c.GetString("vben_user_id"); vbenUserID != "" {
		reviewer = "admin:" + vbenUserID
	}
	repo := datum.NewReportRepo(s.conn)
	report, found, err := repo.FindByID(reportID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "举报查询失败")
		return
	}
	if !found {
		fail(c, http.StatusNotFound, "举报不存在")
		return
	}
	if err := repo.SetResult(reportID, result, reviewer, strings.TrimSpace(req.Remark)); err != nil {
		fail(c, http.StatusInternalServerError, "审核失败")
		return
	}
	if result == "1" {
		_ = datum.NewFileRepo(s.conn).SetStatus(report.FileID, 3, reviewer)
		if file, found, fileErr := datum.NewFileRepo(s.conn).FindByID(report.FileID); fileErr == nil && found {
			if notifyErr := datum.NewNotificationRepo(s.conn).CreateTakedownNotification(file.FileID, file.UserID, file.FileName); notifyErr != nil {
				log.Printf("datum takedown notification failed: %v", notifyErr)
			}
		}
	}
	s.invalidateDatumFiles()
	success(c, gin.H{"reportId": reportID, "result": result})
}

// ---------------------------------------------------------------------------
// bookmark reports
// ---------------------------------------------------------------------------

// datumBookmarkReportGet dispatches GET /datum/bookmarkReport/{list|timeline/:id|:id}.
func (s *Store) datumBookmarkReportGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "list":
		s.datumBookmarkReportList(c)
	case strings.HasPrefix(rest, "timeline/"):
		s.datumBookmarkReportTimeline(c, strings.TrimPrefix(rest, "timeline/"))
	default:
		if reportID, err := strconv.ParseInt(rest, 10, 64); err == nil && reportID > 0 {
			c.Params = append(c.Params, gin.Param{Key: "reportId", Value: rest})
			s.datumBookmarkReportDetail(c)
			return
		}
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

func (s *Store) datumBookmarkReportList(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	page, size := datumPageParams(c)
	repo := datum.NewBookmarkReportRepo(s.conn)
	if bookmarkID := datumQueryInt(c, "bookmarkId"); bookmarkID > 0 {
		reports, err := repo.ListByBookmark(bookmarkID)
		if err != nil {
			fail(c, http.StatusInternalServerError, "举报列表查询失败")
			return
		}
		respondDatumBookmarkReportRows(c, reports, int64(len(reports)))
		return
	}
	if result := strings.TrimSpace(c.Query("result")); result != "" {
		resultPage, err := repo.ListByResult(result, page, size)
		if err != nil {
			fail(c, http.StatusInternalServerError, "举报列表查询失败")
			return
		}
		rows, _ := resultPage.Items.([]datum.BookmarkReport)
		respondDatumBookmarkReportRows(c, rows, resultPage.Total)
		return
	}
	if claimed := datumQueryInt(c, "userId"); claimed != 0 && claimed != userID {
		fail(c, http.StatusForbidden, "没有权限")
		return
	}
	result, err := repo.ListByReporter(userID, page, size)
	if err != nil {
		fail(c, http.StatusInternalServerError, "举报列表查询失败")
		return
	}
	items, _ := result.Items.([]datum.ReportedBookmark)
	rows := make([]gin.H, 0, len(items))
	for _, item := range items {
		rows = append(rows, gin.H{
			"reportId": item.ReportID, "bookmarkId": item.BookmarkID, "userId": item.UserID,
			"reason": item.Reason, "result": item.Result, "remark": item.Remark,
			"createTime": item.CreateTime, "updateTime": item.UpdateTime,
			"bookmarkTitle": item.BookmarkTitle, "bookmarkUrl": item.BookmarkURL,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "msg": "ok", "rows": rows, "total": result.Total})
}

func respondDatumBookmarkReportRows(c *gin.Context, reports []datum.BookmarkReport, total int64) {
	rows := make([]gin.H, 0, len(reports))
	for _, report := range reports {
		rows = append(rows, gin.H{
			"reportId": report.ReportID, "bookmarkId": report.BookmarkID, "userId": report.UserID,
			"reason": report.Reason, "result": report.Result, "remark": report.Remark,
			"createTime": report.CreateTime, "updateTime": report.UpdateTime,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "msg": "ok", "rows": rows, "total": total})
}

type datumBookmarkReportCreateRequest struct {
	BookmarkID int64  `json:"bookmarkId"`
	UserID     int64  `json:"userId"`
	Reason     string `json:"reason"`
	Remark     string `json:"remark"`
}

func (s *Store) datumBookmarkReportCreate(c *gin.Context) {
	var req datumBookmarkReportCreateRequest
	_ = c.ShouldBind(&req)
	userID, ok := datumSessionUser(c, req.UserID)
	if !ok {
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len([]rune(reason)) > 500 {
		fail(c, http.StatusBadRequest, "举报原因不能为空且不超过 500 字")
		return
	}
	bookmarkRepo := datum.NewBookmarkRepo(s.conn)
	bookmark, found, err := bookmarkRepo.FindByID(req.BookmarkID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "书签查询失败")
		return
	}
	if !found || (bookmark.Status != 1 && bookmark.UserID != userID) {
		fail(c, http.StatusNotFound, "书签不存在或已删除")
		return
	}
	if bookmark.UserID == userID {
		fail(c, http.StatusBadRequest, "不能举报自己的书签")
		return
	}
	repo := datum.NewBookmarkReportRepo(s.conn)
	if pending, found, _ := repo.FindByReporterAndBookmark(req.BookmarkID, userID); found && pending.Result == "0" {
		fail(c, http.StatusConflict, "该书签已有待处理的举报，请等待审核")
		return
	}
	reportID, err := repo.Create(req.BookmarkID, userID, reason, strings.TrimSpace(req.Remark))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "该书签已有待处理的举报，请等待审核")
			return
		}
		fail(c, http.StatusInternalServerError, "举报提交失败")
		return
	}
	success(c, gin.H{"reportId": reportID})
}

func (s *Store) datumBookmarkReportDetail(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	reportID, err := strconv.ParseInt(c.Param("reportId"), 10, 64)
	if err != nil || reportID <= 0 {
		fail(c, http.StatusBadRequest, "举报 ID 不正确")
		return
	}
	report, found, err := datum.NewBookmarkReportRepo(s.conn).FindByID(reportID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "举报查询失败")
		return
	}
	if !found || report.UserID != userID {
		fail(c, http.StatusNotFound, "举报不存在")
		return
	}
	success(c, gin.H{
		"reportId": report.ReportID, "bookmarkId": report.BookmarkID, "userId": report.UserID,
		"reason": report.Reason, "result": report.Result, "remark": report.Remark,
		"createTime": report.CreateTime, "updateTime": report.UpdateTime,
	})
}

type datumBookmarkReportUpdateRequest struct {
	ReportID int64  `json:"reportId"`
	Reason   string `json:"reason"`
	Remark   string `json:"remark"`
}

func (s *Store) datumBookmarkReportUpdate(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumBookmarkReportUpdateRequest
	_ = c.ShouldBind(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" || len([]rune(reason)) > 500 {
		fail(c, http.StatusBadRequest, "举报原因不能为空且不超过 500 字")
		return
	}
	if err := datum.NewBookmarkReportRepo(s.conn).UpdateOwn(req.ReportID, userID, reason, strings.TrimSpace(req.Remark)); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "举报不存在或已进入审核，无法修改")
			return
		}
		fail(c, http.StatusInternalServerError, "举报更新失败")
		return
	}
	success(c, true)
}

func (s *Store) datumBookmarkReportDelete(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	repo := datum.NewBookmarkReportRepo(s.conn)
	deleted := 0
	for _, raw := range strings.Split(c.Param("ids"), ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		if err := repo.DeleteOwn(id, userID); err == nil {
			deleted++
		}
	}
	if deleted == 0 {
		fail(c, http.StatusNotFound, "举报不存在或无权删除")
		return
	}
	success(c, gin.H{"deleted": deleted})
}

func (s *Store) datumBookmarkReportTimeline(c *gin.Context, rest string) {
	reportID, err := strconv.ParseInt(strings.TrimPrefix(rest, "timeline/"), 10, 64)
	if err != nil || reportID <= 0 {
		fail(c, http.StatusBadRequest, "举报 ID 不正确")
		return
	}
	userID, _ := datumUserIDFrom(c)
	repo := datum.NewBookmarkReportRepo(s.conn)
	report, found, err := repo.FindByID(reportID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "举报查询失败")
		return
	}
	if !found || report.UserID != userID {
		fail(c, http.StatusNotFound, "举报不存在")
		return
	}
	events := []gin.H{{
		"step":   "提交举报",
		"time":   report.CreateTime,
		"result": "0",
		"reason": report.Reason,
	}}
	if report.Result != "0" {
		conclusion := "举报不属实，书签维持原状"
		if report.Result == "1" {
			conclusion = "举报属实，书签已下架"
		}
		events = append(events, gin.H{
			"step":       "审核完成",
			"time":       report.UpdateTime,
			"result":     report.Result,
			"remark":     report.Remark,
			"conclusion": conclusion,
		})
	}
	success(c, events)
}

func (s *Store) datumBookmarkReportAudit(c *gin.Context) {
	reportID, err := strconv.ParseInt(c.Param("reportId"), 10, 64)
	if err != nil || reportID <= 0 {
		fail(c, http.StatusBadRequest, "举报 ID 不正确")
		return
	}
	var req datumReportAuditRequest
	_ = c.ShouldBind(&req)
	result := strings.TrimSpace(req.Result)
	if result != "1" && result != "2" {
		fail(c, http.StatusBadRequest, "审核结果不正确：1-属实 2-不属实")
		return
	}
	reviewer := "admin"
	if vbenUserID := c.GetString("vben_user_id"); vbenUserID != "" {
		reviewer = "admin:" + vbenUserID
	}
	repo := datum.NewBookmarkReportRepo(s.conn)
	report, found, err := repo.FindByID(reportID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "举报查询失败")
		return
	}
	if !found {
		fail(c, http.StatusNotFound, "举报不存在")
		return
	}
	if err := repo.SetResult(reportID, result, reviewer, strings.TrimSpace(req.Remark)); err != nil {
		fail(c, http.StatusInternalServerError, "审核失败")
		return
	}
	if result == "1" {
		_ = datum.NewBookmarkRepo(s.conn).SetStatus(report.BookmarkID, 3)
	}
	success(c, gin.H{"reportId": reportID, "result": result})
}
