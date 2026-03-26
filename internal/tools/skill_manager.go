package tools

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SkillType distinguishes between agent-created, user-uploaded, and built-in skills.
type SkillType string

const (
	SkillTypeAgent   SkillType = "agent"
	SkillTypeUser    SkillType = "user"
	SkillTypeBuiltIn SkillType = "builtin"
)

// SecurityStatus represents the security scan result of a skill.
type SecurityStatus string

const (
	SecurityPending   SecurityStatus = "pending"
	SecurityClean     SecurityStatus = "clean"
	SecurityWarning   SecurityStatus = "warning"
	SecurityDangerous SecurityStatus = "dangerous"
	SecurityError     SecurityStatus = "error"
)

// SkillRegistryEntry extends SkillManifest with metadata for the Skill Manager.
type SkillRegistryEntry struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Executable     string            `json:"executable"`
	Parameters     map[string]string `json:"parameters,omitempty"`
	Dependencies   []string          `json:"dependencies,omitempty"`
	VaultKeys      []string          `json:"vault_keys,omitempty"`
	Type           SkillType         `json:"type"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	CreatedBy      string            `json:"created_by"` // "agent", "user", "system"
	Enabled        bool              `json:"enabled"`
	SecurityStatus SecurityStatus    `json:"security_status"`
	SecurityReport *SecurityReport   `json:"security_report,omitempty"`
	LastScanAt     *time.Time        `json:"last_scan_at,omitempty"`
	FilePath       string            `json:"file_path"`
	FileHash       string            `json:"file_hash"`
}

// SkillManager manages the skill registry and lifecycle.
type SkillManager struct {
	db        *sql.DB
	skillsDir string
	logger    *slog.Logger
}

// InitSkillsDB opens (or creates) the skills registry SQLite database and runs migrations.
func InitSkillsDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open skills database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS skills_registry (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT CHECK(type IN ('agent', 'user', 'builtin')) DEFAULT 'agent',
		description TEXT,
		executable TEXT,
		parameters TEXT,
		dependencies TEXT,
		vault_keys TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_by TEXT DEFAULT 'agent',
		enabled INTEGER DEFAULT 1,
		security_status TEXT DEFAULT 'pending',
		security_report TEXT,
		last_scan_at DATETIME,
		file_path TEXT,
		file_hash TEXT
	);

	CREATE TABLE IF NOT EXISTS skills_scan_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		skill_id TEXT REFERENCES skills_registry(id) ON DELETE CASCADE,
		scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		scanner_type TEXT,
		score REAL,
		verdict TEXT,
		details TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_skills_type ON skills_registry(type);
	CREATE INDEX IF NOT EXISTS idx_skills_status ON skills_registry(security_status);
	CREATE INDEX IF NOT EXISTS idx_skills_enabled ON skills_registry(enabled);
	`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create skills schema: %w", err)
	}

	return db, nil
}

// NewSkillManager creates a new SkillManager instance.
func NewSkillManager(db *sql.DB, skillsDir string, logger *slog.Logger) *SkillManager {
	return &SkillManager{
		db:        db,
		skillsDir: skillsDir,
		logger:    logger,
	}
}

// SyncFromDisk scans the skills directory and reconciles with the database.
// New skills found on disk are inserted; skills removed from disk are marked as disabled.
func (m *SkillManager) SyncFromDisk() error {
	manifests, err := ListSkills(m.skillsDir)
	if err != nil {
		return fmt.Errorf("listing skills from disk: %w", err)
	}

	// Build set of skill names found on disk
	diskNames := make(map[string]struct{})
	for _, manifest := range manifests {
		diskNames[manifest.Name] = struct{}{}

		// Compute file hash if executable exists
		var fileHash string
		execPath := filepath.Join(m.skillsDir, manifest.Executable)
		if data, err := os.ReadFile(execPath); err == nil {
			h := sha256.Sum256(data)
			fileHash = hex.EncodeToString(h[:])
		}

		// Check if skill already exists in DB
		var existingID string
		err := m.db.QueryRow("SELECT id FROM skills_registry WHERE name = ?", manifest.Name).Scan(&existingID)
		if err == sql.ErrNoRows {
			// Insert new skill
			id := fmt.Sprintf("%s_%d", manifest.Name, time.Now().UnixMilli())
			skillType := detectSkillType(manifest.Name, m.skillsDir)

			deps, _ := json.Marshal(manifest.Dependencies)
			params, _ := json.Marshal(manifest.Parameters)
			vaultKeys, _ := json.Marshal(manifest.VaultKeys)

			_, err := m.db.Exec(`INSERT INTO skills_registry 
				(id, name, type, description, executable, parameters, dependencies, vault_keys, 
				 created_by, enabled, security_status, file_path, file_hash)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?)`,
				id, manifest.Name, string(skillType), manifest.Description,
				manifest.Executable, string(params), string(deps), string(vaultKeys),
				string(skillType), string(SecurityClean), manifest.Executable, fileHash,
			)
			if err != nil {
				m.logger.Warn("Failed to insert skill", "name", manifest.Name, "error", err)
			}
		} else if err == nil {
			// Update type for __builtin__ executables (may have been stored as "agent" previously)
			if manifest.Executable == "__builtin__" {
				m.db.Exec("UPDATE skills_registry SET type = ?, file_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
					string(SkillTypeBuiltIn), fileHash, existingID)
			} else {
				// Update hash if changed
				m.db.Exec("UPDATE skills_registry SET file_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
					fileHash, existingID)
			}
		}
	}

	return nil
}

