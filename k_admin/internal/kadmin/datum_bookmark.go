package kadmin

import (
	"errors"
	"io"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/gin-gonic/gin"
)

// Desktop bookmark module: anonymous list/detail over approved-and-alive
// rows, owner-scoped CRUD, and cover upload to object storage. Paths mirror
// the legacy /datum/bookmark contract consumed by PezMax-Desktop.

const datumBookmarkCoverMaxBytes = 5 << 20 // 5MB

// ---------------------------------------------------------------------------
// routes
// ---------------------------------------------------------------------------

func (s *Store) registerDatumBookmarkRoutes(datumGroup *gin.RouterGroup) {
	bookmarks := datumGroup.Group("/bookmark")
	// 与 datum/file 相同的 gin(v1.3) 限制：GET 静态 /list 与 /:id 不能并存，
	// 读接口全部走通配分发；写接口无冲突，直接注册。
	bookmarks.GET("/*rest", s.datumBookmarkGet)
	bookmarks.POST("", s.requireDatumAuth(), s.datumBookmarkCreate)
	bookmarks.PUT("", s.requireDatumAuth(), s.datumBookmarkUpdate)
	bookmarks.DELETE("/:bookmarkId", s.requireDatumAuth(), s.datumBookmarkDelete)
	bookmarks.POST("/uploadCover", s.requireDatumAuth(), s.datumBookmarkUploadCover)
}

// datumBookmarkGet dispatches the read-only routes: /list, /{bookmarkId} and
// the bookmark-favorite relation list (gin v1.3 can't mix that static GET
// subtree with this wildcard, so the activity module delegates it here).
func (s *Store) datumBookmarkGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "list":
		s.datumBookmarkList(c)
	case rest == "favorite/list":
		s.requireDatumAuth()(c)
		if c.IsAborted() {
			return
		}
		s.datumBookmarkFavoriteRelations(c)
	default:
		if bookmarkID, err := strconv.ParseInt(rest, 10, 64); err == nil && bookmarkID > 0 {
			c.Params = append(c.Params, gin.Param{Key: "bookmarkId", Value: rest})
			s.datumBookmarkDetail(c)
			return
		}
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

// ---------------------------------------------------------------------------
// anonymous reads
// ---------------------------------------------------------------------------

func (s *Store) datumBookmarkList(c *gin.Context) {
	page, size := datumPageParams(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		// 桌面端侧栏搜索框以 title 传参，落为标题/描述/URL 模糊匹配
		keyword = strings.TrimSpace(c.Query("title"))
	}
	filter := datum.BookmarkFilter{
		Page:         page,
		PageSize:     size,
		Subject:      c.Query("subject"),
		ResourceType: c.Query("resourceType"),
		Collection:   c.Query("collection"),
		URL:          c.Query("url"),
		Keyword:      keyword,
		// 与文件列表同例：匿名仅出已审核；带 userId 时出该用户全部状态（我的书签管理）
		OnlyApproved: strings.TrimSpace(c.Query("userId")) == "",
	}
	if raw := strings.TrimSpace(c.Query("userId")); raw != "" {
		filter.UserID = toDatumInt64(raw)
	}
	result, err := datum.NewBookmarkRepo(s.conn).List(filter)
	if err != nil {
		fail(c, http.StatusInternalServerError, "书签列表查询失败")
		return
	}
	items, _ := result.Items.([]datum.Bookmark)
	rows := make([]gin.H, 0, len(items))
	for _, bookmark := range items {
		rows = append(rows, datumBookmarkPayload(bookmark))
	}
	// RuoYi TableDataInfo 形状：rows/total 在顶层（桌面端直接读 response.rows/total）
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"msg":     "ok",
		"rows":    rows,
		"total":   result.Total,
	})
}

func (s *Store) datumBookmarkDetail(c *gin.Context) {
	bookmarkID, ok := datumPathInt64(c, "bookmarkId")
	if !ok {
		return
	}
	bookmark, found, err := datum.NewBookmarkRepo(s.conn).FindByID(bookmarkID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "书签查询失败")
		return
	}
	if !found {
		fail(c, http.StatusNotFound, "书签不存在或已删除")
		return
	}
	// 匿名只看已审核；属主凭会话可见自己的待审书签
	if bookmark.Status != 1 && !s.datumTokenOwns(c, bookmark.UserID) {
		fail(c, http.StatusNotFound, "书签不存在或已删除")
		return
	}
	success(c, datumBookmarkPayload(bookmark))
}

