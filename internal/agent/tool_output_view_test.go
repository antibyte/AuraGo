package agent

import (
	"strings"
	"testing"

	"aurago/internal/memory"
	"aurago/internal/security"
)

func TestRenderToolOutputViewLineModes(t *testing.T) {
	out := &memory.CompressedToolOutput{
		OutputRef:         "toolout_lines",
		OriginalContent:   strings.Join([]string{"alpha", "beta", "gamma", "delta", "epsilon"}, "\n"),
		CompressedContent: "compact",
		SummaryContent:    "summary",
	}

	for _, tc := range []struct {
		name string
		req  toolOutputViewRequest
		want string
	}{
		{name: "summary", req: toolOutputViewRequest{View: "summary"}, want: "summary"},
		{name: "head", req: toolOutputViewRequest{View: "head", MaxLines: 2}, want: "alpha\nbeta"},
		{name: "tail", req: toolOutputViewRequest{View: "tail", MaxLines: 2}, want: "delta\nepsilon"},
		{name: "range", req: toolOutputViewRequest{View: "range", StartLine: 2, EndLine: 4}, want: "beta\ngamma\ndelta"},
		{name: "grep", req: toolOutputViewRequest{View: "grep", Query: "mm"}, want: "gamma"},
		{name: "full", req: toolOutputViewRequest{View: "full"}, want: "alpha\nbeta\ngamma\ndelta\nepsilon"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := vaultReadablePayload(out.OriginalContent)
			got, err := renderToolOutputView(out, payload, tc.req)
			if err != nil {
				t.Fatalf("renderToolOutputView: %v", err)
			}
			if !strings.HasPrefix(got, tc.want) {
				t.Fatalf("view = %q, want prefix %q", got, tc.want)
			}
		})
	}
}

func TestRenderToolOutputViewJSONPath(t *testing.T) {
	out := &memory.CompressedToolOutput{
		OutputRef:       "toolout_json",
		OriginalContent: `{"status":"ok","items":[{"name":"alpha"},{"name":"beta"}]}`,
	}

	payload, _ := vaultReadablePayload(out.OriginalContent)
	got, err := renderToolOutputView(out, payload, toolOutputViewRequest{
		View:  "jsonpath",
		Query: "$.items[1].name",
	})
	if err != nil {
		t.Fatalf("renderToolOutputView jsonpath: %v", err)
	}
	if got != `"beta"` {
		t.Fatalf("jsonpath view = %q, want %q", got, `"beta"`)
	}

	// Archived dispatch output carries the presentation prefix and the
	// escaped isolation; jsonpath reads the tool's JSON inside them.
	out.OriginalContent = security.IsolateExternalData("Tool Output: " + `{"status":"ok","items":[{"name":"<alpha>"}]}`)
	payload, isolated := vaultReadablePayload(out.OriginalContent)
	got, err = renderToolOutputView(out, payload, toolOutputViewRequest{View: "jsonpath", Query: "$.items[0].name"})
	// JSON re-encodes the angle brackets as unicode escapes.
	if err != nil || got != "\"\x5cu003calpha\x5cu003e\"" || !isolated {
		t.Fatalf("jsonpath on isolated output = %q (%v, isolated=%v)", got, err, isolated)
	}
}
