package mysql

import (
	"context"
	"errors"
	"strings"

	"github.com/apito-io/engine/relationmigrate"
	"github.com/apito-io/open_driver/project/sqlcommon"
	"github.com/apito-io/open_driver/project/sqlquery"
)

func (d *Driver) relationMigrateSession(ctx context.Context) (sqlquery.SQLSession, error) {
	if d == nil || d.ORM == nil {
		return sqlquery.SQLSession{}, errors.New("sql driver is not open")
	}
	eng := ""
	if d.DriverCredential != nil {
		eng = strings.ToLower(strings.TrimSpace(d.DriverCredential.Engine))
	}
	return sqlquery.NewBunSession(ctx, eng, relationmigrate.ScopeKey(d.DriverCredential), d.ORM, func(table, column string) error {
		if sqlcommon.EngineIsSQLiteLike(eng) {
			return sqliteLikeExecDropColumnTx(ctx, d.ORM, table, column)
		}
		return sqlquery.DropSQLColumn(ctx, d.ORM, eng, table, column)
	}), nil
}

func (d *Driver) PreviewRelationCardinalityMigration(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Preview, error) {
	if req == nil {
		return nil, errors.New("relation migration request is required")
	}
	session, err := d.relationMigrateSession(ctx)
	if err != nil {
		return nil, err
	}
	preview, err := sqlquery.PreviewRelationCardinality(session, *req)
	if err != nil {
		return nil, err
	}
	return &preview, nil
}

func (d *Driver) MigrateRelationCardinality(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Receipt, error) {
	if req == nil {
		return nil, errors.New("relation migration request is required")
	}
	session, err := d.relationMigrateSession(ctx)
	if err != nil {
		return nil, err
	}
	receipt, err := sqlquery.MigrateRelationCardinality(session, *req)
	if err != nil {
		return &receipt, err
	}
	return &receipt, nil
}

func (d *Driver) VerifyRelationCardinalityMigration(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Receipt, error) {
	if req == nil {
		return nil, errors.New("relation migration request is required")
	}
	session, err := d.relationMigrateSession(ctx)
	if err != nil {
		return nil, err
	}
	receipt, err := sqlquery.VerifyRelationCardinality(session, *req)
	if err != nil {
		return &receipt, err
	}
	return &receipt, nil
}
