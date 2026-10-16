package kadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/convert"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/fileurl"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/storage"
	"github.com/gin-gonic/gin"
)

// Desktop file module (A4): anonymous browsing (list/tree/subjects/schools/
// search/detail), owner upload/edit/delete with LibreOffice conversion, and
// hash-validated tree/rank caching.
//
// Tree & rank cache protocol: the server keeps {hash, payload} per resource
// in Redis; the hash is a fresh random 64-bit value whenever the payload is
// (re)computed after an invalidation. Clients send ?hash=<stored>; when it
// still matches, the response carries unchanged=true and no payload, so the
// client keeps reading its local copy.

const (
	datumStateTTLDefault    = 24 * time.Hour
	datumTreeStateVersion   = 1
	datumUploadMaxBytesDef  = 200 << 20 // 200MB
	datumSearchLimit        = 50
	datumFileStateTTLKey    = "KADMIN_DATUM_STATE_TTL"
	datumUploadMaxMBKey     = "KADMIN_DATUM_UPLOAD_MAX_MB"
	datumMinioPublicBaseKey = "KADMIN_MINIO_PUBLIC_BASE"
)

var datumFileTypeNames = map[int64]string{
	1: "期末", 2: "期中", 3: "补考", 4: "资料", 5: "其他学校", 6: "神秘文件",
	7: "电子书",
}

var datumConvertExts = map[string]bool{"doc": true, "docx": true, "ppt": true, "pptx": true}

// ---------------------------------------------------------------------------
// hash-validated state cache (tree / rank)
// ---------------------------------------------------------------------------

type datumStateEntry struct {
	Version int             `json:"version,omitempty"`
	Hash    string          `json:"hash"`
	Payload json.RawMessage `json:"payload"`
}

type datumStateStore struct {
	redis   redisDoer
	ttl     time.Duration
	mu      *sync.Mutex
	version int
}

func newDatumStateStore(redis redisDoer) *datumStateStore {
	ttl := datumStateTTLDefault
	if parsed, err := time.ParseDuration(strings.TrimSpace(os.Getenv(datumFileStateTTLKey))); err == nil && parsed > time.Minute {
		ttl = parsed
	}
	return &datumStateStore{redis: redis, ttl: ttl, mu: &sync.Mutex{}}
}

// load returns the cached entry, computing it when missing or expired. A new
// random 64-bit hash is minted on every recompute, so content changes are
// always visible to clients comparing hashes.
func (s *datumStateStore) load(key string, compute func() (interface{}, error)) (datumStateEntry, bool, error) {
	// Hot reads can keep using the current snapshot during an early refresh.
	if entry, found, err := s.cached(key); err != nil || found {
		return entry, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A request or the warmer may have filled the cache while we waited.
	if entry, found, err := s.cached(key); err != nil || found {
		return entry, false, err
	}
	entry, err := s.rebuild(key, compute, nil)
	return entry, err == nil, err
}

func (s *datumStateStore) cached(key string) (datumStateEntry, bool, error) {
	if raw, err := s.redis.do("GET", key); err == nil {
		if text, ok := raw.(string); ok {
			var entry datumStateEntry
			if json.Unmarshal([]byte(text), &entry) == nil && entry.Version == s.version && entry.Hash != "" && len(entry.Payload) > 0 {
				return entry, true, nil
			}
		}
	} else if !errors.Is(err, errRedisNil) {
		return datumStateEntry{}, false, err
	}
	return datumStateEntry{}, false, nil
}

// warm fills missing entries and refreshes ones approaching expiration without
// deleting the old snapshot. A failed refresh leaves that snapshot available.
func (s *datumStateStore) warm(key string, compute func() (interface{}, error), refreshBefore time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found, err := s.cached(key)
	if err != nil {
		return false, err
	}
	if found {
		if refreshBefore <= 0 {
			return false, nil
		}
		rawTTL, err := s.redis.do("TTL", key)
		if err != nil {
			return false, err
		}
		ttl, ok := rawTTL.(int64)
		if !ok {
			return false, fmt.Errorf("缓存 %s 的 TTL 响应无效", key)
		}
		if ttl > int64(refreshBefore/time.Second) {
			return false, nil
		}
	}
	_, err = s.rebuild(key, compute, &entry)
	return err == nil, err
}

func (s *datumStateStore) rebuild(key string, compute func() (interface{}, error), previous *datumStateEntry) (datumStateEntry, error) {
	payload, err := compute()
	if err != nil {
		return datumStateEntry{}, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return datumStateEntry{}, err
	}
	hash, err := randomHex(8) // 64-bit
	if err != nil {
		return datumStateEntry{}, err
	}
	// Proactive refreshes of identical content keep clients' local copies valid.
	if previous != nil && previous.Hash != "" && bytes.Equal(previous.Payload, encoded) {
		hash = previous.Hash
	}
	entry := datumStateEntry{Version: s.version, Hash: hash, Payload: encoded}
	blob, err := json.Marshal(entry)
	if err != nil {
		return datumStateEntry{}, err
	}
	if _, err := s.redis.do("SET", key, string(blob), "EX", durationSeconds(s.ttl)); err != nil {
		return datumStateEntry{}, err
	}
	return entry, nil
}

func (s *datumStateStore) invalidate(keys ...string) {
	// A mutation's DEL must run after any in-flight rebuild has published its
	// snapshot, so that an older snapshot cannot undo the invalidation.
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		_, _ = s.redis.do("DEL", key)
	}
}

