package config

import (
	"aurago/internal/kgquality"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type testSecretVault struct {
	data map[string]string
}

func (v *testSecretVault) ReadSecret(key string) (string, error) {
	return v.data[key], nil
}

func (v *testSecretVault) WriteSecret(key, value string) error {
	if v.data == nil {
		v.data = map[string]string{}
	}
	v.data[key] = value
	return nil
}

func TestMigratePlaintextSecretsToVaultMovesKlipperPrinterAPIKeys(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `
three_d_printers:
  enabled: true
  klipper:
    enabled: true
    printers:
      - id: voron-main
        name: Voron 2.4
        url: http://192.168.6.60:7125
        api_key: moon-secret
`
	if err := os.WriteFile(configPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault := &testSecretVault{data: map[string]string{}}

	MigratePlaintextSecretsToVault(configPath, vault, slog.Default())

	vaultKey := ThreeDPrinterKlipperAPIKeyVaultKey("voron-main")
	if got := vault.data[vaultKey]; got != "moon-secret" {
		t.Fatalf("vault[%q] = %q, want moon-secret", vaultKey, got)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if strings.Contains(string(data), "moon-secret") || strings.Contains(string(data), "api_key") {
		t.Fatalf("migrated config still contains plaintext api_key:\n%s", string(data))
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load migrated config: %v", err)
	}
	cfg.ApplyVaultSecrets(vault)
	if got := cfg.ThreeDPrinters.Klipper.Printers[0].APIKey; got != "moon-secret" {
		t.Fatalf("runtime APIKey = %q, want moon-secret", got)
	}
}

func TestMigratePlaintextSecretsToVaultRemovesUnaddressableKlipperAPIKeys(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `
three_d_printers:
  klipper:
    printers:
      - name: Legacy Klipper
        url: http://192.168.6.60:7125
        api_key: orphaned-secret
`
	if err := os.WriteFile(configPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	vault := &testSecretVault{data: map[string]string{}}

	MigratePlaintextSecretsToVault(configPath, vault, slog.Default())

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if strings.Contains(string(data), "orphaned-secret") || strings.Contains(string(data), "api_key") {
		t.Fatalf("migrated config still contains unaddressable api_key:\n%s", string(data))
	}
	if len(vault.data) != 0 {
		t.Fatalf("vault should not receive a key for printer without id, got %+v", vault.data)
	}
}

func TestLoadAbsolutePaths(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	workspacePath := "/tmp/workspace"
	if os.PathSeparator == '\\' {
		workspacePath = "C:\\absolute\\path\\workspace"
	}

	configContent := `
directories:
  data_dir: './data'
  workspace_dir: '` + workspacePath + `'
  skills_dir: '../skills'
sqlite:
  short_term_path: './data/short_term.db'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Calculate expected paths
	absConfigDir, _ := filepath.Abs(tmpDir)
	expectedDataDir := filepath.Join(absConfigDir, "./data")
	expectedWorkspaceDir := workspacePath
	expectedSkillsDir := filepath.Join(absConfigDir, "../skills")
	expectedShortTermPath := filepath.Join(absConfigDir, "./data/short_term.db")

	if cfg.Directories.DataDir != expectedDataDir {
		t.Errorf("expected DataDir %s, got %s", expectedDataDir, cfg.Directories.DataDir)
	}
	if cfg.Directories.WorkspaceDir != expectedWorkspaceDir {
		t.Errorf("expected WorkspaceDir %s, got %s", expectedWorkspaceDir, cfg.Directories.WorkspaceDir)
	}
	if cfg.Directories.SkillsDir != expectedSkillsDir {
		t.Errorf("expected SkillsDir %s, got %s", expectedSkillsDir, cfg.Directories.SkillsDir)
	}
	if cfg.SQLite.ShortTermPath != expectedShortTermPath {
		t.Errorf("expected ShortTermPath %s, got %s", expectedShortTermPath, cfg.SQLite.ShortTermPath)
	}
}

func TestLoadKnowledgeGraphQualityDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := kgquality.DefaultPolicy()
	if got := cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays; got != want.PendingCoMentionTTLDays {
		t.Fatalf("PendingCoMentionTTLDays = %d, want %d", got, want.PendingCoMentionTTLDays)
	}
	if got := cfg.Tools.KnowledgeGraph.LowConfidenceCoMentionMinWeight; got != want.LowConfidenceCoMentionMinWeight {
		t.Fatalf("LowConfidenceCoMentionMinWeight = %d, want %d", got, want.LowConfidenceCoMentionMinWeight)
	}
	if got := cfg.Tools.KnowledgeGraph.HideLowConfidenceByDefault; got != want.HideLowConfidenceByDefault {
		t.Fatalf("HideLowConfidenceByDefault = %v, want %v", got, want.HideLowConfidenceByDefault)
	}
}

func TestLoadAppliesSupertonicTTSDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.TTS.Supertonic.AutoStart {
		t.Fatal("tts.supertonic.auto_start must default to false")
	}
	if cfg.TTS.Supertonic.URL != "http://127.0.0.1:7788" {
		t.Fatalf("tts.supertonic.url = %q", cfg.TTS.Supertonic.URL)
	}
	if cfg.TTS.Supertonic.ContainerName != "aurago-supertonic-tts" {
		t.Fatalf("tts.supertonic.container_name = %q", cfg.TTS.Supertonic.ContainerName)
	}
	if cfg.TTS.Supertonic.Image != "ghcr.io/antibyte/aurago-supertonic:latest" {
		t.Fatalf("tts.supertonic.image = %q", cfg.TTS.Supertonic.Image)
	}
	if cfg.TTS.Supertonic.ContainerPort != 7788 {
		t.Fatalf("tts.supertonic.container_port = %d", cfg.TTS.Supertonic.ContainerPort)
	}
	if cfg.TTS.Supertonic.DataPath != "data/supertonic" {
		t.Fatalf("tts.supertonic.data_path = %q", cfg.TTS.Supertonic.DataPath)
	}
	if cfg.TTS.Supertonic.Model != "supertonic-3" {
		t.Fatalf("tts.supertonic.model = %q", cfg.TTS.Supertonic.Model)
	}
	if cfg.TTS.Supertonic.Voice != "M1" {
		t.Fatalf("tts.supertonic.voice = %q", cfg.TTS.Supertonic.Voice)
	}
	if cfg.TTS.Supertonic.Speed != 1.0 {
		t.Fatalf("tts.supertonic.speed = %v", cfg.TTS.Supertonic.Speed)
	}
	if cfg.TTS.Supertonic.Steps != 8 {
		t.Fatalf("tts.supertonic.steps = %d", cfg.TTS.Supertonic.Steps)
	}
	if cfg.TTS.Supertonic.ResponseFormat != "wav" {
		t.Fatalf("tts.supertonic.response_format = %q", cfg.TTS.Supertonic.ResponseFormat)
	}
}

func TestSupertonicTTSYAMLRoundTrip(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `
tts:
  provider: supertonic
  supertonic:
    auto_start: true
    url: http://127.0.0.1:7789
    container_name: aurago-supertonic-custom
    image: ghcr.io/example/supertonic:test
    container_port: 7789
    data_path: data/custom-supertonic
    model: supertonic-3
    voice: studio_voice
    speed: 1.15
    steps: 12
    response_format: ogg
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var roundTrip Config
	if err := yaml.Unmarshal(out, &roundTrip); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	got := roundTrip.TTS.Supertonic
	if !got.AutoStart ||
		got.URL != "http://127.0.0.1:7789" ||
		got.ContainerName != "aurago-supertonic-custom" ||
		got.Image != "ghcr.io/example/supertonic:test" ||
		got.ContainerPort != 7789 ||
		got.DataPath != "data/custom-supertonic" ||
		got.Model != "supertonic-3" ||
		got.Voice != "studio_voice" ||
		got.Speed != 1.15 ||
		got.Steps != 12 ||
		got.ResponseFormat != "ogg" {
		t.Fatalf("round-tripped Supertonic config = %+v", got)
	}
}

func TestLoadKnowledgeGraphQualityOverrides(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `
tools:
  knowledge_graph:
    pending_co_mention_ttl_days: 11
    low_confidence_co_mention_min_weight: 4
    hide_low_confidence_by_default: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays; got != 11 {
		t.Fatalf("PendingCoMentionTTLDays = %d, want 11", got)
	}
	if got := cfg.Tools.KnowledgeGraph.LowConfidenceCoMentionMinWeight; got != 4 {
		t.Fatalf("LowConfidenceCoMentionMinWeight = %d, want 4", got)
	}
	if cfg.Tools.KnowledgeGraph.HideLowConfidenceByDefault {
		t.Fatal("HideLowConfidenceByDefault = true, want false")
	}
}

func TestLoadFritzBoxDefaultsAndTLSFields(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.FritzBox.Port != 49000 {
		t.Fatalf("fritzbox.port = %d, want 49000", cfg.FritzBox.Port)
	}
	if cfg.FritzBox.HTTPS {
		t.Fatal("fritzbox.https must default to false")
	}
	if cfg.FritzBox.Timeout != 10 {
		t.Fatalf("fritzbox.timeout = %d, want 10", cfg.FritzBox.Timeout)
	}
	if cfg.FritzBox.InsecureSkipVerify {
		t.Fatal("fritzbox.insecure_skip_verify must default to false")
	}
	if cfg.FritzBox.WebPort != 0 {
		t.Fatalf("fritzbox.web_port = %d, want 0", cfg.FritzBox.WebPort)
	}
}

func TestLoadFritzBoxLegacySmartHomeAlias(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `
fritzbox:
  smarthome:
    enabled: true
    readonly: true
    sub_features:
      templates: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.FritzBox.SmartHome.Enabled {
		t.Fatal("legacy fritzbox.smarthome.enabled was not mapped to smart_home")
	}
	if !cfg.FritzBox.SmartHome.ReadOnly {
		t.Fatal("legacy fritzbox.smarthome.readonly was not mapped to smart_home")
	}
	if !cfg.FritzBox.SmartHome.SubFeatures.Templates {
		t.Fatal("legacy fritzbox.smarthome.sub_features.templates was not mapped")
	}
}

func TestLoadFritzBoxCanonicalSmartHomeWinsOverLegacyAlias(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `
fritzbox:
  smarthome:
    enabled: false
  smart_home:
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.FritzBox.SmartHome.Enabled {
		t.Fatal("canonical fritzbox.smart_home should win over legacy smarthome")
	}
}

func TestLoadFritzBoxLegacyTelephonyCallListAlias(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `
fritzbox:
  telephony:
    enabled: true
    sub_features:
      call_list: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.FritzBox.Telephony.SubFeatures.CallLists {
		t.Fatal("legacy fritzbox.telephony.sub_features.call_list=false was not mapped to call_lists=false")
	}
}

func TestNormalizeDockerWorkspaceDirUsesMountedWorkdir(t *testing.T) {
	t.Parallel()

	got := normalizeDockerWorkspaceDir("/app/data", "./agent_workspace/workdir", true)
	if got != "/app/agent_workspace/workdir" {
		t.Fatalf("normalizeDockerWorkspaceDir() = %q, want /app/agent_workspace/workdir", got)
	}
}

func TestNormalizeDockerWorkspaceDirKeepsCustomPath(t *testing.T) {
	t.Parallel()

	got := normalizeDockerWorkspaceDir("/app/data", "/custom/workdir", true)
	if got != "/custom/workdir" {
		t.Fatalf("normalizeDockerWorkspaceDir() = %q, want /custom/workdir", got)
	}
}

func TestComputeFeatureAvailabilityDisablesUpdatesInDocker(t *testing.T) {
	t.Parallel()

	features := ComputeFeatureAvailability(Runtime{IsDocker: true}, false)
	updates, ok := features["updates"]
	if !ok {
		t.Fatal("expected updates feature availability to be reported")
	}
	if updates.Available {
		t.Fatal("expected updates to be unavailable in Docker runtime")
	}
	if !strings.Contains(strings.ToLower(updates.Reason), "docker") {
		t.Fatalf("updates reason = %q, want Docker explanation", updates.Reason)
	}
}

func TestProbeDockerSocketHonorsDockerHostTCP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp listener unavailable on this host: %v", err)
	}
	defer listener.Close()

	t.Setenv("DOCKER_HOST", "tcp://"+listener.Addr().String())

	if !probeDockerSocket() {
		t.Fatal("expected Docker probe to accept reachable DOCKER_HOST tcp endpoint")
	}
}

func TestLoadInheritsDockerHostFromEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
docker:
  enabled: true
  host: ""
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("DOCKER_HOST", "tcp://docker-proxy:2375")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Docker.Host != "tcp://docker-proxy:2375" {
		t.Fatalf("docker.host = %q, want tcp://docker-proxy:2375", cfg.Docker.Host)
	}
}

func TestLoadKeepsExplicitDockerHost(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
docker:
  enabled: true
  host: "tcp://custom-docker:2375"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Setenv("DOCKER_HOST", "tcp://docker-proxy:2375")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Docker.Host != "tcp://custom-docker:2375" {
		t.Fatalf("docker.host = %q, want explicit custom host", cfg.Docker.Host)
	}
}

func TestLoadOutputCompressionAdvancedDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rs := cfg.Agent.OutputCompression.RepetitiveSubstitution
	if rs.Enabled {
		t.Fatal("repetitive_substitution.enabled must default to false")
	}
	if !rs.LZWEnabled {
		t.Fatal("repetitive_substitution.lzw_enabled must default to true")
	}
	if rs.LTSCLiteEnabled {
		t.Fatal("repetitive_substitution.ltsc_lite_enabled must default to false")
	}
	if rs.MinPhraseChars != 15 {
		t.Fatalf("min_phrase_chars = %d, want 15", rs.MinPhraseChars)
	}
	if rs.MinOccurrences != 3 {
		t.Fatalf("min_occurrences = %d, want 3", rs.MinOccurrences)
	}
	if rs.MinSavingsPercent != 15 {
		t.Fatalf("min_savings_percent = %d, want 15", rs.MinSavingsPercent)
	}
	if rs.MaxInputChars != 50000 {
		t.Fatalf("max_input_chars = %d, want 50000", rs.MaxInputChars)
	}
	if rs.MaxDictionaryEntries != 16 {
		t.Fatalf("max_dictionary_entries = %d, want 16", rs.MaxDictionaryEntries)
	}

	toon := cfg.Agent.OutputCompression.TOONJSON
	if toon.Enabled {
		t.Fatal("toon_json.enabled must default to false")
	}
	if toon.MinSavingsPercent != 10 {
		t.Fatalf("toon_json.min_savings_percent = %d, want 10", toon.MinSavingsPercent)
	}
	if toon.MaxRows != 200 {
		t.Fatalf("toon_json.max_rows = %d, want 200", toon.MaxRows)
	}
	if cfg.Agent.DiscoverToolsSnapshotTTLMinutes != 5 {
		t.Fatalf("discover_tools_snapshot_ttl_minutes = %d, want default 5", cfg.Agent.DiscoverToolsSnapshotTTLMinutes)
	}
}

