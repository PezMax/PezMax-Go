-- Rollback for admin_2026_07_22_000000_postgres.sql.
-- Executing this file discards data written to the new listener columns.

BEGIN;

DROP INDEX IF EXISTS public.goadmin_operation_log_expires_at_index;
DROP INDEX IF EXISTS public.goadmin_operation_log_trace_id_index;
DROP INDEX IF EXISTS public.goadmin_operation_log_request_id_index;
DROP INDEX IF EXISTS public.goadmin_operation_log_failure_time_index;
DROP INDEX IF EXISTS public.goadmin_operation_log_user_time_index;
DROP INDEX IF EXISTS public.goadmin_operation_log_event_type_time_index;
DROP INDEX IF EXISTS public.goadmin_operation_log_occurred_at_index;
DROP INDEX IF EXISTS public.goadmin_operation_log_event_id_unique;

ALTER TABLE public.goadmin_operation_log
    DROP CONSTRAINT IF EXISTS goadmin_operation_log_expires_at_check,
    DROP CONSTRAINT IF EXISTS goadmin_operation_log_metadata_check,
    DROP CONSTRAINT IF EXISTS goadmin_operation_log_duration_ms_check,
    DROP CONSTRAINT IF EXISTS goadmin_operation_log_status_code_check,
    DROP CONSTRAINT IF EXISTS goadmin_operation_log_level_check,
    DROP CONSTRAINT IF EXISTS goadmin_operation_log_event_type_check,
    DROP COLUMN IF EXISTS expires_at,
    DROP COLUMN IF EXISTS occurred_at,
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS error_message,
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS duration_ms,
    DROP COLUMN IF EXISTS success,
    DROP COLUMN IF EXISTS status_code,
    DROP COLUMN IF EXISTS trace_id,
    DROP COLUMN IF EXISTS request_id,
    DROP COLUMN IF EXISTS actor_name,
    DROP COLUMN IF EXISTS message,
    DROP COLUMN IF EXISTS action,
    DROP COLUMN IF EXISTS module,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS level,
    DROP COLUMN IF EXISTS event_type,
    DROP COLUMN IF EXISTS event_id,
    ALTER COLUMN input DROP DEFAULT,
    ALTER COLUMN ip DROP DEFAULT,
    ALTER COLUMN ip TYPE character varying(15),
    ALTER COLUMN method DROP DEFAULT,
    ALTER COLUMN method TYPE character varying(10),
    ALTER COLUMN path DROP DEFAULT,
    ALTER COLUMN path TYPE character varying(255),
    ALTER COLUMN user_id SET NOT NULL,
    ALTER COLUMN id TYPE integer;

ALTER SEQUENCE public.goadmin_operation_log_myid_seq
    AS integer
    MAXVALUE 99999999;

COMMENT ON TABLE public.goadmin_operation_log IS NULL;

COMMIT;
