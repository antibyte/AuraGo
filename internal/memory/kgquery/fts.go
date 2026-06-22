package kgquery

import "strings"

// EscapeFTS5 prepares a user query for FTS5 MATCH.
func EscapeFTS5(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return `""`
	}
	if len(query) >= 2 && strings.HasPrefix(query, `"`) && strings.HasSuffix(query, `"`) {
		inner := strings.TrimSpace(query[1 : len(query)-1])
		if inner == "" {
			return `""`
		}
		return `"` + strings.ReplaceAll(inner, `"`, `""`) + `"`
	}
	words := strings.Fields(query)
	if len(words) == 0 {
		return `""`
	}
	var escaped []string
	for _, w := range words {
		w = strings.ReplaceAll(w, `"`, `""`)
		if w != "" {
			escaped = append(escaped, `"`+w+`"`)
		}
	}
	if len(escaped) == 0 {
		return `""`
	}
	if len(escaped) == 1 {
		return escaped[0]
	}
	return strings.Join(escaped, " AND ")
}