package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/voice"

	"github.com/sashabaranov/go-openai"
)

func telephoneAgentTestConfig(t *testing.T) *config.Config {
	t.Helper()
	var sipCfg config.SIPConfig
	config.ApplySIPDefaults(&sipCfg)
	cfg := &config.Config{
		SIP: sipCfg,
		Providers: []config.ProviderEntry{
			{ID: "phone-agent", Name: "Phone agent", Type: "openai", Model: "agent-model", APIKey: "agent-secret"},
			{ID: "phone-asr", Name: "Phone ASR", Type: "openai", Model: "asr-model", APIKey: "asr-secret"},
		},
	}
	cfg.LLM.Provider = "phone-agent"
	cfg.Whisper.Provider = "phone-asr"
	cfg.Whisper.Mode = "whisper"
	cfg.TTS.Provider = "google"
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Directories.ToolsDir = t.TempDir()
	return cfg
}

func TestSIPAgentGETMaterializesLegacyProviderSelection(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	server := &Server{Cfg: cfg}
	recorder := httptest.NewRecorder()
	handleSIPAgent(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/sip/agent", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Config    sipAgentPayload `json:"config"`
		Inherited map[string]bool `json:"inherited"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Config.Voice.AgentProviderID != "phone-agent" ||
		response.Config.Voice.Classic.ASRProviderID != "phone-asr" ||
		response.Config.Voice.Classic.ASRMode != "whisper" ||
		response.Config.Voice.Classic.TTSProvider != "google" {
		t.Fatalf("legacy telephone providers were not resolved: %+v", response.Config.Voice)
	}
	for _, key := range []string{"agent_provider_id", "asr_provider_id", "asr_mode", "tts_provider"} {
		if !response.Inherited[key] {
			t.Fatalf("inheritance marker %q = false", key)
		}
	}
}

func TestSIPAgentCatalogIsSecretFreeAndReportsReadiness(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	cfg.RealtimeSpeech.Profiles = []config.RealtimeSpeechProfile{{
		ID: "phone-live", Name: "Phone Live", Provider: "gemini", Model: "gemini-live", Enabled: true, APIKey: "gemini-secret",
	}}
	server := &Server{Cfg: cfg}
	recorder := httptest.NewRecorder()
	handleSIPAgentCatalog(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/sip/agent/catalog", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, secret := range []string{"agent-secret", "asr-secret", "gemini-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("catalog leaked secret %q", secret)
		}
	}
	var response struct {
		Providers []sipAgentProviderOption `json:"providers"`
		Realtime  []sipAgentNamedOption    `json:"realtime_profiles"`
		Tools     []sipAgentToolOption     `json:"tools"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Providers) != 2 || !response.Providers[0].Ready || !response.Providers[1].Ready {
		t.Fatalf("provider readiness = %+v", response.Providers)
	}
	if len(response.Realtime) != 1 || !response.Realtime[0].Ready {
		t.Fatalf("realtime readiness = %+v", response.Realtime)
	}
	if len(response.Tools) == 0 {
		t.Fatal("telephone tool catalog is unexpectedly empty")
	}
}

