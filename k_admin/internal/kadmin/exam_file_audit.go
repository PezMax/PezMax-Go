package kadmin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/bootstrap"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/gin-gonic/gin"
)

// 文件审核工作台：待审核 / 被举报试卷文件（ptmj_file + ptmj_report）的集中审核。
//
// 队列：GET /pending（file_status=0）、GET /reported（file_status=3 且存在待处理举报）；
// 操作：POST /approve（多选通过）、/approve-user（单用户全部通过）、/reject（待审核拒绝）、
// /audit（被举报复审，approve=举报不属实恢复上架，reject=举报属实下架并通知）。
// 审核原因随结果落库：复审写 ptmj_report.remark，下架/驳回另以 type-4 定向通知
// （content=原因）返回给上传用户；文件变更后失效 datum 树/排行缓存。

const (
	examFileAuditViewPermission = bootstrap.FileAuditViewPermission
	examFileAuditPermission     = bootstrap.FileAuditPermission
)

func (s *Store) registerExamFileAuditRoutes(api *gin.RouterGroup) {
	if err := s.ensureExamFileAuditMenu(); err != nil {
		// 菜单缺失只影响导航入口，不阻塞接口注册；启动日志保留线索。
		fmt.Printf("kadmin: sync exam-file-audit menu: %v\n", err)
	}
	group := api.Group("/exam-file-audit", s.requireAuth())
	group.GET("/pending", s.requirePermission(examFileAuditViewPermission), s.examFileAuditPending)
	group.GET("/reported", s.requirePermission(examFileAuditViewPermission), s.examFileAuditReported)
	group.POST("/approve", s.requirePermission(examFileAuditPermission), s.examFileAuditApprove)
	group.POST("/approve-user", s.requirePermission(examFileAuditPermission), s.examFileAuditApproveUser)
	group.POST("/reject", s.requirePermission(examFileAuditPermission), s.examFileAuditReject)
	group.POST("/audit", s.requirePermission(examFileAuditPermission), s.examFileAuditReview)
}

// ---------------------------------------------------------------------------
// responses
// ---------------------------------------------------------------------------

// examFileAuditItem 是审核队列里的文件行；createBy 返回上传人展示名
// （ptmj_user.user_name 优先，缺失时回退 ptmj_file.create_by），
// fileUrl 返回可读 HTTP 直链，前端 pdfjs 直接按需拉取渲染。
type examFileAuditItem struct {
	FileID      int64  `json:"fileId"`
	UserID      int64  `json:"userId"`
	UserName    string `json:"userName"`
	FileName    string `json:"fileName"`
	FileURL     string `json:"fileUrl"`
	FileSize    int64  `json:"fileSize"`
	FileFormat  string `json:"fileFormat"`
	FileYear    int64  `json:"fileYear"`
	FileType    int64  `json:"fileType"`
	FileSchool  string `json:"fileSchool"`
	FileSubject string `json:"fileSubject"`
	Reviewer    string `json:"reviewer"`
	FileStatus  int64  `json:"fileStatus"`
	CreateBy    string `json:"createBy"`
	CreateTime  string `json:"createTime"`
	Remark      string `json:"remark"`
}

type examFileAuditReportedItem struct {
	examFileAuditItem
	ReportID     int64  `json:"reportId"`
	ReportReason string `json:"reportReason"`
	ReportResult string `json:"reportResult"`
	ReporterName string `json:"reporterName"`
	ReportTime   string `json:"reportTime"`
}

type examFileAuditPage struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

// ---------------------------------------------------------------------------
// queues
// ---------------------------------------------------------------------------

