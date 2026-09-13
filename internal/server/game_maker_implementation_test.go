package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"

	"github.com/sashabaranov/go-openai"
)

type implementationTestRunner func(context.Context, gamemaker.JobRun) error

func (f implementationTestRunner) RunGameMakerJob(ctx context.Context, run gamemaker.JobRun) error {
	return f(ctx, run)
}

func TestGameMakerUnchangedStarterGetsBoundedCodeRecovery(t *testing.T) {
	for _, scenario := range []struct{ dimension, base, finish, code string }{
		{"2d", "platformer", "stop", "export const requestedGame = 7;"},
		{"3d", "fps", "stop", "```typescript\nexport const requestedGame = 7;\n```"},
		{"3d", "three", "stop", "export const requestedGame = 7;"},
		{"2d", "platformer", "length", "export const partial ="},
		{"2d", "platformer", "stop", ""},
		{"2d", "platformer", "stop", "```typescript\npartial"},
		{"2d", "platformer", "", "export const requestedGame = 7;"},
		{"2d", "platformer", "tool_calls", "export const requestedGame = 7;"},
		{"2d", "platformer", "stream_error", "export const requestedGame = 7;"},
		{"2d", "platformer", "timeout", "export const requestedGame = 7;"},
	} {
		t.Run(scenario.dimension+"/"+scenario.finish+fmt.Sprint(len(scenario.code)), func(t *testing.T) {
			root := t.TempDir()
			svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true})
			if err != nil {
				t.Fatal(err)
			}
			defer svc.Close()
			svc.SetSkillStatus(nil, true)
			project, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Source recovery", Description: "Implement the requested rules", Dimension: scenario.dimension})
			if err != nil {
				t.Fatal(err)
			}
			requests := 0
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var body openai.ChatCompletionRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				requests++
				if len(body.Tools) != 0 {
					t.Error("recovery exposed discovery tools")
				}
				if !body.Stream {
					t.Error("source generation must stream instead of waiting for the complete file in response headers")
					<-req.Context().Done()
					return
				}
				if len(body.Messages) != 2 || scenario.base != "three" && !strings.Contains(body.Messages[1].Content, "src/common.ts") || !strings.Contains(body.Messages[1].Content, "src/main.ts") {
					t.Error("missing actual source context")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				w.(http.Flusher).Flush()
				// Full output takes longer than the header timeout; streaming must survive it.
				time.Sleep(120 * time.Millisecond)
				for _, content := range []string{scenario.code[:len(scenario.code)/2], scenario.code[len(scenario.code)/2:]} {
					chunk, _ := json.Marshal(openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Delta: openai.ChatCompletionStreamChoiceDelta{Content: content}}}})
					fmt.Fprintf(w, "data: %s\n\n", chunk)
					w.(http.Flusher).Flush()
				}
				if scenario.finish == "stream_error" {
					fmt.Fprint(w, "data: {\"error\":{\"message\":\"interrupted stream\",\"type\":\"server_error\"}}\n\n")
					return
				}
				choice := openai.ChatCompletionStreamChoice{FinishReason: openai.FinishReason(scenario.finish)}
				if scenario.finish == "tool_calls" {
					choice.Delta.ToolCalls = []openai.ToolCall{{ID: "forbidden", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "game_maker_file", Arguments: `{}`}}}
				}
				chunk, _ := json.Marshal(openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{choice}, Usage: &openai.Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120}})
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
			}))
			defer provider.Close()
			clientCfg := openai.DefaultConfig("test-only")
			clientCfg.BaseURL = provider.URL
			clientCfg.HTTPClient = &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 50 * time.Millisecond}, Timeout: 3 * time.Second}
			if scenario.finish == "timeout" {
				clientCfg.HTTPClient = &http.Client{Timeout: 70 * time.Millisecond}
			}
			cfg := &config.Config{}
			cfg.LLM.Model, cfg.LLM.ProviderType = "test-model", "openai"
			cfg.Agent.ContextWindow = 65536
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			srv := &Server{Cfg: cfg, GameMaker: svc, LLMClient: openai.NewClientWithConfig(clientCfg), Logger: logger}
			runner := &gameMakerAgentRunner{server: srv, service: svc}
			svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
				if run.Stage == "planning" {
					return svc.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Requested game","features":["custom rules"]}`, scenario.base)))
				}
				before, _ := svc.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
				common, _ := svc.ReadJobFile(ctx, run.Job.ID, "src/common.ts")
				run.Stage = "repair"
				run.Diagnostics = []gamemaker.Diagnostic{{Level: "implementation", File: "src/main.ts", Message: "unchanged starter"}}
				err := runner.RunGameMakerJob(ctx, run)
				after, _ := svc.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
				commonAfter, _ := svc.ReadJobFile(ctx, run.Job.ID, "src/common.ts")
				valid := scenario.finish == "stop" && strings.Contains(scenario.code, "requestedGame")
				if valid && (err != nil || !strings.Contains(after, "export const requestedGame = 7;") || strings.Contains(after, "```")) {
					t.Errorf("code not written: %v", err)
				}
				if !valid && (err == nil || after != before) {
					t.Errorf("incomplete response changed the starter: %v", err)
				}
				if common != commonAfter || requests != 1 {
					t.Error("recovery rewrote lifecycle or repeated requests")
				}
				// This fixture checks the write path, not gameplay acceptance.
				return fmt.Errorf("verified code recovery")
			}))
			job, err := svc.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Implement a game"})
			if err != nil {
				t.Fatal(err)
			}
			for deadline := time.Now().Add(15 * time.Second); ; {
				done, err := svc.GetJob(context.Background(), job.ID)
				if err == nil && done.Status == "failed" {
					if done.Error != "verified code recovery" || done.ResultRevision != 0 {
						t.Fatalf("unexpected completion: %+v", done)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("recovery did not finish")
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}
