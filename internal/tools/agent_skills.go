package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/dbutil"
	"aurago/internal/sandbox"
	"aurago/internal/security"

	"gopkg.in/yaml.v3"
)

const (
	maxAgentSkillNameLength        = 64
	maxAgentSkillDescriptionLength = 1024
	maxAgentSkillCompatibilityLen  = 500
	maxAgentSkillMarkdownBytes     = 512 * 1024
	maxAgentSkillPackageBytes      = 10 * 1024 * 1024
)

var agentSkillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// AgentSkillResource describes a bundled file inside scripts/, references/, or assets/.
type AgentSkillResource struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Size       int64  `json:"size"`
	Executable bool   `json:"executable,omitempty"`
}

// AgentSkillPackage is the parsed, on-disk Agent Skills package.
type AgentSkillPackage struct {
	Name          string               `json:"name"`
	Description   string               `json:"description"`
	License       string               `json:"license,omitempty"`
	Compatibility string               `json:"compatibility,omitempty"`
	Metadata      map[string]string    `json:"metadata,omitempty"`
	AllowedTools  string               `json:"allowed_tools,omitempty"`
	Directory     string               `json:"directory"`
	SkillPath     string               `json:"skill_path"`
	Body          string               `json:"body"`
	Resources     []AgentSkillResource `json:"resources,omitempty"`
	Scripts       []AgentSkillResource `json:"scripts,omitempty"`
	Agents        []AgentSkillResource `json:"agents,omitempty"`
	PackageHash   string               `json:"package_hash"`
}

