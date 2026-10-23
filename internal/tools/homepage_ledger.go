package tools

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/security"
)

const homepageArtifactManifestName = ".aurago-site-manifest.json"

// HomepageProjectState is the ledger's current local/remote status summary.
type HomepageProjectState struct {
	ProjectID         int64  `json:"project_id"`
	LocalRoot         string `json:"local_root"`
	CurrentRevisionID int64  `json:"current_revision_id,omitempty"`
	GitSHA            string `json:"git_sha,omitempty"`
	LastReconciledAt  string `json:"last_reconciled_at,omitempty"`
	DriftStatus       string `json:"drift_status"`
	DriftMessage      string `json:"drift_message,omitempty"`
}

// HomepageLedgerResult captures a mutation bookkeeping result.
type HomepageLedgerResult struct {
	JSON       string   `json:"json"`
	Warnings   []string `json:"warnings,omitempty"`
	RevisionID int64    `json:"revision_id,omitempty"`
	Changed    bool     `json:"changed"`
}

type HomepageDeploymentLedgerResult struct {
	Warnings []string
	Fatal    error
}

// HomepageDeploymentRecord is the normalized deployment row written by the ledger.
type HomepageDeploymentRecord struct {
	ProjectID        int64                  `json:"project_id"`
	RevisionID       int64                  `json:"revision_id,omitempty"`
	GitSHA           string                 `json:"git_sha,omitempty"`
	Provider         string                 `json:"provider"`
	ProviderTargetID string                 `json:"provider_target_id,omitempty"`
	ProviderDeployID string                 `json:"provider_deploy_id,omitempty"`
	URL              string                 `json:"url,omitempty"`
	RemotePath       string                 `json:"remote_path,omitempty"`
	BuildDir         string                 `json:"build_dir,omitempty"`
	ArtifactHash     string                 `json:"artifact_hash,omitempty"`
	Status           string                 `json:"status,omitempty"`
	VerificationJSON string                 `json:"verification_json,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// HomepageArtifactManifest links a build artifact to a source revision.
type HomepageArtifactManifest struct {
	GeneratedAt  string                         `json:"generated_at"`
	ProjectID    int64                          `json:"project_id"`
	ProjectDir   string                         `json:"project_dir"`
	RevisionID   int64                          `json:"revision_id,omitempty"`
	GitSHA       string                         `json:"git_sha,omitempty"`
	BuildDir     string                         `json:"build_dir"`
	ArtifactHash string                         `json:"artifact_hash"`
	Files        []HomepageArtifactManifestFile `json:"files"`
	ManifestPath string                         `json:"-"`
}

// HomepageArtifactManifestFile describes one file in a deployed build output.
type HomepageArtifactManifestFile struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// HomepageManagedSite is the compact read model exposed to APIs and agent context.
type HomepageManagedSite struct {
	LocalStatus    string              `json:"local_status"`
	WorkspaceRoot  string              `json:"workspace_root"`
	ContainerPath  string              `json:"container_path"`
	Origin         string              `json:"origin"`
	CreationRunID  string              `json:"creation_run_id,omitempty"`
	LastEditReason string              `json:"last_edit_reason,omitempty"`
	Events         []HomepageSiteEvent `json:"events,omitempty"`

	ID                 int64                              `json:"id"`
	Name               string                             `json:"name"`
	Framework          string                             `json:"framework,omitempty"`
	URL                string                             `json:"url,omitempty"`
	ProjectDir         string                             `json:"project_dir"`
	Status             string                             `json:"status"`
	LastEditedAt       string                             `json:"last_edited_at,omitempty"`
	LastDeployedAt     string                             `json:"last_deployed_at,omitempty"`
	LastDeployURL      string                             `json:"last_deploy_url,omitempty"`
	LocalRoot          string                             `json:"local_root,omitempty"`
	CurrentRevisionID  int64                              `json:"current_revision_id,omitempty"`
	GitSHA             string                             `json:"git_sha,omitempty"`
	LastReconciledAt   string                             `json:"last_reconciled_at,omitempty"`
	DriftStatus        string                             `json:"drift_status"`
	DriftMessage       string                             `json:"drift_message,omitempty"`
	Deployments        []HomepageManagedDeployment        `json:"deployments,omitempty"`
	DeployTargets      []HomepageManagedDeployTarget      `json:"deploy_targets,omitempty"`
	RemoteObservations []HomepageManagedRemoteObservation `json:"remote_observations,omitempty"`
}

// HomepageManagedDeployment is the compact deployment read model.
type HomepageManagedDeployment struct {
	ID               int64  `json:"id"`
	CreatedAt        string `json:"created_at"`
	RevisionID       int64  `json:"revision_id,omitempty"`
	GitSHA           string `json:"git_sha,omitempty"`
	Provider         string `json:"provider"`
	ProviderDeployID string `json:"provider_deploy_id,omitempty"`
	URL              string `json:"url,omitempty"`
	BuildDir         string `json:"build_dir,omitempty"`
	ArtifactHash     string `json:"artifact_hash,omitempty"`
	Status           string `json:"status"`
}

// HomepageManagedDeployTarget is the compact current remote target read model.
type HomepageManagedDeployTarget struct {
	Status      string `json:"status"`
	Account     string `json:"account,omitempty"`
	Environment string `json:"environment,omitempty"`

	ID               int64  `json:"id"`
	Provider         string `json:"provider"`
	ProviderTargetID string `json:"provider_target_id,omitempty"`
	URL              string `json:"url,omitempty"`
	RemotePath       string `json:"remote_path,omitempty"`
	LastSeenAt       string `json:"last_seen_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

// HomepageManagedRemoteObservation is the compact remote observation read model.
type HomepageManagedRemoteObservation struct {
	ID               int64  `json:"id"`
	TargetID         int64  `json:"target_id,omitempty"`
	ObservedAt       string `json:"observed_at"`
	Provider         string `json:"provider"`
	URL              string `json:"url,omitempty"`
	ProviderDeployID string `json:"provider_deploy_id,omitempty"`
	ContentHash      string `json:"content_hash,omitempty"`
	Status           string `json:"status"`
}

// EnsureHomepageProjectForDir returns a project for projectDir, creating or repairing
// registry/project-state rows as needed. projectDir is stored relative to WorkspacePath.
func EnsureHomepageProjectForDir(db *sql.DB, cfg HomepageConfig, projectDir, name, framework string) (*HomepageProject, error) {
	if db == nil {
		return nil, fmt.Errorf("homepage registry DB not initialized")
	}
	canonical, err := canonicalHomepageProjectDir(cfg, projectDir)
	if err != nil {
		return nil, err
	}
	if canonical, err = NormalizeHomepageProjectIdentity(canonical, false); err != nil {
		return nil, err
	}
	if proj, err := GetProjectByDir(db, canonical); err == nil {
		if err := bindHomepageProjectWorkspace(db, cfg, proj.ID, canonical); err != nil {
			return nil, err
		}
		if err := ensureHomepageProjectState(db, cfg, proj.ID, canonical, 0, "", "", ""); err != nil {
			return nil, err
		}
		return proj, nil
	}

	if strings.TrimSpace(name) == "" {
		name = homepageProjectNameFromDir(canonical)
	}
	id, existed, err := RegisterProject(db, HomepageProject{
		Name:       name,
		Framework:  framework,
		ProjectDir: canonical,
		Status:     "active",
		Tags:       []string{"auto-registered"},
	})
	if err != nil {
		return nil, err
	}
	if existed {
		current, getErr := GetProject(db, id)
		if getErr != nil {
			return nil, getErr
		}
		if current.ProjectDir != canonical {
			return nil, fmt.Errorf("homepage project identity mismatch: existing project %d has project_dir %q, requested %q", id, current.ProjectDir, canonical)
		}
		if framework != "" && current.Framework == "" {
			if updateErr := UpdateProject(db, id, map[string]interface{}{"framework": framework}); updateErr != nil {
				return nil, updateErr
			}
		}
	}
	if err := bindHomepageProjectWorkspace(db, cfg, id, canonical); err != nil {
		return nil, err
	}
	if err := ensureHomepageProjectState(db, cfg, id, canonical, 0, "", "", ""); err != nil {
		return nil, err
	}
	return GetProject(db, id)
}

// RecordHomepageEvent appends one structured ledger event.
func RecordHomepageEvent(db *sql.DB, projectID int64, eventType, source, summary string, payload map[string]interface{}) (int64, error) {
	if db == nil || projectID <= 0 {
		return 0, fmt.Errorf("homepage registry and project_id are required")
	}
	if eventType == "" {
		eventType = "note"
	}
	if summary == "" {
		summary = eventType
	}
	actor := ledgerString(payload, "actor")
	if actor == "" {
		actor = "system"
	}
	correlation := ledgerString(payload, "operation_id")
	if correlation == "" {
		correlation = "hp-" + rand.Text()
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	encodedText := security.RedactSensitiveInfo(string(encoded))
	if !json.Valid([]byte(encodedText)) {
		return 0, fmt.Errorf("redacted homepage event is not valid JSON")
	}
	result, err := db.Exec(`INSERT INTO homepage_site_events (project_id,actor,source,event_type,summary,correlation_id,payload_json)
        VALUES (?,?,?,?,?,?,?)`, projectID, actor, source, eventType, security.RedactSensitiveInfo(summary), correlation, encodedText)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func SnapshotHomepageFileState(cfg HomepageConfig, db *sql.DB, projectID int64, projectDir string, revisionID int64, logger *slog.Logger) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	if projectID <= 0 {
		return fmt.Errorf("project_id is required")
	}
	canonical, err := canonicalHomepageProjectDir(cfg, projectDir)
	if err != nil {
		return err
	}
	basePath := homepageLocalRoot(cfg, canonical)
	files, _, err := enumerateProjectFiles(basePath, logger)
	if err != nil {
		return fmt.Errorf("enumerate project files: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM homepage_site_file_state WHERE project_id = ?", projectID); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var nullableRevision interface{}
	if revisionID > 0 {
		nullableRevision = revisionID
	}
	for _, f := range files {
		isBinary := 0
		if f.IsBinary {
			isBinary = 1
		}
		modTime := ""
		if !f.ModTime.IsZero() {
			modTime = f.ModTime.UTC().Format(time.RFC3339)
		}
		if _, err := tx.Exec(`INSERT INTO homepage_site_file_state
			(project_id, rel_path, content_hash, size_bytes, mod_time, is_binary, last_revision_id, last_observed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			projectID, f.Path, f.Hash, f.Size, modTime, isBinary, nullableRevision, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	gitSHA := HomepageProjectGitSHA(cfg, canonical)
	return ensureHomepageProjectState(db, cfg, projectID, canonical, revisionID, gitSHA, "", "")
}

// SaveHomepageRevisionAndState saves a revision, snapshots file state, and logs structured history.
func SaveHomepageRevisionAndState(cfg HomepageConfig, db *sql.DB, projectDir, message, reason, source string, payload map[string]interface{}, logger *slog.Logger) HomepageLedgerResult {
	result := HomepageLedgerResult{}
	if db == nil {
		result.JSON = `{"status":"error","message":"homepage registry DB not initialized"}`
		result.Warnings = append(result.Warnings, "homepage registry DB not initialized")
		return result
	}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "", "")
	if err != nil {
		result.JSON = fmt.Sprintf(`{"status":"error","message":"%s"}`, strings.ReplaceAll(err.Error(), `"`, `'`))
		result.Warnings = append(result.Warnings, err.Error())
		return result
	}
	if actor := ledgerString(payload, "actor"); actor != "" {
		op := homepageCurrentOperation(cfg)
		op.Actor = actor
		cfg.Context = context.WithValue(nativeHTTPContext(cfg.Context), homepageOperationKey{}, op)
	}
	raw := HomepageSaveRevision(cfg, db, proj.ProjectDir, message, reason, logger)
	result.JSON = raw
	parsed := map[string]interface{}{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("revision result was not JSON: %v", err))
		return result
	}
	if status, _ := parsed["status"].(string); status == "error" {
		if msg, _ := parsed["message"].(string); msg != "" {
			result.Warnings = append(result.Warnings, msg)
		}
		return result
	}
	if revNum, ok := parsed["revision_id"].(float64); ok && revNum > 0 {
		result.RevisionID = int64(revNum)
		result.Changed = true
	}
	if result.RevisionID > 0 {
		if err := SnapshotHomepageFileState(cfg, db, proj.ID, proj.ProjectDir, result.RevisionID, logger); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("file state snapshot failed: %v", err))
		}
	}
	if !result.Changed {
		return result
	}
	eventType := "revision_checked"
	if result.Changed {
		eventType = "revision_saved"
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	if op := homepageCurrentOperation(cfg); op.ID != "" {
		if _, ok := payload["operation_id"]; !ok {
			payload["operation_id"] = op.ID
		}
		if _, ok := payload["reason"]; !ok {
			payload["reason"] = op.Reason
		}
		if _, ok := payload["actor"]; !ok {
			payload["actor"] = op.Actor
		}
		payload["run_id"] = op.RunID
		payload["session_id"] = op.SessionID
	}
	payload["revision_id"] = result.RevisionID
	payload["changed"] = result.Changed
	if _, err := RecordHomepageEvent(db, proj.ID, eventType, source, message, payload); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("event logging failed: %v", err))
	}
	if result.Changed && !HomepageOperationInProgress(cfg) {
		_, _ = AddHomepageHistoryEntry(db, proj.ID, "note", fmt.Sprintf("Saved revision: %s", message), source, nil)
	}
	return result
}

