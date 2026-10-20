package mongodb

import (
	"context"
	"errors"
	"strings"

	"github.com/apito-io/open_driver/system/sqlcommon"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var _ interface {
	GetProjectUserSessionRevokedAfter(context.Context, string, string) (int64, error)
	SetProjectUserSessionRevokedAfter(context.Context, string, string, int64) error
} = (*SystemMongoDriver)(nil)

type projectUserSessionRevocationRow struct {
	ProjectID    string `bson:"project_id"`
	UserID       string `bson:"user_id"`
	RevokedAfter int64  `bson:"revoked_after"`
}

func (m *SystemMongoDriver) readProjectUserSessionRevocation(
	ctx context.Context,
	projectID, userID string,
) (projectUserSessionRevocationRow, bool, error) {
	var row projectUserSessionRevocationRow
	err := m.Database.Collection("token_blacklist").
		FindOne(ctx, bson.M{
			"token_id": sqlcommon.ProjectUserSessionRevocationTokenID(projectID, userID),
		}).
		Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return projectUserSessionRevocationRow{}, false, nil
	}
	if err != nil {
		return projectUserSessionRevocationRow{}, false, err
	}
	if err := sqlcommon.ValidateProjectUserSessionRevocationIdentity(
		projectID,
		userID,
		row.ProjectID,
		row.UserID,
	); err != nil {
		return projectUserSessionRevocationRow{}, false, err
	}
	if row.RevokedAfter <= 0 {
		return projectUserSessionRevocationRow{}, false, errors.New("invalid project-user revocation row")
	}
	return row, true, nil
}

func (m *SystemMongoDriver) GetProjectUserSessionRevokedAfter(
	ctx context.Context,
	projectID, userID string,
) (int64, error) {
	if m == nil || m.Database == nil {
		return 0, errors.New("system database is unavailable")
	}
	row, found, err := m.readProjectUserSessionRevocation(
		ctx,
		strings.TrimSpace(projectID),
		strings.TrimSpace(userID),
	)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}
	return row.RevokedAfter, nil
}

func (m *SystemMongoDriver) SetProjectUserSessionRevokedAfter(
	ctx context.Context,
	projectID, userID string,
	revokedAfter int64,
) error {
	if m == nil || m.Database == nil {
		return errors.New("system database is unavailable")
	}
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(userID) == "" || revokedAfter <= 0 {
		return errors.New("project, user, and positive revocation time are required")
	}
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	tokenID := sqlcommon.ProjectUserSessionRevocationTokenID(projectID, userID)
	collection := m.Database.Collection("token_blacklist")

	for attempt := 0; attempt < 4; attempt++ {
		current, found, err := m.readProjectUserSessionRevocation(ctx, projectID, userID)
		if err != nil {
			return err
		}
		if found {
			if current.RevokedAfter >= revokedAfter {
				return nil
			}
			result, err := collection.UpdateOne(
				ctx,
				bson.M{
					"token_id":   tokenID,
					"project_id": projectID,
					"user_id":    userID,
				},
				bson.M{"$max": bson.M{"revoked_after": revokedAfter}},
				options.UpdateOne(),
			)
			if err != nil {
				return err
			}
			if result.MatchedCount > 0 {
				return nil
			}
			continue
		}

		_, err = collection.InsertOne(ctx, bson.M{
			"token_id":      tokenID,
			"project_id":    projectID,
			"user_id":       userID,
			"revoked_after": revokedAfter,
		})
		if err == nil {
			return nil
		}
		if mongo.IsDuplicateKeyError(err) {
			continue
		}
		return err
	}
	return errors.New("project-user revocation update raced repeatedly")
}
