package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

func logInvokeToolArgumentSource(logger *slog.Logger, toolName, source string, input, args map[string]interface{}) {
	if logger == nil {
		return
	}
	logger.Warn("[ToolCommand] invoke_tool received "+source+" arguments",
		"tool", toolName,
		"input_keys", safeToolParamKeys(input),
		"arg_keys", safeToolParamKeys(args),
	)
}

func logInvalidToolCommand(logger *slog.Logger, toolName, operation, issue string, params map[string]interface{}) {
	if logger == nil {
		return
	}
	logger.Warn("[ToolCommand] Invalid tool command",
		"tool", toolName,
		"operation", operation,
		"issue", issue,
		"param_keys", safeToolParamKeys(params),
	)
}

func logToolCommandFailure(logger *slog.Logger, toolName, operation, message string, params map[string]interface{}) {
	if logger == nil {
		return
	}
	logger.Warn("[ToolCommand] Tool command failed",
		"tool", toolName,
		"operation", operation,
		"message", Truncate(strings.TrimSpace(message), 240),
		"param_keys", safeToolParamKeys(params),
	)
}

func safeToolParamKeys(params map[string]interface{}) []string {
	if len(params) == 0 {
		return nil
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func toolCommandErrorMessage(result string) (string, bool) {
	raw := strings.TrimSpace(result)
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "Tool Output:"))
	if raw == "" {
		return "", false
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", false
	}
	if !strings.EqualFold(fmt.Sprint(payload["status"]), "error") {
		return "", false
	}
	msg := strings.TrimSpace(fmt.Sprint(payload["message"]))
	return msg, msg != ""
}
