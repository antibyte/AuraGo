package memory

import (
	"bufio"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func (s *SQLiteMemory) GetMessageCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	return count, err
}

// ReadCoreMemory returns all core memory entries formatted with IDs:
// "[1] fact one\n[2] fact two\n..."
// Returns an empty string when there are no entries.
func (s *SQLiteMemory) ReadCoreMemory() string {
	rows, err := s.db.Query("SELECT id, fact FROM core_memory ORDER BY id ASC")
	if err != nil {
		return ""
	}
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var id int64
		var fact string
		if err := rows.Scan(&id, &fact); err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n", id, fact))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// GetCoreMemoryCount returns the number of stored core memory entries.
func (s *SQLiteMemory) GetCoreMemoryCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM core_memory").Scan(&count)
	return count, err
}

// CoreMemoryFact is a single core memory entry.
type CoreMemoryFact struct {
	ID   int64  `json:"id"`
	Fact string `json:"fact"`
}

// GetCoreMemoryFacts returns all core memory entries as a slice of CoreMemoryFact.
func (s *SQLiteMemory) GetCoreMemoryFacts() ([]CoreMemoryFact, error) {
	rows, err := s.db.Query("SELECT id, fact FROM core_memory ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []CoreMemoryFact
	for rows.Next() {
		var f CoreMemoryFact
		if err := rows.Scan(&f.ID, &f.Fact); err != nil {
			continue
		}
		facts = append(facts, f)
	}
	if facts == nil {
		facts = []CoreMemoryFact{}
	}
	return facts, nil
}

// maxCoreMemoryFactLen is the maximum byte length of a single core memory fact.
const maxCoreMemoryFactLen = 10_000

// AddCoreMemoryFact inserts a new fact and returns its assigned ID.
func (s *SQLiteMemory) AddCoreMemoryFact(fact string) (int64, error) {
	if len(fact) > maxCoreMemoryFactLen {
		fact = fact[:maxCoreMemoryFactLen]
	}
	res, err := s.db.Exec("INSERT INTO core_memory (fact) VALUES (?)", fact)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateCoreMemoryFact overwrites an existing entry's text by ID.
func (s *SQLiteMemory) UpdateCoreMemoryFact(id int64, fact string) error {
	res, err := s.db.Exec(
		"UPDATE core_memory SET fact = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		fact, id,
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("no entry with id %d", id)
	}
	return nil
}

// DeleteCoreMemoryFact removes an entry by ID.
func (s *SQLiteMemory) DeleteCoreMemoryFact(id int64) error {
	res, err := s.db.Exec("DELETE FROM core_memory WHERE id = ?", id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("no entry with id %d", id)
	}
	return nil
}

// FindCoreMemoryIDByFact returns the ID of the first entry whose fact text
// matches exactly (used for backwards-compatible text-based deletion).
func (s *SQLiteMemory) FindCoreMemoryIDByFact(fact string) (int64, error) {
	var id int64
	err := s.db.QueryRow("SELECT id FROM core_memory WHERE fact = ? LIMIT 1", fact).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("fact not found")
	}
	return id, err
}

// CoreMemoryFactExists reports whether the given text is already stored.
func (s *SQLiteMemory) CoreMemoryFactExists(fact string) bool {
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM core_memory WHERE fact = ?", fact).Scan(&count)
	return count > 0
}

// ── User Profile (Profiling Engine) ──────────────────────────────────────────

// ProfileEntry represents a single user profile attribute.
type ProfileEntry struct {
	Category   string
	Key        string
	Value      string
	Confidence int
	Source     string
	UpdatedAt  string
	FirstSeen  string
}

// UpsertProfileEntry inserts or updates a user profile attribute.
// If the same category+key already exists with the same value, confidence is incremented.
// If the value differs, it's overwritten and confidence resets to 1.
func (s *SQLiteMemory) UpsertProfileEntry(category, key, value, source string) error {
	// Enforce length limits
	if len(key) > 50 {
		key = key[:50]
	}
	if len(value) > 200 {
		value = value[:200]
	}
	if len(category) > 20 {
		category = category[:20]
	}

	// Check if entry exists with same value → increment confidence
	var existing string
	var conf int
	err := s.db.QueryRow("SELECT value, confidence FROM user_profile WHERE category = ? AND key = ?", category, key).Scan(&existing, &conf)
	if err == nil {
		if strings.EqualFold(existing, value) {
			// Same value → increment confidence
			_, err = s.db.Exec("UPDATE user_profile SET confidence = confidence + 1, updated_at = CURRENT_TIMESTAMP WHERE category = ? AND key = ?", category, key)
			return err
		}
		// Different value → overwrite + reset confidence
		_, err = s.db.Exec("UPDATE user_profile SET value = ?, confidence = 1, source = ?, updated_at = CURRENT_TIMESTAMP WHERE category = ? AND key = ?", value, source, category, key)
		return err
	}

	// New entry
	_, err = s.db.Exec("INSERT INTO user_profile (category, key, value, confidence, source) VALUES (?, ?, ?, 1, ?)", category, key, value, source)
	return err
}

// GetUserProfileSummary returns a compact, token-efficient summary of the user profile.
// Only entries with confidence >= minConfidence are included.
func (s *SQLiteMemory) GetUserProfileSummary(minConfidence int) string {
	query := `SELECT category, key, value, confidence FROM user_profile WHERE confidence >= ? ORDER BY category, confidence DESC`
	rows, err := s.db.Query(query, minConfidence)
	if err != nil {
		return ""
	}
	defer rows.Close()

	// Group by category
	categoryData := make(map[string][]string)
	categoryOrder := []string{} // preserve insertion order
	for rows.Next() {
		var cat, k, v string
		var conf int
		if err := rows.Scan(&cat, &k, &v, &conf); err != nil {
			continue
		}
		if _, exists := categoryData[cat]; !exists {
			categoryOrder = append(categoryOrder, cat)
		}
		categoryData[cat] = append(categoryData[cat], fmt.Sprintf("%s: %s", k, v))
	}

	if len(categoryData) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### User Profile\n")
	categoryLabels := map[string]string{
		"tech":      "Tech",
		"prefs":     "Prefs",
		"interests": "Interests",
		"context":   "Context",
		"comm":      "Comm",
	}
	for _, cat := range categoryOrder {
		label := categoryLabels[cat]
		if label == "" {
			label = cat
		}
		entries := categoryData[cat]
		if len(entries) > 5 {
			entries = entries[:5] // Cap per category
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", label, strings.Join(entries, "; ")))
	}
	return sb.String()
}

// GetProfileEntries returns all entries for a given category (or all if category is empty).
func (s *SQLiteMemory) GetProfileEntries(category string) ([]ProfileEntry, error) {
	var rows *sql.Rows
	var err error
	if category == "" {
		rows, err = s.db.Query("SELECT category, key, value, confidence, source, updated_at, COALESCE(first_seen, updated_at) FROM user_profile ORDER BY category, confidence DESC")
	} else {
		rows, err = s.db.Query("SELECT category, key, value, confidence, source, updated_at, COALESCE(first_seen, updated_at) FROM user_profile WHERE category = ? ORDER BY confidence DESC", category)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ProfileEntry
	for rows.Next() {
		var e ProfileEntry
		if err := rows.Scan(&e.Category, &e.Key, &e.Value, &e.Confidence, &e.Source, &e.UpdatedAt, &e.FirstSeen); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return entries, nil
}

// DeleteProfileEntry removes a single user profile entry by category and key.
func (s *SQLiteMemory) DeleteProfileEntry(category, key string) error {
	res, err := s.db.Exec("DELETE FROM user_profile WHERE category = ? AND key = ?", category, key)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("profile entry not found: %s/%s", category, key)
	}
	return nil
}

// ResetUserProfile deletes all automatically collected profile data.
func (s *SQLiteMemory) ResetUserProfile() error {
	_, err := s.db.Exec("DELETE FROM user_profile")
	return err
}

// CleanupStaleProfileEntries removes profile entries with confidence=1 that
// haven't been updated in the given number of days.
func (s *SQLiteMemory) CleanupStaleProfileEntries(olderThanDays int) (int64, error) {
	res, err := s.db.Exec("DELETE FROM user_profile WHERE confidence <= 1 AND updated_at < datetime('now', '-' || ? || ' days')", olderThanDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PruneStaleProfileEntries applies time-based confidence rules:
//   - confidence = 1 and not updated for >= 24 h → DELETE (unconfirmed noise)
//   - confidence = 2 and not updated for >= 48 h → downgrade to 1, reset timer
//
// Call this after every batch of profile upserts to keep the profile lean.
func (s *SQLiteMemory) PruneStaleProfileEntries() (deleted int64, downgraded int64, err error) {
	res, err := s.db.Exec(`DELETE FROM user_profile WHERE confidence = 1 AND updated_at < datetime('now', '-24 hours')`)
	if err != nil {
		return 0, 0, fmt.Errorf("profile prune delete: %w", err)
	}
	deleted, _ = res.RowsAffected()

	res, err = s.db.Exec(`UPDATE user_profile
		SET confidence = 1, updated_at = CURRENT_TIMESTAMP
		WHERE confidence = 2 AND updated_at < datetime('now', '-48 hours')`)
	if err != nil {
		return deleted, 0, fmt.Errorf("profile prune downgrade: %w", err)
	}
	downgraded, _ = res.RowsAffected()
	return deleted, downgraded, nil
}

// EnforceProfileSizeLimit keeps the user_profile table at most maxEntries rows
// by deleting the lowest-confidence, oldest entries.
func (s *SQLiteMemory) EnforceProfileSizeLimit(maxEntries int) error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM user_profile").Scan(&count); err != nil {
		return err
	}
	if count <= maxEntries {
		return nil
	}
	excess := count - maxEntries
	_, err := s.db.Exec(`DELETE FROM user_profile WHERE rowid IN (
		SELECT rowid FROM user_profile ORDER BY confidence ASC, updated_at ASC LIMIT ?
	)`, excess)
	return err
}

// MigrateCoreMemoryFromMarkdown reads the legacy core_memory.md file,
// imports its bullet-point lines into SQLite, renames the file to
// core_memory.md.migrated, and returns whether the system is on its
// first start (no prior facts existed).
func (s *SQLiteMemory) MigrateCoreMemoryFromMarkdown(dataDir string, logger *slog.Logger) (isFirstStart bool) {
	mdPath := filepath.Join(dataDir, "core_memory.md")
	migratedPath := mdPath + ".migrated"

	// .migrated sentinel exists → first-start was already completed at some point.
	// Even if the DB was subsequently wiped (e.g. corruption recovery), we must NOT
	// trigger the naming prompt again — the agent already has an identity.
	if _, err := os.Stat(migratedPath); err == nil {
		return false
	}

	count, _ := s.GetCoreMemoryCount()

	data, err := os.ReadFile(mdPath)
	if err != nil {
		// No .md file. If the table is also empty, this is a genuine first start.
		if count == 0 {
			// Write the sentinel now so that a future DB wipe or corruption recovery
			// does not re-trigger the naming prompt. The prompt fires exactly once.
			if werr := os.WriteFile(migratedPath, []byte(""), 0644); werr != nil {
				logger.Warn("[Memory] Could not create first-start sentinel", "path", migratedPath, "error", werr)
			}
			return true
		}
		return false
	}

	var facts []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "- ") {
			fact := strings.TrimPrefix(line, "- ")
			if fact != "" {
				facts = append(facts, fact)
			}
		}
	}

	if len(facts) > 0 && count == 0 {
		for _, f := range facts {
			if _, err := s.AddCoreMemoryFact(f); err != nil {
				logger.Error("Core memory migration: failed to insert fact", "fact", f, "error", err)
			}
		}
		logger.Info("Core memory migrated from markdown", "facts_imported", len(facts))
	}

	// Rename the .md file so migration only runs once.
	if err := os.Rename(mdPath, migratedPath); err != nil {
		logger.Warn("Could not rename core_memory.md after migration", "error", err)
	}

	// isFirstStart: no prior facts in either source.
	return len(facts) == 0 && count == 0
}