// swaggerExamFileAuditPending documents GET /exam-file-audit/pending.
// @Summary 待审核文件队列
// @Tags 文件审核
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param fileName query string false "文件名关键词"
// @Param user query string false "上传用户（ID 或用户名）"
// @Param fileType query int false "文件类型"
// @Success 200 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Router /exam-file-audit/pending [get]
func (s *Store) examFileAuditPending(c *gin.Context) {
	page, pageSize := examFileAuditPageParams(c)
	where := ` WHERE f.file_status = 0 AND f.del_flag = 0`
	args := []interface{}{}
	if keyword := strings.TrimSpace(c.Query("fileName")); keyword != "" {
		where += ` AND f.file_name ILIKE ?`
		args = append(args, "%"+keyword+"%")
	}
	if fileType := examFileAuditInt64(c.Query("fileType")); fileType > 0 {
		where += ` AND f.file_type = ?`
		args = append(args, fileType)
	}
	if user := strings.TrimSpace(c.Query("user")); user != "" {
		where += ` AND (CAST(f.user_id AS TEXT) = ? OR f.create_by ILIKE ? OR u.user_name ILIKE ?)`
		args = append(args, user, "%"+user+"%", "%"+user+"%")
	}

	pageResult, err := s.examFileAuditList(where, args, page, pageSize)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, pageResult)
}

// swaggerExamFileAuditReported documents GET /exam-file-audit/reported.
// @Summary 被举报文件复审队列（联查最新待处理举报）
// @Tags 文件审核
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param fileName query string false "文件名或举报原因关键词"
// @Success 200 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Router /exam-file-audit/reported [get]
func (s *Store) examFileAuditReported(c *gin.Context) {
	page, pageSize := examFileAuditPageParams(c)
	join := ` JOIN LATERAL (
		SELECT rp.report_id, rp.reason AS report_reason, rp.result AS report_result,
			rp.user_id AS reporter_id, rp.create_time AS report_time
		FROM ptmj_report rp
		WHERE rp.file_id = f.file_id AND rp.result = '0'
		ORDER BY rp.report_id DESC
		LIMIT 1
	) rp ON true
	LEFT JOIN ptmj_user ru ON ru.user_id = rp.reporter_id`
	where := ` WHERE f.file_status = 3 AND f.del_flag = 0`
	args := []interface{}{}
	if keyword := strings.TrimSpace(c.Query("fileName")); keyword != "" {
		where += ` AND (f.file_name ILIKE ? OR rp.report_reason ILIKE ?)`
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	countRows, err := s.conn.Query(`SELECT count(*) AS count FROM ptmj_file f`+join+where, args...)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = datum.ScanInt64(countRows[0]["count"])
	}
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.conn.Query(`SELECT f.file_id, f.user_id, f.file_name, f.file_url, f.file_size, f.file_format,
			f.file_year, f.file_type, f.file_school, f.file_subject, f.reviewer, f.file_status, f.create_by, f.create_time, f.remark,
			COALESCE(NULLIF(u.user_name, ''), f.create_by) AS display_name,
			rp.report_id, rp.report_reason, rp.report_result, rp.reporter_id, rp.report_time,
			COALESCE(NULLIF(ru.user_name, ''), CAST(rp.reporter_id AS TEXT)) AS reporter_name
		FROM ptmj_file f
		LEFT JOIN ptmj_user u ON u.user_id = f.user_id`+join+where+`
		ORDER BY rp.report_time DESC, f.file_id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]examFileAuditReportedItem, 0, len(rows))
	for _, row := range rows {
		item := examFileAuditReportedItem{
			examFileAuditItem: examFileAuditItemFromRow(row),
			ReportID:          datum.ScanInt64(row["report_id"]),
			ReportReason:      datum.ScanString(row["report_reason"]),
			ReportResult:      datum.ScanString(row["report_result"]),
			ReporterName:      datum.ScanString(row["reporter_name"]),
			ReportTime:        examFileAuditTimeText(row["report_time"]),
		}
		items = append(items, item)
	}
	httpx.Success(c, examFileAuditPage{Items: items, Total: total, Page: page, PageSize: pageSize})
}

// examFileAuditList 组装待审核队列的分页应答（reported 队列因需联查举报单单独实现）。
func (s *Store) examFileAuditList(where string, args []interface{}, page, pageSize int) (examFileAuditPage, error) {
	countRows, err := s.conn.Query(`SELECT count(*) AS count FROM ptmj_file f
		LEFT JOIN ptmj_user u ON u.user_id = f.user_id`+where, args...)
	if err != nil {
		return examFileAuditPage{}, err
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = datum.ScanInt64(countRows[0]["count"])
	}
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.conn.Query(`SELECT f.file_id, f.user_id, f.file_name, f.file_url, f.file_size, f.file_format,
			f.file_year, f.file_type, f.file_school, f.file_subject, f.reviewer, f.file_status, f.create_by, f.create_time, f.remark,
			COALESCE(NULLIF(u.user_name, ''), f.create_by) AS display_name
		FROM ptmj_file f
		LEFT JOIN ptmj_user u ON u.user_id = f.user_id`+where+`
		ORDER BY f.create_time DESC, f.file_id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return examFileAuditPage{}, err
	}
	items := make([]examFileAuditItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, examFileAuditItemFromRow(row))
	}
	return examFileAuditPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func examFileAuditItemFromRow(row map[string]interface{}) examFileAuditItem {
	return examFileAuditItem{
		FileID:      datum.ScanInt64(row["file_id"]),
		UserID:      datum.ScanInt64(row["user_id"]),
		UserName:    datum.ScanString(row["display_name"]),
		FileName:    datum.ScanString(row["file_name"]),
		FileURL:     datumReadableFileURL(datum.ScanString(row["file_url"])),
		FileSize:    datum.ScanInt64(row["file_size"]),
		FileFormat:  datum.ScanString(row["file_format"]),
		FileYear:    datum.ScanInt64(row["file_year"]),
		FileType:    datum.ScanInt64(row["file_type"]),
		FileSchool:  datum.ScanString(row["file_school"]),
		FileSubject: datum.ScanString(row["file_subject"]),
		Reviewer:    datum.ScanString(row["reviewer"]),
		FileStatus:  datum.ScanInt64(row["file_status"]),
		CreateBy:    datum.ScanString(row["display_name"]),
		CreateTime:  examFileAuditTimeText(row["create_time"]),
		Remark:      datum.ScanString(row["remark"]),
	}
}

