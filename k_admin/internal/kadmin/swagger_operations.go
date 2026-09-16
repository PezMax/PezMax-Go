package kadmin

// This file keeps the generated API contract close to the KAdmin route registry.
// The functions are documentation anchors consumed by swag and are not handlers.

// swaggerCaptcha documents GET /auth/captcha.
// @Summary 获取一次性登录验证码
// @Tags 认证
// @Success 200 {object} SwaggerResponse
// @Failure 503 {object} SwaggerErrorResponse
// @Router /auth/captcha [get]
func swaggerCaptcha() {}

// swaggerLogin documents POST /auth/login.
// @Summary 登录
// @Tags 认证
// @Param payload body loginRequest true "登录信息"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /auth/login [post]
func swaggerLogin() {}

// swaggerRefreshToken documents POST /auth/refresh.
// @Summary 刷新令牌
// @Tags 认证
// @Param payload body refreshTokenRequest true "刷新令牌"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /auth/refresh [post]
func swaggerRefreshToken() {}

// swaggerLogout documents POST /auth/logout.
// @Summary 退出登录
// @Tags 认证
// @Security BearerAuth
// @Param payload body refreshTokenRequest false "刷新令牌"
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /auth/logout [post]
func swaggerLogout() {}

// swaggerAccessCodes documents GET /auth/codes.
// @Summary 获取权限码
// @Tags 认证
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /auth/codes [get]
func swaggerAccessCodes() {}

// swaggerUserInfo documents GET /user/info.
// @Summary 获取当前用户信息
// @Tags 当前用户
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /user/info [get]
func swaggerUserInfo() {}

// swaggerUserMenu documents GET /user/menu.
// @Summary 获取当前用户菜单
// @Tags 当前用户
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /user/menu [get]
func swaggerUserMenu() {}

// swaggerUserPermissions documents GET /user/permissions.
// @Summary 获取当前用户权限
// @Tags 当前用户
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /user/permissions [get]
func swaggerUserPermissions() {}

// swaggerAllMenus documents GET /menu/all.
// @Summary 获取全部可访问菜单
// @Tags 菜单
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Router /menu/all [get]
func swaggerAllMenus() {}

// swaggerAdminMenus documents GET /admin-menus.
// @Summary 获取菜单管理列表
// @Tags 菜单管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /admin-menus [get]
func swaggerAdminMenus() {}

// swaggerAdminMenuTree documents GET /admin-menus/tree.
// @Summary 获取菜单管理树
// @Tags 菜单管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /admin-menus/tree [get]
func swaggerAdminMenuTree() {}

// swaggerCreateAdminMenu documents POST /admin-menus.
// @Summary 新增菜单
// @Tags 菜单管理
// @Security BearerAuth
// @Param payload body menuPayload true "菜单信息"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /admin-menus [post]
func swaggerCreateAdminMenu() {}

// swaggerUpdateAdminMenuLayout documents PUT /admin-menus.
// @Summary 更新菜单布局
// @Tags 菜单管理
// @Security BearerAuth
// @Param payload body menuLayoutPayload true "菜单位置列表"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /admin-menus [put]
func swaggerUpdateAdminMenuLayout() {}

// swaggerUpdateAdminMenu documents PUT /admin-menus/{id}.
// @Summary 修改菜单
// @Tags 菜单管理
// @Security BearerAuth
// @Param id path int true "菜单 ID"
// @Param payload body menuPayload true "菜单信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /admin-menus/{id} [put]
func swaggerUpdateAdminMenu() {}

// swaggerDeleteAdminMenu documents DELETE /admin-menus/{id}.
// @Summary 删除菜单
// @Tags 菜单管理
// @Security BearerAuth
// @Param id path int true "菜单 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /admin-menus/{id} [delete]
func swaggerDeleteAdminMenu() {}

// swaggerListUsers documents GET /users.
// @Summary 查询用户列表
// @Tags 用户管理
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键词"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /users [get]
func swaggerListUsers() {}

