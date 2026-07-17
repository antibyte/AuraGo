package realtimespeech

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestExchangeOpenAISDPKeepsPermanentKeyServerSide(t *testing.T) {
	const permanentKey = "openai-permanent-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+permanentKey {
			t.Fatalf("Authorization = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if r.FormValue("sdp") != "offer-sdp" {
			t.Fatalf("sdp = %q", r.FormValue("sdp"))
		}
		var session map[string]interface{}
		if err := json.Unmarshal([]byte(r.FormValue("session")), &session); err != nil {
			t.Fatalf("session JSON: %v", err)
		}
		if session["model"] != "gpt-realtime-2.1" {
			t.Fatalf("model = %v", session["model"])
		}
		tools, _ := session["tools"].([]interface{})
		if len(tools) != 2 {
			t.Fatalf("tool count = %d", len(tools))
		}
		_, _ = io.WriteString(w, "answer-sdp")
	}))
	defer server.Close()

	client := NewClient()
	client.OpenAIBaseURL = server.URL
	answer, err := client.ExchangeOpenAISDP(context.Background(), config.RealtimeSpeechProfile{
		Provider: ProviderOpenAI,
		Model:    "gpt-realtime-2.1",
		Voice:    "marin",
		APIKey:   permanentKey,
	}, "offer-sdp")
	if err != nil {
		t.Fatalf("ExchangeOpenAISDP: %v", err)
	}
	if answer != "answer-sdp" || strings.Contains(answer, permanentKey) {
		t.Fatalf("unsafe or unexpected answer %q", answer)
	}
}

func TestCreateXAIClientSecretReturnsOnlyEphemeralValue(t *testing.T) {
	const permanentKey = "xai-permanent-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/realtime/client_secrets" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+permanentKey {
			t.Fatal("permanent key missing from server-side request")
		}
		var request struct {
			ExpiresAfter struct {
				Seconds int `json:"seconds"`
			} `json:"expires_after"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.ExpiresAfter.Seconds != 300 {
			t.Fatalf("client secret TTL = %d, want 300", request.ExpiresAfter.Seconds)
		}
		_, _ = io.WriteString(w, `{"value":"xai-ephemeral","expires_at":1784289900}`)
	}))
	defer server.Close()
	client := NewClient()
	client.XAIBaseURL = server.URL
	secret, err := client.CreateXAIClientSecret(context.Background(), permanentKey)
	if err != nil {
		t.Fatalf("CreateXAIClientSecret: %v", err)
	}
	encoded, _ := json.Marshal(secret)
	if secret.Value != "xai-ephemeral" || secret.ExpiresAt != 1784289900 || strings.Contains(string(encoded), permanentKey) {
		t.Fatalf("unsafe secret response: %s", encoded)
	}
}

func TestCreateGeminiEphemeralTokenIsConstrained(t *testing.T) {
	const permanentKey = "gemini-permanent-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != permanentKey {
			t.Fatal("Gemini API key missing from server-side request")
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["uses"].(float64) != 1 {
			t.Fatalf("uses = %v", payload["uses"])
		}
		setup := payload["bidiGenerateContentSetup"].(map[string]interface{})
		if setup["model"] != "models/gemini-3.1-flash-live-preview" {
			t.Fatalf("constrained model = %v", setup["model"])
		}
		tools := setup["tools"].([]interface{})
		declarations := tools[0].(map[string]interface{})["functionDeclarations"].([]interface{})
		if len(declarations) != 2 {
			t.Fatalf("function declarations = %d", len(declarations))
		}
		for _, rawDeclaration := range declarations {
			declaration := rawDeclaration.(map[string]interface{})
			if _, exists := declaration["parameters"]; exists {
				t.Fatalf("Gemini declaration %q must not use the typed parameters field", declaration["name"])
			}
			schema, ok := declaration["parametersJsonSchema"].(map[string]interface{})
			if !ok {
				t.Fatalf("Gemini declaration %q has no JSON Schema parameters", declaration["name"])
			}
			if additional, exists := schema["additionalProperties"]; !exists || additional != false {
				t.Fatalf("Gemini declaration %q lost additionalProperties=false", declaration["name"])
			}
		}
		_, _ = io.WriteString(w, `{"name":"auth_tokens/ephemeral-one"}`)
	}))
	defer server.Close()
	client := NewClient()
	client.GeminiBaseURL = server.URL
	token, err := client.CreateGeminiEphemeralToken(context.Background(), config.RealtimeSpeechProfile{
		Model:  "gemini-3.1-flash-live-preview",
		Voice:  "Kore",
		APIKey: permanentKey,
	})
	if err != nil {
		t.Fatalf("CreateGeminiEphemeralToken: %v", err)
	}
	encoded, _ := json.Marshal(token)
	if token.Value != "auth_tokens/ephemeral-one" || strings.Contains(string(encoded), permanentKey) {
		t.Fatalf("unsafe token response: %s", encoded)
	}
}

func TestDecodeXAIVoicesSupportsDocumentedShapes(t *testing.T) {
	voices, err := decodeXAIVoices([]byte(`{"voices":[{"voice_id":"ara","name":"Ara"},{"voice_id":"rex","name":"Rex"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) != 2 || voices[0].ID != "ara" || voices[0].Label != "Ara" || voices[1].ID != "rex" {
		t.Fatalf("voices = %+v", voices)
	}
}
