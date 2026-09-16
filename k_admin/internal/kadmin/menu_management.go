package kadmin

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/bootstrap"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/modules/db/dialect"
	"github.com/gin-gonic/gin"
)

const (
	menuTypeDirectory int64 = iota
	menuTypeItem
	menuTypeExternal
)

type managedMenu struct {
	ID         int64         `json:"id"`
	ParentID   int64         `json:"parentId"`
	Type       int64         `json:"type"`
	Order      int64         `json:"order"`
	Title      string        `json:"title"`
	Icon       string        `json:"icon"`
	URI        string        `json:"uri"`
	Header     string        `json:"header"`
	PluginName string        `json:"pluginName"`
	UUID       string        `json:"uuid"`
	Component  string        `json:"component"`
	CreatedAt  string        `json:"createdAt"`
	UpdatedAt  string        `json:"updatedAt"`
	Children   []managedMenu `json:"children,omitempty"`
}

type menuPayload struct {
	ParentID   int64   `json:"parentId"`
	Type       *int64  `json:"type"`
	Order      int64   `json:"order"`
	Title      string  `json:"title"`
	Icon       string  `json:"icon"`
	URI        string  `json:"uri"`
	Header     string  `json:"header"`
	PluginName string  `json:"pluginName"`
	UUID       string  `json:"uuid"`
	Component  *string `json:"component"`
}

type menuPosition struct {
	ID       int64 `json:"id"`
	ParentID int64 `json:"parentId"`
	Order    int64 `json:"order"`
}

type menuLayoutPayload struct {
	Items []menuPosition `json:"items"`
}

func registerMenuManagementRoutes(api *gin.RouterGroup, s *Store) {
	menuGroup := api.Group("/admin-menus", s.requireAuth())
	menuGroup.GET("", s.requirePermission(menuManagePermission), s.adminMenus)
	menuGroup.GET("/tree", s.requirePermission(menuManagePermission), s.adminMenuTree)
	menuGroup.POST("", s.requirePermission(bootstrap.MenuCreatePermission), s.createAdminMenu)
	menuGroup.PUT("", s.requirePermission(bootstrap.MenuLayoutPermission), s.updateAdminMenuLayout)
	menuGroup.PUT("/:id", s.requirePermission(bootstrap.MenuUpdatePermission), s.updateAdminMenu)
	menuGroup.DELETE("/:id", s.requirePermission(bootstrap.MenuDeletePermission), s.deleteAdminMenu)
}

func (s *Store) adminMenus(c *gin.Context) {
	menus, err := s.loadManagedMenus()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, menus)
}

func (s *Store) adminMenuTree(c *gin.Context) {
	menus, err := s.loadManagedMenus()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, buildManagedMenuTree(menus, 0))
}

