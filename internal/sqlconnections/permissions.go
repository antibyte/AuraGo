package sqlconnections

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// StatementType classifies a SQL statement for permission enforcement.
type StatementType int

const (
	StmtUnknown StatementType = iota
	StmtSelect                // SELECT, SHOW, DESCRIBE, EXPLAIN, WITH ... SELECT
	StmtInsert                // INSERT
	StmtUpdate                // UPDATE
	StmtDelete                // DELETE, TRUNCATE
	StmtDDL                   // CREATE, DROP, ALTER
)

var connRefRe = regexp.MustCompile(`connection ['"][^'"]+['"]`)

// String returns a human-readable name for the statement type.
func (s StatementType) String() string {
	switch s {
	case StmtSelect:
		return "SELECT"
	case StmtInsert:
		return "INSERT"
	case StmtUpdate:
		return "UPDATE"
	case StmtDelete:
		return "DELETE"
	case StmtDDL:
		return "DDL"
	default:
		return "UNKNOWN"
	}
}

// stripSQLComments removes SQL line comments (--) and block comments (/* ... */) from a query string.
func stripSQLComments(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		// Block comment /* ... */
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "*/")
			if end == -1 {
				break // unterminated block comment: discard rest
			}
			i = i + 2 + end + 2
			result.WriteByte(' ')
			continue
		}
		// Line comment --
		if i+1 < len(s) && s[i] == '-' && s[i+1] == '-' {
			nl := strings.IndexByte(s[i:], '\n')
			if nl == -1 {
				break // comment to end of string
			}
			i = i + nl + 1
			result.WriteByte(' ')
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// DetectStatementType parses the leading keyword(s) of a SQL string to classify it.
// Only single statements are allowed — semicolons are rejected.
//
// SECURITY NOTE: This function is conservative — ambiguous or unknown statements
// are blocked rather than allowed. Edge cases like PRAGMA, EXPLAIN variants,
// CTEs with unknown inner DML, and administrative commands are treated as
// potentially dangerous and blocked.
func DetectStatementType(query string) (StatementType, error) {
	trimmed := strings.TrimSpace(stripSQLComments(query))
	if trimmed == "" {
		return StmtUnknown, fmt.Errorf("empty query")
	}

	// Block statement chaining: reject if there is a semicolon that is not trailing whitespace.
	if idx := strings.Index(trimmed, ";"); idx >= 0 {
		after := strings.TrimSpace(trimmed[idx+1:])
		if after != "" {
			return StmtUnknown, fmt.Errorf("multiple statements are not allowed")
		}
		// trailing semicolon is fine — strip it for keyword detection
		trimmed = strings.TrimSpace(trimmed[:idx])
	}

	keyword := firstKeyword(trimmed)

	switch keyword {
	case "SELECT":
		// Check for EXPLAIN SELECT (still read-only)
		if isExplainQuery(trimmed) {
			return StmtSelect, nil
		}
		return StmtSelect, nil

	case "SHOW", "DESCRIBE", "DESC":
		// SHOW and DESCRIBE are informational (read-only)
		return StmtSelect, nil

	case "EXPLAIN":
		// EXPLAIN without SELECT is ambiguous — block conservatively.
		// Some databases use EXPLAIN for analyzing INSERT/UPDATE/DELETE plans.
		if isExplainSelectOnly(trimmed) {
			return StmtSelect, nil
		}
		return StmtUnknown, fmt.Errorf("EXPLAIN for non-SELECT statements is not allowed")

	case "PRAGMA":
		// PRAGMA can modify database state (e.g., PRAGMA auto_vacuum=1).
		// Block conservatively — most PRAGMA usage is informational only.
		return StmtUnknown, fmt.Errorf("PRAGMA statements are not allowed for security reasons")

	case "WITH":
		// Common Table Expression — conservatively analyze the inner statement.
		// If we cannot definitively determine it's SELECT, block it.
		inner := cteLeadingDML(trimmed)
		switch inner {
		case "SELECT", "SHOW", "DESCRIBE", "DESC":
			return StmtSelect, nil
		case "INSERT", "REPLACE":
			return StmtInsert, nil
		case "UPDATE":
			return StmtUpdate, nil
		case "DELETE":
			return StmtDelete, nil
		case "CREATE", "DROP", "ALTER", "TRUNCATE", "VACUUM", "ANALYZE", "REINDEX", "OPTIMIZE", "CHECK", "REPAIR", "GRANT", "REVOKE", "DENY":
			return StmtDDL, nil
		case "CALL":
			return StmtUnknown, fmt.Errorf("CALL statements are not allowed")
		default:
			// Conservative: block unknown CTE rather than assuming read-only
			return StmtUnknown, fmt.Errorf("ambiguous CTE statement — only known read/write/DDL statements allowed")
		}

	case "INSERT", "REPLACE":
		return StmtInsert, nil
	case "UPDATE":
		return StmtUpdate, nil
	case "DELETE":
		return StmtDelete, nil
	case "TRUNCATE":
		// TRUNCATE is DDL (not just DML) — requires allow_write AND allow_change
		return StmtDDL, nil
	case "CREATE", "DROP", "ALTER":
		return StmtDDL, nil

		// Administrative commands — block these as they are not typical SQL queries
	case "VACUUM", "ANALYZE", "REINDEX", "OPTIMIZE", "CHECK", "REPAIR":
		return StmtDDL, nil

	case "USE", "SET", "RESET", "START", "COMMIT", "ROLLBACK", "BEGIN":
		// Session/state-modifying commands are not standard data queries
		return StmtUnknown, fmt.Errorf("administrative statements (USE/SET/RESET/etc.) are not allowed")

	case "CALL":
		// Stored procedure calls can have side effects — block conservatively
		return StmtUnknown, fmt.Errorf("CALL statements are not allowed")

	case "GRANT", "REVOKE", "DENY":
		// Permission changes are DDL-like
		return StmtDDL, nil

	default:
		return StmtUnknown, fmt.Errorf("unsupported SQL statement: %s", keyword)
	}
}

// isExplainQuery checks if the query is EXPLAIN SELECT (or EXPLAIN QUERY PLAN).
func isExplainQuery(query string) bool {
	upper := strings.ToUpper(query)
	return strings.HasPrefix(upper, "EXPLAIN ") || strings.HasPrefix(upper, "EXPLAIN/")
}

// isExplainSelectOnly checks if this is EXPLAIN SELECT or EXPLAIN QUERY PLAN only.
func isExplainSelectOnly(query string) bool {
	upper := strings.TrimSpace(strings.ToUpper(query))
	// EXPLAIN SELECT ... or EXPLAIN QUERY PLAN ...
	return strings.HasPrefix(upper, "EXPLAIN SELECT") ||
		strings.HasPrefix(upper, "EXPLAIN QUERY PLAN")
}

// CheckPermission verifies that the connection allows the given statement type.
func CheckPermission(conn ConnectionRecord, stmt StatementType) error {
	switch stmt {
	case StmtSelect:
		if !conn.AllowRead {
			return fmt.Errorf("permission denied: connection %q does not allow SELECT (read)", conn.Name)
		}
	case StmtInsert:
		if !conn.AllowWrite {
			return fmt.Errorf("permission denied: connection %q does not allow INSERT (write)", conn.Name)
		}
	case StmtUpdate:
		if !conn.AllowChange {
			return fmt.Errorf("permission denied: connection %q does not allow UPDATE (change)", conn.Name)
		}
	case StmtDelete:
		if !conn.AllowDelete {
			return fmt.Errorf("permission denied: connection %q does not allow DELETE (delete)", conn.Name)
		}
	case StmtDDL:
		if !conn.AllowWrite || !conn.AllowChange {
			return fmt.Errorf("permission denied: connection %q does not allow DDL (requires write + change)", conn.Name)
		}
	case StmtUnknown:
		return fmt.Errorf("unknown statement type — execution blocked for safety")
	}
	return nil
}

// SanitizeError returns a user-safe error message by stripping potentially sensitive
// internal details (connection names, driver-specific messages, wrapped errors).
func SanitizeError(err error) string {
	if err == nil {
		return "unknown error"
	}
	msg := err.Error()

	// Strip connection name references that might leak internal naming
	// e.g. "connection 'my-db' not found" -> "connection not found"
	msg = stripConnectionRef(msg)

	// Strip driver-specific error suffixes that might leak implementation details
	// e.g. "failed to connect: driver error: ..." -> "failed to connect"
	if idx := strings.Index(msg, ": driver"); idx > 0 {
		msg = msg[:idx]
	}

	// Strip postgres/mysql error codes that might reveal server details
	// e.g. "error: permission denied for table users" -> "permission denied"
	if idx := strings.Index(msg, "pq: "); idx >= 0 {
		msg = msg[idx+4:]
	}
	if idx := strings.Index(msg, "Error "); idx >= 0 && len(msg) > idx+6 && msg[idx+6] >= '0' && msg[idx+6] <= '9' {
		// Strip MySQL error codes like "Error 1045"
		rest := msg[idx+5:]
		if len(rest) > 4 {
			rest = strings.TrimLeft(rest[4:], " ")
			if len(rest) > 0 && (rest[0] < '0' || rest[0] > '9') {
				msg = msg[:idx] + rest
			}
		}
	}

	return msg
}

// stripConnectionRef removes connection name from error messages to prevent leaking
// internal naming conventions while keeping the error meaningful.
func stripConnectionRef(msg string) string {
	return connRefRe.ReplaceAllString(msg, "connection '***'")
}

// firstKeyword extracts the first SQL keyword (uppercase).
func firstKeyword(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || r == '_' {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			break
		}
	}
	return b.String()
}

