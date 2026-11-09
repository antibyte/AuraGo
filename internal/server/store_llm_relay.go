package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/desktopstore"
	"aurago/internal/httporigin"
	"aurago/internal/llm"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

// Store LLM relay: a token-gated, OpenAI-compatible endpoint that lets a Store
// app running in Docker (DeepTutor) use the AuraGo LLM without ever seeing the
// provider key. Contract (see internal/server/AGENTS.md, "Store LLM relay"):
//
//   - Prefix /api/store-llm/deeptutor/v1 with GET /models, POST /chat/completions
//     (JSON and SSE) and POST /embeddings.
//   - The path is in authBypassSubtrees; every request authenticates itself with
//     "Authorization: Bearer <store token>", verified by the Store service for
//     the installed app. No cookie, session, CSRF or query-string path exists.
//   - The request model is ignored: chat always runs on the current AuraGo main
//     model and answers under the alias "aurago".
//   - Upstream errors are mapped to fixed OpenAI-style errors. Prompts, tokens,
//     provider keys and upstream URLs never reach logs or error bodies.
const (
	storeLLMRelayAppID      = "deeptutor" // desktopstore.DeepTutorAppID
	storeLLMRelayPrefix     = "/api/store-llm/deeptutor/v1"
	storeLLMRelayAuthPrefix = storeLLMRelayPrefix + "/"
	storeLLMRelayModelAlias = "aurago"

	storeLLMRelayMaxBodyBytes     = 8 << 20
	storeLLMRelayMaxResponseBytes = 16 << 20 // embeddings response cap, same as the memory helper
	storeLLMRelayMaxConcurrent    = 8
	storeLLMRelayChatTimeout      = 5 * time.Minute
	storeLLMRelayStreamTimeout    = 10 * time.Minute
	storeLLMRelayEmbedTimeout     = 2 * time.Minute
	storeLLMRelayMaxTokenBytes    = 512
	storeLLMRelayBudgetCategory   = "store_relay"
	storeLLMRelayDockerHostAlias  = "host.docker.internal"
	storeLLMRelayAuthLogInterval  = 30 * time.Second
	storeLLMRelayDefaultEmbedding = "text-embedding-3-small"
	storeLLMRelayMaxErrorBytes    = 300
)

// storeRelayTokenVerifier is the only part of the Store service the relay
// needs. *desktopstore.Service satisfies it.
type storeRelayTokenVerifier interface {
	VerifyRelayToken(ctx context.Context, appID, token string) bool
}

// storeLLMRelaySlots caps concurrent relay calls process-wide.
var storeLLMRelaySlots = make(chan struct{}, storeLLMRelayMaxConcurrent)

type storeLLMRelay struct {
	server      *Server
	verifier    func(ctx context.Context) (storeRelayTokenVerifier, error)
	slots       chan struct{}
	embedClient *http.Client // test seam; nil builds the house same-origin client

	lastAuthLog atomic.Int64
}

// registerStoreLLMRelayRoutes mounts the relay. The Store service is resolved
// lazily on the first authenticated-looking request.
func registerStoreLLMRelayRoutes(mux *http.ServeMux, s *Server) {
	mux.Handle(storeLLMRelayAuthPrefix, newStoreLLMRelay(s, s.storeRelayVerifier))
}

func newStoreLLMRelay(s *Server, verifier func(ctx context.Context) (storeRelayTokenVerifier, error)) *storeLLMRelay {
	return &storeLLMRelay{server: s, verifier: verifier, slots: storeLLMRelaySlots}
}

func (s *Server) storeRelayVerifier(ctx context.Context) (storeRelayTokenVerifier, error) {
	svc, err := s.getDesktopStoreService(ctx)
	if err != nil {
		return nil, err
	}
	if svc == nil {
		return nil, errors.New("store relay verifier unavailable")
	}
	return svc, nil
}

// The Store service must keep satisfying the relay's one-method contract.
var _ storeRelayTokenVerifier = (*desktopstore.Service)(nil)

