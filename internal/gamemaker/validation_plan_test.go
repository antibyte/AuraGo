package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// The disclosed plan must be the list the browser run executes, including the
// server-inserted control check, and it must never carry a verdict.
func TestValidationPlanMatchesExecutedChecks(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	var disclosed map[string]any
	var executed []GameScenario
	service.SetRunner(testRunner{service: service, mutate: func(ctx context.Context, run JobRun) error {
		if run.Stage != "building" {
			return nil
		}
		disclosed = service.ValidationPlan(ctx, run.Job.ID)
		result := service.buildJob(ctx, run.Job.ID, "full")
		if !result.OK {
			return errors.New(diagnosticsText(result.Diagnostics))
		}
		service.mu.RLock()
		executed = append([]GameScenario(nil), service.previewCheck.Scenarios...)
		service.mu.RUnlock()
		return nil
	}})
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, service, job.ID); finished.Status != "ready" {
		t.Fatalf("job = %+v", finished)
	}
	if disclosed == nil || disclosed["scope"] != "full" {
		t.Fatalf("validation plan = %+v", disclosed)
	}
	checks, ok := disclosed["checks"].([]ValidationCheck)
	if !ok || len(checks) != len(executed) || len(checks) < 8 {
		t.Fatalf("disclosed %d checks, executed %d", len(checks), len(executed))
	}
	for i, check := range checks {
		if check.ID != executed[i].ID || check.Metric != executed[i].Metric || check.Compare != executed[i].Compare || len(check.Steps) != len(executed[i].Steps) {
			t.Fatalf("check %d differs: disclosed %+v, executed %+v", i, check, executed[i])
		}
		if check.Owner != "server" {
			t.Fatalf("starter plan has no model-authored checks: %+v", check)
		}
	}
	targets, _ := disclosed["target_roles"].([]string)
	if !strings.Contains(strings.Join(targets, ","), "item") || !strings.Contains(strings.Join(targets, ","), "player") {
		t.Fatalf("target roles = %v", targets)
	}
	data, err := json.Marshal(disclosed)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"status"`, `"observed"`, `"passed"`} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("validation plan carries a verdict field %s: %s", forbidden, data)
		}
	}
	if len(data) > 4096 {
		t.Fatalf("validation plan uses %d bytes of context", len(data))
	}
}

func TestBaseChecksNameTargetsPerBase(t *testing.T) {
	flat := BaseChecks("2d")
	for base, want := range map[string]string{"platformer": "reach item", "blocks": "catch ball", "shooter": "aim enemy", "board": "select cell", "topdown": "interact goal", "minimal": "reach item"} {
		entry := flat[base]
		joined := entry["input"] + "|" + entry["primary"] + "|" + entry["rules"]
		if !strings.Contains(joined, want) {
			t.Errorf("%s checks %q do not name %q", base, joined, want)
		}
	}
	if _, exists := flat["fps"]; exists {
		t.Error("2D planning received 3D bases")
	}
	spatial := BaseChecks("3d")
	if !strings.Contains(spatial["transport"]["rules"], "reach cargo") || !strings.Contains(spatial["flight"]["rules"], "reach goal") || !strings.Contains(spatial["fps"]["reload"], "reloads") {
		t.Errorf("3D base checks = %+v", spatial)
	}
	if spatial["three"]["startup"] == "" || spatial["voxel"] != nil {
		t.Errorf("free-code and voxel entries = %+v / %+v", spatial["three"], spatial["voxel"])
	}
	data, _ := json.Marshal(flat)
	if len(data) > 1500 {
		t.Fatalf("2D base checks use %d bytes of planning context", len(data))
	}
}

func TestValidationNextActionNeverClaimsUnobservedSuccess(t *testing.T) {
	for name, test := range map[string]struct {
		result BuildResult
		want   string
	}{
		"passed":      {BuildResult{OK: true, RuntimeStatus: "passed", GameplayStatus: "passed"}, "server validates again"},
		"repairable":  {BuildResult{GameplayStatus: "unavailable", Repairable: true}, "real object"},
		"no browser":  {BuildResult{RuntimeStatus: "unavailable"}, "preview must stay open"},
		"no evidence": {BuildResult{RuntimeStatus: "passed", GameplayStatus: "unavailable"}, "preview must stay open"},
		"runtime":     {BuildResult{RuntimeStatus: "failed"}, "runtime error"},
		"gameplay":    {BuildResult{RuntimeStatus: "passed", GameplayStatus: "failed"}, "first failed check"},
	} {
		if got := ValidationNextAction(test.result); !strings.Contains(got, test.want) {
			t.Errorf("%s: %q lacks %q", name, got, test.want)
		}
	}
}
