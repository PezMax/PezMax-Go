package kadmin

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/fileurl"
	"github.com/gin-gonic/gin"
)

// Desktop ebook module：电子书目录在 ptmj_ebook（后台经 codegen 模块管理，
// 审核语义与 ptmj_file 对齐）。桌面端契约：
//
//   - GET /datum/ebook/list     分页的已上架电子书（仅 ebook_status=1；
//     不做年份/学校细分，页面直接按书名展示，关键词匹配书名/作者/出版社）
//   - GET /datum/ebook/content  在线预览流（inline；pdf/epub 常见格式，
//     pdf 走 Chromium 内建阅读器，epub 由渲染端 epub.js 解析）
//   - 下载设专有端点 GET /datum/download/ebook?ebookId=（datum 会话鉴权、
//     落 ptmj_ebook_download 记录、复用同一条 nginx 边缘整形路径）。
//
// 电子书可携带的内容格式由 datumAllowedExts 保证（pdf/epub/mobi/azw3）。

const (
	datumEbookPreviewFormats = "pdf/epub 在线预览；mobi/azw3 请下载后阅读"
	datumEbookDefaultSubject = "综合"
)

func (s *Store) registerDatumEbookRoutes(datumGroup *gin.RouterGroup) {
	// gin v1.3 的 GET 路由树不允许静态子节点与 :param 并存，统一走通配分发器；
	// POST/DELETE 树无通配冲突，直接注册专有路径。
	ebooks := datumGroup.Group("/ebook")
	ebooks.GET("/*rest", s.datumEbookGet)
	ebooks.POST("/favorite", s.requireDatumAuth(), s.datumEbookFavoriteAdd)
	ebooks.DELETE("/favorite/:ebookId", s.requireDatumAuth(), s.datumEbookFavoriteRemove)
	ebooks.POST("/report", s.requireDatumAuth(), s.datumEbookReportCreate)
}

// datumEbookGet dispatches the read-only ebook routes:
// /list、/content、/subjects、/favorite（收藏状态查询）。
func (s *Store) datumEbookGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "list":
		s.datumEbookList(c)
	case rest == "content":
		s.datumEbookContent(c)
	case rest == "subjects":
		s.datumEbookSubjects(c)
	case rest == "favorite":
		s.datumEbookFavoriteStatus(c)
	case rest == "favorite/list":
		s.datumEbookFavoriteList(c)
	default:
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

// datumEbookPayload shapes one ptmj_ebook row for the desktop bookshelf:
// 封面与正文 URL 统一走 datumReadableFileURL（minio:// 改写为公网可读地址）。
func datumEbookPayload(ebook datum.Ebook) gin.H {
	return gin.H{
		"id":           ebook.EbookID,
		"ebookId":      ebook.EbookID,
		"label":        ebook.EbookName,
		"ebookName":    ebook.EbookName,
		"author":       ebook.Author,
		"publisher":    ebook.Publisher,
		"coverUrl":     datumReadableFileURL(ebook.CoverURL),
		"ebookUrl":     datumReadableFileURL(ebook.EbookURL),
		"ebookSize":    ebook.EbookSize,
		"ebookFormat":  ebook.EbookFormat,
		"ebookSubject": ebook.EbookSubject,
		"ebookType":    ebook.EbookType,
		"ebookStatus":  ebook.EbookStatus,
		"userId":       ebook.UserID,
		"remark":       ebook.Remark,
		"type":         "ebook",
	}
}

// datumEbookList serves the paged approved ebook list. Anonymous browsing
// matches /datum/file/list 语义（只出已过审内容）；书名/作者/出版社关键词联想。
func (s *Store) datumEbookList(c *gin.Context) {
	page, size := datumPageParams(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	if len(keyword) > 100 {
		keyword = keyword[:100]
	}
	result, err := datum.NewEbookRepo(s.conn).List(datum.EbookFilter{
		Page:         page,
		PageSize:     size,
		Keyword:      keyword,
		EbookSubject: strings.TrimSpace(c.Query("subject")),
		EbookType:    datumQueryInt(c, "ebookType"),
		OnlyApproved: true,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "电子书列表查询失败")
		return
	}
	items, _ := result.Items.([]datum.Ebook)
	rows := make([]gin.H, 0, len(items))
	for _, ebook := range items {
		rows = append(rows, datumEbookPayload(ebook))
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"msg":     "ok",
		"rows":    rows,
		"total":   result.Total,
	})
}

