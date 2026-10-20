package tools

import (
	"aurago/internal/security"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// HomepageOperation is a short statement of intent, not a copy of the conversation.
type HomepageOperation struct {
	ProjectID      int64
	Name           string
	ProjectDir     string
	Reason         string
	Actor          string
	Source         string
	SessionID      string
	RunID          string
	ID             string
	FilesMayChange bool
}
type homepageOperationKey struct{}

var homepageOperationLocks sync.Map

func HomepageOperationInProgress(cfg HomepageConfig) bool {
	return cfg.Context != nil && cfg.Context.Value(homepageOperationKey{}) != nil
}
func homepageCurrentOperation(cfg HomepageConfig) HomepageOperation {
	if cfg.Context != nil {
		if op, ok := cfg.Context.Value(homepageOperationKey{}).(HomepageOperation); ok {
			return op
		}
	}
	return HomepageOperation{}
}
func HomepageOperationMutates(name string) bool {
	if strings.HasPrefix(name, "remote_") {
		return true
	}
	switch name {
	case "init", "start", "stop", "rebuild", "destroy", "exec", "init_project", "build", "install_deps", "write_file", "edit_file", "json_edit", "yaml_edit", "xml_edit", "optimize_images", "dev", "deploy", "deploy_netlify", "deploy_here_now", "deploy_vercel", "publish_local", "webserver_start", "webserver_stop", "tunnel", "git_init", "git_commit", "git_rollback", "save_revision", "restore_revision":
		return true
	}
	return false
}
func HomepageOperationChangesFiles(name string) bool {
	switch name {
	case "init", "start", "stop", "destroy", "webserver_stop", "dev", "tunnel":
		return false
	}
	return HomepageOperationMutates(name)
}

// ExecuteHomepageOperation is the production mutation boundary for every caller.
// The intent survives a crash between filesystem/provider effects and database completion.
func ExecuteHomepageOperation(cfg HomepageConfig, db *sql.DB, op HomepageOperation, logger *slog.Logger, execute func(HomepageConfig) string) string {
	if !HomepageOperationMutates(op.Name) {
		return strings.TrimPrefix(execute(cfg), "Tool Output: ")
	}
	if db == nil {
		return ErrorJSON("homepage registry DB is required before changing a managed website")
	}
	op.Reason = strings.TrimSpace(security.RedactSensitiveInfo(op.Reason))
	if op.Reason == "" {
		return ErrorJSON("reason is required before a homepage mutation; explain the purpose of the change")
	}
	if len(op.Reason) > 2000 {
		return ErrorJSON("reason must be at most 2000 bytes")
	}
	if op.Actor == "" {
		op.Actor = "agent"
	}
	if op.ID == "" {
		op.ID = "hp-" + rand.Text()
	}
	// ponytail: one lock per workspace serializes global container operations too.
	// Replace with ordered project locks only if concurrent workspace throughput requires it.
	lockValue, _ := homepageOperationLocks.LoadOrStore(filepath.Clean(cfg.WorkspacePath), &sync.Mutex{})
	mu := lockValue.(*sync.Mutex)
	if err := lockToolHTTPContext(nativeHTTPContext(cfg.Context), mu); err != nil {
		return ErrorJSON(err.Error())
	}
	defer mu.Unlock()
	if op.ProjectDir == "" && op.ProjectID == 0 {
		sites, err := ListHomepageManagedSites(db)
		if err != nil {
			return ErrorJSON(err.Error())
		}
		if op.Name == "webserver_start" || op.Name == "publish_local" {
			if saved, build := loadWebserverState(cfg.WorkspacePath); saved != "" {
				if info, err := os.Stat(filepath.Join(cfg.WorkspacePath, saved, build)); err == nil && info.IsDir() {
					op.ProjectDir = saved
				}
			}
			if op.ProjectDir == "" {
				op.ProjectDir, _ = findServableProject(cfg.WorkspacePath)
			}
			if op.ProjectDir == "" {
				op.ProjectDir = findNextJsProjectWithoutExport(cfg.WorkspacePath)
			}
			if op.ProjectDir == "" {
				candidates, err := DiscoverHomepageWorkspace(cfg, db)
				if err != nil {
					return ErrorJSON(err.Error())
				}
				if len(candidates) > 0 || len(sites) > 0 {
					return ErrorJSON("select an explicit project_dir before starting a web server for this workspace")
				}
				for _, name := range []string{"index.html", "out", "dist", "build"} {
					if _, err := os.Stat(filepath.Join(cfg.WorkspacePath, name)); err == nil {
						return ErrorJSON("move root website files into a named project directory and adopt it before publishing")
					}
				}
			}
		}
		if op.ProjectDir == "" && op.ProjectID == 0 {
			// An environment operation affects every known project, never an invented root site.
			ids := []int64{}
			for _, site := range sites {
				if site.Status == "archived" || !homepageSiteInWorkspace(site, cfg) {
					continue
				}
				if _, err := recordHomepageOperationEvent(db, site.ID, op, "operation_started", "pending", nil); err != nil {
					return ErrorJSON(err.Error())
				}
				ids = append(ids, site.ID)
			}
			raw := strings.TrimPrefix(execute(cfg), "Tool Output: ")
			result, outcome := homepageOperationOutcome(raw)
			for _, id := range ids {
				if op.Name == "webserver_stop" || op.Name == "destroy" {
					status := "unknown"
					if outcome == "succeeded" {
						status = "inactive"
					}
					if _, err := db.Exec("UPDATE homepage_deploy_targets SET status=?,updated_at=CURRENT_TIMESTAMP WHERE project_id=? AND provider IN ('caddy','local')", status, id); err != nil {
						return homepageOperationPersistenceError(result, op, err)
					}
				}
				if _, err := recordHomepageOperationEvent(db, id, op, "operation_finished", outcome, nil); err != nil {
					return homepageOperationPersistenceError(result, op, err)
				}
			}
			return homepageOperationResult(result, op, 0, 0)
		}
	}
	var proj *HomepageProject
	var err error
	if op.ProjectID > 0 {
		proj, err = GetProject(db, op.ProjectID)
		if err == nil && proj.ProjectDir != "" {
			proj, err = EnsureHomepageProjectForDir(db, cfg, proj.ProjectDir, "", "")
		}
	} else {
		proj, err = EnsureHomepageProjectForDir(db, cfg, op.ProjectDir, "", "")
	}
	if err != nil {
		return ErrorJSON(err.Error())
	}
	if proj.Status == "archived" {
		return ErrorJSON("project is archived; explicitly adopt/reactivate it before changing files")
	}
	op.ProjectDir = proj.ProjectDir
	if _, err := recordHomepageOperationEvent(db, proj.ID, op, "operation_started", "pending", nil); err != nil {
		return ErrorJSON(err.Error())
	}
	cfg.ManagedProjectDir = op.ProjectDir
	cfg.Context = context.WithValue(nativeHTTPContext(cfg.Context), homepageOperationKey{}, op)
	// Capture pre-existing external edits with unknown provenance, before attributing our work.
	if op.Name != "init_project" && proj.ProjectDir != "" {
		if _, err := os.Stat(homepageLocalRoot(cfg, proj.ProjectDir)); err == nil {
			before := SaveHomepageRevisionAndState(cfg, db, proj.ProjectDir, "Observed project state before "+op.Name, "External or legacy changes; original reason unknown", "external_observation", map[string]interface{}{"actor": "external", "reason_known": false, "operation_id": op.ID}, logger)
			if len(before.Warnings) > 0 {
				recordHomepageOperationEvent(db, proj.ID, op, "operation_finished", "failed", map[string]interface{}{"error": "preflight snapshot failed"})
				return ErrorJSON(strings.Join(before.Warnings, "; "))
			}
		}
	}
	otherSites := []HomepageManagedSite{}
	candidatesBefore := map[string]bool{}
	if op.Name == "exec" {
		sites, err := ListHomepageManagedSites(db)
		if err != nil {
			return ErrorJSON(err.Error())
		}
		for _, site := range sites {
			if site.ID == proj.ID || site.Status == "archived" || !homepageSiteInWorkspace(site, cfg) || site.LocalStatus != "available" {
				continue
			}
			baseline := SaveHomepageRevisionAndState(cfg, db, site.ProjectDir, "State before workspace command", "Original reason unknown", "external_observation", map[string]interface{}{"actor": "external", "reason": "unknown"}, logger)
			if len(baseline.Warnings) > 0 {
				return homepageOperationPersistenceError(map[string]interface{}{}, op, fmt.Errorf("could not observe workspace project %s", site.ProjectDir))
			}
			otherSites = append(otherSites, site)
		}
		candidates, err := DiscoverHomepageWorkspace(cfg, db)
		if err != nil {
			return homepageOperationPersistenceError(map[string]interface{}{}, op, err)
		}
		for _, candidate := range candidates {
			candidatesBefore[candidate.ProjectDir] = true
		}
	}
	raw := strings.TrimPrefix(execute(cfg), "Tool Output: ")
	result, outcome := homepageOperationOutcome(raw)
	revisionID := int64(0)
	var ledgerErr error
	if op.FilesMayChange || op.Name == "init_project" {
		if _, err := os.Stat(homepageLocalRoot(cfg, proj.ProjectDir)); err == nil {
			saved := SaveHomepageRevisionAndState(cfg, db, proj.ProjectDir, op.Name, op.Reason, op.Source, map[string]interface{}{"operation_id": op.ID, "actor": op.Actor, "reason": op.Reason, "run_id": op.RunID, "session_id": op.SessionID}, logger)
			revisionID = saved.RevisionID
			if len(saved.Warnings) > 0 {
				ledgerErr = fmt.Errorf("%s", strings.Join(saved.Warnings, "; "))
			}
		} else if outcome == "succeeded" {
			ledgerErr = fmt.Errorf("project files are missing after operation: %w", err)
		}
	}
	if revisionID == 0 && (op.Name == "save_revision" || op.Name == "restore_revision") {
		if id, ok := result["revision_id"].(float64); ok && id > 0 {
			if revision, err := GetHomepageRevision(db, int64(id)); err == nil && revision.ProjectDir == proj.ProjectDir {
				revisionID = revision.ID
			}
		}
	}
	if op.Name == "exec" {
		missing := false
		if _, err := os.Stat(homepageLocalRoot(cfg, proj.ProjectDir)); os.IsNotExist(err) {
			missing = true
		}
		for _, site := range otherSites {
			if _, err := os.Stat(site.LocalRoot); os.IsNotExist(err) {
				missing = true
			}
		}
		candidates, err := DiscoverHomepageWorkspace(cfg, db)
		if err != nil {
			ledgerErr = err
		} else {
			result["discovered_projects"] = candidates
			for _, candidate := range candidates {
				if candidatesBefore[candidate.ProjectDir] || missing {
					continue
				}
				created, err := EnsureHomepageProjectForDir(db, cfg, candidate.ProjectDir, "", "")
				if err != nil {
					ledgerErr = err
					continue
				}
				if _, err := db.Exec("UPDATE homepage_projects SET origin='agent_created',creation_run_id=? WHERE id=?", op.RunID, created.ID); err != nil {
					ledgerErr = err
					continue
				}
				site, err := GetHomepageManagedSite(db, created.ID)
				if err != nil {
					ledgerErr = err
					continue
				}
				otherSites = append(otherSites, site)
			}
		}
		for _, site := range otherSites {
			if _, err := os.Stat(site.LocalRoot); os.IsNotExist(err) {
				continue
			}
			saved := SaveHomepageRevisionAndState(cfg, db, site.ProjectDir, "Workspace command: "+op.Name, op.Reason, op.Source, map[string]interface{}{"operation_id": op.ID, "reason": op.Reason, "actor": op.Actor}, logger)
			if len(saved.Warnings) > 0 {
				ledgerErr = fmt.Errorf("could not finalize workspace project %s", site.ProjectDir)
				continue
			}
			if saved.Changed {
				details := map[string]interface{}{"revision_id": saved.RevisionID, "observed_during_workspace_command": true}
				if _, err := recordHomepageOperationEvent(db, site.ID, op, "operation_finished", outcome, details); err != nil {
					ledgerErr = err
				}
				if outcome == "failed" {
					outcome = "partial"
					result["status"] = "partial_success"
					result["retry_safe"] = false
				}
			}
		}
		if missing {
			result["next_action"] = "Use homepage_registry relocate with the existing project id, explicit new project_dir and reason; no identity was guessed."
		}
	}
	if outcome == "failed" && revisionID > 0 {
		outcome = "partial"
		result["status"] = "partial_success"
		result["retry_safe"] = false
	}
	if op.Name == "init_project" && outcome == "succeeded" {
		_, err := db.Exec("UPDATE homepage_projects SET origin='agent_created',creation_run_id=? WHERE id=? AND origin='unknown'", op.RunID, proj.ID)
		if err != nil {
			ledgerErr = err
		}
	}
	details := map[string]interface{}{"revision_id": revisionID}
	if revisionID > 0 {
		if files, err := GetHomepageRevisionFiles(db, revisionID); err == nil {
			changed := []map[string]string{}
			for _, f := range files {
				changed = append(changed, map[string]string{"path": f.Path, "change": f.ChangeType})
			}
			details["files_changed"] = changed
		} else {
			ledgerErr = err
		}
	}
	if op.Name == "webserver_stop" || op.Name == "destroy" {
		status := "unknown"
		if outcome == "succeeded" {
			status = "inactive"
		}
		if _, err := db.Exec("UPDATE homepage_deploy_targets SET status=?,updated_at=CURRENT_TIMESTAMP WHERE project_id=? AND provider IN ('local','caddy')", status, proj.ID); err != nil {
			ledgerErr = err
		}
	}
	if ledgerErr != nil {
		outcome = "unknown"
		details["error"] = "ledger finalization failed"
	}
	if outcome == "succeeded" {
		kind := map[string]string{"write_file": "file_written", "edit_file": "file_edited", "json_edit": "json_edited", "yaml_edit": "yaml_edited", "xml_edit": "xml_edited", "init_project": "project_initialized"}[op.Name]
		if kind != "" {
			if _, err := recordHomepageOperationEvent(db, proj.ID, op, kind, outcome, details); err != nil {
				ledgerErr = err
			}
		}
	}
	if _, err := recordHomepageOperationEvent(db, proj.ID, op, "operation_finished", outcome, details); err != nil {
		ledgerErr = err
	}
	if ledgerErr != nil {
		return homepageOperationPersistenceError(result, op, ledgerErr)
	}
	return homepageOperationResult(result, op, proj.ID, revisionID)
}

func homepageOperationOutcome(raw string) (map[string]interface{}, string) {
	result := map[string]interface{}{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil || result == nil {
		return map[string]interface{}{"status": "unknown", "message": "operation returned an unreadable result", "retry_safe": false}, "unknown"
	}
	switch status, _ := result["status"].(string); status {
	case "ok", "success":
		return result, "succeeded"
	case "error", "failed", "cancelled":
		return result, "failed"
	case "partial_success":
		return result, "partial"
	default:
		return result, "unknown"
	}
}
func homepageOperationResult(result map[string]interface{}, op HomepageOperation, siteID, revisionID int64) string {
	result["operation_id"] = op.ID
	result["reason"] = op.Reason
	if siteID > 0 {
		result["managed_site_id"] = siteID
	}
	if revisionID > 0 {
		result["current_revision_id"] = revisionID
	}
	if _, reported := result["ledger_updated"]; !reported {
		result["ledger_updated"] = true
	}
	raw, _ := json.Marshal(result)
	return string(raw)
}
func homepageOperationPersistenceError(result map[string]interface{}, op HomepageOperation, err error) string {
	result["status"] = "partial_success"
	result["retry_safe"] = false
	result["ledger_updated"] = false
	result["operation_id"] = op.ID
	result["reason"] = op.Reason
	result["message"] = "The operation may have changed files or a provider, but ledger completion failed. Verify before retrying."
	result["ledger_error"] = security.RedactSensitiveInfo(err.Error())
	raw, _ := json.Marshal(result)
	return string(raw)
}
func recordHomepageOperationEvent(db *sql.DB, id int64, op HomepageOperation, kind, outcome string, details map[string]interface{}) (int64, error) {
	if details == nil {
		details = map[string]interface{}{}
	}
	details["operation_id"] = op.ID
	details["operation"] = op.Name
	details["reason"] = op.Reason
	details["actor"] = op.Actor
	details["outcome"] = outcome
	details["run_id"] = op.RunID
	details["session_id"] = op.SessionID
	return RecordHomepageEvent(db, id, kind, op.Source, op.Name+": "+op.Reason, details)
}

// RecoverHomepageOperations runs once before the server accepts new work.
func RecoverHomepageOperations(db *sql.DB) error {
	rows, err := db.Query(`SELECT e.project_id,e.correlation_id,e.source,e.payload_json FROM homepage_site_events e
        WHERE e.event_type='operation_started' AND NOT EXISTS
        (SELECT 1 FROM homepage_site_events f WHERE f.correlation_id=e.correlation_id AND f.project_id=e.project_id AND f.event_type='operation_finished')`)
	if err != nil {
		return err
	}
	type pending struct {
		id                           int64
		correlation, source, payload string
	}
	items := []pending{}
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.correlation, &p.source, &p.payload); err != nil {
			rows.Close()
			return err
		}
		items = append(items, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, p := range items {
		payload := map[string]interface{}{}
		json.Unmarshal([]byte(p.payload), &payload)
		op := HomepageOperation{ID: p.correlation, Name: ledgerString(payload, "operation"), Reason: ledgerString(payload, "reason"), Actor: ledgerString(payload, "actor"), Source: p.source, RunID: ledgerString(payload, "run_id"), SessionID: ledgerString(payload, "session_id")}
		if _, err := recordHomepageOperationEvent(db, p.id, op, "operation_finished", "unknown", map[string]interface{}{"recovered_after_restart": true, "retry_safe": false}); err != nil {
			return err
		}
	}
	return nil
}
