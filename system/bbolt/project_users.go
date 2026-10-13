package bbolt

import (
	"context"

	"github.com/apito-io/open_driver/system/sysusers"
	"github.com/apito-io/engine/models"
)

func (d *BBoltSystemDriver) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	return sysusers.CreateUser(ctx, user)
}

func (d *BBoltSystemDriver) GetUser(ctx context.Context, projectID, userID string) (*models.User, error) {
	return sysusers.GetUser(ctx, projectID, userID)
}

func (d *BBoltSystemDriver) UpdateUser(ctx context.Context, user *models.User) error {
	return sysusers.UpdateUser(ctx, user)
}

func (d *BBoltSystemDriver) DeleteUser(ctx context.Context, projectID, userID string) error {
	return sysusers.DeleteUser(ctx, projectID, userID)
}

func (d *BBoltSystemDriver) SearchProjectUsers(ctx context.Context, projectID, q string, limit, offset int) ([]*models.User, int, error) {
	return sysusers.SearchProjectUsers(ctx, projectID, q, limit, offset)
}

func (d *BBoltSystemDriver) GetUserByUsername(ctx context.Context, projectID, username string) (*models.User, error) {
	return sysusers.GetUserByUsername(ctx, projectID, username)
}

func (d *BBoltSystemDriver) ListUsersByEmail(ctx context.Context, projectID, email string) ([]*models.User, error) {
	return sysusers.ListUsersByEmail(ctx, projectID, email)
}

func (d *BBoltSystemDriver) ListUsersByPhone(ctx context.Context, projectID, phone string) ([]*models.User, error) {
	return sysusers.ListUsersByPhone(ctx, projectID, phone)
}

func (d *BBoltSystemDriver) ListUsersByGoogleSub(ctx context.Context, projectID, googleSub string) ([]*models.User, error) {
	return sysusers.ListUsersByGoogleSub(ctx, projectID, googleSub)
}

func (d *BBoltSystemDriver) CountProjectUsersByRole(ctx context.Context, projectID string) (map[string]int, error) {
	return sysusers.CountProjectUsersByRole(ctx, projectID)
}