// respondState emits the hash-aware envelope: `data` stays the payload the
// client expects (array), while `hash`/`unchanged` ride as extra fields.
func respondState(c *gin.Context, entry datumStateEntry, clientHash string, payloadOnUnchanged bool) {
	unchanged := clientHash != "" && clientHash == entry.Hash
	var payload interface{}
	if unchanged && !payloadOnUnchanged {
		payload = nil
	} else {
		payload = json.RawMessage(entry.Payload)
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "ok", "msg": "ok",
		"data":      payload,
		"hash":      entry.Hash,
		"unchanged": unchanged,
	})
}

// ---------------------------------------------------------------------------
// routes
// ---------------------------------------------------------------------------

func (s *Store) registerDatumFileRoutes(datumGroup *gin.RouterGroup) {
	files := datumGroup.Group("/file")
	// gin(v1.3) 的 GET 路由树不允许静态子节点与 :param 并存，全部 GET 走一个
	// 通配分发器；POST/PUT/DELETE 树无静态冲突，保持直接注册。
	files.GET("/*rest", s.datumFileGet)
	files.POST("", s.requireDatumAuth(), s.datumFileUpload)
	files.PUT("", s.requireDatumAuth(), s.datumFileUpdate)
	files.DELETE("/:fileId", s.requireDatumAuth(), s.datumFileDelete)

	// /datum/user 的 GET/DELETE 通配分发器由 registerDatumUserRoutes 挂载：
	// rank 读取与 rank/cache 清理经 datumUserGet / datumUserDeleteDispatch 分发到这里。
	// 缓存强制刷新是管理操作：管理端 JWT + datum 文件管理权限（原为 datum 会话）
	datumGroup.DELETE("/file-tree/cache", s.requireAuth(), s.requirePermission(datumFileManagePermission), s.datumTreeCachePurge)
}

// datumFileGet dispatches the read-only file routes:
// /list /tree /subjects /schools /schools/check /search /{fileId}
func (s *Store) datumFileGet(c *gin.Context) {
	rest := strings.Trim(c.Param("rest"), "/")
	switch {
	case rest == "list":
		s.datumFileList(c)
	case rest == "tree":
		s.datumFileTree(c)
	case rest == "subjects":
		s.datumFileSubjects(c)
	case rest == "schools":
		s.datumFileSchools(c)
	case rest == "schools/check":
		s.datumFileSchoolCheck(c)
	case rest == "search":
		s.datumFileSearch(c)
	default:
		if fileID, err := strconv.ParseInt(rest, 10, 64); err == nil && fileID > 0 {
			c.Params = append(c.Params, gin.Param{Key: "fileId", Value: rest})
			s.datumFileDetail(c)
			return
		}
		fail(c, http.StatusNotFound, "接口不存在")
	}
}

func (s *Store) stateStore() *datumStateStore {
	state := newDatumStateStore(s.datum.redis)
	state.mu = &s.datumStateMu
	return state
}

