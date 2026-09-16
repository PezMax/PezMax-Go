package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type fakeSettings struct {
	mu      sync.Mutex
	enabled bool
	savedBy int64
}

func (s *fakeSettings) LoadEnabled() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled, nil
}

func (s *fakeSettings) SaveEnabled(enabled bool, updatedBy int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	s.savedBy = updatedBy
	return nil
}

type fakeCollector struct {
	mu    sync.Mutex
	calls int
}

func (c *fakeCollector) Collect(context.Context) (Metrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return Metrics{SampledAt: time.Now().Format(time.RFC3339), CPU: CPUMetrics{UsagePercent: 12.5}}, nil
}

func (c *fakeCollector) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestManagerStopsCollectingWhenDisabled(t *testing.T) {
	settings := &fakeSettings{}
	collector := &fakeCollector{}
	manager, err := NewManager(ManagerOptions{
		Settings: settings, Collector: collector, Interval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()

	time.Sleep(40 * time.Millisecond)
	if calls := collector.Calls(); calls != 0 {
		t.Fatalf("disabled monitor collected %d times", calls)
	}
	if _, err := manager.SetEnabled(true, 7); err != nil {
		t.Fatalf("enable monitor: %v", err)
	}
	waitForCollection(t, collector)
	status := manager.Status()
	if !status.Enabled || status.Metrics == nil || status.Metrics.CPU.UsagePercent != 12.5 {
		t.Fatalf("unexpected enabled status: %#v", status)
	}
	if settings.savedBy != 7 {
		t.Fatalf("updated by = %d, want 7", settings.savedBy)
	}

	if _, err := manager.SetEnabled(false, 8); err != nil {
		t.Fatalf("disable monitor: %v", err)
	}
	time.Sleep(25 * time.Millisecond)
	callsAfterDisable := collector.Calls()
	time.Sleep(50 * time.Millisecond)
	if calls := collector.Calls(); calls != callsAfterDisable {
		t.Fatalf("disabled monitor continued collecting: %d -> %d", callsAfterDisable, calls)
	}
	status = manager.Status()
	if status.Enabled || status.Metrics != nil {
		t.Fatalf("unexpected disabled status: %#v", status)
	}
}

func TestRoundPercent(t *testing.T) {
	for input, wanted := range map[float64]float64{-1: 0, 12.345: 12.35, 120: 100} {
		if actual := roundPercent(input); actual != wanted {
			t.Fatalf("roundPercent(%v) = %v, want %v", input, actual, wanted)
		}
	}
}

func TestMonitorBool(t *testing.T) {
	if !monitorBool(true) || !monitorBool("true") || !monitorBool([]byte("true")) || monitorBool("false") {
		t.Fatal("unexpected boolean conversion")
	}
}

func TestMonitorRoutesReadAndUpdateStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager, err := NewManager(ManagerOptions{
		Settings: &fakeSettings{}, Collector: &fakeCollector{}, Interval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()

	engine := gin.New()
	RegisterRoutes(engine.Group("/api"), manager, Dependencies{
		RequireAuth: func(c *gin.Context) { c.Next() },
		RequirePermission: func(...string) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
	})

	response := performRequest(engine, http.MethodGet, "/api/system-monitor", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Code int    `json:"code"`
		Data Status `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if envelope.Code != 0 || envelope.Data.Enabled || envelope.Data.Metrics != nil {
		t.Fatalf("unexpected disabled response: %#v", envelope)
	}

	response = performRequest(engine, http.MethodPatch, "/api/system-monitor/status", []byte(`{}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid PATCH status = %d, body = %s", response.Code, response.Body.String())
	}

	response = performRequest(engine, http.MethodPatch, "/api/system-monitor/status", []byte(`{"enabled":true}`))
	if response.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, body = %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode updated status: %v", err)
	}
	if envelope.Code != 0 || !envelope.Data.Enabled {
		t.Fatalf("unexpected enabled response: %#v", envelope)
	}
}

func performRequest(engine http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func waitForCollection(t *testing.T, collector *fakeCollector) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if collector.Calls() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("monitor did not collect a snapshot")
}
