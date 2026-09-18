package agent

import (
	"aurago/internal/config"
	"context"
	"errors"
	openai "github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestExecutionHookCannotBeBypassedByInvokeTool(t *testing.T) {
	for _, item := range []struct{ schema, name, action string }{{"native_read", "native_read", "native_read"}, {"skill__reader", "reader", "execute_skill"}, {"tool__reader.py", "reader.py", "run_tool"}} {
		t.Run(item.action, func(t *testing.T) {
			resetToolCatalogForTest(t)
			session := "hook-" + item.action
			SetDiscoverToolsState(session, []openai.Tool{testToolSchema(item.schema, "fixture")}, nil, "")
			calls := 0
			dc := &DispatchContext{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionID: session, ExecutionHooks: &ExecutionHooks{BeforeTool: func(_ context.Context, tc ToolCall) error {
				if tc.Action != item.action {
					t.Fatalf("wrong routed action: %+v", tc)
				}
				calls++
				return errors.New("research scope denied")
			}}}
			out := dispatchInvokeTool(context.Background(), ToolCall{Params: map[string]any{"tool_name": item.name, "arguments": map[string]any{"operation": "read"}}}, dc)
			if calls != 1 || !strings.Contains(out, "research scope denied") {
				t.Fatalf("hook bypassed: calls=%d output=%s", calls, out)
			}
		})
	}
}