func datumBookmarkPayload(bookmark datum.Bookmark) gin.H {
	return gin.H{
		"id":           bookmark.ID,
		"bookmarkId":   bookmark.ID,
		"userId":       bookmark.UserID,
		"url":          bookmark.URL,
		"title":        bookmark.Title,
		"description":  bookmark.Description,
		"coverImage":   datumReadableFileURL(bookmark.CoverImage),
		"subject":      bookmark.Subject,
		"resourceType": bookmark.ResourceType,
		"collection":   bookmark.Collection,
		"status":       bookmark.Status,
		"remark":       bookmark.Remark,
		"createBy":     bookmark.CreateBy,
		"createTime":   bookmark.CreateTime,
		"updateTime":   bookmark.UpdateTime,
	}
}

// ---------------------------------------------------------------------------
// owner writes: create / update / delete
// ---------------------------------------------------------------------------

type datumBookmarkWriteRequest struct {
	URL          string `json:"url"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	CoverImage   string `json:"coverImage"`
	Subject      string `json:"subject"`
	ResourceType string `json:"resourceType"`
	Collection   string `json:"collection"`
	Remark       string `json:"remark"`
}

// validateBookmarkWrite enforces the ptmj_bookmark column limits (url 512,
// title 500, cover 1000, subject 64, resource_type 32, collection 64).
func validateBookmarkWrite(req *datumBookmarkWriteRequest) string {
	req.URL = strings.TrimSpace(req.URL)
	req.Title = strings.TrimSpace(req.Title)
	if req.URL == "" || len(req.URL) > 512 {
		return "网址不能为空且不超过 512 个字符"
	}
	if req.Title == "" || len([]rune(req.Title)) > 500 {
		return "标题不能为空且不超过 500 个字符"
	}
	if len(req.CoverImage) > 1000 {
		return "封面地址不能超过 1000 个字符"
	}
	if len([]rune(req.Subject)) > 64 {
		return "科目不能超过 64 个字符"
	}
	if len([]rune(req.ResourceType)) > 32 {
		return "资源类型不能超过 32 个字符"
	}
	if len([]rune(req.Collection)) > 64 {
		return "合集不能超过 64 个字符"
	}
	if req.ResourceType == "" {
		req.ResourceType = "other"
	}
	return ""
}

func (s *Store) datumBookmarkCreate(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumBookmarkWriteRequest
	_ = c.ShouldBind(&req)
	if message := validateBookmarkWrite(&req); message != "" {
		fail(c, http.StatusBadRequest, message)
		return
	}
	bookmarkID, err := datum.NewBookmarkRepo(s.conn).Insert(datum.BookmarkPayload{
		UserID: userID, URL: req.URL, Title: req.Title, Description: req.Description,
		CoverImage: req.CoverImage, Subject: req.Subject, ResourceType: req.ResourceType,
		Collection: req.Collection, Status: 0, Remark: req.Remark,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "书签保存失败")
		return
	}
	// 双键返回：桌面端 resolveBookmarkId 取 data.bookmarkId 或 data.id 均可
	success(c, gin.H{"bookmarkId": bookmarkID, "id": bookmarkID, "status": 0})
}

// datumBookmarkUpdateRequest accepts the desktop's partial updates (cover
// sync sends {id, coverImage} only, with id as a JS string) plus the
// full-row edits from the upload manager; unprovided fields keep their
// stored values. Extra row fields the desktop spreads back
// (status/userId/createTime...) are ignored.
type datumBookmarkUpdateRequest struct {
	ID           datumFlexID `json:"id"`
	BookmarkID   datumFlexID `json:"bookmarkId"`
	URL          *string     `json:"url"`
	Title        *string     `json:"title"`
	Description  *string     `json:"description"`
	CoverImage   *string     `json:"coverImage"`
	Subject      *string     `json:"subject"`
	ResourceType *string     `json:"resourceType"`
	Collection   *string     `json:"collection"`
	Remark       *string     `json:"remark"`
}

// datumFlexID accepts a JSON number or numeric string — the desktop's
// resolveBookmarkId hands back ids as strings while the upload manager
// spreads them as numbers.
type datumFlexID int64

func (v *datumFlexID) UnmarshalJSON(raw []byte) error {
	text := strings.Trim(string(raw), `"`)
	if text == "" || text == "null" {
		*v = 0
		return nil
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		return err
	}
	*v = datumFlexID(parsed)
	return nil
}

func (v datumFlexID) value() int64 { return int64(v) }