// Keep the existing Redis key while replacing cached payloads created by older
// tree layouts. Ranking entries retain their existing unversioned protocol.
func (s *Store) treeStateStore() *datumStateStore {
	state := s.stateStore()
	state.version = datumTreeStateVersion
	return state
}
func (s *Store) treeStateKey() string  { return s.datum.keyPrefix + ":tree-state" }
func (s *Store) rankStateKey() string  { return s.datum.keyPrefix + ":rank-state" }
func (s *Store) invalidateDatumTree()  { s.stateStore().invalidate(s.treeStateKey()) }
func (s *Store) invalidateDatumRank()  { s.stateStore().invalidate(s.rankStateKey()) }
func (s *Store) invalidateDatumFiles() { s.invalidateDatumTree(); s.invalidateDatumRank() }

// ---------------------------------------------------------------------------
// anonymous reads
// ---------------------------------------------------------------------------

func datumPageParams(c *gin.Context) (int, int) {
	page, size := 1, 20
	for _, key := range []string{"pageNum", "page"} {
		if raw := strings.TrimSpace(c.Query(key)); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				page = parsed
			}
			break
		}
	}
	for _, key := range []string{"pageSize", "size"} {
		if raw := strings.TrimSpace(c.Query(key)); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				size = parsed
			}
			break
		}
	}
	// 单页上限 100：约束最坏情况下的响应体（配合 gzip 约 5–10KB），防止
	// 超大 pageSize 把上行打满。需要“全量”的桌面端视图按 total 逐页拉取
	// （见桌面端 utils/pagination.js 的 fetchAllPages）。
	if size > 100 {
		size = 100
	}
	return page, size
}

// Owner lists include pending/rejected rows, so the requested owner must be
// the current datum session user. Public lists omit userId and stay approved-only.
func (s *Store) datumOwnerListUser(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("userId"))
	if raw == "" {
		return 0, true
	}
	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || userID <= 0 {
		fail(c, http.StatusBadRequest, "用户 ID 不正确")
		return 0, false
	}
	s.requireDatumAuth()(c)
	if c.IsAborted() {
		return 0, false
	}
	return datumSessionUser(c, userID)
}

func (s *Store) datumFileList(c *gin.Context) {
	userID, ok := s.datumOwnerListUser(c)
	if !ok {
		return
	}
	page, size := datumPageParams(c)
	filter := datum.FileFilter{
		Page:         page,
		PageSize:     size,
		FileType:     datumQueryInt(c, "fileType"),
		FileSubject:  c.Query("fileSubject"),
		FileSchool:   c.Query("fileSchool"),
		FileYear:     datumQueryInt(c, "fileYear"),
		Keyword:      c.Query("keyword"),
		UserID:       userID,
		OnlyApproved: userID == 0,
	}
	result, err := datum.NewFileRepo(s.conn).List(filter)
	if err != nil {
		fail(c, http.StatusInternalServerError, "文件列表查询失败")
		return
	}
	items, _ := result.Items.([]datum.File)
	rows := make([]gin.H, 0, len(items))
	for _, file := range items {
		rows = append(rows, datumFilePayload(file))
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

func (s *Store) datumFileTree(c *gin.Context) {
	entry, _, err := s.treeStateStore().load(s.treeStateKey(), s.computeDatumTree)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "文件树暂不可用，请稍后重试")
		return
	}
	respondState(c, entry, strings.TrimSpace(c.Query("hash")), false)
}

func (s *Store) computeDatumTree() (interface{}, error) {
	files, err := datum.NewFileRepo(s.conn).ListApprovedFlat()
	if err != nil {
		return nil, err
	}
	type folderNode struct {
		ID       string        `json:"id"`
		Label    string        `json:"label"`
		Type     string        `json:"type"`
		Children []interface{} `json:"children"`
	}

	// Restore the explorer's subject -> school -> type -> year hierarchy,
	// followed by any custom directories stored in remark by folder uploads.
	tree := make([]*folderNode, 0)
	folders := make(map[string]*folderNode)
	for _, file := range files {
		subject, school := strings.TrimSpace(file.FileSubject), strings.TrimSpace(file.FileSchool)
		if subject == "" {
			subject = "未知科目"
		}
		if school == "" {
			school = "未知学校"
		}
		year := file.FileYear
		if year <= 0 {
			year = int64(time.Now().Year())
		}
		parts := []string{subject, school, datumFileTypeName(file.FileType), strconv.FormatInt(year, 10)}
		for _, part := range strings.Split(file.Remark, "/") {
			if name := strings.TrimSpace(part); name != "" {
				parts = append(parts, name)
			}
		}
		var parent *folderNode
		for index, label := range parts {
			// Encoding path components separately avoids collisions between a
			// label containing '/' and two adjacent directory components.
			encodedPath, _ := json.Marshal(parts[:index+1])
			key := string(encodedPath)
			folder := folders[key]
			if folder == nil {
				folder = &folderNode{
					ID: "folder-" + tokenHash(key), Label: label,
					Type: "folder", Children: make([]interface{}, 0),
				}
				folders[key] = folder
				if parent == nil {
					tree = append(tree, folder)
				} else {
					parent.Children = append(parent.Children, folder)
				}
			}
			parent = folder
		}
		leaf := datumFilePayload(file)
		leaf["id"] = "file-" + strconv.FormatInt(file.FileID, 10)
		leaf["children"] = []interface{}{}
		parent.Children = append(parent.Children, leaf)
	}
	return tree, nil
}

