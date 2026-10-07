package sqlconnections

import (
	"strings"
	"testing"
	"time"
)

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
	drivers := []string{"postgres", "mysql", "sqlite"}

	denied := []string{
		// Author-provided core cases.
		"INSERT INTO t SELECT pg_read_file('/etc/passwd')",
		"UPDATE t SET c = load_extension('/tmp/x.so')",
		"INSERT INTO t VALUES (LOAD_FILE('/etc/passwd'))",
		"CREATE EXTENSION IF NOT EXISTS plpython3u",
		"CREATE FUNCTION f() RETURNS int AS 'x' LANGUAGE c",
		"INSERT INTO t SELECT lo_import('/etc/passwd')",
		"WITH x AS (SELECT 1) INSERT INTO t SELECT pg_ls_dir('/')",
		"ALTER SYSTEM SET log_directory = '/tmp'",
		// PostgreSQL file / large-object / admin functions.
		"INSERT INTO t SELECT pg_read_binary_file('/etc/passwd')",
		"UPDATE t SET c = pg_stat_file('/etc/passwd')",
		"INSERT INTO t SELECT lo_export(1, '/tmp/x')",
		"UPDATE t SET c = pg_execute_server_program('id')",
		"UPDATE t SET c = pg_reload_conf()",
		// PostgreSQL / admin statement forms.
		"COPY t TO PROGRAM 'curl http://x'",
		"CREATE OR REPLACE FUNCTION f() RETURNS int AS 'x' LANGUAGE c",
		"CREATE OR REPLACE PROCEDURE p() LANGUAGE sql AS 'x'",
		"CREATE PROCEDURE p() BEGIN END",
		"CREATE LANGUAGE plpython3u",
		"CREATE AGGREGATE a (int) (SFUNC = f, STYPE = int)",
		"CREATE SERVER s FOREIGN DATA WRAPPER postgres_fdw",
		"CREATE FOREIGN DATA WRAPPER w HANDLER h",
		"ALTER EXTENSION hstore UPDATE",
		// GRANT of a PostgreSQL predefined file/program role.
		"GRANT pg_read_server_files TO bob",
		"GRANT pg_execute_server_program TO bob",
		// MySQL/MariaDB file export, bulk import and plugin/component loading.
		"INSERT INTO t SELECT a INTO OUTFILE '/tmp/x' FROM s",
		"INSERT INTO t SELECT a INTO DUMPFILE '/tmp/x' FROM s",
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

		// --- Evasion forms the write check must still catch (review). ---
		// Quoting bypass: a denied function name in quotes glued to a keyword.
		"INSERT INTO t SELECT\"pg_read_file\"('/etc/passwd')",
		"INSERT INTO t SELECT\"lo_import\"('/etc/passwd')",
		"UPDATE t SET c=\"pg_read_file\"('/x')",
		// Quoting bypass: a quoted object name glued after a denied phrase.
		"CREATE EXTENSION\"plpython3u\"",
		"CREATE FUNCTION\"f\"() RETURNS int AS 'x' LANGUAGE c",
		"CREATE LANGUAGE\"plpython3u\"",
		"CREATE SERVER\"s\" FOREIGN DATA WRAPPER postgres_fdw",
		"ALTER EXTENSION\"hstore\" UPDATE",
		"CREATE FUNCTION`f`RETURNS STRING SONAME 'udf.so'",
		// OUTFILE/DUMPFILE glued to a literal, incl. a MySQL CREATE EVENT body
		// (an embedded form that reaches the CREATE -> DDL write branch).
		"INSERT INTO t SELECT a FROM s INTO OUTFILE'/tmp/x'",
		"CREATE EVENT e ON SCHEDULE AT CURRENT_TIMESTAMP DO SELECT a FROM s INTO OUTFILE'/tmp/x'",
		"CREATE EVENT e ON SCHEDULE AT CURRENT_TIMESTAMP DO SELECT a FROM s INTO DUMPFILE'/tmp/x'",
		// Schema-prefix bypass: 2-part, 3-part, quoted parts, spaced dots, mixed case.
		"INSERT INTO t SELECT pg_catalog.pg_read_file('/etc/passwd')",
		"INSERT INTO t SELECT mydb.pg_catalog.pg_read_file('/etc/passwd')",
		"INSERT INTO t SELECT \"pg_catalog\".\"pg_read_file\"('/x')",
		"INSERT INTO t SELECT pg_catalog . pg_read_file('/x')",
		"INSERT INTO t SELECT public.load_file('/x')",
		"INSERT INTO t SELECT Pg_Read_File('/etc/passwd')",
		// MySQL CREATE DEFINER=... FUNCTION/PROCEDURE (clause split by DEFINER).
		"CREATE DEFINER=CURRENT_USER FUNCTION f() RETURNS INT RETURN 1",
		"CREATE DEFINER=CURRENT_USER PROCEDURE p() SELECT 1",
		"CREATE DEFINER=`u`@`h` FUNCTION f() RETURNS INT RETURN 1",
		// PostgreSQL unicode-escaped identifier hiding a function name.
		"INSERT INTO t SELECT U&\"pg_read_fil!0065\" UESCAPE '!' ('/etc/passwd')",
		// --- Neighbour / lexical evasion forms (review). ---
		// SQLite bracket-quoted identifier (driver-specific; asserted on sqlite below).
		"INSERT INTO t SELECT [pg_read_file]('/x')",
		// Doubled double-quote inside a quoted identifier resolves to one char.
		"INSERT INTO t SELECT \"pg_read\"\"file\"('/x')",
		// E'…' escape-string arg glued near the call.
		"INSERT INTO t SELECT pg_read_file(E'/etc/passwd')",
		// A comment between CREATE and FUNCTION.
		"CREATE/**/FUNCTION f() RETURNS int AS 'x' LANGUAGE c",
		// Spaced DEFINER = CURRENT_USER.
		"CREATE DEFINER = CURRENT_USER FUNCTION f() RETURNS INT RETURN 1",
		// FUNCTION keyword is not an object-name position, so the call is still caught.
		"GRANT ALL ON FUNCTION pg_read_file(text) TO bob",
		// CREATE OPERATOR binds a function without call parentheses.
		"CREATE OPERATOR === (LEFTARG = text, RIGHTARG = text, FUNCTION = pg_read_file)",
		// GRANT of a whole-data role.
		"GRANT pg_read_all_data TO bob",
	}
	for _, q := range denied {
		q := q
		t.Run("denied/"+q, func(t *testing.T) {
			for _, driver := range drivers {
				// The SQLite bracket form only quotes on the sqlite dialect; on
				// other dialects '[' is ordinary syntax, so only assert sqlite.
				if strings.Contains(q, "[pg_read_file]") && driver != "sqlite" {
					continue
				}
				if _, err := detectStatementType(q, driver); err == nil {
					t.Errorf("%s (%s): expected rejection", q, driver)
				}
			}
		})
	}

	// Every denylist entry must be provably live: embed each function and phrase
	// in a minimal write statement that reaches validateWriteStructure and assert
	// it is refused. New entries are then covered automatically.
	for name := range sqlWriteDeniedFunctions {
		name := name
		t.Run("fn/"+name, func(t *testing.T) {
			q := "UPDATE zz SET c = " + strings.ToLower(name) + "('x')"
			if err := validateWriteStructure(q, "postgres"); err == nil {
				t.Errorf("denied function %q not refused in %q", name, q)
			}
		})
	}
	for _, phrase := range sqlWriteDeniedPhrases {
		phrase := phrase
		t.Run("phrase/"+phrase, func(t *testing.T) {
			// Prefixed with INSERT INTO zz so the CREATE-prefix modifier strip does
			// not apply; the phrase then stands as whole words in the token stream.
			q := "INSERT INTO zz " + strings.ToLower(phrase) + " zz"
			if err := validateWriteStructure(q, "postgres"); err == nil {
				t.Errorf("denied phrase %q not refused in %q", phrase, q)
			}
		})
	}

	// Top-level-only forms: these never classify as a write/DDL branch — the
	// statement classifier rejects them outright at its default/LOAD/COPY/INSTALL
	// handling. The matching write-denylist phrases are defence in depth for any
	// embedded occurrence (e.g. INTO OUTFILE above, which does reach a write branch).
	topLevelRejected := []string{
		"LOAD DATA INFILE '/etc/passwd' INTO TABLE t",
		"LOAD XML INFILE '/etc/passwd' INTO TABLE t",
		"COPY t FROM '/etc/passwd'",
		"INSTALL PLUGIN p SONAME 'p.so'",
		"INSTALL COMPONENT 'file://component'",
	}
	for _, q := range topLevelRejected {
		q := q
		t.Run("toplevel/"+q, func(t *testing.T) {
			for _, driver := range drivers {
				if _, err := detectStatementType(q, driver); err == nil {
					t.Errorf("%s (%s): expected top-level rejection", q, driver)
				}
			}
		})
	}

	allowed := map[string]StatementType{
		"INSERT INTO t (a, b) VALUES (NOW(), UUID())":                      StmtInsert,
		"UPDATE t SET updated = CURRENT_TIMESTAMP, n = COALESCE(n, 0) + 1": StmtUpdate,
		"INSERT INTO t SELECT id, LOWER(name) FROM s WHERE id > 5":         StmtInsert,
		"DELETE FROM t WHERE created < DATE('now', '-7 days')":             StmtDelete,
		"CREATE INDEX idx_t_a ON t (a)":                                    StmtDDL,
		"CREATE UNIQUE INDEX idx_t_ab ON t (a, b)":                         StmtDDL,
		"ALTER TABLE t ADD COLUMN c int":                                   StmtDDL,
		// A trigger-shaped DDL stays allowed by the denylist. The lexer rejects
		// embedded ';' (multi-statement) independently, so the body carries none.
		"CREATE TRIGGER trg AFTER INSERT ON t BEGIN UPDATE t SET n = 1 END": StmtDDL,
		// PostgreSQL trigger referencing an ordinary function (not denied).
		"CREATE TRIGGER trg BEFORE UPDATE ON t FOR EACH ROW EXECUTE FUNCTION set_updated()": StmtDDL,
		"CREATE VIEW v AS SELECT id, lower(name) FROM t":                                    StmtDDL,
		"CREATE OR REPLACE VIEW v AS SELECT 1":                                              StmtDDL,
		"CREATE DEFINER = CURRENT_USER VIEW v AS SELECT 1":                                  StmtDDL,
		// A denied name inside a string literal must NOT trigger (literals are
		// stripped before matching).
		"INSERT INTO t VALUES ('pg_read_file')":  StmtInsert,
		"INSERT INTO t VALUES (N'pg_read_file')": StmtInsert,
		// Identifiers that merely contain a denied phrase/word as a substring.
		"UPDATE t SET outfile_count = outfile_count + 1": StmtUpdate,
		"INSERT INTO copy_jobs (n) VALUES (1)":           StmtInsert,
		// Ordinary columns/options named copy / outfile / dumpfile / definer now
		// write fine (COPY is position-restricted; OUTFILE/DUMPFILE need INTO).
		"UPDATE docs SET copy = 'x' WHERE id = 1":  StmtUpdate,
		"INSERT INTO docs (copy) VALUES ('x')":     StmtInsert,
		"ALTER TABLE docs ADD COLUMN copy text":    StmtDDL,
		"CREATE TABLE jobs (id int, outfile text)": StmtDDL,
		// A table literally named outfile/dumpfile is the INSERT target, not a
		// MySQL file export, so it stays writable.
		"INSERT INTO outfile (n) VALUES (1)":                      StmtInsert,
		"INSERT INTO dumpfile (n) VALUES (1)":                     StmtInsert,
		"UPDATE t SET definer = 'x' WHERE id = 1":                 StmtUpdate,
		"INSERT INTO t (definer, n) VALUES ('x', 1)":              StmtInsert,
		"UPDATE t SET c = c + 1 WHERE REPLACE(n, 'a', 'b') = 'c'": StmtUpdate,
		// 'edit' as a bare column (not a function call) is fine.
		"UPDATE t SET edit = 1 WHERE id = 2": StmtUpdate,
		"UPDATE edit SET n = 1":              StmtUpdate,
		// A table/view/index/constraint literally named 'edit' is an object name,
		// not the SQLite edit() function, so it is not treated as a denied call.
		"INSERT INTO edit (n) VALUES (1)":                               StmtInsert,
		"INSERT INTO \"edit\" (n) VALUES (1)":                           StmtInsert,
		"CREATE TABLE edit (id int)":                                    StmtDDL,
		"CREATE TABLE IF NOT EXISTS edit (id int)":                      StmtDDL,
		"CREATE VIEW edit (a) AS SELECT 1":                              StmtDDL,
		"CREATE INDEX idx_edit_n ON edit (n)":                           StmtDDL,
		"CREATE TABLE notes (id int, edit_id int REFERENCES edit (id))": StmtDDL,
		"CREATE TABLE tt (id int, KEY edit (id))":                       StmtDDL,
		// Ordinary CTE write with normal functions keeps working.
		"WITH r AS (SELECT id FROM s) INSERT INTO t SELECT COALESCE(id, 0) FROM r":  StmtInsert,
		"WITH r AS (SELECT id FROM s) DELETE FROM t WHERE id IN (SELECT id FROM r)": StmtDelete,
	}
	for q, want := range allowed {
		q, want := q, want
		t.Run("allowed/"+q, func(t *testing.T) {
			for _, driver := range drivers {
				got, err := detectStatementType(q, driver)
				if err != nil || got != want {
					t.Errorf("%s (%s): got %v/%v, want %v", q, driver, got, err, want)
				}
			}
		})
	}
}

