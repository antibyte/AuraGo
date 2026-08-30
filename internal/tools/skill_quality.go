package tools

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SkillOrigin is the proven provenance of a managed skill. Only OriginAgent is
// eligible for automatic quality mutations.
type SkillOrigin string

const (
	OriginAgent                   SkillOrigin = "agent"
	OriginUser                    SkillOrigin = "user"
	OriginSystem                  SkillOrigin = "system"
	OriginLegacyUnknown           SkillOrigin = "legacy_unknown"
	MinimumSkillImproveConfidence             = 0.95
	MinimumSkillDeleteConfidence              = 0.98
)

// SkillUsageStats exposes per-skill execution outcomes without making lack of
// usage a quality verdict.
type SkillUsageStats struct {
	Attempts   int64      `json:"attempts"`
	Successes  int64      `json:"successes"`
	Failures   int64      `json:"failures"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// SkillQualityAction is a durable, source-code-free maintenance record. Delete
// actions are tombstones and intentionally survive registry/file deletion.
type SkillQualityAction struct {
	ID          int64       `json:"id"`
	SkillKind   string      `json:"skill_kind"`
	SkillID     string      `json:"skill_id"`
	SkillName   string      `json:"skill_name"`
	ContentHash string      `json:"content_hash"`
	Origin      SkillOrigin `json:"origin"`
	Verdict     string      `json:"verdict"`
	Confidence  float64     `json:"confidence"`
	Decision    string      `json:"decision"`
	Reason      string      `json:"reason"`
	Details     string      `json:"details,omitempty"`
	Actor       string      `json:"actor"`
	CreatedAt   time.Time   `json:"created_at"`
}

func originForCreation(skillType SkillType, actor string) SkillOrigin {
	if skillType == SkillTypeBuiltIn {
		return OriginSystem
	}
	switch strings.ToLower(strings.TrimSpace(actor)) {
	case "agent":
		return OriginAgent
	case "user":
		return OriginUser
	case "system":
		return OriginSystem
	case "system:sync":
		return OriginLegacyUnknown
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(actor)), "agent:") {
		return OriginAgent
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(actor)), "system:") {
		return OriginSystem
	}
	return OriginLegacyUnknown
}

func migrateSkillQualityColumns(db *sql.DB, table string) error {
	if db == nil {
		return fmt.Errorf("skills database is not initialized")
	}
	allowed := map[string]bool{"skills_registry": true, "agent_skills_registry": true}
	if !allowed[table] {
		return fmt.Errorf("unsupported skill registry table %q", table)
	}
	columns := []string{
		"origin TEXT NOT NULL DEFAULT 'legacy_unknown'",
		"usage_count INTEGER NOT NULL DEFAULT 0",
		"success_count INTEGER NOT NULL DEFAULT 0",
		"failure_count INTEGER NOT NULL DEFAULT 0",
		"last_used_at DATETIME",
		"last_quality_review_at DATETIME",
		"last_quality_verdict TEXT NOT NULL DEFAULT ''",
		"last_quality_confidence REAL NOT NULL DEFAULT 0",
		"last_quality_hash TEXT NOT NULL DEFAULT ''",
	}
	for _, column := range columns {
		if _, err := db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("migrate %s quality metadata: %w", table, err)
		}
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS skill_quality_maintenance_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			skill_kind TEXT NOT NULL,
			skill_id TEXT NOT NULL,
			skill_name TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			origin TEXT NOT NULL,
			verdict TEXT NOT NULL,
			confidence REAL NOT NULL,
			decision TEXT NOT NULL,
			reason TEXT NOT NULL,
			details TEXT NOT NULL DEFAULT '',
			actor TEXT NOT NULL DEFAULT 'maintenance',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_skill_quality_name ON skill_quality_maintenance_log(skill_kind, skill_name, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_skill_quality_decision ON skill_quality_maintenance_log(decision, created_at DESC);
	`); err != nil {
		return fmt.Errorf("create skill quality maintenance log: %w", err)
	}
	return nil
}

