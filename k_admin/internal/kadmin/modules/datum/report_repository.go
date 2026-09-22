package datum

import (
	"errors"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

// ReportRepo owns ptmj_report (file reports). result: 0 pending,
// 1 confirmed, 2 dismissed; the audit timeline is submodule concern —
// this layer stores the current verdict and its timestamps.
type ReportRepo struct {
	conn db.Connection
}

func NewReportRepo(conn db.Connection) *ReportRepo {
	return &ReportRepo{conn: conn}
}

func (r *ReportRepo) Create(fileID, userID int64, reason string) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_report (file_id, user_id, reason, result, create_by, create_time, update_by, update_time)
		VALUES (?, ?, ?, '0', ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP) RETURNING report_id`,
		fileID, userID, reason, userID, userID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errReportNoID
	}
	return ScanInt64(rows[0]["report_id"]), nil
}

func (r *ReportRepo) FindByID(reportID int64) (Report, bool, error) {
	rows, err := r.conn.Query(`SELECT report_id, file_id, user_id, reason, result, create_by, create_time, update_by, update_time, remark
		FROM ptmj_report WHERE report_id = ?`, reportID)
	if err != nil {
		return Report{}, false, err
	}
	if len(rows) == 0 {
		return Report{}, false, nil
	}
	return scanReport(rows[0]), true, nil
}

// FindByFileAndReporter detects duplicate reports of one file by one user.
func (r *ReportRepo) FindByFileAndReporter(fileID, userID int64) (Report, bool, error) {
	rows, err := r.conn.Query(`SELECT report_id, file_id, user_id, reason, result, create_by, create_time, update_by, update_time, remark
		FROM ptmj_report WHERE file_id = ? AND user_id = ?`, fileID, userID)
	if err != nil {
		return Report{}, false, err
	}
	if len(rows) == 0 {
		return Report{}, false, nil
	}
	return scanReport(rows[0]), true, nil
}

// ListByFile feeds the report timeline view (newest first).
func (r *ReportRepo) ListByFile(fileID int64) ([]Report, error) {
	rows, err := r.conn.Query(`SELECT report_id, file_id, user_id, reason, result, create_by, create_time, update_by, update_time, remark
		FROM ptmj_report WHERE file_id = ? ORDER BY report_id DESC`, fileID)
	if err != nil {
		return nil, err
	}
	reports := make([]Report, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, scanReport(row))
	}
	return reports, nil
}

// ListByResult pages admin moderation queues; result < 0 means all.
func (r *ReportRepo) ListByResult(result string, page, size int) (Page, error) {
	page, size = normalizePage(page, size)
	where := "WHERE 1=1"
	args := []interface{}{}
	if result == "0" || result == "1" || result == "2" {
		where = "WHERE result = ?"
		args = append(args, result)
	}
	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_report `+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])
	queryArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	rows, err := r.conn.Query(`SELECT report_id, file_id, user_id, reason, result, create_by, create_time, update_by, update_time, remark
		FROM ptmj_report `+where+` ORDER BY report_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	reports := make([]Report, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, scanReport(row))
	}
	return Page{Items: reports, Total: total, Page: page, PageSize: size}, nil
}

// SetResult records the moderation verdict and the acting admin.
func (r *ReportRepo) SetResult(reportID int64, result, updateBy, remark string) error {
	_, err := r.conn.Exec(`UPDATE ptmj_report SET result = ?, update_by = ?, update_time = CURRENT_TIMESTAMP, remark = ?
		WHERE report_id = ?`, result, updateBy, remark, reportID)
	return err
}

// BookmarkReportRepo owns ptmj_bookmark_report; same shape as ReportRepo
// with the bookmark target and a UNIQUE (user_id, bookmark_id) constraint.
type BookmarkReportRepo struct {
	conn db.Connection
}

func NewBookmarkReportRepo(conn db.Connection) *BookmarkReportRepo {
	return &BookmarkReportRepo{conn: conn}
}

func (r *BookmarkReportRepo) Create(bookmarkID, userID int64, reason string) (int64, error) {
	rows, err := r.conn.Query(`INSERT INTO ptmj_bookmark_report (bookmark_id, user_id, reason, result, create_by, create_time, update_by, update_time)
		VALUES (?, ?, ?, '0', ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP) RETURNING report_id`,
		bookmarkID, userID, reason, userID, userID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errReportNoID
	}
	return ScanInt64(rows[0]["report_id"]), nil
}

func (r *BookmarkReportRepo) FindByID(reportID int64) (BookmarkReport, bool, error) {
	rows, err := r.conn.Query(`SELECT report_id, bookmark_id, user_id, reason, result, create_by, create_time, update_by, update_time, remark
		FROM ptmj_bookmark_report WHERE report_id = ?`, reportID)
	if err != nil {
		return BookmarkReport{}, false, err
	}
	if len(rows) == 0 {
		return BookmarkReport{}, false, nil
	}
	return scanBookmarkReport(rows[0]), true, nil
}

func (r *BookmarkReportRepo) ListByBookmark(bookmarkID int64) ([]BookmarkReport, error) {
	rows, err := r.conn.Query(`SELECT report_id, bookmark_id, user_id, reason, result, create_by, create_time, update_by, update_time, remark
		FROM ptmj_bookmark_report WHERE bookmark_id = ? ORDER BY report_id DESC`, bookmarkID)
	if err != nil {
		return nil, err
	}
	reports := make([]BookmarkReport, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, scanBookmarkReport(row))
	}
	return reports, nil
}

func (r *BookmarkReportRepo) SetResult(reportID int64, result, updateBy, remark string) error {
	_, err := r.conn.Exec(`UPDATE ptmj_bookmark_report SET result = ?, update_by = ?, update_time = CURRENT_TIMESTAMP, remark = ?
		WHERE report_id = ?`, result, updateBy, remark, reportID)
	return err
}

var errReportNoID = errors.New("ptmj_report insert returned no id")

func scanReport(row map[string]interface{}) Report {
	return Report{
		ReportID:   ScanInt64(row["report_id"]),
		FileID:     ScanInt64(row["file_id"]),
		UserID:     ScanInt64(row["user_id"]),
		Reason:     ScanString(row["reason"]),
		Result:     ScanString(row["result"]),
		CreateBy:   ScanString(row["create_by"]),
		CreateTime: ScanString(row["create_time"]),
		UpdateBy:   ScanString(row["update_by"]),
		UpdateTime: ScanString(row["update_time"]),
		Remark:     ScanString(row["remark"]),
	}
}

func scanBookmarkReport(row map[string]interface{}) BookmarkReport {
	return BookmarkReport{
		ReportID:   ScanInt64(row["report_id"]),
		BookmarkID: ScanInt64(row["bookmark_id"]),
		UserID:     ScanInt64(row["user_id"]),
		Reason:     ScanString(row["reason"]),
		Result:     ScanString(row["result"]),
		CreateBy:   ScanString(row["create_by"]),
		CreateTime: ScanString(row["create_time"]),
		UpdateBy:   ScanString(row["update_by"]),
		UpdateTime: ScanString(row["update_time"]),
		Remark:     ScanString(row["remark"]),
	}
}
