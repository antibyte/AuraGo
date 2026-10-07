package sqlconnections

import (
	"fmt"
	"strings"
)

// This file holds the write/DDL denylist. The read path (validateReadStructure,
// in sql_lexer.go) is a separate allowlist and is not affected by anything here.
// detectStatementType applies validateWriteStructure once, after classification,
// to every statement that is not a read.

// sqlWriteDeniedFunctions name functions that reach the database server's file
// system, dynamic loader, large-object store, other servers or the operating
// system. Writes keep every ordinary function but never these. Matching strips
// any schema/catalog prefix (comparing the last dotted segment) and also keeps a
// full-name entry for the one dotted builtin (DBMS_SCHEDULER.CREATE_JOB).
var sqlWriteDeniedFunctions = map[string]bool{
	// PostgreSQL: server-side file reads, large-object import/export, server
	// program execution and configuration reload.
	"PG_READ_FILE": true, "PG_READ_BINARY_FILE": true, "PG_LS_DIR": true, "PG_STAT_FILE": true,
	"PG_EXECUTE_SERVER_PROGRAM": true, "LO_IMPORT": true, "LO_EXPORT": true, "PG_RELOAD_CONF": true,
	// PostgreSQL adminpack: write/rename/unlink/sync server files, list the log dir.
	"PG_FILE_WRITE": true, "PG_FILE_RENAME": true, "PG_FILE_UNLINK": true, "PG_FILE_SYNC": true,
	"PG_LOGDIR_LS": true,
	// PostgreSQL directory listings over server paths.
	"PG_LS_LOGDIR": true, "PG_LS_WALDIR": true, "PG_LS_TMPDIR": true, "PG_LS_ARCHIVE_STATUSDIR": true,
	// PostgreSQL server administration (log rotation, backend control, promotion).
	"PG_ROTATE_LOGFILE": true, "PG_TERMINATE_BACKEND": true, "PG_CANCEL_BACKEND": true, "PG_PROMOTE": true,
	// PostgreSQL dblink: reach another database server from inside a statement.
	"DBLINK": true, "DBLINK_EXEC": true, "DBLINK_CONNECT": true, "DBLINK_CONNECT_U": true,
	"DBLINK_SEND_QUERY": true,
	// PostgreSQL functions that run a SQL string (or SQL built from text
	// arguments) the denylist never sees: XML export, text-search statistics
	// and rewrite, and the tablefunc crosstab/connectby family.
	"QUERY_TO_XML": true, "QUERY_TO_XMLSCHEMA": true, "QUERY_TO_XML_AND_XMLSCHEMA": true,
	"TS_STAT": true, "TS_REWRITE": true,
	"CROSSTAB": true, "CROSSTAB2": true, "CROSSTAB3": true, "CROSSTAB4": true, "CONNECTBY": true,
	// MySQL/MariaDB file read, and SQLite extension loading, file read/write,
	// directory listing, zip archive access and the external-editor hook.
	"LOAD_FILE": true, "LOAD_EXTENSION": true, "READFILE": true, "WRITEFILE": true, "FSDIR": true,
	"ZIPFILE": true, "EDIT": true,
	// MSSQL / Oracle defence in depth: shell-out and OS job scheduling.
	"XP_CMDSHELL": true, "SYS_EXEC": true, "SYS_EVAL": true, "DBMS_SCHEDULER.CREATE_JOB": true,
}