// AgentSkillRegistryEntry is the persisted Skill Manager row for Agent Skills.
type AgentSkillRegistryEntry struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	License         string               `json:"license,omitempty"`
	Compatibility   string               `json:"compatibility,omitempty"`
	Metadata        map[string]string    `json:"metadata,omitempty"`
	AllowedTools    string               `json:"allowed_tools,omitempty"`
	Directory       string               `json:"directory"`
	SkillPath       string               `json:"skill_path"`
	Resources       []AgentSkillResource `json:"resources,omitempty"`
	Scripts         []AgentSkillResource `json:"scripts,omitempty"`
	Agents          []AgentSkillResource `json:"agents,omitempty"`
	Enabled         bool                 `json:"enabled"`
	WarningApproved bool                 `json:"warning_approved"`
	SecurityStatus  SecurityStatus       `json:"security_status"`
	SecurityReport  *SecurityReport      `json:"security_report,omitempty"`
	PackageHash     string               `json:"package_hash"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	CreatedBy       string               `json:"created_by"`
}

type agentSkillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
}

// AgentSkillManager manages Agent Skills packages independently from Python skills.
type AgentSkillManager struct {
	db             *sql.DB
	agentSkillsDir string
	workspaceDir   string
	logger         *slog.Logger
}

var (
	defaultAgentSkillManager   *AgentSkillManager
	defaultAgentSkillManagerMu sync.RWMutex
)

func SetDefaultAgentSkillManager(mgr *AgentSkillManager) {
	defaultAgentSkillManagerMu.Lock()
	defer defaultAgentSkillManagerMu.Unlock()
	defaultAgentSkillManager = mgr
}

func DefaultAgentSkillManager() *AgentSkillManager {
	defaultAgentSkillManagerMu.RLock()
	defer defaultAgentSkillManagerMu.RUnlock()
	mgr := defaultAgentSkillManager
	return mgr
}

// InitAgentSkillsDB opens the Agent Skills registry database and runs migrations.
func InitAgentSkillsDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open agent skills database: %w", err)
	}
	if err := MigrateAgentSkillsDB(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// MigrateAgentSkillsDB creates the Agent Skills registry tables on an existing database.
func MigrateAgentSkillsDB(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS agent_skills_registry (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL,
		license TEXT DEFAULT '',
		compatibility TEXT DEFAULT '',
		metadata TEXT DEFAULT '{}',
		allowed_tools TEXT DEFAULT '',
		directory TEXT NOT NULL,
		skill_path TEXT NOT NULL,
		resources TEXT DEFAULT '[]',
		scripts TEXT DEFAULT '[]',
		agents TEXT DEFAULT '[]',
		enabled INTEGER DEFAULT 0,
		warning_approved INTEGER DEFAULT 0,
		security_status TEXT DEFAULT 'pending',
		security_report TEXT,
		package_hash TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_by TEXT DEFAULT 'system'
	);
	CREATE TABLE IF NOT EXISTS agent_skill_scan_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		skill_id TEXT REFERENCES agent_skills_registry(id) ON DELETE CASCADE,
		scanned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		scanner_type TEXT,
		score REAL,
		verdict TEXT,
		details TEXT
	);
	CREATE TABLE IF NOT EXISTS agent_skill_audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		skill_id TEXT REFERENCES agent_skills_registry(id) ON DELETE CASCADE,
		skill_name TEXT,
		action TEXT NOT NULL,
		actor TEXT DEFAULT 'system',
		details TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_agent_skills_enabled ON agent_skills_registry(enabled);
	CREATE INDEX IF NOT EXISTS idx_agent_skills_status ON agent_skills_registry(security_status);
	CREATE INDEX IF NOT EXISTS idx_agent_skills_name ON agent_skills_registry(name);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create agent skills schema: %w", err)
	}
	// Additive migration: agents column for existing databases.
	if _, err := db.Exec(`ALTER TABLE agent_skills_registry ADD COLUMN agents TEXT DEFAULT '[]'`); err != nil {
		// Column already exists — safe to ignore.
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("failed to migrate agent skills schema (agents column): %w", err)
		}
	}
	return nil
}

func NewAgentSkillManager(db *sql.DB, agentSkillsDir, workspaceDir string, logger *slog.Logger) *AgentSkillManager {
	return &AgentSkillManager{
		db:             db,
		agentSkillsDir: agentSkillsDir,
		workspaceDir:   workspaceDir,
		logger:         logger,
	}
}

// ParseAgentSkillPackage validates and parses one Agent Skills directory.
func ParseAgentSkillPackage(skillDir string) (*AgentSkillPackage, error) {
	absDir, err := filepath.Abs(skillDir)
	if err != nil {
		return nil, fmt.Errorf("resolving skill directory: %w", err)
	}
	info, err := os.Lstat(absDir)
	if err != nil {
		return nil, fmt.Errorf("reading skill directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("agent skill path must be a directory")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("agent skill directory symlinks are not allowed")
	}

	skillPath := filepath.Join(absDir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, fmt.Errorf("reading SKILL.md: %w", err)
	}
	if len(data) > maxAgentSkillMarkdownBytes {
		return nil, fmt.Errorf("SKILL.md too large")
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("SKILL.md must be valid UTF-8")
	}
	front, body, err := parseAgentSkillMarkdown(data)
	if err != nil {
		return nil, err
	}
	if err := validateAgentSkillFrontmatter(absDir, front); err != nil {
		return nil, err
	}

	resources, scripts, hash, err := enumerateAgentSkillFiles(absDir)
	if err != nil {
		return nil, err
	}
	var agents []AgentSkillResource
	for _, r := range resources {
		if r.Kind == "agent" {
			agents = append(agents, r)
		}
	}
	return &AgentSkillPackage{
		Name:          front.Name,
		Description:   front.Description,
		License:       front.License,
		Compatibility: front.Compatibility,
		Metadata:      front.Metadata,
		AllowedTools:  front.AllowedTools,
		Directory:     absDir,
		SkillPath:     skillPath,
		Body:          body,
		Resources:     resources,
		Scripts:       scripts,
		Agents:        agents,
		PackageHash:   hash,
	}, nil
}

func parseAgentSkillMarkdown(data []byte) (agentSkillFrontmatter, string, error) {
	text := string(data)
	if !strings.HasPrefix(text, "---") {
		return agentSkillFrontmatter{}, "", fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	lines := strings.Split(text, "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return agentSkillFrontmatter{}, "", fmt.Errorf("SKILL.md must start with YAML frontmatter delimiter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return agentSkillFrontmatter{}, "", fmt.Errorf("SKILL.md frontmatter is not closed")
	}
	var front agentSkillFrontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &front); err != nil {
		return agentSkillFrontmatter{}, "", fmt.Errorf("parsing SKILL.md frontmatter: %w", err)
	}
	if front.Metadata == nil {
		front.Metadata = map[string]string{}
	}
	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	if body == "" {
		return agentSkillFrontmatter{}, "", fmt.Errorf("SKILL.md body is required")
	}
	return front, body, nil
}

func validateAgentSkillFrontmatter(skillDir string, front agentSkillFrontmatter) error {
	name := strings.TrimSpace(front.Name)
	if name == "" {
		return fmt.Errorf("agent skill name is required")
	}
	if len(name) > maxAgentSkillNameLength || !agentSkillNamePattern.MatchString(name) {
		return fmt.Errorf("agent skill name must be 1-%d lowercase alphanumeric characters and single hyphens (no leading/trailing/consecutive hyphens)", maxAgentSkillNameLength)
	}
	if filepath.Base(skillDir) != name {
		return fmt.Errorf("agent skill name %q must match parent directory %q", name, filepath.Base(skillDir))
	}
	desc := strings.TrimSpace(front.Description)
	if desc == "" || len(desc) > maxAgentSkillDescriptionLength {
		return fmt.Errorf("agent skill description is required and must be at most %d characters", maxAgentSkillDescriptionLength)
	}
	if front.Compatibility != "" && len(front.Compatibility) > maxAgentSkillCompatibilityLen {
		return fmt.Errorf("agent skill compatibility is too long")
	}
	return nil
}

func enumerateAgentSkillFiles(skillDir string) ([]AgentSkillResource, []AgentSkillResource, string, error) {
	var resources []AgentSkillResource
	var scripts []AgentSkillResource
	type hashInput struct {
		path string
		data []byte
	}
	var files []hashInput
	totalBytes := 0

	err := filepath.WalkDir(skillDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == skillDir {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in agent skills: %s", path)
		}
		rel, err := filepath.Rel(skillDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "SKILL.md" {
			if d.IsDir() {
				return fmt.Errorf("SKILL.md must be a file")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files = append(files, hashInput{path: rel, data: data})
			totalBytes += len(data)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		parts := strings.Split(rel, "/")
		if len(parts) < 2 {
			return nil
		}
		topDir := parts[0]
		if !isAgentSkillAllowedTopDir(topDir) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		totalBytes += len(data)
		if totalBytes > maxAgentSkillPackageBytes {
			return fmt.Errorf("agent skill package too large")
		}
		kind := classifyAgentSkillResource(topDir, rel)
		scriptExts := map[string]bool{".py": true, ".sh": true, ".js": true}
		ext := strings.ToLower(filepath.Ext(parts[len(parts)-1]))
		isScript := topDir == "scripts" && scriptExts[ext]
		res := AgentSkillResource{
			Path:       rel,
			Kind:       kind,
			Size:       info.Size(),
			Executable: isScript && ext == ".py",
		}
		resources = append(resources, res)
		if isScript {
			scripts = append(scripts, res)
		}
		files = append(files, hashInput{path: rel, data: data})
		return nil
	})
	if err != nil {
		return nil, nil, "", err
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].Path < resources[j].Path })
	sort.Slice(scripts, func(i, j int) bool { return scripts[i].Path < scripts[j].Path })
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.path))
		h.Write([]byte{0})
		h.Write(f.data)
		h.Write([]byte{0})
	}
	return resources, scripts, hex.EncodeToString(h.Sum(nil)), nil
}

func classifyAgentSkillResource(topDir, relPath string) string {
	switch topDir {
	case "scripts":
		return "script"
	case "references":
		return "reference"
	case "assets":
		return "asset"
	case "agents":
		return "agent"
	default:
		return "file"
	}
}

func isAgentSkillAllowedTopDir(name string) bool {
	switch name {
	case "scripts", "references", "assets", "agents":
		return true
	default:
		return false
	}
}

// ScanAgentSkillPackage scans SKILL.md, references, and Python scripts for security risks.
func ScanAgentSkillPackage(ctx context.Context, pkg *AgentSkillPackage, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) (*SecurityReport, SecurityStatus, error) {
	if pkg == nil {
		return nil, SecurityError, fmt.Errorf("agent skill package is required")
	}
	report := &SecurityReport{ScannedAt: time.Now().UTC()}
	var findings []Finding
	markdownFindings, err := scanAgentSkillMarkdownFiles(pkg)
	if err != nil {
		return nil, SecurityError, err
	}
	findings = append(findings, markdownFindings...)
	for _, script := range pkg.Scripts {
		data, err := os.ReadFile(filepath.Join(pkg.Directory, filepath.FromSlash(script.Path)))
		if err != nil {
			return nil, SecurityError, fmt.Errorf("reading script %s: %w", script.Path, err)
		}
		ext := strings.ToLower(filepath.Ext(script.Path))
		var scriptFindings []Finding
		switch ext {
		case ".sh":
			scriptFindings = StaticCodeAnalysisBash(string(data))
		case ".js":
			scriptFindings = StaticCodeAnalysisJS(string(data))
		default:
			scriptFindings = StaticCodeAnalysis(string(data))
		}
		for _, f := range scriptFindings {
			f.Message = fmt.Sprintf("%s (%s)", f.Message, script.Path)
			findings = append(findings, f)
		}
	}
	report.StaticAnalysis = findings
	hasNonPythonScript := false
	for _, s := range pkg.Scripts {
		ext := strings.ToLower(filepath.Ext(s.Path))
		if ext == ".sh" || ext == ".js" {
			hasNonPythonScript = true
			break
		}
	}
	if useGuardian && guardian != nil {
		guardianText := buildAgentSkillGuardianText(pkg)
		result := guardian.EvaluateContent(ctx, "agent_skill_package", guardianText)
		report.GuardianScore = result.RiskScore
		report.GuardianVerdict = string(result.Decision)
		report.GuardianReason = result.Reason
	}
	var scanErr error
	if cfg, ok := firstSkillSpectorConfig(skillSpector); ok && cfg.Enabled {
		var ssStatus SecurityStatus
		report.SkillSpector, ssStatus, scanErr = RunSkillSpectorScan(ctx, pkg.Directory, cfg)
		if ssStatus == SecurityError && scanErr != nil && report.SkillSpector == nil {
			report.SkillSpector = &SkillSpectorReport{Error: scanErr.Error()}
		}
	}
	status := DetermineSecurityStatus(report)
	if hasNonPythonScript && status == SecurityClean {
		status = SecurityWarning
		report.OverallStatus = string(SecurityWarning)
		report.OverallScore = 0.5
	}
	return report, status, scanErr
}

func scanAgentSkillMarkdownFiles(pkg *AgentSkillPackage) ([]Finding, error) {
	var findings []Finding
	files := []string{"SKILL.md"}
	for _, res := range pkg.Resources {
		if res.Kind == "reference" || res.Kind == "agent" {
			files = append(files, res.Path)
		}
	}
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(pkg.Directory, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", rel, err)
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			lower := strings.ToLower(line)
			switch {
			case strings.Contains(lower, "ignore previous instructions"),
				strings.Contains(lower, "reveal system prompt"),
				strings.Contains(lower, "reveal system prompts"),
				strings.Contains(lower, "reveal secrets"),
				strings.Contains(lower, "exfiltrate"):
				findings = append(findings, Finding{
					Severity: "critical",
					Category: "prompt",
					Message:  "Potential prompt-injection instruction in Agent Skill Markdown (" + rel + ")",
					Line:     i + 1,
					Pattern:  "prompt_injection",
				})
			case strings.Contains(lower, "/etc/passwd"),
				strings.Contains(lower, "password"),
				strings.Contains(lower, "credential"),
				strings.Contains(lower, "token"):
				findings = append(findings, Finding{
					Severity: "warning",
					Category: "prompt",
					Message:  "Sensitive access language in Agent Skill Markdown (" + rel + ")",
					Line:     i + 1,
					Pattern:  "sensitive_reference",
				})
			}
		}
	}
	return findings, nil
}

func buildAgentSkillGuardianText(pkg *AgentSkillPackage) string {
	var b strings.Builder
	b.WriteString("Agent Skill: ")
	b.WriteString(pkg.Name)
	b.WriteString("\nDescription: ")
	b.WriteString(pkg.Description)
	b.WriteString("\n\n")
	if data, err := os.ReadFile(pkg.SkillPath); err == nil {
		b.Write(data)
	}
	for _, res := range pkg.Resources {
		if res.Kind == "reference" || res.Kind == "script" {
			b.WriteString("\n\n--- ")
			b.WriteString(res.Path)
			b.WriteString(" ---\n")
			if data, err := os.ReadFile(filepath.Join(pkg.Directory, filepath.FromSlash(res.Path))); err == nil {
				b.Write(data)
			}
		}
	}
	return b.String()
}

func (m *AgentSkillManager) CreateAgentSkill(ctx context.Context, name, description, body, createdBy string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) (*AgentSkillRegistryEntry, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if !agentSkillNamePattern.MatchString(name) || len(name) > maxAgentSkillNameLength {
		return nil, fmt.Errorf("invalid agent skill name")
	}
	if description == "" || len(description) > maxAgentSkillDescriptionLength {
		return nil, fmt.Errorf("invalid agent skill description")
	}
	if createdBy == "" {
		createdBy = "user"
	}
	if err := os.MkdirAll(m.agentSkillsDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating agent skills directory: %w", err)
	}
	dir := filepath.Join(m.agentSkillsDir, name)
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("agent skill %q already exists", name)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating agent skill: %w", err)
	}
	markdown := composeAgentSkillMarkdown(name, description, body)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(markdown), 0o640); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("writing SKILL.md: %w", err)
	}
	pkg, err := ParseAgentSkillPackage(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	report, status, err := ScanAgentSkillPackage(ctx, pkg, guardian, useGuardian, skillSpector...)
	if err != nil {
		status = SecurityError
	}
	entry, saveErr := m.upsertAgentSkillPackage(pkg, createdBy, report, status, false, false)
	if saveErr != nil {
		_ = os.RemoveAll(dir)
		return nil, saveErr
	}
	m.audit(entry.ID, entry.Name, "create", createdBy, "")
	return entry, err
}

func composeAgentSkillMarkdown(name, description, body string) string {
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "---") {
		return body + "\n"
	}
	if body == "" {
		body = "# " + name + "\n\nUse this skill when the task matches the description."
	}
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n%s\n", name, yamlQuoteIfNeeded(description), body)
}

func yamlQuoteIfNeeded(value string) string {
	if strings.ContainsAny(value, ":#[]{}\n\r\t") || strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		b, _ := yaml.Marshal(value)
		return strings.TrimSpace(string(b))
	}
	return value
}

func (m *AgentSkillManager) ImportAgentSkillZIP(ctx context.Context, data []byte, actor string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) (*AgentSkillRegistryEntry, *ValidationResult, error) {
	validation := &ValidationResult{Passed: true}
	if len(data) == 0 || len(data) > maxAgentSkillPackageBytes {
		validation.Passed = false
		validation.Message = "Agent Skill package is empty or too large"
		return nil, validation, errors.New(validation.Message)
	}
	if err := os.MkdirAll(m.agentSkillsDir, 0o750); err != nil {
		return nil, validation, fmt.Errorf("creating agent skills directory: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		validation.Passed = false
		validation.Message = "Invalid ZIP package"
		return nil, validation, err
	}
	tmpBase, err := os.MkdirTemp(m.agentSkillsDir, ".import-*")
	if err != nil {
		return nil, validation, err
	}
	defer os.RemoveAll(tmpBase)

	rootName := ""
	total := 0
	for _, f := range zr.File {
		rel := filepath.ToSlash(f.Name)
		if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") || strings.Contains(rel, ":") {
			validation.Passed = false
			validation.Message = "ZIP contains unsafe paths"
			return nil, validation, errors.New(validation.Message)
		}
		parts := strings.Split(strings.TrimSuffix(rel, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		if rootName == "" {
			rootName = parts[0]
		}
		if parts[0] != rootName {
			validation.Passed = false
			validation.Message = "ZIP must contain exactly one Agent Skill root directory"
			return nil, validation, errors.New(validation.Message)
		}
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			validation.Passed = false
			validation.Message = "ZIP symlinks are not allowed"
			return nil, validation, errors.New(validation.Message)
		}
		target := filepath.Join(tmpBase, filepath.FromSlash(rel))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return nil, validation, err
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, validation, err
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, maxAgentSkillPackageBytes+1))
		_ = rc.Close()
		if readErr != nil {
			return nil, validation, readErr
		}
		total += len(content)
		if total > maxAgentSkillPackageBytes {
			validation.Passed = false
			validation.Message = "Agent Skill package is too large"
			return nil, validation, errors.New(validation.Message)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return nil, validation, err
		}
		if err := os.WriteFile(target, content, 0o640); err != nil {
			return nil, validation, err
		}
	}
	if rootName == "" {
		validation.Passed = false
		validation.Message = "ZIP package is empty"
		return nil, validation, errors.New(validation.Message)
	}
	tmpSkillDir := filepath.Join(tmpBase, rootName)
	pkg, err := ParseAgentSkillPackage(tmpSkillDir)
	if err != nil {
		validation.Passed = false
		validation.Message = err.Error()
		return nil, validation, err
	}
	finalDir := filepath.Join(m.agentSkillsDir, pkg.Name)
	if _, err := os.Stat(finalDir); err == nil {
		validation.Passed = false
		validation.Message = "Agent Skill already exists"
		return nil, validation, errors.New(validation.Message)
	} else if !os.IsNotExist(err) {
		return nil, validation, err
	}
	if err := os.Rename(tmpSkillDir, finalDir); err != nil {
		return nil, validation, fmt.Errorf("installing agent skill: %w", err)
	}
	pkg, err = ParseAgentSkillPackage(finalDir)
	if err != nil {
		_ = os.RemoveAll(finalDir)
		validation.Passed = false
		validation.Message = err.Error()
		return nil, validation, err
	}
	report, status, scanErr := ScanAgentSkillPackage(ctx, pkg, guardian, useGuardian, skillSpector...)
	entry, err := m.upsertAgentSkillPackage(pkg, actor, report, status, false, false)
	if err != nil {
		_ = os.RemoveAll(finalDir)
		return nil, validation, err
	}
	m.audit(entry.ID, entry.Name, "import", actor, "")
	return entry, validation, scanErr
}

// ImportAgentSkillDirectory copies a local Agent Skill directory into the managed registry.
func (m *AgentSkillManager) ImportAgentSkillDirectory(ctx context.Context, sourceDir, actor string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) (*AgentSkillRegistryEntry, *ValidationResult, error) {
	validation := &ValidationResult{Passed: true}
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		validation.Passed = false
		validation.Message = "Agent Skill source directory is required"
		return nil, validation, errors.New(validation.Message)
	}
	pkg, err := ParseAgentSkillPackage(sourceDir)
	if err != nil {
		validation.Passed = false
		validation.Message = err.Error()
		return nil, validation, err
	}
	if err := os.MkdirAll(m.agentSkillsDir, 0o750); err != nil {
		return nil, validation, fmt.Errorf("creating agent skills directory: %w", err)
	}
	finalDir := filepath.Join(m.agentSkillsDir, pkg.Name)
	absSource, _ := filepath.Abs(sourceDir)
	absFinal, _ := filepath.Abs(finalDir)
	if agentSkillSamePath(absSource, absFinal) {
		report, status, scanErr := ScanAgentSkillPackage(ctx, pkg, guardian, useGuardian, skillSpector...)
		entry, err := m.upsertAgentSkillPackage(pkg, actor, report, status, false, false)
		if err != nil {
			return nil, validation, err
		}
		m.audit(entry.ID, entry.Name, "import_directory", actor, "in_place=true")
		return entry, validation, scanErr
	}
	if _, err := os.Stat(finalDir); err == nil {
		validation.Passed = false
		validation.Message = "Agent Skill already exists"
		return nil, validation, errors.New(validation.Message)
	} else if !os.IsNotExist(err) {
		return nil, validation, err
	}
	if err := copyAgentSkillDirectory(pkg.Directory, finalDir); err != nil {
		_ = os.RemoveAll(finalDir)
		return nil, validation, err
	}
	pkg, err = ParseAgentSkillPackage(finalDir)
	if err != nil {
		_ = os.RemoveAll(finalDir)
		validation.Passed = false
		validation.Message = err.Error()
		return nil, validation, err
	}
	report, status, scanErr := ScanAgentSkillPackage(ctx, pkg, guardian, useGuardian, skillSpector...)
	entry, err := m.upsertAgentSkillPackage(pkg, actor, report, status, false, false)
	if err != nil {
		_ = os.RemoveAll(finalDir)
		return nil, validation, err
	}
	m.audit(entry.ID, entry.Name, "import_directory", actor, "")
	return entry, validation, scanErr
}

func copyAgentSkillDirectory(sourceDir, destDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed in agent skills: %s", path)
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(destDir, 0o750)
		}
		target := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o640)
	})
}

func agentSkillSamePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func (m *AgentSkillManager) SyncFromDisk(ctx context.Context, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) error {
	entries, err := os.ReadDir(m.agentSkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, ent := range entries {
		if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		pkg, err := ParseAgentSkillPackage(filepath.Join(m.agentSkillsDir, ent.Name()))
		if err != nil {
			if m.logger != nil {
				m.logger.Warn("Skipping invalid Agent Skill", "name", ent.Name(), "error", err)
			}
			continue
		}
		existing, _ := m.GetAgentSkillByName(pkg.Name)
		if existing != nil && existing.PackageHash == pkg.PackageHash {
			continue
		}
		report, status, scanErr := ScanAgentSkillPackage(ctx, pkg, guardian, useGuardian, skillSpector...)
		if scanErr != nil && m.logger != nil {
			m.logger.Warn("Agent Skill scan failed during sync", "name", pkg.Name, "error", scanErr)
		}
		enabled := false
		warningApproved := false
		if existing != nil {
			warningApproved = existing.WarningApproved && status == SecurityWarning
			enabled = existing.Enabled && (status == SecurityClean || (status == SecurityWarning && warningApproved))
		}
		if _, err := m.upsertAgentSkillPackage(pkg, "system:sync", report, status, enabled, warningApproved); err != nil && m.logger != nil {
			m.logger.Warn("Failed to sync Agent Skill", "name", pkg.Name, "error", err)
		}
	}
	return nil
}

// VerifyAgentSkill reparses, rescans, and disables a package when its hash changed.
func (m *AgentSkillManager) VerifyAgentSkill(ctx context.Context, id, actor string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) (*AgentSkillRegistryEntry, error) {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return nil, err
	}
	pkg, err := ParseAgentSkillPackage(entry.Directory)
	if err != nil {
		_, _ = m.db.Exec(`UPDATE agent_skills_registry SET enabled = 0, warning_approved = 0, security_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(SecurityError), id)
		m.audit(id, entry.Name, "verify_error", actor, err.Error())
		return nil, err
	}
	report, status, scanErr := ScanAgentSkillPackage(ctx, pkg, guardian, useGuardian, skillSpector...)
	enabled := entry.Enabled && entry.PackageHash == pkg.PackageHash && (status == SecurityClean || (status == SecurityWarning && entry.WarningApproved))
	warningApproved := entry.WarningApproved && entry.PackageHash == pkg.PackageHash
	updated, err := m.upsertAgentSkillPackage(pkg, actor, report, status, enabled, warningApproved)
	if err != nil {
		return nil, err
	}
	m.audit(updated.ID, updated.Name, "verify", actor, fmt.Sprintf("hash_changed=%t", entry.PackageHash != pkg.PackageHash))
	return updated, scanErr
}

