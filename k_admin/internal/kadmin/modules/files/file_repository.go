package files

import (
	"fmt"
	"strconv"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/modules/db/dialect"
)

const fileTableName = "kadmin_files"

type fileRepository interface {
	Create(record *fileRecord) error
	FindReady(id int64) (fileRecord, bool, error)
	UpdateStatus(id int64, status string, storageName string, bucket string, deletedAt *time.Time) error
}

type databaseFileRepository struct {
	conn db.Connection
}

func newDatabaseFileRepository(conn db.Connection) *databaseFileRepository {
	return &databaseFileRepository{conn: conn}
}

func (r *databaseFileRepository) Create(record *fileRecord) error {
	if record == nil {
		return fmt.Errorf("file record is required")
	}
	now := time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	id, err := db.WithDriver(r.conn).Table(fileTableName).Insert(dialect.H{
		"object_key":    record.ObjectKey,
		"original_name": record.OriginalName,
		"extension":     record.Extension,
		"content_type":  record.ContentType,
		"size":          record.Size,
		"sha256":        record.SHA256,
		"storage":       record.Storage,
		"bucket":        record.Bucket,
		"purpose":       record.Purpose,
		"visibility":    record.Visibility,
		"status":        record.Status,
		"created_by":    record.CreatedBy,
		"created_at":    record.CreatedAt,
		"updated_at":    record.UpdatedAt,
		"expires_at":    record.ExpiresAt,
	})
	if err != nil {
		return err
	}
	record.ID = id
	return nil
}

func (r *databaseFileRepository) FindReady(id int64) (fileRecord, bool, error) {
	rows, err := db.WithDriver(r.conn).
		Table(fileTableName).
		Where("id", "=", id).
		Where("status", "=", fileStatusReady).
		All()
	if err != nil {
		return fileRecord{}, false, err
	}
	if len(rows) == 0 {
		return fileRecord{}, false, nil
	}
	return mapFileRecord(rows[0]), true, nil
}

func (r *databaseFileRepository) UpdateStatus(id int64, status string, storageName string, bucket string, deletedAt *time.Time) error {
	values := dialect.H{
		"status":     status,
		"updated_at": time.Now(),
	}
	if storageName != "" {
		values["storage"] = storageName
	}
	if bucket != "" {
		values["bucket"] = bucket
	}
	if deletedAt != nil {
		values["deleted_at"] = *deletedAt
	}
	_, err := db.WithDriver(r.conn).
		Table(fileTableName).
		Where("id", "=", id).
		Update(values)
	return err
}

func mapFileRecord(row map[string]interface{}) fileRecord {
	return fileRecord{
		ID:           toInt64(row["id"]),
		ObjectKey:    toString(row["object_key"]),
		OriginalName: toString(row["original_name"]),
		Extension:    toString(row["extension"]),
		ContentType:  toString(row["content_type"]),
		Size:         toInt64(row["size"]),
		SHA256:       toString(row["sha256"]),
		Storage:      toString(row["storage"]),
		Bucket:       toString(row["bucket"]),
		Purpose:      toString(row["purpose"]),
		Visibility:   toString(row["visibility"]),
		Status:       toString(row["status"]),
		CreatedBy:    toInt64(row["created_by"]),
		CreatedAt:    fileRecordTime(row["created_at"]),
		UpdatedAt:    fileRecordTime(row["updated_at"]),
		DeletedAt:    optionalFileRecordTime(row["deleted_at"]),
		ExpiresAt:    optionalFileRecordTime(row["expires_at"]),
	}
}

func fileRecordTime(value interface{}) time.Time {
	if parsed, ok := value.(time.Time); ok {
		return parsed
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, toString(value)); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func optionalFileRecordTime(value interface{}) *time.Time {
	if value == nil {
		return nil
	}
	parsed := fileRecordTime(value)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return ""
	}
}

func toInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}
