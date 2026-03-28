package agent

import (
	"testing"

	"aurago/internal/memory"
)

func TestDetectUserEmotionTriggerRecognizesPositiveFeedback(t *testing.T) {
	trigger, detail, hours := detectUserEmotionTrigger("Danke, das war perfekt gelöst.", nil, "")
	if trigger != memory.EmotionTriggerPositiveFeedback {
		t.Fatalf("trigger = %q, want %q", trigger, memory.EmotionTriggerPositiveFeedback)
	}
	if detail == "" {
		t.Fatal("expected trigger detail")
	}
	if hours != 0 {
		t.Fatalf("hours = %.1f, want 0", hours)
	}
}

func TestDetectUserEmotionTriggerRecognizesNegativeFeedback(t *testing.T) {
	trigger, _, _ := detectUserEmotionTrigger("Das funktioniert immer noch nicht.", nil, "")
	if trigger != memory.EmotionTriggerNegativeFeedback {
		t.Fatalf("trigger = %q, want %q", trigger, memory.EmotionTriggerNegativeFeedback)
	}
}

func TestDetectToolEmotionTriggerRecognizesPlanEvents(t *testing.T) {
	trigger, detail := detectToolEmotionTrigger(ToolCall{Action: "manage_plan", Operation: "advance"}, 0, 0)
	if trigger != memory.EmotionTriggerPlanAdvanced {
		t.Fatalf("trigger = %q, want %q", trigger, memory.EmotionTriggerPlanAdvanced)
	}
	if detail == "" {
		t.Fatal("expected plan trigger detail")
	}
}

func TestDetectToolEmotionTriggerRecognizesErrorAndSuccessStreaks(t *testing.T) {
	trigger, _ := detectToolEmotionTrigger(ToolCall{Action: "filesystem"}, 2, 0)
	if trigger != memory.EmotionTriggerToolErrorStreak {
		t.Fatalf("error streak trigger = %q, want %q", trigger, memory.EmotionTriggerToolErrorStreak)
	}

	trigger, _ = detectToolEmotionTrigger(ToolCall{Action: "filesystem"}, 0, 3)
	if trigger != memory.EmotionTriggerToolSuccessStreak {
		t.Fatalf("success streak trigger = %q, want %q", trigger, memory.EmotionTriggerToolSuccessStreak)
	}
}
