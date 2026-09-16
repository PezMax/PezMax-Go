package kadmin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/models"
	"github.com/gin-gonic/gin"
)

const (
	businessAuditQueueSize = 512
	businessAuditBodyLimit = 128 << 10
)

type businessAuditRecorder struct {
	conn      db.Connection
	events    chan models.OperationLogEvent
	closeOnce sync.Once
	workers   sync.WaitGroup
}

type businessAuditDescriptor struct {
	Action     string
	Resource   string
	ResourceID string
	// Loader optionally loads the resource snapshot for before/after diffing.
	// When nil, the built-in snapshot switch is consulted.
	Loader AuditSnapshotLoader
}

func newBusinessAuditRecorder(conn db.Connection) *businessAuditRecorder {
	recorder := &businessAuditRecorder{conn: conn, events: make(chan models.OperationLogEvent, businessAuditQueueSize)}
	recorder.workers.Add(1)
	go recorder.writeEvents()
	return recorder
}

func (r *businessAuditRecorder) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		close(r.events)
		r.workers.Wait()
	})
}

func (r *businessAuditRecorder) Record(event models.OperationLogEvent) {
	if r == nil {
		return
	}
	select {
	case r.events <- event:
	default:
		log.Print("业务审计队列已满，本次审计事件被丢弃")
	}
}

func (r *businessAuditRecorder) writeEvents() {
	defer r.workers.Done()
	for event := range r.events {
		if _, err := models.OperationLog().SetConn(r.conn).NewEvent(event); err != nil {
			log.Printf("写入业务审计失败：%v", err)
		}
	}
}

func (s *Store) businessAuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		descriptor, ok := describeBusinessMutation(c.Request.Method, c.Request.URL.Path)
		if !ok {
			c.Next()
			return
		}
		startedAt := time.Now()
		before := s.loadBusinessAuditSnapshot(descriptor, c.Request.URL.Path)
		input := captureRequestLogInput(c.Request)
		capture := &boundedResponseWriter{ResponseWriter: c.Writer, limit: businessAuditBodyLimit}
		c.Writer = capture
		c.Next()
		if c.Writer.Header().Get("X-Idempotent-Replay") == "true" {
			return
		}

		after, resultSummary := auditResponse(capture.body.Bytes())
		status := c.Writer.Status()
		successful := status < http.StatusBadRequest
		if successful {
			descriptor.ResourceID = auditResourceID(descriptor.ResourceID, after)
			if descriptor.Action == "delete" {
				after = nil
			} else if snapshot := s.loadBusinessAuditSnapshot(descriptor, c.Request.URL.Path); snapshot != nil {
				after = snapshot
			}
		}
		metadata := map[string]interface{}{
			"resource":      descriptor.Resource,
			"resourceId":    descriptor.ResourceID,
			"operation":     descriptor.Action,
			"before":        sanitizeAuditValue(before),
			"after":         sanitizeAuditValue(after),
			"changes":       auditChanges(before, after),
			"resultSummary": resultSummary,
		}
		metadataJSON, _ := json.Marshal(metadata)
		durationMs := time.Since(startedAt).Milliseconds()
		statusCode := int64(status)
		level := models.OperationLogLevelInfo
		if status >= http.StatusInternalServerError {
			level = models.OperationLogLevelError
		} else if status >= http.StatusBadRequest {
			level = models.OperationLogLevelWarn
		}
		userID, _ := auditUserID(c)
		requestID, _ := c.Get("request_id")
		traceID, _ := c.Get("trace_id")
		event := models.OperationLogEvent{
			UserId:     userID,
			Path:       truncateRunes(c.Request.URL.Path, 2048),
			Method:     truncateRunes(c.Request.Method, 16),
			Ip:         truncateRunes(c.ClientIP(), 45),
			Input:      input,
			EventId:    newRequestLogID(),
			EventType:  models.OperationLogEventTypeAudit,
			Level:      level,
			Source:     "kadmin",
			Module:     descriptor.Resource,
			Action:     descriptor.Action,
			Message:    fmt.Sprintf("%s %s %s -> %d", descriptor.Action, descriptor.Resource, descriptor.ResourceID, status),
			ActorName:  auditActorName(c),
			RequestId:  fmt.Sprint(requestID),
			TraceId:    fmt.Sprint(traceID),
			StatusCode: &statusCode,
			Success:    &successful,
			DurationMs: &durationMs,
			UserAgent:  truncateRunes(c.Request.UserAgent(), 1024),
			Metadata:   metadataJSON,
			OccurredAt: &startedAt,
		}
		if !successful {
			event.ErrorCode = fmt.Sprintf("http_%d", status)
			event.ErrorMessage = resultSummary
		}
		s.audit.Record(event)
	}
}

