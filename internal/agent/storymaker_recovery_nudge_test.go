package agent

import (
	"strings"
	"testing"
)

func TestRecoveryNudgeReplacesGenericFeedback(t *testing.T) {
	allowed := []string{"storymaker_story"}
	nudge := func(native bool) string {
		if native {
			return "NATIVE NUDGE"
		}
		return "TEXT NUDGE"
	}
	for _, native := range []bool{false, true} {
		want := nudge(native)

		// With RunComplete: a text-only reply gets exactly one correction.
		s := announcementState(t, native, allowed)
		s.runCfg.RecoveryNudge = nudge
		s.runCfg.RunComplete = func() bool { return false }
		prose := "Die Szene ist fast fertig."
		parsed := ParsedToolResponse{Content: prose, SanitizedContent: prose}
		if _, _, retry, _ := handleAgentLoopRecoveries(s, prose, ToolCall{}, parsed, false, emotionBehaviorPolicy{}); !retry {
			t.Fatalf("native=%v: the first text-only reply was not nudged", native)
		}
		if got := s.req.Messages[len(s.req.Messages)-1].Content; got != want {
			t.Fatalf("native=%v: prose nudge = %q, want %q", native, got, want)
		}
		if _, _, retry, _ := handleAgentLoopRecoveries(s, prose, ToolCall{}, parsed, false, emotionBehaviorPolicy{}); retry {
			t.Fatalf("native=%v: the second text-only reply was nudged again", native)
		}

		// Without RunComplete: the generic detector uses the same text.
		s = announcementState(t, native, allowed)
		s.runCfg.RecoveryNudge = nudge
		if got := recoveryNudge(t, s); got != want || strings.Contains(got, "<done/>") {
			t.Fatalf("native=%v: generic path nudge = %q, want %q", native, got, want)
		}

		// The XML note must not send the model to tools the run cannot call.
		s = announcementState(t, native, allowed)
		s.runCfg.RecoveryNudge = nudge
		long := "<tool_call><function=storymaker_story>" + strings.Repeat("x", 600) + "</function></tool_call>"
		call := ToolCall{Action: "storymaker_story", IsTool: true, XMLFallbackDetected: true}
		if _, _, _, handled := handleAgentLoopRecoveries(s, long, call, ParsedToolResponse{}, false, emotionBehaviorPolicy{}); !handled {
			t.Fatalf("native=%v: XML fallback was not handled", native)
		}
		note := s.req.Messages[len(s.req.Messages)-1].Content
		if strings.Contains(note, "discover_tools") || strings.Contains(note, "file_editor") || !strings.Contains(note, "do NOT repeat it") {
			t.Fatalf("native=%v: XML note = %q", native, note)
		}
	}
}