// sqlWriteDeniedPhrases name statement forms that export data off the server,
// bulk-import into it, extend/administer the engine, or bind a function without
// call parentheses, plus the PostgreSQL predefined roles that confer file,
// program or whole-schema access when granted. Matched as whole words against
// the identifier-tokenised upper text: punctuation is dropped, so a glued quote
// or operator cannot evade a phrase, and a phrase may also match across
// punctuation (erring on refusal). COPY, the MySQL INTO OUTFILE / INTO DUMPFILE
// forms and the CONNECT FILE_NAME table option are position-matched instead
// (see validateWriteStructure) so ordinary tables/columns named copy, outfile,
// dumpfile or file_name are not refused.
var sqlWriteDeniedPhrases = []string{
	// MySQL/MariaDB bulk import.
	"LOAD DATA", "LOAD XML",
	// Extension / procedural-language / aggregate / foreign-data / server /
	// operator / replication / tablespace and system administration. A leading
	// OR REPLACE, the optional TRUSTED / PROCEDURAL words and a MySQL
	// DEFINER=… / ALGORITHM=… / SQL SECURITY clause are stripped before matching
	// (stripWriteModifierClauses), so the contiguous CREATE … phrase still
	// catches CREATE OR REPLACE FUNCTION, CREATE TRUSTED LANGUAGE and
	// CREATE DEFINER=… PROCEDURE.
	"CREATE EXTENSION", "ALTER EXTENSION", "CREATE FUNCTION", "CREATE PROCEDURE",
	"CREATE LANGUAGE", "CREATE AGGREGATE", "CREATE SERVER", "CREATE FOREIGN DATA WRAPPER",
	"CREATE OPERATOR", "ALTER SYSTEM", "CREATE SUBSCRIPTION", "CREATE TABLESPACE",
	// PostgreSQL library preloading (ALTER ROLE/DATABASE/SYSTEM … SET). The
	// whole setting names are listed because '_' joins an identifier word.
	"SHARED_PRELOAD_LIBRARIES", "SESSION_PRELOAD_LIBRARIES", "LOCAL_PRELOAD_LIBRARIES",
	// MySQL/MariaDB table data/index files placed at a server path.
	"DATA DIRECTORY", "INDEX DIRECTORY",
	// MySQL/MariaDB plugin and component loading.
	"INSTALL PLUGIN", "INSTALL COMPONENT",
	// PostgreSQL predefined roles: *_SERVER_FILES / EXECUTE_SERVER_PROGRAM confer
	// server file and program access; *_ALL_DATA bypass per-table read/write
	// privileges. A GRANT of any of them is refused.
	"PG_READ_SERVER_FILES", "PG_WRITE_SERVER_FILES", "PG_EXECUTE_SERVER_PROGRAM",
	"PG_READ_ALL_DATA", "PG_WRITE_ALL_DATA",
}

// sqlWriteObjectNameContext lists keywords after which the next identifier is an
// object name being defined or referenced, not a function call. A denied name in
// one of these positions (a table/view/index/constraint literally named edit,
// e.g. INSERT INTO edit (…), CREATE TABLE edit (…), REFERENCES edit (id),
// CREATE VIEW edit (…), MySQL KEY edit (…)) is not a file/loader call, so its
// match is skipped. FROM/JOIN stay denied so a table-valued function such as
// fsdir('/') after FROM is still refused; ON is object context only inside a
// CREATE [UNIQUE] INDEX statement (handled in validateWriteStructure).
var sqlWriteObjectNameContext = map[string]bool{
	"INTO": true, "TABLE": true, "UPDATE": true, "EXISTS": true,
	"REFERENCES": true, "VIEW": true, "INDEX": true, "KEY": true,
}

// sqlCreateObjectKeyword marks the object-type keyword that ends a stripped
// CREATE-prefix modifier run.
var sqlCreateObjectKeyword = map[string]bool{
	"FUNCTION": true, "PROCEDURE": true, "TRIGGER": true, "EVENT": true, "VIEW": true,
	"AGGREGATE": true, "TABLE": true, "INDEX": true, "SEQUENCE": true, "SCHEMA": true,
	"DATABASE": true, "SERVER": true, "EXTENSION": true, "LANGUAGE": true, "OPERATOR": true,
	"TYPE": true, "DOMAIN": true, "ROLE": true, "USER": true,
}

// sqlWriteModifierKeyword marks the CREATE-prefix modifier keywords, so a
// DEFINER principal scan does not swallow the next modifier.
var sqlWriteModifierKeyword = map[string]bool{
	"OR": true, "DEFINER": true, "ALGORITHM": true, "SQL": true,
	"TRUSTED": true, "PROCEDURAL": true,
}

// whitespaceOnly reports whether s is empty or all ASCII/Unicode whitespace.
func whitespaceOnly(s string) bool { return strings.TrimSpace(s) == "" }

// connectFileNameTableOption reports whether the normalised statement carries a
// MySQL CONNECT-engine FILE_NAME table option: a FILE_NAME identifier at
// parenthesis depth 0 that is immediately (bar whitespace) followed by '=',
// appearing before the first top-level SELECT (so a FILE_NAME= table option is
// caught, while a file_name column inside the column list — depth > 0 — or in
// the body of a CREATE TABLE … AS SELECT is not). Caller restricts this to
// CREATE/ALTER TABLE statements.
func connectFileNameTableOption(ps string) bool {
	depth := 0
	for i := 0; i < len(ps); {
		c := ps[i]
		switch {
		case c == '(':
			depth++
			i++
		case c == ')':
			if depth > 0 {
				depth--
			}
			i++
		case isSQLIdentChar(rune(c)):
			j := i
			for j < len(ps) && isSQLIdentChar(rune(ps[j])) {
				j++
			}
			if depth == 0 {
				word := strings.ToUpper(ps[i:j])
				if word == "SELECT" {
					return false // CTAS body: options can only precede it
				}
				if word == "FILE_NAME" {
					k := j
					for k < len(ps) && (ps[k] == ' ' || ps[k] == '\t' || ps[k] == '\n' || ps[k] == '\r') {
						k++
					}
					if k < len(ps) && ps[k] == '=' {
						return true
					}
				}
			}
			i = j
		default:
			i++
		}
	}
	return false
}