func (s *Store) datumBookmarkUpdate(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumBookmarkUpdateRequest
	_ = c.ShouldBind(&req)
	bookmarkID := req.ID.value()
	if bookmarkID <= 0 {
		bookmarkID = req.BookmarkID.value()
	}
	if bookmarkID <= 0 {
		fail(c, http.StatusBadRequest, "书签 ID 不能为空")
		return
	}
	repo := datum.NewBookmarkRepo(s.conn)
	existing, found, err := repo.FindByID(bookmarkID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "书签查询失败")
		return
	}
	if !found || existing.UserID != userID {
		fail(c, http.StatusNotFound, "书签不存在或无权修改")
		return
	}
	merged := datumBookmarkWriteRequest{
		URL: existing.URL, Title: existing.Title, Description: existing.Description,
		CoverImage: existing.CoverImage, Subject: existing.Subject,
		ResourceType: existing.ResourceType, Collection: existing.Collection, Remark: existing.Remark,
	}
	if req.URL != nil {
		merged.URL = *req.URL
	}
	if req.Title != nil {
		merged.Title = *req.Title
	}
	if req.Description != nil {
		merged.Description = *req.Description
	}
	if req.CoverImage != nil {
		merged.CoverImage = *req.CoverImage
	}
	if req.Subject != nil {
		merged.Subject = *req.Subject
	}
	if req.ResourceType != nil {
		merged.ResourceType = *req.ResourceType
	}
	if req.Collection != nil {
		merged.Collection = *req.Collection
	}
	if req.Remark != nil {
		merged.Remark = *req.Remark
	}
	if message := validateBookmarkWrite(&merged); message != "" {
		fail(c, http.StatusBadRequest, message)
		return
	}
	payload := datum.BookmarkPayload{
		UserID: userID, URL: merged.URL, Title: merged.Title, Description: merged.Description,
		CoverImage: merged.CoverImage, Subject: merged.Subject, ResourceType: merged.ResourceType,
		Collection: merged.Collection, Remark: merged.Remark,
	}
	if err := repo.UpdateOwner(bookmarkID, userID, payload); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "书签不存在或无权修改")
			return
		}
		fail(c, http.StatusInternalServerError, "书签更新失败")
		return
	}
	success(c, true)
}

func (s *Store) datumBookmarkDelete(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	bookmarkID, ok := datumPathInt64(c, "bookmarkId")
	if !ok {
		return
	}
	if err := datum.NewBookmarkRepo(s.conn).MarkDeleted(bookmarkID, userID); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "书签不存在或无权删除")
			return
		}
		fail(c, http.StatusInternalServerError, "书签删除失败")
		return
	}
	success(c, true)
}

// ---------------------------------------------------------------------------
// cover upload
// ---------------------------------------------------------------------------

// datumBookmarkUploadCover stores the cover image and syncs cover_image on
// the row. The desktop additionally PUTs {id, coverImage} afterwards; doing
// both keeps the row correct even if that follow-up call fails.
func (s *Store) datumBookmarkUploadCover(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	bookmarkID := toDatumInt64(strings.TrimSpace(c.PostForm("bookmarkId")))
	if bookmarkID <= 0 {
		// 桌面端 metadata 同时携带 id 字段
		bookmarkID = toDatumInt64(strings.TrimSpace(c.PostForm("id")))
	}
	if bookmarkID <= 0 {
		fail(c, http.StatusBadRequest, "书签 ID 不能为空")
		return
	}
	repo := datum.NewBookmarkRepo(s.conn)
	// 先验属主再落存储：非属主请求不允许在对象存储里留下孤儿文件
	existing, found, err := repo.FindByID(bookmarkID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "书签查询失败")
		return
	}
	if !found || existing.UserID != userID {
		fail(c, http.StatusNotFound, "书签不存在或无权修改")
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "封面文件不能为空")
		return
	}
	if file.Size <= 0 || file.Size > datumBookmarkCoverMaxBytes {
		fail(c, http.StatusBadRequest, "封面文件过大：不能超过 5MB")
		return
	}
	opened, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "封面文件读取失败")
		return
	}
	defer opened.Close()
	body, err := io.ReadAll(io.LimitReader(opened, datumBookmarkCoverMaxBytes+1))
	if err != nil || len(body) == 0 || int64(len(body)) > datumBookmarkCoverMaxBytes {
		fail(c, http.StatusBadRequest, "封面文件读取失败")
		return
	}
	contentType := http.DetectContentType(body)
	var ext string
	switch contentType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	default:
		fail(c, http.StatusBadRequest, "封面仅支持 JPG / PNG / GIF / WEBP 格式")
		return
	}
	randomName, err := randomHex(12)
	if err != nil {
		fail(c, http.StatusInternalServerError, "封面上传失败，请稍后重试")
		return
	}
	objectKey := path.Join("bookmarks", strconv.FormatInt(bookmarkID, 10), time.Now().Format("20060102"), randomName+ext)
	coverURL, err := datumFilePut(c.Request.Context(), objectKey, body, contentType)
	if err != nil {
		log.Printf("datum bookmark cover storage put failed: %v", err)
		fail(c, http.StatusServiceUnavailable, "封面存储暂不可用，请稍后重试")
		return
	}
	if err := repo.UpdateCover(bookmarkID, userID, coverURL); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "书签不存在或无权修改")
			return
		}
		fail(c, http.StatusInternalServerError, "封面更新失败")
		return
	}
	success(c, gin.H{"bookmarkId": bookmarkID, "url": coverURL, "fileUrl": coverURL})
}
