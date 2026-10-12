package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/apito-io/open_driver/system/sqlcommon"
	"log"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/open_driver/system/bootstrapmeta"
)

func (d *Driver) ensureBootstrapAdmin(ctx context.Context) error {
	email := bootstrapmeta.BootstrapAdminEmail()
	u, err := d.GetSystemUserByEmail(ctx, email)
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: check admin user: %w", err)
	}
	if u != nil {
		return nil
	}

	hash, plain, err := bootstrapmeta.HashBootstrapAdminSecret()
	if err != nil {
		return fmt.Errorf("bootstrap: hash password: %w", err)
	}

	id := utility.NewID()
	now := utility.GetCurrentTime()
	user := &models.SystemUser{
		XKey:                      id,
		ID:                        id,
		Email:                     email,
		Username:                  utility.NewID(),
		FirstName:                 bootstrapmeta.AdminFirstName,
		LastName:                  bootstrapmeta.AdminLastName,
		Secret:                    string(hash),
		IsAdmin:                   true,
		IsSuperAdmin:              true,
		IsActive:                  true,
		AdministrativePermissions: []string{"all"},
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	if _, err := d.CreateSystemUser(ctx, user); err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}

	bootstrapmeta.LogDefaultAdminCredentials(email, plain)
	log.Println("system bootstrap: default admin user created (SQL)")
	return nil
}

// EnsureBootstrapAdmin creates the default admin user if missing (idempotent).
// Exported for pro layered system bootstrap (e.g. libsql starter provisioned outside OSS driver defaults).
func (d *Driver) EnsureBootstrapAdmin(ctx context.Context) error {
	return d.ensureBootstrapAdmin(ctx)
}

func (d *Driver) ensureBootstrapOrgTeamProject(ctx context.Context) error {
	return d.EnsureBootstrapOrgTeamProjectWithStarterCreate(ctx, func(ctx context.Context, userID string, proj *models.Project) error {
		_, err := d.CreateProject(ctx, userID, proj)
		return err
	})
}

// EnsureBootstrapOrgTeamProjectWithStarterCreate seeds the starter project only.
// Named Team/Organization graph is retired; grants live on user_projects.
func (d *Driver) EnsureBootstrapOrgTeamProjectWithStarterCreate(ctx context.Context, fn bootstrapmeta.CreateStarterProjectFn) error {
	user, err := d.GetSystemUserByEmail(ctx, bootstrapmeta.BootstrapAdminEmail())
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: load admin for starter project: %w", err)
	}
	if user == nil {
		return nil
	}

	starterID := bootstrapmeta.ExistingStarterProjectID(ctx, d.GetProject)
	if starterID == bootstrapmeta.StarterProjectID {
		if _, err := d.GetProject(ctx, starterID); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("bootstrap GetProject: %w", err)
			}
			if fn == nil {
				return fmt.Errorf("bootstrap: starter project create fn is nil")
			}
			proj := &models.Project{
				ID:          starterID,
				Name:        bootstrapmeta.StarterProjectName,
				Description: "Starter project",
			}
			if err := fn(ctx, user.ID, proj); err != nil {
				return fmt.Errorf("bootstrap create starter project: %w", err)
			}
		}
	}

	if err := d.ensureProjectSchemaRow(ctx, starterID); err != nil {
		return err
	}
	if err := d.ensureProjectSettingsRow(ctx, starterID); err != nil {
		return err
	}
	log.Println("system bootstrap: starter project linked (SQL)")
	return nil
}

func (d *Driver) repairProjectSchemaIntegrity(ctx context.Context) error {
	if _, err := d.ORM.NewDelete().
		Model((*models.ModelType)(nil)).
		Where("project_id = '' OR project_id IS NULL").
		Exec(ctx); err != nil {
		return fmt.Errorf("repair model_types orphans: %w", err)
	}
	if _, err := d.ORM.NewDelete().
		Model((*models.ApitoFunction)(nil)).
		Where("project_id = '' OR project_id IS NULL").
		Exec(ctx); err != nil {
		return fmt.Errorf("repair apito_functions orphans: %w", err)
	}

	var projectIDs []string
	if err := d.ORM.NewSelect().
		Model((*models.Project)(nil)).
		Column("id").
		Scan(ctx, &projectIDs); err != nil {
		return fmt.Errorf("repair project_schemas list projects: %w", err)
	}
	for _, projectID := range projectIDs {
		if projectID == "" {
			continue
		}
		if err := d.ensureProjectSchemaRow(ctx, projectID); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) ensureProjectSchemaRow(ctx context.Context, projectID string) error {
	exists, err := d.ORM.NewSelect().
		Model((*models.ProjectSchema)(nil)).
		Where("project_id = ?", projectID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: check project_schema: %w", err)
	}
	if exists {
		return nil
	}
	row := &models.ProjectSchema{ProjectID: projectID}
	if _, err := d.ORM.NewInsert().Model(row).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap project_schemas: %w", err)
	}
	return nil
}

// EnsureBootstrapProjectSettingsRow ensures a project_settings row exists (idempotent).
// Exported for pro layered bootstrap that skips open-core ensureStarterProjectDriver.
func (d *Driver) EnsureBootstrapProjectSettingsRow(ctx context.Context, projectID string) error {
	return d.ensureProjectSettingsRow(ctx, projectID)
}

func (d *Driver) ensureProjectSettingsRow(ctx context.Context, projectID string) error {
	exists, err := d.ORM.NewSelect().
		Model((*models.ProjectSettings)(nil)).
		Where("project_id = ?", projectID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: check project_settings: %w", err)
	}
	if exists {
		return nil
	}
	row := &models.ProjectSettings{
		ProjectID: projectID,
		Locals:    []string{"en"},
	}
	if _, err := d.ORM.NewInsert().Model(row).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap project_settings: %w", err)
	}
	return nil
}

// ensureStarterProjectDriver backfills driver_credentials for the starter project when config is present
// (e.g. user already had default org/team and ensureBootstrapOrgTeamProject returned early).
func (d *Driver) ensureStarterProjectDriver(ctx context.Context) error {
	starterID := bootstrapmeta.ExistingStarterProjectID(ctx, d.GetProject)
	if err := d.ensureProjectDriverCredentials(ctx, starterID); err != nil {
		return fmt.Errorf("bootstrap starter project driver: %w", err)
	}
	if err := d.ensureProjectSettingsRow(ctx, starterID); err != nil {
		return fmt.Errorf("bootstrap starter project settings: %w", err)
	}
	return nil
}
