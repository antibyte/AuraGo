package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidationReusePreflightSkipsIneligibleCandidatesAndRechecksEvidence(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	checksDone := make(chan error, 1)
	finished := errors.New("validation reuse preflight checks complete")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project))
		}

		stage, err := s.JobDirectory(run.Job.ID)
		if err != nil {
			checksDone <- err
			return err
		}
		fingerprint, err := validationFingerprint(stage)
		if err != nil {
			checksDone <- err
			return err
		}
		check := &previewCheck{
			ID:               "preflight-check",
			JobID:            run.Job.ID,
			ReadyAt:          time.Now().Add(-4 * time.Second),
			GameplayReceived: true,
			Scenarios:        []GameScenario{{ID: "movement"}},
		}
		result := &BuildResult{
			OK:              true,
			RuntimeStatus:   "passed",
			GameplayStatus:  "passed",
			check:           check,
			fingerprint:     fingerprint,
			validationScope: "full",
		}
		encodedScenarios, _ := json.Marshal(check.Scenarios)
		result.scenarioFingerprint = sourceHash(string(encodedScenarios))

		s.mu.Lock()
		s.previewCheck = check
		s.previewJobs[project.ID] = run.Job.ID
		s.lastValidation[run.Job.ID] = result
		s.mu.Unlock()
		grant, err := s.CreatePreviewGrant(project.ID)
		if err != nil {
			checksDone <- err
			return err
		}
		s.mu.Lock()
		check.BoundToken = grant.Token
		tokenGrant := s.tokens[grant.Token]
		s.mu.Unlock()

		readyAt := check.ReadyAt
		resetCandidate := func() {
			s.mu.Lock()
			check.ReadyAt = readyAt
			check.Diagnostics = nil
			check.GameplayReceived = true
			check.Scenarios = []GameScenario{{ID: "movement"}}
			check.BoundToken = grant.Token
			s.previewCheck = check
			s.activeJobID = run.Job.ID
			s.tokens[grant.Token] = tokenGrant
			copy := *result
			s.lastValidation[run.Job.ID] = &copy
			s.mu.Unlock()
		}

		ineligible := []struct {
			name   string
			mutate func()
		}{
			{name: "missing result", mutate: func() { s.lastValidation[run.Job.ID] = nil }},
			{name: "failed result", mutate: func() { s.lastValidation[run.Job.ID].OK = false }},
			{name: "targeted result", mutate: func() { s.lastValidation[run.Job.ID].TargetedChecks = true }},
			{name: "wrong scope", mutate: func() { s.lastValidation[run.Job.ID].validationScope = "startup" }},
			{name: "missing fingerprint", mutate: func() { s.lastValidation[run.Job.ID].fingerprint = "" }},
			{name: "failed runtime", mutate: func() { s.lastValidation[run.Job.ID].RuntimeStatus = "failed" }},
			{name: "stale preview", mutate: func() { s.previewCheck = &previewCheck{ID: "replacement", JobID: run.Job.ID} }},
			{name: "browser evidence too recent", mutate: func() { check.ReadyAt = time.Now() }},
			{name: "missing browser grant", mutate: func() { delete(s.tokens, grant.Token) }},
			{name: "expired browser grant", mutate: func() {
				expired := s.tokens[grant.Token]
				expired.ExpiresAt = time.Now().Add(-time.Second)
				s.tokens[grant.Token] = expired
			}},
			{name: "changed scenarios", mutate: func() { check.Scenarios[0].ID = "changed" }},
			{name: "unverified gameplay", mutate: func() { s.lastValidation[run.Job.ID].GameplayStatus = "unverified" }},
		}
		for _, candidate := range ineligible {
			resetCandidate()
			s.mu.Lock()
			candidate.mutate()
			s.mu.Unlock()
			fingerprintCalls := 0
			if _, reused := s.reusableValidationWithFingerprint(ctx, run.Job.ID, "full", func(string) (string, error) {
				fingerprintCalls++
				return "", errors.New("ineligible candidate reached fingerprint walk")
			}); reused || fingerprintCalls != 0 {
				t.Errorf("%s: reused=%t fingerprint calls=%d; want no reuse and no fingerprint read", candidate.name, reused, fingerprintCalls)
			}
		}

		resetCandidate()
		fingerprintCalls := 0
		if _, reused := s.reusableValidationWithFingerprint(ctx, run.Job.ID, "full", func(stage string) (string, error) {
			fingerprintCalls++
			return validationFingerprint(stage)
		}); !reused || fingerprintCalls != 1 {
			t.Errorf("current complete result: reused=%t fingerprint calls=%d; want reuse after one fingerprint", reused, fingerprintCalls)
		}

		sourcePath := filepath.Join(stage, "src", "main.ts")
		originalSource, err := os.ReadFile(sourcePath)
		if err != nil {
			checksDone <- err
			return err
		}
		if err := os.WriteFile(sourcePath, append(originalSource, []byte("\n// changed during preflight test")...), 0o600); err != nil {
			checksDone <- err
			return err
		}
		resetCandidate()
		fingerprintCalls = 0
		if _, reused := s.reusableValidationWithFingerprint(ctx, run.Job.ID, "full", func(stage string) (string, error) {
			fingerprintCalls++
			return validationFingerprint(stage)
		}); reused || fingerprintCalls != 1 {
			t.Errorf("changed source: reused=%t fingerprint calls=%d; want rejection after one fingerprint", reused, fingerprintCalls)
		}
		if err := os.WriteFile(sourcePath, originalSource, 0o600); err != nil {
			checksDone <- err
			return err
		}

		resetCandidate()
		fingerprintCalls = 0
		if _, reused := s.reusableValidationWithFingerprint(ctx, run.Job.ID, "full", func(stage string) (string, error) {
			fingerprintCalls++
			current, err := validationFingerprint(stage)
			s.mu.Lock()
			expired := s.tokens[grant.Token]
			expired.ExpiresAt = time.Now().Add(-time.Second)
			s.tokens[grant.Token] = expired
			s.mu.Unlock()
			return current, err
		}); reused || fingerprintCalls != 1 {
			t.Errorf("grant stale during fingerprint: reused=%t fingerprint calls=%d; want final evidence rejection", reused, fingerprintCalls)
		}

		resetCandidate()
		fingerprintCalls = 0
		if _, reused := s.reusableValidationWithFingerprint(ctx, run.Job.ID, "full", func(stage string) (string, error) {
			fingerprintCalls++
			current, err := validationFingerprint(stage)
			s.mu.Lock()
			s.previewCheck = &previewCheck{ID: "replacement", JobID: run.Job.ID}
			s.mu.Unlock()
			return current, err
		}); reused || fingerprintCalls != 1 {
			t.Errorf("preview stale during fingerprint: reused=%t fingerprint calls=%d; want final evidence rejection", reused, fingerprintCalls)
		}

		resetCandidate()
		fingerprintCalls = 0
		if _, reused := s.reusableValidationWithFingerprint(ctx, run.Job.ID, "full", func(stage string) (string, error) {
			fingerprintCalls++
			current, err := validationFingerprint(stage)
			s.mu.Lock()
			check.Scenarios[0].ID = "changed while hashing"
			s.mu.Unlock()
			return current, err
		}); reused || fingerprintCalls != 1 {
			t.Errorf("scenario evidence stale during fingerprint: reused=%t fingerprint calls=%d; want final evidence rejection", reused, fingerprintCalls)
		}

		checksDone <- nil
		return finished
	}))

	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-checksDone; err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != finished.Error() {
		t.Fatalf("job error = %q, want %q", done.Error, finished.Error())
	}
}
