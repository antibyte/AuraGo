package gamemaker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A project-wide search locates the owning file without scanning managed
// runtimes, compiled output, assets or internal state.
func TestProjectSearchCoversOnlyEditableSources(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	var found, bounded, single ProjectSearch
	var invalid error
	service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
		if run.Stage != "building" {
			return nil
		}
		stage, err := service.JobDirectory(run.Job.ID)
		if err != nil {
			return err
		}
		marker := "auragoSearchMarker"
		for path, content := range map[string]string{
			"src/enemies.ts":          "export const patrol = 1; // " + marker + "\n",
			"src/levels.json":         "{\"note\":\"" + marker + "\"}\n",
			"vendor/extra.js":         "// " + marker + "\n",
			"dist/game.js":            "// " + marker + "\n",
			"assets/readme.txt":       marker + "\n",
			".aurago/notes.json":      "{\"note\":\"" + marker + "\"}\n",
			"src/binary.json":         marker + "\x00\n",
			"src/repeated.ts":         strings.Repeat("// "+marker+"\n", 30),
			"src/deep/nested/more.ts": "// " + marker + " nested\n",
		} {
			target := filepath.Join(stage, filepath.FromSlash(path))
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return err
			}
			if err := os.WriteFile(target, []byte(content), 0o640); err != nil {
				return err
			}
		}
		if found, err = service.SearchJobFiles(ctx, run.Job.ID, "patrol = 1"); err != nil {
			return err
		}
		if bounded, err = service.SearchJobFiles(ctx, run.Job.ID, marker); err != nil {
			return err
		}
		if single, err = service.SearchJobFiles(ctx, run.Job.ID, "extends GameScene"); err != nil {
			return err
		}
		_, invalid = service.SearchJobFiles(ctx, run.Job.ID, "two\nlines")
		return errors.New("fixture complete")
	}})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, service, job.ID)

	if len(found.Files) != 1 || found.Files[0].Path != "src/enemies.ts" || len(found.Files[0].Matches) != 1 || found.Files[0].Matches[0].Line != 1 || len(found.Files[0].SHA256) != 64 || found.Truncated {
		t.Fatalf("targeted search = %+v", found)
	}
	total := 0
	for _, file := range bounded.Files {
		total += len(file.Matches)
		for _, prefix := range []string{"vendor/", "dist/", "assets/", ".aurago/"} {
			if strings.HasPrefix(file.Path, prefix) {
				t.Errorf("search scanned managed path %s", file.Path)
			}
		}
		if file.Path == "src/binary.json" {
			t.Error("search scanned a binary file")
		}
	}
	if total != maxSearchMatches || !bounded.Truncated {
		t.Fatalf("bounded search returned %d matches, truncated=%v: %+v", total, bounded.Truncated, bounded)
	}
	owner := false
	for _, file := range single.Files {
		owner = owner || file.Path == "src/main.ts"
	}
	if !owner {
		t.Fatalf("starter search did not locate src/main.ts: %+v", single)
	}
	if invalid == nil {
		t.Fatal("multi-line query was accepted")
	}
	if found.SearchedFiles < 3 || found.SearchedFiles > maxSearchedFiles {
		t.Fatalf("searched %d files", found.SearchedFiles)
	}
}
