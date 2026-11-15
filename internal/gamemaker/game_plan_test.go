package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type planningRunner func(context.Context, JobRun) error

func (r planningRunner) RunGameMakerJob(ctx context.Context, run JobRun) error { return r(ctx, run) }

func TestOptionalPlanScenariosRetainRequiredChecks(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	plan := ExampleGamePlan(project)
	plan.Template = "blocks"
	if len(plan.Scenarios) != 0 {
		t.Fatal("schema example should not require duplicate gameplay checks")
	}
	if err := s.checkPlan(project, plan); err != nil {
		t.Fatal(err)
	}
	scenarios := gameScenarios(&plan)
	if len(scenarios) != 8 || scenarios[2].ID != "required_rules" || scenarios[2].Steps[0].Mode != "catch" || scenarios[5].ID != "required_end" || scenarios[5].Value != 1 {
		t.Fatalf("empty additional scenarios weakened template checks: %+v", scenarios)
	}
	observations := successfulObservationFixture(scenarios)
	observations[5].After["ended"] = 0
	checks := compareGameObservations(scenarios, observations)
	if checks[5].Status != "failed" {
		t.Fatal("missing ESC handling must still fail")
	}
	// Keep valid existing custom checks and their exact input/timing evidence.
	plan.Scenarios = []GameScenario{{ID: "launch_ball", Metric: "actions", Compare: "increased", Steps: []GameTestStep{{Action: "key", Key: "SPACE", MS: 200}}}}
	if err := s.checkPlan(project, plan); err != nil {
		t.Fatal(err)
	}
	scenarios = gameScenarios(&plan)
	checks = compareGameObservations(scenarios, successfulObservationFixture(scenarios))
	encoded, err := json.Marshal(checks[len(checks)-1])
	if err != nil || !bytes.Contains(encoded, []byte(`"steps":[{"action":"key","key":"SPACE","ms":200}]`)) || len(checks) != 9 {
		t.Fatalf("custom test evidence missing: %s, %v", encoded, err)
	}
	plan.Scenarios[0].ID = "required_rules"
	if err := s.checkPlan(project, plan); err == nil {
		t.Fatal("plan replaced a server-owned scenario")
	}
	plan.Scenarios = make([]GameScenario, 9)
	if err := s.checkPlan(project, plan); err == nil {
		t.Fatal("extra scenario limit removed")
	}
}

func TestShooterHitChecksRetainFailureEvidence(t *testing.T) {
	scenario := requiredScenarios("shooter")[2]
	// Server-owned IDs are reserved; validate the same steps as an optional check.
	copy := scenario
	copy.ID = "shooter_rules"
	if err := validateScenario(copy); err != nil {
		t.Fatal(err)
	}
	observation := GameObservation{ID: scenario.ID,
		Before:     map[string]float64{"hits": 0, "actions": 0, "spawns": 0, "hit_events": 0, "ended": 0},
		After:      map[string]float64{"hits": 0, "actions": 3, "spawns": 1, "hit_events": 0, "ended": 0},
		TargetRuns: []TargetRun{{Target: "enemy", Mode: "aim", Samples: 30, Inputs: 3, Contacts: 1, Reason: "timeout"}},
	}
	check := compareGameObservations([]GameScenario{scenario}, []GameObservation{observation})[0]
	if check.Status != "failed" || !strings.Contains(check.Observed, "actions=0->3; spawns=0->1; hit_events=0->0; ended=0->0") {
		t.Fatalf("shots/spawns must explain a missing hit, never pass it: %+v", check)
	}
	delete(observation.After, "hits")
	if check := compareGameObservations([]GameScenario{scenario}, []GameObservation{observation})[0]; check.Status != "unavailable" {
		t.Fatalf("missing hits must not be inferred from shots: %+v", check)
	}
}