// swaggerUserOptions documents GET /users/options.
// @Summary 获取用户表单中的职位与部门选项
// @Tags 用户管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 401 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /users/options [get]
func swaggerUserOptions() {}

// swaggerCreateUser documents POST /users.
// @Summary 新增用户
// @Tags 用户管理
// @Security BearerAuth
// @Param payload body userPayload true "用户信息"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /users [post]
func swaggerCreateUser() {}

// swaggerExportUsers documents GET /users/export.
// @Summary 导出用户
// @Tags 用户管理
// @Security BearerAuth
// @Success 200 {file} file
// @Failure 403 {object} SwaggerErrorResponse
// @Router /users/export [get]
func swaggerExportUsers() {}

// swaggerImportUsers documents POST /users/import.
// @Summary 导入用户
// @Tags 用户管理
// @Security BearerAuth
// @Accept application/json
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Param payload body importUsersPayload true "导入格式与文件内容"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /users/import [post]
func swaggerImportUsers() {}

// swaggerUpdateUserStatus documents PUT /users/{id}/status.
// @Summary 修改用户状态
// @Tags 用户管理
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param payload body userStatusPayload true "用户状态"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /users/{id}/status [put]
func swaggerUpdateUserStatus() {}

// swaggerUpdateUser documents PUT /users/{id}.
// @Summary 修改用户
// @Tags 用户管理
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param payload body userPayload true "用户信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /users/{id} [put]
func swaggerUpdateUser() {}

// swaggerDeleteUser documents DELETE /users/{id}.
// @Summary 删除用户
// @Tags 用户管理
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /users/{id} [delete]
func swaggerDeleteUser() {}

// swaggerResetUserPassword documents PUT /users/{id}/password.
// @Summary 重置用户密码
// @Tags 用户管理
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param payload body resetPasswordPayload true "新密码"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /users/{id}/password [put]
func swaggerResetUserPassword() {}

// swaggerUnlockUser documents PUT /users/{id}/unlock.
// @Summary 清除用户临时登录锁定
// @Tags 用户管理
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param payload body unlockUserPayload false "可选 IP"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 503 {object} SwaggerErrorResponse
// @Router /users/{id}/unlock [put]
func swaggerUnlockUser() {}

// swaggerPublicLoginConfig documents GET /system/config/login.
// @Summary 获取公开登录配置
// @Tags 系统配置
// @Success 200 {object} SwaggerResponse
// @Router /system/config/login [get]
func swaggerPublicLoginConfig() {}

// swaggerSystemConfig documents GET /system/config.
// @Summary 获取系统配置
// @Tags 系统配置
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /system/config [get]
func swaggerSystemConfig() {}

// swaggerUpdateSystemConfig documents PUT /system/config.
// @Summary 修改系统配置
// @Tags 系统配置
// @Security BearerAuth
// @Param payload body systemConfigPayload true "系统配置"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /system/config [put]
func swaggerUpdateSystemConfig() {}

// swaggerRBACOverview documents GET /rbac/overview.
// @Summary 获取权限管理概览
// @Tags 权限管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/overview [get]
func swaggerRBACOverview() {}

// swaggerRBACDepartments documents GET /rbac/departments.
// @Summary 获取部门列表
// @Tags 权限管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/departments [get]
func swaggerRBACDepartments() {}

// swaggerCreateDepartment documents POST /rbac/departments.
// @Summary 新增部门
// @Tags 权限管理
// @Security BearerAuth
// @Param payload body departmentPayload true "部门信息"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /rbac/departments [post]
func swaggerCreateDepartment() {}

// swaggerUpdateDepartment documents PUT /rbac/departments/{id}.
// @Summary 修改部门
// @Tags 权限管理
// @Security BearerAuth
// @Param id path int true "部门 ID"
// @Param payload body departmentPayload true "部门信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/departments/{id} [put]
func swaggerUpdateDepartment() {}

