package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"

	"github.com/sashabaranov/go-openai"
)

func TestGameMakerSourceStreamRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, dimension, first, second string
		stale                          bool
	}{
		{"eof_2d", "2d", "eof", "stop", false},
		{"eof_3d", "3d", "eof", "stop", false},
		{"deadline_3d", "3d", "deadline", "stop", false},
		{"repeated_eof", "3d", "eof", "eof", false},
		{"repeated_deadline", "3d", "deadline", "deadline", false},
		{"cancel", "3d", "cancel", "stop", false},
		{"length", "3d", "length", "stop", false},
		{"provider_error", "3d", "provider_error", "stop", false},
		{"stale_revision", "3d", "eof", "stop", true},
		{"format_2d", "2d", "format", "stop", false},
		{"format_3d", "3d", "format", "stop", false},
		{"repeated_format", "3d", "format", "format", false},
		{"format_after_eof", "3d", "eof", "format", false},
		{"eof_after_format", "3d", "format", "eof", false},
		{"deadline_after_format", "3d", "format", "deadline", false},
		{"cancel_after_format", "3d", "format", "cancel", false},
		{"format_stale_revision", "3d", "format", "stop", true},
		{"truncated_format", "3d", "format_length", "stop", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true})
			if err != nil {
				t.Fatal(err)
			}
			defer svc.Close()
			svc.SetSkillStatus(nil, true)
			var requests, active atomic.Int32
			var jobID string
			var cancelRequest context.CancelFunc
			var firstRequest openai.ChatCompletionRequest
			const partial = "export const discardedPartial = 1;"
			const complete = "export const recoveredGame = 2;"
			const concurrent = "export const concurrentEdit = 3;"
			const priorReasoning = "Retain the distinctive movement mechanic."
			const rejected = "I will write the game and update its helper.\n" +
				"<tool_call>game_maker_file<arg_key>operation</arg_key><arg_value>write</arg_value><arg_key>job_id</arg_key><arg_value>job_old</arg_value><arg_key>path</arg_key><arg_value>src/main.ts</arg_value><arg_key>content</arg_key><arg_value>export const rejectedSource = 1;</arg_value></tool_call>\n" +
				"<tool_call>game_maker_file<arg_key>operation</arg_key><arg_value>replace</arg_value><arg_key>path</arg_key><arg_value>src/common.ts</arg_value><arg_key>old_text</arg_key><arg_value>placeholder</arg_value><arg_key>new_text</arg_key><arg_value>unsafe</arg_value><arg_key>expected_sha256</arg_key><arg_value>old-revision</arg_value></tool_call>"
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				active.Add(1)
				defer active.Add(-1)
				n := requests.Add(1)
				var body openai.ChatCompletionRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				if !body.Stream || len(body.Tools) != 0 || n > 2 {
					t.Error("unbounded retry or unexpected tool-enabled request")
				}
				if n == 1 {
					firstRequest = body
					last := body.Messages[len(body.Messages)-1].Content
					if !strings.Contains(last, "Source-generation phase:") || !strings.HasSuffix(last, "Return only the complete TypeScript source for src/main.ts.") {
						t.Error("missing explicit transition from tool use to source generation")
					}
				}
				if n == 2 {
					if len(body.Messages) <= len(firstRequest.Messages) || !reflect.DeepEqual(body.Messages[:len(firstRequest.Messages)], firstRequest.Messages) {
						t.Error("correction rewrote the cache prefix or previous context")
					}
					dataCount, retainedReasoning, retainedPrior := 0, false, false
					retainedCalls, retainedResults := 0, 0
					for _, message := range body.Messages {
						if strings.Contains(message.Content, `"previous_user_requests"`) {
							dataCount++
						}
						retainedReasoning = retainedReasoning || message.ReasoningContent == "private interrupted game reasoning"
						retainedPrior = retainedPrior || message.ReasoningContent == priorReasoning
						retainedCalls += len(message.ToolCalls)
						if message.Role == "tool" && message.ToolCallID == "historical-read" {
							retainedResults++
						}
						if strings.Contains(message.Content, partial) {
							t.Error("retry received unsafe partial source")
						}
					}
					if dataCount != 1 || !retainedReasoning || !retainedPrior || retainedCalls != 1 || retainedResults != 1 {
						t.Error("retry lost context/reasoning or duplicated the source snapshot")
					}
					correction := body.Messages[len(body.Messages)-1].Content
					if tc.first == "format" {
						if !strings.Contains(correction, "None of those calls were executed") || !strings.Contains(correction, "complete TypeScript source for src/main.ts") {
							t.Error("missing bounded code-only format correction")
						}
					} else if !strings.Contains(correction, "from the beginning") {
						t.Error("missing stream restart guidance")
					}
					if tc.stale {
						if _, err := svc.WriteJobFileChecked(req.Context(), jobID, "src/main.ts", concurrent, ""); err != nil {
							t.Error(err)
						}
					}
				}
				mode := tc.first
				if n > 1 {
					mode = tc.second
				}
				w.Header().Set("Content-Type", "text/event-stream")
				send := func(content, reasoning, finish string) {
					chunk, _ := json.Marshal(openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Delta: openai.ChatCompletionStreamChoiceDelta{Content: content, ReasoningContent: reasoning}, FinishReason: openai.FinishReason(finish)}}})
					fmt.Fprintf(w, "data: %s\n\n", chunk)
					w.(http.Flusher).Flush()
				}
				if mode == "stop" {
					send(complete, "", "stop")
					fmt.Fprint(w, "data: [DONE]\n\n")
					return
				}
				if mode == "format" || mode == "format_length" {
					finish := "stop"
					if mode == "format_length" {
						finish = "length"
					}
					send(rejected, "private interrupted game reasoning", finish)
					fmt.Fprint(w, "data: [DONE]\n\n")
					return
				}
				send(partial, "private interrupted game reasoning", "")
				switch mode {
				case "deadline", "cancel":
					if mode == "cancel" {
						cancelRequest()
					}
					// Continuous activity must not bypass the configured total deadline.
					ticker := time.NewTicker(25 * time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-req.Context().Done():
							return
						case <-ticker.C:
							send("", "", "")
						}
					}
				case "length":
					send("", "", "length")
				case "provider_error":
					fmt.Fprint(w, "data: {\"error\":{\"message\":\"provider rejected request\",\"type\":\"invalid_request_error\"}}\n\n")
				}
				// Plain EOF deliberately lacks both finish_reason and [DONE].
			}))
			defer provider.Close()
			cc := openai.DefaultConfig("test-only")
			cc.BaseURL = provider.URL
			cfg := &config.Config{}
			cfg.LLM.Model, cfg.LLM.ProviderType = "test-stream", "openai"
			cfg.Agent.ContextWindow, cfg.CircuitBreaker.LLMTimeoutSeconds = 65536, 1
			srv := &Server{Cfg: cfg, GameMaker: svc, LLMClient: openai.NewClientWithConfig(cc), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			runner := &gameMakerAgentRunner{server: srv, service: svc}
			svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
				if run.Stage == "planning" {
					base := "flight"
					if tc.dimension == "2d" {
						base = "platformer"
					}
					return svc.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Requested game","features":["custom movement"]}`, base)))
				}
				jobID = run.Job.ID
				before, _ := svc.ReadJobFile(ctx, jobID, "src/main.ts")
				commonBefore, _ := svc.ReadJobFile(ctx, jobID, "src/common.ts")
				prior, _ := json.Marshal([]openai.ChatCompletionMessage{
					{Role: "user", Content: "Keep the original unusual game mechanic."},
					{Role: "assistant", ReasoningContent: priorReasoning, ToolCalls: []openai.ToolCall{{ID: "historical-read", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "game_maker_file", Arguments: `{"operation":"read","path":"src/common.ts"}`}}}},
					{Role: "tool", ToolCallID: "historical-read", Content: `{"content":"historical helper snapshot"}`},
				})
				if err := svc.SaveAgentConversation(ctx, jobID, cfg.LLM.Provider, cfg.LLM.Model, prior); err != nil {
					return err
				}
				callCtx, cancel := context.WithCancel(ctx)
				defer cancel()
				cancelRequest = cancel
				started := time.Now()
				err := runner.implementGameStarter(callCtx, cfg, srv.LLMClient, run)
				if time.Since(started) > 5*time.Second {
					t.Error("configured per-attempt deadline was not enforced")
				}
				wantRequests := int32(1)
				if tc.first == "eof" || tc.first == "deadline" || tc.first == "format" {
					wantRequests = 2
				}
				if requests.Load() != wantRequests {
					t.Errorf("requests=%d, want %d", requests.Load(), wantRequests)
				}
				after, _ := svc.ReadJobFile(ctx, jobID, "src/main.ts")
				commonAfter, _ := svc.ReadJobFile(ctx, jobID, "src/common.ts")
				if commonAfter != commonBefore || strings.Contains(after, "rejectedSource") {
					t.Error("rejected tool text was applied to project files")
				}
				want := before
				valid := wantRequests == 2 && tc.second == "stop" && !tc.stale
				if valid {
					want = complete
				} else if tc.stale {
					want = concurrent
				}
				matches := after == want
				if valid || tc.stale {
					// BuildJob prepends the normal diagnostics bridge to main.ts.
					matches = strings.HasSuffix(strings.TrimSpace(after), want) && !strings.Contains(after, partial)
				}
				if !matches || valid != (err == nil) {
					t.Errorf("invalid write or recovery result: valid=%v, source matches=%v, err=%v", valid, matches, err)
				}
				if (tc.first == "cancel" || tc.second == "cancel") && !errors.Is(err, context.Canceled) || tc.second == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("lost cancellation/deadline: %v", err)
				}
				if valid {
					build := svc.BuildJob(ctx, jobID)
					if !build.OK || build.RuntimeStatus != "unverified" {
						t.Error("normal compiler/browser validation was bypassed")
					}
				}
				for until := time.Now().Add(time.Second); active.Load() != 0 && time.Now().Before(until); {
					time.Sleep(time.Millisecond)
				}
				if active.Load() != 0 {
					t.Error("provider stream was not closed")
				}
				// Exercise production recovery and compilation without claiming gameplay acceptance.
				return errors.New("stream recovery verified")
			}))
			project, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Recovery", Description: "Implement the requested game", Dimension: tc.dimension})
			if err != nil {
				t.Fatal(err)
			}
			job, err := svc.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Implement the requested game"})
			if err != nil {
				t.Fatal(err)
			}
			for until := time.Now().Add(15 * time.Second); ; {
				done, err := svc.GetJob(context.Background(), job.ID)
				if err == nil && done.Status == "failed" {
					if done.Error != "stream recovery verified" || done.ResultRevision != 0 {
						t.Fatalf("unexpected result: %+v", done)
					}
					break
				}
				if time.Now().After(until) {
					t.Fatal("test job did not complete")
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}
