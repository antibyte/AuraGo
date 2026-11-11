package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/security"
)

func TestInvasionReadonlyResultsAndMessages(t *testing.T) {
	db, err := invasion.InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	egg, _ := invasion.CreateEgg(db, invasion.EggRecord{Name: "worker", Active: true})
	nest, _ := invasion.CreateNest(db, invasion.NestRecord{Name: "nest", EggID: egg, Active: true})
	task, _ := invasion.CreateTask(db, nest, egg, "admitted work", 60)
	cfg := &config.Config{}
	cfg.InvasionControl.Enabled = true
	cfg.InvasionControl.ReadOnly = true
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("b", 64)
	if err := vault.WriteSecret(eggSharedKeyName(nest), key); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, InvasionDB: db, Vault: vault}
	if err := s.recordInvasionTaskResult(nest, bridge.ResultPayload{TaskID: task, Output: "finished", Status: "success"}); err != nil {
		t.Fatal(err)
	}
	record, _ := invasion.GetTaskByID(db, task)
	if record.Status != "completed" || record.ResultOutput != "finished" {
		t.Fatal("readonly dropped result")
	}
	if err := s.validateInvasionWork("other-nest", egg, task, "", true); err == nil {
		t.Fatal("accepted foreign task")
	}
	if err := s.validateInvasionWork(nest, egg, "", "", true); err == nil {
		t.Fatal("accepted artifact without work")
	}
	body := `{"title":"finished","body":"report","wakeup_requested":true}`
	request := func(signed bool) *http.Request {
		r := httptest.NewRequest("POST", "/api/invasion/messages", strings.NewReader(body))
		r.Header.Set("X-AuraGo-Nest-ID", nest)
		r.Header.Set("X-AuraGo-Egg-ID", egg)
		if signed {
			signEggRequest(t, r, key, []byte(body))
		}
		return r
	}
	w := httptest.NewRecorder()
	s.guardInvasion(handleInvasionEggMessage(s))(w, request(true))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"wakeup_allowed":false`) {
		t.Fatalf("readonly message: %d %s", w.Code, w.Body)
	}
	var requested, allowed int
	if err := db.QueryRow(`SELECT wakeup_requested,wakeup_allowed FROM invasion_egg_messages`).Scan(&requested, &allowed); err != nil || requested != 1 || allowed != 0 {
		t.Fatalf("stored wakeup: %d %d %v", requested, allowed, err)
	}
	w = httptest.NewRecorder()
	s.guardInvasion(handleInvasionEggMessage(s))(w, request(false))
	if w.Code != 401 {
		t.Fatalf("unsigned message: %d", w.Code)
	}
	disabled := cfg.Clone()
	disabled.InvasionControl.Enabled = false
	s.replaceConfigSnapshot(disabled)
	w = httptest.NewRecorder()
	s.guardInvasion(handleInvasionEggMessage(s))(w, request(true))
	if w.Code != 503 {
		t.Fatalf("disabled message: %d", w.Code)
	}
	if err := s.recordInvasionTaskResult(nest, bridge.ResultPayload{TaskID: task}); err == nil {
		t.Fatal("disabled WS callback stored result")
	}
}

type stopConfirmationConnector struct {
	invasion.NestConnector
	stopErr, statusErr error
	status             string
}

func (c stopConfirmationConnector) Stop(context.Context, invasion.NestRecord, []byte) error {
	return c.stopErr
}
func (c stopConfirmationConnector) Status(context.Context, invasion.NestRecord, []byte) (string, error) {
	return c.status, c.statusErr
}

func TestInvasionStopRequiresProcessConfirmation(t *testing.T) {
	for _, c := range []stopConfirmationConnector{
		{status: "stopped"}, {status: "running"}, {status: "unknown"}, {status: "stopped", stopErr: errors.New("transport")}, {status: "stopped", statusErr: errors.New("probe")},
	} {
		err := confirmInvasionStop(context.Background(), c, invasion.NestRecord{}, nil)
		want := c.status == "stopped" && c.stopErr == nil && c.statusErr == nil
		if (err == nil) != want {
			t.Fatalf("stop %+v: %v", c, err)
		}
	}
}