// swaggerDeleteDepartment documents DELETE /rbac/departments/{id}.
// @Summary 删除部门
// @Tags 权限管理
// @Security BearerAuth
// @Param id path int true "部门 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/departments/{id} [delete]
func swaggerDeleteDepartment() {}

// swaggerUpdateDepartmentRoles documents PUT /rbac/departments/{id}/roles.
// @Summary 设置部门角色
// @Tags 权限管理
// @Security BearerAuth
// @Param id path int true "部门 ID"
// @Param payload body departmentRolesPayload true "角色 ID 列表"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/departments/{id}/roles [put]
func swaggerUpdateDepartmentRoles() {}

// swaggerRBACRoles documents GET /rbac/roles.
// @Summary 获取角色列表
// @Tags 权限管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/roles [get]
func swaggerRBACRoles() {}

// swaggerCreateRole documents POST /rbac/roles.
// @Summary 新增角色
// @Tags 权限管理
// @Security BearerAuth
// @Param payload body rolePayload true "角色信息"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /rbac/roles [post]
func swaggerCreateRole() {}

// swaggerUpdateRole documents PUT /rbac/roles/{id}.
// @Summary 修改角色
// @Tags 权限管理
// @Security BearerAuth
// @Param id path int true "角色 ID"
// @Param payload body rolePayload true "角色信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/roles/{id} [put]
func swaggerUpdateRole() {}

// swaggerDeleteRole documents DELETE /rbac/roles/{id}.
// @Summary 删除角色
// @Tags 权限管理
// @Security BearerAuth
// @Param id path int true "角色 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/roles/{id} [delete]
func swaggerDeleteRole() {}

// swaggerUpdateRoleMenus documents PUT /rbac/roles/{id}/menus.
// @Summary 设置角色菜单
// @Tags 权限管理
// @Security BearerAuth
// @Param id path int true "角色 ID"
// @Param payload body roleMenuPayload true "菜单 ID 列表"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/roles/{id}/menus [put]
func swaggerUpdateRoleMenus() {}

// swaggerUpdateRolePermissions documents PUT /rbac/roles/{id}/permissions.
// @Summary 设置角色权限标识
// @Tags 权限管理
// @Security BearerAuth
// @Param id path int true "角色 ID"
// @Param payload body rolePermissionsPayload true "权限标识 ID 列表"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/roles/{id}/permissions [put]
func swaggerUpdateRolePermissions() {}

// swaggerUpdateRoleUsers documents PUT /rbac/roles/{id}/users.
// @Summary 设置角色用户
// @Tags 权限管理
// @Security BearerAuth
// @Param id path int true "角色 ID"
// @Param payload body roleUsersPayload true "用户 ID 列表"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/roles/{id}/users [put]
func swaggerUpdateRoleUsers() {}

// swaggerRBACMenus documents GET /rbac/menus.
// @Summary 获取授权菜单
// @Tags 权限管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/menus [get]
func swaggerRBACMenus() {}

// swaggerRBACUsers documents GET /rbac/users.
// @Summary 获取授权用户
// @Tags 权限管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /rbac/users [get]
func swaggerRBACUsers() {}

// swaggerDictionaryOverview documents GET /dictionaries/overview.
// @Summary 获取字典概览
// @Tags 字典管理
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /dictionaries/overview [get]
func swaggerDictionaryOverview() {}

// swaggerDictionaryTypes documents GET /dictionaries/types.
// @Summary 查询字典类型
// @Tags 字典管理
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键词"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /dictionaries/types [get]
func swaggerDictionaryTypes() {}

// swaggerCreateDictionaryType documents POST /dictionaries/types.
// @Summary 新增字典类型
// @Tags 字典管理
// @Security BearerAuth
// @Param payload body dictionaryTypePayload true "字典类型"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /dictionaries/types [post]
func swaggerCreateDictionaryType() {}

// swaggerUpdateDictionaryType documents PUT /dictionaries/types/{id}.
// @Summary 修改字典类型
// @Tags 字典管理
// @Security BearerAuth
// @Param id path int true "字典类型 ID"
// @Param payload body dictionaryTypePayload true "字典类型"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /dictionaries/types/{id} [put]
func swaggerUpdateDictionaryType() {}

