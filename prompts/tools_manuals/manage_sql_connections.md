# SQL Connections (`manage_sql_connections`)

Manage external database connections that the agent can query. Create, update, delete, list, and test connections to PostgreSQL, MySQL/MariaDB, and SQLite databases. Credentials are stored securely in the encrypted vault.

## Operations

| Operation | Description |
|-----------|-------------|
| `list` | List all registered database connections |
| `get` | Get details of a specific connection |
| `create` | Register a new database connection |
| `update` | Update an existing connection |
| `delete` | Remove a database connection and its vault credentials |
| `test` | Test connectivity to a registered database |
| `docker_create` | Prepare a Docker container configuration for a new database |

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `operation` | string | yes | One of: list, get, create, update, delete, test, docker_create |
| `connection_name` | string | for get, create, update, delete, test, docker_create | Unique name for the connection |
| `driver` | string | for create | Database type: `postgres`, `mysql`, or `sqlite` |
| `host` | string | for create | Database host/IP (not needed for sqlite) |
| `port` | integer | for create | Database port (default: 5432 postgres, 3306 mysql) |
| `database_name` | string | for create | Database name or SQLite file path |
| `description` | string | for create | Short description |
| `username` | string | for create | Database username (stored in vault) |
| `password` | string | for create | Database password (stored in vault) |
| `ssl_mode` | string | for create | SSL mode: `disable`, `require`, `verify-ca`, `verify-full` |
| `credential_action` | string | for update | Credential handling for updates: `keep`, `replace`, or `delete` |
| `allow_read` | boolean | for create | Allow SELECT queries (default: true) |
| `allow_write` | boolean | for create | Allow INSERT queries (default: false) |
| `allow_change` | boolean | for create | Allow UPDATE queries (default: false) |
| `allow_delete` | boolean | for create | Allow DELETE queries (default: false) |
| `docker_template` | string | for docker_create | Template: `postgres`, `mysql`, or `mariadb` |

## Examples

**List all connections:**
```json
{"action": "manage_sql_connections", "operation": "list"}
```

**Create a PostgreSQL connection:**
```json
{"action": "manage_sql_connections", "operation": "create", "connection_name": "app_db", "driver": "postgres", "host": "192.168.1.100", "port": 5432, "database_name": "myapp", "description": "Main application database", "username": "admin", "password": "secret123", "allow_read": true, "allow_write": true}
```

**Test a connection:**
```json
{"action": "manage_sql_connections", "operation": "test", "connection_name": "app_db"}
```

**Delete stored credentials but keep the connection:**
```json
{"action": "manage_sql_connections", "operation": "update", "connection_name": "app_db", "credential_action": "delete"}
```

**Create a database via Docker:**
```json
{"action": "manage_sql_connections", "operation": "docker_create", "connection_name": "test_pg", "docker_template": "postgres", "database_name": "testdb"}
```

**Delete a connection:**
```json
{"action": "manage_sql_connections", "operation": "delete", "connection_name": "old_db"}
```

## Workflow: Creating a Docker Database

1. Use `docker_create` to get the container configuration
2. Use the `docker` tool with `run` operation to start the container
3. Wait a few seconds for the database to initialize
4. Use `create` to register the connection (credentials are in the docker_create output)
5. Use `test` to verify connectivity

## Notes

- **Credentials security**: Usernames and passwords are stored in the encrypted vault, never in plain text
- **Credential updates**: Use `credential_action="replace"` together with `username` / `password`, or `credential_action="delete"` to remove stored credentials without deleting the connection
- **Permission model**: Each connection has granular permissions (read, write, change, delete)
- **SQLite**: For sqlite, use the file path as `database_name` and omit `host` and `port`
- **Docker templates**: The `docker_create` operation returns configuration ready to use with the `docker` tool
