-- Add KAdmin system monitoring settings, permissions, and menu.
-- This migration is idempotent so it can be used against an existing database.

BEGIN;

CREATE TABLE IF NOT EXISTS public.kadmin_monitor_settings (
    id smallint PRIMARY KEY,
    enabled boolean NOT NULL DEFAULT FALSE,
    updated_by bigint NOT NULL DEFAULT 0,
    updated_at timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT kadmin_monitor_settings_singleton_check CHECK (id = 1)
);

INSERT INTO public.kadmin_monitor_settings (id, enabled, updated_by)
VALUES (1, FALSE, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO public.goadmin_permissions
    (name, slug, http_method, http_path, created_at, updated_at)
SELECT seed.name, seed.slug, seed.http_method, seed.http_path, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM (VALUES
    ('查看系统监控', 'system:monitor:view', 'GET', '/api/system-monitor'),
    ('启停系统监控', 'system:monitor:update', 'PATCH', '/api/system-monitor/status')
) AS seed(name, slug, http_method, http_path)
WHERE NOT EXISTS (
    SELECT 1 FROM public.goadmin_permissions permission WHERE permission.slug = seed.slug
);

INSERT INTO public.goadmin_menu
    (parent_id, type, "order", title, icon, uri, plugin_name, created_at, updated_at)
SELECT parent.id, 1, 9, '系统监控', 'lucide:monitor-cog', '/kadmin/monitor', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM public.goadmin_menu parent
WHERE parent.uri = '/kadmin'
  AND NOT EXISTS (SELECT 1 FROM public.goadmin_menu menu WHERE menu.uri = '/kadmin/monitor')
LIMIT 1;

INSERT INTO public.goadmin_role_menu (role_id, menu_id, created_at, updated_at)
SELECT role.id, menu.id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM public.goadmin_roles role
JOIN public.goadmin_menu menu ON menu.uri = '/kadmin/monitor'
WHERE role.slug = 'administrator'
  AND NOT EXISTS (
      SELECT 1 FROM public.goadmin_role_menu binding
      WHERE binding.role_id = role.id AND binding.menu_id = menu.id
  );

COMMIT;
