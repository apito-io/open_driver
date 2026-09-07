package sqlquery

import (
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// ShadowTarget is an id-only upsert into a common-model shadow table.
type ShadowTarget struct {
	Table string
	ID    string
}

// InsertShadowIDSQL returns an idempotent insert of a single id into a shadow table.
func InsertShadowIDSQL(engine, table, id string) (string, []interface{}) {
	eng := strings.ToLower(strings.TrimSpace(engine))
	table = strings.TrimSpace(table)
	switch eng {
	case _const.PostgreSQLDriver:
		return fmt.Sprintf(`INSERT INTO "%s" (id) VALUES (?) ON CONFLICT (id) DO NOTHING`, table), []interface{}{id}
	case _const.MySQLDriver, _const.MariaDBDriver:
		return fmt.Sprintf("INSERT IGNORE INTO `%s` (`id`) VALUES (?)", table), []interface{}{id}
	default:
		return fmt.Sprintf("INSERT OR IGNORE INTO `%s` (`id`) VALUES (?)", table), []interface{}{id}
	}
}

// SchemaModelByName finds a schema model by name (case-insensitive).
func SchemaModelByName(modelsList []*models.ModelType, name string) *models.ModelType {
	name = strings.TrimSpace(name)
	for _, m := range modelsList {
		if m != nil && strings.EqualFold(strings.TrimSpace(m.Name), name) {
			return m
		}
	}
	return nil
}

// CommonShadowTargets returns id-only upserts for common-model endpoints of a connect.
// ForwardConnectionID is always the mutating document (root model). ActionIDs are
// the related documents. Pairing ForwardConnectionType.Model with ForwardConnectionID
// is wrong — that field names the peer, not the owner of the uid.
func CommonShadowTargets(modelsList []*models.ModelType, rootModel *models.ModelType, cdp *models.ConnectDisconnectParam) []ShadowTarget {
	if cdp == nil {
		return nil
	}
	var out []ShadowTarget
	add := func(modelName, id string) {
		id = strings.TrimSpace(id)
		if id == "" || strings.TrimSpace(modelName) == "" {
			return
		}
		m := SchemaModelByName(modelsList, modelName)
		if !models.ModelIsCommon(m) {
			return
		}
		out = append(out, ShadowTarget{
			Table: utility.PhysicalSQLTableName(modelName),
			ID:    id,
		})
	}
	rootName := ""
	if rootModel != nil {
		rootName = rootModel.Name
	}
	add(rootName, cdp.ForwardConnectionID)
	peer := connectPeerModelName(rootName, cdp)
	for _, id := range cdp.ActionIDs {
		add(peer, id)
	}
	return out
}

func connectPeerModelName(rootName string, cdp *models.ConnectDisconnectParam) string {
	if cdp == nil {
		return ""
	}
	rootName = strings.TrimSpace(rootName)
	if cdp.ForwardConnectionType != nil {
		m := strings.TrimSpace(cdp.ForwardConnectionType.Model)
		if m != "" && !strings.EqualFold(m, rootName) {
			return m
		}
	}
	if cdp.BackwardConnectionType != nil {
		m := strings.TrimSpace(cdp.BackwardConnectionType.Model)
		if m != "" && !strings.EqualFold(m, rootName) {
			return m
		}
	}
	return ""
}
