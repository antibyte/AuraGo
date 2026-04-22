package invasion

import (
	"aurago/internal/config"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// minimalMasterCfg returns a Config with the bare minimum for GenerateEggConfig / ResolveMasterURL.
func minimalMasterCfg() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Host = "192.168.1.10"
	cfg.Server.Port = 8080
	cfg.Agent.SystemLanguage = "en"
	cfg.Agent.ContextWindow = 128000
	return cfg
}

// ── GenerateEggConfig ───────────────────────────────────────────────────────

func TestGenerateEggConfig_MinimalConfig(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "egg-1", Name: "Worker", EggPort: 8099}
	nest := NestRecord{ID: "nest-1", Name: "Server"}

	data, err := GenerateEggConfig(masterCfg, egg, nest, "aabb", "ws://localhost:8080/api/invasion/ws", "ccdd")
	if err != nil {
		t.Fatalf("GenerateEggConfig: %v", err)
	}

	// Must be valid YAML
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("generated config is not valid YAML: %v", err)
	}

	// Verify egg_mode section
	eggMode, ok := parsed["egg_mode"].(map[string]interface{})
	if !ok {
		t.Fatal("egg_mode section missing")
	}
	if eggMode["enabled"] != true {
		t.Error("egg_mode.enabled should be true")
	}
	if eggMode["master_url"] != "ws://localhost:8080/api/invasion/ws" {
		t.Errorf("master_url = %v", eggMode["master_url"])
	}
	if eggMode["shared_key"] != "aabb" {
		t.Errorf("shared_key = %v", eggMode["shared_key"])
	}
	if eggMode["egg_id"] != "egg-1" {
		t.Errorf("egg_id = %v", eggMode["egg_id"])
	}
	if eggMode["nest_id"] != "nest-1" {
		t.Errorf("nest_id = %v", eggMode["nest_id"])
	}
}

func TestGenerateEggConfig_InheritLLM(t *testing.T) {
	masterCfg := minimalMasterCfg()
	masterCfg.LLM.ProviderType = "openrouter"
	masterCfg.LLM.BaseURL = "https://openrouter.ai/api/v1"
	masterCfg.LLM.APIKey = "sk-test"
	masterCfg.LLM.Model = "gpt-4"
	masterCfg.LLM.UseNativeFunctions = true
	masterCfg.LLM.Temperature = 0.7

	egg := EggRecord{ID: "e1", Name: "W", InheritLLM: true, EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	data, err := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost", "bb")
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	yaml.Unmarshal(data, &parsed)

	llm, ok := parsed["llm"].(map[string]interface{})
	if !ok {
		t.Fatal("llm section missing")
	}
	if llm["model"] != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", llm["model"])
	}
	if llm["base_url"] != "https://openrouter.ai/api/v1" {
		t.Errorf("base_url = %v", llm["base_url"])
	}
}

func TestGenerateEggConfig_AllowedTools_ShellOnly(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{
		ID: "e1", Name: "W", EggPort: 8099,
		AllowedTools: `["shell","file_write"]`,
	}
	nest := NestRecord{ID: "n1", Name: "S"}

	data, err := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost", "bb")
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	yaml.Unmarshal(data, &parsed)

	agent, ok := parsed["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("agent section missing")
	}
	if agent["allow_shell"] != true {
		t.Error("allow_shell should be true for shell tool")
	}
	if agent["allow_python"] != false {
		t.Error("allow_python should be false when python not in AllowedTools")
	}
}

func TestGenerateEggConfig_AllowedTools_PythonOnly(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{
		ID: "e1", Name: "W", EggPort: 8099,
		AllowedTools: `["python_execute","file_read"]`,
	}
	nest := NestRecord{ID: "n1", Name: "S"}

	data, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost", "bb")

	var parsed map[string]interface{}
	yaml.Unmarshal(data, &parsed)

	agent := parsed["agent"].(map[string]interface{})
	if agent["allow_shell"] != false {
		t.Error("allow_shell should be false")
	}
	if agent["allow_python"] != true {
		t.Error("allow_python should be true for python_execute tool")
	}
}

func TestGenerateEggConfig_EmptyAllowedTools_DefaultsTrue(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099, AllowedTools: ""}
	nest := NestRecord{ID: "n1", Name: "S"}

	data, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost", "bb")

	var parsed map[string]interface{}
	yaml.Unmarshal(data, &parsed)

	agent := parsed["agent"].(map[string]interface{})
	if agent["allow_shell"] != true {
		t.Error("allow_shell should default to true with empty AllowedTools")
	}
	if agent["allow_python"] != true {
		t.Error("allow_python should default to true with empty AllowedTools")
	}
}