// RecordHomepageDeployment persists a normalized deployment target and deployment row.
func RecordHomepageDeployment(db *sql.DB, rec HomepageDeploymentRecord) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	if rec.ProjectID <= 0 {
		return fmt.Errorf("project_id is required")
	}
	rec.Provider = strings.ToLower(strings.TrimSpace(rec.Provider))
	if rec.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	rec.URL = strings.TrimSpace(rec.URL)
	rec.RemotePath = strings.TrimSpace(rec.RemotePath)
	if rec.URL == "" && rec.RemotePath == "" {
		return fmt.Errorf("deployment target could not be recorded; deploy URL or remote_path is required")
	}
	if rec.Status == "" {
		rec.Status = "ok"
	}
	metadataJSON := ""
	if rec.Metadata != nil {
		b, _ := json.Marshal(rec.Metadata)
		metadataJSON = security.RedactSensitiveInfo(string(b))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	targetKey := homepageDeploymentTargetKey(rec)
	// Reuse a migrated target once; never erase its historical foreign-key links.
	if _, err := db.Exec("UPDATE homepage_deploy_targets SET target_key=? WHERE project_id=? AND provider=? AND provider_target_id=? AND target_key LIKE 'legacy:%'", targetKey, rec.ProjectID, rec.Provider, rec.ProviderTargetID); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT INTO homepage_deploy_targets
		(project_id, provider, provider_target_id, url, remote_path, metadata_json, last_seen_at, updated_at,target_key,status)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?,?,?)
        ON CONFLICT(project_id, provider,target_key) DO UPDATE SET
            status=excluded.status,
			provider_target_id = excluded.provider_target_id,
			url = excluded.url,
			remote_path = excluded.remote_path,
			metadata_json = excluded.metadata_json,
			last_seen_at = excluded.last_seen_at,
			updated_at = excluded.updated_at`,
		rec.ProjectID, rec.Provider, rec.ProviderTargetID, rec.URL, rec.RemotePath, metadataJSON, now, now, targetKey, func() string {
			if rec.Status == "ok" || rec.Status == "verified" {
				return "active"
			}
			return "unknown"
		}())
	if err != nil {
		return fmt.Errorf("failed to upsert deploy target: %w", err)
	}
	var targetID int64
	if err := db.QueryRow(
		"SELECT id FROM homepage_deploy_targets WHERE project_id = ? AND provider = ? AND target_key=?",
		rec.ProjectID,
		rec.Provider, targetKey,
	).Scan(&targetID); err != nil {
		return fmt.Errorf("failed to read deploy target id: %w", err)
	}
	var revisionID interface{}
	if rec.RevisionID > 0 {
		revisionID = rec.RevisionID
	}
	var targetIDValue interface{}
	if targetID > 0 {
		targetIDValue = targetID
	}
	if _, err := db.Exec(`INSERT INTO homepage_deployments
		(project_id, target_id, revision_id, git_sha, provider, provider_deploy_id, url, build_dir, artifact_hash, status, verification_json, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ProjectID, targetIDValue, revisionID, rec.GitSHA, rec.Provider, rec.ProviderDeployID, rec.URL, rec.BuildDir, rec.ArtifactHash, rec.Status, rec.VerificationJSON, metadataJSON); err != nil {
		return fmt.Errorf("failed to insert deployment: %w", err)
	}
	if _, err := RecordHomepageEvent(db, rec.ProjectID, "deployed", "homepage_deploy", fmt.Sprintf("Deployed to %s", rec.Provider), map[string]interface{}{
		"provider_deploy_id": rec.ProviderDeployID,
		"url":                rec.URL,
		"build_dir":          rec.BuildDir,
		"artifact_hash":      rec.ArtifactHash,
	}); err != nil {
		return err
	}
	if _, err := db.Exec("UPDATE homepage_projects SET last_deploy_url=?,last_deployed_at=?,updated_at=? WHERE id=?", rec.URL, now, now, rec.ProjectID); err != nil {
		return err
	}
	_, _ = db.Exec("UPDATE homepage_project_state SET drift_status = 'clean', drift_message = '', updated_at = ? WHERE project_id = ?", now, rec.ProjectID)
	return nil
}

