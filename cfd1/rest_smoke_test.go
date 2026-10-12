package cfd1

import (
	"context"
	"os"
	"testing"
)

// TestRESTThreeProjectRoutingSmoke verifies REST mode can reach project D1 while system D1 uses bound.
func TestRESTThreeProjectRoutingSmoke(t *testing.T) {
	account := os.Getenv("CF_ACCOUNT_ID")
	token := os.Getenv("CF_API_TOKEN")
	projectDB := os.Getenv("APITO_PROJECT_D1_ID")
	systemID := os.Getenv("APITO_SYSTEM_D1_ID")
	if account == "" || token == "" || projectDB == "" {
		t.Skip("CF_ACCOUNT_ID, CF_API_TOKEN, APITO_PROJECT_D1_ID required for live D1 REST smoke")
	}

	SetGlobalCredentials(account, token)
	SetGlobalSystemD1DatabaseID(systemID)

	cfgREST := DefaultConfig(projectDB, nil)
	if cfgREST.Mode != AccessREST {
		t.Fatalf("expected REST for project db, got %v", cfgREST.Mode)
	}
	if cfgREST.BoundExec != nil {
		t.Fatal("REST config must not set BoundExec")
	}
	dbREST, err := NewDB(cfgREST)
	if err != nil {
		t.Fatal(err)
	}
	_, err = dbREST.Exec(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("project REST query: %v", err)
	}

	cfgBound := DefaultConfig(systemID, func(ctx context.Context, sql string, params []any) (Result, error) {
		return Result{}, nil
	})
	if cfgBound.Mode != AccessBound {
		t.Fatalf("expected bound for system d1, got %v", cfgBound.Mode)
	}
	if cfgBound.BoundExec == nil {
		t.Fatal("system config must set BoundExec")
	}
}
