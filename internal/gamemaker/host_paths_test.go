package gamemaker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func leaksHostPath(t *testing.T, service *Service, message string) bool {
	t.Helper()
	normalized := strings.ToLower(filepath.ToSlash(message))
	for _, root := range []string{service.stagingDir, service.blobDir, service.opts.WorkspacePath, filepath.Dir(service.opts.DBPath), os.TempDir()} {
		abs, err := filepath.Abs(root)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(normalized, strings.ToLower(filepath.ToSlash(abs))) {
			return true
		}
	}
	return false
}

// File errors reach the model, Studio and the job ledger. They must name the
// project-relative path and explain the next step, never the host location.
func TestAgentFileErrorsStayProjectRelative(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	messages := map[string]string{}
	service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
		if run.Stage != "building" {
			return nil
		}
		record := func(name string, err error) {
			if err == nil {
				messages[name] = ""
				return
			}
			messages[name] = err.Error()
		}
		_, err := service.ReadJobFile(ctx, run.Job.ID, "src/missing.ts")
		record("read", err)
		// Existing callers distinguish a missing file from other failures.
		if !os.IsNotExist(err) || !errors.Is(err, os.ErrNotExist) {
			return errors.New("a missing file is no longer recognisable: " + err.Error())
		}
		_, err = service.ReadJobFileRange(ctx, run.Job.ID, "src/missing.ts", 1, 20)
		record("range", err)
		_, err = service.SearchJobFile(ctx, run.Job.ID, "src/missing.ts", "player")
		record("search", err)
		_, err = service.ReplaceJobFile(ctx, run.Job.ID, "src/missing.ts", "a", "b", "0000")
		record("replace", err)
		replacement := "b"
		_, err = service.ReplaceJobFiles(ctx, run.Job.ID, []SourceReplacement{{Path: "src/missing.ts", ExpectedSHA256: "0000", OldText: "a", NewText: &replacement}})
		record("replace_many", err)
		// A directory where a file is expected produces a different OS error.
		stage, stageErr := service.JobDirectory(run.Job.ID)
		if stageErr != nil {
			return stageErr
		}
		if err := os.MkdirAll(filepath.Join(stage, "src", "folder.ts"), 0o750); err != nil {
			return err
		}
		_, err = service.ReadJobFile(ctx, run.Job.ID, "src/folder.ts")
		record("directory", err)
		return errors.New("fixture complete")
	}})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, service, job.ID)
	for name, message := range messages {
		if message == "" {
			t.Errorf("%s reported no error", name)
			continue
		}
		if leaksHostPath(t, service, message) {
			t.Errorf("%s exposes a host path: %s", name, message)
		}
		want := "src/missing.ts"
		if name == "directory" {
			want = "src/folder.ts"
		}
		if !strings.Contains(message, want) {
			t.Errorf("%s does not name the project path: %s", name, message)
		}
	}
	if len(messages) != 6 {
		t.Fatalf("recorded %d errors: %+v", len(messages), messages)
	}
	if leaksHostPath(t, service, finished.Error) {
		t.Errorf("job error exposes a host path: %s", finished.Error)
	}
}

func TestRedactHostPaths(t *testing.T) {
	service := newTestService(t)
	staging, err := filepath.Abs(service.stagingDir)
	if err != nil {
		t.Fatal(err)
	}
	workspace, _ := filepath.Abs(service.opts.WorkspacePath)
	for name, test := range map[string]struct{ message, want string }{
		"staged file":   {"open " + filepath.Join(staging, "job_0a1b2c3d4e5f", "src", "main.ts") + ": access denied", "open src" + string(os.PathSeparator) + "main.ts: access denied"},
		"forward slash": {"open " + filepath.ToSlash(filepath.Join(staging, "job_0a1b2c3d4e5f", "src", "main.ts")) + ": busy", "open src/main.ts: busy"},
		"upper case":    {"stat " + strings.ToUpper(filepath.Join(staging, "job_aa11", "game.json")), "stat GAME.JSON"},
		"rename pair":   {"rename " + filepath.Join(staging, "job_1f", ".gm-write-1") + " " + filepath.Join(staging, "job_1f", "src", "a.ts") + ": in use", "rename .gm-write-1 src" + string(os.PathSeparator) + "a.ts: in use"},
		"published":     {"read " + filepath.Join(workspace, "Games", "orbit", "dist", "game.js"), "read Games" + string(os.PathSeparator) + "orbit" + string(os.PathSeparator) + "dist" + string(os.PathSeparator) + "game.js"},
		"plain text":    {"old_text matches 2 locations; supply one unique exact block", "old_text matches 2 locations; supply one unique exact block"},
		"relative path": {"src/main.ts:12:4: Expected \";\"", "src/main.ts:12:4: Expected \";\""},
	} {
		if got := service.RedactHostPaths(test.message); got != test.want {
			t.Errorf("%s: got %q, want %q", name, got, test.want)
		}
	}
	if got := (*Service)(nil).RedactHostPaths("unchanged"); got != "unchanged" {
		t.Errorf("nil service changed the message: %q", got)
	}
}
