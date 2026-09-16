package loadrank

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

// Aggregation window and cardinality bounds. The summary table is bounded by
// retention and by the fact that status codes form an enumerated domain;
// route cardinality is bounded by template normalization (see route_index.go).
const (
	bucketSize           = time.Minute
	defaultRetentionDays = 30
	maxQueryWindow       = 31 * 24 * time.Hour
	maxPageSize          = 100
	defaultPageSize      = 20
	maxRouteLength       = 512
)

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS public.kadmin_loadrank_settings (
		id smallint PRIMARY KEY,
		enabled boolean NOT NULL DEFAULT FALSE,
		updated_by bigint NOT NULL DEFAULT 0,
		updated_at timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT kadmin_loadrank_settings_singleton_check CHECK (id = 1)
	)`,
	`INSERT INTO public.kadmin_loadrank_settings (id, enabled, updated_by)
		VALUES (1, FALSE, 0) ON CONFLICT (id) DO NOTHING`,
	`CREATE TABLE IF NOT EXISTS public.kadmin_http_metric_buckets (
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
	)`,
}

type settingsStore interface {
	LoadEnabled() (bool, error)
	SaveEnabled(enabled bool, updatedBy int64) error
}

// bucketAggregate is the per-bucket summary written to the summary table.
type bucketAggregate struct {
	route           string
	method          string
	statusCode      int64
	requestCount    int64
	errorCount      int64
	totalDurationMs int64
	maxDurationMs   int64
}

// bucketRow is one aggregated row read back from the summary table.
type bucketRow struct {
	bucketStart     time.Time
	route           string
	method          string
	statusCode      int64
	requestCount    int64
	errorCount      int64
	totalDurationMs int64
	maxDurationMs   int64
}

// bucketQueryFilter restricts the summary-table scan. StartedAt/EndedAt are
// always set by the handler, so every scan is a bounded range on the primary
// key prefix (bucket_start) instead of an unbounded log-table sweep.
type bucketQueryFilter struct {
	StartedAt  time.Time
	EndedAt    time.Time
	Route      string
	Method     string
	StatusCode *int64
}

type bucketStore interface {
	UpsertBucket(bucketStart time.Time, row bucketAggregate) error
	QueryBuckets(filter bucketQueryFilter) ([]bucketRow, error)
	PruneBefore(before time.Time) error
}

type settingsRepository struct {
	conn db.Connection
}

type bucketRepository struct {
	conn db.Connection
}

func EnsureSchema(conn db.Connection) error {
	if conn == nil {
		return errors.New("load ranking database connection is required")
	}
	for _, statement := range schemaStatements {
		if _, err := conn.Exec(statement); err != nil {
			return fmt.Errorf("initialize load ranking schema: %w", err)
		}
	}
	return nil
}

func (r *settingsRepository) LoadEnabled() (bool, error) {
	rows, err := r.conn.Query(`SELECT enabled FROM public.kadmin_loadrank_settings WHERE id = 1`)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, errors.New("load ranking setting is missing")
	}
	return loadrankBool(rows[0]["enabled"]), nil
}

func (r *settingsRepository) SaveEnabled(enabled bool, updatedBy int64) error {
	_, err := r.conn.Exec(`INSERT INTO public.kadmin_loadrank_settings
		(id, enabled, updated_by, updated_at) VALUES (1, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET enabled = EXCLUDED.enabled,
		updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP`, enabled, updatedBy)
	return err
}

func (r *bucketRepository) UpsertBucket(bucketStart time.Time, row bucketAggregate) error {
	_, err := r.conn.Exec(`INSERT INTO public.kadmin_http_metric_buckets
		(bucket_start, route, method, status_code,
		 request_count, error_count, total_duration_ms, max_duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (bucket_start, route, method, status_code) DO UPDATE SET
			request_count = public.kadmin_http_metric_buckets.request_count + EXCLUDED.request_count,
			error_count = public.kadmin_http_metric_buckets.error_count + EXCLUDED.error_count,
			total_duration_ms = public.kadmin_http_metric_buckets.total_duration_ms + EXCLUDED.total_duration_ms,
			max_duration_ms = GREATEST(public.kadmin_http_metric_buckets.max_duration_ms, EXCLUDED.max_duration_ms)`,
		bucketStart, row.route, row.method, row.statusCode,
		row.requestCount, row.errorCount, row.totalDurationMs, row.maxDurationMs)
	return err
}

func (r *bucketRepository) PruneBefore(before time.Time) error {
	_, err := r.conn.Exec(`DELETE FROM public.kadmin_http_metric_buckets WHERE bucket_start < ?`, before)
	return err
}

func (r *bucketRepository) QueryBuckets(filter bucketQueryFilter) ([]bucketRow, error) {
	conditions := []string{`bucket_start >= ?`, `bucket_start < ?`}
	args := []interface{}{filter.StartedAt, filter.EndedAt}
	if route := strings.TrimSpace(filter.Route); route != "" {
		conditions = append(conditions, `route ILIKE ?`)
		args = append(args, "%"+route+"%")
	}
	if method := strings.TrimSpace(filter.Method); method != "" {
		conditions = append(conditions, `method = ?`)
		args = append(args, method)
	}
	if filter.StatusCode != nil {
		conditions = append(conditions, `status_code = ?`)
		args = append(args, *filter.StatusCode)
	}

	query := `SELECT bucket_start, route, method, status_code,
		SUM(request_count) AS request_count,
		SUM(error_count) AS error_count,
		SUM(total_duration_ms) AS total_duration_ms,
		MAX(max_duration_ms) AS max_duration_ms
	FROM public.kadmin_http_metric_buckets
	WHERE ` + strings.Join(conditions, " AND ") + `
	GROUP BY bucket_start, route, method, status_code`
	rows, err := r.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	result := make([]bucketRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, bucketRow{
			bucketStart:     loadrankTime(row["bucket_start"]),
			route:           loadrankString(row["route"]),
			method:          loadrankString(row["method"]),
			statusCode:      loadrankInt64(row["status_code"]),
			requestCount:    loadrankInt64(row["request_count"]),
			errorCount:      loadrankInt64(row["error_count"]),
			totalDurationMs: loadrankInt64(row["total_duration_ms"]),
			maxDurationMs:   loadrankInt64(row["max_duration_ms"]),
		})
	}
	return result, nil
}

func loadrankBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case []byte:
		parsed, _ := strconv.ParseBool(string(typed))
		return parsed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	default:
		return false
	}
}

func loadrankInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

func loadrankString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
}

func loadrankTime(value interface{}) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed
	case []byte:
		return parseTimeString(string(typed))
	case string:
		return parseTimeString(typed)
	default:
		return time.Time{}
	}
}

func parseTimeString(value string) time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999-07", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