// RecordHomepageDeploymentFromResult parses a homepage deploy tool result and records it.
func RecordHomepageDeploymentFromResult(cfg HomepageConfig, db *sql.DB, projectDir, provider, buildDir, rawResult string, logger *slog.Logger) []string {
	result := recordHomepageDeploymentFromResult(cfg, db, projectDir, provider, buildDir, rawResult, logger)
	warnings := append([]string{}, result.Warnings...)
	if result.Fatal != nil {
		warnings = append(warnings, result.Fatal.Error())
	}
	return warnings
}

func RecordHomepageDeploymentFromResultStrict(cfg HomepageConfig, db *sql.DB, projectDir, provider, buildDir, rawResult string, logger *slog.Logger) ([]string, error) {
	result := recordHomepageDeploymentFromResult(cfg, db, projectDir, provider, buildDir, rawResult, logger)
	return result.Warnings, result.Fatal
}

func recordHomepageDeploymentFromResult(cfg HomepageConfig, db *sql.DB, projectDir, provider, buildDir, rawResult string, logger *slog.Logger) HomepageDeploymentLedgerResult {
	result := HomepageDeploymentLedgerResult{}
	if db == nil {
		result.Fatal = fmt.Errorf("homepage registry DB not initialized")
		return result
	}
	parsed := map[string]interface{}{}
	if err := json.Unmarshal([]byte(rawResult), &parsed); err != nil {
		result.Fatal = fmt.Errorf("deployment result was not JSON: %v", err)
		return result
	}
	if status, _ := parsed["status"].(string); status == "error" {
		return result
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "here_now" {
		verified, ok := parsed["verified"].(bool)
		if !ok || !verified {
			return result
		}
	}
	if fallbackProjectDir := ledgerString(parsed, "fallback_project_dir"); fallbackProjectDir != "" {
		projectDir = fallbackProjectDir
	}
	if fallbackBuildDir := ledgerString(parsed, "fallback_build_dir"); fallbackBuildDir != "" {
		buildDir = fallbackBuildDir
	}
	if parsedProjectDir := ledgerString(parsed, "project_dir"); parsedProjectDir != "" && provider != "netlify" {
		projectDir = parsedProjectDir
	}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "", "")
	if err != nil {
		result.Fatal = err
		return result
	}
	if buildDir == "" {
		buildDir = ledgerString(parsed, "build_dir")
	}
	if buildDir == "" {
		buildDir = ledgerBuildDirFromDeployPath(proj.ProjectDir, ledgerString(parsed, "deploy_path"))
	}
	if buildDir == "" {
		buildDir = "."
	}
	if op := homepageCurrentOperation(cfg); op.ID != "" {
		saved := SaveHomepageRevisionAndState(cfg, db, proj.ProjectDir, "Source for "+provider+" deployment", op.Reason, op.Source, map[string]interface{}{"operation_id": op.ID}, logger)
		if len(saved.Warnings) > 0 {
			result.Fatal = fmt.Errorf("source revision could not be linked: %s", strings.Join(saved.Warnings, "; "))
			return result
		}
	}
	latest, _ := GetLatestHomepageRevision(db, proj.ProjectDir)
	revisionID := int64(0)
	if latest != nil {
		revisionID = latest.ID
	}
	gitSHA := HomepageProjectGitSHA(cfg, proj.ProjectDir)
	artifactHash := ""
	buildRoot := filepath.Join(homepageLocalRoot(cfg, proj.ProjectDir), filepath.FromSlash(buildDir))
	if info, err := os.Stat(buildRoot); err != nil {
		if logger != nil {
			logger.Warn("[Homepage] Artifact manifest failed", "project_dir", proj.ProjectDir, "build_dir", buildDir, "error", err)
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("artifact manifest failed: build directory %q unavailable: %v", buildDir, err))
	} else if !info.IsDir() {
		err := fmt.Errorf("build path is not a directory")
		if logger != nil {
			logger.Warn("[Homepage] Artifact manifest failed", "project_dir", proj.ProjectDir, "build_dir", buildDir, "error", err)
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("artifact manifest failed: build path %q is not a directory", buildDir))
	} else {
		if manifest, err := BuildHomepageArtifactManifest(cfg, proj.ID, proj.ProjectDir, revisionID, gitSHA, buildDir); err == nil {
			artifactHash = manifest.ArtifactHash
			if writeErr := WriteHomepageArtifactManifest(cfg, manifest); writeErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("artifact manifest write failed: %v", writeErr))
			}
		} else {
			if logger != nil {
				logger.Warn("[Homepage] Artifact manifest failed", "project_dir", proj.ProjectDir, "build_dir", buildDir, "error", err)
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("artifact manifest failed: %v", err))
		}
	}
	targetKeys := []string{"site_id", "new_site_id", "deploy_site_id", "project_id"}
	deployKeys := []string{"deployment_id", "deploy_id", "id"}
	if provider == "here_now" {
		targetKeys = append(targetKeys, "slug")
		deployKeys = []string{"current_version_id", "deployment_id", "deploy_id", "id"}
	}
	targetID := firstLedgerString(parsed, targetKeys...)
	deployID := firstLedgerString(parsed, deployKeys...)
	url := firstLedgerString(parsed, "verified_url", "deployment_url", "deploy_url", "url", "served_url", "deploy_ssl_url", "deploy_deploy_url")
	remotePath := firstLedgerString(parsed, "path", "remote_path", "source_path")
	status := firstLedgerString(parsed, "status")
	if status == "" {
		status = "ok"
	}
	verification := map[string]interface{}{}
	if verified, ok := parsed["verified"].(bool); ok {
		verification["verified"] = verified
	}
	if verifiedURL := ledgerString(parsed, "verified_url"); verifiedURL != "" {
		verification["verified_url"] = verifiedURL
	}
	verificationJSON := ""
	if len(verification) > 0 {
		b, _ := json.Marshal(verification)
		verificationJSON = string(b)
	}
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{
		ProjectID:        proj.ID,
		RevisionID:       revisionID,
		GitSHA:           gitSHA,
		Provider:         provider,
		ProviderTargetID: targetID,
		ProviderDeployID: deployID,
		URL:              url,
		RemotePath:       remotePath,
		BuildDir:         buildDir,
		ArtifactHash:     artifactHash,
		Status:           status,
		VerificationJSON: verificationJSON,
		Metadata:         parsed,
	}); err != nil {
		result.Fatal = err
		return result
	}
	return result
}

