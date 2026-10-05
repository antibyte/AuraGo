package server

import (
	"aurago/internal/llm"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"

	"github.com/sashabaranov/go-openai"
)

func TestGameMakerDirectGenerationPreservesValidationAndRepairBudget(t *testing.T) {
	for _, tc := range []struct{ dimension, base, output string }{
		{"2d", "platformer", "broken"}, {"3d", "fps", "broken"},
		{"2d", "blocks", "unchanged"},
	} {
		t.Run(tc.base+"/"+tc.output, func(t *testing.T) {
			root := t.TempDir()
			svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
			if err != nil {
				t.Fatal(err)
			}
			defer svc.Close()
			svc.SetSkillStatus(nil, true)
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var body openai.ChatCompletionRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				calls.Add(1)
				if len(body.Tools) != 0 || !body.Stream {
					t.Error("initial implementation must use the streaming source-only route")
				}
				current := body.Messages[len(body.Messages)-1].Content
				if !strings.Contains(current, `"validation_plan"`) || !strings.Contains(body.Messages[0].Content, "Delivering the design") {
					t.Error("direct generation lost gameplay checks or design obligations")
				}
				code := "const broken = ;"
				if tc.output == "unchanged" {
					_, data, _ := strings.Cut(current, "<external_data>\n")
					data, _, _ = strings.Cut(data, "\n</external_data>")
					var packet struct {
						Files map[string]string `json:"files"`
					}
					if err := json.Unmarshal([]byte(data), &packet); err != nil {
						t.Error(err)
					}
					code = packet.Files["src/main.ts"]
				}
				chunk, _ := json.Marshal(openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Delta: openai.ChatCompletionStreamChoiceDelta{Content: code}, FinishReason: openai.FinishReasonStop}}})
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
			}))
			defer provider.Close()
			cfg := &config.Config{}
			cfg.LLM.Model, cfg.LLM.ProviderType = "test-model", "openai"
			cfg.Agent.ContextWindow = 65536
			cc := openai.DefaultConfig("test-only")
			cc.BaseURL = provider.URL
			srv := &Server{Cfg: cfg, GameMaker: svc, LLMClient: llm.WrapOpenAIClient(openai.NewClientWithConfig(cc)), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			runner := &gameMakerAgentRunner{server: srv, service: svc}
			svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
				switch run.Stage {
				case "planning":
					return svc.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Distinct game","features":["custom scoring"]}`, tc.base)))
				case "building":
					err := runner.RunGameMakerJob(ctx, run)
					if svc.RemainingRepairs(run.Job.ID) != 3 || calls.Load() != 1 {
						t.Error("initial generation consumed repairs or performed discovery requests")
					}
					return err
				case "repair":
					if len(run.Diagnostics) == 0 || tc.output == "unchanged" && run.Diagnostics[0].Level != "implementation" || tc.output == "broken" && !strings.Contains(run.Diagnostics[0].Message, "Unexpected") {
						return fmt.Errorf("direct generation bypassed validation: %+v", run.Diagnostics)
					}
					return errors.New("verified direct generation and validation")
				}
				return errors.New("unexpected phase")
			}))
			p, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Direct generation", Dimension: tc.dimension, Description: "Build a distinct game"})
			if err != nil {
				t.Fatal(err)
			}
			job, err := svc.StartJob(context.Background(), p.ID, gamemaker.StartJobRequest{Prompt: "Implement custom scoring"})
			if err != nil {
				t.Fatal(err)
			}
			for deadline := time.Now().Add(15 * time.Second); ; {
				done, err := svc.GetJob(context.Background(), job.ID)
				if err == nil && done.Status == "failed" {
					if done.Error != "verified direct generation and validation" || done.ResultRevision != 0 || calls.Load() != 1 {
						t.Fatalf("unexpected completion: %+v; calls=%d", done, calls.Load())
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("direct generation did not finish")
				}
				time.Sleep(5 * time.Millisecond)
			}
		})
	}
}
