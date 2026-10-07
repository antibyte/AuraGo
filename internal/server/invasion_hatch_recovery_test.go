package server

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/invasion"
)

func newHatchRecoveryFixture(t *testing.T) (*Server, invasion.NestRecord, invasion.EggRecord) {
	t.Helper()
	db := setupInvasionTestDB(t)
	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{Name: "egg", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	nestID, err := invasion.CreateNest(db, invasion.NestRecord{Name: "nest", Active: true, EggID: eggID, DeployMethod: "docker_remote", Host: "10.0.0.5"})
	if err != nil {
		t.Fatal(err)
	}
	if err := invasion.UpdateNestHatchStatus(db, nestID, "hatching", ""); err != nil {
		t.Fatal(err)
	}
	nest, err := invasion.GetNest(db, nestID)
	if err != nil {
		t.Fatal(err)
	}
	egg, err := invasion.GetEgg(db, eggID)
	if err != nil {
		t.Fatal(err)
	}
	return &Server{InvasionDB: db, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, nest, egg
}

func TestRunEggHatchRecordsDeployPanicAsFailedHatch(t *testing.T) {
	s, nest, egg := newHatchRecoveryFixture(t)
	s.runEggHatch(nest, egg, func(invasion.NestRecord, invasion.EggRecord) error {
		panic("runtime error: slice bounds out of range [:8] with length 5")
	})
	got, err := invasion.GetNest(s.InvasionDB, nest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.HatchStatus != "failed" || !strings.Contains(got.HatchError, "deployment panicked") {
		t.Fatalf("hatch = %q / %q, want failed with the panic recorded", got.HatchStatus, got.HatchError)
	}
}

func TestRunEggHatchRecordsDeployError(t *testing.T) {
	s, nest, egg := newHatchRecoveryFixture(t)
	s.runEggHatch(nest, egg, func(invasion.NestRecord, invasion.EggRecord) error {
		return errors.New("failed to pull image: manifest unknown")
	})
	got, _ := invasion.GetNest(s.InvasionDB, nest.ID)
	if got.HatchStatus != "failed" || got.HatchError != "failed to pull image: manifest unknown" {
		t.Fatalf("hatch = %q / %q, want failed with the deploy error", got.HatchStatus, got.HatchError)
	}
}

func TestRunEggHatchMarksSuccessfulDeployRunning(t *testing.T) {
	s, nest, egg := newHatchRecoveryFixture(t)
	s.runEggHatch(nest, egg, func(invasion.NestRecord, invasion.EggRecord) error { return nil })
	got, _ := invasion.GetNest(s.InvasionDB, nest.ID)
	if got.HatchStatus != "running" || got.HatchError != "" {
		t.Fatalf("hatch = %q / %q, want running without error", got.HatchStatus, got.HatchError)
	}
}
