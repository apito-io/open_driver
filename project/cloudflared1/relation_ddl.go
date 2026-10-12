package cloudflared1

import (
	"context"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/open_driver/project/sqlcommon"
	"github.com/apito-io/open_driver/project/sqlquery"
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

// EnsureRelationArtifactsFromSchema creates missing D1 relation tables and foreign keys.
// Each relation is applied once from its forward connection, in the same argument order as addRelation.
func (d *Driver) EnsureRelationArtifactsFromSchema(ctx context.Context, modelsList []*models.ModelType) error {
	if d == nil || len(modelsList) == 0 {
		return nil
	}
	byName := make(map[string]*models.ModelType, len(modelsList))
	for _, m := range modelsList {
		if m == nil {
			continue
		}
		name := strings.TrimSpace(utility.PhysicalSQLTableName(m.Name))
		if name != "" {
			byName[name] = m
		}
	}
	seen := make(map[string]struct{})
	for _, fromModel := range modelsList {
		if fromModel == nil {
			continue
		}
		fromName := strings.TrimSpace(utility.PhysicalSQLTableName(fromModel.Name))
		for _, conn := range fromModel.Connections {
			if conn == nil || strings.TrimSpace(conn.Type) != "forward" {
				continue
			}
			toKey := strings.TrimSpace(utility.PhysicalSQLTableName(conn.Model))
			toModel := byName[toKey]
			if toModel == nil {
				continue
			}
			rev := d1ReverseConnection(toModel, fromName, conn)
			if rev == nil || strings.TrimSpace(rev.Type) != "backward" {
				continue
			}
			a, b := fromName, toKey
			if a > b {
				a, b = b, a
			}
			dedup := a + "\x00" + b + "\x00" + conn.KnownAs
			if _, ok := seen[dedup]; ok {
				continue
			}
			seen[dedup] = struct{}{}
			if err := d.AddRelationFields(ctx, conn, rev); err != nil {
				return err
			}
		}
	}
	return nil
}

func d1ReverseConnection(toModel *models.ModelType, fromPhysicalName string, conn *models.ConnectionType) *models.ConnectionType {
	if toModel == nil || conn == nil {
		return nil
	}
	for _, back := range toModel.Connections {
		if back == nil || back.KnownAs != conn.KnownAs {
			continue
		}
		if strings.TrimSpace(utility.PhysicalSQLTableName(back.Model)) == fromPhysicalName {
			return back
		}
	}
	return nil
}
