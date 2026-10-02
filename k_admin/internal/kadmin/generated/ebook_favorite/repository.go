// Maintained manually for the (ebook_id, user_id) composite primary key.
// Do not regenerate with codegen until it supports composite-key CRUD.

package ebook_favorite

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

var errEbookFavoriteNotFound = errors.New("ebook_favorite not found")

type repository struct {
	conn db.Connection
}

func newRepository(conn db.Connection) *repository {
	return &repository{conn: conn}
}

func (r *repository) list(filter EbookFavoriteFilter) (EbookFavoritePage, error) {
	where, args := ebook_favoriteWhere(filter)
	countRows, err := r.conn.Query(`SELECT count(*) AS count FROM public.ptmj_ebook_favorite `+where, args...)
	if err != nil {
		return EbookFavoritePage{}, err
	}
	total := int64(0)
	if len(countRows) > 0 {
		total = toInt64(countRows[0]["count"])
	}
	queryArgs := append(append([]interface{}{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.conn.Query(`SELECT ebook_id, user_id FROM public.ptmj_ebook_favorite `+where+` ORDER BY ebook_id DESC, user_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return EbookFavoritePage{}, err
	}
	items := make([]EbookFavorite, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapEbookFavorite(row))
	}
	return EbookFavoritePage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func ebook_favoriteWhere(filter EbookFavoriteFilter) (string, []interface{}) {
	conditions := make([]string, 0, 4)
	args := make([]interface{}, 0, 4)
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (r *repository) findByID(id favoriteKey) (EbookFavorite, bool, error) {
	rows, err := r.conn.Query(`SELECT ebook_id, user_id FROM public.ptmj_ebook_favorite WHERE ebook_id = ? AND user_id = ?`, id.resourceID, id.userID)
	if err != nil {
		return EbookFavorite{}, false, err
	}
	if len(rows) == 0 {
		return EbookFavorite{}, false, nil
	}
	return mapEbookFavorite(rows[0]), true, nil
}

func (r *repository) mustFind(id favoriteKey) (EbookFavorite, error) {
	item, found, err := r.findByID(id)
	if err != nil {
		return EbookFavorite{}, err
	}
	if !found {
		return EbookFavorite{}, errEbookFavoriteNotFound
	}
	return item, nil
}

func (r *repository) create(payload EbookFavoritePayload) (EbookFavorite, error) {
	_, err := r.conn.Exec(`INSERT INTO public.ptmj_ebook_favorite (ebook_id, user_id) VALUES (?, ?)`, payload.EbookId, payload.UserId)
	if err != nil {
		return EbookFavorite{}, fmt.Errorf("create ebook_favorite: %w", err)
	}
	return r.mustFind(favoriteKey{payload.EbookId, payload.UserId})
}

func (r *repository) update(id favoriteKey, payload EbookFavoritePayload) (EbookFavorite, error) {
	result, err := r.conn.Exec(`UPDATE public.ptmj_ebook_favorite SET user_id = ? WHERE ebook_id = ? AND user_id = ?`, payload.UserId, id.resourceID, id.userID)
	if err != nil {
		return EbookFavorite{}, fmt.Errorf("update ebook_favorite %d,%d: %w", id.resourceID, id.userID, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return EbookFavorite{}, errEbookFavoriteNotFound
	}
	return r.mustFind(favoriteKey{id.resourceID, payload.UserId})
}

func (r *repository) delete(id favoriteKey) error {
	result, err := r.conn.Exec(`DELETE FROM public.ptmj_ebook_favorite WHERE ebook_id = ? AND user_id = ?`, id.resourceID, id.userID)
	if err != nil {
		return fmt.Errorf("delete ebook_favorite %d,%d: %w", id.resourceID, id.userID, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errEbookFavoriteNotFound
	}
	return nil
}

func mapEbookFavorite(row map[string]interface{}) EbookFavorite {
	return EbookFavorite{
		EbookId: toInt64(row["ebook_id"]),
		UserId:  toInt64(row["user_id"]),
	}
}

func toInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float64:
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

func toFloat64(value interface{}) float64 {
	switch typed := value.(type) {
	case float32:
		return float64(typed)
	case float64:
		return typed
	case int64:
		return float64(typed)
	case []byte:
		parsed, _ := strconv.ParseFloat(string(typed), 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(typed, 64)
		return parsed
	default:
		return 0
	}
}

func toBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(typed)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseBool(string(typed))
		return parsed
	default:
		return false
	}
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func timeText(value interface{}) string {
	if value == nil {
		return ""
	}
	if parsed, ok := value.(time.Time); ok {
		return parsed.In(time.Local).Format("2006-01-02 15:04:05")
	}
	text := strings.TrimSpace(toString(value))
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.In(time.Local).Format("2006-01-02 15:04:05")
		}
	}
	return text
}

// optionalTime maps an empty string to NULL for nullable date/time columns.
func optionalTime(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
