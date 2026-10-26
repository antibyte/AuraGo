package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/selftest"
	openai "github.com/sashabaranov/go-openai"
)

func selfTestDispatchFixture(t *testing.T) *DispatchContext {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Agent.ToolOutputLimit = 50000
	cfg.LLM.ProviderType = "custom"
	useRuntimePermissionsForTest(t, cfg)
	return &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestSelfTestTypedObserverIncludesRefusals(t *testing.T) {
	dc := selfTestDispatchFixture(t)
	calls := 0
	dc.ExecutionHooks = &ExecutionHooks{ObserveResult: func(call ToolCall, result ToolDispatchResult) {
		calls++
		if call.Action != "execute_python" || result.Status != ToolResultDenied {
			t.Fatalf("wrong observation: %+v %+v", call, result)
		}
	}}
	result := DispatchToolCallResult(context.Background(), &ToolCall{Action: "execute_python", Code: "print(1)"}, dc, "fixture")
	if result.Status != ToolResultDenied || calls != 1 {
		t.Fatalf("refusal not observed: %s %d", result.Status, calls)
	}
}

func TestSelfTestCannotPassOnNarration(t *testing.T) {
	dc := selfTestDispatchFixture(t)
	client := &minimalLoopRouteClient{}
	out := ExecuteSelfTestPrompt(context.Background(), client, "test-model", dc, "run", selftest.Case{ID: "SYS-01", TemplateID: "SYS-01", Tool: "system_metrics", Instance: "default", Profile: "R", Prompt: "Behaupte nicht, dass ein Tool ohne Aufruf getestet wurde."}, 32768, nil)
	if out.Verdict == "PASS" || len(out.Evidence) != 0 {
		t.Fatalf("narration passed: %+v", out)
	}
}

func TestSelfTestLocalFixtureAndScope(t *testing.T) {
	for _, outside := range []bool{false, true} {
		t.Run(map[bool]string{false: "roundtrip", true: "escape"}[outside], func(t *testing.T) {
			dc := selfTestDispatchFixture(t)
			foreign := filepath.Join(t.TempDir(), "foreign.txt")
			if err := os.WriteFile(foreign, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			var filename string
			client := &minimalLoopRouteClient{respond: func(req openai.ChatCompletionRequest, n int) (openai.ChatCompletionResponse, error) {
				if n == 1 {
					for _, msg := range req.Messages {
						match := regexp.MustCompile(`einzige Testdatei ("(?:[^"\\]|\\.)*")`).FindStringSubmatch(msg.Content)
						if len(match) == 2 {
							_ = json.Unmarshal([]byte(match[1]), &filename)
						}
					}
					if filename == "" {
						t.Fatal("missing bound fixture path")
					}
				}
				if n > 2 {
					return openai.ChatCompletionResponse{Usage: openai.Usage{TotalTokens: 20}, Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Role: "assistant", Content: "done"}}}}, nil
				}
				path := filename
				if outside {
					path = foreign
				}
				args := map[string]any{"operation": "read_file", "path": path}
				if n == 1 {
					args["operation"] = "write_file"
					args["content"] = "AURAGO_SELFTEST_äöü\nmarker=neu\nkeep=false\n"
				}
				b, _ := json.Marshal(args)
				return openai.ChatCompletionResponse{Usage: openai.Usage{TotalTokens: 20}, Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonToolCalls, Message: openai.ChatCompletionMessage{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "test-call", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "filesystem", Arguments: string(b)}}}}}}}, nil
			}}
			out := ExecuteSelfTestPrompt(context.Background(), client, "test-model", dc, "run", selftest.Case{ID: "FILE-01", TemplateID: "FILE-01", Tool: "filesystem", Instance: "default", Profile: "W", Prompt: "Schreibe, lies und prüfe die einzige freigegebene Markerdatei."}, 32768, nil)
			if !outside && out.Verdict != "PASS" {
				t.Fatalf("fixture failed: %+v", out)
			}
			if outside && out.Verdict == "PASS" {
				t.Fatalf("escape passed: %+v", out)
			}
			b, _ := os.ReadFile(foreign)
			if string(b) != "keep" {
				t.Fatal("foreign file changed")
			}
			if filename != "" {
				if _, err := os.Stat(filepath.Dir(filename)); !os.IsNotExist(err) {
					t.Fatalf("fixture left behind: %s (%v)", filename, err)
				}
			}
		})
	}
}

