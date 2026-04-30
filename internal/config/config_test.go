package config

import (
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
	if cfg.RemoteControl.MaxFileSizeMB != 50 {
		t.Fatalf("max_file_size_mb = %d, want 50", cfg.RemoteControl.MaxFileSizeMB)
	}
	if !cfg.RemoteControl.AuditLog {
		t.Fatal("expected remote_control.audit_log to default to true")
	}
	if cfg.RemoteControl.ReadOnly {
		t.Fatal("expected remote_control.readonly to default to false")
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
