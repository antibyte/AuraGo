package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSourceBatchPreflightSearchAndRuntime(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}
		if run.Stage != "building" {
			return errors.New("unexpected repair")
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
		invalid := "import meta from '../assets/builtin/missing/1/sheet.json'; export const value = meta;"
		edits[1].NewText = &invalid
		if _, err := s.ReplaceJobFiles(ctx, run.Job.ID, edits); err == nil {
			t.Error("invalid asset import accepted")
		}
		if got, _ := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts"); got != a {
			t.Error("failed import preflight mutated source")
		}
		edits[1].NewText = &newB
		written, err := s.ReplaceJobFiles(ctx, run.Job.ID, edits)
		if err != nil || !written.Written || !written.Build.OK || len(written.Files) != 2 {
			t.Errorf("batch: %+v %v", written, err)
		}
		search, err := s.SearchJobFile(ctx, run.Job.ID, "src/main.ts", "console.log")
		if err != nil || len(search.Matches) != 1 || !strings.Contains(search.Matches[0].Text, "value + 2") || search.SHA256 != written.Files[0].SHA256 {
			t.Errorf("search: %+v %v", search, err)
		}
		if _, err := s.SearchJobFile(ctx, run.Job.ID, "../escape", "value"); err == nil {
			t.Error("search escaped job")
		}
		if s.RuntimeContext(ctx, run.Job.ID)["version"] != "phaser-2" {
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

func TestValidationReuseRequiresExactCurrentEvidence(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}
		stage, _ := s.JobDirectory(run.Job.ID)
		fingerprint, err := validationFingerprint(stage)
		if err != nil {
			return err
		}
		check := &previewCheck{ID: "check", JobID: run.Job.ID, ReadyAt: time.Now().Add(-4 * time.Second), BoundToken: "grant", GameplayReceived: true, Scenarios: []GameScenario{{ID: "movement"}}}
		result := &BuildResult{OK: true, RuntimeStatus: "passed", GameplayStatus: "passed", check: check, fingerprint: fingerprint, validationScope: "full"}
		encodedScenarios, _ := json.Marshal(check.Scenarios)
		result.scenarioFingerprint = sourceHash(string(encodedScenarios))
		s.mu.Lock()
		s.previewCheck = check
		s.previewJobs[project.ID] = run.Job.ID
		s.lastValidation[run.Job.ID] = result
		s.mu.Unlock()
		grant, err := s.CreatePreviewGrant(project.ID)
		if err != nil {
			return err
		}
		s.mu.Lock()
		check.BoundToken = grant.Token
		s.mu.Unlock()
		if _, ok := s.reusableValidation(ctx, run.Job.ID, "full"); !ok {
			t.Error("unchanged complete result not reused")
		}
		for _, path := range []string{"src/main.ts", "vendor/game-flow.js", gamePlanPath} {
			full := filepath.Join(stage, filepath.FromSlash(path))
			before, err := os.ReadFile(full)
			if err != nil {
				return err
			}
			if err := os.WriteFile(full, append(before, []byte("\n ")...), 0600); err != nil {
				return err
			}
			if _, ok := s.reusableValidation(ctx, run.Job.ID, "full"); ok {
				t.Errorf("reused changed %s", path)
			}
			if err := os.WriteFile(full, before, 0600); err != nil {
				return err
			}
		}
		result.TargetedChecks = true
		if _, ok := s.reusableValidation(ctx, run.Job.ID, "full"); ok {
			t.Error("targeted result reused")
		}
		result.TargetedChecks = false
		if _, ok := s.reusableValidation(ctx, run.Job.ID, "startup"); ok {
			t.Error("different scope reused")
		}
		check.Scenarios[0].ID = "changed"
		if _, ok := s.reusableValidation(ctx, run.Job.ID, "full"); ok {
			t.Error("changed checks reused")
		}
		check.Scenarios[0].ID = "movement"
		s.mu.Lock()
		token := s.tokens[grant.Token]
		token.ExpiresAt = time.Now().Add(-time.Second)
		s.tokens[grant.Token] = token
		s.mu.Unlock()
		if _, ok := s.reusableValidation(ctx, run.Job.ID, "full"); ok {
			t.Error("expired evidence reused")
		}
		return errors.New("verified reuse")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "verified reuse" {
		t.Fatal(done.Error)
	}
}

func TestDesignReportsIndependentCorrectionsWithinBudget(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		err := s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"","features":[],"assets":[{"role":""}],"settings":{"goal":2}}`))
		var detail *DesignValidationError
		if !errors.As(err, &detail) || len(detail.Issues) < 4 || detail.RemainingAttempts != 2 {
			t.Errorf("incomplete corrections: %v", err)
		}
		data, _ := json.Marshal(map[string]any{"objective": "Explore", "features": []string{"A peaceful garden"}, "assets": []DesignAsset{{Role: "player", Fallback: "A green circle"}}, "settings": nil})
		if err := s.SetDesignJSON(ctx, run.Job.ID, data); err != nil {
			return err
		}
		plan, err := s.GetPlan(ctx, run.Job.ID)
		if err != nil {
			return err
		}
		if plan.Template != "minimal" {
			t.Error("correction changed retained base")
		}
		return errors.New("verified corrections")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "verified corrections" {
		t.Fatal(done.Error)
	}
}
