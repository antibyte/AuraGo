package tools

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
)

const (
	maxSkillQualityReviewBytes = 96 * 1024
	skillQualityReviewInterval = 30 * 24 * time.Hour
)

// SkillQualityCandidate is the bounded, untrusted input presented to the
// maintenance classifier. ContentComplete must be true for any mutation.
type SkillQualityCandidate struct {
	Kind                string          `json:"kind"`
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Origin              SkillOrigin     `json:"origin"`
	ContentHash         string          `json:"content_hash"`
	Content             string          `json:"content"`
	ContentComplete     bool            `json:"content_complete"`
	Enabled             bool            `json:"enabled"`
	SecurityStatus      SecurityStatus  `json:"security_status"`
	Usage               SkillUsageStats `json:"usage"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
	LastQualityReviewAt *time.Time      `json:"last_quality_review_at,omitempty"`
	LastQualityHash     string          `json:"last_quality_hash,omitempty"`
	HasFixedReference   bool            `json:"has_fixed_reference"`
	IsDaemon            bool            `json:"is_daemon,omitempty"`
	VerifiedDuplicateOf string          `json:"verified_duplicate_of,omitempty"`
}

func qualityCandidateLess(a, b SkillQualityCandidate) bool {
	aChanged := a.LastQualityReviewAt == nil || a.LastQualityHash != a.ContentHash
	bChanged := b.LastQualityReviewAt == nil || b.LastQualityHash != b.ContentHash
	if aChanged != bChanged {
		return aChanged
	}
	if a.LastQualityReviewAt == nil || b.LastQualityReviewAt == nil {
		return a.LastQualityReviewAt == nil
	}
	return a.LastQualityReviewAt.Before(*b.LastQualityReviewAt)
}

// ListPythonQualityCandidates returns only skills with proven agent origin.
func (m *SkillManager) ListPythonQualityCandidates(limit int) ([]SkillQualityCandidate, error) {
	entries, err := m.ListSkillsFiltered("", "", "", nil)
	if err != nil {
		return nil, err
	}
	out := make([]SkillQualityCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.Origin != OriginAgent || entry.Type == SkillTypeBuiltIn {
			continue
		}
		if entry.LastQualityReviewAt != nil && entry.LastQualityHash == entry.FileHash && time.Since(*entry.LastQualityReviewAt) < skillQualityReviewInterval {
			continue
		}
		code, readErr := m.GetSkillCode(entry.ID)
		if readErr != nil {
			continue
		}
		doc, _ := m.GetSkillDocumentation(entry.ID)
		payload, marshalErr := json.Marshal(map[string]any{
			"name": entry.Name, "description": entry.Description, "parameters": entry.Parameters,
			"dependencies": entry.Dependencies, "vault_keys": entry.VaultKeys, "internal_tools": entry.InternalTools,
			"code": code, "documentation": doc,
		})
		if marshalErr != nil {
			continue
		}
		complete := len(payload) <= maxSkillQualityReviewBytes
		content := string(payload)
		if !complete {
			content = ""
		}
		out = append(out, SkillQualityCandidate{
			Kind: "python", ID: entry.ID, Name: entry.Name, Origin: entry.Origin, ContentHash: entry.FileHash,
			Content: content, ContentComplete: complete, Enabled: entry.Enabled, SecurityStatus: entry.SecurityStatus,
			Usage: entry.Usage, CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
			LastQualityReviewAt: entry.LastQualityReviewAt, LastQualityHash: entry.LastQualityHash,
			HasFixedReference: len(entry.CheatsheetIDs) > 0, IsDaemon: entry.IsDaemon,
		})
	}
	markVerifiedPythonDuplicates(out, entries)
	sort.SliceStable(out, func(i, j int) bool { return qualityCandidateLess(out[i], out[j]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func markVerifiedPythonDuplicates(candidates []SkillQualityCandidate, entries []SkillRegistryEntry) {
	for i := range candidates {
		var sourceContract string
		for _, source := range entries {
			if source.ID == candidates[i].ID {
				sourceContract = pythonSkillContractFingerprint(source)
				break
			}
		}
		for _, replacement := range entries {
			if replacement.ID == candidates[i].ID || replacement.FileHash == "" || replacement.FileHash != candidates[i].ContentHash {
				continue
			}
			if !replacement.Enabled || replacement.SecurityStatus != SecurityClean || pythonSkillContractFingerprint(replacement) != sourceContract {
				continue
			}
			candidates[i].VerifiedDuplicateOf = replacement.Name
			break
		}
	}
}

func pythonSkillContractFingerprint(entry SkillRegistryEntry) string {
	payload, _ := json.Marshal(struct {
		Parameters    map[string]interface{} `json:"parameters"`
		Dependencies  []string               `json:"dependencies"`
		VaultKeys     []string               `json:"vault_keys"`
		InternalTools []string               `json:"internal_tools"`
		IsDaemon      bool                   `json:"is_daemon"`
	}{entry.Parameters, entry.Dependencies, entry.VaultKeys, entry.InternalTools, entry.IsDaemon})
	return string(payload)
}

// ListAgentSkillQualityCandidates returns only Agent Skill packages with
// proven agent origin.
func (m *AgentSkillManager) ListAgentSkillQualityCandidates(limit int) ([]SkillQualityCandidate, error) {
	entries, err := m.ListAgentSkills(false, "")
	if err != nil {
		return nil, err
	}
	out := make([]SkillQualityCandidate, 0, len(entries))
	for _, entry := range entries {
		if entry.Origin != OriginAgent {
			continue
		}
		if entry.LastQualityReviewAt != nil && entry.LastQualityHash == entry.PackageHash && time.Since(*entry.LastQualityReviewAt) < skillQualityReviewInterval {
			continue
		}
		pkg, parseErr := ParseAgentSkillPackage(entry.Directory)
		if parseErr != nil {
			out = append(out, SkillQualityCandidate{
				Kind: "agent_skill", ID: entry.ID, Name: entry.Name, Origin: entry.Origin, ContentHash: entry.PackageHash,
				ContentComplete: false, Enabled: entry.Enabled, SecurityStatus: entry.SecurityStatus, Usage: entry.Usage,
				CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt, LastQualityReviewAt: entry.LastQualityReviewAt,
				LastQualityHash: entry.LastQualityHash,
			})
			continue
		}
		content := buildAgentSkillGuardianText(pkg)
		complete := len(content) <= maxSkillQualityReviewBytes
		if !complete {
			content = ""
		}
		out = append(out, SkillQualityCandidate{
			Kind: "agent_skill", ID: entry.ID, Name: entry.Name, Origin: entry.Origin, ContentHash: pkg.PackageHash,
			Content: content, ContentComplete: complete, Enabled: entry.Enabled, SecurityStatus: entry.SecurityStatus,
			Usage: entry.Usage, CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
			LastQualityReviewAt: entry.LastQualityReviewAt, LastQualityHash: entry.LastQualityHash,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return qualityCandidateLess(out[i], out[j]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func insertQualityAction(execer interface {
	Exec(string, ...any) (sql.Result, error)
}, action SkillQualityAction) error {
	if action.Actor == "" {
		action.Actor = "maintenance"
	}
	_, err := execer.Exec(`INSERT INTO skill_quality_maintenance_log
		(skill_kind, skill_id, skill_name, content_hash, origin, verdict, confidence, decision, reason, details, actor)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, action.SkillKind, action.SkillID, action.SkillName, action.ContentHash,
		string(action.Origin), action.Verdict, action.Confidence, action.Decision, action.Reason, action.Details, action.Actor)
	return err
}

