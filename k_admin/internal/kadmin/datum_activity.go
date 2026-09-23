package kadmin

import (
	"context"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/fileurl"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/storage"
	"github.com/gin-gonic/gin"
)

// Desktop download/favorite module (A5): download records (CRUD + streaming
// download that appends a record), file favorites, bookmark favorites. Every
// route requires a datum session; any userId passed in the path or body must
// match the session user — the desktop endpoints are strictly self-service.

func (s *Store) registerDatumActivityRoutes(datumGroup *gin.RouterGroup) {
	// 下载记录：GET 树里静态 "file"（流式下载）与 :downloadId 并存冲突，
	// gin v1.3 下统一走通配分发器。
	downloads := datumGroup.Group("/download", s.requireDatumAuth())
	downloads.GET("/*rest", s.datumDownloadGet)
	downloads.POST("", s.datumDownloadCreate)
	downloads.PUT("", s.datumDownloadUpdate)
	downloads.DELETE("/:ids", s.datumDownloadDelete)

	desktop := datumGroup.Group("/desktop", s.requireDatumAuth())
	desktop.GET("/download/list/:userId", s.datumMyDownloads)
	desktop.DELETE("/download/:userId/:fileId", s.datumMyDownloadRemove)
	desktop.GET("/favorite/list/:userId", s.datumMyFavorites)
	desktop.DELETE("/favorite/:userId/:fileId", s.datumMyFavoriteRemove)
	desktop.GET("/bookmark/favorite/list/:userId", s.datumMyBookmarkFavorites)
	desktop.DELETE("/bookmark/favorite/:userId/:bookmarkId", s.datumMyBookmarkFavoriteRemove)

	favorites := datumGroup.Group("/favorite", s.requireDatumAuth())
	favorites.POST("", s.datumFavoriteAdd)
	favorites.GET("/:fileId", s.datumFavoriteStatus)

	bookmarkFavorites := datumGroup.Group("/bookmark/favorite", s.requireDatumAuth())
	// GET /list 与 datum/bookmark 的通配分发器在 gin v1.3 GET 树中冲突，
	// 由 datum_bookmark.go 的 datumBookmarkGet 统一分发到 datumBookmarkFavoriteRelations。
	bookmarkFavorites.POST("", s.datumBookmarkFavoriteAdd)
}

// datumSessionUser resolves the session user and rejects mismatches with a
// userId taken from the path or body.
func datumSessionUser(c *gin.Context, claimed int64) (int64, bool) {
	userID, ok := datumUserIDFrom(c)
	if !ok {
		fail(c, http.StatusUnauthorized, "会话已过期，请重新登录")
		return 0, false
	}
	if claimed != 0 && claimed != userID {
		fail(c, http.StatusForbidden, "没有权限")
		return 0, false
	}
	return userID, true
}

// ---------------------------------------------------------------------------
// streaming download + records
// ---------------------------------------------------------------------------

// datumDownloadGet dispatches GET /datum/download/{file|:downloadId}.
func (s *Store) datumDownloadGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "file":
		s.datumDownloadStream(c)
	case rest == "":
		fail(c, http.StatusBadRequest, "下载记录 ID 不能为空")
	default:
		if downloadID, err := strconv.ParseInt(rest, 10, 64); err == nil && downloadID > 0 {
			c.Params = append(c.Params, gin.Param{Key: "downloadId", Value: rest})
			s.datumDownloadDetail(c)
			return
		}
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

func (s *Store) datumDownloadDetail(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	downloadID, err := strconv.ParseInt(c.Param("downloadId"), 10, 64)
	if err != nil || downloadID <= 0 {
		fail(c, http.StatusBadRequest, "下载记录 ID 不正确")
		return
	}
	record, found, err := datum.NewDownloadRepo(s.conn).FindByID(downloadID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "下载记录查询失败")
		return
	}
	if !found || record.UserID != userID {
		fail(c, http.StatusNotFound, "下载记录不存在")
		return
	}
	success(c, gin.H{
		"downloadId": record.DownloadID,
		"fileId":     record.FileID,
		"userId":     record.UserID,
		"remark":     record.Remark,
		"createTime": record.CreatTime,
	})
}

type datumDownloadCreateRequest struct {
	FileID int64  `json:"fileId"`
	UserID int64  `json:"userId"`
	Remark string `json:"remark"`
}

