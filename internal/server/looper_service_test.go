package server

import (
	"fmt"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/desktop"

	"github.com/sashabaranov/go-openai"
)

func TestParseEvaluation(t *testing.T) {
	t.Parallel()

	t.Run("plain json", func(t *testing.T) {
		ev, ok := parseEvaluation(`{"score": 82, "done": false, "feedback": "tighten the ending", "summary": "solid draft"}`)
		if !ok || ev.Score != 82 || ev.Done || ev.Feedback != "tighten the ending" || ev.Summary != "solid draft" {
			t.Fatalf("parsed = %+v ok=%v", ev, ok)
		}
	})

	t.Run("fenced json", func(t *testing.T) {
		ev, ok := parseEvaluation("```json\n{\"score\": 90.4, \"done\": true, \"feedback\": \"ship it\", \"summary\": \"ready\"}\n```")
		if !ok || ev.Score != 90 || !ev.Done {
			t.Fatalf("fenced = %+v ok=%v", ev, ok)
		}
	})

	t.Run("prose wrapper", func(t *testing.T) {
		ev, ok := parseEvaluation("Here you go:\n{\"score\": 40, \"done\": false, \"feedback\": \"rewrite\"}\nThanks")
		if !ok || ev.Score != 40 || ev.Feedback != "rewrite" {
			t.Fatalf("wrapper = %+v ok=%v", ev, ok)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, ok := parseEvaluation("score is fine I guess"); ok {
			t.Fatal("expected invalid")
		}
		if _, ok := parseEvaluation(`{"done": true}`); ok {
			t.Fatal("missing score must be invalid")
		}
	})
}

func TestBuildLooperWorkPromptRoundOneVsLater(t *testing.T) {
	t.Parallel()
	cfg := desktop.LooperRunConfig{Goal: "Write the story", Work: "Improve the file", MaxRounds: 8}
	first := buildLooperWorkPrompt(cfg, 1, "", "", nil)
	if !strings.Contains(first, "Round 1 of 8") || !strings.Contains(first, "Write the story") {
		t.Fatalf("round-1 prompt missing framing: %q", first)
	}
	if !strings.Contains(first, "If the target artifact does not exist yet") {
		t.Fatalf("round-1 prompt missing create-first guidance: %q", first)
	}
	if !strings.Contains(first, "Do not open the artifact in a desktop app") {
		t.Fatalf("work prompt must keep finish/open out of the work step: %q", first)
	}
	if strings.Contains(first, "Previous evaluation") {
		t.Fatalf("round-1 prompt unexpectedly includes previous evaluation: %q", first)
	}

	later := buildLooperWorkPrompt(cfg, 3, "add a twist", "rewrote scene 2", []int{40, 55})
	if strings.Contains(later, "If the target artifact does not exist yet") {
		t.Fatalf("later prompt still tells the model to create the first draft: %q", later)
	}
	if !strings.Contains(later, "Previous evaluation (score 55)") || !strings.Contains(later, "add a twist") {
		t.Fatalf("later prompt missing feedback: %q", later)
	}
	if !strings.Contains(later, "Score history: 40, 55") || !strings.Contains(later, "Previous work summary") {
		t.Fatalf("later prompt missing history: %q", later)
	}
}

func TestLooperReachedTargetIgnoresOptimisticDone(t *testing.T) {
	t.Parallel()
	if looperReachedTarget(looperEvaluation{Score: 88, Done: true}, 98) {
		t.Fatal("done below the target score must not complete the loop")
	}
	if looperReachedTarget(looperEvaluation{Score: 97, Done: false}, 98) {
		t.Fatal("score below target must continue")
	}
	if !looperReachedTarget(looperEvaluation{Score: 98, Done: false}, 98) {
		t.Fatal("score at target must complete even when done is false")
	}
	if !looperReachedTarget(looperEvaluation{Score: 100, Done: true}, 85) {
		t.Fatal("score above target must complete")
	}
}

func TestBuildLooperEvaluatePromptNamesTargetScore(t *testing.T) {
	t.Parallel()
	got := buildLooperEvaluatePrompt(desktop.LooperRunConfig{
		Goal:        "Write the story",
		Evaluate:    "Judge the ending",
		TargetScore: 98,
	}, "wrote draft")
	if !strings.Contains(got, "98") || !strings.Contains(got, "does not stop the loop") {
		t.Fatalf("evaluate prompt must bind completion to the target score: %q", got)
	}
	if strings.Contains(strings.ToLower(got), "read files") || strings.Contains(strings.ToLower(got), "run tests") {
		t.Fatalf("evaluate prompt must stay tool-free: %q", got)
	}
	if !strings.Contains(got, "Do not call tools") {
		t.Fatalf("evaluate prompt must forbid tool calls: %q", got)
	}
}

func TestLooperEvaluationOrFallbackKeepsTheLoopAlive(t *testing.T) {
	t.Parallel()
	failed := looperEvaluationOrFallback("", fmt.Errorf("required minimal loop tool schemas were dropped"))
	if failed.Score != 0 || !strings.Contains(failed.Feedback, "tool schemas were dropped") {
		t.Fatalf("exec failure fallback = %+v", failed)
	}
	if failed.Summary == "" {
		t.Fatal("exec failure fallback needs a summary")
	}
	invalid := looperEvaluationOrFallback("not json", nil)
	if invalid.Score != 0 || invalid.Feedback != "not json" {
		t.Fatalf("parse failure fallback = %+v", invalid)
	}
	ok := looperEvaluationOrFallback(`{"score":77,"done":false,"feedback":"tighten","summary":"draft"}`, nil)
	if ok.Score != 77 || ok.Feedback != "tighten" {
		t.Fatalf("valid evaluation fallback = %+v", ok)
	}
	recovered := looperEvaluationOrFallback(`{"score":83,"done":false,"feedback":"keep going","summary":"draft"}`, fmt.Errorf("unexpected tool-call text in llm response"))
	if recovered.Score != 83 || recovered.Feedback != "keep going" {
		t.Fatalf("parseable evaluate JSON must win over exec error: %+v", recovered)
	}
}

func TestLooperRecoversWorkFromToolOutputsAfterFormatError(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("unexpected tool-call text in llm response")
	history := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Work prompt"},
		{Role: openai.ChatMessageRoleAssistant, Content: `{"action":"filesystem","operation":"write"}`},
		{Role: openai.ChatMessageRoleTool, Name: "filesystem", Content: `{"status":"ok","path":"Documents/Looper/story.md"}`},
	}
	got, ok := looperRecoveredWorkReport(err, history, "Work prompt")
	if !ok || !strings.Contains(got, "Documents/Looper/story.md") {
		t.Fatalf("recovered work = %q ok=%v", got, ok)
	}
	if strings.Contains(got, `"action":"filesystem"`) {
		t.Fatalf("recovered work must not treat textual tool JSON as the artifact: %q", got)
	}
	if _, ok := looperRecoveredWorkReport(err, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Work prompt"},
		{Role: openai.ChatMessageRoleAssistant, Content: `{"action":"filesystem","operation":"write"}`},
	}, "Work prompt"); ok {
		t.Fatal("format error without tool output must not look recovered")
	}
	if _, ok := looperRecoveredWorkReport(fmt.Errorf("llm call failed"), history, "Work prompt"); ok {
		t.Fatal("non-format errors must not recover")
	}
}

