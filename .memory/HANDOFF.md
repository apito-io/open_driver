# open_driver — Handoff

- **2026-10-13:** Local `.env` ports are engine `5055` and console origin `4055`. Restart the process that loads this file before a local connect.
- **Do not:** Commit if this file's secrets should stay out of a casual diff review. `./command.sh save` until asked.
- **2026-10-11:** Public Cloudflare D1 no longer creates `pro_projects` or `pro_tenants`. Those tables belong to `pro_driver/system/procloudflared1`. `cmd/oss` does not import the private engine, nats, or plugin modules.
- **Next:** Do not tag. Drop the local `replace` only on a real release tag.
- **Do not:** Put host catalog DDL back into `system/cloudflared1`. `./command.sh save` until asked.
- **2026-10-11:** SQL cardinality migration is in `sqlquery`. SQLite execution tests passed. Postgres/MySQL/MariaDB share the functions; they were not run against live servers. D1 fails closed. Mongo and bbolt do not pretend SQL ran.
- **Next:** Engine restart, then a local Prottoy preview. Do not tag. Do not `./command.sh save` until asked.
- **2026-10-05:** `ApplyStaleOneToOne` drops `idx_<table>_<fk>_unique` only. Both sides still `has_one` drops nothing. Shared SaaS prefixes `tenant_id` except common and tenant-catalogue models. Duplicate rows fail index create.
- **Next:** Engine rebuild before Suchok dry-run. Do not tag. Do not `./command.sh save` until asked.
- **2026-09-23:** `project/sqlite` food-order trace only. Restart engine `:5050` before a local order. Do not tag. Do not copy the logs into mysql/postgres.
- Tag **v1.0.25** for common-model storage.
- Do not import `sqlquery` from `sqlcommon` (cycle).
- SQLite reduce restores `PRAGMA foreign_keys` on every exit.
- PG/MySQL/MariaDB hide-deleted compiles; not E2E-certified.

## Last Updated
2026-09-07
