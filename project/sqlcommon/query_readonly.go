//go:build !cloudflare

package sqlcommon

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// QueryReadOnly runs one SELECT, WITH, EXPLAIN, or PRAGMA statement.
func (d *Driver) QueryReadOnly(ctx context.Context, query string, limit int) ([]string, [][]interface{}, error) {
	if d == nil || d.ORM == nil {
		return nil, nil, fmt.Errorf("sql driver is not open")
	}
	q := strings.TrimSpace(query)
	q = strings.TrimSuffix(q, ";")
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil, fmt.Errorf("empty query")
	}
	if strings.Contains(q, ";") {
		return nil, nil, fmt.Errorf("only one statement is allowed")
	}
	upper := strings.ToUpper(q)
	switch {
	case strings.HasPrefix(upper, "SELECT"),
		strings.HasPrefix(upper, "WITH"),
		strings.HasPrefix(upper, "EXPLAIN"),
		strings.HasPrefix(upper, "PRAGMA"):
	default:
		return nil, nil, fmt.Errorf("only read-only queries are allowed")
	}
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	rows, err := d.ORM.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	out := make([][]interface{}, 0)
	for rows.Next() {
		if len(out) >= limit {
			break
		}
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make([]interface{}, len(cols))
		for i, v := range vals {
			switch t := v.(type) {
			case []byte:
				row[i] = string(t)
			case sql.RawBytes:
				row[i] = string(t)
			default:
				row[i] = t
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return cols, out, nil
}