// LoadCurrentAgentSkillPackage reparses a registry entry and verifies it still
// matches the hash that was scanned before the package can be used.
func (m *AgentSkillManager) LoadCurrentAgentSkillPackage(entry *AgentSkillRegistryEntry, actor string) (*AgentSkillPackage, error) {
	if m == nil || entry == nil {
		return nil, fmt.Errorf("agent skill is missing")
	}
	if actor == "" {
		actor = "system"
	}
	pkg, err := ParseAgentSkillPackage(entry.Directory)
	if err != nil {
		_, _ = m.db.Exec(`UPDATE agent_skills_registry
			SET enabled = 0, warning_approved = 0, security_status = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`, string(SecurityError), entry.ID)
		m.audit(entry.ID, entry.Name, "package_invalid", actor, err.Error())
		return nil, fmt.Errorf("agent skill package is invalid; verify before use: %w", err)
	}
	if pkg.PackageHash != entry.PackageHash {
		_, _ = m.db.Exec(`UPDATE agent_skills_registry
			SET enabled = 0, warning_approved = 0, security_status = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`, string(SecurityPending), entry.ID)
		m.audit(entry.ID, entry.Name, "package_changed", actor, "hash changed since last verification")
		return nil, fmt.Errorf("agent skill package changed since last verification; verify before use")
	}
	return pkg, nil
}

