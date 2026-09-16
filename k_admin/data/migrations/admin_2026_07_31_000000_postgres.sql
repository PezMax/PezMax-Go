-- Add KAdmin interface load ranking: HTTP metric summary table, sampling settings, permissions, and menu.
-- The summary table keeps aggregation bounded: it is written only while sampling
-- is enabled, pruned by retention, and queried exclusively through the
-- bucket_start primary-key prefix instead of scanning the unbounded log table.
-- This migration is idempotent so it can be used against an existing database.

BEGIN;

CREATE TABLE IF NOT EXISTS public.kadmin_loadrank_settings (
    id smallint PRIMARY KEY,
    enabled boolean NOT NULL DEFAULT FALSE,
    updated_by bigint NOT NULL DEFAULT 0,
    updated_at timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT kadmin_loadrank_settings_singleton_check CHECK (id = 1)
);

INSERT INTO public.kadmin_loadrank_settings (id, enabled, updated_by)
VALUES (1, FALSE, 0)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS public.kadmin_http_metric_buckets (
    bucket_start timestamp with time zone NOT NULL,
    route character varying(512) NOT NULL,
    method character varying(16) NOT NULL,
    status_code smallint NOT NULL,
    request_count bigint NOT NULL DEFAULT 0,
    error_count bigint NOT NULL DEFAULT 0,
    total_duration_ms bigint NOT NULL DEFAULT 0,
    max_duration_ms bigint NOT NULL DEFAULT 0,
    CONSTRAINT kadmin_http_metric_buckets_pkey
        PRIMARY KEY (bucket_start, route, method, status_code),
    CONSTRAINT kadmin_http_metric_buckets_status_code_check
        CHECK (status_code BETWEEN 100 AND 599),
    CONSTRAINT kadmin_http_metric_buckets_counts_check
        CHECK (request_count >= 0 AND error_count >= 0
            AND total_duration_ms >= 0 AND max_duration_ms >= 0)
);

COMMENT ON TABLE public.kadmin_http_metric_buckets
    IS 'Per-minute HTTP metric summaries for interface load ranking; bounded by retention pruning and template-normalized routes';

INSERT INTO public.goadmin_permissions
    (name, slug, http_method, http_path, created_at, updated_at)
SELECT seed.name, seed.slug, seed.http_method, seed.http_path, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM (VALUES
    ('查看接口负载排行', 'system:load-rank:view', 'GET', '/api/load-ranking*'),
    ('启停接口采样', 'system:load-rank:update', 'PATCH', '/api/load-ranking/status')
) AS seed(name, slug, http_method, http_path)
WHERE NOT EXISTS (
    SELECT 1 FROM public.goadmin_permissions permission WHERE permission.slug = seed.slug
);

INSERT INTO public.goadmin_menu
    (parent_id, type, "order", title, icon, uri, plugin_name, created_at, updated_at)
SELECT parent.id, 1, 11, '接口负载排行', 'lucide:bar-chart-3', '/kadmin/load-ranking', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM public.goadmin_menu parent
WHERE parent.uri = '/kadmin'
  AND NOT EXISTS (SELECT 1 FROM public.goadmin_menu menu WHERE menu.uri = '/kadmin/load-ranking')
LIMIT 1;

INSERT INTO public.goadmin_role_menu (role_id, menu_id, created_at, updated_at)
SELECT role.id, menu.id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
FROM public.goadmin_roles role
JOIN public.goadmin_menu menu ON menu.uri = '/kadmin/load-ranking'
WHERE role.slug = 'administrator'
  AND NOT EXISTS (
      SELECT 1 FROM public.goadmin_role_menu binding
      WHERE binding.role_id = role.id AND binding.menu_id = menu.id
  );

COMMIT;
