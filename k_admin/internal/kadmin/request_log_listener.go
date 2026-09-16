package kadmin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

const (
	requestLogBodyLimit  = 32 * 1024
	requestLogInputLimit = 16 * 1024
	requestLogQueueSize  = 1024
	requestLogWorkers    = 2
)

// metricSink receives sanitized request events for derived aggregations such
// as the interface load ranking. It is optional and must never block the log
// writer.
type metricSink interface {
	Observe(models.OperationLogEvent)
}

// RequestLogListener captures every Gin request and persists a sanitized event.
type RequestLogListener struct {
	auth      *authService
	conn      db.Connection
	events    chan models.OperationLogEvent
	metrics   metricSink
	closeOnce sync.Once
	workers   sync.WaitGroup
}

// NewRequestLogListener starts the bounded database writer queue.
func NewRequestLogListener(conn db.Connection) *RequestLogListener {
	listener := &RequestLogListener{
		auth:   newAuthServiceFromEnv(),
		conn:   conn,
		events: make(chan models.OperationLogEvent, requestLogQueueSize),
	}
	listener.workers.Add(requestLogWorkers)
	for i := 0; i < requestLogWorkers; i++ {
		go listener.writeEvents()
	}
	return listener
}

// Middleware records a request after downstream handlers have completed.
func (l *RequestLogListener) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		requestID := requestLogIdentifier(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestLogID()
		}
		traceID := requestLogIdentifier(c.GetHeader("X-Trace-ID"))
		if traceID == "" {
			traceID = requestID
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Set("trace_id", traceID)

		input := captureRequestLogInput(c.Request)
		c.Next()
		if shouldSkipRequestLog(c.Request.Method, c.Request.URL.Path, c.Writer.Status()) {
			return
		}

		event := l.requestEvent(c, startedAt, requestID, traceID, input)
		l.events <- event
	}
}

// AttachMetricSink wires an optional derived aggregation consumer. Requests
// observed before attaching are only persisted, never aggregated.
func (l *RequestLogListener) AttachMetricSink(sink metricSink) {
	l.metrics = sink
}

// Close flushes queued events before the database connection is closed.
func (l *RequestLogListener) Close() {
	if l == nil {
		return
	}
	l.closeOnce.Do(func() {
		close(l.events)
		l.workers.Wait()
	})
}

func (l *RequestLogListener) writeEvents() {
	defer l.workers.Done()
	for event := range l.events {
		if _, err := models.OperationLog().SetConn(l.conn).NewEvent(event); err != nil {
			log.Printf("写入请求日志失败：%v", err)
		}
		if l.metrics != nil {
			l.metrics.Observe(event)
		}
	}
}

func (l *RequestLogListener) requestEvent(
	c *gin.Context,
	startedAt time.Time,
	requestID string,
	traceID string,
	input string,
) models.OperationLogEvent {
	statusCode := int64(c.Writer.Status())
	durationMs := time.Since(startedAt).Milliseconds()
	successful := statusCode < http.StatusBadRequest
	path := truncateRunes(c.Request.URL.Path, 2048)
	route := path

	metadata, _ := json.Marshal(map[string]interface{}{
		"contentLength": c.Request.ContentLength,
		"contentType":   requestLogContentType(c.Request.Header.Get("Content-Type")),
		"referer":       sanitizedReferer(c.Request.Referer()),
		"responseBytes": c.Writer.Size(),
		"route":         route,
	})

	eventID := newRequestLogID()
	eventType := models.OperationLogEventTypeRequest
	if strings.HasPrefix(path, "/api/auth") || strings.HasPrefix(path, "/admin/login") {
		eventType = models.OperationLogEventTypeAuth
	}
	level := models.OperationLogLevelInfo
	if statusCode >= http.StatusInternalServerError {
		level = models.OperationLogLevelError
	} else if statusCode >= http.StatusBadRequest {
		level = models.OperationLogLevelWarn
	}

	event := models.OperationLogEvent{
		Path:       path,
		Method:     truncateRunes(c.Request.Method, 16),
		Ip:         truncateRunes(c.ClientIP(), 45),
		Input:      input,
		EventId:    eventID,
		EventType:  eventType,
		Level:      level,
		Source:     requestLogSource(path),
		Module:     requestLogModule(path),
		Action:     truncateRunes(route, 100),
		Message:    fmt.Sprintf("%s %s -> %d", c.Request.Method, path, statusCode),
		RequestId:  requestID,
		TraceId:    traceID,
		StatusCode: &statusCode,
		Success:    &successful,
		DurationMs: &durationMs,
		UserAgent:  truncateRunes(c.Request.UserAgent(), 1024),
		Metadata:   metadata,
		OccurredAt: &startedAt,
	}
	if statusCode >= http.StatusBadRequest {
		event.ErrorCode = fmt.Sprintf("http_%d", statusCode)
		event.ErrorMessage = http.StatusText(int(statusCode))
	}
	if userID, ok := requestLogUserID(c, l.auth); ok {
		event.UserId = &userID
	}
	if value, ok := c.Get("vben_user"); ok {
		if user, valid := value.(models.UserModel); valid {
			event.ActorName = truncateRunes(firstNonEmpty(user.Name, user.UserName), 100)
		}
	}
	return event
}

