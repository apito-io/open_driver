//go:build cloudflare

package project

import (
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateOpenCoreProjectDriver is unavailable on Workers; use cfg.DriverFactory (cloudflared1).
func CreateOpenCoreProjectDriver(_ *models.Config, engineConfig *models.DriverCredentials) (interfaces.ProjectDBInterface, error) {
	eng := ""
	if engineConfig != nil {
		eng = engineConfig.Engine
	}
	return nil, fmt.Errorf("open-core project driver %q not available on Workers; configure cloudflared1 factory", eng)
}
