package rag

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

type schemaTestConnection struct {
	db.Connection
	vectorInstalled bool
	trgmInstalled   bool
	queryErr        error
	execErr         error
	queries         []string
	execs           []string
}

func (c *schemaTestConnection) Query(query string, _ ...interface{}) ([]map[string]interface{}, error) {
	c.queries = append(c.queries, query)
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	return []map[string]interface{}{{
		"vector_installed":  c.vectorInstalled,
		"pg_trgm_installed": c.trgmInstalled,
	}}, nil
}

func (c *schemaTestConnection) Exec(query string, _ ...interface{}) (sql.Result, error) {
	c.execs = append(c.execs, query)
	if c.execErr != nil {
		return nil, c.execErr
	}
	return driver.RowsAffected(1), nil
}

func TestEnsureSchemaSkipsMissingExtensionWhenOptional(t *testing.T) {
	connection := &schemaTestConnection{}
	applied, err := EnsureSchema(connection, false)
	if err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if applied || len(connection.execs) != 0 {
		t.Fatalf("schema applied without pgvector: applied=%v execs=%d", applied, len(connection.execs))
	}
}

func TestEnsureSchemaRequiresExtensionWhenEnabled(t *testing.T) {
	connection := &schemaTestConnection{}
	if applied, err := EnsureSchema(connection, true); err == nil || applied || !strings.Contains(err.Error(), "pgvector extension is not installed") {
		t.Fatalf("EnsureSchema = (%v, %v), want pgvector-required error", applied, err)
	}
	if len(connection.execs) != 0 {
		t.Fatalf("schema executed without pgvector: %d statements", len(connection.execs))
	}
}

func TestEnsureSchemaRunsIdempotentDDL(t *testing.T) {
	connection := &schemaTestConnection{vectorInstalled: true, trgmInstalled: true}
	for range 2 {
		applied, err := EnsureSchema(connection, true)
		if err != nil || !applied {
			t.Fatalf("EnsureSchema = (%v, %v)", applied, err)
		}
	}
	if len(connection.queries) != 2 || len(connection.execs) != 2 {
		t.Fatalf("query/exec counts = %d/%d, want 2/2", len(connection.queries), len(connection.execs))
	}
}

func TestEnsureSchemaCreatesMissingTrigram(t *testing.T) {
	connection := &schemaTestConnection{vectorInstalled: true}
	applied, err := EnsureSchema(connection, true)
	if err != nil || !applied {
		t.Fatalf("EnsureSchema = (%v, %v)", applied, err)
	}
	if len(connection.execs) != 2 {
		t.Fatalf("exec count = %d, want 2 (CREATE EXTENSION pg_trgm + schema)", len(connection.execs))
	}
	if !strings.Contains(connection.execs[0], "CREATE EXTENSION IF NOT EXISTS pg_trgm") {
		t.Fatalf("first exec = %q, want pg_trgm extension creation", connection.execs[0])
	}
	if connection.execs[1] != schemaSQL {
		t.Fatalf("second exec is not the RAG schema")
	}
}

func TestEnsureSchemaWrapsTrigramFailure(t *testing.T) {
	connection := &schemaTestConnection{vectorInstalled: true, execErr: errors.New("permission denied")}
	applied, err := EnsureSchema(connection, true)
	if err == nil || applied || !strings.Contains(err.Error(), "enable pg_trgm extension") {
		t.Fatalf("EnsureSchema = (%v, %v), want pg_trgm enable error", applied, err)
	}
	if len(connection.execs) != 1 {
		t.Fatalf("exec count = %d, want 1 (schema must not run)", len(connection.execs))
	}
}

func TestEnsureSchemaWrapsDatabaseFailures(t *testing.T) {
	queryFailure := &schemaTestConnection{queryErr: errors.New("db unavailable")}
	if _, err := EnsureSchema(queryFailure, false); err == nil || !strings.Contains(err.Error(), "check pgvector extension") {
		t.Fatalf("query error = %v, want wrapped extension check error", err)
	}

	execFailure := &schemaTestConnection{vectorInstalled: true, trgmInstalled: true, execErr: errors.New("DDL rejected")}
	if _, err := EnsureSchema(execFailure, true); err == nil || !strings.Contains(err.Error(), "initialize RAG schema") {
		t.Fatalf("exec error = %v, want wrapped schema error", err)
	}
}

func TestEnsureSchemaRequiresConnection(t *testing.T) {
	if _, err := EnsureSchema(nil, false); err == nil || !strings.Contains(err.Error(), "connection is required") {
		t.Fatalf("nil connection error = %v", err)
	}
}