// detectSkillType guesses the type based on naming conventions.
func detectSkillType(name string, skillsDir string) SkillType {
	jsonPath := filepath.Join(skillsDir, name+".json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return SkillTypeAgent
	}

	var manifest SkillManifest
	if json.Unmarshal(data, &manifest) != nil {
		return SkillTypeAgent
	}

	// Skills that delegate to a Go built-in tool are internal and should not
	// appear as user-visible skills in the Fähigkeiten page.
	if manifest.Executable == "__builtin__" {
		return SkillTypeBuiltIn
	}

	// Hardcoded built-in names shipped with the project
	builtins := map[string]bool{
		"pdf_extractor": true, "scan": true, "screenshot": true,
	}
	if builtins[name] {
		return SkillTypeBuiltIn
	}

	return SkillTypeAgent
}

// ListSkillsFiltered returns skills from the registry with optional filters.
func (m *SkillManager) ListSkillsFiltered(skillType, status, search string, enabledFilter *bool) ([]SkillRegistryEntry, error) {
	query := "SELECT id, name, type, description, executable, parameters, dependencies, vault_keys, " +
		"created_at, updated_at, created_by, enabled, security_status, security_report, last_scan_at, " +
		"file_path, file_hash FROM skills_registry WHERE 1=1"
	var args []interface{}

	if skillType != "" {
		query += " AND type = ?"
		args = append(args, skillType)
	} else {
		// By default, exclude built-in skills from listings
		query += " AND type != ?"
		args = append(args, SkillTypeBuiltIn)
	}
	if status != "" {
		query += " AND security_status = ?"
		args = append(args, status)
	}
	if enabledFilter != nil {
		query += " AND enabled = ?"
		if *enabledFilter {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if search != "" {
		query += " AND (name LIKE ? OR description LIKE ?)"
		searchParam := "%" + search + "%"
		args = append(args, searchParam, searchParam)
	}
	query += " ORDER BY name ASC"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying skills: %w", err)
	}
	defer rows.Close()

	var skills []SkillRegistryEntry
	for rows.Next() {
		var s SkillRegistryEntry
		var params, deps, vaultKeys, secReport sql.NullString
		var lastScan sql.NullTime
		var enabled int

		err := rows.Scan(&s.ID, &s.Name, &s.Type, &s.Description, &s.Executable,
			&params, &deps, &vaultKeys,
			&s.CreatedAt, &s.UpdatedAt, &s.CreatedBy, &enabled,
			&s.SecurityStatus, &secReport, &lastScan,
			&s.FilePath, &s.FileHash)
		if err != nil {
			m.logger.Warn("Failed to scan skill row", "error", err)
			continue
		}
		s.Enabled = enabled == 1
		if params.Valid {
			json.Unmarshal([]byte(params.String), &s.Parameters)
		}
		if deps.Valid {
			json.Unmarshal([]byte(deps.String), &s.Dependencies)
		}
		if vaultKeys.Valid {
			json.Unmarshal([]byte(vaultKeys.String), &s.VaultKeys)
		}
		if secReport.Valid {
			var report SecurityReport
			if json.Unmarshal([]byte(secReport.String), &report) == nil {
				s.SecurityReport = &report
			}
		}
		if lastScan.Valid {
			s.LastScanAt = &lastScan.Time
		}
		skills = append(skills, s)
	}
	return skills, nil
}

// GetSkill retrieves a single skill from the registry by ID.
func (m *SkillManager) GetSkill(id string) (*SkillRegistryEntry, error) {
	var s SkillRegistryEntry
	var params, deps, vaultKeys, secReport sql.NullString
	var lastScan sql.NullTime
	var enabled int

	err := m.db.QueryRow(`SELECT id, name, type, description, executable, parameters, dependencies, vault_keys,
		created_at, updated_at, created_by, enabled, security_status, security_report, last_scan_at,
		file_path, file_hash FROM skills_registry WHERE id = ?`, id).
		Scan(&s.ID, &s.Name, &s.Type, &s.Description, &s.Executable,
			&params, &deps, &vaultKeys,
			&s.CreatedAt, &s.UpdatedAt, &s.CreatedBy, &enabled,
			&s.SecurityStatus, &secReport, &lastScan,
			&s.FilePath, &s.FileHash)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("skill not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying skill: %w", err)
	}

	s.Enabled = enabled == 1
	if params.Valid {
		json.Unmarshal([]byte(params.String), &s.Parameters)
	}
	if deps.Valid {
		json.Unmarshal([]byte(deps.String), &s.Dependencies)
	}
	if vaultKeys.Valid {
		json.Unmarshal([]byte(vaultKeys.String), &s.VaultKeys)
	}
	if secReport.Valid {
		var report SecurityReport
		if json.Unmarshal([]byte(secReport.String), &report) == nil {
			s.SecurityReport = &report
		}
	}
	if lastScan.Valid {
		s.LastScanAt = &lastScan.Time
	}

	return &s, nil
}

