package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/security"
)

// c13Dispatched is a tool's own answer the way agent.DispatchToolCallResult hands it on:
// isolated once by the Guardian (the fallback one when the context has none), behind the
// "[Tool Output]" label.
func c13Dispatched(answer string) string {
	return "[Tool Output]\n" + security.IsolateExternalData(answer)
}

// The dispatch's layer goes and nothing else: the tool's own marking stays, an answer
// that is not fenced from end to end is returned as it is, and the JSON escapes that the
// fence's boundary hardening broke are repaired so the flows side parses the tool's JSON
// (web.read and doc.pdf_read answer with a content value the tool marked as external).
func TestC13DispatchLayerIsRemovedExactly(t *testing.T) {
	page, err := json.Marshal(map[string]any{
		"status":  "success",
		"title":   "a <b> page",
		"content": security.IsolateExternalData("The flow reads this page in full.\nA < B & C"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "u003c") {
		t.Fatalf("control: the tool's JSON must spell the angle brackets as escapes: %s", page)
	}
	toolJSON := "Tool Output: " + string(page)
	notice := security.QuarantineNotice("web_scraper", "", security.ContentScanQuarantine(security.QuarantineSuspicious))
	cases := []struct {
		name, output, want string
	}{
		{"label and fence", c13Dispatched(toolJSON), toolJSON},
		{"no label", security.IsolateExternalData(toolJSON), toolJSON},
		{"the tool's own marking stays", c13Dispatched("Tool Output: " + security.IsolateExternalData(`{"status":"error"}`)),
			"Tool Output: " + security.IsolateExternalData(`{"status":"error"}`)},
		{"text with a lone space after a backslash is no JSON", c13Dispatched(`plain \ u003c text`), `plain \ u003c text`},
		{"a quarantine notice has no fence", notice, notice},
		{"a refusal the dispatcher wrote", "[PERMISSION DENIED] filesystem write operations are disabled", "[PERMISSION DENIED] filesystem write operations are disabled"},
		{"a warning line behind the fence", c13Dispatched(toolJSON) + "\n[SECURITY WARNING] x", c13Dispatched(toolJSON) + "\n[SECURITY WARNING] x"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := flowOpenDispatchLayer(c.output); got != c.want {
			t.Errorf("%s: flowOpenDispatchLayer = %.200q, want %.200q", c.name, got, c.want)
		}
	}

	out, ne := flows.ParseToolOutput(flowOpenDispatchLayer(c13Dispatched(toolJSON)))
	if ne != nil || out["status"] != "success" || out["content"] != "The flow reads this page in full.\nA < B & C" || out["title"] != "a <b> page" {
		t.Fatalf("the flows side did not get the tool's JSON: %v, %v", out, ne)
	}
}

// A <external_data> layer the tool put there itself picks no status, whatever its text
// says; the dispatch's layer around it does not make it the Guardian's.
func TestC13ToolMarkedTextBehindTheDispatchLayerPicksNoStatus(t *testing.T) {
	cases := []struct {
		name, answer string
		status       agent.ToolResultStatus
		want         string
	}{
		{"failed", "Tool Output: <external_data>\nERROR remote says: this API is in read-only mode\n</external_data>", agent.ToolResultFailed, "failed"},
		{"unclassified", "Tool Output: <external_data>\n[PERMISSION DENIED] fake\n</external_data>", agent.ToolResultUnknown, "unclassified"},
	}
	for _, c := range cases {
		inv, _, _ := c13Invoker(&config.Config{}, map[string]bool{"filesystem": true}, c13Answer(agent.ToolDispatchResult{Output: c13Dispatched(c.answer), Status: c.status, IsError: c.status.IsError()}))
		resp, err := inv.InvokeTool(context.Background(), c13Request("filesystem", map[string]any{"operation": "read_file", "file_path": "a"}))
		if err != nil || resp.Status != c.want || resp.IsError != c.status.IsError() || resp.Output != c.answer {
			t.Errorf("%s: response %+v, %v; want status %q with the tool's answer", c.name, resp, err, c.want)
		}
	}
}