// swaggerDeleteDictionaryType documents DELETE /dictionaries/types/{id}.
// @Summary 删除字典类型
// @Tags 字典管理
// @Security BearerAuth
// @Param id path int true "字典类型 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /dictionaries/types/{id} [delete]
func swaggerDeleteDictionaryType() {}

// swaggerDictionaryData documents GET /dictionaries/data.
// @Summary 查询字典数据
// @Tags 字典管理
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param typeId query int false "字典类型 ID"
// @Param keyword query string false "关键词"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /dictionaries/data [get]
func swaggerDictionaryData() {}

// swaggerCreateDictionaryData documents POST /dictionaries/data.
// @Summary 新增字典数据
// @Tags 字典管理
// @Security BearerAuth
// @Param payload body dictionaryDataPayload true "字典数据"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /dictionaries/data [post]
func swaggerCreateDictionaryData() {}

// swaggerUpdateDictionaryData documents PUT /dictionaries/data/{id}.
// @Summary 修改字典数据
// @Tags 字典管理
// @Security BearerAuth
// @Param id path int true "字典数据 ID"
// @Param payload body dictionaryDataPayload true "字典数据"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /dictionaries/data/{id} [put]
func swaggerUpdateDictionaryData() {}

// swaggerDeleteDictionaryData documents DELETE /dictionaries/data/{id}.
// @Summary 删除字典数据
// @Tags 字典管理
// @Security BearerAuth
// @Param id path int true "字典数据 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /dictionaries/data/{id} [delete]
func swaggerDeleteDictionaryData() {}

// swaggerListLogs documents GET /logs.
// @Summary 查询请求日志
// @Tags 日志管理
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键词"
// @Param eventType query string false "事件类型"
// @Param level query string false "日志级别"
// @Param success query bool false "是否成功"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /logs [get]
func swaggerListLogs() {}

// swaggerLogDetail documents GET /logs/{id}.
// @Summary 获取请求日志详情
// @Tags 日志管理
// @Security BearerAuth
// @Param id path int true "日志 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /logs/{id} [get]
func swaggerLogDetail() {}

// swaggerDeleteLogs documents DELETE /logs.
// @Summary 批量删除请求日志
// @Tags 日志管理
// @Security BearerAuth
// @Param payload body deleteLogsPayload true "日志 ID 列表或清理条件"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /logs [delete]
func swaggerDeleteLogs() {}

// swaggerDeleteLog documents DELETE /logs/{id}.
// @Summary 删除请求日志
// @Tags 日志管理
// @Security BearerAuth
// @Param id path int true "日志 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /logs/{id} [delete]
func swaggerDeleteLog() {}

// swaggerListJobs documents GET /jobs.
// @Summary 查询定时任务
// @Tags 定时任务
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键词"
// @Param handler query string false "处理器"
// @Param status query string false "状态" Enums(enabled,paused)
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /jobs [get]
func swaggerListJobs() {}

// swaggerGetJob documents GET /jobs/{id}.
// @Summary 获取定时任务详情
// @Tags 定时任务
// @Security BearerAuth
// @Param id path int true "任务 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /jobs/{id} [get]
func swaggerGetJob() {}

// swaggerCreateJob documents POST /jobs.
// @Summary 新增定时任务
// @Tags 定时任务
// @Security BearerAuth
// @Param payload body SwaggerJobPayload true "任务信息"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /jobs [post]
func swaggerCreateJob() {}

// swaggerUpdateJob documents PUT /jobs/{id}.
// @Summary 修改定时任务
// @Tags 定时任务
// @Security BearerAuth
// @Param id path int true "任务 ID"
// @Param payload body SwaggerJobPayload true "任务信息"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /jobs/{id} [put]
func swaggerUpdateJob() {}