func TestLooperShouldKeepMinimalLoopResult(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("unexpected tool-call text in llm response")
	workHistory := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Work prompt"},
		{Role: openai.ChatMessageRoleTool, Name: "filesystem", Content: `{"status":"ok"}`},
	}
	if !looperShouldKeepMinimalLoopResult("work", err, agent.MinimalLoopResult{}, workHistory, "Work prompt") {
		t.Fatal("work with tool output should skip retries")
	}
	evalJSON := `{"score":91,"done":false,"feedback":"tighten","summary":"ok"}`
	if !looperShouldKeepMinimalLoopResult("evaluate", err, agent.MinimalLoopResult{}, []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleAssistant, Content: evalJSON},
	}, "Evaluate prompt") {
		t.Fatal("parseable evaluate JSON should skip retries")
	}
	if looperShouldKeepMinimalLoopResult("evaluate", err, agent.MinimalLoopResult{}, nil, "Evaluate prompt") {
		t.Fatal("empty evaluate failure should still retry")
	}
}

func TestLooperEvaluationJSONIsNotAToolCall(t *testing.T) {
	t.Parallel()
	raw := `{"score":88,"done":true,"feedback":"tighten the ending","summary":"draft saved"}`
	if agent.ParseToolCall(raw).IsTool {
		t.Fatal("evaluation JSON must not be classified as a textual tool call")
	}
}

func TestStallWithoutImprovementUsesDesktopHelper(t *testing.T) {
	t.Parallel()
	if !desktop.StallWithoutImprovement([]int{60, 60, 60, 60}, 3) {
		t.Fatal("expected stall")
	}
}

func TestBuildLooperFinishHistoryIncludesGoalAndDesktopRules(t *testing.T) {
	t.Parallel()
	got := buildLooperFinishHistory(
		"base system",
		"Write a story in Documents/Looper/short-story.md",
		"Saved Documents/Looper/short-story.md",
		"Score 88: ending works",
		"Ready to open",
	)
	if len(got) < 2 {
		t.Fatalf("finish history too short: %#v", got)
	}
	if !strings.Contains(got[0].Content, "open_in_app") || !strings.Contains(got[0].Content, "app_id \"writer\"") {
		t.Fatalf("finish system prompt missing desktop open rules: %q", got[0].Content)
	}
	joined := ""
	for _, msg := range got {
		joined += msg.Content
	}
	for _, want := range []string{"Write a story", "Final work result:", "short-story.md", "Final evaluation:", "ending works"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("finish history missing %q: %q", want, joined)
		}
	}
}

func TestBuildActionFinishResultIncludesActionToolOutput(t *testing.T) {
	t.Parallel()
	history := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Plan prompt"},
		{Role: openai.ChatMessageRoleTool, Name: "virtual_desktop", Content: `{"path":"Documents/old.docx"}`},
		{Role: openai.ChatMessageRoleUser, Content: "Action prompt"},
		{Role: openai.ChatMessageRoleTool, Name: "virtual_desktop", Content: `{"status":"ok","data":{"path":"Documents/final-story.docx"}}`},
	}

	got := buildActionFinishResult("Saved the final story.", history, "Action prompt")
	if !strings.Contains(got, "Saved the final story.") || !strings.Contains(got, "Documents/final-story.docx") {
		t.Fatalf("action finish result missing response or tool output: %q", got)
	}
	if strings.Contains(got, "Documents/old.docx") {
		t.Fatalf("action finish result included tool output before the action prompt: %q", got)
	}
}

func TestEstimateLooperCostUSDPositive(t *testing.T) {
	t.Parallel()
	cost := estimateLooperCostUSD(1_000_000, 1_000_000)
	if cost <= 0 {
		t.Fatalf("cost = %v, want > 0", cost)
	}
}
