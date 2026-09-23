package kadmin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// fake DB（内嵌 db.Connection 接口以完整实现接口，仅覆盖 Query/Exec）
// ---------------------------------------------------------------------------

type fakeDatumDB struct {
	db.Connection
	mu              sync.Mutex
	users           map[string]*datumUser
	security        map[int64][2]string
	files           []map[string]interface{}
	bookmarks       map[int64]map[string]interface{}
	downloads       []map[string]interface{}
	fileFavs        [][2]int64
	bookmarkFavs    map[int64]int64
	notifications   []map[string]interface{}
	reports         []map[string]interface{}
	bookmarkReports []map[string]interface{}
	nextDownloadID  int64
	nextID          int64
	seedSeq         int
}

func newFakeDatumDB() *fakeDatumDB {
	return &fakeDatumDB{
		users:        map[string]*datumUser{},
		security:     map[int64][2]string{},
		bookmarks:    map[int64]map[string]interface{}{},
		bookmarkFavs: map[int64]int64{},
		nextID:       100,
	}
}

var errFakeDuplicateKey = errors.New("duplicate key value violates unique constraint")
var errFakeUnsupportedQuery = errors.New("fakeDatumDB: unsupported query")

type fakeDatumResult struct{ rows int64 }

func (r fakeDatumResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeDatumResult) RowsAffected() (int64, error) { return r.rows, nil }

func (f *fakeDatumDB) userRow(user *datumUser) map[string]interface{} {
	return map[string]interface{}{
		"user_id": user.UserID, "user_name": user.UserName, "password": user.Password,
		"avatar": user.Avatar, "count": user.Count, "status": user.Status,
	}
}