func datumFileTypeName(fileType int64) string {
	if name, ok := datumFileTypeNames[fileType]; ok {
		return name
	}
	return "其他"
}

func datumFilePayload(file datum.File) gin.H {
	return gin.H{
		"id":          file.FileID,
		"fileId":      file.FileID,
		"label":       file.FileName,
		"fileName":    file.FileName,
		"fileUrl":     datumReadableFileURL(file.FileURL),
		"fileSize":    file.FileSize,
		"fileFormat":  file.FileFormat,
		"fileYear":    file.FileYear,
		"fileType":    file.FileType,
		"fileSchool":  file.FileSchool,
		"fileSubject": file.FileSubject,
		"fileStatus":  file.FileStatus,
		"reviewer":    file.Reviewer,
		"delFlag":     file.DelFlag,
		"userId":      file.UserID,
		"createBy":    file.CreateBy,
		"createTime":  file.CreateTime,
		"updateBy":    file.UpdateBy,
		"updateTime":  file.UpdateTime,
		"remark":      file.Remark,
		"type":        "file",
	}
}

// datumReadableFileURL keeps stored URLs untouched except for the minio://
// scheme, which desktop clients cannot resolve — those are rebuilt against
// the current public base. Internal http hosts stay as-is; the desktop
// normalizeFileUrl fallback handles them until the deferred ETL rewrites
// legacy rows.
func datumReadableFileURL(stored string) string {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(stored)), "minio://") {
		return stored
	}
	bucket, objectKey, err := fileurl.Parse(stored)
	if err != nil {
		return stored
	}
	builder, err := fileurl.NewBuilder(bucket, datumEnv(datumMinioPublicBaseKey, datumMinioFallbackBase()))
	if err != nil {
		return stored
	}
	url, err := builder.ObjectURL(objectKey)
	if err != nil {
		return stored
	}
	return url
}

func datumMinioFallbackBase() string {
	scheme := "http"
	if datumEnvBool("KADMIN_MINIO_USE_SSL") {
		scheme = "https"
	}
	endpoint := datumEnv("KADMIN_MINIO_ENDPOINT", "127.0.0.1:19000")
	if index := strings.Index(endpoint, "://"); index >= 0 {
		return strings.TrimRight(endpoint, "/")
	}
	return scheme + "://" + strings.TrimRight(endpoint, "/")
}

func (s *Store) datumFileSubjects(c *gin.Context) {
	subjects, err := datum.NewFileRepo(s.conn).Subjects()
	if err != nil {
		fail(c, http.StatusInternalServerError, "科目查询失败")
		return
	}
	success(c, subjects)
}

func (s *Store) datumFileSchools(c *gin.Context) {
	schools, err := datum.NewFileRepo(s.conn).Schools(c.Query("keyword"))
	if err != nil {
		fail(c, http.StatusInternalServerError, "学校查询失败")
		return
	}
	success(c, schools)
}

func (s *Store) datumFileSchoolCheck(c *gin.Context) {
	school := strings.TrimSpace(c.Query("schoolName"))
	if school == "" {
		fail(c, http.StatusBadRequest, "学校名称不能为空")
		return
	}
	schools, err := datum.NewFileRepo(s.conn).Schools(school)
	if err != nil {
		fail(c, http.StatusInternalServerError, "学校查询失败")
		return
	}
	success(c, gin.H{"exists": len(schools) > 0})
}

