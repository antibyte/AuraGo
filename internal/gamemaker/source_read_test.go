package gamemaker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Tool-free source generation needs the whole entry file, not one interactive
// 240-line window; only its explicit byte bound may reject it.
func TestReadJobSourceReturnsCompleteBoundedFile(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	var checks []string
	service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
		if run.Stage != "building" {
			return nil
		}
		long := strings.Repeat("// note\n", 400) + "export const done = true;\n"
		if err := service.WriteJobFile(ctx, run.Job.ID, "src/main.ts", long); err != nil {
			return err
		}
		window, err := service.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", 1, 240)
		if err != nil {
			return err
		}
		// Writes may add the entry's diagnostic prelude; compare with the stored file.
		stored, err := service.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		source, err := service.ReadJobSource(ctx, run.Job.ID, "src/main.ts", SourceGenerationMaxBytes)
		switch {
		case err != nil:
			checks = append(checks, "complete read failed: "+err.Error())
		case source.Content != stored || !strings.HasSuffix(stored, long) || source.StartLine != 1 || source.EndLine != source.TotalLines || source.TotalLines != len(strings.Split(stored, "\n")) || source.TotalLines <= 240:
			checks = append(checks, fmt.Sprintf("content or line bounds are incomplete: len=%d/%d lines=%d-%d/%d", len(source.Content), len(stored), source.StartLine, source.EndLine, source.TotalLines))
		case source.SHA256 != window.SHA256:
			checks = append(checks, "full-file sha256 differs from the interactive read")
		}
		if _, err := service.ReadJobSource(ctx, run.Job.ID, "src/main.ts", 100); err == nil || !strings.Contains(err.Error(), "src/main.ts") || !strings.Contains(err.Error(), "100") {
			checks = append(checks, "byte bound was not enforced with the path and limit")
		}
		return errors.New("fixture complete")
	}})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, service, job.ID); finished.Error != "fixture complete" {
		t.Fatalf("fixture did not run: %s", finished.Error)
	}
	if len(checks) > 0 {
		t.Fatal(strings.Join(checks, "; "))
	}
}
