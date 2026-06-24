package cfd1

import (
	"context"
	"testing"
)

func TestDefaultConfig_projectDBUsesRESTWhenBoundExecSet(t *testing.T) {
	SetGlobalCredentials("acct", "token")
	SetGlobalSystemD1DatabaseID("system-uuid-123")
	SetGlobalBoundExec(func(ctx context.Context, query string, params []any) (Result, error) {
		return Result{}, nil
	})
	t.Cleanup(func() {
		SetGlobalBoundExec(nil)
		SetGlobalSystemD1DatabaseID("")
		SetGlobalCredentials("", "")
	})

	cfg := DefaultConfig("project-uuid-456", nil)
	if cfg.Mode != AccessREST {
		t.Fatalf("project DB should use REST, got mode %v", cfg.Mode)
	}
	if cfg.BoundExec != nil {
		t.Fatal("project DB should not carry bound exec")
	}
	if cfg.AccountID != "acct" || cfg.APIToken != "token" {
		t.Fatalf("REST creds missing: %+v", cfg)
	}
}

func TestDefaultConfig_systemDBUsesBoundWhenBoundExecSet(t *testing.T) {
	SetGlobalCredentials("acct", "token")
	SetGlobalSystemD1DatabaseID("system-uuid-123")
	var called bool
	SetGlobalBoundExec(func(ctx context.Context, query string, params []any) (Result, error) {
		called = true
		return Result{}, nil
	})
	t.Cleanup(func() {
		SetGlobalBoundExec(nil)
		SetGlobalSystemD1DatabaseID("")
		SetGlobalCredentials("", "")
	})

	for _, id := range []string{"system-uuid-123", "bound", ""} {
		cfg := DefaultConfig(id, nil)
		if cfg.Mode != AccessBound {
			t.Fatalf("system DB %q should use bound, got %v", id, cfg.Mode)
		}
		if cfg.BoundExec == nil {
			t.Fatalf("system DB %q missing bound exec", id)
		}
	}
	_ = called
}
