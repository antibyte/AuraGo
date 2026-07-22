package realtimespeech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

const maxProviderResponseBytes = 1 << 20

// Client contains the provider-facing, server-side HTTP operations. Base URLs
// are fields to make the non-generative contract testable without live costs.
type Client struct {
	HTTPClient      *http.Client
	OpenAIBaseURL   string
	XAIBaseURL      string
	XAIWebSocketURL string
	GeminiBaseURL   string
}

// NewClient creates a provider client with conservative timeouts.
func NewClient() *Client {
	return &Client{
		HTTPClient:      &http.Client{Timeout: 20 * time.Second},
		OpenAIBaseURL:   "https://api.openai.com",
		XAIBaseURL:      "https://api.x.ai",
		XAIWebSocketURL: "wss://api.x.ai/v1/realtime",
		GeminiBaseURL:   "https://generativelanguage.googleapis.com",
	}
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (c *Client) openAIBaseURL() string {
	if c != nil && strings.TrimSpace(c.OpenAIBaseURL) != "" {
		return strings.TrimRight(c.OpenAIBaseURL, "/")
	}
	return "https://api.openai.com"
}

func (c *Client) xaiBaseURL() string {
	if c != nil && strings.TrimSpace(c.XAIBaseURL) != "" {
		return strings.TrimRight(c.XAIBaseURL, "/")
	}
	return "https://api.x.ai"
}

func (c *Client) xaiWebSocketURL() string {
	if c != nil && strings.TrimSpace(c.XAIWebSocketURL) != "" {
		return c.XAIWebSocketURL
	}
	return "wss://api.x.ai/v1/realtime"
}

func (c *Client) geminiBaseURL() string {
	if c != nil && strings.TrimSpace(c.GeminiBaseURL) != "" {
		return strings.TrimRight(c.GeminiBaseURL, "/")
	}
	return "https://generativelanguage.googleapis.com"
}

// ExchangeOpenAISDP performs the authenticated server-side SDP exchange. The
// permanent key never appears in the returned answer or browser configuration.
func (c *Client) ExchangeOpenAISDP(ctx context.Context, profile config.RealtimeSpeechProfile, offerSDP string) (string, error) {
	if strings.TrimSpace(profile.APIKey) == "" {
		return "", fmt.Errorf("OpenAI API key is not configured")
	}
	if strings.TrimSpace(offerSDP) == "" {
		return "", fmt.Errorf("offer_sdp is required for OpenAI")
	}

	session := map[string]interface{}{
		"type":         "realtime",
		"model":        profile.Model,
		"instructions": AuraGoSystemContract,
		"tools":        PrivateTools(),
		"tool_choice":  "auto",
		"audio": map[string]interface{}{
			"input": map[string]interface{}{
				"format":         map[string]interface{}{"type": "audio/pcm", "rate": 24000},
				"turn_detection": nil,
				"transcription":  map[string]interface{}{"model": "gpt-4o-mini-transcribe"},
			},
			"output": map[string]interface{}{
				"format": map[string]interface{}{"type": "audio/pcm", "rate": 24000},
				"voice":  profile.Voice,
			},
		},
	}
	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("encode OpenAI realtime session: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	sessionHeader := make(textproto.MIMEHeader)
	sessionHeader.Set("Content-Disposition", `form-data; name="session"`)
	sessionHeader.Set("Content-Type", "application/json")
	sessionPart, err := writer.CreatePart(sessionHeader)
	if err != nil {
		return "", fmt.Errorf("create OpenAI session form: %w", err)
	}
	if _, err := sessionPart.Write(sessionJSON); err != nil {
		return "", fmt.Errorf("write OpenAI session form: %w", err)
	}
	if err := writer.WriteField("sdp", offerSDP); err != nil {
		return "", fmt.Errorf("write OpenAI SDP form: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("finalize OpenAI SDP form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.openAIBaseURL()+"/v1/realtime/calls", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+profile.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI realtime SDP exchange failed: %w", err)
	}
	defer resp.Body.Close()
	payload, err := readProviderBody(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", providerStatusError("OpenAI", resp.StatusCode, payload)
	}
	answer := strings.TrimSpace(string(payload))
	if answer == "" {
		return "", fmt.Errorf("OpenAI returned an empty SDP answer")
	}
	return answer, nil
}

// XAIClientSecret is a short-lived browser credential.
type XAIClientSecret struct {
	Value     string `json:"value"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

// CreateXAIClientSecret creates a five-minute client secret.
func (c *Client) CreateXAIClientSecret(ctx context.Context, apiKey string) (XAIClientSecret, error) {
	var result XAIClientSecret
	body := strings.NewReader(`{"expires_after":{"seconds":300}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.xaiBaseURL()+"/v1/realtime/client_secrets", body)
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return result, fmt.Errorf("xAI client secret request failed: %w", err)
	}
	defer resp.Body.Close()
	payload, err := readProviderBody(resp.Body)
	if err != nil {
		return result, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, providerStatusError("xAI", resp.StatusCode, payload)
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		return result, fmt.Errorf("decode xAI client secret: %w", err)
	}
	if strings.TrimSpace(result.Value) == "" {
		return result, fmt.Errorf("xAI returned an empty client secret")
	}
	return result, nil
}

// GeminiEphemeralToken is constrained to one new Live API session.
type GeminiEphemeralToken struct {
	Value                 string    `json:"value"`
	ExpiresAt             time.Time `json:"expires_at"`
	NewSessionExpiresAt   time.Time `json:"new_session_expires_at"`
	ConstrainedModel      string    `json:"constrained_model"`
	ConstrainedVoice      string    `json:"constrained_voice"`
	ConstrainedToolCount  int       `json:"constrained_tool_count"`
	ManualActivityEnabled bool      `json:"manual_activity_enabled"`
}

// CreateGeminiEphemeralToken creates a Developer API token constrained to the
// selected model, voice, system contract, and two private AuraGo functions.
func (c *Client) CreateGeminiEphemeralToken(ctx context.Context, profile config.RealtimeSpeechProfile) (GeminiEphemeralToken, error) {
	var result GeminiEphemeralToken
	now := time.Now().UTC()
	expireAt := now.Add(30 * time.Minute)
	newSessionExpireAt := now.Add(time.Minute)
	setup := geminiSetup(profile)
	payload := map[string]interface{}{
		"uses":                     1,
		"expireTime":               expireAt.Format(time.RFC3339),
		"newSessionExpireTime":     newSessionExpireAt.Format(time.RFC3339),
		"bidiGenerateContentSetup": setup,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return result, fmt.Errorf("encode Gemini ephemeral token request: %w", err)
	}
	endpoint := c.geminiBaseURL() + "/v1alpha/auth_tokens?key=" + url.QueryEscape(profile.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return result, fmt.Errorf("Gemini ephemeral token request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := readProviderBody(resp.Body)
	if err != nil {
		return result, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, providerStatusError("Gemini", resp.StatusCode, body)
	}
	var response struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return result, fmt.Errorf("decode Gemini ephemeral token: %w", err)
	}
	if strings.TrimSpace(response.Name) == "" {
		return result, fmt.Errorf("Gemini returned an empty ephemeral token")
	}
	result = GeminiEphemeralToken{
		Value:                 response.Name,
		ExpiresAt:             expireAt,
		NewSessionExpiresAt:   newSessionExpireAt,
		ConstrainedModel:      profile.Model,
		ConstrainedVoice:      profile.Voice,
		ConstrainedToolCount:  len(PrivateTools()),
		ManualActivityEnabled: true,
	}
	return result, nil
}

func geminiSetup(profile config.RealtimeSpeechProfile) map[string]interface{} {
	return geminiSetupWithTools(profile, PrivateTools())
}

func geminiSetupWithTools(profile config.RealtimeSpeechProfile, tools []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"model": "models/" + strings.TrimPrefix(profile.Model, "models/"),
		"generationConfig": map[string]interface{}{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]interface{}{
				"voiceConfig": map[string]interface{}{
					"prebuiltVoiceConfig": map[string]interface{}{"voiceName": profile.Voice},
				},
			},
		},
		"systemInstruction": map[string]interface{}{"parts": []map[string]string{{"text": AuraGoSystemContract}}},
		"tools": []map[string]interface{}{{
			"functionDeclarations": geminiFunctionDeclarations(tools),
		}},
		"realtimeInputConfig": map[string]interface{}{
			"automaticActivityDetection": map[string]interface{}{"disabled": true},
		},
		"inputAudioTranscription":  map[string]interface{}{},
		"outputAudioTranscription": map[string]interface{}{},
		"sessionResumption":        map[string]interface{}{},
		"contextWindowCompression": map[string]interface{}{"slidingWindow": map[string]interface{}{}},
	}
}

// GeminiSessionSetup returns the setup message the browser sends as its first
// Live API WebSocket frame. It exactly matches the ephemeral-token constraints.
func GeminiSessionSetup(profile config.RealtimeSpeechProfile) map[string]interface{} {
	return geminiSetup(profile)
}

// GeminiSIPSessionSetup is used only by AuraGo's server-side SIP connection.
// Browser ephemeral-token constraints intentionally remain unchanged.
func GeminiSIPSessionSetup(profile config.RealtimeSpeechProfile) map[string]interface{} {
	return geminiSetupWithTools(profile, SIPPrivateTools())
}

// XAISessionConfig returns the manual-turn, resumable session update used by
// the browser adapter.
func XAISessionConfig(profile config.RealtimeSpeechProfile) map[string]interface{} {
	return map[string]interface{}{
		"instructions":   AuraGoSystemContract,
		"voice":          profile.Voice,
		"tools":          PrivateTools(),
		"tool_choice":    "auto",
		"turn_detection": nil,
		"audio": map[string]interface{}{
			"input": map[string]interface{}{
				"format":        map[string]interface{}{"type": "audio/pcm", "rate": 16000},
				"transcription": map[string]interface{}{"model": "grok-transcribe"},
			},
			"output": map[string]interface{}{
				"format": map[string]interface{}{"type": "audio/pcm", "rate": 24000},
			},
		},
		"resumption": map[string]interface{}{"enabled": true},
	}
}

func geminiFunctionDeclarations(tools []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		parameters, _ := tool["parameters"].(map[string]interface{})
		out = append(out, map[string]interface{}{
			"name":        tool["name"],
			"description": tool["description"],
			"parameters":  geminiTypedSchema(parameters),
		})
	}
	return out
}

