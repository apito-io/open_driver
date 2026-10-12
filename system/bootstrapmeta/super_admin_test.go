package bootstrapmeta

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestSuperAdminEmails_DefaultAndEnv(t *testing.T) {
	t.Setenv(superAdminEmailsEnv, "")
	require.Equal(t, []string{AdminEmail}, SuperAdminEmails())

	t.Setenv(superAdminEmailsEnv, "ops@apito.io, admin@apito.io")
	got := SuperAdminEmails()
	require.Equal(t, AdminEmail, got[0])
	require.Contains(t, got, "ops@apito.io")
}

func TestRepairSuperAdminEmails_SetsFlag(t *testing.T) {
	users := map[string]*models.SystemUser{
		AdminEmail: {ID: "1", Email: AdminEmail, IsSuperAdmin: false},
	}
	get := func(_ context.Context, email string) (*models.SystemUser, error) {
		u := users[email]
		if u == nil {
			return nil, nil
		}
		cp := *u
		return &cp, nil
	}
	var updated bool
	update := func(_ context.Context, user *models.SystemUser, _ bool) error {
		updated = true
		users[user.Email] = user
		return nil
	}
	require.NoError(t, RepairSuperAdminEmails(context.Background(), get, update))
	require.True(t, updated)
	require.True(t, users[AdminEmail].IsSuperAdmin)
}

func TestNewBootstrapAdminUser_IsSuperAdmin(t *testing.T) {
	u := NewBootstrapAdminUser("id", "user", "now", "hash")
	require.True(t, u.IsSuperAdmin)
	require.True(t, u.IsActive)
	require.Equal(t, BootstrapAdminEmail(), u.Email)
}