// datumEbookSubjects serves the distinct subjects of approved ebooks, feeding
// the desktop ebook tree（学科 → 书，与 /datum/file/subjects 同语义）.
func (s *Store) datumEbookSubjects(c *gin.Context) {
	subjects, err := datum.NewEbookRepo(s.conn).Subjects()
	if err != nil {
		fail(c, http.StatusInternalServerError, "电子书学科查询失败")
		return
	}
	success(c, subjects)
}

type datumEbookFavoriteAddRequest struct {
	EbookID int64 `json:"ebookId"`
	UserID  int64 `json:"userId"`
}

// datumEbookFavoriteAdd implements POST /datum/ebook/favorite — mirrors the
// file favorite contract: session identity, target must be visible to the
// caller, duplicate association answers 409.
func (s *Store) datumEbookFavoriteAdd(c *gin.Context) {
	var req datumEbookFavoriteAddRequest
	_ = c.ShouldBind(&req)
	userID, ok := datumSessionUser(c, req.UserID)
	if !ok {
		return
	}
	if req.EbookID <= 0 {
		fail(c, http.StatusBadRequest, "电子书 ID 不能为空")
		return
	}
	ebook, found, err := datum.NewEbookRepo(s.conn).FindByID(req.EbookID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "电子书查询失败")
		return
	}
	if !found || (ebook.EbookStatus != 1 && ebook.UserID != userID) {
		fail(c, http.StatusNotFound, "电子书不存在或未上架")
		return
	}
	repo := datum.NewEbookFavoriteRepo(s.conn)
	if exists, err := repo.Exists(req.EbookID, userID); err == nil && exists {
		fail(c, http.StatusConflict, "收藏已存在")
		return
	}
	if err := repo.Add(req.EbookID, userID); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "收藏已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "收藏失败")
		return
	}
	success(c, true)
}

// datumUserIDOptional resolves the datum session user without requiring it:
// the shared GET wildcard dispatcher carries no auth middleware, so favorite
// status lookups re-resolve the token themselves. Anonymous callers get 0.
func (s *Store) datumUserIDOptional(c *gin.Context) (int64, bool) {
	if userID, ok := datumUserIDFrom(c); ok {
		return userID, true
	}
	token := tokenFromRequest(c)
	if token == "" || s.datum == nil {
		return 0, false
	}
	userID, err := s.datum.ResolveSession(token)
	if err != nil {
		return 0, false
	}
	return userID, true
}

// datumEbookFavoriteStatus implements GET /datum/ebook/favorite?ebookId= —
// anonymous callers simply get favorited=false（与 /datum/favorite/:fileId 同语义）.
func (s *Store) datumEbookFavoriteStatus(c *gin.Context) {
	userID, _ := s.datumUserIDOptional(c)
	ebookID := toDatumInt64(strings.TrimSpace(c.Query("ebookId")))
	if ebookID <= 0 {
		fail(c, http.StatusBadRequest, "电子书 ID 不能为空")
		return
	}
	favorited := false
	if userID > 0 {
		if exists, err := datum.NewEbookFavoriteRepo(s.conn).Exists(ebookID, userID); err == nil && exists {
			favorited = true
		}
	}
	success(c, gin.H{"favorited": favorited})
}

// datumEbookFavoriteList implements GET /datum/ebook/favorite/list — the
// session user's favorite ebook ids（桌面端收藏状态集合用；匿名返回空表）.
func (s *Store) datumEbookFavoriteList(c *gin.Context) {
	userID, _ := s.datumUserIDOptional(c)
	ids := []int64{}
	if userID > 0 {
		var err error
		ids, err = datum.NewEbookFavoriteRepo(s.conn).ListByUser(userID)
		if err != nil {
			fail(c, http.StatusInternalServerError, "收藏列表查询失败")
			return
		}
	}
	success(c, ids)
}

