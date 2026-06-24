package cloudflared1

import (
	"strings"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/open_driver/cfd1"
)

// Driver is the Cloudflare D1 project driver.
type Driver struct {
	Conf             *models.Config
	DriverCredential *models.DriverCredentials
	DB               *cfd1.DB
}

// GetDriver opens a project cloudflared1 driver.
func GetDriver(cfg *models.Config, cred *models.DriverCredentials, boundExec cfd1.BoundExecFunc) (*Driver, error) {
	if cred == nil {
		return nil, cfd1.ErrCredentialsRequired
	}
	ec := *cred
	ec.Engine = cfd1.EngineName
	db, err := cfd1.NewDB(cfd1.DefaultConfig(strings.TrimSpace(ec.Database), boundExec))
	if err != nil {
		return nil, err
	}
	return &Driver{Conf: cfg, DriverCredential: &ec, DB: db}, nil
}

var _ interfaces.ProjectDBInterface = (*Driver)(nil)