// geminiTypedSchema converts the private JSON schemas to Gemini's typed Schema
// format. The Live API rejects JSON Schema-only fields such as
// additionalProperties in bidi setup constraints.
func geminiTypedSchema(schema map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(schema))
	for key, value := range schema {
		if key == "additionalProperties" {
			continue
		}
		switch typed := value.(type) {
		case map[string]interface{}:
			out[key] = geminiTypedSchema(typed)
		case []interface{}:
			items := make([]interface{}, len(typed))
			for index, item := range typed {
				if itemSchema, ok := item.(map[string]interface{}); ok {
					items[index] = geminiTypedSchema(itemSchema)
				} else {
					items[index] = item
				}
			}
			out[key] = items
		default:
			out[key] = value
		}
	}
	return out
}

// TestProfile validates credentials, model, and voice without requesting a
// generated audio response.
func (c *Client) TestProfile(ctx context.Context, profile config.RealtimeSpeechProfile) ([]Voice, error) {
	switch profile.Provider {
	case ProviderOpenAI:
		endpoint := c.openAIBaseURL() + "/v1/models/" + url.PathEscape(profile.Model)
		return nil, c.testBearerMetadata(ctx, "OpenAI", endpoint, profile.APIKey)
	case ProviderXAI:
		voices, err := c.FetchXAIVoices(ctx, profile.APIKey)
		if err != nil {
			return nil, err
		}
		if !voiceExists(voices, profile.Voice) {
			return voices, fmt.Errorf("xAI voice %q is not available", profile.Voice)
		}
		if err := c.testXAIHandshake(ctx, profile); err != nil {
			return voices, err
		}
		return voices, nil
	case ProviderGemini:
		endpoint := c.geminiBaseURL() + "/v1beta/models/" + url.PathEscape(strings.TrimPrefix(profile.Model, "models/")) + "?key=" + url.QueryEscape(profile.APIKey)
		return nil, c.testMetadata(ctx, "Gemini", endpoint)
	default:
		return nil, fmt.Errorf("unsupported realtime speech provider %q", profile.Provider)
	}
}

