package agent

import (
	"strings"
	"testing"

	"aurago/internal/memory"
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
		{name: "full capped", req: toolOutputViewRequest{View: "full", MaxChars: 5}, want: "alpha\n[TRUNCATED:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := renderToolOutputView(out, tc.req)
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

	got, _, err := renderToolOutputView(out, toolOutputViewRequest{
		View:  "jsonpath",
		Query: "$.items[1].name",
	})
	if err != nil {
		t.Fatalf("renderToolOutputView jsonpath: %v", err)
	}
	if got != `"beta"` {
		t.Fatalf("jsonpath view = %q, want %q", got, `"beta"`)
	}
}
