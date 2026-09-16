package loginlogs

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	manager, err := NewManager(&repository{conn: dependencies.Connection})
	if err != nil {
		return nil, err
	}
	RegisterRoutes(api, manager, dependencies)
	return manager, nil
}

func RegisterRoutes(api *gin.RouterGroup, manager *Manager, dependencies Dependencies) {
	group := api.Group("/login-audits", dependencies.RequireAuth)
	group.GET("", dependencies.RequirePermission(ListPermission), func(c *gin.Context) {
		filter, err := parseFilter(c)
		if err != nil {
			httpx.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		page, err := manager.List(filter)
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.Success(c, page)
	})
	group.DELETE("", dependencies.RequirePermission(DeletePermission), func(c *gin.Context) {
		var payload struct {
			IDs []int64 `json:"ids"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			httpx.Fail(c, http.StatusBadRequest, "invalid login audit ids")
			return
		}
		ids := uniqueIDs(payload.IDs)
		if len(ids) == 0 || len(ids) > maxPageSize {
			httpx.Fail(c, http.StatusBadRequest, "select between 1 and 100 login audits")
			return
		}
		result, err := manager.Delete(ids)
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.Success(c, result)
	})
	group.POST("/cleanup", dependencies.RequirePermission(DeletePermission), func(c *gin.Context) {
		result, err := manager.CleanupExpired()
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.Success(c, result)
	})
	group.GET("/retention", dependencies.RequirePermission(ListPermission), func(c *gin.Context) {
		setting, err := manager.Retention()
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.Success(c, setting)
	})
	group.PATCH("/retention", dependencies.RequirePermission(RetentionPermission), func(c *gin.Context) {
		var payload struct {
			Days int `json:"days"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil || payload.Days < 1 || payload.Days > maxRetentionDays {
			httpx.Fail(c, http.StatusBadRequest, fmt.Sprintf("retention days must be between 1 and %d", maxRetentionDays))
			return
		}
		setting, err := manager.SetRetention(payload.Days, currentUserID(c))
		if err != nil {
			httpx.Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.Success(c, setting)
	})
}

func parseFilter(c *gin.Context) (Filter, error) {
	filter := Filter{
		Page: positiveInt(c.Query("page"), 1), PageSize: positiveInt(c.Query("pageSize"), 20),
		Account: truncate(strings.TrimSpace(c.Query("account")), 100), IP: truncate(strings.TrimSpace(c.Query("ip")), 45),
		Status: strings.TrimSpace(c.Query("status")), Result: strings.TrimSpace(c.Query("result")),
	}
	if filter.PageSize > maxPageSize {
		filter.PageSize = maxPageSize
	}
	if filter.Status != "" && filter.Status != StatusSuccess && filter.Status != StatusFailed {
		return filter, fmt.Errorf("invalid login status")
	}
	if filter.Result != "" && !validResult(filter.Result) {
		return filter, fmt.Errorf("invalid login result")
	}
	var err error
	if filter.StartedAt, err = optionalTime(c.Query("startedAt")); err != nil {
		return filter, fmt.Errorf("invalid startedAt")
	}
	if filter.EndedAt, err = optionalTime(c.Query("endedAt")); err != nil {
		return filter, fmt.Errorf("invalid endedAt")
	}
	if filter.StartedAt != nil && filter.EndedAt != nil && filter.EndedAt.Before(*filter.StartedAt) {
		return filter, fmt.Errorf("endedAt must not precede startedAt")
	}
	return filter, nil
}

func optionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid time")
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func uniqueIDs(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func currentUserID(c *gin.Context) int64 {
	if value, exists := c.Get("vben_user_id"); exists {
		if id, ok := value.(int64); ok {
			return id
		}
	}
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
