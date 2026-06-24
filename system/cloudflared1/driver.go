package cloudflared1

import (
	"context"
	"fmt"
	"strings"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/open_driver/cfd1"
)

// Driver is the Cloudflare D1 system (control-plane) driver.
type Driver struct {
	Conf             *models.Config
	DriverCredential *models.DriverCredentials
	DB               *cfd1.DB
}

// GetCloudflareD1SystemDriver opens the system cloudflared1 driver.
func GetCloudflareD1SystemDriver(cfg *models.Config, cred *models.DriverCredentials) (*Driver, error) {
	if cred == nil {
		return nil, cfd1.ErrCredentialsRequired
	}
	ec := *cred
	ec.Engine = cfd1.EngineName
	db, err := cfd1.NewDB(cfd1.DefaultConfig(strings.TrimSpace(ec.Database), nil))
	if err != nil {
		return nil, err
	}
	return &Driver{Conf: cfg, DriverCredential: &ec, DB: db}, nil
}

var _ interfaces.ApitoSystemDB = (*Driver)(nil)

func (d *Driver) Ping(ctx context.Context) error {
	if d == nil || d.DB == nil {
		return fmt.Errorf("cloudflared1 system: not initialized")
	}
	_, err := d.DB.QueryRow(ctx, "SELECT 1 AS ok")
	return err
}

func (d *Driver) Close() error { return nil }
