//go:build cloudflare

package system

import (
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateOpenCoreSystemDriver is unavailable on Workers; use cfg.DriverFactory (cloudflared1).
func CreateOpenCoreSystemDriver(_ *models.Config, engineConfig *models.DriverCredentials) (interfaces.ApitoSystemDB, error) {
	eng := ""
	if engineConfig != nil {
		eng = engineConfig.Engine
	}
	return nil, fmt.Errorf("open-core system driver %q not available on Workers; configure cloudflared1 factory", eng)
}
