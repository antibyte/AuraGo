# SQL Query Tool Manual

## Tool Name: `sql_query`

## Purpose
Execute SQL queries against registered external database connections. Supports PostgreSQL, MySQL/MariaDB, and SQLite databases. Each connection has granular permissions (read/write/change/delete) that are enforced per query.

## Operations

### `query`
Execute a SQL statement against a database connection.
- **connection_name** (required): Name of the registered database connection.
- **sql_query** (required): SQL statement to execute.
- Only single statements allowed (no semicolon-separated chains).
- Permissions are checked: SELECT requires read, INSERT requires write, UPDATE requires change, DELETE requires delete permission.
- Results are limited to the configured maximum rows (default: 1000).

### `describe`
Get the column structure of a specific table.
- **connection_name** (required): Name of the registered database connection.
- **table_name** (required): Name of the table to describe.
- Returns column names, types, nullable, defaults, and key information.

### `list_tables`
List all tables in a database.
- **connection_name** (required): Name of the registered database connection.
- Returns an array of table names.

## Permission Model
Each connection has four independent permission flags:
- **allow_read**: Permits SELECT, SHOW, DESCRIBE, EXPLAIN, PRAGMA queries.
- **allow_write**: Permits INSERT statements and DDL (CREATE, ALTER).
- **allow_change**: Permits UPDATE statements and DDL.
- **allow_delete**: Permits DELETE and TRUNCATE statements.

## Best Practices
- Always use `list_tables` first to discover available tables.
- Use `describe` to understand table structure before writing queries.
- Use parameterized-style queries where possible (avoid string concatenation for user data).
- For large result sets, add LIMIT clauses to avoid hitting the row cap.
- When a connection has read-only permissions, only SELECT queries will work.

## Examples
```
// List all tables in a PostgreSQL connection
{"operation": "list_tables", "connection_name": "app_db"}

// Describe a table
{"operation": "describe", "connection_name": "app_db", "table_name": "users"}

// Run a SELECT query
{"operation": "query", "connection_name": "app_db", "sql_query": "SELECT id, name, email FROM users WHERE active = true LIMIT 50"}

// Insert a row (requires write permission)
{"operation": "query", "connection_name": "app_db", "sql_query": "INSERT INTO logs (message, level) VALUES ('Agent action', 'info')"}
```
