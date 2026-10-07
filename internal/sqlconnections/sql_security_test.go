package sqlconnections

import "testing"

func TestSQLLexicalBoundary(t *testing.T) {
	for _, q := range []string{
		"SELECT '--'; DELETE FROM accounts", "SELECT '/*'; DROP TABLE accounts; -- */",
		"SELECT 1 /*! INTO OUTFILE '/tmp/leak' */", "SELECT 1 /* unterminated",
		"SELECT 1 INTO OUTFILE '/tmp/leak'", "SELECT * FROM accounts FOR UPDATE",
		"SELECT pg_read_file('/etc/passwd')", "SELECT load_extension('x')",
		"SELECT ARRAY[pg_read_file('/etc/passwd')]",
		"EXPLAIN QUERY PLAN DELETE FROM accounts",
	} {
		if typ, err := DetectStatementType(q); err == nil && typ == StmtSelect {
			t.Errorf("unsafe query classified as read-only: %q", q)
		}
	}
	for _, q := range []string{
		"SELECT '--', '/*', ';'", "SELECT 'it''s; fine'", "SELECT 1; -- trailing comment",
		"SELECT $$--; /* literal */$$", "SELECT $body$a;b$body$", "SELECT count(*) FROM accounts",
	} {
		if typ, err := DetectStatementType(q); err != nil || typ != StmtSelect {
			t.Errorf("safe query rejected: %q: %v (%v)", q, typ, err)
		}
	}
}

func TestSQLDialectCannotHideReadSideEffects(t *testing.T) {
	for _, tc := range []struct{ driver, query string }{
		{"postgres", "SELECT ARRAY[pg_read_file('/etc/passwd')]"},
		{"mysql", "SELECT $tag$ INTO OUTFILE '/tmp/audit-export' FROM records -- $tag$"},
		{"sqlite", "SELECT $tag$; DELETE FROM records -- $tag$"},
		{"postgres", "SELECT 'payload'::custom_type"},
		{"postgres", "SELECT custom_type 'payload'"},
	} {
		if _, err := detectStatementType(tc.query, tc.driver); err == nil {
			t.Fatalf("%s side effect accepted: %s", tc.driver, tc.query)
		}
	}
	for _, tc := range []struct{ driver, query string }{
		{"postgres", "SELECT ARRAY[1,2,3]"},
		{"postgres", "SELECT $$arbitrary;literal$$"},
		{"sqlite", "SELECT [column;name] FROM [table-name]"},
	} {
		if _, err := detectStatementType(tc.query, tc.driver); err != nil {
			t.Fatalf("%s safe query rejected: %v", tc.driver, err)
		}
	}
}
