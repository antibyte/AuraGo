package server

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/tools"
)

func setupInvasionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := invasion.InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func registerFakeEggConnection(t *testing.T, hub *bridge.EggHub, nestID, eggID string) {
	t.Helper()
	if err := hub.Register(nestID, &bridge.EggConnection{
		EggID:     eggID,
		NestID:    nestID,
		SharedKey: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatalf("hub.Register(%s): %v", nestID, err)
	}
}

func TestMissionRemoteTargetsFiltersInactiveNestsAndEggs(t *testing.T) {
	db := setupInvasionTestDB(t)
	activeEggID, _ := invasion.CreateEgg(db, invasion.EggRecord{Name: "Active Egg", Active: true})
	inactiveEggID, _ := invasion.CreateEgg(db, invasion.EggRecord{Name: "Inactive Egg", Active: false})
	activeNestID, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "Active Nest", Active: true, EggID: activeEggID})
	inactiveNestID, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "Inactive Nest", Active: false, EggID: activeEggID})
	inactiveEggNestID, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "Inactive Egg Nest", Active: true, EggID: inactiveEggID})

	hub := bridge.NewEggHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	registerFakeEggConnection(t, hub, activeNestID, activeEggID)
	registerFakeEggConnection(t, hub, inactiveNestID, activeEggID)
	registerFakeEggConnection(t, hub, inactiveEggNestID, inactiveEggID)

	s := &Server{InvasionDB: db, EggHub: hub, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	req := httptest.NewRequest(http.MethodGet, "/api/missions/v2/remote-targets", nil)
	rr := httptest.NewRecorder()
	handleMissionRemoteTargets(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Targets []remoteMissionTarget `json:"targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body.Targets) != 1 || body.Targets[0].NestID != activeNestID {
		t.Fatalf("targets = %+v, want only active nest", body.Targets)
	}
}

func TestRemoteMissionValidateTargetRejectsInactiveNestAndEgg(t *testing.T) {
	db := setupInvasionTestDB(t)
	activeEggID, _ := invasion.CreateEgg(db, invasion.EggRecord{Name: "Active Egg", Active: true})
	inactiveEggID, _ := invasion.CreateEgg(db, invasion.EggRecord{Name: "Inactive Egg", Active: false})
	inactiveNestID, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "Inactive Nest", Active: false, EggID: activeEggID})
	inactiveEggNestID, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "Inactive Egg Nest", Active: true, EggID: inactiveEggID})

	hub := bridge.NewEggHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	registerFakeEggConnection(t, hub, inactiveNestID, activeEggID)
	registerFakeEggConnection(t, hub, inactiveEggNestID, inactiveEggID)
	client := &remoteMissionClient{hub: hub, db: db}

	err := client.validateTarget(tools.MissionV2{RunnerType: tools.MissionRunnerRemote, RemoteNestID: inactiveNestID, RemoteEggID: activeEggID})
	if err == nil || !strings.Contains(err.Error(), "inactive") {
		t.Fatalf("inactive nest error = %v, want inactive rejection", err)
	}
	err = client.validateTarget(tools.MissionV2{RunnerType: tools.MissionRunnerRemote, RemoteNestID: inactiveEggNestID, RemoteEggID: inactiveEggID})
	if err == nil || !strings.Contains(err.Error(), "inactive") {
		t.Fatalf("inactive egg error = %v, want inactive rejection", err)
	}
}
