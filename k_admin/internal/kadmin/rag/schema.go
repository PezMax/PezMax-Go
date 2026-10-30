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
// has pgvector available. pg_trgm backs the lexical fallback (trigram indexes
// on chunk text and document titles); it is created here when missing so
// existing data volumes that predate the extension need no manual migration.
// If RAG is disabled and pgvector is absent, the schema is skipped so legacy
// PostgreSQL deployments keep starting unchanged.
func EnsureSchema(conn db.Connection, required bool) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("RAG database connection is required")
	}
	rows, err := conn.Query(`SELECT
		EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') AS vector_installed,
		EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') AS pg_trgm_installed`)
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
	trigramInstalled := len(rows) > 0 && strings.EqualFold(fmt.Sprint(rows[0]["pg_trgm_installed"]), "true")
	if !trigramInstalled {
		if _, err := conn.Exec(`CREATE EXTENSION IF NOT EXISTS pg_trgm`); err != nil {
			return false, fmt.Errorf("enable pg_trgm extension: %w", err)
		}
	}
	if _, err := conn.Exec(schemaSQL); err != nil {
		return false, fmt.Errorf("initialize RAG schema: %w", err)
	}
	return true, nil
}
