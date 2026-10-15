package gamemaker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSourceBatchSameFileEditsUseOriginalSnapshot(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}
		const first = "export const first = 1;\nexport const second = 2;\n// aaaa\n"
		const other = "export const other = 1;\n"
		edit := func(path, before, old, next string) SourceReplacement {
			return SourceReplacement{path, sourceHash(before), old, &next}
		}
		for _, tc := range []struct {
			name  string
			edits []SourceReplacement
			want  string
		}{
			{"simultaneous reverse order", []SourceReplacement{
				edit("src/probe.ts", first, "export const second = 2;", ""),
				edit("src/probe.ts", first, "export const first = 1;", "export const second = 2; // moved"),
				edit("src/other.ts", other, "1", "9"),
			}, "export const second = 2; // moved\n\n// aaaa\n"},
			{"overlap", []SourceReplacement{
				edit("src/probe.ts", first, "export const first = 1;", ""),
				edit("src/probe.ts", first, "first = 1", "first = 9"),
			}, ""},
			{"stale second hash", []SourceReplacement{
				edit("src/other.ts", other, "1", "9"),
				edit("src/probe.ts", first, "first = 1", "first = 9"),
				edit("src/probe.ts", first+"stale", "second = 2", "second = 8"),
			}, ""},
			{"duplicate block", []SourceReplacement{
				edit("src/probe.ts", first, "first = 1", "first = 9"),
				edit("src/probe.ts", first, "first = 1", "first = 8"),
			}, ""},
			{"ambiguous overlapping matches", []SourceReplacement{edit("src/probe.ts", first, "aaa", "b")}, ""},
			{"not in original", []SourceReplacement{
				edit("src/probe.ts", first, "first = 1", "first = 9"),
				edit("src/probe.ts", first, "first = 9", "first = 8"),
			}, ""},
			{"managed path", []SourceReplacement{
				edit("src/probe.ts", first, "first = 1", "first = 9"),
				edit("vendor/game-flow.js", other, "1", "9"),
			}, ""},
			{"invalid combined asset import", []SourceReplacement{
				edit("src/other.ts", other, "1", "9"),
				edit("src/probe.ts", first, "export const first = 1;", "import missing from '../assets/builtin/missing/1/sheet.json';"),
				edit("src/probe.ts", first, "second = 2", "second = 8"),
			}, ""},
		} {
			for path, source := range map[string]string{"src/probe.ts": first, "src/other.ts": other} {
				if err := s.writeJobFile(ctx, run.Job.ID, path, source); err != nil {
					return err
				}
			}
			result, err := s.ReplaceJobFiles(ctx, run.Job.ID, tc.edits)
			got, _ := s.ReadJobFile(ctx, run.Job.ID, "src/probe.ts")
			gotOther, _ := s.ReadJobFile(ctx, run.Job.ID, "src/other.ts")
			if tc.want == "" {
				if err == nil || result.Written || got != first || gotOther != other {
					t.Errorf("%s: invalid batch mutated source or was accepted: %v", tc.name, err)
				}
				continue
			}
			if err != nil || !result.Written || !result.Build.OK || got != tc.want || gotOther != strings.Replace(other, "1", "9", 1) || len(result.Files) != 2 {
				t.Errorf("%s: batch failed: result=%+v, err=%v, source=%q", tc.name, result, err, got)
			}
			if len(result.Files) > 0 && result.Files[0].SHA256 != sourceHash(got) {
				t.Error("batch did not return the final file hash")
			}
		}
		// Each edit fits separately, but the combined file must obey the limit.
		originalLimit := s.opts.MaxFileBytes
		s.opts.MaxFileBytes = int64(len(first) + 8)
		_, limitErr := s.ReplaceJobFiles(ctx, run.Job.ID, []SourceReplacement{
			edit("src/probe.ts", first, "first = 1", "first = 12345678"),
			edit("src/probe.ts", first, "second = 2", "second = 23456789"),
		})
		s.opts.MaxFileBytes = originalLimit
		if got, _ := s.ReadJobFile(ctx, run.Job.ID, "src/probe.ts"); limitErr == nil || !strings.Contains(limitErr.Error(), "file exceeds") || got != first {
			t.Errorf("combined file limit not enforced before mutation: %v", limitErr)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if result, err := s.ReplaceJobFiles(cancelled, run.Job.ID, []SourceReplacement{edit("src/probe.ts", first, "first = 1", "first = 9")}); err == nil || result.Written {
			t.Error("cancelled batch was applied")
		}
		if got, _ := s.ReadJobFile(ctx, run.Job.ID, "src/probe.ts"); got != first {
			t.Error("cancelled batch changed source")
		}
		return errors.New("same-file batch verified")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "same-file batch verified" {
		t.Fatal(done.Error)
	}
}