// examFileAuditTimeText 统一时间为 "2006-01-02 15:04:05"：PG 驱动可能返回
// time.Time、RFC3339 或无时区的 "2026-09-26T14:42:20.124345" 原文。
func examFileAuditTimeText(value interface{}) string {
	text := strings.TrimSpace(datum.ScanString(value))
	if text == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return t.In(time.Local).Format("2006-01-02 15:04:05")
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999",
	} {
		if t, err := time.Parse(layout, text); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
	}
	return text
}

// ---------------------------------------------------------------------------
// actions
// ---------------------------------------------------------------------------

func (s *Store) examFileAuditReviewer(c *gin.Context) string {
	// requireAuth 以 int64 写入 vben_user_id（gin.GetString 对非 string 会取空）。
	if raw, ok := c.Get("vben_user_id"); ok {
		if id, ok := raw.(int64); ok && id > 0 {
			return "admin:" + strconv.FormatInt(id, 10)
		}
	}
	return "admin"
}

// swaggerExamFileAuditApprove documents POST /exam-file-audit/approve.
// @Summary 多选批量通过（仅待审核行，0→1）
// @Tags 文件审核
// @Security BearerAuth
// @Param payload body object true "{\"fileIds\": [1007]}"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Router /exam-file-audit/approve [post]
func (s *Store) examFileAuditApprove(c *gin.Context) {
	var req struct {
		FileIds []int64 `json:"fileIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.FileIds) == 0 {
		httpx.Fail(c, http.StatusBadRequest, "请选择要通过的文件")
		return
	}
	if len(req.FileIds) > 200 {
		httpx.Fail(c, http.StatusBadRequest, "单次最多通过 200 个文件")
		return
	}
	ids := make([]int64, 0, len(req.FileIds))
	seen := make(map[int64]bool, len(req.FileIds))
	placeholders := make([]string, 0, len(req.FileIds))
	for _, id := range req.FileIds {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		placeholders = append(placeholders, "?")
	}
	if len(ids) == 0 {
		httpx.Fail(c, http.StatusBadRequest, "请选择要通过的文件")
		return
	}
	reviewer := s.examFileAuditReviewer(c)
	queryArgs := append([]interface{}{reviewer, reviewer}, idsToInterfaces(ids)...)
	result, err := s.conn.Exec(`UPDATE ptmj_file SET file_status = 1, reviewer = ?, update_by = ?, update_time = CURRENT_TIMESTAMP
		WHERE file_id IN (`+strings.Join(placeholders, ",")+`) AND file_status = 0 AND del_flag = 0`, queryArgs...)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	approved, _ := result.RowsAffected()
	s.invalidateDatumFiles()
	httpx.Success(c, gin.H{"approved": approved})
}

// swaggerExamFileAuditApproveUser documents POST /exam-file-audit/approve-user.
// @Summary 单个用户的全部待审核文件一键通过
// @Tags 文件审核
// @Security BearerAuth
// @Param payload body object true "{\"userId\": 455}"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Router /exam-file-audit/approve-user [post]
func (s *Store) examFileAuditApproveUser(c *gin.Context) {
	var req struct {
		UserId int64 `json:"userId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 {
		httpx.Fail(c, http.StatusBadRequest, "用户 ID 不正确")
		return
	}
	reviewer := s.examFileAuditReviewer(c)
	approved, err := datum.NewFileRepo(s.conn).ApproveAllByUser(req.UserId, reviewer)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	s.invalidateDatumFiles()
	httpx.Success(c, gin.H{"approved": approved})
}

