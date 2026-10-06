package sqlconnections

import (
	"fmt"
	"regexp"
	"strings"
)

// sqlStructure retains SQL syntax while replacing literal contents. Unsupported
// escape modes are rejected: a connection's SQL mode must never change our boundary.
func sqlStructure(s string) (string, error) { return sqlStructureDialectMode(s, "", false) }

func sqlStructureDialect(s, driver string) (string, error) {
	return sqlStructureDialectMode(s, driver, false)
}

// sqlStructureWrite is the write-path normalisation: identical to the read
// normalisation except that every string literal and quoted-identifier
// replacement is surrounded by spaces, so a token glued to an adjacent keyword
// (e.g. SELECT"pg_read_file"(...) or CREATE EXTENSION"plpython3u") cannot hide
// from the write denylist. The read path keeps pad=false and its output is
// byte-identical to before.
func sqlStructureWrite(s, driver string) (string, error) {
	return sqlStructureDialectMode(s, driver, true)
}

func sqlStructureDialectMode(s, driver string, pad bool) (string, error) {
	var out strings.Builder
	// writeLiteral emits a normalised literal/identifier replacement, padding it
	// with surrounding spaces in write mode so it never merges with a neighbour.
	writeLiteral := func(text string) {
		if pad {
			out.WriteByte(' ')
			out.WriteString(text)
			out.WriteByte(' ')
			return
		}
		out.WriteString(text)
	}
	ended := false
	for i := 0; i < len(s); {
		c := s[i]
		if c == 0 {
			return "", fmt.Errorf("SQL contains NUL")
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			out.WriteByte(' ')
			i++
			continue
		}
		if i+1 < len(s) && s[i:i+2] == "--" {
			// MySQL only recognizes -- followed by whitespace. Reject ambiguous forms.
			if i+2 < len(s) && s[i+2] > ' ' {
				return "", fmt.Errorf("ambiguous SQL comment")
			}
			i += 2
			for i < len(s) && s[i] != '\n' && s[i] != '\r' {
				i++
			}
			out.WriteByte(' ')
			continue
		}
		if i+1 < len(s) && s[i:i+2] == "/*" {
			if i+2 < len(s) && (s[i+2] == '!' || s[i+2] == '+') || strings.HasPrefix(s[i:], "/*M!") {
				return "", fmt.Errorf("executable SQL comments and hints are not allowed")
			}
			end := strings.Index(s[i+2:], "*/")
			if end < 0 || strings.Contains(s[i+2:i+2+end], "/*") {
				return "", fmt.Errorf("unterminated or nested SQL comment")
			}
			i += end + 4
			out.WriteByte(' ')
			continue
		}
		if ended {
			return "", fmt.Errorf("multiple statements are not allowed")
		}
		if c == ';' {
			ended = true
			i++
			continue
		}
		if c == '#' || c == '\\' {
			return "", fmt.Errorf("unsupported SQL escape or comment syntax")
		}
		// PostgreSQL unicode-escaped strings/identifiers (U&'…' / U&"…") can encode
		// an arbitrary name through their escape sequences. They have no legitimate
		// use in an agent write, so refuse them in write mode rather than let one
		// hide a denied function name.
		if pad && (c == 'u' || c == 'U') && i+2 < len(s) && s[i+1] == '&' && (s[i+2] == '"' || s[i+2] == '\'') {
			return "", fmt.Errorf("unicode-escaped identifiers are not allowed in write statements")
		}
		if c == '$' {
			if driver != "" && driver != "postgres" {
				return "", fmt.Errorf("dollar syntax is unsupported in this SQL dialect")
			}
			j := i + 1
			for j < len(s) && ((s[j] >= 'a' && s[j] <= 'z') || (s[j] >= 'A' && s[j] <= 'Z') || s[j] == '_' || (j > i+1 && s[j] >= '0' && s[j] <= '9')) {
				j++
			}
			if j < len(s) && s[j] == '$' {
				delim := s[i : j+1]
				end := strings.Index(s[j+1:], delim)
				if end < 0 {
					return "", fmt.Errorf("unterminated SQL dollar string")
				}
				i = j + 1 + end + len(delim)
				writeLiteral("'literal'")
				continue
			}
		}
		if c == '\'' || c == '"' || c == '`' || (c == '[' && driver == "sqlite") {
			close := c
			if c == '[' {
				close = ']'
			}
			j := i + 1
			var ident strings.Builder
			for ; j < len(s); j++ {
				if s[j] == '\\' || s[j] == 0 {
					return "", fmt.Errorf("unsupported SQL literal escape")
				}
				if s[j] == close {
					if j+1 < len(s) && s[j+1] == close {
						ident.WriteByte('_')
						j++
						continue
					}
					break
				}
				if isSQLIdentChar(rune(s[j])) {
					ident.WriteByte(s[j])
				} else {
					ident.WriteByte('_')
				}
			}
			if j == len(s) {
				return "", fmt.Errorf("unterminated SQL literal or identifier")
			}
			if c == '\'' {
				writeLiteral("'literal'")
			} else {
				writeLiteral(ident.String())
			}
			i = j + 1
			continue
		}
		out.WriteByte(c)
		i++
	}
	return strings.TrimSpace(out.String()), nil
}

