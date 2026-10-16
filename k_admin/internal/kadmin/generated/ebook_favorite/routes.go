// Maintained manually for the (ebook_id, user_id) composite primary key.
// Do not regenerate with codegen until it supports composite-key CRUD.

package ebook_favorite

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/transport/httpx"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

// Dependencies wires the generated module into the KAdmin API group.
type Dependencies struct {
	Connection              db.Connection
	RequireAuth             gin.HandlerFunc
	RequirePermission       func(...string) gin.HandlerFunc
	RegisterAuditResource   func(prefix, resource string, loader func(string) interface{})
	RegisterIdempotentRoute func(prefix string)
}

// Register installs the CRUD routes for table ptmj_ebook_favorite and keeps the
// module permission and menu rows in sync.
func Register(api *gin.RouterGroup, deps Dependencies) error {
	repo := newRepository(deps.Connection)
	if err := ensurePermissions(deps.Connection); err != nil {
		return fmt.Errorf("sync ebook_favorite permissions: %w", err)
	}
	if err := ensureMenu(deps.Connection); err != nil {
		return fmt.Errorf("sync ebook_favorite menu: %w", err)
	}
	deps.RegisterAuditResource("ebook-favorites", "ebook_favorite", func(resourceID string) interface{} {
		id, err := parseFavoriteKey(resourceID)
		if err != nil {
			return nil
		}
		item, found, err := repo.findByID(id)
		if err != nil || !found {
			return nil
		}
		return item
	})
	deps.RegisterIdempotentRoute("ebook-favorites")
	h := &handler{repo: repo}
	group := api.Group("/ebook-favorites", deps.RequireAuth)
	group.GET("", deps.RequirePermission(ListPermission), h.list)
	group.GET("/:id", deps.RequirePermission(ListPermission), h.get)
	group.POST("", deps.RequirePermission(CreatePermission), h.create)
	group.PUT("/:id", deps.RequirePermission(UpdatePermission), h.update)
	group.DELETE("/:id", deps.RequirePermission(DeletePermission), h.delete)
	return nil
}

type handler struct {
	repo *repository
}

// swaggerListEbookFavorite documents GET /ebook-favorites.
// @Summary 查询电子书收藏列表
// @Tags 生成业务
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键词"
// @Success 200 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Router /ebook-favorites [get]
func (h *handler) list(c *gin.Context) {
	filter := EbookFavoriteFilter{
		Page:     positiveInt(c.Query("page"), 1),
		PageSize: positiveInt(c.Query("pageSize"), 20),
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	page, err := h.repo.list(filter)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Success(c, page)
}

// swaggerGetEbookFavorite documents GET /ebook-favorites/{id}.
// @Summary 获取电子书收藏详情
// @Tags 生成业务
// @Security BearerAuth
// @Param id path string true "复合收藏 ID，resourceId,userId"
// @Success 200 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Failure 404 {object} object
// @Router /ebook-favorites/{id} [get]
func (h *handler) get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, found, err := h.repo.findByID(id)
	if err != nil {
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		httpx.Fail(c, http.StatusNotFound, errEbookFavoriteNotFound.Error())
		return
	}
	httpx.Success(c, item)
}

// swaggerCreateEbookFavorite documents POST /ebook-favorites.
// @Summary 新增电子书收藏
// @Tags 生成业务
// @Security BearerAuth
// @Param payload body EbookFavoritePayload true "电子书收藏信息"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Failure 409 {object} object
// @Router /ebook-favorites [post]
func (h *handler) create(c *gin.Context) {
	var payload EbookFavoritePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid ebook_favorite payload")
		return
	}
	if message := validateEbookFavoritePayload(payload); message != "" {
		httpx.Fail(c, http.StatusBadRequest, message)
		return
	}
	item, err := h.repo.create(payload)
	if err != nil {
		respondEbookFavoriteError(c, err)
		return
	}
	httpx.Success(c, item)
}

// swaggerUpdateEbookFavorite documents PUT /ebook-favorites/{id}.
// @Summary 修改电子书收藏
// @Tags 生成业务
// @Security BearerAuth
// @Param id path string true "复合收藏 ID，resourceId,userId"
// @Param payload body EbookFavoritePayload true "电子书收藏信息"
// @Success 200 {object} object
// @Failure 400 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Failure 404 {object} object
// @Router /ebook-favorites/{id} [put]
func (h *handler) update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var payload EbookFavoritePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Fail(c, http.StatusBadRequest, "invalid ebook_favorite payload")
		return
	}
	if message := validateEbookFavoritePayload(payload); message != "" {
		httpx.Fail(c, http.StatusBadRequest, message)
		return
	}
	if payload.EbookId != id.resourceID {
		httpx.Fail(c, http.StatusBadRequest, "ebookId cannot be changed")
		return
	}
	item, err := h.repo.update(id, payload)
	if err != nil {
		respondEbookFavoriteError(c, err)
		return
	}
	httpx.Success(c, item)
}

