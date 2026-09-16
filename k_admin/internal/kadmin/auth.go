package kadmin

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loginlogs"
	"github.com/GoAdminGroup/go-admin/modules/auth"
	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username      string `json:"username" form:"username"`
	Password      string `json:"password" form:"password"`
	CaptchaID     string `json:"captchaId" form:"captchaId"`
	CaptchaAnswer string `json:"captchaAnswer" form:"captchaAnswer"`
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" form:"refreshToken"`
}

type loginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Token        string `json:"token"`
	TokenType    string `json:"tokenType"`
	ExpiresAt    int64  `json:"expiresAt"`
}

func registerAuthRoutes(api *gin.RouterGroup, s *Store) {
	authGroup := api.Group("/auth")
	authGroup.GET("/captcha", s.captcha)
	authGroup.POST("/login", s.login)
	authGroup.POST("/refresh", s.refreshToken)
	authGroup.POST("/logout", s.logout)
	authGroup.GET("/codes", s.requireAuth(), s.accessCodes)
}

func (s *Store) captcha(c *gin.Context) {
	policy := s.loadSecurityPolicy()
	if !policy.CaptchaEnabled {
		success(c, gin.H{"enabled": false})
		return
	}
	challenge, err := s.security.issueCaptcha(policy.CaptchaTTL)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "captcha storage unavailable")
		return
	}
	success(c, gin.H{
		"enabled":   true,
		"id":        challenge.ID,
		"image":     challenge.Image,
		"expiresIn": challenge.ExpiresIn,
	})
}

func (s *Store) login(c *gin.Context) {
	startedAt := time.Now()
	var req loginRequest
	_ = c.ShouldBind(&req)
	if req.Username == "" {
		req.Username = c.PostForm("username")
	}
	if req.Password == "" {
		req.Password = c.PostForm("password")
	}
	policy := s.loadSecurityPolicy()
	if policy.CaptchaEnabled {
		if err := s.security.verifyCaptcha(req.CaptchaID, req.CaptchaAnswer); err != nil {
			s.recordLoginAttempt(c, startedAt, req.Username, nil, loginlogs.ResultCaptchaInvalid, "验证码错误或已过期")
			if errors.Is(err, errCaptchaInvalid) {
				fail(c, http.StatusBadRequest, "captcha invalid or expired")
			} else {
				fail(c, http.StatusServiceUnavailable, "captcha storage unavailable")
			}
			return
		}
	}
	locked, err := s.security.loginLocked(req.Username, c.ClientIP(), policy)
	if err != nil {
		s.recordLoginAttempt(c, startedAt, req.Username, nil, loginlogs.ResultSystemError, "登录锁定状态检查失败")
		fail(c, http.StatusServiceUnavailable, "login security storage unavailable")
		return
	}
	if locked {
		s.recordLoginAttempt(c, startedAt, req.Username, nil, loginlogs.ResultAccountLocked, "登录失败次数达到阈值")
		fail(c, http.StatusTooManyRequests, "too many login attempts; try again later")
		return
	}

	knownUserID, accountStatus, err := s.loginAccountState(req.Username)
	if err != nil {
		s.recordLoginAttempt(c, startedAt, req.Username, nil, loginlogs.ResultSystemError, "账号状态检查失败")
		fail(c, http.StatusInternalServerError, "account lookup failed")
		return
	}
	if accountStatus == "locked" || accountStatus == "lock" {
		s.recordLoginAttempt(c, startedAt, req.Username, knownUserID, loginlogs.ResultAccountLocked, "账号已锁定")
		fail(c, http.StatusForbidden, "account locked")
		return
	}

	user, ok := auth.Check(req.Password, req.Username, s.conn)
	if !ok {
		user, ok = s.checkConfiguredDefaultAdmin(req.Username, req.Password)
		if !ok {
			result := loginlogs.ResultInvalidPassword
			userID := knownUserID
			if knownUserID == nil {
				result = loginlogs.ResultAccountNotFound
			}
			lockedNow, lockErr := s.security.recordLoginFailure(req.Username, c.ClientIP(), policy)
			if lockErr != nil {
				s.recordLoginAttempt(c, startedAt, req.Username, userID, loginlogs.ResultSystemError, "登录失败计数写入失败")
				fail(c, http.StatusServiceUnavailable, "login security storage unavailable")
				return
			}
			reason := "账号或密码错误"
			if lockedNow {
				reason = "登录失败次数达到阈值"
				result = loginlogs.ResultAccountLocked
			}
			s.recordLoginAttempt(c, startedAt, req.Username, userID, result, reason)
			if lockedNow {
				fail(c, http.StatusTooManyRequests, "too many login attempts; try again later")
			} else {
				fail(c, http.StatusUnauthorized, "wrong username or password")
			}
			return
		}
	}
	accountStatus, found, err := s.userAccountStatus(user.Id)
	if err != nil {
		userID := user.Id
		s.recordLoginAttempt(c, startedAt, req.Username, &userID, loginlogs.ResultSystemError, "账号状态检查失败")
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if found && (accountStatus == "locked" || accountStatus == "lock") {
		userID := user.Id
		s.recordLoginAttempt(c, startedAt, req.Username, &userID, loginlogs.ResultAccountLocked, "账号已锁定")
		fail(c, http.StatusForbidden, "account locked")
		return
	}
	if !found || accountStatus != "enable" {
		userID := user.Id
		s.recordLoginAttempt(c, startedAt, req.Username, &userID, loginlogs.ResultAccountDisabled, "账号已禁用")
		fail(c, http.StatusForbidden, "account disabled")
		return
	}

	if err := s.security.clearLoginFailures(req.Username, c.ClientIP()); err != nil {
		userID := user.Id
		s.recordLoginAttempt(c, startedAt, req.Username, &userID, loginlogs.ResultSystemError, "登录失败计数清理失败")
		fail(c, http.StatusServiceUnavailable, "login security storage unavailable")
		return
	}

	tokens, err := s.auth.issueTokenPair(user.Id)
	if err != nil {
		userID := user.Id
		s.recordLoginAttempt(c, startedAt, req.Username, &userID, loginlogs.ResultSystemError, "令牌签发失败")
		fail(c, http.StatusInternalServerError, "create token failed: "+err.Error())
		return
	}
	c.Set("vben_user_id", user.Id)
	c.Set("vben_user", user)
	userID := user.Id
	s.recordLoginAttempt(c, startedAt, req.Username, &userID, loginlogs.ResultSuccess, "")

	success(c, loginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Token:        tokens.AccessToken,
		TokenType:    "Bearer",
		ExpiresAt:    tokens.AccessExpiresAt.UnixMilli(),
	})
}

