package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	gosync "sync"
	"text/template"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// SSEBroadcaster is an interface for pushing events to connected browsers.
type SSEBroadcaster interface {
	Send(event, detail string)
}

// Handler processes incoming webhook HTTP requests.
type Handler struct {
	mu             gosync.RWMutex // protects maxPayloadSize and rateLimiter during hot-reload
	manager        *Manager
	tokenManager   *security.TokenManager
	vault          *security.Vault
	guardian       *security.Guardian
	llmGuardian    *security.LLMGuardian
	cfg            *config.Config
	logger         *slog.Logger
	serverPort     int
	maxPayloadSize int64
	rateLimiter    *RateLimiter
	sse            SSEBroadcaster
}

// NewHandler creates a webhook receiver handler.
func NewHandler(manager *Manager, tokenManager *security.TokenManager, vault *security.Vault, guardian *security.Guardian, llmGuardian *security.LLMGuardian, cfg *config.Config, logger *slog.Logger, serverPort int, maxPayloadSize int64, rateLimit int) *Handler {
	return &Handler{
		manager:        manager,
		tokenManager:   tokenManager,
		vault:          vault,
		guardian:       guardian,
		llmGuardian:    llmGuardian,
		cfg:            cfg,
		logger:         logger,
		serverPort:     serverPort,
		maxPayloadSize: maxPayloadSize,
		rateLimiter:    NewRateLimiter(rateLimit),
	}
}

// Reconfigure updates the payload size limit and rate limiter without restarting.
// Safe to call concurrently with in-flight requests.
func (h *Handler) Reconfigure(maxPayloadSize int64, rateLimit int) {
	h.mu.Lock()
	h.maxPayloadSize = maxPayloadSize
	h.rateLimiter = NewRateLimiter(rateLimit)
	h.mu.Unlock()
}