// swaggerDeleteEbookFavorite documents DELETE /ebook-favorites/{id}.
// @Summary 删除电子书收藏
// @Tags 生成业务
// @Security BearerAuth
// @Param id path string true "复合收藏 ID，resourceId,userId"
// @Success 200 {object} object
// @Failure 401 {object} object
// @Failure 403 {object} object
// @Failure 404 {object} object
// @Router /ebook-favorites/{id} [delete]
func (h *handler) delete(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.repo.delete(id); err != nil {
		respondEbookFavoriteError(c, err)
		return
	}
	httpx.Success(c, true)
}

func validateEbookFavoritePayload(payload EbookFavoritePayload) string {
	if payload.EbookId <= 0 || payload.UserId <= 0 {
		return "ebookId and userId must be positive integers"
	}
	return ""
}

func respondEbookFavoriteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errEbookFavoriteNotFound):
		httpx.Fail(c, http.StatusNotFound, err.Error())
	case strings.Contains(err.Error(), "duplicate key") || strings.Contains(strings.ToLower(err.Error()), "unique constraint"):
		httpx.Fail(c, http.StatusConflict, "记录已存在")
	default:
		httpx.Fail(c, http.StatusInternalServerError, err.Error())
	}
}

// ensurePermissions inserts the four CRUD permission rows when missing.
func ensurePermissions(conn db.Connection) error {
	seeds := []struct{ name, slug, method, path string }{
		{"电子书收藏查看", ListPermission, "GET", "/api/ebook-favorites*"},
		{"电子书收藏新增", CreatePermission, "POST", "/api/ebook-favorites*"},
		{"电子书收藏修改", UpdatePermission, "PUT", "/api/ebook-favorites*"},
		{"电子书收藏删除", DeletePermission, "DELETE", "/api/ebook-favorites*"},
	}
	for _, seed := range seeds {
		rows, err := conn.Query(`SELECT id FROM goadmin_permissions WHERE slug = ?`, seed.slug)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			continue
		}
		if _, err := conn.Exec(`INSERT INTO goadmin_permissions (name, slug, http_method, http_path, created_at, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			seed.name, seed.slug, seed.method, seed.path); err != nil {
			return err
		}
	}
	return nil
}

// ensureMenu inserts the module menu under the shared /business directory
// and grants it (together with the parent directory) to the Administrator
// role, so a freshly generated page is visible without manual authorization.
// 菜单可见即接口可用：授权菜单即同时放开对应接口权限（见 menu_permission.go）。
func ensureMenu(conn db.Connection) error {
	parentRows, err := conn.Query(`SELECT id FROM goadmin_menu WHERE uri = ?`, "/business")
	if err != nil || len(parentRows) == 0 {
		if err == nil {
			err = fmt.Errorf("parent menu /business is missing")
		}
		return err
	}
	parentID := toInt64(parentRows[0]["id"])
	rows, err := conn.Query(`SELECT id FROM goadmin_menu WHERE uri = ?`, "/ebook-favorites")
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		if _, err := conn.Exec(`INSERT INTO goadmin_menu
			(parent_id, type, "order", title, icon, uri, plugin_name, component, created_at, updated_at)
			VALUES (?, 1, 99, ?, 'lucide:package-open', ?, '', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			parentID, "电子书收藏", "/ebook-favorites", "/kadmin/generated/ebook_favorite/EbookFavoriteListView"); err != nil {
			return err
		}
		rows, err = conn.Query(`SELECT id FROM goadmin_menu WHERE uri = ?`, "/ebook-favorites")
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("menu /ebook-favorites insert did not persist")
		}
	}
	menuID := toInt64(rows[0]["id"])
	for _, grantMenuID := range []int64{parentID, menuID} {
		if _, err := conn.Exec(`INSERT INTO goadmin_role_menu (role_id, menu_id, created_at, updated_at)
			SELECT r.id, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP FROM goadmin_roles r
			WHERE r.slug = 'administrator'
			  AND NOT EXISTS (
				SELECT 1 FROM goadmin_role_menu rm
				WHERE rm.role_id = r.id AND rm.menu_id = ?
			)`, grantMenuID, grantMenuID); err != nil {
			return err
		}
	}
	return nil
}

func pathID(c *gin.Context) (favoriteKey, bool) {
	id, err := parseFavoriteKey(c.Param("id"))
	if err != nil {
		httpx.Fail(c, http.StatusBadRequest, "id must contain resourceId,userId")
		return favoriteKey{}, false
	}
	return id, true
}

func parseFavoriteKey(value string) (favoriteKey, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return favoriteKey{}, fmt.Errorf("invalid composite favorite id")
	}
	resourceID, resourceErr := strconv.ParseInt(parts[0], 10, 64)
	userID, userErr := strconv.ParseInt(parts[1], 10, 64)
	if resourceErr != nil || userErr != nil || resourceID <= 0 || userID <= 0 {
		return favoriteKey{}, fmt.Errorf("invalid composite favorite id")
	}
	return favoriteKey{resourceID, userID}, nil
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
