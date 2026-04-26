package tools

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// MaxSkillDocumentationBytes is the upper bound for a skill's agent manual.
// 64 KB keeps the file readable in any editor and bounds context usage when
// the agent fetches the manual via get_skill_documentation.
const MaxSkillDocumentationBytes = 64 * 1024

// SkillDocumentationFilename returns the conventional manual filename for a
// skill, derived from the executable so it stays in lock-step with the
// manifest/code files (<basename>.md).
func SkillDocumentationFilename(executable string) string {
	base := strings.TrimSuffix(executable, filepath.Ext(executable))
	if base == "" {
		return ""
	}
	return base + ".md"
}

// validateSkillDocumentation enforces the size and encoding constraints used
// for any documentation write path (UI upload, REST PUT, agent tool, import).
func validateSkillDocumentation(content string) error {
	if len(content) > MaxSkillDocumentationBytes {
		return fmt.Errorf("documentation exceeds %d byte limit", MaxSkillDocumentationBytes)
	}
	if !utf8.ValidString(content) {
		return fmt.Errorf("documentation must be valid UTF-8")
	}
	return nil
}

// hashDocumentation returns the hex-encoded SHA-256 of the raw markdown bytes.
func hashDocumentation(content string) string {
	if content == "" {
		return ""
	}
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// GetSkillDocumentation reads the on-disk manual for a skill. Returns an empty
// string with no error if the skill has no manual yet.
func (m *SkillManager) GetSkillDocumentation(id string) (string, error) {
	skill, err := m.GetSkill(id)
	if err != nil {
		return "", err
	}
	docPath := SkillDocumentationFilename(skill.Executable)
	if docPath == "" {
		return "", nil
	}
	full := filepath.Join(m.skillsDir, docPath)
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading skill documentation: %w", err)
	}
	return string(data), nil
}

// SetSkillDocumentation writes (or replaces) the on-disk manual for a skill,
// updates the registry's path/hash columns, and records an audit entry. An
// empty content removes the manual.
func (m *SkillManager) SetSkillDocumentation(id, content, updatedBy string) error {
	skill, err := m.GetSkill(id)
	if err != nil {
		return err
	}
	if skill.Type == SkillTypeBuiltIn {
		return fmt.Errorf("built-in skills cannot have a custom manual")
	}
	if strings.TrimSpace(content) == "" {
		return m.DeleteSkillDocumentation(id, updatedBy)
	}
	if err := validateSkillDocumentation(content); err != nil {
		return err
	}
	docName := SkillDocumentationFilename(skill.Executable)
	if docName == "" {
		return fmt.Errorf("cannot derive documentation filename for skill %s", skill.Name)
	}
	if err := os.MkdirAll(m.skillsDir, 0o750); err != nil {
		return fmt.Errorf("creating skills directory: %w", err)
	}
	docPath := filepath.Join(m.skillsDir, docName)
	if err := os.WriteFile(docPath, []byte(content), 0o640); err != nil {
		return fmt.Errorf("writing skill documentation: %w", err)
	}

	hash := hashDocumentation(content)
	action := "documentation_updated"
	if skill.DocumentationPath == "" {
		action = "documentation_added"
	}
	if _, err := m.db.Exec(
		`UPDATE skills_registry SET documentation_path = ?, documentation_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		docName, hash, id,
	); err != nil {
		return fmt.Errorf("updating documentation metadata: %w", err)
	}
	m.recordSkillAudit(id, skill.Name, action, updatedBy, fmt.Sprintf("hash=%s size=%d", hash, len(content)))
	m.logger.Info("Skill documentation saved", "id", id, "name", skill.Name, "size", len(content))
	return nil
}

// DeleteSkillDocumentation removes the on-disk manual and clears the registry
// columns. Idempotent: if no manual exists, this is a no-op.
func (m *SkillManager) DeleteSkillDocumentation(id, updatedBy string) error {
	skill, err := m.GetSkill(id)
	if err != nil {
		return err
	}
	docName := SkillDocumentationFilename(skill.Executable)
	hadDoc := skill.DocumentationPath != ""
	if docName != "" {
		fullPath := filepath.Join(m.skillsDir, docName)
		if rmErr := os.Remove(fullPath); rmErr != nil && !os.IsNotExist(rmErr) {
			m.logger.Warn("Failed to remove skill documentation file", "id", id, "path", fullPath, "error", rmErr)
		}
	}
	if _, err := m.db.Exec(
		`UPDATE skills_registry SET documentation_path = '', documentation_hash = '', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	); err != nil {
		return fmt.Errorf("clearing documentation metadata: %w", err)
	}
	if hadDoc {
		m.recordSkillAudit(id, skill.Name, "documentation_removed", updatedBy, "")
	}
	return nil
}

// UpdateSkillCheatsheetIDs replaces the linked cheatsheet IDs for a skill.
// Updates both the DB column and the on-disk manifest so the file stays
// authoritative for portability/exports.
func (m *SkillManager) UpdateSkillCheatsheetIDs(id string, ids []string, updatedBy string) error {
	skill, err := m.GetSkill(id)
	if err != nil {
		return err
	}
	clean := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, raw := range ids {
		v := strings.TrimSpace(raw)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		clean = append(clean, v)
	}
	idsJSON, err := json.Marshal(clean)
	if err != nil {
		return fmt.Errorf("serializing cheatsheet ids: %w", err)
	}
	if _, err := m.db.Exec(
		`UPDATE skills_registry SET cheatsheet_ids = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		string(idsJSON), id,
	); err != nil {
		return fmt.Errorf("updating cheatsheet ids: %w", err)
	}
	manifestPath := filepath.Join(m.skillsDir, strings.TrimSuffix(skill.Executable, filepath.Ext(skill.Executable))+".json")
	if raw, readErr := os.ReadFile(manifestPath); readErr == nil {
		var manifest map[string]json.RawMessage
		if jsonErr := json.Unmarshal(raw, &manifest); jsonErr == nil {
			manifest["cheatsheet_ids"] = idsJSON
			if updated, marshalErr := json.MarshalIndent(manifest, "", "  "); marshalErr == nil {
				_ = os.WriteFile(manifestPath, updated, 0o600)
			}
		}
	}
	m.recordSkillAudit(id, skill.Name, "cheatsheet_links_updated", updatedBy, fmt.Sprintf("count=%d", len(clean)))
	InvalidateSkillsCache(m.skillsDir)
	return nil
}

// syncDocumentationForSkill refreshes the documentation_path/documentation_hash
// columns based on whether <basename>.md exists on disk. Called by
// SyncFromDisk so manual files added/removed outside the UI stay tracked.
func (m *SkillManager) syncDocumentationForSkill(skillID, executable string) {
	docName := SkillDocumentationFilename(executable)
	if docName == "" {
		return
	}
	full := filepath.Join(m.skillsDir, docName)
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			m.db.Exec(`UPDATE skills_registry SET documentation_path = '', documentation_hash = '' WHERE id = ?`, skillID)
		}
		return
	}
	if validateSkillDocumentation(string(data)) != nil {
		return
	}
	hash := hashDocumentation(string(data))
	m.db.Exec(`UPDATE skills_registry SET documentation_path = ?, documentation_hash = ? WHERE id = ?`, docName, hash, skillID)
}

// loadCheatsheetIDs deserializes the cheatsheet_ids column for a skill row.
func loadCheatsheetIDs(raw sql.NullString) []string {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw.String), &ids); err != nil {
		return nil
	}
	return ids
}
