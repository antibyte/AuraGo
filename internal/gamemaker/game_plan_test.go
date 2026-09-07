package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type planningRunner func(context.Context, JobRun) error

func (r planningRunner) RunGameMakerJob(ctx context.Context, run JobRun) error { return r(ctx, run) }

func TestPlanPhaseLocksMutationsAndBoundsCorrections(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "planning" {
			return errors.New("invalid plan entered building")
		}
		if s.PlanningComplete(run.Job.ID) || s.PlanningComplete("unknown-job") {
			t.Error("unsubmitted plan completed planning")
		}
		for _, err := range []error{
			s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", "bad"),
			func() error { _, e := s.ImportAssetPack(ctx, run.Job.ID, "space-shooter"); return e }(),
			func() error {
				_, e := s.StoreJobAsset(ctx, run.Job.ID, "assets/test.svg", "image", "test", "", []byte("x"))
				return e
			}(),
		} {
			if err == nil || !strings.Contains(err.Error(), "planning_required") {
				t.Errorf("planning write gate: %v", err)
			}
		}
		for attempt := range 3 {
			if err := s.SetPlan(ctx, run.Job.ID, GamePlan{}); err == nil {
				t.Error("invalid plan accepted")
			}
			if s.PlanningComplete(run.Job.ID) != (attempt == 2) {
				t.Error("completion does not match the correction budget")
			}
		}
		if err := s.SetPlan(ctx, run.Job.ID, ExampleGamePlan(project)); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Errorf("correction limit: %v", err)
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, job.ID)
	if finished.Status != "failed" || finished.ResultRevision != 0 {
		t.Fatalf("invalid plan published: %+v", finished)
	}
	if !strings.Contains(finished.Error, "plan.schema_version: must be 1") {
		t.Fatalf("last plan error hidden by correction limit: %s", finished.Error)
	}
}

func TestPlanningRoundsRetainFieldError(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	rounds := 0
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "planning" {
			return errors.New("invalid plan entered building")
		}
		if rounds > 0 && (len(run.Diagnostics) != 1 || run.Diagnostics[0].Message != "plan.controls.restart: describe the input") {
			t.Errorf("field correction lost across rounds: %+v", run.Diagnostics)
		}
		rounds++
		p := ExampleGamePlan(project)
		delete(p.Controls, "restart")
		if err := s.SetPlan(ctx, run.Job.ID, p); err == nil {
			t.Error("missing restart accepted")
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, job.ID)
	if rounds != 3 || finished.Status != "failed" || !strings.Contains(finished.Error, "plan.controls.restart: describe the input") {
		t.Fatalf("planning failure lost context: rounds=%d, %+v", rounds, finished)
	}
}

func TestCorrectedPlanClearsObsoleteDiagnostics(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	rounds := 0
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "building" {
			s.mu.RLock()
			planErr := s.planErrors[run.Job.ID]
			s.mu.RUnlock()
			if len(run.Diagnostics) != 0 || planErr != nil {
				t.Errorf("corrected plan retains error: %+v, %v", run.Diagnostics, planErr)
			}
			return errors.New("test reached building")
		}
		rounds++
		p := ExampleGamePlan(project)
		if rounds == 1 {
			p.SchemaVersion = 0
			_ = s.SetPlan(ctx, run.Job.ID, p)
			return nil
		}
		return s.SetPlan(ctx, run.Job.ID, p)
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); rounds != 2 || finished.Error != "test reached building" {
		t.Fatalf("correction did not advance: rounds=%d, %+v", rounds, finished)
	}
}

func TestPlanReferencesAndReadOnly(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	p := ExampleGamePlan(project)
	p.Assets = nil
	if err := s.checkPlan(project, p); err == nil {
		t.Fatal("missing visual roles passed")
	}
	p.Assets = []PlanAsset{{Role: "player", PackID: "robots-drones-animated-top-down", Version: "2", AssetID: "service_robot_move_down", Direction: "left", DisplayHeight: 64, Origin: Point{.5, .5}, Collider: "rectangle", Animations: []string{"service_robot_move_left"}}}
	if err := s.checkPlan(project, p); err != nil {
		t.Fatal(err)
	}
	p.Assets[0].Animations = []string{"service_robot_attack_down"}
	if err := s.checkPlan(project, p); err == nil {
		t.Fatal("invented attack passed")
	}
	p.Assets[0].Animations = nil
	p.Perspective = "side"
	if err := s.checkPlan(project, p); err == nil {
		t.Fatal("wrong perspective passed")
	}
	p = ExampleGamePlan(project)
	p.Scenarios[0].Steps[0].Action = "eval"
	if err := s.checkPlan(project, p); err == nil {
		t.Fatal("executable test command passed")
	}
	s.UpdatePolicy(Policy{Enabled: true, ReadOnly: true, AllowEdit: true})
	if err := s.SetPlan(context.Background(), "unused", p); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("readonly plan: %v", err)
	}
}

