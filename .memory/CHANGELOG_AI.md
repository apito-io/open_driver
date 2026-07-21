# open_driver — AI Changelog

Not git history — the *reasoning* behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---
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
