package memory

import (
	"errors"
	"fmt"
)

type sqliteErrorWithCode interface {
	Code() int
}

func isSQLiteCorruptionError(err error) bool {
	var sqliteErr sqliteErrorWithCode
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == 11 // SQLITE_CORRUPT, including extended codes.
}

// repairKGFTSAfterQueryFailure repairs only a missing FTS artifact or a failed
// FTS5 integrity check. A malformed MATCH expression or an unrelated read error
// must not trigger a destructive rebuild.
func (kg *KnowledgeGraph) repairKGFTSAfterQueryFailure(queryErr error) {
	if kg == nil || kg.db == nil || queryErr == nil {
		return
	}
	kg.ftsMu.Lock()
	defer kg.ftsMu.Unlock()

	tx, err := kg.db.Begin()
	if err != nil {
		return
	}
	provenCorruption := false
	var integrityErr error
	for _, table := range []string{"kg_nodes_fts", "kg_edges_fts"} {
		for _, artifact := range []string{table, table + "_data", table + "_idx", table + "_docsize", table + "_config"} {
			var exists bool
			if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, artifact).Scan(&exists); err != nil {
				integrityErr = err
				break
			}
			if !exists {
				provenCorruption = true
				integrityErr = fmt.Errorf("FTS artifact %s is missing", artifact)
				break
			}
		}
		if provenCorruption || integrityErr != nil {
			break
		}
		if _, err := tx.Exec(`INSERT INTO ` + table + `(` + table + `, rank) VALUES('integrity-check', 1)`); err != nil {
			provenCorruption = isSQLiteCorruptionError(err)
			integrityErr = err
			break
		}
	}
	_ = tx.Rollback()
	if !provenCorruption {
		return
	}
	if kg.logger != nil {
		kg.logger.Warn("KG FTS integrity check failed; rebuilding indexes", "query_error", queryErr, "integrity_error", integrityErr)
	}
	if err := kg.rebuildFTSIndexes(); err != nil && kg.logger != nil {
		kg.logger.Error("KG FTS integrity repair failed", "error", err)
	}
}
