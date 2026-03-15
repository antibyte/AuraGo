package agent

import (
	"log/slog"
	"os"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestNativeToolCallToToolCall_TruncatedJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name      string
		funcName  string
		args      string
		wantField string
		wantValue string
	}{
		{
			name:      "truncated generate_image recovers prompt",
			funcName:  "generate_image",
			args:      `{"prompt": "A dystopian cityscape with neon lights", "size": "1024x10`,
			wantField: "prompt",
			wantValue: "A dystopian cityscape with neon lights",
		},
		{
			name:      "truncated shell recovers command",
			funcName:  "shell",
			args:      `{"command": "ls -la /tmp", "background": tr`,
			wantField: "command",
			wantValue: "ls -la /tmp",
		},
		{
			name:      "truncated execute_skill recovers skill",
			funcName:  "execute_skill",
			args:      `{"skill": "weather_check", "skill_args": {"ci`,
			wantField: "skill",
			wantValue: "weather_check",
		},
		{
			name:      "truncated query_memory recovers query",
			funcName:  "query_memory",
			args:      `{"query": "user preferences for docker`,
			wantField: "query",
			wantValue: "user preferences for docker",
		},
		{
			name:      "truncated with escaped quotes in prompt",
			funcName:  "generate_image",
			args:      `{"prompt": "A sign saying \"hello world\"", "size": "10`,
			wantField: "prompt",
			wantValue: `A sign saying "hello world"`,
		},
		{
			name:      "valid JSON still works",
			funcName:  "generate_image",
			args:      `{"prompt": "A beautiful sunset", "size": "1024x1024"}`,
			wantField: "prompt",
			wantValue: "A beautiful sunset",
		},
		{
			name:      "empty arguments",
			funcName:  "generate_image",
			args:      "",
			wantField: "prompt",
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			native := openai.ToolCall{
				ID:   "call_test123",
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tt.funcName,
					Arguments: tt.args,
				},
			}
			tc := NativeToolCallToToolCall(native, logger)

			var got string
			switch tt.wantField {
			case "prompt":
				got = tc.Prompt
			case "command":
				got = tc.Command
			case "skill":
				got = tc.Skill
			case "query":
				got = tc.Query
			case "content":
				got = tc.Content
			case "operation":
				got = tc.Operation
			}

			if got != tt.wantValue {
				t.Errorf("field %q = %q, want %q", tt.wantField, got, tt.wantValue)
			}

			if tc.Action != tt.funcName {
				t.Errorf("Action = %q, want %q", tc.Action, tt.funcName)
			}
		})
	}
}
