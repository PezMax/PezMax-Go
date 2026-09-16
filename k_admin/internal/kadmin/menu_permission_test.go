package kadmin

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/codegen"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/files"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/jobs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loginlogs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/monitor"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

// fakeMenuPermissionConnection 返回角色菜单绑定查询所需的菜单 URI，
// 模拟 goadmin_role_menu JOIN goadmin_menu 的结果。
type fakeMenuPermissionConnection struct {
	db.Connection
	permissionRows []map[string]interface{}
	uris           []string
}

func (f *fakeMenuPermissionConnection) Name() string { return "sqlite" }

func (f *fakeMenuPermissionConnection) QueryWith(_ *sql.Tx, _ string, query string, _ ...interface{}) ([]map[string]interface{}, error) {
	if strings.Contains(query, "goadmin_permissions") {
		return f.permissionRows, nil
	}
	if !strings.Contains(query, "goadmin_role_menu") {
		return nil, nil
	}
	rows := make([]map[string]interface{}, 0, len(f.uris))
	for _, uri := range f.uris {
		rows = append(rows, map[string]interface{}{"uri": uri})
	}
	return rows, nil
}

func TestMenuPermissionSlugsMatchSeedsAndBindings(t *testing.T) {
	seededURIs := make(map[string]bool)
	for _, root := range defaultMenuSeeds {
		seededURIs[root.URI] = true
		for _, child := range root.Children {
			seededURIs[child.URI] = true
		}
	}
	seededSlugs := make(map[string]bool)
	for _, seed := range defaultPermissionSeeds {
		seededSlugs[seed.Slug] = true
	}

	for uri, slugs := range menuPermissionSlugs {
		if _, ok := vbenMenuRouteBindings[uri]; !ok {
			t.Errorf("menu URI %q has no vben route binding", uri)
		}
		if !seededURIs[uri] {
			t.Errorf("menu URI %q has no menu seed", uri)
		}
		for _, slug := range slugs {
			if !seededSlugs[slug] {
				t.Errorf("menu URI %q maps to unseeded permission %q", uri, slug)
			}
		}
	}
}

func TestMenuPermissionSlugsForUser(t *testing.T) {
	store := &Store{conn: &fakeMenuPermissionConnection{uris: []string{"/kadmin", "/kadmin/jobs", "/unbound-page"}}}
	user := models.UserModel{Roles: []models.RoleModel{{Id: 2}}}

	slugs, err := store.menuPermissionSlugsForUser(user)
	if err != nil {
		t.Fatal(err)
	}
	if !slugs[jobs.ListPermission] {
		t.Errorf("expected menu /kadmin/jobs to grant page permission %q", jobs.ListPermission)
	}
	for _, action := range []string{
		jobs.CreatePermission,
		jobs.UpdatePermission,
		jobs.DeletePermission,
		jobs.RunPermission,
		jobs.LogListPermission,
	} {
		if slugs[action] {
			t.Errorf("menu must not implicitly grant button permission %q", action)
		}
	}
	if slugs[files.UploadPermission] {
		t.Fatal("unrelated resource permission must not be granted")
	}

	empty, err := (&Store{conn: &fakeMenuPermissionConnection{}}).menuPermissionSlugsForUser(models.UserModel{})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("user without roles must have no menu permissions, got %v", empty)
	}
}

func TestGeneratedMenuGrantsOnlyGeneratedReadPermission(t *testing.T) {
	store := &Store{conn: &fakeMenuPermissionConnection{
		uris: []string{"/product"},
		permissionRows: []map[string]interface{}{
			{"slug": "system:product:list", "http_method": "GET"},
			{"slug": "system:product:create", "http_method": "POST"},
			{"slug": "system:product:export", "http_method": "GET"},
		},
	}}
	user := models.UserModel{Roles: []models.RoleModel{{Id: 2}}}

	slugs, err := store.menuPermissionSlugsForUser(user)
	if err != nil {
		t.Fatal(err)
	}
	if !slugs["system:product:list"] {
		t.Fatal("generated menu must grant its list permission")
	}
	for _, forbidden := range []string{"system:product:create", "system:product:export"} {
		if slugs[forbidden] {
			t.Errorf("generated menu must not grant action permission %q", forbidden)
		}
	}
}