func (s *Store) datumFileSearch(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		success(c, []gin.H{})
		return
	}
	page, err := datum.NewFileRepo(s.conn).List(datum.FileFilter{Page: 1, PageSize: datumSearchLimit, Keyword: keyword, OnlyApproved: true})
	if err != nil {
		fail(c, http.StatusInternalServerError, "搜索失败")
		return
	}
	items, _ := page.Items.([]datum.File)
	sort.SliceStable(items, func(i, j int) bool {
		return strings.Contains(items[i].FileSubject, keyword) && !strings.Contains(items[j].FileSubject, keyword)
	})
	rows := make([]gin.H, 0, len(items))
	for _, file := range items {
		rows = append(rows, datumFilePayload(file))
	}
	success(c, rows)
}

func (s *Store) datumFileDetail(c *gin.Context) {
	fileID, ok := datumPathInt64(c, "fileId")
	if !ok {
		return
	}
	file, found, err := datum.NewFileRepo(s.conn).FindByID(fileID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "文件查询失败")
		return
	}
	if !found {
		fail(c, http.StatusNotFound, "文件不存在或已删除")
		return
	}
	// Anonymous visitors only see approved files; the owner (when a valid
	// datum session is presented) also sees own pending/rejected uploads.
	if file.FileStatus != 1 {
		if !s.datumTokenOwns(c, file.UserID) {
			fail(c, http.StatusNotFound, "文件不存在或已删除")
			return
		}
	}
	success(c, datumFilePayload(file))
}

func (s *Store) datumTokenOwns(c *gin.Context, ownerID int64) bool {
	userID, ok := s.datumUserIDOptional(c)
	return ok && userID == ownerID
}

func (s *Store) datumUserRank(c *gin.Context) {
	entry, _, err := s.stateStore().load(s.rankStateKey(), s.computeDatumRank)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "排行榜暂不可用，请稍后重试")
		return
	}
	respondState(c, entry, strings.TrimSpace(c.Query("hash")), false)
}

func (s *Store) computeDatumRank() (interface{}, error) {
	ranks, err := datum.NewUserStats(s.conn).TopUploaders(50)
	if err != nil {
		return nil, err
	}
	rows := make([]gin.H, 0, len(ranks))
	for _, rank := range ranks {
		rows = append(rows, gin.H{
			"userId": rank.UserID, "userName": rank.UserName,
			"avatar": rank.Avatar, "count": rank.Uploads, "uploads": rank.Uploads,
		})
	}
	return rows, nil
}

func (s *Store) datumRankCachePurge(c *gin.Context) {
	s.invalidateDatumRank()
	success(c, true)
}

// ---------------------------------------------------------------------------
// owner writes: upload / update / delete
// ---------------------------------------------------------------------------

var datumAllowedExts = map[string]bool{
	"doc": true, "docx": true, "pdf": true, "jpg": true, "jpeg": true,
	"png": true, "webp": true, "txt": true, "md": true, "ppt": true, "pptx": true,
	"epub": true, "mobi": true, "azw3": true,
}

// datumFilePut stores bytes and returns the public URL; swappable in tests.
var datumFilePut = func(ctx context.Context, objectKey string, body []byte, contentType string) (string, error) {
	minio := storage.NewMinio(storage.MinioConfig{
		Endpoints: []string{datumEnv("KADMIN_MINIO_ENDPOINT", "127.0.0.1:19000"), datumEnv("KADMIN_MINIO_INTERNAL_ENDPOINT", "minio:9000")},
		AccessKey: datumEnv("KADMIN_MINIO_ACCESS_KEY", "kadmin_minio"),
		SecretKey: datumEnv("KADMIN_MINIO_SECRET_KEY", "kadmin_minio_pwd"),
		Bucket:    datumEnv("KADMIN_MINIO_BUCKET", "kadmin"),
		UseSSL:    datumEnvBool("KADMIN_MINIO_USE_SSL"),
		Region:    datumEnv("KADMIN_MINIO_REGION", "us-east-1"),
		Timeout:   10 * time.Second,
	})
	if err := minio.Put(ctx, objectKey, bytes.NewReader(body), int64(len(body)), contentType); err != nil {
		return "", err
	}
	bucket := datumEnv("KADMIN_MINIO_BUCKET", "kadmin")
	builder, err := fileurl.NewBuilder(bucket, datumEnv(datumMinioPublicBaseKey, datumMinioFallbackBase()))
	if err != nil {
		return "", err
	}
	return builder.ObjectURL(objectKey)
}