var sqlTypedLiteralPattern = regexp.MustCompile(`(?i)([a-z_][a-z0-9_.$]*)\s*'literal'`)

var sqlFunctionPattern = regexp.MustCompile(`(?i)([a-z_][a-z0-9_.$]*)\s*\(`)

// Unknown/user-defined functions are not read capabilities. DB read-only mode
// alone cannot prevent network/file effects of privileged extension functions.
func validateReadStructure(s string) error {
	upper := strings.Join(strings.Fields(strings.ToUpper(s)), " ")
	if strings.Contains(s, "::") {
		return fmt.Errorf("SQL casts require an explicitly supported form")
	}
	for _, match := range sqlTypedLiteralPattern.FindAllStringSubmatch(s, -1) {
		if !strings.Contains(" SELECT AS LIKE ILIKE WHEN THEN ELSE IS NOT DISTINCT FROM WHERE AND OR IN ON HAVING BETWEEN ESCAPE INTERVAL DATE TIME TIMESTAMP TEXT UUID BYTEA X N E ", " "+strings.ToUpper(match[1])+" ") {
			return fmt.Errorf("unsupported typed SQL literal")
		}
	}
	for _, word := range []string{"INTO", "OUTFILE", "DUMPFILE", "FOR UPDATE", "FOR SHARE", "LOCK IN", "PROCEDURE", "ANALYZE"} {
		if strings.Contains(" "+upper+" ", " "+word+" ") {
			return fmt.Errorf("side-effecting SQL is not allowed in a read query")
		}
	}
	allowed := " COUNT SUM AVG MIN MAX ABS ROUND FLOOR CEIL CEILING LENGTH CHAR_LENGTH CHARACTER_LENGTH OCTET_LENGTH LOWER UPPER TRIM LTRIM RTRIM SUBSTR SUBSTRING REPLACE CONCAT CONCAT_WS COALESCE NULLIF IFNULL IIF IF EXTRACT DATE TIME DATETIME STRFTIME JULIANDAY UNIXEPOCH DATE_TRUNC DATE_PART TO_CHAR TO_DATE TO_TIMESTAMP NOW CURRENT_DATE CURRENT_TIME CURRENT_TIMESTAMP AGE GREATEST LEAST MOD POWER SQRT SIGN TRUNC RANDOM RAND STRING_AGG GROUP_CONCAT ARRAY_AGG JSON_AGG JSONB_AGG JSON_OBJECT JSON_ARRAY JSON_EXTRACT JSON_VALUE JSON_TYPE JSON_ARRAY_LENGTH ROW_NUMBER RANK DENSE_RANK NTILE LAG LEAD FIRST_VALUE LAST_VALUE NTH_VALUE PERCENT_RANK CUME_DIST BOOL_AND BOOL_OR EVERY STDDEV VARIANCE IN EXISTS AS OVER FILTER VALUES DISTINCT ALL SELECT WITH PARTITION "
	for _, match := range sqlFunctionPattern.FindAllStringSubmatch(s, -1) {
		name := strings.ToUpper(match[1])
		name = strings.TrimPrefix(name, "PG_CATALOG.")
		if !strings.Contains(allowed, " "+name+" ") {
			return fmt.Errorf("function %q is not an approved read function", match[1])
		}
	}
	return nil
}

