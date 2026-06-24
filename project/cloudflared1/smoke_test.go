//go:build cloudflare

package cloudflared1_test

import (
	"testing"

	"gitlab.com/apito.io/open_driver/cfd1"
	projcfd1 "gitlab.com/apito.io/open_driver/project/cloudflared1"
	syscfd1 "gitlab.com/apito.io/open_driver/system/cloudflared1"
)

func TestEngineName(t *testing.T) {
	if cfd1.EngineName != "cloudflared1" {
		t.Fatalf("engine name = %q", cfd1.EngineName)
	}
}

func TestDriversCompile(t *testing.T) {
	var _ = (*projcfd1.Driver)(nil)
	var _ = (*syscfd1.Driver)(nil)
}