func (s *Store) datumDownloadCreate(c *gin.Context) {
	var req datumDownloadCreateRequest
	_ = c.ShouldBind(&req)
	userID, ok := datumSessionUser(c, req.UserID)
	if !ok {
		return
	}
	if req.FileID <= 0 {
		fail(c, http.StatusBadRequest, "文件 ID 不能为空")
		return
	}
	file, found, err := datum.NewFileRepo(s.conn).FindByID(req.FileID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "文件查询失败")
		return
	}
	if !found || (file.FileStatus != 1 && file.UserID != userID) {
		fail(c, http.StatusNotFound, "文件不存在或已删除")
		return
	}
	if err := datum.NewDownloadRepo(s.conn).Create(req.FileID, userID); err != nil {
		fail(c, http.StatusInternalServerError, "下载记录写入失败")
		return
	}
	success(c, true)
}

type datumDownloadUpdateRequest struct {
	DownloadID int64  `json:"downloadId"`
	Remark     string `json:"remark"`
}

func (s *Store) datumDownloadUpdate(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumDownloadUpdateRequest
	_ = c.ShouldBind(&req)
	if req.DownloadID <= 0 {
		fail(c, http.StatusBadRequest, "下载记录 ID 不能为空")
		return
	}
	if err := datum.NewDownloadRepo(s.conn).UpdateRemark(req.DownloadID, userID, strings.TrimSpace(req.Remark)); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "下载记录不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "下载记录更新失败")
		return
	}
	success(c, true)
}

func (s *Store) datumDownloadDelete(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	ids := make([]int64, 0)
	for _, raw := range strings.Split(c.Param("ids"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		fail(c, http.StatusBadRequest, "下载记录 ID 不能为空")
		return
	}
	if _, err := datum.NewDownloadRepo(s.conn).DeleteByIDs(ids, userID); err != nil {
		fail(c, http.StatusInternalServerError, "下载记录删除失败")
		return
	}
	success(c, true)
}

func (s *Store) datumMyDownloads(c *gin.Context) {
	claimed := toDatumInt64(c.Param("userId"))
	userID, ok := datumSessionUser(c, claimed)
	if !ok {
		return
	}
	page, size := datumPageParams(c)
	result, err := datum.NewDownloadRepo(s.conn).ListByUser(userID, page, size)
	if err != nil {
		fail(c, http.StatusInternalServerError, "下载列表查询失败")
		return
	}
	respondDatumRows(c, result, datumDownloadRowMapper(result))
}

func (s *Store) datumMyDownloadRemove(c *gin.Context) {
	claimed := toDatumInt64(c.Param("userId"))
	userID, ok := datumSessionUser(c, claimed)
	if !ok {
		return
	}
	fileID := toDatumInt64(c.Param("fileId"))
	if fileID <= 0 {
		fail(c, http.StatusBadRequest, "文件 ID 不正确")
		return
	}
	if err := datum.NewDownloadRepo(s.conn).Delete(fileID, userID); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "下载记录不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "下载记录删除失败")
		return
	}
	success(c, true)
}

// ---------------------------------------------------------------------------
// file favorites
// ---------------------------------------------------------------------------

type datumFavoriteAddRequest struct {
	FileID int64 `json:"fileId"`
	UserID int64 `json:"userId"`
}

func (s *Store) datumFavoriteAdd(c *gin.Context) {
	var req datumFavoriteAddRequest
	_ = c.ShouldBind(&req)
	userID, ok := datumSessionUser(c, req.UserID)
	if !ok {
		return
	}
	if req.FileID <= 0 {
		fail(c, http.StatusBadRequest, "文件 ID 不能为空")
		return
	}
	file, found, err := datum.NewFileRepo(s.conn).FindByID(req.FileID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "文件查询失败")
		return
	}
	if !found || (file.FileStatus != 1 && file.UserID != userID) {
		fail(c, http.StatusNotFound, "文件不存在或已删除")
		return
	}
	repo := datum.NewFileFavoriteRepo(s.conn)
	if exists, err := repo.Exists(req.FileID, userID); err == nil && exists {
		fail(c, http.StatusConflict, "收藏已存在")
		return
	}
	if err := repo.Add(req.FileID, userID); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "收藏已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "收藏失败")
		return
	}
	success(c, true)
}

// datumFavoriteStatus reports whether the session user favorited a file.
func (s *Store) datumFavoriteStatus(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	fileID := toDatumInt64(c.Param("fileId"))
	if fileID <= 0 {
		fail(c, http.StatusBadRequest, "文件 ID 不正确")
		return
	}
	exists, err := datum.NewFileFavoriteRepo(s.conn).Exists(fileID, userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "收藏查询失败")
		return
	}
	success(c, gin.H{"favorited": exists})
}

