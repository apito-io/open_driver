# open_driver — Handoff

- **2026-09-23:** `project/sqlite` food-order trace only. Restart engine `:5050` before a local order. Do not tag. Do not copy the logs into mysql/postgres.
- Tag **v1.0.25** for common-model storage.
- Do not import `sqlquery` from `sqlcommon` (cycle).
- SQLite reduce restores `PRAGMA foreign_keys` on every exit.
- PG/MySQL/MariaDB hide-deleted compiles; not E2E-certified.

## Last Updated
2026-09-07