func TestLoadDiscoverToolsSnapshotTTLExplicitAndFallback(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    int
	}{
		{
			name: "explicit",
			content: `agent:
  discover_tools_snapshot_ttl_minutes: 12
`,
			want: 12,
		},
		{
			name: "zero falls back",
			content: `agent:
  discover_tools_snapshot_ttl_minutes: 0
`,
			want: 5,
		},
		{
			name: "negative falls back",
			content: `agent:
  discover_tools_snapshot_ttl_minutes: -3
`,
			want: 5,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(configPath, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Agent.DiscoverToolsSnapshotTTLMinutes != tc.want {
				t.Fatalf("discover_tools_snapshot_ttl_minutes = %d, want %d", cfg.Agent.DiscoverToolsSnapshotTTLMinutes, tc.want)
			}
		})
	}
}

func TestLoadOutputCompressionAdvancedExplicitValues(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `
agent:
  output_compression:
    repetitive_substitution:
      enabled: true
      lzw_enabled: false
      ltsc_lite_enabled: true
      min_phrase_chars: 24
      min_occurrences: 5
      min_savings_percent: 30
      max_input_chars: 12345
      max_dictionary_entries: 7
    toon_json:
      enabled: true
      min_savings_percent: 22
      max_rows: 42
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	rs := cfg.Agent.OutputCompression.RepetitiveSubstitution
	if !rs.Enabled || rs.LZWEnabled || !rs.LTSCLiteEnabled {
		t.Fatalf("repetitive_substitution bools = %+v, want enabled true, lzw false, ltsc true", rs)
	}
	if rs.MinPhraseChars != 24 || rs.MinOccurrences != 5 || rs.MinSavingsPercent != 30 ||
		rs.MaxInputChars != 12345 || rs.MaxDictionaryEntries != 7 {
		t.Fatalf("repetitive_substitution values not preserved: %+v", rs)
	}

	toon := cfg.Agent.OutputCompression.TOONJSON
	if !toon.Enabled || toon.MinSavingsPercent != 22 || toon.MaxRows != 42 {
		t.Fatalf("toon_json values not preserved: %+v", toon)
	}
}

func TestGetSpecialist(t *testing.T) {
	cfg := &Config{}
	cfg.CoAgents.Specialists.Coder.Enabled = true

	tests := []struct {
		role    string
		wantNil bool
	}{
		{"researcher", false},
		{"coder", false},
		{"designer", false},
		{"security", false},
		{"writer", false},
		{"unknown", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := cfg.GetSpecialist(tt.role)
			if (got == nil) != tt.wantNil {
				t.Errorf("GetSpecialist(%q) nil=%v, wantNil=%v", tt.role, got == nil, tt.wantNil)
			}
		})
	}

	// Verify the coder specialist we set is actually enabled
	coder := cfg.GetSpecialist("coder")
	if coder == nil || !coder.Enabled {
		t.Error("expected coder specialist to be enabled")
	}
}

func TestValidSpecialistRoles(t *testing.T) {
	expected := []string{"researcher", "coder", "designer", "security", "writer"}
	for _, role := range expected {
		if !ValidSpecialistRoles[role] {
			t.Errorf("expected %q in ValidSpecialistRoles", role)
		}
	}
	if ValidSpecialistRoles["unknown"] {
		t.Error("unexpected role 'unknown' in ValidSpecialistRoles")
	}
}

func TestLoadDefaultsWriterSpecialistAdditionalPromptWhenMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got := cfg.CoAgents.Specialists.Writer.AdditionalPrompt
	for _, want := range []string{
		"## Multilingual natural writing defaults",
		"You are the AuraGo Writer Specialist.",
		"Do not use English-only rules blindly.",
		"Do not claim text was written by a human",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("writer additional prompt missing %q: %q", want, got)
		}
	}
}

func TestLoadPreservesExplicitEmptyWriterSpecialistAdditionalPrompt(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `
co_agents:
  specialists:
    writer:
      additional_prompt: ""
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CoAgents.Specialists.Writer.AdditionalPrompt != "" {
		t.Fatalf("writer additional prompt = %q, want explicit empty preserved", cfg.CoAgents.Specialists.Writer.AdditionalPrompt)
	}
}

