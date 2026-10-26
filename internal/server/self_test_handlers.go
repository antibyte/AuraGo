package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/selftest"
	"aurago/internal/tools"
)

type selfTestRunContextKey struct{}
type selfTestRunContext struct {
	cfg    *config.Config
	client llm.ChatClient
}

func (s *Server) selfTestRuntime() *agent.DispatchContext {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil
	}
	return &agent.DispatchContext{
		Cfg: cfg, Logger: s.Logger, LLMClient: s.LLMClient, Vault: s.Vault, Registry: s.Registry,
		Manifest: tools.NewManifest(cfg.Directories.ToolsDir), CronManager: s.CronManager, MissionManagerV2: s.MissionManagerV2, WebhookManager: s.WebhookManager,
		LongTermMem: s.LongTermMem, ShortTermMem: s.ShortTermMem, KG: s.KG, InventoryDB: s.InventoryDB, InvasionDB: s.InvasionDB,
		CheatsheetDB: s.CheatsheetDB, ImageGalleryDB: s.ImageGalleryDB, MediaRegistryDB: s.MediaRegistryDB, HomepageRegistryDB: s.HomepageRegistryDB,
		ContactsDB: s.ContactsDB, PlannerDB: s.PlannerDB, SQLConnectionsDB: s.SQLConnectionsDB, SQLConnectionPool: s.SQLConnectionPool,
		RemoteHub: s.RemoteHub, HistoryMgr: s.HistoryManager, Guardian: s.Guardian, LLMGuardian: s.LLMGuardian,
		CoAgentRegistry: s.CoAgentRegistry, BudgetTracker: s.BudgetTracker, DaemonSupervisor: s.DaemonSupervisor, WorkspaceSearch: s.WorkspaceSearch,
		SessionID: "selftest-inventory", MessageSource: "self_test", Broker: agent.NoopBroker{},
	}
}

