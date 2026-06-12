package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestDispatchReadToolOutputViewsAndLegacyAlias(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	ctx := context.Background()
	if err := stm.StoreCompressedOutput(ctx, &memory.CompressedToolOutput{
		SessionID:         "sess-output",
		ToolCallID:        "call-output",
		OutputRef:         "toolout_test_ref",
		ToolName:          "execute_shell",
		OriginalContent:   "alpha\nbeta\ngamma\n",
		CompressedContent: "compact",
		SummaryContent:    "summary",
		CompressionRatio:  0.25,
		FilterUsed:        "vault",
	}); err != nil {
		t.Fatalf("StoreCompressedOutput: %v", err)
	}
	dc := &DispatchContext{
		Cfg:          &config.Config{},
		Logger:       logger,
		ShortTermMem: stm,
		SessionID:    "sess-output",
	}

	out, handled := dispatchExec(ctx, ToolCall{
		Action: "read_tool_output",
		Params: map[string]interface{}{
			"ref":       "toolout_test_ref",
			"view":      "tail",
			"max_lines": float64(2),
			"max_chars": float64(1000),
		},
	}, dc)
	if !handled {
		t.Fatal("read_tool_output was not handled")
	}
	if !strings.Contains(out, `"output_ref":"toolout_test_ref"`) || !strings.Contains(out, "beta\\ngamma") {
		t.Fatalf("unexpected read_tool_output response: %s", out)
	}

	legacy, handled := dispatchExec(ctx, ToolCall{
		Action: "retrieve_original_output",
		Params: map[string]interface{}{
			"tool_call_id": "call-output",
		},
	}, dc)
	if !handled {
		t.Fatal("retrieve_original_output was not handled")
	}
	if !strings.Contains(legacy, "alpha\nbeta\ngamma") {
		t.Fatalf("legacy alias did not return full output: %s", legacy)
	}
}
