package sqlquery

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func explainDetailUsesIndex(t *testing.T, ctx context.Context, db *bun.DB, query string, args ...interface{}) {
	t.Helper()
	rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rows.Close() })
	var b strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		b.WriteString(detail)
		b.WriteByte('\n')
	}
	s := strings.ToLower(b.String())
	require.True(t, strings.Contains(s, "using index") || strings.Contains(s, "using covering index"),
		"plan should use index:\n%s", b.String())
}

func TestExplain_nestedObjectAndRepeatedFilters(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	model := customerFixtureModel()
	table := "customer"
	_, err = db.NewRaw(`
CREATE TABLE customer (
  id TEXT PRIMARY KEY,
  tenant_id TEXT,
  status TEXT,
  address TEXT,
  addresses TEXT,
  address__name AS (json_extract(address, '$.name')) STORED
);
CREATE INDEX idx_customer_tenant_address__name ON customer(tenant_id, address__name);
CREATE TABLE customer__addresses (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  parent_id TEXT NOT NULL,
  tenant_id TEXT,
  name TEXT
);
CREATE INDEX idx_customer__addresses_tenant_name ON customer__addresses(tenant_id, name);
CREATE INDEX idx_customer__addresses_parent ON customer__addresses(parent_id);
`).Exec(ctx)
	require.NoError(t, err)
	_, _ = db.NewRaw("ANALYZE").Exec(ctx)

	var args []interface{}
	preds, err := FilterBuilder("x", map[string]interface{}{
		"address": map[string]interface{}{
			"name": map[string]interface{}{"eq": "NYC"},
		},
	}, model, &args)
	require.NoError(t, err)
	q := "SELECT x.id FROM `" + table + "` AS x WHERE x.tenant_id = ? AND " + strings.Join(preds, " AND ")
	explainDetailUsesIndex(t, ctx, db, q, append([]interface{}{"t1"}, args...)...)

	args = nil
	preds, err = FilterBuilder("x", map[string]interface{}{
		"addresses": map[string]interface{}{
			"name": map[string]interface{}{"eq": "Main"},
		},
	}, model, &args)
	require.NoError(t, err)
	q = "SELECT x.id FROM `" + table + "` AS x WHERE x.tenant_id = ? AND " + strings.Join(preds, " AND ")
	// child-table EXISTS or json_each — at minimum must not full-scan without predicate
	rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+q, append([]interface{}{"t1"}, args...)...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rows.Close() })
	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		plan.WriteString(detail)
		plan.WriteByte('\n')
	}
	planStr := strings.ToLower(plan.String())
	require.False(t, strings.Contains(planStr, "scan table customer") && !strings.Contains(planStr, "search"),
		"repeated filter should avoid plain table scan:\n%s", plan.String())
}

func TestRelationFieldsDDL_hasManyPivot(t *testing.T) {
	stmts := RelationFieldsDDLStatements(
		&models.ConnectionType{Model: "post", Relation: "has_many"},
		&models.ConnectionType{Model: "tag", Relation: "has_many"},
	)
	require.NotEmpty(t, stmts)
	joined := strings.Join(stmts, "\n")
	require.Contains(t, joined, "post_tag")
	require.Contains(t, joined, "CREATE INDEX")
}

func TestNestedIndexDDL_generatedColumn(t *testing.T) {
	model := &models.ModelType{
		Name: "customer",
		Fields: []*models.FieldInfo{
			{Identifier: "address", FieldType: _const.ObjectField, SubFieldInfo: []*models.FieldInfo{
				{Identifier: "name", FieldType: _const.TextField, EnableIndexing: true},
			}},
		},
	}
	stmts := NestedIndexDDLStatements(model, true)
	require.NotEmpty(t, stmts)
	require.Contains(t, strings.Join(stmts, " "), "address__name")
}
