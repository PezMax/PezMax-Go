package datum

import (
	"github.com/GoAdminGroup/go-admin/modules/db"
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