// BuildHomepageArtifactManifest hashes a deployable build directory.
func BuildHomepageArtifactManifest(cfg HomepageConfig, projectID int64, projectDir string, revisionID int64, gitSHA, buildDir string) (HomepageArtifactManifest, error) {
	canonical, err := canonicalHomepageProjectDir(cfg, projectDir)
	if err != nil {
		return HomepageArtifactManifest{}, err
	}
	if buildDir == "" {
		buildDir = "."
	}
	if err := sanitizeProjectDir(buildDir); err != nil {
		return HomepageArtifactManifest{}, err
	}
	root := filepath.Join(homepageLocalRoot(cfg, canonical), filepath.FromSlash(buildDir))
	files, _, err := enumerateProjectFiles(root, nil)
	if err != nil {
		return HomepageArtifactManifest{}, err
	}
	manifestFiles := make([]HomepageArtifactManifestFile, 0, len(files))
	for _, f := range files {
		if f.Path == homepageArtifactManifestName {
			continue
		}
		manifestFiles = append(manifestFiles, HomepageArtifactManifestFile{Path: f.Path, Hash: f.Hash, Size: f.Size})
	}
	sort.Slice(manifestFiles, func(i, j int) bool { return manifestFiles[i].Path < manifestFiles[j].Path })
	h := sha256.New()
	for _, f := range manifestFiles {
		fmt.Fprintf(h, "%s:%s:%d\n", f.Path, f.Hash, f.Size)
	}
	return HomepageArtifactManifest{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		ProjectID:    projectID,
		ProjectDir:   canonical,
		RevisionID:   revisionID,
		GitSHA:       gitSHA,
		BuildDir:     filepath.ToSlash(filepath.Clean(buildDir)),
		ArtifactHash: hex.EncodeToString(h.Sum(nil)),
		Files:        manifestFiles,
		ManifestPath: filepath.Join(root, homepageArtifactManifestName),
	}, nil
}

