package kadmin

import (
	"net/http"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/bootstrap"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/modules/db/dialect"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

const (
	logListPermission            = bootstrap.LogListPermission
	logDeletePermission          = bootstrap.LogDeletePermission
	userManagePermission         = bootstrap.UserManagePermission
	userCreatePermission         = bootstrap.UserCreatePermission
	userUpdatePermission         = bootstrap.UserUpdatePermission
	userStatusPermission         = bootstrap.UserStatusPermission
	userDeletePermission         = bootstrap.UserDeletePermission
	userPasswordPermission       = bootstrap.UserPasswordPermission
	userUnlockPermission         = bootstrap.UserUnlockPermission
	userImportPermission         = bootstrap.UserImportPermission
	userExportPermission         = bootstrap.UserExportPermission
	rbacManagePermission         = bootstrap.RbacManagePermission
	menuManagePermission         = bootstrap.MenuManagePermission
	dictionaryManagePermission   = bootstrap.DictionaryManagePermission
	systemConfigManagePermission = bootstrap.SystemConfigManagePermission
)

var defaultPermissionSeeds = bootstrap.DefaultPermissions()

func (s *Store) syncDefaultPermissions() error {
	rows, err := db.WithDriver(s.conn).Table("goadmin_permissions").Select("slug").All()
	if err != nil {
		return err
	}
	existing := make(map[string]bool, len(rows))
	for _, row := range rows {
		existing[toString(row["slug"])] = true
	}

	for _, seed := range defaultPermissionSeeds {
		values := dialect.H{
			"name":        seed.Name,
			"http_method": seed.HTTPMethod,
			"http_path":   seed.HTTPPath,
			"updated_at":  nowString(),
		}
		if existing[seed.Slug] {
			if _, err := db.WithDriver(s.conn).Table("goadmin_permissions").Where("slug", "=", seed.Slug).Update(values); err != nil {
				return err
			}
			continue
		}
		values["slug"] = seed.Slug
		values["created_at"] = nowString()
		if _, err := db.WithDriver(s.conn).Table("goadmin_permissions").Insert(values); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) requirePermission(codes ...string) gin.HandlerFunc {
	return s.permissionRequired(s.currentUser, codes...)
}

func (s *Store) permissionRequired(resolveUser func(*gin.Context) (models.UserModel, bool), codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := resolveUser(c)
		if !ok {
			fail(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}
		if user.IsSuperAdmin() || isAdminUser(user.UserName) || userHasPermission(user, codes...) {
			c.Set("vben_user", user)
			c.Next()
			return
		}
		// 菜单授权隐含页面权限：角色在权限工作台勾选菜单后即可访问该页面接口。
		hasMenuPermission, menuErr := s.userHasMenuPermission(user, codes...)
		if menuErr != nil {
			fail(c, http.StatusInternalServerError, menuErr.Error())
			c.Abort()
			return
		}
		if !hasMenuPermission {
			fail(c, http.StatusForbidden, "permission denied")
			c.Abort()
			return
		}
		c.Set("vben_user", user)
		c.Next()
	}
}

func userHasPermission(user models.UserModel, codes ...string) bool {
	wanted := make(map[string]bool, len(codes))
	for _, code := range codes {
		wanted[code] = true
	}
	for _, permission := range user.Permissions {
		if permission.Slug == "*" || wanted[permission.Slug] {
			return true
		}
	}
	return false
}
