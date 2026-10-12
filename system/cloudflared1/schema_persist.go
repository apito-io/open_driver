package cloudflared1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
)

func (d *Driver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	if d == nil || d.DB == nil || project == nil || strings.TrimSpace(project.ID) == "" {
		return fmt.Errorf("cloudflared1: invalid update project")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	project.UpdatedAt = now

	schemaJSON := "{}"
	if project.Schema != nil {
		raw, err := json.Marshal(project.Schema)
		if err != nil {
			return err
		}
		schemaJSON = string(raw)
	}
	driverJSON := ""
	if project.Driver != nil {
		raw, _ := json.Marshal(project.Driver)
		driverJSON = string(raw)
	}

	if replace {
		_, err := d.DB.Exec(ctx,
			`UPDATE projects SET name = ?, driver = ?, schema = ?, updated_at = ? WHERE id = ?`,
			project.Name, driverJSON, schemaJSON, now, project.ID)
		if err != nil {
			return err
		}
	} else {
		_, err := d.DB.Exec(ctx,
			`UPDATE projects SET schema = ?, updated_at = ? WHERE id = ?`,
			schemaJSON, now, project.ID)
		if err != nil {
			return err
		}
	}

	if project.Schema != nil {
		if err := d.syncProjectSchemaModels(ctx, project); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) syncProjectSchemaModels(ctx context.Context, project *models.Project) error {
	if project == nil || project.Schema == nil {
		return nil
	}
	_, _ = d.DB.Exec(ctx,
		`INSERT INTO project_schemas (project_id, schema) VALUES (?, ?) ON CONFLICT(project_id) DO UPDATE SET schema = excluded.schema`,
		project.ID, "{}")
	for _, m := range project.Schema.Models {
		if m == nil || strings.TrimSpace(m.Name) == "" {
			continue
		}
		if err := d.UpsertModelType(ctx, project.ID, m); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) UpsertModelType(ctx context.Context, projectID string, m *models.ModelType) error {
	if projectID == "" || m == nil || strings.TrimSpace(m.Name) == "" {
		return nil
	}
	row := *m
	row.ProjectID = projectID
	raw, err := json.Marshal(&row)
	if err != nil {
		return err
	}
	_, err = d.DB.Exec(ctx,
		`INSERT INTO model_types (project_id, name, data) VALUES (?, ?, ?)
		 ON CONFLICT(project_id, name) DO UPDATE SET data = excluded.data`,
		projectID, row.Name, string(raw))
	return err
}

func (d *Driver) TouchProjectUpdatedAt(ctx context.Context, projectID string) error {
	if projectID == "" {
		return nil
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.DB.Exec(ctx, `UPDATE projects SET updated_at = ? WHERE id = ?`, ts, projectID)
	return err
}
