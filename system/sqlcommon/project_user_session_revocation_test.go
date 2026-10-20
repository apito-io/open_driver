package sqlcommon

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func newProjectUserRevocationTestDB(t *testing.T) *bun.DB {
	t.Helper()

	return openProjectUserRevocationTestDB(
		t,
		"file:project-user-revocation?mode=memory&cache=shared",
	)
}

func openProjectUserRevocationTestDB(t *testing.T, dsn string) *bun.DB {
	t.Helper()

	sqlDB, err := sql.Open(sqliteshim.ShimName, dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)

	db := bun.NewDB(sqlDB, sqlitedialect.New())
	_, err = db.NewCreateTable().
		Model((*TokenBlacklistRow)(nil)).
		IfNotExists().
		Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func TestSetProjectUserSessionRevokedAfterIsMonotonic(t *testing.T) {
	db := newProjectUserRevocationTestDB(t)
	ctx := context.Background()

	require.NoError(t, SetProjectUserSessionRevokedAfter(ctx, db, "project", "user", 100))
	require.NoError(t, SetProjectUserSessionRevokedAfter(ctx, db, "project", "user", 300))
	require.NoError(t, SetProjectUserSessionRevokedAfter(ctx, db, "project", "user", 200))

	revokedAfter, err := GetProjectUserSessionRevokedAfter(ctx, db, "project", "user")
	require.NoError(t, err)
	require.Equal(t, int64(300), revokedAfter)
}

func TestGetProjectUserSessionRevokedAfterRejectsIdentityMismatch(t *testing.T) {
	db := newProjectUserRevocationTestDB(t)
	ctx := context.Background()
	payload, err := json.Marshal(projectUserRevocationData{
		ProjectID:    "project-b",
		UserID:       "user",
		RevokedAfter: 100,
	})
	require.NoError(t, err)
	_, err = db.NewInsert().Model(&TokenBlacklistRow{
		TokenID: ProjectUserSessionRevocationTokenID("project", "user"),
		Data:    string(payload),
	}).Exec(ctx)
	require.NoError(t, err)

	_, err = GetProjectUserSessionRevokedAfter(ctx, db, "project", "user")
	require.ErrorContains(t, err, "identity mismatch")
}

func TestProjectUserSessionRevocationPersistsAcrossSQLiteFileReopen(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "system.db")
	ctx := context.Background()

	db := openProjectUserRevocationTestDB(t, dsn)
	require.NoError(t, SetProjectUserSessionRevokedAfter(ctx, db, "project", "user", 900))
	require.NoError(t, db.Close())

	reopened := openProjectUserRevocationTestDB(t, dsn)
	revokedAfter, err := GetProjectUserSessionRevokedAfter(ctx, reopened, "project", "user")
	require.NoError(t, err)
	require.Equal(t, int64(900), revokedAfter)
}

func TestSetProjectUserSessionRevokedAfterConcurrentWritersKeepMaximum(t *testing.T) {
	db := newProjectUserRevocationTestDB(t)
	ctx := context.Background()
	values := []int64{400, 100, 700, 250}
	errs := make(chan error, len(values))
	var wg sync.WaitGroup

	for _, value := range values {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- SetProjectUserSessionRevokedAfter(ctx, db, "project", "user", value)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	revokedAfter, err := GetProjectUserSessionRevokedAfter(ctx, db, "project", "user")
	require.NoError(t, err)
	require.Equal(t, int64(700), revokedAfter)
}
