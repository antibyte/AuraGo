package tools

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Sentinel errors let callers map delete refusals to HTTP statuses.
var (
	// ErrHomepageSiteNested means another registered site shares or nests with the folder.
	ErrHomepageSiteNested = errors.New("homepage site folder overlaps another registered site")
	// ErrHomepageSiteFolderUnsafe means the folder identity cannot be deleted safely.
	ErrHomepageSiteFolderUnsafe = errors.New("homepage site folder cannot be deleted safely")
)

// HomepageSiteDeletion reports what DeleteHomepageSite removed.
type HomepageSiteDeletion struct {
	ID            int64            `json:"id"`
	Name          string           `json:"name"`
	ProjectDir    string           `json:"project_dir"`
	FolderRemoved bool             `json:"folder_removed"`
	RowsRemoved   map[string]int64 `json:"rows_removed"`
	TotalRows     int64            `json:"total_rows"`
}

// homepageSiteDeleteTables lists the per-project tables, children first. The
// registry has no content outside the database (revisions keep file bodies in
// homepage_revision_files); the artifact manifest lives inside the project folder.
var homepageSiteDeleteStatements = []struct{ table, query string }{
	{"homepage_remote_observations", "DELETE FROM homepage_remote_observations WHERE project_id = ?"},
	{"homepage_deployments", "DELETE FROM homepage_deployments WHERE project_id = ?"},
	{"homepage_deploy_targets", "DELETE FROM homepage_deploy_targets WHERE project_id = ?"},
	{"homepage_site_file_state", "DELETE FROM homepage_site_file_state WHERE project_id = ?"},
	{"homepage_project_state", "DELETE FROM homepage_project_state WHERE project_id = ?"},
	{"homepage_site_events", "DELETE FROM homepage_site_events WHERE project_id = ?"},
	{"homepage_history", "DELETE FROM homepage_history WHERE project_id = ?"},
	{"homepage_revision_files", "DELETE FROM homepage_revision_files WHERE revision_id IN (SELECT id FROM homepage_revisions WHERE project_id = ?)"},
	{"homepage_revisions", "DELETE FROM homepage_revisions WHERE project_id = ?"},
	{"homepage_projects", "DELETE FROM homepage_projects WHERE id = ?"},
}

// DeleteHomepageSite removes a managed site completely: its folder in the
// workspace (local sites) and every registry row. All safety checks run before
// anything is removed. If the folder cannot be removed the registry is left
// untouched so the site stays listed and the delete can be retried.
func DeleteHomepageSite(cfg HomepageConfig, db *sql.DB, id int64, actor string, logger *slog.Logger) (HomepageSiteDeletion, error) {
	if !HomepageOperationInProgress(cfg) {
		lock, _ := homepageOperationLocks.LoadOrStore(filepath.Clean(cfg.WorkspacePath), &sync.Mutex{})
		mu := lock.(*sync.Mutex)
		if err := lockToolHTTPContext(nativeHTTPContext(cfg.Context), mu); err != nil {
			return HomepageSiteDeletion{}, err
		}
		defer mu.Unlock()
	}
	site, err := homepageSiteStatusLookup(db, id)
	if err != nil {
		return HomepageSiteDeletion{}, err
	}
	result := HomepageSiteDeletion{ID: site.ID, Name: site.Name, ProjectDir: site.ProjectDir, RowsRemoved: map[string]int64{}}

	dir := ""
	if site.ProjectDir != "" {
		dir, err = homepageSiteDeletableDir(cfg, db, site)
		if err != nil {
			return result, err
		}
		removed, err := homepageRemoveSiteFolder(cfg, dir, logger)
		if err != nil {
			return result, err
		}
		result.FolderRemoved = removed
	}

	if err := homepageDeleteSiteRows(db, site.ID, dir, &result); err != nil {
		if result.FolderRemoved {
			return result, fmt.Errorf("the site folder %q was removed but the registry entry remains (retry deletes the rest): %w", dir, err)
		}
		return result, fmt.Errorf("delete registry rows: %w", err)
	}
	if logger != nil {
		logger.Info("[Homepage] Site deleted", "id", result.ID, "name", result.Name, "project_dir", result.ProjectDir,
			"actor", actor, "folder_removed", result.FolderRemoved, "rows", result.TotalRows)
	}
	return result, nil
}

