package kadmin

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Datum (PezMax desktop client) identity: an intentionally simple, self
// contained session scheme. It deliberately does NOT share the admin JWT
// machinery — the desktop gets an opaque bearer token backed by Redis with a
// five-day TTL, enough for basic constraints (captcha, login throttling,
// banned-status checks) against the existing ptmj_user table.

const (
	datumSessionTTLDefault = 120 * time.Hour // 5 days per the migration spec
	datumResetTicketTTL    = 10 * time.Minute
	datumSessionTTLKey     = "KADMIN_DATUM_SESSION_TTL"
	datumRole              = "ptmj_user"
)

var (
	errDatumSessionInvalid = errors.New("datum session invalid")
	errDatumUserNotFound   = errors.New("datum user not found")
)

type datumIdentity struct {
	keyPrefix  string
	redis      redisDoer
	sessionTTL time.Duration
}

func newDatumIdentity(keyPrefix string, redis redisDoer, sessionTTL time.Duration) *datumIdentity {
	if sessionTTL <= 0 {
		sessionTTL = datumSessionTTLDefault
	}
	return &datumIdentity{keyPrefix: keyPrefix, redis: redis, sessionTTL: sessionTTL}
}

func datumSessionTTL() time.Duration {
	if parsed, err := time.ParseDuration(strings.TrimSpace(os.Getenv(datumSessionTTLKey))); err == nil && parsed > time.Minute {
		return parsed
	}
	return datumSessionTTLDefault
}

func (d *datumIdentity) sessionKey(token string) string {
	return d.keyPrefix + ":datum:session:" + tokenHash(token)
}

// IssueSession mints an opaque bearer token bound to a ptmj_user id.
func (d *datumIdentity) IssueSession(userID int64) (string, time.Time, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(d.sessionTTL)
	result, err := d.redis.do("SET", d.sessionKey(token), strconv.FormatInt(userID, 10), "EX", durationSeconds(d.sessionTTL))
	if err != nil {
		return "", time.Time{}, err
	}
	if result != "OK" {
		return "", time.Time{}, errors.New("datum session was not stored")
	}
	return token, expiresAt, nil
}

// ResolveSession maps a bearer token back to its ptmj_user id.
func (d *datumIdentity) ResolveSession(token string) (int64, error) {
	result, err := d.redis.do("GET", d.sessionKey(token))
	if errors.Is(err, errRedisNil) {
		return 0, errDatumSessionInvalid
	}
	if err != nil {
		return 0, err
	}
	value, ok := result.(string)
	if !ok {
		return 0, errDatumSessionInvalid
	}
	userID, parseErr := strconv.ParseInt(value, 10, 64)
	if parseErr != nil || userID <= 0 {
		return 0, errDatumSessionInvalid
	}
	return userID, nil
}

func (d *datumIdentity) RevokeSession(token string) error {
	_, err := d.redis.do("DEL", d.sessionKey(token))
	return err
}

// Password recovery is a two-step flow sharing one single-use captcha: the
// first step (securityQuestions) consumes the captcha and mints a short-lived
// ticket bound to uuid+username; the second step (resetPasswordBySecurity)
// consumes the ticket instead of the captcha.
func (d *datumIdentity) resetTicketKey(uuid, username string) string {
	return d.keyPrefix + ":datum:reset:" + tokenHash(uuid+":"+normalizeAccount(username))
}

func (d *datumIdentity) IssueResetTicket(uuid, username string) error {
	result, err := d.redis.do("SET", d.resetTicketKey(uuid, username), normalizeAccount(username), "EX", durationSeconds(datumResetTicketTTL))
	if err != nil {
		return err
	}
	if result != "OK" {
		return errors.New("datum reset ticket was not stored")
	}
	return nil
}

