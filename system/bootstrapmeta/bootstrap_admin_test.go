package bootstrapmeta

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestBootstrapAdminEmail_DefaultAndEnv(t *testing.T) {
	t.Setenv(bootstrapAdminEmailEnv, "")
	require.Equal(t, AdminEmail, BootstrapAdminEmail())

	t.Setenv(bootstrapAdminEmailEnv, "  fahim@apito.io ")
	require.Equal(t, "fahim@apito.io", BootstrapAdminEmail())
}

func TestResolveBootstrapAdminPassword_ExplicitAndRandom(t *testing.T) {
	t.Setenv(bootstrapAdminPasswordEnv, "")
	t.Setenv(bootstrapAdminRandomEnv, "")
	plain, generated := ResolveBootstrapAdminPassword()
	require.Equal(t, DefaultAdminPass, plain)
	require.False(t, generated)

	t.Setenv(bootstrapAdminPasswordEnv, "explicit-secret")
	plain, generated = ResolveBootstrapAdminPassword()
	require.Equal(t, "explicit-secret", plain)
	require.False(t, generated)

	t.Setenv(bootstrapAdminPasswordEnv, "")
	t.Setenv(bootstrapAdminRandomEnv, "true")
	plain, generated = ResolveBootstrapAdminPassword()
	require.True(t, generated)
	require.Len(t, plain, bootstrapPasswordLen)
	require.NotEqual(t, DefaultAdminPass, plain)
}

func TestHashBootstrapAdminSecret_RoundTrip(t *testing.T) {
	t.Setenv(bootstrapAdminPasswordEnv, "round-trip-secret")
	t.Setenv(bootstrapAdminRandomEnv, "")
	hash, plain, err := HashBootstrapAdminSecret()
	require.NoError(t, err)
	require.Equal(t, "round-trip-secret", plain)
	require.NoError(t, bcrypt.CompareHashAndPassword(hash, []byte(plain)))
}

func TestNewBootstrapAdminUser_UsesEnvEmail(t *testing.T) {
	t.Setenv(bootstrapAdminEmailEnv, "ops@example.com")
	u := NewBootstrapAdminUser("id", "user", "now", "hash")
	require.Equal(t, "ops@example.com", u.Email)
	require.True(t, u.IsSuperAdmin)
}
