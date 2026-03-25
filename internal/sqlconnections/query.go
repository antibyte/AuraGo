package sqlconnections

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// QueryResult holds the result of a SQL query execution.
type QueryResult struct {
	Columns      []string                 `json:"columns,omitempty"`
	Rows         []map[string]interface{} `json:"rows,omitempty"`
	RowsAffected int64                    `json:"rows_affected,omitempty"`
	Message      string                   `json:"message,omitempty"`
}

// ExecuteQuery runs a SQL query on the given connection with permission checks.
func ExecuteQuery(ctx context.Context, pool *ConnectionPool, metaDB *sql.DB, connName string, query string, maxRows int, queryTimeout time.Duration) (*QueryResult, error) {
	rec, err := GetByName(metaDB, connName)
	if err != nil {
		return nil, fmt.Errorf("connection '%s' not found: %w", connName, err)
	}

	stmtType, err := DetectStatementType(query)
	if err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	if err := CheckPermission(rec, stmtType); err != nil {
		return nil, err
	}
	slog.Default().Info("SQL query executed", "connection", connName, "type", stmtType.String())

	db, err := pool.GetConnection(rec.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	if stmtType == StmtSelect {
		return executeSelect(queryCtx, db, query, maxRows)
	}
	return executeExec(queryCtx, db, query)
}

// executeSelect runs a SELECT-like query and returns columnar results.
func executeSelect(ctx context.Context, db *sql.DB, query string, maxRows int) (*QueryResult, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}
	count := 0
	for rows.Next() {
		if count >= maxRows {
			break
		}
		values := make([]interface{}, len(cols))
		scanTargets := make([]interface{}, len(cols))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			val := values[i]
			// Convert byte slices to strings for JSON serialization
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	msg := fmt.Sprintf("%d row(s) returned", len(results))
	if count >= maxRows {
		msg += fmt.Sprintf(" (limited to %d)", maxRows)
	}

	return &QueryResult{
		Columns: cols,
		Rows:    results,
		Message: msg,
	}, nil
}

// executeExec runs an INSERT/UPDATE/DELETE/DDL statement.
func executeExec(ctx context.Context, db *sql.DB, query string) (*QueryResult, error) {
	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("execution error: %w", err)
	}
	affected, rowErr := result.RowsAffected()
	if rowErr != nil {
		slog.Default().Warn("failed to retrieve rows affected", "error", rowErr)
	}
	return &QueryResult{
		RowsAffected: affected,
		Message:      fmt.Sprintf("%d row(s) affected", affected),
	}, nil
}

// ListTables returns all table names for a connection.
func ListTables(ctx context.Context, pool *ConnectionPool, metaDB *sql.DB, connName string, queryTimeout time.Duration) ([]string, error) {
	rec, err := GetByName(metaDB, connName)
	if err != nil {
		return nil, fmt.Errorf("connection '%s' not found: %w", connName, err)
	}

	if !rec.AllowRead {
		return nil, fmt.Errorf("read permission denied on connection '%s'", connName)
	}

	db, err := pool.GetConnection(rec.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var query string
	switch rec.Driver {
	case "postgres":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name"
	case "mysql":
		query = "SHOW TABLES"
	case "sqlite":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	default:
		return nil, fmt.Errorf("unsupported driver: %s", rec.Driver)
	}

	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// DescribeTable returns column metadata for a specific table.
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable string `json:"nullable"`
	Default  string `json:"default,omitempty"`
	Key      string `json:"key,omitempty"`
}

// DescribeTable returns column info for a table on the given connection.
func DescribeTable(ctx context.Context, pool *ConnectionPool, metaDB *sql.DB, connName string, tableName string, queryTimeout time.Duration) ([]ColumnInfo, error) {
	rec, err := GetByName(metaDB, connName)
	if err != nil {
		return nil, fmt.Errorf("connection '%s' not found: %w", connName, err)
	}

	if !rec.AllowRead {
		return nil, fmt.Errorf("read permission denied on connection '%s'", connName)
	}

	// Sanitize table name to prevent injection
	if !isValidIdentifier(tableName) {
		return nil, fmt.Errorf("invalid table name: %s", tableName)
	}

	db, err := pool.GetConnection(rec.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	switch rec.Driver {
	case "postgres":
		return describePostgres(queryCtx, db, tableName)
	case "mysql":
		return describeMySQL(queryCtx, db, tableName)
	case "sqlite":
		return describeSQLite(queryCtx, db, tableName)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", rec.Driver)
	}
}

func describePostgres(ctx context.Context, db *sql.DB, table string) ([]ColumnInfo, error) {
	query := `SELECT column_name, data_type, is_nullable, column_default,
		CASE WHEN pk.column_name IS NOT NULL THEN 'PRI' ELSE '' END as key
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT ku.column_name FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku ON tc.constraint_name = ku.constraint_name
			WHERE tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.column_name = pk.column_name
		WHERE c.table_name = $1 AND c.table_schema = 'public'
		ORDER BY c.ordinal_position`
	rows, err := db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanColumnInfo(rows)
}

func describeMySQL(ctx context.Context, db *sql.DB, table string) ([]ColumnInfo, error) {
	quotedTable := "`" + strings.ReplaceAll(table, "`", "``") + "`"
	rows, err := db.QueryContext(ctx, "DESCRIBE "+quotedTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var field, colType, null, key string
		var defVal, extra sql.NullString
		if err := rows.Scan(&field, &colType, &null, &key, &defVal, &extra); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:     field,
			Type:     colType,
			Nullable: null,
			Default:  defVal.String,
			Key:      key,
		})
	}
	return cols, rows.Err()
}

func describeSQLite(ctx context.Context, db *sql.DB, table string) ([]ColumnInfo, error) {
	quotedTable := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+quotedTable+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defVal sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defVal, &pk); err != nil {
			return nil, err
		}
		nullable := "YES"
		if notNull == 1 {
			nullable = "NO"
		}
		keyStr := ""
		if pk > 0 {
			keyStr = "PRI"
		}
		cols = append(cols, ColumnInfo{
			Name:     name,
			Type:     colType,
			Nullable: nullable,
			Default:  defVal.String,
			Key:      keyStr,
		})
	}
	return cols, rows.Err()
}

func scanColumnInfo(rows *sql.Rows) ([]ColumnInfo, error) {
	var cols []ColumnInfo
	for rows.Next() {
		var name, colType, nullable string
		var defVal, key sql.NullString
		if err := rows.Scan(&name, &colType, &nullable, &defVal, &key); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:     name,
			Type:     colType,
			Nullable: nullable,
			Default:  defVal.String,
			Key:      key.String,
		})
	}
	return cols, rows.Err()
}

// isValidIdentifier checks that a table name only contains safe characters.
func isValidIdentifier(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for i, r := range name {
		if r == '_' || r == '.' {
			continue
		}
		if i == 0 && !isLetter(r) {
			return false
		}
		if !isLetter(r) && !isDigit(r) {
			return false
		}
	}
	return true
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
