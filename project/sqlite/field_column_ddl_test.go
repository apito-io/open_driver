package sqlite

import (
	"strings"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func TestAlterTableAddFieldSQL_sqliteUniqueSplitsAlterAndIndex(t *testing.T) {
	f := &models.FieldInfo{
		Identifier: "slug",
		FieldType:  _const.TextField,
		InputType:  _const.StringInput,
		Validation: &models.Validation{
			Unique: true,
		},
	}
	for _, engine := range []string{_const.SQLiteDriver, "libsql"} {
		stmts, err := AlterTableAddFieldSQL(engine, "articles", f)
		if err != nil {
			t.Fatalf("%s: %v", engine, err)
		}
		if len(stmts) != 2 {
			t.Fatalf("%s: want 2 statements, got %d: %#v", engine, len(stmts), stmts)
		}
		alter := stmts[0]
		if strings.Contains(strings.ToUpper(alter), "UNIQUE") {
			t.Fatalf("%s: ALTER must not contain UNIQUE, got %q", engine, alter)
		}
		idx := stmts[1]
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(idx)), "CREATE UNIQUE INDEX") {
			t.Fatalf("%s: second stmt should be CREATE UNIQUE INDEX, got %q", engine, idx)
		}
	}
}

func TestAlterTableAddFieldSQL_sqliteLocalsNoIfNotExists(t *testing.T) {
	f := &models.FieldInfo{
		Identifier: "bio",
		FieldType:  _const.MultilineField,
		InputType:  _const.StringInput,
		Validation: &models.Validation{
			Locals: []string{"en", "bn"},
		},
	}
	for _, engine := range []string{_const.SQLiteDriver, "libsql"} {
		stmts, err := AlterTableAddFieldSQL(engine, "tenant", f)
		if err != nil {
			t.Fatalf("%s: %v", engine, err)
		}
		if len(stmts) != 2 {
			t.Fatalf("%s: want 2 locale columns, got %d: %#v", engine, len(stmts), stmts)
		}
		for _, stmt := range stmts {
			up := strings.ToUpper(stmt)
			if strings.Contains(up, "IF NOT EXISTS") {
				t.Fatalf("%s: sqlite locals ALTER must not use IF NOT EXISTS: %q", engine, stmt)
			}
			if !strings.HasPrefix(up, "ALTER TABLE") {
				t.Fatalf("%s: expected ALTER TABLE, got %q", engine, stmt)
			}
		}
	}
}

func TestAlterTableAddFieldSQL_postgresKeepsUniqueInline(t *testing.T) {
	f := &models.FieldInfo{
		Identifier: "slug",
		FieldType:  _const.TextField,
		InputType:  _const.StringInput,
		Validation: &models.Validation{
			Unique: true,
		},
	}
	stmts, err := AlterTableAddFieldSQL(_const.PostgreSQLDriver, "articles", f)
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement for postgres, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "UNIQUE") {
		t.Fatalf("postgres ALTER should include UNIQUE when requested: %q", stmts[0])
	}
}