// SetSSE sets the SSE broadcaster for notification delivery.
func (h *Handler) SetSSE(sse SSEBroadcaster) {
	h.sse = sse
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Snapshot hot-reloadable values under read lock so Reconfigure can run concurrently.
	h.mu.RLock()
	maxPayloadSize := h.maxPayloadSize
	rateLimiter := h.rateLimiter
	h.mu.RUnlock()

	// Extract slug from path: /webhook/{slug}
	slug := strings.TrimPrefix(r.URL.Path, "/webhook/")
	slug = strings.TrimSuffix(slug, "/")
	if slug == "" {
		http.Error(w, `{"error":"missing webhook slug"}`, http.StatusNotFound)
		return
	}

	sourceIP := r.RemoteAddr

	// 1. Lookup webhook by slug
	wh, err := h.manager.GetBySlug(slug)
	if err != nil {
		h.logEvent("", "", 404, sourceIP, 0, false, "webhook not found: "+slug)
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	if !wh.Enabled {
		h.logEvent(wh.ID, wh.Name, 403, sourceIP, 0, false, "webhook disabled")
		http.Error(w, `{"error":"webhook disabled"}`, http.StatusForbidden)
		return
	}
	if wh.Format.SignatureSecret == "" && h.vault != nil {
		if secret, err := h.vault.ReadSecret(SignatureSecretVaultKey(wh.ID)); err == nil && strings.TrimSpace(secret) != "" {
			wh.Format.SignatureSecret = secret
		}
	}

	// 2. Token validation
	rawToken := extractToken(r)
	if rawToken == "" {
		h.logEvent(wh.ID, wh.Name, 401, sourceIP, 0, false, "no token provided")
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	tokenMeta, valid := h.tokenManager.Validate(rawToken, "webhook")
	if !valid {
		h.logEvent(wh.ID, wh.Name, 401, sourceIP, 0, false, "invalid or expired token")
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	h.tokenManager.TouchLastUsed(tokenMeta.ID)

	// 3. Rate limiting
	if rateLimiter != nil && !rateLimiter.Allow(tokenMeta.ID) {
		h.logEvent(wh.ID, wh.Name, 429, sourceIP, 0, false, "rate limit exceeded")
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
		return
	}

	// 4. Read body
	body, err := io.ReadAll(io.LimitReader(r.Body, maxPayloadSize+1))
	if err != nil {
		h.logEvent(wh.ID, wh.Name, 400, sourceIP, 0, false, "failed to read body")
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}
	if int64(len(body)) > maxPayloadSize {
		h.logEvent(wh.ID, wh.Name, 413, sourceIP, len(body), false, "payload too large")
		http.Error(w, `{"error":"payload too large"}`, http.StatusRequestEntityTooLarge)
		return
	}

	// 5. Content-Type validation
	ct := r.Header.Get("Content-Type")
	if !isAcceptedContentType(ct, wh.Format.AcceptedContentTypes) {
		h.logEvent(wh.ID, wh.Name, 415, sourceIP, len(body), false, "unsupported content type: "+ct)
		http.Error(w, `{"error":"unsupported content type"}`, http.StatusUnsupportedMediaType)
		return
	}

	// 6. HMAC signature validation (optional)
	if wh.Format.SignatureHeader != "" && wh.Format.SignatureSecret != "" {
		sigHeader := r.Header.Get(wh.Format.SignatureHeader)
		if !verifySignature(body, sigHeader, wh.Format.SignatureSecret, wh.Format.SignatureAlgo) {
			h.logEvent(wh.ID, wh.Name, 403, sourceIP, len(body), false, "signature verification failed")
			http.Error(w, `{"error":"signature verification failed"}`, http.StatusForbidden)
			return
		}
	}

	// 7. Parse payload and extract fields
	fields := extractFields(body, wh.Format.Fields)
	headers := extractHeaders(r)

	// 8. Scan raw payload for injection attempts before rendering.
	// All webhook payloads are external, untrusted data — always isolate them.
	if h.guardian != nil {
		scan := h.guardian.ScanForInjection(string(body))
		if scan.Level >= security.ThreatHigh {
			h.logger.Warn("[Webhook] High-threat injection pattern in payload", "webhook", wh.Name, "threat", scan.Level, "source_ip", sourceIP)
		} else if h.llmGuardian != nil && h.cfg != nil && h.cfg.LLMGuardian.ScanDocuments {
			// LLM Guardian: deeper content scan if regex didn't flag HIGH
			llmResult := h.llmGuardian.EvaluateContent(r.Context(), "document", string(body))
			if llmResult.Decision == security.DecisionBlock {
				h.logger.Warn("[Webhook] LLM Guardian blocked payload", "webhook", wh.Name, "reason", llmResult.Reason, "source_ip", sourceIP)
			}
		}
		// Always wrap in isolation tags — webhook payloads are external content
		body = []byte(security.IsolateExternalData(string(body)))
	}

	// 9. Render prompt
	prompt, err := renderPrompt(wh, string(body), fields, headers)
	if err != nil {
		h.logger.Error("Failed to render webhook prompt", "error", err, "webhook", wh.Name)
		prompt = fmt.Sprintf("[Webhook: %s]\nPayload:\n%s", wh.Name, string(body))
	}

	// 9. Respond immediately, deliver async
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})

	// 10. Async delivery
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				h.logger.Error("[Webhook] Delivery panic", "error", rec, "webhook", wh.Name)
			}
		}()

		delivered := false
		var deliveryErr string

		switch wh.Delivery.Mode {
		case DeliveryModeMessage:
			if deliverErr := h.deliverMessage(prompt); deliverErr != nil {
				deliveryErr = deliverErr.Error()
				h.logger.Error("Webhook message delivery failed", "error", deliverErr, "webhook", wh.Name)
			} else {
				delivered = true
			}
			if h.sse != nil {
				h.sse.Send("webhook_received", fmt.Sprintf(`{"name":%q,"slug":%q}`, wh.Name, wh.Slug))
			}

		case DeliveryModeNotify:
			if h.sse != nil {
				h.sse.Send("webhook_received", fmt.Sprintf(`{"name":%q,"slug":%q,"payload":%s}`, wh.Name, wh.Slug, truncateJSON(string(body), 500)))
			}
			delivered = true

		case DeliveryModeSilent:
			delivered = true
		}

		h.manager.RecordFire(wh.ID)
		h.manager.NotifyWebhookFired(wh.ID, body)
		h.logEvent(wh.ID, wh.Name, 200, sourceIP, len(body), delivered, deliveryErr)
	}()
}

// internalAPIURL returns the base URL for internal loopback API calls, respecting HTTPS config.
func (h *Handler) internalAPIURL() string {
	scheme := "http"
	port := h.serverPort
	if h.cfg.Server.HTTPS.Enabled {
		scheme = "https"
		if h.cfg.Server.HTTPS.HTTPSPort > 0 {
			port = h.cfg.Server.HTTPS.HTTPSPort
		} else {
			port = 443
		}
	}
	return fmt.Sprintf("%s://127.0.0.1:%d", scheme, port)
}