// datumEbookFavoriteRemove implements DELETE /datum/ebook/favorite/:ebookId —
// session user only; removing an absent association answers 404 like the
// file favorite removal does.
func (s *Store) datumEbookFavoriteRemove(c *gin.Context) {
	userID, ok := datumUserIDFrom(c)
	if !ok {
		fail(c, http.StatusUnauthorized, "会话已过期，请重新登录")
		return
	}
	ebookID, err := strconv.ParseInt(c.Param("ebookId"), 10, 64)
	if err != nil || ebookID <= 0 {
		fail(c, http.StatusBadRequest, "电子书 ID 不正确")
		return
	}
	if err := datum.NewEbookFavoriteRepo(s.conn).Remove(ebookID, userID); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "收藏关系不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "取消收藏失败")
		return
	}
	success(c, true)
}

type datumEbookReportCreateRequest struct {
	EbookID int64  `json:"ebookId"`
	UserID  int64  `json:"userId"`
	Reason  string `json:"reason"`
	Remark  string `json:"remark"`
}

// datumEbookReportCreate implements POST /datum/ebook/report — same contract
// as /datum/report: reason required, cannot report own ebook, one pending
// report per (user, ebook)（uk_user_ebook 兜底 duplicate key）.
func (s *Store) datumEbookReportCreate(c *gin.Context) {
	var req datumEbookReportCreateRequest
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
	ebook, found, err := datum.NewEbookRepo(s.conn).FindByID(req.EbookID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "电子书查询失败")
		return
	}
	if !found || (ebook.EbookStatus != 1 && ebook.UserID != userID) {
		fail(c, http.StatusNotFound, "电子书不存在或未上架")
		return
	}
	if ebook.UserID == userID {
		fail(c, http.StatusBadRequest, "不能举报自己的电子书")
		return
	}
	reportID, err := datum.NewEbookReportRepo(s.conn).Create(req.EbookID, userID, reason, strings.TrimSpace(req.Remark))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "该电子书已有你提交的举报，请等待审核")
			return
		}
		fail(c, http.StatusInternalServerError, "举报提交失败")
		return
	}
	success(c, gin.H{"reportId": reportID})
}

// datumEbookContent implements GET /datum/ebook/content?ebookId= — the
// inline preview stream. 与试卷预览同语义：已过审内容匿名可读，属主可看
// 自己未过审的书；inline 便于 Chromium 内建 PDF 阅读器与 epub.js 拉流，
// 不落下载记录（下载走 /datum/download/ebook）。
func (s *Store) datumEbookContent(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	ebookID := toDatumInt64(strings.TrimSpace(c.Query("ebookId")))
	if ebookID <= 0 {
		fail(c, http.StatusBadRequest, "电子书 ID 不能为空")
		return
	}
	ebook, found, err := datum.NewEbookRepo(s.conn).FindByID(ebookID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "电子书查询失败")
		return
	}
	if !found || (ebook.EbookStatus != 1 && ebook.UserID != userID) {
		fail(c, http.StatusNotFound, "电子书不存在或未上架")
		return
	}
	bucket, objectKey, parseErr := fileurl.Parse(ebook.EbookURL)
	if parseErr != nil {
		fail(c, http.StatusNotFound, "电子书内容不可用")
		return
	}
	body, info, err := openDatumObject(c.Request.Context(), bucket, objectKey)
	if err != nil {
		fail(c, http.StatusNotFound, "电子书内容不可用")
		return
	}
	defer body.Close()

	contentType := mimeByExt(strings.ToLower(ebook.EbookFormat))
	fileName := path.Base(strings.ReplaceAll(ebook.EbookName, "\\", "/"))
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": fileName})
	if disposition == "" {
		disposition = "inline"
	}
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", contentType)
	// 强 ETag 支撑客户端缓存与 If-Range 续拉：对象键写入后不变，
	// id+size 足以标识字节表示（与 /datum/download/file 同款约定）。
	if info.Size > 0 {
		c.Header("ETag", fmt.Sprintf(`"ebook-%d-%d"`, ebook.EbookID, info.Size))
	}
	if seeker, ok := body.(io.ReadSeeker); ok {
		http.ServeContent(c.Writer, c.Request, fileName, time.Time{}, seeker)
		return
	}
	if info.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	c.DataFromReader(http.StatusOK, info.Size, contentType, body, map[string]string{})
}
