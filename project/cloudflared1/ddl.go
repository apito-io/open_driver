package cloudflared1

import (
	"context"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"gitlab.com/apito.io/open_driver/cfd1"
	"gitlab.com/apito.io/open_driver/project/sqlcommon"
	"gitlab.com/apito.io/open_driver/project/sqlquery"
)

func sharedSaaSTenantColumn(param *models.CommonSystemParams) bool {
	if param == nil {
		return false
	}
	if param.Ext != nil {
		if v, ok := param.Ext["saas_shared_tenant"].(bool); ok {
			return v
		}
	}
	if param.Model != nil {
		for _, f := range param.Model.Fields {
			if f != nil && f.Identifier == "tenant_id" {
				return true
			}
		}
	}
	return false
}

const engineName = cfd1.EngineName

func (d *Driver) execDDL(ctx context.Context, stmts ...string) error {
	for _, q := range stmts {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if _, err := d.DB.Exec(ctx, q); err != nil {
			if sqlcommon.IsAlreadyExistsErr(err) {
				continue
			}
			return fmt.Errorf("%w\n%s", err, q)
		}
	}
	return nil
}

func (d *Driver) InitProjectBase(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			id TEXT PRIMARY KEY,
			doc_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			created_by TEXT NOT NULL,
			updated_by TEXT,
			status TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_meta_doc_id ON meta(doc_id)`,
		`CREATE INDEX IF NOT EXISTS idx_meta_doc_created ON meta(doc_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			file_type TEXT NOT NULL,
			file_name TEXT NOT NULL,
			created_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_files_type_created ON files(file_type, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS _apito_row_counts (
			table_name TEXT NOT NULL,
			tenant_id TEXT NOT NULL DEFAULT '',
			row_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (table_name, tenant_id)
		)`,
	}
	if err := d.execDDL(ctx, stmts...); err != nil {
		return err
	}
	for _, idx := range indexes {
		if err := d.execDDL(ctx, idx); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) CreateTableOrCollection(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	if param == nil || param.Model == nil {
		return fmt.Errorf("cloudflared1: model required")
	}
	table := utility.PhysicalSQLTableName(param.Model.Name)
	cols := []string{"id TEXT PRIMARY KEY"}
	if sharedSaaSTenantColumn(param) {
		cols = append(cols, "tenant_id TEXT")
	}
	ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (%s)", table, strings.Join(cols, ", "))
	if err := d.execDDL(ctx, ddl); err != nil {
		return err
	}
	stmts, err := sqlquery.ModelUserFieldColumnsDDL(engineName, param.Model)
	if err != nil {
		return err
	}
	if err := d.execDDL(ctx, stmts...); err != nil {
		return err
	}
	if sharedSaaSTenantColumn(param) {
		idx := sqlquery.CreateIndexDDL(table, fmt.Sprintf("idx_%s_tenant_id", table), "tenant_id")
		if err := d.execDDL(ctx, idx); err != nil {
			return err
		}
		uidx := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS `idx_%s_id_tenant` ON `%s` (id, tenant_id)", table, table)
		if err := d.execDDL(ctx, uidx); err != nil {
			return err
		}
	}
	nested := sqlquery.NestedIndexDDLStatements(param.Model, sharedSaaSTenantColumn(param))
	if err := d.execDDL(ctx, nested...); err != nil {
		return err
	}
	for _, f := range param.Model.Fields {
		if f != nil && f.EnableIndexing {
			col := sqlquery.PhysicalSQLColumnForSystemRelationField(f.Identifier, param.Model)
			cols := sqlquery.TenantFirstIndexColumns(sharedSaaSTenantColumn(param), col)
			idx := sqlquery.CreateIndexDDL(table, fmt.Sprintf("idx_%s_%s", table, col), cols...)
			if err := d.execDDL(ctx, idx); err != nil {
				return err
			}
		}
	}
	if err := d.installRowCountTriggersForModel(ctx, param.Model, sharedSaaSTenantColumn(param)); err != nil {
		return err
	}
	_, _ = d.DB.Exec(ctx, "ANALYZE")
	return nil
}

func (d *Driver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parentField string) (*models.ModelType, error) {
	if param == nil || param.Model == nil || param.FieldInfo == nil {
		return nil, fmt.Errorf("cloudflared1: invalid add field params")
	}
	if parentField != "" {
		return param.Model, nil
	}
	table := utility.PhysicalSQLTableName(param.Model.Name)
	stmts, err := sqlquery.AlterTableAddFieldSQL(engineName, table, param.FieldInfo)
	if err != nil {
		return nil, err
	}
	if err := d.execDDL(ctx, stmts...); err != nil {
		return nil, err
	}
	if !isUpdate && param.FieldInfo.EnableIndexing {
		col := sqlquery.PhysicalSQLColumnForSystemRelationField(param.FieldInfo.Identifier, param.Model)
		cols := sqlquery.TenantFirstIndexColumns(sharedSaaSTenantColumn(param), col)
		idx := sqlquery.CreateIndexDDL(table, fmt.Sprintf("idx_%s_%s", table, col), cols...)
		if err := d.execDDL(ctx, idx); err != nil {
			return nil, err
		}
	}
	nested := sqlquery.NestedIndexDDLStatements(param.Model, sharedSaaSTenantColumn(param))
	if err := d.execDDL(ctx, nested...); err != nil {
		return nil, err
	}
	_, _ = d.DB.Exec(ctx, "ANALYZE")
	return param.Model, nil
}

func (d *Driver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parentField string) error {
	if param == nil || param.Model == nil {
		return fmt.Errorf("cloudflared1: model required")
	}
	_ = parentField
	table := utility.PhysicalSQLTableName(param.Model.Name)
	cols := sqlquery.TenantFirstIndexColumns(sharedSaaSTenantColumn(param), fieldName)
	idx := sqlquery.CreateIndexDDL(table, fmt.Sprintf("idx_%s_%s", table, fieldName), cols...)
	return d.execDDL(ctx, idx)
}
