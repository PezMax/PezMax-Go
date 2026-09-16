package kadmin

import (
	"net/http"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

func (s *Store) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := tokenFromRequest(c)
		if token == "" {
			fail(c, http.StatusUnauthorized, "missing token")
			c.Abort()
			return
		}

		claims, err := s.auth.parseAccessToken(token)
		if err != nil {
			fail(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		blacklisted, err := s.auth.isAccessTokenBlacklisted(claims.JTI)
		if err != nil {
			fail(c, http.StatusInternalServerError, "auth storage unavailable")
			c.Abort()
			return
		}
		if blacklisted {
			fail(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}

		enabled, err := s.userAccountEnabled(claims.UserID)
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			c.Abort()
			return
		}
		if !enabled {
			_ = s.auth.blacklistAccessToken(claims)
			fail(c, http.StatusForbidden, "account disabled")
			c.Abort()
			return
		}

		c.Set("vben_user_id", claims.UserID)
		c.Set("vben_token", token)
		c.Set("vben_token_jti", claims.JTI)
		c.Next()
	}
}

func (s *Store) currentUser(c *gin.Context) (models.UserModel, bool) {
	idVal, ok := c.Get("vben_user_id")
	if !ok {
		return models.User(), false
	}

	id, ok := idVal.(int64)
	if !ok {
		return models.User(), false
	}

	user := models.User().SetConn(s.conn).Find(id)
	if user.IsEmpty() {
		return user, false
	}

	user = user.WithRoles().WithPermissions().WithMenus()
	return user, true
}

func (s *Store) userAccountEnabled(userID int64) (bool, error) {
	status, found, err := s.userAccountStatus(userID)
	return found && status == "enable", err
}

func (s *Store) userAccountStatus(userID int64) (string, bool, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_users").Where("id", "=", userID).All()
	if err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return normalizeUserStatus(toString(rows[0]["status"])), true, nil
}

func (s *Store) loginAccountState(username string) (*int64, string, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_users").Where("username", "=", username).All()
	if err != nil {
		return nil, "", err
	}
	if len(rows) == 0 {
		return nil, "", nil
	}
	id := toInt64(rows[0]["id"])
	return &id, normalizeUserStatus(toString(rows[0]["status"])), nil
}

func tokenFromRequest(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
		return strings.TrimSpace(authHeader)
	}

	if token := c.GetHeader("Access-Token"); token != "" {
		return token
	}
	if token := c.GetHeader("X-Access-Token"); token != "" {
		return token
	}
	if token := c.Query("token"); token != "" {
		return token
	}
	return ""
}
