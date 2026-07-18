package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"gitlab.com/apito.io/open_driver/system/sqlcommon"
)

func TestApitoFunctionsPersistAndHydrate(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	sqlcommon.RegisterSystemSQLSchemaModels(db)
	ctx := context.Background()
	for _, m := range sqlcommon.SystemSQLSchemaModels() {
		_, err := db.NewCreateTable().IfNotExists().Model(m).Exec(ctx)
		require.NoError(t, err, "%T", m)
	}
	require.NoError(t, sqlcommon.EnsureApitoFunctionsCompositePK(ctx, db))

	var createSQL string
	err = sqldb.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='apito_functions'`).Scan(&createSQL)
	require.NoError(t, err)
	require.Contains(t, createSQL, "project_id")
	require.Contains(t, createSQL, "name")

	projectID := "proj-fn-1"
	_, err = db.NewInsert().Model(&models.Project{
		ID:   projectID,
		Name: "fn-test",
	}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewInsert().Model(&models.ProjectSchema{ProjectID: projectID}).Exec(ctx)
	require.NoError(t, err)

	fns := []*models.ApitoFunction{
		{
			ProjectID: projectID,
			Name:      "closeOrder",
			ID:        utility.NewID(),
			RuntimeConfig: &models.ApitoFunctionRuntimeConfig{
				Runtime: models.FunctionRuntimeDeno,
				TimeOut: 15,
			},
			Request:  &models.ApitoFunctionRequestResponseType{Model: "JSON"},
			Response: &models.ApitoFunctionRequestResponseType{Model: "JSON"},
			Source:   "export default async (req) => req",
			TriggerType: models.FunctionTriggerCallable,
		},
		{
			ProjectID: projectID,
			Name:      "closeAllOrders",
			ID:        utility.NewID(),
			RuntimeConfig: &models.ApitoFunctionRuntimeConfig{
				Runtime: models.FunctionRuntimeDeno,
			},
			Request:  &models.ApitoFunctionRequestResponseType{Model: "JSON"},
			Response: &models.ApitoFunctionRequestResponseType{Model: "JSON"},
			TriggerType: models.FunctionTriggerCallable,
		},
	}

	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return sqlcommon.PersistProjectFunctionsTx(ctx, tx, projectID, fns)
	})
	require.NoError(t, err)

	var rows []*models.ApitoFunction
	err = db.NewSelect().Model(&rows).Where("project_id = ?", projectID).Order("name ASC").Scan(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, "closeAllOrders", rows[0].Name)
	require.Equal(t, "closeOrder", rows[1].Name)
	require.Equal(t, models.FunctionRuntimeDeno, rows[1].RuntimeConfig.Runtime)

	// Update one, delete the other via sync
	updated := []*models.ApitoFunction{fns[0]}
	updated[0].Description = "updated"
	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return sqlcommon.PersistProjectFunctionsTx(ctx, tx, projectID, updated)
	})
	require.NoError(t, err)

	var count int
	err = db.NewSelect().ColumnExpr("count(*)").Model((*models.ApitoFunction)(nil)).Where("project_id = ?", projectID).Scan(ctx, &count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	var one models.ApitoFunction
	err = db.NewSelect().Model(&one).Where("project_id = ? AND name = ?", projectID, "closeOrder").Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, "updated", one.Description)
}

func TestApitoFunctionsCompositePKSchema(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	sqlcommon.RegisterSystemSQLSchemaModels(db)
	ctx := context.Background()
	// Legacy broken table: only project_id PK
	_, err = db.ExecContext(ctx, `CREATE TABLE apito_functions (project_id TEXT PRIMARY KEY, name TEXT, id TEXT)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO apito_functions (project_id, name, id) VALUES ('p1', 'fnA', '1')`)
	require.NoError(t, err)

	require.NoError(t, sqlcommon.EnsureApitoFunctionsCompositePK(ctx, db))

	var createSQL string
	err = sqldb.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='apito_functions'`).Scan(&createSQL)
	require.NoError(t, err)
	require.Contains(t, createSQL, "PRIMARY KEY")
	require.Contains(t, createSQL, "name")
}
