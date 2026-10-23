package gamemaker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestOpenChecksKeepOnlyOpenChecksAndBoundThem(t *testing.T) {
	long := strings.Repeat("x", maxOpenCheckText+40)
	steps := make([]GameTestStep, maxOpenCheckSteps+3)
	for i := range steps {
		steps[i] = GameTestStep{Action: "wait", MS: 100}
	}
	result := BuildResult{
		RuntimeStatus: "ready",
		Checks: []CheckResult{
			{ID: "startup", Status: "passed"},
			{ID: "movement", Status: "skipped"},
			{ID: "hits", Status: "failed", Expected: long, Observed: "0", Steps: steps},
			{ID: "goal", Status: "unavailable", Repairable: true, Expected: "goal reached"},
			{ID: "second", Status: "failed"},
			{ID: "third", Status: "failed"},
			{ID: "fourth", Status: "failed"},
			{ID: "fifth", Status: "failed"},
			{ID: "sixth", Status: "failed"},
		},
	}
	checks := openChecksFromResult(&result)
	if len(checks) != maxOpenChecks {
		t.Fatalf("open checks = %d, want %d", len(checks), maxOpenChecks)
	}
	if checks[0].ID != "hits" || checks[0].Status != "failed" {
		t.Fatalf("first open check = %+v", checks[0])
	}
	if len([]rune(checks[0].Expected)) > maxOpenCheckText+1 {
		t.Fatalf("expected text not truncated: %d runes", len([]rune(checks[0].Expected)))
	}
	if len(checks[0].Steps) != maxOpenCheckSteps {
		t.Fatalf("steps = %d, want %d", len(checks[0].Steps), maxOpenCheckSteps)
	}
	if !checks[1].Repairable || checks[1].Status != "unavailable" {
		t.Fatalf("repairable check lost its flag: %+v", checks[1])
	}
	for _, check := range checks {
		if strings.TrimSpace(check.NextAction) == "" {
			t.Fatalf("open check %q carries no next action", check.ID)
		}
	}
	if got := openChecksFromResult(nil); got != nil {
		t.Fatalf("nil result produced %+v", got)
	}
}

func TestOpenChecksRoundTripThroughTheOutcomeColumn(t *testing.T) {
	checks := []OpenCheck{
		{ID: "hits", Status: "failed", Expected: "hits increased", Observed: "hits stayed 0", NextAction: "Fix the first failed check."},
		{ID: "goal", Status: "unavailable", Repairable: true, Steps: []GameTestStep{{Action: "move", Key: "RIGHT", MS: 500}}},
	}
	encoded := encodeOpenChecks(checks)
	decoded := decodeOpenChecks(encoded)
	if len(decoded) != len(checks) || decoded[0].ID != "hits" || decoded[1].Steps[0].Key != "RIGHT" {
		t.Fatalf("round trip = %+v", decoded)
	}
	if encodeOpenChecks(nil) != "" || decodeOpenChecks("") != nil {
		t.Fatal("empty list must not serialize payload")
	}
	if decodeOpenChecks("{not json") != nil {
		t.Fatal("broken column must degrade to no open checks")
	}
	if decodeOpenChecks(`{"id":"object"}`) != nil {
		t.Fatal("non-list column must degrade to no open checks")
	}
}

func TestFailedJobStoresOpenChecks(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	ctx := context.Background()
	if _, err := service.db.Exec(`INSERT INTO gm_jobs
		(id,project_id,kind,prompt,status,phase,base_revision,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, "job_open", project.ID, "create", "make a game", "building", "building", 0, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	job, err := service.GetJob(ctx, "job_open")
	if err != nil {
		t.Fatal(err)
	}
	result := BuildResult{
		RuntimeStatus:  "ready",
		GameplayStatus: "unavailable",
		Repairable:     true,
		Checks: []CheckResult{
			{ID: "startup", Status: "passed"},
			{ID: "hits", Status: "unavailable", Repairable: true, Expected: "hits increased", Observed: "no target"},
			{ID: "goal", Status: "failed", Expected: "goal reached", Observed: "goal untouched"},
		},
	}
	passes := 2
	service.finishFailure(job, ctx, &JobError{Code: "validation", Err: errors.New("game validation failed")}, &result, &passes)

	stored, ok, err := service.LatestJob(ctx, project.ID)
	if err != nil || !ok {
		t.Fatalf("latest job: %v %v", ok, err)
	}
	if stored.Status != "failed" || stored.ErrorCode != "validation" {
		t.Fatalf("stored job = %+v", stored)
	}
	if len(stored.OpenChecks) != 2 {
		t.Fatalf("open checks = %+v", stored.OpenChecks)
	}
	if stored.OpenChecks[0].ID != "hits" || stored.OpenChecks[0].Status != "unavailable" || !stored.OpenChecks[0].Repairable {
		t.Fatalf("first open check = %+v", stored.OpenChecks[0])
	}
	if stored.OpenChecks[1].Expected != "goal reached" || strings.TrimSpace(stored.OpenChecks[1].NextAction) == "" {
		t.Fatalf("second open check = %+v", stored.OpenChecks[1])
	}

	// The terminal job_status event carries the same bounded list for the card.
	events, err := service.EventsAfter(ctx, project.ID, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type != "job_status" {
			continue
		}
		if checks, ok := event.Payload["open_checks"].([]OpenCheck); ok && len(checks) == 2 {
			found = true
		} else if raw, ok := event.Payload["open_checks"].([]any); ok && len(raw) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("terminal job_status carried no open checks: %+v", events)
	}
}
