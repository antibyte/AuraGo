package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestLeafyClockAndGrowth(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	p := newPlant(731, now)
	p.Revision = 1
	early := advancePlant(p, now.Add(time.Hour-time.Nanosecond))
	if early.AgeHours != 0 || len(early.Branches[0].Nodes) != 2 {
		t.Fatal("plant grew before its first complete hour")
	}
	for h := int64(1); h <= 2; h++ {
		grown := advancePlant(p, now.Add(time.Duration(h)*time.Hour))
		for _, branch := range grown.Branches {
			if len(branch.Nodes) != 2+int(h) || branch.Nodes[len(branch.Nodes)-1] != h {
				t.Fatalf("hour %d: expected a visible new growth node per hour, got %v", h, branch.Nodes)
			}
		}
	}
	before, _ := json.Marshal(p)
	one := advancePlant(p, now.Add(24*time.Hour))
	split := p
	for i := 1; i <= 24; i++ {
		split = advancePlant(split, now.Add(time.Duration(i)*time.Hour))
	}
	if !reflect.DeepEqual(one, split) {
		t.Fatal("hourly and offline advancement differ")
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("simulation mutated its input")
	}
	if one.AgeHours != 24 || one.Moisture != 52 || one.Nutrients != 88 || one.Vitality != 100 || len(one.Branches) < 4 {
		t.Fatalf("day one: %+v", one)
	}
	if got := advancePlant(p, now.Add(-time.Hour)); !reflect.DeepEqual(p, got) {
		t.Fatal("clock rollback changed state")
	}
	wilt := advancePlant(p, now.Add(60*time.Hour))
	if wilt.Dead || wilt.Vitality >= 100 || wilt.Moisture != 0 {
		t.Fatal("missing recoverable wilt")
	}
	wilt.Moisture = 100
	wilt.Nutrients = 100
	recovered := advancePlant(wilt, now.Add(72*time.Hour))
	if recovered.Vitality <= wilt.Vitality {
		t.Fatal("care did not restore health")
	}
	expired := advancePlant(p, now.AddDate(10, 0, 0))
	if !expired.Dead || expired.AgeHours > 200 {
		t.Fatal("absence is not bounded by death")
	}
	expired.Moisture = 100
	if !advancePlant(expired, now.AddDate(11, 0, 0)).Dead {
		t.Fatal("dead plant revived without replant")
	}
	// Sustained care reaches bounded, flowering-age growth without repeated-frame simulation.
	mature := p
	for h := int64(1); h <= 24*365; h++ {
		mature = advancePlant(mature, now.Add(time.Duration(h)*time.Hour))
		if h%24 == 0 {
			mature.Moisture = 100
		}
		if h%144 == 0 {
			mature.Nutrients = 100
		}
	}
	if mature.Dead || len(mature.Branches) != plantMaxBranches {
		t.Fatal("healthy long-term plant failed to mature")
	}
	if err := validatePlant(&mature); err != nil {
		t.Fatal(err)
	}
	tip := mature.Branches[len(mature.Branches)-1]
	if err := mature.prune(tip.ID, 2, mature.LastSimulated, false); err != nil {
		t.Fatal(err)
	}
	regrown := advancePlant(mature, mature.LastSimulated.Add(6*time.Hour))
	last := regrown.Branches[len(regrown.Branches)-1]
	if last.Capped || len(last.Nodes) < 3 || last.Nodes[2] <= tip.Nodes[2] {
		t.Fatal("full-capacity cut did not produce fresh growth")
	}
	for _, b := range mature.Branches {
		if len(b.Nodes) > plantMaxNodes {
			t.Fatal("node budget exceeded")
		}
	}
}

