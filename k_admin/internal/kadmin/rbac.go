package kadmin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/bootstrap"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/modules/db/dialect"
	"github.com/gin-gonic/gin"
)

type rbacRole struct {
	ID            int64            `json:"id"`
	Name          string           `json:"name"`
	Slug          string           `json:"slug"`
	IsAdmin       bool             `json:"isAdmin"`
	MenuIDs       []int64          `json:"menuIds"`
	PermissionIDs []int64          `json:"permissionIds"`
	UserIDs       []int64          `json:"userIds"`
	Menus         []rbacMenu       `json:"menus"`
	Permissions   []rbacPermission `json:"permissions"`
	Users         []rbacUser       `json:"users"`
	CreatedAt     string           `json:"createdAt"`
	UpdatedAt     string           `json:"updatedAt"`
}

type rbacDepartment struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Code        string     `json:"code"`
	Description string     `json:"description"`
	Sort        int64      `json:"sort"`
	Status      int64      `json:"status"`
	RoleIDs     []int64    `json:"roleIds"`
	Roles       []rbacRole `json:"roles"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
}

type rbacMenu struct {
	ID       int64      `json:"id"`
	ParentID int64      `json:"parentId"`
	Title    string     `json:"title"`
	Icon     string     `json:"icon"`
	URI      string     `json:"uri"`
	Children []rbacMenu `json:"children,omitempty"`
}

type rbacPermission struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	HTTPMethod string `json:"httpMethod"`
	HTTPPath   string `json:"httpPath"`
	PageURI    string `json:"pageUri"`
	PageTitle  string `json:"pageTitle"`
	Scope      string `json:"scope"`
	Button     string `json:"button"`
}

type rbacUser struct {
	ID       int64    `json:"id"`
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Avatar   string   `json:"avatar"`
	RoleIDs  []int64  `json:"roleIds"`
	Roles    []string `json:"roles"`
}

type rolePayload struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type roleMenuPayload struct {
	MenuIDs []int64 `json:"menuIds"`
}

type roleUsersPayload struct {
	UserIDs []int64 `json:"userIds"`
}

type rolePermissionsPayload struct {
	PermissionIDs []int64 `json:"permissionIds"`
}

type departmentPayload struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Sort        int64  `json:"sort"`
	Status      int64  `json:"status"`
}

type departmentRolesPayload struct {
	RoleIDs []int64 `json:"roleIds"`
}

func registerRBACRoutes(api *gin.RouterGroup, s *Store) {
	rbacGroup := api.Group("/rbac", s.requireAuth())
	rbacGroup.GET("/overview", s.requirePermission(rbacManagePermission), s.rbacOverview)
	rbacGroup.GET("/departments", s.requirePermission(rbacManagePermission), s.rbacDepartments)
	rbacGroup.POST("/departments", s.requirePermission(bootstrap.RbacDepartmentCreatePermission), s.createDepartment)
	rbacGroup.PUT("/departments/:id", s.requirePermission(bootstrap.RbacDepartmentUpdatePermission), s.updateDepartment)
	rbacGroup.DELETE("/departments/:id", s.requirePermission(bootstrap.RbacDepartmentDeletePermission), s.deleteDepartment)
	rbacGroup.PUT("/departments/:id/roles", s.requirePermission(bootstrap.RbacDepartmentRolesPermission), s.updateDepartmentRoles)
	rbacGroup.GET("/roles", s.requirePermission(rbacManagePermission), s.rbacRoles)
	rbacGroup.POST("/roles", s.requirePermission(bootstrap.RbacRoleCreatePermission), s.createRole)
	rbacGroup.PUT("/roles/:id", s.requirePermission(bootstrap.RbacRoleUpdatePermission), s.updateRole)
	rbacGroup.DELETE("/roles/:id", s.requirePermission(bootstrap.RbacRoleDeletePermission), s.deleteRole)
	rbacGroup.PUT("/roles/:id/menus", s.requirePermission(bootstrap.RbacRoleMenusPermission), s.updateRoleMenus)
	rbacGroup.PUT("/roles/:id/users", s.requirePermission(bootstrap.RbacRoleUsersPermission), s.updateRoleUsers)
	rbacGroup.PUT("/roles/:id/permissions", s.requirePermission(bootstrap.RbacRolePermissionsPermission), s.updateRolePermissions)
	rbacGroup.GET("/menus", s.requirePermission(rbacManagePermission), s.rbacMenus)
	rbacGroup.GET("/users", s.requirePermission(rbacManagePermission), s.rbacUsers)
}

func (s *Store) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := s.currentUser(c)
		if !ok {
			fail(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}
		if !isAdminUser(user.UserName) && !user.IsSuperAdmin() {
			fail(c, http.StatusForbidden, "permission denied")
			c.Abort()
			return
		}
		c.Set("vben_user", user)
		c.Next()
	}
}

func isAdminUser(username string) bool {
	return strings.EqualFold(strings.TrimSpace(username), "admin")
}

func (s *Store) rbacOverview(c *gin.Context) {
	roles, err := s.loadRBACRoles()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	menus, err := s.loadRBACMenus()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	users, err := s.loadRBACUsers()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	departments, err := s.loadRBACDepartments()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	permissions, err := s.loadRBACPermissions()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, gin.H{
		"roles":       roles,
		"menus":       menus,
		"users":       users,
		"departments": departments,
		"permissions": permissions,
	})
}

func (s *Store) rbacDepartments(c *gin.Context) {
	departments, err := s.loadRBACDepartments()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, departments)
}

func (s *Store) rbacRoles(c *gin.Context) {
	roles, err := s.loadRBACRoles()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, roles)
}

func (s *Store) rbacMenus(c *gin.Context) {
	menus, err := s.loadRBACMenus()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, menus)
}

func (s *Store) rbacUsers(c *gin.Context) {
	users, err := s.loadRBACUsers()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, users)
}

func (s *Store) createDepartment(c *gin.Context) {
	var req departmentPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid department payload")
		return
	}
	normalizeDepartmentPayload(&req)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "department name is required")
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}

	id, err := db.WithDriver(s.conn).Table("goadmin_department").Insert(dialect.H{
		"name":        req.Name,
		"code":        req.Code,
		"description": req.Description,
		"sort":        req.Sort,
		"status":      req.Status,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	department, err := s.loadRBACDepartment(id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, department)
}

func (s *Store) updateDepartment(c *gin.Context) {
	departmentID, ok := pathID(c)
	if !ok {
		return
	}

	var req departmentPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid department payload")
		return
	}
	normalizeDepartmentPayload(&req)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "department name is required")
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}

	_, err := db.WithDriver(s.conn).
		Table("goadmin_department").
		Where("id", "=", departmentID).
		Update(dialect.H{
			"name":        req.Name,
			"code":        req.Code,
			"description": req.Description,
			"sort":        req.Sort,
			"status":      req.Status,
			"updated_at":  nowString(),
		})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	department, err := s.loadRBACDepartment(departmentID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, department)
}

func (s *Store) deleteDepartment(c *gin.Context) {
	departmentID, ok := pathID(c)
	if !ok {
		return
	}

	_ = db.WithDriver(s.conn).Table("goadmin_department_roles").Where("department_id", "=", departmentID).Delete()
	if err := db.WithDriver(s.conn).Table("goadmin_department").Where("id", "=", departmentID).Delete(); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

func (s *Store) updateDepartmentRoles(c *gin.Context) {
	departmentID, ok := pathID(c)
	if !ok {
		return
	}

	var req departmentRolesPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid department role payload")
		return
	}

	_ = db.WithDriver(s.conn).Table("goadmin_department_roles").Where("department_id", "=", departmentID).Delete()
	for _, roleID := range uniqueInt64(req.RoleIDs) {
		if roleID == 0 {
			continue
		}
		if err := s.insertDepartmentRole(departmentID, roleID); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	department, err := s.loadRBACDepartment(departmentID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, department)
}

func (s *Store) createRole(c *gin.Context) {
	var req rolePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid role payload")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	if req.Name == "" {
		fail(c, http.StatusBadRequest, "role name is required")
		return
	}
	if req.Slug == "" {
		req.Slug = slugFromName(req.Name)
	}

	id, err := db.WithDriver(s.conn).Table("goadmin_roles").Insert(dialect.H{
		"name": req.Name,
		"slug": req.Slug,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	role, err := s.loadRBACRole(id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, role)
}

func (s *Store) updateRole(c *gin.Context) {
	roleID, ok := pathID(c)
	if !ok {
		return
	}
	if protectedAdminRole(roleID) {
		fail(c, http.StatusBadRequest, "administrator role cannot be renamed")
		return
	}

	var req rolePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid role payload")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	if req.Name == "" || req.Slug == "" {
		fail(c, http.StatusBadRequest, "role name and slug are required")
		return
	}

	_, err := db.WithDriver(s.conn).
		Table("goadmin_roles").
		Where("id", "=", roleID).
		Update(dialect.H{
			"name":       req.Name,
			"slug":       req.Slug,
			"updated_at": nowString(),
		})
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	role, err := s.loadRBACRole(roleID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, role)
}

func (s *Store) deleteRole(c *gin.Context) {
	roleID, ok := pathID(c)
	if !ok {
		return
	}
	if protectedAdminRole(roleID) {
		fail(c, http.StatusBadRequest, "administrator role cannot be deleted")
		return
	}

	_ = db.WithDriver(s.conn).Table("goadmin_role_menu").Where("role_id", "=", roleID).Delete()
	_ = db.WithDriver(s.conn).Table("goadmin_role_permissions").Where("role_id", "=", roleID).Delete()
	_ = db.WithDriver(s.conn).Table("goadmin_role_users").Where("role_id", "=", roleID).Delete()
	_ = db.WithDriver(s.conn).Table("goadmin_department_roles").Where("role_id", "=", roleID).Delete()
	if err := db.WithDriver(s.conn).Table("goadmin_roles").Where("id", "=", roleID).Delete(); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

func (s *Store) updateRoleMenus(c *gin.Context) {
	roleID, ok := pathID(c)
	if !ok {
		return
	}
	if protectedAdminRole(roleID) {
		fail(c, http.StatusBadRequest, "administrator role menus are unrestricted")
		return
	}

	var req roleMenuPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid role menu payload")
		return
	}

	_ = db.WithDriver(s.conn).Table("goadmin_role_menu").Where("role_id", "=", roleID).Delete()
	for _, menuID := range uniqueInt64(req.MenuIDs) {
		if menuID == 0 {
			continue
		}
		if err := s.insertRoleMenu(roleID, menuID); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	role, err := s.loadRBACRole(roleID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, role)
}

func (s *Store) updateRolePermissions(c *gin.Context) {
	roleID, ok := pathID(c)
	if !ok {
		return
	}
	if protectedAdminRole(roleID) {
		fail(c, http.StatusBadRequest, "administrator role permissions are unrestricted")
		return
	}

	var req rolePermissionsPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid role permission payload")
		return
	}

	permissions, err := s.loadRBACPermissions()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	valid := make(map[int64]bool, len(permissions))
	for _, permission := range permissions {
		valid[permission.ID] = true
	}
	permissionIDs := uniqueInt64(req.PermissionIDs)
	for _, permissionID := range permissionIDs {
		if !valid[permissionID] {
			fail(c, http.StatusBadRequest, "invalid permission id")
			return
		}
	}

	existingPermissionIDsByRole, err := s.loadRolePermissionIDs()
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if _, exists := existingPermissionIDsByRole[roleID]; exists {
		if err := db.WithDriver(s.conn).Table("goadmin_role_permissions").Where("role_id", "=", roleID).Delete(); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	for _, permissionID := range permissionIDs {
		if err := s.insertRolePermission(roleID, permissionID); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	role, err := s.loadRBACRole(roleID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, role)
}

func (s *Store) updateRoleUsers(c *gin.Context) {
	roleID, ok := pathID(c)
	if !ok {
		return
	}
	if protectedAdminRole(roleID) {
		fail(c, http.StatusBadRequest, "administrator role users are managed by seed data")
		return
	}

	var req roleUsersPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid role user payload")
		return
	}

	_ = db.WithDriver(s.conn).Table("goadmin_role_users").Where("role_id", "=", roleID).Delete()
	for _, userID := range uniqueInt64(req.UserIDs) {
		if userID == 0 || userID == 1 {
			continue
		}
		if err := s.insertRoleUser(roleID, userID); err != nil {
			fail(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	role, err := s.loadRBACRole(roleID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, role)
}

func (s *Store) loadRBACRole(id int64) (rbacRole, error) {
	roles, err := s.loadRBACRoles()
	if err != nil {
		return rbacRole{}, err
	}
	for _, role := range roles {
		if role.ID == id {
			return role, nil
		}
	}
	return rbacRole{}, nil
}

func (s *Store) loadRBACDepartment(id int64) (rbacDepartment, error) {
	departments, err := s.loadRBACDepartments()
	if err != nil {
		return rbacDepartment{}, err
	}
	for _, department := range departments {
		if department.ID == id {
			return department, nil
		}
	}
	return rbacDepartment{}, nil
}

func (s *Store) loadRBACDepartments() ([]rbacDepartment, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_department").
		OrderBy("sort", "asc").
		All()
	if err != nil {
		return nil, err
	}

	roleIDsByDepartment, err := s.loadDepartmentRoleIDs()
	if err != nil {
		return nil, err
	}
	roles, err := s.loadRBACRoles()
	if err != nil {
		return nil, err
	}
	rolesByID := make(map[int64]rbacRole)
	for _, role := range roles {
		rolesByID[role.ID] = role
	}

	departments := make([]rbacDepartment, 0, len(rows))
	for _, row := range rows {
		id := toInt64(row["id"])
		roleIDs := uniqueInt64(roleIDsByDepartment[id])
		department := rbacDepartment{
			ID:          id,
			Name:        toString(row["name"]),
			Code:        toString(row["code"]),
			Description: toString(row["description"]),
			Sort:        toInt64(row["sort"]),
			Status:      toInt64(row["status"]),
			RoleIDs:     roleIDs,
			CreatedAt:   toString(row["created_at"]),
			UpdatedAt:   toString(row["updated_at"]),
		}
		for _, roleID := range roleIDs {
			if role, ok := rolesByID[roleID]; ok {
				department.Roles = append(department.Roles, role)
			}
		}
		departments = append(departments, department)
	}
	return departments, nil
}

func (s *Store) loadRBACRoles() ([]rbacRole, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_roles").
		OrderBy("id", "asc").
		All()
	if err != nil {
		return nil, err
	}

	menuIDsByRole, err := s.loadRoleMenuIDs()
	if err != nil {
		return nil, err
	}
	permissionIDsByRole, err := s.loadRolePermissionIDs()
	if err != nil {
		return nil, err
	}
	userIDsByRole, err := s.loadRoleUserIDs()
	if err != nil {
		return nil, err
	}
	menusByID, err := s.loadMenuMap()
	if err != nil {
		return nil, err
	}
	permissions, err := s.loadRBACPermissions()
	if err != nil {
		return nil, err
	}
	permissionsByID := make(map[int64]rbacPermission, len(permissions))
	for _, permission := range permissions {
		permissionsByID[permission.ID] = permission
	}
	usersByID, err := s.loadUserMap()
	if err != nil {
		return nil, err
	}

	roles := make([]rbacRole, 0, len(rows))
	for _, row := range rows {
		id := toInt64(row["id"])
		menuIDs := uniqueInt64(menuIDsByRole[id])
		permissionIDs := uniqueInt64(permissionIDsByRole[id])
		userIDs := uniqueInt64(userIDsByRole[id])
		role := rbacRole{
			ID:            id,
			Name:          toString(row["name"]),
			Slug:          toString(row["slug"]),
			IsAdmin:       protectedAdminRole(id),
			MenuIDs:       menuIDs,
			PermissionIDs: permissionIDs,
			UserIDs:       userIDs,
			CreatedAt:     toString(row["created_at"]),
			UpdatedAt:     toString(row["updated_at"]),
		}
		if role.IsAdmin {
			role.MenuIDs = mapKeys(menusByID)
			role.PermissionIDs = make([]int64, 0, len(permissions))
			for _, permission := range permissions {
				role.PermissionIDs = append(role.PermissionIDs, permission.ID)
			}
		}
		for _, menuID := range role.MenuIDs {
			if menu, ok := menusByID[menuID]; ok {
				role.Menus = append(role.Menus, menu)
			}
		}
		for _, permissionID := range role.PermissionIDs {
			if permission, ok := permissionsByID[permissionID]; ok {
				role.Permissions = append(role.Permissions, permission)
			}
		}
		for _, userID := range userIDs {
			if user, ok := usersByID[userID]; ok {
				role.Users = append(role.Users, user)
			}
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (s *Store) loadRBACMenus() ([]rbacMenu, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_menu").
		OrderBy("order", "asc").
		All()
	if err != nil {
		return nil, err
	}

	items := make([]rbacMenu, 0, len(rows))
	for _, row := range rows {
		items = append(items, rbacMenu{
			ID:       toInt64(row["id"]),
			ParentID: toInt64(row["parent_id"]),
			Title:    toString(row["title"]),
			Icon:     toString(row["icon"]),
			URI:      toString(row["uri"]),
		})
	}
	return buildRBACMenuTree(items, 0), nil
}

func (s *Store) loadRBACPermissions() ([]rbacPermission, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_permissions").
		OrderBy("id", "asc").
		All()
	if err != nil {
		return nil, err
	}

	permissions := make([]rbacPermission, 0, len(rows))
	for _, row := range rows {
		slug := toString(row["slug"])
		httpPath := toString(row["http_path"])
		pageURI, pageTitle, scope, button := permissionMetadata(slug, httpPath)
		permissions = append(permissions, rbacPermission{
			ID:         toInt64(row["id"]),
			Name:       toString(row["name"]),
			Slug:       slug,
			HTTPMethod: toString(row["http_method"]),
			HTTPPath:   httpPath,
			PageURI:    pageURI,
			PageTitle:  pageTitle,
			Scope:      scope,
			Button:     button,
		})
	}
	return permissions, nil
}

func permissionMetadata(slug, httpPath string) (pageURI, pageTitle, scope, button string) {
	for _, seed := range defaultPermissionSeeds {
		if seed.Slug == slug {
			return seed.PageURI, seed.PageTitle, string(seed.Scope), seed.Button
		}
	}
	pageURI = generatedPermissionPageURI(httpPath)
	if pageURI == "" {
		return "", "其他 API", "api", ""
	}
	pageTitle = strings.TrimPrefix(pageURI, "/")
	action := slug[strings.LastIndex(slug, ":")+1:]
	if action == "list" || action == "view" {
		return pageURI, pageTitle, string(bootstrap.PermissionScopePage), ""
	}
	button = strings.ReplaceAll(strings.TrimPrefix(slug, "system:"), ":", ".")
	return pageURI, pageTitle, string(bootstrap.PermissionScopeButton), button
}

func generatedPermissionPageURI(httpPath string) string {
	path := strings.TrimSpace(strings.Split(httpPath, ",")[0])
	if !strings.HasPrefix(path, "/api/") {
		return ""
	}
	path = strings.TrimPrefix(path, "/api")
	path = strings.TrimSuffix(path, "*")
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 0 || segments[0] == "" || strings.HasPrefix(segments[0], "{") {
		return ""
	}
	return "/" + segments[0]
}

func (s *Store) loadRBACUsers() ([]rbacUser, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_users").
		OrderBy("id", "asc").
		All()
	if err != nil {
		return nil, err
	}

	roleIDsByUser, roleNamesByUser, err := s.loadUserRoles()
	if err != nil {
		return nil, err
	}

	users := make([]rbacUser, 0, len(rows))
	for _, row := range rows {
		id := toInt64(row["id"])
		users = append(users, rbacUser{
			ID:       id,
			Username: toString(row["username"]),
			Name:     toString(row["name"]),
			Avatar:   toString(row["avatar"]),
			RoleIDs:  uniqueInt64(roleIDsByUser[id]),
			Roles:    roleNamesByUser[id],
		})
	}
	return users, nil
}

func (s *Store) loadRoleMenuIDs() (map[int64][]int64, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_role_menu").All()
	if err != nil {
		return nil, err
	}
	res := make(map[int64][]int64)
	for _, row := range rows {
		roleID := toInt64(row["role_id"])
		res[roleID] = append(res[roleID], toInt64(row["menu_id"]))
	}
	return res, nil
}

func (s *Store) loadRolePermissionIDs() (map[int64][]int64, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_role_permissions").All()
	if err != nil {
		return nil, err
	}
	res := make(map[int64][]int64)
	for _, row := range rows {
		roleID := toInt64(row["role_id"])
		res[roleID] = append(res[roleID], toInt64(row["permission_id"]))
	}
	return res, nil
}

func (s *Store) loadRoleUserIDs() (map[int64][]int64, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_role_users").All()
	if err != nil {
		return nil, err
	}
	res := make(map[int64][]int64)
	for _, row := range rows {
		roleID := toInt64(row["role_id"])
		res[roleID] = append(res[roleID], toInt64(row["user_id"]))
	}
	return res, nil
}

func (s *Store) loadDepartmentRoleIDs() (map[int64][]int64, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_department_roles").All()
	if err != nil {
		return nil, err
	}
	res := make(map[int64][]int64)
	for _, row := range rows {
		departmentID := toInt64(row["department_id"])
		res[departmentID] = append(res[departmentID], toInt64(row["role_id"]))
	}
	return res, nil
}

func (s *Store) loadMenuMap() (map[int64]rbacMenu, error) {
	rows, err := db.WithDriver(s.conn).Table("goadmin_menu").All()
	if err != nil {
		return nil, err
	}
	res := make(map[int64]rbacMenu)
	for _, row := range rows {
		id := toInt64(row["id"])
		res[id] = rbacMenu{
			ID:       id,
			ParentID: toInt64(row["parent_id"]),
			Title:    toString(row["title"]),
			Icon:     toString(row["icon"]),
			URI:      toString(row["uri"]),
		}
	}
	return res, nil
}

func (s *Store) loadUserMap() (map[int64]rbacUser, error) {
	users, err := s.loadRBACUsers()
	if err != nil {
		return nil, err
	}
	res := make(map[int64]rbacUser)
	for _, user := range users {
		res[user.ID] = user
	}
	return res, nil
}

func (s *Store) loadUserRoles() (map[int64][]int64, map[int64][]string, error) {
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_role_users").
		LeftJoin("goadmin_roles", "goadmin_roles.id", "=", "goadmin_role_users.role_id").
		Select("goadmin_role_users.user_id", "goadmin_role_users.role_id", "goadmin_roles.name").
		All()
	if err != nil {
		return nil, nil, err
	}

	roleIDs := make(map[int64][]int64)
	roleNames := make(map[int64][]string)
	for _, row := range rows {
		userID := toInt64(row["user_id"])
		roleIDs[userID] = append(roleIDs[userID], toInt64(row["role_id"]))
		if name := toString(row["name"]); name != "" {
			roleNames[userID] = append(roleNames[userID], name)
		}
	}
	return roleIDs, roleNames, nil
}

func buildRBACMenuTree(items []rbacMenu, parentID int64) []rbacMenu {
	res := make([]rbacMenu, 0)
	for _, item := range items {
		if item.ParentID != parentID {
			continue
		}
		item.Children = buildRBACMenuTree(items, item.ID)
		res = append(res, item)
	}
	return res
}

func pathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func protectedAdminRole(roleID int64) bool {
	return roleID == 1
}

func slugFromName(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.ReplaceAll(slug, " ", "-")
	if slug == "" {
		return "role"
	}
	return slug
}

func nowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func normalizeDepartmentPayload(req *departmentPayload) {
	req.Name = strings.TrimSpace(req.Name)
	req.Code = strings.TrimSpace(req.Code)
	req.Description = strings.TrimSpace(req.Description)
	if req.Sort < 0 {
		req.Sort = 0
	}
}

func (s *Store) insertRoleMenu(roleID, menuID int64) error {
	_, err := s.conn.Exec(
		"INSERT INTO `goadmin_role_menu` (`role_id`, `menu_id`) VALUES (?, ?)",
		roleID,
		menuID,
	)
	return err
}

func (s *Store) insertRolePermission(roleID, permissionID int64) error {
	_, err := s.conn.Exec(
		"INSERT INTO `goadmin_role_permissions` (`role_id`, `permission_id`) VALUES (?, ?)",
		roleID,
		permissionID,
	)
	return err
}

func (s *Store) insertRoleUser(roleID, userID int64) error {
	_, err := s.conn.Exec(
		"INSERT INTO `goadmin_role_users` (`role_id`, `user_id`) VALUES (?, ?)",
		roleID,
		userID,
	)
	return err
}

func (s *Store) insertDepartmentRole(departmentID, roleID int64) error {
	_, err := s.conn.Exec(
		"INSERT INTO `goadmin_department_roles` (`department_id`, `role_id`) VALUES (?, ?)",
		departmentID,
		roleID,
	)
	return err
}

func uniqueInt64(values []int64) []int64 {
	seen := make(map[int64]bool)
	res := make([]int64, 0, len(values))
	for _, value := range values {
		if value == 0 || seen[value] {
			continue
		}
		seen[value] = true
		res = append(res, value)
	}
	return res
}

func mapKeys(m map[int64]rbacMenu) []int64 {
	res := make([]int64, 0, len(m))
	for key := range m {
		res = append(res, key)
	}
	return res
}
