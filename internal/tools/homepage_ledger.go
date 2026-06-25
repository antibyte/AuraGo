package tools

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	ID                int64                       `json:"id"`
	Name              string                      `json:"name"`
	Framework         string                      `json:"framework,omitempty"`
	URL               string                      `json:"url,omitempty"`
	ProjectDir        string                      `json:"project_dir"`
	Status            string                      `json:"status"`
	LastEditedAt      string                      `json:"last_edited_at,omitempty"`
	LastDeployedAt    string                      `json:"last_deployed_at,omitempty"`
	LastDeployURL     string                      `json:"last_deploy_url,omitempty"`
	LocalRoot         string                      `json:"local_root,omitempty"`
	CurrentRevisionID int64                       `json:"current_revision_id,omitempty"`
	GitSHA            string                      `json:"git_sha,omitempty"`
	LastReconciledAt  string                      `json:"last_reconciled_at,omitempty"`
	DriftStatus       string                      `json:"drift_status"`
	DriftMessage      string                      `json:"drift_message,omitempty"`
	Deployments       []HomepageManagedDeployment `json:"deployments,omitempty"`
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
	if canonical == "" {
		canonical = "."
	}
	if proj, err := GetProjectByDir(db, canonical); err == nil {
		if err := ensureHomepageProjectState(db, cfg, proj.ID, canonical, 0, "", "not_deployed", ""); err != nil {
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
		if current, getErr := GetProject(db, id); getErr == nil && current.ProjectDir != canonical {
			fields := map[string]interface{}{"project_dir": canonical}
			if framework != "" && current.Framework == "" {
				fields["framework"] = framework
			}
			if updateErr := UpdateProject(db, id, fields); updateErr != nil {
				return nil, updateErr
			}
		}
	}
	if err := ensureHomepageProjectState(db, cfg, id, canonical, 0, "", "not_deployed", ""); err != nil {
		return nil, err
	}
	return GetProject(db, id)
}

