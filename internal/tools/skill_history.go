package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func mustJSONString(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func (m *SkillManager) recordSkillAudit(skillID, skillName, action, actor, details string) {
	if strings.TrimSpace(actor) == "" {
		actor = "system"
	}
	if _, err := m.db.Exec(`INSERT INTO skill_audit_log (skill_id, skill_name, action, actor, details, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, skillID, skillName, action, actor, details, time.Now().UTC()); err != nil {
		m.logger.Warn("Failed to record skill audit log", "skill_id", skillID, "action", action, "error", err)
	}
}

func (m *SkillManager) appendSkillVersion(skillID, codeHash, code, createdBy, changeNote string) error {
	if strings.TrimSpace(createdBy) == "" {
		createdBy = "system"
	}
	var nextVersion int
	if err := m.db.QueryRow(`SELECT COALESCE(MAX(version_num), 0) + 1 FROM skill_versions WHERE skill_id = ?`, skillID).Scan(&nextVersion); err != nil {
		return fmt.Errorf("determining next skill version: %w", err)
	}
	_, err := m.db.Exec(`INSERT INTO skill_versions (skill_id, version_num, code_hash, code, created_at, created_by, change_note)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, skillID, nextVersion, codeHash, code, time.Now().UTC(), createdBy, changeNote)
	if err != nil {
		return fmt.Errorf("inserting skill version: %w", err)
	}
	return nil
}

func (m *SkillManager) EnsureInitialVersion(skillID, createdBy, changeNote string) error {
	var count int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM skill_versions WHERE skill_id = ?`, skillID).Scan(&count); err != nil {
		return fmt.Errorf("checking skill versions: %w", err)
	}
	if count > 0 {
		return nil
	}
	skill, err := m.GetSkill(skillID)
	if err != nil {
		return err
	}
	code, err := m.GetSkillCode(skillID)
	if err != nil {
		return err
	}
	return m.appendSkillVersion(skillID, skill.FileHash, code, createdBy, changeNote)
}

func (m *SkillManager) ListSkillVersions(skillID string) ([]SkillVersion, error) {
	rows, err := m.db.Query(`SELECT skill_id, version_num, code_hash, code, created_at, created_by, change_note
		FROM skill_versions WHERE skill_id = ? ORDER BY version_num DESC`, skillID)
	if err != nil {
		return nil, fmt.Errorf("querying skill versions: %w", err)
	}
	defer rows.Close()

	var versions []SkillVersion
	for rows.Next() {
		var v SkillVersion
		if err := rows.Scan(&v.SkillID, &v.Version, &v.CodeHash, &v.Code, &v.CreatedAt, &v.CreatedBy, &v.ChangeNote); err != nil {
			return nil, fmt.Errorf("scanning skill version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (m *SkillManager) GetSkillVersionCode(skillID string, version int) (string, error) {
	var code string
	err := m.db.QueryRow(`SELECT code FROM skill_versions WHERE skill_id = ? AND version_num = ?`, skillID, version).Scan(&code)
	if err != nil {
		return "", fmt.Errorf("querying skill version: %w", err)
	}
	return code, nil
}

func (m *SkillManager) ListSkillAudit(skillID string, limit int) ([]SkillAuditEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := m.db.Query(`SELECT id, skill_id, skill_name, action, actor, details, created_at
		FROM skill_audit_log WHERE skill_id = ? ORDER BY created_at DESC LIMIT ?`, skillID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying skill audit log: %w", err)
	}
	defer rows.Close()

	var entries []SkillAuditEntry
	for rows.Next() {
		var entry SkillAuditEntry
		if err := rows.Scan(&entry.ID, &entry.SkillID, &entry.SkillName, &entry.Action, &entry.Actor, &entry.Details, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning skill audit log: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (m *SkillManager) ExportSkillBundle(id string) (*SkillExportBundle, error) {
	skill, err := m.GetSkill(id)
	if err != nil {
		return nil, err
	}
	code, err := m.GetSkillCode(id)
	if err != nil {
		return nil, err
	}
	versions, err := m.ListSkillVersions(id)
	if err != nil {
		return nil, err
	}
	audit, err := m.ListSkillAudit(id, 100)
	if err != nil {
		return nil, err
	}
	manifest := SkillManifest{
		Name:          skill.Name,
		Description:   skill.Description,
		Executable:    skill.Executable,
		Category:      skill.Category,
		Tags:          skill.Tags,
		Parameters:    skill.Parameters,
		Returns:       "JSON object with 'status' and 'result' or 'message' fields.",
		Dependencies:  skill.Dependencies,
		VaultKeys:     skill.VaultKeys,
		CheatsheetIDs: skill.CheatsheetIDs,
	}
	docContent, _ := m.GetSkillDocumentation(id)
	if docContent != "" {
		manifest.Documentation = SkillDocumentationFilename(skill.Executable)
	}
	return &SkillExportBundle{
		Format:        "aurago-skill-bundle/v1",
		Exported:      time.Now().UTC(),
		Skill:         skill,
		Manifest:      manifest,
		Code:          code,
		Documentation: docContent,
		Versions:      versions,
		Audit:         audit,
	}, nil
}

func (m *SkillManager) replaceSkillVersions(skillID string, versions []SkillVersion) error {
	if _, err := m.db.Exec(`DELETE FROM skill_versions WHERE skill_id = ?`, skillID); err != nil {
		return fmt.Errorf("clearing skill versions: %w", err)
	}
	for _, version := range versions {
		if _, err := m.db.Exec(`INSERT INTO skill_versions (skill_id, version_num, code_hash, code, created_at, created_by, change_note)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			skillID, version.Version, version.CodeHash, version.Code, version.CreatedAt, version.CreatedBy, version.ChangeNote); err != nil {
			return fmt.Errorf("restoring skill version %d: %w", version.Version, err)
		}
	}
	return nil
}