func TestMovementScenarioDoesNotInventPrimaryActions(t *testing.T) {
	scenario := GameScenario{ID: "smooth_steering_no_jump", Metric: "actions", Compare: "increased", Steps: []GameTestStep{
		{Action: "target", Target: "player", Mode: "move", MS: 800},
		{Action: "key", Key: "D", MS: 600}, {Action: "observe"},
	}}
	if err := validateScenario(scenario); err == nil || !strings.Contains(err.Error(), "player_distance") {
		t.Fatalf("movement/action mismatch was not explained before acceptance: %v", err)
	}
	// Already accepted older plans need useful repair guidance, not a false pass.
	observation := GameObservation{ID: scenario.ID, Before: map[string]float64{"actions": 0}, After: map[string]float64{"actions": 0},
		TargetRuns: []TargetRun{{Target: "player", Mode: "move", Samples: 12, Inputs: 3, Contacts: 12, Effects: 1, Reason: "complete"}},
	}
	check := compareGameObservations([]GameScenario{scenario}, []GameObservation{observation})[0]
	if check.Status != "failed" || !strings.Contains(check.Observed, "check definition:") || !strings.Contains(check.Observed, "smoothness") {
		t.Fatalf("bad scenario hidden or misdiagnosed as broken steering: %+v", check)
	}
	for _, variant := range []string{"position", "primary", "custom_key"} {
		t.Run(variant, func(t *testing.T) {
			copy := scenario
			copy.Steps = append([]GameTestStep(nil), scenario.Steps...)
			switch variant {
			case "position":
				copy.Metric = "player_distance"
			case "primary":
				copy.Steps = append(copy.Steps, GameTestStep{Action: "key", Key: "SPACE", MS: 100})
			case "custom_key":
				copy.Steps = copy.Steps[1:]
			}
			if err := validateScenario(copy); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAssetSearchPrefersContentsOverPackName(t *testing.T) {
	s := newTestService(t)
	for _, packID := range []string{"", "blocks-and-balls"} {
		for _, query := range []string{"ball", "ball circle bounce", "ball_01"} {
			matches, err := s.SearchAssets(query, packID, "top", 6)
			if err != nil || len(matches) == 0 || matches[0].AssetID != "ball_01" {
				t.Fatalf("ball hidden by pack-name matches (%s/%s): %+v, %v", packID, query, matches, err)
			}
			if len(matches) > 6 {
				t.Fatal("search limit exceeded")
			}
		}
	}
}

func TestPlanAndChecksAreRevisionedButNotExported(t *testing.T) {
	s := newTestService(t)
	p := createTestProject(t, s, "2d")
	s.SetRunner(testRunner{service: s, mutate: func(ctx context.Context, run JobRun) error {
		stage, err := s.JobDirectory(run.Job.ID)
		if err != nil {
			return err
		}
		path := filepath.Join(stage, filepath.FromSlash(gamePlanPath))
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return err
		}
		data, _ := json.Marshal(ExampleGamePlan(p))
		return os.WriteFile(path, data, 0640)
	}})
	job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); finished.Status != "ready" {
		t.Fatal(finished.Error)
	}
	data, err := os.ReadFile(filepath.Join(s.opts.WorkspacePath, p.ProjectKey, filepath.FromSlash(gamePlanPath)))
	if err != nil {
		t.Fatal(err)
	}
	var plan GamePlan
	if json.Unmarshal(data, &plan) != nil || plan.Template != "minimal" {
		t.Fatal("published plan missing")
	}
	// Existing export tests exercise hidden-file filtering; assert this exact path.
	var exported bytes.Buffer
	if _, err := s.WriteExport(context.Background(), p.ID, &exported); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(exported.Bytes()), int64(exported.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range z.File {
		if file.Name == gamePlanPath {
			t.Fatal("internal plan included in export")
		}
	}
}

func TestGameObservationsCannotSelfCertifyOrOmitEvidence(t *testing.T) {
	scenarios := gameScenarios(&GamePlan{Template: "shooter"})
	observations := successfulObservationFixture(scenarios)
	for _, check := range compareGameObservations(scenarios, observations) {
		if check.Status != "passed" {
			t.Fatal(check)
		}
	}
	observations[0].After["player_x"] = observations[0].Before["player_x"]
	delete(observations[1].After, "actions")
	last := len(observations) - 1
	observations[last].After["restart1_listener_count"] = 4
	checks := compareGameObservations(scenarios, observations)
	if checks[0].Status != "failed" || checks[1].Status != "unavailable" || checks[last].Status != "failed" {
		t.Fatal(checks)
	}
}

func TestTargetEvidenceCannotCertifyMissingOrInactiveActions(t *testing.T) {
	scenario := requiredScenarios("shooter")[2]
	for _, kind := range []string{"missing", "mismatch", "counter", "no_input", "blocked", "inactive", "unsupported", "host_hidden"} {
		t.Run(kind, func(t *testing.T) {
			observations := successfulObservationFixture([]GameScenario{scenario})
			run := &observations[0].TargetRuns[0]
			switch kind {
			case "missing":
				observations[0].TargetRuns = nil
			case "mismatch":
				run.Target = "another-enemy"
			case "counter":
				run.Effects = 0
			case "no_input":
				run.Inputs = 0
			default:
				run.Reason = kind
			}
			if got := compareGameObservations([]GameScenario{scenario}, observations)[0]; got.Status != "unavailable" {
				t.Fatal(got)
			}
		})
	}
	for _, invalid := range []TargetRun{
		{Target: "enemy", Mode: "teleport", Reason: "complete"},
		{Target: "enemy", Mode: "aim", Reason: "passed"},
		{Target: "enemy", Mode: "aim", Reason: "complete", Samples: 161},
		{Target: "enemy", Mode: "aim", Reason: "complete", Samples: 2, Effects: 3},
	} {
		if err := validateGameReport(PreviewReport{Observations: []GameObservation{{TargetRuns: []TargetRun{invalid}}}}); err == nil {
			t.Fatalf("accepted invalid evidence: %+v", invalid)
		}
	}
}

func TestBlindGameplayFailureRemainsUnverified(t *testing.T) {
	for _, metric := range []string{"outcome", "pickup_events", "hit_events", "hits", "health", "lives", "goal_remaining", "win_events", "lose_events", "actions"} {
		scenario := GameScenario{ID: "stomp_enemy", Metric: metric, Compare: "changed", Steps: []GameTestStep{{Action: "key", Key: "RIGHT", MS: 1000}, {Action: "key", Key: "SPACE", MS: 200}}}
		observation := GameObservation{ID: scenario.ID, Before: map[string]float64{metric: 0}, After: map[string]float64{metric: 0}}
		want := "unavailable"
		if metric == "actions" {
			want = "failed" // A direct input counter is still testable without a target.
		}
		if got := compareGameObservations([]GameScenario{scenario}, []GameObservation{observation})[0]; got.Status != want {
			t.Fatalf("%s: %+v", metric, got)
		}
	}
}

func TestSpriteUsageReferencesAndTransforms(t *testing.T) {
	s := newTestService(t)
	packs, _ := s.ListAssetPacks()
	for _, pack := range packs {
		if pack.Kind == "model3d" || presentationPack(pack.ID) {
			continue
		}
		_, assets, assemblies, animations, err := readPackUsage(pack.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range assets {
			if a.Entity == "" || a.Action == "" || !finite(a.Transform.ForwardRadians) || a.Transform.Mode == "" {
				t.Fatalf("incomplete usage metadata %s/%s", pack.ID, a.ID)
			}
		}
		for _, a := range assemblies {
			if a.Transform.Mode == "" || a.Direction == "" {
				t.Fatal("missing assembly transform")
			}
		}
		for _, a := range animations {
			if a.Entity == "" || a.Action == "" || a.Direction == "" {
				t.Fatal("missing animation grouping")
			}
		}
	}
	found, err := s.SearchAssets("tank", "vehicles-planes-top-down", "top", 6)
	if err != nil || len(found) == 0 || found[0].AssemblyID != "tank" {
		t.Fatalf("fragment outranked tank: %+v %v", found, err)
	}
	d, err := s.DescribeAsset("robots-drones-animated-top-down", "service_robot_move_down", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(d.MissingActions, ","), "attack") || !d.allowsDirection("up") {
		t.Fatal("invented attack or missing direction")
	}
}

func TestOptionalMechanicsDoNotImposeOutcomes(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	plan := ExampleGamePlan(project)
	plan.SchemaVersion = 4
	plan.Mechanics = map[string]any{
		"outcomes": []any{"won"},
		"lives":    float64(1),
	}
	if err := service.checkPlan(project, plan); err != nil {
		t.Fatalf("optional peaceful mechanics rejected: %v", err)
	}
	plan.Mechanics = map[string]any{"blocks": []any{}}
	if err := service.checkPlan(project, plan); err != nil {
		t.Fatalf("continuous-play mechanics rejected: %v", err)
	}
	plan.SchemaVersion = 3
	if err := service.checkPlan(project, plan); err == nil || !strings.Contains(err.Error(), "requires schema 4") {
		t.Fatalf("schema3 mechanics accepted: %v", err)
	}
	plan.SchemaVersion = 4
	plan.Mechanics = map[string]any{"outcomes": []any{"won", "won"}}
	if err := service.checkPlan(project, plan); err == nil || !strings.Contains(err.Error(), "duplicate won") {
		t.Fatalf("duplicate outcomes accepted: %v", err)
	}
	plan.Mechanics = map[string]any{"lives": float64(0)}
	if err := service.checkPlan(project, plan); err == nil || !strings.Contains(err.Error(), "lives") {
		t.Fatalf("invalid lives accepted: %v", err)
	}
}
