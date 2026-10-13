package mariadb

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun/dialect"
	"github.com/apito-io/open_driver/system/bootstrapmeta"
	"github.com/apito-io/open_driver/system/sqlcommon"
)

type Driver struct {
	sqlcommon.Base
}

type SystemSQLDriver = Driver

func GetMariaDBSystemDriver(cfg *models.Config, cred *models.DriverCredentials) (*Driver, error) {
	if cred == nil {
		return nil, sqlcommon.ErrCredentialsRequired()
	}
	orm, err := sqlcommon.OpenMariaDB(cred)
	if err != nil {
		return nil, err
	}
	sqlcommon.RegisterSystemSQLSchemaModels(orm)
	return &Driver{Base: sqlcommon.Base{Conf: cfg, ORM: orm, DriverCredential: cred}}, nil
}

func (d *Driver) Ping() error {
	ctx := context.Background()
	if d == nil || d.ORM == nil {
		return fmt.Errorf("system sql driver not initialized")
	}
	switch d.ORM.Dialect().Name() {
	case dialect.SQLite, dialect.PG, dialect.MySQL:
		var result int
		return d.ORM.NewSelect().ColumnExpr("1").Scan(ctx, &result)
	default:
		return fmt.Errorf("database dialect not supported: %s", d.ORM.Dialect().Name())
	}
}

func (d *Driver) Close() error {
	if d == nil || d.ORM == nil {
		return nil
	}
	return d.ORM.Close()
}

func (d *Driver) RunMigration(ctx context.Context) error {
	sqlcommon.RegisterSystemSQLSchemaModels(d.ORM)
	for _, m := range sqlcommon.SystemSQLSchemaModels() {
		if _, err := d.ORM.NewCreateTable().IfNotExists().Model(m).Exec(ctx); err != nil {
			return err
		}
	}
	if err := ensureProjectSettingsSQLColumns(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureSystemUserAccessTokensColumn(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureSystemUserIsSuperAdminColumn(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureProjectUsersOAuthSubColumn(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureProjectTokensMetadataColumns(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureProjectPlansColumn(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureProjectIconColumn(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureUserProjectInviteColumns(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureProjectTempBannedColumn(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureProjectPlanSeedOmissionsColumn(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.NormalizeAllProjectPlanSystemFlags(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureApitoFunctionsCompositePK(ctx, d.ORM); err != nil {
		return err
	}
	if err := sqlcommon.EnsureModelTypeUniqueKeysColumn(ctx, d.ORM); err != nil {
		return err
	}
	if err := d.repairProjectSchemaIntegrity(ctx); err != nil {
		return err
	}
	if err := sqlcommon.DropObsoleteTeamOrgTables(ctx, d.ORM); err != nil {
		return err
	}
	return d.ensureSystemSecondaryIndexes(ctx)
}

func (d *Driver) EnsureSystemBootstrap(ctx context.Context) error {
	if err := d.ensureBootstrapAdmin(ctx); err != nil {
		return err
	}
	if err := bootstrapmeta.RepairSuperAdminEmails(ctx, d.GetSystemUserByEmail, d.UpdateSystemUser); err != nil {
		return err
	}
	if err := d.ensureBootstrapOrgTeamProject(ctx); err != nil {
		return err
	}
	return d.ensureStarterProjectDriver(ctx)
}

func (d *Driver) GetSQLBase() *sqlcommon.Base { return &d.Base }

var _ interfaces.ApitoSystemDB = (*Driver)(nil)
