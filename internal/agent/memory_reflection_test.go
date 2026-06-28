package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

type reflectionTestClient struct {
	responses []string
	requests  []openai.ChatCompletionRequest
}

func (c *reflectionTestClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	if len(c.responses) == 0 {
		return openai.ChatCompletionResponse{}, nil
	}
	content := c.responses[0]
	c.responses = c.responses[1:]
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: content,
			},
		}},
	}, nil
}

func (c *reflectionTestClient) CreateChatCompletionStream(_ context.Context, _ openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, nil
}

func TestMemoryReflectNativeSchemaIncludesFocusAndOutputFormat(t *testing.T) {
	tools := appendMemoryToolSchemas(nil, ToolFeatureFlags{
		MemoryEnabled:         true,
		MemoryAnalysisEnabled: true,
	})
	params := nativeToolParams(t, tools, "memory_reflect")
	properties := params["properties"].(map[string]interface{})

	for _, key := range []string{"scope", "focus", "output_format"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("memory_reflect schema missing property %q", key)
		}
	}
	focusEnum := properties["focus"].(map[string]interface{})["enum"].([]interface{})
	if !stringSliceContainsInterface(focusEnum, "errors") || !stringSliceContainsInterface(focusEnum, "relationships") {
		t.Fatalf("focus enum = %#v, want errors and relationships", focusEnum)
	}
	formatEnum := properties["output_format"].(map[string]interface{})["enum"].([]interface{})
	if !stringSliceContainsInterface(formatEnum, "action_items") || !stringSliceContainsInterface(formatEnum, "insights_only") {
		t.Fatalf("output_format enum = %#v, want action_items and insights_only", formatEnum)
	}
}

func TestBuildMemoryReflectionInputIncludesErrorLearningAndRules(t *testing.T) {
	stm := newReflectionTestDB(t)
	if err := stm.RecordError("docker", "container 12345 missing"); err != nil {
		t.Fatalf("RecordError first: %v", err)
	}
	if err := stm.RecordError("docker", "container 67890 missing"); err != nil {
		t.Fatalf("RecordError second: %v", err)
	}
	if err := stm.UpsertLearnedRule(&memory.LearnedRule{
		ToolName:   "docker",
		Pattern:    "container <ID> missing",
		Rule:       "List containers before inspecting a container ID.",
		Confidence: 0.8,
		Hits:       2,
		Active:     true,
	}); err != nil {
		t.Fatalf("UpsertLearnedRule: %v", err)
	}

	input := buildMemoryReflectionInput(stm, nil, nil, memoryReflectionRequest{
		Scope: "week",
		Focus: "errors",
	})
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal input: %v", err)
	}
	text := string(payload)
	for _, want := range []string{"docker", "container", "missing", "List containers before inspecting"} {
		if !strings.Contains(text, want) {
			t.Fatalf("reflection input missing %q in %s", want, text)
		}
	}
}

func TestParseMemoryReflectionResultFlagsLowQualityAndAcceptsActionableJSON(t *testing.T) {
	low, err := parseMemoryReflectionResult(`{"summary":"ok"}`)
	if err != nil {
		t.Fatalf("parse low quality result: %v", err)
	}
	flags := validateMemoryReflectionResult(low, true)
	if !stringSliceContains(flags, "low_quality") {
		t.Fatalf("quality flags = %v, want low_quality", flags)
	}

	good, err := parseMemoryReflectionResult(`{
		"patterns":["Docker inspection failures recur"],
		"contradictions":["Core memory says NAS is alpha, recent journal says beta"],
		"gaps":["Confirm current NAS host"],
		"suggestions":["Verify NAS host before storing another fact"],
		"error_patterns":["docker inspect fails repeatedly without a learned rule"],
		"learned_rule_review":["Add a rule to list containers first"],
		"action_items":["Verify NAS host","Create a docker inspection learned rule"],
		"summary":"The recent memory window shows recurring Docker inspection failures and one host identity inconsistency. The next useful move is to verify the NAS host fact and add a small learned rule for container lookups."
	}`)
	if err != nil {
		t.Fatalf("parse actionable result: %v", err)
	}
	if flags := validateMemoryReflectionResult(good, true); len(flags) != 0 {
		t.Fatalf("quality flags = %v, want none", flags)
	}
}

func TestRunMemoryReflectionKeepsParseFailedFlagWhenRetryIsInvalid(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	client := &reflectionTestClient{responses: []string{
		`{"summary":"ok"}`,
		`not json`,
	}}

	result, err := runMemoryReflection(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil, client, nil, memoryReflectionRequest{})
	if err != nil {
		t.Fatalf("runMemoryReflection: %v", err)
	}
	if !stringSliceContains(result.QualityFlags, "low_quality") {
		t.Fatalf("quality flags = %v, want low_quality", result.QualityFlags)
	}
	if !stringSliceContains(result.QualityFlags, "parse_failed") {
		t.Fatalf("quality flags = %v, want parse_failed after invalid retry", result.QualityFlags)
	}
}