func (m *AgentSkillManager) upsertAgentSkillPackage(pkg *AgentSkillPackage, actor string, report *SecurityReport, status SecurityStatus, enabled, warningApproved bool) (*AgentSkillRegistryEntry, error) {
	if actor == "" {
		actor = "system"
	}
	if status == "" {
		status = SecurityPending
	}
	resourcesJSON, _ := json.Marshal(pkg.Resources)
	scriptsJSON, _ := json.Marshal(pkg.Scripts)
	agentsJSON, _ := json.Marshal(pkg.Agents)
	metadataJSON, _ := json.Marshal(pkg.Metadata)
	reportJSON, _ := json.Marshal(report)
	var existingID string
	err := m.db.QueryRow("SELECT id FROM agent_skills_registry WHERE name = ?", pkg.Name).Scan(&existingID)
	if err == sql.ErrNoRows {
		existingID = fmt.Sprintf("%s_%d", pkg.Name, time.Now().UnixMilli())
		_, err = m.db.Exec(`INSERT INTO agent_skills_registry
			(id, name, description, license, compatibility, metadata, allowed_tools, directory, skill_path,
			 resources, scripts, agents, enabled, warning_approved, security_status, security_report, package_hash, created_by)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			existingID, pkg.Name, pkg.Description, pkg.License, pkg.Compatibility, string(metadataJSON), pkg.AllowedTools,
			pkg.Directory, pkg.SkillPath, string(resourcesJSON), string(scriptsJSON), string(agentsJSON),
			boolInt(enabled), boolInt(warningApproved),
			string(status), nullableJSON(reportJSON, report), pkg.PackageHash, actor)
	} else if err == nil {
		_, err = m.db.Exec(`UPDATE agent_skills_registry SET
			description = ?, license = ?, compatibility = ?, metadata = ?, allowed_tools = ?, directory = ?, skill_path = ?,
			resources = ?, scripts = ?, agents = ?, enabled = ?, warning_approved = ?, security_status = ?, security_report = ?,
			package_hash = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			pkg.Description, pkg.License, pkg.Compatibility, string(metadataJSON), pkg.AllowedTools, pkg.Directory,
			pkg.SkillPath, string(resourcesJSON), string(scriptsJSON), string(agentsJSON),
			boolInt(enabled), boolInt(warningApproved),
			string(status), nullableJSON(reportJSON, report), pkg.PackageHash, existingID)
	}
	if err != nil {
		return nil, err
	}
	if report != nil {
		_, _ = m.db.Exec(`INSERT INTO agent_skill_scan_history (skill_id, scanner_type, score, verdict, details)
			VALUES (?, ?, ?, ?, ?)`, existingID, "combined", report.OverallScore, string(status), nullableJSON(reportJSON, report))
	}
	return m.GetAgentSkill(existingID)
}

