package db

import (
	"database/sql"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/config"
	"github.com/GoAdminGroup/go-admin/modules/db/dialect"
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/mssql"
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/postgres"
	"github.com/magiconair/properties/assert"
)

func testSQLWhereIn(t *testing.T, conn Connection) {

	item, _ := WithDriver(conn).Table("goadmin_users").WhereIn("id", []interface{}{"1", "2"}).First()
	assert.Equal(t, len(item), 2)

	_, _ = WithDriver(conn).WithTransaction(func(tx *sql.Tx) (e error, i map[string]interface{}) {
		item, _ := WithDriver(conn).WithTx(tx).Table("goadmin_users").WhereIn("id", []interface{}{"1", "2"}).All()
		assert.Equal(t, len(item), 2)
		return nil, nil
	})
}

func testSQLCount(t *testing.T, conn Connection) {
	count, _ := WithDriver(conn).Table("goadmin_users").Count()
	assert.Equal(t, count, int64(2))
}

func TestPostgresInsertUsesReturningIDWhenTableHasID(t *testing.T) {
	conn := &postgresInsertFakeConn{columns: []map[string]interface{}{{"column_name": "id"}}}

	id, err := WithDriver(conn).Table("custom_log_table").Insert(dialect.H{"name": "demo"})
	if err != nil {
		t.Fatalf("insert returned error: %v", err)
	}
	if id != 42 {
		t.Fatalf("unexpected id: %d", id)
	}
	if !conn.queryWithCalled || conn.execWithCalled {
		t.Fatalf("expected RETURNING id query path, queryWith=%v execWith=%v", conn.queryWithCalled, conn.execWithCalled)
	}
}

func TestPostgresInsertWithoutIDDoesNotCallLastInsertID(t *testing.T) {
	conn := &postgresInsertFakeConn{columns: []map[string]interface{}{{"column_name": "role_id"}}}

	id, err := WithDriver(conn).Table("goadmin_role_users").Insert(dialect.H{"role_id": 1, "user_id": 2})
	if err != nil {
		t.Fatalf("insert returned error: %v", err)
	}
	if id != 0 {
		t.Fatalf("tables without id should return 0, got %d", id)
	}
	if !conn.execWithCalled || conn.lastInsertIDCalled {
		t.Fatalf("expected exec path without LastInsertId, execWith=%v lastInsertID=%v", conn.execWithCalled, conn.lastInsertIDCalled)
	}
}

func TestPostgresExecDoesNotCallLastInsertID(t *testing.T) {
	conn := &postgresInsertFakeConn{}
	stmt := WithDriver(conn).Table("custom_table")
	stmt.Values = dialect.H{"name": "demo"}

	affectedRows, err := stmt.Exec()
	if err != nil {
		t.Fatalf("exec returned error: %v", err)
	}
	if affectedRows != 1 {
		t.Fatalf("expected affected rows, got %d", affectedRows)
	}
	if !conn.execWithCalled || conn.lastInsertIDCalled {
		t.Fatalf("expected exec path without LastInsertId, execWith=%v lastInsertID=%v", conn.execWithCalled, conn.lastInsertIDCalled)
	}
}

type postgresInsertFakeConn struct {
	columns            []map[string]interface{}
	queryWithCalled    bool
	execWithCalled     bool
	lastInsertIDCalled bool
}

func (c *postgresInsertFakeConn) Query(string, ...interface{}) ([]map[string]interface{}, error) {
	return nil, nil
}

func (c *postgresInsertFakeConn) Exec(string, ...interface{}) (sql.Result, error) {
	c.execWithCalled = true
	return postgresInsertFakeResult{conn: c}, nil
}

func (c *postgresInsertFakeConn) QueryWithConnection(string, string, ...interface{}) ([]map[string]interface{}, error) {
	return c.columns, nil
}

func (c *postgresInsertFakeConn) QueryWithTx(*sql.Tx, string, ...interface{}) ([]map[string]interface{}, error) {
	return nil, nil
}

func (c *postgresInsertFakeConn) QueryWith(*sql.Tx, string, string, ...interface{}) ([]map[string]interface{}, error) {
	c.queryWithCalled = true
	return []map[string]interface{}{{"id": int64(42)}}, nil
}

func (c *postgresInsertFakeConn) ExecWithConnection(string, string, ...interface{}) (sql.Result, error) {
	c.execWithCalled = true
	return postgresInsertFakeResult{conn: c}, nil
}

func (c *postgresInsertFakeConn) ExecWithTx(*sql.Tx, string, ...interface{}) (sql.Result, error) {
	c.execWithCalled = true
	return postgresInsertFakeResult{conn: c}, nil
}

