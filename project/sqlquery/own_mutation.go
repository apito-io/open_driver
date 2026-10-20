package sqlquery

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// ErrOwnDocumentViolation is the driver-level denial for an own-scoped write
// whose caller or canonical owner cannot be proven.
var ErrOwnDocumentViolation = errors.New("permission denied: not the document owner")

// OwnMutationConstraint returns a parameterized SQL predicate for an own-scoped
// update or delete. Admin and non-own scopes return an empty predicate.
// A missing caller fails closed.
func OwnMutationConstraint(param *models.CommonSystemParams, action string) (string, []interface{}, error) {
	active, err := ownScopeActive(param, action)
	if err != nil || !active {
		return "", nil, err
	}
	uid := strings.TrimSpace(param.UserID)
	if isUserModelName(param.Model.Name) {
		return "id = ?", []interface{}{uid}, nil
	}
	tableName := utility.PhysicalSQLTableName(param.Model.Name)
	creatorPredicate := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM meta WHERE meta.doc_id = %s.id AND meta.created_by = ?)",
		tableName,
	)
	return canonicalOwnerConstraint(param, tableName, creatorPredicate, uid)
}

// OwnReadConstraint returns the row-level predicate for an own-scoped list or
// count. Canonical owner fields take precedence over audit metadata: a
// non-empty owner field must match the caller, while rows with no owner field
// fall back to meta.created_by. Conflicting owner_id/owner_uid values never
// fall back to the audit actor.
func OwnReadConstraint(param *models.CommonSystemParams, documentAlias, metaAlias string) (string, []interface{}, error) {
	active, err := ownScopeActive(param, "read")
	if err != nil || !active {
		return "", nil, err
	}
	uid := strings.TrimSpace(param.UserID)
	if isUserModelName(param.Model.Name) {
		return fmt.Sprintf("%s.id = ?", documentAlias), []interface{}{uid}, nil
	}
	creatorPredicate := fmt.Sprintf("%s.created_by = ?", metaAlias)
	return canonicalOwnerConstraint(param, documentAlias, creatorPredicate, uid)
}

func canonicalOwnerConstraint(param *models.CommonSystemParams, documentAlias, creatorPredicate, uid string) (string, []interface{}, error) {
	var ownerIDColumn, ownerUIDColumn string
	for _, field := range param.Model.Fields {
		if field == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(field.Identifier)) {
		case "owner_id":
			ownerIDColumn = PhysicalSQLColumnForSystemRelationField(field.Identifier, param.Model)
		case "owner_uid":
			ownerUIDColumn = PhysicalSQLColumnForSystemRelationField(field.Identifier, param.Model)
		}
	}

	blank := func(column string) string {
		return fmt.Sprintf("(%s.%s IS NULL OR %s.%s = '')", documentAlias, column, documentAlias, column)
	}
	switch {
	case ownerIDColumn != "" && ownerUIDColumn != "":
		return fmt.Sprintf(
			"((%s.%s = ? AND (%s.%s = ? OR %s)) OR (%s.%s = ? AND (%s.%s = ? OR %s)) OR (%s AND %s AND %s))",
			documentAlias, ownerIDColumn, documentAlias, ownerUIDColumn, blank(ownerUIDColumn),
			documentAlias, ownerUIDColumn, documentAlias, ownerIDColumn, blank(ownerIDColumn),
			blank(ownerIDColumn), blank(ownerUIDColumn), creatorPredicate,
		), []interface{}{uid, uid, uid, uid, uid}, nil
	case ownerIDColumn != "":
		return fmt.Sprintf("(%s.%s = ? OR (%s AND %s))", documentAlias, ownerIDColumn, blank(ownerIDColumn), creatorPredicate),
			[]interface{}{uid, uid}, nil
	case ownerUIDColumn != "":
		return fmt.Sprintf("(%s.%s = ? OR (%s AND %s))", documentAlias, ownerUIDColumn, blank(ownerUIDColumn), creatorPredicate),
			[]interface{}{uid, uid}, nil
	default:
		return creatorPredicate, []interface{}{uid}, nil
	}
}

// OwnMetadataConstraint returns the ownership predicate for a metadata-only
// common-model mutation. Keep this separate from OwnMutationConstraint: the
// common-model delete path updates meta, so inferring its meaning from the SQL
// text generated for a document table is both brittle and easy to get wrong.
func OwnMetadataConstraint(param *models.CommonSystemParams, action string) (string, []interface{}, error) {
	active, err := ownScopeActive(param, action)
	if err != nil || !active {
		return "", nil, err
	}
	uid := strings.TrimSpace(param.UserID)
	if isUserModelName(param.Model.Name) {
		return "doc_id = ?", []interface{}{uid}, nil
	}
	return "created_by = ?", []interface{}{uid}, nil
}

// EnforceOwnDocumentID rejects an own-scoped user-model write for another user
// before a meta-only update can run.
func EnforceOwnDocumentID(param *models.CommonSystemParams, action, documentID string) error {
	active, err := ownScopeActive(param, action)
	if err != nil || !active {
		return err
	}
	if isUserModelName(param.Model.Name) && strings.TrimSpace(documentID) != strings.TrimSpace(param.UserID) {
		return ErrOwnDocumentViolation
	}
	return nil
}

func ownScopeActive(param *models.CommonSystemParams, action string) (bool, error) {
	if param == nil || param.Role == nil || param.Model == nil {
		return false, nil
	}
	if utility.RoleBypassesDataACL(param.Role) {
		return false, nil
	}
	perm := utility.EffectivePermission(param.Role, param.Model.Name)
	if perm == nil {
		return false, nil
	}
	var scope string
	switch action {
	case "read":
		scope = perm.Read
	case "update":
		scope = perm.Update
	case "delete":
		scope = perm.Delete
	default:
		return false, nil
	}
	if scope != "own" {
		return false, nil
	}
	if strings.TrimSpace(param.UserID) == "" {
		return false, ErrOwnDocumentViolation
	}
	if !param.Role.IsProjectUser {
		return false, ErrOwnDocumentViolation
	}
	return true, nil
}

func isUserModelName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "user", "users":
		return true
	default:
		return false
	}
}
