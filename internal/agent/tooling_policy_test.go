package agent

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/prompts"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestBuildToolingPolicyAutoEnablesNativeFunctionsForDeepSeek(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "deepseek-chat"

	policy := buildToolingPolicy(cfg, "")

	if !policy.UseNativeFunctions {
		t.Fatal("expected native function calling to be enabled for DeepSeek")
	}
	if !policy.AutoEnabledNativeFunctions {
		t.Fatal("expected DeepSeek native function calling to be marked as auto-enabled")
	}
}

func TestPlannerNativeToolsRespectConfigAndRuntimeDB(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Planner.Enabled = true

	configNames := ToolNamesFromConfig(cfg)
	if !containsName(configNames, "manage_appointments") {
		t.Fatalf("ToolNamesFromConfig missing manage_appointments: %v", configNames)
	}
	if !containsName(configNames, "manage_todos") {
		t.Fatalf("ToolNamesFromConfig missing manage_todos: %v", configNames)
	}

	policy := buildToolingPolicy(cfg, "")
	withoutDB := buildToolFeatureFlags(RunConfig{Config: cfg, SessionID: "planner-runtime"}, policy)
	if withoutDB.PlannerEnabled {
		t.Fatal("PlannerEnabled = true without PlannerDB, want false")
	}

	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB: %v", err)
	}
	defer db.Close()

	withDB := buildToolFeatureFlags(RunConfig{Config: cfg, PlannerDB: db, SessionID: "planner-runtime"}, policy)
	if !withDB.PlannerEnabled {
		t.Fatal("PlannerEnabled = false with PlannerDB, want true")
	}
}

func TestBuildToolingPolicyRecognizesStepFunCapabilities(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "step-3.5-flash-2603"
	cfg.LLM.StructuredOutputs = true

	policy := buildToolingPolicy(cfg, "")

	if !policy.UseNativeFunctions {
		t.Fatal("expected native function calling to be enabled for Stepfun flash models")
	}
	if !policy.AutoEnabledNativeFunctions {
		t.Fatal("expected Stepfun native function calling to be marked as auto-enabled")
	}
	if !policy.StructuredOutputsEnabled {
		t.Fatal("expected structured outputs to remain enabled for Stepfun flash models")
	}
	if !policy.ParallelToolCallsEnabled {
		t.Fatal("expected parallel tool calls to remain enabled for Stepfun flash models")
	}
	if policy.Capabilities.DisableNativeFunctionCalling {
		t.Fatal("did not expect Stepfun flash models to be forced into text tool-call mode")
	}
}

func TestBuildToolingPolicyHonorsExplicitNativeFunctions(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.LLM.UseNativeFunctions = true

	policy := buildToolingPolicy(cfg, "")

	if !policy.UseNativeFunctions {
		t.Fatal("expected explicit native function calling setting to be preserved")
	}
	if policy.AutoEnabledNativeFunctions {
		t.Fatal("did not expect explicit native function calling to be treated as auto-enabled")
	}
}

func TestBuildToolingPolicyUsesProviderCapabilities(t *testing.T) {
	autoFalse := false
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "custom"
	cfg.LLM.Model = "manual-tools-model"
	cfg.LLM.StructuredOutputs = false
	cfg.LLM.UseNativeFunctions = false
	cfg.Providers = []config.ProviderEntry{{
		ID:      "main",
		Type:    "custom",
		BaseURL: "https://example.test/v1",
		Model:   "manual-tools-model",
		Capabilities: config.ProviderCapabilities{
			Auto:              &autoFalse,
			ToolCalling:       true,
			StructuredOutputs: true,
			Multimodal:        false,
			DetectedModel:     "manual-tools-model",
			Source:            "manual",
		},
	}}

	policy := buildToolingPolicy(cfg, "")

	if !policy.UseNativeFunctions {
		t.Fatal("expected provider tool_calling capability to enable native functions")
	}
	if !policy.StructuredOutputsRequested {
		t.Fatal("expected provider structured_outputs capability to request structured outputs")
	}
	if !policy.StructuredOutputsEnabled {
		t.Fatal("expected provider structured_outputs capability to enable strict schemas")
	}
}

