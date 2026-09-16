-- This migration extends the existing GoAdmin operation log table so that a
-- Vben API log listener can write request, audit, authentication, and system
-- events without breaking the legacy GoAdmin writer.

BEGIN;

-- The current integer sequence stops at 99,999,999, which is too small for a
-- continuously written log table. The Go model already stores the id as int64.
ALTER SEQUENCE public.goadmin_operation_log_myid_seq
    AS bigint
    NO MAXVALUE;

ALTER TABLE public.goadmin_operation_log
    ALTER COLUMN id TYPE bigint,
    ALTER COLUMN user_id DROP NOT NULL,
    ALTER COLUMN path TYPE character varying(2048),
    ALTER COLUMN path SET DEFAULT '',
    ALTER COLUMN method TYPE character varying(16),
    ALTER COLUMN method SET DEFAULT '',
    ALTER COLUMN ip TYPE character varying(45),
    ALTER COLUMN ip SET DEFAULT '',
    ALTER COLUMN input SET DEFAULT '',
    ADD COLUMN event_id character varying(64),
    ADD COLUMN event_type character varying(32) NOT NULL DEFAULT 'operation',
    ADD COLUMN level character varying(16) NOT NULL DEFAULT 'info',
    ADD COLUMN source character varying(100) NOT NULL DEFAULT 'goadmin',
    ADD COLUMN module character varying(100) NOT NULL DEFAULT '',
    ADD COLUMN action character varying(100) NOT NULL DEFAULT '',
    ADD COLUMN message text NOT NULL DEFAULT '',
    ADD COLUMN actor_name character varying(100) NOT NULL DEFAULT '',
    ADD COLUMN request_id character varying(64),
    ADD COLUMN trace_id character varying(64),
    ADD COLUMN status_code smallint,
    ADD COLUMN success boolean,
    ADD COLUMN duration_ms bigint,
    ADD COLUMN error_code character varying(100),
    ADD COLUMN error_message text,
    ADD COLUMN user_agent text,
    ADD COLUMN metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN occurred_at timestamp with time zone,
    ADD COLUMN expires_at timestamp with time zone;

-- Existing timestamps were written as Asia/Shanghai local time without an
-- offset. Preserve their instant when introducing the canonical event time.
UPDATE public.goadmin_operation_log
SET occurred_at = COALESCE(
        created_at,
        updated_at,
        CURRENT_TIMESTAMP::timestamp without time zone
    ) AT TIME ZONE 'Asia/Shanghai'
WHERE occurred_at IS NULL;

ALTER TABLE public.goadmin_operation_log
    ALTER COLUMN occurred_at SET DEFAULT CURRENT_TIMESTAMP,
    ALTER COLUMN occurred_at SET NOT NULL,
    ADD CONSTRAINT goadmin_operation_log_event_type_check
        CHECK (event_type IN ('operation', 'request', 'auth', 'audit', 'system')),
    ADD CONSTRAINT goadmin_operation_log_level_check
        CHECK (level IN ('debug', 'info', 'warn', 'error', 'fatal')),
    ADD CONSTRAINT goadmin_operation_log_status_code_check
        CHECK (status_code IS NULL OR status_code BETWEEN 100 AND 599),
    ADD CONSTRAINT goadmin_operation_log_duration_ms_check
        CHECK (duration_ms IS NULL OR duration_ms >= 0),
    ADD CONSTRAINT goadmin_operation_log_metadata_check
        CHECK (jsonb_typeof(metadata) = 'object'),
    ADD CONSTRAINT goadmin_operation_log_expires_at_check
        CHECK (expires_at IS NULL OR expires_at >= occurred_at);

-- event_id lets an asynchronous listener retry delivery without duplicating
-- an event. Legacy rows and synchronous writers may leave it NULL.
CREATE UNIQUE INDEX goadmin_operation_log_event_id_unique
    ON public.goadmin_operation_log (event_id)
    WHERE event_id IS NOT NULL AND event_id <> '';

-- These indexes cover the expected list, actor, failure, and trace lookups.
-- A JSONB GIN index is intentionally omitted until metadata query patterns are
-- known because it materially increases write amplification and disk usage.
CREATE INDEX goadmin_operation_log_occurred_at_index
    ON public.goadmin_operation_log (occurred_at DESC, id DESC);

CREATE INDEX goadmin_operation_log_event_type_time_index
    ON public.goadmin_operation_log (event_type, occurred_at DESC, id DESC);

CREATE INDEX goadmin_operation_log_user_time_index
    ON public.goadmin_operation_log (user_id, occurred_at DESC, id DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX goadmin_operation_log_failure_time_index
    ON public.goadmin_operation_log (occurred_at DESC, id DESC)
    WHERE success = false;

CREATE INDEX goadmin_operation_log_request_id_index
    ON public.goadmin_operation_log (request_id)
    WHERE request_id IS NOT NULL AND request_id <> '';

CREATE INDEX goadmin_operation_log_trace_id_index
    ON public.goadmin_operation_log (trace_id)
    WHERE trace_id IS NOT NULL AND trace_id <> '';

CREATE INDEX goadmin_operation_log_expires_at_index
    ON public.goadmin_operation_log (expires_at)
    WHERE expires_at IS NOT NULL;

COMMENT ON TABLE public.goadmin_operation_log IS
    'Operation and listener events for GoAdmin and Vben API';
COMMENT ON COLUMN public.goadmin_operation_log.event_id IS
    'Producer-generated idempotency key; NULL for legacy synchronous events';
COMMENT ON COLUMN public.goadmin_operation_log.user_id IS
    'Optional actor id; no foreign key so audit history survives user deletion';
COMMENT ON COLUMN public.goadmin_operation_log.actor_name IS
    'Actor name snapshot captured when the event occurs';
COMMENT ON COLUMN public.goadmin_operation_log.message IS
    'Sanitized human-readable event summary';
COMMENT ON COLUMN public.goadmin_operation_log.input IS
    'Redacted request or operation input; credentials and tokens must not be stored';
COMMENT ON COLUMN public.goadmin_operation_log.metadata IS
    'Sanitized event-specific attributes as a JSON object';
COMMENT ON COLUMN public.goadmin_operation_log.occurred_at IS
    'Canonical event time with time zone';
COMMENT ON COLUMN public.goadmin_operation_log.expires_at IS
    'Optional retention deadline used by a future cleanup job';

COMMIT;
