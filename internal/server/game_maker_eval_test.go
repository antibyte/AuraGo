package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"
	"gopkg.in/yaml.v3"
)

// Explicit opt-in: runs the real isolated agent against selected configured
// providers on their host. Credentials never enter reports or the browser.
// Serve the loopback parent through an SSH tunnel and keep it open in Chrome.
func TestGameMakerLiveEvaluation(t *testing.T) {
	configPath := os.Getenv("GAMEMAKER_EVAL_CONFIG")
	if configPath == "" {
		t.Skip("set GAMEMAKER_EVAL_CONFIG and GAMEMAKER_EVAL_ENV_FILE on the provider host")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var original config.Config
	if err = yaml.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	master := os.Getenv("AURAGO_MASTER_KEY")
	if master == "" {
		env, err := os.Open(os.Getenv("GAMEMAKER_EVAL_ENV_FILE"))
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(env)
		for scanner.Scan() {
			key, value, ok := strings.Cut(scanner.Text(), "=")
			if ok && strings.TrimSpace(key) == "AURAGO_MASTER_KEY" {
				master = strings.Trim(strings.TrimSpace(value), "\"'")
			}
		}
		env.Close()
		if err = scanner.Err(); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := original.Directories.DataDir
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(filepath.Dir(configPath), dataDir)
	}
	vault, err := security.NewVault(master, filepath.Join(dataDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	master = ""
	root := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true, JobTimeout: 5 * time.Minute, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.SetSkillStatus(nil, true)
	previous := gamemaker.DefaultService()
	gamemaker.SetDefaultService(service)
	defer gamemaker.SetDefaultService(previous)
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 65536
	cfg.CircuitBreaker.LLMTimeoutSeconds = 100
	cfg.CircuitBreaker.MaxToolCalls = 24
	cfg.GameMaker.Enabled = true
	cfg.GameMaker.AllowCreate = true
	cfg.GameMaker.AllowEdit = true
	cfg.Directories.ToolsDir = filepath.Join(root, "tools")
	cfg.Directories.WorkspaceDir = root
	for _, id := range []string{"agnesai", "stepfun"} {
		p := original.FindProvider(id)
		if p == nil {
			t.Fatalf("missing selected provider %s", id)
		}
		selected := *p
		selected.APIKey, err = vault.ReadSecret("provider_" + id + "_api_key")
		if err != nil {
			t.Fatalf("configured credential unavailable for %s", id)
		}
		security.RegisterSensitive(selected.APIKey)
		cfg.Providers = append(cfg.Providers, selected)
	}
	server := &Server{Cfg: cfg, GameMaker: service, Logger: logger, HistoryManager: memory.NewEphemeralHistoryManager()}
	server.Registry = tools.NewProcessRegistry(logger)
	server.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer server.ShortTermMem.Close()
	first := cfg.Providers[0]
	server.LLMClient = llm.NewClientFromProviderWithConfig(cfg, first.Type, first.BaseURL, first.APIKey, first.AccountID)
	service.SetRunner(&gameMakerAgentRunner{server: server, service: service})
	var mu sync.RWMutex
	var projectID, jobID string
	parent, err := os.ReadFile("../gamemaker/testdata/studio-parent.html")
	if err != nil {
		t.Fatal(err)
	}
	// Keep polling across successive evaluation jobs; this is a local test parent.
	html := strings.Replace(string(parent), "if(polling||window.done)return", "if(polling)return", 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		pid, jid := projectID, jobID
		mu.RUnlock()
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, html)
			return
		case "/state":
			job, _ := service.GetJob(r.Context(), jid)
			grant, _ := service.CreatePreviewGrant(pid)
			json.NewEncoder(w).Encode(map[string]any{"job": job, "grant": grant})
			return
		case "/report":
			var report gamemaker.PreviewReport
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1600000)).Decode(&report) != nil {
				w.WriteHeader(400)
				return
			}
			if err := service.ReportPreview(pid, report); err != nil {
				http.Error(w, err.Error(), 400)
			}
			return
		}
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/api/game-maker/preview/"), "/", 2)
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		data, kind, err := service.PreviewFile(parts[0], parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Security-Policy", gameMakerPreviewCSP)
		w.Header().Set("Content-Type", kind)
		w.Write(data)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:8896")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go httpServer.Serve(listener)
	defer httpServer.Close()
	t.Log("Evaluation parent ready at http://127.0.0.1:8896/ (SSH tunnel, no service replacement)")
	reports := filepath.Join(filepath.Dir(configPath), "reports", "game-maker-evaluation")
	if err = os.MkdirAll(reports, 0700); err != nil {
		t.Fatal(err)
	}
	for _, provider := range cfg.Providers {
		for _, task := range []struct{ name, dimension, prompt string }{
			{"forest-fps", "3d", "Make a first-person forest patrol game using the local low-poly models: modern arms, rifle, pine trees and five soldiers as targets. WASD movement, mouse or arrow aiming, Space to shoot, F reload, health, a 90-second limit, restart and a visible Forest Patrol title. Award ten points per target and complete after five targets. Use the supported FPS base and implement all requested scoring rules."},
			{"breakout", "2d", "Create Breakout using built-in sprite art for the paddle, ball and bricks. Three lives, ten points per brick, left/right movement, Space to launch and R restart. Show a clear victory when all bricks are cleared. Use the blocks base and retain its working lifecycle."},
		} {
			if only := os.Getenv("GAMEMAKER_EVAL_PROVIDER"); only != "" && only != provider.ID {
				continue
			}
			if only := os.Getenv("GAMEMAKER_EVAL_TASK"); only != "" && only != task.name {
				continue
			}
			p, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: task.name, Dimension: task.dimension, Description: task.prompt, ProviderID: provider.ID, Model: provider.Model})
			if err != nil {
				t.Fatal(err)
			}
			job, err := service.StartJob(context.Background(), p.ID, gamemaker.StartJobRequest{Prompt: task.prompt})
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			projectID = p.ID
			jobID = job.ID
			mu.Unlock()
			started := time.Now()
			for time.Since(started) < 6*time.Minute {
				time.Sleep(250 * time.Millisecond)
				job, _ = service.GetJob(context.Background(), job.ID)
				if job.Status == "ready" || job.Status == "failed" || job.Status == "cancelled" {
					break
				}
			}
			events, _ := service.EventsAfter(context.Background(), p.ID, 0, 500)
			result := map[string]any{"model": provider.Model, "provider": provider.ID, "task": task.name, "seconds": time.Since(started).Seconds(), "job": job, "events": events, "context_cap": 65536, "tool_limit": 24}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			if err = os.WriteFile(filepath.Join(reports, provider.ID+"-"+task.name+".json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			t.Logf("model=%s task=%s status=%s seconds=%.1f error=%s", provider.Model, task.name, job.Status, time.Since(started).Seconds(), job.Error)
		}
	}
}