func (f *fakeDatumDB) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.Contains(query, "INSERT INTO ptmj_user"):
		userName := toDatumString(args[0])
		if _, exists := f.users[strings.ToLower(userName)]; exists {
			return nil, errFakeDuplicateKey
		}
		f.nextID++
		user := &datumUser{UserID: f.nextID, UserName: userName, Password: toDatumString(args[1]), Avatar: toDatumString(args[2]), Status: "1"}
		f.users[strings.ToLower(userName)] = user
		return []map[string]interface{}{{"user_id": user.UserID}}, nil
	case strings.Contains(query, "FROM ptmj_user WHERE user_name"):
		user, exists := f.users[strings.ToLower(toDatumString(args[0]))]
		if !exists {
			return nil, nil
		}
		return []map[string]interface{}{f.userRow(user)}, nil
	case strings.Contains(query, "FROM ptmj_user WHERE user_id"):
		for _, user := range f.users {
			if user.UserID == toDatumInt64(args[0]) {
				return []map[string]interface{}{f.userRow(user)}, nil
			}
		}
		return nil, nil
	case strings.Contains(query, "goadmin_users") && strings.Contains(query, "id"):
		return []map[string]interface{}{{"id": toDatumInt64(args[len(args)-1]), "status": "enable"}}, nil
	case strings.Contains(query, "FROM ptmj_security WHERE user_id"):
		row, exists := f.security[toDatumInt64(args[0])]
		if !exists {
			return nil, nil
		}
		return []map[string]interface{}{{"question": row[0], "answer": row[1]}}, nil
	case strings.Contains(query, "INSERT INTO ptmj_notification") && strings.Contains(query, "RETURNING notify_id"):
		f.nextDownloadID++
		f.notifications = append(f.notifications, map[string]interface{}{
			"notify_id": f.nextDownloadID, "notify_type": toDatumString(args[0]), "title": toDatumString(args[1]),
			"content": toDatumString(args[2]), "status": toDatumString(args[3]), "sort": toDatumInt64(args[4]),
			"display_mode":   toDatumString(args[5]),
			"upload_user_id": toDatumInt64(args[11]), "material_id": toDatumInt64(args[12]),
			"material_title_snapshot": toDatumString(args[13]),
			"scroll_time_interval":    toDatumInt64(args[16]), "remark": toDatumString(args[17]),
		})
		return []map[string]interface{}{{"notify_id": f.nextDownloadID}}, nil
	case strings.Contains(query, "INSERT INTO ptmj_report"):
		f.nextDownloadID++
		row := map[string]interface{}{
			"report_id": f.nextDownloadID, "file_id": toDatumInt64(args[0]), "user_id": toDatumInt64(args[1]),
			"reason": toDatumString(args[2]), "result": "0", "remark": toDatumString(args[3]),
			"create_time": "2026-09-22 10:00:00", "update_time": "2026-09-22 10:00:00",
		}
		f.reports = append(f.reports, row)
		return []map[string]interface{}{{"report_id": row["report_id"]}}, nil
	case strings.Contains(query, "INSERT INTO ptmj_bookmark_report"):
		f.nextDownloadID++
		row := map[string]interface{}{
			"report_id": f.nextDownloadID, "bookmark_id": toDatumInt64(args[0]), "user_id": toDatumInt64(args[1]),
			"reason": toDatumString(args[2]), "result": "0", "remark": toDatumString(args[3]),
			"create_time": "2026-09-22 10:00:00", "update_time": "2026-09-22 10:00:00",
		}
		f.bookmarkReports = append(f.bookmarkReports, row)
		return []map[string]interface{}{{"report_id": row["report_id"]}}, nil
	case strings.Contains(query, "FROM ptmj_report WHERE report_id = ?"):
		for _, report := range f.reports {
			if report["report_id"] == toDatumInt64(args[0]) {
				return []map[string]interface{}{report}, nil
			}
		}
		return nil, nil
	case strings.Contains(query, "FROM ptmj_report WHERE file_id = ?"):
		rows := []map[string]interface{}{}
		for _, report := range f.reports {
			if report["file_id"] == toDatumInt64(args[0]) {
				rows = append(rows, report)
			}
		}
		return rows, nil
	case strings.Contains(query, "FROM ptmj_report rp") && strings.Contains(query, "count(*)"):
		total := int64(0)
		for _, report := range f.reports {
			if report["user_id"] == toDatumInt64(args[0]) {
				total++
			}
		}
		return []map[string]interface{}{{"count": total}}, nil
	case strings.Contains(query, "FROM ptmj_report rp"):
		rows := []map[string]interface{}{}
		for _, report := range f.reports {
			if report["user_id"] != toDatumInt64(args[0]) {
				continue
			}
			row := map[string]interface{}{}
			for key, value := range report {
				row[key] = value
			}
			for _, file := range f.files {
				if file["file_id"] == report["file_id"] {
					row["file_name"] = file["file_name"]
					row["file_subject"] = file["file_subject"]
					row["file_school"] = file["file_school"]
				}
			}
			rows = append(rows, row)
		}
		return rows, nil
	case strings.Contains(query, "FROM ptmj_bookmark_report WHERE report_id = ?"):
		for _, report := range f.bookmarkReports {
			if report["report_id"] == toDatumInt64(args[0]) {
				return []map[string]interface{}{report}, nil
			}
		}
		return nil, nil
	case strings.Contains(query, "FROM ptmj_bookmark_report WHERE bookmark_id = ?"):
		rows := []map[string]interface{}{}
		for _, report := range f.bookmarkReports {
			if report["bookmark_id"] == toDatumInt64(args[0]) {
				rows = append(rows, report)
			}
		}
		return rows, nil
	case strings.Contains(query, "FROM ptmj_bookmark_report rp") && strings.Contains(query, "count(*)"):
		total := int64(0)
		for _, report := range f.bookmarkReports {
			if report["user_id"] == toDatumInt64(args[0]) {
				total++
			}
		}
		return []map[string]interface{}{{"count": total}}, nil
	case strings.Contains(query, "FROM ptmj_bookmark_report rp"):
		rows := []map[string]interface{}{}
		for _, report := range f.bookmarkReports {
			if report["user_id"] != toDatumInt64(args[0]) {
				continue
			}
			row := map[string]interface{}{}
			for key, value := range report {
				row[key] = value
			}
			for id, bookmark := range f.bookmarks {
				if toDatumInt64(id) == report["bookmark_id"] {
					row["title"] = bookmark["title"]
					row["url"] = bookmark["url"]
				}
			}
			rows = append(rows, row)
		}
		return rows, nil
	case strings.Contains(query, "FROM ptmj_notification") && strings.Contains(query, "display_mode = '0'"):
		return f.notificationRows("0"), nil
	case strings.Contains(query, "FROM ptmj_notification") && strings.Contains(query, "display_mode = '1'"):
		return f.notificationRows("1"), nil
	case strings.Contains(query, "FROM ptmj_notification WHERE notify_id = ?"):
		for _, notification := range f.notifications {
			if notification["notify_id"] == toDatumInt64(args[0]) {
				return []map[string]interface{}{notification}, nil
			}
		}
		return nil, nil
	case strings.Contains(query, "FROM ptmj_notification") && strings.Contains(query, "count(*)") && !strings.Contains(query, "WHERE notify_id"):
		total := int64(len(f.notifications))
		if strings.Contains(query, "notify_type = ?") && len(args) > 0 {
			want := toDatumString(args[0])
			total = 0
			for _, notification := range f.notifications {
				if toDatumString(notification["notify_type"]) == want {
					total++
				}
			}
		}
		return []map[string]interface{}{{"count": total}}, nil
	case strings.Contains(query, "FROM ptmj_notification"):
		rows := []map[string]interface{}{}
		for _, notification := range f.notifications {
			if strings.Contains(query, "notify_type = ?") && toDatumString(notification["notify_type"]) != toDatumString(args[0]) {
				continue
			}
			rows = append(rows, notification)
		}
		return rows, nil
	case strings.Contains(query, "FROM ptmj_file_download WHERE download_id = ?") || strings.Contains(query, "FROM ptmj_file_download WHERE download_id"):
		for _, record := range f.downloads {
			if record["download_id"] == toDatumInt64(args[0]) {
				return []map[string]interface{}{record}, nil
			}
		}
		return nil, nil
	case strings.Contains(query, "FROM ptmj_file_download d") && strings.Contains(query, "count(*)"):
		total := int64(0)
		for _, record := range f.downloads {
			if record["user_id"] == toDatumInt64(args[0]) {
				total++
			}
		}
		return []map[string]interface{}{{"count": total}}, nil
	case strings.Contains(query, "FROM ptmj_file_download d"):
		rows := []map[string]interface{}{}
		for _, record := range f.downloads {
			if record["user_id"] != toDatumInt64(args[0]) {
				continue
			}
			row := map[string]interface{}{}
			for key, value := range record {
				row[key] = value
			}
			for _, file := range f.files {
				if file["file_id"] == record["file_id"] {
					for key, value := range file {
						if _, exists := row[key]; !exists {
							row[key] = value
						}
					}
				}
			}
			rows = append(rows, row)
		}
		return rows, nil
	case strings.Contains(query, "SELECT 1 FROM ptmj_file_favorite WHERE"):
		pair := [2]int64{toDatumInt64(args[0]), toDatumInt64(args[1])}
		for _, existing := range f.fileFavs {
			if existing == pair {
				return []map[string]interface{}{{"1": int64(1)}}, nil
			}
		}
		return nil, nil
	case strings.Contains(query, "FROM ptmj_file_favorite fav") && strings.Contains(query, "count(*)"):
		total := int64(0)
		for _, pair := range f.fileFavs {
			if pair[1] == toDatumInt64(args[0]) {
				total++
			}
		}
		return []map[string]interface{}{{"count": total}}, nil
	case strings.Contains(query, "FROM ptmj_file_favorite fav"):
		rows := []map[string]interface{}{}
		for _, pair := range f.fileFavs {
			if pair[1] != toDatumInt64(args[0]) {
				continue
			}
			for _, file := range f.files {
				if file["file_id"] == toDatumInt64(pair[0]) {
					rows = append(rows, file)
				}
			}
		}
		return rows, nil
	case strings.Contains(query, "SELECT count(*) AS count FROM ptmj_bookmark_favorite") && !strings.Contains(query, "WHERE"):
		return []map[string]interface{}{{"count": int64(len(f.bookmarkFavs))}}, nil
	case strings.Contains(query, "SELECT bookmark_id, user_id FROM ptmj_bookmark_favorite"):
		rows := []map[string]interface{}{}
		for bookmarkID, userID := range f.bookmarkFavs {
			rows = append(rows, map[string]interface{}{"bookmark_id": bookmarkID, "user_id": userID})
		}
		return rows, nil
	case strings.Contains(query, "FROM ptmj_bookmark_favorite fav") && strings.Contains(query, "JOIN ptmj_bookmark b"):
		rows := []map[string]interface{}{}
		for bookmarkID, userID := range f.bookmarkFavs {
			if userID != toDatumInt64(args[0]) {
				continue
			}
			if bookmark, exists := f.bookmarks[bookmarkID]; exists {
				row := map[string]interface{}{}
				for key, value := range bookmark {
					row[key] = value
				}
				rows = append(rows, row)
			}
		}
		return rows, nil
	case strings.Contains(query, "SELECT 1 FROM ptmj_bookmark_favorite WHERE"):
		userID, exists := f.bookmarkFavs[toDatumInt64(args[0])]
		if exists && userID == toDatumInt64(args[1]) {
			return []map[string]interface{}{{"1": int64(1)}}, nil
		}
		return nil, nil
	case strings.Contains(query, "FROM ptmj_bookmark WHERE id = ?"):
		row, exists := f.bookmarks[toDatumInt64(args[0])]
		if !exists {
			return nil, nil
		}
		return []map[string]interface{}{row}, nil
	case strings.Contains(query, "INSERT INTO ptmj_bookmark") && strings.Contains(query, "RETURNING id"):
		f.nextID++
		row := map[string]interface{}{
			"id": f.nextID, "user_id": toDatumInt64(args[0]), "url": toDatumString(args[1]),
			"title": toDatumString(args[2]), "description": toDatumString(args[3]),
			"cover_image": toDatumString(args[4]), "subject": toDatumString(args[5]),
			"resource_type": toDatumString(args[6]), "collection": toDatumString(args[7]),
			"status": toDatumInt64(args[8]), "del_flag": int64(0),
			"create_time": "2026-09-23 09:00:00", "update_time": "2026-09-23 09:00:00",
			"remark": toDatumString(args[11]),
		}
		f.bookmarks[f.nextID] = row
		return []map[string]interface{}{{"id": f.nextID}}, nil
	case strings.Contains(query, "count(*)") && strings.Contains(query, "FROM ptmj_bookmark WHERE del_flag"):
		return []map[string]interface{}{{"count": int64(len(f.matchBookmarks(query, args)))}}, nil
	case strings.Contains(query, "FROM ptmj_bookmark WHERE del_flag"):
		return f.matchBookmarks(query, args), nil
	case strings.Contains(query, "FROM ptmj_file_download WHERE user_id"):
		return []map[string]interface{}{{"count": int64(5)}}, nil
	case strings.Contains(query, "FROM ptmj_file_favorite WHERE user_id"):
		return []map[string]interface{}{{"count": int64(3)}}, nil
	case strings.Contains(query, "FROM ptmj_bookmark_favorite WHERE user_id"):
		return []map[string]interface{}{{"count": int64(2)}}, nil
	case strings.Contains(query, "INSERT INTO ptmj_file"):
		f.nextID++
		row := map[string]interface{}{
			"file_id": f.nextID, "user_id": toDatumInt64(args[0]), "file_name": toDatumString(args[1]),
			"file_url": toDatumString(args[2]), "file_size": toDatumInt64(args[3]), "file_format": toDatumString(args[4]),
			"file_year": toDatumInt64(args[5]), "file_type": toDatumInt64(args[6]), "file_school": toDatumString(args[7]),
			"file_subject": toDatumString(args[8]), "file_status": toDatumInt64(args[10]), "del_flag": int64(0),
			"remark": toDatumString(args[13]),
		}
		f.files = append(f.files, row)
		return []map[string]interface{}{{"file_id": row["file_id"]}}, nil
	case strings.Contains(query, "status = '1' AND count > 0"):
		rows := []map[string]interface{}{}
		for _, user := range f.users {
			if user.Status == "1" {
				rows = append(rows, map[string]interface{}{"user_id": user.UserID, "user_name": user.UserName, "avatar": user.Avatar, "count": user.Count})
			}
		}
		return rows, nil
	case strings.Contains(query, "file_status = 1 AND del_flag = 0") && strings.Contains(query, "ORDER BY file_type"):
		approved := []map[string]interface{}{}
		for _, file := range f.files {
			if toDatumInt64(file["file_status"]) == 1 && toDatumInt64(file["del_flag"]) == 0 {
				approved = append(approved, file)
			}
		}
		return approved, nil
	case strings.Contains(query, "count(*)") && strings.Contains(query, "FROM ptmj_file"):
		total := int64(0)
		for _, file := range f.files {
			if toDatumInt64(file["del_flag"]) == 0 {
				total++
			}
		}
		return []map[string]interface{}{{"count": total}}, nil
	case strings.Contains(query, "FROM ptmj_file WHERE file_id"):
		rows := []map[string]interface{}{}
		for _, file := range f.files {
			if file["file_id"] == toDatumInt64(args[0]) && toDatumInt64(file["del_flag"]) == 0 {
				rows = append(rows, file)
			}
		}
		return rows, nil
	case strings.Contains(query, "FROM ptmj_file"):
		rows := []map[string]interface{}{}
		for _, file := range f.files {
			if toDatumInt64(file["del_flag"]) == 0 {
				rows = append(rows, file)
			}
		}
		if strings.Contains(query, "LIMIT ? OFFSET ?") {
			return rows, nil
		}
		return rows, nil
	case strings.Contains(query, "DISTINCT file_school") || strings.Contains(query, "GROUP BY file_subject"):
		return nil, nil
	}
	return nil, fmt.Errorf("fakeDatumDB: unsupported query: %s", query)
}