// WriteHomepageArtifactManifest writes .aurago-site-manifest.json into the build directory.
func WriteHomepageArtifactManifest(cfg HomepageConfig, manifest HomepageArtifactManifest) error {
	if manifest.ManifestPath == "" {
		root := filepath.Join(homepageLocalRoot(cfg, manifest.ProjectDir), filepath.FromSlash(manifest.BuildDir))
		manifest.ManifestPath = filepath.Join(root, homepageArtifactManifestName)
	}
	if err := os.MkdirAll(filepath.Dir(manifest.ManifestPath), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := fileutil.WriteFileContext(context.Background(), manifest.ManifestPath, b, 0644); err != nil {
		return fmt.Errorf("write artifact manifest: %w", err)
	}
	return nil
}

// ReconcileHomepageProject updates local state and drift based on current files.
func ReconcileHomepageProject(cfg HomepageConfig, db *sql.DB, projectDir string, logger *slog.Logger) (HomepageProjectState, error) {
	if !HomepageOperationInProgress(cfg) {
		lock, _ := homepageOperationLocks.LoadOrStore(filepath.Clean(cfg.WorkspacePath), &sync.Mutex{})
		mu := lock.(*sync.Mutex)
		if err := lockToolHTTPContext(nativeHTTPContext(cfg.Context), mu); err != nil {
			return HomepageProjectState{}, err
		}
		defer mu.Unlock()
	}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "", "")
	if err != nil {
		return HomepageProjectState{}, err
	}
	current, _, err := enumerateProjectFiles(homepageLocalRoot(cfg, proj.ProjectDir), logger)
	if err != nil {
		return HomepageProjectState{}, err
	}
	stored := map[string]string{}
	rows, err := db.Query("SELECT rel_path, content_hash FROM homepage_site_file_state WHERE project_id = ?", proj.ID)
	if err != nil {
		return HomepageProjectState{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err == nil {
			stored[path] = hash
		}
	}
	if err := rows.Err(); err != nil {
		return HomepageProjectState{}, err
	}
	rows.Close()
	currentMap := map[string]string{}
	for _, f := range current {
		currentMap[f.Path] = f.Hash
	}
	drift := "clean"
	message := ""
	for path, hash := range currentMap {
		if stored[path] != hash {
			drift = "local_changed"
			message = fmt.Sprintf("local file changed: %s", path)
			break
		}
	}
	if drift == "clean" {
		for path := range stored {
			if _, ok := currentMap[path]; !ok {
				drift = "local_changed"
				message = fmt.Sprintf("local file removed: %s", path)
				break
			}
		}
	}
	if drift == "local_changed" {
		saved := SaveHomepageRevisionAndState(cfg, db, proj.ProjectDir, "Observed external file changes", "Original reason unknown; detected during reconciliation", "external_observation", map[string]interface{}{"actor": "external", "reason": "unknown", "reason_known": false}, logger)
		if len(saved.Warnings) > 0 {
			return HomepageProjectState{}, fmt.Errorf("record external changes: %s", strings.Join(saved.Warnings, "; "))
		}
	}
	if drift == "clean" {
		state, stateErr := GetHomepageProjectState(db, proj.ID)
		if stateErr == nil && state.CurrentRevisionID > 0 {
			latestDeployedRevision := latestSuccessfulHomepageDeploymentRevision(db, proj.ID)
			if latestDeployedRevision == 0 {
				drift = "not_deployed"
				message = "current revision has not been deployed"
			} else if latestDeployedRevision < state.CurrentRevisionID {
				drift = "not_deployed"
				message = fmt.Sprintf("current revision %d is newer than deployed revision %d", state.CurrentRevisionID, latestDeployedRevision)
			}
		}
	}
	if drift == "clean" {
		var deployments int
		_ = db.QueryRow("SELECT COUNT(*) FROM homepage_deployments WHERE project_id = ?", proj.ID).Scan(&deployments)
		if deployments == 0 {
			drift = "not_deployed"
			message = "no deployment recorded"
		} else if remoteDrift, remoteMessage := observeHomepageRemote(db, proj.ID, cfg.Context); remoteDrift != "" {
			drift = remoteDrift
			message = remoteMessage
		}
	}
	previous, _ := GetHomepageProjectState(db, proj.ID)
	if previous.DriftStatus != drift || previous.DriftMessage != message {
		if _, err := RecordHomepageEvent(db, proj.ID, "reconciled", "homepage_reconciler", message, map[string]interface{}{"actor": "system", "reason": "Verify current website state", "drift_status": drift}); err != nil {
			return HomepageProjectState{}, err
		}
	}
	gitSHA := HomepageProjectGitSHA(cfg, proj.ProjectDir)
	if err := nativeHTTPContext(cfg.Context).Err(); err != nil {
		return HomepageProjectState{}, err
	}
	if err := ensureHomepageProjectState(db, cfg, proj.ID, proj.ProjectDir, 0, gitSHA, drift, message); err != nil {
		return HomepageProjectState{}, err
	}
	return GetHomepageProjectState(db, proj.ID)
}

// ListHomepageManagedSites returns all managed sites with current state.
func ListHomepageManagedSites(db *sql.DB) ([]HomepageManagedSite, error) {
	return listHomepageManagedSites(db, 0)
}
func listHomepageManagedSites(db *sql.DB, id int64) ([]HomepageManagedSite, error) {
	if db == nil {
		return nil, fmt.Errorf("homepage registry DB not initialized")
	}
	rows, err := db.Query(`SELECT p.id, p.name, p.framework, p.url, p.project_dir, p.status,
			COALESCE(p.last_edited_at, ''), COALESCE(p.last_deployed_at, ''), p.last_deploy_url,
			COALESCE(s.local_root, ''), COALESCE(s.current_revision_id, 0), COALESCE(s.git_sha, ''),
			COALESCE(s.last_reconciled_at, ''), COALESCE(s.drift_status, 'not_deployed'), COALESCE(s.drift_message, ''),
            p.workspace_root,p.origin,p.creation_run_id,p.last_edit_reason
		FROM homepage_projects p
		LEFT JOIN homepage_project_state s ON s.project_id = p.id
		WHERE (?=0 OR p.id=?) ORDER BY p.updated_at DESC,p.id DESC`, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sites []HomepageManagedSite
	for rows.Next() {
		var site HomepageManagedSite
		if err := rows.Scan(&site.ID, &site.Name, &site.Framework, &site.URL, &site.ProjectDir, &site.Status,
			&site.LastEditedAt, &site.LastDeployedAt, &site.LastDeployURL, &site.LocalRoot, &site.CurrentRevisionID,
			&site.GitSHA, &site.LastReconciledAt, &site.DriftStatus, &site.DriftMessage, &site.WorkspaceRoot, &site.Origin, &site.CreationRunID, &site.LastEditReason); err != nil {
			return nil, err
		}
		site.LocalStatus = "unknown"
		if site.LocalRoot != "" {
			if info, err := os.Stat(site.LocalRoot); err == nil && info.IsDir() {
				site.LocalStatus = "available"
			} else if os.IsNotExist(err) {
				site.LocalStatus = "missing"
			}
		}
		if site.ProjectDir == "" {
			site.ContainerPath = ""
			site.LocalStatus = "remote_only"
			site.DriftStatus = "remote_unknown"
		} else {
			site.ContainerPath = "/workspace/" + site.ProjectDir
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

// GetHomepageManagedSite returns one managed site with recent deployments.
func GetHomepageManagedSite(db *sql.DB, id int64) (HomepageManagedSite, error) {
	if id <= 0 {
		return HomepageManagedSite{}, fmt.Errorf("id is required")
	}
	sites, err := listHomepageManagedSites(db, id)
	if err != nil {
		return HomepageManagedSite{}, err
	}
	for _, site := range sites {
		if site.ID == id {
			deployments, err := ListHomepageManagedDeployments(db, id, 10)
			if err != nil {
				return HomepageManagedSite{}, err
			}
			targets, err := ListHomepageManagedDeployTargets(db, id)
			if err != nil {
				return HomepageManagedSite{}, err
			}
			observations, err := ListHomepageManagedRemoteObservations(db, id, 10)
			if err != nil {
				return HomepageManagedSite{}, err
			}
			site.Deployments = deployments
			site.DeployTargets = targets
			site.Events, err = ListHomepageSiteEvents(db, id, 20, 0)
			if err != nil {
				return HomepageManagedSite{}, err
			}
			site.RemoteObservations = observations
			return site, nil
		}
	}
	return HomepageManagedSite{}, sql.ErrNoRows
}

// ListHomepageManagedDeployments returns recent deployments for a project.
func ListHomepageManagedDeployments(db *sql.DB, projectID int64, limit int) ([]HomepageManagedDeployment, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := db.Query(`SELECT id, created_at, COALESCE(revision_id, 0), git_sha, provider, provider_deploy_id, url, build_dir, artifact_hash, status
		FROM homepage_deployments WHERE project_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deployments []HomepageManagedDeployment
	for rows.Next() {
		var dep HomepageManagedDeployment
		if err := rows.Scan(&dep.ID, &dep.CreatedAt, &dep.RevisionID, &dep.GitSHA, &dep.Provider, &dep.ProviderDeployID, &dep.URL, &dep.BuildDir, &dep.ArtifactHash, &dep.Status); err != nil {
			return nil, err
		}
		deployments = append(deployments, dep)
	}
	return deployments, rows.Err()
}

// ListHomepageManagedDeployTargets returns current deploy targets for a project.
func ListHomepageManagedDeployTargets(db *sql.DB, projectID int64) ([]HomepageManagedDeployTarget, error) {
	rows, err := db.Query(`SELECT id, provider, provider_target_id, url, remote_path,
			COALESCE(last_seen_at, ''), COALESCE(updated_at, ''),status,metadata_json
		FROM homepage_deploy_targets
		WHERE project_id = ?
		ORDER BY updated_at DESC, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var targets []HomepageManagedDeployTarget
	for rows.Next() {
		var target HomepageManagedDeployTarget
		var metadata string
		if err := rows.Scan(&target.ID, &target.Provider, &target.ProviderTargetID, &target.URL, &target.RemotePath, &target.LastSeenAt, &target.UpdatedAt, &target.Status, &metadata); err != nil {
			return nil, err
		}
		var details map[string]interface{}
		json.Unmarshal([]byte(metadata), &details)
		target.Account = firstLedgerString(details, "account_id", "account", "team_id")
		target.Environment = firstLedgerString(details, "target", "environment")
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

// ListHomepageManagedRemoteObservations returns recent remote observations for a project.
func ListHomepageManagedRemoteObservations(db *sql.DB, projectID int64, limit int) ([]HomepageManagedRemoteObservation, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := db.Query(`SELECT id, COALESCE(target_id, 0), observed_at, provider, url,
			provider_deploy_id, content_hash, status
		FROM homepage_remote_observations
		WHERE project_id = ?
		ORDER BY observed_at DESC, id DESC
		LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var observations []HomepageManagedRemoteObservation
	for rows.Next() {
		var obs HomepageManagedRemoteObservation
		if err := rows.Scan(&obs.ID, &obs.TargetID, &obs.ObservedAt, &obs.Provider, &obs.URL, &obs.ProviderDeployID, &obs.ContentHash, &obs.Status); err != nil {
			return nil, err
		}
		observations = append(observations, obs)
	}
	return observations, rows.Err()
}

func latestSuccessfulHomepageDeploymentRevision(db *sql.DB, projectID int64) int64 {
	var revision sql.NullInt64
	_ = db.QueryRow(`SELECT revision_id
		FROM homepage_deployments
		WHERE project_id = ? AND status NOT IN ('error', 'failed') AND revision_id IS NOT NULL
		ORDER BY created_at DESC, id DESC LIMIT 1`, projectID).Scan(&revision)
	if revision.Valid {
		return revision.Int64
	}
	return 0
}

func observeHomepageRemote(db *sql.DB, projectID int64, contexts ...context.Context) (string, string) {
	targets, err := ListHomepageManagedDeployTargets(db, projectID)
	if err != nil {
		return "remote_unknown", err.Error()
	}
	if len(targets) == 0 {
		return observeHomepageRemoteTarget(db, projectID, 0, contexts...)
	}
	drift, message := "", ""
	for _, target := range targets {
		if target.Status == "inactive" {
			continue
		}
		status, note := observeHomepageRemoteTarget(db, projectID, target.ID, contexts...)
		if status == "remote_unknown" || (status != "" && drift == "") {
			drift, message = status, note
		}
	}
	return drift, message
}
func observeHomepageRemoteTarget(db *sql.DB, projectID, targetFilter int64, contexts ...context.Context) (string, string) {
	ctx := nativeHTTPContext(contexts...)
	if ctx.Err() != nil {
		return "", ""
	}
	var deploymentID int64
	var targetID sql.NullInt64
	var provider, url, providerDeployID string
	err := db.QueryRow(`SELECT id, target_id, provider, url, provider_deploy_id
		FROM homepage_deployments
		WHERE project_id = ? AND url != '' AND (?=0 OR target_id=?) AND status NOT IN ('failed','error')
		ORDER BY created_at DESC, id DESC LIMIT 1`, projectID, targetFilter, targetFilter).
		Scan(&deploymentID, &targetID, &provider, &url, &providerDeployID)
	if err != nil {
		return "", ""
	}
	client, err := homepageRemoteProbeClient(ctx, provider, url)
	var resp *http.Response
	if err == nil {
		resp, err = getToolHTTP(ctx, client, url)
	}
	if err != nil {
		if ctx.Err() != nil {
			return "", ""
		}
		insertHomepageRemoteObservation(db, projectID, targetID, provider, url, providerDeployID, "", "error", map[string]interface{}{"error": err.Error()})
		return "remote_unknown", fmt.Sprintf("remote observation failed for %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if ctx.Err() != nil {
		return "", ""
	}
	if err != nil {
		insertHomepageRemoteObservation(db, projectID, targetID, provider, url, providerDeployID, "", "error", map[string]interface{}{"error": err.Error()})
		return "remote_unknown", fmt.Sprintf("remote body read failed for %s: %v", url, err)
	}
	hash := hashContent(body)
	status := "ok"
	if resp.StatusCode >= 400 {
		status = "error"
	}
	var previousHash string
	if targetID.Valid {
		_ = db.QueryRow(`SELECT content_hash FROM homepage_remote_observations
			WHERE project_id = ? AND target_id = ? AND content_hash != ''
			ORDER BY observed_at DESC, id DESC LIMIT 1`, projectID, targetID.Int64).Scan(&previousHash)
	} else {
		_ = db.QueryRow(`SELECT content_hash FROM homepage_remote_observations
			WHERE project_id = ? AND provider = ? AND url = ? AND content_hash != ''
			ORDER BY observed_at DESC, id DESC LIMIT 1`, projectID, provider, url).Scan(&previousHash)
	}
	if ctx.Err() != nil {
		return "", ""
	}
	insertHomepageRemoteObservation(db, projectID, targetID, provider, url, providerDeployID, hash, status, map[string]interface{}{
		"http_status":   resp.StatusCode,
		"deployment_id": deploymentID,
	})
	if status != "ok" {
		return "remote_unknown", fmt.Sprintf("remote returned HTTP %d for %s", resp.StatusCode, url)
	}
	if previousHash != "" && previousHash != hash {
		return "remote_changed", fmt.Sprintf("remote content changed for %s", url)
	}
	return "", ""
}

func insertHomepageRemoteObservation(db *sql.DB, projectID int64, targetID sql.NullInt64, provider, url, providerDeployID, contentHash, status string, payload map[string]interface{}) {
	payloadJSON := ""
	if payload != nil {
		b, _ := json.Marshal(payload)
		payloadJSON = string(b)
	}
	var target interface{}
	if targetID.Valid {
		target = targetID.Int64
	}
	_, _ = db.Exec(`INSERT INTO homepage_remote_observations
		(project_id, target_id, provider, url, provider_deploy_id, content_hash, status, payload_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, target, provider, url, providerDeployID, contentHash, status, payloadJSON)
}

// GetHomepageProjectState reads the current ledger state.
func GetHomepageProjectState(db *sql.DB, projectID int64) (HomepageProjectState, error) {
	var state HomepageProjectState
	var revision sql.NullInt64
	var lastReconciled sql.NullString
	err := db.QueryRow(`SELECT project_id, local_root, current_revision_id, git_sha, last_reconciled_at, drift_status, drift_message
		FROM homepage_project_state WHERE project_id = ?`, projectID).
		Scan(&state.ProjectID, &state.LocalRoot, &revision, &state.GitSHA, &lastReconciled, &state.DriftStatus, &state.DriftMessage)
	if err != nil {
		return state, err
	}
	if revision.Valid {
		state.CurrentRevisionID = revision.Int64
	}
	if lastReconciled.Valid {
		state.LastReconciledAt = lastReconciled.String
	}
	return state, nil
}

// HomepageProjectGitSHA returns HEAD for local git repositories when available.
func HomepageProjectGitSHA(cfg HomepageConfig, projectDir string) string {
	root := homepageLocalRoot(cfg, projectDir)
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ensureHomepageProjectState(db *sql.DB, cfg HomepageConfig, projectID int64, projectDir string, revisionID int64, gitSHA, driftStatus, driftMessage string) error {
	if projectID <= 0 {
		return fmt.Errorf("project_id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var revision interface{}
	if revisionID > 0 {
		revision = revisionID
	}
	reconciled := ""
	if driftStatus != "" {
		reconciled = now
	}
	initial := driftStatus
	if initial == "" {
		initial = "not_deployed"
	}
	_, err := db.Exec(`INSERT INTO homepage_project_state
        (project_id,local_root,current_revision_id,git_sha,last_reconciled_at,drift_status,drift_message,updated_at)
        VALUES (?,?,?,?,?,?,?,?) ON CONFLICT(project_id) DO UPDATE SET
        current_revision_id=COALESCE(excluded.current_revision_id,homepage_project_state.current_revision_id),
        git_sha=CASE WHEN excluded.git_sha!='' THEN excluded.git_sha ELSE homepage_project_state.git_sha END,
        last_reconciled_at=CASE WHEN ?!='' THEN excluded.last_reconciled_at ELSE homepage_project_state.last_reconciled_at END,
        drift_status=CASE WHEN ?!='' THEN excluded.drift_status
            WHEN excluded.current_revision_id>COALESCE(homepage_project_state.current_revision_id,0) THEN 'not_deployed'
            ELSE homepage_project_state.drift_status END,
        drift_message=CASE WHEN ?!='' THEN excluded.drift_message
            WHEN excluded.current_revision_id>COALESCE(homepage_project_state.current_revision_id,0) THEN 'current revision has not been deployed'
            ELSE homepage_project_state.drift_message END,
        updated_at=excluded.updated_at`, projectID, homepageLocalRoot(cfg, projectDir), revision, gitSHA, reconciled, initial, driftMessage, now, driftStatus, driftStatus, driftStatus)
	return err
}

func canonicalHomepageProjectDir(cfg HomepageConfig, projectDir string) (string, error) {
	projectDir = strings.TrimSpace(filepath.ToSlash(projectDir))
	if projectDir == "" {
		projectDir = "."
	}
	isAbsProjectDir := filepath.IsAbs(filepath.FromSlash(projectDir)) ||
		isWindowsDriveAbsolutePath(projectDir) ||
		strings.HasPrefix(projectDir, "/") ||
		strings.HasPrefix(projectDir, "\\")
	if isAbsProjectDir {
		if strings.TrimSpace(cfg.WorkspacePath) == "" {
			return "", fmt.Errorf("project_dir must be relative to or inside the homepage workspace")
		}
		rel, err := filepath.Rel(
			filepath.Clean(cfg.WorkspacePath),
			filepath.Clean(filepath.FromSlash(projectDir)),
		)
		if err != nil || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("project_dir must be relative to or inside the homepage workspace")
		}
		projectDir = filepath.ToSlash(rel)
	}
	projectDir = strings.Trim(filepath.ToSlash(filepath.Clean(filepath.FromSlash(projectDir))), "/")
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return "", err
		}
	}
	return projectDir, nil
}

func homepageLocalRoot(cfg HomepageConfig, projectDir string) string {
	if cfg.WorkspacePath == "" {
		return filepath.FromSlash(projectDir)
	}
	if projectDir == "" || projectDir == "." {
		return cfg.WorkspacePath
	}
	return filepath.Join(cfg.WorkspacePath, filepath.FromSlash(projectDir))
}

func homepageProjectNameFromDir(projectDir string) string {
	projectDir = strings.Trim(filepath.ToSlash(projectDir), "/")
	if projectDir == "" || projectDir == "." {
		return "homepage-root"
	}
	if strings.Contains(projectDir, "/") {
		return strings.ReplaceAll(projectDir, "/", "-")
	}
	return projectDir
}

func ledgerString(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	if v, ok := values[key]; ok {
		switch typed := v.(type) {
		case string:
			return strings.TrimSpace(typed)
		case fmt.Stringer:
			return strings.TrimSpace(typed.String())
		}
	}
	return ""
}

func firstLedgerString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v := ledgerString(values, key); v != "" {
			return v
		}
	}
	return ""
}

func ledgerBuildDirFromDeployPath(projectDir, deployPath string) string {
	deployPath = strings.Trim(filepath.ToSlash(deployPath), "/")
	projectDir = strings.Trim(filepath.ToSlash(projectDir), "/")
	if deployPath == "" {
		return ""
	}
	if projectDir != "" && projectDir != "." {
		if deployPath == projectDir {
			return "."
		}
		prefix := projectDir + "/"
		if strings.HasPrefix(deployPath, prefix) {
			return strings.TrimPrefix(deployPath, prefix)
		}
	}
	return deployPath
}

func bindHomepageProjectWorkspace(db *sql.DB, cfg HomepageConfig, id int64, dir string) error {
	root := homepageWorkspaceRoot(cfg)
	var pinned, local string
	if err := db.QueryRow("SELECT p.workspace_root,COALESCE(s.local_root,'') FROM homepage_projects p LEFT JOIN homepage_project_state s ON s.project_id=p.id WHERE p.id=?", id).Scan(&pinned, &local); err != nil {
		return err
	}
	if pinned != "" && filepath.Clean(pinned) != root {
		return fmt.Errorf("project belongs to workspace %q; explicit relocation is required", pinned)
	}
	if pinned == "" && local != "" && filepath.Clean(local) != homepageLocalRoot(cfg, dir) {
		return fmt.Errorf("legacy project location does not match the configured workspace; explicit relocation is required")
	}
	_, err := db.Exec("UPDATE homepage_projects SET workspace_root=? WHERE id=? AND workspace_root=''", root, id)
	return err
}