func TestLoadPreservesCustomWriterSpecialistAdditionalPrompt(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configContent := `
co_agents:
  specialists:
    writer:
      additional_prompt: "Keep my own writer instructions."
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.CoAgents.Specialists.Writer.AdditionalPrompt, "Keep my own writer instructions."; got != want {
		t.Fatalf("writer additional prompt = %q, want %q", got, want)
	}
}

func TestLoadUpgradesLegacyIndexingExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
indexing:
  enabled: true
  directories:
    - ./knowledge
  extensions:
    - .txt
    - .md
    - .json
    - .csv
    - .log
    - .yaml
    - .yml
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	for _, want := range []string{".pdf", ".docx", ".xlsx", ".pptx", ".odt", ".rtf"} {
		if !slices.Contains(cfg.Indexing.Extensions, want) {
			t.Fatalf("expected upgraded indexing extensions to include %s, got %v", want, cfg.Indexing.Extensions)
		}
	}
}

func TestLoadKeepsCustomIndexingExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
indexing:
  enabled: true
  directories:
    - ./knowledge
  extensions:
    - .txt
    - .md
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Indexing.Extensions) != 2 || cfg.Indexing.Extensions[0] != ".txt" || cfg.Indexing.Extensions[1] != ".md" {
		t.Fatalf("expected custom indexing extensions to stay unchanged, got %v", cfg.Indexing.Extensions)
	}
}

func TestLoadIndexingChunkingDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
indexing:
  enabled: true
  directories:
    - ./knowledge
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if got, want := cfg.Indexing.Chunking.Strategy, "recursive"; got != want {
		t.Fatalf("chunking strategy = %q, want %q", got, want)
	}
	if got, want := cfg.Indexing.Chunking.MaxChars, 3500; got != want {
		t.Fatalf("chunking max chars = %d, want %d", got, want)
	}
	if got, want := cfg.Indexing.Chunking.OverlapChars, 200; got != want {
		t.Fatalf("chunking overlap chars = %d, want %d", got, want)
	}
	if got, want := cfg.Indexing.Chunking.MaxChunksPerFile, 200; got != want {
		t.Fatalf("chunking max chunks per file = %d, want %d", got, want)
	}
}

func TestLoadNormalizesIndexingChunkingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
indexing:
  enabled: true
  directories:
    - ./knowledge
  chunking:
    strategy: unknown
    max_chars: -10
    overlap_chars: 9000
    max_chunks_per_file: 0
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if got, want := cfg.Indexing.Chunking.Strategy, "recursive"; got != want {
		t.Fatalf("normalized strategy = %q, want %q", got, want)
	}
	if got, want := cfg.Indexing.Chunking.MaxChars, 3500; got != want {
		t.Fatalf("normalized max chars = %d, want %d", got, want)
	}
	if got, want := cfg.Indexing.Chunking.OverlapChars, 200; got != want {
		t.Fatalf("normalized overlap chars = %d, want %d", got, want)
	}
	if got, want := cfg.Indexing.Chunking.MaxChunksPerFile, 200; got != want {
		t.Fatalf("normalized max chunks per file = %d, want %d", got, want)
	}
}

func TestLoadDoesNotAutoEnableInnerVoiceFromEmotionSynthesizer(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
personality:
  engine_v2: true
  emotion_synthesizer:
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Personality.InnerVoice.Enabled {
		t.Fatal("inner_voice.enabled should stay opt-in even when emotion_synthesizer is enabled")
	}
	if !cfg.Personality.EmotionSynthesizer.Enabled {
		t.Fatal("emotion_synthesizer.enabled should remain enabled")
	}
}

func TestLoadEmotionSynthesizerTriggersOnMoodChangeByDefault(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
personality:
  engine_v2: true
  emotion_synthesizer:
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange {
		t.Fatal("trigger_on_mood_change should default to true when omitted")
	}
}

func TestLoadEmotionSynthesizerKeepsExplicitMoodChangeTriggerFalse(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
personality:
  engine_v2: true
  emotion_synthesizer:
    enabled: true
    trigger_on_mood_change: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange {
		t.Fatal("explicit trigger_on_mood_change=false should be preserved")
	}
}

func TestLoadInnerVoiceStillEnablesDependenciesWhenExplicit(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
personality:
  inner_voice:
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Personality.InnerVoice.Enabled {
		t.Fatal("explicit inner_voice.enabled=true should be preserved")
	}
	if !cfg.Personality.EmotionSynthesizer.Enabled {
		t.Fatal("explicit inner voice should enable emotion synthesizer dependency")
	}
	if !cfg.Personality.EngineV2 {
		t.Fatal("explicit inner voice should enable personality engine v2 dependency")
	}
}

func TestLoadRemoteControlDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RemoteControl.DiscoveryPort != 8092 {
		t.Fatalf("discovery_port = %d, want 8092", cfg.RemoteControl.DiscoveryPort)
	}
	if cfg.RemoteControl.ConnectionMode != "auto" {
		t.Fatalf("connection_mode = %q, want auto", cfg.RemoteControl.ConnectionMode)
	}
	if cfg.RemoteControl.TailscaleAddress != "" {
		t.Fatalf("tailscale_address = %q, want empty", cfg.RemoteControl.TailscaleAddress)
	}
	if cfg.RemoteControl.SupervisorURL != "" {
		t.Fatalf("supervisor_url = %q, want empty", cfg.RemoteControl.SupervisorURL)
	}
	if cfg.RemoteControl.MaxFileSizeMB != 50 {
		t.Fatalf("max_file_size_mb = %d, want 50", cfg.RemoteControl.MaxFileSizeMB)
	}
	if !cfg.RemoteControl.AuditLog {
		t.Fatal("expected remote_control.audit_log to default to true")
	}
	if !cfg.RemoteControl.ReadOnly {
		t.Fatal("expected remote_control.readonly to default to true")
	}
}

func TestLoadRemoteControlReadOnlyExplicitFalsePreserved(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
remote_control:
  readonly: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RemoteControl.ReadOnly {
		t.Fatal("expected explicit remote_control.readonly=false to be preserved")
	}
}

func TestLoadRemoteControlAuditLogExplicitFalsePreserved(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
remote_control:
  audit_log: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RemoteControl.AuditLog {
		t.Fatal("expected explicit remote_control.audit_log=false to be preserved")
	}
}

func TestLoadSandboxLocalPythonFallbackDefaultAndExplicit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "default",
			content: "server:\n  ui_language: en\n",
			want:    false,
		},
		{
			name: "explicit true",
			content: `
sandbox:
  allow_local_python_fallback: true
`,
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(configPath, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Sandbox.AllowLocalPythonFallback != tc.want {
				t.Fatalf("allow_local_python_fallback = %v, want %v", cfg.Sandbox.AllowLocalPythonFallback, tc.want)
			}
		})
	}
}

func TestLoadRemoteControlConnectionSettings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
remote_control:
  connection_mode: tailscale
  tailscale_address: aurago.tailnet.ts.net
  supervisor_url: wss://manual.example.com/api/remote/ws
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RemoteControl.ConnectionMode != "tailscale" {
		t.Fatalf("connection_mode = %q, want tailscale", cfg.RemoteControl.ConnectionMode)
	}
	if cfg.RemoteControl.TailscaleAddress != "aurago.tailnet.ts.net" {
		t.Fatalf("tailscale_address = %q", cfg.RemoteControl.TailscaleAddress)
	}
	if cfg.RemoteControl.SupervisorURL != "wss://manual.example.com/api/remote/ws" {
		t.Fatalf("supervisor_url = %q", cfg.RemoteControl.SupervisorURL)
	}
}

func TestLoadBrowserAutomationDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BrowserAutomation.Enabled {
		t.Fatal("expected browser_automation.enabled to default to false")
	}
	if cfg.Tools.BrowserAutomation.Enabled {
		t.Fatal("expected tools.browser_automation.enabled to default to false")
	}
	if cfg.BrowserAutomation.Mode != "sidecar" {
		t.Fatalf("mode = %q, want sidecar", cfg.BrowserAutomation.Mode)
	}
	if cfg.BrowserAutomation.URL == "" {
		t.Fatal("expected browser automation URL default to be populated")
	}
	if cfg.BrowserAutomation.ContainerName != "aurago_browser_automation" {
		t.Fatalf("container_name = %q, want aurago_browser_automation", cfg.BrowserAutomation.ContainerName)
	}
	if cfg.BrowserAutomation.Image != "aurago-browser-automation:latest" {
		t.Fatalf("image = %q, want aurago-browser-automation:latest", cfg.BrowserAutomation.Image)
	}
	if !cfg.BrowserAutomation.AutoStart {
		t.Fatal("expected browser_automation.auto_start to default to true")
	}
	if cfg.BrowserAutomation.SessionTTLMinutes != 30 {
		t.Fatalf("session_ttl_minutes = %d, want 30", cfg.BrowserAutomation.SessionTTLMinutes)
	}
	if cfg.BrowserAutomation.MaxSessions != 3 {
		t.Fatalf("max_sessions = %d, want 3", cfg.BrowserAutomation.MaxSessions)
	}
	if cfg.BrowserAutomation.Viewport.Width != 1280 || cfg.BrowserAutomation.Viewport.Height != 720 {
		t.Fatalf("viewport = %+v, want 1280x720", cfg.BrowserAutomation.Viewport)
	}
	if !cfg.BrowserAutomation.Headless {
		t.Fatal("expected browser_automation.headless to default to true")
	}
	if cfg.BrowserAutomation.ReadOnly {
		t.Fatal("expected browser_automation.readonly to default to false")
	}
}

func TestLoadSpaceAgentDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SpaceAgent.Enabled {
		t.Fatal("expected space_agent.enabled to default to false")
	}
	if !cfg.SpaceAgent.AutoStart {
		t.Fatal("expected space_agent.auto_start to default to true")
	}
	if cfg.SpaceAgent.RepoURL != "https://github.com/agent0ai/space-agent" {
		t.Fatalf("repo_url = %q", cfg.SpaceAgent.RepoURL)
	}
	if cfg.SpaceAgent.GitRef != "main" {
		t.Fatalf("git_ref = %q, want main", cfg.SpaceAgent.GitRef)
	}
	if cfg.SpaceAgent.ContainerName != "aurago_space_agent" {
		t.Fatalf("container_name = %q, want aurago_space_agent", cfg.SpaceAgent.ContainerName)
	}
	if cfg.SpaceAgent.Image != "aurago-space-agent:main" {
		t.Fatalf("image = %q, want aurago-space-agent:main", cfg.SpaceAgent.Image)
	}
	if cfg.SpaceAgent.Host != "0.0.0.0" {
		t.Fatalf("host = %q, want 0.0.0.0", cfg.SpaceAgent.Host)
	}
	if cfg.SpaceAgent.Port != 3100 {
		t.Fatalf("port = %d, want 3100", cfg.SpaceAgent.Port)
	}
	if !cfg.SpaceAgent.HTTPSEnabled {
		t.Fatal("expected space_agent.https_enabled to default to true")
	}
	if cfg.SpaceAgent.HTTPSPort != 3101 {
		t.Fatalf("https_port = %d, want 3101", cfg.SpaceAgent.HTTPSPort)
	}
	if cfg.SpaceAgent.AdminUser != "admin" {
		t.Fatalf("admin_user = %q, want admin", cfg.SpaceAgent.AdminUser)
	}
	if cfg.SpaceAgent.PublicURL != "" {
		t.Fatalf("public_url = %q, want empty direct-URL derivation default", cfg.SpaceAgent.PublicURL)
	}
	if cfg.Tailscale.TsNet.SpaceAgentHostname != "aurago-space-agent" {
		t.Fatalf("tailscale.tsnet.space_agent_hostname = %q, want aurago-space-agent", cfg.Tailscale.TsNet.SpaceAgentHostname)
	}
	if cfg.Tailscale.TsNet.ManifestHostname != "aurago-manifest" {
		t.Fatalf("tailscale.tsnet.manifest_hostname = %q, want aurago-manifest", cfg.Tailscale.TsNet.ManifestHostname)
	}
	if cfg.Tailscale.TsNet.ManifestPort != 443 {
		t.Fatalf("tailscale.tsnet.manifest_port = %d, want 443", cfg.Tailscale.TsNet.ManifestPort)
	}
	if !filepath.IsAbs(cfg.SpaceAgent.CustomwarePath) || !strings.Contains(cfg.SpaceAgent.CustomwarePath, filepath.Join("data", "sidecars", "space-agent", "customware")) {
		t.Fatalf("customware_path = %q, want absolute sidecar customware path", cfg.SpaceAgent.CustomwarePath)
	}
	if !filepath.IsAbs(cfg.SpaceAgent.DataPath) || !strings.Contains(cfg.SpaceAgent.DataPath, filepath.Join("data", "sidecars", "space-agent", "data")) {
		t.Fatalf("data_path = %q, want absolute sidecar data path", cfg.SpaceAgent.DataPath)
	}
}

func TestLoadMigratesLegacyManifestTsNetPortDefault(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	body := []byte("server:\n  ui_language: en\ntailscale:\n  tsnet:\n    manifest_port: 8444\n")
	if err := os.WriteFile(configPath, body, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Tailscale.TsNet.ManifestPort != 443 {
		t.Fatalf("tailscale.tsnet.manifest_port = %d, want migrated HTTPS default 443", cfg.Tailscale.TsNet.ManifestPort)
	}
}

func TestLoadManifestDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Manifest.Enabled {
		t.Fatal("expected manifest.enabled to default to false")
	}
	if !cfg.Manifest.AutoStart {
		t.Fatal("expected manifest.auto_start to default to true")
	}
	if cfg.Manifest.Mode != "managed" {
		t.Fatalf("mode = %q, want managed", cfg.Manifest.Mode)
	}
	if cfg.Manifest.URL == "" {
		t.Fatal("expected manifest.url default to be populated")
	}
	if cfg.Manifest.ExternalBaseURL != "https://app.manifest.build/v1" {
		t.Fatalf("external_base_url = %q, want hosted Manifest endpoint", cfg.Manifest.ExternalBaseURL)
	}
	if cfg.Manifest.ContainerName != "aurago_manifest" {
		t.Fatalf("container_name = %q, want aurago_manifest", cfg.Manifest.ContainerName)
	}
	if cfg.Manifest.Image != "manifestdotbuild/manifest:5" {
		t.Fatalf("image = %q, want manifestdotbuild/manifest:5", cfg.Manifest.Image)
	}
	if cfg.Manifest.Host != "127.0.0.1" {
		t.Fatalf("host = %q, want 127.0.0.1", cfg.Manifest.Host)
	}
	if cfg.Manifest.Port != 2099 || cfg.Manifest.HostPort != 2099 {
		t.Fatalf("port/host_port = %d/%d, want 2099/2099", cfg.Manifest.Port, cfg.Manifest.HostPort)
	}
	if cfg.Manifest.NetworkName != "aurago_manifest" {
		t.Fatalf("network_name = %q, want aurago_manifest", cfg.Manifest.NetworkName)
	}
	if cfg.Manifest.PostgresContainerName != "aurago_manifest_postgres" {
		t.Fatalf("postgres_container_name = %q, want aurago_manifest_postgres", cfg.Manifest.PostgresContainerName)
	}
	if cfg.Manifest.PostgresImage != "postgres:15-alpine" {
		t.Fatalf("postgres_image = %q, want postgres:15-alpine", cfg.Manifest.PostgresImage)
	}
	if cfg.Manifest.PostgresUser != "manifest" || cfg.Manifest.PostgresDatabase != "manifest" {
		t.Fatalf("postgres user/db = %q/%q, want manifest/manifest", cfg.Manifest.PostgresUser, cfg.Manifest.PostgresDatabase)
	}
	if cfg.Manifest.PostgresVolume != "aurago_manifest_pgdata" {
		t.Fatalf("postgres_volume = %q, want aurago_manifest_pgdata", cfg.Manifest.PostgresVolume)
	}
	if cfg.Manifest.Routing.Enabled {
		t.Fatal("manifest.routing.enabled should default to false")
	}
	if cfg.Manifest.Routing.SpecificityMode != "off" {
		t.Fatalf("manifest.routing.specificity_mode = %q, want off", cfg.Manifest.Routing.SpecificityMode)
	}
	if cfg.Manifest.Routing.Specificity != "" {
		t.Fatalf("manifest.routing.specificity = %q, want empty", cfg.Manifest.Routing.Specificity)
	}
	if len(cfg.Manifest.Routing.Headers) != 0 {
		t.Fatalf("manifest.routing.headers = %#v, want empty", cfg.Manifest.Routing.Headers)
	}
}

func TestLoadOmniRouteDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.OmniRoute.Enabled {
		t.Fatal("expected omniroute.enabled to default to false")
	}
	if !cfg.OmniRoute.AutoStart {
		t.Fatal("expected omniroute.auto_start to default to true")
	}
	if cfg.OmniRoute.Mode != "managed" {
		t.Fatalf("mode = %q, want managed", cfg.OmniRoute.Mode)
	}
	if cfg.OmniRoute.URL == "" {
		t.Fatal("expected omniroute.url default to be populated")
	}
	if cfg.OmniRoute.ExternalBaseURL != "http://127.0.0.1:20128/v1" {
		t.Fatalf("external_base_url = %q, want local OmniRoute /v1 endpoint", cfg.OmniRoute.ExternalBaseURL)
	}
	if cfg.OmniRoute.ContainerName != "aurago_omniroute" {
		t.Fatalf("container_name = %q, want aurago_omniroute", cfg.OmniRoute.ContainerName)
	}
	if cfg.OmniRoute.Image != "diegosouzapw/omniroute:3.8.39" {
		t.Fatalf("image = %q, want pinned OmniRoute image", cfg.OmniRoute.Image)
	}
	if cfg.OmniRoute.Host != "127.0.0.1" {
		t.Fatalf("host = %q, want 127.0.0.1", cfg.OmniRoute.Host)
	}
	if cfg.OmniRoute.Port != 20128 || cfg.OmniRoute.HostPort != 20128 {
		t.Fatalf("port/host_port = %d/%d, want 20128/20128", cfg.OmniRoute.Port, cfg.OmniRoute.HostPort)
	}
	if cfg.OmniRoute.NetworkName != "aurago_omniroute" {
		t.Fatalf("network_name = %q, want aurago_omniroute", cfg.OmniRoute.NetworkName)
	}
	if cfg.OmniRoute.DataVolume != "aurago_omniroute_data" {
		t.Fatalf("data_volume = %q, want aurago_omniroute_data", cfg.OmniRoute.DataVolume)
	}
	if cfg.OmniRoute.HealthPath != "/api/monitoring/health" {
		t.Fatalf("health_path = %q, want /api/monitoring/health", cfg.OmniRoute.HealthPath)
	}
	if cfg.OmniRoute.MemoryMB != 512 {
		t.Fatalf("memory_mb = %d, want 512", cfg.OmniRoute.MemoryMB)
	}
}

func TestLoadManifestRoutingExplicitValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
manifest:
  enabled: true
  routing:
    enabled: true
    specificity_mode: fixed
    specificity: coding
    headers:
      x-aurago-task: coding
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Manifest.Routing.Enabled {
		t.Fatal("manifest.routing.enabled = false, want true")
	}
	if cfg.Manifest.Routing.SpecificityMode != "fixed" {
		t.Fatalf("manifest.routing.specificity_mode = %q, want fixed", cfg.Manifest.Routing.SpecificityMode)
	}
	if cfg.Manifest.Routing.Specificity != "coding" {
		t.Fatalf("manifest.routing.specificity = %q, want coding", cfg.Manifest.Routing.Specificity)
	}
	if got := cfg.Manifest.Routing.Headers["x-aurago-task"]; got != "coding" {
		t.Fatalf("manifest.routing.headers[x-aurago-task] = %q, want coding", got)
	}
}

func TestLoadManifestRoutingNormalizesInvalidModeAndSpecificity(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
manifest:
  routing:
    enabled: true
    specificity_mode: surprise
    specificity: root_shell
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Manifest.Routing.SpecificityMode != "off" {
		t.Fatalf("manifest.routing.specificity_mode = %q, want off", cfg.Manifest.Routing.SpecificityMode)
	}
	if cfg.Manifest.Routing.Specificity != "" {
		t.Fatalf("manifest.routing.specificity = %q, want empty for invalid category", cfg.Manifest.Routing.Specificity)
	}
}

func TestLoadDograhDefaultsUseOfficialGHCRImages(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Dograh.APIImage != "ghcr.io/dograh-hq/dograh-api:latest" {
		t.Fatalf("dograh.api_image = %q, want official GHCR API image", cfg.Dograh.APIImage)
	}
	if cfg.Dograh.UIImage != "ghcr.io/dograh-hq/dograh-ui:latest" {
		t.Fatalf("dograh.ui_image = %q, want official GHCR UI image", cfg.Dograh.UIImage)
	}
}

func TestLoadDograhMigratesLegacyDockerHubDefaultImages(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	raw := []byte(`server:
  ui_language: en
dograh:
  enabled: true
  api_image: dograhai/dograh-api:latest
  ui_image: dograhai/dograh-ui:latest
`)
	if err := os.WriteFile(configPath, raw, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Dograh.APIImage != "ghcr.io/dograh-hq/dograh-api:latest" {
		t.Fatalf("dograh.api_image = %q, want migrated GHCR API image", cfg.Dograh.APIImage)
	}
	if cfg.Dograh.UIImage != "ghcr.io/dograh-hq/dograh-ui:latest" {
		t.Fatalf("dograh.ui_image = %q, want migrated GHCR UI image", cfg.Dograh.UIImage)
	}
}

func TestLoadDograhKeepsExplicitPinnedImages(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	raw := []byte(`server:
  ui_language: en
dograh:
  enabled: true
  api_image: dograhai/dograh-api:1.30.1
  ui_image: registry.example/dograh-ui:custom
`)
	if err := os.WriteFile(configPath, raw, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Dograh.APIImage != "dograhai/dograh-api:1.30.1" {
		t.Fatalf("dograh.api_image = %q, want explicit pinned API image preserved", cfg.Dograh.APIImage)
	}
	if cfg.Dograh.UIImage != "registry.example/dograh-ui:custom" {
		t.Fatalf("dograh.ui_image = %q, want explicit custom UI image preserved", cfg.Dograh.UIImage)
	}
}

func TestLoadSpaceAgentMigratesLegacyDefaultPortAwayFromGotenberg(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	raw := []byte(`server:
  ui_language: en
space_agent:
  enabled: true
  port: 3000
  public_url: "http://127.0.0.1:3000"
`)
	if err := os.WriteFile(configPath, raw, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SpaceAgent.Port != 3100 {
		t.Fatalf("port = %d, want migrated default 3100", cfg.SpaceAgent.Port)
	}
	if cfg.SpaceAgent.PublicURL != "" {
		t.Fatalf("public_url = %q, want empty direct-URL derivation default", cfg.SpaceAgent.PublicURL)
	}
}

func TestLoadSpaceAgentMigratesLegacyDefaultURLWhenPortOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	raw := []byte(`server:
  ui_language: en
space_agent:
  enabled: true
  public_url: "http://space-agent:3000"
`)
	if err := os.WriteFile(configPath, raw, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SpaceAgent.Port != 3100 {
		t.Fatalf("port = %d, want migrated default 3100", cfg.SpaceAgent.Port)
	}
	if cfg.SpaceAgent.PublicURL != "" {
		t.Fatalf("public_url = %q, want empty direct-URL derivation default", cfg.SpaceAgent.PublicURL)
	}
}

func TestLoadSpaceAgentKeepsExplicitCustomPort(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	raw := []byte(`server:
  ui_language: en
space_agent:
  enabled: true
  port: 3000
  public_url: "http://space.example:3000"
`)
	if err := os.WriteFile(configPath, raw, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SpaceAgent.Port != 3000 {
		t.Fatalf("port = %d, want explicit custom 3000", cfg.SpaceAgent.Port)
	}
	if cfg.SpaceAgent.PublicURL != "http://space.example:3000" {
		t.Fatalf("public_url = %q, want explicit custom URL", cfg.SpaceAgent.PublicURL)
	}
}

func TestLoadMediaConversionDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Tools.MediaConversion.Enabled {
		t.Fatal("expected tools.media_conversion.enabled to default to false")
	}
	if cfg.Tools.MediaConversion.ReadOnly {
		t.Fatal("expected tools.media_conversion.readonly to default to false")
	}
	if cfg.Tools.MediaConversion.TimeoutSeconds != 120 {
		t.Fatalf("timeout_seconds = %d, want 120", cfg.Tools.MediaConversion.TimeoutSeconds)
	}
	if cfg.Tools.MediaConversion.FFmpegPath != "" {
		t.Fatalf("ffmpeg_path = %q, want empty default", cfg.Tools.MediaConversion.FFmpegPath)
	}
	if cfg.Tools.MediaConversion.ImageMagickPath != "" {
		t.Fatalf("imagemagick_path = %q, want empty default", cfg.Tools.MediaConversion.ImageMagickPath)
	}
}

func TestLoadWebScraperCanonicalEnabledWinsOverLegacyAgentFlag(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	raw := []byte(`
agent:
  allow_web_scraper: false
tools:
  web_scraper:
    enabled: true
    summary_mode: false
`)
	if err := os.WriteFile(configPath, raw, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Tools.WebScraper.Enabled {
		t.Fatal("expected tools.web_scraper.enabled=true to win over deprecated agent.allow_web_scraper=false")
	}
}

func TestLoadWebScraperMigratesLegacyAgentFlagWhenCanonicalMissing(t *testing.T) {
	tests := []struct {
		name   string
		legacy bool
		want   bool
	}{
		{name: "legacy enabled", legacy: true, want: true},
		{name: "legacy disabled", legacy: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			raw := []byte(`
agent:
  allow_web_scraper: ` + map[bool]string{true: "true", false: "false"}[tt.legacy] + `
`)
			if err := os.WriteFile(configPath, raw, 0o644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.Tools.WebScraper.Enabled != tt.want {
				t.Fatalf("tools.web_scraper.enabled = %v, want %v", cfg.Tools.WebScraper.Enabled, tt.want)
			}
		})
	}
}

func TestLoadVideoDownloadDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Tools.VideoDownload.Enabled {
		t.Fatal("expected tools.video_download.enabled to default to true so search/info are available")
	}
	if cfg.Tools.VideoDownload.AllowDownload {
		t.Fatal("expected tools.video_download.allow_download to default to false")
	}
	if cfg.Tools.VideoDownload.AllowTranscribe {
		t.Fatal("expected tools.video_download.allow_transcribe to default to false")
	}
	if cfg.Tools.VideoDownload.Mode != "docker" {
		t.Fatalf("mode = %q, want docker", cfg.Tools.VideoDownload.Mode)
	}
	if cfg.Tools.VideoDownload.DownloadDir != "data/downloads" {
		t.Fatalf("download_dir = %q, want data/downloads", cfg.Tools.VideoDownload.DownloadDir)
	}
	if cfg.Tools.VideoDownload.ContainerImage != "ghcr.io/jauderho/yt-dlp:latest" {
		t.Fatalf("container_image = %q", cfg.Tools.VideoDownload.ContainerImage)
	}
	if !cfg.Tools.VideoDownload.AutoPull {
		t.Fatal("expected auto_pull to default to true")
	}
	if cfg.Tools.VideoDownload.TimeoutSeconds != 300 {
		t.Fatalf("timeout_seconds = %d, want 300", cfg.Tools.VideoDownload.TimeoutSeconds)
	}
}

func TestLoadSendYouTubeVideoDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Tools.SendYouTubeVideo.Enabled {
		t.Fatal("expected tools.send_youtube_video.enabled to default to true")
	}
}

func TestLoadCodeStudioDefaultsToPublishedImage(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.VirtualDesktop.CodeStudio.Image != "ghcr.io/antibyte/aurago-code-studio:latest" {
		t.Fatalf("code studio image = %q, want published image", cfg.VirtualDesktop.CodeStudio.Image)
	}
}

func TestLoadMigratesLegacyCodeStudioDefaultImage(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  ui_language: en
virtual_desktop:
  code_studio:
    image: aurago/code-studio:latest
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.VirtualDesktop.CodeStudio.Image != "ghcr.io/antibyte/aurago-code-studio:latest" {
		t.Fatalf("legacy code studio image = %q, want published image", cfg.VirtualDesktop.CodeStudio.Image)
	}
}

func TestLoadOpenSCADDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got := cfg.VirtualDesktop.OpenSCAD
	if !got.Enabled {
		t.Fatal("OpenSCAD should default enabled")
	}
	if got.Image != "openscad/openscad:latest" {
		t.Fatalf("OpenSCAD image = %q, want openscad/openscad:latest", got.Image)
	}
	if got.AutoStart {
		t.Fatal("OpenSCAD auto_start should default false")
	}
	if got.AutoStopMinutes != 20 {
		t.Fatalf("OpenSCAD auto_stop_minutes = %d, want 20", got.AutoStopMinutes)
	}
	if got.MaxMemoryMB != 2048 || got.MaxCPUCores != 2 || got.MaxConcurrentJobs != 1 {
		t.Fatalf("OpenSCAD resource defaults = %+v", got)
	}
	if got.MaxSourceKB != 512 || got.MaxOutputMB != 100 {
		t.Fatalf("OpenSCAD size limit defaults = %+v", got)
	}
	if got.RenderTimeoutSeconds != 120 || got.MaxRenderTimeoutSeconds != 600 || got.JobRetentionDays != 7 {
		t.Fatalf("OpenSCAD timeout defaults = %+v", got)
	}
	if got.GeometryBackend != "auto" {
		t.Fatalf("OpenSCAD geometry_backend = %q, want auto", got.GeometryBackend)
	}
	if len(got.DefaultExports) != 2 || got.DefaultExports[0] != "png" || got.DefaultExports[1] != "stl" {
		t.Fatalf("OpenSCAD default exports = %#v, want [png stl]", got.DefaultExports)
	}
}

func TestLoadMigratesLegacyBrowserAutomationDockerURLOutsideDocker(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  ui_language: en
browser_automation:
  url: http://browser-automation:7331
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BrowserAutomation.URL != "http://127.0.0.1:7331" {
		t.Fatalf("url = %q, want http://127.0.0.1:7331", cfg.BrowserAutomation.URL)
	}
}

func TestDefaultSidecarURL(t *testing.T) {
	tests := []struct {
		name            string
		runningInDocker bool
		service         string
		port            int
		want            string
	}{
		{
			name:            "docker runtime uses service hostname",
			runningInDocker: true,
			service:         "browser-automation",
			port:            7331,
			want:            "http://browser-automation:7331",
		},
		{
			name:            "host runtime uses loopback",
			runningInDocker: false,
			service:         "browser-automation",
			port:            7331,
			want:            "http://127.0.0.1:7331",
		},
		{
			name:            "other services share same rule",
			runningInDocker: false,
			service:         "gotenberg",
			port:            3000,
			want:            "http://127.0.0.1:3000",
		},
		{
			name:            "manifest inside docker uses service DNS",
			runningInDocker: true,
			service:         "manifest",
			port:            2099,
			want:            "http://manifest:2099",
		},
		{
			name:            "manifest on host uses loopback",
			runningInDocker: false,
			service:         "manifest",
			port:            2099,
			want:            "http://127.0.0.1:2099",
		},
	}

	for _, tt := range tests {
		if got := defaultSidecarURL(tt.runningInDocker, tt.service, tt.port); got != tt.want {
			t.Fatalf("%s: defaultSidecarURL(%v, %q, %d) = %q, want %q", tt.name, tt.runningInDocker, tt.service, tt.port, got, tt.want)
		}
	}
}

func TestNormalizeLegacySidecarURL(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		runningInDocker bool
		service         string
		port            int
		want            string
	}{
		{
			name:            "non-docker rewrites legacy service host",
			raw:             "http://browser-automation:7331",
			runningInDocker: false,
			service:         "browser-automation",
			port:            7331,
			want:            "http://127.0.0.1:7331",
		},
		{
			name:            "docker keeps service host",
			raw:             "http://browser-automation:7331",
			runningInDocker: true,
			service:         "browser-automation",
			port:            7331,
			want:            "http://browser-automation:7331",
		},
		{
			name:            "non-matching host stays unchanged",
			raw:             "http://automation.internal:7331",
			runningInDocker: false,
			service:         "browser-automation",
			port:            7331,
			want:            "http://automation.internal:7331",
		},
	}

	for _, tt := range tests {
		if got := NormalizeLegacySidecarURL(tt.raw, tt.runningInDocker, tt.service, tt.port); got != tt.want {
			t.Fatalf("%s: NormalizeLegacySidecarURL(%q, %v, %q, %d) = %q, want %q", tt.name, tt.raw, tt.runningInDocker, tt.service, tt.port, got, tt.want)
		}
	}
}

func TestLoadAdaptiveSystemPromptTokenBudgetDefaultsToTrue(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
agent:
  system_prompt_token_budget: 12000
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !cfg.Agent.AdaptiveSystemPromptTokenBudget {
		t.Fatal("expected adaptive_system_prompt_token_budget to default to true")
	}
}

func TestLoadAdaptiveSystemPromptTokenBudgetExplicitDisablePreserved(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
agent:
  system_prompt_token_budget: 12000
  adaptive_system_prompt_token_budget: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Agent.AdaptiveSystemPromptTokenBudget {
		t.Fatal("expected explicit adaptive_system_prompt_token_budget=false to be preserved")
	}
}

func TestLoadUptimeKumaDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.UptimeKuma.RequestTimeout != 15 {
		t.Fatalf("request_timeout = %d, want 15", cfg.UptimeKuma.RequestTimeout)
	}
	if cfg.UptimeKuma.PollIntervalSeconds != 30 {
		t.Fatalf("poll_interval_seconds = %d, want 30", cfg.UptimeKuma.PollIntervalSeconds)
	}
	if cfg.UptimeKuma.RelayToAgent {
		t.Fatal("expected relay_to_agent to default to false")
	}
	if cfg.UptimeKuma.RelayInstruction != "" {
		t.Fatalf("relay_instruction = %q, want empty string", cfg.UptimeKuma.RelayInstruction)
	}
}

func TestApplyVaultSecretsLoadsUptimeKumaAPIKey(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"uptime_kuma_api_key": "uk2_secret_from_vault",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.UptimeKuma.APIKey != "uk2_secret_from_vault" {
		t.Fatalf("APIKey = %q, want uptime kuma secret", cfg.UptimeKuma.APIKey)
	}
}

func TestConfigSaveOmitsUptimeKumaAPIKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	cfg := &Config{}
	cfg.UptimeKuma.Enabled = true
	cfg.UptimeKuma.BaseURL = "https://uptime.local"
	cfg.UptimeKuma.APIKey = "uk2_should_not_be_serialized"
	cfg.UptimeKuma.RelayInstruction = "Restart the service and inform me if that fails."

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), "uk2_should_not_be_serialized") || strings.Contains(string(raw), "api_key:") {
		t.Fatalf("expected uptime kuma API key to stay out of YAML, got:\n%s", string(raw))
	}
	if !strings.Contains(string(raw), "relay_instruction:") {
		t.Fatalf("expected relay_instruction to be serialized, got:\n%s", string(raw))
	}
}

func TestLoadAgentMailDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AgentMail.Enabled {
		t.Fatal("expected agentmail.enabled to default to false")
	}
	if cfg.AgentMail.BaseURL != "https://api.agentmail.to" {
		t.Fatalf("base_url = %q", cfg.AgentMail.BaseURL)
	}
	if cfg.AgentMail.WebSocketURL != "wss://ws.agentmail.to/v0" {
		t.Fatalf("websocket_url = %q", cfg.AgentMail.WebSocketURL)
	}
	if cfg.AgentMail.PollIntervalSeconds != 120 {
		t.Fatalf("poll_interval_seconds = %d, want 120", cfg.AgentMail.PollIntervalSeconds)
	}
	if cfg.AgentMail.MaxAttachmentMB != 10 {
		t.Fatalf("max_attachment_mb = %d, want 10", cfg.AgentMail.MaxAttachmentMB)
	}
	if !cfg.AgentMail.UseWebSocket {
		t.Fatal("expected use_websocket to default to true")
	}
	if cfg.AgentMail.RelayCheatsheetID != "" {
		t.Fatalf("relay_cheatsheet_id = %q, want empty default", cfg.AgentMail.RelayCheatsheetID)
	}
}

func TestLoadEmailRelayCheatsheetDefault(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Email.RelayCheatsheetID != "" {
		t.Fatalf("relay_cheatsheet_id = %q, want empty default", cfg.Email.RelayCheatsheetID)
	}
}

func TestLoadRulesDefaultsEnabled(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Rules.Enabled {
		t.Fatal("expected rules.enabled to default to true")
	}
}

func TestConfigSavePersistsEmailRelayCheatsheetID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\nemail:\n  enabled: false\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	cfg := &Config{}
	cfg.Email.RelayCheatsheetID = "cs-email"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if !strings.Contains(string(raw), "relay_cheatsheet_id: cs-email") {
		t.Fatalf("expected email relay_cheatsheet_id to be serialized, got:\n%s", string(raw))
	}
}

func TestApplyVaultSecretsLoadsAgentMailAPIKey(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"agentmail_api_key": "am_secret_from_vault",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.AgentMail.APIKey != "am_secret_from_vault" {
		t.Fatalf("APIKey = %q, want agentmail secret", cfg.AgentMail.APIKey)
	}
}

func TestConfigSaveOmitsAgentMailAPIKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	cfg := &Config{}
	cfg.AgentMail.Enabled = true
	cfg.AgentMail.APIKey = "am_should_not_be_serialized"
	cfg.AgentMail.InboxID = "inbox-1"
	cfg.AgentMail.RelayCheatsheetID = "cs-mail"
	cfg.AgentMail.BaseURL = "https://api.agentmail.to"
	cfg.AgentMail.WebSocketURL = "wss://ws.agentmail.to/v0"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	got := string(raw)
	if strings.Contains(got, "am_should_not_be_serialized") || strings.Contains(got, "api_key:") {
		t.Fatalf("expected AgentMail API key to stay out of YAML, got:\n%s", got)
	}
	if !strings.Contains(got, "agentmail:") || !strings.Contains(got, "inbox_id: inbox-1") || !strings.Contains(got, "relay_cheatsheet_id: cs-mail") {
		t.Fatalf("expected non-secret AgentMail settings to be serialized, got:\n%s", got)
	}
}

func TestLoadGrafanaDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Grafana.RequestTimeout != 15 {
		t.Fatalf("request_timeout = %d, want 15", cfg.Grafana.RequestTimeout)
	}
	if !cfg.Grafana.ReadOnly {
		t.Fatal("expected readonly to default to true")
	}
}

func TestApplyVaultSecretsLoadsGrafanaAPIKey(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"grafana_api_key": "gf_secret_from_vault",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.Grafana.APIKey != "gf_secret_from_vault" {
		t.Fatalf("APIKey = %q, want grafana secret", cfg.Grafana.APIKey)
	}
}

func TestConfigSaveOmitsGrafanaAPIKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	cfg := &Config{}
	cfg.Grafana.Enabled = true
	cfg.Grafana.BaseURL = "https://grafana.local"
	cfg.Grafana.ReadOnly = true
	cfg.Grafana.APIKey = "gf_should_not_be_serialized"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), "gf_should_not_be_serialized") || strings.Contains(string(raw), "api_key:") {
		t.Fatalf("expected grafana API key to stay out of YAML, got:\n%s", string(raw))
	}
	if !strings.Contains(string(raw), "grafana:") || !strings.Contains(string(raw), "base_url: https://grafana.local") {
		t.Fatalf("expected grafana settings to be serialized, got:\n%s", string(raw))
	}
}

func TestLoadEvomapDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Evomap.Enabled {
		t.Fatal("expected evomap to be disabled by default")
	}
	if !cfg.Evomap.ReadOnly {
		t.Fatal("expected evomap readonly to default to true")
	}
	if cfg.Evomap.BaseURL != "https://evomap.ai" {
		t.Fatalf("base_url = %q, want https://evomap.ai", cfg.Evomap.BaseURL)
	}
	if cfg.Evomap.RequestTimeoutSeconds != 30 {
		t.Fatalf("request_timeout_seconds = %d, want 30", cfg.Evomap.RequestTimeoutSeconds)
	}
	if cfg.Evomap.MaxResultBytes != 262144 {
		t.Fatalf("max_result_bytes = %d, want 262144", cfg.Evomap.MaxResultBytes)
	}
	if cfg.Evomap.KGEnabled || cfg.Evomap.AllowPublish || cfg.Evomap.AllowReport || cfg.Evomap.AllowBounties {
		t.Fatalf("expected paid/mutating evomap gates to default false: %+v", cfg.Evomap)
	}
}

func TestApplyVaultSecretsLoadsEvomapSecrets(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"evomap_node_secret": "node-secret-from-vault",
		"evomap_api_key":     "kg-secret-from-vault",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.Evomap.NodeSecret != "node-secret-from-vault" {
		t.Fatalf("NodeSecret = %q, want node secret", cfg.Evomap.NodeSecret)
	}
	if cfg.Evomap.APIKey != "kg-secret-from-vault" {
		t.Fatalf("APIKey = %q, want KG API key", cfg.Evomap.APIKey)
	}
}

func TestConfigSaveOmitsEvomapSecrets(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	cfg := &Config{}
	cfg.Evomap.Enabled = true
	cfg.Evomap.ReadOnly = true
	cfg.Evomap.BaseURL = "https://evomap.ai"
	cfg.Evomap.NodeID = "node-123"
	cfg.Evomap.NodeSecret = "node_secret_should_not_be_serialized"
	cfg.Evomap.APIKey = "kg_key_should_not_be_serialized"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	got := string(raw)
	if strings.Contains(got, "node_secret_should_not_be_serialized") || strings.Contains(got, "kg_key_should_not_be_serialized") || strings.Contains(got, "node_secret:") || strings.Contains(got, "api_key:") {
		t.Fatalf("expected EvoMap secrets to stay out of YAML, got:\n%s", got)
	}
	if !strings.Contains(got, "evomap:") || !strings.Contains(got, "node_id: node-123") {
		t.Fatalf("expected non-secret EvoMap settings to be serialized, got:\n%s", got)
	}
}

func TestApplyVaultSecretsLoadsSpaceAgentSecrets(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"space_agent_admin_password": "admin-secret",
		"space_agent_bridge_token":   "bridge-secret",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.SpaceAgent.AdminPassword != "admin-secret" {
		t.Fatalf("AdminPassword = %q, want vault secret", cfg.SpaceAgent.AdminPassword)
	}
	if cfg.SpaceAgent.BridgeToken != "bridge-secret" {
		t.Fatalf("BridgeToken = %q, want vault secret", cfg.SpaceAgent.BridgeToken)
	}
}

func TestApplyVaultSecretsLoadsManifestSecrets(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"manifest_api_key":            "mnfst_from_vault",
		"manifest_postgres_password":  "pg-from-vault",
		"manifest_better_auth_secret": "better-auth-from-vault",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.Manifest.APIKey != "mnfst_from_vault" {
		t.Fatalf("APIKey = %q, want manifest API key", cfg.Manifest.APIKey)
	}
	if cfg.Manifest.PostgresPassword != "pg-from-vault" {
		t.Fatalf("PostgresPassword = %q, want Postgres password", cfg.Manifest.PostgresPassword)
	}
	if cfg.Manifest.BetterAuthSecret != "better-auth-from-vault" {
		t.Fatalf("BetterAuthSecret = %q, want Better Auth secret", cfg.Manifest.BetterAuthSecret)
	}
}

func TestApplyVaultSecretsLoadsOmniRouteSecrets(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"omniroute_api_key":          "omni-api-key",
		"omniroute_initial_password": "initial-admin-password",
		"omniroute_jwt_secret":       "jwt-secret",
		"omniroute_api_key_secret":   "api-key-secret",
		"omniroute_ws_bridge_secret": "ws-bridge-secret",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.OmniRoute.APIKey != "omni-api-key" {
		t.Fatalf("APIKey = %q, want OmniRoute API key", cfg.OmniRoute.APIKey)
	}
	if cfg.OmniRoute.InitialPassword != "initial-admin-password" {
		t.Fatalf("InitialPassword = %q, want initial admin password", cfg.OmniRoute.InitialPassword)
	}
	if cfg.OmniRoute.JWTSecret != "jwt-secret" {
		t.Fatalf("JWTSecret = %q, want JWT secret", cfg.OmniRoute.JWTSecret)
	}
	if cfg.OmniRoute.APIKeySecret != "api-key-secret" {
		t.Fatalf("APIKeySecret = %q, want API key secret", cfg.OmniRoute.APIKeySecret)
	}
	if cfg.OmniRoute.WSBridgeSecret != "ws-bridge-secret" {
		t.Fatalf("WSBridgeSecret = %q, want websocket bridge secret", cfg.OmniRoute.WSBridgeSecret)
	}
}

func TestApplyVaultSecretsLoadsComposioAPIKey(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"composio_api_key": "cmp-secret",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.Composio.APIKey != "cmp-secret" {
		t.Fatalf("composio api key = %q, want cmp-secret", cfg.Composio.APIKey)
	}
}

func TestApplyVaultSecretsLoadsLegacyComposioDottedAPIKey(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"composio.api_key": "cmp-legacy-secret",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.Composio.APIKey != "cmp-legacy-secret" {
		t.Fatalf("composio api key = %q, want legacy secret", cfg.Composio.APIKey)
	}
}

func TestLoadAppliesComposioDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("providers: []\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Composio.BaseURL != "https://backend.composio.dev/api/v3.1" {
		t.Fatalf("Composio.BaseURL = %q", cfg.Composio.BaseURL)
	}
	if cfg.Composio.UserID != "aurago-default" {
		t.Fatalf("Composio.UserID = %q", cfg.Composio.UserID)
	}
	if !cfg.Composio.ReadOnly {
		t.Fatal("expected Composio.ReadOnly default true")
	}
	if cfg.Composio.AllowDestructive {
		t.Fatal("expected Composio.AllowDestructive default false")
	}
	if cfg.Composio.AllowNaturalLanguageInput {
		t.Fatal("expected Composio.AllowNaturalLanguageInput default false")
	}
	if cfg.Composio.RequestTimeoutSeconds <= 0 || cfg.Composio.CacheTTLSeconds <= 0 || cfg.Composio.MaxResultBytes <= 0 {
		t.Fatalf("unexpected Composio timeout/cache/result defaults: %+v", cfg.Composio)
	}
}

func TestManifestProviderManagedDefaultBaseURL(t *testing.T) {
	cfg := &Config{}
	cfg.Manifest.Mode = "managed"
	cfg.Manifest.URL = "http://manifest:2099"
	cfg.Manifest.APIKey = "mnfst_from_manifest_section"
	cfg.Providers = []ProviderEntry{{
		ID:    "manifest",
		Type:  "manifest",
		Model: "manifest/auto",
	}}
	cfg.LLM.Provider = "manifest"

	cfg.ResolveProviders()

	if cfg.Providers[0].BaseURL != "http://manifest:2099/v1" {
		t.Fatalf("provider base_url = %q, want managed Manifest /v1 URL", cfg.Providers[0].BaseURL)
	}
	if cfg.LLM.BaseURL != "http://manifest:2099/v1" {
		t.Fatalf("LLM base_url = %q, want managed Manifest /v1 URL", cfg.LLM.BaseURL)
	}
	if cfg.LLM.ProviderType != "manifest" {
		t.Fatalf("ProviderType = %q, want manifest", cfg.LLM.ProviderType)
	}
	if cfg.LLM.APIKey != "mnfst_from_manifest_section" {
		t.Fatalf("LLM APIKey = %q, want Manifest section API key", cfg.LLM.APIKey)
	}
}

func TestManifestProviderManagedIgnoresBrowserBaseURL(t *testing.T) {
	cfg := &Config{}
	cfg.Manifest.Mode = "managed"
	cfg.Manifest.URL = "http://manifest:2099"
	cfg.Manifest.APIKey = "mnfst_from_manifest_section"
	cfg.Providers = []ProviderEntry{{
		ID:      "manifest",
		Type:    "manifest",
		BaseURL: "https://aurago-manifest.taild1480.ts.net/v1",
		Model:   "manifest/auto",
	}}
	cfg.LLM.Provider = "manifest"

	cfg.ResolveProviders()

	if cfg.Providers[0].BaseURL != "http://manifest:2099/v1" {
		t.Fatalf("provider base_url = %q, want managed internal Manifest /v1 URL", cfg.Providers[0].BaseURL)
	}
	if cfg.LLM.BaseURL != "http://manifest:2099/v1" {
		t.Fatalf("LLM base_url = %q, want managed internal Manifest /v1 URL", cfg.LLM.BaseURL)
	}
}

func TestManifestProviderManagedIgnoresBrowserManifestURL(t *testing.T) {
	cfg := &Config{}
	cfg.Runtime.IsDocker = true
	cfg.Manifest.Mode = "managed"
	cfg.Manifest.URL = "https://aurago-manifest.taild1480.ts.net"
	cfg.Manifest.APIKey = "mnfst_from_manifest_section"
	cfg.Providers = []ProviderEntry{{
		ID:    "manifest",
		Type:  "manifest",
		Model: "manifest/auto",
	}}
	cfg.LLM.Provider = "manifest"

	cfg.ResolveProviders()

	if cfg.LLM.BaseURL != "http://manifest:2099/v1" {
		t.Fatalf("LLM base_url = %q, want managed Docker-internal Manifest /v1 URL", cfg.LLM.BaseURL)
	}
}

func TestManifestProviderExternalDefaultBaseURL(t *testing.T) {
	cfg := &Config{}
	cfg.Manifest.Mode = "external"
	cfg.Manifest.ExternalBaseURL = "https://manifest.example.test/v1"
	cfg.Providers = []ProviderEntry{{
		ID:    "manifest",
		Type:  "manifest",
		Model: "manifest/auto",
	}}
	cfg.LLM.Provider = "manifest"

	cfg.ResolveProviders()

	if cfg.LLM.BaseURL != "https://manifest.example.test/v1" {
		t.Fatalf("LLM base_url = %q, want external Manifest /v1 URL", cfg.LLM.BaseURL)
	}
}

func TestManifestProviderSpecificAPIKeyOverridesManifestSectionKey(t *testing.T) {
	cfg := &Config{}
	cfg.Manifest.Mode = "managed"
	cfg.Manifest.URL = "http://manifest:2099"
	cfg.Manifest.APIKey = "mnfst_shared"
	cfg.Providers = []ProviderEntry{{
		ID:     "manifest",
		Type:   "manifest",
		Model:  "manifest/auto",
		APIKey: "mnfst_provider_specific",
	}}
	cfg.LLM.Provider = "manifest"

	cfg.ResolveProviders()

	if cfg.LLM.APIKey != "mnfst_provider_specific" {
		t.Fatalf("LLM APIKey = %q, want provider-specific Manifest key", cfg.LLM.APIKey)
	}
}

func TestOmniRouteProviderManagedDefaultBaseURL(t *testing.T) {
	cfg := &Config{}
	cfg.OmniRoute.Mode = "managed"
	cfg.OmniRoute.URL = "http://omniroute:20128"
	cfg.OmniRoute.APIKey = "omni_from_section"
	cfg.Providers = []ProviderEntry{{
		ID:    "omniroute",
		Type:  "omniroute",
		Model: "auto",
	}}
	cfg.LLM.Provider = "omniroute"

	cfg.ResolveProviders()

	if cfg.Providers[0].BaseURL != "http://omniroute:20128/v1" {
		t.Fatalf("provider base_url = %q, want managed OmniRoute /v1 URL", cfg.Providers[0].BaseURL)
	}
	if cfg.LLM.BaseURL != "http://omniroute:20128/v1" {
		t.Fatalf("LLM base_url = %q, want managed OmniRoute /v1 URL", cfg.LLM.BaseURL)
	}
	if cfg.LLM.ProviderType != "omniroute" {
		t.Fatalf("ProviderType = %q, want omniroute", cfg.LLM.ProviderType)
	}
	if cfg.LLM.APIKey != "omni_from_section" {
		t.Fatalf("LLM APIKey = %q, want OmniRoute section API key", cfg.LLM.APIKey)
	}
}

func TestOmniRouteProviderManagedIgnoresBrowserBaseURL(t *testing.T) {
	cfg := &Config{}
	cfg.OmniRoute.Mode = "managed"
	cfg.OmniRoute.URL = "http://omniroute:20128"
	cfg.OmniRoute.APIKey = "omni_from_section"
	cfg.Providers = []ProviderEntry{{
		ID:      "omniroute",
		Type:    "omniroute",
		BaseURL: "http://192.168.6.43:20128/v1",
		Model:   "auto",
	}}
	cfg.LLM.Provider = "omniroute"

	cfg.ResolveProviders()

	if cfg.LLM.BaseURL != "http://omniroute:20128/v1" {
		t.Fatalf("LLM base_url = %q, want managed internal OmniRoute /v1 URL", cfg.LLM.BaseURL)
	}
}

func TestOmniRouteProviderExternalDefaultBaseURL(t *testing.T) {
	cfg := &Config{}
	cfg.OmniRoute.Mode = "external"
	cfg.OmniRoute.ExternalBaseURL = "https://omniroute.example.test/v1"
	cfg.Providers = []ProviderEntry{{
		ID:    "omniroute",
		Type:  "omniroute",
		Model: "auto",
	}}
	cfg.LLM.Provider = "omniroute"

	cfg.ResolveProviders()

	if cfg.LLM.BaseURL != "https://omniroute.example.test/v1" {
		t.Fatalf("LLM base_url = %q, want external OmniRoute /v1 URL", cfg.LLM.BaseURL)
	}
}

func TestOmniRouteProviderExternalIgnoresProviderBaseURL(t *testing.T) {
	cfg := &Config{}
	cfg.OmniRoute.Mode = "external"
	cfg.OmniRoute.ExternalBaseURL = "https://omniroute.example.test/v1"
	cfg.Providers = []ProviderEntry{{
		ID:      "omniroute",
		Type:    "omniroute",
		BaseURL: "https://stale.example.test/v1",
		Model:   "auto",
	}}
	cfg.LLM.Provider = "omniroute"

	cfg.ResolveProviders()

	if cfg.LLM.BaseURL != "https://omniroute.example.test/v1" {
		t.Fatalf("LLM base_url = %q, want external OmniRoute settings URL", cfg.LLM.BaseURL)
	}
}

func TestOmniRouteProviderSpecificAPIKeyOverridesSectionKey(t *testing.T) {
	cfg := &Config{}
	cfg.OmniRoute.Mode = "managed"
	cfg.OmniRoute.URL = "http://omniroute:20128"
	cfg.OmniRoute.APIKey = "omni_shared"
	cfg.Providers = []ProviderEntry{{
		ID:     "omniroute",
		Type:   "omniroute",
		Model:  "auto",
		APIKey: "omni_provider_specific",
	}}
	cfg.LLM.Provider = "omniroute"

	cfg.ResolveProviders()

	if cfg.LLM.APIKey != "omni_provider_specific" {
		t.Fatalf("LLM APIKey = %q, want provider-specific OmniRoute key", cfg.LLM.APIKey)
	}
}

func TestConfigSaveOmitsSpaceAgentSecrets(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	cfg := &Config{}
	cfg.SpaceAgent.Enabled = true
	cfg.SpaceAgent.AutoStart = true
	cfg.SpaceAgent.RepoURL = "https://github.com/agent0ai/space-agent"
	cfg.SpaceAgent.GitRef = "main"
	cfg.SpaceAgent.ContainerName = "aurago_space_agent"
	cfg.SpaceAgent.Image = "aurago-space-agent:main"
	cfg.SpaceAgent.Host = "0.0.0.0"
	cfg.SpaceAgent.Port = 3100
	cfg.SpaceAgent.HTTPSEnabled = true
	cfg.SpaceAgent.HTTPSPort = 3101
	cfg.SpaceAgent.CustomwarePath = "data/sidecars/space-agent/customware"
	cfg.SpaceAgent.DataPath = "data/sidecars/space-agent/data"
	cfg.SpaceAgent.AdminUser = "admin"
	cfg.SpaceAgent.PublicURL = "http://127.0.0.1:3100"
	cfg.SpaceAgent.AdminPassword = "space-admin-secret"
	cfg.SpaceAgent.BridgeToken = "space-bridge-secret"
	cfg.Tailscale.TsNet.ExposeSpaceAgent = true
	cfg.Tailscale.TsNet.SpaceAgentHostname = "aurago-space-agent"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	got := string(raw)
	for _, needle := range []string{"space-admin-secret", "space-bridge-secret", "admin_password:", "bridge_token:"} {
		if strings.Contains(got, needle) {
			t.Fatalf("expected Space Agent secrets to stay out of YAML, found %q in:\n%s", needle, got)
		}
	}
	if !strings.Contains(got, "space_agent:") || !strings.Contains(got, "enabled: true") {
		t.Fatalf("expected non-secret Space Agent settings to be serialized, got:\n%s", got)
	}
	if !strings.Contains(got, "https_enabled: true") || !strings.Contains(got, "https_port: 3101") {
		t.Fatalf("expected Space Agent HTTPS settings to be serialized, got:\n%s", got)
	}
	if !strings.Contains(got, "expose_space_agent: true") {
		t.Fatalf("expected Tailscale Space Agent exposure setting to be serialized, got:\n%s", got)
	}
	if !strings.Contains(got, "space_agent_hostname: aurago-space-agent") {
		t.Fatalf("expected Tailscale Space Agent hostname setting to be serialized, got:\n%s", got)
	}
}

func TestMigrateEggModeSharedKeyToVault(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
egg_mode:
  enabled: true
  master_url: ws://master.local/ws
  shared_key: deadbeef
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	vault := &testSecretVault{data: map[string]string{}}

	MigrateEggModeSharedKeyToVault(configPath, vault, slog.Default())

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.ApplyVaultSecrets(vault)

	if cfg.EggMode.SharedKey != "deadbeef" {
		t.Fatalf("shared key = %q, want deadbeef", cfg.EggMode.SharedKey)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after migration: %v", err)
	}
	if strings.Contains(string(raw), "shared_key:") {
		t.Fatalf("expected shared_key to be removed from config.yaml, got:\n%s", string(raw))
	}
}

func TestMigrateEggModeSharedKeyToVaultOverwritesStaleEggKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
egg_mode:
  enabled: true
  master_url: ws://master.local/ws
  shared_key: fresh-key
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	vault := &testSecretVault{data: map[string]string{"egg_shared_key": "stale-key"}}

	MigrateEggModeSharedKeyToVault(configPath, vault, slog.Default())

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.ApplyVaultSecrets(vault)

	if cfg.EggMode.SharedKey != "fresh-key" {
		t.Fatalf("shared key = %q, want fresh-key from latest egg config", cfg.EggMode.SharedKey)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after migration: %v", err)
	}
	if strings.Contains(string(raw), "shared_key:") {
		t.Fatalf("expected shared_key to be removed from config.yaml, got:\n%s", string(raw))
	}
}

func TestMigratePlaintextSecretsToVaultMovesProviderAndAccountSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
providers:
  - id: main
    name: Main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: sk-provider-secret
    model: openai/gpt-4o-mini
llm:
  provider: main
email_accounts:
  - id: work
    name: Work
    imap_host: mail.example.com
    password: mailbox-secret
truenas:
  enabled: true
  host: truenas.local
  api_key: tn-secret
telegram:
  enabled: true
  bot_token: tg-secret
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	vault := &testSecretVault{data: map[string]string{}}

	MigratePlaintextSecretsToVault(configPath, vault, slog.Default())

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.ApplyVaultSecrets(vault)
	cfg.ResolveProviders()

	if cfg.LLM.APIKey != "sk-provider-secret" {
		t.Fatalf("provider api key = %q, want sk-provider-secret", cfg.LLM.APIKey)
	}
	if len(cfg.EmailAccounts) != 1 || cfg.EmailAccounts[0].Password != "mailbox-secret" {
		t.Fatalf("email account password not restored from vault: %+v", cfg.EmailAccounts)
	}
	if cfg.TrueNAS.APIKey != "tn-secret" {
		t.Fatalf("truenas api key = %q, want tn-secret", cfg.TrueNAS.APIKey)
	}
	if cfg.Telegram.BotToken != "tg-secret" {
		t.Fatalf("telegram bot token = %q, want tg-secret", cfg.Telegram.BotToken)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after migration: %v", err)
	}
	got := string(raw)
	for _, needle := range []string{"api_key: sk-provider-secret", "password: mailbox-secret", "api_key: tn-secret", "bot_token: tg-secret"} {
		if strings.Contains(got, needle) {
			t.Fatalf("expected secret %q to be removed from config.yaml, got:\n%s", needle, got)
		}
	}
}

func TestConfigSaveWritesUpdatedField(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  ui_language: en
auth:
  enabled: false
personality:
  core_personality: neutral
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.Server.UILanguage = "de"
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(raw), "ui_language: de") {
		t.Fatalf("expected saved config to contain updated ui_language, got:\n%s", string(raw))
	}
}

func TestConfigSavePreservesComments(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `# top-level comment
server:
  # keep this language comment
  ui_language: en
auth:
  # keep auth comment
  enabled: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.Server.UILanguage = "de"
	cfg.Auth.Enabled = true
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	got := string(raw)
	for _, comment := range []string{"# top-level comment", "# keep this language comment", "# keep auth comment"} {
		if !strings.Contains(got, comment) {
			t.Fatalf("expected comment %q to be preserved, got:\n%s", comment, got)
		}
	}
}

