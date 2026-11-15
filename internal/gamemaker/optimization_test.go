package gamemaker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceBatchPreflightSearchAndRuntime(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "building" {
			return errors.New("unexpected repair")
		}
		if err := s.UseTemplate(ctx, run.Job.ID, "minimal"); err != nil {
			return err
		}
		for path, source := range map[string]string{"src/main.ts": "import {value} from './value'; console.log(value);", "src/value.ts": "export const value = 1;"} {
			if err := s.writeJobFile(ctx, run.Job.ID, path, source); err != nil {
				return err
			}
		}
		a, _ := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		b, _ := s.ReadJobFile(ctx, run.Job.ID, "src/value.ts")
		newA, newB := "console.log(value + 2)", "export const value = 2;"
		edits := []SourceReplacement{{"src/main.ts", sourceHash(a), "console.log(value)", &newA}, {"src/value.ts", "stale", b, &newB}}
		if _, err := s.ReplaceJobFiles(ctx, run.Job.ID, edits); err == nil {
			t.Error("stale batch accepted")
		}
		if got, _ := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts"); got != a {
			t.Error("first file mutated before second precondition")
		}
		edits[1].ExpectedSHA256 = sourceHash(b)
		edits[1].NewText = &newB
		written, err := s.ReplaceJobFiles(ctx, run.Job.ID, edits)
		if err != nil || !written.Written || written.Build.OK || len(written.Files) != 2 {
			t.Errorf("batch: %+v %v", written, err)
		}
		search, err := s.SearchJobFile(ctx, run.Job.ID, "src/main.ts", "console.log")
		if err != nil || len(search.Matches) != 1 || !strings.Contains(search.Matches[0].Text, "value + 2") || search.SHA256 != written.Files[0].SHA256 {
			t.Errorf("search: %+v %v", search, err)
		}
		if _, err := s.SearchJobFile(ctx, run.Job.ID, "../escape", "value"); err == nil {
			t.Error("search escaped job")
		}
		if s.FrameworkReference(ctx, run.Job.ID, "phaser")["bundled_helper_version"] != "phaser-2" {
			t.Error("installed runtime not described")
		}
		if err := s.writeJobFile(ctx, run.Job.ID, "src/common.ts", "export const custom = 1;"); err != nil {
			return err
		}
		if s.RuntimeContext(ctx, run.Job.ID)["version"] != "legacy/custom" {
			t.Error("new helpers advertised to legacy source")
		}
		return errors.New("verified optimization")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "verified optimization" {
		t.Fatal(done.Error)
	}
}

func TestPrivateBuildSourceDiagnostics(t *testing.T) {
	root := t.TempDir()
	if err := WriteScaffold(root, Project{Name: "diagnostics", Dimension: "3d"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "src/main.ts")
	if err := os.WriteFile(path, []byte("function fail() { throw new Error('specific_failure'); }\nfail();"), 0600); err != nil {
		t.Fatal(err)
	}
	result := buildDirectory(context.Background(), root, 100, 64<<20)
	if !result.OK || result.sourceMap == nil {
		t.Fatalf("build/map: %+v", result)
	}
	bundle, _ := os.ReadFile(filepath.Join(root, "dist/game.js"))
	var frames []RuntimeFrame
	for i, line := range strings.Split(string(bundle), "\n") {
		if column := strings.Index(line, "throw new Error"); column >= 0 {
			frames = []RuntimeFrame{{"dist/game.js", i + 1, column + 1}}
			break
		}
	}
	d := result.sourceMap.diagnostic("specific_failure", frames)
	current, _ := os.ReadFile(path)
	if d.File != "src/main.ts" || d.Line == 0 || d.SourceSHA256 != sourceHash(string(current)) || !strings.Contains(d.Excerpt, "specific_failure") {
		t.Fatalf("unmapped location: %+v", d)
	}
	if _, err := os.Stat(filepath.Join(root, "dist/game.js.map")); !os.IsNotExist(err) {
		t.Fatal("source map published to disk")
	}
	if strings.Contains(string(bundle), "sourceMappingURL") {
		t.Fatal("private map referenced by public bundle")
	}
	if err := os.WriteFile(path, append(current, []byte("\n// changed")...), 0600); err != nil {
		t.Fatal(err)
	}
	if stale := result.sourceMap.diagnostic("specific_failure", frames); stale.File != "" {
		t.Fatal("mapped stale source")
	}
	if bad := result.sourceMap.diagnostic("x", []RuntimeFrame{{"../../private", 1, 1}}); bad.File != "" {
		t.Fatal("mapped foreign source")
	}
}
