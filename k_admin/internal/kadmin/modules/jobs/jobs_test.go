package jobs

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

func TestNormalizePayloadAcceptsStandardCron(t *testing.T) {
	payload := JobPayload{
		Name:           " daily cleanup ",
		Handler:        HandlerLogCleanup,
		CronExpression: "0 2 * * *",
		Parameters:     json.RawMessage(`{"retentionDays":30,"taskLogRetentionDays":90}`),
		Status:         statusEnabled,
	}
	if err := normalizePayload(&payload); err != nil {
		t.Fatalf("normalize payload: %v", err)
	}
	if payload.Name != "daily cleanup" {
		t.Fatalf("unexpected normalized name %q", payload.Name)
	}
	if err := validateHandlerParameters(payload.Handler, payload.Parameters); err != nil {
		t.Fatalf("validate handler parameters: %v", err)
	}
}

func TestNormalizePayloadRejectsInvalidCronAndParameters(t *testing.T) {
	tests := []JobPayload{
		{Name: "bad cron", Handler: HandlerCacheRefresh, CronExpression: "70 * * * *", Parameters: json.RawMessage(`{}`)},
		{Name: "bad json", Handler: HandlerCacheRefresh, CronExpression: "* * * * *", Parameters: json.RawMessage(`[]`)},
	}
	for _, payload := range tests {
		if err := normalizePayload(&payload); err == nil {
			t.Fatalf("expected payload %q to fail", payload.Name)
		}
	}
}

func TestNextRunUsesFiveFieldCron(t *testing.T) {
	from := time.Date(2026, time.July, 26, 1, 59, 30, 0, time.Local)
	next, err := nextRun("0 2 * * *", from)
	if err != nil {
		t.Fatalf("calculate next run: %v", err)
	}
	want := time.Date(2026, time.July, 26, 2, 0, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("next run = %s, want %s", next, want)
	}
}

func TestNextRunSupportsStepWithOffset(t *testing.T) {
	from := time.Date(2026, time.July, 26, 10, 6, 0, 0, time.Local)
	next, err := nextRun("5/10 * * * *", from)
	if err != nil {
		t.Fatalf("calculate offset step: %v", err)
	}
	want := time.Date(2026, time.July, 26, 10, 15, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("next run = %s, want %s", next, want)
	}
}

func TestValidateCleanupRetentionRange(t *testing.T) {
	for _, raw := range []string{
		`{"retentionDays":0,"taskLogRetentionDays":90}`,
		`{"retentionDays":30,"taskLogRetentionDays":3651}`,
	} {
		if err := validateHandlerParameters(HandlerLogCleanup, json.RawMessage(raw)); err == nil {
			t.Fatalf("expected parameters %s to fail", raw)
		}
	}
}

func TestJobWhereBuildsStableFilters(t *testing.T) {
	where, args := jobWhere(JobFilter{Keyword: "cleanup", Handler: HandlerLogCleanup, Status: statusPaused})
	if where != "WHERE (name ILIKE ? OR description ILIKE ?) AND handler = ? AND status = ?" {
		t.Fatalf("unexpected where clause %q", where)
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 arguments, got %d", len(args))
	}
}

func TestToTimeStringNormalizesPostgresTimestamp(t *testing.T) {
	got := toTimeString("2026-07-26T14:33:37.31207+08:00")
	if got != "2026-07-26 14:33:37" {
		t.Fatalf("normalized timestamp = %q", got)
	}
}

type captureExecConnection struct {
	db.Connection
	query string
	args  []interface{}
}

func (c *captureExecConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	c.query = query
	c.args = append([]interface{}(nil), args...)
	return nil, nil
}

func TestUpdateRunTimesCastsTimestampParameters(t *testing.T) {
	connection := &captureExecConnection{}
	repository := &repository{conn: connection}
	last := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	next := last.Add(time.Hour)

	if err := repository.updateRunTimes(7, last, &next); err != nil {
		t.Fatalf("update run times: %v", err)
	}
	if !strings.Contains(connection.query, "last_run_at = ?::timestamptz") {
		t.Fatalf("last_run_at is not explicitly cast: %q", connection.query)
	}
	if !strings.Contains(connection.query, "THEN ?::timestamptz") {
		t.Fatalf("next_run_at CASE value is not explicitly cast: %q", connection.query)
	}
	if len(connection.args) != 3 {
		t.Fatalf("captured %d arguments, want 3", len(connection.args))
	}
}

func TestUpdateNextRunCastsTimestampParameter(t *testing.T) {
	connection := &captureExecConnection{}
	repository := &repository{conn: connection}
	next := time.Date(2026, time.August, 8, 13, 0, 0, 0, time.UTC)

	if err := repository.updateNextRun(7, &next); err != nil {
		t.Fatalf("update next run: %v", err)
	}
	if !strings.Contains(connection.query, "next_run_at = ?::timestamptz") {
		t.Fatalf("next_run_at is not explicitly cast: %q", connection.query)
	}
}
