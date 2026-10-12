package cfd1

import (
	"strings"
	"sync"
)

var (
	globalMu                 sync.RWMutex
	globalAccountID          string
	globalAPIToken           string
	globalBoundExec          BoundExecFunc
	globalSystemD1DatabaseID string
)

// SetGlobalCredentials configures REST API access for dynamic D1 databases.
func SetGlobalCredentials(accountID, apiToken string) {
	globalMu.Lock()
	globalAccountID = accountID
	globalAPIToken = apiToken
	globalMu.Unlock()
}

// SetGlobalBoundExec configures bound D1 execution (Worker system DB).
func SetGlobalBoundExec(exec BoundExecFunc) {
	globalMu.Lock()
	globalBoundExec = exec
	globalMu.Unlock()
}

// SetGlobalSystemD1DatabaseID records the SYSTEM_D1 database UUID (or "bound" on Workers).
// Project/tenant drivers with a different database ID use REST instead of the bound binding.
func SetGlobalSystemD1DatabaseID(databaseID string) {
	globalMu.Lock()
	globalSystemD1DatabaseID = strings.TrimSpace(databaseID)
	globalMu.Unlock()
}

func globalConfig() (accountID, apiToken, systemD1ID string, bound BoundExecFunc) {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalAccountID, globalAPIToken, globalSystemD1DatabaseID, globalBoundExec
}

// isSystemD1Database reports whether databaseID should use the Worker SYSTEM_D1 binding.
func isSystemD1Database(databaseID, systemD1ID string) bool {
	id := strings.TrimSpace(databaseID)
	if id == "" || id == "bound" {
		return true
	}
	sys := strings.TrimSpace(systemD1ID)
	if sys == "" || sys == "bound" {
		return id == "bound"
	}
	return strings.EqualFold(id, sys)
}

// DefaultConfig builds a Config using global credentials and optional bound exec override.
// Bound mode is used only when the target database is the system D1 (same UUID or "bound").
// All other database UUIDs use REST so project/tenant D1s are queried correctly.
func DefaultConfig(databaseID string, boundOverride BoundExecFunc) Config {
	accountID, apiToken, systemD1ID, bound := globalConfig()
	if boundOverride != nil {
		bound = boundOverride
	}
	mode := AccessREST
	var boundExec BoundExecFunc
	useBound := bound != nil && isSystemD1Database(databaseID, systemD1ID)
	if useBound {
		mode = AccessBound
		boundExec = bound
	}
	return Config{
		Mode:       mode,
		DatabaseID: databaseID,
		AccountID:  accountID,
		APIToken:   apiToken,
		BoundExec:  boundExec,
	}
}
