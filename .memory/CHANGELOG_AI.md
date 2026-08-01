# open_driver — AI Changelog

Not git history — the *reasoning* behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---
## 2026-08-01 — Released v1.0.11
- **Changed:** SQLite `groupBy` aggregates (`group_by_identifier_N`), `ne`/`not_in`
  keep NULL rows (`IS NULL OR …`), list-filter element type-check, FK column
  indexes on `AddRelationFields`, relation-column ensure before groupBy.
  Tests: `filter_ne_null_test.go`, `aggregate_groupby_test.go`.
- **Why:** Suchok ledger filters dropped NULL `hidden`; returnable aggregates
  need groupBy on relation FKs. Must be tagged so Studio/non-replace engine
  builds do not stay on v1.0.10.
- **Affected:** `project/sqlite/{missing_methods,mutation,query_builder}.go` + tests.
  Engine pin → **v1.0.11**.

## 2026-07-30 — Released v1.0.10
- **Changed:** Ship aggregate helpers omitted from incomplete v1.0.9 tag.
- **Why:** Studio build failed on missing symbols from broken v1.0.9 module.
- **Affected:** sqlite aggregate helpers; engine pin skipped v1.0.9 → v1.0.10.

## 2026-07-30 — RenameModel ALTER TABLE (SQLite/Postgres)
- **Changed:** `RenameModel` uses dialect `ALTER TABLE … RENAME TO`; SQLite
  renames relation pivot/FK artifacts.
- **Affected:** Released as **v1.0.9** (incomplete) then fixed in **v1.0.10**.

---
