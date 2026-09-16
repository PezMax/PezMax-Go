-- Add the Vben request-log menu and API permissions.
-- This migration is idempotent so it can be used against an existing KAdmin database.

BEGIN;

INSERT INTO public.goadmin_permissions
    (name, slug, http_method, http_path, created_at, updated_at)
SELECT
    seed.name,
    seed.slug,
    seed.http_method,
    seed.http_path,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM (VALUES
    ('查看请求日志', 'system:log:list', 'GET', '/api/logs*'),
    ('删除请求日志', 'system:log:delete', 'DELETE', '/api/logs*')
) AS seed(name, slug, http_method, http_path)
WHERE NOT EXISTS (
    SELECT 1
    FROM public.goadmin_permissions permission
    WHERE permission.slug = seed.slug
);

INSERT INTO public.goadmin_menu
    (parent_id, type, "order", title, plugin_name, header, icon, uri, created_at, updated_at)
SELECT
    parent.id,
    1,
    7,
    '日志管理',
    '',
    NULL,
    'lucide:scroll-text',
    '/kadmin/logs',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM public.goadmin_menu parent
WHERE parent.uri = '/kadmin'
  AND NOT EXISTS (
      SELECT 1
      FROM public.goadmin_menu menu
      WHERE menu.uri = '/kadmin/logs'
  )
ORDER BY parent.id
LIMIT 1;

INSERT INTO public.goadmin_role_menu (role_id, menu_id, created_at, updated_at)
SELECT 1, menu.id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM public.goadmin_menu menu
WHERE menu.uri = '/kadmin/logs'
  AND NOT EXISTS (
      SELECT 1
      FROM public.goadmin_role_menu relation
      WHERE relation.role_id = 1
        AND relation.menu_id = menu.id
  );

COMMIT;