func TestReconcileToolPromptModeDowngradesNativeWhenNoSchemas(t *testing.T) {
	flags := prompts.ContextFlags{
		NativeToolsEnabled: true,
		IsTextModeModel:    false,
	}
	policy := ToolingPolicy{UseNativeFunctions: true}
	useNativeFunctions := true

	reconcileToolPromptModeWithSchemas(&flags, &policy, &useNativeFunctions, 0, nil)

	if useNativeFunctions {
		t.Fatal("expected native function calling to be disabled when no native schemas are attached")
	}
	if policy.UseNativeFunctions {
		t.Fatal("expected policy to be downgraded for the current request")
	}
	if flags.NativeToolsEnabled {
		t.Fatal("expected prompt flags to stop advertising native tool calls")
	}
	if !flags.IsTextModeModel {
		t.Fatal("expected prompt flags to use text JSON tool mode when no native schemas are attached")
	}
}

func TestReconcileToolPromptModeKeepsNativeWhenSchemasExist(t *testing.T) {
	flags := prompts.ContextFlags{
		NativeToolsEnabled: true,
		IsTextModeModel:    false,
	}
	policy := ToolingPolicy{UseNativeFunctions: true}
	useNativeFunctions := true

	reconcileToolPromptModeWithSchemas(&flags, &policy, &useNativeFunctions, 1, nil)

	if !useNativeFunctions {
		t.Fatal("expected native function calling to stay enabled when schemas are attached")
	}
	if !policy.UseNativeFunctions {
		t.Fatal("expected policy to stay native")
	}
	if !flags.NativeToolsEnabled {
		t.Fatal("expected prompt flags to keep native tool mode")
	}
	if flags.IsTextModeModel {
		t.Fatal("did not expect text JSON mode while native schemas exist")
	}
}

func TestReconcilePromptToolModeWithRequestDowngradesNativeWhenRequestHasNoTools(t *testing.T) {
	flags := prompts.ContextFlags{
		NativeToolsEnabled: true,
		IsTextModeModel:    false,
	}
	policy := ToolingPolicy{UseNativeFunctions: true}

	reconcilePromptToolModeWithRequest(&flags, &policy, nil, nil)

	if flags.NativeToolsEnabled {
		t.Fatal("expected prompt flags to stop advertising native tool calls when request has no tools")
	}
	if !flags.IsTextModeModel {
		t.Fatal("expected text JSON mode when no native tools are attached to the request")
	}
	if policy.UseNativeFunctions {
		t.Fatal("expected policy to be downgraded for this request")
	}
}

func TestExecuteAgentLoopUsesInitializedNativeSchemas(t *testing.T) {
	promptsDir, err := filepath.Abs("../../prompts")
	if err != nil {
		t.Fatalf("resolve prompts dir: %v", err)
	}
	cfg := &config.Config{}
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.LLM.UseNativeFunctions = true
	cfg.Agent.SystemLanguage = "English"
	cfg.Server.UILanguage = "en"
	cfg.Directories.PromptsDir = promptsDir
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Agent.ContextWindow = 32768
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.WebScraper.Enabled = true

	client := &mockChatClient{response: "Done. <done/>"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "hi"},
		},
	}
	_, err = ExecuteAgentLoop(context.Background(), req, RunConfig{
		Config:       cfg,
		Logger:       logger,
		LLMClient:    client,
		ShortTermMem: stm,
		Registry:     tools.NewProcessRegistry(logger),
		SessionID:    "native-schema-test",
	}, false, NoopBroker{})
	if err != nil {
		t.Fatalf("ExecuteAgentLoop: %v", err)
	}
	if len(client.lastReq.Tools) == 0 {
		t.Fatal("expected initialized native tool schemas to be attached to the LLM request")
	}
}

func TestBuildToolingPolicyDisablesStructuredOutputsAndParallelForOllama(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.StructuredOutputs = true
	cfg.LLM.UseNativeFunctions = true

	policy := buildToolingPolicy(cfg, "")

	if !policy.StructuredOutputsRequested {
		t.Fatal("expected structured outputs request to be preserved")
	}
	if policy.StructuredOutputsEnabled {
		t.Fatal("expected structured outputs to be disabled for Ollama")
	}
	if policy.ParallelToolCallsEnabled {
		t.Fatal("expected parallel tool calls to be disabled for Ollama")
	}
}