// cteLeadingDML inspects WITH CTE bodies before returning the outer statement.
// A mutating or administrative CTE body is returned immediately so callers can
// enforce the stronger permission, even when the outer statement is SELECT.
func cteLeadingDML(query string) string {
	i := skipSQLSpace(query, len("WITH"))
	if hasKeywordAt(query, i, "RECURSIVE") {
		i = skipSQLSpace(query, i+len("RECURSIVE"))
	}

	for i < len(query) {
		asIdx := findSQLKeywordAtDepthZero(query, i, "AS")
		if asIdx == -1 {
			return ""
		}

		openIdx := findSQLByteAtDepthZero(query, asIdx+len("AS"), '(')
		if openIdx == -1 {
			return ""
		}

		closeIdx := findMatchingSQLParen(query, openIdx)
		if closeIdx == -1 {
			return ""
		}

		bodyKeyword := cteBodyLeadingKeyword(query[openIdx+1 : closeIdx])
		if bodyKeyword == "" {
			return ""
		}
		if !isReadOnlyCTEBodyKeyword(bodyKeyword) {
			return bodyKeyword
		}

		i = skipSQLSpace(query, closeIdx+1)
		if i < len(query) && query[i] == ',' {
			i = skipSQLSpace(query, i+1)
			continue
		}

		outerKeyword := firstKeyword(strings.TrimSpace(query[i:]))
		if outerKeyword != "" {
			return outerKeyword
		}
		return ""
	}
	return ""
}

