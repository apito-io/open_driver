package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// A document store treats a missing field as "not equal" to any value, so `ne`
// must keep rows whose column is NULL. Plain `col != ?` evaluates to NULL for
// those rows and silently drops them.
func TestFilterBuilder_neMatchesNullColumns(t *testing.T) {
	model := &models.ModelType{
		Name: "ledger",
		Fields: []*models.FieldInfo{
			{Identifier: "hidden", InputType: _const.BoolInput, FieldType: "boolean"},
			{Identifier: "item_type", InputType: _const.StringInput, FieldType: "text"},
		},
	}

	var args []interface{}
	preds, err := FilterBuilder("x", map[string]interface{}{
		"hidden": map[string]interface{}{"ne": true},
	}, model, &args)
	require.NoError(t, err)
	require.Len(t, preds, 1)
	require.Contains(t, preds[0], "IS NULL OR")
	require.Equal(t, []interface{}{true}, args)

	// `in` on a string field must type-check elements, not the list itself.
	args = nil
	preds, err = FilterBuilder("x", map[string]interface{}{
		"item_type": map[string]interface{}{"in": []interface{}{"product", "returnable"}},
	}, model, &args)
	require.NoError(t, err)
	require.Len(t, preds, 1)
	require.Equal(t, []interface{}{"product", "returnable"}, args)

	// Wrong element type is still rejected.
	args = nil
	_, err = FilterBuilder("x", map[string]interface{}{
		"item_type": map[string]interface{}{"in": []interface{}{42}},
	}, model, &args)
	require.Error(t, err)

	// `eq` must stay a plain comparison.
	args = nil
	preds, err = FilterBuilder("x", map[string]interface{}{
		"item_type": map[string]interface{}{"eq": "returnable"},
	}, model, &args)
	require.NoError(t, err)
	require.Len(t, preds, 1)
	require.NotContains(t, preds[0], "IS NULL")
}

// Same predicate, executed against real rows: `hidden` is NULL on every row the
// app writes, and the profile page filters with `hidden ne true`.
func TestFilterBuilder_nePredicateKeepsNullRowsInSQLite(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:filter_ne_null_test?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx := context.Background()

	_, err = db.ExecContext(ctx, `
CREATE TABLE ledger (id TEXT PRIMARY KEY, hidden BOOLEAN);
INSERT INTO ledger(id, hidden) VALUES ('a', NULL), ('b', 0), ('c', 1);
`)
	require.NoError(t, err)

	model := &models.ModelType{
		Name:   "ledger",
		Fields: []*models.FieldInfo{{Identifier: "hidden", InputType: _const.BoolInput, FieldType: "boolean"}},
	}

	var args []interface{}
	preds, err := FilterBuilder("x", map[string]interface{}{
		"hidden": map[string]interface{}{"ne": true},
	}, model, &args)
	require.NoError(t, err)

	query := fmt.Sprintf("SELECT id FROM ledger AS x WHERE %s ORDER BY id", strings.Join(preds, " AND "))
	rows, err := db.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"a", "b"}, ids)
}
