package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"

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
