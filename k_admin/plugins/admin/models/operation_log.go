package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/modules/db/dialect"
)

const (
	OperationLogEventTypeOperation = "operation"
	OperationLogEventTypeRequest   = "request"
	OperationLogEventTypeAuth      = "auth"
	OperationLogEventTypeAudit     = "audit"
	OperationLogEventTypeSystem    = "system"

	OperationLogLevelDebug = "debug"
	OperationLogLevelInfo  = "info"
	OperationLogLevelWarn  = "warn"
	OperationLogLevelError = "error"
	OperationLogLevelFatal = "fatal"
)

// OperationLogModel is operation log model structure.
type OperationLogModel struct {
	Base

	Id           int64
	UserId       int64
	Path         string
	Method       string
	Ip           string
	Input        string
	EventId      string
	EventType    string
	Level        string
	Source       string
	Module       string
	Action       string
	Message      string
	ActorName    string
	RequestId    string
	TraceId      string
	StatusCode   *int64
	Success      *bool
	DurationMs   *int64
	ErrorCode    string
	ErrorMessage string
	UserAgent    string
	Metadata     json.RawMessage
	OccurredAt   string
	ExpiresAt    string
	CreatedAt    string
	UpdatedAt    string
}

// OperationLogEvent contains the fields accepted from the new log listener.
// Request bodies and metadata must be sanitized before constructing the event.
type OperationLogEvent struct {
	UserId       *int64
	Path         string
	Method       string
	Ip           string
	Input        string
	EventId      string
	EventType    string
	Level        string
	Source       string
	Module       string
	Action       string
	Message      string
	ActorName    string
	RequestId    string
	TraceId      string
	StatusCode   *int64
	Success      *bool
	DurationMs   *int64
	ErrorCode    string
	ErrorMessage string
	UserAgent    string
	Metadata     json.RawMessage
	OccurredAt   *time.Time
	ExpiresAt    *time.Time
}

// OperationLog return a default operation log model.
func OperationLog() OperationLogModel {
	return OperationLogModel{Base: Base{TableName: "goadmin_operation_log"}}
}

// Find return a default operation log model of given id.
func (t OperationLogModel) Find(id interface{}) OperationLogModel {
	item, _ := t.Table(t.TableName).Find(id)
	return t.MapToModel(item)
}

func (t OperationLogModel) SetConn(con db.Connection) OperationLogModel {
	t.Conn = con
	return t
}

// New create a new operation log model.
func (t OperationLogModel) New(userId int64, path, method, ip, input string) OperationLogModel {

	id, _ := t.Table(t.TableName).Insert(dialect.H{
		"user_id": userId,
		"path":    path,
		"method":  method,
		"ip":      ip,
		"input":   input,
	})

	t.Id = id
	t.UserId = userId
	t.Path = path
	t.Method = method
	t.Ip = ip
	t.Input = input

	return t
}

// NewEvent creates a structured listener event. The database migration must be
// applied before this method is used; New remains compatible with the old table.
func (t OperationLogModel) NewEvent(event OperationLogEvent) (OperationLogModel, error) {
	values, err := event.values()
	if err != nil {
		return t, err
	}

	id, err := t.Table(t.TableName).Insert(values)
	if err != nil {
		return t, err
	}

	t.Id = id
	t.applyEvent(event)
	return t, nil
}

func (event *OperationLogEvent) values() (dialect.H, error) {
	if event.EventType == "" {
		event.EventType = OperationLogEventTypeOperation
	}
	if !validOperationLogEventType(event.EventType) {
		return nil, fmt.Errorf("invalid operation log event type %q", event.EventType)
	}

	if event.Level == "" {
		event.Level = OperationLogLevelInfo
	}
	if !validOperationLogLevel(event.Level) {
		return nil, fmt.Errorf("invalid operation log level %q", event.Level)
	}

	if event.Source == "" {
		event.Source = "goadmin"
	}
	if event.StatusCode != nil && (*event.StatusCode < 100 || *event.StatusCode > 599) {
		return nil, fmt.Errorf("invalid operation log status code %d", *event.StatusCode)
	}
	if event.DurationMs != nil && *event.DurationMs < 0 {
		return nil, fmt.Errorf("invalid operation log duration %d", *event.DurationMs)
	}
	if event.ExpiresAt != nil && event.OccurredAt != nil && event.ExpiresAt.Before(*event.OccurredAt) {
		return nil, errors.New("operation log expiration must not precede occurrence")
	}

	if len(event.Metadata) == 0 {
		event.Metadata = json.RawMessage("{}")
	}
	var metadata map[string]json.RawMessage
	if err := json.Unmarshal(event.Metadata, &metadata); err != nil || metadata == nil {
		return nil, errors.New("operation log metadata must be a JSON object")
	}

	values := dialect.H{
		"path":          event.Path,
		"method":        event.Method,
		"ip":            event.Ip,
		"input":         event.Input,
		"event_type":    event.EventType,
		"level":         event.Level,
		"source":        event.Source,
		"module":        event.Module,
		"action":        event.Action,
		"message":       event.Message,
		"actor_name":    event.ActorName,
		"error_code":    operationLogNullableString(event.ErrorCode),
		"error_message": operationLogNullableString(event.ErrorMessage),
		"user_agent":    operationLogNullableString(event.UserAgent),
		"metadata":      string(event.Metadata),
	}

	values["user_id"] = operationLogNullableInt64(event.UserId)
	values["event_id"] = operationLogNullableString(event.EventId)
	values["request_id"] = operationLogNullableString(event.RequestId)
	values["trace_id"] = operationLogNullableString(event.TraceId)
	values["status_code"] = operationLogNullableInt64(event.StatusCode)
	values["success"] = operationLogNullableBool(event.Success)
	values["duration_ms"] = operationLogNullableInt64(event.DurationMs)
	if event.OccurredAt != nil {
		values["occurred_at"] = *event.OccurredAt
	}
	if event.ExpiresAt != nil {
		values["expires_at"] = *event.ExpiresAt
	}
	return values, nil
}