// sqlWriteDeniedFunctions name functions that reach the database server's file
// system, dynamic loader, large-object store or operating system. The read path
// refuses every non-allowlisted function; writes keep all ordinary functions but
// never these. Grouped by the engine they belong to and retained for defence in
// depth even on dialects where a given name is not callable. Matching strips any
// schema/catalog prefix (comparing the last dotted segment) and keeps a
// full-name entry for the one dotted builtin (DBMS_SCHEDULER.CREATE_JOB).
var sqlWriteDeniedFunctions = map[string]bool{
	// PostgreSQL: server-side file reads, large-object import/export, server
	// program execution and configuration reload.
	"PG_READ_FILE": true, "PG_READ_BINARY_FILE": true, "PG_LS_DIR": true, "PG_STAT_FILE": true,
	"PG_EXECUTE_SERVER_PROGRAM": true, "LO_IMPORT": true, "LO_EXPORT": true, "PG_RELOAD_CONF": true,
	// MySQL/MariaDB file read, and SQLite extension loading, file read/write,
	// directory listing and the external-editor hook.
	"LOAD_FILE": true, "LOAD_EXTENSION": true, "READFILE": true, "WRITEFILE": true, "FSDIR": true, "EDIT": true,
	// MSSQL / Oracle defence in depth: shell-out and OS job scheduling.
	"XP_CMDSHELL": true, "SYS_EXEC": true, "SYS_EVAL": true, "DBMS_SCHEDULER.CREATE_JOB": true,
}

// sqlWriteDeniedPhrases name statement forms that export data off the server,
// bulk-import into it, or extend/administer the engine, plus the PostgreSQL
// predefined roles that grant file/program access. Matched as whole,
// space-delimited words against the normalised, literal-stripped, write-padded
// upper text so identifiers such as OUTFILE_COUNT or a COPY_JOBS table never
// trigger. (Tables or columns named exactly COPY, OUTFILE or DUMPFILE are a
// documented residual false positive.)
var sqlWriteDeniedPhrases = []string{
	// MySQL/MariaDB file export and bulk import.
	"OUTFILE", "DUMPFILE", "LOAD DATA", "LOAD XML",
	// PostgreSQL bulk copy (also refused as a leading keyword by the classifier).
	"COPY",
	// Extension / procedural-language / aggregate / foreign-data / server and
	// system administration. OR REPLACE and DEFINER=… clauses are stripped before
	// matching so the contiguous CREATE … FUNCTION/PROCEDURE phrase still catches
	// e.g. CREATE OR REPLACE FUNCTION and CREATE DEFINER=`u`@`h` PROCEDURE.
	"CREATE EXTENSION", "ALTER EXTENSION", "CREATE FUNCTION", "CREATE OR REPLACE FUNCTION",
	"CREATE PROCEDURE", "CREATE OR REPLACE PROCEDURE", "CREATE LANGUAGE", "CREATE AGGREGATE",
	"CREATE SERVER", "CREATE FOREIGN DATA WRAPPER", "ALTER SYSTEM",
	// MySQL/MariaDB plugin and component loading.
	"INSTALL PLUGIN", "INSTALL COMPONENT",
	// PostgreSQL predefined roles that confer server file / program access when
	// granted, so a GRANT of them is refused. PG_EXECUTE_SERVER_PROGRAM is also a
	// function above; as a bare role name it has no call parens, so it is listed
	// here too.
	"PG_READ_SERVER_FILES", "PG_WRITE_SERVER_FILES", "PG_READ_ALL_DATA", "PG_WRITE_ALL_DATA",
	"PG_EXECUTE_SERVER_PROGRAM",
}

