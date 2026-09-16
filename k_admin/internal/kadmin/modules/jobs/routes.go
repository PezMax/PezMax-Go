package jobs

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Connection        db.Connection
	RequireAuth       gin.HandlerFunc
	RequirePermission func(...string) gin.HandlerFunc
	RefreshCache      func() error
}

type handler struct {
	manager *Manager
}

func Register(api *gin.RouterGroup, dependencies Dependencies) (*Manager, error) {
	if err := EnsureSchema(dependencies.Connection); err != nil {
		return nil, err
	}
	manager, err := NewManager(ManagerOptions{
		Connection:   dependencies.Connection,
		RefreshCache: dependencies.RefreshCache,
	})
	if err != nil {
		return nil, err
	}
	RegisterRoutes(api, manager, dependencies)
	return manager, nil
}

func RegisterRoutes(api *gin.RouterGroup, manager *Manager, dependencies Dependencies) {
	h := &handler{manager: manager}
	jobs := api.Group("/jobs", dependencies.RequireAuth)
	jobs.GET("", dependencies.RequirePermission(ListPermission), h.listJobs)
	jobs.GET("/:id", dependencies.RequirePermission(ListPermission), h.getJob)
	jobs.POST("", dependencies.RequirePermission(CreatePermission), h.createJob)
	jobs.PUT("/:id", dependencies.RequirePermission(UpdatePermission), h.updateJob)
	jobs.PATCH("/:id/status", dependencies.RequirePermission(UpdatePermission), h.setStatus)
	jobs.DELETE("/:id", dependencies.RequirePermission(DeletePermission), h.deleteJob)
	jobs.POST("/:id/run", dependencies.RequirePermission(RunPermission), h.runJob)

	logs := api.Group("/job-logs", dependencies.RequireAuth, dependencies.RequirePermission(LogListPermission))
	logs.GET("", h.listExecutions)
	logs.GET("/:id", h.getExecution)
}

func (h *handler) listJobs(c *gin.Context) {
	filter := JobFilter{
		Page: positiveInt(c.Query("page"), 1), PageSize: positiveInt(c.Query("pageSize"), 20),
		Keyword: truncate(c.Query("keyword"), 100), Handler: strings.TrimSpace(c.Query("handler")), Status: strings.TrimSpace(c.Query("status")),
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	if filter.Status != "" && filter.Status != statusEnabled && filter.Status != statusPaused {
		httpx.Fail(c, http.StatusBadRequest, "invalid task status")
		return
	}
	page, err := h.manager.ListJobs(filter)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, page)
}

func (h *handler) getJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	job, found, err := h.manager.GetJob(id)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		httpx.Fail(c, http.StatusNotFound, "task not found")
		return
	}
	httpx.Success(c, job)
}

func (h *handler) createJob(c *gin.Context) {
	var payload JobPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid task payload")
		return
	}
	job, err := h.manager.CreateJob(payload, currentUserID(c))
	if err != nil {
		respondManagerError(c, err)
		return
	}
	httpx.Success(c, job)
}

func (h *handler) updateJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var payload JobPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid task payload")
		return
	}
	job, err := h.manager.UpdateJob(id, payload)
	if err != nil {
		respondManagerError(c, err)
		return
	}
	httpx.Success(c, job)
}

func (h *handler) setStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid task status")
		return
	}
	job, err := h.manager.SetStatus(id, strings.TrimSpace(payload.Status))
	if err != nil {
		respondManagerError(c, err)
		return
	}
	httpx.Success(c, job)
}

func (h *handler) deleteJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.manager.DeleteJob(id); err != nil {
		respondManagerError(c, err)
		return
	}
	httpx.Success(c, true)
}

func (h *handler) runJob(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	execution, err := h.manager.RunNow(id, currentUserID(c))
	if err != nil {
		if execution.ID > 0 {
			httpx.Success(c, execution)
			return
		}
		respondManagerError(c, err)
		return
	}
	httpx.Success(c, execution)
}

func (h *handler) listExecutions(c *gin.Context) {
	filter := ExecutionFilter{
		Page: positiveInt(c.Query("page"), 1), PageSize: positiveInt(c.Query("pageSize"), 20),
		Keyword: truncate(c.Query("keyword"), 100), Status: strings.TrimSpace(c.Query("status")), Trigger: strings.TrimSpace(c.Query("trigger")),
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	if value := strings.TrimSpace(c.Query("jobId")); value != "" {
		filter.JobID, _ = strconv.ParseInt(value, 10, 64)
		if filter.JobID <= 0 {
			httpx.Fail(c, http.StatusBadRequest, "invalid task id")
			return
		}
	}
	if filter.Status != "" && filter.Status != executionRunning && filter.Status != executionSuccess && filter.Status != executionFailed {
		httpx.Fail(c, http.StatusBadRequest, "invalid execution status")
		return
	}
	if filter.Trigger != "" && filter.Trigger != triggerManual && filter.Trigger != triggerScheduled {
		httpx.Fail(c, http.StatusBadRequest, "invalid execution trigger")
		return
	}
	page, err := h.manager.ListExecutions(filter)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, page)
}

func (h *handler) getExecution(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	execution, found, err := h.manager.GetExecution(id)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		httpx.Fail(c, http.StatusNotFound, "execution log not found")
		return
	}
	httpx.Success(c, execution)
}

func respondManagerError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrJobNotFound):
		httpx.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrAlreadyRunning):
		httpx.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrBuiltInTask), errors.Is(err, ErrNameExists):
		httpx.Fail(c, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "required"), strings.Contains(err.Error(), "invalid"),
		strings.Contains(err.Error(), "unknown"), strings.Contains(err.Error(), "must be"), strings.Contains(err.Error(), "too long"):
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

func currentUserID(c *gin.Context) int64 {
	if value, ok := c.Get("vben_user"); ok {
		if user, valid := value.(models.UserModel); valid {
			return user.Id
		}
	}
	if value, ok := c.Get("vben_user_id"); ok {
		if id, valid := value.(int64); valid {
			return id
		}
	}
	return 0
}
