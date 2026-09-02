# open_driver — AI Changelog

## 2026-09-02 — D1 partner_id ALTER (v1.0.21)

- **Changed:** Idempotent `ALTER TABLE pro_tenants ADD COLUMN partner_id`.
- **Why:** Cloudflare D1 tenant catalogs need the same FK SQL/SQLite already persist.
- **Affected:** `system/cloudflared1/system_methods.go`. Tagged **v1.0.21**.

## 2026-08-30 — COUNT total + ListUserProjects + ListTeams hydrate (v1.0.20)

- **Changed:** `SearchSystemUsers` Total from COUNT; `ListUserProjects`;
  SQLite ListTeams fills role/permissions from `user_projects`.
- **Why:** Accounts page showed page size as total; Teams Role column
  was blank after Arango migrate.
- **Affected:** `system/sqlcommon/system_user_search.go`,
  `system/sqlite/functions.go`. Tagged **v1.0.20**.

## 2026-08-22 — is_super_admin column + unscoped search (1.0.19 unreleased)

- **Changed:** `EnsureSystemUserIsSuperAdminColumn`; bootstrap
  `admin@apito.io` sets `IsSuperAdmin`; `RepairSuperAdminEmails` on
  `EnsureSystemBootstrap`. Unscoped `SearchSystemUsers` returns empty
  unless `AllowUnscopedSystemUserList` or an email/name filter.
- **Why:** Flag must persist independently of `IsAdmin`; catalog dump
  was a privilege hole.
- **Affected:** `system/sqlcommon/is_super_admin_column.go`,
  `system/bootstrapmeta/super_admin.go`, SQL/mongo/bbolt/d1/arango
  bootstrap. Next tag **v1.0.19** (do not overwrite tagged v1.0.18).

## 2026-08-14 — SearchSystemUsers email eq (v1.0.18)

- **Changed:** sqlite/postgres/mysql/mariadb `SearchSystemUsers` filters `where.email.eq` via `sqlcommon.EmailEqFromParams`. Tag **v1.0.18**.
- **Why:** Unfiltered search made console Google OAuth treat `Results[0]` as the login user (bootstrap admin).
- **Affected:** `system/sqlcommon/email_eq.go`, `system/{sqlite,postgres,mysql,mariadb}/functions.go`. Engine pin → v1.0.18.

## 2026-08-08 — v1.0.15 on Studio prod

- **Changed:** Tag **v1.0.15** live via engine **v2.4.31** Studio deploy.
- **Why:** Close SaaS createUser `pu.oauth_sub` gap on production.

## 2026-08-07 — v1.0.15 project_users.oauth_sub

- **Changed:** `EnsureProjectUsersOAuthSubColumn` on system SQL migrate;
  UpdateUser includes `oauth_sub`. Pro `ensureProUsersSQLMigration` also
  calls it.
- **Why:** SaaS `createUser` SELECT/INSERT on `project_users` (`pu`) failed
  `no such column: pu.oauth_sub` — only project DB `users` was migrated in
  v1.0.13/14.
- **Affected:** `system/sqlcommon/project_users_oauth_sub.go`, system
  init/users, mongodb UpdateUser, pro sqlpro. Tag **v1.0.15** pending.

## 2026-08-06 — v1.0.13 oauth_sub + migrate order fix

- **Changed:** `oauth_sub` column/indexes; `ListProjectAuthUsersByOAuthSub`;
  ALTER ADD COLUMN **before** indexes that reference it. Regression test
  on legacy schema without column.
- **Why:** Android `loginUser` failed `no such column: oauth_sub` —
  CREATE INDEX ran before ALTER on existing SQLite.
- **Affected:** `project/projectauthusers/sql.go`, `sql_test.go`. Tag
  **v1.0.13** pending confirm.

## 2026-08-05 — v1.0.12 list ORDER BY from sort arg

- **Changed:** `sqlquery.RootListOrderByFromArgs` applies GraphQL `sort` for
  FieldIsSortable columns; wired into sqlite/sqlquery/postgres/mysql/mariadb
  list builders. Falls back to created_at default.
- **Why:** Sort was schema-only; SQLite always ordered by created_at.
- **Affected:** `project/sqlquery/root_list_order_by.go`, query builders.
  Requires open-core **v1.8.8** FieldIsSortable.

---
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
