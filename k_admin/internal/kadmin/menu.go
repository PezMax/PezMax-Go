package kadmin

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

type menuItem struct {
	ID        int64
	ParentID  int64
	Type      int64
	Order     int64
	Title     string
	Icon      string
	URI       string
	Component string
}

type vbenMenu struct {
	ID        int64                  `json:"id,omitempty"`
	ParentID  int64                  `json:"parentId,omitempty"`
	Name      string                 `json:"name"`
	Path      string                 `json:"path"`
	Component string                 `json:"component,omitempty"`
	Redirect  string                 `json:"redirect,omitempty"`
	Meta      map[string]interface{} `json:"meta"`
	Children  []vbenMenu             `json:"children,omitempty"`
}

type menuRouteBinding struct {
	Name      string
	Path      string
	Component string
	Icon      string
	Meta      map[string]interface{}
}

var vbenMenuRouteBindings = map[string]menuRouteBinding{
	"/": {
		Name: "Dashboard",
		Path: "/dashboard",
		Icon: "lucide:layout-dashboard",
	},
	"/dashboard": {
		Name: "Dashboard",
		Path: "/dashboard",
		Icon: "lucide:layout-dashboard",
	},
	"/dashboard/analytics": {
		Name:      "Analytics",
		Path:      "/dashboard/analytics",
		Component: "/dashboard/analytics/index",
		Icon:      "lucide:area-chart",
		Meta: map[string]interface{}{
			"affixTab": true,
		},
	},
	"/dashboard/workspace": {
		Name:      "Workspace",
		Path:      "/dashboard/workspace",
		Component: "/dashboard/workspace/index",
		Icon:      "carbon:workspace",
	},
	"/kadmin": {
		Name: "KAdmin",
		Path: "/kadmin",
		Icon: "lucide:settings-2",
	},
	"/kadmin/users": {
		Name:      "KAdminUsers",
		Path:      "/kadmin/users",
		Component: "/kadmin/components/UserManagementView",
		Icon:      "lucide:users",
	},
	"/kadmin/rbac": {
		Name:      "KAdminRbac",
		Path:      "/kadmin/rbac",
		Component: "/kadmin/components/RbacWorkbench",
		Icon:      "lucide:shield-check",
	},
	"/kadmin/menus": {
		Name:      "KAdminMenus",
		Path:      "/kadmin/menus",
		Component: "/kadmin/components/MenuManagementView",
		Icon:      "lucide:menu",
	},
	"/kadmin/dictionary": {
		Name:      "KAdminDictionary",
		Path:      "/kadmin/dictionary",
		Component: "/kadmin/components/DictionaryManagementView",
		Icon:      "lucide:book-open",
	},
	"/kadmin/settings": {
		Name:      "KAdminSettings",
		Path:      "/kadmin/settings",
		Component: "/kadmin/components/SettingsView",
		Icon:      "lucide:sliders-horizontal",
	},
	"/kadmin/resources": {
		Name:      "KAdminResources",
		Path:      "/kadmin/resources",
		Component: "/kadmin/components/ResourceWorkbench",
		Icon:      "lucide:folder-kanban",
	},
	"/kadmin/logs": {
		Name:      "KAdminLogs",
		Path:      "/kadmin/logs",
		Component: "/kadmin/components/LogManagementView",
		Icon:      "lucide:scroll-text",
	},
	"/kadmin/login-audits": {
		Name:      "KAdminLoginAudits",
		Path:      "/kadmin/login-audits",
		Component: "/kadmin/components/LoginAuditManagementView",
		Icon:      "lucide:shield-check",
	},
	"/kadmin/jobs": {
		Name:      "KAdminJobs",
		Path:      "/kadmin/jobs",
		Component: "/kadmin/components/JobManagementView",
		Icon:      "lucide:clock-3",
	},
	"/kadmin/monitor": {
		Name:      "KAdminMonitor",
		Path:      "/kadmin/monitor",
		Component: "/kadmin/components/SystemMonitorView",
		Icon:      "lucide:monitor-cog",
	},
	"/kadmin/load-ranking": {
		Name:      "KAdminLoadRanking",
		Path:      "/kadmin/load-ranking",
		Component: "/kadmin/components/InterfaceLoadRankingView",
		Icon:      "lucide:bar-chart-3",
	},
	"/kadmin/codegen": {
		Name:      "KAdminCodegen",
		Path:      "/kadmin/codegen",
		Component: "/kadmin/components/CodegenManagementView",
		Icon:      "lucide:wand-2",
	},
}

