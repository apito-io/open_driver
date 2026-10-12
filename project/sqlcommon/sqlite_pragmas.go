//go:build !cloudflare

package sqlcommon

import (
	"context"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/uptrace/bun"
)

// EngineIsSQLiteLike reports engines that use SQLite semantics (embedded SQLite, libsql, Turso).
func EngineIsSQLiteLike(engine string) bool {
	e := strings.ToLower(strings.TrimSpace(engine))
	switch e {
	case strings.ToLower(_const.SQLiteDriver), "libsql", "turso":
		return true
	default:
		return false
	}
}

func libsqlDSNIsLocalFile(dsn string) bool {
	d := strings.TrimSpace(strings.ToLower(dsn))
	return strings.HasPrefix(d, "file:")
}

// ApplySQLiteConnectionPragmas sets FK enforcement and performance-related PRAGMAs for SQLite/libsql.
// Optional cacheSizeKB / mmapBytes override defaults (20000 KiB cache, 128 MiB mmap) when > 0.
func ApplySQLiteConnectionPragmas(ctx context.Context, orm *bun.DB, engine, dsn string, opts ...SQLitePragmaOpts) error {
	if orm == nil || !EngineIsSQLiteLike(engine) {
		return nil
	}
	cacheKB := 20000
	mmap := int64(134217728)
	if len(opts) > 0 {
		if opts[0].CacheSizeKB > 0 {
			cacheKB = opts[0].CacheSizeKB
		}
		if opts[0].MmapBytes > 0 {
			mmap = opts[0].MmapBytes
		}
	}
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA temp_store = MEMORY;",
		fmt.Sprintf("PRAGMA cache_size = -%d;", cacheKB),
	}
	if libsqlDSNIsLocalFile(dsn) {
		stmts = append(stmts,
			"PRAGMA journal_mode = WAL;",
			"PRAGMA synchronous = NORMAL;",
			fmt.Sprintf("PRAGMA mmap_size = %d;", mmap),
		)
	}
	for _, s := range stmts {
		if _, err := orm.NewRaw(s).Exec(ctx); err != nil {
			continue
		}
	}
	return nil
}

// SQLitePragmaOpts tunes cache/mmap for multi-tenant capacity.
type SQLitePragmaOpts struct {
	CacheSizeKB int
	MmapBytes   int64
}

// ApplyLitestreamConnectionPragmas configures SQLite for embedded Litestream local files.
func ApplyLitestreamConnectionPragmas(ctx context.Context, orm *bun.DB, engine string) error {
	if orm == nil || !EngineIsSQLiteLike(engine) {
		return nil
	}
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA cache_size = -20000;",
	}
	for _, s := range stmts {
		if _, err := orm.NewRaw(s).Exec(ctx); err != nil {
			continue
		}
	}
	return nil
}