func (f *fakeDatumDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.Contains(query, "UPDATE ptmj_user"):
		userID := toDatumInt64(args[len(args)-1])
		for _, user := range f.users {
			if user.UserID != userID {
				continue
			}
			switch {
			case strings.Contains(query, "SET user_name"):
				newName := strings.ToLower(toDatumString(args[0]))
				for _, other := range f.users {
					if other.UserID != userID && strings.ToLower(other.UserName) == newName {
						return nil, errFakeDuplicateKey
					}
				}
				user.UserName = toDatumString(args[0])
			case strings.Contains(query, "SET avatar"):
				user.Avatar = toDatumString(args[0])
			case strings.Contains(query, "SET count = count + 1"):
				user.Count++
			case strings.Contains(query, "SET count"):
				if user.Count > 0 {
					user.Count--
				}
			default:
				user.Password = toDatumString(args[0])
			}
		}
	case strings.Contains(query, "INSERT INTO ptmj_file_download"):
		f.nextDownloadID++
		f.downloads = append(f.downloads, map[string]interface{}{
			"download_id": f.nextDownloadID, "file_id": toDatumInt64(args[0]), "user_id": toDatumInt64(args[1]),
			"creat_time": "2026-09-22 10:00:00",
		})
	case strings.Contains(query, "DELETE FROM ptmj_file_download WHERE download_id IN"):
		kept := []map[string]interface{}{}
		for _, record := range f.downloads {
			matched := false
			for _, id := range args[:len(args)-1] {
				if record["download_id"] == toDatumInt64(id) && record["user_id"] == toDatumInt64(args[len(args)-1]) {
					matched = true
					break
				}
			}
			if !matched {
				kept = append(kept, record)
			}
		}
		f.downloads = kept
	case strings.Contains(query, "DELETE FROM ptmj_file_download WHERE file_id"):
		kept := []map[string]interface{}{}
		for _, record := range f.downloads {
			if !(record["file_id"] == toDatumInt64(args[0]) && record["user_id"] == toDatumInt64(args[1])) {
				kept = append(kept, record)
			}
		}
		f.downloads = kept
	case strings.Contains(query, "INSERT INTO ptmj_file_favorite"):
		f.fileFavs = append(f.fileFavs, [2]int64{toDatumInt64(args[0]), toDatumInt64(args[1])})
	case strings.Contains(query, "DELETE FROM ptmj_file_favorite"):
		kept := [][2]int64{}
		removed := int64(0)
		for _, pair := range f.fileFavs {
			if pair[0] == toDatumInt64(args[0]) && pair[1] == toDatumInt64(args[1]) {
				removed++
				continue
			}
			kept = append(kept, pair)
		}
		f.fileFavs = kept
		return fakeDatumResult{rows: removed}, nil
	case strings.Contains(query, "INSERT INTO ptmj_bookmark_favorite"):
		f.bookmarkFavs[toDatumInt64(args[0])] = toDatumInt64(args[1])
	case strings.Contains(query, "DELETE FROM ptmj_bookmark_favorite WHERE bookmark_id"):
		if _, existed := f.bookmarkFavs[toDatumInt64(args[0])]; existed {
			delete(f.bookmarkFavs, toDatumInt64(args[0]))
			return fakeDatumResult{rows: 1}, nil
		}
		return fakeDatumResult{rows: 0}, nil
	case strings.Contains(query, "UPDATE ptmj_bookmark SET del_flag"):
		row, exists := f.bookmarks[toDatumInt64(args[0])]
		if !exists || toDatumInt64(row["user_id"]) != toDatumInt64(args[1]) || toDatumInt64(row["del_flag"]) != 0 {
			return fakeDatumResult{rows: 0}, nil
		}
		row["del_flag"] = int64(1)
		return fakeDatumResult{rows: 1}, nil
	case strings.Contains(query, "UPDATE ptmj_bookmark SET cover_image"):
		row, exists := f.bookmarks[toDatumInt64(args[2])]
		if !exists || toDatumInt64(row["user_id"]) != toDatumInt64(args[3]) || toDatumInt64(row["del_flag"]) != 0 {
			return fakeDatumResult{rows: 0}, nil
		}
		row["cover_image"] = toDatumString(args[0])
		return fakeDatumResult{rows: 1}, nil
	case strings.Contains(query, "UPDATE ptmj_bookmark SET") && strings.Contains(query, "WHERE id = ? AND user_id = ?"):
		// UpdateOwner：SET 与首个列名之间有换行，用 WHERE 子句识别
		row, exists := f.bookmarks[toDatumInt64(args[9])]
		if !exists || toDatumInt64(row["user_id"]) != toDatumInt64(args[10]) || toDatumInt64(row["del_flag"]) != 0 {
			return fakeDatumResult{rows: 0}, nil
		}
		row["url"] = toDatumString(args[0])
		row["title"] = toDatumString(args[1])
		row["description"] = toDatumString(args[2])
		row["cover_image"] = toDatumString(args[3])
		row["subject"] = toDatumString(args[4])
		row["resource_type"] = toDatumString(args[5])
		row["collection"] = toDatumString(args[6])
		row["remark"] = toDatumString(args[7])
		return fakeDatumResult{rows: 1}, nil
	case strings.Contains(query, "ON CONFLICT (user_id)"):
		f.security[toDatumInt64(args[0])] = [2]string{toDatumString(args[1]), toDatumString(args[2])}
	case strings.Contains(query, "INSERT INTO ptmj_notification") && strings.Contains(query, "VALUES ('4', ?"):
		f.notifications = append(f.notifications, map[string]interface{}{
			"notify_type": "4", "title": toDatumString(args[0]),
			"upload_user_id": toDatumInt64(args[1]), "material_id": toDatumInt64(args[2]),
			"material_title_snapshot": toDatumString(args[3]),
		})
		return fakeDatumResult{rows: 1}, nil
	case strings.Contains(query, "INSERT INTO ptmj_notification"):
		f.nextDownloadID++
		f.notifications = append(f.notifications, map[string]interface{}{
			"notify_id": f.nextDownloadID, "notify_type": toDatumString(args[0]), "title": toDatumString(args[1]),
			"content": toDatumString(args[2]), "status": toDatumString(args[3]), "sort": toDatumInt64(args[4]),
			"display_mode":   toDatumString(args[5]),
			"upload_user_id": toDatumInt64(args[11]), "material_id": toDatumInt64(args[12]),
			"material_title_snapshot": toDatumString(args[13]),
			"scroll_time_interval":    toDatumInt64(args[16]), "remark": toDatumString(args[17]),
		})
		return fakeDatumResult{rows: 1}, nil
	case strings.Contains(query, "UPDATE ptmj_notification") && strings.Contains(query, "WHERE notify_id = ?"):
		notifyID := toDatumInt64(args[len(args)-1])
		for _, notification := range f.notifications {
			if notification["notify_id"] == notifyID {
				// Update SQL: notify_id=?, notify_type=?, title=?, content=?, status=?...
				notification["title"] = toDatumString(args[2])
				notification["status"] = toDatumString(args[4])
			}
		}
	case strings.Contains(query, "DELETE FROM ptmj_notification WHERE notify_id IN"):
		kept := []map[string]interface{}{}
		for _, notification := range f.notifications {
			removed := false
			for _, id := range args {
				if notification["notify_id"] == toDatumInt64(id) {
					removed = true
					break
				}
			}
			if !removed {
				kept = append(kept, notification)
			}
		}
		f.notifications = kept
	case strings.Contains(query, "INSERT INTO ptmj_security"):
		f.security[toDatumInt64(args[0])] = [2]string{toDatumString(args[1]), toDatumString(args[2])}
	case strings.Contains(query, "UPDATE ptmj_report SET result"):
		for _, report := range f.reports {
			if report["report_id"] == toDatumInt64(args[3]) {
				report["result"] = toDatumString(args[0])
				report["update_by"] = toDatumString(args[1])
				report["update_time"] = "2026-09-22 11:00:00"
				if len(args) > 4 {
					report["remark"] = toDatumString(args[2])
				}
			}
		}
	case strings.Contains(query, "UPDATE ptmj_report SET reason"):
		updated := int64(0)
		for _, report := range f.reports {
			if report["report_id"] == toDatumInt64(args[3]) && report["user_id"] == toDatumInt64(args[4]) && report["result"] == "0" {
				report["reason"] = toDatumString(args[0])
				report["remark"] = toDatumString(args[1])
				updated++
			}
		}
		return fakeDatumResult{rows: updated}, nil
	case strings.Contains(query, "UPDATE ptmj_bookmark_report SET result"):
		for _, report := range f.bookmarkReports {
			if report["report_id"] == toDatumInt64(args[3]) {
				report["result"] = toDatumString(args[0])
				report["update_time"] = "2026-09-22 11:00:00"
			}
		}
	case strings.Contains(query, "UPDATE ptmj_bookmark_report SET reason"):
		updated := int64(0)
		for _, report := range f.bookmarkReports {
			if report["report_id"] == toDatumInt64(args[3]) && report["user_id"] == toDatumInt64(args[4]) && report["result"] == "0" {
				report["reason"] = toDatumString(args[0])
				report["remark"] = toDatumString(args[1])
				updated++
			}
		}
		return fakeDatumResult{rows: updated}, nil
	case strings.Contains(query, "DELETE FROM ptmj_report WHERE report_id"):
		kept := []map[string]interface{}{}
		removed := int64(0)
		for _, report := range f.reports {
			if report["report_id"] == toDatumInt64(args[0]) && report["user_id"] == toDatumInt64(args[1]) {
				removed++
				continue
			}
			kept = append(kept, report)
		}
		f.reports = kept
		return fakeDatumResult{rows: removed}, nil
	case strings.Contains(query, "DELETE FROM ptmj_bookmark_report WHERE report_id"):
		keptBR := []map[string]interface{}{}
		removedBR := int64(0)
		for _, report := range f.bookmarkReports {
			if report["report_id"] == toDatumInt64(args[0]) && report["user_id"] == toDatumInt64(args[1]) {
				removedBR++
				continue
			}
			keptBR = append(keptBR, report)
		}
		f.bookmarkReports = keptBR
		return fakeDatumResult{rows: removedBR}, nil
	case strings.Contains(query, "UPDATE ptmj_file_download SET remark"):
		for _, record := range f.downloads {
			if record["download_id"] == toDatumInt64(args[2]) && record["user_id"] == toDatumInt64(args[3]) {
				record["remark"] = toDatumString(args[0])
			}
		}
	case strings.Contains(query, "UPDATE ptmj_file SET file_status"):
		for _, file := range f.files {
			if file["file_id"] == toDatumInt64(args[2]) {
				file["file_status"] = toDatumInt64(args[0])
				file["reviewer"] = toDatumString(args[1])
			}
		}
	case strings.Contains(query, "UPDATE ptmj_bookmark SET status"):
		for _, bookmark := range f.bookmarks {
			if bookmarkID, ok := bookmark["id"]; ok && bookmarkID == toDatumInt64(args[1]) {
				bookmark["status"] = toDatumInt64(args[0])
			}
		}
	case strings.Contains(query, "UPDATE ptmj_file SET del_flag"):
		for _, file := range f.files {
			if file["file_id"] == toDatumInt64(args[0]) && file["user_id"] == toDatumInt64(args[1]) {
				file["del_flag"] = int64(1)
			}
		}
	case strings.Contains(query, "UPDATE ptmj_file"):
		for _, file := range f.files {
			if file["file_id"] == toDatumInt64(args[len(args)-1]) && file["user_id"] == toDatumInt64(args[len(args)-2]) {
				file["file_name"] = toDatumString(args[0])
				file["file_year"] = toDatumInt64(args[1])
				file["file_type"] = toDatumInt64(args[2])
				file["file_school"] = toDatumString(args[3])
				file["file_subject"] = toDatumString(args[4])
				file["remark"] = toDatumString(args[5])
			}
		}
	}
	return fakeDatumResult{rows: 1}, nil
}

