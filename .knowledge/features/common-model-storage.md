---
type: feature
title: Common Model Storage
description: Id-only tenant shadows, deleted-row filter, SQLite reduce that keeps FKs
resource: project/sqlite/model_ddl.go
tags: [open_driver, sqlite, saas]
timestamp: 2026-09-07T00:00:00Z
---

# Common Model Storage

## Purpose

SQL drivers store common-model **documents in the base DB** and **id-only shadows in tenant DBs**. SQLite reduce is the certified migration; other SQL compiles.

## Flows

- **List/count:** `sqlquery` joins `meta` and keeps rows where status is null or not `deleted`.
- **GetSingle:** `HideDeletedCommonDocument` / `DocumentIsSoftDeleted`.
- **Shadows:** `sqlcommon.ListDistinctNonEmptyColumn`, `InsertShadowIDs` (inline SQL — do not import `sqlquery` from `sqlcommon`, cycle).
- **Reduce (SQLite):** record `PRAGMA foreign_keys`, OFF, copy IDs to tmp, verify counts, drop source only after verify, rename, recreate id-only indexes from `sqlite_master`, restore pragma on **every** exit (`defer`). Child FKs (e.g. `tenant_record.catalog_item_id`) stay.

## Main files

- `project/sqlquery/query_builder.go`, `common_deleted.go`, `common_shadow*.go`
- `project/sqlcommon/shadow.go`
- `project/sqlite/model_ddl.go`, `reduce_shadow_test.go`
- postgres/mysql/mariadb GetSingle hide-deleted (compile; not E2E-certified)

## Invariants

- Missing shadow table → empty list, not error.
- Verification failure leaves source data.
- `ProSQLiteDriver` embeds `*sqlite.Driver` so shadow methods promote.

## Common bugs

- `sqlcommon` importing `sqlquery` (cycle).
- bun `RawQuery` has no `.Query` — use `.Scan`.
- Leaving `PRAGMA foreign_keys=OFF` after a failed reduce.

## Tests

`go test ./project/sqlquery/ ./project/sqlite/`

## Related

- `apito/.knowledge/features/common-models.md`
- open-core `repairCommonShadows`