func (m *SkillManager) replaceSkillAudit(skillID, skillName string, audit []SkillAuditEntry) error {
	if _, err := m.db.Exec(`DELETE FROM skill_audit_log WHERE skill_id = ?`, skillID); err != nil {
		return fmt.Errorf("clearing skill audit log: %w", err)
	}
	for _, entry := range audit {
		if _, err := m.db.Exec(`INSERT INTO skill_audit_log (skill_id, skill_name, action, actor, details, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			skillID, skillName, entry.Action, entry.Actor, entry.Details, entry.CreatedAt); err != nil {
			return fmt.Errorf("restoring skill audit log: %w", err)
		}
	}
	return nil
}

func (m *SkillManager) ImportSkillBundle(bundle *SkillExportBundle, createdBy string) (*SkillRegistryEntry, error) {
	if bundle == nil {
		return nil, fmt.Errorf("skill bundle is required")
	}
	if bundle.Format != "" && bundle.Format != "aurago-skill-bundle/v1" {
		return nil, fmt.Errorf("unsupported skill bundle format: %s", bundle.Format)
	}
	manifest := bundle.Manifest
	entry, err := m.CreateSkillEntry(manifest.Name, manifest.Description, bundle.Code, SkillTypeUser, createdBy, manifest.Category, manifest.Tags)
	if err != nil {
		return nil, err
	}
	if len(manifest.VaultKeys) > 0 {
		if err := m.UpdateVaultKeys(entry.ID, manifest.VaultKeys); err != nil {
			return nil, err
		}
	}
	if len(manifest.CheatsheetIDs) > 0 {
		if err := m.UpdateSkillCheatsheetIDs(entry.ID, manifest.CheatsheetIDs, createdBy); err != nil {
			return nil, err
		}
	}
	if bundle.Documentation != "" {
		if err := m.SetSkillDocumentation(entry.ID, bundle.Documentation, createdBy); err != nil {
			return nil, err
		}
	}
	if len(bundle.Versions) > 0 {
		if err := m.replaceSkillVersions(entry.ID, bundle.Versions); err != nil {
			return nil, err
		}
	}
	if len(bundle.Audit) > 0 {
		if err := m.replaceSkillAudit(entry.ID, entry.Name, bundle.Audit); err != nil {
			return nil, err
		}
	}
	m.recordSkillAudit(entry.ID, entry.Name, "imported", createdBy, "imported from skill bundle")
	return m.GetSkill(entry.ID)
}
