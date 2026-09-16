package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOperationLogMapToModel(t *testing.T) {
	statusCode := int64(201)
	durationMs := int64(42)
	model := OperationLog().MapToModel(map[string]interface{}{
		"id":            int64(9),
		"user_id":       nil,
		"path":          "/api/users",
		"method":        "POST",
		"ip":            "2001:db8::1",
		"input":         "{}",
		"event_id":      "event-9",
		"event_type":    OperationLogEventTypeRequest,
		"level":         OperationLogLevelInfo,
		"source":        "vbenapi",
		"module":        "user",
		"action":        "create",
		"message":       "user created",
		"actor_name":    "admin",
		"request_id":    "request-9",
		"trace_id":      "trace-9",
		"status_code":   statusCode,
		"success":       true,
		"duration_ms":   durationMs,
		"error_code":    nil,
		"error_message": nil,
		"user_agent":    "test-agent",
		"metadata":      []byte(`{"resource_id":9}`),
		"occurred_at":   "2026-07-22 10:00:00+08",
		"expires_at":    nil,
		"created_at":    "2026-07-22 10:00:00",
		"updated_at":    "2026-07-22 10:00:00",
	})

	if model.Id != 9 || model.UserId != 0 {
		t.Fatalf("unexpected ids: id=%d user_id=%d", model.Id, model.UserId)
	}
	if model.StatusCode == nil || *model.StatusCode != statusCode {
		t.Fatalf("unexpected status code: %#v", model.StatusCode)
	}
	if model.Success == nil || !*model.Success {
		t.Fatalf("unexpected success: %#v", model.Success)
	}
	if model.DurationMs == nil || *model.DurationMs != durationMs {
		t.Fatalf("unexpected duration: %#v", model.DurationMs)
	}
	if !json.Valid(model.Metadata) || string(model.Metadata) != `{"resource_id":9}` {
		t.Fatalf("unexpected metadata: %s", model.Metadata)
	}
	if model.Ip != "2001:db8::1" || model.ExpiresAt != "" {
		t.Fatalf("unexpected optional fields: ip=%q expires_at=%q", model.Ip, model.ExpiresAt)
	}
}

func TestOperationLogMapToModelSupportsLegacyRows(t *testing.T) {
	model := OperationLog().MapToModel(map[string]interface{}{
		"id":         int64(1),
		"user_id":    int64(2),
		"path":       "/admin/logout",
		"method":     "GET",
		"ip":         "127.0.0.1",
		"input":      "",
		"created_at": "2026-07-22 10:00:00",
		"updated_at": "2026-07-22 10:00:00",
	})

	if model.Id != 1 || model.UserId != 2 || model.EventType != "" {
		t.Fatalf("unexpected legacy model: %#v", model)
	}
	if model.StatusCode != nil || model.Success != nil || model.DurationMs != nil {
		t.Fatalf("legacy nullable fields must remain nil: %#v", model)
	}
}

func TestOperationLogEventValues(t *testing.T) {
	userId := int64(3)
	statusCode := int64(204)
	success := true
	durationMs := int64(12)
	occurredAt := time.Date(2026, 7, 22, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	expiresAt := occurredAt.Add(30 * 24 * time.Hour)
	event := OperationLogEvent{
		UserId:     &userId,
		Path:       "/api/users/3",
		Method:     "PUT",
		Ip:         "127.0.0.1",
		EventId:    "event-3",
		StatusCode: &statusCode,
		Success:    &success,
		DurationMs: &durationMs,
		Metadata:   json.RawMessage(`{"resource_id":3}`),
		OccurredAt: &occurredAt,
		ExpiresAt:  &expiresAt,
	}

	values, err := event.values()
	if err != nil {
		t.Fatalf("values returned error: %v", err)
	}
	if values["event_type"] != OperationLogEventTypeOperation || values["level"] != OperationLogLevelInfo {
		t.Fatalf("defaults were not applied: %#v", values)
	}
	if values["source"] != "goadmin" || values["user_id"] != userId {
		t.Fatalf("unexpected source or user: %#v", values)
	}
	if values["metadata"] != `{"resource_id":3}` {
		t.Fatalf("unexpected metadata value: %#v", values["metadata"])
	}
}

func TestOperationLogEventValuesRejectInvalidInput(t *testing.T) {
	invalidStatusCode := int64(99)
	negativeDuration := int64(-1)
	now := time.Now()

	tests := []struct {
		name  string
		event OperationLogEvent
	}{
		{name: "event type", event: OperationLogEvent{EventType: "unknown"}},
		{name: "level", event: OperationLogEvent{Level: "notice"}},
		{name: "status code", event: OperationLogEvent{StatusCode: &invalidStatusCode}},
		{name: "duration", event: OperationLogEvent{DurationMs: &negativeDuration}},
		{name: "metadata array", event: OperationLogEvent{Metadata: json.RawMessage(`[]`)}},
		{name: "metadata null", event: OperationLogEvent{Metadata: json.RawMessage(`null`)}},
		{name: "expiration", event: OperationLogEvent{OccurredAt: &now, ExpiresAt: timePointer(now.Add(-time.Second))}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.event.values(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}
