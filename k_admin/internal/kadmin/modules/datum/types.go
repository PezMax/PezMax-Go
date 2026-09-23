package datum

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Domain rows for the ptmj_* tables. Column names and quirks (creat_by,
// single-column bookmark_favorite key, char(1) status flags) mirror the
// legacy schema exactly; see schema.go and sql/ptmj_schema.sql.

type File struct {
	FileID      int64
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
	DelFlag     int64
	CreateBy    string
	CreateTime  string
	UpdateBy    string
	UpdateTime  string
	Remark      string
}

type FileDownload struct {
	DownloadID int64
	FileID     int64
	UserID     int64
	CreatBy    string
	CreatTime  string
	UpdateBy   string
	UpdateTime string
	Remark     string
}

type Report struct {
	ReportID   int64
	FileID     int64
	UserID     int64
	Reason     string
	Result     string
	CreateBy   string
	CreateTime string
	UpdateBy   string
	UpdateTime string
	Remark     string
}

type Bookmark struct {
	ID           int64
	UserID       int64
	URL          string
	Title        string
	Description  string
	CoverImage   string
	Subject      string
	ResourceType string
	Collection   string
	Status       int64
	DelFlag      int64
	CreateBy     string
	CreateTime   string
	UpdateBy     string
	UpdateTime   string
	Remark       string
}

type BookmarkReport struct {
	ReportID   int64
	BookmarkID int64
	UserID     int64
	Reason     string
	Result     string
	CreateBy   string
	CreateTime string
	UpdateBy   string
	UpdateTime string
	Remark     string
}

type Notification struct {
	NotifyID              int64
	NotifyType            string
	Title                 string
	Content               string
	Status                string
	Sort                  int64
	DisplayMode           string
	FaultStartTime        string
	FaultEndTime          string
	MaintenanceStartTime  string
	MaintenanceEndTime    string
	RemindBeforeMinutes   int64
	UploadUserID          int64
	MaterialID            int64
	MaterialTitleSnapshot string
	PublishStart          string
	PublishEnd            string
	ScrollTimeInterval    int64
	CreateBy              string
	CreateTime            string
	UpdateBy              string
	UpdateTime            string
	Remark                string
}

// Page carries the shared pagination answer for list queries.
type Page struct {
	Items    interface{}
	Total    int64
	Page     int
	PageSize int
}

// FileFilter narrows file lists. OnlyApproved enforces the anonymous desktop
// contract: file_status = 1 AND del_flag = 0.
type FileFilter struct {
	Page         int
	PageSize     int
	FileType     int64
	FileSubject  string
	FileSchool   string
	FileYear     int64
	Keyword      string
	UserID       int64 // >0 restricts to one owner (my-uploads)
	OnlyApproved bool
}

func (f *FileFilter) page() (int, int) {
	page, size := f.Page, f.PageSize
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

// BookmarkFilter narrows bookmark lists for anonymous browsing.
type BookmarkFilter struct {
	Page         int
	PageSize     int
	Subject      string
	ResourceType string
	Collection   string
	URL          string // exact match; the desktop re-queries a just-saved bookmark by URL
	Keyword      string
	UserID       int64
	OnlyApproved bool
}

func (f *BookmarkFilter) page() (int, int) {
	page, size := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	// 桌面端上传管理器一次拉取本人全部书签（pageSize=1000），书签行较窄，上限对齐 datumPageParams
	if size > 1000 {
		size = 1000
	}
	return page, size
}

const fileColumns = `file_id, user_id, file_name, file_url, file_size, file_format, file_year, file_type,
	file_school, file_subject, reviewer, file_status, del_flag, create_by, create_time, update_by, update_time, remark`

const bookmarkColumns = `id, user_id, url, title, description, cover_image, subject, resource_type, collection,
	status, del_flag, create_by, create_time, update_by, update_time, remark`

const notificationColumns = `notify_id, notify_type, title, content, status, sort, display_mode,
	fault_start_time, fault_end_time, maintenance_start_time, maintenance_end_time, remind_before_minutes,
	upload_user_id, material_id, material_title_snapshot, publish_start, publish_end, scroll_time_interval,
	create_by, create_time, update_by, update_time, remark`

func scanFile(row map[string]interface{}) File {
	return File{
		FileID:      ScanInt64(row["file_id"]),
		UserID:      ScanInt64(row["user_id"]),
		FileName:    ScanString(row["file_name"]),
		FileURL:     ScanString(row["file_url"]),
		FileSize:    ScanInt64(row["file_size"]),
		FileFormat:  ScanString(row["file_format"]),
		FileYear:    ScanInt64(row["file_year"]),
		FileType:    ScanInt64(row["file_type"]),
		FileSchool:  ScanString(row["file_school"]),
		FileSubject: ScanString(row["file_subject"]),
		Reviewer:    ScanString(row["reviewer"]),
		FileStatus:  ScanInt64(row["file_status"]),
		DelFlag:     ScanInt64(row["del_flag"]),
		CreateBy:    ScanString(row["create_by"]),
		CreateTime:  ScanString(row["create_time"]),
		UpdateBy:    ScanString(row["update_by"]),
		UpdateTime:  ScanString(row["update_time"]),
		Remark:      ScanString(row["remark"]),
	}
}

func scanBookmark(row map[string]interface{}) Bookmark {
	return Bookmark{
		ID:           ScanInt64(row["id"]),
		UserID:       ScanInt64(row["user_id"]),
		URL:          ScanString(row["url"]),
		Title:        ScanString(row["title"]),
		Description:  ScanString(row["description"]),
		CoverImage:   ScanString(row["cover_image"]),
		Subject:      ScanString(row["subject"]),
		ResourceType: ScanString(row["resource_type"]),
		Collection:   ScanString(row["collection"]),
		Status:       ScanInt64(row["status"]),
		DelFlag:      ScanInt64(row["del_flag"]),
		CreateBy:     ScanString(row["create_by"]),
		CreateTime:   ScanString(row["create_time"]),
		UpdateBy:     ScanString(row["update_by"]),
		UpdateTime:   ScanString(row["update_time"]),
		Remark:       ScanString(row["remark"]),
	}
}

func scanNotification(row map[string]interface{}) Notification {
	return Notification{
		NotifyID:              ScanInt64(row["notify_id"]),
		NotifyType:            ScanString(row["notify_type"]),
		Title:                 ScanString(row["title"]),
		Content:               ScanString(row["content"]),
		Status:                ScanString(row["status"]),
		Sort:                  ScanInt64(row["sort"]),
		DisplayMode:           ScanString(row["display_mode"]),
		FaultStartTime:        ScanString(row["fault_start_time"]),
		FaultEndTime:          ScanString(row["fault_end_time"]),
		MaintenanceStartTime:  ScanString(row["maintenance_start_time"]),
		MaintenanceEndTime:    ScanString(row["maintenance_end_time"]),
		RemindBeforeMinutes:   ScanInt64(row["remind_before_minutes"]),
		UploadUserID:          ScanInt64(row["upload_user_id"]),
		MaterialID:            ScanInt64(row["material_id"]),
		MaterialTitleSnapshot: ScanString(row["material_title_snapshot"]),
		PublishStart:          ScanString(row["publish_start"]),
		PublishEnd:            ScanString(row["publish_end"]),
		ScrollTimeInterval:    ScanInt64(row["scroll_time_interval"]),
		CreateBy:              ScanString(row["create_by"]),
		CreateTime:            ScanString(row["create_time"]),
		UpdateBy:              ScanString(row["update_by"]),
		UpdateTime:            ScanString(row["update_time"]),
		Remark:                ScanString(row["remark"]),
	}
}

// ScanInt64 coerces driver-dependent column values (int64, int, float64,
// []byte, string) into int64; nil and garbage become 0.
func ScanInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case []byte:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(string(typed)), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}

// ScanString coerces column values into string; nil becomes "".
func ScanString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format("2006-01-02 15:04:05")
	case nil:
		return ""
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	}
}
