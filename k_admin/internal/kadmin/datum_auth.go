package kadmin

import (
	"errors"
	"net/http"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/gin-gonic/gin"
)

// registerDatumRoutes mounts the application-side (PezMax desktop client)
// adapter routes. They stay outside /api because the desktop app calls
// RuoYi-style paths anonymously, e.g. GET /datum/user/captchaImage from the
// login, registration and password recovery pages.
func registerDatumRoutes(r *gin.Engine, s *Store) {
	datum := r.Group("/datum", cors())
	datum.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	user := datum.Group("/user")
	user.GET("/captchaImage", s.datumCaptchaImage)
}

// datumCaptchaImage implements GET /datum/user/captchaImage following the
// RuoYi contract the desktop client parses: the envelope wraps a payload of
// captchaEnabled/uuid/img, where img is a bare base64 JPEG payload. The
// desktop captcha stays always on: security.captcha_enabled only governs the
// admin console login, so the switch is intentionally not consulted here.
func (s *Store) datumCaptchaImage(c *gin.Context) {
	policy := s.loadSecurityPolicy()
	challenge, err := s.security.issueCaptchaRendered(captchaJPEG, policy.CaptchaTTL)
	if err != nil {
		datumFail(c, "验证码获取失败，请稍后重试")
		return
	}
	datumSuccess(c, gin.H{
		"captchaEnabled": true,
		"uuid":           challenge.ID,
		"img":            challenge.Image,
	})
}

// verifyDatumCaptcha checks the uuid/code pair submitted by the desktop
// client against the one-time challenge store and answers with the RuoYi-style
// business error on failure. Challenges are single use and share the storage
// namespace with the admin console captchas, so login, registration and
// password recovery can all funnel through this helper.
func (s *Store) verifyDatumCaptcha(c *gin.Context, uuid, code string) bool {
	if err := s.security.verifyCaptcha(uuid, code); err != nil {
		if errors.Is(err, errCaptchaInvalid) {
			datumFail(c, "验证码错误或已过期")
		} else {
			datumFail(c, "验证码服务暂不可用，请稍后重试")
		}
		return false
	}
	return true
}

func datumSuccess(c *gin.Context, data interface{}) {
	httpx.Success(c, data)
}

// datumFail mirrors RuoYi AjaxResult.error: HTTP 200 with a code 500 body so
// the desktop request interceptor surfaces the business message instead of a
// generic network-error banner.
func datumFail(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"code":    http.StatusInternalServerError,
		"message": message,
		"msg":     message,
		"data":    nil,
	})
}
