package agent

import (
	"encoding/json"
	"strings"
)

func precheckMessagingToolArgs(tc ToolCall, runCfg RunConfig, sessionID string) (string, bool) {
	if !tc.IsTool {
		return "", false
	}

	if out, blocked := precheckAutonomousMissionMutation(tc, runCfg, sessionID); blocked {
		return out, true
	}

	var message string
	switch tc.Action {
	case "send_telegram":
		message = decodeSendTelegramArgs(tc).Message
	case "send_notification", "notification_center", "send_push_notification", "web_push":
		message = decodeNotificationArgs(tc).Message
	case "send_discord":
		message = decodeDiscordMessageArgs(tc).Message
	default:
		return "", false
	}

	if strings.TrimSpace(message) != "" {
		return "", false
	}

	payload := map[string]interface{}{}
	if isAutonomousAgentRun(runCfg, sessionID) {
		payload["status"] = "skipped"
		payload["message"] = tc.Action + " skipped: no message to send"
	} else {
		payload["status"] = "error"
		payload["message"] = "message is required"
	}
	encoded, _ := json.Marshal(payload)
	return "Tool Output: " + string(encoded), true
}

func precheckAutonomousMissionMutation(tc ToolCall, runCfg RunConfig, sessionID string) (string, bool) {
	if tc.Action != "manage_missions" || !isHeartbeatRun(runCfg, sessionID) {
		return "", false
	}
	operation := strings.ToLower(strings.TrimSpace(firstNonEmptyToolString(
		tc.Operation,
		stringValueFromMap(tc.Params, "operation"),
	)))
	switch operation {
	case "list", "history", "get", "status":
		return "", false
	case "create", "add", "update", "edit", "delete", "remove", "run", "run_now", "execute":
		payload := map[string]interface{}{
			"status":  "skipped",
			"message": "manage_missions " + operation + " skipped during heartbeat: heartbeat runs are read-only for mission control",
		}
		encoded, _ := json.Marshal(payload)
		return "Tool Output: " + string(encoded), true
	default:
		return "", false
	}
}

func isHeartbeatRun(runCfg RunConfig, sessionID string) bool {
	return strings.EqualFold(strings.TrimSpace(runCfg.MessageSource), "heartbeat") ||
		strings.EqualFold(strings.TrimSpace(sessionID), "heartbeat")
}
