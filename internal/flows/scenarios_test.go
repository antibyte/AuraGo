package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"aurago/internal/security"
)

type scenarioTestKeys struct {
	mu      sync.Mutex
	key     []byte
	created int
}

func (k *scenarioTestKeys) ScenarioKey(_ context.Context, create bool) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.key == nil && create {
		k.key = bytes.Repeat([]byte{7}, 32)
		k.created++
	}
	if k.key == nil {
		return nil, errors.New("missing key")
	}
	return bytes.Clone(k.key), nil
}

func TestScenariosEncryptValidateAndDetectStale(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Scenario")})
	if err != nil {
		t.Fatal(err)
	}
	keys := &scenarioTestKeys{}
	want := Scenario{Name: "First", TriggerNode: rec.Draft.Nodes[0].ID,
		TriggerData: map[string]any{"value": "private-input-value"}}
	saved, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0, want, keys)
	if err != nil || saved.Version != 1 {
		t.Fatalf("save = %+v, %v", saved, err)
	}
	var ciphertext []byte
	if err := s.store.db.QueryRow(`SELECT ciphertext FROM flow_test_scenarios WHERE flow_id=? AND id=?`, rec.ID, saved.ID).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("private-input-value")) {
		t.Fatal("scenario was not encrypted")
	}
	list, err := s.ListScenarios(ctx, rec.ID, keys)
	if err != nil || len(list) != 1 {
		t.Fatalf("scenario list = %+v, %v", list, err)
	}
	if encoded, _ := json.Marshal(list); bytes.Contains(encoded, []byte("private-input-value")) {
		t.Fatal("scenario collection disclosed fixture data")
	}
	got, err := s.GetScenario(ctx, rec.ID, saved.ID, keys)
	if err != nil || got.Stale || got.TriggerData["value"] != "private-input-value" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if simulated, err := s.SimulateScenario(ctx, rec.ID, saved.ID, rec.DraftRevision, 1, keys); err != nil || simulated.Mode != ModeSimulation {
		t.Fatalf("scenario simulation = %+v, %v", simulated, err)
	}
	if _, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 99, got, keys); !errors.Is(err, ErrScenarioConflict) {
		t.Fatalf("stale scenario update = %v", err)
	}
	bad := want
	bad.TriggerData = map[string]any{"value": "[redacted]"}
	if _, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0, bad, keys); !errors.Is(err, ErrTestDataRedacted) {
		t.Fatalf("redaction marker = %v", err)
	}
	security.RegisterSensitive("scenario-secret-fixture-123456")
	bad.TriggerData = map[string]any{"value": "scenario-secret-fixture-123456"}
	if _, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0, bad, keys); !errors.Is(err, ErrScenarioSensitive) {
		t.Fatalf("registered secret = %v", err)
	}
	bad.TriggerData = map[string]any{"password": "short"}
	if _, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0, bad, keys); !errors.Is(err, ErrScenarioSensitive) {
		t.Fatalf("credential field = %v", err)
	}
	bad.TriggerData = map[string]any{"prompt_tokens": 12}
	if _, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0, bad, keys); err != nil {
		t.Fatalf("ordinary token count was rejected: %v", err)
	}
	doc, err := rec.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[1].Position.X += 50
	doc.Nodes[0].Label = "Edited label"
	doc.Nodes[0].Settings.Notes = "Editor note"
	revision, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.GetScenario(ctx, rec.ID, saved.ID, keys)
	if err != nil || got.Stale {
		t.Fatalf("layout-only change invalidated scenario: %+v %v", got, err)
	}
	if _, err := s.SimulateScenario(ctx, rec.ID, saved.ID, revision, 1, keys); err != nil {
		t.Fatalf("layout-only revision should remain compatible: %v", err)
	}
	loaded, observedRevision, err := s.getScenarioAtDraftRevision(ctx, rec.ID, saved.ID, keys)
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes[1].Params["fields"] = []any{}
	revision, _, err = s.SaveDraft(ctx, rec.ID, doc, revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.simulateLoadedScenario(ctx, rec.ID, revision, 1, observedRevision, loaded); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("scenario validated against an older snapshot was simulated: %v", err)
	}
	got, err = s.GetScenario(ctx, rec.ID, saved.ID, keys)
	if err != nil || !got.Stale {
		t.Fatalf("changed graph did not invalidate scenario: %+v %v", got, err)
	}
	if _, err := s.SimulateScenario(ctx, rec.ID, saved.ID, revision, 1, keys); !errors.Is(err, ErrScenarioStale) {
		t.Fatalf("stale scenario was simulated: %v", err)
	}
	keys.mu.Lock()
	keys.key = bytes.Repeat([]byte{9}, 32)
	keys.mu.Unlock()
	if _, err := s.GetScenario(ctx, rec.ID, saved.ID, keys); !errors.Is(err, ErrScenarioUnreadable) {
		t.Fatalf("wrong key was accepted: %v", err)
	}
	keys.mu.Lock()
	keys.key = bytes.Repeat([]byte{7}, 32)
	keys.mu.Unlock()
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err := s.store.db.Exec(`UPDATE flow_test_scenarios SET ciphertext=? WHERE flow_id=? AND id=?`, ciphertext, rec.ID, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetScenario(ctx, rec.ID, saved.ID, keys); !errors.Is(err, ErrScenarioUnreadable) {
		t.Fatalf("tampered ciphertext = %v", err)
	}
}

