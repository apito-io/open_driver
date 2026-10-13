package cloudflared1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/open_driver/cfd1"
	"github.com/apito-io/open_driver/system/bootstrapmeta"
)

func (d *Driver) RunMigration(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			user_id TEXT,
			driver TEXT,
			schema TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS system_users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE,
			password TEXT,
			data TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS project_schemas (
			project_id TEXT PRIMARY KEY,
			schema TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS model_types (
			project_id TEXT NOT NULL,
			name TEXT NOT NULL,
			data TEXT,
			PRIMARY KEY (project_id, name)
		)`,
	}
	for _, sql := range stmts {
		if _, err := d.DB.Exec(ctx, sql); err != nil {
			return err
		}
	}
	_, _ = d.DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name)`)
	_, _ = d.DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_apito_kv_expires ON apito_kv(expires_at)`)
	return nil
}

func (d *Driver) EnsureSystemBootstrap(ctx context.Context) error {
	if err := d.ensureBootstrapAdmin(ctx); err != nil {
		return err
	}
	return bootstrapmeta.RepairSuperAdminEmails(ctx, d.GetSystemUserByEmail, d.UpdateSystemUser)
}

func (d *Driver) PersistProjectModelTypes(ctx context.Context, projectID string, schemaModels []*models.ModelType) error {
	if projectID == "" {
		return nil
	}
	for _, m := range schemaModels {
		if err := d.UpsertModelType(ctx, projectID, m); err != nil {
			return err
		}
	}
	return nil
}
func (d *Driver) DeleteModelType(ctx context.Context, projectID, modelName string) error {
	if projectID == "" || modelName == "" {
		return nil
	}
	_, err := d.DB.Exec(ctx, `DELETE FROM model_types WHERE project_id = ? AND name = ?`, projectID, modelName)
	return err
}
func (d *Driver) GetProjects(ctx context.Context, keys []string) ([]*models.Project, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	rows, err := d.FindUserProjectsWithRoles(ctx, userId)
	if err != nil {
		return nil, err
	}
	out := make([]*models.Project, 0, len(rows))
	for _, r := range rows {
		if r != nil && r.Project != nil {
			out = append(out, r.Project)
		}
	}
	return out, nil
}

func (d *Driver) SaveProjectAuthenticationSettings(ctx context.Context, projectID string, auth *models.AuthenticationSettings) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) SaveProjectStorageSettings(ctx context.Context, projectID string, storage *models.StorageSettings) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetUser(ctx context.Context, projectID, userID string) (*models.User, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) UpdateUser(ctx context.Context, user *models.User) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DeleteUser(ctx context.Context, projectID, userID string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) SearchProjectUsers(ctx context.Context, projectID, q string, limit, offset int) ([]*models.User, int, error) {
	return nil, 0, cfd1.ErrNotImplemented
}
func (d *Driver) CountProjectUsersByRole(ctx context.Context, projectID string) (map[string]int, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetUserByUsername(ctx context.Context, projectID, username string) (*models.User, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) ListUsersByEmail(ctx context.Context, projectID, email string) ([]*models.User, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) ListUsersByPhone(ctx context.Context, projectID, phone string) ([]*models.User, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) ListUsersByGoogleSub(ctx context.Context, projectID, googleSub string) ([]*models.User, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, userId string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) DeleteSystemUser(ctx context.Context, id string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	if strings.TrimSpace(projectId) == "" {
		return nil, fmt.Errorf("project id is empty")
	}
	if strings.TrimSpace(userId) == "" {
		return nil, fmt.Errorf("user id is empty")
	}
	row, err := d.DB.QueryRow(ctx, `SELECT id, name, user_id, driver, schema FROM projects WHERE id = ? LIMIT 1`, projectId)
	if err != nil || row == nil {
		return nil, err
	}
	owner := asString(row["user_id"])
	if owner != "" && owner != userId {
		return nil, fmt.Errorf("user does not have access to project")
	}
	p := &models.Project{ID: asString(row["id"]), Name: asString(row["name"])}
	if raw := asString(row["driver"]); raw != "" {
		_ = json.Unmarshal([]byte(raw), &p.Driver)
	}
	if raw := asString(row["schema"]); raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &p.Schema)
	}
	user, err := d.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}
	return &models.ProjectWithRoles{
		Project:     p,
		User:        user,
		Role:        "admin",
		Permissions: []string{"*"},
	}, nil
}
func (d *Driver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	return cfd1.ErrNotImplemented
}