func describeBusinessMutation(method, path string) (businessAuditDescriptor, bool) {
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
		return businessAuditDescriptor{}, false
	}
	parts := splitAPIPath(path)
	if len(parts) == 0 {
		return businessAuditDescriptor{}, false
	}
	if resolver := businessAuditResolverFor(parts[0]); resolver != nil {
		if descriptor, ok := resolver(method, parts); ok {
			return descriptor, true
		}
	}
	action := strings.ToLower(method)
	resource := parts[0]
	resourceID := ""
	if len(parts) > 1 {
		resourceID = parts[1]
	}
	switch parts[0] {
	case "users":
		resource = "user"
		if len(parts) == 2 && parts[1] == "import" {
			action, resourceID = "import", "batch"
		} else if len(parts) >= 3 {
			resourceID = parts[1]
			action = parts[2]
		} else if method == http.MethodPost {
			action, resourceID = "create", "new"
		}
	case "rbac":
		if len(parts) < 2 || (parts[1] != "departments" && parts[1] != "roles") {
			return businessAuditDescriptor{}, false
		}
		resource = strings.TrimSuffix(parts[1], "s")
		if len(parts) >= 3 {
			resourceID = parts[2]
		}
		if len(parts) >= 4 {
			action = "assign-" + parts[3]
		} else if method == http.MethodPost {
			action, resourceID = "create", "new"
		}
	case "admin-menus":
		resource = "menu"
		if len(parts) == 1 && method == http.MethodPost {
			action, resourceID = "create", "new"
		} else if len(parts) == 1 {
			action, resourceID = "reorder", "all"
		}
	case "dictionaries":
		if len(parts) < 2 || (parts[1] != "types" && parts[1] != "data") {
			return businessAuditDescriptor{}, false
		}
		resource = "dictionary-" + strings.TrimSuffix(parts[1], "s")
		if len(parts) >= 3 {
			resourceID = parts[2]
		} else {
			resourceID = "new"
		}
	case "system":
		if len(parts) < 2 || parts[1] != "config" {
			return businessAuditDescriptor{}, false
		}
		resource, resourceID, action = "system-config", "singleton", "update"
	case "files":
		if method != http.MethodDelete || len(parts) != 2 {
			return businessAuditDescriptor{}, false
		}
		resource, resourceID, action = "file", parts[1], "delete"
	case "jobs":
		resource = "job"
		if len(parts) == 1 && method == http.MethodPost {
			action, resourceID = "create", "new"
		} else if len(parts) >= 2 {
			resourceID = parts[1]
			if len(parts) >= 3 {
				action = parts[2]
			}
		}
	default:
		return businessAuditDescriptor{}, false
	}
	if action == strings.ToLower(http.MethodPost) {
		action = "create"
	} else if action == strings.ToLower(http.MethodPut) || action == strings.ToLower(http.MethodPatch) {
		action = "update"
	} else if action == strings.ToLower(http.MethodDelete) {
		action = "delete"
	}
	return businessAuditDescriptor{Action: action, Resource: resource, ResourceID: resourceID}, true
}