func (s *Store) datumFileUpload(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "文件不能为空")
		return
	}
	maxBytes := int(datumUploadMaxBytesDef >> 20)
	if parsed, err := strconv.Atoi(strings.TrimSpace(os.Getenv(datumUploadMaxMBKey))); err == nil && parsed > 0 && parsed <= 4096 {
		maxBytes = parsed
	}
	if file.Size <= 0 || file.Size > int64(maxBytes)<<20 {
		fail(c, http.StatusBadRequest, fmt.Sprintf("文件过大：不能超过 %dMB", maxBytes))
		return
	}
	opened, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "文件读取失败")
		return
	}
	defer opened.Close()
	body, err := io.ReadAll(io.LimitReader(opened, file.Size+1))
	if err != nil || int64(len(body)) != file.Size {
		fail(c, http.StatusBadRequest, "文件读取失败")
		return
	}

	fileName := strings.TrimSpace(c.PostForm("fileName"))
	if fileName == "" {
		fileName = file.Filename
	}
	if fileName == "" || len([]rune(fileName)) > 255 {
		fail(c, http.StatusBadRequest, "文件名不能为空且不超过 255 个字符")
		return
	}
	subject := strings.TrimSpace(c.PostForm("fileSubject"))
	school := strings.TrimSpace(c.PostForm("fileSchool"))
	year := toDatumInt64(strings.TrimSpace(c.PostForm("fileYear")))
	fileType := toDatumInt64(strings.TrimSpace(c.PostForm("fileType")))
	folderPath, err := storage.CleanObjectKey(strings.TrimSpace(c.PostForm("remark")))
	if err != nil {
		folderPath = ""
	}
	if subject == "" || len(subject) > 64 {
		fail(c, http.StatusBadRequest, "科目不能为空且不超过 64 个字符")
		return
	}
	if len(school) > 100 {
		fail(c, http.StatusBadRequest, "学校名称不能超过 100 个字符")
		return
	}
	if year < 1900 || year > 2100 {
		fail(c, http.StatusBadRequest, "年份需在 1900 到 2100 之间")
		return
	}
	if _, known := datumFileTypeNames[fileType]; !known {
		fail(c, http.StatusBadRequest, "文件类型不正确")
		return
	}

	ext := strings.ToLower(strings.TrimPrefix(path.Ext(fileName), "."))
	if ext == "" {
		ext = strings.ToLower(strings.TrimPrefix(path.Ext(file.Filename), "."))
	}
	if !datumAllowedExts[ext] {
		fail(c, http.StatusBadRequest, "不支持的文件格式：仅支持 doc/docx/pdf/jpg/png/webp/txt/md/ppt/pptx")
		return
	}
	contentType := file.Header.Get("Content-Type")
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mimeByExt(ext)
	}
	if ext == "txt" || ext == "md" {
		if !utf8.Valid(body) {
			fail(c, http.StatusBadRequest, "文本文件仅支持 UTF-8 编码")
			return
		}
	}
	if datumConvertExts[ext] {
		converter := convert.New(convert.ConfigFromEnv(datumEnv))
		pdf, err := converter.ToPDF(c.Request.Context(), fileName, bytes.NewReader(body))
		if err != nil {
			if errors.Is(err, convert.ErrUnavailable) {
				fail(c, http.StatusServiceUnavailable, "转档服务不可用，暂时无法上传该格式，请稍后重试")
			} else {
				fail(c, http.StatusBadRequest, "文件转档失败，请检查文件是否损坏")
			}
			return
		}
		body = pdf
		ext = "pdf"
		contentType = "application/pdf"
	}

	unique, err := randomHex(6)
	if err != nil {
		fail(c, http.StatusInternalServerError, "上传失败，请稍后重试")
		return
	}
	baseName := strings.TrimSuffix(path.Base(fileName), path.Ext(fileName))
	objectSegments := []string{subject}
	if school != "" {
		objectSegments = append(objectSegments, school)
	}
	objectSegments = append(objectSegments, datumFileTypeName(fileType), strconv.FormatInt(year, 10))
	if folderPath != "" {
		objectSegments = append(objectSegments, folderPath)
	}
	objectSegments = append(objectSegments, baseName+"_"+unique+"."+ext)
	objectKey := path.Join(objectSegments...)

	fileURL, err := datumFilePut(c.Request.Context(), objectKey, body, contentType)
	if err != nil {
		log.Printf("datum file storage put failed: %v", err)
		fail(c, http.StatusServiceUnavailable, "文件存储暂不可用，请稍后重试")
		return
	}

	repo := datum.NewFileRepo(s.conn)
	fileID, err := repo.Insert(datum.FilePayload{
		UserID: userID, FileName: fileName, FileURL: fileURL, FileSize: int64(len(body)),
		FileFormat: ext, FileYear: year, FileType: fileType, FileSchool: school,
		FileSubject: subject, FileStatus: 0, Remark: folderPath,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "上传失败")
		return
	}
	_ = datum.NewUserStats(s.conn).IncrementUploads(userID)
	s.invalidateDatumFiles()
	success(c, gin.H{"fileId": fileID, "fileUrl": fileURL, "fileName": fileName})
}