func (f *fakeDatumDB) findUserByID(userID int64) *datumUser {
	for _, user := range f.users {
		if user.UserID == userID {
			return user
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mustHashSecret(t *testing.T, plain string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash secret: %v", err)
	}
	return string(hash)
}

func newDatumAuthStore(t *testing.T, overrides map[string]string) (*Store, *fakeDatumDB) {
	t.Helper()
	store, redis := newDatumTestStore(t, overrides)
	db := newFakeDatumDB()
	store.conn = db
	store.datum = newDatumIdentity("test:datum", redis, time.Hour)
	return store, db
}

func datumEngine(t *testing.T, store *Store) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerDatumRoutes(engine, store)
	return engine
}

func datumJSON(t *testing.T, engine *gin.Engine, method, path string, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func datumBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", recorder.Body.String(), err)
	}
	return body
}

func seedDatumUser(t *testing.T, db *fakeDatumDB, username, password, status string, withSecurity bool) {
	t.Helper()
	db.seedSeq++
	user := &datumUser{UserID: int64(6 + db.seedSeq), UserName: username, Password: mustHashSecret(t, password), Avatar: "/a.png", Count: 3, Status: status}
	db.users[strings.ToLower(username)] = user
	if withSecurity {
		db.security[user.UserID] = [2]string{
			"你小学的名字|你宠物名字|出生城市",
			strings.Join([]string{mustHashSecret(t, "a1"), mustHashSecret(t, "a2"), mustHashSecret(t, "a3")}, "|"),
		}
	}
}