func recordQualityReview(db *sql.DB, table string, candidate SkillQualityCandidate, verdict string, confidence float64, decision, reason, details string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if decision != "review_failed" {
		if _, err := tx.Exec("UPDATE "+table+" SET last_quality_review_at = CURRENT_TIMESTAMP, last_quality_verdict = ?, last_quality_confidence = ?, last_quality_hash = ? WHERE id = ?",
			verdict, confidence, candidate.ContentHash, candidate.ID); err != nil {
			return err
		}
	}
	if err := insertQualityAction(tx, SkillQualityAction{
		SkillKind: candidate.Kind, SkillID: candidate.ID, SkillName: candidate.Name, ContentHash: candidate.ContentHash,
		Origin: candidate.Origin, Verdict: verdict, Confidence: confidence, Decision: decision, Reason: reason, Details: details,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *SkillManager) RecordPythonQualityReview(candidate SkillQualityCandidate, verdict string, confidence float64, decision, reason string) error {
	return recordQualityReview(m.db, "skills_registry", candidate, verdict, confidence, decision, reason, "")
}

func (m *AgentSkillManager) RecordAgentSkillQualityReview(candidate SkillQualityCandidate, verdict string, confidence float64, decision, reason string) error {
	return recordQualityReview(m.db, "agent_skills_registry", candidate, verdict, confidence, decision, reason, "")
}

// ApplyPythonSkillQualityRevision validates in isolation, atomically swaps the
// code, and records a maintenance-owned version and quality action.
func (m *SkillManager) ApplyPythonSkillQualityRevision(ctx context.Context, candidate SkillQualityCandidate, revisedCode string, confidence float64, reason string, guardian *security.LLMGuardian, useGuardian, useVirusTotal bool, virusTotalAPIKey string, skillSpector SkillSpectorConfig) error {
	if confidence < MinimumSkillImproveConfidence {
		return fmt.Errorf("quality confidence is below the automatic improvement threshold")
	}
	m.qualityMutationMu.Lock()
	defer m.qualityMutationMu.Unlock()
	entry, err := m.GetSkill(candidate.ID)
	if err != nil {
		return err
	}
	if entry.Origin != OriginAgent || entry.FileHash != candidate.ContentHash || !candidate.ContentComplete {
		return fmt.Errorf("skill provenance or content changed during review")
	}
	currentCode, err := m.GetSkillCode(entry.ID)
	if err != nil {
		return err
	}
	currentHash := sha256.Sum256([]byte(currentCode))
	if hex.EncodeToString(currentHash[:]) != candidate.ContentHash {
		return fmt.Errorf("skill source changed on disk during review")
	}
	if len(entry.CheatsheetIDs) > 0 {
		return fmt.Errorf("skill has a fixed cheat-sheet reference")
	}
	report, err := m.validatePythonQualityRevision(ctx, entry, revisedCode, guardian, useGuardian, useVirusTotal, virusTotalAPIKey, skillSpector)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	codePath := filepath.Join(m.skillsDir, entry.Executable)
	if _, err := os.Stat(codePath); err != nil {
		return err
	}
	stagedPath := codePath + fmt.Sprintf(".maintenance-%d", time.Now().UnixNano())
	backupPath := stagedPath + ".rollback"
	if err := os.WriteFile(stagedPath, []byte(revisedCode), 0o640); err != nil {
		return err
	}
	defer os.Remove(stagedPath)
	if err := os.Rename(codePath, backupPath); err != nil {
		return fmt.Errorf("stage original skill: %w", err)
	}
	restore := func() {
		_ = os.Remove(codePath)
		_ = os.Rename(backupPath, codePath)
	}
	if err := os.Rename(stagedPath, codePath); err != nil {
		restore()
		return fmt.Errorf("activate staged skill: %w", err)
	}
	newHashBytes := sha256.Sum256([]byte(revisedCode))
	newHash := hex.EncodeToString(newHashBytes[:])
	reportJSON, _ := json.Marshal(report)
	tx, err := m.db.Begin()
	if err != nil {
		restore()
		return err
	}
	defer tx.Rollback()
	var nextVersion int
	if err := tx.QueryRow("SELECT COALESCE(MAX(version_num),0)+1 FROM skill_versions WHERE skill_id = ?", entry.ID).Scan(&nextVersion); err != nil {
		restore()
		return err
	}
	if _, err := tx.Exec(`INSERT INTO skill_versions (skill_id, version_num, code_hash, code, created_by, change_note) VALUES (?, ?, ?, ?, 'maintenance', ?)`, entry.ID, nextVersion, newHash, revisedCode, reason); err != nil {
		restore()
		return err
	}
	if _, err := tx.Exec(`UPDATE skills_registry SET file_hash = ?, security_status = ?, security_report = ?, last_scan_at = CURRENT_TIMESTAMP,
		updated_at = CURRENT_TIMESTAMP, last_quality_review_at = CURRENT_TIMESTAMP, last_quality_verdict = 'improved',
		last_quality_confidence = ?, last_quality_hash = ?, enabled = ? WHERE id = ?`, newHash, string(SecurityClean), string(reportJSON), confidence, newHash, boolToInt(entry.Enabled), entry.ID); err != nil {
		restore()
		return err
	}
	if _, err := tx.Exec(`INSERT INTO skill_audit_log (skill_id, skill_name, action, actor, details) VALUES (?, ?, 'quality_improved', 'maintenance', ?)`, entry.ID, entry.Name, reason); err != nil {
		restore()
		return err
	}
	if err := insertQualityAction(tx, SkillQualityAction{SkillKind: "python", SkillID: entry.ID, SkillName: entry.Name, ContentHash: newHash, Origin: OriginAgent, Verdict: "improve", Confidence: confidence, Decision: "improved", Reason: reason}); err != nil {
		restore()
		return err
	}
	if err := tx.Commit(); err != nil {
		restore()
		return err
	}
	_ = os.Remove(backupPath)
	InvalidateSkillsCache(m.skillsDir)
	return nil
}

func (m *SkillManager) validatePythonQualityRevision(ctx context.Context, entry *SkillRegistryEntry, code string, guardian *security.LLMGuardian, useGuardian, useVirusTotal bool, virusTotalAPIKey string, skillSpector SkillSpectorConfig) (*SecurityReport, error) {
	if err := validateSkillCode(code); err != nil {
		return nil, err
	}
	stageDir, err := os.MkdirTemp(m.skillsDir, ".quality-review-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stageDir)
	stageCode := filepath.Join(stageDir, filepath.Base(entry.Executable))
	if err := os.WriteFile(stageCode, []byte(code), 0o600); err != nil {
		return nil, err
	}
	python := GetPythonBin("")
	if _, statErr := os.Stat(python); statErr != nil {
		python = findSystemPython()
	}
	if python == "" {
		return nil, fmt.Errorf("python is unavailable for syntax validation")
	}
	if output, compileErr := exec.CommandContext(ctx, python, "-m", "py_compile", stageCode).CombinedOutput(); compileErr != nil {
		return nil, fmt.Errorf("python syntax validation failed: %s", strings.TrimSpace(string(output)))
	}
	originalCode, err := m.GetSkillCode(entry.ID)
	if err != nil {
		return nil, err
	}
	originalPath := filepath.Join(stageDir, "original.py")
	if err := os.WriteFile(originalPath, []byte(originalCode), 0o600); err != nil {
		return nil, err
	}
	originalContract, err := extractPythonCallableContract(ctx, python, originalPath)
	if err != nil {
		return nil, fmt.Errorf("original Python callable contract is not verifiable: %w", err)
	}
	revisedContract, err := extractPythonCallableContract(ctx, python, stageCode)
	if err != nil {
		return nil, fmt.Errorf("revised Python callable contract is not verifiable: %w", err)
	}
	if originalContract != revisedContract {
		return nil, fmt.Errorf("revised Python callable contract changed")
	}
	report := &SecurityReport{StaticAnalysis: StaticCodeAnalysis(code), ScannedAt: time.Now().UTC()}
	if useVirusTotal {
		if strings.TrimSpace(virusTotalAPIKey) == "" {
			return nil, fmt.Errorf("VirusTotal scan requested but API key is unavailable")
		}
		hash := sha256.Sum256([]byte(code))
		report.VirusTotalReport = ExecuteVirusTotalScan(virusTotalAPIKey, hex.EncodeToString(hash[:]))
		clean, vtErr := virusTotalMaintenanceResultClean(report.VirusTotalReport)
		if vtErr != nil {
			return nil, vtErr
		}
		if !clean {
			report.VirusTotalScore = 1
		}
	}
	if useGuardian {
		if guardian == nil {
			return nil, fmt.Errorf("guardian scan requested but unavailable")
		}
		result := guardian.EvaluateContent(ctx, "python_skill_maintenance", code)
		report.GuardianScore = result.RiskScore
		report.GuardianVerdict = string(result.Decision)
		report.GuardianReason = result.Reason
	}
	if skillSpector.Enabled {
		ssReport, _, scanErr := RunSkillSpectorScan(ctx, stageDir, skillSpector)
		report.SkillSpector = ssReport
		if scanErr != nil {
			return nil, fmt.Errorf("SkillSpector validation failed: %w", scanErr)
		}
	}
	if status := DetermineSecurityStatus(report); status != SecurityClean {
		return nil, fmt.Errorf("revised skill did not pass clean security validation: %s", status)
	}
	return report, nil
}

func virusTotalMaintenanceResultClean(raw string) (bool, error) {
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false, fmt.Errorf("VirusTotal returned an invalid response")
	}
	var inspect func(any) (bool, bool)
	inspect = func(value any) (flagged, failed bool) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				lowerKey := strings.ToLower(key)
				if lowerKey == "status" {
					if status, ok := child.(string); ok && strings.EqualFold(status, "error") {
						failed = true
					}
				}
				if lowerKey == "malicious" || lowerKey == "suspicious" || lowerKey == "positives" {
					if score, ok := child.(float64); ok && score > 0 {
						flagged = true
					}
				}
				childFlagged, childFailed := inspect(child)
				flagged = flagged || childFlagged
				failed = failed || childFailed
			}
		case []any:
			for _, child := range typed {
				childFlagged, childFailed := inspect(child)
				flagged = flagged || childFlagged
				failed = failed || childFailed
			}
		}
		return flagged, failed
	}
	flagged, failed := inspect(payload)
	if failed {
		return false, fmt.Errorf("VirusTotal validation failed")
	}
	return !flagged, nil
}

