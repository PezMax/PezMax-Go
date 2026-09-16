package codegen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/GoAdminGroup/go-admin/modules/db"
)

var errConfigNotFound = fmt.Errorf("codegen table config not found")

// repository persists import configurations in kadmin_codegen_tables.
type repository struct {
	conn db.Connection
}

func newRepository(conn db.Connection) *repository {
	return &repository{conn: conn}
}

func (r *repository) listConfigs(keyword string) ([]TableConfig, error) {
	query := `SELECT id, table_name, module_name, class_name, business_name, route_prefix, columns, generated, created_at, updated_at
		FROM public.kadmin_codegen_tables`
	args := []interface{}{}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query += ` WHERE table_name ILIKE ? OR business_name ILIKE ?`
		pattern := "%" + keyword + "%"
		args = append(args, pattern, pattern)
	}
	query += ` ORDER BY id DESC`
	rows, err := r.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	configs := make([]TableConfig, 0, len(rows))
	for _, row := range rows {
		configs = append(configs, mapTableConfig(row))
	}
	return configs, nil
}

func (r *repository) getConfig(id int64) (TableConfig, bool, error) {
	rows, err := r.conn.Query(`SELECT id, table_name, module_name, class_name, business_name, route_prefix, columns, generated, created_at, updated_at
		FROM public.kadmin_codegen_tables WHERE id = ?`, id)
	if err != nil {
		return TableConfig{}, false, err
	}
	if len(rows) == 0 {
		return TableConfig{}, false, nil
	}
	return mapTableConfig(rows[0]), true, nil
}

func (r *repository) findConfigByTable(table string) (TableConfig, bool, error) {
	rows, err := r.conn.Query(`SELECT id, table_name, module_name, class_name, business_name, route_prefix, columns, generated, created_at, updated_at
		FROM public.kadmin_codegen_tables WHERE table_name = ?`, table)
	if err != nil {
		return TableConfig{}, false, err
	}
	if len(rows) == 0 {
		return TableConfig{}, false, nil
	}
	return mapTableConfig(rows[0]), true, nil
}

func (r *repository) createConfig(config TableConfig) (TableConfig, error) {
	columnsJSON, err := json.Marshal(config.Columns)
	if err != nil {
		return TableConfig{}, err
	}
	rows, err := r.conn.Query(`INSERT INTO public.kadmin_codegen_tables
		(table_name, module_name, class_name, business_name, route_prefix, columns, generated, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?::jsonb, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id`, config.TableName, config.ModuleName, config.ClassName, config.BusinessName, config.RoutePrefix, string(columnsJSON))
	if err != nil {
		return TableConfig{}, err
	}
	config.ID = intOf(rows[0]["id"])
	return config, nil
}

func (r *repository) updateConfig(id int64, config TableConfig) (TableConfig, error) {
	columnsJSON, err := json.Marshal(config.Columns)
	if err != nil {
		return TableConfig{}, err
	}
	_, err = r.conn.Exec(`UPDATE public.kadmin_codegen_tables
		SET module_name = ?, class_name = ?, business_name = ?, route_prefix = ?, columns = ?::jsonb, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		config.ModuleName, config.ClassName, config.BusinessName, config.RoutePrefix, string(columnsJSON), id)
	if err != nil {
		return TableConfig{}, err
	}
	updated, found, err := r.getConfig(id)
	if err != nil || !found {
		return TableConfig{}, errConfigNotFound
	}
	return updated, nil
}

func (r *repository) markGenerated(id int64) error {
	_, err := r.conn.Exec(`UPDATE public.kadmin_codegen_tables SET generated = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (r *repository) deleteConfig(id int64) error {
	_, err := r.conn.Exec(`DELETE FROM public.kadmin_codegen_tables WHERE id = ?`, id)
	return err
}

// generatedModules returns module names of every config that was generated,
// used to rebuild the shared generated registry file.
func (r *repository) generatedModules() ([]TableConfig, error) {
	rows, err := r.conn.Query(`SELECT id, table_name, module_name, class_name, business_name, route_prefix, columns, generated, created_at, updated_at
		FROM public.kadmin_codegen_tables WHERE generated = TRUE ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	configs := make([]TableConfig, 0, len(rows))
	for _, row := range rows {
		configs = append(configs, mapTableConfig(row))
	}
	return configs, nil
}

func mapTableConfig(row map[string]interface{}) TableConfig {
	config := TableConfig{
		ID:           intOf(row["id"]),
		TableName:    textOf(row["table_name"]),
		ModuleName:   textOf(row["module_name"]),
		ClassName:    textOf(row["class_name"]),
		BusinessName: textOf(row["business_name"]),
		RoutePrefix:  textOf(row["route_prefix"]),
		Generated:    boolOf(row["generated"]),
		CreatedAt:    timeText(row["created_at"]),
		UpdatedAt:    timeText(row["updated_at"]),
	}
	_ = json.Unmarshal([]byte(textOf(row["columns"])), &config.Columns)
	if config.Columns == nil {
		config.Columns = []ColumnConfig{}
	}
	return config
}
