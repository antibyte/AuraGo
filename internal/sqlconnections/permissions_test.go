package sqlconnections

import (
	"fmt"
	"testing"
)

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		contains string
		excludes []string
	}{
		{
			name:     "nil error returns generic message",
			input:    nil,
			contains: "unknown error",
		},
		{
			name:     "driver error suffix is stripped",
			input:    fmt.Errorf("failed to connect: driver error: pq: connection refused"),
			contains: "failed to connect",
			excludes: []string{": driver", "pq:"},
		},
		{
			name:     "postgres pq prefix is stripped",
			input:    fmt.Errorf("pq: permission denied for table users"),
			contains: "permission denied",
			excludes: []string{"pq:"},
		},
		{
			name:     "permission denied message is preserved",
			input:    fmt.Errorf("permission denied: connection %q does not allow SELECT (read)", "test"),
			contains: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeError(tt.input)

			if tt.contains != "" && !containsString(tt.contains, result) {
				t.Errorf("SanitizeError(%v) = %q, want to contain %q", tt.input, result, tt.contains)
			}

			for _, exclude := range tt.excludes {
				if containsString(exclude, result) {
					t.Errorf("SanitizeError(%v) = %q, should NOT contain %q", tt.input, result, exclude)
				}
			}
		})
	}
}

func containsString(substr, s string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDetectStatementType(t *testing.T) {
	tests := []struct {
		query    string
		expected StatementType
		wantErr  bool
	}{
		// Basic SELECT variants
		{"SELECT * FROM users", StmtSelect, false},
		{"SELECT id, name FROM users WHERE id = 1", StmtSelect, false},
		{"WITH cte AS (SELECT id FROM users) SELECT * FROM cte", StmtSelect, false},

		// INSERT/UPDATE/DELETE
		{"INSERT INTO users VALUES (1)", StmtInsert, false},
		{"INSERT INTO users (id, name) VALUES (1, 'test')", StmtInsert, false},
		{"REPLACE INTO users VALUES (1, 'test')", StmtInsert, false},
		{"UPDATE users SET name = 'test'", StmtUpdate, false},
		{"UPDATE users SET name = 'test' WHERE id = 1", StmtUpdate, false},
		{"DELETE FROM users", StmtDelete, false},
		{"DELETE FROM users WHERE id = 1", StmtDelete, false},

		// DDL
		{"CREATE TABLE test (id INT)", StmtDDL, false},
		{"DROP TABLE users", StmtDDL, false},
		{"ALTER TABLE users ADD col INT", StmtDDL, false},
		{"TRUNCATE TABLE users", StmtDDL, false},

		// CTE with write operations (conservative: unknown CTEs are blocked)
		{"WITH cte AS (SELECT id FROM users) INSERT INTO logs SELECT id FROM cte", StmtInsert, false},
		{"WITH cte AS (SELECT id FROM users) UPDATE users SET name='x' WHERE id IN (SELECT id FROM cte)", StmtUpdate, false},
		{"WITH cte AS (SELECT id FROM users) DELETE FROM users WHERE id IN (SELECT id FROM cte)", StmtDelete, false},

		// SHOW/DESCRIBE (read-only)
		{"SHOW TABLES", StmtSelect, false},
		{"DESCRIBE users", StmtSelect, false},
		{"DESC users", StmtSelect, false},

		// EXPLAIN (conservative: EXPLAIN SELECT only)
		{"EXPLAIN SELECT * FROM users", StmtSelect, false},
		{"EXPLAIN QUERY PLAN SELECT * FROM users", StmtSelect, false},

		// Blocked: empty, multi-statement, PRAGMA, administrative commands
		{"", StmtUnknown, true},
		{"SELECT * FROM users; SELECT * FROM orders", StmtUnknown, true},
		{"SELECT * FROM users; DROP TABLE users", StmtUnknown, true},
		{"PRAGMA table_info(users)", StmtUnknown, true},
		{"PRAGMA foreign_keys=ON", StmtUnknown, true},

		// Administrative commands (blocked)
		{"SET SESSION wait_timeout = 60", StmtUnknown, true},
		{"USE mydb", StmtUnknown, true},
		{"BEGIN", StmtUnknown, true},
		{"COMMIT", StmtUnknown, true},
		{"ROLLBACK", StmtUnknown, true},
		{"CALL my_proc()", StmtUnknown, true},

		// EXPLAIN for non-SELECT (blocked)
		{"EXPLAIN INSERT INTO users VALUES (1)", StmtUnknown, true},
		{"EXPLAIN UPDATE users SET name='x'", StmtUnknown, true},
		{"EXPLAIN DELETE FROM users", StmtUnknown, true},

		// Maintenance commands (treated as DDL)
		{"VACUUM", StmtDDL, false},
		{"ANALYZE", StmtDDL, false},
		{"REINDEX", StmtDDL, false},
		{"OPTIMIZE TABLE users", StmtDDL, false},

		// Permission changes (DDL)
		{"GRANT SELECT ON users TO 'user'", StmtDDL, false},
		{"REVOKE SELECT ON users FROM 'user'", StmtDDL, false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := DetectStatementType(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectStatementType(%q) error = %v, wantErr %v", tt.query, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("DetectStatementType(%q) = %v, want %v", tt.query, got, tt.expected)
			}
		})
	}
}

func TestCheckPermission(t *testing.T) {
	conn := ConnectionRecord{
		Name:        "test-conn",
		AllowRead:   true,
		AllowWrite:  false,
		AllowChange: false,
		AllowDelete: false,
	}

	tests := []struct {
		stmt    StatementType
		wantErr bool
	}{
		{StmtSelect, false},
		{StmtInsert, true},
		{StmtUpdate, true},
		{StmtDelete, true},
		{StmtDDL, true},
	}

	for _, tt := range tests {
		t.Run(conn.Name+"_"+tt.stmt.String(), func(t *testing.T) {
			err := CheckPermission(conn, tt.stmt)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPermission(%v, %v) = %v, wantErr %v", conn.Name, tt.stmt, err, tt.wantErr)
			}
		})
	}
}

func TestCheckPermissionDDLRequiresBothWriteAndChange(t *testing.T) {
	// DDL requires BOTH allow_write AND allow_change
	connBoth := ConnectionRecord{Name: "both", AllowWrite: true, AllowChange: true}
	connOnlyWrite := ConnectionRecord{Name: "write-only", AllowWrite: true, AllowChange: false}
	connOnlyChange := ConnectionRecord{Name: "change-only", AllowWrite: false, AllowChange: true}
	connNeither := ConnectionRecord{Name: "neither", AllowWrite: false, AllowChange: false}

	tests := []struct {
		conn    ConnectionRecord
		wantErr bool
	}{
		{connBoth, false},
		{connOnlyWrite, true},
		{connOnlyChange, true},
		{connNeither, true},
	}

	for _, tt := range tests {
		t.Run(tt.conn.Name, func(t *testing.T) {
			err := CheckPermission(tt.conn, StmtDDL)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPermission(%v, DDL) = %v, wantErr %v", tt.conn.Name, err, tt.wantErr)
			}
		})
	}
}