func shouldSkipRequestLog(method string, path string, statusCode int) bool {
	if statusCode >= http.StatusBadRequest {
		return false
	}
	if method == http.MethodGet && path == "/api/logs" {
		return true
	}
	// Keep the load ranking page's own polls out of the aggregated metrics.
	if method == http.MethodGet && (path == "/api/load-ranking/status" || path == "/api/load-ranking/rankings") {
		return true
	}
	return false
}

func requestLogUserID(c *gin.Context, auth *authService) (int64, bool) {
	if value, ok := c.Get("vben_user_id"); ok {
		if userID, valid := value.(int64); valid && userID > 0 {
			return userID, true
		}
	}
	if auth == nil {
		return 0, false
	}
	claims, err := auth.parseAccessToken(tokenFromRequest(c))
	if err != nil || claims.UserID <= 0 {
		return 0, false
	}
	return claims.UserID, true
}

func captureRequestLogInput(request *http.Request) string {
	result := make(map[string]interface{})
	if query := sanitizedValues(request.URL.Query()); len(query) > 0 {
		result["query"] = query
	}

	contentType := requestLogContentType(request.Header.Get("Content-Type"))
	if request.Body != nil && request.ContentLength > 0 && request.ContentLength <= requestLogBodyLimit &&
		(contentType == "application/json" || contentType == "application/x-www-form-urlencoded") {
		body, err := io.ReadAll(request.Body)
		if err == nil {
			request.Body = io.NopCloser(bytes.NewReader(body))
			if contentType == "application/json" {
				var payload interface{}
				decoder := json.NewDecoder(bytes.NewReader(body))
				decoder.UseNumber()
				if decoder.Decode(&payload) == nil {
					result["body"] = sanitizeJSONValue(payload, 0)
				}
			} else if values, parseErr := url.ParseQuery(string(body)); parseErr == nil {
				result["body"] = sanitizedValues(values)
			}
		}
	}

	if len(result) == 0 {
		return ""
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	if len(encoded) > requestLogInputLimit {
		encoded, _ = json.Marshal(map[string]interface{}{
			"capturedBytes": len(encoded),
			"truncated":     true,
		})
	}
	return string(encoded)
}

func sanitizedValues(values url.Values) map[string]interface{} {
	result := make(map[string]interface{}, len(values))
	for key, items := range values {
		if sensitiveLogField(key) {
			result[key] = "[REDACTED]"
			continue
		}
		clean := make([]string, 0, len(items))
		for _, item := range items {
			clean = append(clean, truncateRunes(item, 512))
		}
		if len(clean) == 1 {
			result[key] = clean[0]
		} else {
			result[key] = clean
		}
	}
	return result
}

func sanitizeJSONValue(value interface{}, depth int) interface{} {
	if depth >= 12 {
		return "[TRUNCATED]"
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		configuredKey := ""
		if value, ok := typed["key"].(string); ok {
			configuredKey = value
		}
		result := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			if sensitiveLogField(key) || key == "value" && sensitiveLogField(configuredKey) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = sanitizeJSONValue(child, depth+1)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(typed))
		for i, child := range typed {
			result[i] = sanitizeJSONValue(child, depth+1)
		}
		return result
	case string:
		return truncateRunes(typed, 512)
	default:
		return value
	}
}

func sensitiveLogField(field string) bool {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, field)
	for _, keyword := range []string{"password", "passwd", "token", "authorization", "cookie", "secret", "apikey", "captcha", "content"} {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}

func requestLogIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return ""
	}
	for _, r := range value {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_.:", r)) {
			return ""
		}
	}
	return value
}

func newRequestLogID() string {
	value, err := randomHex(16)
	if err == nil {
		return value
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func requestLogSource(path string) string {
	switch {
	case strings.HasPrefix(path, "/api"):
		return "vbenapi"
	case strings.HasPrefix(path, "/admin"):
		return "goadmin"
	default:
		return "server"
	}
}

func requestLogModule(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "server"
	}
	if (parts[0] == "api" || parts[0] == "admin") && len(parts) > 1 {
		return truncateRunes(parts[1], 100)
	}
	return truncateRunes(parts[0], 100)
}

func requestLogContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return truncateRunes(value, 100)
	}
	return mediaType
}

func sanitizedReferer(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return truncateRunes(parsed.Scheme+"://"+parsed.Host+parsed.EscapedPath(), 512)
}

func truncateRunes(value string, max int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
