package codegen

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/gin-gonic/gin"
)

// Register installs the codegen schema and management routes.
func Register(api *gin.RouterGroup, deps Dependencies) error {
	if err := EnsureSchema(deps.Connection); err != nil {
		return err
	}
	RegisterRoutes(api, deps)
	return nil
}

func RegisterRoutes(api *gin.RouterGroup, deps Dependencies) {
	h := &handler{
		introspector: newIntrospector(deps.Connection),
		repository:   newRepository(deps.Connection),
		writer:       NewWriter(deps.RepoRoot),
	}
	group := api.Group("/codegen", deps.RequireAuth)
	group.GET("/candidates", deps.RequirePermission(ListPermission), h.listCandidates)
	group.GET("/tables", deps.RequirePermission(ListPermission), h.listConfigs)
	group.POST("/tables/import", deps.RequirePermission(ImportPermission), h.importTable)
	// The :id subtree lives under /configs so gin v1.3 never has to mix static
	// and param children under one node.
	group.GET("/configs/:id", deps.RequirePermission(ListPermission), h.getConfig)
	group.PUT("/configs/:id", deps.RequirePermission(ImportPermission), h.updateConfig)
	group.DELETE("/configs/:id", deps.RequirePermission(ImportPermission), h.deleteConfig)
	group.POST("/configs/:id/preview", deps.RequirePermission(GeneratePermission), h.preview)
	group.POST("/configs/:id/generate", deps.RequirePermission(GeneratePermission), h.generate)
	group.GET("/configs/:id/download", deps.RequirePermission(GeneratePermission), h.download)
}

type handler struct {
	introspector *introspector
	repository   *repository
	writer       *Writer
}

func (h *handler) listCandidates(c *gin.Context) {
	candidates, err := h.introspector.listCandidateTables(truncateText(c.Query("keyword"), 100))
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, candidates)
}

func (h *handler) listConfigs(c *gin.Context) {
	configs, err := h.repository.listConfigs(c.Query("keyword"))
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, configs)
}

func (h *handler) importTable(c *gin.Context) {
	var payload ImportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid import payload")
		return
	}
	payload.TableName = strings.TrimSpace(payload.TableName)
	if !h.introspector.validTableName(payload.TableName) || isSystemTable(payload.TableName) {
		httpx.Fail(c, http.StatusBadRequest, "invalid table name")
		return
	}
	if existing, found, err := h.repository.findConfigByTable(payload.TableName); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if found {
		httpx.Success(c, existing)
		return
	}
	columns, err := h.introspector.describeColumns(payload.TableName)
	if err != nil {
		if err == errTableNotFound {
			httpx.Fail(c, http.StatusNotFound, "table not found")
			return
		}
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	module, className, routePrefix, business := deriveImportNames(payload.TableName)
	if strings.TrimSpace(payload.ModuleName) != "" {
		module = strings.TrimSpace(payload.ModuleName)
	}
	if strings.TrimSpace(payload.ClassName) != "" {
		className = strings.TrimSpace(payload.ClassName)
	}
	if strings.TrimSpace(payload.RoutePrefix) != "" {
		routePrefix = strings.TrimSpace(payload.RoutePrefix)
	}
	if strings.TrimSpace(payload.BusinessName) != "" {
		business = strings.TrimSpace(payload.BusinessName)
	}
	config := TableConfig{
		TableName:    payload.TableName,
		ModuleName:   module,
		ClassName:    className,
		BusinessName: business,
		RoutePrefix:  routePrefix,
		Columns:      inferColumnConfigs(columns),
	}
	if err := validateConfig(config); err != nil {
		httpx.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.ensureUniqueModule(config.ModuleName, 0); err != nil {
		httpx.Fail(c, http.StatusConflict, err.Error())
		return
	}
	created, err := h.repository.createConfig(config)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, created)
}

func (h *handler) getConfig(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	config, found, err := h.repository.getConfig(id)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		httpx.Fail(c, http.StatusNotFound, "codegen table config not found")
		return
	}
	httpx.Success(c, config)
}