// GetSkillCode reads the Python source code of a skill.
func (m *SkillManager) GetSkillCode(id string) (string, error) {
	s, err := m.GetSkill(id)
	if err != nil {
		return "", err
	}
	codePath := filepath.Join(m.skillsDir, s.Executable)
	data, err := os.ReadFile(codePath)
	if err != nil {
		return "", fmt.Errorf("reading skill code: %w", err)
	}
	return string(data), nil
}

// UpdateSkillCode writes new Python source code for an existing skill and updates its file hash.
func (m *SkillManager) UpdateSkillCode(id, code string) error {
	s, err := m.GetSkill(id)
	if err != nil {
		return err
	}
	if s.Type == SkillTypeBuiltIn {
		return fmt.Errorf("built-in skills cannot be edited")
	}

	codePath := filepath.Join(m.skillsDir, s.Executable)
	if err := os.WriteFile(codePath, []byte(code), 0o644); err != nil {
		return fmt.Errorf("writing skill code: %w", err)
	}

	h := sha256.Sum256([]byte(code))
	fileHash := hex.EncodeToString(h[:])
	if _, err := m.db.Exec("UPDATE skills_registry SET file_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		fileHash, id); err != nil {
		return fmt.Errorf("updating skill hash: %w", err)
	}

	// Reset security status since code changed
	if _, err := m.db.Exec("UPDATE skills_registry SET security_status = ? WHERE id = ?",
		string(SecurityPending), id); err != nil {
		m.logger.Warn("Failed to reset security status after code update", "id", id, "error", err)
	}

	m.logger.Info("Skill code updated", "id", id, "name", s.Name)
	return nil
}

