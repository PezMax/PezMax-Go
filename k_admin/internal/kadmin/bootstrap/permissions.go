package bootstrap

import (
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/codegen"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/files"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/jobs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loadrank"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/loginlogs"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/monitor"
	"github.com/GoAdminGroup/go-admin/internal/kadmin/modules/notifications"
)

const (
	LogListPermission      = "system:log:list"
	LogDeletePermission    = "system:log:delete"
	UserManagePermission   = "system:user:manage"
	UserCreatePermission   = "system:user:create"
	UserUpdatePermission   = "system:user:update"
	UserStatusPermission   = "system:user:status"
	UserDeletePermission   = "system:user:delete"
	UserPasswordPermission = "system:user:password"
	UserUnlockPermission   = "system:user:unlock"
	UserImportPermission   = "system:user:import"
	UserExportPermission   = "system:user:export"
)

const (
	RbacManagePermission           = "system:rbac:manage"
	RbacDepartmentCreatePermission = "system:rbac:department:create"
	RbacDepartmentUpdatePermission = "system:rbac:department:update"
	RbacDepartmentDeletePermission = "system:rbac:department:delete"
	RbacDepartmentRolesPermission  = "system:rbac:department:roles"
	RbacRoleCreatePermission       = "system:rbac:role:create"
	RbacRoleUpdatePermission       = "system:rbac:role:update"
	RbacRoleDeletePermission       = "system:rbac:role:delete"
	RbacRoleMenusPermission        = "system:rbac:role:menus"
	RbacRoleUsersPermission        = "system:rbac:role:users"
	RbacRolePermissionsPermission  = "system:rbac:role:permissions"
	MenuManagePermission           = "system:menu:manage"
	MenuCreatePermission           = "system:menu:create"
	MenuUpdatePermission           = "system:menu:update"
	MenuLayoutPermission           = "system:menu:layout"
	MenuDeletePermission           = "system:menu:delete"
	DictionaryManagePermission     = "system:dict:manage"
	DictionaryTypeCreatePermission = "system:dict:type:create"
	DictionaryTypeUpdatePermission = "system:dict:type:update"
	DictionaryTypeDeletePermission = "system:dict:type:delete"
	DictionaryDataCreatePermission = "system:dict:data:create"
	DictionaryDataUpdatePermission = "system:dict:data:update"
	DictionaryDataDeletePermission = "system:dict:data:delete"
	SystemConfigManagePermission   = "system:config:manage"
	SystemConfigUpdatePermission   = "system:config:update"
)

type PermissionScope string

const (
	PermissionScopePage   PermissionScope = "page"
	PermissionScopeButton PermissionScope = "button"
)

type PermissionSeed struct {
	Name       string
	Slug       string
	HTTPMethod string
	HTTPPath   string
	PageURI    string
	PageTitle  string
	Scope      PermissionScope
	Button     string
}

