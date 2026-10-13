// Package bootstrapmeta holds shared constants and helpers for system DB first-run bootstrap.
// Orchestration lives in each ApitoSystemDB driver implementation.
package bootstrapmeta

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"

	"github.com/apito-io/engine/models"
	"golang.org/x/crypto/bcrypt"
)

// CreateStarterProjectFn creates the starter project row when it does not exist yet.
type CreateStarterProjectFn func(ctx context.Context, userID string, proj *models.Project) error

const (
	AdminEmail         = "admin@apito.io"
	DefaultAdminPass   = "#ApitoRocks#"
	AdminFirstName     = "Apito"
	AdminLastName      = "Admin"
	StarterProjectID   = "todo_notes"
	StarterProjectName = "Apito Website"
	// PreviousStarterProjectID is the starter id used before Todo Notes.
	// A system database that already has this project keeps it.
	PreviousStarterProjectID = "apito_website"
	// OSSStarterSQLiteFile is the SQLite filename (under Config.DefaultDatabaseDir) for the bundled
	// starter project when SYSTEM_DB_ENGINE=coredb. Not read from PROJECT_DB_* env.
	OSSStarterSQLiteFile = "apito_starter.sqlite"
	// StarterMongoDatabaseName is the logical Mongo database for the OSS starter project when the
	// system database is MongoDB (uses SYSTEM_DB_* connection fields).
	StarterMongoDatabaseName = "apito_starter_todo_notes"
	// StarterPostgresDatabaseName is the physical database for the OSS starter project when the
	// system database engine is PostgreSQL (uses SYSTEM_DB_* connection as admin; InitProjectBase creates the DB).
	StarterPostgresDatabaseName = "apito_starter_todo_notes"
	bboltUserNotFound           = "user not found"
)

// ExistingStarterProjectID returns the starter row this system database should keep using.
// A database that already contains the previous starter id does not gain a second project.
func ExistingStarterProjectID(ctx context.Context, get func(context.Context, string) (*models.Project, error)) string {
	if get == nil {
		return StarterProjectID
	}
	if project, err := get(ctx, StarterProjectID); err == nil && project != nil && project.ID != "" {
		return StarterProjectID
	}
	if project, err := get(ctx, PreviousStarterProjectID); err == nil && project != nil && project.ID != "" {
		return PreviousStarterProjectID
	}
	return StarterProjectID
}

// HashDefaultAdminPassword returns a bcrypt hash for the default bootstrap password.
func HashDefaultAdminPassword() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(DefaultAdminPass), bcrypt.DefaultCost)
}

// LogDefaultAdminCredentials prints the bordered login banner (plaintext password).
func LogDefaultAdminCredentials(email, plainPassword string) {
	border := "╔══════════════════════════════════════════════════════════════════════╗"
	empty := "║                                                                      ║"
	footer := "╚══════════════════════════════════════════════════════════════════════╝"

	log.Println(border)
	log.Println(empty)
	log.Printf("║                        BOOTSTRAP ADMIN CREDENTIALS                   ║")
	log.Println(empty)
	log.Printf("║  Email:    %-54s ║", email)
	log.Printf("║  Password: %-54s ║", plainPassword)
	log.Println(empty)
	log.Printf("║  Copy once (like an API key). Change after first login.              ║")
	log.Println(empty)
	log.Println(footer)
}

// IsUserLookupMiss reports whether err means the user document was not found.
func IsUserLookupMiss(err error) bool {
	if err == nil {
		return false
	}
	if isMongoUserLookupMiss(err) {
		return true
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.TrimSpace(err.Error())
	if msg == bboltUserNotFound {
		return true
	}
	// Arango returns "user not found : email@..."; Badger uses "user not found with email: ...".
	return strings.HasPrefix(msg, "user not found")
}