func (s *Store) loadBusinessAuditSnapshot(descriptor businessAuditDescriptor, path string) interface{} {
	if descriptor.Loader != nil {
		return descriptor.Loader(descriptor.ResourceID)
	}
	id, _ := strconv.ParseInt(descriptor.ResourceID, 10, 64)
	switch descriptor.Resource {
	case "user":
		if id > 0 {
			item, _ := s.loadManagedUser(id)
			return item
		}
	case "department":
		if id > 0 {
			item, _ := s.loadRBACDepartment(id)
			return item
		}
	case "role":
		if id > 0 {
			item, _ := s.loadRBACRole(id)
			return item
		}
	case "menu":
		if descriptor.ResourceID == "all" {
			items, _ := s.loadManagedMenus()
			return items
		}
		if id > 0 {
			item, _ := s.loadManagedMenu(id)
			return item
		}
	case "dictionary-type":
		if id > 0 {
			item, _ := s.loadDictionaryType(id)
			return item
		}
	case "dictionary-data":
		if id > 0 {
			item, _ := s.loadDictionaryDataItem(id)
			return item
		}
	case "system-config":
		values, _ := s.readSystemConfig()
		return values
	case "file":
		return s.queryAuditRow("SELECT id, original_name, content_type, size, storage, purpose, visibility, status, created_by FROM public.kadmin_files WHERE id = ?", id)
	case "job":
		return s.queryAuditRow("SELECT id, name, handler, cron_expression, description, status, built_in, created_by FROM public.kadmin_jobs WHERE id = ?", id)
	}
	return nil
}

func (s *Store) queryAuditRow(query string, args ...interface{}) interface{} {
	rows, err := s.conn.Query(query, args...)
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func auditResponse(body []byte) (interface{}, string) {
	if len(body) == 0 {
		return nil, ""
	}
	var envelope struct {
		Data    interface{} `json:"data"`
		Message string      `json:"message"`
		Msg     string      `json:"msg"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return nil, truncateRunes(string(body), 512)
	}
	message := firstNonEmpty(envelope.Message, envelope.Msg)
	return envelope.Data, truncateRunes(message, 512)
}

func auditResourceID(current string, value interface{}) string {
	if current != "" && current != "new" {
		return current
	}
	item, ok := value.(map[string]interface{})
	if !ok {
		return current
	}
	switch id := item["id"].(type) {
	case float64:
		if id > 0 {
			return strconv.FormatInt(int64(id), 10)
		}
	case json.Number:
		if parsed, err := id.Int64(); err == nil && parsed > 0 {
			return strconv.FormatInt(parsed, 10)
		}
	case string:
		if parsed, err := strconv.ParseInt(id, 10, 64); err == nil && parsed > 0 {
			return strconv.FormatInt(parsed, 10)
		}
	}
	return current
}

func sanitizeAuditValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var generic interface{}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if decoder.Decode(&generic) != nil {
		return nil
	}
	return sanitizeJSONValue(generic, 0)
}

func auditChanges(before, after interface{}) interface{} {
	beforeMap, beforeOK := sanitizeAuditValue(before).(map[string]interface{})
	afterMap, afterOK := sanitizeAuditValue(after).(map[string]interface{})
	if !beforeOK || !afterOK {
		return nil
	}
	changes := make(map[string]interface{})
	for key, afterValue := range afterMap {
		if key == "createdAt" || key == "updatedAt" {
			continue
		}
		beforeValue, exists := beforeMap[key]
		if !exists || !reflect.DeepEqual(beforeValue, afterValue) {
			changes[key] = map[string]interface{}{"before": beforeValue, "after": afterValue}
		}
	}
	for key, beforeValue := range beforeMap {
		if key == "createdAt" || key == "updatedAt" {
			continue
		}
		if _, exists := afterMap[key]; !exists {
			changes[key] = map[string]interface{}{"before": beforeValue, "after": nil}
		}
	}
	if len(changes) == 0 {
		return nil
	}
	return changes
}

func auditUserID(c *gin.Context) (*int64, bool) {
	value, ok := c.Get("vben_user_id")
	if !ok {
		return nil, false
	}
	id, ok := value.(int64)
	if !ok || id <= 0 {
		return nil, false
	}
	return &id, true
}

func auditActorName(c *gin.Context) string {
	value, ok := c.Get("vben_user")
	if !ok {
		return ""
	}
	user, ok := value.(models.UserModel)
	if !ok {
		return ""
	}
	return truncateRunes(firstNonEmpty(user.Name, user.UserName), 100)
}
