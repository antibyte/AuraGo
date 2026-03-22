# Manage SQL Connections Tool Manual

## Tool Name: `manage_sql_connections`

## Purpose
Manage external database connections that the agent can query. Create, update, delete, list, and test database connections to PostgreSQL, MySQL/MariaDB, and SQLite databases. Credentials are stored securely in the encrypted vault.

## Operations

### `list`
List all registered database connections.
- No parameters required.
- Returns connection metadata (name, driver, host, port, database, description, permissions).
- Credentials are never exposed in the listing.

### `get`
Get details of a specific connection.
- **connection_name** (required): Name of the connection.

### `create`
Register a new database connection.
- **connection_name** (required): Unique name for the connection.
- **driver** (required): Database type — `postgres`, `mysql`, or `sqlite`.
- **host** (optional): Database host/IP (not needed for sqlite).
- **port** (optional): Database port (defaults: 5432 for postgres, 3306 for mysql).
- **database_name** (optional): Database name or SQLite file path.
- **description** (optional): Short description of what the database is for.
- **username** (optional): Database username (stored in vault).
- **password** (optional): Database password (stored in vault).
- **ssl_mode** (optional): SSL mode — `disable` (default), `require`, `verify-ca`, `verify-full`.
- **allow_read** (optional): Allow SELECT queries. Default: true.
- **allow_write** (optional): Allow INSERT queries. Default: false.
- **allow_change** (optional): Allow UPDATE queries. Default: false.
- **allow_delete** (optional): Allow DELETE queries. Default: false.

### `update`
Update an existing connection. Only provided fields are changed.
- **connection_name** (required): Name of the connection to update.
- All other fields from `create` are optional — only provided values are updated.

### `delete`
Remove a database connection and its vault credentials.
- **connection_name** (required): Name of the connection to delete.

### `test`
Test connectivity to a registered database.
- **connection_name** (required): Name of the connection to test.
- Opens a temporary connection, pings, and reports success/failure.

### `docker_create`
Prepare a Docker container configuration for a new database.
- **connection_name** (required): Name for the new connection.
- **docker_template** (required): Template — `postgres`, `mysql`, or `mariadb`.
- **database_name** (optional): Database name (defaults to connection_name).
- Returns a Docker configuration that can be used with the `docker` tool to start the container.
- After starting the container, create the connection with the `create` operation.

## Workflow: Creating a Docker Database
1. Use `docker_create` to get the container configuration.
2. Use the `docker` tool with `run` operation to start the container.
3. Wait a few seconds for the database to initialize.
4. Use `create` to register the connection (credentials are in the docker_create output).
5. Use `test` to verify connectivity.

## Examples
```
// List all connections
{"operation": "list"}

// Create a PostgreSQL connection
{"operation": "create", "connection_name": "app_db", "driver": "postgres", "host": "192.168.1.100", "port": 5432, "database_name": "myapp", "description": "Main application database", "username": "admin", "password": "secret123", "allow_read": true, "allow_write": true}

// Test a connection
{"operation": "test", "connection_name": "app_db"}

// Create a database via Docker
{"operation": "docker_create", "connection_name": "test_pg", "docker_template": "postgres", "database_name": "testdb"}

// Delete a connection
{"operation": "delete", "connection_name": "old_db"}
```
