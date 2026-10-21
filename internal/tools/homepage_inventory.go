package tools

import (
	"aurago/internal/security"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type HomepageSiteEvent struct {
	ID          int64                  `json:"id"`
	CreatedAt   string                 `json:"created_at"`
	Actor       string                 `json:"actor"`
	Source      string                 `json:"source"`
	EventType   string                 `json:"event_type"`
	Summary     string                 `json:"summary"`
	OperationID string                 `json:"operation_id"`
	Reason      string                 `json:"reason"`
	Outcome     string                 `json:"outcome,omitempty"`
	Payload     map[string]interface{} `json:"details,omitempty"`
}

func ListHomepageSiteEvents(db *sql.DB, projectID int64, limit int, after int64) ([]HomepageSiteEvent, error) {
	return listHomepageSiteEvents(db, projectID, limit, after, 0)
}
func listHomepageSiteEvents(db *sql.DB, projectID int64, limit int, after int64, offset int) ([]HomepageSiteEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := db.Query(`SELECT id,created_at,actor,source,event_type,summary,correlation_id,payload_json FROM homepage_site_events
        WHERE project_id=? AND id>? ORDER BY id DESC LIMIT ? OFFSET ?`, projectID, after, limit, max(0, offset))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []HomepageSiteEvent{}
	for rows.Next() {
		var e HomepageSiteEvent
		var payload string
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.Actor, &e.Source, &e.EventType, &e.Summary, &e.OperationID, &payload); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(payload), &e.Payload)
		e.Reason = ledgerString(e.Payload, "reason")
		if e.Reason == "" {
			e.Reason = "unknown"
		}
		e.Outcome = ledgerString(e.Payload, "outcome")
		events = append(events, e)
	}
	return events, rows.Err()
}
func HomepageLedgerVersion(db *sql.DB) (int64, error) {
	var version int64
	err := db.QueryRow("SELECT COALESCE(MAX(id),0) FROM homepage_site_events").Scan(&version)
	return version, err
}

func homepageWorkspaceRoot(cfg HomepageConfig) string {
	if cfg.WorkspacePath == "" {
		return ""
	}
	absolute, err := filepath.Abs(cfg.WorkspacePath)
	if err != nil {
		return ""
	}
	return filepath.Clean(absolute)
}
func homepageSiteInWorkspace(site HomepageManagedSite, cfg HomepageConfig) bool {
	root := homepageWorkspaceRoot(cfg)
	if site.WorkspaceRoot != "" {
		return filepath.Clean(site.WorkspaceRoot) == root
	}
	return site.LocalRoot == "" || filepath.Clean(site.LocalRoot) == filepath.Join(root, filepath.FromSlash(site.ProjectDir))
}

// ResolveHomepageProjectDir selects the longest registered boundary, never just the first folder.
func ResolveHomepageProjectDir(db *sql.DB, cfg HomepageConfig, path, explicit string, managedID int64) (string, error) {
	if managedID > 0 {
		site, err := GetHomepageManagedSite(db, managedID)
		if err != nil {
			return "", err
		}
		if !homepageSiteInWorkspace(site, cfg) {
			return "", fmt.Errorf("site belongs to a different workspace")
		}
		if explicit != "" && explicit != site.ProjectDir {
			return "", fmt.Errorf("managed_site_id and project_dir disagree")
		}
		explicit = site.ProjectDir
	}
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path != "" && (!filepath.IsLocal(filepath.FromSlash(path)) || path == ".") {
		return "", fmt.Errorf("path must stay inside a named project")
	}
	if explicit != "" {
		dir, err := NormalizeHomepageProjectIdentity(explicit, false)
		if err != nil {
			return "", err
		}
		if path != "" && path != dir && !strings.HasPrefix(path, dir+"/") {
			return "", fmt.Errorf("path does not belong to project_dir")
		}
		return dir, nil
	}
	if path == "" {
		return "", nil
	}
	sites, err := ListHomepageManagedSites(db)
	if err != nil {
		return "", err
	}
	best := ""
	for _, site := range sites {
		if homepageSiteInWorkspace(site, cfg) && (path == site.ProjectDir || strings.HasPrefix(path, site.ProjectDir+"/")) && len(site.ProjectDir) > len(best) {
			best = site.ProjectDir
		}
	}
	if best != "" {
		return best, nil
	}
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("path must include a named project directory")
	}
	return NormalizeHomepageProjectIdentity(parts[0], false)
}