func DefaultPermissions() []PermissionSeed {
	return []PermissionSeed{
		{
			Name: "查看用户", Slug: UserManagePermission,
			HTTPMethod: "GET", HTTPPath: "/api/users",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopePage,
		},
		{
			Name: "新增用户", Slug: UserCreatePermission, HTTPMethod: "POST", HTTPPath: "/api/users",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.create",
		},
		{
			Name: "编辑用户", Slug: UserUpdatePermission, HTTPMethod: "PUT", HTTPPath: "/api/users/{id}",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.update",
		},
		{
			Name: "修改用户状态", Slug: UserStatusPermission, HTTPMethod: "PUT", HTTPPath: "/api/users/{id}/status",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.status",
		},
		{
			Name: "删除用户", Slug: UserDeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/users/{id}",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.delete",
		},
		{
			Name: "重置用户密码", Slug: UserPasswordPermission, HTTPMethod: "PUT", HTTPPath: "/api/users/{id}/password",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.password",
		},
		{
			Name: "解锁用户登录", Slug: UserUnlockPermission, HTTPMethod: "PUT", HTTPPath: "/api/users/{id}/unlock",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.unlock",
		},
		{
			Name: "导入用户", Slug: UserImportPermission, HTTPMethod: "POST", HTTPPath: "/api/users/import",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.import",
		},
		{
			Name: "导出用户", Slug: UserExportPermission, HTTPMethod: "GET", HTTPPath: "/api/users/export",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.export",
		},
		{
			Name: "查看权限配置", Slug: RbacManagePermission,
			HTTPMethod: "GET", HTTPPath: "/api/rbac*",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopePage,
		},
		{
			Name: "新增部门", Slug: RbacDepartmentCreatePermission, HTTPMethod: "POST", HTTPPath: "/api/rbac/departments",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.departments.create",
		},
		{
			Name: "编辑部门", Slug: RbacDepartmentUpdatePermission, HTTPMethod: "PUT", HTTPPath: "/api/rbac/departments/{id}",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.departments.update",
		},
		{
			Name: "删除部门", Slug: RbacDepartmentDeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/rbac/departments/{id}",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.departments.delete",
		},
		{
			Name: "配置部门职位", Slug: RbacDepartmentRolesPermission, HTTPMethod: "PUT", HTTPPath: "/api/rbac/departments/{id}/roles",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.departments.roles",
		},
		{
			Name: "新增职位", Slug: RbacRoleCreatePermission, HTTPMethod: "POST", HTTPPath: "/api/rbac/roles",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.roles.create",
		},
		{
			Name: "编辑职位", Slug: RbacRoleUpdatePermission, HTTPMethod: "PUT", HTTPPath: "/api/rbac/roles/{id}",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.roles.update",
		},
		{
			Name: "删除职位", Slug: RbacRoleDeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/rbac/roles/{id}",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.roles.delete",
		},
		{
			Name: "配置职位菜单", Slug: RbacRoleMenusPermission, HTTPMethod: "PUT", HTTPPath: "/api/rbac/roles/{id}/menus",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.roles.menus",
		},
		{
			Name: "配置职位用户", Slug: RbacRoleUsersPermission, HTTPMethod: "PUT", HTTPPath: "/api/rbac/roles/{id}/users",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.roles.users",
		},
		{
			Name: "配置职位 API 权限", Slug: RbacRolePermissionsPermission, HTTPMethod: "PUT", HTTPPath: "/api/rbac/roles/{id}/permissions",
			PageURI: "/kadmin/rbac", PageTitle: "权限管理", Scope: PermissionScopeButton, Button: "rbac.roles.permissions",
		},
		{
			Name: "查看菜单", Slug: MenuManagePermission,
			HTTPMethod: "GET", HTTPPath: "/api/admin-menus*",
			PageURI: "/kadmin/menus", PageTitle: "菜单管理", Scope: PermissionScopePage,
		},
		{
			Name: "新增菜单", Slug: MenuCreatePermission, HTTPMethod: "POST", HTTPPath: "/api/admin-menus",
			PageURI: "/kadmin/menus", PageTitle: "菜单管理", Scope: PermissionScopeButton, Button: "menus.create",
		},
		{
			Name: "编辑菜单", Slug: MenuUpdatePermission, HTTPMethod: "PUT", HTTPPath: "/api/admin-menus/{id}",
			PageURI: "/kadmin/menus", PageTitle: "菜单管理", Scope: PermissionScopeButton, Button: "menus.update",
		},
		{
			Name: "调整菜单布局", Slug: MenuLayoutPermission, HTTPMethod: "PUT", HTTPPath: "/api/admin-menus",
			PageURI: "/kadmin/menus", PageTitle: "菜单管理", Scope: PermissionScopeButton, Button: "menus.layout",
		},
		{
			Name: "删除菜单", Slug: MenuDeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/admin-menus/{id}",
			PageURI: "/kadmin/menus", PageTitle: "菜单管理", Scope: PermissionScopeButton, Button: "menus.delete",
		},
		{
			Name: "查看字典", Slug: DictionaryManagePermission,
			HTTPMethod: "GET", HTTPPath: "/api/dictionaries*",
			PageURI: "/kadmin/dictionary", PageTitle: "字典管理", Scope: PermissionScopePage,
		},
		{
			Name: "新增字典类型", Slug: DictionaryTypeCreatePermission, HTTPMethod: "POST", HTTPPath: "/api/dictionaries/types",
			PageURI: "/kadmin/dictionary", PageTitle: "字典管理", Scope: PermissionScopeButton, Button: "dictionary.types.create",
		},
		{
			Name: "编辑字典类型", Slug: DictionaryTypeUpdatePermission, HTTPMethod: "PUT", HTTPPath: "/api/dictionaries/types/{id}",
			PageURI: "/kadmin/dictionary", PageTitle: "字典管理", Scope: PermissionScopeButton, Button: "dictionary.types.update",
		},
		{
			Name: "删除字典类型", Slug: DictionaryTypeDeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/dictionaries/types/{id}",
			PageURI: "/kadmin/dictionary", PageTitle: "字典管理", Scope: PermissionScopeButton, Button: "dictionary.types.delete",
		},
		{
			Name: "新增字典项", Slug: DictionaryDataCreatePermission, HTTPMethod: "POST", HTTPPath: "/api/dictionaries/data",
			PageURI: "/kadmin/dictionary", PageTitle: "字典管理", Scope: PermissionScopeButton, Button: "dictionary.data.create",
		},
		{
			Name: "编辑字典项", Slug: DictionaryDataUpdatePermission, HTTPMethod: "PUT", HTTPPath: "/api/dictionaries/data/{id}",
			PageURI: "/kadmin/dictionary", PageTitle: "字典管理", Scope: PermissionScopeButton, Button: "dictionary.data.update",
		},
		{
			Name: "删除字典项", Slug: DictionaryDataDeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/dictionaries/data/{id}",
			PageURI: "/kadmin/dictionary", PageTitle: "字典管理", Scope: PermissionScopeButton, Button: "dictionary.data.delete",
		},
		{
			Name: "查看参数配置", Slug: SystemConfigManagePermission,
			HTTPMethod: "GET", HTTPPath: "/api/system/config",
			PageURI: "/kadmin/settings", PageTitle: "参数配置", Scope: PermissionScopePage,
		},
		{
			Name: "更新参数配置", Slug: SystemConfigUpdatePermission, HTTPMethod: "PUT", HTTPPath: "/api/system/config",
			PageURI: "/kadmin/settings", PageTitle: "参数配置", Scope: PermissionScopeButton, Button: "settings.update",
		},
		{
			Name: "查看请求日志", Slug: LogListPermission, HTTPMethod: "GET", HTTPPath: "/api/logs*",
			PageURI: "/kadmin/logs", PageTitle: "日志管理", Scope: PermissionScopePage,
		},
		{
			Name: "删除请求日志", Slug: LogDeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/logs*",
			PageURI: "/kadmin/logs", PageTitle: "日志管理", Scope: PermissionScopeButton, Button: "logs.delete",
		},
		{
			Name: "查看登录审计", Slug: loginlogs.ListPermission, HTTPMethod: "GET", HTTPPath: "/api/login-audits*",
			PageURI: "/kadmin/login-audits", PageTitle: "登录审计", Scope: PermissionScopePage,
		},
		{
			Name: "清理登录审计", Slug: loginlogs.DeletePermission, HTTPMethod: "DELETE,POST", HTTPPath: "/api/login-audits*",
			PageURI: "/kadmin/login-audits", PageTitle: "登录审计", Scope: PermissionScopeButton, Button: "login-audits.delete",
		},
		{
			Name: "设置登录审计保留周期", Slug: loginlogs.RetentionPermission, HTTPMethod: "PATCH", HTTPPath: "/api/login-audits/retention",
			PageURI: "/kadmin/login-audits", PageTitle: "登录审计", Scope: PermissionScopeButton, Button: "login-audits.retention",
		},
		{
			Name: "上传文件", Slug: files.UploadPermission, HTTPMethod: "POST", HTTPPath: "/api/files",
			PageURI: "/kadmin/resources", PageTitle: "资源工作台", Scope: PermissionScopeButton, Button: "resources.upload",
		},
		{
			Name: "读取文件", Slug: files.ReadPermission, HTTPMethod: "GET", HTTPPath: "/api/files*",
			PageURI: "/kadmin/resources", PageTitle: "资源工作台", Scope: PermissionScopePage,
		},
		{
			Name: "删除文件", Slug: files.DeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/files/{id}",
			PageURI: "/kadmin/resources", PageTitle: "资源工作台", Scope: PermissionScopeButton, Button: "resources.delete",
		},
		{
			Name: "上传用户头像", Slug: files.UserAvatarPermission, HTTPMethod: "POST", HTTPPath: "/api/users/avatar",
			PageURI: "/kadmin/users", PageTitle: "用户管理", Scope: PermissionScopeButton, Button: "users.avatar",
		},
		{
			Name: "查看定时任务", Slug: jobs.ListPermission, HTTPMethod: "GET", HTTPPath: "/api/jobs*",
			PageURI: "/kadmin/jobs", PageTitle: "定时任务", Scope: PermissionScopePage,
		},
		{
			Name: "创建定时任务", Slug: jobs.CreatePermission, HTTPMethod: "POST", HTTPPath: "/api/jobs",
			PageURI: "/kadmin/jobs", PageTitle: "定时任务", Scope: PermissionScopeButton, Button: "jobs.create",
		},
		{
			Name: "修改定时任务", Slug: jobs.UpdatePermission, HTTPMethod: "PUT,PATCH", HTTPPath: "/api/jobs/{id}*",
			PageURI: "/kadmin/jobs", PageTitle: "定时任务", Scope: PermissionScopeButton, Button: "jobs.update",
		},
		{
			Name: "删除定时任务", Slug: jobs.DeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/jobs/{id}",
			PageURI: "/kadmin/jobs", PageTitle: "定时任务", Scope: PermissionScopeButton, Button: "jobs.delete",
		},
		{
			Name: "立即执行任务", Slug: jobs.RunPermission, HTTPMethod: "POST", HTTPPath: "/api/jobs/{id}/run",
			PageURI: "/kadmin/jobs", PageTitle: "定时任务", Scope: PermissionScopeButton, Button: "jobs.run",
		},
		{
			Name: "查看任务日志", Slug: jobs.LogListPermission, HTTPMethod: "GET", HTTPPath: "/api/job-logs*",
			PageURI: "/kadmin/jobs", PageTitle: "定时任务", Scope: PermissionScopeButton, Button: "jobs.logs",
		},
		{
			Name: "查看系统监控", Slug: monitor.ViewPermission, HTTPMethod: "GET", HTTPPath: "/api/system-monitor",
			PageURI: "/kadmin/monitor", PageTitle: "系统监控", Scope: PermissionScopePage,
		},
		{
			Name: "启停系统监控", Slug: monitor.UpdatePermission, HTTPMethod: "PATCH", HTTPPath: "/api/system-monitor/status",
			PageURI: "/kadmin/monitor", PageTitle: "系统监控", Scope: PermissionScopeButton, Button: "monitor.update",
		},
		{
			Name: "查看接口负载排行", Slug: loadrank.ViewPermission, HTTPMethod: "GET", HTTPPath: "/api/load-ranking*",
			PageURI: "/kadmin/load-ranking", PageTitle: "接口负载排行", Scope: PermissionScopePage,
		},
		{
			Name: "启停接口采样", Slug: loadrank.UpdatePermission, HTTPMethod: "PATCH", HTTPPath: "/api/load-ranking/status",
			PageURI: "/kadmin/load-ranking", PageTitle: "接口负载排行", Scope: PermissionScopeButton, Button: "load-ranking.update",
		},
		{
			Name: "查看代码生成", Slug: codegen.ListPermission, HTTPMethod: "GET", HTTPPath: "/api/codegen*",
			PageURI: "/kadmin/codegen", PageTitle: "代码生成", Scope: PermissionScopePage,
		},
		{
			Name: "导入与配置代码生成", Slug: codegen.ImportPermission, HTTPMethod: "POST,PUT,DELETE", HTTPPath: "/api/codegen*",
			PageURI: "/kadmin/codegen", PageTitle: "代码生成", Scope: PermissionScopeButton, Button: "codegen.import",
		},
		{
			Name: "预览与生成代码", Slug: codegen.GeneratePermission, HTTPMethod: "POST,GET", HTTPPath: "/api/codegen/configs/{id}/*",
			PageURI: "/kadmin/codegen", PageTitle: "代码生成", Scope: PermissionScopeButton, Button: "codegen.generate",
		},
		{
			Name: "查看站内通知", Slug: notifications.ListPermission, HTTPMethod: "GET", HTTPPath: "/api/notifications",
			PageURI: "/kadmin/notifications", PageTitle: "站内通知", Scope: PermissionScopePage,
		},
		{
			Name: "发送站内通知", Slug: notifications.CreatePermission, HTTPMethod: "POST", HTTPPath: "/api/notifications",
			PageURI: "/kadmin/notifications", PageTitle: "站内通知", Scope: PermissionScopeButton, Button: "notifications.create",
		},
		{
			Name: "标记通知已读", Slug: notifications.ReadPermission, HTTPMethod: "PATCH", HTTPPath: "/api/notifications/{id}/read",
			PageURI: "/kadmin/notifications", PageTitle: "站内通知", Scope: PermissionScopeButton, Button: "notifications.read",
		},
		{
			Name: "删除通知", Slug: notifications.DeletePermission, HTTPMethod: "DELETE", HTTPPath: "/api/notifications/{id}",
			PageURI: "/kadmin/notifications", PageTitle: "站内通知", Scope: PermissionScopeButton, Button: "notifications.delete",
		},
		{
			Name: "全部标记已读", Slug: notifications.ReadAllPermission, HTTPMethod: "PATCH", HTTPPath: "/api/notification-batch/read-all",
			PageURI: "/kadmin/notifications", PageTitle: "站内通知", Scope: PermissionScopeButton, Button: "notifications.read-all",
		},
		{
			Name: "清空已读通知", Slug: notifications.ClearReadPermission, HTTPMethod: "DELETE", HTTPPath: "/api/notification-batch/read",
			PageURI: "/kadmin/notifications", PageTitle: "站内通知", Scope: PermissionScopeButton, Button: "notifications.clear-read",
		},
	}
}
