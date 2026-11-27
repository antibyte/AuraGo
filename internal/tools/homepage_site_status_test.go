package tools

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHomepageArchiveRestoreSite(t *testing.T) {
	setHomepageFoldCase(t, false)
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	os.MkdirAll(filepath.Join(cfg.WorkspacePath, "a", "inner"), 0755)
	a := identityTestEnsure(t, db, cfg, "a")

	if err := ArchiveHomepageSite(db, 9999, "user", "x"); !errors.Is(err, ErrHomepageSiteNotFound) {
		t.Fatalf("unknown id archive = %v", err)
	}
	if err := RestoreHomepageSite(db, cfg, 9999, "user", "x"); !errors.Is(err, ErrHomepageSiteNotFound) {
		t.Fatalf("unknown id restore = %v", err)
	}
	if err := RestoreHomepageSite(db, cfg, a.ID, "user", "x"); !errors.Is(err, ErrHomepageSiteNotArchived) {
		t.Fatalf("restore active = %v", err)
	}
	if err := ArchiveHomepageSite(db, a.ID, "user", "done"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if err := ArchiveHomepageSite(db, a.ID, "user", "again"); !errors.Is(err, ErrHomepageSiteArchived) {
		t.Fatalf("double archive = %v", err)
	}
	site, err := GetHomepageManagedSite(db, a.ID)
	if err != nil || site.Status != "archived" {
		t.Fatalf("status after archive = %q, %v", site.Status, err)
	}

	// An active site nested inside the archived one blocks the restore.
	identityTestEnsure(t, db, cfg, "a/inner")
	if err := RestoreHomepageSite(db, cfg, a.ID, "user", "back"); !IsHomepageOverlapError(err) {
		t.Fatalf("restore with overlap = %v", err)
	}
	if site, _ := GetHomepageManagedSite(db, a.ID); site.Status != "archived" {
		t.Fatalf("overlap refusal changed status to %q", site.Status)
	}

	b := identityTestEnsure(t, db, cfg, "b")
	if err := ArchiveHomepageSite(db, b.ID, "user", "later"); err != nil {
		t.Fatal(err)
	}
	if err := RestoreHomepageSite(db, cfg, b.ID, "user", "back"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	site, _ = GetHomepageManagedSite(db, b.ID)
	if site.Status != "active" {
		t.Fatalf("status after restore = %q", site.Status)
	}
	kinds := map[string]bool{}
	for _, e := range site.Events {
		kinds[e.EventType] = true
		if (e.EventType == "site_archived" || e.EventType == "site_restored") && e.Actor != "user" {
			t.Fatalf("event %s actor = %q", e.EventType, e.Actor)
		}
	}
	if !kinds["site_archived"] || !kinds["site_restored"] {
		t.Fatalf("events missing: %v", kinds)
	}
}
