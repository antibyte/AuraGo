package agent

import (
	"log/slog"
	"os"
	"testing"

	"aurago/internal/config"
	"aurago/internal/invasion"
)

func TestInvasionLoopbackURLUsesDedicatedLoopbackPort(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 8088
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443
	cfg.CloudflareTunnel.LoopbackPort = 18080

	got := invasionLoopbackURL(cfg, "/api/invasion/nests/n1/status")
	want := "http://127.0.0.1:18080/api/invasion/nests/n1/status"
	if got != want {
		t.Fatalf("invasionLoopbackURL = %q, want %q", got, want)
	}
}

func TestResolveInvasionTaskNestByEggNamePrefersRunningAssignedNest(t *testing.T) {
	db, err := invasion.InitDB(t.TempDir() + "/invasion.db")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{
		Name:   "Web Scraper",
		Active: true,
	})
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}

	stoppedID, err := invasion.CreateNest(db, invasion.NestRecord{
		Name:        "stopped nest",
		AccessType:  "docker",
		Active:      true,
		EggID:       eggID,
		HatchStatus: "stopped",
	})
	if err != nil {
		t.Fatalf("CreateNest stopped: %v", err)
	}
	runningID, err := invasion.CreateNest(db, invasion.NestRecord{
		Name:        "running nest",
		AccessType:  "docker",
		Active:      true,
		EggID:       eggID,
		HatchStatus: "running",
	})
	if err != nil {
		t.Fatalf("CreateNest running: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	nest, egg, err := resolveInvasionTaskNest(db, ToolCall{EggName: "web scraper"}, logger)
	if err != nil {
		t.Fatalf("resolveInvasionTaskNest: %v", err)
	}
	if nest.ID != runningID {
		t.Fatalf("resolved nest ID = %q, want running nest %q (stopped was %q)", nest.ID, runningID, stoppedID)
	}
	if egg.ID != eggID || egg.Name != "Web Scraper" {
		t.Fatalf("resolved egg = %#v, want Web Scraper %q", egg, eggID)
	}
}
