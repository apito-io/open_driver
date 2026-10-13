package bbolt

import (
	"context"
	"fmt"
	"log"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/open_driver/system/bootstrapmeta"
)

// EnsureSystemBootstrap creates default admin, then default team/org/project via junction collections.
func (d *BBoltSystemDriver) EnsureSystemBootstrap(ctx context.Context) error {
	if err := d.ensureBootstrapAdmin(ctx); err != nil {
		return err
	}
	if err := bootstrapmeta.RepairSuperAdminEmails(ctx, d.GetSystemUserByEmail, d.UpdateSystemUser); err != nil {
		return err
	}
	return d.ensureBootstrapOrgTeamProject(ctx)
}

func (d *BBoltSystemDriver) ensureBootstrapAdmin(ctx context.Context) error {
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
	log.Println("system bootstrap: default admin user created (bBolt)")
	return nil
}

func (d *BBoltSystemDriver) ensureBootstrapOrgTeamProject(ctx context.Context) error {
	user, err := d.GetSystemUserByEmail(ctx, bootstrapmeta.BootstrapAdminEmail())
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: load admin for starter project: %w", err)
	}
	if user == nil {
		return nil
	}

	starterID := bootstrapmeta.ExistingStarterProjectID(ctx, d.GetProject)
	if starterID == bootstrapmeta.PreviousStarterProjectID {
		log.Println("system bootstrap: starter project linked (bBolt)")
		return nil
	}
	proj := &models.Project{
		ID:          starterID,
		Name:        bootstrapmeta.StarterProjectName,
		Description: "Starter project",
	}
	if _, err := d.GetProject(ctx, starterID); err != nil {
		if _, err := d.CreateProject(ctx, user.ID, proj); err != nil {
			return fmt.Errorf("bootstrap create starter project: %w", err)
		}
	}
	log.Println("system bootstrap: starter project linked (bBolt)")
	return nil
}