type datumFileUpdateRequest struct {
	FileID      int64  `json:"fileId"`
	UserID      int64  `json:"userId"`
	FileName    string `json:"fileName"`
	FileYear    int64  `json:"fileYear"`
	FileType    int64  `json:"fileType"`
	FileSchool  string `json:"fileSchool"`
	FileSubject string `json:"fileSubject"`
	Remark      string `json:"remark"`
}

func (s *Store) datumFileUpdate(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumFileUpdateRequest
	_ = c.ShouldBind(&req)
	if req.FileID <= 0 {
		fail(c, http.StatusBadRequest, "文件 ID 不能为空")
		return
	}
	if strings.TrimSpace(req.FileName) == "" || len([]rune(strings.TrimSpace(req.FileName))) > 255 {
		fail(c, http.StatusBadRequest, "文件名不能为空且不超过 255 个字符")
		return
	}
	if strings.TrimSpace(req.FileSubject) == "" {
		fail(c, http.StatusBadRequest, "科目不能为空")
		return
	}
	if _, known := datumFileTypeNames[req.FileType]; !known {
		fail(c, http.StatusBadRequest, "文件类型不正确")
		return
	}
	payload := datum.FilePayload{
		UserID: userID, FileName: strings.TrimSpace(req.FileName), FileYear: req.FileYear,
		FileType: req.FileType, FileSchool: strings.TrimSpace(req.FileSchool),
		FileSubject: strings.TrimSpace(req.FileSubject), Remark: strings.TrimSpace(req.Remark),
	}
	if err := datum.NewFileRepo(s.conn).UpdateOwner(req.FileID, userID, payload); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "文件不存在或无权修改")
			return
		}
		fail(c, http.StatusInternalServerError, "文件更新失败")
		return
	}
	s.invalidateDatumFiles()
	success(c, true)
}

func (s *Store) datumFileDelete(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	fileID, ok := datumPathInt64(c, "fileId")
	if !ok {
		return
	}
	if err := datum.NewFileRepo(s.conn).MarkDeleted(fileID, userID); err != nil {
		if errors.Is(err, datum.ErrNotFound) {
			fail(c, http.StatusNotFound, "文件不存在或无权删除")
			return
		}
		fail(c, http.StatusInternalServerError, "文件删除失败")
		return
	}
	_ = datum.NewUserStats(s.conn).DecrementUploads(userID)
	s.invalidateDatumFiles()
	success(c, true)
}

func mimeByExt(ext string) string {
	switch ext {
	case "pdf":
		return "application/pdf"
	case "epub":
		return "application/epub+zip"
	case "mobi", "azw3":
		return "application/x-mobipocket-ebook"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "txt", "md":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func datumQueryInt(c *gin.Context, key string) int64 {
	return toDatumInt64(strings.TrimSpace(c.Query(key)))
}

func datumPathInt64(c *gin.Context, param string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Param(param)), 10, 64)
	if err != nil || value <= 0 {
		fail(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return value, true
}

// datumTreeCachePurge lets an admin force a tree refresh after writing
// ptmj_file directly (e.g. bulk approvals via SQL or the generated CRUD
// module, which bypass the datum write hooks).
func (s *Store) datumTreeCachePurge(c *gin.Context) {
	s.invalidateDatumTree()
	success(c, true)
}