func TestConfigSavePersistsOutgoingWebhooks(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  ui_language: en
auth:
  enabled: false
personality:
  core_personality: neutral
webhooks:
  enabled: true
  readonly: false
  outgoing: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.Webhooks.Outgoing = []OutgoingWebhook{{
		ID:          "hook_1",
		Name:        "Deploy",
		Description: "Trigger deploy",
		Method:      "POST",
		URL:         "https://example.test/deploy",
		Headers:     map[string]string{"X-Test": "1"},
		Parameters: []WebhookParameter{{
			Name:        "service",
			Type:        "string",
			Description: "Service name",
			Required:    true,
		}},
		PayloadType: "json",
	}}

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Webhooks.Outgoing) != 1 {
		t.Fatalf("outgoing webhook count = %d, want 1", len(reloaded.Webhooks.Outgoing))
	}
	if reloaded.Webhooks.Outgoing[0].URL != "https://example.test/deploy" {
		t.Fatalf("saved webhook url = %q", reloaded.Webhooks.Outgoing[0].URL)
	}
}

func TestConfigSavePersistsManifestRouting(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  ui_language: en
auth:
  enabled: false
personality:
  core_personality: neutral
manifest:
  enabled: true
  routing:
    enabled: false
    specificity_mode: off
    specificity: ""
    headers: {}
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.Manifest.Routing.Enabled = true
	cfg.Manifest.Routing.SpecificityMode = "fixed"
	cfg.Manifest.Routing.Specificity = "coding"
	cfg.Manifest.Routing.Headers = map[string]string{"x-aurago-task": "coding"}

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !reloaded.Manifest.Routing.Enabled {
		t.Fatal("manifest.routing.enabled = false, want true")
	}
	if reloaded.Manifest.Routing.SpecificityMode != "fixed" {
		t.Fatalf("specificity_mode = %q, want fixed", reloaded.Manifest.Routing.SpecificityMode)
	}
	if reloaded.Manifest.Routing.Specificity != "coding" {
		t.Fatalf("specificity = %q, want coding", reloaded.Manifest.Routing.Specificity)
	}
	if reloaded.Manifest.Routing.Headers["x-aurago-task"] != "coding" {
		t.Fatalf("routing header = %q, want coding", reloaded.Manifest.Routing.Headers["x-aurago-task"])
	}
}

