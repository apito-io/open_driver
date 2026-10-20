package sqlcommon

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

const projectUserRevocationPrefix = "project-user-session:"

type projectUserRevocationData struct {
	ProjectID    string `json:"project_id"`
	UserID       string `json:"user_id"`
	RevokedAfter int64  `json:"revoked_after"`
}

// ProjectUserSessionRevocationTokenID is a stable non-secret key shared by
// system-driver implementations.
func ProjectUserSessionRevocationTokenID(projectID, userID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(projectID) + "\x00" + strings.TrimSpace(userID)))
	return projectUserRevocationPrefix + hex.EncodeToString(sum[:])
}

// ValidateProjectUserSessionRevocationIdentity verifies the redundant
// identity fields stored with a revocation cutoff. The hashed document key
// alone is not enough to trust a row: a corrupt or manually edited row must
// fail closed rather than revoke or exempt the wrong user.
func ValidateProjectUserSessionRevocationIdentity(
	projectID, userID, storedProjectID, storedUserID string,
) error {
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	storedProjectID = strings.TrimSpace(storedProjectID)
	storedUserID = strings.TrimSpace(storedUserID)
	if projectID == "" || userID == "" {
		return errors.New("project and user are required")
	}
	if storedProjectID != projectID || storedUserID != userID {
		return errors.New("project-user revocation row identity mismatch")
	}
	return nil
}

// GetProjectUserSessionRevokedAfter reads the durable cutoff from the existing
// system token_blacklist table. The row contains no bearer secret.
func GetProjectUserSessionRevokedAfter(ctx context.Context, db bun.IDB, projectID, userID string) (int64, error) {
	if db == nil {
		return 0, errors.New("system database is unavailable")
	}
	var row TokenBlacklistRow
	err := db.NewSelect().
		Model(&row).
		Where("token_id = ?", ProjectUserSessionRevocationTokenID(projectID, userID)).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no rows") {
			return 0, nil
		}
		return 0, err
	}
	var data projectUserRevocationData
	if err := json.Unmarshal([]byte(row.Data), &data); err != nil {
		return 0, fmt.Errorf("invalid project-user revocation row: %w", err)
	}
	if err := ValidateProjectUserSessionRevocationIdentity(
		projectID,
		userID,
		data.ProjectID,
		data.UserID,
	); err != nil {
		return 0, err
	}
	if data.RevokedAfter <= 0 {
		return 0, errors.New("invalid project-user revocation time")
	}
	return data.RevokedAfter, nil
}

// SetProjectUserSessionRevokedAfter upserts the durable cutoff. The read and
// write happen in a serializable transaction so concurrent password resets
// cannot move the cutoff backwards. Insert races are retried after the losing
// transaction observes the row created by the winner.
func SetProjectUserSessionRevokedAfter(ctx context.Context, db bun.IDB, projectID, userID string, revokedAfter int64) error {
	if db == nil {
		return errors.New("system database is unavailable")
	}
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	if projectID == "" || userID == "" || revokedAfter <= 0 {
		return errors.New("project, user, and positive revocation time are required")
	}
	payload, err := json.Marshal(projectUserRevocationData{
		ProjectID:    projectID,
		UserID:       userID,
		RevokedAfter: revokedAfter,
	})
	if err != nil {
		return err
	}
	tokenID := ProjectUserSessionRevocationTokenID(projectID, userID)
	if ctx == nil {
		ctx = context.Background()
	}

	const maxAttempts = 8
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, func(txCtx context.Context, tx bun.Tx) error {
			var existing TokenBlacklistRow
			err := tx.NewSelect().
				Model(&existing).
				Where("token_id = ?", tokenID).
				Limit(1).
				Scan(txCtx)
			if err == nil {
				var current projectUserRevocationData
				if err := json.Unmarshal([]byte(existing.Data), &current); err != nil {
					return fmt.Errorf("invalid project-user revocation row: %w", err)
				}
				if err := ValidateProjectUserSessionRevocationIdentity(
					projectID,
					userID,
					current.ProjectID,
					current.UserID,
				); err != nil {
					return err
				}
				if current.RevokedAfter >= revokedAfter {
					return nil
				}
				_, err = tx.NewUpdate().
					Model((*TokenBlacklistRow)(nil)).
					Set("data = ?", string(payload)).
					Where("token_id = ?", tokenID).
					Exec(txCtx)
				return err
			}
			if !isNoRowsError(err) {
				return err
			}
			_, err = tx.NewInsert().Model(&TokenBlacklistRow{
				TokenID: tokenID,
				Data:    string(payload),
			}).Exec(txCtx)
			return err
		})
		if lastErr == nil || !isRetryableRevocationError(lastErr) {
			return lastErr
		}
		if attempt+1 < maxAttempts {
			timer := time.NewTimer(time.Duration(attempt+1) * 5 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return lastErr
}

func isNoRowsError(err error) bool {
	return errors.Is(err, sql.ErrNoRows) ||
		(err != nil && strings.Contains(strings.ToLower(err.Error()), "no rows"))
}

func isRetryableRevocationError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"serialization",
		"deadlock",
		"database is locked",
		"database table is locked",
		"database is busy",
		"unique constraint",
		"duplicate key",
		"duplicate entry",
		"constraint failed",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
