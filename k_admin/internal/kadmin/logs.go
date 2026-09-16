package kadmin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
)

const maxLogPageSize = 100

type managedLog struct {
	ID           int64           `json:"id"`
	UserID       *int64          `json:"userId"`
	ActorName    string          `json:"actorName"`
	Path         string          `json:"path"`
	Method       string          `json:"method"`
	IP           string          `json:"ip"`
	Input        string          `json:"input"`
	EventID      string          `json:"eventId"`
	EventType    string          `json:"eventType"`
	Level        string          `json:"level"`
	Source       string          `json:"source"`
	Module       string          `json:"module"`
	Action       string          `json:"action"`
	Message      string          `json:"message"`
	RequestID    string          `json:"requestId"`
	TraceID      string          `json:"traceId"`
	StatusCode   *int64          `json:"statusCode"`
	Success      *bool           `json:"success"`
	DurationMs   *int64          `json:"durationMs"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	UserAgent    string          `json:"userAgent"`
	Metadata     json.RawMessage `json:"metadata"`
	OccurredAt   string          `json:"occurredAt"`
	ExpiresAt    string          `json:"expiresAt"`
	CreatedAt    string          `json:"createdAt"`
}

type managedLogListResponse struct {
	Items    []managedLog `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

type managedLogFilter struct {
	Page       int
	PageSize   int
	Keyword    string
	EventType  string
	Level      string
	Source     string
	Method     string
	Success    *bool
	StatusCode *int64
	StartedAt  *time.Time
	EndedAt    *time.Time
}

type deleteLogsPayload struct {
	IDs []int64 `json:"ids"`
}

func registerLogRoutes(api *gin.RouterGroup, s *Store) {
	logs := api.Group("/logs", s.requireAuth(), s.requirePermission(logListPermission))
	logs.GET("", s.listLogs)
	logs.GET("/:id", s.logDetail)
	logs.DELETE("", s.requirePermission(logDeletePermission), s.deleteLogs)
	logs.DELETE("/:id", s.requirePermission(logDeletePermission), s.deleteLog)
}

func (s *Store) listLogs(c *gin.Context) {
	filter, err := parseManagedLogFilter(c)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	where, args := managedLogWhere(filter)

	countRows, err := s.conn.Query("SELECT count(*) AS count FROM public.goadmin_operation_log l LEFT JOIN public.goadmin_users u ON u.id = l.user_id "+where, args...)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = toInt64(countRows[0]["count"])
	}

	query := `SELECT l.id, l.user_id,
		COALESCE(NULLIF(l.actor_name, ''), NULLIF(u.name, ''), u.username, '') AS actor_display_name,
		l.path, l.method, l.ip, l.input, l.event_id, l.event_type, l.level,
		l.source, l.module, l.action, l.message, l.request_id, l.trace_id,
		l.status_code, l.success, l.duration_ms, l.error_code, l.error_message,
		l.user_agent, l.metadata::text AS metadata, l.occurred_at, l.expires_at, l.created_at
	FROM public.goadmin_operation_log l
	LEFT JOIN public.goadmin_users u ON u.id = l.user_id ` + where + `
	ORDER BY l.occurred_at DESC, l.id DESC LIMIT ? OFFSET ?`
	queryArgs := append(append([]interface{}{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.conn.Query(query, queryArgs...)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]managedLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapManagedLog(row))
	}
	success(c, managedLogListResponse{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize})
}

func (s *Store) logDetail(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, found, err := s.loadManagedLog(id)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		fail(c, http.StatusNotFound, "log not found")
		return
	}
	success(c, item)
}

func (s *Store) deleteLog(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, found, err := s.loadManagedLog(id); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	} else if !found {
		fail(c, http.StatusNotFound, "log not found")
		return
	}
	if err := db.WithDriver(s.conn).Table("goadmin_operation_log").Where("id", "=", id).Delete(); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

func (s *Store) deleteLogs(c *gin.Context) {
	var payload deleteLogsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		fail(c, http.StatusBadRequest, "invalid log ids")
		return
	}
	ids := uniquePositiveIDs(payload.IDs)
	if len(ids) == 0 || len(ids) > maxLogPageSize {
		fail(c, http.StatusBadRequest, "select between 1 and 100 logs")
		return
	}
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if err := db.WithDriver(s.conn).Table("goadmin_operation_log").WhereIn("id", args).Delete(); err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	success(c, true)
}

func (s *Store) loadManagedLog(id int64) (managedLog, bool, error) {
	rows, err := s.conn.Query(`SELECT l.id, l.user_id,
		COALESCE(NULLIF(l.actor_name, ''), NULLIF(u.name, ''), u.username, '') AS actor_display_name,
		l.path, l.method, l.ip, l.input, l.event_id, l.event_type, l.level,
		l.source, l.module, l.action, l.message, l.request_id, l.trace_id,
		l.status_code, l.success, l.duration_ms, l.error_code, l.error_message,
		l.user_agent, l.metadata::text AS metadata, l.occurred_at, l.expires_at, l.created_at
	FROM public.goadmin_operation_log l
	LEFT JOIN public.goadmin_users u ON u.id = l.user_id
	WHERE l.id = ?`, id)
	if err != nil {
		return managedLog{}, false, err
	}
	if len(rows) == 0 {
		return managedLog{}, false, nil
	}
	return mapManagedLog(rows[0]), true, nil
}