func TestLoadAndSaveAIGatewayRequestControls(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  ui_language: en
ai_gateway:
  enabled: true
  account_id: acct
  gateway_id: gw
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AIGateway.Mode != "auto" {
		t.Fatalf("ai_gateway.mode = %q, want auto", cfg.AIGateway.Mode)
	}
	if cfg.AIGateway.LogMode != "metadata_only" {
		t.Fatalf("ai_gateway.log_mode = %q, want metadata_only", cfg.AIGateway.LogMode)
	}
	if cfg.AIGateway.Metadata == nil {
		t.Fatal("ai_gateway.metadata map should default to an empty map")
	}

	cfg.AIGateway.Mode = "provider_native"
	cfg.AIGateway.LogMode = "off"
	cfg.AIGateway.Metadata = map[string]string{"env": "lab"}
	cfg.AIGateway.RequestTimeoutMS = 5000
	cfg.AIGateway.MaxAttempts = 3
	cfg.AIGateway.RetryDelayMS = 250
	cfg.AIGateway.Backoff = "linear"
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.AIGateway.Mode != "provider_native" {
		t.Fatalf("saved mode = %q, want provider_native", reloaded.AIGateway.Mode)
	}
	if reloaded.AIGateway.LogMode != "off" {
		t.Fatalf("saved log_mode = %q, want off", reloaded.AIGateway.LogMode)
	}
	if reloaded.AIGateway.Metadata["env"] != "lab" {
		t.Fatalf("saved metadata = %#v, want env=lab", reloaded.AIGateway.Metadata)
	}
	if reloaded.AIGateway.RequestTimeoutMS != 5000 || reloaded.AIGateway.MaxAttempts != 3 ||
		reloaded.AIGateway.RetryDelayMS != 250 || reloaded.AIGateway.Backoff != "linear" {
		t.Fatalf("saved request controls = %+v", reloaded.AIGateway)
	}
}

