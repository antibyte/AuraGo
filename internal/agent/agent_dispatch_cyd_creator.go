package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"

	"aurago/internal/cydcreator"
)

// cydCreatorWriteTools change the working copy or start builds; they are
// refused while the desktop is read-only.
var cydCreatorWriteTools = []string{cydcreator.ToolUpdateLayout, cydcreator.ToolWriteFile, cydcreator.ToolDeleteFile,
	cydcreator.ToolBuild, cydcreator.ToolSubmitPlan, cydcreator.ToolFinish}

func cydCreatorToolJSON(value any) string {
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return toolErrorJSON("could not serialize the CYD Creator result")
	}
	return "Tool Output: " + strings.TrimSuffix(data.String(), "\n")
}

// cydCreatorToolError reports a service error with its CYD_* code and details
// so the agent can correct the call.
func cydCreatorToolError(err error) string {
	var typed *cydcreator.Error
	if errors.As(err, &typed) {
		body := map[string]any{"status": "error", "code": typed.Code, "message": typed.Message}
		if len(typed.Details) > 0 {
			body["details"] = typed.Details
		}
		return cydCreatorToolJSON(body)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return cydCreatorToolJSON(map[string]any{"status": "cancelled", "message": "the job was stopped"})
	}
	// An untyped error can carry host paths: log it, answer generically (like
	// the service's CYD_INTERNAL).
	slog.Warn("CYD Creator tool failed", "error", err)
	return cydCreatorToolJSON(map[string]any{"status": "error", "code": cydcreator.CodeInternal, "message": "the CYD Creator tool failed"})
}

func cydCreatorPolicyDenied(message string) string {
	return cydCreatorToolJSON(map[string]any{"status": "policy_denied", "message": message})
}

// cydLooseObjectKeys are object-valued layout op fields a provider may send as
// JSON strings (provider-fragile free objects become string schemas).
var cydLooseObjectKeys = map[string]bool{"widget": true, "screen_data": true, "set": true, "props": true, "style": true, "action": true}

func decodeCYDLooseObjects(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		for key, value := range typed {
			if s, ok := value.(string); ok && cydLooseObjectKeys[key] {
				var obj map[string]any
				if trimmed := strings.TrimSpace(s); strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &obj) == nil {
					value = obj
				}
			}
			typed[key] = decodeCYDLooseObjects(value)
		}
		return typed
	case []any:
		for i := range typed {
			typed[i] = decodeCYDLooseObjects(typed[i])
		}
		return typed
	}
	return v
}

// decodeCYDLayoutOps accepts the ops array (or a JSON string holding it) with
// string-encoded nested objects and decodes it strictly into LayoutOps.
func decodeCYDLayoutOps(raw any) ([]cydcreator.LayoutOp, error) {
	if s, ok := raw.(string); ok {
		var decoded any
		if err := json.Unmarshal([]byte(s), &decoded); err != nil {
			return nil, fmt.Errorf("ops is not a JSON array: %w", err)
		}
		raw = decoded
	}
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, errors.New("ops must be a non-empty array")
	}
	data, err := json.Marshal(decodeCYDLooseObjects(list))
	if err != nil {
		return nil, err
	}
	var ops []cydcreator.LayoutOp
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ops); err != nil {
		return nil, fmt.Errorf("invalid op: %w", err)
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, errors.New("invalid ops: trailing data")
	}
	return ops, nil
}

func isCYDCreatorTool(name string) bool { return slices.Contains(cydcreator.ToolNames(), name) }

