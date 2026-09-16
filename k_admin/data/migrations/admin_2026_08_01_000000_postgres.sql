-- Materialize legacy menu-derived API permissions for the explicit RBAC identifier model.
-- Page permissions remain inherited from menus at runtime; button permissions become
-- explicit role bindings after this idempotent migration.

BEGIN;

INSERT INTO public.goadmin_permissions
    (name, slug, http_method, http_path, created_at, updated_at)
SELECT seed.name, seed.slug, seed.http_method, seed.http_path,
       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM (VALUES
    ('用户管理', 'system:user:manage', 'GET,POST,PUT,DELETE', '/api/users*'),
    ('权限管理', 'system:rbac:manage', 'GET,POST,PUT,DELETE', '/api/rbac*'),
    ('菜单管理', 'system:menu:manage', 'GET,POST,PUT,DELETE', '/api/admin-menus*'),
    ('字典管理', 'system:dict:manage', 'GET,POST,PUT,DELETE', '/api/dictionaries*'),
    ('参数配置', 'system:config:manage', 'GET,PUT', '/api/system/config*'),
    ('查看请求日志', 'system:log:list', 'GET', '/api/logs*'),
    ('删除请求日志', 'system:log:delete', 'DELETE', '/api/logs*'),
    ('查看登录审计', 'system:login-log:list', 'GET', '/api/login-audits*'),
    ('清理登录审计', 'system:login-log:delete', 'DELETE,POST', '/api/login-audits*'),
    ('设置登录审计保留周期', 'system:login-log:retention', 'PATCH', '/api/login-audits/retention'),
    ('上传文件', 'system:file:upload', 'POST', '/api/files'),
    ('读取文件', 'system:file:read', 'GET', '/api/files*'),
    ('删除文件', 'system:file:delete', 'DELETE', '/api/files/*'),
    ('查看定时任务', 'system:job:list', 'GET', '/api/jobs*'),
    ('创建定时任务', 'system:job:create', 'POST', '/api/jobs'),
    ('修改定时任务', 'system:job:update', 'PUT,PATCH', '/api/jobs/*'),
    ('删除定时任务', 'system:job:delete', 'DELETE', '/api/jobs/*'),
    ('立即执行任务', 'system:job:run', 'POST', '/api/jobs/*/run'),
    ('查看任务日志', 'system:job-log:list', 'GET', '/api/job-logs*'),
    ('查看系统监控', 'system:monitor:view', 'GET', '/api/system-monitor'),
    ('启停系统监控', 'system:monitor:update', 'PATCH', '/api/system-monitor/status'),
    ('查看接口负载排行', 'system:load-rank:view', 'GET', '/api/load-ranking*'),
    ('启停接口采样', 'system:load-rank:update', 'PATCH', '/api/load-ranking/status')
) AS seed(name, slug, http_method, http_path)
WHERE NOT EXISTS (
    SELECT 1
    FROM public.goadmin_permissions permission
    WHERE permission.slug = seed.slug
);

WITH menu_permissions(menu_uri, permission_slug) AS (VALUES
    ('/kadmin/users', 'system:user:manage'),
    ('/kadmin/rbac', 'system:rbac:manage'),
    ('/kadmin/menus', 'system:menu:manage'),
    ('/kadmin/dictionary', 'system:dict:manage'),
    ('/kadmin/settings', 'system:config:manage'),
    ('/kadmin/logs', 'system:log:list'),
    ('/kadmin/logs', 'system:log:delete'),
    ('/kadmin/login-audits', 'system:login-log:list'),
    ('/kadmin/login-audits', 'system:login-log:delete'),
    ('/kadmin/login-audits', 'system:login-log:retention'),
    ('/kadmin/resources', 'system:file:read'),
    ('/kadmin/resources', 'system:file:upload'),
    ('/kadmin/resources', 'system:file:delete'),
    ('/kadmin/jobs', 'system:job:list'),
    ('/kadmin/jobs', 'system:job:create'),
    ('/kadmin/jobs', 'system:job:update'),
    ('/kadmin/jobs', 'system:job:delete'),
    ('/kadmin/jobs', 'system:job:run'),
    ('/kadmin/jobs', 'system:job-log:list'),
    ('/kadmin/monitor', 'system:monitor:view'),
    ('/kadmin/monitor', 'system:monitor:update'),
    ('/kadmin/load-ranking', 'system:load-rank:view'),
    ('/kadmin/load-ranking', 'system:load-rank:update')
)
INSERT INTO public.goadmin_role_permissions
    (role_id, permission_id, created_at, updated_at)
SELECT DISTINCT binding.role_id, permission.id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM public.goadmin_role_menu binding
JOIN public.goadmin_menu menu ON menu.id = binding.menu_id
JOIN menu_permissions mapping ON mapping.menu_uri = menu.uri
JOIN public.goadmin_permissions permission ON permission.slug = mapping.permission_slug
WHERE NOT EXISTS (
    SELECT 1
    FROM public.goadmin_role_permissions existing
    WHERE existing.role_id = binding.role_id
      AND existing.permission_id = permission.id
);

COMMIT;