func TestLoadPreservesWebhookRateLimitZeroAsUnlimited(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  ui_language: en
auth:
  enabled: false
personality:
  core_personality: neutral
webhooks:
  enabled: true
  rate_limit: 0
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Webhooks.RateLimit != 0 {
		t.Fatalf("RateLimit = %d, want 0 for unlimited", cfg.Webhooks.RateLimit)
	}
}

func TestLoadResolvesHelperLLMFromProvider(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: main-secret
    model: main-model
  - id: helper
    type: openai
    base_url: https://helper.example/v1
    api_key: helper-secret
    model: helper-default
llm:
  provider: main
  helper_enabled: true
  helper_provider: helper
  helper_model: helper-override
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.LLM.HelperEnabled {
		t.Fatal("expected llm.helper_enabled=true to be preserved")
	}
	if cfg.LLM.HelperProviderType != "openai" {
		t.Fatalf("helper provider type = %q, want openai", cfg.LLM.HelperProviderType)
	}
	if cfg.LLM.HelperBaseURL != "https://helper.example/v1" {
		t.Fatalf("helper base url = %q", cfg.LLM.HelperBaseURL)
	}
	if cfg.LLM.HelperAPIKey != "" {
		t.Fatalf("helper api key = %q, want empty until vault/env resolution", cfg.LLM.HelperAPIKey)
	}
	if cfg.LLM.HelperResolvedModel != "helper-override" {
		t.Fatalf("helper resolved model = %q, want helper-override", cfg.LLM.HelperResolvedModel)
	}
}