// EnableSkill enables or disables a skill.
func (m *SkillManager) EnableSkill(id string, enabled bool) error {
	result, err := m.db.Exec("UPDATE skills_registry SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		boolToInt(enabled), id)
	if err != nil {
		return fmt.Errorf("updating skill: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("skill not found: %s", id)
	}
	return nil
}

// DeleteSkill removes a skill from the registry and optionally deletes its files.
func (m *SkillManager) DeleteSkill(id string, deleteFiles bool) error {
	s, err := m.GetSkill(id)
	if err != nil {
		return err
	}

	if _, err := m.db.Exec("DELETE FROM skills_registry WHERE id = ?", id); err != nil {
		return fmt.Errorf("deleting skill from registry: %w", err)
	}

	if deleteFiles {
		// Delete Python file
		pyPath := filepath.Join(m.skillsDir, s.Executable)
		os.Remove(pyPath)

		// Delete JSON manifest
		jsonPath := filepath.Join(m.skillsDir, strings.TrimSuffix(s.Name, filepath.Ext(s.Name))+".json")
		if jsonPath == pyPath {
			jsonPath = filepath.Join(m.skillsDir, s.Name+".json")
		}
		os.Remove(jsonPath)
	}

	m.logger.Info("Skill deleted", "id", id, "name", s.Name, "files_deleted", deleteFiles)
	return nil
}

// UpdateSkillSecurity updates the security status and report for a skill.
func (m *SkillManager) UpdateSkillSecurity(id string, status SecurityStatus, report *SecurityReport) error {
	now := time.Now().UTC()
	reportJSON, _ := json.Marshal(report)

	_, err := m.db.Exec(`UPDATE skills_registry 
		SET security_status = ?, security_report = ?, last_scan_at = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?`, string(status), string(reportJSON), now, id)
	if err != nil {
		return fmt.Errorf("updating skill security: %w", err)
	}

	// Record in scan history
	if report != nil {
		m.db.Exec(`INSERT INTO skills_scan_history (skill_id, scanner_type, score, verdict, details)
			VALUES (?, 'combined', ?, ?, ?)`, id, report.OverallScore, string(status), string(reportJSON))
	}
	return nil
}

// CreateSkillEntry inserts a new skill into the registry from uploaded code.
func (m *SkillManager) CreateSkillEntry(name, description, code string, skillType SkillType, createdBy string) (*SkillRegistryEntry, error) {
	// Validate name
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid skill name: must not contain path separators or '..'")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("skill name is required")
	}

	// Check for existing skill
	pyPath := filepath.Join(m.skillsDir, name+".py")
	if _, err := os.Stat(pyPath); err == nil {
		return nil, fmt.Errorf("skill '%s' already exists", name)
	}

	// Write Python file
	if err := os.MkdirAll(m.skillsDir, 0750); err != nil {
		return nil, fmt.Errorf("creating skills directory: %w", err)
	}
	if err := os.WriteFile(pyPath, []byte(code), 0640); err != nil {
		return nil, fmt.Errorf("writing skill file: %w", err)
	}

	// Compute hash
	h := sha256.Sum256([]byte(code))
	fileHash := hex.EncodeToString(h[:])

	// Write JSON manifest
	manifest := SkillManifest{
		Name:        name,
		Description: description,
		Executable:  name + ".py",
	}

	// Detect dependencies from imports
	deps := extractImportsFromCode(code)
	manifest.Dependencies = deps

	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	jsonPath := filepath.Join(m.skillsDir, name+".json")
	if err := os.WriteFile(jsonPath, manifestJSON, 0640); err != nil {
		os.Remove(pyPath)
		return nil, fmt.Errorf("writing skill manifest: %w", err)
	}

	// Create registry entry
	id := fmt.Sprintf("%s_%s_%d", string(skillType), name, time.Now().UnixMilli())
	now := time.Now().UTC()

	depsJSON, _ := json.Marshal(deps)
	_, err := m.db.Exec(`INSERT INTO skills_registry 
		(id, name, type, description, executable, dependencies, created_at, updated_at,
		 created_by, enabled, security_status, file_path, file_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?)`,
		id, name, string(skillType), description, name+".py",
		string(depsJSON), now, now, createdBy, string(SecurityPending),
		name+".py", fileHash)
	if err != nil {
		os.Remove(pyPath)
		os.Remove(jsonPath)
		return nil, fmt.Errorf("inserting skill into registry: %w", err)
	}

	return &SkillRegistryEntry{
		ID:             id,
		Name:           name,
		Description:    description,
		Executable:     name + ".py",
		Dependencies:   deps,
		Type:           skillType,
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      createdBy,
		Enabled:        false,
		SecurityStatus: SecurityPending,
		FilePath:       name + ".py",
		FileHash:       fileHash,
	}, nil
}

// GetStats returns counts for dashboard display.
func (m *SkillManager) GetStats() (total, agent, user, pending int, err error) {
	err = m.db.QueryRow("SELECT COUNT(*) FROM skills_registry WHERE type != 'builtin'").Scan(&total)
	if err != nil {
		return
	}
	m.db.QueryRow("SELECT COUNT(*) FROM skills_registry WHERE type = 'agent'").Scan(&agent)
	m.db.QueryRow("SELECT COUNT(*) FROM skills_registry WHERE type = 'user'").Scan(&user)
	m.db.QueryRow("SELECT COUNT(*) FROM skills_registry WHERE type != 'builtin' AND security_status = 'pending'").Scan(&pending)
	return
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// extractImportsFromCode extracts top-level Python import names from source code.
// Returns only third-party package names (excludes stdlib).
func extractImportsFromCode(code string) []string {
	seen := make(map[string]bool)
	var deps []string
	for _, line := range strings.Split(code, "\n") {
		line = strings.TrimSpace(line)
		matches := importRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		mod := matches[1]
		if mod == "" {
			mod = matches[2]
		}
		if mod == "" || pythonStdlib[mod] || seen[mod] {
			continue
		}
		seen[mod] = true
		// Map import name to PyPI package if known
		if pypi, ok := importToPyPI[mod]; ok {
			deps = append(deps, pypi)
		} else {
			deps = append(deps, mod)
		}
	}
	return deps
}