func extractPythonCallableContract(ctx context.Context, python, path string) (string, error) {
	const script = `import ast,json,sys
tree=ast.parse(open(sys.argv[1],encoding="utf-8").read())
out=[]
for n in tree.body:
    if isinstance(n,(ast.FunctionDef,ast.AsyncFunctionDef)):
        a=n.args
        out.append({"name":n.name,"async":isinstance(n,ast.AsyncFunctionDef),"posonly":[x.arg for x in a.posonlyargs],"args":[x.arg for x in a.args],"vararg":a.vararg.arg if a.vararg else "","kwonly":[x.arg for x in a.kwonlyargs],"kwarg":a.kwarg.arg if a.kwarg else "","defaults":len(a.defaults),"kwdefaults":[x is not None for x in a.kw_defaults],"returns":ast.unparse(n.returns) if n.returns else ""})
print(json.dumps(out,sort_keys=True,separators=(",",":")))`
	output, err := exec.CommandContext(ctx, python, "-c", script, path).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// DeletePythonSkillForMaintenance permanently removes an agent-created skill
// and its version history while retaining only the quality tombstone.
func (m *SkillManager) DeletePythonSkillForMaintenance(candidate SkillQualityCandidate, confidence float64, reason string) error {
	if confidence < MinimumSkillDeleteConfidence {
		return fmt.Errorf("quality confidence is below the automatic deletion threshold")
	}
	m.qualityMutationMu.Lock()
	defer m.qualityMutationMu.Unlock()
	entry, err := m.GetSkill(candidate.ID)
	if err != nil {
		return err
	}
	if entry.Origin != OriginAgent || entry.FileHash != candidate.ContentHash || len(entry.CheatsheetIDs) > 0 {
		return fmt.Errorf("skill is not eligible for automatic deletion")
	}
	currentCode, err := m.GetSkillCode(entry.ID)
	if err != nil {
		return err
	}
	currentHash := sha256.Sum256([]byte(currentCode))
	if hex.EncodeToString(currentHash[:]) != candidate.ContentHash {
		return fmt.Errorf("skill source changed on disk during review")
	}
	stageDir, err := os.MkdirTemp(m.skillsDir, ".quality-delete-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)
	paths := []string{filepath.Join(m.skillsDir, entry.Executable), filepath.Join(m.skillsDir, strings.TrimSuffix(entry.Executable, filepath.Ext(entry.Executable))+".json")}
	if doc := SkillDocumentationFilename(entry.Executable); doc != "" {
		paths = append(paths, filepath.Join(m.skillsDir, doc))
	}
	moved := make([][2]string, 0, len(paths))
	for _, source := range paths {
		if _, statErr := os.Stat(source); os.IsNotExist(statErr) {
			continue
		}
		dest := filepath.Join(stageDir, filepath.Base(source))
		if err := os.Rename(source, dest); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = os.Rename(moved[i][1], moved[i][0])
			}
			return err
		}
		moved = append(moved, [2]string{source, dest})
	}
	restore := func() {
		for i := len(moved) - 1; i >= 0; i-- {
			_ = os.Rename(moved[i][1], moved[i][0])
		}
	}
	tx, err := m.db.Begin()
	if err != nil {
		restore()
		return err
	}
	defer tx.Rollback()
	if err := insertQualityAction(tx, SkillQualityAction{SkillKind: "python", SkillID: entry.ID, SkillName: entry.Name, ContentHash: entry.FileHash, Origin: OriginAgent, Verdict: "delete", Confidence: confidence, Decision: "deleted", Reason: reason}); err != nil {
		restore()
		return err
	}
	if _, err := tx.Exec("DELETE FROM skills_registry WHERE id = ?", entry.ID); err != nil {
		restore()
		return err
	}
	if err := tx.Commit(); err != nil {
		restore()
		return err
	}
	if err := os.RemoveAll(stageDir); err != nil {
		return fmt.Errorf("remove deleted skill files: %w", err)
	}
	InvalidateSkillsCache(m.skillsDir)
	return nil
}

