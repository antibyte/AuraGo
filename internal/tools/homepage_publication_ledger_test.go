package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomepageLedgerDoesNotPromoteFailedVerification(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	for _, result := range []string{`{"status":"partial_success","verified":false,"deploy_id":"uploaded"}`, `{"status":"ok","verified":false,"deploy_id":"uploaded"}`} {
		if _, err := RecordHomepageDeploymentFromResultStrict(cfg, db, "site", "netlify", ".", result, slogDiscard()); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_deployments").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unverified upload became a successful deployment")
	}
}

func TestHomepageRevisionRetainsSupportedDocument(t *testing.T) {
	for _, size := range []int{626 * 1024, maxHomepageWriteFileSize, maxHomepageWriteFileSize + 1} {
		dir := t.TempDir()
		data := []byte(strings.Repeat("x", size))
		if err := os.WriteFile(filepath.Join(dir, "index.html"), data, 0600); err != nil {
			t.Fatal(err)
		}
		files, _, err := enumerateProjectFiles(dir, slogDiscard(), true)
		if err != nil || len(files) != 1 {
			t.Fatalf("enumerate supported document: %v %d", err, len(files))
		}
		if size <= maxHomepageWriteFileSize && string(files[0].Data) != string(data) {
			t.Fatalf("supported %d-byte document lost its revision content", size)
		}
		if size > maxHomepageWriteFileSize && files[0].Data != nil {
			t.Fatal("oversized revision retained unbounded content")
		}
	}
}

func TestHomepageArtifactHashIgnoresPrivateBackups(t *testing.T) {
	workspace := t.TempDir()
	project := filepath.Join(workspace, "site")
	writeNetlifyDeployFiles(t, project, map[string]string{"index.html": "<html>site</html>", "index.html.bak-pre325": "before", ".scripts/repair.py": "private"})
	cfg := HomepageConfig{WorkspacePath: workspace}
	manifest, err := BuildHomepageArtifactManifest(cfg, 0, "site", 0, "", ".")
	if err != nil || len(manifest.Files) != 1 {
		t.Fatalf("private artifacts entered deployment hash: %+v %v", manifest, err)
	}
	if err := os.WriteFile(filepath.Join(project, "index.html.bak-pre325"), []byte("changed backup"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := BuildHomepageArtifactManifest(cfg, 0, "site", 0, "", ".")
	if err != nil || after.ArtifactHash != manifest.ArtifactHash {
		t.Fatal("backup edit changed publication identity")
	}
}
