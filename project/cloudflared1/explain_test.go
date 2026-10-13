package cloudflared1

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"

	"github.com/apito-io/open_driver/project/sqlquery"
)

func explainUsesIndex(t *testing.T, ctx context.Context, db *bun.DB, query string, args ...interface{}) {
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

func TestExplainQueryPlans_cloudflared1Shapes(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE customer (
  id TEXT PRIMARY KEY,
  tenant_id TEXT,
  status TEXT,
  address TEXT,
  address__name AS (json_extract(address, '$.name')) STORED
);
CREATE INDEX idx_customer_tenant_status ON customer(tenant_id, status);
CREATE INDEX idx_customer_tenant_address__name ON customer(tenant_id, address__name);
CREATE TABLE meta (
  id TEXT PRIMARY KEY,
  doc_id TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  created_by TEXT NOT NULL,
  updated_by TEXT,
  status TEXT
);
CREATE INDEX idx_meta_doc_created ON meta(doc_id, created_at DESC);
`).Exec(ctx)
	require.NoError(t, err)
	_, _ = db.NewRaw("ANALYZE").Exec(ctx)

	explainUsesIndex(t, ctx, db, `SELECT x.id FROM customer AS x WHERE x.tenant_id = ? AND x.status = ?`, "t1", "active")
	explainUsesIndex(t, ctx, db, `SELECT x.id FROM customer AS x WHERE x.tenant_id = ? AND x.address__name = ?`, "t1", "NYC")

	cfg := &models.Config{}
	model := &models.ModelType{
		Name: "customer",
		Fields: []*models.FieldInfo{
			{Identifier: "status", FieldType: _const.TextField, InputType: _const.StringInput},
			{Identifier: "tenant_id", FieldType: _const.TextField, InputType: _const.StringInput},
		},
	}
	param := &models.CommonSystemParams{
		Model: model,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"local": "en",
				"where": map[string]interface{}{
					"status": map[string]interface{}{"eq": "active"},
				},
			},
			Info: graphql.ResolveInfo{FieldName: "customer"},
		},
	}
	q, args, err := sqlquery.RootResolverQueryBuilder(cfg, param, false)
	require.NoError(t, err)
	explainUsesIndex(t, ctx, db, q, args...)
}
