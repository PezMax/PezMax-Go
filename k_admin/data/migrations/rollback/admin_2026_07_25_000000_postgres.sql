-- Rollback for admin_2026_07_25_000000_postgres.sql.
-- Executing this file permanently discards KAdmin file metadata. Stored
-- objects are not deleted and must be handled separately.

BEGIN;

DELETE FROM public.goadmin_permissions
WHERE slug IN ('system:file:upload', 'system:file:read', 'system:file:delete');

DROP TABLE IF EXISTS public.kadmin_files;

COMMIT;
