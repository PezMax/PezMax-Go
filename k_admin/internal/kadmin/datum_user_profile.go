package kadmin

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/datum"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/fileurl"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/storage"
	"github.com/gin-gonic/gin"
)

// Desktop personal-center (A3): profile read, stats, username/avatar edits,
// password verify/change/security-reset, and security Q&A management. All
// routes sit behind requireDatumAuth; the security GET never returns stored
// answers and the security PUT always re-hashes the three new answers.

const (
	datumAvatarMaxSize  = 2 << 20 // 2MB, mirrors the admin avatar policy
	datumAvatarLocalDir = "upload"
)

func (s *Store) registerDatumDesktopRoutes(datum *gin.RouterGroup) {
	desktop := datum.Group("/desktop/user", s.requireDatumAuth())
	desktop.GET("/profile", s.datumProfile)
	desktop.GET("/profile/stats", s.datumProfileStats)
	desktop.PUT("/profile/username", s.datumUpdateUserName)
	desktop.PUT("/profile/avatar", s.datumUpdateAvatar)
	desktop.POST("/profile/avatar/upload", s.datumUploadAvatar)
	desktop.POST("/profile/password/verify", s.datumVerifyPassword)
	desktop.PUT("/profile/password", s.datumUpdatePassword)
	desktop.PUT("/profile/password/by-security", s.datumResetPasswordBySecurity)
	desktop.GET("/profile/security", s.datumGetSecurity)
	desktop.PUT("/profile/security", s.datumUpdateSecurity)
	desktop.POST("/profile/security/answer/verify", s.datumVerifySecurityAnswer)
}

func (s *Store) datumProfile(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	user, err := (&datumUserRepo{conn: s.conn}).findByUserID(userID)
	if err != nil {
		if errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusForbidden, "账号不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "账号查询失败")
		return
	}
	success(c, gin.H{
		"userId":   user.UserID,
		"userName": user.UserName,
		"nickName": user.UserName,
		"avatar":   user.Avatar,
		"count":    user.Count,
		"status":   user.Status,
	})
}

func (s *Store) datumProfileStats(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	uploads, downloads, fileFavorites, bookmarkFavorites, err := datum.NewUserStats(s.conn).ProfileStats(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "统计查询失败")
		return
	}
	success(c, gin.H{
		"uploadCount":           uploads,
		"downloadCount":         downloads,
		"favoriteCount":         fileFavorites + bookmarkFavorites,
		"fileFavoriteCount":     fileFavorites,
		"bookmarkFavoriteCount": bookmarkFavorites,
	})
}

type datumUserNameRequest struct {
	UserName string `json:"userName"`
}

func (s *Store) datumUpdateUserName(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumUserNameRequest
	_ = c.ShouldBind(&req)
	userName := strings.TrimSpace(req.UserName)
	if userName == "" || len([]rune(userName)) > 64 {
		fail(c, http.StatusBadRequest, "用户名格式错误：需为 1-64 个字符")
		return
	}
	repo := &datumUserRepo{conn: s.conn}
	if err := repo.updateUserName(userID, userName); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "用户名已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "用户名修改失败")
		return
	}
	success(c, gin.H{"userName": userName})
}

type datumAvatarRequest struct {
	Avatar string `json:"avatar"`
}

func (s *Store) datumUpdateAvatar(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumAvatarRequest
	_ = c.ShouldBind(&req)
	avatar := strings.TrimSpace(req.Avatar)
	if avatar == "" || len(avatar) > 500 {
		fail(c, http.StatusBadRequest, "头像地址格式错误")
		return
	}
	if err := (&datumUserRepo{conn: s.conn}).updateAvatar(userID, avatar); err != nil {
		fail(c, http.StatusInternalServerError, "头像更新失败")
		return
	}
	success(c, gin.H{"avatar": avatar})
}

// datumUploadAvatar accepts a multipart "file" (jpg/png/gif, ≤2MB), stores it
// via MinIO when configured (falling back to local like the admin files
// service), points the account at the resulting URL and returns it.
func (s *Store) datumUploadAvatar(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "头像文件不能为空")
		return
	}
	if file.Size <= 0 || file.Size > datumAvatarMaxSize {
		fail(c, http.StatusBadRequest, "头像文件过大：不能超过 2MB")
		return
	}
	opened, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "头像文件读取失败")
		return
	}
	defer opened.Close()
	body, err := io.ReadAll(io.LimitReader(opened, datumAvatarMaxSize+1))
	if err != nil || len(body) == 0 || int64(len(body)) > datumAvatarMaxSize {
		fail(c, http.StatusBadRequest, "头像文件读取失败")
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
	default:
		fail(c, http.StatusBadRequest, "头像仅支持 JPG / PNG / GIF 格式")
		return
	}

	randomName, err := randomHex(12)
	if err != nil {
		fail(c, http.StatusInternalServerError, "头像上传失败，请稍后重试")
		return
	}
	objectKey := path.Join("avatars", "datum", time.Now().Format("20060102"), randomName+ext)
	avatarURL, storageName, err := putDatumAvatar(c.Request.Context(), objectKey, body, contentType)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "头像存储暂不可用，请稍后重试")
		return
	}
	if err := (&datumUserRepo{conn: s.conn}).updateAvatar(userID, avatarURL); err != nil {
		fail(c, http.StatusInternalServerError, "头像更新失败")
		return
	}
	success(c, gin.H{"url": avatarURL, "storage": storageName, "name": file.Filename})
}

