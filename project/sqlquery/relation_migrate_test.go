package sqlquery

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/apito-io/engine/relationmigrate"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func testSession(t *testing.T) (context.Context, *bun.DB, SQLSession) {
	t.Helper()
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`PRAGMA foreign_keys = ON`).Exec(ctx)
	require.NoError(t, err)
	session := NewBunSession(ctx, "sqlite", "sqlite:memory", db, func(table, column string) error {
		_, err := db.NewRaw(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", table, column)).Exec(ctx)
		return err
	})
	return ctx, db, session
}

func seedOrderTests(t *testing.T, ctx context.Context, db *bun.DB) {
	t.Helper()
	_, err := db.NewRaw(`PRAGMA foreign_keys = OFF`).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`
CREATE TABLE lab_order (id VARCHAR(36) NOT NULL PRIMARY KEY, note VARCHAR(32));
CREATE TABLE lab_test (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  name VARCHAR(64),
  lab_order_id VARCHAR(36) REFERENCES lab_order(id) ON DELETE CASCADE,
  test_category_id VARCHAR(36)
);
INSERT INTO lab_order (id, note) VALUES ('o1', 'keep'), ('o2', 'keep');
INSERT INTO lab_test (id, name, lab_order_id, test_category_id) VALUES
  ('t1', 'breakfast', 'o1', 'cat'),
  ('t2', 'dangling', 'missing', 'cat'),
  ('t3', 'other', 'o2', 'cat');
`).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`PRAGMA foreign_keys = ON`).Exec(ctx)
	require.NoError(t, err)
}

func widenReq() relationmigrate.Request {
	return relationmigrate.Request{
		SchemaVersion: 38, FromModel: "lab_order", ToModel: "lab_test",
		FromRelation: "has_many", ToRelation: "has_one",
		TargetFromRelation: "has_many", TargetToRelation: "has_many",
	}
}

func TestWidenOneToManyCopiesValidPairsAndDropsFK(t *testing.T) {
	ctx, db, session := testSession(t)
	seedOrderTests(t, ctx, db)
	preview, err := PreviewRelationCardinality(session, widenReq())
	require.NoError(t, err)
	require.False(t, preview.Blocked)
	require.Equal(t, relationmigrate.KindWidenToNN, preview.TransitionKind)
	require.Equal(t, 2, preview.ValidPairs)
	require.Equal(t, 1, preview.Dangling)

	receipt, err := MigrateRelationCardinality(session, widenReq())
	require.NoError(t, err)
	require.Equal(t, "passed", receipt.Status)
	require.Equal(t, 2, receipt.DestinationPairs)

	var n int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM lab_order_lab_test`).Scan(ctx, &n))
	require.Equal(t, 2, n)
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('lab_test') WHERE name = 'lab_order_id'`).Scan(ctx, &n))
	require.Equal(t, 0, n)
	var category string
	require.NoError(t, db.NewRaw(`SELECT test_category_id FROM lab_test WHERE id = 't1'`).Scan(ctx, &category))
	require.Equal(t, "cat", category)

	again, err := MigrateRelationCardinality(session, widenReq())
	require.NoError(t, err)
	require.True(t, again.Idempotent)
	require.Equal(t, 2, again.DestinationPairs)

	_, err = db.NewRaw(`DELETE FROM lab_order WHERE id = 'o1'`).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM lab_test WHERE id = 't1'`).Scan(ctx, &n))
	require.Equal(t, 1, n, "catalog row must survive order delete")
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM lab_order_lab_test WHERE lab_order_id = 'o1'`).Scan(ctx, &n))
	require.Equal(t, 0, n, "only the pivot link is removed")
}

