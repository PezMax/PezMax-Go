package datum

import (
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