func TestGenerateEggConfig_DisabledIntegrations(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	data, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost", "bb")

	var parsed map[string]interface{}
	yaml.Unmarshal(data, &parsed)

	// All integrations should be disabled
	for _, key := range []string{"discord", "email", "home_assistant", "docker", "proxmox", "tailscale"} {
		section, ok := parsed[key].(map[string]interface{})
		if !ok {
			continue
		}
		if enabled, exists := section["enabled"]; exists && enabled != false {
			t.Errorf("%s.enabled = %v, want false", key, enabled)
		}
	}
}

func TestGenerateEggConfig_MasterKey(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	data, _ := GenerateEggConfig(masterCfg, egg, nest, "shared", "ws://localhost", "egg-master-key-hex")

	var parsed map[string]interface{}
	yaml.Unmarshal(data, &parsed)

	server := parsed["server"].(map[string]interface{})
	if server["master_key"] != "egg-master-key-hex" {
		t.Errorf("master_key = %v", server["master_key"])
	}
}

// ── ResolveMasterURL ────────────────────────────────────────────────────────

func TestResolveMasterURL_Direct(t *testing.T) {
	cfg := minimalMasterCfg()
	nest := NestRecord{Host: "10.0.0.5", Route: "direct"}

	got := ResolveMasterURL(cfg, nest)
	want := "ws://10.0.0.5:8080/api/invasion/ws"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveMasterURL_Tailscale(t *testing.T) {
	cfg := minimalMasterCfg()
	nest := NestRecord{Host: "ts-node.tail.net", Route: "tailscale"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.Contains(got, "ts-node.tail.net") {
		t.Errorf("expected tailscale host in URL, got %q", got)
	}
}

func TestResolveMasterURL_SSHTunnel(t *testing.T) {
	cfg := minimalMasterCfg()
	nest := NestRecord{Host: "10.0.0.5", Route: "ssh_tunnel"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.HasPrefix(got, "ws://localhost:") {
		t.Errorf("ssh_tunnel should use localhost, got %q", got)
	}
}

func TestResolveMasterURL_DockerLocal(t *testing.T) {
	cfg := minimalMasterCfg()
	cfg.Server.Host = "0.0.0.0" // bind-all

	nest := NestRecord{Host: "", DeployMethod: "docker_local", Route: "direct"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.Contains(got, "host.docker.internal") {
		t.Errorf("docker_local with empty host should use host.docker.internal, got %q", got)
	}
}

func TestResolveMasterURL_DockerLocal_WithHost(t *testing.T) {
	cfg := minimalMasterCfg()
	nest := NestRecord{Host: "192.168.1.50", DeployMethod: "docker_local", Route: "direct"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.Contains(got, "192.168.1.50") {
		t.Errorf("docker_local with explicit host should use it, got %q", got)
	}
}

func TestResolveMasterURL_Custom(t *testing.T) {
	cfg := minimalMasterCfg()
	nest := NestRecord{
		Host:        "10.0.0.5",
		Route:       "custom",
		RouteConfig: "wss://custom.example.com:9443/api/invasion/ws",
	}

	got := ResolveMasterURL(cfg, nest)
	if got != "wss://custom.example.com:9443/api/invasion/ws" {
		t.Errorf("custom route should return RouteConfig, got %q", got)
	}
}

func TestResolveMasterURL_CustomEmpty(t *testing.T) {
	cfg := minimalMasterCfg()
	nest := NestRecord{Host: "10.0.0.5", Port: 22, Route: "custom", RouteConfig: ""}

	got := ResolveMasterURL(cfg, nest)
	// Should fall back to ws://host:port
	if !strings.Contains(got, "10.0.0.5") {
		t.Errorf("custom with empty config should fall back, got %q", got)
	}
}

func TestResolveMasterURL_DefaultRoute(t *testing.T) {
	cfg := minimalMasterCfg()
	nest := NestRecord{Host: "10.0.0.5", Route: ""}

	got := ResolveMasterURL(cfg, nest)
	if !strings.Contains(got, "10.0.0.5:8080") {
		t.Errorf("default route should use host:port, got %q", got)
	}
}

func TestResolveMasterURL_FallbackHost(t *testing.T) {
	cfg := minimalMasterCfg()
	cfg.Server.Host = "0.0.0.0"
	nest := NestRecord{Host: "", Route: "direct"}

	got := ResolveMasterURL(cfg, nest)
	// 0.0.0.0 should be replaced with localhost
	if !strings.Contains(got, "localhost") {
		t.Errorf("0.0.0.0 should fall back to localhost, got %q", got)
	}
}

func TestResolveMasterURL_DefaultPort(t *testing.T) {
	cfg := minimalMasterCfg()
	cfg.Server.Port = 0 // not set
	nest := NestRecord{Host: "10.0.0.1", Route: "direct"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.Contains(got, ":8080/") {
		t.Errorf("default port should be 8080, got %q", got)
	}
}

// ── TLS / wss tests (E.1) ──────────────────────────────────────────────────

func TestResolveMasterURL_HTTPS_Direct(t *testing.T) {
	cfg := minimalMasterCfg()
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443
	nest := NestRecord{Host: "10.0.0.5", Route: "direct"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.HasPrefix(got, "wss://") {
		t.Errorf("HTTPS should produce wss:// scheme, got %q", got)
	}
	if !strings.Contains(got, ":8443/") {
		t.Errorf("should use HTTPSPort 8443, got %q", got)
	}
}

func TestResolveMasterURL_HTTPS_DefaultPort(t *testing.T) {
	cfg := minimalMasterCfg()
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 0 // not set → default 443
	nest := NestRecord{Host: "10.0.0.5", Route: "direct"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.HasPrefix(got, "wss://") {
		t.Errorf("should be wss://, got %q", got)
	}
	if !strings.Contains(got, ":443/") {
		t.Errorf("should default to port 443, got %q", got)
	}
}

func TestResolveMasterURL_HTTPS_DockerLocal(t *testing.T) {
	cfg := minimalMasterCfg()
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 9443
	nest := NestRecord{Host: "", DeployMethod: "docker_local", Route: "direct"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.HasPrefix(got, "wss://") {
		t.Errorf("should be wss://, got %q", got)
	}
	if !strings.Contains(got, "host.docker.internal") {
		t.Errorf("docker_local should use host.docker.internal, got %q", got)
	}
	if !strings.Contains(got, ":9443/") {
		t.Errorf("should use HTTPS port, got %q", got)
	}
}

func TestResolveMasterURL_HTTPS_SSHTunnel(t *testing.T) {
	cfg := minimalMasterCfg()
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443
	nest := NestRecord{Host: "10.0.0.5", Route: "ssh_tunnel"}

	got := ResolveMasterURL(cfg, nest)
	if !strings.HasPrefix(got, "wss://localhost:") {
		t.Errorf("ssh_tunnel+HTTPS should be wss://localhost:, got %q", got)
	}
}

func TestResolveMasterURL_Custom_OverridesHTTPS(t *testing.T) {
	cfg := minimalMasterCfg()
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443
	nest := NestRecord{
		Host:        "10.0.0.5",
		Route:       "custom",
		RouteConfig: "ws://custom-url:1234/api/invasion/ws",
	}

	got := ResolveMasterURL(cfg, nest)
	// Custom route should return RouteConfig as-is, ignoring HTTPS settings
	if got != "ws://custom-url:1234/api/invasion/ws" {
		t.Errorf("custom route should return RouteConfig verbatim, got %q", got)
	}
}

func TestGenerateEggConfig_SelfSigned_TLSSkipVerify(t *testing.T) {
	masterCfg := minimalMasterCfg()
	masterCfg.Server.HTTPS.Enabled = true
	masterCfg.Server.HTTPS.CertMode = "selfsigned"

	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	data, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "wss://localhost:8443", "bb")

	var parsed map[string]interface{}
	yaml.Unmarshal(data, &parsed)

	eggMode := parsed["egg_mode"].(map[string]interface{})
	if eggMode["tls_skip_verify"] != true {
		t.Error("self-signed cert mode should set tls_skip_verify: true")
	}
}

func TestGenerateEggConfig_AutoCert_NoTLSSkipVerify(t *testing.T) {
	masterCfg := minimalMasterCfg()
	masterCfg.Server.HTTPS.Enabled = true
	masterCfg.Server.HTTPS.CertMode = "auto" // Let's Encrypt

	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	data, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "wss://localhost:443", "bb")

	var parsed map[string]interface{}
	yaml.Unmarshal(data, &parsed)

	eggMode := parsed["egg_mode"].(map[string]interface{})
	if _, exists := eggMode["tls_skip_verify"]; exists {
		t.Error("auto cert mode should NOT set tls_skip_verify")
	}
}

// ── ApplySafeConfigPatch tests ──────────────────────────────────────────────

func TestApplySafeConfigPatch_Model(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	original, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost:8080/api/invasion/ws", "bb")

	model := "gpt-4o"
	patched, err := ApplySafeConfigPatch(original, SafeConfigPatch{Model: &model})
	if err != nil {
		t.Fatalf("ApplySafeConfigPatch: %v", err)
	}

	var cfg map[string]interface{}
	yaml.Unmarshal(patched, &cfg)
	llm := cfg["llm"].(map[string]interface{})
	if llm["model"] != "gpt-4o" {
		t.Errorf("model = %v, want gpt-4o", llm["model"])
	}
}

func TestApplySafeConfigPatch_RuntimeFlags(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	original, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost:8080/api/invasion/ws", "bb")

	fsFalse := false
	netTrue := true
	patched, err := ApplySafeConfigPatch(original, SafeConfigPatch{
		AllowFilesystemWrite: &fsFalse,
		AllowNetworkRequests: &netTrue,
	})
	if err != nil {
		t.Fatalf("ApplySafeConfigPatch: %v", err)
	}

	var cfg map[string]interface{}
	yaml.Unmarshal(patched, &cfg)
	agent := cfg["agent"].(map[string]interface{})
	if agent["allow_filesystem_write"] != false {
		t.Error("allow_filesystem_write should be false")
	}
	if agent["allow_network_requests"] != true {
		t.Error("allow_network_requests should be true")
	}
}

func TestApplySafeConfigPatch_AllowedTools(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	original, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost:8080/api/invasion/ws", "bb")

	// Only python, no shell
	patched, err := ApplySafeConfigPatch(original, SafeConfigPatch{
		AllowedTools: []string{"python"},
	})
	if err != nil {
		t.Fatalf("ApplySafeConfigPatch: %v", err)
	}

	var cfg map[string]interface{}
	yaml.Unmarshal(patched, &cfg)
	agent := cfg["agent"].(map[string]interface{})
	if agent["allow_shell"] != false {
		t.Error("allow_shell should be false when only python is allowed")
	}
	if agent["allow_python"] != true {
		t.Error("allow_python should be true")
	}
}

func TestApplySafeConfigPatch_EmptyPatch(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	original, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost:8080/api/invasion/ws", "bb")

	patched, err := ApplySafeConfigPatch(original, SafeConfigPatch{})
	if err != nil {
		t.Fatalf("ApplySafeConfigPatch: %v", err)
	}

	// Empty patch should produce identical config
	if string(patched) != string(original) {
		t.Error("empty patch should not change config")
	}
}

func TestApplySafeConfigPatch_InvalidYAML(t *testing.T) {
	_, err := ApplySafeConfigPatch([]byte("not: [valid: yaml"), SafeConfigPatch{})
	if err == nil {
		t.Error("invalid YAML should return error")
	}
}

func TestApplySafeConfigPatch_ProviderAndBaseURL(t *testing.T) {
	masterCfg := minimalMasterCfg()
	egg := EggRecord{ID: "e1", Name: "W", EggPort: 8099}
	nest := NestRecord{ID: "n1", Name: "S"}

	original, _ := GenerateEggConfig(masterCfg, egg, nest, "aa", "ws://localhost:8080/api/invasion/ws", "bb")

	provider := "openai"
	baseURL := "https://api.openai.com/v1"
	patched, err := ApplySafeConfigPatch(original, SafeConfigPatch{
		Provider: &provider,
		BaseURL:  &baseURL,
	})
	if err != nil {
		t.Fatalf("ApplySafeConfigPatch: %v", err)
	}

	var cfg map[string]interface{}
	yaml.Unmarshal(patched, &cfg)
	llm := cfg["llm"].(map[string]interface{})
	if llm["provider"] != "openai" {
		t.Errorf("provider = %v, want openai", llm["provider"])
	}
	if llm["base_url"] != "https://api.openai.com/v1" {
		t.Errorf("base_url = %v, want https://api.openai.com/v1", llm["base_url"])
	}
}
