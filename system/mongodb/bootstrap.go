package mongodb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/open_driver/system/bootstrapmeta"
	"github.com/apito-io/open_driver/system/driverdefaults"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func mongoIsDup(err error) bool {
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		return ce.HasErrorCode(11000)
	}
	return false
}

// EnsureSystemBootstrap creates default admin, then default team/org/project and junction rows when needed.
func (m *SystemMongoDriver) EnsureSystemBootstrap(ctx context.Context) error {
	if err := m.ensureBootstrapAdmin(ctx); err != nil {
		return err
	}
	if err := bootstrapmeta.RepairSuperAdminEmails(ctx, m.GetSystemUserByEmail, m.UpdateSystemUser); err != nil {
		return err
	}
	if err := m.ensureBootstrapOrgTeamProject(ctx); err != nil {
		return err
	}
	return m.ensureStarterProjectDriver(ctx)
}

func mongoProjectNeedsDefaultDriver(p *models.Project) bool {
	if p == nil {
		return false
	}
	if p.Driver == nil {
		return true
	}
	d := p.Driver
	if strings.TrimSpace(d.Engine) != "" {
		return false
	}
	return strings.TrimSpace(d.Database) == "" && strings.TrimSpace(d.Host) == "" && strings.TrimSpace(d.File) == ""
}

// ensureStarterProjectDriver sets OSS starter driver defaults when driver is missing (idempotent).
func (m *SystemMongoDriver) ensureStarterProjectDriver(ctx context.Context) error {
	if m.Conf == nil {
		return nil
	}
	proj, err := m.GetProject(ctx, bootstrapmeta.StarterProjectID)
	if err != nil {
		return nil
	}
	if !mongoProjectNeedsDefaultDriver(proj) {
		return nil
	}
	proj.Driver = driverdefaults.OSSBootstrapProjectDriver(m.Conf, proj.ID)
	if proj.Driver == nil {
		return nil
	}
	return m.UpdateProject(ctx, proj, true)
}

func (m *SystemMongoDriver) ensureBootstrapAdmin(ctx context.Context) error {
	email := bootstrapmeta.BootstrapAdminEmail()
	u, err := m.GetSystemUserByEmail(ctx, email)
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

	if _, err := m.CreateSystemUser(ctx, user); err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}

	bootstrapmeta.LogDefaultAdminCredentials(email, plain)
	log.Println("system bootstrap: default admin user created (Mongo)")
	return nil
}

func (m *SystemMongoDriver) ensureBootstrapOrgTeamProject(ctx context.Context) error {
	user, err := m.GetSystemUserByEmail(ctx, bootstrapmeta.BootstrapAdminEmail())
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: load admin for starter project: %w", err)
	}
	if user == nil {
		return nil
	}

	starterID := bootstrapmeta.ExistingStarterProjectID(ctx, m.GetProject)
	if starterID == bootstrapmeta.PreviousStarterProjectID {
		log.Println("system bootstrap: starter project linked (Mongo)")
		return nil
	}
	if _, err := m.GetProject(ctx, starterID); err != nil {
		proj := &models.Project{
			ID:          starterID,
			Name:        bootstrapmeta.StarterProjectName,
			Description: "Starter project",
		}
		if _, err := m.CreateProject(ctx, user.ID, proj); err != nil {
			return fmt.Errorf("bootstrap create starter project: %w", err)
		}
	}
	log.Println("system bootstrap: starter project linked (Mongo)")
	return nil
}
