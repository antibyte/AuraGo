package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageInitTargetCheck(t *testing.T) {
	ws := t.TempDir()
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := HomepageConfig{WorkspacePath: ws}
	if err := HomepageInitTargetCheck(cfg, db, "fresh"); err != nil {
		t.Fatalf("missing dir must pass: %v", err)
	}
	os.MkdirAll(filepath.Join(ws, "empty"), 0o755)
	if err := HomepageInitTargetCheck(cfg, db, "empty"); err != nil {
		t.Fatalf("empty dir must pass: %v", err)
	}
	os.MkdirAll(filepath.Join(ws, "full"), 0o755)
	os.WriteFile(filepath.Join(ws, "full", "index.html"), []byte("keep"), 0o644)
	id, _, _ := RegisterProject(db, HomepageProject{Name: "full", ProjectDir: "full"})
	err = HomepageInitTargetCheck(cfg, db, "full")
	if err == nil || !strings.Contains(err.Error(), "not empty") || !strings.Contains(err.Error(), "managed_site_id") {
		t.Fatalf("non-empty dir must fail with site id, got %v (id %d)", err, id)
	}
	out := HomepageInitProject(cfg, "html", "full", "", slogDiscard())
	if !strings.Contains(out, "already exists") {
		t.Fatalf("HomepageInitProject must refuse: %s", out)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, "full", "index.html")); string(b) != "keep" {
		t.Fatal("existing index.html was overwritten")
	}
}

func TestHomepageLighthouseSiteAttribution(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a, _, _ := RegisterProject(db, HomepageProject{Name: "a", ProjectDir: "a", URL: "https://Example.com/"})
	b, _, _ := RegisterProject(db, HomepageProject{Name: "b", ProjectDir: "b", URL: "https://example.com/blog"})
	c, _, _ := RegisterProject(db, HomepageProject{Name: "c", ProjectDir: "c", URL: "https://dup.test"})
	d, _, _ := RegisterProject(db, HomepageProject{Name: "d", ProjectDir: "d", URL: "https://dup.test/"})
	if id, ok := HomepageLighthouseSite(db, 0, " https://example.COM "); !ok || id != a {
		t.Fatalf("exact match: %d %v want %d", id, ok, a)
	}
	if _, ok := HomepageLighthouseSite(db, 0, "example.com"); ok {
		t.Fatal("substring URL must not match")
	}
	if _, ok := HomepageLighthouseSite(db, 0, "https://dup.test"); ok {
		t.Fatalf("ambiguous (%d,%d) must not match", c, d)
	}
	if id, ok := HomepageLighthouseSite(db, b, "whatever"); !ok || id != b {
		t.Fatal("explicit managed_site_id must win")
	}
	if _, ok := HomepageLighthouseSite(db, 9999, "https://example.com"); ok {
		t.Fatal("unknown managed_site_id must not store")
	}
}

func TestHomepageDeployedEventCarriesOperation(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, _, _ := RegisterProject(db, HomepageProject{Name: "a", ProjectDir: "a"})
	if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{ProjectID: id, Provider: "local", URL: "http://x", OperationID: "hp-op1", Actor: "agent"}); err != nil {
		t.Fatal(err)
	}
	var actor, corr string
	if err := db.QueryRow("SELECT actor, correlation_id FROM homepage_site_events WHERE project_id=? AND event_type='deployed'", id).Scan(&actor, &corr); err != nil || actor != "agent" || corr != "hp-op1" {
		t.Fatalf("deployed event actor=%q corr=%q err=%v", actor, corr, err)
	}
}

func TestDeleteHomepageHistoryEntryRecorded(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	pid, _, _ := RegisterProject(db, HomepageProject{Name: "a", ProjectDir: "a"})
	hid, _ := AddHomepageHistoryEntry(db, pid, "decision", strings.Repeat("x", 300), "t", nil)
	if err := DeleteHomepageHistoryEntryRecorded(db, hid, "agent", "cleanup"); err != nil {
		t.Fatal(err)
	}
	var payload string
	if err := db.QueryRow("SELECT payload_json FROM homepage_site_events WHERE project_id=? AND event_type='history_deleted'", pid).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, "cleanup") || !strings.Contains(payload, `"entry_type":"decision"`) || strings.Contains(payload, strings.Repeat("x", 200)) {
		t.Fatalf("payload: %s", payload)
	}
	if _, err := GetHomepageHistoryEntry(db, hid); err == nil {
		t.Fatal("entry still present")
	}
}

func TestHomepageRegisterPrecheck(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	site, _, _ := RegisterProject(db, HomepageProject{Name: "Shop", ProjectDir: "Shop", Status: "active"})
	old := homepageFoldsCase
	defer func() { homepageFoldsCase = old }()
	homepageFoldsCase = true
	if id, err := HomepageRegisterPrecheck(db, cfg, "shop"); err != nil || id != site {
		t.Fatalf("folded duplicate: %d %v", id, err)
	}
	if _, err := HomepageRegisterPrecheck(db, cfg, "Shop/sub"); err == nil || !strings.Contains(err.Error(), "lies inside") {
		t.Fatalf("descendant must be rejected: %v", err)
	}
	if id, err := HomepageRegisterPrecheck(db, cfg, "other"); err != nil || id != 0 {
		t.Fatalf("unrelated: %d %v", id, err)
	}
}