// stripWriteModifierClauses removes the optional CREATE-statement prefix
// modifiers that would otherwise split a denied phrase: a leading OR REPLACE and
// the MySQL DEFINER=<principal> / ALGORITHM=<x> / SQL SECURITY <x> clauses. It
// transforms ONLY the prefix of a CREATE statement and copies everything from
// the first non-modifier token verbatim, so a DEFINER, ALGORITHM or OR token
// appearing in a column name, SET clause or WHERE clause of any statement is
// never stripped (that earlier fail-open dropped the rest of such statements).
// Tokens are identifier-only and upper-case (punctuation already removed).
func stripWriteModifierClauses(tokens []string) []string {
	if len(tokens) == 0 || tokens[0] != "CREATE" {
		return tokens
	}
	out := []string{"CREATE"}
	i := 1
	for i < len(tokens) {
		switch {
		case tokens[i] == "OR" && i+1 < len(tokens) && tokens[i+1] == "REPLACE":
			i += 2
		case tokens[i] == "TRUSTED" || tokens[i] == "PROCEDURAL":
			// PostgreSQL CREATE [TRUSTED] [PROCEDURAL] LANGUAGE: drop the optional
			// words so the contiguous CREATE LANGUAGE phrase still matches. They
			// only appear here in the prefix (a table named trusted/procedural is
			// reached only after the object keyword, which returns below).
			i++
		case tokens[i] == "ALGORITHM":
			i++
			if i < len(tokens) && !sqlCreateObjectKeyword[tokens[i]] && !sqlWriteModifierKeyword[tokens[i]] {
				i++ // the algorithm value (e.g. COPY / INPLACE / MERGE)
			}
		case tokens[i] == "SQL" && i+1 < len(tokens) && tokens[i+1] == "SECURITY":
			i += 2
			if i < len(tokens) && !sqlCreateObjectKeyword[tokens[i]] && !sqlWriteModifierKeyword[tokens[i]] {
				i++ // DEFINER or INVOKER
			}
		case tokens[i] == "DEFINER":
			i++
			// principal: CURRENT_USER / CURRENT_ROLE / SESSION_USER, or user[@host]
			// — any number of tokens (a dotted host or an IPv4 literal expands to
			// several). Scan up to the object keyword or the next modifier, so a
			// multi-token principal (CREATE DEFINER=root@127.0.0.1 FUNCTION …) no
			// longer hides the object keyword behind a fixed two-token bound. A
			// real CREATE … FUNCTION/PROCEDURE/VIEW always has an object keyword,
			// so the scan stops there; the only way to reach the end is a prefix
			// with no object keyword, which carries no phrase to match anyway.
			for i < len(tokens) && !sqlCreateObjectKeyword[tokens[i]] && !sqlWriteModifierKeyword[tokens[i]] {
				i++
			}
		default:
			out = append(out, tokens[i:]...)
			return out
		}
	}
	return out
}

// precedingIdentToken returns the upper-case identifier run that immediately
// precedes position start in s, skipping any non-identifier separators between
// them, together with sepStart: the index just past that identifier, i.e. the
// start of the separator run between the identifier and start (so the caller
// can inspect s[sepStart:start]). It is O(token length) with no allocation of
// the whole prefix, so it stays cheap even for a statement with thousands of
// function calls.
func precedingIdentToken(s string, start int) (ident string, sepStart int) {
	i := start
	for i > 0 && !isSQLIdentChar(rune(s[i-1])) {
		i--
	}
	end := i
	for i > 0 && isSQLIdentChar(rune(s[i-1])) {
		i--
	}
	return strings.ToUpper(s[i:end]), end
}