func (t *OperationLogModel) applyEvent(event OperationLogEvent) {
	if event.UserId != nil {
		t.UserId = *event.UserId
	}
	t.Path = event.Path
	t.Method = event.Method
	t.Ip = event.Ip
	t.Input = event.Input
	t.EventId = event.EventId
	t.EventType = event.EventType
	t.Level = event.Level
	t.Source = event.Source
	t.Module = event.Module
	t.Action = event.Action
	t.Message = event.Message
	t.ActorName = event.ActorName
	t.RequestId = event.RequestId
	t.TraceId = event.TraceId
	t.StatusCode = event.StatusCode
	t.Success = event.Success
	t.DurationMs = event.DurationMs
	t.ErrorCode = event.ErrorCode
	t.ErrorMessage = event.ErrorMessage
	t.UserAgent = event.UserAgent
	t.Metadata = append(json.RawMessage(nil), event.Metadata...)
	if event.OccurredAt != nil {
		t.OccurredAt = event.OccurredAt.Format(time.RFC3339Nano)
	}
	if event.ExpiresAt != nil {
		t.ExpiresAt = event.ExpiresAt.Format(time.RFC3339Nano)
	}
}

// MapToModel get the operation log model from given map.
func (t OperationLogModel) MapToModel(m map[string]interface{}) OperationLogModel {
	t.Id = operationLogInt64(m["id"])
	t.UserId = operationLogInt64(m["user_id"])
	t.Path = operationLogString(m["path"])
	t.Method = operationLogString(m["method"])
	t.Ip = operationLogString(m["ip"])
	t.Input = operationLogString(m["input"])
	t.EventId = operationLogString(m["event_id"])
	t.EventType = operationLogString(m["event_type"])
	t.Level = operationLogString(m["level"])
	t.Source = operationLogString(m["source"])
	t.Module = operationLogString(m["module"])
	t.Action = operationLogString(m["action"])
	t.Message = operationLogString(m["message"])
	t.ActorName = operationLogString(m["actor_name"])
	t.RequestId = operationLogString(m["request_id"])
	t.TraceId = operationLogString(m["trace_id"])
	t.StatusCode = operationLogInt64Pointer(m["status_code"])
	t.Success = operationLogBoolPointer(m["success"])
	t.DurationMs = operationLogInt64Pointer(m["duration_ms"])
	t.ErrorCode = operationLogString(m["error_code"])
	t.ErrorMessage = operationLogString(m["error_message"])
	t.UserAgent = operationLogString(m["user_agent"])
	t.Metadata = json.RawMessage(operationLogString(m["metadata"]))
	t.OccurredAt = operationLogString(m["occurred_at"])
	t.ExpiresAt = operationLogString(m["expires_at"])
	t.CreatedAt = operationLogString(m["created_at"])
	t.UpdatedAt = operationLogString(m["updated_at"])
	return t
}

func validOperationLogEventType(value string) bool {
	switch value {
	case OperationLogEventTypeOperation, OperationLogEventTypeRequest, OperationLogEventTypeAuth,
		OperationLogEventTypeAudit, OperationLogEventTypeSystem:
		return true
	default:
		return false
	}
}

func validOperationLogLevel(value string) bool {
	switch value {
	case OperationLogLevelDebug, OperationLogLevelInfo, OperationLogLevelWarn,
		OperationLogLevelError, OperationLogLevelFatal:
		return true
	default:
		return false
	}
}

func operationLogString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func operationLogInt64(value interface{}) int64 {
	if v, ok := value.(int64); ok {
		return v
	}
	return 0
}

func operationLogInt64Pointer(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	v := operationLogInt64(value)
	return &v
}

func operationLogBoolPointer(value interface{}) *bool {
	if value == nil {
		return nil
	}
	v, ok := value.(bool)
	if !ok {
		return nil
	}
	return &v
}

func operationLogNullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func operationLogNullableInt64(value *int64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func operationLogNullableBool(value *bool) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
