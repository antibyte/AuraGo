package server

import (
	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/tools"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGameMakerCloudflareRESTPreservesSelectedGateway(t *testing.T) {
	headers := make(chan string, 20)
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		headers <- r.Header.Get("cf-aig-gateway-id")
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Done.<done/>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer provider.Close()
	transport := provider.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, provider.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = previous; transport.CloseIdleConnections() }()
	root := t.TempDir()
	svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	svc.SetSkillStatus(nil, true)
	var cfg config.Config
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.BaseURL = "https://api.openai.com/v1"
	cfg.LLM.APIKey = "fixture-main"
	cfg.LLM.Model = "gpt-4.1"
	p := config.ProviderEntry{ID: "selected", Type: "cloudflare-rest", AccountID: "account", GatewayID: "selected-gateway", APIKey: "fixture-selected", Model: "google/gemini-2.5-flash", ContextWindow: 65536, MaxOutputTokens: 32768}
	main := p
	main.ID, main.GatewayID, main.APIKey = "main", "main-gateway", "fixture-main"
	cfg.Providers = []config.ProviderEntry{main, p}
	cfg.ResolveProviders()
	cfg.Agent.ContextWindow = 65536
	cfg.CircuitBreaker.LLMTimeoutSeconds, cfg.CircuitBreaker.MaxToolCalls = 2, 4
	cfg.Directories.WorkspaceDir, cfg.Directories.ToolsDir = root, filepath.Join(root, "tools")
	cfg.GameMaker.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := llm.NewFailoverManager(&cfg, logger)
	defer client.Stop()
	s := &Server{Cfg: &cfg, LLMClient: client, GameMaker: svc, Logger: logger, HistoryManager: memory.NewEphemeralHistoryManager(), Registry: tools.NewProcessRegistry(logger)}
	s.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ShortTermMem.Close()
	runner := &gameMakerAgentRunner{server: s, service: svc}
	finished := make(chan error, 1)
	svc.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		finished <- runner.RunGameMakerJob(ctx, run)
		return errors.New("review fixture complete")
	}))
	project, err := svc.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Gateway review", Dimension: "2d", Description: "Build a small game", ProviderID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil && strings.Contains(err.Error(), "Cloudflare REST requires account_id, gateway_id") {
			t.Fatalf("selected Game Maker REST provider lost gateway identity: %v", err)
		}
		select {
		case gateway := <-headers:
			if gateway != p.GatewayID {
				t.Fatalf("selected %s, got %s", p.GatewayID, gateway)
			}
		default:
			t.Fatalf("fixture reached no provider: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("fixture timed out")
	}
}
