package postgres

import (
	"context"

	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/open_driver/project/sqlquery"
)

func (d *Driver) ApplyUniqueKeyDDL(ctx context.Context, modelsList []*models.ModelType, drops []models.UniqueIndexDrop) error {
	if d == nil || d.ORM == nil {
		return nil
	}
	return sqlquery.ApplyUniqueKeyDDL(d.Engine(), models.SharedSaaSIndexFromContext(ctx), modelsList, drops, func(q string) error {
		_, err := d.ORM.NewRaw(q).Exec(ctx)
		return err
	})
}

func (d *Driver) dropStaleOneToOneUnique(ctx context.Context, from, to *models.ConnectionType) error {
	if d == nil || d.ORM == nil {
		return nil
	}
	return sqlquery.ApplyStaleOneToOne(d.Engine(), from, to, func(q string) error {
		_, err := d.ORM.NewRaw(q).Exec(ctx)
		return err
	})
}