func TestPermissionRequiredAcceptsMenuGrantedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &Store{conn: &fakeMenuPermissionConnection{uris: []string{"/kadmin/jobs"}}}
	engine := gin.New()
	engine.GET("/api/jobs", store.permissionRequired(func(*gin.Context) (models.UserModel, bool) {
		return models.UserModel{Roles: []models.RoleModel{{Id: 2}}}, true
	}, jobs.ListPermission), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/jobs", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}

func TestPermissionRequiredDeniesUserWithoutMenuOrPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &Store{conn: &fakeMenuPermissionConnection{uris: []string{"/kadmin/jobs"}}}
	engine := gin.New()
	engine.GET("/api/login-audits", store.permissionRequired(func(*gin.Context) (models.UserModel, bool) {
		return models.UserModel{Roles: []models.RoleModel{{Id: 2}}}, true
	}, loginlogs.ListPermission), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/login-audits", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
}

func TestUserAccessCodesIncludeMenuPagePermission(t *testing.T) {
	store := &Store{conn: &fakeMenuPermissionConnection{uris: []string{"/kadmin/logs"}}}
	user := models.UserModel{Roles: []models.RoleModel{{Id: 2, Slug: "operator"}}}

	codes, err := store.userAccessCodes(user)
	if err != nil {
		t.Fatal(err)
	}
	set := make(map[string]bool, len(codes))
	for _, code := range codes {
		set[code] = true
	}
	for _, wanted := range []string{"operator", logListPermission} {
		if !set[wanted] {
			t.Errorf("access code %q was not returned", wanted)
		}
	}
	if set[logDeletePermission] {
		t.Fatal("button permission must require an explicit role or user grant")
	}

	if codes, err := store.userAccessCodes(models.UserModel{}); err != nil || len(codes) != 0 {
		t.Fatalf("user without roles should have no access codes, got %v err=%v", codes, err)
	}
}

func TestUserMenuGrantsReadButNotMutationPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &Store{conn: &fakeMenuPermissionConnection{uris: []string{"/kadmin/users"}}}
	userFromMenu := models.UserModel{Roles: []models.RoleModel{{Id: 2}}}

	engine := gin.New()
	engine.GET("/api/users", store.permissionRequired(func(*gin.Context) (models.UserModel, bool) {
		return userFromMenu, true
	}, userManagePermission), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	engine.POST("/api/users", store.permissionRequired(func(*gin.Context) (models.UserModel, bool) {
		return userFromMenu, true
	}, userCreatePermission), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	engine.POST("/api/users-with-create", store.permissionRequired(func(*gin.Context) (models.UserModel, bool) {
		return models.UserModel{Permissions: []models.PermissionModel{{Slug: userCreatePermission}}}, true
	}, userCreatePermission), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, testCase := range []struct {
		method string
		path   string
		want   int
	}{
		{method: http.MethodGet, path: "/api/users", want: http.StatusNoContent},
		{method: http.MethodPost, path: "/api/users", want: http.StatusForbidden},
		{method: http.MethodPost, path: "/api/users-with-create", want: http.StatusNoContent},
	} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(testCase.method, testCase.path, nil))
		if response.Code != testCase.want {
			t.Errorf("%s %s status = %d, want %d; body=%s", testCase.method, testCase.path, response.Code, testCase.want, response.Body.String())
		}
	}
}

func TestUserPermissionCatalogCoversEveryManagementAPI(t *testing.T) {
	wanted := map[string]struct {
		method string
		path   string
		button string
	}{
		userManagePermission:       {method: http.MethodGet, path: "/api/users"},
		userCreatePermission:       {method: http.MethodPost, path: "/api/users", button: "users.create"},
		userUpdatePermission:       {method: http.MethodPut, path: "/api/users/{id}", button: "users.update"},
		userStatusPermission:       {method: http.MethodPut, path: "/api/users/{id}/status", button: "users.status"},
		userDeletePermission:       {method: http.MethodDelete, path: "/api/users/{id}", button: "users.delete"},
		userPasswordPermission:     {method: http.MethodPut, path: "/api/users/{id}/password", button: "users.password"},
		userUnlockPermission:       {method: http.MethodPut, path: "/api/users/{id}/unlock", button: "users.unlock"},
		userImportPermission:       {method: http.MethodPost, path: "/api/users/import", button: "users.import"},
		userExportPermission:       {method: http.MethodGet, path: "/api/users/export", button: "users.export"},
		files.UserAvatarPermission: {method: http.MethodPost, path: "/api/users/avatar", button: "users.avatar"},
	}

	for _, seed := range defaultPermissionSeeds {
		expected, ok := wanted[seed.Slug]
		if !ok {
			continue
		}
		if seed.HTTPMethod != expected.method || seed.HTTPPath != expected.path || seed.Button != expected.button {
			t.Errorf("permission %q metadata = %s %s button=%q, want %s %s button=%q", seed.Slug, seed.HTTPMethod, seed.HTTPPath, seed.Button, expected.method, expected.path, expected.button)
		}
		delete(wanted, seed.Slug)
	}
	for slug := range wanted {
		t.Errorf("user API permission %q is missing from the catalog", slug)
	}
}

func TestMenuPermissionSlugsCoverPermissionGatedMenus(t *testing.T) {
	// 每个带权限保护的页面菜单都必须有映射，否则“菜单可见即接口可用”会再次失效。
	for _, uri := range []string{
		"/kadmin/users",
		"/kadmin/rbac",
		"/kadmin/menus",
		"/kadmin/dictionary",
		"/kadmin/settings",
		"/kadmin/logs",
		"/kadmin/login-audits",
		"/kadmin/jobs",
		"/kadmin/monitor",
		"/kadmin/load-ranking",
		"/kadmin/codegen",
		"/kadmin/resources",
	} {
		if _, ok := menuPermissionSlugs[uri]; !ok {
			t.Errorf("permission-gated menu %q has no permission mapping", uri)
		}
	}
}

func TestMenuPermissionSlugsGrantOnlyPagePermissions(t *testing.T) {
	wanted := map[string]string{
		"/kadmin/users":        userManagePermission,
		"/kadmin/rbac":         rbacManagePermission,
		"/kadmin/menus":        menuManagePermission,
		"/kadmin/dictionary":   dictionaryManagePermission,
		"/kadmin/settings":     systemConfigManagePermission,
		"/kadmin/logs":         logListPermission,
		"/kadmin/login-audits": loginlogs.ListPermission,
		"/kadmin/jobs":         jobs.ListPermission,
		"/kadmin/monitor":      monitor.ViewPermission,
		"/kadmin/load-ranking": "system:load-rank:view",
		"/kadmin/codegen":      codegen.ListPermission,
		"/kadmin/resources":    files.ReadPermission,
	}
	for uri, slug := range wanted {
		actual := menuPermissionSlugs[uri]
		if len(actual) != 1 || actual[0] != slug {
			t.Errorf("menu %q page permissions = %v, want [%s]", uri, actual, slug)
		}
	}
}

func TestPermissionCatalogLinksPageButtonAndAPI(t *testing.T) {
	seen := make(map[string]bool, len(defaultPermissionSeeds))
	for _, seed := range defaultPermissionSeeds {
		if seen[seed.Slug] {
			t.Errorf("duplicate permission slug %q", seed.Slug)
		}
		seen[seed.Slug] = true
		if seed.PageURI == "" || seed.PageTitle == "" {
			t.Errorf("permission %q is not linked to a page", seed.Slug)
		}
		if seed.HTTPMethod == "" || seed.HTTPPath == "" {
			t.Errorf("permission %q is not linked to an API", seed.Slug)
		}
		switch seed.Scope {
		case "page":
			if seed.Button != "" {
				t.Errorf("page permission %q must not declare a button", seed.Slug)
			}
		case "button":
			if seed.Button == "" {
				t.Errorf("button permission %q has no button identifier", seed.Slug)
			}
		default:
			t.Errorf("permission %q has invalid scope %q", seed.Slug, seed.Scope)
		}
	}
}