// homepageSiteDeletableDir validates the folder of a local site and rejects
// overlaps with any other registered site, whatever its status.
func homepageSiteDeletableDir(cfg HomepageConfig, db *sql.DB, site HomepageManagedSite) (string, error) {
	if homepageWorkspaceRoot(cfg) == "" {
		return "", fmt.Errorf("%w: no workspace configured", ErrHomepageSiteFolderUnsafe)
	}
	dir, err := NormalizeHomepageProjectIdentity(site.ProjectDir, false)
	if err != nil || dir == "." {
		return "", fmt.Errorf("%w: project_dir %q is not a valid project folder", ErrHomepageSiteFolderUnsafe, site.ProjectDir)
	}
	if !homepageSiteInWorkspace(site, cfg) {
		return "", fmt.Errorf("%w: site %d belongs to a different workspace", ErrHomepageSiteFolderUnsafe, site.ID)
	}
	sites, err := ListHomepageManagedSites(db)
	if err != nil {
		return "", err
	}
	root, rootErr := os.OpenRoot(homepageWorkspaceRoot(cfg))
	if rootErr == nil {
		defer root.Close()
	}
	var own os.FileInfo
	if root != nil {
		own, _ = root.Stat(filepath.FromSlash(dir))
	}
	for _, other := range sites {
		if other.ID == site.ID || other.ProjectDir == "" || !homepageDeleteSameWorkspace(other, cfg) {
			continue
		}
		otherDir, nerr := NormalizeHomepageProjectIdentity(other.ProjectDir, true)
		if nerr != nil {
			return "", fmt.Errorf("%w: site %q (managed_site_id %d) has an unreadable folder %q (%v); fix or remove that entry first",
				ErrHomepageSiteFolderUnsafe, other.Name, other.ID, other.ProjectDir, nerr)
		}
		if otherDir == "." || homepageDirInside(otherDir, dir) || homepageDirInside(dir, otherDir) {
			return "", fmt.Errorf("%w: site %q (managed_site_id %d, folder %q) lies inside or contains %q; deleting would destroy its files",
				ErrHomepageSiteNested, other.Name, other.ID, other.ProjectDir, dir)
		}
		// Defense in depth: two spellings of one folder (aliases, links) are the same file.
		if own != nil && root != nil {
			if info, serr := root.Stat(filepath.FromSlash(otherDir)); serr == nil && os.SameFile(own, info) {
				return "", fmt.Errorf("%w: site %q (managed_site_id %d, folder %q) is the same folder as %q",
					ErrHomepageSiteNested, other.Name, other.ID, other.ProjectDir, dir)
			}
		}
	}
	return dir, nil
}

// homepageDeleteSameWorkspace is homepageSiteInWorkspace with the workspace roots
// compared through homepageFoldDir, so a case-variant root still counts (fail closed).
func homepageDeleteSameWorkspace(site HomepageManagedSite, cfg HomepageConfig) bool {
	if site.WorkspaceRoot == "" {
		return homepageSiteInWorkspace(site, cfg)
	}
	fold := func(p string) string { return homepageFoldDir(filepath.ToSlash(filepath.Clean(p))) }
	return fold(site.WorkspaceRoot) == fold(homepageWorkspaceRoot(cfg))
}

// homepageRemoveSiteFolder deletes dir below the workspace root without ever
// following a symlink out of it. A missing folder is not an error.
func homepageRemoveSiteFolder(cfg HomepageConfig, dir string, logger *slog.Logger) (bool, error) {
	root, err := os.OpenRoot(homepageWorkspaceRoot(cfg))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("open workspace: %w", err)
	}
	defer root.Close()
	rel := filepath.FromSlash(dir)
	if _, err := root.Lstat(rel); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("inspect site folder %q: %w", dir, err)
	}
	err = root.RemoveAll(rel)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, fs.ErrPermission) {
		return false, fmt.Errorf("remove site folder %q: %w", dir, err)
	}
	// Files written by the Docker container may be owned by another uid. The
	// repair only runs on non-Windows hosts and for a real directory, never a symlink.
	if info, lerr := root.Lstat(rel); runtime.GOOS == "windows" || lerr != nil || !info.IsDir() {
		return false, fmt.Errorf("remove site folder %q: %w; the registry entry was not changed", dir, err)
	}
	dockerCfg := DockerConfig{Context: cfg.Context, Host: cfg.DockerHost, WorkspaceDir: cfg.WorkspacePath}
	uidGid, repairErr := homepageDetectContainerUIDGID(dockerCfg)
	if repairErr == nil {
		raw := homepageDockerExecInternalFunc(dockerCfg, homepageContainerName, homepageProjectWritableRepairCommand(uidGid, dir), "0:0", nil)
		if code, out := homepageDockerExecResult(raw); code != 0 {
			repairErr = fmt.Errorf("permission repair failed with exit code %d: %s", code, truncateStr(out, 300))
		}
	}
	if err = root.RemoveAll(rel); err != nil {
		if repairErr != nil {
			return false, fmt.Errorf("remove site folder %q: %w (permission repair: %v); the registry entry was not changed", dir, err, repairErr)
		}
		return false, fmt.Errorf("remove site folder %q: %w; the registry entry was not changed", dir, err)
	}
	if logger != nil {
		logger.Info("[Homepage] Repaired permissions before deleting site folder", "project_dir", dir)
	}
	return true, nil
}

func homepageDeleteSiteRows(db *sql.DB, id int64, dir string, result *HomepageSiteDeletion) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	counts := map[string]int64{}
	for _, st := range homepageSiteDeleteStatements {
		res, err := tx.Exec(st.query, id)
		if err != nil {
			return fmt.Errorf("%s: %w", st.table, err)
		}
		n, _ := res.RowsAffected()
		counts[st.table] = n
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	var total int64
	for _, n := range counts {
		total += n
	}
	result.RowsRemoved = counts
	result.TotalRows = total
	return nil
}
