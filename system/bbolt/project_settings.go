package bbolt

import (
	"context"
	"errors"

	"github.com/apito-io/engine/models"
)

func (d *BBoltSystemDriver) SaveProjectAuthenticationSettings(ctx context.Context, projectID string, auth *models.AuthenticationSettings) error {
	return errors.New("bbolt: SaveProjectAuthenticationSettings not implemented")
}

func (d *BBoltSystemDriver) SaveProjectStorageSettings(ctx context.Context, projectID string, storage *models.StorageSettings) error {
	return errors.New("bbolt: SaveProjectStorageSettings not implemented")
}
