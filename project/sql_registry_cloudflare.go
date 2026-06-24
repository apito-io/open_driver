//go:build cloudflare

package project

import (
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// GetProjectSQLDriver is unavailable on Workers builds.
func GetProjectSQLDriver(_ *models.Config, cred *models.DriverCredentials) (interfaces.ProjectDBInterface, error) {
	eng := ""
	if cred != nil {
		eng = cred.Engine
	}
	return nil, fmt.Errorf("sql project driver %q not available on Workers", eng)
}
