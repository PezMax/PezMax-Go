package kadmin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerDatumRoutes mounts the application-side (PezMax desktop client)
// adapter routes. They stay outside /api because the desktop app calls
// RuoYi-origin paths anonymously, e.g. GET /datum/user/captchaImage from the
// login, registration and password recovery pages. Responses use the native
// KAdmin envelope; the desktop client normalizes error status codes and
// pagination centrally in its request interceptor.
func registerDatumRoutes(r *gin.Engine, s *Store) {
	datum := r.Group("/datum", cors())
	datum.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	user := datum.Group("/user")
	user.GET("/captchaImage", s.datumCaptchaImage)
}

// datumCaptchaImage implements GET /datum/user/captchaImage: the payload
// carries captchaEnabled/uuid/img, where img is a bare base64 JPEG payload
// (the desktop client prepends its own data URI prefix). The desktop captcha
// stays always on: security.captcha_enabled only governs the admin console
// login, so the switch is intentionally not consulted here.
func (s *Store) datumCaptchaImage(c *gin.Context) {
	policy := s.loadSecurityPolicy()
	challenge, err := s.security.issueCaptchaRendered(captchaJPEG, policy.CaptchaTTL)
	if err != nil {
		fail(c, http.StatusServiceUnavailable, "验证码获取失败，请稍后重试")
		return
	}
	success(c, gin.H{
		"captchaEnabled": true,
		"uuid":           challenge.ID,
		"img":            challenge.Image,
	})
}

// verifyDatumCaptcha checks the uuid/code pair submitted by the desktop
// client against the one-time challenge store and answers with a real HTTP
// status code on failure. Challenges are single use and share the storage
// namespace with the admin console captchas, so login, registration and
// password recovery can all funnel through this helper.
func (s *Store) verifyDatumCaptcha(c *gin.Context, uuid, code string) bool {
	if err := s.security.verifyCaptcha(uuid, code); err != nil {
		if errors.Is(err, errCaptchaInvalid) {
			fail(c, http.StatusBadRequest, "验证码错误或已过期")
		} else {
			fail(c, http.StatusServiceUnavailable, "验证码服务暂不可用，请稍后重试")
		}
		return false
	}
	return true
}