func TestBuildMemoryReflectionActionIssuesCapsAndFingerprintsFindings(t *testing.T) {
	result := memoryReflectionResult{
		Summary: "The recent memory window has actionable reflection findings that require follow-up.",
		Contradictions: []string{
			"Core memory says NAS is alpha, recent journal says beta.",
			"Project owner differs between memory sources.",
		},
		ErrorPatterns: []string{"docker inspect fails repeatedly without a learned rule."},
		Gaps:          []string{"Confirm current NAS host before storing another fact."},
		QualityFlags:  []string{"low_quality"},
	}

	issues := buildMemoryReflectionActionIssues("recent", result)
	if len(issues) != 3 {
		t.Fatalf("issues = %d, want capped 3: %#v", len(issues), issues)
	}
	wantFingerprints := []string{
		"memory_reflect|recent|contradiction|0",
		"memory_reflect|recent|contradiction|1",
		"memory_reflect|recent|missing_rule|0",
	}
	for i, want := range wantFingerprints {
		if issues[i].Fingerprint != want {
			t.Fatalf("issue %d fingerprint = %q, want %q", i, issues[i].Fingerprint, want)
		}
	}
}

func TestRunWeeklyReflectionJobRetriesLowQualityStoresJournalAndNotification(t *testing.T) {
	releaseWeeklyReflectionClaim()
	t.Cleanup(releaseWeeklyReflectionClaim)

	stm := newReflectionTestDB(t)
	cfg := &config.Config{}
	cfg.MemoryAnalysis.Enabled = true
	cfg.MemoryAnalysis.WeeklyReflection = true
	cfg.MemoryAnalysis.ReflectionDay = strings.ToLower(time.Now().Weekday().String())
	cfg.LLM.Model = "test-model"

	client := &reflectionTestClient{responses: []string{
		`{"summary":"ok"}`,
		`{
			"patterns":["Docker inspection failures recur"],
			"contradictions":["Core memory says NAS is alpha, recent journal says beta"],
			"gaps":["Confirm current NAS host"],
			"suggestions":["Verify NAS host before storing another fact"],
			"error_patterns":["docker inspect fails repeatedly without a learned rule"],
			"learned_rule_review":["Add a rule to list containers first"],
			"action_items":["Verify NAS host","Create a docker inspection learned rule"],
			"summary":"The weekly reflection found recurring Docker inspection failures and one host identity inconsistency. The next useful move is to verify the NAS host fact and add a small learned rule for container lookups."
		}`,
	}}

	ran, err := runWeeklyReflectionJob(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), client, stm, nil, nil, nil)
	if err != nil {
		t.Fatalf("runWeeklyReflectionJob: %v", err)
	}
	if !ran {
		t.Fatal("expected weekly reflection job to run")
	}
	if got := len(client.requests); got != 2 {
		t.Fatalf("LLM calls = %d, want retry count 2", got)
	}

	today := time.Now().Format("2006-01-02")
	entries, err := stm.GetJournalEntries(today, today, []string{"reflection"}, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("reflection entries = %d, want 1", len(entries))
	}
	if !strings.Contains(entries[0].Content, "Action Items") || !stringSliceContains(entries[0].Tags, "reflection") || !stringSliceContains(entries[0].Tags, "recent") {
		t.Fatalf("journal reflection not richly stored: %+v", entries[0])
	}

	notifications, err := stm.GetUnreadNotifications()
	if err != nil {
		t.Fatalf("GetUnreadNotifications: %v", err)
	}
	if len(notifications) != 1 || !strings.Contains(notifications[0], "Weekly memory reflection") {
		t.Fatalf("notifications = %#v, want weekly reflection notification", notifications)
	}

	ranAgain, err := runWeeklyReflectionJob(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), client, stm, nil, nil, nil)
	if err != nil {
		t.Fatalf("second runWeeklyReflectionJob: %v", err)
	}
	if ranAgain {
		t.Fatal("expected second weekly reflection job to skip after claim/journal entry")
	}
}

func nativeToolParams(t *testing.T, tools []openai.Tool, name string) map[string]interface{} {
	t.Helper()
	for _, candidate := range tools {
		if candidate.Function.Name != name {
			continue
		}
		raw, err := json.Marshal(candidate.Function.Parameters)
		if err != nil {
			t.Fatalf("marshal parameters: %v", err)
		}
		var params map[string]interface{}
		if err := json.Unmarshal(raw, &params); err != nil {
			t.Fatalf("unmarshal parameters: %v", err)
		}
		return params
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func newReflectionTestDB(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.InitErrorLearningTable(); err != nil {
		t.Fatalf("InitErrorLearningTable: %v", err)
	}
	if err := stm.InitLearnedRulesTable(); err != nil {
		t.Fatalf("InitLearnedRulesTable: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func stringSliceContainsInterface(values []interface{}, want string) bool {
	for _, value := range values {
		if text, ok := value.(string); ok && text == want {
			return true
		}
	}
	return false
}