func TestSIPAgentMutationRequiresSameOriginAndStrictSchema(t *testing.T) {
	server := &Server{Cfg: telephoneAgentTestConfig(t)}
	for _, test := range []struct {
		name   string
		origin string
		body   string
		status int
	}{
		{name: "cross origin", origin: "https://attacker.example", body: `{}`, status: http.StatusForbidden},
		{name: "unknown field", origin: "https://aurago.local", body: `{"unknown":true}`, status: http.StatusBadRequest},
		{name: "trailing object", origin: "https://aurago.local", body: `{} {}`, status: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "https://aurago.local/api/sip/agent", strings.NewReader(test.body))
			request.Header.Set("Origin", test.origin)
			recorder := httptest.NewRecorder()
			handleSIPAgent(server).ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSIPAgentStatusReportsRouteSpecificBlockersWithoutBlockingPipelineTest(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	server := &Server{Cfg: cfg}
	voiceCfg := effectiveSIPVoiceConfig(cfg, cfg.SIP.Voice)
	joined := strings.Join(sipAgentBlockers(server, cfg.SIP, voiceCfg), ",")
	for _, blocker := range []string{"sip_disabled", "sip_readonly", "inbound_permission_disabled", "outbound_permission_disabled"} {
		if !strings.Contains(joined, blocker) {
			t.Fatalf("missing blocker %q in %s", blocker, joined)
		}
	}

	request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/agent/test", strings.NewReader(`{"live":false}`))
	request.Header.Set("Origin", "https://aurago.local")
	recorder := httptest.NewRecorder()
	handleSIPAgentTest(server, nil).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status=%d content-type=%q body=%s", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}

	cfg.SIP.Inbound.Route = "manual"
	joined = strings.Join(sipAgentBlockers(server, cfg.SIP, voiceCfg), ",")
	if strings.Contains(joined, "inbound_permission_disabled") {
		t.Fatalf("manual inbound route was blocked by agent answer permission: %s", joined)
	}
}

func TestSIPAgentPreflightRejectsUnavailableToolScope(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	cfg.SIP.Voice.AllowedTools = []string{"telephone_tool_that_does_not_exist"}
	server := &Server{Cfg: cfg}
	voiceCfg := effectiveSIPVoiceConfig(cfg, cfg.SIP.Voice)
	if blockers := strings.Join(sipAgentBlockers(server, cfg.SIP, voiceCfg), ","); !strings.Contains(blockers, "tool_scope_unavailable") {
		t.Fatalf("blockers = %s", blockers)
	}
	request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/agent/test", strings.NewReader(`{"live":false}`))
	request.Header.Set("Origin", "https://aurago.local")
	recorder := httptest.NewRecorder()
	handleSIPAgentTest(server, nil).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d cache-control=%q body=%s", recorder.Code, recorder.Header().Get("Cache-Control"), recorder.Body.String())
	}
	runner := NewVoiceActionRunner(server)
	if _, err := runner.backendFactory(context.Background(), voiceCfg); err == nil || !strings.Contains(err.Error(), "unknown or unavailable") {
		t.Fatalf("backend preflight error = %v", err)
	}
}

func TestTelephoneAgentToolCatalogExcludesBroadMetaTools(t *testing.T) {
	for _, name := range []string{
		"discover_tools", "invoke_tool", "execute_skill", "list_agent_skills",
		"activate_agent_skill", "run_agent_skill_script", "run_tool",
	} {
		if telephoneAgentToolAllowed(name) {
			t.Fatalf("broad meta-tool %q is allowed in telephone catalog", name)
		}
	}
	if !telephoneAgentToolAllowed("get_weather") {
		t.Fatal("ordinary scoped tool was unexpectedly rejected")
	}
}

type sipLiveTestClient struct {
	calls int
}

func (c *sipLiveTestClient) CreateChatCompletion(_ context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.calls++
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "OK"},
	}}}, nil
}

func (*sipLiveTestClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, fmt.Errorf("streaming is not expected")
}

type sipLiveTestSynthesizer struct {
	calls int
}

func (s *sipLiveTestSynthesizer) Synthesize(_ context.Context, text, _ string) ([]int16, int, error) {
	s.calls++
	if text != "OK" {
		return nil, 0, fmt.Errorf("unexpected TTS text %q", text)
	}
	return make([]int16, 160), 8000, nil
}

type sipLiveTestRecognizer struct {
	calls int
}

func (r *sipLiveTestRecognizer) Recognize(_ context.Context, wav []byte, sampleRate int, _ string) (string, error) {
	r.calls++
	if len(wav) < 44 || sampleRate != 8000 {
		return "", fmt.Errorf("invalid ASR probe audio")
	}
	return "OK", nil
}

func TestClassicTelephoneLiveTestExercisesLLMTTSAndASR(t *testing.T) {
	client := &sipLiveTestClient{}
	synthesizer := &sipLiveTestSynthesizer{}
	recognizer := &sipLiveTestRecognizer{}
	classic := &voice.ClassicBackend{Synthesizer: synthesizer, Recognizer: recognizer}
	if err := runClassicSIPAgentLiveTest(context.Background(), client, classic, "phone-model", "de"); err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 || synthesizer.calls != 1 || recognizer.calls != 1 {
		t.Fatalf("live test stages: LLM=%d TTS=%d ASR=%d", client.calls, synthesizer.calls, recognizer.calls)
	}
}