// isStoreLLMRelayPath reports whether p is a clean path inside the relay
// subtree. Dot-dot segments, doubled slashes, backslashes, percent signs and
// NUL bytes never qualify, so an encoded or relative path cannot borrow the
// session bypass or the Docker host alias for another route.
func isStoreLLMRelayPath(p string) bool {
	if !strings.HasPrefix(p, storeLLMRelayAuthPrefix) {
		return false
	}
	return !strings.ContainsAny(p, "\\%\x00") && !strings.Contains(p, "..") && !strings.Contains(p, "//")
}

// storeLLMRelayHostAllowed lets a container reach the relay through the Docker
// host alias while login is disabled. The DNS-rebinding guard stays in force
// for every other route; this route needs the Bearer token a web page cannot
// obtain, so the alias adds no browser attack surface.
func storeLLMRelayHostAllowed(r *http.Request) bool {
	return isStoreLLMRelayPath(r.URL.Path) && hostnameOnly(requestHost(r)) == storeLLMRelayDockerHostAlias
}

func (h *storeLLMRelay) log() *slog.Logger {
	if h.server != nil && h.server.Logger != nil {
		return h.server.Logger
	}
	return slog.Default()
}

func (h *storeLLMRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !h.authorized(w, r) {
		return
	}
	switch strings.TrimPrefix(r.URL.Path, storeLLMRelayPrefix) {
	case "/models":
		if r.Method != http.MethodGet {
			writeStoreRelayMethodNotAllowed(w, http.MethodGet)
			return
		}
		h.handleModels(w)
	case "/chat/completions":
		if r.Method != http.MethodPost {
			writeStoreRelayMethodNotAllowed(w, http.MethodPost)
			return
		}
		h.handleChat(w, r)
	case "/embeddings":
		if r.Method != http.MethodPost {
			writeStoreRelayMethodNotAllowed(w, http.MethodPost)
			return
		}
		h.handleEmbeddings(w, r)
	default:
		writeStoreRelayError(w, http.StatusNotFound, "invalid_request_error", "not_found", "Unknown endpoint.")
	}
}

// authorized answers 401 with one constant body for a missing, malformed,
// wrong or revoked token and when the Store service cannot be reached.
func (h *storeLLMRelay) authorized(w http.ResponseWriter, r *http.Request) bool {
	token, present := bearerCredential(r.Header.Get("Authorization"))
	if !present || token == "" || len(token) > storeLLMRelayMaxTokenBytes || h.verifier == nil {
		h.denyAuth(w, r)
		return false
	}
	verifier, err := h.verifier(r.Context())
	if err != nil || verifier == nil {
		h.log().Debug("[StoreLLMRelay] token verifier unavailable", "error", err)
		h.denyAuth(w, r)
		return false
	}
	if !verifier.VerifyRelayToken(r.Context(), storeLLMRelayAppID, token) {
		h.denyAuth(w, r)
		return false
	}
	return true
}

func (h *storeLLMRelay) denyAuth(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UnixNano()
	if last := h.lastAuthLog.Load(); now-last >= int64(storeLLMRelayAuthLogInterval) && h.lastAuthLog.CompareAndSwap(last, now) {
		h.log().Warn("[StoreLLMRelay] request refused: missing or invalid relay token", "remote", remoteIP(r.RemoteAddr))
	}
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeStoreRelayError(w, http.StatusUnauthorized, "invalid_request_error", "invalid_api_key", "Incorrect API key provided.")
}

func (h *storeLLMRelay) acquire(w http.ResponseWriter) (func(), bool) {
	slots := h.slots
	if slots == nil {
		slots = storeLLMRelaySlots
	}
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, true
	default:
		w.Header().Set("Retry-After", "2")
		writeStoreRelayError(w, http.StatusTooManyRequests, "rate_limit_error", "rate_limit_exceeded", "Too many concurrent requests. Try again shortly.")
		return nil, false
	}
}

func (h *storeLLMRelay) handleModels(w http.ResponseWriter) {
	writeStoreRelayJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{{
			"id":       storeLLMRelayModelAlias,
			"object":   "model",
			"created":  0,
			"owned_by": "aurago",
		}},
	})
}

