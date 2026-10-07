package agent

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestManageMissionsErrorRepliesAreValidJSONForHostileText(t *testing.T) {
	f := mm1c09New(t)
	hostile := "a\tb\"c\\d\ne\x00" + strings.Repeat("x", 5*1024)
	short := "id\t\"quoted\"\\back"

	// An id that names nothing: the update path echoes it.
	for _, id := range []string{short, hostile} {
		out := f.call(t, map[string]interface{}{"operation": "update", "id": id, "title": "Neu"})
		status, message := mm1c09Decode(t, out)
		if status != "error" || !strings.HasSuffix(message, " not found") || !strings.HasPrefix(message, "Mission ") {
			t.Fatalf("update of an unknown id: %q", out)
		}
		if !strings.HasPrefix(out, `Tool Output: {"status": "error", "message": "`) {
			t.Fatalf("the envelope spacing changed: %q", out)
		}
		if len(out) > 1200 {
			t.Fatalf("the reply echoes the id unbounded: %d bytes", len(out))
		}
		if id == short && !strings.Contains(message, short) {
			t.Fatalf("a short id is echoed whole: %q", message)
		}
		if id == hostile && !strings.Contains(message, "…") {
			t.Fatalf("a long id must be cut: %q", message)
		}
	}

	// An operation that does not exist is echoed too.
	for _, op := range []string{"say \"hi\"\\\t", "enable" + strings.Repeat("z", 5*1024)} {
		out := f.call(t, map[string]interface{}{"operation": op})
		status, message := mm1c09Decode(t, out)
		if status != "error" || !strings.HasPrefix(message, "Unknown operation: ") || len(out) > 1200 {
			t.Fatalf("unknown operation %.20q: %d bytes, %.200q", op, len(out), out)
		}
	}

	// Errors that the manager returns are marshalled, not formatted into a JSON string.
	out := f.call(t, map[string]interface{}{"operation": "run", "id": "missing\"id\\"})
	if status, message := mm1c09Decode(t, out); status != "error" || message == "" {
		t.Fatalf("run of an unknown id: %q", out)
	}
	if status, _ := mm1c09Decode(t, f.call(t, map[string]interface{}{"operation": "delete", "id": short})); status != "error" {
		t.Fatal("delete of an unknown id must be an error")
	}
}

func TestFlowMissionRefusalSaysWhatTheAgentMayDo(t *testing.T) {
	f := mm1c09New(t)
	out := f.call(t, map[string]interface{}{"operation": "delete", "id": f.flowID})
	mm1c09RequireRefused(t, "delete", out)
	_, message := mm1c09Decode(t, out)
	want := "Mission " + f.flowID + " is an EasyDrag flow. manage_missions can only list it, show its history or run it; change or delete it in the EasyDrag app."
	if message != want {
		t.Fatalf("message = %q, want %q", message, want)
	}
	// Creating is a different request with its own text.
	created := f.call(t, map[string]interface{}{"operation": "add", "title": "Neu", "command": "x", "execution_type": "flow"})
	mm1c09RequireRefused(t, "add", created)
	if _, createMessage := mm1c09Decode(t, created); createMessage == message || strings.Contains(createMessage, "Mission ") || !strings.Contains(createMessage, "cannot create") {
		t.Fatalf("the creation refusal must have its own text: %q", createMessage)
	}
}

func TestInvokeToolRoutesManageMissionsToTheFlowRefusal(t *testing.T) {
	resetToolCatalogForTest(t)
	f := mm1c09New(t)
	f.dc.SessionID = "mm1c09-invoke"
	SetDiscoverToolsState(f.dc.SessionID, builtinToolSchemas(buildToolFlagsFromConfig(f.dc.Cfg)), nil, "")
	before := f.flow(t)

	for label, params := range map[string]map[string]interface{}{
		"arguments object": {"tool_name": "manage_missions", "arguments": map[string]interface{}{"operation": "delete", "id": f.flowID}},
		"flattened update": {"tool_name": "manage_missions", "operation": "update", "id": f.flowID, "title": "X"},
		"params alias":     {"name": "manage_missions", "params": map[string]interface{}{"operation": "update", "id": f.flowID, "locked": true}},
		"enable":           {"tool_name": "manage_missions", "arguments": map[string]interface{}{"operation": "enable", "id": f.flowID}},
	} {
		out, handled := dispatchComm(context.Background(), ToolCall{Action: "invoke_tool", Params: params}, f.dc)
		if !handled {
			t.Fatalf("%s: invoke_tool was not handled", label)
		}
		mm1c09RequireRefused(t, label, out)
	}
	if after := f.flow(t); after.Name != before.Name || after.Enabled != before.Enabled || after.Locked != before.Locked {
		t.Fatalf("invoke_tool changed the flow mission: %+v", after)
	}

	// Reading through invoke_tool still works.
	out, handled := dispatchComm(context.Background(), ToolCall{Action: "invoke_tool", Params: map[string]interface{}{
		"tool_name": "manage_missions", "arguments": map[string]interface{}{"operation": "list"},
	}}, f.dc)
	if !handled || !strings.Contains(out, f.flowID) || strings.Contains(out, "flow_mission") {
		t.Fatalf("list through invoke_tool: handled=%v %q", handled, out)
	}
}

func TestManageMissionsLogsFlowRefusalsWithBoundedText(t *testing.T) {
	f := mm1c09New(t)
	var logs bytes.Buffer
	f.dc.Logger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))

	f.call(t, map[string]interface{}{"operation": "update", "id": f.flowID, "title": "Neu"})
	f.call(t, map[string]interface{}{"operation": "disable" + strings.Repeat("z", 5000), "id": f.flowID})
	f.call(t, map[string]interface{}{"operation": "add", "title": "Neu", "command": "x", "execution_type": "flow"})

	got := logs.String()
	if !strings.Contains(got, "manage_missions refused to change a flow mission") ||
		!strings.Contains(got, "op=update") || !strings.Contains(got, "id="+f.flowID) {
		t.Fatalf("the refusal of a change must be logged with its operation and id:\n%s", got)
	}
	if !strings.Contains(got, "manage_missions refused to create a flow mission") {
		t.Fatalf("the refusal of a create must be logged:\n%s", got)
	}
	if strings.Contains(got, strings.Repeat("z", missionEchoRunes+1)) {
		t.Fatalf("a model-supplied operation reached the log unbounded (%d bytes)", len(got))
	}
}