// sqlWriteObjectNameContext lists keywords after which the next identifier is an
// object name being defined or targeted, not a function call. A denied function
// name in one of these positions (e.g. a table literally named edit in
// INSERT INTO edit (…) or CREATE TABLE edit (…)) is not a file/loader call, so
// its match is skipped. FROM/JOIN/ON are deliberately excluded because a
// table-valued function such as fsdir('/') lives there and must stay denied.
var sqlWriteObjectNameContext = map[string]bool{"INTO": true, "TABLE": true, "UPDATE": true}

// sqlWriteDefinerStop marks the keywords that end a stripped DEFINER=… clause.
var sqlWriteDefinerStop = map[string]bool{
	"FUNCTION": true, "PROCEDURE": true, "TRIGGER": true, "EVENT": true, "AGGREGATE": true, "VIEW": true,
}

// stripWriteModifierClauses removes the optional CREATE modifiers that would
// otherwise split a denied phrase: a leading-position OR REPLACE and a
// MySQL DEFINER=<principal> clause. Operates on the space-delimited upper tokens.
func stripWriteModifierClauses(tokens []string) []string {
	noDefiner := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t == "DEFINER" || strings.HasPrefix(t, "DEFINER=") {
			for i+1 < len(tokens) && !sqlWriteDefinerStop[tokens[i+1]] {
				i++
			}
			continue
		}
		noDefiner = append(noDefiner, t)
	}
	res := make([]string, 0, len(noDefiner))
	for i := 0; i < len(noDefiner); i++ {
		if noDefiner[i] == "OR" && i+1 < len(noDefiner) && noDefiner[i+1] == "REPLACE" {
			i++
			continue
		}
		res = append(res, noDefiner[i])
	}
	return res
}

// validateWriteStructure rejects file, loader and administrative SQL in write
// and DDL statements without constraining ordinary functions. Unlike the read
// path it is a denylist: every ordinary write function and statement form keeps
// working; only server-reaching names and forms are refused. It re-normalises
// the original query in write-padded mode so that quoted/glued tokens, schema
// prefixes and DEFINER clauses cannot smuggle a denied name past the check.
func validateWriteStructure(query, driver string) error {
	ps, err := sqlStructureWrite(query, driver)
	if err != nil {
		return err
	}
	tokens := stripWriteModifierClauses(strings.Fields(strings.ToUpper(ps)))
	phrase := " " + strings.Join(tokens, " ") + " "
	for _, p := range sqlWriteDeniedPhrases {
		if strings.Contains(phrase, " "+p+" ") {
			return fmt.Errorf("file, loader or administrative SQL is not allowed: %s", p)
		}
	}
	for _, m := range sqlFunctionPattern.FindAllStringSubmatchIndex(ps, -1) {
		raw := ps[m[2]:m[3]]
		name := strings.ToUpper(raw)
		prev := ""
		if fields := strings.Fields(strings.ToUpper(ps[:m[2]])); len(fields) > 0 {
			prev = fields[len(fields)-1]
		}
		if sqlWriteObjectNameContext[prev] {
			continue
		}
		if sqlWriteDeniedFunctions[name] {
			return fmt.Errorf("function %q is not allowed in write statements", raw)
		}
		if idx := strings.LastIndex(name, "."); idx >= 0 && sqlWriteDeniedFunctions[name[idx+1:]] {
			return fmt.Errorf("function %q is not allowed in write statements", raw)
		}
	}
	return nil
}