func nullableJSON(data []byte, source any) any {
	if source == nil {
		return nil
	}
	return string(data)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (m *AgentSkillManager) ListAgentSkills(enabledOnly bool, search string) ([]AgentSkillRegistryEntry, error) {
	query := `SELECT id, name, description, license, compatibility, metadata, allowed_tools, directory, skill_path,
		resources, scripts, agents, enabled, warning_approved, security_status, security_report, package_hash,
		created_at, updated_at, created_by FROM agent_skills_registry WHERE 1=1`
	var args []any
	if enabledOnly {
		query += " AND enabled = 1 AND security_status IN ('clean','warning')"
	}
	if strings.TrimSpace(search) != "" {
		query += " AND (name LIKE ? OR description LIKE ?)"
		term := "%" + strings.TrimSpace(search) + "%"
		args = append(args, term, term)
	}
	query += " ORDER BY name ASC"
	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentSkillRegistryEntry
	for rows.Next() {
		entry, err := scanAgentSkillEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *entry)
	}
	return out, rows.Err()
}

func (m *AgentSkillManager) GetAgentSkill(id string) (*AgentSkillRegistryEntry, error) {
	row := m.db.QueryRow(`SELECT id, name, description, license, compatibility, metadata, allowed_tools, directory, skill_path,
		resources, scripts, agents, enabled, warning_approved, security_status, security_report, package_hash,
		created_at, updated_at, created_by FROM agent_skills_registry WHERE id = ?`, id)
	return scanAgentSkillEntry(row)
}