// ApplyAgentSkillQualityRevision validates a complete staging copy and only
// permits edits to existing UTF-8 files. Package paths and allowed tools cannot
// expand.
func (m *AgentSkillManager) ApplyAgentSkillQualityRevision(ctx context.Context, candidate SkillQualityCandidate, revisedFiles map[string]string, confidence float64, reason string, guardian *security.LLMGuardian, useGuardian bool, skillSpector SkillSpectorConfig) error {
	if confidence < MinimumSkillImproveConfidence {
		return fmt.Errorf("quality confidence is below the automatic improvement threshold")
	}
	m.qualityMutationMu.Lock()
	defer m.qualityMutationMu.Unlock()
	entry, err := m.GetAgentSkill(candidate.ID)
	if err != nil {
		return err
	}
	if entry.Origin != OriginAgent || entry.PackageHash != candidate.ContentHash || !candidate.ContentComplete || len(revisedFiles) == 0 {
		return fmt.Errorf("Agent Skill provenance or content changed during review")
	}
	original, err := ParseAgentSkillPackage(entry.Directory)
	if err != nil {
		return err
	}
	if original.PackageHash != candidate.ContentHash {
		return fmt.Errorf("Agent Skill package changed on disk during review")
	}
	stageBase, err := os.MkdirTemp(m.agentSkillsDir, ".quality-review-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageBase)
	stageDir := filepath.Join(stageBase, entry.Name)
	if err := copyAgentSkillDirectory(entry.Directory, stageDir); err != nil {
		return err
	}
	for rel, content := range revisedFiles {
		rel = filepath.ToSlash(filepath.Clean(strings.TrimSpace(rel)))
		if rel == "." || filepath.IsAbs(rel) || strings.HasPrefix(rel, "../") || strings.Contains(rel, ":") {
			return fmt.Errorf("invalid revised Agent Skill path %q", rel)
		}
		target := filepath.Join(stageDir, filepath.FromSlash(rel))
		info, statErr := os.Lstat(target)
		if statErr != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("maintenance cannot add Agent Skill files: %s", rel)
		}
		current, readErr := os.ReadFile(target)
		if readErr != nil || !utf8.Valid(current) {
			return fmt.Errorf("maintenance can only revise existing UTF-8 files: %s", rel)
		}
		if len(content) > maxAgentSkillMarkdownBytes || !utf8.ValidString(content) {
			return fmt.Errorf("invalid revised Agent Skill content: %s", rel)
		}
		if err := os.WriteFile(target, []byte(content), info.Mode().Perm()); err != nil {
			return err
		}
	}
	staged, err := ParseAgentSkillPackage(stageDir)
	if err != nil {
		return err
	}
	if staged.Name != original.Name || staged.AllowedTools != original.AllowedTools || !sameAgentSkillResourcePaths(staged, original) {
		return fmt.Errorf("Agent Skill contract or resource paths changed")
	}
	report, status, scanErr := ScanAgentSkillPackage(ctx, staged, guardian, useGuardian, skillSpector)
	if scanErr != nil {
		return scanErr
	}
	if status != SecurityClean {
		return fmt.Errorf("revised Agent Skill did not pass clean security validation: %s", status)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	backupDir := entry.Directory + fmt.Sprintf(".maintenance-%d", time.Now().UnixNano())
	if err := os.Rename(entry.Directory, backupDir); err != nil {
		return err
	}
	restore := func() { _ = os.RemoveAll(entry.Directory); _ = os.Rename(backupDir, entry.Directory) }
	if err := os.Rename(stageDir, entry.Directory); err != nil {
		restore()
		return err
	}
	installed, err := ParseAgentSkillPackage(entry.Directory)
	if err != nil {
		restore()
		return err
	}
	resourcesJSON, _ := json.Marshal(installed.Resources)
	scriptsJSON, _ := json.Marshal(installed.Scripts)
	agentsJSON, _ := json.Marshal(installed.Agents)
	metadataJSON, _ := json.Marshal(installed.Metadata)
	reportJSON, _ := json.Marshal(report)
	snapshot := buildAgentSkillGuardianText(installed)
	tx, err := m.db.Begin()
	if err != nil {
		restore()
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE agent_skills_registry SET description = ?, license = ?, compatibility = ?, metadata = ?, allowed_tools = ?,
		directory = ?, skill_path = ?, resources = ?, scripts = ?, agents = ?, enabled = ?, warning_approved = 0,
		security_status = ?, security_report = ?, package_hash = ?, updated_at = CURRENT_TIMESTAMP,
		last_quality_review_at = CURRENT_TIMESTAMP, last_quality_verdict = 'improved', last_quality_confidence = ?, last_quality_hash = ?
		WHERE id = ?`, installed.Description, installed.License, installed.Compatibility, string(metadataJSON), installed.AllowedTools,
		installed.Directory, installed.SkillPath, string(resourcesJSON), string(scriptsJSON), string(agentsJSON), boolInt(entry.Enabled),
		string(SecurityClean), string(reportJSON), installed.PackageHash, confidence, installed.PackageHash, entry.ID); err != nil {
		restore()
		return err
	}
	if _, err := tx.Exec(`INSERT INTO agent_skill_versions (skill_id, version_num, package_hash, package_snapshot, created_by, change_note)
		VALUES (?, (SELECT COALESCE(MAX(version_num),0)+1 FROM agent_skill_versions WHERE skill_id = ?), ?, ?, 'maintenance', ?)`, entry.ID, entry.ID, installed.PackageHash, snapshot, reason); err != nil {
		restore()
		return err
	}
	if _, err := tx.Exec(`INSERT INTO agent_skill_scan_history (skill_id, scanner_type, score, verdict, details) VALUES (?, 'combined', ?, ?, ?)`, entry.ID, report.OverallScore, string(SecurityClean), string(reportJSON)); err != nil {
		restore()
		return err
	}
	if _, err := tx.Exec(`INSERT INTO agent_skill_audit_log (skill_id, skill_name, action, actor, details) VALUES (?, ?, 'quality_improved', 'maintenance', ?)`, entry.ID, entry.Name, reason); err != nil {
		restore()
		return err
	}
	if err := insertQualityAction(tx, SkillQualityAction{SkillKind: "agent_skill", SkillID: entry.ID, SkillName: entry.Name, ContentHash: installed.PackageHash, Origin: OriginAgent, Verdict: "improve", Confidence: confidence, Decision: "improved", Reason: reason}); err != nil {
		restore()
		return err
	}
	if err := tx.Commit(); err != nil {
		restore()
		return err
	}
	_ = os.RemoveAll(backupDir)
	return nil
}

func sameAgentSkillResourcePaths(a, b *AgentSkillPackage) bool {
	paths := func(pkg *AgentSkillPackage) []string {
		out := []string{"SKILL.md"}
		for _, group := range [][]AgentSkillResource{pkg.Resources, pkg.Scripts, pkg.Agents} {
			for _, resource := range group {
				out = append(out, resource.Path)
			}
		}
		sort.Strings(out)
		return out
	}
	return strings.Join(paths(a), "\n") == strings.Join(paths(b), "\n")
}

// DeleteAgentSkillForMaintenance permanently deletes package files, registry,
// versions, and ordinary audits while preserving the quality tombstone.
func (m *AgentSkillManager) DeleteAgentSkillForMaintenance(candidate SkillQualityCandidate, confidence float64, reason string) error {
	if confidence < MinimumSkillDeleteConfidence {
		return fmt.Errorf("quality confidence is below the automatic deletion threshold")
	}
	m.qualityMutationMu.Lock()
	defer m.qualityMutationMu.Unlock()
	entry, err := m.GetAgentSkill(candidate.ID)
	if err != nil {
		return err
	}
	if entry.Origin != OriginAgent || entry.PackageHash != candidate.ContentHash {
		return fmt.Errorf("Agent Skill is not eligible for automatic deletion")
	}
	currentPackage, err := ParseAgentSkillPackage(entry.Directory)
	if err != nil {
		return err
	}
	if currentPackage.PackageHash != candidate.ContentHash {
		return fmt.Errorf("Agent Skill package changed on disk during review")
	}
	stageDir := entry.Directory + fmt.Sprintf(".quality-delete-%d", time.Now().UnixNano())
	if err := os.Rename(entry.Directory, stageDir); err != nil {
		return err
	}
	restore := func() { _ = os.Rename(stageDir, entry.Directory) }
	tx, err := m.db.Begin()
	if err != nil {
		restore()
		return err
	}
	defer tx.Rollback()
	if err := insertQualityAction(tx, SkillQualityAction{SkillKind: "agent_skill", SkillID: entry.ID, SkillName: entry.Name, ContentHash: entry.PackageHash, Origin: OriginAgent, Verdict: "delete", Confidence: confidence, Decision: "deleted", Reason: reason}); err != nil {
		restore()
		return err
	}
	if _, err := tx.Exec("DELETE FROM agent_skills_registry WHERE id = ?", entry.ID); err != nil {
		restore()
		return err
	}
	if err := tx.Commit(); err != nil {
		restore()
		return err
	}
	if err := os.RemoveAll(stageDir); err != nil {
		return fmt.Errorf("remove deleted Agent Skill files: %w", err)
	}
	return nil
}
