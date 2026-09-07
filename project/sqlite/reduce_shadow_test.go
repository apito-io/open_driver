package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestReduceModelTableToShadow_PreservesChildFKAndRestoresPragma(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:reduce_shadow_fk?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`PRAGMA foreign_keys = ON`).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`
CREATE TABLE catalog_item (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  name TEXT
);
CREATE INDEX catalog_item_name ON catalog_item(name);
CREATE TABLE tenant_record (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  title TEXT,
  catalog_item_id VARCHAR(36),
  FOREIGN KEY (catalog_item_id) REFERENCES catalog_item(id)
);
CREATE INDEX tenant_record_catalog_item_id ON tenant_record(catalog_item_id);
INSERT INTO catalog_item (id, name) VALUES ('c1', 'Alpha'), ('c2', 'Beta');
INSERT INTO tenant_record (id, title, catalog_item_id) VALUES ('t1', 'row', 'c1');
`).Exec(ctx)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	require.NoError(t, drv.ReduceModelTableToShadow(ctx, "catalog_item"))

	assertTable(t, db, ctx, "catalog_item", true)
	assertColumn(t, db, ctx, "catalog_item", "id", true)
	assertColumn(t, db, ctx, "catalog_item", "name", false)
	assertColumn(t, db, ctx, "tenant_record", "catalog_item_id", true)
	assertTable(t, db, ctx, "tenant_record", true)

	var fk int
	require.NoError(t, db.NewRaw("PRAGMA foreign_keys").Scan(ctx, &fk))
	require.Equal(t, 1, fk, "foreign_keys pragma must be restored")

	var n int
	require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM catalog_item").Scan(ctx, &n))
	require.Equal(t, 2, n)
	require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM tenant_record WHERE catalog_item_id = 'c1'").Scan(ctx, &n))
	require.Equal(t, 1, n)

	var idxCount int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='tenant_record_catalog_item_id'`).Scan(ctx, &idxCount))
	require.Equal(t, 1, idxCount, "dependent index on tenant_record must survive")
}

func TestReduceModelTableToShadow_LeavesSourceWhenCopyFailsVerification(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:reduce_shadow_missing?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	err = drv.ReduceModelTableToShadow(ctx, "does_not_exist")
	require.Error(t, err)
	require.Contains(t, err.Error(), "source table missing")
}

func TestReduceModelTableToShadow_IdempotentOnAlreadyShadow(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:reduce_shadow_twice?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`CREATE TABLE catalog_item (id VARCHAR(36) NOT NULL PRIMARY KEY); INSERT INTO catalog_item (id) VALUES ('c1');`).Exec(ctx)
	require.NoError(t, err)
	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	require.NoError(t, drv.ReduceModelTableToShadow(ctx, "catalog_item"))
	require.NoError(t, drv.ReduceModelTableToShadow(ctx, "catalog_item"))
	var n int
	require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM catalog_item").Scan(ctx, &n))
	require.Equal(t, 1, n)
}
