# External SQL connections

## Purpose and ownership

Own connection metadata, Vault references, permission classification, execution, pools and protected SQLite imports. Agent dispatch, admin APIs, UI and tool manuals share this contract.

## Contracts

- Connection creation/deletion, endpoints, credentials and permission changes are administrator-only. Agent management may update descriptions only. Legacy model arguments cannot widen this boundary.
- Parse exactly one statement using the connection dialect without interpreting comment markers inside literals. Only SQLite treats brackets as identifier quoting; PostgreSQL arrays remain syntax. Unsupported escape modes, executable comments, dollar syntax outside PostgreSQL, custom typed literals/casts and unknown read functions fail closed. Enforce each required operation permission and run SELECT through a prepared read-only transaction; SQLite uses a separate mode=ro/query_only handle.
- Read-only transactions are not a sandbox for privileged server extensions. Require least-privilege database accounts and reject file/export/administrative SQL. The read path uses a function allowlist (`validateReadStructure`); write and DDL statements use a denylist (`validateWriteStructure`, applied to every write/DDL branch of `detectStatementType`) that keeps all ordinary functions but refuses server-reaching functions and statement forms (e.g. `pg_read_file`, `LOAD_FILE`, `load_extension`, `INTO OUTFILE`, `CREATE EXTENSION`, `ALTER SYSTEM`, and a GRANT of a file/program role such as `pg_read_server_files`). The write check re-normalises the query in write-padded mode (`sqlStructureWrite`) so quoted/glued tokens, schema-prefixed names (compared by last dotted segment), `CREATE OR REPLACE`/`DEFINER=` clauses and `U&"…"` identifiers cannot smuggle a denied name past it; the read path's normalisation is byte-identical to before. Covered by `TestWriteStatementsRejectFileAndAdminSQL`. Residual: a table/column named exactly `copy`, `outfile` or `dumpfile` cannot be written through the tool.
- SQLite database_name is an opaque managed ID after explicit admin upload/import. Never open a legacy raw path. Report sqlite_import_required until a consistent standalone backup passes bounded integrity checking. Preserve the previous generation and a private metadata backup, sync staging, then publish atomically; failed/cancelled imports leave the prior generation active.
- sql_databases lives beside the protected metadata database, outside agent file roots. DSNs and credentials never enter model output. Use typed PostgreSQL/MySQL DSN builders.

## Verification

Run SQL connection package tests, agent SQL dispatch tests, server SQL API tests and the SQL Config browser flow. Exercise PostgreSQL/MySQL read-only behavior against isolated databases when available; local SQLite or parser tests do not establish external database acceptance.

## Child DOX Index

None.
