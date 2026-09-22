package datum

import (
	"fmt"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"strings"
)

// NotificationRepo owns ptmj_notification. Desktop-facing reads filter by
// enablement (status = '0') and the publish window (null bounds = unbounded)
// and split by display mode: '0' popup, '1' scrolling banner. Type 4
// (material takedown) additionally targets a specific upload user.
type NotificationRepo struct {
	conn db.Connection
}

func NewNotificationRepo(conn db.Connection) *NotificationRepo {
	return &NotificationRepo{conn: conn}
}

// activeWhere is shared by popup and scroll reads. now is expected as a
// timestamp value bind.
const activeWhere = ` WHERE status = '0'
	AND (publish_start IS NULL OR publish_start <= ?)
	AND (publish_end IS NULL OR publish_end >= ?)`

func (r *NotificationRepo) FindByID(notifyID int64) (Notification, bool, error) {
	rows, err := r.conn.Query(`SELECT `+notificationColumns+` FROM ptmj_notification WHERE notify_id = ?`, notifyID)
	if err != nil {
		return Notification{}, false, err
	}
	if len(rows) == 0 {
		return Notification{}, false, nil
	}
	return scanNotification(rows[0]), true, nil
}

// ActivePopup lists popup notifications visible at now, highest sort first.
// userID scopes type-4 takedown notices to the affected uploader (<=0 shows
// only untargeted ones).
func (r *NotificationRepo) ActivePopup(now string, userID int64) ([]Notification, error) {
	rows, err := r.conn.Query(`SELECT `+notificationColumns+` FROM ptmj_notification`+activeWhere+`
		AND display_mode = '0'
		AND (notify_type <> '4' OR (upload_user_id IS NOT NULL AND upload_user_id = ?))
		ORDER BY sort DESC, notify_id DESC`, now, now, userID)
	if err != nil {
		return nil, err
	}
	return scanNotifications(rows), nil
}

// ActiveScroll lists scrolling-banner notifications visible at now.
func (r *NotificationRepo) ActiveScroll(now string) ([]Notification, error) {
	rows, err := r.conn.Query(`SELECT `+notificationColumns+` FROM ptmj_notification`+activeWhere+`
		AND display_mode = '1'
		ORDER BY sort DESC, notify_id DESC`, now, now)
	if err != nil {
		return nil, err
	}
	return scanNotifications(rows), nil
}

// AdminList pages every notification for the admin console.
func (r *NotificationRepo) AdminList(page, size int) (Page, error) {
	page, size = normalizePage(page, size)
	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_notification`)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])
	rows, err := r.conn.Query(`SELECT `+notificationColumns+` FROM ptmj_notification
		ORDER BY notify_id DESC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: scanNotifications(rows), Total: total, Page: page, PageSize: size}, nil
}

func scanNotifications(rows []map[string]interface{}) []Notification {
	notifications := make([]Notification, 0, len(rows))
	for _, row := range rows {
		notifications = append(notifications, scanNotification(row))
	}
	return notifications
}

// CreateTakedownNotification auto-generates the type-4 (资料下架) popup for
// the affected uploader once a report is confirmed. The unique material_id
// constraint keeps at most one takedown notification per material.
func (r *NotificationRepo) CreateTakedownNotification(materialID, uploadUserID int64, materialTitle string) error {
	if materialID <= 0 {
		return nil
	}
	title := fmt.Sprintf("您的资料《%s》因举报属实已下架", materialTitle)
	_, err := r.conn.Exec(`INSERT INTO ptmj_notification
		(notify_type, title, status, display_mode, sort, upload_user_id, material_id, material_title_snapshot,
		 create_by, create_time, update_by, update_time)
		VALUES ('4', ?, '0', '0', 0, ?, ?, ?, 'audit', CURRENT_TIMESTAMP, 'audit', CURRENT_TIMESTAMP)
		ON CONFLICT (material_id) DO NOTHING`,
		title, uploadUserID, materialID, materialTitle)
	return err
}

