package agent

import (
	"fmt"
	"strings"

	"aurago/internal/memory"
)

func detectUserEmotionTrigger(lastUserMsg string, stm *memory.SQLiteMemory, sessionID string) (memory.EmotionTriggerType, string, float64) {
	msg := strings.ToLower(strings.TrimSpace(lastUserMsg))
	if msg == "" {
		return "", "", 0
	}

	inactivityHours := 0.0
	if stm != nil && strings.TrimSpace(sessionID) != "" {
		if hours, err := stm.GetHoursSinceLastUserMessage(sessionID); err == nil {
			inactivityHours = hours
		}
	}

	positiveMarkers := []string{
		"danke", "dankeschön", "thank you", "thanks", "great job", "good job", "super", "perfekt", "perfect", "nice work",
	}
	negativeMarkers := []string{
		"funktioniert nicht", "immer noch nicht", "geht nicht", "wrong", "falsch", "broken", "not working", "kaputt", "still broken", "doesn't work", "doesnt work",
	}

	for _, marker := range negativeMarkers {
		if strings.Contains(msg, marker) {
			detail := strings.TrimSpace(lastUserMsg)
			if inactivityHours >= 6 {
				detail = fmt.Sprintf("%s (after %.1f hours away)", detail, inactivityHours)
			}
			return memory.EmotionTriggerNegativeFeedback, detail, inactivityHours
		}
	}
	for _, marker := range positiveMarkers {
		if strings.Contains(msg, marker) {
			detail := strings.TrimSpace(lastUserMsg)
			if inactivityHours >= 6 {
				detail = fmt.Sprintf("%s (after %.1f hours away)", detail, inactivityHours)
			}
			return memory.EmotionTriggerPositiveFeedback, detail, inactivityHours
		}
	}
	if inactivityHours >= 6 {
		return memory.EmotionTriggerUserReturn, fmt.Sprintf("User returned after %.1f hours", inactivityHours), inactivityHours
	}
	return "", "", inactivityHours
}

func detectToolEmotionTrigger(tc ToolCall, consecutiveErrors, successCount int) (memory.EmotionTriggerType, string) {
	if tc.Action == "manage_plan" {
		switch tc.Operation {
		case "create":
			return memory.EmotionTriggerPlanCreated, "Created a new session plan"
		case "advance":
			return memory.EmotionTriggerPlanAdvanced, "Advanced the active plan task"
		case "set_blocker":
			return memory.EmotionTriggerPlanBlocked, "A plan task became blocked"
		case "clear_blocker":
			return memory.EmotionTriggerPlanUnblocked, "A blocked plan task was cleared"
		case "set_status":
			switch strings.ToLower(strings.TrimSpace(tc.Status)) {
			case memory.PlanStatusCompleted:
				return memory.EmotionTriggerPlanCompleted, "The active plan was marked completed"
			case memory.PlanStatusBlocked:
				return memory.EmotionTriggerPlanBlocked, "The active plan was marked blocked"
			case memory.PlanStatusActive:
				return memory.EmotionTriggerPlanUnblocked, "The active plan resumed"
			}
		}
	}
	if consecutiveErrors >= 2 {
		return memory.EmotionTriggerToolErrorStreak, fmt.Sprintf("There have been %d consecutive tool errors", consecutiveErrors)
	}
	if successCount >= 3 {
		return memory.EmotionTriggerToolSuccessStreak, fmt.Sprintf("There have been %d successful tool steps in a row", successCount)
	}
	return "", ""
}