// dispatchCYDCreator executes the CYD Creator job tools. The job comes from
// the context bound by the server's runner; an explicit job_id argument must
// match it. The cyd_creator switch is re-checked on every call (live
// authorization intersects it), writes also need a writable desktop.
func dispatchCYDCreator(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	if !isCYDCreatorTool(tc.Action) {
		return "", false
	}
	jobID := cydcreator.JobIDFromContext(ctx)
	if jobID == "" {
		return cydCreatorPolicyDenied("CYD Creator tools only work inside CYD Creator agent jobs"), true
	}
	if arg := toolArgString(tc.Params, "job_id"); arg != "" && arg != jobID {
		return cydCreatorToolJSON(map[string]any{"status": "error", "message": "job_id does not match this CYD Creator job"}), true
	}
	if dc == nil || dc.Cfg == nil || !dc.Cfg.CYDCreator.Enabled {
		return cydCreatorPolicyDenied("CYD Creator is switched off"), true
	}
	if dc.Cfg.VirtualDesktop.ReadOnly && slices.Contains(cydCreatorWriteTools, tc.Action) {
		return cydCreatorPolicyDenied("the desktop is read-only"), true
	}
	service := cydcreator.DefaultService()
	if service == nil {
		return cydCreatorToolJSON(map[string]any{"status": "error", "message": "CYD Creator is not available"}), true
	}
	path := toolArgString(tc.Params, "path")
	switch tc.Action {
	case cydcreator.ToolReadProject:
		view, err := service.ReadProjectForJob(ctx, jobID)
		if err != nil {
			return cydCreatorToolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "project": view}), true
	case cydcreator.ToolAPIReference:
		topic := toolArgString(tc.Params, "topic")
		if topic == "" {
			topic = "overview"
		}
		text, err := service.APIReference(ctx, jobID, topic)
		if err != nil {
			return cydCreatorToolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "topic": topic, "reference": text}), true
	case cydcreator.ToolSubmitPlan:
		raw, ok := tc.Params["plan"]
		if !ok || raw == nil {
			return cydCreatorToolJSON(map[string]any{"status": "error", "message": "plan is required"}), true
		}
		data, err := json.Marshal(raw)
		if err != nil {
			return cydCreatorToolError(err), true
		}
		plan, err := service.SubmitPlan(ctx, jobID, data)
		if err != nil {
			return cydCreatorToolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "plan": plan,
			"next_action": "Plan accepted. Stop now: the user confirms the plan before anything is built."}), true
	case cydcreator.ToolUpdateLayout:
		ops, err := decodeCYDLayoutOps(tc.Params["ops"])
		if err != nil {
			return cydCreatorToolJSON(map[string]any{"status": "error", "code": cydcreator.CodeBadRequest, "message": err.Error()}), true
		}
		update, err := service.UpdateLayoutForJob(ctx, jobID, ops)
		if err != nil {
			return cydCreatorToolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "results": update.Results, "issues": update.Issues}), true
	case cydcreator.ToolReadFile:
		file, err := service.ReadJobFile(ctx, jobID, path)
		if err != nil {
			return cydCreatorToolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "file": file}), true
	case cydcreator.ToolWriteFile:
		content, _ := tc.Params["content"].(string)
		sha, err := service.WriteJobFile(ctx, jobID, path, content)
		if err != nil {
			return cydCreatorToolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "path": path, "sha256": sha,
			"next_action": "Run cyd_creator_build when your change is complete."}), true
	case cydcreator.ToolDeleteFile:
		if err := service.DeleteJobFile(ctx, jobID, path); err != nil {
			return cydCreatorToolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "path": path}), true
	case cydcreator.ToolBuild:
		res, err := service.BuildForJob(ctx, jobID)
		if err != nil {
			return cydCreatorToolError(err), true
		}
		status := "failed"
		if res.Status == cydcreator.BuildSucceeded {
			status = "ok"
		}
		return cydCreatorToolJSON(map[string]any{"status": status, "build_status": res.Status, "build_id": res.BuildID,
			"error_code": res.ErrorCode, "diagnostics": res.Diagnostics, "usage": res.Usage, "next_action": res.NextAction}), true
	case cydcreator.ToolFinish:
		summary, _ := tc.Params["summary"].(string)
		if err := service.FinishJob(ctx, jobID, summary); err != nil {
			return cydCreatorToolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "next_action": "Done. Do not call more tools."}), true
	}
	return "", false
}
