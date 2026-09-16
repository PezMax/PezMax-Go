package notifications

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

var (
	errNotificationNotFound = errors.New("notification not found")
	errInvalidType          = errors.New("notification type must be info, success or warning")
)

// repository persists in-site notifications.
type repository struct {
	conn db.Connection
}

func newRepository(conn db.Connection) *repository {
	return &repository{conn: conn}
}

// Notifier is the exported writer facade for in-tree event producers, e.g.
// user management pushes a notification when a new user is created.
type Notifier struct {
	repo *repository
}

// NewNotifier builds a notification writer on the shared connection.
func NewNotifier(conn db.Connection) *Notifier {
	return &Notifier{repo: newRepository(conn)}
}

// Push persists one in-site notification.
func (n *Notifier) Push(payload Payload) (Notification, error) {
	return n.repo.create(payload)
}

func (r *repository) create(payload Payload) (Notification, error) {
	notificationType := strings.TrimSpace(payload.Type)
	if notificationType == "" {
		notificationType = TypeInfo
	}
	if notificationType != TypeInfo && notificationType != TypeSuccess && notificationType != TypeWarning {
		return Notification{}, errInvalidType
	}
	rows, err := r.conn.Query(`INSERT INTO public.kadmin_notifications (title, content, link, type)
		VALUES (?, ?, ?, ?) RETURNING id`,
		payload.Title, payload.Content, payload.Link, notificationType)
	if err != nil {
		return Notification{}, fmt.Errorf("create notification: %w", err)
	}
	return r.mustFind(toInt64(rows[0]["id"]))
}

func (r *repository) mustFind(id int64) (Notification, error) {
	item, found, err := r.findByID(id)
	if err != nil {
		return Notification{}, err
	}
	if !found {
		return Notification{}, errNotificationNotFound
	}
	return item, nil
}

func (r *repository) findByID(id int64) (Notification, bool, error) {
	rows, err := r.conn.Query(`SELECT id, title, content, link, type, is_read, created_at
		FROM public.kadmin_notifications WHERE id = ?`, id)
	if err != nil {
		return Notification{}, false, err
	}
	if len(rows) == 0 {
		return Notification{}, false, nil
	}
	return mapNotification(rows[0]), true, nil
}

func (r *repository) list(filter Filter) (Page, error) {
	where, args := notificationWhere(filter)
	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM public.kadmin_notifications `+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = toInt64(countRows[0]["count"])
	}
	unreadRows, err := r.conn.Query(`SELECT count(*) AS count FROM public.kadmin_notifications WHERE is_read = FALSE`)
	if err != nil {
		return Page{}, err
	}
	unread := int64(0)
	if len(unreadRows) > 0 {
		unread = toInt64(unreadRows[0]["count"])
	}
	queryArgs := append(append([]interface{}{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.conn.Query(`SELECT id, title, content, link, type, is_read, created_at
		FROM public.kadmin_notifications `+where+`
		ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	items := make([]Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapNotification(row))
	}
	return Page{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize, Unread: unread}, nil
}

func notificationWhere(filter Filter) (string, []interface{}) {
	if !filter.UnreadOnly {
		return "", []interface{}{}
	}
	return "WHERE is_read = FALSE", []interface{}{}
}

func (r *repository) markRead(id int64) (Notification, error) {
	if _, found, err := r.findByID(id); err != nil {
		return Notification{}, err
	} else if !found {
		return Notification{}, errNotificationNotFound
	}
	if _, err := r.conn.Exec(`UPDATE public.kadmin_notifications SET is_read = TRUE WHERE id = ?`, id); err != nil {
		return Notification{}, fmt.Errorf("mark notification read: %w", err)
	}
	return r.mustFind(id)
}

func (r *repository) markAllRead() error {
	if _, err := r.conn.Exec(`UPDATE public.kadmin_notifications SET is_read = TRUE WHERE is_read = FALSE`); err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

func (r *repository) delete(id int64) error {
	result, err := r.conn.Exec(`DELETE FROM public.kadmin_notifications WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete notification: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errNotificationNotFound
	}
	return nil
}

// clearRead deletes every read message, keeping unread ones.
func (r *repository) clearRead() error {
	if _, err := r.conn.Exec(`DELETE FROM public.kadmin_notifications WHERE is_read = TRUE`); err != nil {
		return fmt.Errorf("clear read notifications: %w", err)
	}
	return nil
}

func mapNotification(row map[string]interface{}) Notification {
	return Notification{
		ID:        toInt64(row["id"]),
		Title:     toString(row["title"]),
		Content:   toString(row["content"]),
		Link:      toString(row["link"]),
		Type:      toString(row["type"]),
		IsRead:    toBool(row["is_read"]),
		CreatedAt: toTimeString(row["created_at"]),
	}
}

func toInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
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