func registerMenuRoutes(api *gin.RouterGroup, s *Store) {
	menuGroup := api.Group("/menu", s.requireAuth())
	menuGroup.GET("/all", s.menus)
	menuGroup.GET("/list", s.menus)
}

func (s *Store) menus(c *gin.Context) {
	user, ok := s.currentUser(c)
	if !ok {
		fail(c, http.StatusUnauthorized, "invalid token")
		return
	}

	rows, err := db.WithDriver(s.conn).
		Table("goadmin_menu").
		OrderBy("order", "asc").
		All()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]menuItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, menuItem{
			ID:        toInt64(row["id"]),
			ParentID:  toInt64(row["parent_id"]),
			Type:      toInt64(row["type"]),
			Order:     toInt64(row["order"]),
			Title:     toString(row["title"]),
			Icon:      toString(row["icon"]),
			URI:       toString(row["uri"]),
			Component: toString(row["component"]),
		})
	}
	sortMenuItems(items)

	allowed := make(map[int64]bool)
	if user.IsSuperAdmin() || isAdminUser(user.UserName) {
		for _, item := range items {
			allowed[item.ID] = true
		}
	} else {
		directlyAllowed, loadErr := s.loadAllowedMenuIDsForRoles(user.GetAllRoleId())
		if loadErr != nil {
			fail(c, http.StatusInternalServerError, loadErr.Error())
			return
		}
		allowed = expandAllowedMenuAncestors(items, directlyAllowed)
	}

	visibleItems := make([]menuItem, 0, len(items))
	for _, item := range items {
		if !allowed[item.ID] {
			continue
		}
		visibleItems = append(visibleItems, item)
	}

	menus := buildMenuTree(visibleItems, 0)
	values, err := s.readSystemConfig()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	applyExternalLinkTarget(menus, values["navigation.external_link_target"])
	success(c, menus)
}

func (s *Store) loadAllowedMenuIDsForRoles(roleIDs []interface{}) (map[int64]bool, error) {
	allowed := make(map[int64]bool)
	if len(roleIDs) == 0 {
		return allowed, nil
	}
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_role_menu").
		WhereIn("role_id", roleIDs).
		Select("menu_id").
		All()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		allowed[toInt64(row["menu_id"])] = true
	}
	return allowed, nil
}

func expandAllowedMenuAncestors(items []menuItem, directlyAllowed map[int64]bool) map[int64]bool {
	parentByID := make(map[int64]int64, len(items))
	for _, item := range items {
		parentByID[item.ID] = item.ParentID
	}

	allowed := make(map[int64]bool, len(directlyAllowed))
	for menuID := range directlyAllowed {
		visited := make(map[int64]bool)
		for current := menuID; current != 0 && !visited[current]; current = parentByID[current] {
			visited[current] = true
			if _, exists := parentByID[current]; !exists {
				break
			}
			allowed[current] = true
		}
	}
	return allowed
}

func sortMenuItems(items []menuItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ParentID != items[j].ParentID {
			return items[i].ParentID < items[j].ParentID
		}
		if items[i].Order != items[j].Order {
			return items[i].Order < items[j].Order
		}
		return items[i].ID < items[j].ID
	})
}

func buildMenuTree(items []menuItem, parentID int64) []vbenMenu {
	res := make([]vbenMenu, 0)
	for _, item := range items {
		if item.ParentID != parentID {
			continue
		}

		children := buildMenuTree(items, item.ID)
		menu := item.toVbenMenu(children)
		res = append(res, menu)
	}
	return res
}

func applyExternalLinkTarget(menus []vbenMenu, target string) {
	openInNewWindow := target != "current_page"
	for index := range menus {
		if _, external := menus[index].Meta["link"]; external {
			menus[index].Meta["openInNewWindow"] = openInNewWindow
		}
		applyExternalLinkTarget(menus[index].Children, target)
	}
}

