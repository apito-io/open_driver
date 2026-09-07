# open_driver — Knowledge

Part of the **apito** ecosystem. See `/.knowledge/projects/apito.md` for how this repo fits and its blast radius.

## Read order
1. This file. 2. `ARCHITECTURE.md`. 3. `DECISIONS.md`. 4. `features/README.md`. 5. `../.memory/CURRENT.md` and `../.memory/HANDOFF.md`.

## Purpose
OSS project/system database drivers (`gitlab.com/apito.io/open_driver`): SQLite, Postgres, MySQL, MariaDB, plus non-SQL stores. Pro drivers wrap these.

## Responsibilities
- Physical DDL, CRUD, relation pivots, id-only common-model shadows (`features/common-model-storage.md`)

## Consumers / blast radius
open-core via interfaces; pro_driver embed; engine pins the tag. Studio clone must `go mod download` the published tag, not local `replace`.

## Reasoning archive
- Historical Cursor plans distilled into this knowledge base live in `archive/plans/`.

Last Updated: 2026-07-06