// readBody reads the capped request body; it answers 413/400 itself.
func (h *storeLLMRelay) readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if r.ContentLength > storeLLMRelayMaxBodyBytes {
		writeStoreRelayError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request_too_large", "The request body is too large.")
		return nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, storeLLMRelayMaxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeStoreRelayError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request_too_large", "The request body is too large.")
		} else {
			writeStoreRelayError(w, http.StatusBadRequest, "invalid_request_error", "invalid_request", "The request body could not be read.")
		}
		return nil, false
	}
	return body, true
}

// decodeStoreRelayChatRequest tolerates unknown fields (the typed request drops
// them) and the OpenAI-legal scalar "stop" that go-openai cannot decode.
func decodeStoreRelayChatRequest(body []byte) (openai.ChatCompletionRequest, error) {
	var req openai.ChatCompletionRequest
	if bytes.Contains(body, []byte(`"stop"`)) {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			return req, err
		}
		if stop, ok := raw["stop"]; ok {
			switch trimmed := bytes.TrimSpace(stop); {
			case len(trimmed) > 0 && trimmed[0] == '"':
				raw["stop"] = json.RawMessage("[" + string(trimmed) + "]")
			case string(trimmed) == "null":
				delete(raw, "stop")
			}
			normalized, err := json.Marshal(raw)
			if err != nil {
				return req, err
			}
			body = normalized
		}
	}
	err := json.Unmarshal(body, &req)
	return req, err
}