func seedDatumCaptcha(t *testing.T, store *Store, uuid string) {
	t.Helper()
	if err := store.security.storeCaptcha(uuid, "123456", time.Minute); err != nil {
		t.Fatalf("store captcha: %v", err)
	}
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestDatumLoginIssuesSession(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumCaptcha(t, store, "cap-1")
	engine := datumEngine(t, store)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "123456", "uuid": "cap-1",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", recorder.Code, recorder.Body.String())
	}
	data, _ := datumBody(t, recorder)["data"].(map[string]interface{})
	token, _ := data["token"].(string)
	if token == "" {
		t.Fatalf("missing token in %v", data)
	}
	if _, ok := data["expiresAt"].(float64); !ok {
		t.Fatalf("missing expiresAt in %v", data)
	}
	userID, err := store.datum.ResolveSession(token)
	if err != nil || userID != 7 {
		t.Fatalf("ResolveSession = (%d, %v), want (7, nil)", userID, err)
	}
}

func TestDatumLoginEnforcesConstraints(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "0", false) // status 0 = 封禁
	engine := datumEngine(t, store)

	// 验证码错误
	seedDatumCaptcha(t, store, "cap-wrong")
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "000000", "uuid": "cap-wrong",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("wrong captcha status = %d", recorder.Code)
	}

	// 封禁账号
	seedDatumCaptcha(t, store, "cap-banned")
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "123456", "uuid": "cap-banned",
	})
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("banned status = %d", recorder.Code)
	}
}