func TestBreakoutPlanCorrectionsPreserveAssetIntent(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	p := ExampleGamePlan(project)
	p.Template = "blocks"
	// Validate the documented array against the actual bundled pack IDs.
	skill, err := os.ReadFile("skills/aurago-game-maker-director/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	_, example, ok := strings.Cut(strings.ReplaceAll(string(skill), "\r\n", "\n"), "```json\n[\n")
	if !ok {
		t.Fatal("missing complete Breakout asset example")
	}
	example, _, _ = strings.Cut(example, "```")
	if err := json.Unmarshal([]byte("[\n"+example), &p.Assets); err != nil {
		t.Fatal(err)
	}
	if err := s.checkPlan(project, p); err != nil {
		t.Fatalf("documented Breakout plan rejected: %v", err)
	}
	rounds := 0
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "building" {
			if len(run.Diagnostics) != 0 || len(run.Plan.Assets) != 4 || run.Plan.Assets[2].AssetID != "colored_block_01" {
				t.Error("accepted plan lost variants or retained obsolete diagnostics")
			}
			return errors.New("test reached building")
		}
		rounds++
		plan := p
		plan.Assets = append([]PlanAsset(nil), p.Assets...)
		if rounds < 3 {
			plan.Perspective = "side"
		}
		if rounds == 2 {
			plan.Assets[2].AssetID = "missing-block"
		}
		data, _ := json.Marshal(plan)
		if rounds == 1 {
			data = bytes.Replace(data, []byte(`"asset_id":"colored_block_01"`), []byte(`"asset_ids":["colored_block_01","colored_block_02"]`), 1)
		}
		err := s.SetPlanJSON(ctx, run.Job.ID, data)
		if rounds == 1 && (err == nil || !strings.Contains(err.Error(), `unknown field "asset_ids"`) || !strings.Contains(err.Error(), "Split variants")) {
			t.Errorf("plural IDs were silently lost: %v", err)
		}
		if rounds == 2 {
			for _, want := range []string{"plan.perspective", `set plan.perspective to "top"`, "assets[2]", "missing-block"} {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("independent asset error %q missing: %v", want, err)
				}
			}
		}
		if rounds < 3 {
			if s.PlanningComplete(run.Job.ID) {
				t.Error("invalid plan accepted before correction")
			}
			return nil
		}
		return err
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); rounds != 3 || finished.Error != "test reached building" {
		t.Fatalf("bounded corrections failed: rounds=%d, %+v", rounds, finished)
	}
}

func TestPlanSchemaErrorsExhaustCorrections(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "planning" {
			return errors.New("schema-invalid plan entered building")
		}
		data, _ := json.Marshal(ExampleGamePlan(project))
		for attempt, extra := range []string{`"pack_ids":["blocks-and-balls"]`, `"asset_ids":["ball_01"]`, `"view":"top"`} {
			bad := bytes.Replace(data, []byte(`"role":"player"`), []byte(`"role":"player",`+extra), 1)
			if err := s.SetPlanJSON(ctx, run.Job.ID, bad); err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Errorf("unknown asset field was discarded: %v", err)
			}
			if s.PlanningComplete(run.Job.ID) != (attempt == 2) {
				t.Error("schema error did not consume the shared correction budget")
			}
		}
		if err := s.SetPlanJSON(ctx, run.Job.ID, data); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Errorf("fourth plan admitted: %v", err)
		}
		if plan, err := s.GetPlan(ctx, run.Job.ID); err != nil || plan != nil {
			t.Error("invalid plan was persisted")
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, s, job.ID); !strings.Contains(finished.Error, `unknown field "view"`) || !strings.Contains(finished.Error, "plan.perspective") {
		t.Fatalf("concrete schema correction was lost: %+v", finished)
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

func TestRepairLimitIsSharedWithAgentValidation(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	runs := 0
	var lastFailure string
	s.SetRunner(testRunner{service: s, mutate: func(ctx context.Context, run JobRun) error {
		runs++
		for attempt := 0; attempt < 4; attempt++ {
			stop := s.StopAfterValidation(run.Job.ID, false)
			if stop() {
				t.Error("previous validation ended a new round")
			}
			if err := s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", fmt.Sprintf("invalid TypeScript <<< %d", attempt)); err != nil {
				return err
			}
			for repeat := 0; repeat < 2; repeat++ {
				result := s.ValidateJob(ctx, run.Job.ID)
				if result.OK {
					t.Error("invalid source passed")
				}
				lastFailure = diagnosticsText(result.Diagnostics)
			}
			if stop() != (attempt == 3) {
				t.Error("building must end when the shared repair budget is exhausted")
			}
			s.mu.RLock()
			failures := s.validationFailures[run.Job.ID]
			s.mu.RUnlock()
			if failures != attempt+1 {
				t.Errorf("unchanged failure consumed another repair: %d", failures)
			}
		}
		if err := s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", "// fifth attempt"); err == nil || !strings.Contains(err.Error(), "repair_limit_reached") {
			t.Errorf("repair gate: %v", err)
		}
		return nil
	}})
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, job.ID)
	if finished.Status != "failed" || finished.ResultRevision != 0 || runs != 1 {
		t.Fatalf("nested repair or false publication: %+v, runs=%d", finished, runs)
	}
	if lastFailure == "" || strings.Contains(lastFailure, "repair_limit_reached") || !strings.Contains(finished.Error, lastFailure) {
		t.Fatalf("last concrete failure was lost: %q, job: %q", lastFailure, finished.Error)
	}
}

