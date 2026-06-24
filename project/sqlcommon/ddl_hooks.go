package sqlcommon

import (
	"context"

	"github.com/apito-io/engine/models"
)

// InvokeDDLPostCreateHook runs Config.DDLPostCreateHook after open-core creates a model table.
func InvokeDDLPostCreateHook(ctx context.Context, cfg *models.Config, project *models.Project, model *models.ModelType, dbHandle interface{}) error {
	if cfg == nil || cfg.DDLPostCreateHook == nil || project == nil || model == nil {
		return nil
	}
	return cfg.DDLPostCreateHook(ctx, project, model, dbHandle)
}