func TestDatumLoginLocksAfterRepeatedFailures(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	engine := datumEngine(t, store)

	// 默认阈值为 5：连续 5 次错误密码后触发锁定
	for attempt := 1; attempt <= 5; attempt++ {
		seedDatumCaptcha(t, store, "cap-fail")
		recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
			"username": "alice", "password": "wrong-password", "code": "123456", "uuid": "cap-fail",
		})
		want := http.StatusBadRequest
		if attempt == 5 {
			want = http.StatusTooManyRequests
		}
		if recorder.Code != want {
			t.Fatalf("attempt %d status = %d body %s", attempt, recorder.Code, recorder.Body.String())
		}
	}
	// 锁定后即使密码正确也拒绝
	seedDatumCaptcha(t, store, "cap-locked")
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "123456", "uuid": "cap-locked",
	})
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("locked status = %d", recorder.Code)
	}
}

func TestDatumRegisterCreatesUserAndSecurity(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumCaptcha(t, store, "cap-reg")
	engine := datumEngine(t, store)
	payload := map[string]string{
		"username": "bob", "password": "pass123", "confirmPassword": "pass123",
		"securityQuestionOne": "q1", "securityAnswerOne": "a1",
		"securityQuestionTwo": "q2", "securityAnswerTwo": "a2",
		"securityQuestionThree": "q3", "securityAnswerThree": "a3",
		"code": "123456", "uuid": "cap-reg", "avatar": "/default.png",
	}
	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/register", "", payload)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", recorder.Code, recorder.Body.String())
	}
	user := db.users["bob"]
	if user == nil {
		t.Fatal("user was not created")
	}
	if !verifyDatumSecret(user.Password, "pass123") || verifyDatumSecret(user.Password, "other") {
		t.Fatal("stored password hash mismatch")
	}
	security := db.security[user.UserID]
	if security[0] != "q1|q2|q3" {
		t.Fatalf("question = %q", security[0])
	}
	parts := strings.Split(security[1], "|")
	if len(parts) != 3 || !verifyDatumSecret(parts[0], "a1") || !verifyDatumSecret(parts[2], "a3") {
		t.Fatal("stored answer hashes mismatch")
	}

	// 重复用户名 → 409
	seedDatumCaptcha(t, store, "cap-reg-2")
	payload["uuid"], payload["code"] = "cap-reg-2", "123456"
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/register", "", payload)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestDatumGetInfoAndLogout(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	seedDatumCaptcha(t, store, "cap-1")
	engine := datumEngine(t, store)

	recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
		"username": "alice", "password": "secret5", "code": "123456", "uuid": "cap-1",
	})
	token := datumBody(t, recorder)["data"].(map[string]interface{})["token"].(string)

	recorder = datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("getInfo status = %d body %s", recorder.Code, recorder.Body.String())
	}
	data := datumBody(t, recorder)["data"].(map[string]interface{})
	user := data["user"].(map[string]interface{})
	if user["userId"].(float64) != 7 || user["userName"] != "alice" || user["status"] != "1" {
		t.Fatalf("getInfo user = %v", user)
	}
	roles := data["roles"].([]interface{})
	if len(roles) != 1 || roles[0] != "ptmj_user" {
		t.Fatalf("roles = %v", roles)
	}

	// 未携带 token → 401
	if recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", "", nil); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no-token status = %d", recorder.Code)
	}
	// 登出后会话失效
	if recorder := datumJSON(t, engine, http.MethodPost, "/datum/user/logout", token, nil); recorder.Code != http.StatusOK {
		t.Fatalf("logout status = %d", recorder.Code)
	}
	if recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", token, nil); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout status = %d", recorder.Code)
	}
}