type HomepageDiscoveryCandidate struct {
	ProjectDir string `json:"project_dir"`
	LocalRoot  string `json:"local_root"`
}

func DiscoverHomepageWorkspace(cfg HomepageConfig, db *sql.DB) ([]HomepageDiscoveryCandidate, error) {
	known, err := ListHomepageManagedSites(db)
	if err != nil {
		return nil, err
	}
	registered := map[string]bool{}
	for _, site := range known {
		if homepageSiteInWorkspace(site, cfg) {
			registered[site.ProjectDir] = true
		}
	}
	found := []HomepageDiscoveryCandidate{}
	root, err := os.OpenRoot(cfg.WorkspacePath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	visited := 0
	err = filepath.Walk(cfg.WorkspacePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(cfg.WorkspacePath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !info.IsDir() {
			return nil
		}
		visited++
		if visited > 10000 {
			return fmt.Errorf("workspace discovery exceeded 10000 directories")
		}
		if rel == "." {
			return nil
		}
		if shouldExcludePath(rel) || strings.Count(rel, "/") >= 4 {
			return filepath.SkipDir
		}
		candidate := false
		for _, marker := range []string{"index.html", "package.json"} {
			if f, err := root.Lstat(filepath.Join(rel, marker)); err == nil && f.Mode().IsRegular() {
				candidate = true
			}
		}
		if candidate {
			if !registered[rel] {
				found = append(found, HomepageDiscoveryCandidate{ProjectDir: rel, LocalRoot: filepath.Join(homepageWorkspaceRoot(cfg), filepath.FromSlash(rel))})
			}
			return filepath.SkipDir
		}
		return nil
	})
	sort.Slice(found, func(i, j int) bool { return found[i].ProjectDir < found[j].ProjectDir })
	return found, err
}

func DispatchHomepageInventory(cfg HomepageConfig, db *sql.DB, operation string, id int64, projectDir, query string, limit, offset int, reason string, logger *slog.Logger) (string, bool) {
	if (operation == "relocate" || operation == "adopt") && !HomepageOperationInProgress(cfg) {
		lock, _ := homepageOperationLocks.LoadOrStore(filepath.Clean(cfg.WorkspacePath), &sync.Mutex{})
		mu := lock.(*sync.Mutex)
		if err := lockToolHTTPContext(nativeHTTPContext(cfg.Context), mu); err != nil {
			return ErrorJSON(err.Error()), true
		}
		defer mu.Unlock()
	}
	output := map[string]interface{}{"status": "success"}
	switch operation {
	case "overview":
		sites, err := ListHomepageManagedSites(db)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		filtered := []HomepageManagedSite{}
		for _, site := range sites {
			if query == "" || strings.Contains(strings.ToLower(site.Name+" "+site.ProjectDir+" "+site.LastDeployURL), strings.ToLower(query)) {
				filtered = append(filtered, site)
			}
		}
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].ID < filtered[j].ID })
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		if offset < 0 {
			offset = 0
		}
		if offset > len(filtered) {
			offset = len(filtered)
		}
		end := min(offset+limit, len(filtered))
		output["sites"] = filtered[offset:end]
		output["total"] = len(filtered)
		output["offset"] = offset
		if end < len(filtered) {
			output["next_offset"] = end
		}
	case "get_site":
		if id <= 0 && projectDir != "" {
			p, err := GetProjectByDir(db, projectDir)
			if err != nil {
				return ErrorJSON(err.Error()), true
			}
			id = p.ID
		}
		site, err := GetHomepageManagedSite(db, id)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		output["site"] = site
	case "list_events":
		events, err := listHomepageSiteEvents(db, id, limit, 0, offset)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		output["events"] = events
		if len(events) > 0 {
			output["next_offset"] = offset + len(events)
		}
		if len(events) > 0 {
			output["last_event_id"] = events[0].ID
		}
	case "discover_workspace":
		candidates, err := DiscoverHomepageWorkspace(cfg, db)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		output["candidates"] = candidates
		output["total"] = len(candidates)
	case "relocate":
		if id <= 0 || strings.TrimSpace(reason) == "" {
			return ErrorJSON("id, new project_dir and reason are required for relocation"), true
		}
		site, err := GetHomepageManagedSite(db, id)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		dir, err := NormalizeHomepageProjectIdentity(projectDir, false)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		root, err := os.OpenRoot(cfg.WorkspacePath)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		info, err := root.Lstat(filepath.FromSlash(dir))
		root.Close()
		if err != nil || !info.IsDir() {
			return ErrorJSON("relocation requires an existing directory inside the configured workspace"), true
		}
		if other, err := GetProjectByDir(db, dir); err == nil && other.ID != id {
			return ErrorJSON("new project_dir belongs to another project"), true
		}
		tx, err := db.Begin()
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		defer tx.Rollback()
		for _, statement := range []struct {
			sql  string
			args []interface{}
		}{
			{"UPDATE homepage_projects SET project_dir=?,workspace_root=?,updated_at=CURRENT_TIMESTAMP WHERE id=?", []interface{}{dir, homepageWorkspaceRoot(cfg), id}},
			{"UPDATE homepage_revisions SET project_dir=? WHERE project_id=?", []interface{}{dir, id}},
			{"INSERT INTO homepage_project_state(project_id,local_root,drift_status,drift_message) VALUES(?,?,'local_changed','project location changed') ON CONFLICT(project_id) DO UPDATE SET local_root=excluded.local_root,drift_status=excluded.drift_status,drift_message=excluded.drift_message,updated_at=CURRENT_TIMESTAMP", []interface{}{id, filepath.Join(homepageWorkspaceRoot(cfg), filepath.FromSlash(dir))}},
		} {
			if _, err := tx.Exec(statement.sql, statement.args...); err != nil {
				return ErrorJSON(err.Error()), true
			}
		}
		payload, _ := json.Marshal(map[string]interface{}{"reason": security.RedactSensitiveInfo(reason), "previous_project_dir": site.ProjectDir, "project_dir": dir})
		if _, err := tx.Exec("INSERT INTO homepage_site_events(project_id,actor,source,event_type,summary,payload_json) VALUES(?,'agent','homepage_registry','project_relocated',?,?)", id, security.RedactSensitiveInfo(reason), string(payload)); err != nil {
			return ErrorJSON(err.Error()), true
		}
		if err := tx.Commit(); err != nil {
			return ErrorJSON(err.Error()), true
		}
		updated, err := GetHomepageManagedSite(db, id)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		output["site"] = updated
	case "audit_revisions":
		rows, err := db.Query("SELECT DISTINCT path FROM homepage_revision_files")
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		paths := []string{}
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				rows.Close()
				return ErrorJSON(err.Error()), true
			}
			if hereNowSensitivePath(path) {
				paths = append(paths, path)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		output["sensitive_paths"] = paths
		output["sensitive_path_count"] = len(paths)
		output["scan_scope"] = "credential filenames; no credential values returned"
	case "adopt":
		if strings.TrimSpace(reason) == "" {
			return ErrorJSON("reason is required to adopt a homepage project"), true
		}
		dir, err := NormalizeHomepageProjectIdentity(projectDir, false)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		root, err := os.OpenRoot(cfg.WorkspacePath)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		info, statErr := root.Lstat(filepath.FromSlash(dir))
		root.Close()
		if statErr != nil || !info.IsDir() {
			return ErrorJSON("adopt requires an existing project directory"), true
		}
		proj, err := EnsureHomepageProjectForDir(db, cfg, dir, "", "")
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		if _, err := db.Exec("UPDATE homepage_projects SET origin=CASE WHEN origin='unknown' THEN 'adopted' ELSE origin END,status='active' WHERE id=?", proj.ID); err != nil {
			return ErrorJSON(err.Error()), true
		}
		saved := SaveHomepageRevisionAndState(cfg, db, dir, "Adopt existing project", reason, "homepage_registry", map[string]interface{}{"actor": "agent", "reason": reason}, logger)
		if len(saved.Warnings) > 0 {
			return ErrorJSON(strings.Join(saved.Warnings, "; ")), true
		}
		if _, err := RecordHomepageEvent(db, proj.ID, "project_adopted", "homepage_registry", "Adopt existing project", map[string]interface{}{"reason": reason, "actor": "agent"}); err != nil {
			return ErrorJSON(err.Error()), true
		}
		site, err := GetHomepageManagedSite(db, proj.ID)
		if err != nil {
			return ErrorJSON(err.Error()), true
		}
		output["site"] = site
	default:
		return "", false
	}
	version, err := HomepageLedgerVersion(db)
	if err != nil {
		return ErrorJSON(err.Error()), true
	}
	output["ledger_version"] = version
	raw, _ := json.Marshal(output)
	return security.RedactSensitiveInfo(string(raw)), true
}
