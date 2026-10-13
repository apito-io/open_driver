# Apito open_driver

Apache 2.0 database, cache, and key-value drivers for the Apito engine.

Module path: `github.com/apito-io/open_driver`. This module is Apache 2.0 now. It is not Fair Source.

## Driver matrix

| Kind | Engines |
| --- | --- |
| Project / system SQL | BBolt, SQLite, PostgreSQL, MySQL, MariaDB |
| Document | MongoDB |
| Cache | memory, Redis |
| Key-value | memory, Redis |
| Realtime | in-process memory bus |

The self-hosted binary is `cmd/oss`. It listens on `:5050` by default and logs `Apito engine listening`.

Functions and a networked realtime bus are an optional profile. The default binary uses the in-process bus and a plugin host that loads nothing. A host binary can inject those factories at boot.

Release tags drop unpublished `replace` directives. The monorepo working copy keeps local `replace` lines so this tree builds next to the engine.
