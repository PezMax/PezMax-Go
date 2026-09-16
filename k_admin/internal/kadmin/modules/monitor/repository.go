package monitor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS public.kadmin_monitor_settings (
		id smallint PRIMARY KEY,
		enabled boolean NOT NULL DEFAULT FALSE,
		updated_by bigint NOT NULL DEFAULT 0,
		updated_at timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT kadmin_monitor_settings_singleton_check CHECK (id = 1)
	)`,
	`INSERT INTO public.kadmin_monitor_settings (id, enabled, updated_by)
		VALUES (1, FALSE, 0) ON CONFLICT (id) DO NOTHING`,
}

type settingsStore interface {
	LoadEnabled() (bool, error)
	SaveEnabled(enabled bool, updatedBy int64) error
}

type repository struct {
	conn db.Connection
}

func EnsureSchema(conn db.Connection) error {
	if conn == nil {
		return errors.New("monitor database connection is required")
	}
	for _, statement := range schemaStatements {
		if _, err := conn.Exec(statement); err != nil {
			return fmt.Errorf("initialize monitor schema: %w", err)
		}
	}
	return nil
}

func (r *repository) LoadEnabled() (bool, error) {
	rows, err := r.conn.Query(`SELECT enabled FROM public.kadmin_monitor_settings WHERE id = 1`)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, errors.New("monitor setting is missing")
	}
	return monitorBool(rows[0]["enabled"]), nil
}

func (r *repository) SaveEnabled(enabled bool, updatedBy int64) error {
	_, err := r.conn.Exec(`INSERT INTO public.kadmin_monitor_settings
		(id, enabled, updated_by, updated_at) VALUES (1, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET enabled = EXCLUDED.enabled,
		updated_by = EXCLUDED.updated_by, updated_at = CURRENT_TIMESTAMP`, enabled, updatedBy)
	return err
}

func monitorBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case []byte:
		parsed, _ := strconv.ParseBool(string(typed))
		return parsed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	default:
		return false
	}
}
