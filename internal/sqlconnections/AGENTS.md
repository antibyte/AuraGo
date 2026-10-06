# External SQL connections

## Purpose and ownership

Own connection metadata, Vault references, permission classification, execution, pools and protected SQLite imports. Agent dispatch, admin APIs, UI and tool manuals share this contract.

## Contracts

- Connection creation/deletion, endpoints, credentials and permission changes are administrator-only. Agent management may update descriptions only. Legacy model arguments cannot widen this boundary.
- Parse exactly one statement using the connection dialect without interpreting comment markers inside literals. Only SQLite treats brackets as identifier quoting; PostgreSQL arrays remain syntax. Unsupported escape modes, executable comments, dollar syntax outside PostgreSQL, custom typed literals/casts and unknown read functions fail closed. Enforce each required operation permission and run SELECT through a prepared read-only transaction; SQLite uses a separate mode=ro/query_only handle.
- Read-only transactions are not a sandbox for privileged server extensions. Require least-privilege database accounts and reject file/export/administrative SQL. The read path uses a function allowlist (`validateReadStructure`, in `sql_lexer.go`); write and DDL statements use a denylist (`validateWriteStructure`, in `sql_write_denylist.go`). `detectStatementType` classifies first (`classifyStatement`) then applies the write denylist once, through a single gate on "not a read" (`typ != StmtSelect`), so every current and future write/DDL branch is covered. The denylist keeps all ordinary functions but refuses server-reaching functions and statement forms (e.g. `pg_read_file`, `pg_file_write`, `lo_import`, the `dblink` family, `LOAD_FILE`, `load_extension`, `writefile`, `INTO OUTFILE`, `CREATE EXTENSION`/`FUNCTION`/`OPERATOR`, `ALTER SYSTEM`, `COPY`, and a GRANT of a privileged role such as `pg_read_server_files` or `pg_read_all_data`). The write check re-normalises the query in write-padded mode (`sqlStructureWrite`) and tokenises on identifier boundaries so quoted/glued tokens, schema-prefixed names (compared by last dotted segment), `CREATE OR REPLACE`/`DEFINER=`/`ALGORITHM=` prefixes (stripped only in the CREATE prefix) and `U&"…"` identifiers cannot smuggle a denied name past it; `COPY` and `INTO OUTFILE`/`INTO DUMPFILE` are position-matched so ordinary tables/columns named `copy`, `outfile` or `dumpfile` still write. The read path's normalisation (`pad=false`) is byte-identical to before. Covered by `TestWriteStatementsRejectFileAndAdminSQL`, which also asserts every denylist entry is live.
- SQLite database_name is an opaque managed ID after explicit admin upload/import. Never open a legacy raw path. Report sqlite_import_required until a consistent standalone backup passes bounded integrity checking. Preserve the previous generation and a private metadata backup, sync staging, then publish atomically; failed/cancelled imports leave the prior generation active.
- Creating a PostgreSQL/MySQL connection requires an explicit `ssl_mode` (`ErrSSLModeRequired`, checked before any Vault write; the API returns its message with 400). `disable` is accepted and meant for local containers (Docker templates set it); SQLite keeps the stored `disable` default. Updates without `ssl_mode` and stored rows are left unchanged.
- sql_databases lives beside the protected metadata database, outside agent file roots. DSNs and credentials never enter model output. Use typed PostgreSQL/MySQL DSN builders.

## Verification

Run SQL connection package tests, agent SQL dispatch tests, server SQL API tests and the SQL Config browser flow. Exercise PostgreSQL/MySQL read-only behavior against isolated databases when available; local SQLite or parser tests do not establish external database acceptance.

## Child DOX Index

None.
