package sqlconnections

import (
	"testing"
)

func TestDetectStatementType(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected StatementType
		wantErr  bool
	}{
		{"SELECT", "SELECT * FROM users", StmtSelect, false},
		{"SELECT lower", "select id from t", StmtSelect, false},
		{"SHOW", "SHOW TABLES", StmtSelect, false},
		{"DESCRIBE", "DESCRIBE users", StmtSelect, false},
		{"DESC", "DESC users", StmtSelect, false},
		{"EXPLAIN", "EXPLAIN SELECT 1", StmtSelect, false},
		{"PRAGMA", "PRAGMA table_info(users)", StmtSelect, false},
		{"INSERT", "INSERT INTO users (name) VALUES ('test')", StmtInsert, false},
		{"REPLACE", "REPLACE INTO users (id, name) VALUES (1, 'test')", StmtInsert, false},
		{"UPDATE", "UPDATE users SET name='new' WHERE id=1", StmtUpdate, false},
		{"DELETE", "DELETE FROM users WHERE id=1", StmtDelete, false},
		{"TRUNCATE", "TRUNCATE TABLE users", StmtDelete, false},
		{"CREATE", "CREATE TABLE foo (id INT)", StmtDDL, false},
		{"DROP", "DROP TABLE foo", StmtDDL, false},
		{"ALTER", "ALTER TABLE foo ADD COLUMN bar TEXT", StmtDDL, false},
		{"WITH SELECT", "WITH cte AS (SELECT 1) SELECT * FROM cte", StmtSelect, false},
		{"WITH INSERT", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte", StmtInsert, false},
		{"trailing semicolon", "SELECT 1;", StmtSelect, false},
		{"trailing semicolon+space", "SELECT 1;  ", StmtSelect, false},
		{"multiple statements", "SELECT 1; DROP TABLE users", StmtUnknown, true},
		{"empty", "", StmtUnknown, true},
		{"whitespace only", "   ", StmtUnknown, true},
		{"unsupported", "GRANT ALL ON *.* TO 'user'", StmtUnknown, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectStatementType(tt.query)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil (type=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestCheckPermission(t *testing.T) {
	readOnly := ConnectionRecord{Name: "ro", AllowRead: true}
	fullAccess := ConnectionRecord{Name: "full", AllowRead: true, AllowWrite: true, AllowChange: true, AllowDelete: true}
	writeOnly := ConnectionRecord{Name: "wo", AllowWrite: true, AllowChange: true}

	tests := []struct {
		name    string
		conn    ConnectionRecord
		stmt    StatementType
		wantErr bool
	}{
		{"read-only allows SELECT", readOnly, StmtSelect, false},
		{"read-only denies INSERT", readOnly, StmtInsert, true},
		{"read-only denies UPDATE", readOnly, StmtUpdate, true},
		{"read-only denies DELETE", readOnly, StmtDelete, true},
		{"read-only denies DDL", readOnly, StmtDDL, true},
		{"full allows SELECT", fullAccess, StmtSelect, false},
		{"full allows INSERT", fullAccess, StmtInsert, false},
		{"full allows UPDATE", fullAccess, StmtUpdate, false},
		{"full allows DELETE", fullAccess, StmtDelete, false},
		{"full allows DDL", fullAccess, StmtDDL, false},
		{"unknown blocked", fullAccess, StmtUnknown, true},
		{"write+change allows DDL", writeOnly, StmtDDL, false},
		{"write-only denies SELECT", writeOnly, StmtSelect, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPermission(tt.conn, tt.stmt)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"users", true},
		{"my_table", true},
		{"Table123", true},
		{"_private", true},
		{"public.users", true},
		{"schema.table_name", true},
		{"", false},
		{"DROP TABLE", false},
		{"users; --", false},
		{"table'name", false},
		{"table\"name", false},
		{"table(name)", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidIdentifier(tt.input)
			if got != tt.valid {
				t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}
