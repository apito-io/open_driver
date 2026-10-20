package mysql

import (
	"context"

	"github.com/apito-io/engine/services"
	"github.com/apito-io/open_driver/system/sqlcommon"
)

var _ services.ProjectUserSessionRevocationStore = (*Driver)(nil)

func (d *Driver) GetProjectUserSessionRevokedAfter(ctx context.Context, projectID, userID string) (int64, error) {
	return sqlcommon.GetProjectUserSessionRevokedAfter(ctx, d.ORM, projectID, userID)
}

func (d *Driver) SetProjectUserSessionRevokedAfter(ctx context.Context, projectID, userID string, revokedAfter int64) error {
	return sqlcommon.SetProjectUserSessionRevokedAfter(ctx, d.ORM, projectID, userID, revokedAfter)
}
