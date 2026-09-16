-- Revert explicit RBAC identifier bindings added by the 2026-08-01 migration.
-- Permission definitions are retained because runtime bootstrap and API middleware
-- still reference them; only the materialized role bindings are removed.

BEGIN;

DELETE FROM public.goadmin_role_permissions binding
USING public.goadmin_permissions permission
WHERE binding.permission_id = permission.id
  AND permission.slug IN (
    'system:user:manage',
    'system:rbac:manage',
    'system:menu:manage',
    'system:dict:manage',
    'system:config:manage',
    'system:log:list',
    'system:log:delete',
    'system:login-log:list',
    'system:login-log:delete',
    'system:login-log:retention',
    'system:file:upload',
    'system:file:read',
    'system:file:delete',
    'system:job:list',
    'system:job:create',
    'system:job:update',
    'system:job:delete',
    'system:job:run',
    'system:job-log:list',
    'system:monitor:view',
    'system:monitor:update',
    'system:load-rank:view',
    'system:load-rank:update'
  );

COMMIT;
