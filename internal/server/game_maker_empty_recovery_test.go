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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/memory"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestGameMakerReasoningOnlyBuildReachesBoundedImplementationRecovery(t *testing.T) {
	root := t.TempDir()
	svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.SetSkillStatus(nil, true)
	var toolRequests, codeRequests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if len(req.Tools) > 0 {
			n := toolRequests.Add(1)
			if n == 2 {
				last := req.Messages[len(req.Messages)-1]
				if !strings.Contains(last.Content, "next small concrete step") {
					t.Error("retry did not request a concrete next step")
				}
				found := false
				for _, msg := range req.Messages {
					found = found || msg.ReasoningContent == "private runner reasoning"
				}
				if !found {
					t.Error("retry lost provider reasoning")
				}
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"private runner reasoning\"},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		codeRequests.Add(1)
		// Deliberately broken source: recovery must still pass through validation.
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"const broken = ;\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer provider.Close()
	cfg := &config.Config{}
	cfg.LLM.Model, cfg.LLM.ProviderType = "test-runner", "openai"
	cfg.Agent.ContextWindow, cfg.CircuitBreaker.LLMTimeoutSeconds = 65536, 10
	cfg.GameMaker.Enabled = true
	cfg.Directories.ToolsDir, cfg.Directories.WorkspaceDir = filepath.Join(root, "tools"), root
	cc := openai.DefaultConfig("test-only")
	cc.BaseURL = provider.URL
	s := &Server{Cfg: cfg, LLMClient: openai.NewClientWithConfig(cc), GameMaker: svc, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), HistoryManager: memory.NewEphemeralHistoryManager()}
	s.Registry = tools.NewProcessRegistry(s.Logger)
	s.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ShortTermMem.Close()
	runner := &gameMakerAgentRunner{server: s, service: svc}
	svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage == "planning" {
			return svc.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"three","objective":"Endless runner","features":["lane changes and obstacles"]}`))
		}
		if run.Stage == "repair" && codeRequests.Load() > 0 {
			if len(run.Diagnostics) == 0 || !strings.Contains(run.Diagnostics[0].Message, "Unexpected") {
				return fmt.Errorf("recovered code bypassed compiler: %+v", run.Diagnostics)
			}
			return errors.New("verified bounded recovery and compiler rejection")
		}
		return runner.RunGameMakerJob(ctx, run)
	}))
	p, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Runner", Description: "3D endless runner", Dimension: "3d"})
	if err != nil {
		t.Fatal(err)
	}
	j, err := svc.StartJob(context.Background(), p.ID, gamemaker.StartJobRequest{Prompt: "Implement the runner"})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); ; {
		done, err := svc.GetJob(context.Background(), j.ID)
		if err == nil && done.Status == "failed" {
			if done.Error != "verified bounded recovery and compiler rejection" || done.ResultRevision != 0 || toolRequests.Load() != 2 || codeRequests.Load() != 1 {
				t.Fatalf("unexpected completion: %+v, tool requests=%d, code requests=%d", done, toolRequests.Load(), codeRequests.Load())
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("recovery did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
