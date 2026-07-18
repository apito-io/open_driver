package sqlcommon

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// PersistProjectFunctionsTx deletes apito_functions rows for projectID whose name is not in
// schemaFunctions, then inserts or updates each function row. nil means no-op; empty slice deletes all.
func PersistProjectFunctionsTx(ctx context.Context, tx bun.Tx, projectID string, schemaFunctions []*models.ApitoFunction) error {
	if projectID == "" || schemaFunctions == nil {
		return nil
	}

	var keepNames []string
	for _, f := range schemaFunctions {
		if f == nil || f.Name == "" {
			continue
		}
		keepNames = append(keepNames, f.Name)
	}

	var err error
	if len(keepNames) == 0 {
		_, err = tx.NewDelete().
			Model((*models.ApitoFunction)(nil)).
			Where("project_id = ?", projectID).
			Exec(ctx)
	} else {
		_, err = tx.NewDelete().
			Model((*models.ApitoFunction)(nil)).
			Where("project_id = ?", projectID).
			Where("name NOT IN (?)", bun.In(keepNames)).
			Exec(ctx)
	}
	if err != nil {
		return err
	}

	for _, f := range schemaFunctions {
		if f == nil || f.Name == "" {
			continue
		}
		row := *f
		row.ProjectID = projectID
		if row.TriggerType == "" {
			row.TriggerType = models.FunctionTriggerCallable
		}
		_, err := tx.NewInsert().Model(&row).Exec(ctx)
		if err != nil {
			if !IsSQLUniqueViolation(err) {
				return err
			}
			if _, err = tx.NewUpdate().
				Model(&row).
				Where("project_id = ? AND name = ?", row.ProjectID, row.Name).
				ExcludeColumn("project_id", "name").
				Exec(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureApitoFunctionsCompositePK migrates legacy apito_functions tables that only had
// project_id as PK (or missing name column) to the composite (project_id, name) key used by ModelType.
// Safe to call on every RunMigration (idempotent).
func EnsureApitoFunctionsCompositePK(ctx context.Context, orm *bun.DB) error {
	if orm == nil {
		return nil
	}
	exists, err := tableExists(ctx, orm, "apito_functions")
	if err != nil || !exists {
		return err
	}
	ok, err := hasCompositePK(ctx, orm, "apito_functions", []string{"project_id", "name"})
	if err != nil {
		return err
	}
	if ok {
		return ensureApitoFunctionsExtraColumns(ctx, orm)
	}

	// Rebuild table with correct schema. Existing rows that lack a usable name are dropped
	// (legacy broken PK could not store multiple functions correctly anyway).
	tmp := "apito_functions_migrate_tmp"
	_, _ = orm.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tmp))

	if _, err := orm.NewCreateTable().Model((*models.ApitoFunction)(nil)).ModelTableExpr(tmp).Exec(ctx); err != nil {
		// Fallback: create via raw SQL for SQLite-compatible DDL.
		if _, err2 := orm.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS apito_functions_migrate_tmp (
  project_id TEXT NOT NULL,
  name TEXT NOT NULL,
  id TEXT,
  description TEXT,
  graphql_schema_type TEXT,
  env_vars JSON,
  function_path TEXT,
  request JSON,
  response JSON,
  function_provider_id TEXT,
  provider_exported_variable TEXT,
  function_exported_variable TEXT,
  runtime_config JSON,
  trigger_type TEXT,
  active_revision_id TEXT,
  capabilities JSON,
  source TEXT,
  updated_at TEXT,
  created_at TEXT,
  language TEXT,
  binary_url TEXT,
  rest_api_secret_url_key TEXT,
  PRIMARY KEY (project_id, name)
)`); err2 != nil {
			return fmt.Errorf("apito_functions migrate create tmp: %w", err)
		}
	}

	// Copy rows that have a non-empty name. Ignore duplicates (keep first).
	_, _ = orm.ExecContext(ctx, `
INSERT OR IGNORE INTO apito_functions_migrate_tmp (
  project_id, name, id, description, graphql_schema_type, env_vars, function_path,
  request, response, function_provider_id, provider_exported_variable, function_exported_variable,
  runtime_config, updated_at, created_at, language, binary_url, rest_api_secret_url_key
)
SELECT project_id, name, id, description, graphql_schema_type, env_vars, function_path,
  request, response, function_provider_id, provider_exported_variable, function_exported_variable,
  runtime_config, updated_at, created_at, language, binary_url, rest_api_secret_url_key
FROM apito_functions
WHERE name IS NOT NULL AND TRIM(name) != ''
`)

	if _, err := orm.ExecContext(ctx, `DROP TABLE apito_functions`); err != nil {
		return fmt.Errorf("apito_functions migrate drop old: %w", err)
	}
	if _, err := orm.ExecContext(ctx, `ALTER TABLE apito_functions_migrate_tmp RENAME TO apito_functions`); err != nil {
		return fmt.Errorf("apito_functions migrate rename: %w", err)
	}
	return ensureApitoFunctionsExtraColumns(ctx, orm)
}

func ensureApitoFunctionsExtraColumns(ctx context.Context, orm *bun.DB) error {
	cols := []string{
		"trigger_type",
		"active_revision_id",
		"capabilities",
		"source",
		"language",
		"binary_url",
	}
	for _, col := range cols {
		if err := addColumnIfMissing(ctx, orm, "apito_functions", col, "TEXT"); err != nil {
			return err
		}
	}
	return nil
}

func tableExists(ctx context.Context, orm *bun.DB, table string) (bool, error) {
	var n int
	// Works on SQLite; for PG/MySQL use information_schema via dialect-agnostic try-select.
	err := orm.NewSelect().ColumnExpr("1").TableExpr(table).Limit(1).Scan(ctx, &n)
	if err == nil {
		return true, nil
	}
	if err == sql.ErrNoRows {
		return true, nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "doesn't exist") {
		return false, nil
	}
	// Table exists but empty / other scan quirk
	if strings.Contains(msg, "no rows") {
		return true, nil
	}
	return true, nil
}

func hasCompositePK(ctx context.Context, orm *bun.DB, table string, cols []string) (bool, error) {
	_ = cols
	var createSQL string
	err := orm.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&createSQL)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		// Non-SQLite engines: CreateTable IfNotExists on fresh installs is enough;
		// additive columns are ensured separately.
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no such table") ||
			strings.Contains(msg, "sqlite_master") ||
			strings.Contains(msg, "does not exist") {
			return true, nil
		}
		return true, nil
	}
	lower := strings.ToLower(createSQL)
	hasNameInPK := strings.Contains(lower, "primary key") &&
		strings.Contains(lower, "project_id") &&
		strings.Contains(lower, "name")
	onlyProjectPK := strings.Contains(lower, `primary key ("project_id")`) ||
		strings.Contains(lower, `primary key (project_id)`)
	return hasNameInPK && !onlyProjectPK, nil
}

func addColumnIfMissing(ctx context.Context, orm *bun.DB, table, column, typ string) error {
	// Try SELECT — if it fails with unknown column, ALTER.
	var probe interface{}
	err := orm.NewSelect().ColumnExpr(column).TableExpr(table).Limit(1).Scan(ctx, &probe)
	if err == nil || err == sql.ErrNoRows {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "no such column") &&
		!strings.Contains(msg, "unknown column") &&
		!strings.Contains(msg, "does not exist") {
		// Column likely exists or table empty with different error
		if strings.Contains(msg, "no rows in result") {
			return nil
		}
	}
	_, alterErr := orm.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, typ))
	if alterErr != nil {
		amsg := strings.ToLower(alterErr.Error())
		if strings.Contains(amsg, "duplicate") || strings.Contains(amsg, "already exists") {
			return nil
		}
		// Ignore if column already present under another probe path
		if strings.Contains(amsg, "no such table") {
			return nil
		}
	}
	return nil
}