// swaggerSetJobStatus documents PATCH /jobs/{id}/status.
// @Summary 暂停或恢复定时任务
// @Tags 定时任务
// @Security BearerAuth
// @Param id path int true "任务 ID"
// @Param payload body object true "任务状态，status 为 enabled 或 paused"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /jobs/{id}/status [patch]
func swaggerSetJobStatus() {}

// swaggerDeleteJob documents DELETE /jobs/{id}.
// @Summary 删除定时任务
// @Tags 定时任务
// @Security BearerAuth
// @Param id path int true "任务 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /jobs/{id} [delete]
func swaggerDeleteJob() {}

// swaggerRunJob documents POST /jobs/{id}/run.
// @Summary 立即执行定时任务
// @Tags 定时任务
// @Security BearerAuth
// @Param id path int true "任务 ID"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /jobs/{id}/run [post]
func swaggerRunJob() {}

// swaggerListJobLogs documents GET /job-logs.
// @Summary 查询任务执行日志
// @Tags 定时任务
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param jobId query int false "任务 ID"
// @Param keyword query string false "关键词"
// @Param status query string false "执行状态" Enums(running,success,failed)
// @Param trigger query string false "触发方式" Enums(manual,scheduled)
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /job-logs [get]
func swaggerListJobLogs() {}

// swaggerGetJobLog documents GET /job-logs/{id}.
// @Summary 获取任务执行日志详情
// @Tags 定时任务
// @Security BearerAuth
// @Param id path int true "执行日志 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /job-logs/{id} [get]
func swaggerGetJobLog() {}

// swaggerSystemMonitor documents GET /system-monitor.
// @Summary 获取系统监控状态
// @Tags 系统监控
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /system-monitor [get]
func swaggerSystemMonitor() {}

// swaggerUpdateSystemMonitorStatus documents PATCH /system-monitor/status.
// @Summary 启用或关闭系统监控
// @Tags 系统监控
// @Security BearerAuth
// @Param payload body object true "监控开关，enabled 为布尔值"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /system-monitor/status [patch]
func swaggerUpdateSystemMonitorStatus() {}

// swaggerLoadRankingStatus documents GET /load-ranking/status.
// @Summary 获取接口采样状态
// @Tags 接口负载排行
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /load-ranking/status [get]
func swaggerLoadRankingStatus() {}

// swaggerUpdateLoadRankingStatus documents PATCH /load-ranking/status.
// @Summary 启用或关闭接口采样
// @Tags 接口负载排行
// @Security BearerAuth
// @Param payload body object true "采样开关，enabled 为布尔值"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /load-ranking/status [patch]
func swaggerUpdateLoadRankingStatus() {}

// swaggerLoadRankings documents GET /load-ranking/rankings.
// @Summary 查询接口负载排行
// @Description 按接口、方法或状态码聚合 HTTP 指标，支持时间范围与统计维度排序。QPS 为请求量除以数据覆盖窗口秒数；错误率为 4xx/5xx 占比；平均耗时为耗时总和除以请求量。
// @Tags 接口负载排行
// @Security BearerAuth
// @Param startedAt query string false "开始时间（RFC3339），默认最近 1 小时"
// @Param endedAt query string false "结束时间（RFC3339），默认当前时间"
// @Param route query string false "按接口路径模糊筛选"
// @Param method query string false "按请求方法筛选"
// @Param statusCode query integer false "按状态码筛选（100-599）"
// @Param groupBy query string false "聚合维度：route / method / status"
// @Param dimension query string false "排序维度：requestCount / qps / errorRate / avgDurationMs"
// @Param order query string false "排序方向：asc / desc"
// @Param page query integer false "页码"
// @Param pageSize query integer false "每页条数（最大 100）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /load-ranking/rankings [get]
func swaggerLoadRankings() {}

// swaggerUploadManagedFile documents POST /files.
// @Summary 上传文件
// @Tags 文件管理
// @Security BearerAuth
// @Accept multipart/form-data
// @Param file formData file true "文件"
// @Param purpose formData string false "用途"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /files [post]
func swaggerUploadManagedFile() {}