func backfillPythonSkillOrigins(db *sql.DB) error {
	statements := []string{
		`UPDATE skills_registry SET origin = 'system' WHERE origin = 'legacy_unknown' AND type = 'builtin'`,
		`UPDATE skills_registry SET origin = 'user' WHERE origin = 'legacy_unknown' AND (type = 'user' OR lower(created_by) = 'user')`,
		`UPDATE skills_registry SET origin = 'system' WHERE origin = 'legacy_unknown' AND lower(created_by) LIKE 'system%'`,
		`UPDATE skills_registry SET origin = 'agent' WHERE origin = 'legacy_unknown' AND lower(created_by) = 'agent'
		 AND (EXISTS (SELECT 1 FROM skill_versions v WHERE v.skill_id = skills_registry.id AND lower(v.created_by) = 'agent')
		 OR EXISTS (SELECT 1 FROM skill_audit_log a WHERE a.skill_id = skills_registry.id AND lower(a.actor) = 'agent'))`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("backfill Python skill origins: %w", err)
		}
	}
	return nil
}

func backfillAgentSkillOrigins(db *sql.DB) error {
	statements := []string{
		`UPDATE agent_skills_registry SET origin = 'user' WHERE origin = 'legacy_unknown' AND lower(created_by) = 'user'`,
		`UPDATE agent_skills_registry SET origin = 'system' WHERE origin = 'legacy_unknown' AND lower(created_by) LIKE 'system%' AND lower(created_by) != 'system:sync'`,
		`UPDATE agent_skills_registry SET origin = 'agent' WHERE origin = 'legacy_unknown' AND lower(created_by) = 'agent'
		 AND EXISTS (SELECT 1 FROM agent_skill_audit_log a WHERE a.skill_id = agent_skills_registry.id AND lower(a.actor) = 'agent')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("backfill Agent Skill origins: %w", err)
		}
	}
	return nil
}

func scanQualityMetadata(origin *SkillOrigin, usage *SkillUsageStats, lastReview **time.Time, rawOrigin string, attempts, successes, failures int64, lastUsed, reviewed sql.NullTime) {
	*origin = SkillOrigin(rawOrigin)
	usage.Attempts = attempts
	usage.Successes = successes
	usage.Failures = failures
	if lastUsed.Valid {
		t := lastUsed.Time
		usage.LastUsedAt = &t
	}
	if reviewed.Valid {
		t := reviewed.Time
		*lastReview = &t
	}
}

func recordSkillUsage(db *sql.DB, table, keyColumn, key string, success bool) error {
	if db == nil {
		return fmt.Errorf("skills database is not initialized")
	}
	if (table != "skills_registry" && table != "agent_skills_registry") || (keyColumn != "name" && keyColumn != "id") {
		return fmt.Errorf("invalid skill usage target")
	}
	successDelta, failureDelta := 0, 0
	if success {
		successDelta = 1
	} else {
		failureDelta = 1
	}
	result, err := db.Exec("UPDATE "+table+" SET usage_count = usage_count + 1, success_count = success_count + ?, failure_count = failure_count + ?, last_used_at = CURRENT_TIMESTAMP WHERE "+keyColumn+" = ?", successDelta, failureDelta, key)
	if err != nil {
		return fmt.Errorf("record skill usage: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return fmt.Errorf("skill not found: %s", key)
	}
	return nil
}

// RecordSkillUsage records one successful or failed Python-skill invocation.
func (m *SkillManager) RecordSkillUsage(name string, success bool) error {
	return recordSkillUsage(m.db, "skills_registry", "name", strings.TrimSpace(strings.TrimSuffix(name, ".py")), success)
}

// AcquireSkillExecutionLease prevents maintenance from swapping or deleting a
// Python skill while a foreground invocation is reading/executing it.
func (m *SkillManager) AcquireSkillExecutionLease() func() {
	if m == nil {
		return func() {}
	}
	m.qualityMutationMu.RLock()
	return m.qualityMutationMu.RUnlock
}

// RecordAgentSkillUsage records one activation or script execution outcome.
func (m *AgentSkillManager) RecordAgentSkillUsage(id string, success bool) error {
	return recordSkillUsage(m.db, "agent_skills_registry", "id", id, success)
}

// RecordAgentSkillUsageByName records an attempted activation/script call when
// dispatch policy rejects it before the registry ID is loaded.
func (m *AgentSkillManager) RecordAgentSkillUsageByName(name string, success bool) error {
	return recordSkillUsage(m.db, "agent_skills_registry", "name", strings.TrimSpace(name), success)
}
