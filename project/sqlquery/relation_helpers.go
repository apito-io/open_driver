package sqlquery

import (
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

func findSchemaModel(modelsList []*models.ModelType, name string) *models.ModelType {
	for _, m := range modelsList {
		if m != nil && utility.ModelIDMatchesGraphQLField(m.Name, name) {
			return m
		}
	}
	return nil
}

func sqlConnectionIsTrueManyToMany(modelsList []*models.ModelType, fromModelName, toModelName string) bool {
	if len(modelsList) == 0 {
		return false
	}
	a := findSchemaModel(modelsList, fromModelName)
	b := findSchemaModel(modelsList, toModelName)
	if a == nil || b == nil {
		return false
	}
	var out *models.ConnectionType
	for _, c := range a.Connections {
		if c == nil || c.Relation != "has_many" {
			continue
		}
		if !utility.ModelIDMatchesGraphQLField(c.Model, b.Name) {
			continue
		}
		out = c
		break
	}
	if out == nil {
		return false
	}
	for _, c := range b.Connections {
		if c == nil || c.Relation != "has_many" {
			continue
		}
		if !utility.ModelIDMatchesGraphQLField(c.Model, a.Name) {
			continue
		}
		if c.KnownAs != out.KnownAs {
			continue
		}
		return true
	}
	return false
}

func sqlConnectionAnchorID(connection map[string]interface{}) (string, bool) {
	if connection == nil {
		return "", false
	}
	v, ok := connection["_id"]
	if !ok || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		return s, s != ""
	default:
		return "", false
	}
}

func sqlConnectionAnchorModelName(connectionType, fromModel, toModel string) string {
	switch strings.TrimSpace(connectionType) {
	case "forward":
		return strings.TrimSpace(fromModel)
	case "backward":
		return strings.TrimSpace(toModel)
	default:
		return ""
	}
}

func fkPhysicalColumnOnModelToTarget(holder *models.ModelType, targetModelName, knownAs string) (string, bool) {
	if holder == nil {
		return "", false
	}
	want := relationFKColumnNameParts(targetModelName, knownAs)
	if want == "" {
		return "", false
	}
	for _, f := range holder.Fields {
		if f == nil || !f.SystemGenerated {
			continue
		}
		gql := strings.TrimSpace(f.Identifier)
		if gql == "" {
			continue
		}
		phys := PhysicalSQLColumnForSystemRelationField(gql, holder)
		if phys == want {
			return phys, true
		}
	}
	known := strings.TrimSpace(knownAs)
	for _, conn := range holder.Connections {
		if conn == nil || conn.Relation != "has_one" {
			continue
		}
		if known != "" && strings.TrimSpace(conn.KnownAs) != known {
			continue
		}
		if !utility.ModelIDMatchesGraphQLField(conn.Model, targetModelName) {
			continue
		}
		if col := relationFKColumnNameParts(conn.Model, conn.KnownAs); col == want {
			return col, true
		}
	}
	return "", false
}
