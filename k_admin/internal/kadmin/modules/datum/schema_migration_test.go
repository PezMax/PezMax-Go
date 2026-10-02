package datum

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/db"
	_ "github.com/mattn/go-sqlite3"
)

// schemaSQLConnection adapts database/sql to the db.Connection interface the
// repositories use, so SQLite/PostgreSQL regressions can exercise real SQL.
type schemaSQLConnection struct {
	db.Connection
	database *sql.DB
}

func (c *schemaSQLConnection) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.database.Exec(query, args...)
}

func (c *schemaSQLConnection) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := c.database.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := []map[string]interface{}{}
	for rows.Next() {
		values, targets := make([]interface{}, len(columns)), make([]interface{}, len(columns))
		for index := range values {
			targets[index] = &values[index]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		row := make(map[string]interface{}, len(columns))
		for index, column := range columns {
			row[column] = values[index]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// 双键表上 A、B 各自收藏同一书签后，取消 A 只删 A 的关联；
// 重复取消与未关联删除按 ErrNotFound 处理。
func TestBookmarkFavoritesSQLitePreservesOtherUsers(t *testing.T) {
	database, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec(`CREATE TABLE ptmj_bookmark_favorite (
		bookmark_id INTEGER NOT NULL, user_id INTEGER NOT NULL, PRIMARY KEY (bookmark_id, user_id))`); err != nil {
		t.Fatal(err)
	}
	repo := NewBookmarkFavoriteRepo(&schemaSQLConnection{database: database})
	for _, userID := range []int64{7, 8} {
		if err := repo.Add(100, userID); err != nil {
			t.Fatalf("user %d cannot favorite the same bookmark: %v", userID, err)
		}
	}
	if err := repo.Remove(100, 7); err != nil {
		t.Fatal(err)
	}
	if exists, err := repo.Exists(100, 8); err != nil || !exists {
		t.Fatalf("removing user 7 affected user 8: exists=%v, err=%v", exists, err)
	}
	if exists, err := repo.Exists(100, 7); err != nil || exists {
		t.Fatalf("user 7 favorite still exists: exists=%v, err=%v", exists, err)
	}
	if err := repo.Remove(100, 7); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing favorite error=%v", err)
	}
}
