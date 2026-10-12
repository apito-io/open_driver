package bootstrapmeta

import (
	"context"
	"os"
	"strings"

	"github.com/apito-io/engine/models"
)

const superAdminEmailsEnv = "SUPER_ADMIN_EMAILS"

// SuperAdminEmails returns the platform-operator email list. Default is
// BootstrapAdminEmail(). SUPER_ADMIN_EMAILS is a comma-separated extra list.
func SuperAdminEmails() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	add := func(raw string) {
		email := strings.ToLower(strings.TrimSpace(raw))
		if email == "" {
			return
		}
		if _, ok := seen[email]; ok {
			return
		}
		seen[email] = struct{}{}
		out = append(out, email)
	}
	add(BootstrapAdminEmail())
	for _, part := range strings.Split(os.Getenv(superAdminEmailsEnv), ",") {
		add(part)
	}
	return out
}

// NewBootstrapAdminUser builds the first-run operator row with the platform flag.
func NewBootstrapAdminUser(id, username, now, passwordHash string) *models.SystemUser {
	if username == "" {
		username = id
	}
	return &models.SystemUser{
		XKey:                      id,
		ID:                        id,
		Email:                     BootstrapAdminEmail(),
		Username:                  username,
		FirstName:                 AdminFirstName,
		LastName:                  AdminLastName,
		Secret:                    passwordHash,
		IsAdmin:                   true,
		IsSuperAdmin:              true,
		IsActive:                  true,
		AdministrativePermissions: []string{"all"},
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
}

// SuperAdminGetter loads a system user by email.
type SuperAdminGetter func(ctx context.Context, email string) (*models.SystemUser, error)

// SuperAdminUpdater persists a system user row.
type SuperAdminUpdater func(ctx context.Context, user *models.SystemUser, replace bool) error

// RepairSuperAdminEmails sets is_super_admin=true on existing rows whose email is in
// SuperAdminEmails. Missing emails are skipped (no auto-create except bootstrap admin).
func RepairSuperAdminEmails(ctx context.Context, getByEmail SuperAdminGetter, update SuperAdminUpdater) error {
	if getByEmail == nil || update == nil {
		return nil
	}
	for _, email := range SuperAdminEmails() {
		u, err := getByEmail(ctx, email)
		if err != nil && !IsUserLookupMiss(err) {
			return err
		}
		if u == nil {
			continue
		}
		if u.IsSuperAdmin {
			continue
		}
		u.IsSuperAdmin = true
		if err := update(ctx, u, false); err != nil {
			return err
		}
	}
	return nil
}