func TestBuildPromptContextFlagsKeepsHomepageFallbackWhenDockerSocketUnavailable(t *testing.T) {
	cfg := &config.Config{}
	cfg.Runtime.IsDocker = true
	cfg.Runtime.DockerSocketOK = false
	cfg.Docker.Enabled = true
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowLocalServer = true

	runCfg := RunConfig{
		Config:    cfg,
		SessionID: "default",
	}

	policy := buildToolingPolicy(cfg, "")
	flags := buildPromptContextFlags(runCfg, policy, promptContextOptions{})

	if flags.DockerEnabled {
		t.Fatal("expected docker-enabled prompt flag to be false without docker socket access")
	}
	if !flags.HomepageEnabled {
		t.Fatal("expected homepage prompt flag to stay enabled via local server fallback")
	}
	if !flags.HomepageAllowLocalServer {
		t.Fatal("expected homepage local server fallback flag to be exposed")
	}
}

func TestBuildPromptContextFlagsAndToolFeatureFlagsShareResolvedCapabilities(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.Enabled = true
	cfg.HomeAssistant.Enabled = true
	cfg.Koofr.Enabled = true
	cfg.BrowserAutomation.Enabled = true
	cfg.GoogleWorkspace.Enabled = true
	cfg.Vercel.Enabled = true
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.BrowserAutomation.Enabled = true
	cfg.Tools.WebCapture.Enabled = true
	cfg.Tools.NetworkPing.Enabled = true
	cfg.Tools.NetworkScan.Enabled = true
	cfg.Tools.FormAutomation.Enabled = true
	cfg.Tools.UPnPScan.Enabled = true
	cfg.S3.Enabled = true
	cfg.VirusTotal.Enabled = true
	cfg.SpaceAgent.Enabled = true
	cfg.SpaceAgent.PublicURL = " https://aurago-space-agent.example.ts.net/ "
	cfg.Agent.AllowShell = true
	cfg.Agent.AllowPython = true
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Agent.AllowNetworkRequests = true
	cfg.Agent.AllowRemoteShell = true
	cfg.Agent.AllowSelfUpdate = true

	runCfg := RunConfig{Config: cfg, SessionID: "default"}
	policy := buildToolingPolicy(cfg, "")

	contextFlags := buildPromptContextFlags(runCfg, policy, promptContextOptions{})
	toolFlags := buildToolFeatureFlags(runCfg, policy)

	if contextFlags.DiscordEnabled != toolFlags.DiscordEnabled {
		t.Fatal("discord capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.HomeAssistantEnabled != toolFlags.HomeAssistantEnabled {
		t.Fatal("home assistant capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.KoofrEnabled != toolFlags.KoofrEnabled {
		t.Fatal("koofr capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.BrowserAutomationEnabled != toolFlags.BrowserAutomationEnabled {
		t.Fatal("browser automation capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.GoogleWorkspaceEnabled != toolFlags.GoogleWorkspaceEnabled {
		t.Fatal("google workspace capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.VercelEnabled != toolFlags.VercelEnabled {
		t.Fatal("vercel capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.MemoryEnabled != toolFlags.MemoryEnabled {
		t.Fatal("memory capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.WebCaptureEnabled != toolFlags.WebCaptureEnabled {
		t.Fatal("web capture capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.NetworkPingEnabled != toolFlags.NetworkPingEnabled {
		t.Fatal("network ping capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.NetworkScanEnabled != toolFlags.NetworkScanEnabled {
		t.Fatal("network scan capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.FormAutomationEnabled != toolFlags.FormAutomationEnabled {
		t.Fatal("form automation capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.UPnPScanEnabled != toolFlags.UPnPScanEnabled {
		t.Fatal("upnp capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.S3Enabled != toolFlags.S3Enabled {
		t.Fatal("s3 capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.VirusTotalEnabled != toolFlags.VirusTotalEnabled {
		t.Fatal("virustotal capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.SpaceAgentEnabled != toolFlags.SpaceAgentEnabled {
		t.Fatal("space agent capability mismatch between prompt context and tool feature flags")
	}
	if contextFlags.SpaceAgentPublicURL != "https://aurago-space-agent.example.ts.net/" {
		t.Fatalf("space agent public URL = %q, want trimmed configured URL", contextFlags.SpaceAgentPublicURL)
	}
	if contextFlags.AllowShell != toolFlags.AllowShell ||
		contextFlags.AllowPython != toolFlags.AllowPython ||
		contextFlags.AllowFilesystemWrite != toolFlags.AllowFilesystemWrite ||
		contextFlags.AllowNetworkRequests != toolFlags.AllowNetworkRequests ||
		contextFlags.AllowRemoteShell != toolFlags.AllowRemoteShell ||
		contextFlags.AllowSelfUpdate != toolFlags.AllowSelfUpdate {
		t.Fatal("danger-zone capability mismatch between prompt context and tool feature flags")
	}
}

func TestBuildPromptContextFlagsIncludesComposioServicesContext(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SystemLanguage = "en"
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "cmp-secret"
	cfg.Composio.ReadOnly = true
	cfg.Composio.AllowDestructive = false
	cfg.Composio.Toolkits = []config.ComposioToolkitConfig{
		{Slug: "gmail", Enabled: true},
		{Slug: "slack", Enabled: false},
	}

	flags := buildPromptContextFlags(RunConfig{Config: cfg, SessionID: "default"}, buildToolingPolicy(cfg, "prüfe gmail"), promptContextOptions{})
	ctx := flags.ComposioServicesContext
	for _, want := range []string{"gmail", "composio_call", "read_only=true", "allow_destructive=false", "tool_access=policy_allowed_catalog", "allowlist=disabled"} {
		if !strings.Contains(ctx, want) {
			t.Fatalf("ComposioServicesContext missing %q: %q", want, ctx)
		}
	}
	for _, notWant := range []string{"cmp-secret", "slack", "allowed_tool_count=0"} {
		if strings.Contains(ctx, notWant) {
			t.Fatalf("ComposioServicesContext leaked %q: %q", notWant, ctx)
		}
	}
}

func TestBuildPromptContextFlagsInjectsReachableChatChannelsForAutonomousRuns(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SystemLanguage = "en"
	cfg.Telegram.BotToken = "telegram-token"
	cfg.Telegram.UserID = 42
	cfg.Discord.Enabled = true
	cfg.Discord.GuildID = "guild-1"
	cfg.Discord.DefaultChannelID = "channel-1"
	cfg.Telnyx.Enabled = true
	cfg.Telnyx.PhoneNumber = "+15550001000"
	cfg.Telnyx.AllowedNumbers = []string{"+15550001001"}

	runCfg := RunConfig{
		Config:        cfg,
		SessionID:     "heartbeat",
		MessageSource: "heartbeat",
	}
	policy := buildToolingPolicy(cfg, "notify the user")
	flags := buildPromptContextFlags(runCfg, policy, promptContextOptions{})

	prompt, _ := prompts.BuildSystemPrompt("", &flags, "", slog.Default())
	for _, want := range []string{
		"# REACHABLE CHAT CHANNELS",
		"Telegram",
		"send_telegram",
		"Discord",
		"send_discord",
		"SMS",
		"send_notification",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing reachable chat channel marker %q:\n%s", want, prompt)
		}
	}
}

func TestBuildPromptContextFlagsInjectsManagedSitesContext(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "homepage.db"))
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	defer db.Close()
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	sitePath := filepath.Join(cfg.Homepage.WorkspacePath, "site-a")
	if err := os.MkdirAll(sitePath, 0755); err != nil {
		t.Fatalf("mkdir site: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sitePath, "index.html"), []byte("<h1>Site A</h1>"), 0644); err != nil {
		t.Fatalf("write site: %v", err)
	}
	homepageCfg := tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}
	proj, err := tools.EnsureHomepageProjectForDir(db, homepageCfg, "site-a", "Site A", "html")
	if err != nil {
		t.Fatalf("EnsureHomepageProjectForDir failed: %v", err)
	}
	save := tools.SaveHomepageRevisionAndState(homepageCfg, db, "site-a", "initial", "test", "test", nil, slog.Default())
	if len(save.Warnings) > 0 {
		t.Fatalf("save revision warnings: %v", save.Warnings)
	}
	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("remote"))
	}))
	defer remoteServer.Close()
	if err := tools.RecordHomepageDeployment(db, tools.HomepageDeploymentRecord{
		ProjectID:        proj.ID,
		RevisionID:       save.RevisionID,
		Provider:         "netlify",
		ProviderTargetID: "site-1",
		ProviderDeployID: "deploy-1",
		URL:              remoteServer.URL,
		BuildDir:         ".",
		Status:           "ok",
	}); err != nil {
		t.Fatalf("record deployment: %v", err)
	}
	if _, err := tools.ReconcileHomepageProject(homepageCfg, db, "site-a", slog.Default()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	flags := buildPromptContextFlags(RunConfig{
		Config:             cfg,
		HomepageRegistryDB: db,
	}, buildToolingPolicy(cfg, "update the homepage"), promptContextOptions{})

	if !strings.Contains(flags.ReuseContext, "# MANAGED WEBSITES") || !strings.Contains(flags.ReuseContext, "site-a") {
		t.Fatalf("ReuseContext missing managed site summary: %q", flags.ReuseContext)
	}
	if !strings.Contains(flags.ReuseContext, "netlify=") {
		t.Fatalf("ReuseContext missing deploy target provider: %q", flags.ReuseContext)
	}
	if !strings.Contains(flags.ReuseContext, "remote=") {
		t.Fatalf("ReuseContext missing remote observation summary: %q", flags.ReuseContext)
	}
}

func TestBuildToolingPolicyKeepsConfiguredGuideBudgetByDefault(t *testing.T) {
	resetAgentTelemetryForTest()

	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.Agent.MaxToolGuides = 6

	policy := buildToolingPolicy(cfg, "")

	if policy.TelemetryProfile != "default" {
		t.Fatalf("unexpected telemetry profile: %s", policy.TelemetryProfile)
	}
	if policy.EffectiveMaxToolGuides != 6 {
		t.Fatalf("effective guide budget = %d, want 6", policy.EffectiveMaxToolGuides)
	}
}

func TestBuildToolingPolicyKeepsDefaultTelemetryProfileForRegularChat(t *testing.T) {
	resetAgentTelemetryForTest()

	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "openai/gpt-4o-mini"
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 16
	cfg.Agent.MaxToolGuides = 5

	policy := buildToolingPolicy(cfg, "what is the current docker status?")
	if policy.TelemetryProfile != "default" {
		t.Fatalf("TelemetryProfile = %q, want default", policy.TelemetryProfile)
	}
	if policy.EffectiveMaxToolGuides != 5 {
		t.Fatalf("EffectiveMaxToolGuides = %d, want 5", policy.EffectiveMaxToolGuides)
	}
}

func TestBuildToolingPolicyCapsMiniMaxToolBudgets(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "minimax"
	cfg.LLM.Model = "minimax-m2.7"
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 32
	cfg.Agent.AdaptiveTools.MaxTotalTools = 52
	cfg.Agent.AdaptiveTools.ProviderProfilesEnabled = true

	policy := buildToolingPolicy(cfg, "open the desktop")

	if policy.ProviderToolProfile != "minimax_stability" {
		t.Fatalf("ProviderToolProfile = %q, want minimax_stability", policy.ProviderToolProfile)
	}
	if policy.EffectiveMaxAdaptiveTools != 12 {
		t.Fatalf("EffectiveMaxAdaptiveTools = %d, want 12", policy.EffectiveMaxAdaptiveTools)
	}
	if policy.EffectiveMaxTotalTools != 24 {
		t.Fatalf("EffectiveMaxTotalTools = %d, want 24", policy.EffectiveMaxTotalTools)
	}
	if policy.EffectiveHeaderTimeoutSec != 90 {
		t.Fatalf("EffectiveHeaderTimeoutSec = %d, want 90", policy.EffectiveHeaderTimeoutSec)
	}
}

func TestBuildToolingPolicyHonorsProviderProfileOptOut(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "minimax"
	cfg.LLM.Model = "minimax-m2.7"
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 32
	cfg.Agent.AdaptiveTools.MaxTotalTools = 52
	cfg.Agent.AdaptiveTools.ProviderProfilesEnabled = false

	policy := buildToolingPolicy(cfg, "")

	if policy.ProviderToolProfile != "default" {
		t.Fatalf("ProviderToolProfile = %q, want default", policy.ProviderToolProfile)
	}
	if policy.EffectiveMaxAdaptiveTools != 32 {
		t.Fatalf("EffectiveMaxAdaptiveTools = %d, want 32", policy.EffectiveMaxAdaptiveTools)
	}
	if policy.EffectiveMaxTotalTools != 52 {
		t.Fatalf("EffectiveMaxTotalTools = %d, want 52", policy.EffectiveMaxTotalTools)
	}
}

func TestBuildToolingPolicyCapsGLMToolBudgets(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "zhipuai/glm-4.5"
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 32
	cfg.Agent.AdaptiveTools.MaxTotalTools = 52
	cfg.Agent.AdaptiveTools.ProviderProfilesEnabled = true

	policy := buildToolingPolicy(cfg, "")

	if policy.ProviderToolProfile != "glm_stability" {
		t.Fatalf("ProviderToolProfile = %q, want glm_stability", policy.ProviderToolProfile)
	}
	if policy.EffectiveMaxAdaptiveTools != 12 {
		t.Fatalf("EffectiveMaxAdaptiveTools = %d, want 12", policy.EffectiveMaxAdaptiveTools)
	}
	if policy.EffectiveMaxTotalTools != 24 {
		t.Fatalf("EffectiveMaxTotalTools = %d, want 24", policy.EffectiveMaxTotalTools)
	}
}

func TestBuildToolingPolicyKeepsProviderNeutralBudgets(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "openai/gpt-4o-mini"
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 16
	cfg.Agent.AdaptiveTools.MaxTotalTools = 32
	cfg.Agent.AdaptiveTools.ProviderProfilesEnabled = true

	policy := buildToolingPolicy(cfg, "")

	if policy.ProviderToolProfile != "default" {
		t.Fatalf("ProviderToolProfile = %q, want default", policy.ProviderToolProfile)
	}
	if policy.EffectiveMaxAdaptiveTools != 16 {
		t.Fatalf("EffectiveMaxAdaptiveTools = %d, want 16", policy.EffectiveMaxAdaptiveTools)
	}
	if policy.EffectiveMaxTotalTools != 32 {
		t.Fatalf("EffectiveMaxTotalTools = %d, want 32", policy.EffectiveMaxTotalTools)
	}
}

func TestBuildToolingPolicyReducesGuideBudgetForWeakScope(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "deepseek-chat"}
	for i := 0; i < 8; i++ {
		RecordScopedToolResult(scope, i < 4)
	}

	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "deepseek-chat"
	cfg.Agent.MaxToolGuides = 5

	policy := buildToolingPolicy(cfg, "")

	if policy.TelemetryProfile != "conservative" {
		t.Fatalf("telemetry profile = %s, want conservative", policy.TelemetryProfile)
	}
	if policy.EffectiveMaxToolGuides != 3 {
		t.Fatalf("effective guide budget = %d, want 3", policy.EffectiveMaxToolGuides)
	}
	if policy.TelemetrySnapshot.ToolCalls != 8 {
		t.Fatalf("telemetry tool calls = %d, want 8", policy.TelemetrySnapshot.ToolCalls)
	}
	if !policy.EffectiveGuideStrategy.PreferSemantics {
		t.Fatal("expected conservative profile to prefer semantic guides")
	}
	if policy.EffectiveGuideStrategy.DisableRecentHeuristics {
		t.Fatal("did not expect conservative profile to disable recent guide fallback")
	}
	if !policy.EffectiveGuideStrategy.DisableStatisticalHeuristics {
		t.Fatal("expected conservative profile to disable statistical heuristics")
	}
	if !policy.EffectiveGuideStrategy.DisableFrequencyHeuristics {
		t.Fatal("expected conservative profile to disable frequency heuristics")
	}
}

func TestBuildToolFeatureFlagsRequiresSQLPoolForSQLConnections(t *testing.T) {
	cfg := &config.Config{}
	cfg.SQLConnections.Enabled = true

	t.Run("disabled when pool missing", func(t *testing.T) {
		runCfg := RunConfig{
			Config:           cfg,
			SessionID:        "default",
			SQLConnectionsDB: &sql.DB{},
		}

		flags := buildToolFeatureFlags(runCfg, buildToolingPolicy(cfg, ""))
		if flags.SQLConnectionsEnabled {
			t.Fatal("expected SQL connections to stay disabled without an initialized pool")
		}
	})

	t.Run("enabled when db and pool exist", func(t *testing.T) {
		runCfg := RunConfig{
			Config:            cfg,
			SessionID:         "default",
			SQLConnectionsDB:  &sql.DB{},
			SQLConnectionPool: &sqlconnections.ConnectionPool{},
		}

		flags := buildToolFeatureFlags(runCfg, buildToolingPolicy(cfg, ""))
		if !flags.SQLConnectionsEnabled {
			t.Fatal("expected SQL connections to be enabled when db and pool are available")
		}
	})
}

func TestApplyTelemetryAwarePromptTierDowngradesFullToCompactForWeakScope(t *testing.T) {
	policy := ToolingPolicy{
		TelemetryProfile: "conservative",
		TelemetrySnapshot: AgentTelemetryScopeSnapshot{
			ToolCalls:    10,
			FailureRate:  0.5,
			SuccessRate:  0.5,
			TotalEvents:  10,
			ProviderType: "openrouter",
			Model:        "deepseek-chat",
		},
	}
	flags := prompts.ContextFlags{
		MessageCount:    8,
		PredictedGuides: nil,
	}

	got := applyTelemetryAwarePromptTier(policy, flags, "full")

	if got != "compact" {
		t.Fatalf("tier = %s, want compact", got)
	}
}

func TestApplyTelemetryAwarePromptTierKeepsFullWhenGuidesOrCodingAreNeeded(t *testing.T) {
	policy := ToolingPolicy{TelemetryProfile: "conservative"}

	gotWithGuides := applyTelemetryAwarePromptTier(policy, prompts.ContextFlags{
		MessageCount:    8,
		PredictedGuides: []string{"guide"},
	}, "full")
	if gotWithGuides != "full" {
		t.Fatalf("tier with guides = %s, want full", gotWithGuides)
	}

	gotWithCoding := applyTelemetryAwarePromptTier(policy, prompts.ContextFlags{
		MessageCount:   8,
		RequiresCoding: true,
	}, "full")
	if gotWithCoding != "full" {
		t.Fatalf("tier with coding = %s, want full", gotWithCoding)
	}
}

func TestBuildToolingPolicyUsesFamilyGuardForProblematicIntentFamily(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "gpt-4o-mini"}
	for i := 0; i < 4; i++ {
		RecordScopedToolResultForTool(scope, "homepage", false)
	}

	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.Agent.MaxToolGuides = 5

	policy := buildToolingPolicy(cfg, "please deploy the homepage to netlify")

	if policy.TelemetryProfile != "family_guarded" {
		t.Fatalf("telemetry profile = %s, want family_guarded", policy.TelemetryProfile)
	}
	if policy.IntentFamily != "deployment" {
		t.Fatalf("intent family = %s, want deployment", policy.IntentFamily)
	}
	if policy.FamilyTelemetry.ToolCalls != 4 || policy.FamilyTelemetry.ToolFailures != 4 {
		t.Fatalf("unexpected family telemetry: %+v", policy.FamilyTelemetry)
	}
	if policy.EffectiveMaxToolGuides != 4 {
		t.Fatalf("effective guide budget = %d, want 4", policy.EffectiveMaxToolGuides)
	}
	if !policy.EffectiveGuideStrategy.PreferSemantics {
		t.Fatal("expected family-guarded profile to prefer semantics")
	}
}

func TestCalculateEffectivePromptTokenBudgetScalesForHomepageFlow(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SystemPromptTokenBudget = 12000
	cfg.Agent.ContextWindow = 64000
	cfg.CircuitBreaker.MaxToolCalls = 10
	cfg.Homepage.Enabled = true
	cfg.Homepage.CircuitBreakerMaxCalls = 35
	cfg.Homepage.AllowTemporaryTokenBudgetOverflow = true

	got := calculateEffectivePromptTokenBudget(cfg, ToolCall{Action: "homepage"}, false, nil)

	if got != 42000 {
		t.Fatalf("effective prompt token budget = %d, want 42000", got)
	}
}

func TestCalculateEffectivePromptTokenBudgetKeepsBaseWhenHomepageOverflowDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SystemPromptTokenBudget = 12000
	cfg.Agent.AdaptiveSystemPromptTokenBudget = false
	cfg.CircuitBreaker.MaxToolCalls = 10
	cfg.Homepage.Enabled = true
	cfg.Homepage.CircuitBreakerMaxCalls = 35
	cfg.Homepage.AllowTemporaryTokenBudgetOverflow = false

	got := calculateEffectivePromptTokenBudget(cfg, ToolCall{Action: "homepage"}, false, nil)

	if got != 12000 {
		t.Fatalf("effective prompt token budget = %d, want 12000", got)
	}
}

func TestCalculateEffectivePromptTokenBudgetAddsAdaptiveBaseSurcharge(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SystemPromptTokenBudget = 12000
	cfg.Agent.AdaptiveSystemPromptTokenBudget = true
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Docker.Enabled = true

	got := calculateEffectivePromptTokenBudget(cfg, ToolCall{}, false, nil)

	if got != 12256 {
		t.Fatalf("effective prompt token budget = %d, want 12256", got)
	}
}

func TestCalculateEffectivePromptTokenBudgetHomepageScalesAdaptiveBase(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SystemPromptTokenBudget = 12000
	cfg.Agent.AdaptiveSystemPromptTokenBudget = true
	cfg.Tools.Memory.Enabled = true
	cfg.Docker.Enabled = true
	cfg.Agent.ContextWindow = 64000
	cfg.CircuitBreaker.MaxToolCalls = 10
	cfg.Homepage.Enabled = true
	cfg.Homepage.CircuitBreakerMaxCalls = 35
	cfg.Homepage.AllowTemporaryTokenBudgetOverflow = true

	got := calculateEffectivePromptTokenBudget(cfg, ToolCall{Action: "homepage"}, false, nil)

	if got != 43288 {
		t.Fatalf("effective prompt token budget = %d, want 43288", got)
	}
}

func TestBuildToolingPolicyDisablesNativeFunctionsForGLMModels(t *testing.T) {
	cases := []struct {
		name  string
		model string
	}{
		{"GLM direct", "glm-4.7"},
		{"GLM dash prefix", "glm-4-air"},
		{"GLM via OpenRouter", "zhipuai/glm-4.7"},
		{"GLM via OpenRouter slash", "zhipuai/glm-4-air"},
		{"MiniMax", "minimax-text-01"},
		{"Abab", "abab5.5-chat"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.LLM.Model = tc.model
			cfg.LLM.UseNativeFunctions = true // would normally enable native functions

			policy := buildToolingPolicy(cfg, "")

			if policy.UseNativeFunctions {
				t.Fatalf("model %q: expected native function calling to be disabled (GLM/MiniMax family)", tc.model)
			}
		})
	}
}

func TestBuildToolingPolicyDoesNotDisableNativeFunctionsForNonGLMModels(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "gpt-4o"
	cfg.LLM.UseNativeFunctions = true

	policy := buildToolingPolicy(cfg, "")

	if !policy.UseNativeFunctions {
		t.Fatal("expected native functions to remain enabled for non-GLM model")
	}
}