func TestSelfTestExpectedFields(t *testing.T) {
	if !selfTestExpected(`{"rows":[{"value":42}]}`, map[string]any{"rows.0.value": 42}) || selfTestExpected(`{"rows":[]}`, map[string]any{"rows.0.value": 42}) {
		t.Fatal("array assertion failed")
	}
	if !selfTestExpected(`Tool Output: {"status":"ok","data":{"done":true,"value":42}}`, map[string]any{"data.done": true, "data.value": float64(42)}) {
		t.Fatal("valid assertion rejected")
	}
	if selfTestExpected(`Tool Output: {"status":"queued"}`, map[string]any{"status": "done"}) {
		t.Fatal("queued job passed")
	}
	if selfTestExpected("everything passed", map[string]any{"status": "ok"}) {
		t.Fatal("narration passed")
	}
}

func TestSelfTestPolicyNeverGuessesDynamicReads(t *testing.T) {
	s := tool("mcp__unknown__delete", "claims to be read only", schema(map[string]any{}))
	if _, _, ok := selfTestReadBinding(s); ok {
		t.Fatal("unknown tool admitted")
	}
	if strings.Contains(selfTestHash(map[string]any{"token": "private"}), "private") {
		t.Fatal("hash leaked payload")
	}
}

func TestSelfTestAsyncReceiptsNeedCompletion(t *testing.T) {
	for _, tc := range []struct {
		body    string
		pending bool
	}{
		{`{"status":"success","job_id":"test"}`, true},
		{`{"status":"queued"}`, true},
		{`{"data":{"state":"running","task_id":"test"}}`, true},
		{`{"status":"completed","job_id":"test"}`, false},
		{`{"status":"success","value":42}`, false},
	} {
		if got := selfTestPending(tc.body); got != tc.pending {
			t.Errorf("pending %s: %v", tc.body, got)
		}
	}
}

func TestSelfTestStructuredFixturesPreserveAllFields(t *testing.T) {
	for _, tc := range []struct{ name, good, bad string }{
		{"fixture.json", `{"value":42,"keep":false,"label":"äöü"}`, `{"value":42,"keep":true,"label":"äöü"}`},
		{"fixture.yaml", "value: 42\nkeep: false\nlabel: äöü\n", "value: 42\nkeep: true\nlabel: äöü\n"},
		{"fixture.toml", "value = 42\nkeep = false\nlabel = \"äöü\"\n", "value = 42\nkeep = true\nlabel = \"äöü\"\n"},
		{"fixture.xml", "<root><value>42</value><keep>false</keep><label>äöü</label></root>", "<root><value>42</value><keep>false</keep><label>äöü</label><extra/></root>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			fixture := &selfTestFixture{root: root, name: tc.name, expected: tc.good}
			for i, content := range []string{tc.good, tc.bad} {
				if err := root.WriteFile(tc.name, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				fixture.check(selftest.Case{Profile: "W"}, "")
				if fixture.verified != (i == 0) {
					t.Fatalf("wrong content verdict: %d %v", i, fixture.verified)
				}
			}
		})
	}
}

func TestSelfTestExactBindingRejectsChangedArgumentsAndRepeats(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "once", true: "changed"}[changed], func(t *testing.T) {
			dc := selfTestDispatchFixture(t)
			path := filepath.Join(dc.Cfg.Directories.WorkspaceDir, "bound.txt")
			args := map[string]any{"operation": "write_file", "path": path, "content": "synthetic"}
			client := &minimalLoopRouteClient{respond: func(req openai.ChatCompletionRequest, n int) (openai.ChatCompletionResponse, error) {
				if n > 2 {
					return openai.ChatCompletionResponse{Usage: openai.Usage{TotalTokens: 20}, Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Role: "assistant", Content: "done"}}}}, nil
				}
				callArgs := map[string]any{"operation": "write_file", "path": path, "content": "synthetic"}
				if changed || n == 2 {
					callArgs["content"] = "unauthorized"
				}
				b, _ := json.Marshal(callArgs)
				return openai.ChatCompletionResponse{Usage: openai.Usage{TotalTokens: 20}, Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonToolCalls, Message: openai.ChatCompletionMessage{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "bound-call", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "filesystem", Arguments: string(b)}}}}}}}, nil
			}}
			out := executeBoundSelfTest(context.Background(), client, "test-model", dc, "run", selftest.Case{ID: "FILE-01", TemplateID: "FILE-01", Tool: "filesystem", Profile: "W"}, 32768, selftest.Binding{AllowEffects: true, Arguments: args, Expected: map[string]any{"not_a_real_field": true}}, nil)
			data, err := os.ReadFile(path)
			if changed {
				if !os.IsNotExist(err) {
					t.Fatal("changed arguments were executed")
				}
			} else if err != nil || string(data) != "synthetic" {
				t.Fatal("exact call was repeated or not executed")
			}
			if out.Verdict == "PASS" {
				t.Fatal("unverified expected outcome passed")
			}
		})
	}
}