func (m menuItem) toVbenMenu(children []vbenMenu) vbenMenu {
	component := m.pageComponent()
	path, iframeSrc := menuPath(m)
	binding, hasBinding := vbenMenuBinding(m.URI)
	if hasBinding && component == "" {
		path = binding.Path
	}

	meta := map[string]interface{}{
		"title": m.Title,
		"order": m.Order,
	}
	if hasBinding {
		for key, value := range binding.Meta {
			if key == "order" || key == "title" {
				continue
			}
			meta[key] = value
		}
	}
	if pagePermissions := menuPermissionSlugs[normalizeMenuURI(m.URI)]; len(pagePermissions) > 0 {
		meta["authority"] = append([]string(nil), pagePermissions...)
	}
	icon := normalizeMenuIcon(m.Icon)
	if icon == "" && hasBinding {
		icon = binding.Icon
	}
	if icon != "" {
		meta["icon"] = icon
	}
	if m.Type == menuTypeExternal {
		meta["link"] = normalizeMenuURI(m.URI)
		meta["openInNewWindow"] = true
		meta["confirmExternal"] = true
		iframeSrc = ""
	}
	if iframeSrc != "" && !hasBinding {
		meta["iframeSrc"] = iframeSrc
	}

	name := "GoAdminMenu" + strconv.FormatInt(m.ID, 10)
	if hasBinding {
		name = binding.Name
	}

	menu := vbenMenu{
		ID:       m.ID,
		ParentID: m.ParentID,
		Name:     name,
		Path:     path,
		Meta:     meta,
		Children: children,
	}

	if m.Type == menuTypeDirectory || len(children) > 0 {
		if len(children) > 0 {
			menu.Redirect = children[0].Path
		}
		return menu
	}

	if component != "" {
		menu.Component = component
		return menu
	}
	if hasBinding {
		menu.Component = binding.Component
		return menu
	}
	if m.Type == menuTypeExternal {
		return menu
	}

	if iframeSrc != "" {
		menu.Component = "IFrameView"
	}
	return menu
}

func (m menuItem) pageComponent() string {
	if m.Type != menuTypeItem {
		return ""
	}
	return strings.TrimSpace(m.Component)
}

func menuPath(m menuItem) (path string, iframeSrc string) {
	uri := normalizeMenuURI(m.URI)
	if uri == "" || uri == "#" {
		return "/goadmin/menu-" + strconv.FormatInt(m.ID, 10), ""
	}

	// HTTP(S) on a menu item is the existing embedded-IFrame mode. Only
	// menuTypeExternal is emitted as a confirmed external navigation link.
	if isExternalHTTPURL(uri) {
		return "/goadmin/external-" + strconv.FormatInt(m.ID, 10), uri
	}
	if m.pageComponent() != "" {
		return uri, ""
	}

	return "/goadmin" + strings.TrimSuffix(uri, "/"), "/admin" + uri
}

func vbenMenuBinding(uri string) (menuRouteBinding, bool) {
	binding, ok := vbenMenuRouteBindings[normalizeMenuURI(uri)]
	return binding, ok
}

func normalizeMenuURI(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" || uri == "#" {
		return uri
	}
	if isExternalHTTPURL(uri) {
		return uri
	}
	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}
	if uri != "/" {
		uri = strings.TrimSuffix(uri, "/")
	}
	return uri
}

func normalizeMenuIcon(icon string) string {
	icon = strings.TrimSpace(icon)
	if icon == "" || strings.Contains(icon, ":") {
		return icon
	}

	switch icon {
	case "fa-bar-chart", "fa-dashboard":
		return "lucide:layout-dashboard"
	case "fa-tasks":
		return "lucide:settings-2"
	case "fa-users":
		return "lucide:users"
	case "fa-user":
		return "lucide:user"
	case "fa-ban":
		return "lucide:ban"
	case "fa-bars":
		return "lucide:menu"
	case "fa-history":
		return "lucide:history"
	default:
		return icon
	}
}