func TestScenariosRejectMissingKeyAndBoundCount(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Limits")})
	if err != nil {
		t.Fatal(err)
	}
	keys := &scenarioTestKeys{}
	want := Scenario{Name: "Case", TriggerNode: rec.Draft.Nodes[0].ID, TriggerData: map[string]any{}}
	for i := 0; i < maxScenariosPerFlow; i++ {
		if _, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0, want, keys); err != nil {
			t.Fatalf("scenario %d: %v", i, err)
		}
	}
	if _, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0, want, keys); !errors.Is(err, ErrScenarioLimit) {
		t.Fatalf("over limit = %v", err)
	}
	keys.mu.Lock()
	keys.key = nil
	keys.mu.Unlock()
	if _, err := s.ListScenarios(ctx, rec.ID, keys); err == nil || keys.created != 1 {
		t.Fatal("lost key was silently replaced")
	}
}

func TestScenarioCiphertextIsBoundToFlowAndIdentity(t *testing.T) {
	key := bytes.Repeat([]byte{4}, 32)
	payload := scenarioPayload{Scenario: Scenario{ID: "sc_0123456789abcdef01234567", Version: 1}, Fingerprints: map[string]string{}}
	sealed, err := sealScenario(key, "flow_a", payload.Scenario.ID, payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"flow_b", payload.Scenario.ID}, {"flow_a", "sc_0123456789abcdef01234568"}} {
		if _, err := openScenario(key, pair[0], pair[1], sealed); !errors.Is(err, ErrScenarioUnreadable) {
			t.Fatalf("wrong binding = %v", err)
		}
	}
}

func TestScenarioConcurrentUpdatesUseVersionCAS(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Concurrent")})
	if err != nil {
		t.Fatal(err)
	}
	keys := &scenarioTestKeys{}
	item, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0,
		Scenario{Name: "Original", TriggerNode: rec.Draft.Nodes[0].ID}, keys)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, name := range []string{"One", "Two"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			candidate := item
			candidate.Name = name
			_, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, item.Version, candidate, keys)
			results <- err
		}(name)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrScenarioConflict):
			conflicts++
		default:
			t.Fatalf("unexpected update error: %v", err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("concurrent updates: success=%d conflicts=%d", success, conflicts)
	}
}

func TestScenarioAssertionsAndRenamePreserveSamples(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Assertions")})
	if err != nil {
		t.Fatal(err)
	}
	keys := &scenarioTestKeys{}
	node := rec.Draft.Nodes[1].ID
	ports := []string{PortOut}
	item, err := s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 0, Scenario{
		Name: "Original", TriggerNode: rec.Draft.Nodes[0].ID,
		TriggerData: map[string]any{"name": "private-sample"},
		Fixtures:    map[string]SimulationFixture{node: {Output: map[string]any{"greeting": "private-result"}, Ports: ports}},
		Assertions:  []ScenarioAssertion{{NodeID: node, Output: json.RawMessage(`{"greeting":"private-result"}`), Ports: &ports}},
	}, keys)
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := s.RenameScenario(ctx, rec.ID, item.ID, item.Version, "Renamed", keys)
	if err != nil || renamed.Version != 2 {
		t.Fatalf("rename = %+v, %v", renamed, err)
	}
	if _, err := s.RenameScenario(ctx, rec.ID, item.ID, 1, "Stale", keys); !errors.Is(err, ErrScenarioConflict) {
		t.Fatalf("stale rename = %v", err)
	}
	got, err := s.GetScenario(ctx, rec.ID, item.ID, keys)
	if err != nil || got.TriggerData["name"] != "private-sample" || got.Fixtures[node].Output["greeting"] != "private-result" {
		t.Fatalf("rename changed the sample: %+v, %v", got, err)
	}
	result, err := s.SimulateScenario(ctx, rec.ID, item.ID, rec.DraftRevision, 2, keys)
	if err != nil || len(result.Assertions) != 1 || !result.Assertions[0].Passed {
		t.Fatalf("assertions = %+v, %v", result, err)
	}
	got.Assertions[0].Output = json.RawMessage(`null`)
	got, err = s.SaveScenario(ctx, rec.ID, rec.DraftRevision, 2, got, keys)
	if err != nil {
		t.Fatal(err)
	}
	result, err = s.SimulateScenario(ctx, rec.ID, item.ID, rec.DraftRevision, got.Version, keys)
	if err != nil || result.Assertions[0].Code != "output_mismatch" {
		t.Fatalf("explicit null assertion = %+v, %v", result, err)
	}
}
