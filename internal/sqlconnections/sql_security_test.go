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

func TestWriteStatementsRejectFileAndAdminSQL(t *testing.T) {
	denied := []string{
		// Author-provided core cases (audit M22).
		"INSERT INTO t SELECT pg_read_file('/etc/passwd')",
		"UPDATE t SET c = load_extension('/tmp/x.so')",
		"INSERT INTO t VALUES (LOAD_FILE('/etc/passwd'))",
		"CREATE EXTENSION IF NOT EXISTS plpython3u",
		"CREATE FUNCTION f() RETURNS int AS 'x' LANGUAGE c",
		"INSERT INTO t SELECT lo_import('/etc/passwd')",
		"WITH x AS (SELECT 1) INSERT INTO t SELECT pg_ls_dir('/')",
		"ALTER SYSTEM SET log_directory = '/tmp'",
		// PostgreSQL file / large-object / admin, incl. schema prefix and mixed case.
		"INSERT INTO t SELECT pg_read_binary_file('/etc/passwd')",
		"UPDATE t SET c = pg_stat_file('/etc/passwd')",
		"INSERT INTO t SELECT pg_catalog.pg_read_file('/etc/passwd')",
		"INSERT INTO t SELECT Pg_Read_File('/etc/passwd')",
		"INSERT INTO t SELECT lo_export(1, '/tmp/x')",
		"UPDATE t SET c = pg_execute_server_program('id')",
		"COPY t TO PROGRAM 'curl http://x'",
		"CREATE OR REPLACE PROCEDURE p() LANGUAGE sql AS 'x'",
		"CREATE PROCEDURE p() BEGIN END",
		"CREATE LANGUAGE plpython3u",
		"CREATE AGGREGATE a (int) (SFUNC = f, STYPE = int)",
		"CREATE SERVER s FOREIGN DATA WRAPPER postgres_fdw",
		"CREATE FOREIGN DATA WRAPPER w HANDLER h",
		"ALTER EXTENSION hstore UPDATE",
		// MySQL/MariaDB file export, bulk import and plugin/component loading.
		"INSERT INTO t SELECT a INTO OUTFILE '/tmp/x' FROM s",
		"INSERT INTO t SELECT a INTO DUMPFILE '/tmp/x' FROM s",
		"LOAD DATA INFILE '/etc/passwd' INTO TABLE t",
		"LOAD XML INFILE '/etc/passwd' INTO TABLE t",
		"INSTALL PLUGIN p SONAME 'p.so'",
		"INSTALL COMPONENT 'file://component'",
		"UPDATE t SET c = sys_exec('id')",
		"UPDATE t SET c = sys_eval('id')",
		// SQLite extension / file / directory / editor helpers.
		"INSERT INTO t SELECT writefile('/tmp/x', data) FROM s",
		"UPDATE t SET c = readfile('/etc/passwd')",
		"INSERT INTO t SELECT name FROM fsdir('/')",
		"UPDATE t SET c = edit(c)",
		// MSSQL / Oracle defence in depth.
		"UPDATE t SET c = xp_cmdshell('whoami')",
		"UPDATE t SET c = dbms_scheduler.create_job('j')",
	}
	for _, q := range denied {
		for _, driver := range []string{"postgres", "mysql", "sqlite"} {
			if _, err := detectStatementType(q, driver); err == nil {
				t.Fatalf("%s (%s): expected rejection", q, driver)
			}
		}
	}
	allowed := map[string]StatementType{
		"INSERT INTO t (a, b) VALUES (NOW(), UUID())":                      StmtInsert,
		"UPDATE t SET updated = CURRENT_TIMESTAMP, n = COALESCE(n, 0) + 1": StmtUpdate,
		"INSERT INTO t SELECT id, LOWER(name) FROM s WHERE id > 5":         StmtInsert,
		"DELETE FROM t WHERE created < DATE('now', '-7 days')":             StmtDelete,
		"CREATE INDEX idx_t_a ON t (a)":                                    StmtDDL,
		// A trigger-shaped DDL stays allowed by the denylist. The lexer rejects
		// embedded ';' (multi-statement) independently, so the body carries none.
		"CREATE TRIGGER trg AFTER INSERT ON t BEGIN UPDATE t SET n = 1 END": StmtDDL,
		// A denied name inside a string literal must NOT trigger (literals are
		// stripped before matching).
		"INSERT INTO t VALUES ('pg_read_file')": StmtInsert,
		// Identifiers that merely contain a denied phrase as a substring.
		"UPDATE t SET outfile_count = outfile_count + 1": StmtUpdate,
		"INSERT INTO copy_jobs (n) VALUES (1)":           StmtInsert,
		// 'edit' as a bare column (not a function call) is fine.
		"UPDATE t SET edit = 1 WHERE id = 2": StmtUpdate,
		// Ordinary CTE write with normal functions keeps working.
		"WITH r AS (SELECT id FROM s) INSERT INTO t SELECT COALESCE(id, 0) FROM r": StmtInsert,
	}
	for q, want := range allowed {
		got, err := detectStatementType(q, "postgres")
		if err != nil || got != want {
			t.Fatalf("%s: got %v/%v, want %v", q, got, err, want)
		}
	}
}
