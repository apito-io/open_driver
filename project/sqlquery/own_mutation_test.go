package sqlquery

import (
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility/securitytest"
)

func ownRole() *models.Role {
	return &models.Role{
		ID:            "passenger",
		IsProjectUser: true,
		APIPermissions: map[string]*models.APIPermission{
			"post":  {Update: "own", Delete: "own"},
			"user":  {Update: "own", Delete: "own"},
			"users": {Update: "own", Delete: "own"},
		},
	}
}

func TestOwnMutationConstraintFailClosed(t *testing.T) {
	param := &models.CommonSystemParams{
		Model: &models.ModelType{Name: "post"},
		Role:  ownRole(),
	}
	_, _, err := OwnMutationConstraint(param, "update")
	if err != ErrOwnDocumentViolation {
		t.Fatalf("empty caller: %v", err)
	}

	param.UserID = securitytest.FixtureUserA
	param.Role = &models.Role{
		APIPermissions: map[string]*models.APIPermission{
			"post": {Update: "own"},
		},
	}
	_, _, err = OwnMutationConstraint(param, "update")
	if err != ErrOwnDocumentViolation {
		t.Fatalf("non-project-user role: %v", err)
	}
}

func TestOwnMutationConstraintCreatorAndOwner(t *testing.T) {
	param := &models.CommonSystemParams{
		UserID: securitytest.FixtureUserA,
		Model: &models.ModelType{
			Name: "post",
			Fields: []*models.FieldInfo{
				{Identifier: "owner_id"},
			},
		},
		Role: ownRole(),
	}
	clause, args, err := OwnMutationConstraint(param, "delete")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(clause, "meta.doc_id = post.id") ||
		!strings.Contains(clause, "meta.created_by = ?") ||
		!strings.Contains(clause, "post.owner_id = ?") ||
		!strings.Contains(clause, "post.owner_id IS NULL") {
		t.Fatalf("clause %s", clause)
	}
	if len(args) != 2 || args[0] != securitytest.FixtureUserA || args[1] != securitytest.FixtureUserA {
		t.Fatalf("args %#v", args)
	}
}

func TestOwnMetadataConstraintIsExplicit(t *testing.T) {
	param := &models.CommonSystemParams{
		UserID: securitytest.FixtureUserA,
		Model:  &models.ModelType{Name: "post"},
		Role:   ownRole(),
	}
	clause, args, err := OwnMetadataConstraint(param, "delete")
	if err != nil {
		t.Fatal(err)
	}
	if clause != "created_by = ?" || len(args) != 1 || args[0] != securitytest.FixtureUserA {
		t.Fatalf("metadata constraint %q %#v", clause, args)
	}

	param.Model.Name = "users"
	clause, args, err = OwnMetadataConstraint(param, "delete")
	if err != nil {
		t.Fatal(err)
	}
	if clause != "doc_id = ?" || len(args) != 1 || args[0] != securitytest.FixtureUserA {
		t.Fatalf("user metadata constraint %q %#v", clause, args)
	}
}

func TestOwnMutationConstraintSkipsAdminAndAll(t *testing.T) {
	admin := &models.CommonSystemParams{
		UserID: securitytest.FixtureUserA,
		Model:  &models.ModelType{Name: "post"},
		Role:   &models.Role{ID: "admin", IsAdmin: true},
	}
	clause, _, err := OwnMutationConstraint(admin, "update")
	if err != nil || clause != "" {
		t.Fatalf("admin clause %q err %v", clause, err)
	}
	all := &models.CommonSystemParams{
		UserID: securitytest.FixtureUserA,
		Model:  &models.ModelType{Name: "post"},
		Role: &models.Role{
			ID:             "manager",
			IsProjectUser:  true,
			APIPermissions: map[string]*models.APIPermission{"post": {Update: "all"}},
		},
	}
	clause, _, err = OwnMutationConstraint(all, "update")
	if err != nil || clause != "" {
		t.Fatalf("all clause %q err %v", clause, err)
	}
}

func TestEnforceOwnDocumentIDUserModel(t *testing.T) {
	param := &models.CommonSystemParams{
		UserID: securitytest.FixtureUserA,
		Model:  &models.ModelType{Name: "user"},
		Role:   ownRole(),
	}
	if err := EnforceOwnDocumentID(param, "delete", securitytest.FixtureUserB); err != ErrOwnDocumentViolation {
		t.Fatalf("other user: %v", err)
	}
	if err := EnforceOwnDocumentID(param, "delete", securitytest.FixtureUserA); err != nil {
		t.Fatal(err)
	}
}
