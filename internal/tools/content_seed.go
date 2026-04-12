package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// findManifestPath looks for a manifest under installDir and, if the binary lives
// inside a bin/ subdirectory, one level up as well.
func findManifestPath(installDir, subpath string) string {
	candidates := []string{
		filepath.Join(installDir, subpath),
		filepath.Join(filepath.Dir(installDir), subpath),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// copyFileIfNotExists copies src to dst only when dst does not already exist.
func copyFileIfNotExists(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil // already exists
	} else if !os.IsNotExist(err) {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// ── Mission seeding ─────────────────────────────────────────────────────────

type missionSeedEntry struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Prompt        string         `json:"prompt"`
	ExecutionType ExecutionType  `json:"execution_type"`
	Schedule      string         `json:"schedule"`
	TriggerType   TriggerType    `json:"trigger_type"`
	TriggerConfig *TriggerConfig `json:"trigger_config,omitempty"`
	Priority      string         `json:"priority"`
	Enabled       bool           `json:"enabled"`
	Locked        bool           `json:"locked"`
	CheatsheetIDs []string       `json:"cheatsheet_ids,omitempty"`
	AutoPrepare   bool           `json:"auto_prepare,omitempty"`
}

// SeedWelcomeMissions imports bundled example missions on first start.
// It is idempotent: missions whose ID already exists are skipped.
func SeedWelcomeMissions(m *MissionManagerV2, installDir string, logger *slog.Logger) {
	manifestPath := findManifestPath(installDir, filepath.Join("assets", "mission_samples", "metadata.json"))
	if manifestPath == "" {
		logger.Warn("SeedWelcomeMissions: manifest not found, skipping", "searched", filepath.Join(installDir, "assets", "mission_samples", "metadata.json"))
		return
	}

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		logger.Warn("SeedWelcomeMissions: failed to read manifest", "path", manifestPath, "error", err)
		return
	}

	var entries []missionSeedEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		logger.Warn("SeedWelcomeMissions: failed to parse manifest", "error", err)
		return
	}

	for _, e := range entries {
		if e.ID == "" {
			logger.Warn("SeedWelcomeMissions: skipping entry without id")
			continue
		}

		if _, exists := m.Get(e.ID); exists {
			logger.Debug("SeedWelcomeMissions: mission already exists, skipping", "id", e.ID)
			continue
		}

		mission := &MissionV2{
			ID:            e.ID,
			Name:          e.Name,
			Prompt:        e.Prompt,
			ExecutionType: e.ExecutionType,
			Schedule:      e.Schedule,
			TriggerType:   e.TriggerType,
			TriggerConfig: e.TriggerConfig,
			Priority:      e.Priority,
			Enabled:       e.Enabled,
			Locked:        e.Locked,
			CheatsheetIDs: e.CheatsheetIDs,
			AutoPrepare:   e.AutoPrepare,
		}

		if err := m.Create(mission); err != nil {
			logger.Warn("SeedWelcomeMissions: failed to create mission", "id", e.ID, "error", err)
		} else {
			logger.Info("SeedWelcomeMissions: seeded mission", "id", e.ID, "name", e.Name)
		}
	}
}

// ── Cheatsheet seeding ──────────────────────────────────────────────────────

type cheatsheetSeedEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Active    bool   `json:"active"`
	CreatedBy string `json:"created_by"`
}

// SeedWelcomeCheatsheets imports bundled example cheat sheets on first start.
// It is idempotent: cheat sheets whose ID already exists are skipped.
func SeedWelcomeCheatsheets(db *sql.DB, installDir string, logger *slog.Logger) {
	manifestPath := findManifestPath(installDir, filepath.Join("assets", "cheatsheet_samples", "metadata.json"))
	if manifestPath == "" {
		logger.Warn("SeedWelcomeCheatsheets: manifest not found, skipping", "searched", filepath.Join(installDir, "assets", "cheatsheet_samples", "metadata.json"))
		return
	}

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		logger.Warn("SeedWelcomeCheatsheets: failed to read manifest", "path", manifestPath, "error", err)
		return
	}

	var entries []cheatsheetSeedEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		logger.Warn("SeedWelcomeCheatsheets: failed to parse manifest", "error", err)
		return
	}

	for _, e := range entries {
		if e.ID == "" {
			logger.Warn("SeedWelcomeCheatsheets: skipping entry without id")
			continue
		}

		if _, err := CheatsheetGet(db, e.ID); err == nil {
			logger.Debug("SeedWelcomeCheatsheets: cheatsheet already exists, skipping", "id", e.ID)
			continue
		}

		createdBy := e.CreatedBy
		if createdBy == "" {
			createdBy = "system"
		}

		// We must inject the desired ID. CheatsheetCreate generates its own,
		// so we insert directly and then read back.
		if err := seedInsertCheatsheet(db, e.ID, e.Name, e.Content, createdBy, e.Active); err != nil {
			logger.Warn("SeedWelcomeCheatsheets: failed to insert cheatsheet", "id", e.ID, "error", err)
			continue
		}

		logger.Info("SeedWelcomeCheatsheets: seeded cheatsheet", "id", e.ID, "name", e.Name)
	}
}

