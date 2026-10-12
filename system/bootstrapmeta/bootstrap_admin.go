package bootstrapmeta

import (
	"crypto/rand"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	bootstrapAdminEmailEnv    = "BOOTSTRAP_ADMIN_EMAIL"
	bootstrapAdminPasswordEnv = "BOOTSTRAP_ADMIN_PASSWORD"
	bootstrapAdminRandomEnv   = "BOOTSTRAP_ADMIN_RANDOM_PASSWORD"
	bootstrapPasswordAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789-_"
	bootstrapPasswordLen      = 20
)

// BootstrapAdminEmail is the first-run operator email.
// BOOTSTRAP_ADMIN_EMAIL overrides; default is AdminEmail (admin@apito.io).
func BootstrapAdminEmail() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv(bootstrapAdminEmailEnv))); v != "" {
		return v
	}
	return AdminEmail
}

// ResolveBootstrapAdminPassword returns the plaintext used only on first create.
// BOOTSTRAP_ADMIN_PASSWORD wins. Else BOOTSTRAP_ADMIN_RANDOM_PASSWORD=true
// generates a one-shot secret (logged once). Else DefaultAdminPass (local/tests).
func ResolveBootstrapAdminPassword() (plain string, generated bool) {
	if v := strings.TrimSpace(os.Getenv(bootstrapAdminPasswordEnv)); v != "" {
		return v, false
	}
	if isTruthyEnv(os.Getenv(bootstrapAdminRandomEnv)) {
		return generateBootstrapAdminPassword(), true
	}
	return DefaultAdminPass, false
}

// HashBootstrapAdminSecret bcrypt-hashes ResolveBootstrapAdminPassword.
func HashBootstrapAdminSecret() (hash []byte, plain string, err error) {
	plain, _ = ResolveBootstrapAdminPassword()
	hash, err = bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return hash, plain, err
}

func isTruthyEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func generateBootstrapAdminPassword() string {
	buf := make([]byte, bootstrapPasswordLen)
	if _, err := rand.Read(buf); err != nil {
		// Extremely unlikely; fall back so bootstrap still completes.
		return DefaultAdminPass
	}
	out := make([]byte, bootstrapPasswordLen)
	alpha := []byte(bootstrapPasswordAlphabet)
	for i, b := range buf {
		out[i] = alpha[int(b)%len(alpha)]
	}
	return string(out)
}
