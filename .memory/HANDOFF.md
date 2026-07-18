# open_driver — Handoff

## Branch
- Tracks open-core via go.mod replace in monorepo

## Done
- **Function persistence (2026-07-16):** `sqlcommon/function_persist.go`, `schema_models.go` lifecycle tables, driver init/migration hooks, upsert fields (`language`, `binary_url`, `source`, `trigger_type`, `capabilities`). Test: `system/sqlite/function_persist_test.go` green. Uncommitted.

## Broken / watch
- No driver-level transaction batch API exposed to functions gateway yet

## Next
- Batch executor + idempotency persistence on SQLite project DB

## Do not touch
- Hand-edit generated schema migrations without matching open-core model changes

## Last Updated
2026-07-16
