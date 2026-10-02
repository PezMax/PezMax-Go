package datum

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/db"
	_ "github.com/lib/pq"
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

// migrationPostgres dials the opt-in isolated migration database; the name
// prefix is a hard guard against pointing this at real business data.
func migrationPostgres(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("KADMIN_TEST_MIGRATION_DSN")
	if dsn == "" {
		t.Skip("set KADMIN_TEST_MIGRATION_DSN to a disposable pezmax_system_migration_* database")
	}
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	var databaseName string
	if err := database.QueryRow("SELECT current_database()").Scan(&databaseName); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(databaseName, "pezmax_system_migration_") {
		t.Fatal("migration regression requires an isolated pezmax_system_migration_* database")
	}
	if _, err := database.Exec("DROP TABLE IF EXISTS public.ptmj_bookmark_favorite"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE public.ptmj_bookmark_favorite (
		bookmark_id INT NOT NULL PRIMARY KEY, user_id INT)`); err != nil {
		t.Fatal(err)
	}
	return database
}

func bookmarkPrimaryKey(t *testing.T, database *sql.DB) string {
	t.Helper()
	var definition string
	if err := database.QueryRow(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = 'public.ptmj_bookmark_favorite'::regclass AND contype = 'p'`).Scan(&definition); err != nil {
		t.Fatal(err)
	}
	return definition
}

// 旧单列主键迁移：保留数据、支持第二用户收藏同一书签、重复启动幂等。
func TestBookmarkFavoritePostgresMigrationPreservesDataAndIsIdempotent(t *testing.T) {
	database := migrationPostgres(t)
	if _, err := database.Exec("INSERT INTO public.ptmj_bookmark_favorite VALUES (100, 7)"); err != nil {
		t.Fatal(err)
	}
	connection := &schemaSQLConnection{database: database}
	for pass := 0; pass < 2; pass++ {
		if err := EnsureSchema(connection); err != nil {
			t.Fatalf("startup %d: %v", pass+1, err)
		}
	}
	if key := bookmarkPrimaryKey(t, database); key != "PRIMARY KEY (bookmark_id, user_id)" {
		t.Fatalf("migrated key = %s", key)
	}
	if _, err := database.Exec("INSERT INTO public.ptmj_bookmark_favorite VALUES (100, 8)"); err != nil {
		t.Fatalf("second user cannot save the same bookmark: %v", err)
	}
	if err := EnsureSchema(connection); err != nil {
		t.Fatalf("repeated startup: %v", err)
	}
	var rows int
	if err := database.QueryRow("SELECT count(*) FROM public.ptmj_bookmark_favorite WHERE bookmark_id = 100 AND user_id IN (7, 8)").Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("migration did not preserve both associations: rows=%d, err=%v", rows, err)
	}
}

// NULL 属主明确失败：原主键与数据原样保留，不猜测属主。
func TestBookmarkFavoritePostgresMigrationRefusesNullOwnersWithoutDataLoss(t *testing.T) {
	database := migrationPostgres(t)
	if _, err := database.Exec("INSERT INTO public.ptmj_bookmark_favorite VALUES (100, 7), (101, NULL)"); err != nil {
		t.Fatal(err)
	}
	err := EnsureSchema(&schemaSQLConnection{database: database})
	if err == nil || !strings.Contains(err.Error(), "NULL user_id") {
		t.Fatalf("ownerless migration error = %v", err)
	}
	if key := bookmarkPrimaryKey(t, database); key != "PRIMARY KEY (bookmark_id)" {
		t.Fatalf("failed migration changed original key: %s", key)
	}
	var rows, nulls int
	if err := database.QueryRow("SELECT count(*), count(*) FILTER (WHERE user_id IS NULL) FROM public.ptmj_bookmark_favorite").Scan(&rows, &nulls); err != nil || rows != 2 || nulls != 1 {
		t.Fatalf("failed migration changed legacy rows: rows=%d, nulls=%d, err=%v", rows, nulls, err)
	}
}

// 已存在的合法逆序联合主键被接受且不被改动。
func TestBookmarkFavoritePostgresMigrationAcceptsReverseCompositeKeyOrder(t *testing.T) {
	database := migrationPostgres(t)
	if _, err := database.Exec(`ALTER TABLE public.ptmj_bookmark_favorite DROP CONSTRAINT ptmj_bookmark_favorite_pkey;
		ALTER TABLE public.ptmj_bookmark_favorite ADD PRIMARY KEY (user_id, bookmark_id);
		INSERT INTO public.ptmj_bookmark_favorite VALUES (100, 7)`); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(&schemaSQLConnection{database: database}); err != nil {
		t.Fatalf("valid reverse-order composite key rejected: %v", err)
	}
	if key := bookmarkPrimaryKey(t, database); key != "PRIMARY KEY (user_id, bookmark_id)" {
		t.Fatalf("valid existing composite key was changed: %s", key)
	}
	if _, err := database.Exec("INSERT INTO public.ptmj_bookmark_favorite VALUES (100, 8)"); err != nil {
		t.Fatalf("second user favorite failed: %v", err)
	}
}

// Go 启动迁移与权威 bootstrap SQL 的收藏主键迁移逻辑必须保持一致。
func TestBookmarkFavoriteMigrationMatchesBootstrapSQL(t *testing.T) {
	bootstrap, err := os.ReadFile("../../../../sql/ptmj_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ReplaceAll(string(bootstrap), "\r\n", "\n"), bookmarkFavoriteKeyMigration) {
		t.Fatal("startup and bootstrap SQL bookmark migrations differ")
	}
}
