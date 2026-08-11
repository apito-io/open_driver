# open_driver changelog

## [1.0.17] — 2026-08-11

### Project settings

- **Idle tenant columns** on SQL `project_settings`: `idle_tenant_retention_days` (default 90), `auto_soft_delete_idle_tenants` (default false). ALTER ensures on SQLite / Postgres / MySQL / MariaDB persist paths.

### System / project SQL

- Project icon column + plan seed omissions / system-flag helpers (`sqlcommon`).
- Project mutation / init updates across SQLite, Postgres, MySQL, MariaDB, Mongo, D1.

## [1.0.16] — 2026-08-11

### Notes

- Tag already pinned by engine Studio prep. Changelog catch-up.

## [1.0.15] — 2026-08-07

### System project_users

- **`oauth_sub`** ALTER + indexes on system `project_users` (Bun
  `CreateTable().IfNotExists()` never adds columns). Fixes SaaS
  `createUser` / Pro uniqueness SELECTs failing with
  `no such column: pu.oauth_sub`.
- UpdateUser columns include `oauth_sub` (SQL + Mongo).

## [1.0.14] — 2026-08-06

### Project auth users

- **Fix:** run `ALTER … ADD COLUMN oauth_sub` **before** indexes that reference it (existing SQLite DBs were failing `loginUser` with `no such column: oauth_sub`).

## [1.0.13] — 2026-08-05

### Project auth users

- **`oauth_sub`** column + indexes on SQL `users` table (ALTER for existing DBs).
- **`ListProjectAuthUsersByOAuthSub`** for facebook/github/x/linkedin subject lookup.

## [1.0.10] — 2026-07-30

### Project SQL

- **SQLite aggregate helpers** — Ship `aggregateFieldNamesFromSelection`,
  `AggregateSelectExpressions`, `RootResolverAggregateQueryBuilder` that
  `AggregateDocOfProject` already called in v1.0.9. Fixes Studio module build
  (`undefined: aggregateFieldNamesFromSelection`).

## [1.0.9] — 2026-07-30

### Project SQL

- **`RenameModel`** — SQLite and Postgres use dialect-correct
  `ALTER TABLE … RENAME TO` (not MySQL `RENAME TABLE`). SQLite also renames
  relation pivot/FK artifacts. Required by engine schema `rename_model`
  publish on SQL/tenant DBs.

## [1.0.7] — 2026-07-21

### System schema

- **`system_users.access_tokens`** — `EnsureSystemUserAccessTokensColumn` ALTER for existing DBs (Bun `CreateTable().IfNotExists()` does not add columns). Required by open-core **v1.8.0** apt_ token storage; without this, bootstrap panics with `no such column: system_user.access_tokens`.

## [1.0.6] — 2026-07-18

### Apito Functions

- **Lifecycle SQL** — `function_lifecycle.go` / `function_persist.go`: revisions, deployments, invocations; sync `apito_functions` extended columns on project update across SQLite/Postgres/MySQL/MariaDB.
- **SQLite project** — query/mutation/pragma tweaks supporting function data gateway reads.

## [1.0.5]

Prior patch (see git history).
