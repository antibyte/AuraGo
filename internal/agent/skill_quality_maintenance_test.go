package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

type skillQualityTestClient struct{ response string }

func (c skillQualityTestClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: c.response}}}}, nil
}

func (skillQualityTestClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, nil
}

func setupSkillQualityMaintenanceTest(t *testing.T, code string) (*config.Config, *tools.SkillManager, tools.SkillQualityCandidate) {
	t.Helper()
	root := t.TempDir()
	db, err := tools.InitSkillsDB(filepath.Join(root, "skills.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	manager := tools.NewSkillManager(db, skillsDir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	entry, err := manager.CreateSkillEntry("maintenance_placeholder", "placeholder skill", code, tools.SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := manager.ListPythonQualityCandidates(10)
	if err != nil || len(candidates) != 1 || candidates[0].ID != entry.ID {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	previousPython := tools.DefaultSkillManager()
	previousAgent := tools.DefaultAgentSkillManager()
	tools.SetDefaultSkillManager(manager)
	tools.SetDefaultAgentSkillManager(nil)
	t.Cleanup(func() {
		tools.SetDefaultSkillManager(previousPython)
		tools.SetDefaultAgentSkillManager(previousAgent)
	})
	cfg := &config.Config{}
	cfg.Tools.SkillManager.Enabled = true
	cfg.LLM.Model = "quality-test"
	return cfg, manager, candidates[0]
}

func decisionJSON(t *testing.T, candidate tools.SkillQualityCandidate, verdict string, confidence float64, codes []string) string {
	t.Helper()
	payload, err := json.Marshal(skillQualityDecision{
		Kind: candidate.Kind, ID: candidate.ID, ContentHash: candidate.ContentHash, Verdict: verdict,
		Confidence: confidence, Reason: "objectively a placeholder", ReasonCodes: codes,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func TestSkillQualityMaintenanceReadOnlyNeverDeletes(t *testing.T) {
	cfg, manager, candidate := setupSkillQualityMaintenanceTest(t, "# TODO placeholder\ndef run():\n    return 'hello world'\n")
	cfg.Tools.SkillManager.ReadOnly = true
	result := runSkillQualityMaintenance(context.Background(), cfg, slog.Default(), skillQualityTestClient{response: decisionJSON(t, candidate, "delete", 0.99, []string{"placeholder"})}, nil, nil, nil)
	if result.Deleted != 0 || result.ReviewRequired != 1 || result.Reviewed != 1 {
		t.Fatalf("result=%+v", result)
	}
	if _, err := manager.GetSkill(candidate.ID); err != nil {
		t.Fatalf("read-only review deleted skill: %v", err)
	}
}

func TestSkillQualityMaintenanceDeleteThresholdAndObjectiveEvidence(t *testing.T) {
	if hasObjectiveDeleteEvidence(tools.SkillQualityCandidate{Content: "useful implementation", VerifiedDuplicateOf: ""}, skillQualityDecision{ReasonCodes: []string{"unused"}}) {
		t.Fatal("lack of usage was accepted as objective deletion evidence")
	}
	if !hasObjectiveDeleteEvidence(tools.SkillQualityCandidate{Content: "TODO placeholder"}, skillQualityDecision{ReasonCodes: []string{"placeholder"}}) {
		t.Fatal("deterministic placeholder evidence was not recognized")
	}
	cfg, manager, candidate := setupSkillQualityMaintenanceTest(t, "# TODO placeholder\ndef run():\n    return 'hello world'\n")
	result := runSkillQualityMaintenance(context.Background(), cfg, slog.Default(), skillQualityTestClient{response: decisionJSON(t, candidate, "delete", 0.979, []string{"placeholder"})}, nil, nil, nil)
	if result.Deleted != 0 || result.ReviewRequired != 1 {
		t.Fatalf("below-threshold result=%+v", result)
	}
	if _, err := manager.GetSkill(candidate.ID); err != nil {
		t.Fatalf("below-threshold review deleted skill: %v", err)
	}
}

func TestSkillQualityMaintenanceInvalidJSONMakesNoChange(t *testing.T) {
	cfg, manager, candidate := setupSkillQualityMaintenanceTest(t, "# TODO placeholder\ndef run():\n    return 'hello world'\n")
	result := runSkillQualityMaintenance(context.Background(), cfg, slog.Default(), skillQualityTestClient{response: "not-json"}, nil, nil, nil)
	if result.Deleted != 0 || result.Improved != 0 || result.ReviewRequired != 1 {
		t.Fatalf("result=%+v", result)
	}
	if _, err := manager.GetSkill(candidate.ID); err != nil {
		t.Fatalf("invalid JSON changed skill: %v", err)
	}
	unchanged, _ := manager.GetSkill(candidate.ID)
	if unchanged.LastQualityReviewAt != nil {
		t.Fatal("invalid JSON incorrectly postponed the next quality review")
	}
}

func TestSkillQualityMaintenanceReferenceDaemonAndConcurrencyGuards(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cronManager := tools.NewCronManager(t.TempDir())
	t.Cleanup(func() { _ = cronManager.Close() })
	if _, err := cronManager.ManageSchedule("add", "quality-ref", "0 4 * * *", "Run maintenance_placeholder with execute_skill", "en"); err != nil {
		t.Fatal(err)
	}
	candidate := tools.SkillQualityCandidate{Name: "maintenance_placeholder"}
	if !hasScheduledSkillReference(candidate, cronManager) {
		t.Fatal("scheduled skill reference was not detected")
	}
	if _, err := stopMaintenanceDaemon(tools.SkillQualityCandidate{Name: "daemon-helper", IsDaemon: true}, nil); err == nil {
		t.Fatal("daemon mutation was allowed without a supervisor")
	}

	cfg := &config.Config{}
	cfg.Tools.SkillManager.Enabled = true
	skillQualityMaintenanceMu.Lock()
	result := runSkillQualityMaintenance(context.Background(), cfg, slog.Default(), nil, nil, nil, nil)
	skillQualityMaintenanceMu.Unlock()
	if result.ReviewRequired != 1 || len(result.Actions) != 1 {
		t.Fatalf("concurrent result=%+v", result)
	}
}

func TestSkillQualityMaintenanceCancellationMakesNoChange(t *testing.T) {
	cfg, manager, candidate := setupSkillQualityMaintenanceTest(t, "# TODO placeholder\ndef run():\n    return 'hello world'\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := runSkillQualityMaintenance(ctx, cfg, slog.Default(), skillQualityTestClient{response: decisionJSON(t, candidate, "delete", 0.99, []string{"placeholder"})}, nil, nil, nil)
	if result.Deleted != 0 || result.Improved != 0 {
		t.Fatalf("cancelled result=%+v", result)
	}
	if _, err := manager.GetSkill(candidate.ID); err != nil {
		t.Fatalf("cancelled review changed skill: %v", err)
	}
}
