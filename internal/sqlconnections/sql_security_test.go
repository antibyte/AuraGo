package sqlconnections

import "testing"

func TestSQLLexicalBoundary(t *testing.T) {
	for _, q := range []string{
		"SELECT '--'; DELETE FROM accounts", "SELECT '/*'; DROP TABLE accounts; -- */",
		"SELECT 1 /*! INTO OUTFILE '/tmp/leak' */", "SELECT 1 /* unterminated",
		"SELECT 1 INTO OUTFILE '/tmp/leak'", "SELECT * FROM accounts FOR UPDATE",
		"SELECT pg_read_file('/etc/passwd')", "SELECT load_extension('x')",
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