// swaggerManagedFileMetadata documents GET /files/{id}.
// @Summary 获取文件元数据
// @Tags 文件管理
// @Security BearerAuth
// @Param id path int true "文件 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /files/{id} [get]
func swaggerManagedFileMetadata() {}

// swaggerManagedFileContent documents GET /files/{id}/content.
// @Summary 下载文件内容
// @Tags 文件管理
// @Security BearerAuth
// @Produce application/octet-stream
// @Param id path int true "文件 ID"
// @Success 200 {file} file
// @Failure 404 {object} SwaggerErrorResponse
// @Router /files/{id}/content [get]
func swaggerManagedFileContent() {}

// swaggerDeleteManagedFile documents DELETE /files/{id}.
// @Summary 删除文件
// @Tags 文件管理
// @Security BearerAuth
// @Param id path int true "文件 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /files/{id} [delete]
func swaggerDeleteManagedFile() {}

// swaggerListLoginAudits documents GET /login-audits.
// @Summary 查询登录审计记录
// @Tags 登录审计
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param account query string false "账号"
// @Param ip query string false "IP 地址"
// @Param status query string false "登录状态" Enums(success,failed)
// @Param result query string false "登录结果" Enums(success,account_not_found,invalid_password,account_disabled,account_locked,account_unlocked,captcha_invalid,system_error)
// @Param startedAt query string false "开始时间，RFC3339"
// @Param endedAt query string false "结束时间，RFC3339"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /login-audits [get]
func swaggerListLoginAudits() {}

// swaggerDeleteLoginAudits documents DELETE /login-audits.
// @Summary 批量删除登录审计记录
// @Tags 登录审计
// @Security BearerAuth
// @Param payload body object true "审计记录 ID 列表"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /login-audits [delete]
func swaggerDeleteLoginAudits() {}

// swaggerCleanupLoginAudits documents POST /login-audits/cleanup.
// @Summary 清理超过保留周期的登录审计记录
// @Tags 登录审计
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /login-audits/cleanup [post]
func swaggerCleanupLoginAudits() {}

// swaggerLoginAuditRetention documents GET /login-audits/retention.
// @Summary 获取登录审计保留周期
// @Tags 登录审计
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /login-audits/retention [get]
func swaggerLoginAuditRetention() {}

// swaggerUpdateLoginAuditRetention documents PATCH /login-audits/retention.
// @Summary 设置登录审计保留周期
// @Tags 登录审计
// @Security BearerAuth
// @Param payload body object true "保留天数，范围 1 至 3650"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /login-audits/retention [patch]
func swaggerUpdateLoginAuditRetention() {}

// swaggerUploadedFile documents GET /uploads/{path}.
// @Summary 读取公开上传文件
// @Tags 文件管理
// @Param path path string true "对象路径"
// @Success 200 {file} file
// @Failure 404 {object} SwaggerErrorResponse
// @Router /uploads/{path} [get]
func swaggerUploadedFile() {}

// swaggerUploadAvatar documents POST /users/avatar.
// @Summary 上传用户头像
// @Tags 用户管理
// @Security BearerAuth
// @Accept multipart/form-data
// @Param file formData file true "头像文件"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /users/avatar [post]
func swaggerUploadAvatar() {}

// swaggerCodegenCandidates documents GET /codegen/candidates.
// @Summary 查询可导入的数据库表
// @Tags 代码生成
// @Security BearerAuth
// @Param keyword query string false "表名关键词"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /codegen/candidates [get]
func swaggerCodegenCandidates() {}

// swaggerCodegenTables documents GET /codegen/tables.
// @Summary 查询代码生成配置列表
// @Tags 代码生成
// @Security BearerAuth
// @Param keyword query string false "表名或业务名关键词"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /codegen/tables [get]
func swaggerCodegenTables() {}

// swaggerCodegenImportTable documents POST /codegen/tables/import.
// @Summary 导入数据库表
// @Tags 代码生成
// @Security BearerAuth
// @Param payload body object true "表名与可选生成配置"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /codegen/tables/import [post]
func swaggerCodegenImportTable() {}

