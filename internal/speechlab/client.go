package speechlab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
)

const (
	readyPath       = "/ready"
	capabilityPath  = "/api/v1/capability"
	catalogPath     = "/api/v1/catalog"
	suggestionsPath = "/api/v1/suggestions"
	stackPath       = "/api/v1/stack"
	asrPath         = "/v1/audio/transcriptions"
	ttsPath         = "/v1/audio/speech"

	maxJSONBytes     = 1 << 20
	MaxASRBytes      = 8 << 20
	maxTTSBytes      = 32 << 20
	readinessTimeout = 20 * time.Second
	stackTimeout     = 180 * time.Second
)

// Ready is the stable s2s readiness response.
type Ready struct {
	Ready    bool   `json:"ready"`
	ASRID    string `json:"asr_id"`
	TTSID    string `json:"tts_id"`
	ASROK    bool   `json:"asr_ok"`
	TTSOK    bool   `json:"tts_ok"`
	Message  string `json:"message"`
	Language string `json:"language,omitempty"`
	Voice    string `json:"voice,omitempty"`
}

// StackRequest intentionally omits llm_id. AuraGo never owns the s2s LLM.
type StackRequest struct {
	ASRID string `json:"asr_id"`
	TTSID string `json:"tts_id"`
	Voice string `json:"voice,omitempty"`
}

type StackResult struct {
	OK      bool            `json:"ok"`
	Message string          `json:"message"`
	Runtime json.RawMessage `json:"runtime,omitempty"`
}

type Catalog struct {
	SchemaVersion int              `json:"schema_version"`
	Backends      []CatalogBackend `json:"backends"`
	Presets       json.RawMessage  `json:"presets,omitempty"`
}

type CatalogBackend struct {
	ID            string   `json:"id"`
	Stage         string   `json:"stage"`
	Name          string   `json:"name"`
	Protocol      string   `json:"protocol"`
	DefaultVoice  string   `json:"default_voice"`
	Voices        []string `json:"voices"`
	Languages     []string `json:"languages"`
	Available     bool     `json:"available"`
	Reason        string   `json:"reason"`
	Installed     bool     `json:"installed"`
	DownloadState string   `json:"download_state"`
	HostManaged   bool     `json:"host_managed"`
	RuntimeState  string   `json:"runtime_state"`
	RuntimeReason string   `json:"runtime_reason"`
}

type Transcript struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
	ASRID    string `json:"asr_id"`
}

// NotReadyError is safe to return to callers and UI surfaces.
type NotReadyError struct {
	NeedASR bool
	NeedTTS bool
	Status  Ready
	Cause   error
}

func (e *NotReadyError) Error() string {
	return "Speech Lab is not ready for the requested speech component"
}

