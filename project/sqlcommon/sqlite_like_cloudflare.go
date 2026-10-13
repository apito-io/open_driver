//go:build cloudflare

package sqlcommon

import (
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/open_driver/cfd1"
)

// EngineIsSQLiteLike reports engines that use SQLite semantics on Workers (D1).
func EngineIsSQLiteLike(engine string) bool {
	e := strings.ToLower(strings.TrimSpace(engine))
	switch e {
	case strings.ToLower(_const.SQLiteDriver), "libsql", "turso", cfd1.EngineName, "d1":
		return true
	default:
		return false
	}
}