func TestValidationCompletionDistinguishesBuildingAndRepair(t *testing.T) {
	s := newTestService(t)
	s.activeJobID = "job"
	building := s.StopAfterValidation("job", false)
	repair := s.StopAfterValidation("job", true)
	if building() || repair() {
		t.Fatal("round ended without validation")
	}
	s.lastValidation["job"] = &BuildResult{OK: true}
	if building() || !repair() {
		t.Fatal("successful core check must continue building but end a repair round")
	}
	fresh := s.StopAfterValidation("job", true)
	if fresh() {
		t.Fatal("old success ended a new round")
	}
	s.lastValidation["job"] = &BuildResult{RuntimeStatus: "unavailable"}
	if !building() || !fresh() {
		t.Fatal("missing browser feedback must return control to the orchestrator")
	}
	s.activeJobID = "other"
	if building() || fresh() {
		t.Fatal("stale job ended another job")
	}
}

func TestGameplayEvidenceBoundToBuildWindowAndLimits(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "build", JobID: "job", Scenarios: requiredScenarios("minimal")}
	s.previewCheck = check
	s.activeJobID = "job"
	for _, token := range []string{"window1", "window2"} {
		s.tokens[token] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "build", ExpiresAt: time.Now().Add(time.Minute)}
	}
	_ = s.ReportPreview("project", PreviewReport{Token: "window1", Type: "ready", CanvasVisible: true})
	report := PreviewReport{Token: "window2", Type: "gameplay", Observations: successfulObservationFixture(check.Scenarios)}
	if err := s.ReportPreview("project", report); err != nil || check.GameplayReceived {
		t.Fatalf("different window admitted: %v", err)
	}
	report.Token = "window1"
	report.Observations[0].After["player_x"] = math.NaN()
	if err := s.ReportPreview("project", report); err == nil {
		t.Fatal("nonfinite evidence admitted")
	}
	report.Observations = successfulObservationFixture(check.Scenarios)
	report.Images = []string{"data:image/png;base64,broken"}
	if err := s.ReportPreview("project", report); err != nil || !check.GameplayReceived || len(check.Images) != 0 {
		t.Fatalf("optional capture broke technical checks: %v", err)
	}
	newCheck := &previewCheck{ID: "new-build", JobID: "job", Scenarios: check.Scenarios}
	s.previewCheck = newCheck
	if err := s.ReportPreview("project", report); err != nil || newCheck.GameplayReceived {
		t.Fatal("stale build admitted")
	}
	for _, oversized := range []PreviewReport{{Observations: make([]GameObservation, 17)}, {Images: make([]string, 3)}, {Images: []string{strings.Repeat("a", 700001)}}} {
		if validateGameReport(oversized) == nil {
			t.Fatal("unbounded evidence admitted")
		}
	}
}

func TestPublicationRechecksCancellationPolicyAndLateDiagnostics(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	job := Job{ID: "job", ProjectID: project.ID}
	s.activeJobID = job.ID
	check := &previewCheck{ID: "build", JobID: job.ID, GameplayReceived: true}
	s.previewCheck = check
	result := BuildResult{OK: true, check: check, RuntimeStatus: "passed", GameplayStatus: "passed"}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.publishValidated(cancelled, "unused", project, job, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled publication: %v", err)
	}
	s.UpdatePolicy(Policy{Enabled: true, ReadOnly: true, AllowEdit: true})
	if _, err := s.publishValidated(context.Background(), "unused", project, job, result); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("readonly publication: %v", err)
	}
	s.UpdatePolicy(policyFromOptions(s.opts))
	check.Diagnostics = []Diagnostic{{Message: "late spawn error during image review"}}
	if _, err := s.publishValidated(context.Background(), "unused", project, job, result); err == nil || !strings.Contains(err.Error(), "late spawn") {
		t.Fatalf("late error published: %v", err)
	}
	check.Diagnostics = nil
	s.previewCheck = &previewCheck{ID: "replacement"}
	if _, err := s.publishValidated(context.Background(), "unused", project, job, result); err == nil || !strings.Contains(err.Error(), "current validated build") {
		t.Fatalf("stale build published: %v", err)
	}
	current, _ := s.GetProject(context.Background(), project.ID)
	if current.CurrentRevision != 0 {
		t.Fatal("rejected publication changed the revision")
	}
}

func TestPlanAndChecksAreRevisionedButNotExported(t *testing.T) {
	s := newTestService(t)
	p := createTestProject(t, s, "2d")
	s.SetRunner(testRunner{service: s})
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

func TestSpriteUsageReferencesAndTransforms(t *testing.T) {
	s := newTestService(t)
	packs, _ := s.ListAssetPacks()
	for _, pack := range packs {
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
