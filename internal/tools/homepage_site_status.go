package tools

import (
	"database/sql"
	"errors"
	"fmt"
)

// Sentinel errors let callers map archive/restore failures to HTTP statuses.
var (
	ErrHomepageSiteNotFound     = errors.New("homepage managed site not found")
	ErrHomepageSiteArchived     = errors.New("homepage site already archived")
	ErrHomepageSiteNotArchived  = errors.New("homepage site is not archived")
	errHomepageSiteStatusNoteDB = errors.New("homepage registry DB not initialized")
)

// IsHomepageOverlapError reports whether err came from the project_dir overlap check.
func IsHomepageOverlapError(err error) bool {
	return errors.Is(err, errHomepageProjectOverlap)
}

func homepageSiteStatusLookup(db *sql.DB, id int64) (HomepageManagedSite, error) {
	if db == nil {
		return HomepageManagedSite{}, errHomepageSiteStatusNoteDB
	}
	sites, err := listHomepageManagedSites(db, id)
	if err != nil {
		return HomepageManagedSite{}, err
	}
	for _, site := range sites {
		if site.ID == id {
			return site, nil
		}
	}
	return HomepageManagedSite{}, fmt.Errorf("managed site %d not found: %w", id, ErrHomepageSiteNotFound)
}

func setHomepageSiteStatus(db *sql.DB, id int64, status, eventType, summary, actor, reason string) error {
	if _, err := db.Exec("UPDATE homepage_projects SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, id); err != nil {
		return err
	}
	if _, err := RecordHomepageEvent(db, id, eventType, "homepage_site_status", summary, map[string]interface{}{"reason": reason, "actor": actor}); err != nil {
		return fmt.Errorf("site status changed to %s but recording the %s event failed: %w", status, eventType, err)
	}
	return nil
}

// ArchiveHomepageSite marks a managed site archived; files, locations and history stay.
func ArchiveHomepageSite(db *sql.DB, id int64, actor, reason string) error {
	site, err := homepageSiteStatusLookup(db, id)
	if err != nil {
		return err
	}
	if site.Status == "archived" {
		return fmt.Errorf("managed site %d is already archived: %w", id, ErrHomepageSiteArchived)
	}
	return setHomepageSiteStatus(db, id, "archived", "site_archived", "Site archived", actor, reason)
}

// RestoreHomepageSite reactivates an archived site after re-checking overlap with active sites.
func RestoreHomepageSite(db *sql.DB, cfg HomepageConfig, id int64, actor, reason string) error {
	site, err := homepageSiteStatusLookup(db, id)
	if err != nil {
		return err
	}
	if site.Status != "archived" {
		return fmt.Errorf("managed site %d is not archived: %w", id, ErrHomepageSiteNotArchived)
	}
	if site.ProjectDir != "" {
		sites, err := ListHomepageManagedSites(db)
		if err != nil {
			return err
		}
		if err := homepageCheckProjectOverlap(sites, cfg, site.ProjectDir, id); err != nil {
			return err
		}
	}
	return setHomepageSiteStatus(db, id, "active", "site_restored", "Site restored", actor, reason)
}
