package cloudflared1

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"gitlab.com/apito.io/open_driver/project/sqlcommon"
)

func (d *Driver) CheckTableOrCollectionExists(ctx context.Context, param *models.CommonSystemParams) (bool, error) {
	if d == nil || d.DB == nil || param == nil || param.Model == nil {
		return false, fmt.Errorf("cloudflared1: invalid check table params")
	}
	table := strings.ReplaceAll(utility.PhysicalSQLTableName(param.Model.Name), "'", "''")
	row, err := d.DB.QueryRow(ctx,
		`SELECT COUNT(*) AS c FROM sqlite_master WHERE type='table' AND name = ?`, table)
	if err != nil {
		return false, err
	}
	if row == nil {
		return false, nil
	}
	switch v := row["c"].(type) {
	case int64:
		return v > 0, nil
	case float64:
		return v > 0, nil
	case int:
		return v > 0, nil
	case string:
		return v != "0" && v != "", nil
	default:
		return false, nil
	}
}

func (d *Driver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {
	if project == nil || model == nil {
		return nil, fmt.Errorf("cloudflared1: project and model required")
	}
	if project.Schema == nil {
		project.Schema = &models.ProjectSchema{Models: []*models.ModelType{model}}
	} else {
		for _, ct := range project.Schema.Models {
			if ct != nil && ct.Name == model.Name {
				return nil, errors.New("model Already Defined")
			}
		}
		project.Schema.Models = append(project.Schema.Models, model)
	}

	param := &models.CommonSystemParams{Model: model}
	if project.ID != "" {
		param.ProjectID = project.ID
	}
	if err := d.CreateTableOrCollection(ctx, param, nil); err != nil {
		return nil, err
	}
	if err := sqlcommon.InvokeDDLPostCreateHook(ctx, d.Conf, project, model, d); err != nil {
		return nil, err
	}
	return project.Schema, nil
}
