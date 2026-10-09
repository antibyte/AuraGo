package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

func TestInitAgentLoopStateSetsEnabledToolsForTextMode(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	cfg := &config.Config{}
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.PromptsDir = t.TempDir()
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "glm-4"

	state := initAgentLoopState(openai.ChatCompletionRequest{
		Model: "glm-4",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "Nutze Tools im Textmodus."},
		},
	}, RunConfig{
		Config:    cfg,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionID: "sess-text-mode-tools",
	}, nil, false)

	if state.useNativeFunctions {
		t.Fatal("expected text tool mode for GLM-family model")
	}
	if len(state.flags.EnabledNativeTools) == 0 {
		t.Fatal("EnabledNativeTools is empty in text tool mode")
	}
	if len(state.flags.ActiveNativeTools) == 0 {
		t.Fatal("ActiveNativeTools is empty in text tool mode")
	}
	if !containsName(state.flags.EnabledNativeTools, "discover_tools") {
		t.Fatalf("EnabledNativeTools missing discover_tools: %v", state.flags.EnabledNativeTools)
	}
}

func TestInitAgentLoopStateKeepsExplicitHumanIntentAcrossTextToolResults(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.PromptsDir = t.TempDir()
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "glm-4"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "Prüfe den Docker-Status."},
		{Role: openai.ChatMessageRoleAssistant, Content: `{"action":"docker"}`},
		{Role: openai.ChatMessageRoleUser, Content: `Tool Output: {"containers":["postgres"]}`},
	}

	explicit := initAgentLoopState(openai.ChatCompletionRequest{Model: "glm-4", Messages: messages}, RunConfig{
		Config: cfg, Logger: logger, SessionID: "intent-explicit", UserIntent: "Nur den menschlichen Auftrag verwenden.",
	}, nil, false)
	if explicit.initialUserMsg != "Nur den menschlichen Auftrag verwenden." {
		t.Fatalf("explicit intent drifted: initial=%q", explicit.initialUserMsg)
	}

	fallback := initAgentLoopState(openai.ChatCompletionRequest{Model: "glm-4", Messages: messages}, RunConfig{
		Config: cfg, Logger: logger, SessionID: "intent-fallback",
	}, nil, false)
	if fallback.initialUserMsg != "Prüfe den Docker-Status." {
		t.Fatalf("fallback intent = %q, want original human message", fallback.initialUserMsg)
	}
}

func TestInitAgentLoopStateSuppressesTTSForRealtimeSpeechRequest(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.PromptsDir = t.TempDir()
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.LLM.UseNativeFunctions = true
	cfg.TTS.Provider = "google"

	state := initAgentLoopState(openai.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleUser,
			Content: "Prüfe den Gerätestatus.",
		}},
	}, RunConfig{
		Config:            cfg,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionID:         "realtime-speech-tts-suppression",
		VoiceOutputActive: true,
	}, nil, true)

	if !state.voiceOutputSuppressed {
		t.Fatal("request-local voice output suppression was not retained")
	}
	if state.flags.IsVoiceMode || state.flags.VoiceOutputActive {
		t.Fatalf("voice flags were not suppressed: %+v", state.flags)
	}
	if containsName(toolNames(state.req.Tools), "tts") {
		t.Fatal("TTS schema was exposed to a realtime speech action")
	}
}

func TestInitAgentLoopStateSuppressesWriterCoAgentToolSchemas(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})

	cfg := &config.Config{}
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.PromptsDir = t.TempDir()
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.LLM.UseNativeFunctions = true

	state := initAgentLoopState(openai.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "Schreibe eine kurze Geschichte."},
		},
	}, RunConfig{
		Config:            cfg,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		SessionID:         "specialist-writer-1",
		IsCoAgent:         true,
		CoAgentSpecialist: "writer",
	}, nil, false)

	if state.useNativeFunctions {
		t.Fatal("expected native functions to be disabled for writer co-agents")
	}
	if state.flags.NativeToolsEnabled {
		t.Fatal("expected prompt flags to disable native tools for writer co-agents")
	}
	if len(state.req.Tools) != 0 {
		t.Fatalf("writer co-agent request has %d native tool schemas, want 0", len(state.req.Tools))
	}
	if len(state.flags.EnabledNativeTools) != 0 {
		t.Fatalf("writer co-agent enabled tools = %v, want none", state.flags.EnabledNativeTools)
	}
}

// The first selection consumes discover_tools requests already marked for
// the run (pre-existing behaviour); the refresh then pins them like the
// requests it consumes itself.
func TestInitAgentLoopStateRecordsConsumedDiscoverRequests(t *testing.T) {
	t.Cleanup(func() {
		discoverToolsState.mu.Lock()
		discoverToolsState.snapshots = nil
		discoverToolsState.requested = nil
		discoverToolsState.mu.Unlock()
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	cfg := &config.Config{}
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.PromptsDir = t.TempDir()
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "gpt-4o-mini"
	cfg.LLM.UseNativeFunctions = true
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 10
	cfg.Agent.AdaptiveTools.MaxTotalTools = 20
	MarkDiscoverRequestedTool("run:init-consume", "jellyfin")
	state := initAgentLoopState(openai.ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Was sagt Wikipedia über Berlin?"}},
	}, RunConfig{Config: cfg, Logger: logger, SessionID: "sess-init-consume", DiscoveryRunID: "run:init-consume", ShortTermMem: stm}, nil, false)
	if !state.adaptiveInitFiltered {
		t.Fatal("the adaptive first selection did not run")
	}
	if !state.discoverRequestedTools["jellyfin"] {
		t.Fatalf("consumed request not recorded: %v", state.discoverRequestedTools)
	}
	if again := ConsumeDiscoverRequestedTools("run:init-consume"); len(again) != 0 {
		t.Fatalf("the first selection left the request unconsumed: %v", again)
	}
}