func TestDatumPasswordRecoveryFlow(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "old-pass-1", "1", true)
	engine := datumEngine(t, store)

	// 第一步：验证码校验并返回密保问题，签发重置工单
	seedDatumCaptcha(t, store, "cap-step1")
	recorder := datumJSON(t, engine, http.MethodGet, "/datum/user/securityQuestions?userName=alice&code=123456&uuid=cap-step1", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("step1 status = %d body %s", recorder.Code, recorder.Body.String())
	}
	questions := datumBody(t, recorder)["data"].([]interface{})
	if len(questions) != 3 {
		t.Fatalf("questions = %v", questions)
	}

	// 第二步：同一 code/uuid + 工单 + 三问三答 → 重置密码
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetPasswordBySecurity", "", map[string]string{
		"username": "alice", "code": "123456", "uuid": "cap-step1",
		"securityAnswerOne": "a1", "securityAnswerTwo": "a2", "securityAnswerThree": "a3",
		"newPassword": "new-pass-9", "confirmPassword": "new-pass-9",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("step2 status = %d body %s", recorder.Code, recorder.Body.String())
	}
	if verifyDatumSecret(db.users["alice"].Password, "old-pass-1") || !verifyDatumSecret(db.users["alice"].Password, "new-pass-9") {
		t.Fatal("password was not updated")
	}

	// 工单已被消费：重放同一请求 → 400
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetPasswordBySecurity", "", map[string]string{
		"username": "alice", "code": "123456", "uuid": "cap-step1",
		"securityAnswerOne": "a1", "securityAnswerTwo": "a2", "securityAnswerThree": "a3",
		"newPassword": "another-1", "confirmPassword": "another-1",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("replay status = %d", recorder.Code)
	}

	// 错误答案 → 400
	seedDatumCaptcha(t, store, "cap-step2")
	datumJSON(t, engine, http.MethodGet, "/datum/user/securityQuestions?userName=alice&code=123456&uuid=cap-step2", "", nil)
	recorder = datumJSON(t, engine, http.MethodPost, "/datum/user/resetPasswordBySecurity", "", map[string]string{
		"username": "alice", "code": "123456", "uuid": "cap-step2",
		"securityAnswerOne": "wrong", "securityAnswerTwo": "a2", "securityAnswerThree": "a3",
		"newPassword": "another-1", "confirmPassword": "another-1",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("wrong answer status = %d", recorder.Code)
	}
}

