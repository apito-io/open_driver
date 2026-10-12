package mysql

import (
	"context"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/uptrace/bun"
)

// createModelTableExec runs CREATE TABLE DDL on any bun executor (DB or transaction).
func (d *Driver) createModelTableExec(ctx context.Context, db bun.IDB, model *models.ModelType, ifNotExists bool) error {
	tableName := utility.PhysicalSQLTableName(model.Name)
	ifClause := " "
	if ifNotExists {
		ifClause = " IF NOT EXISTS "
	}
	var query string
	switch d.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		t := strings.ReplaceAll(tableName, `"`, `""`)
		query = fmt.Sprintf("CREATE TABLE%s%s ( id VARCHAR(36) NOT NULL PRIMARY KEY );", ifClause, QuotePGIdent(t))
	default:
		t := strings.ReplaceAll(tableName, "`", "``")
		query = fmt.Sprintf("CREATE TABLE%s`%s`( id VARCHAR(36) NOT NULL PRIMARY KEY );", ifClause, t)
	}
	_, err := db.NewRaw(query).Exec(ctx)
	return err
}

// CreateModelTable creates a model table with only an id primary key column.
// Use ifNotExists to guard against duplicate table errors during provisioning.
func (d *Driver) CreateModelTable(ctx context.Context, model *models.ModelType, ifNotExists bool) error {
	return d.createModelTableExec(ctx, d.ORM, model, ifNotExists)
}

// ReduceModelTableToShadow rebuilds a tenant table as id-only.
func (d *Driver) ReduceModelTableToShadow(ctx context.Context, modelName string) error {
	table := utility.PhysicalSQLTableName(modelName)
	if table == "" {
		return nil
	}
	tmp := "__shadow_" + strings.ReplaceAll(table, "`", "")
	stmts := []string{
		"SET FOREIGN_KEY_CHECKS=0",
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (id VARCHAR(36) NOT NULL PRIMARY KEY)", tmp),
		fmt.Sprintf("INSERT IGNORE INTO `%s` (id) SELECT id FROM `%s`", tmp, table),
		fmt.Sprintf("DROP TABLE `%s`", table),
		fmt.Sprintf("RENAME TABLE `%s` TO `%s`", tmp, table),
		"SET FOREIGN_KEY_CHECKS=1",
	}
	for _, q := range stmts {
		if _, err := d.ORM.NewRaw(q).Exec(ctx); err != nil {
			return fmt.Errorf("reduce %s to shadow: %w", table, err)
		}
	}
	return nil
}