func TestOwnershipFlipMovesForeignKey(t *testing.T) {
	ctx, db, session := testSession(t)
	_, err := db.NewRaw(`
CREATE TABLE lab_order (id VARCHAR(36) PRIMARY KEY, note VARCHAR(32));
CREATE TABLE lab_test (
  id VARCHAR(36) PRIMARY KEY,
  lab_order_id VARCHAR(36) REFERENCES lab_order(id) ON DELETE CASCADE,
  test_category_id VARCHAR(36)
);
INSERT INTO lab_order (id, note) VALUES ('o1', 'keep');
INSERT INTO lab_test (id, lab_order_id, test_category_id) VALUES ('t1', 'o1', 'cat');
`).Exec(ctx)
	require.NoError(t, err)
	req := relationmigrate.Request{
		FromModel: "lab_order", ToModel: "lab_test",
		FromRelation: "has_many", ToRelation: "has_one",
		TargetFromRelation: "has_one", TargetToRelation: "has_many",
	}
	receipt, err := MigrateRelationCardinality(session, req)
	require.NoError(t, err)
	require.Equal(t, "passed", receipt.Status)
	var orderID, category string
	require.NoError(t, db.NewRaw(`SELECT lab_test_id FROM lab_order WHERE id = 'o1'`).Scan(ctx, &orderID))
	require.Equal(t, "t1", orderID)
	require.NoError(t, db.NewRaw(`SELECT test_category_id FROM lab_test WHERE id = 't1'`).Scan(ctx, &category))
	require.Equal(t, "cat", category)
	var n int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('lab_test') WHERE name = 'lab_order_id'`).Scan(ctx, &n))
	require.Equal(t, 0, n)
}

func TestNarrowingBlocksWhenDegreeExceedsOne(t *testing.T) {
	ctx, db, session := testSession(t)
	_, err := db.NewRaw(`
CREATE TABLE lab_order (id VARCHAR(36) PRIMARY KEY);
CREATE TABLE lab_test (id VARCHAR(36) PRIMARY KEY);
CREATE TABLE lab_order_lab_test (
  lab_order_id VARCHAR(36), lab_test_id VARCHAR(36), PRIMARY KEY (lab_order_id, lab_test_id)
);
INSERT INTO lab_order (id) VALUES ('o1'), ('o2');
INSERT INTO lab_test (id) VALUES ('t1'), ('t2');
INSERT INTO lab_order_lab_test (lab_order_id, lab_test_id) VALUES ('o1', 't1'), ('o2', 't1');
`).Exec(ctx)
	require.NoError(t, err)
	req := relationmigrate.Request{
		FromModel: "lab_order", ToModel: "lab_test",
		FromRelation: "has_many", ToRelation: "has_many",
		TargetFromRelation: "has_many", TargetToRelation: "has_one",
	}
	preview, err := PreviewRelationCardinality(session, req)
	require.NoError(t, err)
	require.True(t, preview.Blocked)
	_, err = MigrateRelationCardinality(session, req)
	require.Error(t, err)
	var n int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM lab_order_lab_test`).Scan(ctx, &n))
	require.Equal(t, 2, n, "blocked migration must keep the pivot")
}

func TestOneToOneToManyKeepsValueAndDropsUnique(t *testing.T) {
	ctx, db, session := testSession(t)
	_, err := db.NewRaw(`
CREATE TABLE stock (id VARCHAR(36) PRIMARY KEY, variant_id VARCHAR(36), warehouse_id VARCHAR(36));
CREATE TABLE variant (id VARCHAR(36) PRIMARY KEY, stock_id VARCHAR(36));
CREATE UNIQUE INDEX idx_stock_variant_id_unique ON stock (variant_id);
INSERT INTO variant (id, stock_id) VALUES ('v1', 's1');
INSERT INTO stock (id, variant_id, warehouse_id) VALUES ('s1', 'v1', 'w1');
`).Exec(ctx)
	require.NoError(t, err)
	req := relationmigrate.Request{
		FromModel: "stock", ToModel: "variant",
		FromRelation: "has_one", ToRelation: "has_one",
		TargetFromRelation: "has_one", TargetToRelation: "has_many",
	}
	receipt, err := MigrateRelationCardinality(session, req)
	require.NoError(t, err)
	require.Equal(t, "passed", receipt.Status)
	var variantID, warehouse string
	require.NoError(t, db.NewRaw(`SELECT variant_id, warehouse_id FROM stock WHERE id = 's1'`).Scan(ctx, &variantID, &warehouse))
	require.Equal(t, "v1", variantID)
	require.Equal(t, "w1", warehouse)
	var n int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_stock_variant_id_unique'`).Scan(ctx, &n))
	require.Equal(t, 0, n)
}

func TestDialectInsertSQL(t *testing.T) {
	require.Contains(t, insertIgnore("sqlite", "p", "a", "b"), "INSERT OR IGNORE")
	require.Contains(t, insertIgnore("libsql", "p", "a", "b"), "INSERT OR IGNORE")
	require.Contains(t, insertIgnore("postgresql", "p", "a", "b"), "ON CONFLICT DO NOTHING")
	require.Contains(t, insertIgnore("mysql", "p", "a", "b"), "INSERT IGNORE")
	require.Contains(t, insertIgnore("mariadb", "p", "a", "b"), "INSERT IGNORE")
	spec, err := describeStorage("lab_order", "has_many", "lab_test", "has_one", "")
	require.NoError(t, err)
	require.Equal(t, "lab_test", spec.FKs[0].Holder)
	require.Equal(t, "lab_order_id", spec.FKs[0].Column)
	pivot, err := describeStorage("lab_order", "has_many", "lab_test", "has_many", "")
	require.NoError(t, err)
	require.Equal(t, "lab_order_lab_test", pivot.Pivot)
}