func (s *Store) datumMyFavorites(c *gin.Context) {
	claimed := toDatumInt64(c.Param("userId"))
	userID, ok := datumSessionUser(c, claimed)
	if !ok {
		return
	}
	page, size := datumPageParams(c)
	result, err := datum.NewFileFavoriteRepo(s.conn).ListFilesByUser(userID, page, size)
	if err != nil {
		fail(c, http.StatusInternalServerError, "收藏列表查询失败")
		return
	}
	respondDatumRows(c, result, datumFileRowMapper(result))
}

func (s *Store) datumMyFavoriteRemove(c *gin.Context) {
	claimed := toDatumInt64(c.Param("userId"))
	userID, ok := datumSessionUser(c, claimed)
	if !ok {
		return
	}
	fileID := toDatumInt64(c.Param("fileId"))
	if fileID <= 0 {
		fail(c, http.StatusBadRequest, "文件 ID 不正确")
		return
	}
	if err := datum.NewFileFavoriteRepo(s.conn).Remove(fileID, userID); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "收藏不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "取消收藏失败")
		return
	}
	success(c, true)
}

// ---------------------------------------------------------------------------
// bookmark favorites
// ---------------------------------------------------------------------------

type datumBookmarkFavoriteAddRequest struct {
	BookmarkID int64 `json:"bookmarkId"`
	UserID     int64 `json:"userId"`
}

func (s *Store) datumBookmarkFavoriteAdd(c *gin.Context) {
	var req datumBookmarkFavoriteAddRequest
	_ = c.ShouldBind(&req)
	userID, ok := datumSessionUser(c, req.UserID)
	if !ok {
		return
	}
	if req.BookmarkID <= 0 {
		fail(c, http.StatusBadRequest, "书签 ID 不能为空")
		return
	}
	// 书签必须存在且未被删除（匿名可见口径即可收藏）。
	if _, found, err := datum.NewBookmarkRepo(s.conn).FindByID(req.BookmarkID); err != nil {
		fail(c, http.StatusInternalServerError, "书签查询失败")
		return
	} else if !found {
		fail(c, http.StatusNotFound, "书签不存在或已删除")
		return
	}
	repo := datum.NewBookmarkFavoriteRepo(s.conn)
	if exists, err := repo.Exists(req.BookmarkID, userID); err == nil && exists {
		fail(c, http.StatusConflict, "收藏已存在")
		return
	}
	if err := repo.Add(req.BookmarkID, userID); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			// 源库怪癖：ptmj_bookmark_favorite 主键仅 bookmark_id，
			// 一个书签只能有一行收藏记录。
			fail(c, http.StatusConflict, "收藏已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "收藏失败")
		return
	}
	success(c, true)
}

func (s *Store) datumBookmarkFavoriteRelations(c *gin.Context) {
	page, size := datumPageParams(c)
	result, err := datum.NewBookmarkFavoriteRepo(s.conn).ListAll(page, size)
	if err != nil {
		fail(c, http.StatusInternalServerError, "收藏关系查询失败")
		return
	}
	relations, _ := result.Items.([]datum.BookmarkFavoriteRelation)
	rows := make([]gin.H, 0, len(relations))
	for _, relation := range relations {
		rows = append(rows, gin.H{"bookmarkId": relation.BookmarkID, "userId": relation.UserID})
	}
	respondDatumRows(c, result, rows)
}

func (s *Store) datumMyBookmarkFavorites(c *gin.Context) {
	claimed := toDatumInt64(c.Param("userId"))
	userID, ok := datumSessionUser(c, claimed)
	if !ok {
		return
	}
	page, size := datumPageParams(c)
	result, err := datum.NewBookmarkFavoriteRepo(s.conn).ListByUser(userID, page, size)
	if err != nil {
		fail(c, http.StatusInternalServerError, "收藏列表查询失败")
		return
	}
	bookmarks, _ := result.Items.([]datum.Bookmark)
	rows := make([]gin.H, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		rows = append(rows, gin.H{
			"id": bookmark.ID, "bookmarkId": bookmark.ID, "userId": bookmark.UserID,
			"url": bookmark.URL, "title": bookmark.Title, "description": bookmark.Description,
			"coverImage": bookmark.CoverImage, "subject": bookmark.Subject,
			"resourceType": bookmark.ResourceType, "collection": bookmark.Collection,
			"status": bookmark.Status, "createTime": bookmark.CreateTime,
		})
	}
	respondDatumRows(c, result, rows)
}