// Final security sample: object-name context only through whitespace, SQL
// string runners, SQLite VACUUM INTO, optional CREATE words, server-file
// phrases and DEFINER principals of any shape.
func TestWriteDenylistFinalSecuritySample(t *testing.T) {
	drivers := []string{"postgres", "mysql", "sqlite"}
	denied := []string{
		// A context keyword spelled as an alias or column, separated from the
		// denied call by punctuation, is not an object-name position.
		"INSERT INTO t SELECT 1 AS key, pg_read_file('/etc/passwd')",
		`INSERT INTO t SELECT 1 AS "index", pg_read_file('/etc/passwd')`,
		"UPDATE t SET key = pg_read_file('/etc/passwd')",
		`INSERT INTO t SELECT * FROM generate_series(1,1) AS "table", pg_ls_dir('/')`,
		"INSERT INTO t SELECT 1 AS view, lo_import('/etc/shadow')",
		"INSERT INTO t VALUES (1) ON DUPLICATE KEY UPDATE `key` = LOAD_FILE('/etc/passwd')",
		"INSERT INTO t SELECT 1 AS `key`, LOAD_FILE('/etc/passwd')",
		`INSERT INTO t SELECT 1 AS "into", pg_read_file('/x')`,
		`INSERT INTO t SELECT 1 AS "update", pg_read_file('/x')`,
		`INSERT INTO t SELECT 1 AS "exists", pg_read_file('/x')`,
		`INSERT INTO t SELECT 1 AS "references", pg_read_file('/x')`,
		"UPDATE t SET index=pg_read_file('/x')",
		"CREATE INDEX i ON t (lower(a), pg_read_file('/x'))",
		// PostgreSQL functions that run a SQL string.
		"INSERT INTO t SELECT query_to_xml('select pg_read_file(''/etc/passwd'')', true, false, '')",
		"INSERT INTO t SELECT * FROM ts_stat('select to_tsvector(pg_read_file(''/etc/passwd''))')",
		"UPDATE t SET c = query_to_xmlschema('select 1', true, false, '')",
		"UPDATE t SET c = query_to_xml_and_xmlschema('select 1', true, false, '')",
		"INSERT INTO t SELECT * FROM crosstab('select 1, 2, 3') AS ct(a int, b int)",
		"INSERT INTO t SELECT * FROM crosstab3('select 1, 2, 3')",
		// SQLite VACUUM INTO writes a database copy to a host path.
		"VACUUM INTO '/tmp/evil.db'",
		"VACUUM main INTO '/tmp/evil.db'",
		"VACUUM \"main\" INTO '/tmp/evil.db'",
		// Optional CREATE words and further server-reaching phrases.
		"CREATE TRUSTED LANGUAGE plpython3u",
		"CREATE OR REPLACE PROCEDURAL LANGUAGE plperlu",
		"CREATE OR REPLACE TRUSTED PROCEDURAL LANGUAGE plperlu",
		"CREATE SUBSCRIPTION s CONNECTION 'host=evil dbname=x' PUBLICATION p",
		"CREATE TABLESPACE ts LOCATION '/var/lib/x'",
		"ALTER ROLE app SET session_preload_libraries = 'evil'",
		"ALTER ROLE app SET shared_preload_libraries = 'evil'",
		"ALTER DATABASE d SET local_preload_libraries = 'evil'",
		`ALTER ROLE app SET "session_preload_libraries" TO 'evil'`,
		"CREATE TABLE t (a int) DATA DIRECTORY='/var/www'",
		"CREATE TABLE t (a int) INDEX DIRECTORY = '/var/www'",
		"CREATE TABLE t (a int) ENGINE=CONNECT TABLE_TYPE=DOS FILE_NAME='/etc/passwd'",
		"CREATE TABLE t (a int) ENGINE=CONNECT, `file_name` = '/etc/passwd'",
		"ALTER TABLE t ENGINE=CONNECT FILE_NAME='/etc/passwd'",
		"CREATE TABLE t (a int) FILE_NAME='/etc/passwd' AS SELECT 1",
		// DEFINER principals that span more than two tokens (IPs, dotted
		// hostnames, quoted principals) before a denied object type: the
		// principal is scanned up to the object keyword, not a fixed two tokens.
		"CREATE DEFINER=root@127.0.0.1 FUNCTION f() RETURNS INT DETERMINISTIC RETURN 1",
		"CREATE DEFINER=root@local.host.name PROCEDURE p() SELECT 1",
		"CREATE DEFINER='u'@'%' PROCEDURE p() SELECT 1",
		"CREATE DEFINER = 'svc' @ '10.0.0.0' FUNCTION f() RETURNS INT RETURN 1",
		"CREATE DEFINER=CURRENT_USER() FUNCTION f() RETURNS INT RETURN 1",
		"CREATE DEFINER=`svc`@`db.internal` FUNCTION f() RETURNS INT RETURN 1",
		"CREATE ALGORITHM=MERGE DEFINER=admin@localhost SQL SECURITY DEFINER FUNCTION f() RETURNS INT RETURN 1",
		"CREATE OR REPLACE DEFINER=reporter@10.9.8.7 AGGREGATE a(x INT) (SFUNC=f, STYPE=int)",
		// A user or host part spelled like an object keyword or a modifier
		// (MySQL hosts need no quotes) must not end the principal early.
		"CREATE DEFINER=view@table FUNCTION f() RETURNS INT DETERMINISTIC RETURN 1",
		"CREATE DEFINER=`view`@`function` PROCEDURE p() SELECT 1",
		"CREATE DEFINER=event@trigger FUNCTION f() RETURNS INT RETURN 1",
		"CREATE DEFINER=u@index PROCEDURE p() SELECT 1",
		"CREATE DEFINER=sql@or FUNCTION f() RETURNS INT RETURN 1",
		"CREATE DEFINER=type@role FUNCTION f() RETURNS INT RETURN 1",
		"CREATE DEFINER=user@h.view.table FUNCTION f() RETURNS INT RETURN 1",
		"CREATE DEFINER=view FUNCTION f() RETURNS INT RETURN 1",
		"CREATE ALGORITHM=MERGE DEFINER=`table`@h SQL SECURITY DEFINER FUNCTION f() RETURNS INT RETURN 1",
		"CREATE OR REPLACE DEFINER=`index`@h AGGREGATE FUNCTION a(x INT) RETURNS INT BEGIN RETURN 1 END",
		// MySQL DATA/INDEX DIRECTORY table and partition options, with or
		// without '=', and with a double-quoted value (a string in MySQL's
		// default sql_mode).
		"CREATE TABLE t (a int) DATA DIRECTORY '/var/www'",
		`CREATE TABLE t (a int) DATA DIRECTORY "/var/www"`,
		"CREATE TABLE t (a int) DATA/**/DIRECTORY='/var/www'",
		"CREATE TABLE t (a int) PARTITION BY RANGE (a) (PARTITION p0 VALUES LESS THAN (10) DATA DIRECTORY = '/var/www')",
		"ALTER TABLE t ADD PARTITION (PARTITION p1 VALUES LESS THAN (20) INDEX DIRECTORY '/var/www')",
	}
	for _, q := range denied {
		q := q
		t.Run("denied/"+q, func(t *testing.T) {
			for _, driver := range drivers {
				if _, err := detectStatementType(q, driver); err == nil {
					t.Errorf("%s (%s): expected rejection", q, driver)
				}
			}
		})
	}

	allowed := map[string]StatementType{
		// Object names after a context keyword, separated by whitespace only
		// (also newlines, comments and padded quoted names), stay writable.
		"INSERT INTO edit(n) VALUES (1)":                                 StmtInsert,
		"INSERT INTO  \"edit\"  (n) VALUES (1)":                          StmtInsert,
		"INSERT INTO edit\n(n) VALUES (1)":                               StmtInsert,
		"INSERT INTO /* target */ edit (n) VALUES (1)":                   StmtInsert,
		"CREATE TABLE IF NOT EXISTS \"edit\" (id int)":                   StmtDDL,
		"CREATE VIEW \"edit\" (a) AS SELECT 1":                           StmtDDL,
		"CREATE INDEX idx_edit ON \"edit\" (n)":                          StmtDDL,
		"CREATE TABLE notes (id int, edit_id int REFERENCES edit(id))":   StmtDDL,
		"CREATE TABLE tt (id int, KEY edit(id))":                         StmtDDL,
		"CREATE TABLE tt (id int, KEY `edit` (id))":                      StmtDDL,
		"UPDATE t SET key = lower(key) WHERE id = 1":                     StmtUpdate,
		"INSERT INTO t SELECT 1 AS key, lower(name) FROM s":              StmtInsert,
		"INSERT INTO t VALUES (1) ON DUPLICATE KEY UPDATE `key` = NOW()": StmtInsert,
		// Plain VACUUM keeps working.
		"VACUUM main": StmtDDL,
		// Ordinary file_name / data / directory columns are not table options.
		"INSERT INTO uploads (file_name) VALUES ('a.txt')":                            StmtInsert,
		"UPDATE uploads SET file_name = 'b.txt' WHERE id = 1":                         StmtUpdate,
		"CREATE TABLE uploads (id int, file_name text)":                               StmtDDL,
		"CREATE TABLE uploads (file_name text CHECK (file_name = 'x'))":               StmtDDL,
		"ALTER TABLE uploads ADD COLUMN file_name text":                               StmtDDL,
		"CREATE TABLE t2 AS SELECT * FROM uploads WHERE file_name = 'x'":              StmtDDL,
		"CREATE TRIGGER trg AFTER INSERT ON t BEGIN UPDATE u SET file_name = 'x' END": StmtDDL,
		"CREATE TABLE trusted (id int)":                                               StmtDDL,
		// A multi-token DEFINER principal before a permitted object type (VIEW)
		// is consumed without swallowing the object keyword, so the view is
		// still allowed.
		"CREATE DEFINER=root@127.0.0.1 VIEW v AS SELECT 1":                      StmtDDL,
		"CREATE DEFINER='u'@'%' VIEW v AS SELECT 1":                             StmtDDL,
		"CREATE DEFINER=`svc`@`db.internal` VIEW v AS SELECT 1":                 StmtDDL,
		"CREATE DEFINER=CURRENT_USER() SQL SECURITY INVOKER VIEW v AS SELECT 1": StmtDDL,
		"CREATE DEFINER=view@table VIEW v AS SELECT 1":                          StmtDDL,
		"CREATE DEFINER=`table`@`trigger` VIEW v AS SELECT 1":                   StmtDDL,
		"CREATE DEFINER=user@sql SQL SECURITY INVOKER VIEW v AS SELECT 1":       StmtDDL,
		// Ordinary data / directory columns, an index named directory and a CTAS
		// alias are not DATA/INDEX DIRECTORY options.
		"INSERT INTO backups (data, directory) VALUES ('a','b')":      StmtInsert,
		"UPDATE backups SET data = 'a', directory = 'b' WHERE id = 1": StmtUpdate,
		"INSERT INTO backups SELECT data, directory FROM src":         StmtInsert,
		"CREATE INDEX directory ON backups (data)":                    StmtDDL,
		"CREATE TABLE backups AS SELECT data directory FROM src":      StmtDDL,
	}
	for q, want := range allowed {
		q, want := q, want
		t.Run("allowed/"+q, func(t *testing.T) {
			for _, driver := range drivers {
				got, err := detectStatementType(q, driver)
				if err != nil || got != want {
					t.Errorf("%s (%s): got %v/%v, want %v", q, driver, got, err, want)
				}
			}
		})
	}
}