func TestLeafyActionsPersistenceAndVacation(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	empty, err := svc.Plant(ctx, now)
	if err != nil || empty.Plant != nil {
		t.Fatalf("GET planted implicitly: %v", err)
	}
	rev := int64(0)
	seq := 0
	act := func(name string, when time.Time, fields func(*PlantAction)) PlantSnapshot {
		t.Helper()
		seq++
		a := PlantAction{Action: name, ID: fmt.Sprintf("action_%08d", seq), Revision: rev}
		if fields != nil {
			fields(&a)
		}
		out, err := svc.ApplyPlantAction(ctx, a, when)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		rev = out.Plant.Revision
		return out
	}
	planted := act("replant", now, nil)
	var raw string
	svc.getDB().QueryRow("SELECT value FROM desktop_meta WHERE key=?", plantStateKey).Scan(&raw)
	read, _ := svc.Plant(ctx, now.Add(24*time.Hour))
	var unchanged string
	svc.getDB().QueryRow("SELECT value FROM desktop_meta WHERE key=?", plantStateKey).Scan(&unchanged)
	if raw != unchanged || read.Plant.AgeHours != 24 {
		t.Fatal("GET persisted simulation")
	}
	pauseAt := now.Add(10*time.Hour + 30*time.Minute)
	paused := act("vacation", pauseAt, func(a *PlantAction) { v := true; a.Paused = &v })
	future := pauseAt.AddDate(1, 0, 0)
	away, _ := svc.Plant(ctx, future)
	if away.Plant.AgeHours != paused.Plant.AgeHours || away.Plant.Moisture != paused.Plant.Moisture {
		t.Fatal("vacation aged")
	}
	resumed := act("vacation", future, func(a *PlantAction) { v := false; a.Paused = &v })
	if resumed.Plant.AgeHours != 10 {
		t.Fatal("vacation catchup")
	}
	hour, _ := svc.Plant(ctx, future.Add(30*time.Minute))
	if hour.Plant.AgeHours != 11 {
		t.Fatal("partial hour was lost")
	}
	watered := act("water", future.Add(time.Hour), nil)
	a := PlantAction{Action: "fertilize", ID: "retry_action_001", Revision: rev}
	first, err := svc.ApplyPlantAction(ctx, a, future.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	retry, err := svc.ApplyPlantAction(ctx, a, future.Add(2*time.Hour))
	if err != nil || retry.Plant.Revision != first.Plant.Revision || retry.Plant.Nutrients != 99.5 {
		t.Fatal("retry applied twice")
	}
	a.Action = "water"
	if _, err := svc.ApplyPlantAction(ctx, a, future); !errors.Is(err, ErrPlantConflict) {
		t.Fatal("ID reuse accepted")
	}
	if planted.Plant.Seed != watered.Plant.Seed {
		t.Fatal("care replaced plant seed")
	}
	// Reopen the same SQLite state.
	cfg := svc.Config()
	_ = svc.Close() // The test has no optional Docker backend to stop.
	reopened, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := reopened.Plant(ctx, future.Add(2*time.Hour))
	if err != nil || persisted.Plant.Revision != first.Plant.Revision {
		t.Fatalf("restart: %v", err)
	}
}

func TestLeafyPruningAndConcurrency(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p := newPlant(731, now)
	p.Revision = 1
	for h := 1; h <= 200; h++ {
		p = advancePlant(p, now.Add(time.Duration(h)*time.Hour))
		if h%24 == 0 {
			p.Moisture = 100
		}
		if h%144 == 0 {
			p.Nutrients = 100
		}
	}
	later := p.LastSimulated
	original := copyPlantBranches(p.Branches)
	next := p.NextBranch
	if err := p.prune(0, 3, later, false); err != nil {
		t.Fatal(err)
	}
	if len(p.Branches[0].Nodes) != 3 || len(p.Branches) >= len(original) || !p.Branches[0].Capped {
		t.Fatal("prune did not remove distal subtree")
	}
	for h := 1; h <= 6; h++ {
		p = advancePlant(p, later.Add(time.Duration(h)*time.Hour))
	}
	found := false
	for _, b := range p.Branches {
		if b.ID >= next && b.Parent == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("cut did not regrow with a fresh identity")
	}
	if err := validatePlant(&p); err != nil {
		t.Fatal(err)
	}
	if err := p.prune(0, 0, later, false); err == nil {
		t.Fatal("root severing allowed")
	}
	if err := p.prune(0, 0, later, true); err != nil {
		t.Fatal(err)
	}
	if len(p.Branches) != 3 {
		t.Fatal("trim did not preserve exactly the roots")
	}
	raw, _ := json.Marshal(p)
	svc.getDB().Exec("INSERT INTO desktop_meta(key,value) VALUES(?,?)", plantStateKey, string(raw))
	undo, err := svc.ApplyPlantAction(ctx, PlantAction{Action: "undo_prune", ID: "undo_test_001", Revision: 1}, later.Add(time.Second))
	if err != nil || len(undo.Plant.Branches) <= 3 {
		t.Fatalf("undo: %v", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.ApplyPlantAction(ctx, PlantAction{Action: "water", ID: fmt.Sprintf("concurrent_%d", i), Revision: 2}, later.Add(2*time.Second))
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	ok, conflicts := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrPlantConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatal("concurrent writes did not conflict")
	}
}