func (c *postgresInsertFakeConn) ExecWith(*sql.Tx, string, string, ...interface{}) (sql.Result, error) {
	c.execWithCalled = true
	return postgresInsertFakeResult{conn: c}, nil
}

func (c *postgresInsertFakeConn) BeginTxWithReadUncommitted() *sql.Tx { return nil }
func (c *postgresInsertFakeConn) BeginTxWithReadCommitted() *sql.Tx   { return nil }
func (c *postgresInsertFakeConn) BeginTxWithRepeatableRead() *sql.Tx  { return nil }
func (c *postgresInsertFakeConn) BeginTx() *sql.Tx                    { return nil }
func (c *postgresInsertFakeConn) BeginTxWithLevel(sql.IsolationLevel) *sql.Tx {
	return nil
}
func (c *postgresInsertFakeConn) BeginTxWithReadUncommittedAndConnection(string) *sql.Tx {
	return nil
}
func (c *postgresInsertFakeConn) BeginTxWithReadCommittedAndConnection(string) *sql.Tx {
	return nil
}
func (c *postgresInsertFakeConn) BeginTxWithRepeatableReadAndConnection(string) *sql.Tx {
	return nil
}
func (c *postgresInsertFakeConn) BeginTxAndConnection(string) *sql.Tx { return nil }
func (c *postgresInsertFakeConn) BeginTxWithLevelAndConnection(string, sql.IsolationLevel) *sql.Tx {
	return nil
}
func (c *postgresInsertFakeConn) InitDB(map[string]config.Database) Connection { return c }
func (c *postgresInsertFakeConn) Name() string                                 { return DriverPostgresql }
func (c *postgresInsertFakeConn) Close() []error                               { return nil }
func (c *postgresInsertFakeConn) GetDelimiter() string                         { return `"` }
func (c *postgresInsertFakeConn) GetDelimiter2() string                        { return `"` }
func (c *postgresInsertFakeConn) GetDelimiters() []string                      { return []string{`"`, `"`} }
func (c *postgresInsertFakeConn) GetDB(string) *sql.DB                         { return nil }
func (c *postgresInsertFakeConn) GetConfig(string) config.Database             { return config.Database{} }
func (c *postgresInsertFakeConn) CreateDB(string, ...interface{}) error        { return nil }

type postgresInsertFakeResult struct {
	conn *postgresInsertFakeConn
}

func (r postgresInsertFakeResult) LastInsertId() (int64, error) {
	r.conn.lastInsertIDCalled = true
	return 0, nil
}

func (postgresInsertFakeResult) RowsAffected() (int64, error) {
	return 1, nil
}

// TODO
func testSQLSelect(t *testing.T, conn Connection) {}

// TODO
func testSQLOrderBy(t *testing.T, conn Connection) {}

// TODO
func testSQLGroupBy(t *testing.T, conn Connection) {}

// TODO
func testSQLSkip(t *testing.T, conn Connection) {}

// TODO
func testSQLTake(t *testing.T, conn Connection) {}

// TODO
func testSQLWhere(t *testing.T, conn Connection) {}

// TODO
func testSQLWhereNotIn(t *testing.T, conn Connection) {}

// TODO
func testSQLFind(t *testing.T, conn Connection) {}

// TODO
func testSQLSum(t *testing.T, conn Connection) {}

// TODO
func testSQLMax(t *testing.T, conn Connection) {}

// TODO
func testSQLMin(t *testing.T, conn Connection) {}

// TODO
func testSQLAvg(t *testing.T, conn Connection) {}

// TODO
func testSQLWhereRaw(t *testing.T, conn Connection) {}

// TODO
func testSQLUpdateRaw(t *testing.T, conn Connection) {}

// TODO
func testSQLLeftJoin(t *testing.T, conn Connection) {}

// TODO
func testSQLWithTransaction(t *testing.T, conn Connection) {}

// TODO
func testSQLWithTransactionByLevel(t *testing.T, conn Connection) {}

// TODO
func testSQLFirst(t *testing.T, conn Connection) {}

// TODO
func testSQLAll(t *testing.T, conn Connection) {}

// TODO
func testSQLShowColumns(t *testing.T, conn Connection) {}

// TODO
func testSQLShowTables(t *testing.T, conn Connection) {}

// TODO
func testSQLUpdate(t *testing.T, conn Connection) {}

// TODO
func testSQLDelete(t *testing.T, conn Connection) {}

// TODO
func testSQLExec(t *testing.T, conn Connection) {}

// TODO
func testSQLInsert(t *testing.T, conn Connection) {}

// TODO
func testSQLWrap(t *testing.T, conn Connection) {}
