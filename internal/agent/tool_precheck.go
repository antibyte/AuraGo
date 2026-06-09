package agent

import (
	"encoding/json"
	"strings"
)

func precheckMessagingToolArgs(tc ToolCall, runCfg RunConfig, sessionID string) (string, bool) {
	if !tc.IsTool {
		return "", false
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