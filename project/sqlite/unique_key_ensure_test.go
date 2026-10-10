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

func stockVariantSchema(reverse string, keys []*models.UniqueKey) []*models.ModelType {
	stock := &models.ModelType{
		Name: "stock",
		Fields: []*models.FieldInfo{
			{Identifier: "warehouse_id", FieldType: "text"},
		},
		Connections: []*models.ConnectionType{
			{Model: "variant", Relation: "has_one", Type: "forward"},
		},
		UniqueKeys: keys,
	}
	variant := &models.ModelType{
		Name: "variant",
		Connections: []*models.ConnectionType{
			{Model: "stock", Relation: reverse, Type: "backward"},
		},
	}
	return []*models.ModelType{stock, variant}
}

func TestEnsureRelation_oneToOneThenCompositeUnique(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`
CREATE TABLE stock (id VARCHAR(36) NOT NULL PRIMARY KEY, warehouse_id VARCHAR(36));
CREATE TABLE variant (id VARCHAR(36) NOT NULL PRIMARY KEY);
`).Exec(ctx)
	require.NoError(t, err)
	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})

	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, stockVariantSchema("has_one", nil)))
	requireIndex(t, ctx, db, "idx_stock_variant_id_unique", true)

	// A second ensure with the same published 1:1 schema must keep the single-column unique.
	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, stockVariantSchema("has_one", nil)))
	requireIndex(t, ctx, db, "idx_stock_variant_id_unique", true)
	requireIndex(t, ctx, db, "idx_stock_variant_warehouse", false)

	keys := []*models.UniqueKey{{Name: "variant_warehouse", Columns: []string{"variant_id", "warehouse_id"}}}
	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, stockVariantSchema("has_many", keys)))
	requireIndex(t, ctx, db, "idx_stock_variant_id_unique", false)
	requireIndex(t, ctx, db, "idx_stock_variant_warehouse", true)

	// Later ensure must not put the one-variant unique back.
	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, stockVariantSchema("has_many", keys)))
	requireIndex(t, ctx, db, "idx_stock_variant_id_unique", false)

	_, err = db.NewRaw(`INSERT INTO variant (id) VALUES ('v1')`).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO stock (id, variant_id, warehouse_id) VALUES ('s1', 'v1', 'w1')`).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO stock (id, variant_id, warehouse_id) VALUES ('s2', 'v1', 'w1')`).Exec(ctx)
	require.Error(t, err)
	_, err = db.NewRaw(`INSERT INTO stock (id, variant_id, warehouse_id) VALUES ('s3', 'v1', 'w2')`).Exec(ctx)
	require.NoError(t, err)
}

func TestEnsureRelation_sharedSaaSPrefixesTenantID(t *testing.T) {
	ctx := models.WithSharedSaaSIndex(context.Background(), true)
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`
CREATE TABLE stock (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  tenant_id VARCHAR(36),
  variant_id VARCHAR(36),
  warehouse_id VARCHAR(36)
);
`).Exec(ctx)
	require.NoError(t, err)
	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	keys := []*models.UniqueKey{{Name: "variant_warehouse", Columns: []string{"variant_id", "warehouse_id"}}}
	require.NoError(t, drv.ApplyUniqueKeyDDL(ctx, stockVariantSchema("has_many", keys), nil))

	var sqlText string
	err = db.NewRaw(`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?`, "idx_stock_variant_warehouse").Scan(ctx, &sqlText)
	require.NoError(t, err)
	require.Contains(t, sqlText, "tenant_id")
	require.Contains(t, sqlText, "variant_id")
	require.Contains(t, sqlText, "warehouse_id")
}

func TestUniqueKeyIndexColumns_separateDatabaseOmitsTenant(t *testing.T) {
	cols := models.UniqueKeyIndexColumns(false, []string{"variant_id", "warehouse_id"})
	require.Equal(t, []string{"variant_id", "warehouse_id"}, cols)
	shared := models.UniqueKeyIndexColumns(true, []string{"variant_id", "warehouse_id"})
	require.Equal(t, []string{"tenant_id", "variant_id", "warehouse_id"}, shared)
	already := models.UniqueKeyIndexColumns(true, []string{"tenant_id", "variant_id"})
	require.Equal(t, []string{"tenant_id", "variant_id"}, already)
}

func requireIndex(t *testing.T, ctx context.Context, db *bun.DB, name string, want bool) {
	t.Helper()
	var n int
	err := db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(ctx, &n)
	require.NoError(t, err)
	if want {
		require.Equal(t, 1, n, name)
		return
	}
	require.Equal(t, 0, n, name)
}