func (h *storeLLMRelay) handleChat(w http.ResponseWriter, r *http.Request) {
	release, ok := h.acquire(w)
	if !ok {
		return
	}
	defer release()
	body, ok := h.readBody(w, r)
	if !ok {
		return
	}
	req, err := decodeStoreRelayChatRequest(body)
	if err != nil {
		writeStoreRelayError(w, http.StatusBadRequest, "invalid_request_error", "invalid_json", "The request body is not valid JSON for this endpoint.")
		return
	}
	if len(req.Messages) == 0 {
		writeStoreRelayError(w, http.StatusBadRequest, "invalid_request_error", "invalid_request", "messages is required.")
		return
	}
	cfg, client := h.server.chatRuntimeSnapshot()
	model := ""
	if cfg != nil {
		model = strings.TrimSpace(cfg.LLM.Model)
	}
	if client == nil || model == "" {
		writeStoreRelayError(w, http.StatusServiceUnavailable, "server_error", "llm_not_configured", "AuraGo has no LLM configured.")
		return
	}
	if tracker := h.server.BudgetTracker; tracker != nil && tracker.IsBlocked(storeLLMRelayBudgetCategory) {
		writeStoreRelayError(w, http.StatusTooManyRequests, "insufficient_quota", "budget_exceeded", "The AuraGo budget limit is reached.")
		return
	}

	// The client never chooses the model, and provider-side persistence
	// switches have no meaning for a relay.
	req.Model = model
	req.Store = false
	req.Metadata = nil
	timeout := storeLLMRelayChatTimeout
	if req.Stream {
		timeout = storeLLMRelayStreamTimeout
	} else {
		req.StreamOptions = nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	callCtx, capture := llm.CaptureUsage(ctx)

	if req.Stream {
		h.streamChat(callCtx, w, r, capture, client, cfg, req)
		return
	}
	resp, err := client.CreateChatCompletion(callCtx, req)
	h.recordUsage(cfg, model, capture, &resp.Usage, false)
	if err != nil {
		h.writeUpstreamError(w, r, err)
		return
	}
	resp.Model = storeLLMRelayModelAlias
	if resp.Object == "" {
		resp.Object = "chat.completion"
	}
	if resp.Choices == nil {
		resp.Choices = []openai.ChatCompletionChoice{}
	}
	writeStoreRelayJSON(w, http.StatusOK, resp)
}

func (h *storeLLMRelay) streamChat(ctx context.Context, w http.ResponseWriter, r *http.Request, capture *llm.UsageCapture, client llm.ChatClient, cfg *config.Config, req openai.ChatCompletionRequest) {
	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		h.recordUsage(cfg, req.Model, capture, nil, false)
		h.writeUpstreamError(w, r, err)
		return
	}
	var usage *openai.Usage
	defer func() {
		_ = stream.Close()
		h.recordUsage(cfg, req.Model, capture, usage, true)
	}()

	// Wait for the first event before committing the response, so an upstream
	// failure that arrives first still gets a real HTTP status.
	chunk, recvErr := stream.Recv()
	if recvErr != nil && !errors.Is(recvErr, io.EOF) {
		h.writeUpstreamError(w, r, recvErr)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher := http.NewResponseController(w)
	_ = flusher.Flush()

	for {
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			if r.Context().Err() != nil {
				return // the client is gone; nobody reads a frame
			}
			mapped := mapStoreRelayError(recvErr)
			payload, _ := json.Marshal(map[string]any{"error": mapped.body()})
			if writeStoreRelaySSE(w, flusher, payload) != nil {
				return
			}
			break
		}
		chunk.Model = storeLLMRelayModelAlias
		if chunk.Object == "" {
			chunk.Object = "chat.completion.chunk"
		}
		if chunk.Choices == nil {
			chunk.Choices = []openai.ChatCompletionStreamChoice{}
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		payload, err := json.Marshal(chunk)
		if err != nil {
			mapped := storeRelayError{typ: "server_error", code: "internal_error", message: "A response chunk could not be encoded."}
			payload, _ = json.Marshal(map[string]any{"error": mapped.body()})
			_ = writeStoreRelaySSE(w, flusher, payload)
			break
		}
		if writeStoreRelaySSE(w, flusher, payload) != nil {
			return
		}
		chunk, recvErr = stream.Recv()
	}
	_ = writeStoreRelaySSE(w, flusher, []byte("[DONE]"))
}

func writeStoreRelaySSE(w http.ResponseWriter, flusher *http.ResponseController, payload []byte) error {
	frame := make([]byte, 0, len(payload)+8)
	frame = append(frame, "data: "...)
	frame = append(frame, payload...)
	frame = append(frame, '\n', '\n')
	if _, err := w.Write(frame); err != nil {
		return err
	}
	if err := flusher.Flush(); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}

// recordUsage books measured (or, failing that, reported) token counts under
// the "store_relay" budget category and logs counts only.
func (h *storeLLMRelay) recordUsage(cfg *config.Config, model string, capture *llm.UsageCapture, usage *openai.Usage, stream bool) {
	input, output := 0, 0
	if usage != nil {
		input, output = max(0, usage.PromptTokens), max(0, usage.CompletionTokens)
	}
	if tracker := h.server.BudgetTracker; tracker != nil && cfg != nil {
		tracker.RecordResponseUsage(storeLLMRelayBudgetCategory, cfg.LLM.ProviderType, model, capture, input, output, input > 0 || output > 0)
	}
	if input > 0 || output > 0 {
		h.log().Info("[StoreLLMRelay] completion finished", "app", storeLLMRelayAppID, "stream", stream, "prompt_tokens", input, "completion_tokens", output)
	}
}

// ── embeddings ───────────────────────────────────────────────────────────────

type storeRelayEmbeddingTarget struct {
	Endpoint string
	APIKey   string
	Model    string
}

// storeRelayEmbeddingTargetFor resolves the AuraGo embedding provider the same
// way the memory runtime does, but only returns a target for an external,
// OpenAI-compatible HTTP endpoint. Disabled and in-process (local-granite)
// providers have no endpoint to forward to.
func storeRelayEmbeddingTargetFor(cfg *config.Config) (storeRelayEmbeddingTarget, bool) {
	if cfg == nil {
		return storeRelayEmbeddingTarget{}, false
	}
	provider := strings.TrimSpace(cfg.Embeddings.Provider)
	switch strings.ToLower(provider) {
	case "", "disabled", "local-granite":
		return storeRelayEmbeddingTarget{}, false
	}
	baseURL, apiKey, model := cfg.Embeddings.BaseURL, cfg.Embeddings.APIKey, cfg.Embeddings.Model
	switch provider {
	case "internal":
		baseURL, apiKey = cfg.LLM.BaseURL, cfg.LLM.APIKey
		if model == "" {
			model = cfg.Embeddings.InternalModel
		}
	case "external":
		if baseURL == "" {
			baseURL = cfg.Embeddings.ExternalURL
		}
		if model == "" {
			model = cfg.Embeddings.ExternalModel
		}
	}
	if model == "" {
		model = storeLLMRelayDefaultEmbedding
	}
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return storeRelayEmbeddingTarget{}, false
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/embeddings"
	u.RawPath = ""
	u.Fragment = ""
	return storeRelayEmbeddingTarget{Endpoint: u.String(), APIKey: apiKey, Model: model}, true
}

// storeRelayEmbeddingsAvailable tells the Store whether the relay can serve
// /embeddings for this configuration.
func storeRelayEmbeddingsAvailable(cfg *config.Config) bool {
	_, ok := storeRelayEmbeddingTargetFor(cfg)
	return ok
}

func (h *storeLLMRelay) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	target, ok := storeRelayEmbeddingTargetFor(h.server.ConfigSnapshot())
	if !ok {
		writeStoreRelayError(w, http.StatusNotFound, "invalid_request_error", "embeddings_unavailable", "AuraGo has no external OpenAI-compatible embedding provider configured.")
		return
	}
	release, ok := h.acquire(w)
	if !ok {
		return
	}
	defer release()
	body, ok := h.readBody(w, r)
	if !ok {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		writeStoreRelayError(w, http.StatusBadRequest, "invalid_request_error", "invalid_json", "The request body is not a JSON object.")
		return
	}
	if input := bytes.TrimSpace(raw["input"]); len(input) == 0 || string(input) == "null" {
		writeStoreRelayError(w, http.StatusBadRequest, "invalid_request_error", "invalid_request", "input is required.")
		return
	}
	model, _ := json.Marshal(target.Model)
	raw["model"] = model // the request never chooses the model (or, below, the URL)
	payload, err := json.Marshal(raw)
	if err != nil {
		writeStoreRelayError(w, http.StatusBadRequest, "invalid_request_error", "invalid_json", "The request body could not be encoded.")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), storeLLMRelayEmbedTimeout)
	defer cancel()
	upstream, err := http.NewRequestWithContext(ctx, http.MethodPost, target.Endpoint, bytes.NewReader(payload))
	if err != nil {
		writeStoreRelayError(w, http.StatusBadGateway, "server_error", "upstream_error", "The embedding provider request failed.")
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	if target.APIKey != "" {
		upstream.Header.Set("Authorization", "Bearer "+target.APIKey)
	}
	client := h.embedClient
	if client == nil {
		client = httporigin.NewClient(storeLLMRelayEmbedTimeout)
	}
	resp, err := client.Do(upstream)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			writeStoreRelayError(w, http.StatusGatewayTimeout, "server_error", "upstream_timeout", "The embedding provider timed out.")
			return
		}
		writeStoreRelayError(w, http.StatusBadGateway, "server_error", "upstream_error", "The embedding provider request failed.")
		return
	}
	defer resp.Body.Close()
	// The provider body is never relayed on failure: error text can echo the
	// request or name internal endpoints.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		mapped := mapStoreRelayEmbeddingStatus(resp.StatusCode)
		writeStoreRelayError(w, mapped.status, mapped.typ, mapped.code, mapped.message)
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, storeLLMRelayMaxResponseBytes+1))
	if err != nil || len(data) > storeLLMRelayMaxResponseBytes || !json.Valid(data) {
		writeStoreRelayError(w, http.StatusBadGateway, "server_error", "upstream_error", "The embedding provider returned an unusable response.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ── errors ───────────────────────────────────────────────────────────────────

type storeRelayError struct {
	status  int
	typ     string
	code    string
	message string
}

func (e storeRelayError) body() map[string]any {
	return map[string]any{"message": e.message, "type": e.typ, "param": nil, "code": e.code}
}

func mapStoreRelayEmbeddingStatus(status int) storeRelayError {
	switch {
	case status == http.StatusTooManyRequests:
		return storeRelayError{http.StatusTooManyRequests, "rate_limit_error", "rate_limit_exceeded", "The embedding provider is rate limiting AuraGo. Try again later."}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return storeRelayError{http.StatusBadGateway, "server_error", "upstream_auth_error", "The embedding provider rejected AuraGo's credentials."}
	case status >= 400 && status < 500:
		return storeRelayError{http.StatusBadRequest, "invalid_request_error", "invalid_request", "The embedding provider rejected the request."}
	default:
		return storeRelayError{http.StatusBadGateway, "server_error", "upstream_error", "The embedding provider request failed."}
	}
}

var storeRelayURLPattern = regexp.MustCompile(`(?i)\b(?:https?|wss?|ftp)://[^\s"'<>)\]]+`)

// sanitizeStoreRelayMessage strips URLs and registered secrets from a provider
// message and bounds its length.
func sanitizeStoreRelayMessage(message string) string {
	message = storeRelayURLPattern.ReplaceAllString(message, "[url]")
	message = security.RedactSensitiveInfo(security.Scrub(message))
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > storeLLMRelayMaxErrorBytes {
		cut := storeLLMRelayMaxErrorBytes
		for cut > 0 && !utf8.RuneStart(message[cut]) {
			cut--
		}
		message = message[:cut] + "..."
	}
	return message
}

// mapStoreRelayError turns an LLM client error into a fixed OpenAI-style error.
// Only a provider's own 4xx message field is passed on, after sanitizing; raw
// error strings (which carry request URLs) never leave this function.
func mapStoreRelayError(err error) storeRelayError {
	switch llm.ClassifyError(err) {
	case llm.ErrCategoryContextCanceled:
		return storeRelayError{499, "server_error", "request_canceled", "The request was canceled."}
	case llm.ErrCategoryContextDeadline:
		return storeRelayError{http.StatusGatewayTimeout, "server_error", "upstream_timeout", "The LLM request timed out."}
	case llm.ErrCategoryAuthError:
		return storeRelayError{http.StatusBadGateway, "server_error", "upstream_auth_error", "The LLM provider rejected AuraGo's credentials."}
	case llm.ErrCategoryRateLimit:
		return storeRelayError{http.StatusTooManyRequests, "rate_limit_error", "rate_limit_exceeded", "The LLM provider is rate limiting AuraGo. Try again later."}
	case llm.ErrCategoryQuotaExceeded:
		return storeRelayError{http.StatusTooManyRequests, "insufficient_quota", "insufficient_quota", "The LLM provider quota is exhausted."}
	case llm.ErrCategoryContextLimit:
		return storeRelayError{http.StatusBadRequest, "invalid_request_error", "context_length_exceeded", "The request exceeds the model's context window."}
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) && (apiErr.HTTPStatusCode == http.StatusBadRequest || apiErr.HTTPStatusCode == http.StatusUnprocessableEntity) {
		message := sanitizeStoreRelayMessage(apiErr.Message)
		if message == "" {
			message = "The LLM provider rejected the request."
		}
		return storeRelayError{http.StatusBadRequest, "invalid_request_error", "invalid_request", message}
	}
	return storeRelayError{http.StatusBadGateway, "server_error", "upstream_error", "The LLM provider request failed."}
}

func (h *storeLLMRelay) writeUpstreamError(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil {
		return // the client disconnected; there is nobody to answer
	}
	mapped := mapStoreRelayError(err)
	h.log().Warn("[StoreLLMRelay] upstream call failed", "app", storeLLMRelayAppID, "code", mapped.code, "category", string(llm.ClassifyError(err)))
	if mapped.status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "5")
	}
	writeStoreRelayError(w, mapped.status, mapped.typ, mapped.code, mapped.message)
}

func writeStoreRelayError(w http.ResponseWriter, status int, typ, code, message string) {
	writeStoreRelayJSON(w, status, map[string]any{"error": storeRelayError{typ: typ, code: code, message: message}.body()})
}

func writeStoreRelayMethodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeStoreRelayError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed", fmt.Sprintf("Use %s for this endpoint.", allow))
}

func writeStoreRelayJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
