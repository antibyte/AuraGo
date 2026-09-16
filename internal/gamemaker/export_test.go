package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Publish a built fixture through the real revision store. Browser admission is
// covered independently; export tests must not fabricate a revision number.
func publishExportFixture(t *testing.T, s *Service, project Project, dir string) {
	t.Helper()
	stage := filepath.Join(t.TempDir(), "revision")
	if err := os.Rename(dir, stage); err != nil {
		t.Fatal(err)
	}
	if _, err := s.publish(context.Background(), stage, project, Job{ID: "export-fixture"}, "test", "Export fixture"); err != nil {
		t.Fatal(err)
	}
}

func readExportFixture(t *testing.T, s *Service, project Project) map[string][]byte {
	t.Helper()
	var output bytes.Buffer
	if _, err := s.WriteExport(context.Background(), project.ID, &output); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for _, f := range archive.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := files[f.Name]; exists {
			t.Fatalf("duplicate ZIP entry %s", f.Name)
		}
		files[f.Name] = data
	}
	return files
}

func TestExportUsesPublishedRevision(t *testing.T) {
	s := newTestService(t)
	p := createTestProject(t, s, "3d")
	dir := filepath.Join(s.opts.WorkspacePath, p.ProjectKey)
	if err := WriteScaffold(dir, p); err != nil {
		t.Fatal(err)
	}
	if result := buildDirectory(context.Background(), dir, 100, 20<<20); !result.OK {
		t.Fatal(result.Diagnostics)
	}
	publishExportFixture(t, s, p, dir)
	want := readExportFixture(t, s, p)
	// Code Studio edits and files left by a concurrent publication must not mix
	// unvalidated source with the last successfully compiled bundle.
	for _, name := range []string{"src/main.ts", "dist/game.js", "scratch.txt", ".env"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("unpublished edit"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got := readExportFixture(t, s, p)
	if len(got) != len(want) {
		t.Fatalf("export included unpublished files: %d entries, want %d", len(got), len(want))
	}
	for name, data := range want {
		if !bytes.Equal(got[name], data) {
			t.Errorf("export changed without publication: %s", name)
		}
	}
	if err := os.Rename(dir, dir+"-during-publication"); err != nil {
		t.Fatal(err)
	}
	readExportFixture(t, s, p)
}

func TestExportRejectsInvalidRevision(t *testing.T) {
	for _, failure := range []string{"missing-bundle", "missing-blob", "damaged-blob", "unsafe-path"} {
		t.Run(failure, func(t *testing.T) {
			s := newTestService(t)
			p := createTestProject(t, s, "2d")
			dir := filepath.Join(s.opts.WorkspacePath, p.ProjectKey)
			if err := WriteScaffold(dir, p); err != nil {
				t.Fatal(err)
			}
			if result := buildDirectory(context.Background(), dir, 100, 20<<20); !result.OK {
				t.Fatal(result.Diagnostics)
			}
			publishExportFixture(t, s, p, dir)
			switch failure {
			case "missing-bundle":
				if _, err := s.db.Exec(`DELETE FROM gm_revision_files WHERE path='dist/game.js'`); err != nil {
					t.Fatal(err)
				}
			case "missing-blob", "damaged-blob":
				var hash string
				if err := s.db.QueryRow(`SELECT content_hash FROM gm_revision_files WHERE path='dist/game.js'`).Scan(&hash); err != nil {
					t.Fatal(err)
				}
				blob := filepath.Join(s.blobDir, hash[:2], hash)
				var err error
				if failure == "missing-blob" {
					err = os.Remove(blob)
				} else {
					err = os.WriteFile(blob, []byte("damaged"), 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
			case "unsafe-path":
				if _, err := s.db.Exec(`UPDATE gm_revision_files SET path='../escape.js' WHERE path='src/main.ts'`); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			if _, err := s.WriteExport(context.Background(), p.ID, &output); err == nil {
				t.Fatalf("export silently accepted %s", failure)
			} else if strings.Contains(err.Error(), s.blobDir) {
				t.Fatalf("export error disclosed the host path: %v", err)
			}
		})
	}
}

func TestExportLegacyRuntimeAndPrivateFiles(t *testing.T) {
	s := newTestService(t)
	p := createTestProject(t, s, "3d")
	dir := filepath.Join(s.opts.WorkspacePath, p.ProjectKey)
	if err := WriteScaffold(dir, p); err != nil {
		t.Fatal(err)
	}
	if result := buildDirectory(context.Background(), dir, 100, 20<<20); !result.OK {
		t.Fatal(result.Diagnostics)
	}
	if err := os.Remove(filepath.Join(dir, "vendor", "three.core.min.js")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".env", ".aurago/game-plan.json", ".aurago/validation-report.json", "assets/.private/state"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("private state"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	publishExportFixture(t, s, p, dir)
	files := readExportFixture(t, s, p)
	for name := range files {
		if strings.HasPrefix(name, ".") || strings.Contains(name, "/.") || strings.Contains(name, "preview-tests") {
			t.Fatalf("private file exported: %s", name)
		}
	}
	if len(files["vendor/three.core.min.js"]) == 0 {
		t.Fatal("missing legacy runtime fallback")
	}
}
