# SQL Connections (`manage_sql_connections`)

Use administrator-configured PostgreSQL, MySQL/MariaDB and managed SQLite connections.

| Operation | Inputs | Effect |
| --- | --- | --- |
| `list` | none | List configured connections and grants |
| `get` | `connection_name` | Inspect one connection |
| `test` | `connection_name` | Test saved connectivity |
| `update` | `connection_name`, `description` | Change the description when `sql_connections.allow_management` is enabled |

Targets, drivers, credentials, permissions, creation and deletion are administrator-only in Config > SQL Connections. Never request credentials in model arguments. SQLite requires an explicit administrator import of a consistent standalone backup into protected storage; a host path is never an agent database target. `sqlite_import_required` means the administrator must import the database before it can be used.

Use `sql_query` for data operations. Only one supported statement is allowed. Read queries use an approved set of built-in functions and database-enforced read-only connections/transactions. Unknown functions and file/export/administrative operations are blocked. Database accounts must also have least-privilege roles: SQL read-only mode alone cannot sandbox privileged extensions.
