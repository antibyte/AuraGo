package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/memory"
)

func emitSessionPlanUpdate(broker FeedbackBroker, shortTermMem *memory.SQLiteMemory, sessionID string, logger *slog.Logger) {
	if broker == nil || shortTermMem == nil {
		return
	}
	plan, err := shortTermMem.GetSessionPlan(sessionID)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to fetch session plan for SSE update", "session_id", sessionID, "error", err)
		}
		return
	}
	payload := map[string]interface{}{"plan": plan}
	raw, err := json.Marshal(payload)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to marshal session plan SSE payload", "session_id", sessionID, "error", err)
		}
		return
	}
	broker.Send("plan_update", string(raw))
}

func recordPlanToolProgress(shortTermMem *memory.SQLiteMemory, sessionID string, tc ToolCall, result string, logger *slog.Logger) {
	if shortTermMem == nil || strings.TrimSpace(sessionID) == "" || strings.TrimSpace(tc.Action) == "" || tc.Action == "manage_plan" {
		return
	}
	plan, err := shortTermMem.GetSessionPlan(sessionID)
	if err != nil || plan == nil || plan.Status != memory.PlanStatusActive {
		return
	}
	summary := compactActivityToolResult(tc.Action, result)
	if summary != "" {
		if err := shortTermMem.AppendPlanEvent(plan.ID, "tool", summary); err != nil && logger != nil {
			logger.Debug("Failed to append automatic plan tool event", "plan_id", plan.ID, "action", tc.Action, "error", err)
		}
	}
	if plan.CurrentTaskID == "" {
		return
	}
	for _, artifact := range extractPlanArtifacts(tc, result) {
		if _, err := shortTermMem.AttachPlanTaskArtifact(plan.ID, plan.CurrentTaskID, artifact); err != nil && logger != nil {
			logger.Debug("Failed to attach automatic plan artifact", "plan_id", plan.ID, "task_id", plan.CurrentTaskID, "artifact", artifact.Label, "error", err)
		}
	}
}

func extractPlanArtifacts(tc ToolCall, result string) []memory.PlanArtifact {
	artifacts := make([]memory.PlanArtifact, 0, 6)
	add := func(kind, label, value string) {
		kind = strings.TrimSpace(kind)
		label = strings.TrimSpace(label)
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if kind == "" {
			kind = "artifact"
		}
		if label == "" {
			label = kind
		}
		for _, existing := range artifacts {
			if existing.Type == kind && existing.Label == label && existing.Value == value {
				return
			}
		}
		artifacts = append(artifacts, memory.PlanArtifact{Type: kind, Label: label, Value: value})
	}

	if tc.FilePath != "" {
		add("file", "path", tc.FilePath)
	}
	if tc.Destination != "" {
		add("file", "destination", tc.Destination)
	}
	if tc.URL != "" {
		add("url", "url", tc.URL)
	}
	if tc.OutputFile != "" {
		add("file", "output_file", tc.OutputFile)
	}

	payload := strings.TrimSpace(result)
	payload = strings.TrimPrefix(payload, "[Tool Output]\n")
	payload = strings.TrimPrefix(payload, "Tool Output: ")
	payload = strings.TrimSpace(payload)
	if strings.HasPrefix(payload, "<external_data>") && strings.HasSuffix(payload, "</external_data>") {
		payload = strings.TrimPrefix(payload, "<external_data>")
		payload = strings.TrimSuffix(payload, "</external_data>")
		payload = strings.TrimSpace(payload)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &obj); err == nil {
		extractPlanArtifactsFromMap(obj, &artifacts)
	}
	if len(artifacts) > 6 {
		artifacts = artifacts[:6]
	}
	return artifacts
}

func extractPlanArtifactsFromMap(obj map[string]interface{}, artifacts *[]memory.PlanArtifact) {
	add := func(kind, label string, raw interface{}) {
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" || value == "<nil>" {
			return
		}
		for _, existing := range *artifacts {
			if existing.Type == kind && existing.Label == label && existing.Value == value {
				return
			}
		}
		*artifacts = append(*artifacts, memory.PlanArtifact{Type: kind, Label: label, Value: value})
	}
	for key, kind := range map[string]string{
		"file":        "file",
		"path":        "file",
		"output_file": "file",
		"web_path":    "file",
		"url":         "url",
		"ssl_url":     "url",
		"preview_url": "url",
		"site_id":     "id",
		"deploy_id":   "id",
		"id":          "id",
		"report":      "report",
	} {
		if val, ok := obj[key]; ok {
			add(kind, key, val)
		}
	}
	if nested, ok := obj["data"].(map[string]interface{}); ok {
		extractPlanArtifactsFromMap(nested, artifacts)
	}
}
