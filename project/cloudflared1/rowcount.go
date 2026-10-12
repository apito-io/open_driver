package cloudflared1

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

const envCounterTriggers = "D1_ENABLE_COUNTER_TRIGGERS"

func counterTriggersEnabled() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(envCounterTriggers)), "true") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("TURSO_ENABLE_COUNTER_TRIGGERS")), "true")
}

func rowCountTriggerNames(physicalTable string) (insertTrig, deleteTrig string) {
	sanitize := func(tbl string) string {
		return strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				return r
			}
			return '_'
		}, tbl)
	}
	base := sanitize(physicalTable)
	return fmt.Sprintf("tr_rc_%s_ai", base), fmt.Sprintf("tr_rc_%s_ad", base)
}

func (d *Driver) installRowCountTriggersForModel(ctx context.Context, model *models.ModelType, hasTenant bool) error {
	if d == nil || model == nil || !counterTriggersEnabled() {
		return nil
	}
	tbl := utility.PhysicalSQLTableName(model.Name)
	qtbl := strings.ReplaceAll(tbl, "`", "``")
	tlit := strings.ReplaceAll(tbl, "'", "''")
	tenantExpr := `''`
	tenantExprOld := `''`
	if hasTenant {
		tenantExpr = `COALESCE(NEW.tenant_id, '')`
		tenantExprOld = `COALESCE(OLD.tenant_id, '')`
	}
	trAI, trAD := rowCountTriggerNames(tbl)
	stmts := []string{
		fmt.Sprintf("DROP TRIGGER IF EXISTS `%s`", trAI),
		fmt.Sprintf("DROP TRIGGER IF EXISTS `%s`", trAD),
		fmt.Sprintf(`CREATE TRIGGER `+"`%s`"+` AFTER INSERT ON `+"`%s`"+` BEGIN
  INSERT INTO _apito_row_counts(table_name, tenant_id, row_count)
  VALUES('%s', %s, 1)
  ON CONFLICT(table_name, tenant_id) DO UPDATE SET row_count = row_count + 1;
END`, trAI, qtbl, tlit, tenantExpr),
		fmt.Sprintf(`CREATE TRIGGER `+"`%s`"+` AFTER DELETE ON `+"`%s`"+` BEGIN
  UPDATE _apito_row_counts SET row_count = row_count - 1
  WHERE table_name = '%s' AND tenant_id = %s;
END`, trAD, qtbl, tlit, tenantExprOld),
	}
	if err := d.execDDL(ctx, stmts...); err != nil {
		return err
	}
	row, err := d.DB.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) AS n FROM `%s`", qtbl))
	if err != nil {
		return err
	}
	var n int64
	if v, ok := row["n"]; ok {
		switch t := v.(type) {
		case int64:
			n = t
		case float64:
			n = int64(t)
		}
	}
	_, err = d.DB.Exec(ctx, `
		INSERT INTO _apito_row_counts(table_name, tenant_id, row_count) VALUES (?, '', ?)
		ON CONFLICT(table_name, tenant_id) DO UPDATE SET row_count = excluded.row_count`,
		tbl, n,
	)
	return err
}