func (m *AgentSkillManager) GetAgentSkillByName(name string) (*AgentSkillRegistryEntry, error) {
	row := m.db.QueryRow(`SELECT id, name, description, license, compatibility, metadata, allowed_tools, directory, skill_path,
		resources, scripts, agents, enabled, warning_approved, security_status, security_report, package_hash,
		created_at, updated_at, created_by FROM agent_skills_registry WHERE name = ?`, name)
	return scanAgentSkillEntry(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAgentSkillEntry(row rowScanner) (*AgentSkillRegistryEntry, error) {
	var entry AgentSkillRegistryEntry
	var metadata, resources, scripts, agents string
	var report sql.NullString
	var enabled, warningApproved int
	if err := row.Scan(&entry.ID, &entry.Name, &entry.Description, &entry.License, &entry.Compatibility, &metadata,
		&entry.AllowedTools, &entry.Directory, &entry.SkillPath, &resources, &scripts, &agents, &enabled, &warningApproved,
		&entry.SecurityStatus, &report, &entry.PackageHash, &entry.CreatedAt, &entry.UpdatedAt, &entry.CreatedBy); err != nil {
		return nil, err
	}
	entry.Enabled = enabled != 0
	entry.WarningApproved = warningApproved != 0
	_ = json.Unmarshal([]byte(metadata), &entry.Metadata)
	_ = json.Unmarshal([]byte(resources), &entry.Resources)
	_ = json.Unmarshal([]byte(scripts), &entry.Scripts)
	_ = json.Unmarshal([]byte(agents), &entry.Agents)
	if report.Valid && report.String != "" {
		var sec SecurityReport
		if json.Unmarshal([]byte(report.String), &sec) == nil {
			entry.SecurityReport = &sec
		}
	}
	if entry.Metadata == nil {
		entry.Metadata = map[string]string{}
	}
	return &entry, nil
}

func (m *AgentSkillManager) EnableAgentSkill(id string, enabled bool, actor string) error {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return err
	}
	if enabled {
		if _, err := m.LoadCurrentAgentSkillPackage(entry, actor); err != nil {
			return err
		}
		switch entry.SecurityStatus {
		case SecurityClean:
		case SecurityWarning:
			if !entry.WarningApproved {
				return fmt.Errorf("agent skill warning status requires approval before enabling")
			}
		default:
			return fmt.Errorf("agent skill with security status %s cannot be enabled", entry.SecurityStatus)
		}
	}
	if _, err := m.db.Exec("UPDATE agent_skills_registry SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", boolInt(enabled), id); err != nil {
		return err
	}
	m.audit(id, entry.Name, "enable", actor, fmt.Sprintf("enabled=%t", enabled))
	return nil
}

func (m *AgentSkillManager) ApproveAgentSkillWarning(id, actor string) error {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return err
	}
	if entry.SecurityStatus != SecurityWarning {
		return fmt.Errorf("only warning Agent Skills can be approved")
	}
	if _, err := m.db.Exec("UPDATE agent_skills_registry SET warning_approved = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
		return err
	}
	m.audit(id, entry.Name, "approve_warning", actor, "")
	return nil
}