func cteBodyLeadingKeyword(body string) string {
	body = strings.TrimSpace(body)
	keyword := firstKeyword(body)
	if keyword == "WITH" {
		return cteLeadingDML(body)
	}
	return keyword
}

func isReadOnlyCTEBodyKeyword(keyword string) bool {
	switch keyword {
	case "SELECT", "SHOW", "DESCRIBE", "DESC":
		return true
	default:
		return false
	}
}

func skipSQLSpace(s string, i int) int {
	for i < len(s) && unicode.IsSpace(rune(s[i])) {
		i++
	}
	return i
}

func findSQLKeywordAtDepthZero(s string, start int, keyword string) int {
	depth := 0
	quote := byte(0)
	for i := start; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				quote = 0
			}
			continue
		}

		switch c {
		case '\'', '"', '`':
			quote = c
		case '[':
			quote = ']'
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && hasKeywordAt(s, i, keyword) {
				return i
			}
		}
	}
	return -1
}

func findSQLByteAtDepthZero(s string, start int, target byte) int {
	depth := 0
	quote := byte(0)
	for i := start; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				quote = 0
			}
			continue
		}

		switch c {
		case '\'', '"', '`':
			quote = c
		case '[':
			quote = ']'
		case '(':
			if depth == 0 && c == target {
				return i
			}
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && c == target {
				return i
			}
		}
	}
	return -1
}

func findMatchingSQLParen(s string, openIdx int) int {
	depth := 0
	quote := byte(0)
	for i := openIdx; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				quote = 0
			}
			continue
		}

		switch c {
		case '\'', '"', '`':
			quote = c
		case '[':
			quote = ']'
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func hasKeywordAt(s string, idx int, keyword string) bool {
	if idx < 0 || idx+len(keyword) > len(s) {
		return false
	}
	if !strings.EqualFold(s[idx:idx+len(keyword)], keyword) {
		return false
	}
	beforeOK := idx == 0 || !isSQLIdentChar(rune(s[idx-1]))
	afterIdx := idx + len(keyword)
	afterOK := afterIdx == len(s) || !isSQLIdentChar(rune(s[afterIdx]))
	return beforeOK && afterOK
}

func isSQLIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