func (h *Handler) deliverMessage(prompt string) error {
	url := h.internalAPIURL() + "/v1/chat/completions"
	payload := map[string]interface{}{
		"model":  "aurago",
		"stream": false,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Webhook", "true")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("agent returned status %d", resp.StatusCode)
	}
	return nil
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

func isAcceptedContentType(ct string, accepted []string) bool {
	if len(accepted) == 0 {
		return true
	}
	ct = strings.ToLower(strings.Split(ct, ";")[0])
	ct = strings.TrimSpace(ct)
	for _, a := range accepted {
		if strings.ToLower(strings.TrimSpace(a)) == ct {
			return true
		}
	}
	return false
}

func verifySignature(body []byte, sigHeader, secret, algo string) bool {
	if sigHeader == "" {
		return false
	}
	switch strings.ToLower(algo) {
	case "sha256":
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(expected), []byte(sigHeader))
	case "sha1":
		mac := hmac.New(sha1.New, []byte(secret))
		mac.Write(body)
		expected := "sha1=" + hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(expected), []byte(sigHeader))
	case "plain":
		return sigHeader == secret
	default:
		return false
	}
}

func extractFields(body []byte, mappings []FieldMapping) map[string]interface{} {
	if len(mappings) == 0 {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}
	result := make(map[string]interface{})
	for _, m := range mappings {
		alias := m.Alias
		if alias == "" {
			alias = strings.ReplaceAll(m.Source, ".", "_")
		}
		result[alias] = getNestedValue(raw, m.Source)
	}
	return result
}

func getNestedValue(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

func extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	for key := range r.Header {
		headers[key] = r.Header.Get(key)
	}
	return headers
}

func renderPrompt(wh Webhook, rawPayload string, fields map[string]interface{}, headers map[string]string) (string, error) {
	tmplStr := wh.Delivery.PromptTemplate
	if tmplStr == "" {
		tmplStr = DefaultPromptTemplate
	}
	tmpl, err := template.New("webhook").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("invalid prompt template: %w", err)
	}
	data := PromptData{
		WebhookName: wh.Name,
		Slug:        wh.Slug,
		Payload:     truncateStr(rawPayload, 4000),
		Fields:      fields,
		Headers:     headers,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}
	return buf.String(), nil
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

func truncateJSON(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return `"` + s[:maxLen] + `... (truncated)"`
}

func (h *Handler) logEvent(webhookID, webhookName string, statusCode int, sourceIP string, payloadSize int, delivered bool, errMsg string) {
	h.manager.GetLog().Append(LogEntry{
		Timestamp:   time.Now().UTC(),
		WebhookID:   webhookID,
		WebhookName: webhookName,
		StatusCode:  statusCode,
		SourceIP:    sourceIP,
		PayloadSize: payloadSize,
		Delivered:   delivered,
		Error:       errMsg,
	})
}

// RateLimiter provides simple per-token rate limiting using a sliding window.
type RateLimiter struct {
	mu       gosync.Mutex
	limit    int
	counters map[string]*rateBucket
}

type rateBucket struct {
	count    int
	windowAt time.Time
}

// NewRateLimiter creates a rate limiter. limit=0 means unlimited.
func NewRateLimiter(limit int) *RateLimiter {
	if limit <= 0 {
		return nil
	}
	return &RateLimiter{
		limit:    limit,
		counters: make(map[string]*rateBucket),
	}
}

// Allow checks if a request from the given token ID is within rate limits.
func (rl *RateLimiter) Allow(tokenID string) bool {
	if rl == nil || rl.limit <= 0 {
		return true
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.counters[tokenID]
	if !ok || now.Sub(b.windowAt) > time.Minute {
		rl.counters[tokenID] = &rateBucket{count: 1, windowAt: now}
		return true
	}
	b.count++
	return b.count <= rl.limit
}

// --- Public helpers for use by admin handlers (test endpoint) ---

// ExtractFieldsPublic is a public wrapper for extractFields.
func ExtractFieldsPublic(body []byte, mappings []FieldMapping) map[string]interface{} {
	return extractFields(body, mappings)
}

// RenderPromptPublic is a public wrapper for renderPrompt.
func RenderPromptPublic(wh Webhook, rawPayload string, fields map[string]interface{}, headers map[string]string) (string, error) {
	return renderPrompt(wh, rawPayload, fields, headers)
}
