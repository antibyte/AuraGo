package server

import (
	"strings"
	"testing"

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