type datumPasswordVerifyRequest struct {
	Password string `json:"password"`
}

func (s *Store) datumVerifyPassword(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumPasswordVerifyRequest
	_ = c.ShouldBind(&req)
	user, err := (&datumUserRepo{conn: s.conn}).findByUserID(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "账号查询失败")
		return
	}
	if !verifyDatumSecret(user.Password, req.Password) {
		fail(c, http.StatusBadRequest, "密码错误")
		return
	}
	success(c, true)
}

type datumUpdatePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (s *Store) datumUpdatePassword(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumUpdatePasswordRequest
	_ = c.ShouldBind(&req)
	if len(req.NewPassword) < 5 || len(req.NewPassword) > 64 {
		fail(c, http.StatusBadRequest, "密码格式错误：长度需在 5 到 64 位之间")
		return
	}
	repo := &datumUserRepo{conn: s.conn}
	user, err := repo.findByUserID(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "账号查询失败")
		return
	}
	if !verifyDatumSecret(user.Password, req.OldPassword) {
		fail(c, http.StatusBadRequest, "密码错误")
		return
	}
	passwordHash, err := hashDatumSecret(req.NewPassword)
	if err != nil {
		fail(c, http.StatusInternalServerError, "密码修改失败")
		return
	}
	if err := repo.updatePassword(userID, passwordHash); err != nil {
		fail(c, http.StatusInternalServerError, "密码修改失败")
		return
	}
	success(c, true)
}

type datumSecurityAnswerSet struct {
	SecurityAnswerOne   string `json:"securityAnswerOne"`
	SecurityAnswerTwo   string `json:"securityAnswerTwo"`
	SecurityAnswerThree string `json:"securityAnswerThree"`
}

type datumResetBySecurityRequest struct {
	datumSecurityAnswerSet
	NewPassword string `json:"newPassword"`
}

// datumResetPasswordBySecurity is the logged-in variant of the anonymous
// recovery flow: the session replaces the captcha, the three stored answers
// still gate the change.
func (s *Store) datumResetPasswordBySecurity(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumResetBySecurityRequest
	_ = c.ShouldBind(&req)
	if len(req.NewPassword) < 5 || len(req.NewPassword) > 64 {
		fail(c, http.StatusBadRequest, "密码格式错误：长度需在 5 到 64 位之间")
		return
	}
	repo := &datumUserRepo{conn: s.conn}
	if !s.datumAnswersMatch(repo, userID, req.datumSecurityAnswerSet) {
		fail(c, http.StatusBadRequest, "密保答案不正确")
		return
	}
	passwordHash, err := hashDatumSecret(req.NewPassword)
	if err != nil {
		fail(c, http.StatusInternalServerError, "密码重置失败")
		return
	}
	if err := repo.updatePassword(userID, passwordHash); err != nil {
		fail(c, http.StatusInternalServerError, "密码重置失败")
		return
	}
	success(c, true)
}

func (s *Store) datumGetSecurity(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	question, _, found, err := (&datumUserRepo{conn: s.conn}).securityRow(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "密保查询失败")
		return
	}
	questions := []string{}
	if found {
		for _, item := range strings.Split(question, "|") {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				questions = append(questions, trimmed)
			}
		}
	}
	success(c, gin.H{"questions": questions})
}

type datumUpdateSecurityRequest struct {
	datumSecurityAnswerSet
	SecurityQuestionOne   string `json:"securityQuestionOne"`
	SecurityQuestionTwo   string `json:"securityQuestionTwo"`
	SecurityQuestionThree string `json:"securityQuestionThree"`
}

