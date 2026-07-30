# open_driver — AI Changelog

Not git history — the *reasoning* behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---
## 2026-07-30 — Released v1.0.9
- **Changed:** Tagged+pushed **v1.0.9** (RenameModel ALTER TABLE). Engine
  require bumped to v1.0.9.
- **Why:** Studio/non-replace builds must not stay on v1.0.8.
- **Affected:** GitLab `open_driver` tag v1.0.9; engine go.mod pin.

## 2026-07-30 — RenameModel ALTER TABLE (SQLite/Postgres)
- **Changed:** `RenameModel` uses dialect `ALTER TABLE … RENAME TO`; SQLite
  renames relation pivot/FK artifacts; unit test
  `TestRenameModel_SQLiteAlterTableSyntax`. Commit `8036e9c` (local).
- **Why:** MySQL `RENAME TABLE` syntax broke tenant SQLite publish
  (`near "RENAME"`) for schema `rename_model`.
- **Affected:** `project/sqlite/missing_methods.go`,
  `project/postgres/missing_methods.go`, `project/sqlite/rename_model_test.go`.
  Released as **v1.0.9**; engine require bumped.

## 2026-07-20 — Ensure system_user.access_tokens column
- **Changed:** `sqlcommon.EnsureSystemUserAccessTokensColumn`; wired into
  sqlite/postgres/mysql/mariadb `RunMigration`.
- **Why:** Bun `CreateTable().IfNotExists()` does not ALTER existing DBs; engine
  panicked `no such column: system_user.access_tokens` after apt_ tokens landed.
- **Affected:** `system/sqlcommon/access_tokens_column.go`, SQL driver `init.go`
  migrations. Uncommitted — restart engine after migrate.

## 2026-07-16 — Apito Functions SQL persistence
- **Changed:** `PersistProjectFunctionsTx`, `EnsureApitoFunctionsCompositePK`, lifecycle table registration; all SQL system drivers sync `apito_functions` on project update; extended upsert columns.
- **Why:** Functions were never hydrated in SQL drivers — multi-function projects broke invoke and console list; composite key matches tenant-safe identity `(project_id,name)`.
- **Affected:** `system/sqlcommon/function_persist.go`, `system/sqlcommon/schema_models.go`, `system/sqlite|postgres|mysql|mariadb/*`, `system/sqlite/function_persist_test.go`.

## 2026-07-06
- **Changed:** Bootstrapped knowledge system for this repo.
- **Why:** Cross-LLM durable knowledge + working memory.
- **Affected:** this repo only.

Last Updated: 2026-07-16
