package datum

import (
	"fmt"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

// EbookRepo owns ptmj_ebook access for the desktop contract: anonymous reads
// are restricted to approved-and-alive rows（ebook_status=1, del_flag=0），
// owner reads see every status — semantics aligned with ptmj_file.
type EbookRepo struct {
	conn db.Connection
}

func NewEbookRepo(conn db.Connection) *EbookRepo {
	return &EbookRepo{conn: conn}
}

// EbookFilter narrows ebook lists. OnlyApproved enforces the anonymous
// desktop contract: ebook_status = 1 AND del_flag = 0.
type EbookFilter struct {
	Page         int
	PageSize     int
	Keyword      string // 书名/作者/出版社 联想
	EbookSubject string
	EbookType    int64
	UserID       int64 // >0 restricts to one owner
	OnlyApproved bool
}

const ebookColumns = `ebook_id, user_id, ebook_name, author, publisher, cover_url, ebook_url,
	ebook_size, ebook_format, ebook_subject, ebook_type, reviewer, ebook_status, del_flag,
	create_by, create_time, update_by, update_time, remark`

func (r *EbookRepo) FindByID(ebookID int64) (Ebook, bool, error) {
	rows, err := r.conn.Query(`SELECT `+ebookColumns+` FROM ptmj_ebook WHERE ebook_id = ? AND del_flag = 0`, ebookID)
	if err != nil {
		return Ebook{}, false, err
	}
	if len(rows) == 0 {
		return Ebook{}, false, nil
	}
	return ScanEbook(rows[0]), true, nil
}

// List filters and pages ebook rows ordered by newest first.
func (r *EbookRepo) List(filter EbookFilter) (Page, error) {
	page, size := normalizePage(filter.Page, filter.PageSize)
	where, args := ebookFilterWhere(filter)

	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_ebook `+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])

	queryArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	rows, err := r.conn.Query(`SELECT `+ebookColumns+` FROM ptmj_ebook `+where+`
		ORDER BY ebook_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	ebooks := make([]Ebook, 0, len(rows))
	for _, row := range rows {
		ebooks = append(ebooks, ScanEbook(row))
	}
	return Page{Items: ebooks, Total: total, Page: page, PageSize: size}, nil
}

func ebookFilterWhere(filter EbookFilter) (string, []interface{}) {
	conditions := []string{"del_flag = 0"}
	args := []interface{}{}
	if filter.OnlyApproved {
		conditions = append(conditions, "ebook_status = 1")
	}
	if strings.TrimSpace(filter.EbookSubject) != "" {
		conditions = append(conditions, "ebook_subject = ?")
		args = append(args, strings.TrimSpace(filter.EbookSubject))
	}
	if filter.EbookType > 0 {
		conditions = append(conditions, "ebook_type = ?")
		args = append(args, filter.EbookType)
	}
	if filter.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		conditions = append(conditions, "(ebook_name ILIKE ? OR author ILIKE ? OR publisher ILIKE ?)")
		pattern := "%" + keyword + "%"
		args = append(args, pattern, pattern, pattern)
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

// EbookSubjectCount feeds the desktop ebook tree: subject → books.
type EbookSubjectCount struct {
	Subject string
	Total   int64
}

// Subjects lists distinct subjects of approved ebooks with counts.
func (r *EbookRepo) Subjects() ([]EbookSubjectCount, error) {
	rows, err := r.conn.Query(`SELECT ebook_subject, count(*) AS count FROM ptmj_ebook
		WHERE ebook_status = 1 AND del_flag = 0 GROUP BY ebook_subject ORDER BY ebook_subject`)
	if err != nil {
		return nil, err
	}
	subjects := make([]EbookSubjectCount, 0, len(rows))
	for _, row := range rows {
		subjects = append(subjects, EbookSubjectCount{Subject: ScanString(row["ebook_subject"]), Total: ScanInt64(row["count"])})
	}
	return subjects, nil
}

// EbookDownloadRepo writes ptmj_ebook_download rows: one per successful
// ebook stream download（与 ptmj_file_download 同一口径）.
type EbookDownloadRepo struct {
	conn db.Connection
}

func NewEbookDownloadRepo(conn db.Connection) *EbookDownloadRepo {
	return &EbookDownloadRepo{conn: conn}
}

func (r *EbookDownloadRepo) Create(ebookID, userID int64) error {
	_, err := r.conn.Exec(`INSERT INTO ptmj_ebook_download (ebook_id, user_id, create_by, create_time, update_by, update_time)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP)`, ebookID, userID, userID, userID)
	return err
}

// EbookFavoriteRepo owns ptmj_ebook_favorite access: a pure (ebook_id, user_id)
// association like ptmj_file_favorite.
type EbookFavoriteRepo struct {
	conn db.Connection
}

func NewEbookFavoriteRepo(conn db.Connection) *EbookFavoriteRepo {
	return &EbookFavoriteRepo{conn: conn}
}

func (r *EbookFavoriteRepo) Add(ebookID, userID int64) error {
	_, err := r.conn.Exec(`INSERT INTO ptmj_ebook_favorite (ebook_id, user_id) VALUES (?, ?)`, ebookID, userID)
	return err
}

func (r *EbookFavoriteRepo) Remove(ebookID, userID int64) error {
	result, err := r.conn.Exec(`DELETE FROM ptmj_ebook_favorite WHERE ebook_id = ? AND user_id = ?`, ebookID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EbookFavoriteRepo) Exists(ebookID, userID int64) (bool, error) {
	rows, err := r.conn.Query(`SELECT 1 AS one FROM ptmj_ebook_favorite WHERE ebook_id = ? AND user_id = ?`, ebookID, userID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

// ListByUser returns every ebook id the user favorited（收藏状态集合用）.
func (r *EbookFavoriteRepo) ListByUser(userID int64) ([]int64, error) {
	rows, err := r.conn.Query(`SELECT ebook_id FROM ptmj_ebook_favorite WHERE user_id = ? ORDER BY ebook_id DESC`, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, ScanInt64(row["ebook_id"]))
	}
	return ids, nil
}

// EbookReportRepo writes ptmj_ebook_report rows: one pending report per
// (user, ebook) pair（uk_user_ebook 唯一约束由调用方以 duplicate key 兜底）.
type EbookReportRepo struct {
	conn db.Connection
}

func NewEbookReportRepo(conn db.Connection) *EbookReportRepo {
	return &EbookReportRepo{conn: conn}
}

func (r *EbookReportRepo) Create(ebookID, userID int64, reason, remark string) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_ebook_report (ebook_id, user_id, reason, result, create_by, create_time, update_by, update_time, remark)
		VALUES (?, ?, ?, '0', ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING report_id`, ebookID, userID, reason, userID, userID, remark)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("ptmj_ebook_report insert returned no id")
	}
	return ScanInt64(rows[0]["report_id"]), nil
}