func (s *Store) datumUpdateSecurity(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumUpdateSecurityRequest
	_ = c.ShouldBind(&req)
	questions := []string{
		strings.TrimSpace(req.SecurityQuestionOne),
		strings.TrimSpace(req.SecurityQuestionTwo),
		strings.TrimSpace(req.SecurityQuestionThree),
	}
	answers := []string{
		strings.TrimSpace(req.SecurityAnswerOne),
		strings.TrimSpace(req.SecurityAnswerTwo),
		strings.TrimSpace(req.SecurityAnswerThree),
	}
	for index := 0; index < 3; index++ {
		if questions[index] == "" {
			fail(c, http.StatusBadRequest, "密保问题不能为空")
			return
		}
		if answers[index] == "" {
			fail(c, http.StatusBadRequest, "密保答案不能为空")
			return
		}
	}
	hashes := make([]string, 0, 3)
	for _, answer := range answers {
		hash, err := hashDatumSecret(answer)
		if err != nil {
			fail(c, http.StatusInternalServerError, "密保更新失败")
			return
		}
		hashes = append(hashes, hash)
	}
	repo := &datumUserRepo{conn: s.conn}
	if err := repo.updateSecurity(userID, strings.Join(questions, "|"), strings.Join(hashes, "|")); err != nil {
		fail(c, http.StatusInternalServerError, "密保更新失败")
		return
	}
	success(c, true)
}

type datumAnswerVerifyRequest struct {
	Answer string `json:"answer"`
}

// datumVerifySecurityAnswer checks one candidate against all three stored
// answers (any-position match), backing UIs that confirm a single answer.
func (s *Store) datumVerifySecurityAnswer(c *gin.Context) {
	userID, _ := datumUserIDFrom(c)
	var req datumAnswerVerifyRequest
	_ = c.ShouldBind(&req)
	candidate := strings.TrimSpace(req.Answer)
	if candidate == "" {
		fail(c, http.StatusBadRequest, "密保答案不能为空")
		return
	}
	_, stored, found, err := (&datumUserRepo{conn: s.conn}).securityRow(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "密保查询失败")
		return
	}
	matched := false
	if found {
		for _, item := range strings.Split(stored, "|") {
			if verifyDatumSecret(item, candidate) {
				matched = true
				break
			}
		}
	}
	success(c, gin.H{"verified": matched})
}

func (s *Store) datumAnswersMatch(repo *datumUserRepo, userID int64, answers datumSecurityAnswerSet) bool {
	_, stored, found, err := repo.securityRow(userID)
	if err != nil || !found {
		return false
	}
	parts := strings.Split(stored, "|")
	candidates := []string{answers.SecurityAnswerOne, answers.SecurityAnswerTwo, answers.SecurityAnswerThree}
	if len(parts) < len(candidates) {
		return false
	}
	for index, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" || !verifyDatumSecret(parts[index], strings.TrimSpace(candidate)) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// avatar storage: MinIO when configured, local upload dir as fallback — the
// same degradation the admin files service applies.
// ---------------------------------------------------------------------------

func putDatumAvatar(ctx context.Context, objectKey string, body []byte, contentType string) (string, string, error) {
	if datumEnvBool("KADMIN_MINIO_ENABLED") {
		minio := storage.NewMinio(storage.MinioConfig{
			Endpoints: []string{datumEnv("KADMIN_MINIO_ENDPOINT", "127.0.0.1:19000"), datumEnv("KADMIN_MINIO_INTERNAL_ENDPOINT", "minio:9000")},
			AccessKey: datumEnv("KADMIN_MINIO_ACCESS_KEY", "kadmin_minio"),
			SecretKey: datumEnv("KADMIN_MINIO_SECRET_KEY", "kadmin_minio_pwd"),
			Bucket:    datumEnv("KADMIN_MINIO_BUCKET", "kadmin"),
			UseSSL:    datumEnvBool("KADMIN_MINIO_USE_SSL"),
			Region:    datumEnv("KADMIN_MINIO_REGION", "us-east-1"),
			Timeout:   3 * time.Second,
		})
		if err := minio.Put(ctx, objectKey, bytes.NewReader(body), int64(len(body)), contentType); err == nil {
			if builder, buildErr := fileurl.FromEnv(datumEnv); buildErr == nil {
				if url, urlErr := builder.ObjectURL(objectKey); urlErr == nil {
					return url, "minio", nil
				}
			}
			// Stored remotely but the public base is unconfigured: fall
			// through to the local copy so the returned URL is servable.
		}
	}
	local := storage.NewLocal(datumEnv("KADMIN_DATUM_AVATAR_LOCAL_ROOT", datumAvatarLocalDir))
	if err := local.Put(ctx, objectKey, bytes.NewReader(body), int64(len(body)), contentType); err != nil {
		return "", "", err
	}
	return "/api/uploads/" + storage.EscapePath(objectKey), "local", nil
}

func datumEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func datumEnvBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
