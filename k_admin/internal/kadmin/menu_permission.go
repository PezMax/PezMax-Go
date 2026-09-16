package kadmin

import (
	"net/http"
	"strings"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/codegen"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/files"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/jobs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loadrank"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loginlogs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/monitor"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
)

// menuPermissionSlugs 将菜单 URI 映射到页面基础权限。
// 勾选菜单只继承进入页面及读取基础数据所需的权限；按钮/API 操作权限必须显式授予。
// 与 goadmin_role_permissions 中的显式权限是叠加关系。
var menuPermissionSlugs = map[string][]string{
	"/kadmin/users":        {userManagePermission},
	"/kadmin/rbac":         {rbacManagePermission},
	"/kadmin/menus":        {menuManagePermission},
	"/kadmin/dictionary":   {dictionaryManagePermission},
	"/kadmin/settings":     {systemConfigManagePermission},
	"/kadmin/logs":         {logListPermission},
	"/kadmin/login-audits": {loginlogs.ListPermission},
	"/kadmin/jobs":         {jobs.ListPermission},
	"/kadmin/monitor":      {monitor.ViewPermission},
	"/kadmin/load-ranking": {loadrank.ViewPermission},
	"/kadmin/codegen":      {codegen.ListPermission},
	"/kadmin/resources":    {files.ReadPermission},
}

// menuPermissionSlugsForUser 返回用户角色已授权菜单对应的页面权限 slug 集合。
// 与菜单可见性使用同一数据源（goadmin_role_menu），保证“菜单可见即接口可用”。
func (s *Store) menuPermissionSlugsForUser(user models.UserModel) (map[string]bool, error) {
	roleIDs := user.GetAllRoleId()
	if len(roleIDs) == 0 {
		return nil, nil
	}
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_role_menu").
		LeftJoin("goadmin_menu", "goadmin_menu.id", "=", "goadmin_role_menu.menu_id").
		WhereIn("goadmin_role_menu.role_id", roleIDs).
		Select("goadmin_menu.uri").
		All()
	if err != nil {
		return nil, err
	}
	slugs := make(map[string]bool)

	for _, row := range rows {
		uri := normalizeMenuURI(toString(row["uri"]))
		pageSlugs := menuPermissionSlugs[uri]
		if len(pageSlugs) == 0 {
			pageSlugs, err = s.generatedPagePermissionSlugs(uri)
			if err != nil {
				return nil, err
			}
		}
		for _, slug := range pageSlugs {
			slugs[slug] = true
		}
	}
	return slugs, nil
}

func (s *Store) generatedPagePermissionSlugs(menuURI string) ([]string, error) {
	if menuURI == "" || strings.HasPrefix(menuURI, "/kadmin/") {
		return nil, nil
	}
	rows, err := db.WithDriver(s.conn).
		Table("goadmin_permissions").
		Where("http_path", "LIKE", "/api"+menuURI+"%").
		Select("slug", "http_method").
		All()
	if err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(rows))
	for _, row := range rows {
		slug := strings.TrimSpace(toString(row["slug"]))
		method := strings.ToUpper(toString(row["http_method"]))
		if strings.Contains(method, http.MethodGet) && (strings.HasSuffix(slug, ":list") || strings.HasSuffix(slug, ":view")) {
			slugs = append(slugs, slug)
		}
	}
	return slugs, nil
}

// userHasMenuPermission 判断用户是否通过菜单授权获得指定页面基础权限。
func (s *Store) userHasMenuPermission(user models.UserModel, codes ...string) (bool, error) {
	slugs, err := s.menuPermissionSlugsForUser(user)
	if err != nil {
		return false, err
	}
	for _, code := range codes {
		if slugs[code] {
			return true, nil
		}
	}
	return false, nil
}

// userAccessCodes 返回用户全部访问标识：角色标识、直接配置的权限以及菜单隐含的页面权限。
func (s *Store) userAccessCodes(user models.UserModel) ([]string, error) {
	codes := accessCodes(user)
	slugs, err := s.menuPermissionSlugsForUser(user)
	if err != nil {
		return nil, err
	}
	if len(slugs) == 0 {
		return codes, nil
	}

	set := make(map[string]bool, len(codes)+len(slugs))
	for _, code := range codes {
		set[code] = true
	}
	for slug := range slugs {
		set[slug] = true
	}
	merged := make([]string, 0, len(set))
	for code := range set {
		merged = append(merged, code)
	}
	return merged, nil
}