// RecordHomepageEvent appends one structured ledger event.
func RecordHomepageEvent(db *sql.DB, projectID int64, eventType, source, summary string, payload map[string]interface{}) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("homepage registry DB not initialized")
	}
	if projectID <= 0 {
		return 0, fmt.Errorf("project_id is required")
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		eventType = "note"
	}
	source = strings.TrimSpace(source)
	if summary == "" {
		summary = eventType
	}
	payloadJSON := ""
	if payload != nil {
		b, _ := json.Marshal(payload)
		payloadJSON = string(b)
	}
	correlationID := fmt.Sprintf("hp-%d", time.Now().UnixNano())
	res, err := db.Exec(`INSERT INTO homepage_site_events
		(project_id, actor, source, event_type, summary, correlation_id, payload_json)
		VALUES (?, 'agent', ?, ?, ?, ?, ?)`,
		projectID, source, eventType, summary, correlationID, payloadJSON)
	if err != nil {
		return 0, fmt.Errorf("failed to record homepage event: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// SnapshotHomepageFileState replaces the current file-state inventory for a project.
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
	files, err := enumerateProjectFiles(basePath, logger)
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
	eventType := "revision_checked"
	if result.Changed {
		eventType = "revision_saved"
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["revision_id"] = result.RevisionID
	payload["changed"] = result.Changed
	if _, err := RecordHomepageEvent(db, proj.ID, eventType, source, message, payload); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("event logging failed: %v", err))
	}
	if result.Changed {
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
	if rec.Status == "" {
		rec.Status = "ok"
	}
	metadataJSON := ""
	if rec.Metadata != nil {
		b, _ := json.Marshal(rec.Metadata)
		metadataJSON = string(b)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.Exec(`INSERT INTO homepage_deploy_targets
		(project_id, provider, provider_target_id, url, remote_path, metadata_json, last_seen_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, provider) DO UPDATE SET
			provider_target_id = excluded.provider_target_id,
			url = excluded.url,
			remote_path = excluded.remote_path,
			metadata_json = excluded.metadata_json,
			last_seen_at = excluded.last_seen_at,
			updated_at = excluded.updated_at`,
		rec.ProjectID, rec.Provider, rec.ProviderTargetID, rec.URL, rec.RemotePath, metadataJSON, now, now)
	if err != nil {
		return fmt.Errorf("failed to upsert deploy target: %w", err)
	}
	targetID, _ := res.LastInsertId()
	if targetID == 0 {
		_ = db.QueryRow("SELECT id FROM homepage_deploy_targets WHERE project_id = ? AND provider = ?", rec.ProjectID, rec.Provider).Scan(&targetID)
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
	_, _ = db.Exec("UPDATE homepage_project_state SET drift_status = 'clean', drift_message = '', updated_at = ? WHERE project_id = ?", now, rec.ProjectID)
	return nil
}

// RecordHomepageDeploymentFromResult parses a homepage deploy tool result and records it.
func RecordHomepageDeploymentFromResult(cfg HomepageConfig, db *sql.DB, projectDir, provider, buildDir, rawResult string, logger *slog.Logger) []string {
	var warnings []string
	if db == nil {
		return []string{"homepage registry DB not initialized"}
	}
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "", "")
	if err != nil {
		return []string{err.Error()}
	}
	parsed := map[string]interface{}{}
	if err := json.Unmarshal([]byte(rawResult), &parsed); err != nil {
		return []string{fmt.Sprintf("deployment result was not JSON: %v", err)}
	}
	if status, _ := parsed["status"].(string); status == "error" {
		return nil
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
	latest, _ := GetLatestHomepageRevision(db, proj.ProjectDir)
	revisionID := int64(0)
	if latest != nil {
		revisionID = latest.ID
	}
	gitSHA := HomepageProjectGitSHA(cfg, proj.ProjectDir)
	artifactHash := ""
	if manifest, err := BuildHomepageArtifactManifest(cfg, proj.ID, proj.ProjectDir, revisionID, gitSHA, buildDir); err == nil {
		artifactHash = manifest.ArtifactHash
		if writeErr := WriteHomepageArtifactManifest(cfg, manifest); writeErr != nil {
			warnings = append(warnings, fmt.Sprintf("artifact manifest write failed: %v", writeErr))
		}
	} else if logger != nil {
		logger.Warn("[Homepage] Artifact manifest failed", "project_dir", proj.ProjectDir, "build_dir", buildDir, "error", err)
		warnings = append(warnings, fmt.Sprintf("artifact manifest failed: %v", err))
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	targetID := firstLedgerString(parsed, "site_id", "new_site_id", "deploy_site_id", "project_id")
	deployID := firstLedgerString(parsed, "deployment_id", "deploy_id", "id")
	url := firstLedgerString(parsed, "verified_url", "deployment_url", "deploy_url", "url", "deploy_ssl_url", "deploy_deploy_url")
	remotePath := firstLedgerString(parsed, "path", "remote_path")
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
		warnings = append(warnings, err.Error())
	}
	return warnings
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
	files, err := enumerateProjectFiles(root, nil)
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
	return os.WriteFile(manifest.ManifestPath, b, 0644)
}

// ReconcileHomepageProject updates local state and drift based on current files.
func ReconcileHomepageProject(cfg HomepageConfig, db *sql.DB, projectDir string, logger *slog.Logger) (HomepageProjectState, error) {
	proj, err := EnsureHomepageProjectForDir(db, cfg, projectDir, "", "")
	if err != nil {
		return HomepageProjectState{}, err
	}
	current, err := enumerateProjectFiles(homepageLocalRoot(cfg, proj.ProjectDir), logger)
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
	if drift == "clean" {
		var deployments int
		_ = db.QueryRow("SELECT COUNT(*) FROM homepage_deployments WHERE project_id = ?", proj.ID).Scan(&deployments)
		if deployments == 0 {
			drift = "not_deployed"
			message = "no deployment recorded"
		} else if remoteDrift, remoteMessage := observeHomepageRemote(db, proj.ID); remoteDrift != "" {
			drift = remoteDrift
			message = remoteMessage
		}
	}
	gitSHA := HomepageProjectGitSHA(cfg, proj.ProjectDir)
	if err := ensureHomepageProjectState(db, cfg, proj.ID, proj.ProjectDir, 0, gitSHA, drift, message); err != nil {
		return HomepageProjectState{}, err
	}
	return GetHomepageProjectState(db, proj.ID)
}

// ListHomepageManagedSites returns all managed sites with current state.
func ListHomepageManagedSites(db *sql.DB) ([]HomepageManagedSite, error) {
	if db == nil {
		return nil, fmt.Errorf("homepage registry DB not initialized")
	}
	rows, err := db.Query(`SELECT p.id, p.name, p.framework, p.url, p.project_dir, p.status,
			COALESCE(p.last_edited_at, ''), COALESCE(p.last_deployed_at, ''), p.last_deploy_url,
			COALESCE(s.local_root, ''), COALESCE(s.current_revision_id, 0), COALESCE(s.git_sha, ''),
			COALESCE(s.last_reconciled_at, ''), COALESCE(s.drift_status, 'not_deployed'), COALESCE(s.drift_message, '')
		FROM homepage_projects p
		LEFT JOIN homepage_project_state s ON s.project_id = p.id
		ORDER BY p.updated_at DESC, p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sites []HomepageManagedSite
	for rows.Next() {
		var site HomepageManagedSite
		if err := rows.Scan(&site.ID, &site.Name, &site.Framework, &site.URL, &site.ProjectDir, &site.Status,
			&site.LastEditedAt, &site.LastDeployedAt, &site.LastDeployURL, &site.LocalRoot, &site.CurrentRevisionID,
			&site.GitSHA, &site.LastReconciledAt, &site.DriftStatus, &site.DriftMessage); err != nil {
			return nil, err
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
	sites, err := ListHomepageManagedSites(db)
	if err != nil {
		return HomepageManagedSite{}, err
	}
	for _, site := range sites {
		if site.ID == id {
			deployments, err := ListHomepageManagedDeployments(db, id, 10)
			if err != nil {
				return HomepageManagedSite{}, err
			}
			site.Deployments = deployments
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

func observeHomepageRemote(db *sql.DB, projectID int64) (string, string) {
	var deploymentID int64
	var targetID sql.NullInt64
	var provider, url, providerDeployID string
	err := db.QueryRow(`SELECT id, target_id, provider, url, provider_deploy_id
		FROM homepage_deployments
		WHERE project_id = ? AND url != ''
		ORDER BY created_at DESC, id DESC LIMIT 1`, projectID).
		Scan(&deploymentID, &targetID, &provider, &url, &providerDeployID)
	if err != nil {
		return "", ""
	}
	resp, err := homepageVerifyHTTPClient.Get(url)
	if err != nil {
		insertHomepageRemoteObservation(db, projectID, targetID, provider, url, providerDeployID, "", "error", map[string]interface{}{"error": err.Error()})
		return "remote_unknown", fmt.Sprintf("remote observation failed for %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
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
	localRoot := homepageLocalRoot(cfg, projectDir)
	now := time.Now().UTC().Format(time.RFC3339)
	var revision interface{}
	if revisionID > 0 {
		revision = revisionID
	}
	if driftStatus == "" {
		driftStatus = "not_deployed"
	}
	_, err := db.Exec(`INSERT INTO homepage_project_state
		(project_id, local_root, current_revision_id, git_sha, last_reconciled_at, drift_status, drift_message, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET
			local_root = excluded.local_root,
			current_revision_id = COALESCE(excluded.current_revision_id, homepage_project_state.current_revision_id),
			git_sha = CASE WHEN excluded.git_sha != '' THEN excluded.git_sha ELSE homepage_project_state.git_sha END,
			last_reconciled_at = excluded.last_reconciled_at,
			drift_status = CASE WHEN excluded.drift_status != '' THEN excluded.drift_status ELSE homepage_project_state.drift_status END,
			drift_message = excluded.drift_message,
			updated_at = excluded.updated_at`,
		projectID, localRoot, revision, gitSHA, now, driftStatus, driftMessage, now)
	return err
}

func canonicalHomepageProjectDir(cfg HomepageConfig, projectDir string) (string, error) {
	projectDir = strings.TrimSpace(filepath.ToSlash(projectDir))
	if projectDir == "" {
		projectDir = "."
	}
	if strings.HasPrefix(projectDir, "/workspace/") {
		projectDir = strings.TrimPrefix(projectDir, "/workspace/")
	}
	if filepath.IsAbs(projectDir) && cfg.WorkspacePath != "" {
		rel, err := filepath.Rel(cfg.WorkspacePath, filepath.FromSlash(projectDir))
		if err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
			projectDir = filepath.ToSlash(rel)
		}
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
	projectDir = strings.Trim(projectDir, "/")
	if projectDir == "" || projectDir == "." {
		return "homepage-root"
	}
	return filepath.Base(filepath.FromSlash(projectDir))
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
		prefix := projectDir + "/"
		if strings.HasPrefix(deployPath, prefix) {
			return strings.TrimPrefix(deployPath, prefix)
		}
	}
	return deployPath
}
