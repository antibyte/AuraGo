package sqlconnections

import "strings"

// Classification alone cannot describe statements that perform several mutations.
func checkQueryPermissions(conn ConnectionRecord, kind StatementType, query string) error {
	if err := CheckPermission(conn, kind); err != nil {
		return err
	}
	if kind == StmtSelect {
		return nil
	}
	structure, err := sqlStructureWrite(query, conn.Driver)
	if err != nil {
		return err
	}
	tokens := strings.FieldsFunc(strings.ToUpper(structure), func(r rune) bool { return !isSQLIdentChar(r) })
	leading := firstKeyword(structure)
	if leading == "WITH" {
		leading = cteLeadingDML(structure)
	}
	change, remove := false, leading == "REPLACE"
	for i, token := range tokens {
		// Quoted string contents have already been replaced by the dialect lexer.
		if token == "DO" && i+1 < len(tokens) && tokens[i+1] == "UPDATE" {
			change = true
		}
		if token == "ON" && i+3 < len(tokens) && tokens[i+1] == "DUPLICATE" && tokens[i+2] == "KEY" && tokens[i+3] == "UPDATE" {
			change = true
		}
		if token == "OR" && i+1 < len(tokens) && tokens[i+1] == "REPLACE" {
			remove = true
		}
		if (kind == StmtDDL && (token == "DROP" || token == "TRUNCATE")) ||
			(token == "REPLACE" && (i == 0 || (i > 0 && tokens[i-1] == "AS"))) {
			remove = true
		}
	}
	if change {
		if err := CheckPermission(conn, StmtUpdate); err != nil {
			return err
		}
	}
	if remove {
		return CheckPermission(conn, StmtDelete)
	}
	return nil
}
