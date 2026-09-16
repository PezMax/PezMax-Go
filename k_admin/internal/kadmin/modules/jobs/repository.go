package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/lib/pq"
)

type repository struct {
	conn db.Connection
}

func (r *repository) listJobs(filter JobFilter) (Page[Job], error) {
	where, args := jobWhere(filter)
	countRows, err := r.conn.Query("SELECT count(*) AS count FROM public.kadmin_jobs "+where, args...)
	if err != nil {
		return Page[Job]{}, err
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = toInt64(countRows[0]["count"])
	}
	queryArgs := append(append([]interface{}{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.conn.Query(`SELECT id, name, handler, cron_expression, parameters::text AS parameters,
		description, status, built_in, last_run_at, next_run_at, created_by, created_at, updated_at
		FROM public.kadmin_jobs `+where+`
		ORDER BY built_in DESC, id ASC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page[Job]{}, err
	}
	items := make([]Job, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapJob(row))
	}
	return Page[Job]{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func jobWhere(filter JobFilter) (string, []interface{}) {
	conditions := make([]string, 0, 3)
	args := make([]interface{}, 0, 4)
	if filter.Keyword != "" {
		conditions = append(conditions, "(name ILIKE ? OR description ILIKE ?)")
		pattern := "%" + filter.Keyword + "%"
		args = append(args, pattern, pattern)
	}
	if filter.Handler != "" {
		conditions = append(conditions, "handler = ?")
		args = append(args, filter.Handler)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (r *repository) listEnabledJobs() ([]Job, error) {
	rows, err := r.conn.Query(`SELECT id, name, handler, cron_expression, parameters::text AS parameters,
		description, status, built_in, last_run_at, next_run_at, created_by, created_at, updated_at
		FROM public.kadmin_jobs WHERE status = ? ORDER BY id`, statusEnabled)
	if err != nil {
		return nil, err
	}
	items := make([]Job, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapJob(row))
	}
	return items, nil
}

func (r *repository) getJob(id int64) (Job, bool, error) {
	rows, err := r.conn.Query(`SELECT id, name, handler, cron_expression, parameters::text AS parameters,
		description, status, built_in, last_run_at, next_run_at, created_by, created_at, updated_at
		FROM public.kadmin_jobs WHERE id = ?`, id)
	if err != nil {
		return Job{}, false, err
	}
	if len(rows) == 0 {
		return Job{}, false, nil
	}
	return mapJob(rows[0]), true, nil
}

func (r *repository) createJob(payload JobPayload, createdBy int64) (Job, error) {
	rows, err := r.conn.Query(`INSERT INTO public.kadmin_jobs
		(name, handler, cron_expression, parameters, description, status, built_in, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?::jsonb, ?, ?, FALSE, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id`, payload.Name, payload.Handler, payload.CronExpression, string(payload.Parameters), payload.Description, payload.Status, createdBy)
	if err != nil {
		return Job{}, normalizeRepositoryError(err)
	}
	if len(rows) == 0 {
		return Job{}, fmt.Errorf("create task returned no id")
	}
	job, found, err := r.getJob(toInt64(rows[0]["id"]))
	if err != nil {
		return Job{}, err
	}
	if !found {
		return Job{}, ErrJobNotFound
	}
	return job, nil
}

func (r *repository) updateJob(id int64, payload JobPayload) (Job, error) {
	existing, found, err := r.getJob(id)
	if err != nil {
		return Job{}, err
	}
	if !found {
		return Job{}, ErrJobNotFound
	}
	if existing.BuiltIn {
		payload.Name = existing.Name
		payload.Handler = existing.Handler
		payload.Description = existing.Description
	}
	result, err := r.conn.Exec(`UPDATE public.kadmin_jobs SET name = ?, handler = ?, cron_expression = ?,
		parameters = ?::jsonb, description = ?, status = ?, next_run_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, payload.Name, payload.Handler, payload.CronExpression, string(payload.Parameters), payload.Description, payload.Status, id)
	if err != nil {
		return Job{}, normalizeRepositoryError(err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Job{}, ErrJobNotFound
	}
	job, _, err := r.getJob(id)
	return job, err
}

func (r *repository) setStatus(id int64, status string) (Job, error) {
	result, err := r.conn.Exec(`UPDATE public.kadmin_jobs SET status = ?, next_run_at = NULL,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	if err != nil {
		return Job{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Job{}, ErrJobNotFound
	}
	job, _, err := r.getJob(id)
	return job, err
}

func (r *repository) deleteJob(id int64) error {
	job, found, err := r.getJob(id)
	if err != nil {
		return err
	}
	if !found {
		return ErrJobNotFound
	}
	if job.BuiltIn {
		return ErrBuiltInTask
	}
	_, err = r.conn.Exec("DELETE FROM public.kadmin_jobs WHERE id = ?", id)
	return err
}

func (r *repository) updateNextRun(id int64, next *time.Time) error {
	_, err := r.conn.Exec("UPDATE public.kadmin_jobs SET next_run_at = ?::timestamptz, updated_at = CURRENT_TIMESTAMP WHERE id = ?", next, id)
	return err
}

func (r *repository) updateRunTimes(id int64, last time.Time, next *time.Time) error {
	_, err := r.conn.Exec(`UPDATE public.kadmin_jobs SET last_run_at = ?::timestamptz,
		next_run_at = CASE WHEN status = 'enabled' THEN ?::timestamptz ELSE NULL END,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?`, last, next, id)
	return err
}

func (r *repository) createExecution(job Job, trigger string, triggeredBy *int64, started time.Time) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO public.kadmin_job_logs
		(job_id, job_name, handler, trigger, status, triggered_by, started_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP) RETURNING id`,
		job.ID, job.Name, job.Handler, trigger, executionRunning, triggeredBy, started)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("create execution log returned no id")
	}
	return toInt64(rows[0]["id"]), nil
}

func (r *repository) finishExecution(id int64, status, output, errorMessage string, finished time.Time, duration int64) error {
	_, err := r.conn.Exec(`UPDATE public.kadmin_job_logs SET status = ?, output = ?, error_message = ?,
		finished_at = ?, duration_ms = ? WHERE id = ?`, status, output, errorMessage, finished, duration, id)
	return err
}

func (r *repository) markInterruptedExecutions() error {
	_, err := r.conn.Exec(`UPDATE public.kadmin_job_logs SET status = 'failed',
		error_message = 'service restarted while task was running', finished_at = CURRENT_TIMESTAMP,
		duration_ms = GREATEST(0, EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - started_at)) * 1000)::bigint
		WHERE status = 'running'`)
	return err
}

func (r *repository) listExecutions(filter ExecutionFilter) (Page[JobExecution], error) {
	where, args := executionWhere(filter)
	countRows, err := r.conn.Query("SELECT count(*) AS count FROM public.kadmin_job_logs "+where, args...)
	if err != nil {
		return Page[JobExecution]{}, err
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = toInt64(countRows[0]["count"])
	}
	queryArgs := append(append([]interface{}{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.conn.Query(`SELECT id, job_id, job_name, handler, trigger, status, output, error_message,
		duration_ms, triggered_by, started_at, finished_at, created_at FROM public.kadmin_job_logs `+where+`
		ORDER BY started_at DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page[JobExecution]{}, err
	}
	items := make([]JobExecution, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapExecution(row))
	}
	return Page[JobExecution]{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func executionWhere(filter ExecutionFilter) (string, []interface{}) {
	conditions := make([]string, 0, 4)
	args := make([]interface{}, 0, 5)
	if filter.Keyword != "" {
		conditions = append(conditions, "(job_name ILIKE ? OR error_message ILIKE ? OR output ILIKE ?)")
		pattern := "%" + filter.Keyword + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if filter.JobID > 0 {
		conditions = append(conditions, "job_id = ?")
		args = append(args, filter.JobID)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Trigger != "" {
		conditions = append(conditions, "trigger = ?")
		args = append(args, filter.Trigger)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (r *repository) getExecution(id int64) (JobExecution, bool, error) {
	rows, err := r.conn.Query(`SELECT id, job_id, job_name, handler, trigger, status, output, error_message,
		duration_ms, triggered_by, started_at, finished_at, created_at
		FROM public.kadmin_job_logs WHERE id = ?`, id)
	if err != nil {
		return JobExecution{}, false, err
	}
	if len(rows) == 0 {
		return JobExecution{}, false, nil
	}
	return mapExecution(rows[0]), true, nil
}

func (r *repository) tryAdvisoryLock(id int64) (func(), bool, error) {
	tx := r.conn.BeginTx()
	if tx == nil {
		return nil, false, fmt.Errorf("begin task lock transaction failed")
	}
	rows, err := r.conn.QueryWithTx(tx, "SELECT pg_try_advisory_xact_lock(?) AS locked", id)
	if err != nil {
		_ = tx.Rollback()
		return nil, false, err
	}
	if len(rows) == 0 || !toBool(rows[0]["locked"]) {
		_ = tx.Rollback()
		return nil, false, nil
	}
	return func() { _ = tx.Commit() }, true, nil
}

func mapJob(row map[string]interface{}) Job {
	parameters := json.RawMessage(toString(row["parameters"]))
	if !json.Valid(parameters) {
		parameters = json.RawMessage("{}")
	}
	return Job{
		ID: toInt64(row["id"]), Name: toString(row["name"]), Handler: toString(row["handler"]),
		CronExpression: toString(row["cron_expression"]), Parameters: parameters,
		Description: toString(row["description"]), Status: toString(row["status"]), BuiltIn: toBool(row["built_in"]),
		LastRunAt: toTimeString(row["last_run_at"]), NextRunAt: toTimeString(row["next_run_at"]),
		CreatedBy: toInt64(row["created_by"]), CreatedAt: toTimeString(row["created_at"]), UpdatedAt: toTimeString(row["updated_at"]),
	}
}

func mapExecution(row map[string]interface{}) JobExecution {
	return JobExecution{
		ID: toInt64(row["id"]), JobID: nullableInt64(row["job_id"]), JobName: toString(row["job_name"]), Handler: toString(row["handler"]),
		Trigger: toString(row["trigger"]), Status: toString(row["status"]), Output: toString(row["output"]), Error: toString(row["error_message"]),
		DurationMs: toInt64(row["duration_ms"]), TriggeredBy: nullableInt64(row["triggered_by"]), StartedAt: toTimeString(row["started_at"]),
		FinishedAt: toTimeString(row["finished_at"]), CreatedAt: toTimeString(row["created_at"]),
	}
}

func normalizeRepositoryError(err error) error {
	var pqError *pq.Error
	if errors.As(err, &pqError) && pqError.Code == "23505" {
		return ErrNameExists
	}
	return err
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
}

func toInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

func nullableInt64(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	parsed := toInt64(value)
	return &parsed
}

func toBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(typed)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseBool(string(typed))
		return parsed
	default:
		return false
	}
}

func toTimeString(value interface{}) string {
	if value == nil {
		return ""
	}
	if parsed, ok := value.(time.Time); ok {
		return parsed.In(time.Local).Format("2006-01-02 15:04:05")
	}
	text := strings.TrimSpace(toString(value))
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.In(time.Local).Format("2006-01-02 15:04:05")
		}
	}
	return text
}
