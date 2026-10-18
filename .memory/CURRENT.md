# open_driver — Current

- **2026-10-18:** README now says `go get github.com/apito-io/open_driver` and that the engine and Studio use this module. License stays Apache-2.0. Origin is still GitLab. Uncommitted. Leave `.memory` out of a public push. Ask before `./command.sh save`.
- **2026-10-14:** Tag **v1.0.28** publishes `module github.com/apito-io/open_driver` and is what engine v2.5.3 requires. The GitHub tag exists for `go get`. Ask before `./command.sh save`.
- **2026-10-13:** Local `.env` listens on `5055` and allows the console at `http://localhost:4055`. No driver code changed. Ask before `./command.sh save`.
- **2026-10-11:** Public D1 `RunMigration` creates `projects`, `system_users`, `project_schemas`, and `model_types` only. `pro_projects` and `pro_tenants` are not in this driver. `cmd/oss` builds a self-hosted binary with memory realtime and a no-op plugin host. LICENSE stays Apache 2.0. Not tagged. Ask before `./command.sh save`.
- **2026-10-11:** `project/sqlquery/relation_migrate.go` copies relation pairs, verifies the set, then drops obsolete FK columns or the pivot. SQLite tests passed for widen, blocked narrow, ownership flip, 1:1 unique-index drop, and idempotent retry. Postgres, MySQL, and MariaDB call that migrator. D1 returns unsupported. Mongo and bbolt are metadata-only. Not tagged. Ask before `./command.sh save`.
- **2026-10-05:** Relation ensure creates composite unique indexes and drops a stale `has_one`/`has_one` unique index without dropping the column. SQLite signature includes `unique_keys`, so a new key is not skipped by the cache. Postgres and MySQL also drop the column UNIQUE constraint. `TestEnsureRelation_oneToOneThenCompositeUnique` passed. Not tagged. Ask before `./command.sh save`.
- **2026-09-23:** SQLite `food_order` debug logs on insert, relation DDL, write-lock wait, and each connect SQL. No index change. Not tagged.
- **v1.0.25** — common-model shadows, hide-deleted SQL, SQLite
  verified reduce (FK/index/pragma). Engine **v2.4.53**.
- Prior live: **v1.0.24** with engine **v2.4.52**.

## Last Updated
2026-10-13
