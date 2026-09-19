package server

import (
	"aurago/internal/config"
	"aurago/internal/security"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDisclosureAPIAndRuntimeRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	initial := `agent:
  max_tool_calls: 15
  budget: {enabled: false, daily_limit_usd: 5}
  discover_tools_snapshot_ttl_minutes: 10
skill_manager: {read_only: true}
memory_analysis: {real_time: false}
`
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("12", 32), filepath.Join(root, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	get := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if get.Code != 200 || strings.Contains(get.Body.String(), "\"discover_tools_snapshot_ttl_minutes\":") || strings.Contains(get.Body.String(), "\"real_time\":") {
		t.Fatalf("obsolete API settings: %s", get.Body.String())
	}
	var loaded map[string]interface{}
	if err := json.Unmarshal(get.Body.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded["_effective_tool_policy"] == nil || loaded["skill_manager"] != nil {
		t.Fatal("missing effective state or legacy skill root")
	}
	patch := `{"_effective_tool_policy":{"max_tools":999999},"circuit_breaker":{"max_tool_calls":23},"budget":{"daily_limit_usd":7},"tools":{"skill_manager":{"readonly":false}},"agent":{"tool_output_limit":0,"max_tool_guides":4,"announcement_detector":{"enabled":false},"adaptive_tools":{"always_include":[],"session_tool_retention_turns":0},"output_compression":{"enabled":false,"smart_crusher":{"enabled":false},"reversible":{"enabled":false}},"importance_scoring":{"enabled":false},"auto_learning":{"enabled":false}},"memory_analysis":{"auto_confirm_threshold":0.73,"reflection_day":"friday"},"mcp_server":{"allowed_tools":[]}}`
	put := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
	if put.Code != 200 {
		t.Fatalf("save: %d %s", put.Code, put.Body.String())
	}
	after, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.CircuitBreaker.MaxToolCalls != 23 || after.Budget.DailyLimitUSD != 7 || after.Tools.SkillManager.ReadOnly || after.Agent.ToolOutputLimit != 50000 || after.Agent.MaxToolGuides != 4 || after.Agent.AnnouncementDetector.Enabled || after.MemoryAnalysis.AutoConfirm != 0.73 || after.MemoryAnalysis.ReflectionDay != "friday" {
		t.Fatal("saved values do not reach runtime")
	}
	if after.Agent.OutputCompression.Enabled || after.Agent.OutputCompression.SmartCrusher.Enabled || after.Agent.OutputCompression.Reversible.Enabled || after.Agent.ImportanceScoring.Enabled || after.Agent.AutoLearning.Enabled || len(after.Agent.AdaptiveTools.AlwaysInclude) != 0 || len(after.MCPServer.AllowedTools) != 0 {
		t.Fatal("disabled settings or empty lists overwritten")
	}
	data, _ := os.ReadFile(path)
	for _, legacy := range []string{"_effective_tool_policy", "snapshot_ttl", "real_time", "read_only:"} {
		if strings.Contains(string(data), legacy) {
			t.Errorf("metadata or obsolete path persisted: %s", legacy)
		}
	}
	before := string(data)
	for _, invalid := range []string{`{"agent":{"tool_output_limit":-1}}`, `{"memory_analysis":{"real_time":true}}`, `{"agent":{"auto_learning":{"enabled":null}}}`, `{"memory_analysis":{"auto_confirm_threshold":1.1}}`, `{"memory_analysis":{"reflection_day":"invalid"}}`} {
		rec := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(invalid)))
		data, _ := os.ReadFile(path)
		if rec.Code != 400 || string(data) != before {
			t.Fatalf("invalid save changed file: %d", rec.Code)
		}
	}
}