func (d *datumIdentity) ConsumeResetTicket(uuid, username string) (bool, error) {
	result, err := d.redis.do("GETDEL", d.resetTicketKey(uuid, username))
	if errors.Is(err, errRedisNil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	value, ok := result.(string)
	return ok && value == normalizeAccount(username), nil
}

// ---------------------------------------------------------------------------
// ptmj_user / ptmj_security access (small dedicated queries; the generated
// CRUD module has no username lookup or password updates)
// ---------------------------------------------------------------------------

type datumDB interface {
	Query(query string, args ...interface{}) ([]map[string]interface{}, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type datumUser struct {
	UserID   int64
	UserName string
	Password string
	Avatar   string
	Count    int64
	Status   string
}

type datumUserRepo struct {
	conn datumDB
}

const datumUserColumns = "user_id, user_name, password, avatar, count, status"

func (r *datumUserRepo) scanUser(row map[string]interface{}) *datumUser {
	return &datumUser{
		UserID:   toDatumInt64(row["user_id"]),
		UserName: toDatumString(row["user_name"]),
		Password: toDatumString(row["password"]),
		Avatar:   toDatumString(row["avatar"]),
		Count:    toDatumInt64(row["count"]),
		Status:   toDatumString(row["status"]),
	}
}

func (r *datumUserRepo) findByUserName(userName string) (*datumUser, error) {
	rows, err := r.conn.Query(`SELECT `+datumUserColumns+` FROM ptmj_user WHERE user_name = ?`, strings.TrimSpace(userName))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errDatumUserNotFound
	}
	return r.scanUser(rows[0]), nil
}

func (r *datumUserRepo) findByUserID(userID int64) (*datumUser, error) {
	rows, err := r.conn.Query(`SELECT `+datumUserColumns+` FROM ptmj_user WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errDatumUserNotFound
	}
	return r.scanUser(rows[0]), nil
}

func (r *datumUserRepo) create(userName, passwordHash, avatar string) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_user (user_name, password, avatar, status, count, create_time, update_time)
		VALUES (?, ?, ?, '1', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING user_id`,
		strings.TrimSpace(userName), passwordHash, avatar)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errors.New("ptmj_user insert returned no id")
	}
	return toDatumInt64(rows[0]["user_id"]), nil
}

func (r *datumUserRepo) updatePassword(userID int64, passwordHash string) error {
	_, err := r.conn.Exec(`UPDATE ptmj_user SET password = ?, update_time = CURRENT_TIMESTAMP WHERE user_id = ?`, passwordHash, userID)
	return err
}

// securityRow reads the pipe-separated Q/A row: question = q1|q2|q3,
// answer = bcrypt(a1)|bcrypt(a2)|bcrypt(a3).
func (r *datumUserRepo) securityRow(userID int64) (question, answer string, found bool, err error) {
	rows, err := r.conn.Query(`SELECT question, answer FROM ptmj_security WHERE user_id = ?`, userID)
	if err != nil {
		return "", "", false, err
	}
	if len(rows) == 0 {
		return "", "", false, nil
	}
	return toDatumString(rows[0]["question"]), toDatumString(rows[0]["answer"]), true, nil
}

func (r *datumUserRepo) createSecurity(userID int64, question, answer string) error {
	_, err := r.conn.Exec(`INSERT INTO ptmj_security (user_id, question, answer, create_time, update_time)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, userID, question, answer)
	return err
}

func toDatumInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case []byte:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(string(typed)), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

func toDatumString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	}
}

// ---------------------------------------------------------------------------
// password / answer verification (BCrypt with legacy plaintext fallback per
// the migration spec)
// ---------------------------------------------------------------------------

func verifyDatumSecret(stored, candidate string) bool {
	if stored == "" || candidate == "" {
		return false
	}
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		if bcrypt.CompareHashAndPassword([]byte(stored), []byte(candidate)) == nil {
			return true
		}
		return false
	}
	// Legacy rows migrated as plaintext: constant-time compare.
	return subtle.ConstantTimeCompare([]byte(stored), []byte(candidate)) == 1
}

func hashDatumSecret(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ---------------------------------------------------------------------------
// routes & handlers
// ---------------------------------------------------------------------------

func (s *Store) registerDatumUserRoutes(user *gin.RouterGroup) {
	user.GET("/captchaImage", s.datumCaptchaImage)
	user.POST("/login", s.datumLogin)
	user.POST("/register", s.datumRegister)
	user.GET("/securityQuestions", s.datumSecurityQuestions)
	user.POST("/resetPasswordBySecurity", s.datumResetPassword)
	user.GET("/getInfo", s.requireDatumAuth(), s.datumGetInfo)
	user.POST("/logout", s.requireDatumAuth(), s.datumLogout)
}

func (s *Store) requireDatumAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := tokenFromRequest(c)
		if token == "" || s.datum == nil {
			fail(c, http.StatusUnauthorized, "会话已过期，请重新登录")
			c.Abort()
			return
		}
		userID, err := s.datum.ResolveSession(token)
		if err != nil {
			if !errors.Is(err, errDatumSessionInvalid) {
				fail(c, http.StatusServiceUnavailable, "身份存储不可用")
				c.Abort()
				return
			}
			fail(c, http.StatusUnauthorized, "会话已过期，请重新登录")
			c.Abort()
			return
		}
		c.Set("datum_user_id", userID)
		c.Next()
	}
}

func datumUserIDFrom(c *gin.Context) (int64, bool) {
	if value, ok := c.Get("datum_user_id"); ok {
		if userID, ok := value.(int64); ok && userID > 0 {
			return userID, true
		}
	}
	return 0, false
}

type datumLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Code     string `json:"code"`
	UUID     string `json:"uuid"`
}

func (s *Store) datumLogin(c *gin.Context) {
	var req datumLoginRequest
	_ = c.ShouldBind(&req)
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, "用户名或密码错误")
		return
	}
	policy := s.loadSecurityPolicy()
	if !s.verifyDatumCaptcha(c, req.UUID, req.Code) {
		return
	}
	locked, err := s.security.loginLocked(req.Username, c.ClientIP(), policy)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "登录安全存储不可用")
		return
	}
	if locked {
		fail(c, http.StatusTooManyRequests, "登录失败次数过多，账号已锁定，请稍后再试")
		return
	}

	repo := &datumUserRepo{conn: s.conn}
	user, err := repo.findByUserName(req.Username)
	if err != nil {
		if !errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusInternalServerError, "账号查询失败")
			return
		}
		_, _ = s.security.recordLoginFailure(req.Username, c.ClientIP(), policy)
		fail(c, http.StatusBadRequest, "用户名或密码错误")
		return
	}
	if strings.TrimSpace(user.Status) == "0" {
		fail(c, http.StatusForbidden, "账号已被停用")
		return
	}
	if !verifyDatumSecret(user.Password, req.Password) {
		lockedNow, lockErr := s.security.recordLoginFailure(req.Username, c.ClientIP(), policy)
		if lockErr != nil {
			fail(c, http.StatusServiceUnavailable, "登录安全存储不可用")
			return
		}
		if lockedNow {
			fail(c, http.StatusTooManyRequests, "登录失败次数过多，账号已锁定，请稍后再试")
			return
		}
		fail(c, http.StatusBadRequest, "用户名或密码错误")
		return
	}
	if err := s.security.clearLoginFailures(req.Username, c.ClientIP()); err != nil {
		fail(c, http.StatusServiceUnavailable, "登录安全存储不可用")
		return
	}

	token, expiresAt, err := s.datum.IssueSession(user.UserID)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "会话创建失败，请稍后重试")
		return
	}
	success(c, gin.H{
		"token":     token,
		"expiresAt": expiresAt.UnixMilli(),
	})
}

type datumRegisterRequest struct {
	Username              string `json:"username"`
	Password              string `json:"password"`
	ConfirmPassword       string `json:"confirmPassword"`
	SecurityQuestionOne   string `json:"securityQuestionOne"`
	SecurityAnswerOne     string `json:"securityAnswerOne"`
	SecurityQuestionTwo   string `json:"securityQuestionTwo"`
	SecurityAnswerTwo     string `json:"securityAnswerTwo"`
	SecurityQuestionThree string `json:"securityQuestionThree"`
	SecurityAnswerThree   string `json:"securityAnswerThree"`
	Code                  string `json:"code"`
	UUID                  string `json:"uuid"`
	Avatar                string `json:"avatar"`
}

func (s *Store) datumRegister(c *gin.Context) {
	var req datumRegisterRequest
	_ = c.ShouldBind(&req)
	req.Username = strings.TrimSpace(req.Username)
	switch {
	case req.Username == "":
		fail(c, http.StatusBadRequest, "用户名不能为空")
		return
	case len([]rune(req.Username)) > 64:
		fail(c, http.StatusBadRequest, "用户名格式错误")
		return
	case len(req.Password) < 5 || len(req.Password) > 64:
		fail(c, http.StatusBadRequest, "密码格式错误：长度需在 5 到 64 位之间")
		return
	case req.Password != req.ConfirmPassword:
		fail(c, http.StatusBadRequest, "两次输入的密码不一致")
		return
	case strings.TrimSpace(req.SecurityQuestionOne) == "" || strings.TrimSpace(req.SecurityQuestionTwo) == "" || strings.TrimSpace(req.SecurityQuestionThree) == "":
		fail(c, http.StatusBadRequest, "密保问题不能为空")
		return
	case strings.TrimSpace(req.SecurityAnswerOne) == "" || strings.TrimSpace(req.SecurityAnswerTwo) == "" || strings.TrimSpace(req.SecurityAnswerThree) == "":
		fail(c, http.StatusBadRequest, "密保答案不能为空")
		return
	case len(req.Avatar) > 255:
		fail(c, http.StatusBadRequest, "头像地址格式错误")
		return
	}
	if !s.verifyDatumCaptcha(c, req.UUID, req.Code) {
		return
	}

	repo := &datumUserRepo{conn: s.conn}
	if existing, err := repo.findByUserName(req.Username); err == nil && existing != nil {
		fail(c, http.StatusConflict, "用户名已存在")
		return
	} else if err != nil && !errors.Is(err, errDatumUserNotFound) {
		fail(c, http.StatusInternalServerError, "账号查询失败")
		return
	}

	passwordHash, err := hashDatumSecret(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "注册失败")
		return
	}
	answerHashes := make([]string, 0, 3)
	for _, answer := range []string{req.SecurityAnswerOne, req.SecurityAnswerTwo, req.SecurityAnswerThree} {
		hash, hashErr := hashDatumSecret(strings.TrimSpace(answer))
		if hashErr != nil {
			fail(c, http.StatusInternalServerError, "注册失败")
			return
		}
		answerHashes = append(answerHashes, hash)
	}

	userID, err := repo.create(req.Username, passwordHash, strings.TrimSpace(req.Avatar))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			fail(c, http.StatusConflict, "用户名已存在")
			return
		}
		fail(c, http.StatusInternalServerError, "注册失败")
		return
	}
	question := strings.Join([]string{
		strings.TrimSpace(req.SecurityQuestionOne),
		strings.TrimSpace(req.SecurityQuestionTwo),
		strings.TrimSpace(req.SecurityQuestionThree),
	}, "|")
	answer := strings.Join(answerHashes, "|")
	if err := repo.createSecurity(userID, question, answer); err != nil {
		fail(c, http.StatusInternalServerError, "注册失败：密保初始化失败")
		return
	}
	success(c, gin.H{"userId": userID})
}

func (s *Store) datumSecurityQuestions(c *gin.Context) {
	userName := strings.TrimSpace(c.Query("userName"))
	if userName == "" {
		fail(c, http.StatusBadRequest, "用户名不能为空")
		return
	}
	if !s.verifyDatumCaptcha(c, c.Query("uuid"), c.Query("code")) {
		return
	}
	repo := &datumUserRepo{conn: s.conn}
	user, err := repo.findByUserName(userName)
	if err != nil {
		if errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusBadRequest, "用户不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "账号查询失败")
		return
	}
	question, _, found, err := repo.securityRow(user.UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "密保查询失败")
		return
	}
	questions := []gin.H{}
	if found {
		for _, item := range strings.Split(question, "|") {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				questions = append(questions, gin.H{"question": trimmed})
			}
		}
	}
	// Captcha was consumed here; mint the short-lived ticket that authorizes
	// the second step (resetPasswordBySecurity).
	if err := s.datum.IssueResetTicket(c.Query("uuid"), userName); err != nil {
		fail(c, http.StatusServiceUnavailable, "身份存储不可用")
		return
	}
	success(c, questions)
}

type datumResetPasswordRequest struct {
	Username            string `json:"username"`
	Code                string `json:"code"`
	UUID                string `json:"uuid"`
	SecurityAnswerOne   string `json:"securityAnswerOne"`
	SecurityAnswerTwo   string `json:"securityAnswerTwo"`
	SecurityAnswerThree string `json:"securityAnswerThree"`
	NewPassword         string `json:"newPassword"`
	ConfirmPassword     string `json:"confirmPassword"`
}

func (s *Store) datumResetPassword(c *gin.Context) {
	var req datumResetPasswordRequest
	_ = c.ShouldBind(&req)
	req.Username = strings.TrimSpace(req.Username)
	switch {
	case req.Username == "" || req.UUID == "" || req.Code == "":
		fail(c, http.StatusBadRequest, "验证码错误或已过期")
		return
	case strings.TrimSpace(req.SecurityAnswerOne) == "" || strings.TrimSpace(req.SecurityAnswerTwo) == "" || strings.TrimSpace(req.SecurityAnswerThree) == "":
		fail(c, http.StatusBadRequest, "密保答案不能为空")
		return
	case len(req.NewPassword) < 5 || len(req.NewPassword) > 64:
		fail(c, http.StatusBadRequest, "密码格式错误：长度需在 5 到 64 位之间")
		return
	case req.NewPassword != req.ConfirmPassword:
		fail(c, http.StatusBadRequest, "两次输入的密码不一致")
		return
	}
	// The captcha itself was consumed by step one; the ticket proves this
	// browser passed the captcha within the last few minutes.
	matched, err := s.datum.ConsumeResetTicket(req.UUID, req.Username)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "身份存储不可用")
		return
	}
	if !matched {
		fail(c, http.StatusBadRequest, "验证码错误或已过期")
		return
	}

	repo := &datumUserRepo{conn: s.conn}
	user, err := repo.findByUserName(req.Username)
	if err != nil {
		if errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusBadRequest, "用户不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "账号查询失败")
		return
	}
	storedQuestion, storedAnswer, found, err := repo.securityRow(user.UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "密保查询失败")
		return
	}
	answers := []string{
		strings.TrimSpace(req.SecurityAnswerOne),
		strings.TrimSpace(req.SecurityAnswerTwo),
		strings.TrimSpace(req.SecurityAnswerThree),
	}
	storedAnswers := strings.Split(storedAnswer, "|")
	if !found || len(storedAnswers) < len(answers) {
		fail(c, http.StatusBadRequest, "密保答案不正确")
		return
	}
	for index, answer := range answers {
		if !verifyDatumSecret(storedAnswers[index], answer) {
			fail(c, http.StatusBadRequest, "密保答案不正确")
			return
		}
	}
	passwordHash, err := hashDatumSecret(req.NewPassword)
	if err != nil {
		fail(c, http.StatusInternalServerError, "密码重置失败")
		return
	}
	if err := repo.updatePassword(user.UserID, passwordHash); err != nil {
		fail(c, http.StatusInternalServerError, "密码重置失败")
		return
	}
	_ = storedQuestion
	success(c, true)
}

func (s *Store) datumGetInfo(c *gin.Context) {
	userID, ok := datumUserIDFrom(c)
	if !ok {
		fail(c, http.StatusUnauthorized, "会话已过期，请重新登录")
		return
	}
	repo := &datumUserRepo{conn: s.conn}
	user, err := repo.findByUserID(userID)
	if err != nil {
		if errors.Is(err, errDatumUserNotFound) {
			fail(c, http.StatusForbidden, "账号不存在")
			return
		}
		fail(c, http.StatusInternalServerError, "账号查询失败")
		return
	}
	success(c, gin.H{
		"user": gin.H{
			"userId":   user.UserID,
			"userName": user.UserName,
			"nickName": user.UserName,
			"avatar":   user.Avatar,
			"count":    user.Count,
			"status":   user.Status,
		},
		"roles":       []string{datumRole},
		"permissions": []string{},
	})
}

func (s *Store) datumLogout(c *gin.Context) {
	if token := tokenFromRequest(c); token != "" && s.datum != nil {
		if err := s.datum.RevokeSession(token); err != nil {
			fail(c, http.StatusServiceUnavailable, "身份存储不可用")
			return
		}
	}
	success(c, true)
}