func seedInsertCheatsheet(db *sql.DB, id, name, content, createdBy string, active bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len([]rune(content)) > MaxContentChars {
		return fmt.Errorf("content exceeds the %d character limit", MaxContentChars)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	activeInt := 0
	if active {
		activeInt = 1
	}

	_, err := db.Exec(
		"INSERT INTO cheatsheets (id, name, content, active, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, name, content, activeInt, createdBy, now, now,
	)
	return err
}

// ── Skill seeding ───────────────────────────────────────────────────────────

type skillSeedEntry struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Executable   string            `json:"executable"`
	Category     string            `json:"category,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Returns      string            `json:"returns,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	VaultKeys    []string          `json:"vault_keys,omitempty"`
}

// SeedWelcomeSkills copies bundled example skills into the skills directory.
// It is idempotent: files that already exist are not overwritten.
func SeedWelcomeSkills(skillMgr *SkillManager, skillsDir, installDir string, logger *slog.Logger) {
	manifestPath := findManifestPath(installDir, filepath.Join("assets", "skill_samples", "metadata.json"))
	if manifestPath == "" {
		logger.Warn("SeedWelcomeSkills: manifest not found, skipping", "searched", filepath.Join(installDir, "assets", "skill_samples", "metadata.json"))
		return
	}

	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		logger.Warn("SeedWelcomeSkills: failed to read manifest", "path", manifestPath, "error", err)
		return
	}

	var entries []skillSeedEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		logger.Warn("SeedWelcomeSkills: failed to parse manifest", "error", err)
		return
	}

	srcDir := filepath.Dir(manifestPath)
	copied := false

	for _, e := range entries {
		if e.Executable == "" {
			logger.Warn("SeedWelcomeSkills: skipping entry without executable")
			continue
		}

		// Copy the executable
		srcExec := filepath.Join(srcDir, e.Executable)
		dstExec := filepath.Join(skillsDir, e.Executable)
		if err := copyFileIfNotExists(srcExec, dstExec); err != nil {
			logger.Warn("SeedWelcomeSkills: failed to copy executable", "src", srcExec, "dst", dstExec, "error", err)
			continue
		}

		// Copy the manifest JSON (same base name as executable, .json extension)
		base := strings.TrimSuffix(e.Executable, filepath.Ext(e.Executable))
		jsonName := base + ".json"
		srcJSON := filepath.Join(srcDir, jsonName)
		dstJSON := filepath.Join(skillsDir, jsonName)

		// If the specific JSON file does not exist on disk, write it from the metadata entry
		if _, statErr := os.Stat(srcJSON); os.IsNotExist(statErr) {
			if _, dstStatErr := os.Stat(dstJSON); os.IsNotExist(dstStatErr) {
				manifestData, marshalErr := json.MarshalIndent(e, "", "  ")
				if marshalErr != nil {
					logger.Warn("SeedWelcomeSkills: failed to marshal manifest", "name", e.Name, "error", marshalErr)
					continue
				}
				if writeErr := os.WriteFile(dstJSON, manifestData, 0o640); writeErr != nil {
					logger.Warn("SeedWelcomeSkills: failed to write manifest", "dst", dstJSON, "error", writeErr)
					continue
				}
				copied = true
				logger.Info("SeedWelcomeSkills: seeded skill manifest", "name", e.Name)
			}
		} else {
			if err := copyFileIfNotExists(srcJSON, dstJSON); err != nil {
				logger.Warn("SeedWelcomeSkills: failed to copy manifest", "src", srcJSON, "dst", dstJSON, "error", err)
				continue
			}
			copied = true
			logger.Info("SeedWelcomeSkills: seeded skill", "name", e.Name)
		}
	}

	if copied {
		InvalidateSkillsCache(skillsDir)
		if err := skillMgr.SyncFromDisk(); err != nil {
			logger.Warn("SeedWelcomeSkills: SyncFromDisk failed after seeding", "error", err)
		} else {
			logger.Info("SeedWelcomeSkills: skill registry synced")
		}
	}
}