// validateWriteStructure rejects file, loader and administrative SQL in write
// and DDL statements without constraining ordinary functions. Unlike the read
// path it is a denylist: every ordinary write function and statement form keeps
// working; only server-reaching names and forms are refused. It re-normalises
// the original query in write-padded mode (sqlStructureWrite) so quoted/glued
// tokens, schema prefixes, DEFINER clauses and U&"…" identifiers cannot smuggle
// a denied name past the check; the read path's normalisation is unchanged.
func validateWriteStructure(query, driver string) error {
	ps, err := sqlStructureWrite(query, driver)
	if err != nil {
		return err
	}

	// Identifier-only tokens for phrase matching: punctuation (quotes, '=', '@',
	// parens, dots, commas) is a boundary, so a token glued to punctuation cannot
	// hide a phrase, and a phrase never spans across punctuation.
	words := strings.FieldsFunc(strings.ToUpper(ps), func(r rune) bool { return !isSQLIdentChar(r) })
	words = stripWriteModifierClauses(words)

	// COPY (bulk file copy / COPY … TO|FROM PROGRAM) only ever starts a statement
	// or follows DO in a trigger/event body; the classifier already rejects a
	// leading COPY, so this is defence in depth for the embedded form. Restricting
	// it to those positions keeps ordinary columns/options named "copy" writable.
	if len(words) > 0 && words[0] == "COPY" {
		return fmt.Errorf("file, loader or administrative SQL is not allowed: COPY")
	}
	for i := 0; i+1 < len(words); i++ {
		if words[i] == "DO" && words[i+1] == "COPY" {
			return fmt.Errorf("file, loader or administrative SQL is not allowed: COPY")
		}
	}

	// SQLite VACUUM INTO writes a copy of the database to an arbitrary host path.
	// VACUUM is classified DDL (so it reaches here); plain VACUUM / VACUUM <name>
	// stays allowed, only the INTO form is refused.
	if len(words) > 0 && words[0] == "VACUUM" {
		for _, w := range words[1:] {
			if w == "INTO" {
				return fmt.Errorf("file, loader or administrative SQL is not allowed: VACUUM INTO")
			}
		}
	}

	// MySQL CONNECT storage engine: a FILE_NAME table option binds a table to a
	// server file. It is a table option (CREATE/ALTER TABLE, at parenthesis
	// depth 0, before any CTAS SELECT, FILE_NAME followed by '='), so an ordinary
	// file_name column — inside the column list, in a SET clause, or added with
	// ADD COLUMN — is not refused.
	if len(words) >= 2 && (words[0] == "CREATE" || words[0] == "ALTER") && words[1] == "TABLE" &&
		connectFileNameTableOption(ps) {
		return fmt.Errorf("file, loader or administrative SQL is not allowed: FILE_NAME")
	}

	// MySQL file export: SELECT ... INTO OUTFILE/DUMPFILE. Match the two-word form
	// but skip the INSERT/REPLACE target-table INTO (INSERT INTO outfile ...), so a
	// table literally named outfile or dumpfile stays writable.
	for i := 1; i+1 < len(words); i++ {
		if words[i] == "INTO" && (words[i+1] == "OUTFILE" || words[i+1] == "DUMPFILE") {
			if words[i-1] == "INSERT" || words[i-1] == "REPLACE" {
				continue
			}
			return fmt.Errorf("file, loader or administrative SQL is not allowed: INTO %s", words[i+1])
		}
	}

	joined := " " + strings.Join(words, " ") + " "
	for _, p := range sqlWriteDeniedPhrases {
		if strings.Contains(joined, " "+p+" ") {
			return fmt.Errorf("file, loader or administrative SQL is not allowed: %s", p)
		}
	}

	createIndex := len(words) >= 2 && words[0] == "CREATE" &&
		(words[1] == "INDEX" || (len(words) >= 3 && words[1] == "UNIQUE" && words[2] == "INDEX"))

	for _, m := range sqlFunctionPattern.FindAllStringSubmatchIndex(ps, -1) {
		raw := ps[m[2]:m[3]]
		name := strings.ToUpper(raw)
		prev, sepStart := precedingIdentToken(ps, m[2])
		// Only treat prev as an object-name-introducing keyword when the gap
		// between it and the name is pure whitespace (INTO edit (, TABLE edit (,
		// KEY `edit` (). A keyword reached through punctuation — an alias or
		// column spelled KEY/INDEX/VIEW/… then a comma or '=' before the call
		// (1 AS key, pg_read_file(...); SET key = pg_read_file(...)) — is not an
		// object-name position, so the denied call is still checked.
		if whitespaceOnly(ps[sepStart:m[2]]) && (sqlWriteObjectNameContext[prev] || (prev == "ON" && createIndex)) {
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
