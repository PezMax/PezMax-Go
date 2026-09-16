-- Rollback for admin_2026_07_26_010000_postgres.sql.

BEGIN;

DELETE FROM public.goadmin_role_menu
WHERE menu_id IN (
    SELECT id FROM public.goadmin_menu WHERE uri = '/kadmin/monitor'
);

DELETE FROM public.goadmin_menu
WHERE uri = '/kadmin/monitor';

DELETE FROM public.goadmin_role_permissions
WHERE permission_id IN (
    SELECT id FROM public.goadmin_permissions
    WHERE slug IN ('system:monitor:view', 'system:monitor:update')
);

DELETE FROM public.goadmin_user_permissions
WHERE permission_id IN (
    SELECT id FROM public.goadmin_permissions
    WHERE slug IN ('system:monitor:view', 'system:monitor:update')
);

DELETE FROM public.goadmin_permissions
WHERE slug IN ('system:monitor:view', 'system:monitor:update');

DROP TABLE IF EXISTS public.kadmin_monitor_settings;

COMMIT;
