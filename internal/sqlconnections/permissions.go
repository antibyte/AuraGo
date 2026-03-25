package sqlconnections

import (
	"fmt"
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
	case "SELECT", "SHOW", "DESCRIBE", "DESC", "EXPLAIN", "PRAGMA":
		return StmtSelect, nil
	case "WITH":
		// Common Table Expression — check if it leads to SELECT or a write statement
		inner := cteLeadingDML(trimmed)
		switch inner {
		case "SELECT":
			return StmtSelect, nil
		case "INSERT":
			return StmtInsert, nil
		case "UPDATE":
			return StmtUpdate, nil
		case "DELETE":
			return StmtDelete, nil
		default:
			return StmtSelect, nil // conservative: treat unknown CTE as read
		}
	case "INSERT", "REPLACE":
		return StmtInsert, nil
	case "UPDATE":
		return StmtUpdate, nil
	case "DELETE", "TRUNCATE":
		return StmtDelete, nil
	case "CREATE", "DROP", "ALTER":
		return StmtDDL, nil
	default:
		return StmtUnknown, fmt.Errorf("unsupported SQL statement: %s", keyword)
	}
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

// cteLeadingDML tries to find the DML keyword after a WITH ... AS (...) block.
func cteLeadingDML(query string) string {
	upper := strings.ToUpper(query)
	// Find the last unmatched closing paren, then grab the next keyword.
	depth := 0
	i := 0
	// Skip "WITH"
	for i < len(upper) && (unicode.IsLetter(rune(upper[i])) || upper[i] == ' ') {
		i++
	}
	for i < len(upper) {
		switch upper[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				// Look ahead for the DML keyword
				rest := strings.TrimSpace(upper[i+1:])
				// Skip optional comma + another CTE definition
				for strings.HasPrefix(rest, ",") {
					rest = strings.TrimSpace(rest[1:])
					// skip CTE name + AS + ( ... )
					innerDepth := 0
					for j := 0; j < len(rest); j++ {
						if rest[j] == '(' {
							innerDepth++
						} else if rest[j] == ')' {
							innerDepth--
							if innerDepth == 0 {
								rest = strings.TrimSpace(rest[j+1:])
								break
							}
						}
					}
				}
				kw := firstKeyword(rest)
				if kw != "" {
					return kw
				}
			}
		}
		i++
	}
	return "SELECT" // default assumption
}