// The object-name lookback is O(token length): a benign multi-thousand-row
// INSERT full of ordinary function calls must stay fast (it was ~6.7s when
// every match re-split the whole prefix).
func TestWriteDenylistLargeInsertStaysFast(t *testing.T) {
	var b strings.Builder
	b.WriteString("INSERT INTO t (a, b, c) VALUES ")
	for i := 0; i < 5000; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("(lower('x'), COALESCE(NULL, 1), key_of(n))")
	}
	q := b.String()
	start := time.Now()
	typ, err := detectStatementType(q, "postgres")
	elapsed := time.Since(start)
	if err != nil || typ != StmtInsert {
		t.Fatalf("large insert: got %v/%v, want INSERT", typ, err)
	}
	t.Logf("5000-row INSERT classified in %v", elapsed)
	if elapsed > 2*time.Second {
		t.Fatalf("5000-row INSERT took %v, want well under 2s", elapsed)
	}
}

// The CREATE-prefix parser (OR REPLACE, DEFINER principal, ALGORITHM, …) does
// constant work per prefix word, so 10k-token pathological prefixes and
// statements stay linear.
func TestWriteDenylistCreatePrefixStaysLinear(t *testing.T) {
	cols := strings.Repeat("c, ", 10000) + "c"
	cases := map[string]string{
		"definer view, 10k columns": "CREATE DEFINER=view@table VIEW v AS SELECT " + cols + " FROM t",
		"10k OR REPLACE":            "CREATE " + strings.Repeat("OR REPLACE ", 5000) + "VIEW v AS SELECT 1",
		"definer, 10k bare tokens":  "CREATE DEFINER=" + strings.Repeat("x ", 10000),
		"10k DEFINER clauses":       "CREATE " + strings.Repeat("DEFINER=u@h ", 10000) + "VIEW v AS SELECT 1",
	}
	for name, q := range cases {
		start := time.Now()
		_, _ = detectStatementType(q, "mysql")
		elapsed := time.Since(start)
		t.Logf("%s: %v", name, elapsed)
		if elapsed > 2*time.Second {
			t.Fatalf("%s took %v, want well under 2s", name, elapsed)
		}
	}
	if _, err := detectStatementType("CREATE "+strings.Repeat("DEFINER=u@h ", 10000)+"FUNCTION f() RETURNS INT RETURN 1", "mysql"); err == nil {
		t.Fatal("10k DEFINER clauses before FUNCTION must still be refused")
	}
}