func TestLoadDoesNotFallbackHelperLLMToMainProvider(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: main-secret
    model: main-model
llm:
  provider: main
  helper_enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.LLM.HelperEnabled {
		t.Fatal("expected llm.helper_enabled=true to be preserved")
	}
	if cfg.LLM.HelperProviderType != "" {
		t.Fatalf("helper provider type = %q, want empty", cfg.LLM.HelperProviderType)
	}
	if cfg.LLM.HelperBaseURL != "" {
		t.Fatalf("helper base url = %q, want empty", cfg.LLM.HelperBaseURL)
	}
	if cfg.LLM.HelperAPIKey != "" {
		t.Fatalf("helper api key = %q, want empty", cfg.LLM.HelperAPIKey)
	}
	if cfg.LLM.HelperResolvedModel != "" {
		t.Fatalf("helper resolved model = %q, want empty", cfg.LLM.HelperResolvedModel)
	}
}

func TestLoadResolvesHelperOwnedSubsystemsFromHelperLLM(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: main-secret
    model: main-model
  - id: helper
    type: openai
    base_url: https://helper.example/v1
    api_key: helper-secret
    model: helper-model
llm:
  provider: main
  helper_enabled: true
  helper_provider: helper
personality:
  engine_v2: true
memory_analysis:
  enabled: true
tools:
  web_scraper:
    enabled: true
    summary_mode: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Personality.V2ProviderType != "openai" {
		t.Fatalf("personality v2 provider type = %q, want openai", cfg.Personality.V2ProviderType)
	}
	if cfg.Personality.V2ResolvedURL != "https://helper.example/v1" {
		t.Fatalf("personality v2 url = %q", cfg.Personality.V2ResolvedURL)
	}
	if cfg.Personality.V2ResolvedModel != "helper-model" {
		t.Fatalf("personality v2 model = %q, want helper-model", cfg.Personality.V2ResolvedModel)
	}
	if cfg.MemoryAnalysis.ProviderType != "openai" {
		t.Fatalf("memory analysis provider type = %q, want openai", cfg.MemoryAnalysis.ProviderType)
	}
	if cfg.MemoryAnalysis.BaseURL != "https://helper.example/v1" {
		t.Fatalf("memory analysis base url = %q", cfg.MemoryAnalysis.BaseURL)
	}
	if cfg.MemoryAnalysis.ResolvedModel != "helper-model" {
		t.Fatalf("memory analysis model = %q, want helper-model", cfg.MemoryAnalysis.ResolvedModel)
	}
	if cfg.Tools.WebScraper.SummaryBaseURL != "https://helper.example/v1" {
		t.Fatalf("web scraper summary base url = %q", cfg.Tools.WebScraper.SummaryBaseURL)
	}
	if cfg.Tools.WebScraper.SummaryModel != "helper-model" {
		t.Fatalf("web scraper summary model = %q, want helper-model", cfg.Tools.WebScraper.SummaryModel)
	}
}