func (m *AgentSkillManager) WriteAgentSkillFile(ctx context.Context, id, relPath, content, actor string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) error {
	return m.writeAgentSkillFileBytes(ctx, id, relPath, []byte(content), false, actor, guardian, useGuardian, skillSpector...)
}

func (m *AgentSkillManager) WriteAgentSkillFileBytes(ctx context.Context, id, relPath string, content []byte, actor string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) error {
	return m.writeAgentSkillFileBytes(ctx, id, relPath, content, false, actor, guardian, useGuardian, skillSpector...)
}

func (m *AgentSkillManager) writeAgentSkillFileBytes(ctx context.Context, id, relPath string, content []byte, isBinary bool, actor string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) error {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return err
	}
	relPath, err = validateAgentSkillEditablePath(relPath)
	if err != nil {
		return err
	}
	full := filepath.Join(entry.Directory, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(full, content, 0o640); err != nil {
		return err
	}
	return m.applyEditSafety(ctx, entry, relPath, actor, guardian, useGuardian, skillSpector...)
}

func (m *AgentSkillManager) ReadAgentSkillFile(id, relPath string) (string, error) {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return "", err
	}
	relPath, err = validateAgentSkillEditablePath(relPath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(entry.Directory, filepath.FromSlash(relPath)))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("file is not valid UTF-8; use the raw bytes endpoint for binary files")
	}
	return string(data), nil
}

func (m *AgentSkillManager) ReadAgentSkillFileBytes(id, relPath string) ([]byte, bool, error) {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return nil, false, err
	}
	relPath, err = validateAgentSkillEditablePath(relPath)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(filepath.Join(entry.Directory, filepath.FromSlash(relPath)))
	if err != nil {
		return nil, false, err
	}
	return data, !utf8.Valid(data), nil
}

func (m *AgentSkillManager) CreateAgentSkillFile(ctx context.Context, id, relPath string, content []byte, isBinary bool, actor string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) error {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return err
	}
	relPath, err = validateAgentSkillEditablePath(relPath)
	if err != nil {
		return err
	}
	full := filepath.Join(entry.Directory, filepath.FromSlash(relPath))
	if _, err := os.Stat(full); err == nil {
		return fmt.Errorf("file already exists: %s", relPath)
	}
	return m.writeAgentSkillFileBytes(ctx, id, relPath, content, isBinary, actor, guardian, useGuardian, skillSpector...)
}

func (m *AgentSkillManager) DeleteAgentSkillFile(ctx context.Context, id, relPath, actor string, skillSpector ...SkillSpectorConfig) error {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return err
	}
	relPath, err = validateAgentSkillEditablePath(relPath)
	if err != nil {
		return err
	}
	if relPath == "SKILL.md" {
		return fmt.Errorf("cannot delete SKILL.md")
	}
	full := filepath.Join(entry.Directory, filepath.FromSlash(relPath))
	if err := os.Remove(full); err != nil {
		return err
	}
	return m.applyEditSafety(ctx, entry, relPath, actor, nil, false, skillSpector...)
}

func (m *AgentSkillManager) RenameAgentSkillFile(ctx context.Context, id, oldRel, newRel, actor string, skillSpector ...SkillSpectorConfig) error {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return err
	}
	oldRel, err = validateAgentSkillEditablePath(oldRel)
	if err != nil {
		return err
	}
	newRel, err = validateAgentSkillEditablePath(newRel)
	if err != nil {
		return err
	}
	if oldRel == "SKILL.md" {
		return fmt.Errorf("cannot rename SKILL.md")
	}
	oldFull := filepath.Join(entry.Directory, filepath.FromSlash(oldRel))
	newFull := filepath.Join(entry.Directory, filepath.FromSlash(newRel))
	if err := os.MkdirAll(filepath.Dir(newFull), 0o750); err != nil {
		return err
	}
	if err := os.Rename(oldFull, newFull); err != nil {
		return err
	}
	return m.applyEditSafety(ctx, entry, newRel, actor, nil, false, skillSpector...)
}

// applyEditSafety re-scans after a file change and applies D4 edit-safety logic:
// disable and clear warning approval only when (a) the new status is dangerous/error,
// or (b) security-relevant files (SKILL.md, scripts/) changed their hash.
// Pure references/assets edits that remain at the same or better status keep the enabled state.
func (m *AgentSkillManager) applyEditSafety(ctx context.Context, entry *AgentSkillRegistryEntry, changedPath, actor string, guardian *security.LLMGuardian, useGuardian bool, skillSpector ...SkillSpectorConfig) error {
	pkg, err := ParseAgentSkillPackage(entry.Directory)
	if err != nil {
		return err
	}
	report, status, scanErr := ScanAgentSkillPackage(ctx, pkg, guardian, useGuardian, skillSpector...)
	isSecurityRelevant := changedPath == "SKILL.md" || strings.HasPrefix(changedPath, "scripts/")
	hashChanged := pkg.PackageHash != entry.PackageHash
	statusDegraded := status == SecurityDangerous || status == SecurityError || (status == SecurityWarning && entry.SecurityStatus == SecurityClean)
	securityRelevantChange := isSecurityRelevant && hashChanged
	shouldBeDisabled := statusDegraded || securityRelevantChange
	enabled := entry.Enabled && !shouldBeDisabled
	warningApproved := entry.WarningApproved
	if securityRelevantChange || statusDegraded {
		warningApproved = false
	}
	_, err = m.upsertAgentSkillPackage(pkg, actor, report, status, enabled, warningApproved)
	if err != nil {
		return err
	}
	m.audit(entry.ID, entry.Name, "write_file", actor, changedPath)
	return scanErr
}

