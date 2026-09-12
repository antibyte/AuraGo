package gamemaker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTargetedGameMakerChecksAreBoundedAndExisting(t *testing.T) {
	scenarios := requiredScenarios("minimal")
	selected, err := selectTargetedGameMakerScenarios(scenarios, []string{"required_timed", "required_end"})
	if err != nil || len(selected) != 2 || selected[0].ID != "required_timed" || selected[1].ID != "required_end" {
		t.Fatalf("selected checks = %+v, err=%v", selected, err)
	}
	if _, err := selectTargetedGameMakerScenarios(scenarios, []string{"missing"}); err == nil {
		t.Fatal("unknown check ID was accepted")
	}
	ids := make([]string, maxTargetedGameMakerChecks+1)
	for i := range ids {
		ids[i] = "check"
	}
	if _, err := normalizeTargetedGameMakerChecks(ids); err == nil {
		t.Fatal("more than 16 check IDs were accepted")
	}
	if _, err := normalizeTargetedGameMakerChecks([]string{"required_end", "required_end"}); err == nil {
		t.Fatal("duplicate check ID was accepted")
	}
}

func TestSchemaFourNeutralCompositionDoesNotImposeGenreInput(t *testing.T) {
	plan := GamePlan{SchemaVersion: 4, Template: "minimal", Scenarios: []GameScenario{{ID: "dialogue_choice"}}}
	for _, scenario := range gameScenarios(&plan) {
		if scenario.ID == "required_input" {
			t.Fatal("schema-4 neutral plan retained the genre-bound input check")
		}
	}
}

func TestTargetedOrUnverifiedGameplayCannotPublish(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	job := Job{ID: "job", ProjectID: project.ID}
	check := &previewCheck{ID: "validation", JobID: job.ID, GameplayReceived: true}
	service.activeJobID = job.ID
	service.previewCheck = check
	result := BuildResult{OK: true, check: check, RuntimeStatus: "passed", GameplayStatus: "passed", TargetedChecks: true}
	if _, err := service.publishValidated(nilContext{}, "unused", project, job, result); err == nil {
		t.Fatal("targeted validation was publishable")
	}
}

func TestPublishValidatedSceneGateDoesNotReenterServiceLock(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "3d")
	job := Job{ID: "job", ProjectID: project.ID}
	check := &previewCheck{ID: "validation", JobID: job.ID, GameplayReceived: true}
	service.activeJobID = job.ID
	service.previewCheck = check

	stage := t.TempDir()
	if err := os.MkdirAll(filepath.Join(stage, ".aurago"), 0o750); err != nil {
		t.Fatalf("mkdir stage metadata: %v", err)
	}
	planData, err := json.Marshal(GamePlan{SchemaVersion: 4, Template: "three"})
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(gamePlanPath)), planData, 0o640); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(stage, "src"), 0o750); err != nil {
		t.Fatalf("mkdir stage sources: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(SceneFilePath)), []byte("null\n"), 0o640); err != nil {
		t.Fatalf("write scene: %v", err)
	}

	result := BuildResult{
		OK:             true,
		check:          check,
		RuntimeStatus:  "passed",
		GameplayStatus: "passed",
		TargetedChecks: true,
	}
	done := make(chan error, 1)
	go func() {
		_, publishErr := service.publishValidated(context.Background(), stage, project, job, result)
		done <- publishErr
	}()
	select {
	case publishErr := <-done:
		if publishErr == nil {
			t.Fatal("targeted validation unexpectedly published")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("publishValidated deadlocked while checking the staged scene")
	}
}
