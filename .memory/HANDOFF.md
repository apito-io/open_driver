# open_driver — Handoff

## Branch

- `main` ahead 1: `8036e9c` RenameModel ALTER TABLE fix.

## Done

- **2026-07-30:** Dialect-correct `RenameModel` for SQLite + Postgres;
  `project/sqlite/rename_model_test.go`. Pairs with engine
  `changelogs/2026-07-30.md`.

## Dirty / watch

- Uncommitted: `mutation.go`, `query_builder.go`, `aggregate_test.go`,
  `system/sqlite/:memory:.sqlite` — exclude from rename push.

## Next

1. User confirm → push `8036e9c`
2. Studio/engine consume this pin for SQL tenant rename publish

## Do not touch

- Hand-edit generated schema migrations without matching open-core model changes

## Last Updated

2026-07-30