func TestLoadMigratesLegacyPersonalityV2InlineFieldsToHelperLLM(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
llm:
  provider: openrouter
  base_url: https://openrouter.ai/api/v1
  api_key: main-secret
  model: main-model
personality:
  engine_v2: true
  v2_model: helper-model
  v2_url: https://helper.example/v1
  v2_api_key: helper-secret
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.LLM.HelperEnabled {
		t.Fatal("expected legacy v2 inline fields to enable helper llm migration")
	}
	if cfg.LLM.HelperProvider != "helper" {
		t.Fatalf("helper provider = %q, want helper", cfg.LLM.HelperProvider)
	}
	if cfg.LLM.HelperProviderType != "openai" {
		t.Fatalf("helper provider type = %q, want openai", cfg.LLM.HelperProviderType)
	}
	if cfg.LLM.HelperBaseURL != "https://helper.example/v1" {
		t.Fatalf("helper base url = %q", cfg.LLM.HelperBaseURL)
	}
	if cfg.LLM.HelperAPIKey != "helper-secret" {
		t.Fatalf("helper api key = %q, want helper-secret", cfg.LLM.HelperAPIKey)
	}
	if cfg.LLM.HelperResolvedModel != "helper-model" {
		t.Fatalf("helper resolved model = %q, want helper-model", cfg.LLM.HelperResolvedModel)
	}
}

func TestLoadDoesNotFallbackHelperOwnedSubsystemsToMainLLM(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: main-secret
    model: main-model
llm:
  provider: main
personality:
  engine_v2: true
tools:
  web_scraper:
    enabled: true
    summary_mode: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Personality.V2ResolvedModel != "" || cfg.Personality.V2ResolvedURL != "" || cfg.Personality.V2ResolvedKey != "" {
		t.Fatalf("expected personality v2 helper path to stay unresolved without helper llm, got model=%q url=%q", cfg.Personality.V2ResolvedModel, cfg.Personality.V2ResolvedURL)
	}
	if cfg.MemoryAnalysis.ResolvedModel != "" || cfg.MemoryAnalysis.BaseURL != "" || cfg.MemoryAnalysis.APIKey != "" || cfg.MemoryAnalysis.ProviderType != "" {
		t.Fatalf("expected memory analysis helper path to stay unresolved without helper llm, got model=%q url=%q", cfg.MemoryAnalysis.ResolvedModel, cfg.MemoryAnalysis.BaseURL)
	}
	if cfg.Tools.WebScraper.SummaryModel != "" || cfg.Tools.WebScraper.SummaryBaseURL != "" || cfg.Tools.WebScraper.SummaryAPIKey != "" {
		t.Fatalf("expected web scraper summary helper path to stay unresolved without helper llm, got model=%q url=%q", cfg.Tools.WebScraper.SummaryModel, cfg.Tools.WebScraper.SummaryBaseURL)
	}
}

func TestApplyOAuthTokensDoesNotOverwriteProviderStaticSecrets(t *testing.T) {
	cfg := &Config{}
	cfg.Providers = []ProviderEntry{
		{
			ID:       "oauth-provider",
			Type:     "openai",
			AuthType: "oauth2",
			APIKey:   "",
		},
	}
	cfg.LLM.Provider = "oauth-provider"
	cfg.ResolveProviders()

	vault := &testSecretVault{data: map[string]string{
		"oauth_oauth-provider": `{"access_token":"oauth-access-token"}`,
	}}
	cfg.ApplyOAuthTokens(vault)

	if cfg.Providers[0].APIKey != "" {
		t.Fatalf("provider api key = %q, want empty static secret field", cfg.Providers[0].APIKey)
	}
	if cfg.LLM.APIKey != "oauth-access-token" {
		t.Fatalf("resolved llm api key = %q, want oauth-access-token", cfg.LLM.APIKey)
	}
}

func TestApplyOAuthTokensUpdatesHelperAndDerivedSubsystems(t *testing.T) {
	cfg := &Config{}
	cfg.Providers = []ProviderEntry{
		{
			ID:        "helper",
			Type:      "workers-ai",
			BaseURL:   "",
			Model:     "@cf/meta/llama-3.1-8b-instruct",
			AccountID: "cf-account",
			AuthType:  "oauth2",
		},
	}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.ResolveProviders()

	if cfg.LLM.HelperAccountID != "cf-account" {
		t.Fatalf("helper account id = %q, want cf-account", cfg.LLM.HelperAccountID)
	}

	cfg.Personality.V2ResolvedURL = cfg.LLM.HelperBaseURL
	cfg.Personality.V2ResolvedModel = cfg.LLM.HelperResolvedModel
	cfg.MemoryAnalysis.BaseURL = cfg.LLM.HelperBaseURL
	cfg.MemoryAnalysis.ResolvedModel = cfg.LLM.HelperResolvedModel
	cfg.Tools.WebScraper.SummaryBaseURL = cfg.LLM.HelperBaseURL
	cfg.Tools.WebScraper.SummaryModel = cfg.LLM.HelperResolvedModel

	vault := &testSecretVault{data: map[string]string{
		"oauth_helper": `{"access_token":"helper-oauth-token"}`,
	}}
	cfg.ApplyOAuthTokens(vault)

	if cfg.LLM.HelperAPIKey != "helper-oauth-token" {
		t.Fatalf("helper api key = %q, want helper-oauth-token", cfg.LLM.HelperAPIKey)
	}
	if cfg.Personality.V2ResolvedKey != "helper-oauth-token" {
		t.Fatalf("personality oauth key = %q, want helper-oauth-token", cfg.Personality.V2ResolvedKey)
	}
	if cfg.MemoryAnalysis.APIKey != "helper-oauth-token" {
		t.Fatalf("memory analysis oauth key = %q, want helper-oauth-token", cfg.MemoryAnalysis.APIKey)
	}
	if cfg.Tools.WebScraper.SummaryAPIKey != "helper-oauth-token" {
		t.Fatalf("web scraper summary oauth key = %q, want helper-oauth-token", cfg.Tools.WebScraper.SummaryAPIKey)
	}
}

func TestApplyOAuthTokensUsesCurrentPersonalityProviderField(t *testing.T) {
	cfg := &Config{}
	cfg.Providers = []ProviderEntry{
		{
			ID:       "personality",
			Type:     "openai",
			AuthType: "oauth2",
		},
	}
	cfg.Personality.V2Provider = "personality"

	vault := &testSecretVault{data: map[string]string{
		"oauth_personality": `{"access_token":"personality-token"}`,
	}}
	cfg.ApplyOAuthTokens(vault)

	if cfg.Personality.V2ResolvedKey != "personality-token" {
		t.Fatalf("personality resolved key = %q, want personality-token", cfg.Personality.V2ResolvedKey)
	}
}

func TestLoadObsidianLegacyReadOnlyKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
obsidian:
  enabled: true
  read_only: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Obsidian.ReadOnly {
		t.Fatal("expected obsidian.read_only to map to Obsidian.ReadOnly")
	}
}

func TestLoadObsidianCanonicalReadOnlyWins(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
obsidian:
  enabled: true
  readonly: false
  read_only: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Obsidian.ReadOnly {
		t.Fatal("expected canonical obsidian.readonly to take precedence over legacy read_only")
	}
}

func TestLoadJellyfinLegacyReadOnlyKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
jellyfin:
  enabled: true
  read_only: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Jellyfin.ReadOnly {
		t.Fatal("expected jellyfin.read_only to map to Jellyfin.ReadOnly")
	}
}

func TestLoadJellyfinCanonicalReadOnlyWins(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
jellyfin:
  enabled: true
  readonly: false
  read_only: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Jellyfin.ReadOnly {
		t.Fatal("expected canonical jellyfin.readonly to take precedence over legacy read_only")
	}
}

func TestMaintenanceEnabledDefaultsTrueWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
directories:
  data_dir: './data'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Maintenance.Enabled {
		t.Fatal("expected maintenance.enabled to default to true when omitted")
	}
}

func TestMaintenanceRetentionDefaultsWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
directories:
  data_dir: './data'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Maintenance.Retention.PatternsDays != 90 {
		t.Fatalf("patterns_days = %d, want 90", cfg.Maintenance.Retention.PatternsDays)
	}
	if cfg.Maintenance.Retention.ErrorPatternsDays != 7 {
		t.Fatalf("error_patterns_days = %d, want 7", cfg.Maintenance.Retention.ErrorPatternsDays)
	}
	if cfg.Maintenance.Retention.OperationalIssuesDays != 30 {
		t.Fatalf("operational_issues_days = %d, want 30", cfg.Maintenance.Retention.OperationalIssuesDays)
	}
}

func TestMaintenanceRetentionRespectsExplicitValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
maintenance:
  retention:
    patterns_days: 14
    done_notes_days: 2
directories:
  data_dir: './data'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Maintenance.Retention.PatternsDays != 14 {
		t.Fatalf("patterns_days = %d, want 14", cfg.Maintenance.Retention.PatternsDays)
	}
	if cfg.Maintenance.Retention.DoneNotesDays != 2 {
		t.Fatalf("done_notes_days = %d, want 2", cfg.Maintenance.Retention.DoneNotesDays)
	}
	if cfg.Maintenance.Retention.MoodLogDays != 30 {
		t.Fatalf("mood_log_days = %d, want default 30", cfg.Maintenance.Retention.MoodLogDays)
	}
}

func TestMaintenanceEnabledRespectsExplicitFalse(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
maintenance:
  enabled: false
directories:
  data_dir: './data'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Maintenance.Enabled {
		t.Fatal("expected explicit maintenance.enabled=false to be preserved")
	}
}
