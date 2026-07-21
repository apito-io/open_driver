# open_driver changelog

## [1.0.7] — 2026-07-21

### System schema

- **`system_users.access_tokens`** — `EnsureSystemUserAccessTokensColumn` ALTER for existing DBs (Bun `CreateTable().IfNotExists()` does not add columns). Required by open-core **v1.8.0** apt_ token storage; without this, bootstrap panics with `no such column: system_user.access_tokens`.

## [1.0.6] — 2026-07-18

### Apito Functions

- **Lifecycle SQL** — `function_lifecycle.go` / `function_persist.go`: revisions, deployments, invocations; sync `apito_functions` extended columns on project update across SQLite/Postgres/MySQL/MariaDB.
- **SQLite project** — query/mutation/pragma tweaks supporting function data gateway reads.

## [1.0.5]

Prior patch (see git history).