// Only booleans and YAML field names are inspected; configuration values and
// credentials never become inventory payloads.
func selfTestIntegrations(cfg *config.Config) []selftest.Capability {
	if cfg == nil {
		return nil
	}
	var result []selftest.Capability
	// Telegram is activated by its resolved Vault token, without an Enabled field.
	if cfg.Telegram.BotToken != "" {
		result = append(result, selftest.Capability{Name: "integration:telegram", Kind: "integration", Instance: "default", Enabled: true, Status: "admin_only"})
	}
	v := reflect.ValueOf(cfg).Elem()
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.Struct {
			continue
		}
		enabled := field.FieldByName("Enabled")
		if !enabled.IsValid() || enabled.Kind() != reflect.Bool || !enabled.Bool() {
			continue
		}
		name := strings.Split(typ.Field(i).Tag.Get("yaml"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		result = append(result, selftest.Capability{Name: "integration:" + strings.ReplaceAll(name, "_", "-"), Kind: "integration", Instance: "default", Enabled: true, Status: "admin_only"})
	}
	return result
}

func (s *Server) selfTestManager() (*selftest.Manager, error) {
	s.selfTestMu.Lock()
	defer s.selfTestMu.Unlock()
	if s.selfTests != nil {
		return s.selfTests, nil
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || cfg.Directories.DataDir == "" || s.integrationCtx == nil {
		return nil, errors.New("self-test storage or server lifecycle unavailable")
	}
	manager, err := selftest.NewManager(filepath.Join(cfg.Directories.DataDir, "selftests"), func(ctx context.Context, id string, c selftest.Case, tokens int, emit func(selftest.Evidence)) selftest.Execution {
		bound, ok := ctx.Value(selfTestRunContextKey{}).(selfTestRunContext)
		if !ok || bound.cfg != s.ConfigSnapshot() {
			return selftest.Execution{Verdict: "BLOCKED_PERMISSION", Code: "configuration_changed", Cleanup: "not_needed"}
		}
		if strings.HasPrefix(c.Tool, "integration:") {
			probeServer := &Server{Cfg: bound.cfg, Logger: s.Logger, Vault: s.Vault}
			return probeServer.executeSelfTestProbe(ctx, c, emit)
		}
		dc := s.selfTestRuntime()
		if dc == nil {
			return selftest.Execution{Verdict: "BLOCKED_SETUP", Code: "configuration_unavailable"}
		}
		dc.Cfg = bound.cfg
		return agent.ExecuteSelfTestPrompt(ctx, bound.client, bound.cfg.LLM.Model, dc, id, c, tokens, emit)
	})
	if err != nil {
		return nil, err
	}
	s.selfTests = manager
	return manager, nil
}

func (s *Server) closeSelfTests() {
	s.selfTestMu.Lock()
	manager := s.selfTests
	s.selfTests = nil
	s.selfTestMu.Unlock()
	if manager != nil {
		manager.Close()
	}
}

func (s *Server) registerSelfTestRoutes(mux *http.ServeMux) {
	mux.Handle("/api/self-tests", requireAdmin(s, http.HandlerFunc(s.handleSelfTests)))
	mux.Handle("/api/self-tests/", requireAdmin(s, http.HandlerFunc(s.handleSelfTests)))
}

func (s *Server) handleSelfTests(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	path := strings.TrimPrefix(r.URL.Path, "/api/self-tests")
	if path == "/catalog" && r.Method == http.MethodGet {
		catalog, err := selftest.Catalog()
		if err != nil {
			jsonError(w, "Self-test catalog unavailable", 503)
			return
		}
		inventory := agent.SelfTestInventory(s.selfTestRuntime())
		inventory = append(inventory, selfTestIntegrations(s.ConfigSnapshot())...)
		s.bindSelfTestProbes(inventory)
		writeJSON(w, map[string]any{"templates": catalog, "inventory": inventory, "profiles": []string{"read", "fixtures", "targeted"}})
		return
	}
	if r.Method == http.MethodPost && !checkCSRFOriginWithPolicy(r, true) {
		jsonError(w, "Invalid request origin", http.StatusForbidden)
		return
	}
	manager, err := s.selfTestManager()
	if err != nil {
		jsonError(w, "Self-test runtime unavailable", 503)
		return
	}
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]any{"runs": manager.List()})
		case http.MethodPost:
			var req selftest.Request
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
			d.DisallowUnknownFields()
			if d.Decode(&req) != nil {
				jsonError(w, "Invalid self-test request", 400)
				return
			}
			if d.Decode(new(any)) != io.EOF {
				jsonError(w, "Invalid self-test request", 400)
				return
			}
			runtime := s.selfTestRuntime()
			if runtime == nil {
				jsonError(w, "Self-test runtime unavailable", 503)
				return
			}
			inventory := agent.SelfTestInventory(runtime)
			inventory = append(inventory, selfTestIntegrations(runtime.Cfg)...)
			s.bindSelfTestProbes(inventory)
			parent := context.WithValue(s.integrationCtx, selfTestRunContextKey{}, selfTestRunContext{cfg: runtime.Cfg, client: runtime.LLMClient})
			run, err := manager.Start(parent, req, inventory)
			if errors.Is(err, selftest.ErrBusy) {
				jsonError(w, "Self-test already running", 409)
				return
			}
			if err != nil {
				jsonError(w, "Self-test request could not be started", 400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			writeJSON(w, run)
		default:
			jsonError(w, "Method not allowed", 405)
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 2 || len(parts[0]) != 32 || strings.Trim(parts[0], "0123456789abcdef") != "" {
		jsonError(w, "Invalid self-test ID", 400)
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if !manager.Cancel(id) {
			jsonError(w, "Self-test is not active", 409)
			return
		}
		writeJSON(w, map[string]bool{"cancelled": true})
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", 405)
		return
	}
	run, ok := manager.Get(id)
	if !ok {
		jsonError(w, "Self-test not found", 404)
		return
	}
	if len(parts) == 1 {
		writeJSON(w, run)
		return
	}
	switch parts[1] {
	case "report.md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="aurago-selftest-`+id+`.md"`)
		_, _ = io.WriteString(w, selftest.Markdown(run))
	case "results.json":
		w.Header().Set("Content-Disposition", `attachment; filename="aurago-selftest-`+id+`.json"`)
		writeJSON(w, run)
	default:
		jsonError(w, "Not found", 404)
	}
}
