# open_driver — Current

## Working on
- **apt_ access_tokens column (2026-07-20):** `EnsureSystemUserAccessTokensColumn`
  in SQL migrations — required for existing DBs. Uncommitted; restart engine after apply.
- **Apito Functions SQL persistence (2026-07-16):** Composite PK `(project_id,name)`,
  lifecycle tables, sync on UpdateProject across sqlite/postgres/mysql/mariadb drivers.

## Next
- Confirm save with apt_ stack
- SQLite `ProjectBatchExecutor` on real project connection

## Last Updated
2026-07-21
