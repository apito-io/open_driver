package cloudflared1

import (
	"context"

	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/open_driver/project/sqlcommon"
	"gitlab.com/apito.io/open_driver/project/sqlquery"
)

func (d *Driver) AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {
	for _, q := range sqlquery.RelationFieldsDDLStatements(from, to) {
		if err := d.execDDL(ctx, q); err != nil {
			if sqlcommon.IsAlreadyExistsErr(err) {
				continue
			}
			return err
		}
	}
	_, _ = d.DB.Exec(ctx, "ANALYZE")
	return nil
}