func (e *NotReadyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func ErrorCode(err error) string {
	var notReady *NotReadyError
	if errors.As(err, &notReady) {
		return "speech_lab_not_ready"
	}
	return "speech_lab_unavailable"
}

// Client is safe for concurrent use. Reconfigure swaps the immutable runtime
// configuration without interrupting operations already in flight.
type Client struct {
	cfg         atomic.Pointer[config.SpeechLabConfig]
	operations  atomic.Int64
	operationMu sync.RWMutex
}

func NewClient(cfg config.SpeechLabConfig) (*Client, error) {
	config.NormalizeSpeechLabConfig(&cfg, nil)
	if err := config.ValidateSpeechLabConfig(cfg); err != nil {
		return nil, err
	}
	c := &Client{}
	c.Reconfigure(cfg)
	return c, nil
}

func (c *Client) Reconfigure(cfg config.SpeechLabConfig) {
	copy := cfg
	c.cfg.Store(&copy)
}

func (c *Client) Config() config.SpeechLabConfig {
	if c == nil || c.cfg.Load() == nil {
		return config.SpeechLabConfig{}
	}
	return *c.cfg.Load()
}

func (c *Client) ActiveOperations() int64 {
	if c == nil {
		return 0
	}
	return c.operations.Load()
}

func (c *Client) Ready(ctx context.Context) (Ready, error) {
	cfg, err := c.activeConfig()
	if err != nil {
		return Ready{}, err
	}
	ctx, cancel := boundedContext(ctx, readinessTimeout)
	defer cancel()
	body, status, err := c.do(ctx, cfg, http.MethodGet, readyPath, "", nil, maxJSONBytes, false)
	if err != nil {
		return Ready{}, fmt.Errorf("speech lab readiness: %w", err)
	}
	var ready Ready
	if err := json.Unmarshal(body, &ready); err != nil {
		return Ready{}, fmt.Errorf("speech lab readiness response: %w", err)
	}
	if status != http.StatusOK && status != http.StatusServiceUnavailable {
		return Ready{}, fmt.Errorf("speech lab readiness returned HTTP %d", status)
	}
	return ready, nil
}

func (c *Client) Require(ctx context.Context, needASR, needTTS bool) (Ready, error) {
	ready, err := c.Ready(ctx)
	if err != nil {
		return Ready{}, &NotReadyError{NeedASR: needASR, NeedTTS: needTTS, Cause: err}
	}
	if (needASR && !ready.ASROK) || (needTTS && !ready.TTSOK) ||
		(needASR && strings.TrimSpace(ready.ASRID) == "") ||
		(needTTS && strings.TrimSpace(ready.TTSID) == "") {
		return ready, &NotReadyError{NeedASR: needASR, NeedTTS: needTTS, Status: ready}
	}
	return ready, nil
}

func (c *Client) Capability(ctx context.Context) (json.RawMessage, error) {
	return c.getJSON(ctx, capabilityPath, "")
}

func (c *Client) Suggestions(ctx context.Context, values url.Values) (json.RawMessage, error) {
	allowed := url.Values{}
	for _, key := range []string{"language", "stable_only", "max_vram_gb", "limit"} {
		if value := strings.TrimSpace(values.Get(key)); value != "" {
			allowed.Set(key, value)
		}
	}
	return c.getJSON(ctx, suggestionsPath, allowed.Encode())
}

func (c *Client) Catalog(ctx context.Context) (json.RawMessage, Catalog, error) {
	raw, err := c.getJSON(ctx, catalogPath, "")
	if err != nil {
		return nil, Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(raw, &catalog); err != nil {
		return nil, Catalog{}, fmt.Errorf("speech lab catalog response: %w", err)
	}
	return raw, catalog, nil
}

func (c *Client) ActivateStack(ctx context.Context, request StackRequest) (StackResult, Ready, error) {
	if !c.operationMu.TryLock() {
		return StackResult{}, Ready{}, fmt.Errorf("speech lab stack is busy")
	}
	defer c.operationMu.Unlock()
	ctx, cancel := boundedContext(ctx, stackTimeout)
	defer cancel()
	request.ASRID = strings.TrimSpace(request.ASRID)
	request.TTSID = strings.TrimSpace(request.TTSID)
	request.Voice = strings.TrimSpace(request.Voice)
	if request.ASRID == "" || request.TTSID == "" {
		return StackResult{}, Ready{}, fmt.Errorf("asr_id and tts_id are required")
	}
	_, catalog, err := c.Catalog(ctx)
	if err != nil {
		return StackResult{}, Ready{}, err
	}
	if err := validateStackRequest(catalog, request); err != nil {
		return StackResult{}, Ready{}, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return StackResult{}, Ready{}, err
	}
	cfg, err := c.activeConfig()
	if err != nil {
		return StackResult{}, Ready{}, err
	}
	stackCfg := cfg
	stackCfg.TimeoutSeconds = int(stackTimeout / time.Second)
	body, status, err := c.do(ctx, stackCfg, http.MethodPut, stackPath, "application/json", bytes.NewReader(payload), maxJSONBytes, false)
	if err != nil {
		return StackResult{}, Ready{}, fmt.Errorf("speech lab stack activation: %w", err)
	}
	var result StackResult
	if err := json.Unmarshal(body, &result); err != nil {
		return StackResult{}, Ready{}, fmt.Errorf("speech lab stack response: %w", err)
	}
	if status < 200 || status >= 300 || !result.OK {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "stack activation failed"
		}
		return result, Ready{}, fmt.Errorf("speech lab stack activation: %s", message)
	}
	ready, err := c.Require(ctx, true, true)
	return result, ready, err
}

func (c *Client) Transcribe(ctx context.Context, wav []byte, language, expectedASRID string) (Transcript, error) {
	if len(wav) > MaxASRBytes {
		return Transcript{}, fmt.Errorf("speech lab audio exceeds %d bytes", MaxASRBytes)
	}
	if err := ValidatePCM16WAV(wav); err != nil {
		return Transcript{}, err
	}
	cfg, err := c.activeConfig()
	if err != nil {
		return Transcript{}, err
	}
	c.operationMu.RLock()
	defer c.operationMu.RUnlock()
	c.operations.Add(1)
	defer c.operations.Add(-1)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="audio.wav"`)
	partHeader.Set("Content-Type", "audio/wav")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return Transcript{}, err
	}
	if _, err := part.Write(wav); err != nil {
		return Transcript{}, err
	}
	language = strings.TrimSpace(language)
	if language == "" {
		language = cfg.Language
	}
	if language != "" && !strings.EqualFold(language, "auto") {
		if err := writer.WriteField("language", language); err != nil {
			return Transcript{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return Transcript{}, err
	}
	bodyBytes, status, err := c.do(ctx, cfg, http.MethodPost, asrPath, writer.FormDataContentType(), &body, maxJSONBytes, true)
	if err != nil {
		return Transcript{}, fmt.Errorf("speech lab ASR: %w", err)
	}
	if status < 200 || status >= 300 {
		return Transcript{}, fmt.Errorf("speech lab ASR returned HTTP %d", status)
	}
	var result Transcript
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return Transcript{}, fmt.Errorf("speech lab ASR response: %w", err)
	}
	result.Text = strings.TrimSpace(result.Text)
	if result.Text == "" {
		return Transcript{}, fmt.Errorf("speech lab ASR returned an empty transcript")
	}
	if expected := strings.TrimSpace(expectedASRID); expected != "" && result.ASRID != expected {
		return Transcript{}, fmt.Errorf("speech lab ASR backend changed during operation")
	}
	return result, nil
}

func (c *Client) Synthesize(ctx context.Context, text, language, voice, expectedTTSID string) ([]byte, string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, "", fmt.Errorf("text is required")
	}
	cfg, err := c.activeConfig()
	if err != nil {
		return nil, "", err
	}
	c.operationMu.RLock()
	defer c.operationMu.RUnlock()
	c.operations.Add(1)
	defer c.operations.Add(-1)
	if strings.TrimSpace(language) == "" {
		language = cfg.Language
	}
	voice = strings.TrimSpace(voice)
	payload, err := json.Marshal(map[string]string{
		"input": text, "language": language, "voice": voice, "response_format": "wav",
	})
	if err != nil {
		return nil, "", err
	}
	body, status, headers, err := c.doWithHeaders(ctx, cfg, http.MethodPost, ttsPath, "application/json", bytes.NewReader(payload), maxTTSBytes, true)
	if err != nil {
		return nil, "", fmt.Errorf("speech lab TTS: %w", err)
	}
	if status < 200 || status >= 300 {
		return nil, "", fmt.Errorf("speech lab TTS returned HTTP %d", status)
	}
	if !strings.Contains(strings.ToLower(headers.Get("Content-Type")), "audio/wav") {
		return nil, "", fmt.Errorf("speech lab TTS returned an unsupported audio format")
	}
	if err := ValidateWAV(body); err != nil {
		return nil, "", fmt.Errorf("speech lab TTS returned invalid WAV: %w", err)
	}
	ttsID := strings.TrimSpace(headers.Get("X-S2S-TTS-ID"))
	if expected := strings.TrimSpace(expectedTTSID); expected != "" && ttsID != expected {
		return nil, "", fmt.Errorf("speech lab TTS backend changed during operation")
	}
	return body, ttsID, nil
}

func (c *Client) getJSON(ctx context.Context, path, query string) (json.RawMessage, error) {
	cfg, err := c.activeConfig()
	if err != nil {
		return nil, err
	}
	if query != "" {
		path += "?" + query
	}
	body, status, err := c.do(ctx, cfg, http.MethodGet, path, "", nil, maxJSONBytes, false)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("speech lab returned HTTP %d", status)
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("speech lab returned invalid JSON")
	}
	return json.RawMessage(body), nil
}

func (c *Client) activeConfig() (config.SpeechLabConfig, error) {
	cfg := c.Config()
	if !cfg.Active() {
		return cfg, fmt.Errorf("speech lab is disabled")
	}
	if err := config.ValidateSpeechLabConfig(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Client) do(ctx context.Context, cfg config.SpeechLabConfig, method, path, contentType string, body io.Reader, limit int64, operation bool) ([]byte, int, error) {
	responseBody, status, _, err := c.doWithHeaders(ctx, cfg, method, path, contentType, body, limit, operation)
	return responseBody, status, err
}

func (c *Client) doWithHeaders(ctx context.Context, cfg config.SpeechLabConfig, method, path, contentType string, body io.Reader, limit int64, operation bool) ([]byte, int, http.Header, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if operation {
		var cancel context.CancelFunc
		ctx, cancel = boundedContext(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(cfg.BaseURL, "/")+path, body)
	if err != nil {
		return nil, 0, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json, audio/wav")
	client := &http.Client{
		Transport: privateTransport(),
		Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	data, err := readBounded(resp.Body, limit)
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Clone(), err
	}
	return data, resp.StatusCode, resp.Header.Clone(), nil
}

func privateTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, resolved := range ips {
				if !allowedPrivateIP(resolved.IP) {
					continue
				}
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				err = dialErr
			}
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("speech lab destination %s is not private or loopback", host)
		},
		ForceAttemptHTTP2: true,
		DisableKeepAlives: true,
		MaxIdleConns:      8,
		IdleConnTimeout:   30 * time.Second,
	}
}

func allowedPrivateIP(ip net.IP) bool {
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate()) && !ip.IsUnspecified()
}

func validateStackRequest(catalog Catalog, request StackRequest) error {
	byID := make(map[string]CatalogBackend, len(catalog.Backends))
	for _, backend := range catalog.Backends {
		byID[backend.ID] = backend
	}
	asr, ok := byID[request.ASRID]
	if !ok || !asr.Available || !strings.EqualFold(asr.Stage, "asr") {
		return fmt.Errorf("selected ASR backend is not available")
	}
	tts, ok := byID[request.TTSID]
	if !ok || !tts.Available || !strings.EqualFold(tts.Stage, "tts") {
		return fmt.Errorf("selected TTS backend is not available")
	}
	if request.Voice != "" && len(tts.Voices) > 0 {
		found := false
		for _, voice := range tts.Voices {
			if request.Voice == voice {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("selected voice is not available for the TTS backend")
		}
	}
	return nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("invalid response limit")
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("speech lab response exceeds %s bytes", strconv.FormatInt(limit, 10))
	}
	return data, nil
}

func boundedContext(parent context.Context, maximum time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if deadline, ok := parent.Deadline(); ok && time.Until(deadline) <= maximum {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, maximum)
}
