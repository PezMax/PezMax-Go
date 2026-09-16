package monitor

import (
	"net/http"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Connection        db.Connection
	RequireAuth       gin.HandlerFunc
	RequirePermission func(...string) gin.HandlerFunc
}

func Register(api *gin.RouterGroup, dependencies Dependencies) (*Manager, error) {
	if err := EnsureSchema(dependencies.Connection); err != nil {
		return nil, err
	}
	manager, err := NewManager(ManagerOptions{Settings: &repository{conn: dependencies.Connection}})
	if err != nil {
		return nil, err
	}
	RegisterRoutes(api, manager, dependencies)
	return manager, nil
}

func RegisterRoutes(api *gin.RouterGroup, manager *Manager, dependencies Dependencies) {
	group := api.Group("/system-monitor", dependencies.RequireAuth)
	group.GET("", dependencies.RequirePermission(ViewPermission), func(c *gin.Context) {
		httpx.Success(c, manager.Status())
	})
	group.PATCH("/status", dependencies.RequirePermission(UpdatePermission), func(c *gin.Context) {
		var payload struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil || payload.Enabled == nil {
			httpx.Fail(c, http.StatusBadRequest, "invalid monitor status")
			return
		}
		status, err := manager.SetEnabled(*payload.Enabled, currentUserID(c))
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.Success(c, status)
	})
}

func currentUserID(c *gin.Context) int64 {
	value, exists := c.Get("vben_user")
	if !exists {
		return 0
	}
	user, ok := value.(models.UserModel)
	if !ok {
		return 0
	}
	return user.Id
}
