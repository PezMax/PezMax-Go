-- Rollback for admin_2026_07_31_000000_postgres.sql.

BEGIN;

DELETE FROM public.goadmin_role_menu
WHERE menu_id IN (
    SELECT id FROM public.goadmin_menu WHERE uri = '/kadmin/load-ranking'
);

DELETE FROM public.goadmin_menu
WHERE uri = '/kadmin/load-ranking';

DELETE FROM public.goadmin_role_permissions
WHERE permission_id IN (
    SELECT id FROM public.goadmin_permissions
    WHERE slug IN ('system:load-rank:view', 'system:load-rank:update')
);

DELETE FROM public.goadmin_user_permissions
WHERE permission_id IN (
    SELECT id FROM public.goadmin_permissions
    WHERE slug IN ('system:load-rank:view', 'system:load-rank:update')
);

DELETE FROM public.goadmin_permissions
WHERE slug IN ('system:load-rank:view', 'system:load-rank:update');

DROP TABLE IF EXISTS public.kadmin_http_metric_buckets;

DROP TABLE IF EXISTS public.kadmin_loadrank_settings;

COMMIT;
