package datum

import (
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

// DownloadRepo owns ptmj_file_download: one row per (file, user) download,
// written by the streaming download endpoint.
type DownloadRepo struct {
	conn db.Connection
}

func NewDownloadRepo(conn db.Connection) *DownloadRepo {
	return &DownloadRepo{conn: conn}
}

func (r *DownloadRepo) Create(fileID, userID int64) error {
	_, err := r.conn.Exec(`INSERT INTO ptmj_file_download (file_id, user_id, creat_by, creat_time, update_by, update_time)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP)`, fileID, userID, userID, userID)
	return err
}

func (r *DownloadRepo) Delete(fileID, userID int64) error {
	result, err := r.conn.Exec(`DELETE FROM ptmj_file_download WHERE file_id = ? AND user_id = ?`, fileID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUser pages the desktop "my downloads" view, joined with the file
// rows so the UI renders names/subjects without a second round trip. Rows
// whose file has been hard-removed from ptmj_file drop out of the join.
func (r *DownloadRepo) ListByUser(userID int64, page, size int) (Page, error) {
	page, size = normalizePage(page, size)
	where := "WHERE d.user_id = ? AND f.del_flag = 0"
	args := []interface{}{userID}

	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_file_download d
		JOIN ptmj_file f ON f.file_id = d.file_id AND f.user_id = (SELECT min(user_id) FROM ptmj_file mf WHERE mf.file_id = d.file_id)
		`+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])

	queryArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	rows, err := r.conn.Query(`SELECT f.file_id, MAX(d.user_id) AS user_id, MAX(f.file_name) AS file_name,
		MAX(f.file_url) AS file_url, MAX(f.file_size) AS file_size, MAX(f.file_format) AS file_format,
		MAX(f.file_year) AS file_year, MAX(f.file_type) AS file_type, MAX(f.file_school) AS file_school,
		MAX(f.file_subject) AS file_subject, MAX(f.reviewer) AS reviewer, MAX(f.file_status) AS file_status,
		MAX(f.del_flag) AS del_flag, MAX(f.create_by) AS create_by, MAX(f.create_time) AS create_time,
		MAX(f.update_by) AS update_by, MAX(f.update_time) AS update_time, MAX(f.remark) AS remark,
		MIN(d.creat_time) AS first_download_time
		FROM ptmj_file_download d
		JOIN ptmj_file f ON f.file_id = d.file_id AND f.user_id = (SELECT min(user_id) FROM ptmj_file mf WHERE mf.file_id = d.file_id)
		`+where+`
		GROUP BY f.file_id
		ORDER BY MAX(d.download_id) DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	DownloadedItem := make([]DownloadedFile, 0, len(rows))
	for _, row := range rows {
		item := scanFile(row)
		DownloadedItem = append(DownloadedItem, DownloadedFile{File: item, FirstDownloadTime: ScanString(row["first_download_time"])})
	}
	return Page{Items: DownloadedItem, Total: total, Page: page, PageSize: size}, nil
}

// CountByUser feeds the profile stats card.
func (r *DownloadRepo) CountByUser(userID int64) (int64, error) {
	rows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_file_download WHERE user_id = ?`, userID)
	if err != nil {
		return 0, err
	}
	return ScanInt64(rows[0]["count"]), nil
}

// DownloadedFile is one row of the desktop "my downloads" view: the file
// plus when the user first downloaded it.
type DownloadedFile struct {
	File
	FirstDownloadTime string `json:"firstDownloadTime"`
}

// FileFavoriteRepo owns ptmj_file_favorite (pure join table).
type FileFavoriteRepo struct {
	conn db.Connection
}

func NewFileFavoriteRepo(conn db.Connection) *FileFavoriteRepo {
	return &FileFavoriteRepo{conn: conn}
}

func (r *FileFavoriteRepo) Add(fileID, userID int64) error {
	_, err := r.conn.Exec(`INSERT INTO ptmj_file_favorite (file_id, user_id) VALUES (?, ?)`, fileID, userID)
	return err
}

func (r *FileFavoriteRepo) Remove(fileID, userID int64) error {
	result, err := r.conn.Exec(`DELETE FROM ptmj_file_favorite WHERE file_id = ? AND user_id = ?`, fileID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *FileFavoriteRepo) Exists(fileID, userID int64) (bool, error) {
	rows, err := r.conn.Query(`SELECT 1 FROM ptmj_file_favorite WHERE file_id = ? AND user_id = ?`, fileID, userID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func (r *FileFavoriteRepo) CountByUser(userID int64) (int64, error) {
	rows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_file_favorite WHERE user_id = ?`, userID)
	if err != nil {
		return 0, err
	}
	return ScanInt64(rows[0]["count"]), nil
}

// ListFilesByUser pages the desktop "my favorites" view joined with file
// rows (approved-or-not: users can see what they saved).
func (r *FileFavoriteRepo) ListFilesByUser(userID int64, page, size int) (Page, error) {
	page, size = normalizePage(page, size)
	where := "WHERE fav.user_id = ? AND f.del_flag = 0"
	args := []interface{}{userID}

	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_file_favorite fav
		JOIN ptmj_file f ON f.file_id = fav.file_id AND f.user_id = (SELECT min(user_id) FROM ptmj_file mf WHERE mf.file_id = fav.file_id)
		`+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])

	queryArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	rows, err := r.conn.Query(`SELECT f.file_id, f.user_id, f.file_name, f.file_url, f.file_size, f.file_format, f.file_year, f.file_type,
		f.file_school, f.file_subject, f.reviewer, f.file_status, f.del_flag, f.create_by, f.create_time, f.update_by, f.update_time, f.remark
		FROM ptmj_file_favorite fav
		JOIN ptmj_file f ON f.file_id = fav.file_id AND f.user_id = (SELECT min(user_id) FROM ptmj_file mf WHERE mf.file_id = fav.file_id)
		`+where+`
		ORDER BY f.file_id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	files := make([]File, 0, len(rows))
	for _, row := range rows {
		files = append(files, scanFile(row))
	}
	return Page{Items: files, Total: total, Page: page, PageSize: size}, nil
}

// BookmarkFavoriteRepo owns ptmj_bookmark_favorite. Quirk preserved from the
// legacy schema: the primary key is bookmark_id alone, so a bookmark has at
// most one favoriting user row.
type BookmarkFavoriteRepo struct {
	conn db.Connection
}

func NewBookmarkFavoriteRepo(conn db.Connection) *BookmarkFavoriteRepo {
	return &BookmarkFavoriteRepo{conn: conn}
}

func (r *BookmarkFavoriteRepo) Add(bookmarkID, userID int64) error {
	_, err := r.conn.Exec(`INSERT INTO ptmj_bookmark_favorite (bookmark_id, user_id) VALUES (?, ?)`, bookmarkID, userID)
	return err
}

func (r *BookmarkFavoriteRepo) Remove(bookmarkID int64) error {
	result, err := r.conn.Exec(`DELETE FROM ptmj_bookmark_favorite WHERE bookmark_id = ?`, bookmarkID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *BookmarkFavoriteRepo) Exists(bookmarkID, userID int64) (bool, error) {
	rows, err := r.conn.Query(`SELECT 1 FROM ptmj_bookmark_favorite WHERE bookmark_id = ? AND user_id = ?`, bookmarkID, userID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func (r *BookmarkFavoriteRepo) CountByUser(userID int64) (int64, error) {
	rows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_bookmark_favorite WHERE user_id = ?`, userID)
	if err != nil {
		return 0, err
	}
	return ScanInt64(rows[0]["count"]), nil
}

// ListByUser pages the desktop bookmark favorites joined with bookmark rows.
func (r *BookmarkFavoriteRepo) ListByUser(userID int64, page, size int) (Page, error) {
	page, size = normalizePage(page, size)
	where := "WHERE fav.user_id = ? AND b.del_flag = 0"
	args := []interface{}{userID}

	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_bookmark_favorite fav
		JOIN ptmj_bookmark b ON b.id = fav.bookmark_id `+where, args...)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])

	queryArgs := append(append([]interface{}{}, args...), size, (page-1)*size)
	rows, err := r.conn.Query(`SELECT b.id, b.user_id, b.url, b.title, b.description, b.cover_image, b.subject, b.resource_type, b.collection,
		b.status, b.del_flag, b.create_by, b.create_time, b.update_by, b.update_time, b.remark
		FROM ptmj_bookmark_favorite fav
		JOIN ptmj_bookmark b ON b.id = fav.bookmark_id
		`+where+`
		ORDER BY b.id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, err
	}
	bookmarks := make([]Bookmark, 0, len(rows))
	for _, row := range rows {
		bookmarks = append(bookmarks, scanBookmark(row))
	}
	return Page{Items: bookmarks, Total: total, Page: page, PageSize: size}, nil
}

func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

// FindByID reads one download record (owner-checked by callers).
func (r *DownloadRepo) FindByID(downloadID int64) (FileDownload, bool, error) {
	rows, err := r.conn.Query(`SELECT download_id, file_id, user_id, creat_by, creat_time, update_by, update_time, remark
		FROM ptmj_file_download WHERE download_id = ?`, downloadID)
	if err != nil {
		return FileDownload{}, false, err
	}
	if len(rows) == 0 {
		return FileDownload{}, false, nil
	}
	row := rows[0]
	return FileDownload{
		DownloadID: ScanInt64(row["download_id"]),
		FileID:     ScanInt64(row["file_id"]),
		UserID:     ScanInt64(row["user_id"]),
		CreatBy:    ScanString(row["creat_by"]),
		CreatTime:  ScanString(row["creat_time"]),
		UpdateBy:   ScanString(row["update_by"]),
		UpdateTime: ScanString(row["update_time"]),
		Remark:     ScanString(row["remark"]),
	}, true, nil
}

// UpdateRemark edits the note on one owned download record.
func (r *DownloadRepo) UpdateRemark(downloadID, userID int64, remark string) error {
	result, err := r.conn.Exec(`UPDATE ptmj_file_download SET remark = ?, update_by = ?, update_time = CURRENT_TIMESTAMP
		WHERE download_id = ? AND user_id = ?`, remark, userID, downloadID, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByIDs removes owned download records by id (the desktop passes
// comma-separated ids in one call).
func (r *DownloadRepo) DeleteByIDs(ids []int64, userID int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, 0, len(ids)+1)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, userID)
	result, err := r.conn.Exec(`DELETE FROM ptmj_file_download WHERE download_id IN (`+placeholders+`) AND user_id = ?`, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// BookmarkFavoriteRelation is one raw row of ptmj_bookmark_favorite.
type BookmarkFavoriteRelation struct {
	BookmarkID int64
	UserID     int64
}

// ListAll pages raw favorite relations; the desktop home view fetches the
// whole set and marks which bookmarks the current user already favorited.
func (r *BookmarkFavoriteRepo) ListAll(page, size int) (Page, error) {
	page, size = normalizePage(page, size)
	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM ptmj_bookmark_favorite`)
	if err != nil {
		return Page{}, err
	}
	total := ScanInt64(countRows[0]["count"])
	rows, err := r.conn.Query(`SELECT bookmark_id, user_id FROM ptmj_bookmark_favorite ORDER BY bookmark_id DESC LIMIT ? OFFSET ?`, size, (page-1)*size)
	if err != nil {
		return Page{}, err
	}
	relations := make([]BookmarkFavoriteRelation, 0, len(rows))
	for _, row := range rows {
		relations = append(relations, BookmarkFavoriteRelation{
			BookmarkID: ScanInt64(row["bookmark_id"]),
			UserID:     ScanInt64(row["user_id"]),
		})
	}
	return Page{Items: relations, Total: total, Page: page, PageSize: size}, nil
}
