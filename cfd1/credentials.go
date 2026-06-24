package cfd1

import (
	"encoding/json"
	"strings"

	"github.com/apito-io/engine/models"
)

// CredentialsJSON is persisted in pro_tenants.driver_credentials_json for scoped D1 DBs.
type CredentialsJSON struct {
	Engine     string `json:"engine"`
	DatabaseID string `json:"database_id"`
}

// ConfigFromCredentials builds a REST-mode DB config from DriverCredentials.
func ConfigFromCredentials(cred *models.DriverCredentials, accountID, apiToken string) (Config, error) {
	if cred == nil {
		return Config{}, ErrCredentialsRequired
	}
	dbID := strings.TrimSpace(cred.Database)
	if dbID == "" {
		return Config{}, ErrCredentialsRequired
	}
	return Config{
		Mode:       AccessREST,
		DatabaseID: dbID,
		AccountID:  accountID,
		APIToken:   apiToken,
	}, nil
}

// ParseScopedCredentialsJSON unmarshals tenant scoped creds from pro_tenants.
func ParseScopedCredentialsJSON(raw string) (*models.DriverCredentials, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrCredentialsRequired
	}
	var wrap CredentialsJSON
	if err := json.Unmarshal([]byte(raw), &wrap); err != nil {
		// fallback: legacy full DriverCredentials JSON
		var cred models.DriverCredentials
		if err2 := json.Unmarshal([]byte(raw), &cred); err2 != nil {
			return nil, err
		}
		return &cred, nil
	}
	return &models.DriverCredentials{
		Engine:   EngineName,
		Database: wrap.DatabaseID,
	}, nil
}

// MarshalScopedCredentials returns JSON for pro_tenants.driver_credentials_json.
func MarshalScopedCredentials(databaseID string) (string, error) {
	b, err := json.Marshal(CredentialsJSON{
		Engine:     EngineName,
		DatabaseID: strings.TrimSpace(databaseID),
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
