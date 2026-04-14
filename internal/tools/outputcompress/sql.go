// Package outputcompress – SQL query output compressors.
//
// SQL tool outputs follow three patterns from agent_dispatch_services.go:
//   - query:    {"status":"success","result":[{...},{...},...]}
//   - describe: {"status":"success","table":"...","columns":[{...},{...},...]}
//   - list_tables: {"status":"success","tables":[...],"count":N}
//
// These compressors:
//   - Strip the "Tool Output: " wrapper prefix
//   - Convert verbose JSON rows to compact table format
//   - Limit large result sets with "+N more rows"
//   - Keep column metadata compact for describe
//   - Keep table lists compact for list_tables
package outputcompress

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// compressSQLOutput routes SQL tool output to the appropriate sub-compressor.
func compressSQLOutput(output string) (string, string) {
	clean := StripANSI(output)
	clean = strings.TrimPrefix(clean, "Tool Output: ")
	clean = strings.TrimSpace(clean)

	// Must be JSON
	if !strings.HasPrefix(clean, "{") {
		return compressGeneric(output), "sql-nonjson"
	}

	// Parse top-level
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(clean), &raw); err != nil {
		return compressGeneric(output), "sql-parse-err"
	}

	// Error responses: return as-is
	if statusStr := jsonString(raw["status"]); statusStr == "error" {
		return clean, "sql-error"
	}

	// Detect response type
	switch {
	case raw["result"] != nil:
		return compressSQLQueryResult(raw), "sql-query"
	case raw["columns"] != nil:
		return compressSQLDescribe(raw), "sql-describe"
	case raw["tables"] != nil:
		return compressSQLListTables(raw), "sql-list-tables"
	default:
		return compactJSON(clean), "sql-generic"
	}
}

// compressSQLQueryResult compresses SQL query results.
// From: {"status":"success","result":[{"id":1,"name":"Alice","email":"alice@example.com"},...]}
// To:   "N rows (3 cols: id, name, email):\n  id=1 name=Alice email=alice@example.com\n  ..."
func compressSQLQueryResult(raw map[string]json.RawMessage) string {
	var rows []map[string]interface{}
	if err := json.Unmarshal(raw["result"], &rows); err != nil {
		return compactJSON(rawToString(raw))
	}

	if len(rows) == 0 {
		return "0 rows returned"
	}

	// Collect column names from first row
	cols := sortedKeys(rows[0])

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d rows (%d cols: %s):\n", len(rows), len(cols), strings.Join(cols, ", "))

	limit := 20
	if len(rows) < limit {
		limit = len(rows)
	}

	for i := 0; i < limit; i++ {
		row := rows[i]
		parts := make([]string, 0, len(cols))
		for _, col := range cols {
			val := fmt.Sprintf("%v", row[col])
			if len(val) > 50 {
				val = val[:47] + "..."
			}
			parts = append(parts, col+"="+val)
		}
		sb.WriteString("  " + strings.Join(parts, " ") + "\n")
	}

	if len(rows) > limit {
		fmt.Fprintf(&sb, "  + %d more rows\n", len(rows)-limit)
	}

	return sb.String()
}

// compressSQLDescribe compresses table describe output.
// From: {"status":"success","table":"users","columns":[{"name":"id","type":"INTEGER","notnull":true,"pk":true},...]}
// To:   "Table users (4 columns):\n  id INTEGER PK NOT NULL\n  name TEXT NOT NULL\n  ..."
func compressSQLDescribe(raw map[string]json.RawMessage) string {
	tableName := jsonString(raw["table"])

	var columns []map[string]interface{}
	if err := json.Unmarshal(raw["columns"], &columns); err != nil {
		return compactJSON(rawToString(raw))
	}

	var sb strings.Builder
	if tableName != "" {
		fmt.Fprintf(&sb, "Table %s ", tableName)
	}
	fmt.Fprintf(&sb, "(%d columns):\n", len(columns))

	for _, col := range columns {
		name := fmt.Sprintf("%v", col["name"])
		typ := fmt.Sprintf("%v", col["type"])
		sb.WriteString("  " + name + " " + typ)

		if isTrue(col["pk"]) || isTrue(col["primary_key"]) {
			sb.WriteString(" PK")
		}
		if isTrue(col["notnull"]) || isTrue(col["not_null"]) {
			sb.WriteString(" NOT NULL")
		}
		if isTrue(col["unique"]) {
			sb.WriteString(" UNIQUE")
		}
		if defVal, ok := col["default_value"]; ok && defVal != nil {
			dv := fmt.Sprintf("%v", defVal)
			if dv != "" && dv != "<nil>" {
				sb.WriteString(" DEFAULT " + dv)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// compressSQLListTables compresses table list output.
// From: {"status":"success","tables":["users","orders","products",...],"count":15}
// To:   "15 tables:\n  users\n  orders\n  products\n  ..."
func compressSQLListTables(raw map[string]json.RawMessage) string {
	count := jsonInt(raw["count"])

	// Tables can be strings or objects
	var tableNames []string

	// Try string array first
	var strTables []string
	if err := json.Unmarshal(raw["tables"], &strTables); err == nil {
		tableNames = strTables
	} else {
		// Try object array
		var objTables []map[string]interface{}
		if err := json.Unmarshal(raw["tables"], &objTables); err != nil {
			return compactJSON(rawToString(raw))
		}
		for _, t := range objTables {
			if name, ok := t["name"]; ok {
				tableNames = append(tableNames, fmt.Sprintf("%v", name))
			} else if name, ok := t["table_name"]; ok {
				tableNames = append(tableNames, fmt.Sprintf("%v", name))
			}
		}
	}

	var sb strings.Builder
	if count > 0 {
		fmt.Fprintf(&sb, "%d tables", count)
	} else {
		fmt.Fprintf(&sb, "%d tables", len(tableNames))
	}
	sb.WriteString(":\n")

	limit := 50
	if len(tableNames) < limit {
		limit = len(tableNames)
	}

	// Show in columns for space efficiency
	cols := 4
	colWidth := 0
	for _, n := range tableNames {
		if len(n) > colWidth {
			colWidth = len(n)
		}
	}
	colWidth += 2
	if colWidth > 30 {
		colWidth = 30
	}

	for i := 0; i < limit; i++ {
		name := tableNames[i]
		if len(name) > colWidth-2 {
			name = name[:colWidth-5] + "..."
		}
		fmt.Fprintf(&sb, "%-*s", colWidth, name)
		if (i+1)%cols == 0 {
			sb.WriteString("\n")
		}
	}
	if limit%cols != 0 {
		sb.WriteString("\n")
	}

	if len(tableNames) > limit {
		fmt.Fprintf(&sb, "  + %d more\n", len(tableNames)-limit)
	}

	return sb.String()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isTrue checks if a value is truthy (true or "true" or 1).
func isTrue(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case int:
		return val != 0
	case string:
		return strings.EqualFold(val, "true") || val == "1"
	default:
		return false
	}
}
