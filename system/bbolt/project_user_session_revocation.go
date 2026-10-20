package bbolt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apitobolt "github.com/apito-io/apitoBolt"
	"github.com/apito-io/open_driver/system/sqlcommon"
)

var _ interface {
	GetProjectUserSessionRevokedAfter(context.Context, string, string) (int64, error)
	SetProjectUserSessionRevokedAfter(context.Context, string, string, int64) error
} = (*BBoltSystemDriver)(nil)

func (d *BBoltSystemDriver) GetProjectUserSessionRevokedAfter(
	ctx context.Context,
	projectID, userID string,
) (int64, error) {
	_ = ctx
	if d == nil || d.DB == nil {
		return 0, errors.New("system database is unavailable")
	}
	key := sqlcommon.ProjectUserSessionRevocationTokenID(projectID, userID)
	var row map[string]interface{}
	if err := d.DB.Collection("token_blacklist").FindByID(key, &row); err != nil {
		if errors.Is(err, apitobolt.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if len(row) == 0 {
		return 0, errors.New("invalid project-user revocation row")
	}
	storedProjectID, projectOK := row["project_id"].(string)
	storedUserID, userOK := row["user_id"].(string)
	if !projectOK || !userOK {
		return 0, errors.New("invalid project-user revocation row")
	}
	if err := sqlcommon.ValidateProjectUserSessionRevocationIdentity(
		projectID,
		userID,
		storedProjectID,
		storedUserID,
	); err != nil {
		return 0, err
	}
	raw, ok := row["revoked_after"]
	if !ok {
		return 0, errors.New("invalid project-user revocation row")
	}
	return revokedAfterFromBoltValue(raw)
}

func (d *BBoltSystemDriver) SetProjectUserSessionRevokedAfter(
	ctx context.Context,
	projectID, userID string,
	revokedAfter int64,
) error {
	if d == nil || d.DB == nil {
		return errors.New("system database is unavailable")
	}
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	if projectID == "" || userID == "" || revokedAfter <= 0 {
		return errors.New("project, user, and positive revocation time are required")
	}
	key := sqlcommon.ProjectUserSessionRevocationTokenID(projectID, userID)
	return d.DB.Update(func(tx *apitobolt.Tx) error {
		collection := tx.Collection("token_blacklist")
		var existing map[string]interface{}
		err := collection.FindByID(key, &existing)
		if err == nil {
			storedProjectID, projectOK := existing["project_id"].(string)
			storedUserID, userOK := existing["user_id"].(string)
			if !projectOK || !userOK {
				return errors.New("invalid project-user revocation row")
			}
			if err := sqlcommon.ValidateProjectUserSessionRevocationIdentity(
				projectID,
				userID,
				storedProjectID,
				storedUserID,
			); err != nil {
				return err
			}
			current, valueErr := revokedAfterFromBoltValue(existing["revoked_after"])
			if valueErr != nil {
				return valueErr
			}
			if current >= revokedAfter {
				return nil
			}
			existing["revoked_after"] = revokedAfter
			_, err = collection.Save(existing)
			return err
		}
		if !errors.Is(err, apitobolt.ErrNotFound) {
			return err
		}
		_, err = collection.Save(map[string]interface{}{
			"_key":          key,
			"id":            key,
			"project_id":    projectID,
			"user_id":       userID,
			"revoked_after": revokedAfter,
		})
		return err
	})
}

func revokedAfterFromBoltValue(raw interface{}) (int64, error) {
	switch value := raw.(type) {
	case int64:
		if value <= 0 {
			return 0, errors.New("invalid project-user revocation time")
		}
		return value, nil
	case int:
		if value <= 0 {
			return 0, errors.New("invalid project-user revocation time")
		}
		return int64(value), nil
	case float64:
		if value <= 0 {
			return 0, errors.New("invalid project-user revocation time")
		}
		return int64(value), nil
	default:
		return 0, fmt.Errorf("invalid project-user revocation time %T", raw)
	}
}
