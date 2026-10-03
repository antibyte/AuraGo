package gamemaker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishCommitFailurePreservesStageAndPublishedProject(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	makeStage := func(id, content string) string {
		t.Helper()
		stage := filepath.Join(s.stagingDir, id)
		if err := os.MkdirAll(stage, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stage, "marker.txt"), []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
		return stage
	}
	stage1 := makeStage("original", "published")
	if _, err := s.publish(context.Background(), stage1, project, Job{ID: "original"}, "test", "original"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TABLE gm_commit_parent(id INTEGER PRIMARY KEY);
		CREATE TABLE gm_commit_child(id INTEGER REFERENCES gm_commit_parent(id) DEFERRABLE INITIALLY DEFERRED);
		CREATE TRIGGER gm_commit_fail AFTER UPDATE OF current_revision ON gm_projects
		BEGIN INSERT INTO gm_commit_child(id) VALUES(42); END`); err != nil {
		t.Fatal(err)
	}
	stage2 := makeStage("candidate", "staged")
	if _, err := s.publish(context.Background(), stage2, project, Job{ID: "candidate"}, "test", "candidate"); err == nil || !strings.Contains(err.Error(), "commit game maker revision") {
		t.Fatalf("publish error = %v, want commit failure", err)
	}
	for path, want := range map[string]string{
		filepath.Join(stage2, "marker.txt"):                                   "staged",
		filepath.Join(s.opts.WorkspacePath, project.ProjectKey, "marker.txt"): "published",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
}
