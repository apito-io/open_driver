//go:build !cloudflare

package sqlcommon

import (
	"context"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
)

func (d *Driver) quoteIdent(name string) string {
	name = strings.TrimSpace(name)
	if d != nil && d.Dialect != nil {
		return d.Dialect.QuoteIdent(name)
	}
	return QuoteMySQLIdent(name)
}

func isMissingRelationObject(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no such table") ||
		strings.Contains(s, "doesn't exist") ||
		strings.Contains(s, "does not exist") ||
		strings.Contains(s, "unknown table")
}

func insertShadowIDSQL(engine, table, id string) (string, []interface{}) {
	eng := strings.ToLower(strings.TrimSpace(engine))
	table = strings.TrimSpace(table)
	switch eng {
	case _const.PostgreSQLDriver:
		return fmt.Sprintf(`INSERT INTO "%s" (id) VALUES (?) ON CONFLICT (id) DO NOTHING`, table), []interface{}{id}
	case _const.MySQLDriver, _const.MariaDBDriver:
		return fmt.Sprintf("INSERT IGNORE INTO `%s` (`id`) VALUES (?)", table), []interface{}{id}
	default:
		return fmt.Sprintf("INSERT OR IGNORE INTO `%s` (`id`) VALUES (?)", table), []interface{}{id}
	}
}

// ListDistinctNonEmptyColumn returns unique non-empty values from table.column.
// Missing tables yield an empty slice (no error) so shadow repair is idempotent.
func (d *Driver) ListDistinctNonEmptyColumn(ctx context.Context, table, column string) ([]string, error) {
	if d == nil || d.ORM == nil {
		return nil, nil
	}
	table = strings.TrimSpace(table)
	column = strings.TrimSpace(column)
	if table == "" || column == "" {
		return nil, nil
	}
	q := "SELECT DISTINCT " + d.quoteIdent(column) + " FROM " + d.quoteIdent(table) +
		" WHERE " + d.quoteIdent(column) + " IS NOT NULL AND CAST(" + d.quoteIdent(column) + " AS TEXT) != ''"
	var vals []string
	err := d.ORM.NewRaw(q).Scan(ctx, &vals)
	if err != nil {
		if isMissingRelationObject(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	seen := map[string]struct{}{}
	for _, id := range vals {
		v := strings.TrimSpace(id)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out, nil
}

// InsertShadowIDs upserts id-only rows into a common-model shadow table.
func (d *Driver) InsertShadowIDs(ctx context.Context, table string, ids []string) (int, error) {
	if d == nil || d.ORM == nil {
		return 0, nil
	}
	eng := ""
	if d.DriverCredential != nil {
		eng = d.DriverCredential.Engine
	}
	n := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		q, args := insertShadowIDSQL(eng, table, id)
		res, err := d.ORM.NewRaw(q, args...).Exec(ctx)
		if err != nil {
			if isMissingRelationObject(err) {
				continue
			}
			return n, err
		}
		if ra, raErr := res.RowsAffected(); raErr == nil && ra > 0 {
			n++
		}
	}
	return n, nil
}
