package datum

import (
	"fmt"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

// BookmarkRepo owns ptmj_bookmark access. Anonymous reads are restricted to
// approved-and-alive rows (status = 1 AND del_flag = 0).
type BookmarkRepo struct {
	conn db.Connection
}

func NewBookmarkRepo(conn db.Connection) *BookmarkRepo {
	return &BookmarkRepo{conn: conn}
}

// BookmarkPayload carries the writable fields of a bookmark row.
type BookmarkPayload struct {
	UserID       int64
	URL          string
	Title        string
	Description  string
	CoverImage   string
	Subject      string
	ResourceType string
	Collection   string
	Status       int64
	Remark       string
}

func (r *BookmarkRepo) Insert(payload BookmarkPayload) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_bookmark
		(user_id, url, title, description, cover_image, subject, resource_type, collection, status, del_flag, create_by, create_time, update_by, update_time, remark)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING id`,
		payload.UserID, payload.URL, payload.Title, payload.Description, payload.CoverImage,
		payload.Subject, payload.ResourceType, payload.Collection, payload.Status,
		payload.UserID, payload.UserID, payload.Remark)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("ptmj_bookmark insert returned no id")
	}
	return ScanInt64(rows[0]["id"]), nil
}

func (r *BookmarkRepo) UpdateOwner(id, userID int64, payload BookmarkPayload) error {
	result, err := r.conn.Exec(`UPDATE ptmj_bookmark SET
		url = ?, title = ?, description = ?, cover_image = ?, subject = ?, resource_type = ?, collection = ?, remark = ?,
		update_by = ?, update_time = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ? AND del_flag = 0`,
		payload.URL, payload.Title, payload.Description, payload.CoverImage, payload.Subject,
		payload.ResourceType, payload.Collection, payload.Remark, payload.UserID, id, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookmarkRepo) UpdateCover(id, userID int64, coverImage string) error {
	result, err := r.conn.Exec(`UPDATE ptmj_bookmark SET cover_image = ?, update_by = ?, update_time = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ? AND del_flag = 0`, coverImage, userID, id, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookmarkRepo) MarkDeleted(id, userID int64) error {
	result, err := r.conn.Exec(`UPDATE ptmj_bookmark SET del_flag = 1, update_time = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ? AND del_flag = 0`, id, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookmarkRepo) SetStatus(id int64, status int64) error {
	_, err := r.conn.Exec(`UPDATE ptmj_bookmark SET status = ?, update_time = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

func (r *BookmarkRepo) FindByID(id int64) (Bookmark, bool, error) {
	rows, err := r.conn.Query(`SELECT `+bookmarkColumns+` FROM ptmj_bookmark WHERE id = ? AND del_flag = 0`, id)
	if err != nil {
		return Bookmark{}, false, err
	}
	if len(rows) == 0 {
		return Bookmark{}, false, nil
	}
	return scanBookmark(rows[0]), true, nil
}

func (r *BookmarkRepo) List(filter BookmarkFilter) (Page, error) {
	page, size := filter.page()
	where, args := bookmarkFilterWhere(filter)

	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_bookmark `+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])

	queryArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	rows, err := r.conn.Query(`SELECT `+bookmarkColumns+` FROM ptmj_bookmark `+where+`
		ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	bookmarks := make([]Bookmark, 0, len(rows))
	for _, row := range rows {
		bookmarks = append(bookmarks, scanBookmark(row))
	}
	return Page{Items: bookmarks, Total: total, Page: page, PageSize: size}, nil
}

func bookmarkFilterWhere(filter BookmarkFilter) (string, []interface{}) {
	conditions := []string{"del_flag = 0"}
	args := []interface{}{}
	if filter.OnlyApproved {
		conditions = append(conditions, "status = 1")
	}
	if strings.TrimSpace(filter.Subject) != "" {
		conditions = append(conditions, "subject = ?")
		args = append(args, strings.TrimSpace(filter.Subject))
	}
	if strings.TrimSpace(filter.ResourceType) != "" {
		conditions = append(conditions, "resource_type = ?")
		args = append(args, strings.TrimSpace(filter.ResourceType))
	}
	if strings.TrimSpace(filter.Collection) != "" {
		conditions = append(conditions, "collection = ?")
		args = append(args, strings.TrimSpace(filter.Collection))
	}
	if filter.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		conditions = append(conditions, "(title ILIKE ? OR description ILIKE ? OR url ILIKE ?)")
		pattern := "%" + keyword + "%"
		args = append(args, pattern, pattern, pattern)
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}
