package loginlogs

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

type repository struct {
	conn db.Connection
}

func (r *repository) insert(attempt Attempt, browser, operatingSystem string) error {
	status := StatusFailed
	if attempt.Result == ResultSuccess || attempt.Result == ResultAccountUnlocked {
		status = StatusSuccess
	}
	_, err := r.conn.Exec(`INSERT INTO public.kadmin_login_audits
		(account, user_id, ip, user_agent, browser, os, status, result, failure_reason, duration_ms, occurred_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, attempt.Account, nullableID(attempt.UserID), attempt.IP,
		attempt.UserAgent, browser, operatingSystem, status, attempt.Result, attempt.FailureReason,
		attempt.DurationMs, attempt.OccurredAt)
	return err
}

func (r *repository) list(filter Filter) (Page, error) {
	where, args := auditWhere(filter)
	countRows, err := r.conn.Query("SELECT count(*) AS count FROM public.kadmin_login_audits a"+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = int64Value(countRows[0]["count"])
	}
	queryArgs := append(append([]interface{}{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.conn.Query(`SELECT id, account, user_id, ip, user_agent, browser, os, status,
		result, failure_reason, duration_ms, occurred_at, created_at
		FROM public.kadmin_login_audits a`+where+`
		ORDER BY occurred_at DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	items := make([]Entry, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapEntry(row))
	}
	return Page{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (r *repository) delete(ids []int64) (int64, error) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	result, err := r.conn.Exec("DELETE FROM public.kadmin_login_audits WHERE id IN ("+strings.Join(placeholders, ",")+")", args...)
	return rowsAffected(result, err)
}

func (r *repository) retention() (Retention, error) {
	rows, err := r.conn.Query(`SELECT retention_days, updated_by, updated_at
		FROM public.kadmin_login_audit_settings WHERE id = 1`)
	if err != nil {
		return Retention{}, err
	}
	if len(rows) == 0 {
		return Retention{Days: defaultRetentionDays}, nil
	}
	return Retention{
		Days: int(int64Value(rows[0]["retention_days"])), UpdatedBy: int64Value(rows[0]["updated_by"]),
		UpdatedAt: timeValue(rows[0]["updated_at"]),
	}, nil
}

func (r *repository) setRetention(days int, updatedBy int64) (Retention, error) {
	rows, err := r.conn.Query(`INSERT INTO public.kadmin_login_audit_settings
		(id, retention_days, updated_by, updated_at) VALUES (1, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET retention_days = EXCLUDED.retention_days,
		updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP
		RETURNING retention_days, updated_by, updated_at`, days, updatedBy)
	if err != nil {
		return Retention{}, err
	}
	if len(rows) == 0 {
		return Retention{}, fmt.Errorf("retention setting was not returned")
	}
	return Retention{Days: int(int64Value(rows[0]["retention_days"])), UpdatedBy: int64Value(rows[0]["updated_by"]), UpdatedAt: timeValue(rows[0]["updated_at"])}, nil
}

func (r *repository) cleanup(days int) (int64, error) {
	result, err := r.conn.Exec(`DELETE FROM public.kadmin_login_audits
		WHERE occurred_at < CURRENT_TIMESTAMP - (? * INTERVAL '1 day')`, days)
	return rowsAffected(result, err)
}

func auditWhere(filter Filter) (string, []interface{}) {
	conditions := make([]string, 0, 6)
	args := make([]interface{}, 0, 6)
	add := func(condition string, value interface{}) {
		conditions = append(conditions, condition)
		args = append(args, value)
	}
	if filter.Account != "" {
		add("a.account ILIKE ?", "%"+filter.Account+"%")
	}
	if filter.IP != "" {
		add("a.ip ILIKE ?", "%"+filter.IP+"%")
	}
	if filter.Status != "" {
		add("a.status = ?", filter.Status)
	}
	if filter.Result != "" {
		add("a.result = ?", filter.Result)
	}
	if filter.StartedAt != nil {
		add("a.occurred_at >= ?", *filter.StartedAt)
	}
	if filter.EndedAt != nil {
		add("a.occurred_at <= ?", *filter.EndedAt)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func mapEntry(row map[string]interface{}) Entry {
	return Entry{
		ID: int64Value(row["id"]), Account: stringValue(row["account"]), UserID: optionalInt64(row["user_id"]),
		IP: stringValue(row["ip"]), UserAgent: stringValue(row["user_agent"]), Browser: stringValue(row["browser"]),
		OS: stringValue(row["os"]), Status: stringValue(row["status"]), Result: stringValue(row["result"]),
		FailureReason: stringValue(row["failure_reason"]), DurationMs: int64Value(row["duration_ms"]),
		OccurredAt: timeValue(row["occurred_at"]), CreatedAt: timeValue(row["created_at"]),
	}
}

func nullableID(id *int64) interface{} {
	if id == nil || *id <= 0 {
		return nil
	}
	return *id
}

func rowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func int64Value(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		parsed, _ := strconv.ParseInt(fmt.Sprint(typed), 10, 64)
		return parsed
	}
}

func optionalInt64(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	parsed := int64Value(value)
	return &parsed
}

func timeValue(value interface{}) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(time.Time); ok {
		return typed.Format(time.RFC3339Nano)
	}
	return stringValue(value)
}
