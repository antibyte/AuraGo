package sqlconnections

import (
	"fmt"
	"regexp"
	"strings"
)

// sqlStructure retains SQL syntax while replacing literal contents. Unsupported
// escape modes are rejected: a connection's SQL mode must never change our boundary.
func sqlStructure(s string) (string, error) { return sqlStructureDialect(s, "") }

func sqlStructureDialect(s, driver string) (string, error) {
	var out strings.Builder
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
				out.WriteString("'literal'")
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
				out.WriteString("'literal'")
			} else {
				out.WriteString(ident.String())
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