func (c *Client) testBearerMetadata(ctx context.Context, provider, endpoint, apiKey string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	return c.doMetadataRequest(req, provider)
}

func (c *Client) testMetadata(ctx context.Context, provider, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	return c.doMetadataRequest(req, provider)
}

func (c *Client) doMetadataRequest(req *http.Request, provider string) error {
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("%s metadata request failed: %w", provider, err)
	}
	defer resp.Body.Close()
	payload, err := readProviderBody(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return providerStatusError(provider, resp.StatusCode, payload)
	}
	return nil
}

// FetchXAIVoices loads the authoritative xAI voice catalog.
func (c *Client) FetchXAIVoices(ctx context.Context, apiKey string) ([]Voice, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.xaiBaseURL()+"/v1/tts/voices", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("xAI voice catalog request failed: %w", err)
	}
	defer resp.Body.Close()
	payload, err := readProviderBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, providerStatusError("xAI", resp.StatusCode, payload)
	}
	return decodeXAIVoices(payload)
}

func decodeXAIVoices(payload []byte) ([]Voice, error) {
	var raw interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("decode xAI voice catalog: %w", err)
	}
	items := raw
	if object, ok := raw.(map[string]interface{}); ok {
		for _, key := range []string{"voices", "data", "items"} {
			if candidate, exists := object[key]; exists {
				items = candidate
				break
			}
		}
	}
	list, ok := items.([]interface{})
	if !ok {
		return nil, fmt.Errorf("xAI voice catalog has an unexpected shape")
	}
	voices := make([]Voice, 0, len(list))
	for _, item := range list {
		switch value := item.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				voices = append(voices, Voice{ID: value, Label: value})
			}
		case map[string]interface{}:
			id := firstString(value, "voice_id", "id", "voice", "name")
			if id == "" {
				continue
			}
			label := firstString(value, "display_name", "label", "name")
			if label == "" {
				label = id
			}
			voices = append(voices, Voice{ID: id, Label: label, Description: firstString(value, "description", "style")})
		}
	}
	if len(voices) == 0 {
		return nil, fmt.Errorf("xAI returned no voices")
	}
	return voices, nil
}

func firstString(object map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (c *Client) testXAIHandshake(ctx context.Context, profile config.RealtimeSpeechProfile) error {
	endpoint, err := url.Parse(c.xaiWebSocketURL())
	if err != nil {
		return fmt.Errorf("invalid xAI WebSocket URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("model", profile.Model)
	endpoint.RawQuery = query.Encode()
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+profile.APIKey)
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, endpoint.String(), headers)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if resp != nil {
			body, _ := readProviderBody(resp.Body)
			return providerStatusError("xAI realtime handshake", resp.StatusCode, body)
		}
		return fmt.Errorf("xAI realtime handshake failed: %w", err)
	}
	return conn.Close()
}

func readProviderBody(reader io.Reader) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxProviderResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read provider response: %w", err)
	}
	if len(payload) > maxProviderResponseBytes {
		return nil, fmt.Errorf("provider response exceeded %d bytes", maxProviderResponseBytes)
	}
	return payload, nil
}

func providerStatusError(provider string, status int, payload []byte) error {
	message := strings.TrimSpace(security.Scrub(string(payload)))
	if len(message) > 500 {
		message = message[:500]
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("%s returned HTTP %s: %s", provider, strconv.Itoa(status), message)
}