func parseManagedLogFilter(c *gin.Context) (managedLogFilter, error) {
	filter := managedLogFilter{
		Page:      positiveQueryInt(c, "page", 1),
		PageSize:  positiveQueryInt(c, "pageSize", 20),
		Keyword:   truncateRunes(c.Query("keyword"), 100),
		EventType: strings.TrimSpace(c.Query("eventType")),
		Level:     strings.TrimSpace(c.Query("level")),
		Source:    truncateRunes(c.Query("source"), 100),
		Method:    strings.ToUpper(truncateRunes(c.Query("method"), 16)),
	}
	if filter.PageSize > maxLogPageSize {
		filter.PageSize = maxLogPageSize
	}
	if filter.EventType != "" && !validManagedLogEventType(filter.EventType) {
		return filter, fmt.Errorf("invalid event type %q", filter.EventType)
	}
	if filter.Level != "" && !validManagedLogLevel(filter.Level) {
		return filter, fmt.Errorf("invalid log level %q", filter.Level)
	}
	if value := strings.TrimSpace(c.Query("success")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return filter, errorsForQuery("success")
		}
		filter.Success = &parsed
	}
	if value := strings.TrimSpace(c.Query("statusCode")); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 100 || parsed > 599 {
			return filter, errorsForQuery("statusCode")
		}
		filter.StatusCode = &parsed
	}
	var err error
	if filter.StartedAt, err = optionalLogTime(c.Query("startedAt")); err != nil {
		return filter, errorsForQuery("startedAt")
	}
	if filter.EndedAt, err = optionalLogTime(c.Query("endedAt")); err != nil {
		return filter, errorsForQuery("endedAt")
	}
	if filter.StartedAt != nil && filter.EndedAt != nil && filter.EndedAt.Before(*filter.StartedAt) {
		return filter, fmt.Errorf("endedAt must not precede startedAt")
	}
	return filter, nil
}

func managedLogWhere(filter managedLogFilter) (string, []interface{}) {
	conditions := make([]string, 0, 8)
	args := make([]interface{}, 0, 8)
	add := func(condition string, value interface{}) {
		conditions = append(conditions, condition)
		args = append(args, value)
	}
	if filter.Keyword != "" {
		pattern := "%" + filter.Keyword + "%"
		conditions = append(conditions, `(l.path ILIKE ? OR l.message ILIKE ? OR l.request_id ILIKE ? OR l.trace_id ILIKE ? OR l.actor_name ILIKE ? OR l.input ILIKE ? OR l.metadata::text ILIKE ? OR u.username ILIKE ? OR u.name ILIKE ?)`)
		for range 9 {
			args = append(args, pattern)
		}
	}
	if filter.EventType != "" {
		add("l.event_type = ?", filter.EventType)
	}
	if filter.Level != "" {
		add("l.level = ?", filter.Level)
	}
	if filter.Source != "" {
		add("l.source = ?", filter.Source)
	}
	if filter.Method != "" {
		add("l.method = ?", filter.Method)
	}
	if filter.Success != nil {
		add("l.success = ?", *filter.Success)
	}
	if filter.StatusCode != nil {
		add("l.status_code = ?", *filter.StatusCode)
	}
	if filter.StartedAt != nil {
		add("l.occurred_at >= ?", *filter.StartedAt)
	}
	if filter.EndedAt != nil {
		add("l.occurred_at <= ?", *filter.EndedAt)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func mapManagedLog(row map[string]interface{}) managedLog {
	metadata := json.RawMessage(toString(row["metadata"]))
	if !json.Valid(metadata) {
		metadata = json.RawMessage("{}")
	}
	return managedLog{
		ID:           toInt64(row["id"]),
		UserID:       nullableLogInt64(row["user_id"]),
		ActorName:    toString(row["actor_display_name"]),
		Path:         toString(row["path"]),
		Method:       toString(row["method"]),
		IP:           toString(row["ip"]),
		Input:        toString(row["input"]),
		EventID:      toString(row["event_id"]),
		EventType:    toString(row["event_type"]),
		Level:        toString(row["level"]),
		Source:       toString(row["source"]),
		Module:       toString(row["module"]),
		Action:       toString(row["action"]),
		Message:      toString(row["message"]),
		RequestID:    toString(row["request_id"]),
		TraceID:      toString(row["trace_id"]),
		StatusCode:   nullableLogInt64(row["status_code"]),
		Success:      nullableLogBool(row["success"]),
		DurationMs:   nullableLogInt64(row["duration_ms"]),
		ErrorCode:    toString(row["error_code"]),
		ErrorMessage: toString(row["error_message"]),
		UserAgent:    toString(row["user_agent"]),
		Metadata:     metadata,
		OccurredAt:   toDateTimeString(row["occurred_at"]),
		ExpiresAt:    toDateTimeString(row["expires_at"]),
		CreatedAt:    toDateTimeString(row["created_at"]),
	}
}

func positiveQueryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Query(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func optionalLogTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("invalid time")
}

func errorsForQuery(field string) error {
	return fmt.Errorf("invalid %s", field)
}

func nullableLogInt64(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	parsed := toInt64(value)
	return &parsed
}

func nullableLogBool(value interface{}) *bool {
	if value == nil {
		return nil
	}
	parsed, ok := value.(bool)
	if !ok {
		return nil
	}
	return &parsed
}

func uniquePositiveIDs(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]bool, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func validManagedLogEventType(value string) bool {
	switch value {
	case "operation", "request", "auth", "audit", "system":
		return true
	default:
		return false
	}
}

func validManagedLogLevel(value string) bool {
	switch value {
	case "debug", "info", "warn", "error", "fatal":
		return true
	default:
		return false
	}
}
