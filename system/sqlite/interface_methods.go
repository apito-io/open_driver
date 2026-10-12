package sqlite

import (
	"context"

	"github.com/apito-io/open_driver/system/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// FindUserProjectsWithRoles implements interfaces.ApitoSystemDB.
func (d *Driver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	return d.GetProjectWithRolesAndPermission(ctx, userId)
}

// GetTeams implements interfaces.ApitoSystemDB.
func (d *Driver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	return d.FindUserTeams(ctx, userId)
}

// GetTeamsMembers implements interfaces.ApitoSystemDB.
func (d *Driver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	return d.ListTeams(ctx, projectId)
}

// SearchResource implements interfaces.ApitoSystemDB.
func (d *Driver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	return &models.SearchResponse[any]{
		Results: []*any{},
	}, nil
}

// FindOrganizationAdmin implements interfaces.ApitoSystemDB.
func (d *Driver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	return sqlcommon.RetiredFindOrganizationAdmin()
}

// SaveAuditLog implements interfaces.ApitoSystemDB.
func (d *Driver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	if auditLog.ID == "" {
		auditLog.ID = utility.NewID()
	}

	_, err := d.ORM.NewInsert().
		Model(auditLog).
		Exec(ctx)

	return err
}

// SearchAuditLogs implements interfaces.ApitoSystemDB.
func (d *Driver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	var logs []*models.AuditLogs

	query := d.ORM.NewSelect().Model(&logs)

	if param.UserID != "" {
		query = query.Where("user_id = ?", param.UserID)
	}
	if param.ProjectID != "" {
		query = query.Where("project_id = ?", param.ProjectID)
	}

	err := query.Order("created_at DESC").Limit(100).Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.AuditLogs]{
		Results: logs,
	}, nil
}

// GetOrganizations implements interfaces.ApitoSystemDB.
func (d *Driver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	return sqlcommon.RetiredGetOrganizations()
}