func (s *Store) datumMyBookmarkFavoriteRemove(c *gin.Context) {
	claimed := toDatumInt64(c.Param("userId"))
	userID, ok := datumSessionUser(c, claimed)
	if !ok {
		return
	}
	bookmarkID := toDatumInt64(c.Param("bookmarkId"))
	if bookmarkID <= 0 {
		fail(c, http.StatusBadRequest, "书签 ID 不正确")
		return
	}
	if exists, err := datum.NewBookmarkFavoriteRepo(s.conn).Exists(bookmarkID, userID); err == nil && !exists {
		fail(c, http.StatusNotFound, "收藏不存在")
		return
	}
	if err := datum.NewBookmarkFavoriteRepo(s.conn).Remove(bookmarkID); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "收藏不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "取消收藏失败")
		return
	}
	success(c, true)
}

// respondDatumRows emits the RuoYi TableDataInfo shape the desktop reads
// (rows/total at the top level next to code/msg).
func respondDatumRows(c *gin.Context, page datum.Page, rows []gin.H) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"msg":     "ok",
		"rows":    rows,
		"total":   page.Total,
	})
}

func datumFileRowMapper(page datum.Page) []gin.H {
	files, _ := page.Items.([]datum.File)
	rows := make([]gin.H, 0, len(files))
	for _, file := range files {
		rows = append(rows, datumFilePayload(file))
	}
	return rows
}

func datumDownloadRowMapper(page datum.Page) []gin.H {
	items, _ := page.Items.([]datum.DownloadedFile)
	rows := make([]gin.H, 0, len(items))
	for _, item := range items {
		row := datumFilePayload(item.File)
		row["firstDownloadTime"] = item.FirstDownloadTime
		rows = append(rows, row)
	}
	return rows
}

// datumDownloadStream implements GET /datum/download/file?fileId= — the
// desktop blob download. The object is proxied from the server's storage
// (MinIO when configured, local fallback), and every successful stream
// appends a ptmj_file_download record for the session user.
func (s *Store) datumDownloadStream(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	fileID := toDatumInt64(c.Query("fileId"))
	if fileID <= 0 {
		fail(c, http.StatusBadRequest, "文件 ID 不能为空")
		return
	}
	file, found, err := datum.NewFileRepo(s.conn).FindByID(fileID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "文件查询失败")
		return
	}
	if !found || (file.FileStatus != 1 && file.UserID != userID) {
		fail(c, http.StatusNotFound, "文件不存在或已删除")
		return
	}
	bucket, objectKey, parseErr := fileurl.Parse(file.FileURL)
	if parseErr != nil {
		// 旧数据里的相对路径或外链无法从本服务器取流。
		fail(c, http.StatusNotFound, "文件内容不可用")
		return
	}
	body, info, err := openDatumObject(c.Request.Context(), bucket, objectKey)
	if err != nil {
		fail(c, http.StatusNotFound, "文件内容不可用")
		return
	}
	defer body.Close()

	if err := datum.NewDownloadRepo(s.conn).Create(fileID, userID); err != nil {
		// 记录写失败不阻断下载本身。
		log.Printf("datum download record write failed: %v", err)
	}

	contentType := mimeByExt(strings.ToLower(file.FileFormat))
	fileName := path.Base(strings.ReplaceAll(file.FileName, "\\", "/"))
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": fileName})
	if disposition == "" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", disposition)
	if info.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	c.DataFromReader(http.StatusOK, info.Size, contentType, body, map[string]string{})
}

// openDatumObject reads an object via the server-configured storage: MinIO
// first (the stored bucket is respected when it differs from config), local
// upload root as the fallback.
func openDatumObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, storage.ObjectInfo, error) {
	if datumEnvBool("KADMIN_MINIO_ENABLED") {
		minio := storage.NewMinio(storage.MinioConfig{
			Endpoints: []string{datumEnv("KADMIN_MINIO_ENDPOINT", "127.0.0.1:19000"), datumEnv("KADMIN_MINIO_INTERNAL_ENDPOINT", "minio:9000")},
			AccessKey: datumEnv("KADMIN_MINIO_ACCESS_KEY", "kadmin_minio"),
			SecretKey: datumEnv("KADMIN_MINIO_SECRET_KEY", "kadmin_minio_pwd"),
			Bucket:    bucket,
			UseSSL:    datumEnvBool("KADMIN_MINIO_USE_SSL"),
			Region:    datumEnv("KADMIN_MINIO_REGION", "us-east-1"),
			Timeout:   10 * time.Second,
		})
		if body, info, err := minio.Open(ctx, objectKey); err == nil {
			return body, info, nil
		}
	}
	local := storage.NewLocal(datumEnv("KADMIN_FILE_LOCAL_ROOT", "data/files"))
	return local.Open(ctx, objectKey)
}
