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

// EbookPayload carries the writable fields of an ebook row.
type EbookPayload struct {
	UserID      int64
	EbookName   string
	Author      string
	Publisher   string
	EbookURL    string
	EbookSize   int64
	EbookFormat string
	EbookSubject string
	EbookType   int64
	Remark      string
}

// Insert creates a pending ebook row（ebook_status=0，进入与试卷相同的审核渠道）.
func (r *EbookRepo) Insert(payload EbookPayload) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_ebook
		(user_id, ebook_name, author, publisher, ebook_url, ebook_size, ebook_format, ebook_subject, ebook_type, reviewer, ebook_status, del_flag, create_by, create_time, update_by, update_time, remark)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', 0, 0, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING ebook_id`,
		payload.UserID, payload.EbookName, payload.Author, payload.Publisher, payload.EbookURL,
		payload.EbookSize, payload.EbookFormat, payload.EbookSubject, payload.EbookType,
		payload.UserID, payload.UserID, payload.Remark)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("ptmj_ebook insert returned no id")
	}
	return ScanInt64(rows[0]["ebook_id"]), nil
}

// MarkDeleted soft-deletes a row owned by userID.
func (r *EbookRepo) MarkDeleted(ebookID, userID int64) error {
	result, err := r.conn.Exec(`UPDATE ptmj_ebook SET del_flag = 1, update_time = CURRENT_TIMESTAMP
		WHERE ebook_id = ? AND user_id = ? AND del_flag = 0`, ebookID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStatus transitions review/report status (0 pending, 1 approved,
// 2 rejected, 3 reported); reviewer records the acting admin.
func (r *EbookRepo) SetStatus(ebookID, status int64, reviewer string) error {
	_, err := r.conn.Exec(`UPDATE ptmj_ebook SET ebook_status = ?, reviewer = ?, update_time = CURRENT_TIMESTAMP
		WHERE ebook_id = ?`, status, reviewer, ebookID)
	return err
}

// ApproveAllByUser batch-approves every pending ebook upload of one user.
func (r *EbookRepo) ApproveAllByUser(userID int64, reviewer string) (int64, error) {
	result, err := r.conn.Exec(`UPDATE ptmj_ebook SET ebook_status = 1, reviewer = ?, update_time = CURRENT_TIMESTAMP
		WHERE user_id = ? AND ebook_status = 0 AND del_flag = 0`, reviewer, userID)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
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

// ListByUserDetailed pages the desktop "my ebook favorites" view, joined with
// the book rows so the UI renders names/subjects without a second round trip.
func (r *EbookFavoriteRepo) ListByUserDetailed(userID int64, page, size int) (Page, error) {
	page, size = normalizePage(page, size)
	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_ebook_favorite fav
		JOIN ptmj_ebook e ON e.ebook_id = fav.ebook_id AND e.del_flag = 0
		WHERE fav.user_id = ?`, userID)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])

	queryArgs := []interface{}{userID, size, (page - 1) * size}
	rows, err := r.conn.Query(`SELECT e.ebook_id, e.user_id, e.ebook_name, e.author, e.publisher, e.cover_url, e.ebook_url,
		e.ebook_size, e.ebook_format, e.ebook_subject, e.ebook_type, e.reviewer, e.ebook_status, e.del_flag,
		e.create_by, e.create_time, e.update_by, e.update_time, e.remark FROM ptmj_ebook_favorite fav
		JOIN ptmj_ebook e ON e.ebook_id = fav.ebook_id AND e.del_flag = 0
		WHERE fav.user_id = ?
		ORDER BY fav.ebook_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	books := make([]Ebook, 0, len(rows))
	for _, row := range rows {
		books = append(books, ScanEbook(row))
	}
	return Page{Items: books, Total: total, Page: page, PageSize: size}, nil
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

// SetResult records the audit verdict (0 pending, 1 valid, 2 invalid).
func (r *EbookReportRepo) SetResult(reportID int64, result, reviewer, remark string) error {
	_, err := r.conn.Exec(`UPDATE ptmj_ebook_report SET result = ?, remark = ?, update_by = ?, update_time = CURRENT_TIMESTAMP
		WHERE report_id = ?`, result, remark, reviewer, reportID)
	return err
}

// LatestPendingReport returns the newest pending report id of one ebook.
func (r *EbookReportRepo) LatestPendingReport(ebookID int64) (int64, error) {
	rows, err := r.conn.Query(`SELECT report_id FROM ptmj_ebook_report WHERE ebook_id = ? AND result = '0'
		ORDER BY report_id DESC LIMIT 1`, ebookID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return ScanInt64(rows[0]["report_id"]), nil
}

// EbookReportRow is one entry of the desktop "my ebook reports" list.
type EbookReportRow struct {
	ReportID   int64  `json:"reportId"`
	EbookID    int64  `json:"ebookId"`
	EbookName  string `json:"ebookName"`
	Reason     string `json:"reason"`
	Result     string `json:"result"`
	Remark     string `json:"remark"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
}

// ListByUser pages the session user's ebook reports with the book name.
func (r *EbookReportRepo) ListByUser(userID int64, page, size int) (Page, error) {
	page, size = normalizePage(page, size)
	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_ebook_report WHERE user_id = ?`, userID)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])

	queryArgs := []interface{}{userID, size, (page - 1) * size}
	rows, err := r.conn.Query(`SELECT rp.report_id, rp.ebook_id, rp.reason, rp.result, rp.remark, rp.create_time, rp.update_time,
			COALESCE(NULLIF(e.ebook_name, ''), CAST(rp.ebook_id AS TEXT)) AS ebook_name
		FROM ptmj_ebook_report rp
		LEFT JOIN ptmj_ebook e ON e.ebook_id = rp.ebook_id
		WHERE rp.user_id = ?
		ORDER BY rp.report_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	items := make([]EbookReportRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, EbookReportRow{
			ReportID:   ScanInt64(row["report_id"]),
			EbookID:    ScanInt64(row["ebook_id"]),
			EbookName:  ScanString(row["ebook_name"]),
			Reason:     ScanString(row["reason"]),
			Result:     ScanString(row["result"]),
			Remark:     ScanString(row["remark"]),
			CreateTime: ScanString(row["create_time"]),
			UpdateTime: ScanString(row["update_time"]),
		})
	}
	return Page{Items: items, Total: total, Page: page, PageSize: size}, nil
}
