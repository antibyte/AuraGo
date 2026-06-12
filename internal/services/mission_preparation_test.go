package services

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestMissionPreparationServiceLoadsTemplateFromPromptsDir(t *testing.T) {
	promptsDir := t.TempDir()
	template := "mission-prep-template {{.MaxEssentialTools}}"
	if err := os.WriteFile(filepath.Join(promptsDir, "mission_preparation.md"), []byte(template), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.PromptsDir = promptsDir
	var cfgMu sync.RWMutex
	service := NewMissionPreparationService(
		cfg,
		&cfgMu,
		nil,
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	service.loadPromptTemplate()

	if service.promptTpl != template {
		t.Fatalf("promptTpl = %q, want %q", service.promptTpl, template)
	}
	if got := service.buildSystemPrompt(7); !strings.Contains(got, "mission-prep-template 7") {
		t.Fatalf("buildSystemPrompt did not use loaded template: %q", got)
	}
}

func TestAutoPrepareEligibleRequiresMissionOptInForScheduledMissions(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{
		SchedulerEnabled: true,
		MissionsEnabled:  true,
	})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)

	dataDir := t.TempDir()
	cronMgr := tools.NewCronManager(dataDir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("start cron manager: %v", err)
	}
	t.Cleanup(func() {
		cronMgr.Stop()
		if err := cronMgr.Close(); err != nil {
			t.Fatalf("close cron manager: %v", err)
		}
	})

	missionMgr := tools.NewMissionManagerV2(dataDir, cronMgr)
	prepDB, err := tools.InitPreparedMissionsDB(filepath.Join(dataDir, "prepared_missions.db"))
	if err != nil {
		t.Fatalf("init prepared missions db: %v", err)
	}
	t.Cleanup(func() {
		if err := prepDB.Close(); err != nil {
			t.Fatalf("close prepared missions db: %v", err)
		}
	})
	missionMgr.SetPreparedDB(prepDB)

	if err := missionMgr.Create(&tools.MissionV2{
		ID:            "mission_scheduled_without_optin",
		Name:          "Scheduled without opt-in",
		Prompt:        "Run a harmless check.",
		ExecutionType: tools.ExecutionScheduled,
		Schedule:      "0 0 * * *",
		Priority:      "medium",
		Enabled:       true,
		AutoPrepare:   false,
	}); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	cfg := &config.Config{}
	cfg.MissionPreparation.Enabled = true
	cfg.MissionPreparation.AutoPrepareScheduled = true
	cfg.MissionPreparation.TimeoutSeconds = 1
	cfg.MissionPreparation.MaxEssentialTools = 5
	cfg.MissionPreparation.MinConfidence = 0.5
	var cfgMu sync.RWMutex
	service := NewMissionPreparationService(
		cfg,
		&cfgMu,
		prepDB,
		missionMgr,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	service.autoPrepareEligible(context.Background())

	got, ok := missionMgr.Get("mission_scheduled_without_optin")
	if !ok {
		t.Fatal("mission missing after auto-prepare sweep")
	}
	if got.PreparationStatus != "" {
		t.Fatalf("mission was prepared without explicit opt-in; status=%q", got.PreparationStatus)
	}
	prepared, err := tools.GetPreparedMission(prepDB, got.ID)
	if err != nil {
		t.Fatalf("get prepared mission: %v", err)
	}
	if prepared != nil {
		t.Fatalf("unexpected prepared mission entry without opt-in: %+v", prepared)
	}
}
