package optimizer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"aurago/internal/prompts"
)

func initializeExposureSchema(db *OptimizerDB) error {
	if _, err := db.db.Exec(`CREATE TABLE IF NOT EXISTS prompt_exposures (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		manual TEXT NOT NULL, version TEXT NOT NULL,
		source_revision TEXT NOT NULL, prompt_revision TEXT NOT NULL
	)`); err != nil {
		return err
	}
	for _, column := range []struct{ name, ddl string }{
		{"exposure_id", "INTEGER REFERENCES prompt_exposures(id)"}, {"operation", "TEXT NOT NULL DEFAULT ''"},
		{"action_identity", "TEXT NOT NULL DEFAULT ''"},
	} {
		var exists bool
		if err := db.db.QueryRow(`SELECT count(*) > 0 FROM pragma_table_info('tool_traces') WHERE name=?`, column.name).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if _, err := db.db.Exec("ALTER TABLE tool_traces ADD COLUMN " + column.name + " " + column.ddl); err != nil {
				return err
			}
		}
	}
	var hasReason bool
	if err := db.db.QueryRow(`SELECT count(*)>0 FROM pragma_table_info('prompt_overrides') WHERE name='promotion_reason'`).Scan(&hasReason); err != nil {
		return err
	}
	if !hasReason {
		if _, err := db.db.Exec(`ALTER TABLE prompt_overrides ADD COLUMN promotion_reason TEXT NOT NULL DEFAULT 'awaiting_comparable_exposures'`); err != nil {
			return err
		}
	}
	// Historical exposures have no recoverable action identity. Keep their
	// provenance intact, but exclude them from verified comparisons.
	if _, err := db.db.Exec(`DROP INDEX IF EXISTS idx_trace_exposure_operation`); err != nil {
		return err
	}
	if _, err := db.db.Exec(`DROP INDEX IF EXISTS idx_trace_exposure`); err != nil {
		return err
	}
	_, err := db.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_trace_exposure_action_operation ON tool_traces(exposure_id,action_identity,operation) WHERE exposure_id IS NOT NULL AND action_identity<>''`)
	return err
}

// VariantRevision participates in the request cache key, including promotions
// and removals. It contains metadata only, never source text.
func VariantRevision() string {
	if defaultDB == nil {
		return ""
	}
	var value string
	_ = defaultDB.db.QueryRow(`SELECT coalesce(group_concat(state, '|'),'') FROM (SELECT id || ':' || active || ':' || shadow AS state FROM prompt_overrides WHERE active=1 OR shadow=1 ORDER BY id)`).Scan(&value)
	return prompts.PromptRevision(value)
}

func selectToolGuideVariant(manual, canonical, runKey string) prompts.ToolGuideVariant {
	if defaultDB == nil {
		return prompts.ToolGuideVariant{Text: canonical, Version: "v1"}
	}
	digest := sha256.Sum256([]byte(runKey + "\x00" + manual + "\x00" + canonical))
	return defaultDB.selectToolGuideVariant(manual, canonical, float64(binary.BigEndian.Uint64(digest[:8])%10000)/10000)
}

func (o *OptimizerDB) selectToolGuideVariant(manual, canonical string, sample float64) prompts.ToolGuideVariant {
	baseline := prompts.ToolGuideVariant{Text: canonical, Version: "v1"}
	var id int64
	var body string
	hash := sha256.Sum256([]byte(o.loadCanonicalManual(manual)))
	sourceHash := hex.EncodeToString(hash[:])
	if sample < 0.3 {
		if err := o.db.QueryRow(`SELECT id, mutated_prompt FROM prompt_overrides WHERE tool_name=? AND original_hash=? AND shadow=1 AND active=0 ORDER BY id DESC LIMIT 1`, manual, sourceHash).Scan(&id, &body); err == nil {
			if valid, ok := prompts.SanitizeToolGuideOverride(body); ok {
				return prompts.ToolGuideVariant{Text: valid, Version: fmt.Sprintf("v2-shadow-%d", id)}
			}
		}
	}
	if err := o.db.QueryRow(`SELECT id, mutated_prompt FROM prompt_overrides WHERE tool_name=? AND original_hash=? AND active=1 ORDER BY id DESC LIMIT 1`, manual, sourceHash).Scan(&id, &body); err == nil {
		if valid, ok := prompts.SanitizeToolGuideOverride(body); ok {
			return prompts.ToolGuideVariant{Text: valid, Version: fmt.Sprintf("active-%d", id)}
		}
	}
	return baseline
}

// RecordPromptGuideExposures is called only after fitting the request. It records
// metadata, never prompt text. Traces refer to these immutable exposure IDs.
func RecordPromptGuideExposures(guides []prompts.ToolGuideExposure, revision string) map[string]string {
	if defaultDB == nil {
		return nil
	}
	return defaultDB.recordPromptGuideExposures(guides, revision)
}

func (o *OptimizerDB) recordPromptGuideExposures(guides []prompts.ToolGuideExposure, revision string) map[string]string {
	result := map[string]string{}
	for _, guide := range guides {
		if _, seen := result[guide.Manual]; seen {
			continue
		}
		row, err := o.db.Exec(`INSERT INTO prompt_exposures(manual,version,source_revision,prompt_revision) VALUES(?,?,?,?)`, guide.Manual, guide.Version, guide.SourceRevision, revision)
		if err != nil {
			continue
		}
		if id, err := row.LastInsertId(); err == nil {
			result[guide.Manual] = "exposure:" + strconv.FormatInt(id, 10)
		}
	}
	return result
}

func (o *OptimizerDB) logExposedToolTrace(tool string, success bool, recovery int, token, message string, duration int64, operation, action string) error {
	id, err := strconv.ParseInt(strings.TrimPrefix(token, "exposure:"), 10, 64)
	if err != nil {
		return err
	}
	_, err = o.db.Exec(`INSERT INTO tool_traces(tool_name,success,recovery_loops,prompt_version,error_message,execution_time_ms,exposure_id,operation,action_identity)
		SELECT manual, ?, ?, version, ?, ?, id, ?, ? FROM prompt_exposures WHERE id=? AND manual=?
 ON CONFLICT(exposure_id,action_identity,operation) WHERE exposure_id IS NOT NULL AND action_identity<>'' DO UPDATE SET success=MIN(tool_traces.success,excluded.success),
 recovery_loops=MAX(tool_traces.recovery_loops,excluded.recovery_loops),
 error_message=CASE WHEN excluded.success=0 THEN excluded.error_message ELSE tool_traces.error_message END`, success, recovery, message, duration, operation, action, id, prompts.ToolManualID(tool))
	return err
}