// swaggerCodegenGetTable documents GET /codegen/configs/{id}.
// @Summary 获取代码生成配置详情
// @Tags 代码生成
// @Security BearerAuth
// @Param id path int true "配置 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /codegen/configs/{id} [get]
func swaggerCodegenGetTable() {}

// swaggerCodegenUpdateTable documents PUT /codegen/configs/{id}.
// @Summary 修改代码生成配置
// @Tags 代码生成
// @Security BearerAuth
// @Param id path int true "配置 ID"
// @Param payload body object true "模块名、类名、业务名、路由前缀与列配置"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /codegen/configs/{id} [put]
func swaggerCodegenUpdateTable() {}

// swaggerCodegenDeleteTable documents DELETE /codegen/configs/{id}.
// @Summary 删除代码生成配置
// @Tags 代码生成
// @Security BearerAuth
// @Param id path int true "配置 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /codegen/configs/{id} [delete]
func swaggerCodegenDeleteTable() {}

// swaggerCodegenPreview documents POST /codegen/configs/{id}/preview.
// @Summary 预览生成的代码文件
// @Tags 代码生成
// @Security BearerAuth
// @Param id path int true "配置 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /codegen/configs/{id}/preview [post]
func swaggerCodegenPreview() {}

// swaggerCodegenGenerate documents POST /codegen/configs/{id}/generate.
// @Summary 生成代码到项目目录
// @Tags 代码生成
// @Security BearerAuth
// @Param id path int true "配置 ID"
// @Param payload body object false "confirmOverwrite 是否覆盖无生成标记的冲突文件"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Failure 409 {object} SwaggerErrorResponse
// @Router /codegen/configs/{id}/generate [post]
func swaggerCodegenGenerate() {}

// swaggerCodegenDownload documents GET /codegen/configs/{id}/download.
// @Summary 打包下载生成的代码
// @Tags 代码生成
// @Security BearerAuth
// @Param id path int true "配置 ID"
// @Success 200 {file} file
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /codegen/configs/{id}/download [get]
func swaggerCodegenDownload() {}

// swaggerListNotifications documents GET /notifications.
// @Summary 查询站内通知
// @Tags 站内通知
// @Security BearerAuth
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param unreadOnly query boolean false "仅未读"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /notifications [get]
func swaggerListNotifications() {}

// swaggerCreateNotification documents POST /notifications.
// @Summary 发送站内通知
// @Tags 站内通知
// @Security BearerAuth
// @Param payload body object true "标题、内容、跳转链接与类型（info/success/warning）"
// @Param Idempotency-Key header string true "幂等键（8-128 位）"
// @Success 200 {object} SwaggerResponse
// @Failure 400 {object} SwaggerErrorResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /notifications [post]
func swaggerCreateNotification() {}

// swaggerMarkNotificationRead documents PATCH /notifications/{id}/read.
// @Summary 标记通知已读
// @Tags 站内通知
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /notifications/{id}/read [patch]
func swaggerMarkNotificationRead() {}

// swaggerDeleteNotification documents DELETE /notifications/{id}.
// @Summary 删除站内通知
// @Tags 站内通知
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Failure 404 {object} SwaggerErrorResponse
// @Router /notifications/{id} [delete]
func swaggerDeleteNotification() {}

// swaggerMarkAllNotificationsRead documents PATCH /notification-batch/read-all.
// @Summary 全部标记已读
// @Tags 站内通知
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /notification-batch/read-all [patch]
func swaggerMarkAllNotificationsRead() {}

// swaggerClearReadNotifications documents DELETE /notification-batch/read.
// @Summary 清空已读通知
// @Tags 站内通知
// @Security BearerAuth
// @Success 200 {object} SwaggerResponse
// @Failure 403 {object} SwaggerErrorResponse
// @Router /notification-batch/read [delete]
func swaggerClearReadNotifications() {}
