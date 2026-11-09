package tools

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestHomepageInventoryHashOnlyPreservesMetadataWithoutContents(t *testing.T) {
	dir := t.TempDir()
	fixtures := map[string][]byte{
		"index.html": []byte("<html><body>A public page</body></html>"),
		"asset.bin":  {0, 1, 2, 3},
		"large.bin":  append([]byte{0}, bytes.Repeat([]byte{'x'}, maxRevisionFileSize+64)...),
	}
	for name, data := range fixtures {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("excluded"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	hasLink := os.Symlink(outside, filepath.Join(dir, "link.txt")) == nil
	full, _, err := enumerateProjectFiles(dir, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	metadata, skipped, err := enumerateProjectFiles(dir, nil, false)
	if err != nil || len(metadata) != len(fixtures) || len(full) != len(metadata) {
		t.Fatalf("inventory count/error = %d, %v", len(metadata), err)
	}
	for i, f := range metadata {
		if f.Data != nil || f.Hash != hashContent(fixtures[f.Path]) || f.Hash != full[i].Hash ||
			f.Path != full[i].Path || f.Size != full[i].Size || f.IsBinary != full[i].IsBinary ||
			f.Mode != full[i].Mode || !f.ModTime.Equal(full[i].ModTime) {
			t.Fatalf("hash-only inventory changed metadata or retained content for %s", f.Path)
		}
	}
	if hasLink && (len(skipped) != 1 || skipped[0] != "link.txt") {
		t.Fatalf("symlink was not excluded: %v", skipped)
	}
	before := metadata[1]
	if before.Path != "index.html" {
		t.Fatalf("unexpected fixture order: %s", before.Path)
	}
	changed := bytes.ReplaceAll(fixtures["index.html"], []byte("public"), []byte("edited"))
	if err := os.WriteFile(filepath.Join(dir, "index.html"), changed, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, "index.html"), before.ModTime, before.ModTime); err != nil {
		t.Fatal(err)
	}
	after, _, err := enumerateProjectFiles(dir, nil, false)
	if err != nil || after[1].Hash == before.Hash || after[1].Hash != hashContent(changed) {
		t.Fatalf("hash-only inventory missed an edit preserving size and mtime: %v", err)
	}
}

func TestHomepageInventoryRevisionStillExcludesCredentials(t *testing.T) {
	dir := t.TempDir()
	data := []byte("TEST_API_KEY=\"" + strings.Repeat("aB9", 16) + "\"\n")
	if security.Scrub(security.RedactSensitiveInfo(string(data))) == string(data) {
		t.Fatal("synthetic fixture must exercise credential exclusion")
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"), data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, contents := range []bool{false, true} {
		files, _, err := enumerateProjectFiles(dir, nil, contents)
		if err != nil || len(files) != 1 || files[0].Data != nil || files[0].Hash != hashContent(data) {
			t.Fatalf("inventory mode %v retained credential content or changed its hash: %v", contents, err)
		}
	}
}

func TestHomepageRevisionPreflightDetectsSensitiveEditsWithPreservedMetadata(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	cfg := HomepageConfig{WorkspacePath: t.TempDir()}
	dir := filepath.Join(cfg.WorkspacePath, "site")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	credential := []byte("TEST_API_KEY=\"" + strings.Repeat("aB9", 16) + "\"\n")
	path := filepath.Join(dir, "index.js")
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, len(credential)), 0600); err != nil {
		t.Fatal(err)
	}
	first := SaveHomepageRevisionAndState(cfg, db, "site", "Initial", "test", "test", nil, nil)
	if first.RevisionID <= 0 || len(first.Warnings) > 0 {
		t.Fatal("initial revision failed")
	}
	unchanged := decodeLedgerTestJSON(t, HomepageSaveRevision(cfg, db, "site", "Unchanged", "test", nil))
	if unchanged["status"] != "ok" || unchanged["revision_id"] != nil {
		t.Fatal("unchanged save did not retain the no-change response")
	}
	var revisions int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_revisions").Scan(&revisions); err != nil || revisions != 1 {
		t.Fatal("unchanged save created a revision", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, credential, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	saved := SaveHomepageRevisionAndState(cfg, db, "site", "Changed", "test", "test", nil, nil)
	if saved.RevisionID <= first.RevisionID || len(saved.Warnings) > 0 {
		t.Fatal("hash preflight missed a timestamp-preserving edit")
	}
	var content, hash string
	if err := db.QueryRow("SELECT content_after, content_hash_after FROM homepage_revision_files WHERE revision_id = ?", saved.RevisionID).Scan(&content, &hash); err != nil || content != "" || hash != hashContent(credential) {
		t.Fatal("changed revision retained credential content or lost its hash", err)
	}
}

func BenchmarkHomepageSaveUnchangedRevision(b *testing.B) {
	db, err := InitHomepageRegistryDB(filepath.Join(b.TempDir(), "homepage.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	cfg := HomepageConfig{WorkspacePath: b.TempDir()}
	dir := filepath.Join(cfg.WorkspacePath, "site")
	if err := os.Mkdir(dir, 0700); err != nil {
		b.Fatal(err)
	}
	data := bytes.Repeat([]byte("The public page describes a quiet garden with trees and flowers.\n"), 4096)
	if err := os.WriteFile(filepath.Join(dir, "index.html"), data, 0600); err != nil {
		b.Fatal(err)
	}
	initial := SaveHomepageRevisionAndState(cfg, db, "site", "Initial", "test", "test", nil, nil)
	if initial.RevisionID <= 0 || len(initial.Warnings) > 0 {
		b.Fatal("initial revision failed")
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if result := HomepageSaveRevision(cfg, db, "site", "Unchanged", "test", nil); !strings.Contains(result, `"revision_id":null`) {
			b.Fatal("unchanged save did not retain the no-change response")
		}
	}
}

func BenchmarkHomepageInventory(b *testing.B) {
	for _, mode := range []struct {
		name     string
		contents bool
	}{{"revision_contents", true}, {"hash_only", false}} {
		b.Run(mode.name, func(b *testing.B) {
			dir := b.TempDir()
			data := bytes.Repeat([]byte("The public page describes a quiet garden with trees and flowers.\n"), 4096)
			if err := os.WriteFile(filepath.Join(dir, "index.html"), data, 0600); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				files, _, err := enumerateProjectFiles(dir, nil, mode.contents)
				if err != nil || len(files) != 1 || files[0].Hash != hashContent(data) || (!mode.contents && files[0].Data != nil) {
					b.Fatalf("inventory failed: %v", err)
				}
			}
		})
	}
}
