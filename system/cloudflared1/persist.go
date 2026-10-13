package cloudflared1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
)

func (d *Driver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {
	if d == nil || d.DB == nil || project == nil {
		return nil, cfd1Err("create project: invalid state")
	}
	project.ID = strings.TrimSpace(project.ID)
	if project.ID == "" {
		return nil, cfd1Err("create project: id required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	driverJSON, _ := json.Marshal(project.Driver)
	_, err := d.DB.Exec(ctx, `INSERT INTO projects (id, name, user_id, driver, schema, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		project.ID, project.Name, userId, string(driverJSON), "{}", now, now)
	if err != nil {
		return nil, err
	}
	project.CreatedAt = now
	project.UpdatedAt = now
	return project, nil
}

func (d *Driver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	row, err := d.DB.QueryRow(ctx, `SELECT id, name, user_id, driver, schema, created_at, updated_at FROM projects WHERE id = ?`, strings.TrimSpace(id))
	if err != nil || row == nil {
		return nil, err
	}
	p := &models.Project{
		ID:        asString(row["id"]),
		Name:      asString(row["name"]),
		CreatedAt: asString(row["created_at"]),
		UpdatedAt: asString(row["updated_at"]),
	}
	if raw := asString(row["driver"]); raw != "" {
		_ = json.Unmarshal([]byte(raw), &p.Driver)
	}
	if raw := asString(row["schema"]); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &p.Schema)
	}
	return p, nil
}

// CheckProjectName reports ae.ErrProjectNameTaken when name is already used in SYSTEM_D1.
func (d *Driver) CheckProjectName(ctx context.Context, name string) error {
	if d == nil || d.DB == nil {
		return cfd1Err("check project name: invalid state")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name required")
	}
	row, err := d.DB.QueryRow(ctx, `SELECT id FROM projects WHERE name = ? LIMIT 1`, name)
	if err != nil {
		return err
	}
	if row != nil && strings.TrimSpace(asString(row["id"])) != "" {
		return fmt.Errorf("%w", ae.ErrProjectNameTaken)
	}
	return nil
}

func (d *Driver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	rows, err := d.DB.Query(ctx, `SELECT id, name, user_id, driver, created_at, updated_at FROM projects WHERE user_id = ?`, userId)
	if err != nil {
		return nil, err
	}
	out := make([]*models.ProjectWithRoles, 0, len(rows))
	for _, row := range rows {
		p := &models.Project{
			ID:        asString(row["id"]),
			Name:      asString(row["name"]),
			CreatedAt: asString(row["created_at"]),
			UpdatedAt: asString(row["updated_at"]),
		}
		if raw := asString(row["driver"]); raw != "" {
			_ = json.Unmarshal([]byte(raw), &p.Driver)
		}
		out = append(out, &models.ProjectWithRoles{Project: p, Role: "admin"})
	}
	return out, nil
}

func (d *Driver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	row, err := d.DB.QueryRow(ctx, `SELECT id, email, password, data FROM system_users WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	return rowToSystemUser(row)
}

// DeleteProjectFromSystem removes catalog rows for a project from the system database.
func (d *Driver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	if d == nil || d.DB == nil {
		return cfd1Err("delete project from system: invalid state")
	}
	projectId = strings.TrimSpace(projectId)
	if projectId == "" {
		return fmt.Errorf("project id required")
	}
	stmts := []string{
		`DELETE FROM model_types WHERE project_id = ?`,
		`DELETE FROM project_schemas WHERE project_id = ?`,
		`DELETE FROM projects WHERE id = ?`,
	}
	for _, sql := range stmts {
		if _, err := d.DB.Exec(ctx, sql, projectId); err != nil {
			return err
		}
	}
	return nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return strings.Trim(string(b), `"`)
	}
}

func cfd1Err(msg string) error { return cfd1ErrType{msg} }

type cfd1ErrType struct{ msg string }

func (e cfd1ErrType) Error() string { return "cloudflared1: " + e.msg }