func (h *handler) updateConfig(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	config, found, err := h.repository.getConfig(id)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		httpx.Fail(c, http.StatusNotFound, "codegen table config not found")
		return
	}
	var payload UpdatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid update payload")
		return
	}
	config.ModuleName = strings.TrimSpace(payload.ModuleName)
	config.ClassName = strings.TrimSpace(payload.ClassName)
	config.BusinessName = strings.TrimSpace(payload.BusinessName)
	config.RoutePrefix = strings.TrimSpace(payload.RoutePrefix)
	if len(payload.Columns) > 0 {
		config.Columns = payload.Columns
	}
	if err := validateConfig(config); err != nil {
		httpx.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.ensureUniqueModule(config.ModuleName, id); err != nil {
		httpx.Fail(c, http.StatusConflict, err.Error())
		return
	}
	updated, err := h.repository.updateConfig(id, config)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, updated)
}

func (h *handler) deleteConfig(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, found, err := h.repository.getConfig(id); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if !found {
		httpx.Fail(c, http.StatusNotFound, "codegen table config not found")
		return
	}
	if err := h.repository.deleteConfig(id); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, true)
}

func (h *handler) preview(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	config, found, err := h.repository.getConfig(id)
	if err != nil || !found {
		h.configError(c, err, found)
		return
	}
	artifacts, err := h.renderWithRegistry(config)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.Success(c, PreviewResponse{Artifacts: artifacts})
}

func (h *handler) generate(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	config, found, err := h.repository.getConfig(id)
	if err != nil || !found {
		h.configError(c, err, found)
		return
	}
	var payload GeneratePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid generate payload")
		return
	}
	artifacts, err := h.renderWithRegistry(config)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	plan, err := h.writer.plan(artifacts)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if len(plan.conflicts) > 0 && !payload.ConfirmOverwrite {
		c.JSON(http.StatusConflict, gin.H{
			"code":    http.StatusConflict,
			"message": "target files exist without the codegen marker; confirm overwrite to proceed",
			"msg":     "target files exist without the codegen marker; confirm overwrite to proceed",
			"data": GenerateResponse{
				Conflicts:       plan.conflicts,
				PermissionCount: 4,
				MenuURI:         "/" + config.RoutePrefix,
			},
		})
		return
	}
	// Confirmed overwrites are treated exactly like generated-file updates.
	plan.toOverwrite = append(plan.toOverwrite, plan.conflictArtifacts...)
	if err := h.writer.apply(plan); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.repository.markGenerated(config.ID); err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response := GenerateResponse{
		Written:         writtenPaths(plan),
		Overwritten:     overwrittenPaths(plan),
		Conflicts:       plan.conflicts,
		PermissionCount: 4,
		MenuURI:         "/" + config.RoutePrefix,
		Note:            "文件已写入项目；重启后端服务后路由、权限与菜单生效。",
	}
	httpx.Success(c, response)
}

func (h *handler) download(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	config, found, err := h.repository.getConfig(id)
	if err != nil || !found {
		h.configError(c, err, found)
		return
	}
	artifacts, err := h.renderWithRegistry(config)
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	archive, err := buildZip(artifacts)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	filename := config.ModuleName + "-codegen.zip"
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/zip", archive)
}

// renderWithRegistry renders the module artifacts plus the shared registry
// file that wires every generated module into the KAdmin API group.
func (h *handler) renderWithRegistry(config TableConfig) ([]Artifact, error) {
	existing, err := h.repository.generatedModules()
	if err != nil {
		return nil, err
	}
	modules := registryModulesFor(existing, config)
	return renderArtifacts(config, modules)
}

func (h *handler) configError(c *gin.Context, err error, found bool) {
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Fail(c, http.StatusNotFound, "codegen table config not found")
}

// ensureUniqueModule rejects a module name already used by another config.
func (h *handler) ensureUniqueModule(module string, exceptID int64) error {
	configs, err := h.repository.listConfigs("")
	if err != nil {
		return err
	}
	for _, config := range configs {
		if config.ID != exceptID && config.ModuleName == module {
			return errModuleNameTaken(module)
		}
	}
	return nil
}

func errModuleNameTaken(module string) error {
	return &moduleNameTakenError{module: module}
}

type moduleNameTakenError struct {
	module string
}

func (e *moduleNameTakenError) Error() string {
	return "module name " + e.module + " is already used by another table"
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Fail(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

func writtenPaths(plan writePlan) []string {
	paths := make([]string, 0, len(plan.toCreate))
	for _, artifact := range plan.toCreate {
		paths = append(paths, artifact.Path)
	}
	return paths
}

func overwrittenPaths(plan writePlan) []string {
	paths := make([]string, 0, len(plan.toOverwrite))
	for _, artifact := range plan.toOverwrite {
		paths = append(paths, artifact.Path)
	}
	return paths
}
