package rag

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

//go:embed rag_schema.sql
var schemaSQL string

// EnsureSchema creates the RAG retrieval tables and indexes once the database
// has pgvector available. If RAG is disabled and the extension is absent, it
// skips the schema so legacy PostgreSQL deployments keep starting unchanged.
func EnsureSchema(conn db.Connection, required bool) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("RAG database connection is required")
	}
	rows, err := conn.Query(`SELECT EXISTS (
		SELECT 1 FROM pg_extension WHERE extname = 'vector'
	) AS vector_installed`)
	if err != nil {
		return false, fmt.Errorf("check pgvector extension: %w", err)
	}
	installed := len(rows) > 0 && strings.EqualFold(fmt.Sprint(rows[0]["vector_installed"]), "true")
	if !installed {
		if required {
			return false, fmt.Errorf("pgvector extension is not installed; upgrade PostgreSQL to pgvector/pgvector:pg16 and run CREATE EXTENSION vector")
		}
		return false, nil
	}
	if _, err := conn.Exec(schemaSQL); err != nil {
		return false, fmt.Errorf("initialize RAG schema: %w", err)
	}
	return true, nil
}