func (s *Store) createAdminMenu(c *gin.Context) {
	var req menuPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid menu payload")
		return
	}
	normalizeMenuPayload(&req)
	if req.Title == "" {
		fail(c, http.StatusBadRequest, "menu title is required")
		return
	}
	if err := validateMenuType(*req.Type); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateMenuURI(*req.Type, req.URI); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	component, err := resolveMenuComponent(*req.Type, req.URI, "", req.Component)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	s.menuMutationMu.Lock()
	defer s.menuMutationMu.Unlock()

	if validationMessage, err := s.validateMenuParent(req.ParentID); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if validationMessage != "" {
		fail(c, http.StatusBadRequest, validationMessage)
		return
	}

	id, err := db.WithDriver(s.conn).Table("goadmin_menu").Insert(dialect.H{
		"parent_id":   req.ParentID,
		"type":        *req.Type,
		"order":       req.Order,
		"title":       req.Title,
		"icon":        req.Icon,
		"uri":         req.URI,
		"header":      req.Header,
		"plugin_name": req.PluginName,
		"uuid":        req.UUID,
		"component":   component,
		"created_at":  nowString(),
		"updated_at":  nowString(),
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	menu, err := s.loadManagedMenu(id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, menu)
}

func (s *Store) updateAdminMenu(c *gin.Context) {
	menuID, ok := pathID(c)
	if !ok {
		return
	}

	var req menuPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid menu payload")
		return
	}
	normalizeMenuPayload(&req)
	if req.Title == "" {
		fail(c, http.StatusBadRequest, "menu title is required")
		return
	}
	if err := validateMenuType(*req.Type); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateMenuURI(*req.Type, req.URI); err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.ParentID == menuID {
		fail(c, http.StatusBadRequest, "parent menu cannot be itself")
		return
	}
	s.menuMutationMu.Lock()
	defer s.menuMutationMu.Unlock()

	existingMenu, err := s.loadManagedMenu(menuID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existingMenu.ID == 0 {
		fail(c, http.StatusNotFound, "menu does not exist")
		return
	}

	if *req.Type != menuTypeDirectory {
		hasChildren, err := s.menuHasChildren(menuID)
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if hasChildren {
			fail(c, http.StatusBadRequest, "menu with children must be a directory")
			return
		}
	}

	validationMessage, err := s.validateMenuParent(req.ParentID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if validationMessage != "" {
		fail(c, http.StatusBadRequest, validationMessage)
		return
	}
	if req.ParentID != 0 {
		descendant, err := s.isMenuDescendant(req.ParentID, menuID)
		if err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if descendant {
			fail(c, http.StatusBadRequest, "parent menu cannot be a descendant")
			return
		}
	}
	component, err := resolveMenuComponent(*req.Type, req.URI, existingMenu.Component, req.Component)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	_, err = db.WithDriver(s.conn).
		Table("goadmin_menu").
		Where("id", "=", menuID).
		Update(dialect.H{
			"parent_id":   req.ParentID,
			"type":        *req.Type,
			"order":       req.Order,
			"title":       req.Title,
			"icon":        req.Icon,
			"uri":         req.URI,
			"header":      req.Header,
			"plugin_name": req.PluginName,
			"uuid":        req.UUID,
			"component":   component,
			"updated_at":  nowString(),
		})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	menu, err := s.loadManagedMenu(menuID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, menu)
}

func (s *Store) updateAdminMenuLayout(c *gin.Context) {
	var req menuLayoutPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid menu layout payload")
		return
	}
	s.menuMutationMu.Lock()
	defer s.menuMutationMu.Unlock()

	var validationErr error
	_, err := db.WithDriver(s.conn).WithTransaction(func(tx *sql.Tx) (error, map[string]interface{}) {
		menus, loadErr := s.loadManagedMenusWithTx(tx)
		if loadErr != nil {
			return loadErr, nil
		}
		if validationErr = validateMenuPositions(menus, req.Items); validationErr != nil {
			return validationErr, nil
		}

		current := make(map[int64]managedMenu, len(menus))
		for _, menu := range menus {
			current[menu.ID] = menu
		}
		layoutChanged := false
		for _, item := range req.Items {
			menu := current[item.ID]
			if menu.ParentID != item.ParentID || menu.Order != item.Order {
				layoutChanged = true
				break
			}
		}
		if !layoutChanged {
			return nil, nil
		}

		// 布局是完整快照。有变化时覆盖所有行，并固定锁顺序，避免两个并发请求
		// 分别只写“变化行”后拼成一个从未通过校验的层级（例如 A<->B 环）。
		positions := append([]menuPosition(nil), req.Items...)
		sort.Slice(positions, func(i, j int) bool { return positions[i].ID < positions[j].ID })
		updatedAt := nowString()
		for _, item := range positions {
			_, updateErr := db.WithDriver(s.conn).
				WithTx(tx).
				Table("goadmin_menu").
				Where("id", "=", item.ID).
				Update(dialect.H{
					"parent_id":  item.ParentID,
					"order":      item.Order,
					"updated_at": updatedAt,
				})
			if updateErr != nil {
				return updateErr, nil
			}
		}
		return nil, nil
	})
	if err != nil {
		if validationErr != nil {
			fail(c, http.StatusBadRequest, validationErr.Error())
			return
		}
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	menus, err := s.loadManagedMenus()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, buildManagedMenuTree(menus, 0))
}

func (s *Store) deleteAdminMenu(c *gin.Context) {
	menuID, ok := pathID(c)
	if !ok {
		return
	}
	s.menuMutationMu.Lock()
	defer s.menuMutationMu.Unlock()

	hasChildren, err := s.menuHasChildren(menuID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if hasChildren {
		fail(c, http.StatusBadRequest, "menu has children")
		return
	}

	_ = db.WithDriver(s.conn).Table("goadmin_role_menu").Where("menu_id", "=", menuID).Delete()
	if err := db.WithDriver(s.conn).Table("goadmin_menu").Where("id", "=", menuID).Delete(); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

func (s *Store) loadManagedMenus() ([]managedMenu, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_menu").
		OrderBy("order", "asc").
		All()
	if err != nil {
		return nil, err
	}
	return managedMenusFromRows(rows), nil
}

func (s *Store) loadManagedMenusWithTx(tx *sql.Tx) ([]managedMenu, error) {
	rows, err := db.WithDriver(s.conn).
		WithTx(tx).
		Table("goadmin_menu").
		OrderBy("order", "asc").
		All()
	if err != nil {
		return nil, err
	}
	return managedMenusFromRows(rows), nil
}

func managedMenusFromRows(rows []map[string]interface{}) []managedMenu {
	menus := make([]managedMenu, 0, len(rows))
	for _, row := range rows {
		menus = append(menus, managedMenuFromRow(row))
	}
	sort.SliceStable(menus, func(i, j int) bool {
		if menus[i].Order == menus[j].Order {
			return menus[i].ID < menus[j].ID
		}
		return menus[i].Order < menus[j].Order
	})
	return menus
}

func (s *Store) loadManagedMenu(id int64) (managedMenu, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_menu").
		Where("id", "=", id).
		All()
	if err != nil {
		return managedMenu{}, err
	}
	if len(rows) == 0 {
		return managedMenu{}, nil
	}
	return managedMenuFromRow(rows[0]), nil
}

func managedMenuFromRow(row map[string]interface{}) managedMenu {
	return managedMenu{
		ID:         toInt64(row["id"]),
		ParentID:   toInt64(row["parent_id"]),
		Type:       toInt64(row["type"]),
		Order:      toInt64(row["order"]),
		Title:      toString(row["title"]),
		Icon:       toString(row["icon"]),
		URI:        toString(row["uri"]),
		Header:     toString(row["header"]),
		PluginName: toString(row["plugin_name"]),
		UUID:       toString(row["uuid"]),
		Component:  toString(row["component"]),
		CreatedAt:  toDateTimeString(row["created_at"]),
		UpdatedAt:  toDateTimeString(row["updated_at"]),
	}
}

func buildManagedMenuTree(items []managedMenu, parentID int64) []managedMenu {
	res := make([]managedMenu, 0)
	for _, item := range items {
		if item.ParentID != parentID {
			continue
		}
		item.Children = buildManagedMenuTree(items, item.ID)
		res = append(res, item)
	}
	return res
}

func normalizeMenuPayload(req *menuPayload) {
	req.Title = strings.TrimSpace(req.Title)
	req.Icon = strings.TrimSpace(req.Icon)
	req.URI = normalizeMenuURI(req.URI)
	req.Header = strings.TrimSpace(req.Header)
	req.PluginName = strings.TrimSpace(req.PluginName)
	req.UUID = strings.TrimSpace(req.UUID)
	if req.Component != nil {
		component := strings.TrimSpace(*req.Component)
		req.Component = &component
	}
	// type 未传时默认为菜单(1)；显式传 0 表示目录/分组，需原样保留。
	if req.Type == nil {
		t := menuTypeItem
		req.Type = &t
	}
}

func validateMenuType(menuType int64) error {
	if menuType != menuTypeDirectory && menuType != menuTypeItem && menuType != menuTypeExternal {
		return fmt.Errorf("menu type must be directory, menu, or external link")
	}
	return nil
}

func validateMenuURI(menuType int64, uri string) error {
	if menuType != menuTypeExternal {
		return nil
	}
	if !isExternalHTTPURL(uri) {
		return fmt.Errorf("external link must use http or https")
	}
	return nil
}

func validateMenuComponent(menuType int64, uri string, component string) error {
	if component == "" {
		return nil
	}
	if menuType != menuTypeItem {
		return fmt.Errorf("only menu items may define a component")
	}
	normalizedURI := normalizeMenuURI(uri)
	if normalizedURI == "" || normalizedURI == "#" || isExternalHTTPURL(normalizedURI) {
		return fmt.Errorf("menu component requires an internal menu URI")
	}
	if len(component) > 255 {
		return fmt.Errorf("menu component must not exceed 255 characters")
	}
	if !strings.HasPrefix(component, "/") {
		return fmt.Errorf("menu component must start with /")
	}
	if strings.HasSuffix(component, "/") || strings.Contains(component, "//") ||
		strings.Contains(component, "..") || strings.ContainsAny(component, `\\?#`) {
		return fmt.Errorf("menu component contains an invalid path segment")
	}
	for _, char := range component {
		if char == '/' || char == '-' || char == '_' ||
			(char >= '0' && char <= '9') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= 'a' && char <= 'z') {
			continue
		}
		return fmt.Errorf("menu component contains an invalid character")
	}
	return nil
}

func resolveMenuComponent(menuType int64, uri string, existing string, requested *string) (string, error) {
	if requested != nil {
		if err := validateMenuComponent(menuType, uri, *requested); err != nil {
			return "", err
		}
	}
	if menuType != menuTypeItem {
		return "", nil
	}
	component := existing
	if requested != nil {
		component = *requested
	}
	if err := validateMenuComponent(menuType, uri, component); err != nil {
		return "", err
	}
	return component, nil
}

func isExternalHTTPURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}

func validateMenuPositions(menus []managedMenu, positions []menuPosition) error {
	if len(positions) != len(menus) {
		return fmt.Errorf("menu layout payload must contain all menus")
	}

	menuByID := make(map[int64]managedMenu, len(menus))
	for _, menu := range menus {
		if err := validateMenuType(menu.Type); err != nil {
			return fmt.Errorf("menu %d has an invalid type", menu.ID)
		}
		menuByID[menu.ID] = menu
	}
	parentByID := make(map[int64]int64, len(positions))
	siblingOrders := make(map[string]struct{}, len(positions))
	ordersByParent := make(map[int64][]int64)
	for _, position := range positions {
		if _, exists := menuByID[position.ID]; !exists {
			return fmt.Errorf("menu %d does not exist", position.ID)
		}
		if _, duplicate := parentByID[position.ID]; duplicate {
			return fmt.Errorf("menu %d is duplicated", position.ID)
		}
		if position.ParentID == position.ID {
			return fmt.Errorf("menu %d cannot be its own parent", position.ID)
		}
		if position.ParentID != 0 {
			parent, exists := menuByID[position.ParentID]
			if !exists {
				return fmt.Errorf("parent menu %d does not exist", position.ParentID)
			}
			if parent.Type != menuTypeDirectory {
				return fmt.Errorf("parent menu %d must be a directory", position.ParentID)
			}
		}
		if position.Order < 0 {
			return fmt.Errorf("menu %d has an invalid order", position.ID)
		}

		orderKey := fmt.Sprintf("%d:%d", position.ParentID, position.Order)
		if _, duplicate := siblingOrders[orderKey]; duplicate {
			return fmt.Errorf("menus under parent %d have duplicate orders", position.ParentID)
		}
		siblingOrders[orderKey] = struct{}{}
		ordersByParent[position.ParentID] = append(
			ordersByParent[position.ParentID],
			position.Order,
		)
		parentByID[position.ID] = position.ParentID
	}
	for parentID, orders := range ordersByParent {
		sort.Slice(orders, func(i, j int) bool { return orders[i] < orders[j] })
		for index, order := range orders {
			if order != int64(index) {
				return fmt.Errorf("menus under parent %d must have continuous orders", parentID)
			}
		}
	}

	for menuID := range menuByID {
		if _, exists := parentByID[menuID]; !exists {
			return fmt.Errorf("menu %d is missing", menuID)
		}
		visited := make(map[int64]struct{})
		for current := menuID; current != 0; current = parentByID[current] {
			if _, cycle := visited[current]; cycle {
				return fmt.Errorf("menu hierarchy contains a cycle")
			}
			visited[current] = struct{}{}
		}
	}
	return nil
}

func (s *Store) validateMenuParent(parentID int64) (string, error) {
	if parentID == 0 {
		return "", nil
	}
	parent, err := s.loadManagedMenu(parentID)
	if err != nil {
		return "", err
	}
	if parent.ID == 0 {
		return "parent menu does not exist", nil
	}
	if parent.Type != menuTypeDirectory {
		return "parent menu must be a directory", nil
	}
	return "", nil
}

func (s *Store) menuHasChildren(id int64) (bool, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_menu").Where("parent_id", "=", id).All()
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func (s *Store) isMenuDescendant(candidateID, parentID int64) (bool, error) {
	menus, err := s.loadManagedMenus()
	if err != nil {
		return false, err
	}
	parentByID := make(map[int64]int64, len(menus))
	for _, menu := range menus {
		parentByID[menu.ID] = menu.ParentID
	}
	for current := candidateID; current != 0; current = parentByID[current] {
		if current == parentID {
			return true, nil
		}
		next, ok := parentByID[current]
		if !ok || next == current {
			break
		}
	}
	return false, nil
}