// swaggerExamFileAuditReject documents POST /exam-file-audit/reject.
// @Summary 拒绝待审核文件（0→2，原因必填并通知用户）
// @Tags 文件审核
// @Security BearerAuth
// @Param payload body object true "{\"fileId\": 1007, \"reason\": \"清晰度不足\"}"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Failure 404 {object} object
// @Router /exam-file-audit/reject [post]
func (s *Store) examFileAuditReject(c *gin.Context) {
	var req struct {
		FileId int64  `json:"fileId"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.FileId <= 0 {
		httpx.Fail(c, http.StatusBadRequest, "文件 ID 不正确")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		httpx.Fail(c, http.StatusBadRequest, "请填写拒绝原因")
		return
	}
	file, found, err := datum.NewFileRepo(s.conn).FindByID(req.FileId)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !found || file.FileStatus != 0 {
		httpx.Fail(c, http.StatusNotFound, "文件不存在或状态已变化")
		return
	}
	reviewer := s.examFileAuditReviewer(c)
	if err := datum.NewFileRepo(s.conn).SetStatus(req.FileId, 2, reviewer); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if notifyErr := datum.NewNotificationRepo(s.conn).CreateFileAuditNotification(
		file.FileID, file.UserID,
		fmt.Sprintf("您的资料《%s》未通过审核", file.FileName),
		reason,
	); notifyErr != nil {
		fmt.Printf("kadmin: file audit reject notification failed: %v\n", notifyErr)
	}
	s.invalidateDatumFiles()
	httpx.Success(c, gin.H{"fileId": req.FileId, "decision": "reject"})
}

// swaggerExamFileAuditReview documents POST /exam-file-audit/audit.
// @Summary 被举报文件复审（approve=举报不属实恢复上架，reject=举报属实下架）
// @Tags 文件审核
// @Security BearerAuth
// @Param payload body object true "{\"fileId\": 1007, \"decision\": \"approve|reject\", \"reason\": \"复审原因\"}"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Failure 404 {object} object
// @Router /exam-file-audit/audit [post]
func (s *Store) examFileAuditReview(c *gin.Context) {
	var req struct {
		FileId   int64  `json:"fileId"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.FileId <= 0 {
		httpx.Fail(c, http.StatusBadRequest, "文件 ID 不正确")
		return
	}
	if req.Decision != "approve" && req.Decision != "reject" {
		httpx.Fail(c, http.StatusBadRequest, "审核结论不正确：approve-通过 reject-拒绝")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		httpx.Fail(c, http.StatusBadRequest, "请填写审核原因")
		return
	}
	fileRepo := datum.NewFileRepo(s.conn)
	file, found, err := fileRepo.FindByID(req.FileId)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !found || file.FileStatus != 3 {
		httpx.Fail(c, http.StatusNotFound, "文件不存在或不在被举报状态")
		return
	}
	reportID, err := s.examFileAuditLatestPendingReport(req.FileId)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if reportID <= 0 {
		httpx.Fail(c, http.StatusBadRequest, "该文件没有待处理的举报")
		return
	}

	reviewer := s.examFileAuditReviewer(c)
	notificationRepo := datum.NewNotificationRepo(s.conn)
	if req.Decision == "approve" {
		// 举报不属实：文件恢复上架，举报单置不属实并记录原因。
		if err := fileRepo.SetStatus(req.FileId, 1, reviewer); err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if err := datum.NewReportRepo(s.conn).SetResult(reportID, "2", reviewer, reason); err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		// 举报属实：文件下架，举报单置属实并记录原因，原因随下架通知返回用户。
		if err := fileRepo.SetStatus(req.FileId, 2, reviewer); err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if err := datum.NewReportRepo(s.conn).SetResult(reportID, "1", reviewer, reason); err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if notifyErr := notificationRepo.CreateFileAuditNotification(
			file.FileID, file.UserID,
			fmt.Sprintf("您的资料《%s》因举报属实已下架", file.FileName),
			reason,
		); notifyErr != nil {
			fmt.Printf("kadmin: file audit review notification failed: %v\n", notifyErr)
		}
	}
	s.invalidateDatumFiles()
	httpx.Success(c, gin.H{"fileId": req.FileId, "decision": req.Decision})
}

func (s *Store) examFileAuditLatestPendingReport(fileID int64) (int64, error) {
	rows, err := s.conn.Query(`SELECT report_id FROM ptmj_report WHERE file_id = ? AND result = '0'
		ORDER BY report_id DESC LIMIT 1`, fileID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return datum.ScanInt64(rows[0]["report_id"]), nil
}

// ensureExamFileAuditMenu inserts the audit workbench menu under /business
// when missing (same heal-on-startup contract as the generated modules).
func (s *Store) ensureExamFileAuditMenu() error {
	parentRows, err := s.conn.Query(`SELECT id FROM goadmin_menu WHERE uri = ?`, "/business")
	if err != nil {
		return err
	}
	if len(parentRows) == 0 {
		return fmt.Errorf("parent menu /business is missing")
	}
	parentID := datum.ScanInt64(parentRows[0]["id"])
	rows, err := s.conn.Query(`SELECT id FROM goadmin_menu WHERE uri = ?`, "/exam-file-audit")
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return nil
	}
	_, err = s.conn.Exec(`INSERT INTO goadmin_menu
		(parent_id, type, "order", title, icon, uri, plugin_name, component, created_at, updated_at)
		VALUES (?, 1, 98, ?, 'lucide:file-check', ?, '', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		parentID, "文件审核", "/exam-file-audit", "/kadmin/components/FileAuditView")
	return err
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func examFileAuditPageParams(c *gin.Context) (int, int) {
	page := examFileAuditPositiveInt(c.Query("page"), 1)
	pageSize := examFileAuditPositiveInt(c.Query("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func examFileAuditPositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func examFileAuditInt64(value string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func idsToInterfaces(ids []int64) []interface{} {
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	return args
}