func TestSIPAgentPUTPersistsMaterializedProviderIDs(t *testing.T) {
	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	template = bytes.Replace(template, []byte("providers: []"), []byte(`providers:
  - id: phone-agent
    name: Phone Agent
    type: ollama
    base_url: http://127.0.0.1:11434/v1
    model: phone-agent-model
  - id: phone-asr
    name: Phone ASR
    type: ollama
    base_url: http://127.0.0.1:11434/v1
    model: phone-asr-model`), 1)
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, template, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	loaded.Directories.SkillsDir = t.TempDir()
	loaded.Directories.ToolsDir = t.TempDir()
	vault, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(tempDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Cfg: loaded, Vault: vault}

	voiceCfg := loaded.SIP.Voice
	voiceCfg.AgentProviderID = "phone-agent"
	voiceCfg.Classic.ASRProviderID = "phone-asr"
	voiceCfg.Classic.ASRMode = "whisper"
	voiceCfg.Classic.TTSProvider = "google"
	body, err := json.Marshal(sipAgentPayload{
		InboundRoute: loaded.SIP.Inbound.Route, AutoAnswerDelayMS: loaded.SIP.Inbound.AutoAnswerDelayMS, Voice: voiceCfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "https://aurago.local/api/sip/agent", bytes.NewReader(body))
	request.Header.Set("Origin", "https://aurago.local")
	recorder := httptest.NewRecorder()
	handleSIPAgent(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	snapshot := server.ConfigSnapshot()
	if snapshot.SIP.Voice.AgentProviderID != "phone-agent" ||
		snapshot.SIP.Voice.Classic.ASRProviderID != "phone-asr" ||
		snapshot.SIP.Voice.Classic.TTSProvider != "google" {
		t.Fatalf("saved telephone references = %+v", snapshot.SIP.Voice)
	}
	saved, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"agent_provider_id: phone-agent", "asr_provider_id: phone-asr", "tts_provider: google"} {
		if !strings.Contains(string(saved), expected) {
			t.Fatalf("saved config omitted %q", expected)
		}
	}
}

func TestTelephoneAgentPromptOnlyAddsRestrictions(t *testing.T) {
	var sipCfg config.SIPConfig
	config.ApplySIPDefaults(&sipCfg)
	sipCfg.Voice.Behavior.Purpose = "Provide appointment status."
	sipCfg.Voice.Behavior.SpeakingStyle = "Use short German sentences."
	sipCfg.Voice.Behavior.AdditionalProhibitions = "Never disclose private contact data."
	prompt := telephoneAgentPrompt(sipCfg.Voice)
	for _, expected := range []string{
		"untrusted external input",
		"identity or security rules",
		"Provide appointment status.",
		"Use short German sentences.",
		"Never disclose private contact data.",
		"instead of guessing or improvising",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("telephone prompt omitted %q: %s", expected, prompt)
		}
	}
}

func TestTelephoneAgentPromptBindsExplainAndEndToPipelineControl(t *testing.T) {
	var sipCfg config.SIPConfig
	config.ApplySIPDefaults(&sipCfg)
	sipCfg.Voice.Behavior.UnavailableRequestBehavior = "explain_and_end"

	sipCfg.Voice.Backend = "classic"
	if prompt := telephoneAgentPrompt(sipCfg.Voice); !strings.Contains(prompt, voice.EndCallResponseMarker) {
		t.Fatalf("classic explain-and-end prompt lacks private marker: %s", prompt)
	}
	sipCfg.Voice.Backend = "gemini_live"
	prompt := telephoneAgentPrompt(sipCfg.Voice)
	if !strings.Contains(prompt, "aurago_end_call") || strings.Contains(prompt, voice.EndCallResponseMarker) {
		t.Fatalf("Gemini explain-and-end prompt has wrong control: %s", prompt)
	}
}

func TestTelephoneProviderReferencesProtectAgentAndASRProviders(t *testing.T) {
	cfg := &config.Config{}
	cfg.SIP.Voice.AgentProviderID = "phone-agent"
	cfg.SIP.Voice.Classic.ASRProviderID = "phone-asr"
	for _, test := range []struct {
		id   string
		path string
		role string
	}{
		{id: "phone-agent", path: "sip.voice.agent_provider_id", role: "telephone_agent_llm"},
		{id: "phone-asr", path: "sip.voice.classic.asr_provider_id", role: "telephone_agent_asr"},
	} {
		refs := providerReferences(cfg, test.id)
		if len(refs) != 1 || refs[0].Path != test.path || refs[0].Role != test.role {
			t.Fatalf("provider %q refs = %+v", test.id, refs)
		}
	}
}