func (s *Store) recordLoginAttempt(c *gin.Context, startedAt time.Time, account string, userID *int64, result, reason string) {
	if s.loginLogs == nil {
		return
	}
	duration := time.Since(startedAt).Milliseconds()
	if err := s.loginLogs.Record(loginlogs.Attempt{
		Account: account, UserID: userID, IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
		Result: result, FailureReason: reason, DurationMs: duration, OccurredAt: time.Now(),
	}); err != nil {
		log.Printf("kadmin login audit write failed: %v", err)
	}
}

func (s *Store) refreshToken(c *gin.Context) {
	refreshToken := refreshTokenFromRequest(c)
	if refreshToken == "" {
		fail(c, http.StatusUnauthorized, "missing refresh token")
		return
	}

	userID, err := s.auth.consumeRefreshToken(refreshToken)
	if err != nil {
		if err == errInvalidRefreshToken {
			fail(c, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		fail(c, http.StatusInternalServerError, "auth storage unavailable")
		return
	}

	enabled, err := s.userAccountEnabled(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !enabled {
		fail(c, http.StatusForbidden, "account disabled")
		return
	}

	tokens, err := s.auth.issueTokenPair(userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "create token failed")
		return
	}

	success(c, loginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Token:        tokens.AccessToken,
		TokenType:    "Bearer",
		ExpiresAt:    tokens.AccessExpiresAt.UnixMilli(),
	})
}

func (s *Store) logout(c *gin.Context) {
	if token := tokenFromRequest(c); token != "" {
		if claims, err := s.auth.parseAccessToken(token); err == nil {
			if err := s.auth.blacklistAccessToken(claims); err != nil {
				fail(c, http.StatusInternalServerError, "auth storage unavailable")
				return
			}
		}
	}

	if err := s.auth.revokeRefreshToken(refreshTokenFromRequest(c)); err != nil {
		fail(c, http.StatusInternalServerError, "auth storage unavailable")
		return
	}

	success(c, true)
}

func refreshTokenFromRequest(c *gin.Context) string {
	var req refreshTokenRequest
	_ = c.ShouldBind(&req)
	if token := strings.TrimSpace(req.RefreshToken); token != "" {
		return token
	}
	if token := strings.TrimSpace(c.GetHeader("X-Refresh-Token")); token != "" {
		return token
	}
	if token := strings.TrimSpace(c.Query("refreshToken")); token != "" {
		return token
	}
	return ""
}
