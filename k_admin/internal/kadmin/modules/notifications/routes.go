package notifications

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/gin-gonic/gin"
)

// Register installs the notifications schema and routes.
func Register(api *gin.RouterGroup, deps Dependencies) error {
	if err := EnsureSchema(deps.Connection); err != nil {
		return err
	}
	RegisterRoutes(api, deps)
	return nil
}

func RegisterRoutes(api *gin.RouterGroup, deps Dependencies) {
	h := &handler{repo: newRepository(deps.Connection)}
	group := api.Group("/notifications", deps.RequireAuth)
	group.GET("", deps.RequirePermission(ListPermission), h.list)
	group.POST("", deps.RequirePermission(CreatePermission), h.create)
	group.PATCH("/:id/read", deps.RequirePermission(ReadPermission), h.markRead)
	group.DELETE("/:id", deps.RequirePermission(DeletePermission), h.delete)
	// Batch operations live in their own group so gin v1.3 never mixes static
	// and param children.
	batch := api.Group("/notification-batch", deps.RequireAuth)
	batch.PATCH("/read-all", deps.RequirePermission(ReadAllPermission), h.markAllRead)
	batch.DELETE("/read", deps.RequirePermission(ClearReadPermission), h.clearRead)
}

type handler struct {
	repo *repository
}

func (h *handler) list(c *gin.Context) {
	filter := Filter{
		Page:     positiveInt(c.Query("page"), 1),
		PageSize: positiveInt(c.Query("pageSize"), 20),
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.UnreadOnly = strings.TrimSpace(c.Query("unreadOnly")) == "1" ||
		strings.EqualFold(strings.TrimSpace(c.Query("unreadOnly")), "true")
	page, err := h.repo.list(filter)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, page)
}

func (h *handler) create(c *gin.Context) {
	var payload Payload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid notification payload")
		return
	}
	payload.Title = truncate(payload.Title, 200)
	payload.Content = truncate(payload.Content, 1000)
	payload.Link = truncate(payload.Link, 500)
	if payload.Title == "" {
		httpx.Fail(c, http.StatusBadRequest, "notification title must not be empty")
		return
	}
	item, err := h.repo.create(payload)
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Success(c, item)
}

func (h *handler) markRead(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.repo.markRead(id)
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Success(c, item)
}

func (h *handler) markAllRead(c *gin.Context) {
	if err := h.repo.markAllRead(); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (h *handler) delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.delete(id); err != nil {
		respondError(c, err)
		return
	}
	httpx.Success(c, true)
}

func (h *handler) clearRead(c *gin.Context) {
	if err := h.repo.clearRead(); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, true)
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errNotificationNotFound):
		httpx.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, errInvalidType):
		httpx.Fail(c, http.StatusBadRequest, err.Error())
	default:
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
	}
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Fail(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