// 委托 With* 系列到 Query/Exec，满足查询构建器（db.WithDriver().Table().All()）。
func (f *fakeDatumDB) QueryWithConnection(_ string, query string, args ...interface{}) ([]map[string]interface{}, error) {
	return f.Query(query, args...)
}

func (f *fakeDatumDB) QueryWith(_ *sql.Tx, _ string, query string, args ...interface{}) ([]map[string]interface{}, error) {
	return f.Query(query, args...)
}

func (f *fakeDatumDB) QueryWithTx(_ *sql.Tx, query string, args ...interface{}) ([]map[string]interface{}, error) {
	return f.Query(query, args...)
}

func (f *fakeDatumDB) ExecWithConnection(_ string, query string, args ...interface{}) (sql.Result, error) {
	return f.Exec(query, args...)
}

func (f *fakeDatumDB) ExecWith(_ *sql.Tx, _ string, query string, args ...interface{}) (sql.Result, error) {
	return f.Exec(query, args...)
}

func (f *fakeDatumDB) ExecWithTx(_ *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	return f.Exec(query, args...)
}

func (f *fakeDatumDB) Name() string { return "postgresql" }

func (f *fakeDatumDB) notificationRows(displayMode string) []map[string]interface{} {
	rows := []map[string]interface{}{}
	for _, notification := range f.notifications {
		if toDatumString(notification["status"]) == "0" && toDatumString(notification["display_mode"]) == displayMode {
			rows = append(rows, notification)
		}
	}
	return rows
}
