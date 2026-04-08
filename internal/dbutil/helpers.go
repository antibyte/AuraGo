package dbutil

import (
	"database/sql"
	"strings"
)

// EscapeLike escapes LIKE metacharacters (%, _) in a string for safe use in LIKE queries.
// The caller must include ESCAPE '\' in the LIKE clause of the SQL query.
func EscapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// BoolToInt converts a bool to an int (1 for true, 0 for false).
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// NullStr converts a string to sql.NullString for nullable columns.
// Returns nil for empty strings, suitable for write paths where empty strings should become NULL.
func NullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	// Use sql.NullString for proper nullable handling
	return sql.NullString{String: s, Valid: true}
}