func validateAgentSkillEditablePath(relPath string) (string, error) {
	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	if relPath == "" || strings.HasPrefix(relPath, "/") || strings.Contains(relPath, "..") || strings.Contains(relPath, ":") {
		return "", fmt.Errorf("invalid agent skill file path")
	}
	if relPath == "SKILL.md" {
		return relPath, nil
	}
	parts := strings.Split(relPath, "/")
	if len(parts) < 2 || !isAgentSkillAllowedTopDir(parts[0]) {
		return "", fmt.Errorf("agent skill file path must be SKILL.md or a file under scripts/, references/, assets/, or agents/")
	}
	if parts[0] == "scripts" {
		ext := strings.ToLower(filepath.Ext(parts[len(parts)-1]))
		if ext != ".py" && ext != ".sh" && ext != ".js" {
			return "", fmt.Errorf("only .py, .sh, and .js scripts are supported")
		}
	}
	return relPath, nil
}

func (m *AgentSkillManager) RunAgentSkillScript(ctx context.Context, id, scriptPath string, args map[string]interface{}) (string, error) {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return "", err
	}
	if !entry.Enabled {
		return "", fmt.Errorf("agent skill is disabled")
	}
	if entry.SecurityStatus == SecurityDangerous || entry.SecurityStatus == SecurityError || entry.SecurityStatus == SecurityPending {
		return "", fmt.Errorf("agent skill security status %s cannot run scripts", entry.SecurityStatus)
	}
	if entry.SecurityStatus == SecurityWarning && !entry.WarningApproved {
		return "", fmt.Errorf("agent skill warning status requires approval")
	}
	pkg, err := m.LoadCurrentAgentSkillPackage(entry, "agent")
	if err != nil {
		return "", err
	}
	scriptPath, err = validateAgentSkillEditablePath(scriptPath)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(scriptPath, "scripts/") {
		return "", fmt.Errorf("agent skill script path must be under scripts/")
	}
	found := false
	for _, s := range pkg.Scripts {
		if s.Path == scriptPath {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("agent skill script %q is not registered", scriptPath)
	}
	ext := strings.ToLower(filepath.Ext(scriptPath))
	input, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	if len(input) > maxSkillArgsBytes {
		return "", fmt.Errorf("script args too large")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, GetSkillTimeout())
	defer cancel()

	var cmd *exec.Cmd
	switch ext {
	case ".py":
		pythonBin := GetPythonBin(m.workspaceDir)
		if _, err := os.Stat(pythonBin); err != nil {
			if fallback := findSystemPython(); fallback != "" {
				pythonBin = fallback
			}
		}
		if pythonBin == "" {
			return "", fmt.Errorf("python not found")
		}
		cmd = exec.CommandContext(ctx, pythonBin, "-u", filepath.Join(entry.Directory, filepath.FromSlash(scriptPath)))
	case ".sh":
		shellBin := findShellBinary()
		if shellBin == "" {
			return "", fmt.Errorf("shell interpreter (bash/sh) not found")
		}
		cmd = exec.CommandContext(ctx, shellBin, filepath.Join(entry.Directory, filepath.FromSlash(scriptPath)))
	case ".js":
		nodeBin := findNodeBinary()
		if nodeBin == "" {
			return "", fmt.Errorf("node.js interpreter not found")
		}
		cmd = exec.CommandContext(ctx, nodeBin, filepath.Join(entry.Directory, filepath.FromSlash(scriptPath)))
	default:
		return "", fmt.Errorf("unsupported script extension: %s", ext)
	}

	cmd.Dir = entry.Directory
	cmd.Env = sandbox.FilterEnv(os.Environ())
	SetSkillLimits(cmd, 1024, int(GetSkillTimeout().Seconds()))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	out := &limitWriter{limit: maxSkillOutputBytes}
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		return "", err
	}
	defer func() {
		if cmd.Process != nil {
			KillProcessTree(cmd.Process.Pid)
		}
	}()
	if _, err := stdin.Write(input); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return "", err
	}
	_ = stdin.Close()
	err = cmd.Wait()
	output := out.buf.String()
	if out.overflow {
		output += fmt.Sprintf("\n[OUTPUT TRUNCATED: exceeded %d MB limit]", maxSkillOutputBytes/(1024*1024))
	}
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("TIMEOUT: agent skill script exceeded %s limit", GetSkillTimeout().Round(time.Second))
	}
	if err != nil {
		return output, fmt.Errorf("script execution failed: %w", err)
	}
	return output, nil
}

func findShellBinary() string {
	for _, name := range []string{"bash", "sh"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func findNodeBinary() string {
	if p, err := exec.LookPath("node"); err == nil {
		return p
	}
	if p, err := exec.LookPath("nodejs"); err == nil {
		return p
	}
	return ""
}

func (m *AgentSkillManager) DeleteAgentSkill(id string, deleteFiles bool, actor string) error {
	entry, err := m.GetAgentSkill(id)
	if err != nil {
		return err
	}
	if _, err := m.db.Exec("DELETE FROM agent_skills_registry WHERE id = ?", id); err != nil {
		return err
	}
	if deleteFiles {
		_ = os.RemoveAll(entry.Directory)
	}
	m.audit(id, entry.Name, "delete", actor, fmt.Sprintf("delete_files=%t", deleteFiles))
	return nil
}

func (m *AgentSkillManager) audit(id, name, action, actor, details string) {
	if m == nil || m.db == nil {
		return
	}
	if actor == "" {
		actor = "system"
	}
	_, _ = m.db.Exec(`INSERT INTO agent_skill_audit_log (skill_id, skill_name, action, actor, details)
		VALUES (?, ?, ?, ?, ?)`, id, name, action, actor, details)
}
