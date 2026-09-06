package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.com/apito.io/open_driver/system/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// GetProjects retrieves multiple projects by their IDs using SQL
func (d *Driver) GetProjects(ctx context.Context, keys []string) ([]*models.Project, error) {
	var projects []*models.Project
	err := d.ORM.NewSelect().
		Model(&projects).
		Relation("Driver").
		Relation("Settings").
		Relation("Schema.Models").
		Relation("Schema.Functions").
		Where("id IN (?)", bun.In(keys)).
		Scan(ctx)

	return projects, err
}

// AddATeamMemberToProject adds a team member to a project using SQL
func (d *Driver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	return sqlcommon.UpsertUserProjectGrant(ctx, d.ORM, req)
}

// GetSystemUsers retrieves multiple system users by their IDs using SQL
func (d *Driver) GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser
	err := d.ORM.NewSelect().
		Model(&users).
		Where("id IN (?)", bun.In(keys)).
		Scan(ctx)

	return users, err
}

// FindUserOrganizations is retired; named orgs are gone.
func (d *Driver) FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error) {
	return sqlcommon.RetiredFindUserOrganizations()
}

// CreateOrganization is retired; it does not persist.
func (d *Driver) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	return sqlcommon.RetiredCreateOrganization(org)
}

// AssignTeamToOrganization is retired.
func (d *Driver) AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error {
	return sqlcommon.RetiredNoop()
}

// RemoveATeamFromOrganization is retired.
func (d *Driver) RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error {
	return sqlcommon.RetiredNoop()
}

// AssignProjectToOrganization is retired.
func (d *Driver) AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error {
	return sqlcommon.RetiredNoop()
}

// RemoveProjectFromOrganization is retired.
func (d *Driver) RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error {
	return sqlcommon.RetiredNoop()
}

// GetProjectTeams is retired.
func (d *Driver) GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error) {
	return sqlcommon.RetiredGetProjectTeams()
}

// FindUserTeams is retired.
func (d *Driver) FindUserTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	return []*models.Team{}, nil
}

// CreateTeam is retired; it does not persist.
func (d *Driver) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	return sqlcommon.RetiredCreateTeam(team)
}

// FindUserProjects retrieves all projects for a given user using SQL
func (d *Driver) FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	var projects []*models.Project

	err := d.ORM.NewSelect().
		Model(&projects).
		Relation("Driver").
		Join("JOIN user_projects up ON up.project_id = project.id").
		Where("up.user_id = ?", userId).
		Where(sqlcommon.SQLUserProjectAccessGrantedAliased).
		Scan(ctx)

	return projects, err
}

// CheckProjectWithRoles checks if a user belongs to a project and returns roles/permissions using SQL
func (d *Driver) CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	if projectId == "" {
		return nil, fmt.Errorf("project id is empty")
	}

	// Get the project
	project := &models.Project{}
	err := d.ORM.NewSelect().
		Model(project).
		Relation("Driver").
		Relation("Settings").
		Relation("Schema.Models").
		Relation("Schema.Functions").
		Where("project.id = ?", projectId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Get the user
	user := &models.SystemUser{}
	err = d.ORM.NewSelect().
		Model(user).
		Where("id = ?", userId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Get user-project relation with role and permissions
	var relation map[string]interface{}
	err = d.ORM.NewSelect().
		Model(&relation).
		TableExpr("user_projects").
		Where("user_id = ? AND project_id = ?", userId, projectId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	role, _ := relation["role"].(string)
	permissions, _ := relation["permissions"].(string)
	inviteStatus := sqlcommon.MapString(relation, "invite_status")
	inviteExpires := sqlcommon.MapString(relation, "invite_expires_at")
	if err := models.ErrIfInviteBlocksAccess(inviteStatus, inviteExpires); err != nil {
		return nil, err
	}

	var permList []string
	if permissions != "" {
		json.Unmarshal([]byte(permissions), &permList)
	}

	models.ApplyProjectBillingLock(user, project)

	return &models.ProjectWithRoles{
		User:            user,
		Project:         project,
		Role:            role,
		Permissions:     permList,
		InviteStatus:    inviteStatus,
		InviteExpiresAt: inviteExpires,
	}, nil
}
