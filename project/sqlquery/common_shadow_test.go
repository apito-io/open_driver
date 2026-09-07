package sqlquery

import (
	"strings"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func TestInsertShadowIDSQL(t *testing.T) {
	q, args := InsertShadowIDSQL(_const.SQLiteDriver, "medicine", "m1")
	if !strings.Contains(q, "INSERT OR IGNORE") || len(args) != 1 || args[0] != "m1" {
		t.Fatalf("sqlite shadow sql: %s %v", q, args)
	}
	q, _ = InsertShadowIDSQL(_const.PostgreSQLDriver, "medicine", "m1")
	if !strings.Contains(q, "ON CONFLICT") {
		t.Fatalf("postgres shadow sql: %s", q)
	}
	q, _ = InsertShadowIDSQL(_const.MySQLDriver, "medicine", "m1")
	if !strings.Contains(q, "INSERT IGNORE") {
		t.Fatalf("mysql shadow sql: %s", q)
	}
}

func TestCommonShadowTargets(t *testing.T) {
	common := &models.ModelType{Name: "medicine", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}
	stock := &models.ModelType{Name: "stock"}
	// Matches connect_disconnect_builder: uid is the mutating stock row,
	// ActionIDs are the related medicine ids, ForwardConnectionType.Model is the peer.
	cdp := &models.ConnectDisconnectParam{
		ForwardConnectionID: "s1",
		ActionIDs:           []string{"m1", "m2"},
		ForwardConnectionType: &models.ConnectionType{
			Model: "medicine",
		},
		BackwardConnectionType: &models.ConnectionType{
			Model: "stock",
		},
	}
	got := CommonShadowTargets([]*models.ModelType{common, stock}, stock, cdp)
	if len(got) != 2 {
		t.Fatalf("expected two medicine shadows, got %#v", got)
	}
	if got[0].Table != "medicine" || got[0].ID != "m1" {
		t.Fatalf("first target: %#v", got[0])
	}
}

func TestCommonShadowTargetsSkipsTenantModels(t *testing.T) {
	stock := &models.ModelType{Name: "stock"}
	cdp := &models.ConnectDisconnectParam{
		ForwardConnectionID: "s1",
		ActionIDs:           []string{"s2"},
		ForwardConnectionType: &models.ConnectionType{
			Model: "stock",
		},
		BackwardConnectionType: &models.ConnectionType{
			Model: "stock",
		},
	}
	if got := CommonShadowTargets([]*models.ModelType{stock}, stock, cdp); len(got) != 0 {
		t.Fatalf("tenant-to-tenant must not shadow, got %#v", got)
	}
}

func TestCommonShadowTargetsDoesNotSwapIdsOntoPeerTable(t *testing.T) {
	medicine := &models.ModelType{Name: "medicine", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}
	generic := &models.ModelType{Name: "generic", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}
	cdp := &models.ConnectDisconnectParam{
		ForwardConnectionID: "med1",
		ActionIDs:           []string{"gen1"},
		ForwardConnectionType: &models.ConnectionType{
			Model: "generic",
		},
		BackwardConnectionType: &models.ConnectionType{
			Model: "medicine",
		},
	}
	got := CommonShadowTargets([]*models.ModelType{medicine, generic}, medicine, cdp)
	if len(got) != 2 {
		t.Fatalf("expected medicine+generic shadows, got %#v", got)
	}
	byTable := map[string]string{}
	for _, tgot := range got {
		byTable[tgot.Table] = tgot.ID
	}
	if byTable["medicine"] != "med1" || byTable["generic"] != "gen1" {
		t.Fatalf("ids must stay on their own tables, got %#v", got)
	}
}
