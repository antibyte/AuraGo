package tools

import (
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// errHomepageProjectOverlap marks a project_dir that would nest with another registered site.
var errHomepageProjectOverlap = errors.New("homepage project overlap")

type homepageOverlapError struct{ msg string }

func (e *homepageOverlapError) Error() string { return e.msg }
func (e *homepageOverlapError) Unwrap() error { return errHomepageProjectOverlap }

// homepageFoldsCase reports whether the host file system treats names case-insensitively.
// It is a variable so tests can force both modes.
var homepageFoldsCase = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// homepageFoldDir folds case only on case-insensitive platforms.
func homepageFoldDir(dir string) string {
	if homepageFoldsCase {
		return strings.ToLower(dir)
	}
	return dir
}

// homepageDirInside reports whether inner equals outer or lies below it (folded).
func homepageDirInside(inner, outer string) bool {
	inner, outer = homepageFoldDir(inner), homepageFoldDir(outer)
	return inner == outer || strings.HasPrefix(inner, outer+"/")
}

// homepageCheckProjectOverlap rejects a dir that is an ancestor or descendant of another
// non-archived local site of the same workspace. excludeID skips the site being moved.
func homepageCheckProjectOverlap(sites []HomepageManagedSite, cfg HomepageConfig, dir string, excludeID int64) error {
	if dir == "" {
		return nil
	}
	for _, site := range sites {
		if site.ID == excludeID || site.ProjectDir == "" || site.Status == "archived" || !homepageSiteInWorkspace(site, cfg) {
			continue
		}
		switch {
		case homepageFoldDir(site.ProjectDir) == homepageFoldDir(dir):
			return &homepageOverlapError{fmt.Sprintf("project_dir %q belongs to the registered site %q (managed_site_id %d); use that site's managed_site_id", dir, site.ProjectDir, site.ID)}
		case homepageDirInside(site.ProjectDir, dir):
			return &homepageOverlapError{fmt.Sprintf("project_dir %q would contain the registered site %q (managed_site_id %d); use that site's managed_site_id, or choose a project_dir outside it", dir, site.ProjectDir, site.ID)}
		case homepageDirInside(dir, site.ProjectDir):
			return &homepageOverlapError{fmt.Sprintf("project_dir %q lies inside the registered site %q (managed_site_id %d); edit that site instead", dir, site.ProjectDir, site.ID)}
		}
	}
	return nil
}

// homepageSiteLookupError turns a missing row into guidance for the agent.
func homepageSiteLookupError(err error, id int64) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("managed site %d not found; list sites with homepage_registry overview", id)
	}
	return err
}

// homepageFreeProjectName returns base, or base-2 .. base-50 when the name is taken.
func homepageFreeProjectName(db *sql.DB, base string) (string, error) {
	for i := 1; i <= 50; i++ {
		candidate := base
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		var one int
		err := db.QueryRow("SELECT 1 FROM homepage_projects WHERE name=?", candidate).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("homepage project name %q and its numbered variants are all taken; pass an explicit name", base)
}
