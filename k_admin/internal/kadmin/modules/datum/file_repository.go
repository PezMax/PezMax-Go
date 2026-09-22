package datum

import (
	"errors"
	"fmt"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

// ErrNotFound reports a missing row (distinct from duplicate-key violations,
// which callers detect via the "duplicate key" error text like the generated
// modules do).
var ErrNotFound = errors.New("datum: row not found")

// FileRepo owns ptmj_file access for the desktop contract: anonymous reads
// are restricted to approved-and-alive rows, owner reads see every status.
type FileRepo struct {
	conn db.Connection
}

func NewFileRepo(conn db.Connection) *FileRepo {
	return &FileRepo{conn: conn}
}

// FilePayload carries the writable fields of a file row.
type FilePayload struct {
	UserID      int64
	FileName    string
	FileURL     string
	FileSize    int64
	FileFormat  string
	FileYear    int64
	FileType    int64
	FileSchool  string
	FileSubject string
	Reviewer    string
	FileStatus  int64
	Remark      string
}

func (r *FileRepo) Insert(payload FilePayload) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_file
		(user_id, file_name, file_url, file_size, file_format, file_year, file_type, file_school, file_subject, reviewer, file_status, del_flag, create_by, create_time, update_by, update_time, remark)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING file_id`,
		payload.UserID, payload.FileName, payload.FileURL, payload.FileSize, payload.FileFormat,
		payload.FileYear, payload.FileType, payload.FileSchool, payload.FileSubject, payload.Reviewer,
		payload.FileStatus, payload.UserID, payload.UserID, payload.Remark)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("ptmj_file insert returned no id")
	}
	return ScanInt64(rows[0]["file_id"]), nil
}

// UpdateOwner edits the writable metadata of a row owned by userID.
func (r *FileRepo) UpdateOwner(fileID, userID int64, payload FilePayload) error {
	result, err := r.conn.Exec(`UPDATE ptmj_file SET
		file_name = ?, file_year = ?, file_type = ?, file_school = ?, file_subject = ?, remark = ?,
		update_by = ?, update_time = CURRENT_TIMESTAMP
		WHERE file_id = ? AND user_id = ? AND del_flag = 0`,
		payload.FileName, payload.FileYear, payload.FileType, payload.FileSchool, payload.FileSubject,
		payload.Remark, payload.UserID, fileID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkDeleted soft-deletes a row owned by userID.
func (r *FileRepo) MarkDeleted(fileID, userID int64) error {
	result, err := r.conn.Exec(`UPDATE ptmj_file SET del_flag = 1, update_time = CURRENT_TIMESTAMP
		WHERE file_id = ? AND user_id = ? AND del_flag = 0`, fileID, userID)
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
func (r *FileRepo) SetStatus(fileID, status int64, reviewer string) error {
	_, err := r.conn.Exec(`UPDATE ptmj_file SET file_status = ?, reviewer = ?, update_time = CURRENT_TIMESTAMP
		WHERE file_id = ?`, status, reviewer, fileID)
	return err
}

// ApproveAllByUser batch-approves every pending upload of one user.
func (r *FileRepo) ApproveAllByUser(userID, reviewer string) (int64, error) {
	result, err := r.conn.Exec(`UPDATE ptmj_file SET file_status = 1, reviewer = ?, update_time = CURRENT_TIMESTAMP
		WHERE user_id = ? AND file_status = 0 AND del_flag = 0`, reviewer, userID)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (r *FileRepo) FindByID(fileID int64) (File, bool, error) {
	rows, err := r.conn.Query(`SELECT `+fileColumns+` FROM ptmj_file WHERE file_id = ? AND del_flag = 0`, fileID)
	if err != nil {
		return File{}, false, err
	}
	if len(rows) == 0 {
		return File{}, false, nil
	}
	return scanFile(rows[0]), true, nil
}

// List filters and pages file rows. Anonymous browsing passes OnlyApproved.
func (r *FileRepo) List(filter FileFilter) (Page, error) {
	page, size := filter.page()
	where, args := fileFilterWhere(filter)

	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_file `+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])

	queryArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	rows, err := r.conn.Query(`SELECT `+fileColumns+` FROM ptmj_file `+where+`
		ORDER BY file_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	files := make([]File, 0, len(rows))
	for _, row := range rows {
		files = append(files, scanFile(row))
	}
	return Page{Items: files, Total: total, Page: page, PageSize: size}, nil
}

func fileFilterWhere(filter FileFilter) (string, []interface{}) {
	conditions := []string{"del_flag = 0"}
	args := []interface{}{}
	if filter.OnlyApproved {
		conditions = append(conditions, "file_status = 1")
	}
	if filter.FileType > 0 {
		conditions = append(conditions, "file_type = ?")
		args = append(args, filter.FileType)
	}
	if strings.TrimSpace(filter.FileSubject) != "" {
		conditions = append(conditions, "file_subject = ?")
		args = append(args, strings.TrimSpace(filter.FileSubject))
	}
	if strings.TrimSpace(filter.FileSchool) != "" {
		conditions = append(conditions, "file_school = ?")
		args = append(args, strings.TrimSpace(filter.FileSchool))
	}
	if filter.FileYear > 0 {
		conditions = append(conditions, "file_year = ?")
		args = append(args, filter.FileYear)
	}
	if filter.UserID > 0 {
		conditions = append(conditions, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		conditions = append(conditions, "(file_name ILIKE ? OR file_subject ILIKE ? OR file_school ILIKE ?)")
		pattern := "%" + keyword + "%"
		args = append(args, pattern, pattern, pattern)
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

// SubjectCount feeds the subject dropdown and the tree aggregate.
type SubjectCount struct {
	Subject string
	Total   int64
}

func (r *FileRepo) Subjects() ([]SubjectCount, error) {
	rows, err := r.conn.Query(`SELECT file_subject, count(*) AS count FROM ptmj_file
		WHERE file_status = 1 AND del_flag = 0 GROUP BY file_subject ORDER BY file_subject`)
	if err != nil {
		return nil, err
	}
	subjects := make([]SubjectCount, 0, len(rows))
	for _, row := range rows {
		subjects = append(subjects, SubjectCount{Subject: ScanString(row["file_subject"]), Total: ScanInt64(row["count"])})
	}
	return subjects, nil
}

// Schools lists distinct school names of approved files, optionally narrowed
// by keyword (duplicate-name check for uploads).
func (r *FileRepo) Schools(keyword string) ([]string, error) {
	where := "WHERE file_status = 1 AND del_flag = 0"
	args := []interface{}{}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		where += " AND file_school ILIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	rows, err := r.conn.Query(`SELECT DISTINCT file_school FROM ptmj_file `+where+` ORDER BY file_school`, args...)
	if err != nil {
		return nil, err
	}
	schools := make([]string, 0, len(rows))
	for _, row := range rows {
		if value := ScanString(row["file_school"]); value != "" {
			schools = append(schools, value)
		}
	}
	return schools, nil
}

// YearCount is one leaf of the anonymous file tree (type → subject → year).
type YearCount struct {
	Year  int64
	Total int64
}

// TreeYears aggregates approved files of one type+subject by year.
func (r *FileRepo) TreeYears(fileType int64, subject string) ([]YearCount, error) {
	rows, err := r.conn.Query(`SELECT file_year, count(*) AS count FROM ptmj_file
		WHERE file_status = 1 AND del_flag = 0 AND file_type = ? AND file_subject = ?
		GROUP BY file_year ORDER BY file_year DESC`, fileType, strings.TrimSpace(subject))
	if err != nil {
		return nil, err
	}
	years := make([]YearCount, 0, len(rows))
	for _, row := range rows {
		years = append(years, YearCount{Year: ScanInt64(row["file_year"]), Total: ScanInt64(row["count"])})
	}
	return years, nil
}