// NotificationPayload carries the writable columns; time/count fields are
// interface{} so the handler can pass nil for "no value".
type NotificationPayload struct {
	NotifyType            string
	Title                 string
	Content               string
	Status                string
	Sort                  int64
	DisplayMode           string
	FaultStartTime        interface{}
	FaultEndTime          interface{}
	MaintenanceStartTime  interface{}
	MaintenanceEndTime    interface{}
	RemindBeforeMinutes   interface{}
	UploadUserID          interface{}
	MaterialID            interface{}
	MaterialTitleSnapshot string
	PublishStart          interface{}
	PublishEnd            interface{}
	ScrollTimeInterval    interface{}
	Remark                string
}

func notificationInsertArgs(p NotificationPayload) []interface{} {
	return []interface{}{
		p.NotifyType, p.Title, p.Content, p.Status, p.Sort, p.DisplayMode,
		p.FaultStartTime, p.FaultEndTime, p.MaintenanceStartTime, p.MaintenanceEndTime,
		p.RemindBeforeMinutes, p.UploadUserID, p.MaterialID, p.MaterialTitleSnapshot,
		p.PublishStart, p.PublishEnd, p.ScrollTimeInterval, p.Remark,
	}
}

// Create inserts one notification; a duplicate material_id (type-4 takedown
// already exists) surfaces as a "duplicate key" error for the handler.
func (r *NotificationRepo) Create(p NotificationPayload) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_notification
		(notify_type, title, content, status, sort, display_mode,
		 fault_start_time, fault_end_time, maintenance_start_time, maintenance_end_time,
		 remind_before_minutes, upload_user_id, material_id, material_title_snapshot,
		 publish_start, publish_end, scroll_time_interval, remark,
		 create_by, create_time, update_by, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'datum', CURRENT_TIMESTAMP, 'datum', CURRENT_TIMESTAMP)
		RETURNING notify_id`, notificationInsertArgs(p)...)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("ptmj_notification insert returned no id")
	}
	return ScanInt64(rows[0]["notify_id"]), nil
}

// Update rewrites the writable columns of one notification.
func (r *NotificationRepo) Update(notifyID int64, p NotificationPayload) error {
	args := append([]interface{}{notifyID}, notificationInsertArgs(p)...)
	result, err := r.conn.Exec(`UPDATE ptmj_notification SET
		notify_type = ?, title = ?, content = ?, status = ?, sort = ?, display_mode = ?,
		fault_start_time = ?, fault_end_time = ?, maintenance_start_time = ?, maintenance_end_time = ?,
		remind_before_minutes = ?, upload_user_id = ?, material_id = ?, material_title_snapshot = ?,
		publish_start = ?, publish_end = ?, scroll_time_interval = ?, remark = ?,
		update_by = 'datum', update_time = CURRENT_TIMESTAMP
		WHERE notify_id = ?`, append(args, notifyID)...)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByIDs hard-deletes admin-managed notification rows.
func (r *NotificationRepo) DeleteByIDs(ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	result, err := r.conn.Exec(`DELETE FROM ptmj_notification WHERE notify_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// NotificationFilter narrows the desktop management list.
type NotificationFilter struct {
	Page         int
	PageSize     int
	NotifyType   string
	Title        string
	Status       string
	DisplayMode  string
	UploadUserID int64
	MaterialID   int64
}

// List filters and pages notifications for the desktop management view.
func (r *NotificationRepo) List(filter NotificationFilter) (Page, error) {
	page, size := normalizePage(filter.Page, filter.PageSize)
	conditions := []string{"1=1"}
	args := []interface{}{}
	if filter.NotifyType != "" {
		conditions = append(conditions, "notify_type = ?")
		args = append(args, filter.NotifyType)
	}
	if filter.Title != "" {
		conditions = append(conditions, "title ILIKE ?")
		args = append(args, "%"+filter.Title+"%")
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.DisplayMode != "" {
		conditions = append(conditions, "display_mode = ?")
		args = append(args, filter.DisplayMode)
	}
	if filter.UploadUserID > 0 {
		conditions = append(conditions, "upload_user_id = ?")
		args = append(args, filter.UploadUserID)
	}
	if filter.MaterialID > 0 {
		conditions = append(conditions, "material_id = ?")
		args = append(args, filter.MaterialID)
	}
	where := "WHERE " + strings.Join(conditions, " AND ")

	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_notification `+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])
	queryArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	rows, err := r.conn.Query(`SELECT `+notificationColumns+` FROM ptmj_notification `+where+`
		ORDER BY notify_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: scanNotifications(rows), Total: total, Page: page, PageSize: size}, nil
}
