package sqlite

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

func sqliteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func (d *Driver) sqlitePragmaForeignKeys(ctx context.Context) (int, error) {
	var v int
	err := d.ORM.NewRaw("PRAGMA foreign_keys").Scan(ctx, &v)
	return v, err
}

func (d *Driver) sqliteSetForeignKeys(ctx context.Context, on bool) error {
	q := "PRAGMA foreign_keys = OFF"
	if on {
		q = "PRAGMA foreign_keys = ON"
	}
	_, err := d.ORM.NewRaw(q).Exec(ctx)
	return err
}

func (d *Driver) sqliteCountTable(ctx context.Context, table string) (int, error) {
	var n int
	err := d.ORM.NewRaw("SELECT COUNT(*) FROM " + sqliteIdent(table)).Scan(ctx, &n)
	return n, err
}

func (d *Driver) sqliteIndexSQLOnTable(ctx context.Context, table string) ([]string, error) {
	var sqls []string
	err := d.ORM.NewRaw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL`, table).Scan(ctx, &sqls)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, sql := range sqls {
		sql = strings.TrimSpace(sql)
		if sql == "" {
			continue
		}
		up := strings.ToUpper(sql)
		if strings.Contains(up, "CREATE UNIQUE INDEX") || strings.Contains(up, "CREATE INDEX") {
			out = append(out, sql)
		}
	}
	return out, nil
}

func indexSQLIsIDOnly(sqlText, table string) bool {
	up := strings.ToUpper(strings.ReplaceAll(sqlText, " ", ""))
	want := strings.ToUpper(strings.ReplaceAll(fmt.Sprintf("ON %s(ID)", table), " ", ""))
	return strings.Contains(up, want) && !strings.Contains(strings.ToUpper(sqlText), ",")
}

// ReduceModelTableToShadow rebuilds a tenant table as id-only so common-model
// field values live only in the base project DB. Foreign-key pragma is restored
// on every exit. The source table is not dropped until the id copy verifies.
func (d *Driver) ReduceModelTableToShadow(ctx context.Context, modelName string) error {
	table := utility.PhysicalSQLTableName(modelName)
	if table == "" {
		return nil
	}
	tmp := "__shadow_" + strings.ReplaceAll(table, "`", "")

	fkOn := 1
	if v, err := d.sqlitePragmaForeignKeys(ctx); err == nil {
		fkOn = v
	}
	if err := d.sqliteSetForeignKeys(ctx, false); err != nil {
		return fmt.Errorf("reduce %s to shadow: disable foreign_keys: %w", table, err)
	}
	restored := false
	restoreFK := func() {
		if restored {
			return
		}
		restored = true
		_ = d.sqliteSetForeignKeys(ctx, fkOn != 0)
	}
	defer restoreFK()

	srcCount, err := d.sqliteCountTable(ctx, table)
	if err != nil {
		return fmt.Errorf("reduce %s to shadow: source table missing: %w", table, err)
	}

	idIndexes, _ := d.sqliteIndexSQLOnTable(ctx, table)

	if _, err := d.ORM.NewRaw("DROP TABLE IF EXISTS " + sqliteIdent(tmp)).Exec(ctx); err != nil {
		return fmt.Errorf("reduce %s to shadow: %w", table, err)
	}
	if _, err := d.ORM.NewRaw("CREATE TABLE " + sqliteIdent(tmp) + " (id VARCHAR(36) NOT NULL PRIMARY KEY)").Exec(ctx); err != nil {
		return fmt.Errorf("reduce %s to shadow: %w", table, err)
	}
	if _, err := d.ORM.NewRaw("INSERT OR IGNORE INTO " + sqliteIdent(tmp) + " (id) SELECT id FROM " + sqliteIdent(table)).Exec(ctx); err != nil {
		_, _ = d.ORM.NewRaw("DROP TABLE IF EXISTS " + sqliteIdent(tmp)).Exec(ctx)
		return fmt.Errorf("reduce %s to shadow: copy ids: %w", table, err)
	}
	tmpCount, err := d.sqliteCountTable(ctx, tmp)
	if err != nil {
		_, _ = d.ORM.NewRaw("DROP TABLE IF EXISTS " + sqliteIdent(tmp)).Exec(ctx)
		return fmt.Errorf("reduce %s to shadow: %w", table, err)
	}
	if tmpCount != srcCount {
		_, _ = d.ORM.NewRaw("DROP TABLE IF EXISTS " + sqliteIdent(tmp)).Exec(ctx)
		return fmt.Errorf("reduce %s to shadow: verification failed (source=%d shadow=%d); source table left intact", table, srcCount, tmpCount)
	}

	if _, err := d.ORM.NewRaw("DROP TABLE " + sqliteIdent(table)).Exec(ctx); err != nil {
		_, _ = d.ORM.NewRaw("DROP TABLE IF EXISTS " + sqliteIdent(tmp)).Exec(ctx)
		return fmt.Errorf("reduce %s to shadow: drop source: %w", table, err)
	}
	if _, err := d.ORM.NewRaw("ALTER TABLE " + sqliteIdent(tmp) + " RENAME TO " + sqliteIdent(table)).Exec(ctx); err != nil {
		return fmt.Errorf("reduce %s to shadow: rename failed after drop: %w", table, err)
	}
	for _, idxSQL := range idIndexes {
		if !indexSQLIsIDOnly(idxSQL, table) {
			continue
		}
		if _, err := d.ORM.NewRaw(idxSQL).Exec(ctx); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "already exists") {
				continue
			}
			return fmt.Errorf("reduce %s to shadow: recreate index: %w", table, err)
		}
	}
	restoreFK()
	return nil
}
