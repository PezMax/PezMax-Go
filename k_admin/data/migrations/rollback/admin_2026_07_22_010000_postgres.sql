BEGIN;

DELETE FROM public.goadmin_role_menu
WHERE menu_id IN (
    SELECT id FROM public.goadmin_menu WHERE uri = '/kadmin/logs'
);

DELETE FROM public.goadmin_menu
WHERE uri = '/kadmin/logs';

DELETE FROM public.goadmin_role_permissions
WHERE permission_id IN (
    SELECT id
    FROM public.goadmin_permissions
    WHERE slug IN ('system:log:list', 'system:log:delete')
);

DELETE FROM public.goadmin_permissions
WHERE slug IN ('system:log:list', 'system:log:delete');

COMMIT;
