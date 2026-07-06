package evomap

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"
)

const (
	ProtocolName        = "gep-a2a"
	ProtocolVersion     = "1.0.0"
	DefaultBaseURL      = "https://evomap.ai"
	DefaultTimeout      = 30 * time.Second
	DefaultMaxResultLen = 262144
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Config contains the runtime-safe EvoMap client settings.
type Config struct {
	BaseURL        string
	NodeID         string
	NodeSecret     string
	APIKey         string
	Timeout        time.Duration
	MaxResultBytes int64
	HTTPClient     httpDoer
}

// Client talks to the EvoMap GEP/A2A and optional KG endpoints.
type Client struct {
	baseURL        string
	nodeID         string
	nodeSecret     string
	apiKey         string
	maxResultBytes int64
	httpClient     httpDoer
}

type StatusResult struct {
	Status string          `json:"status,omitempty"`
	Raw    json.RawMessage `json:"raw,omitempty"`
}

type RegisterRequest struct {
	Capabilities []string               `json:"capabilities,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type RegisterResponse struct {
	NodeID     string          `json:"node_id,omitempty"`
	NodeSecret string          `json:"node_secret,omitempty"`
	ClaimURL   string          `json:"claim_url,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

type FetchRequest struct {
	Problem string                 `json:"problem,omitempty"`
	Query   string                 `json:"query,omitempty"`
	Signals map[string]interface{} `json:"signals,omitempty"`
	Limit   int                    `json:"limit,omitempty"`
}

type FetchResponse struct {
	Raw json.RawMessage `json:"raw,omitempty"`
}

type AssetRequest struct {
	AssetID string `json:"asset_id,omitempty"`
}

type AssetResponse struct {
	Raw json.RawMessage `json:"raw,omitempty"`
}

type KGQueryRequest struct {
	Question string `json:"question,omitempty"`
	Query    string `json:"query,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type KGQueryResponse struct {
	Raw json.RawMessage `json:"raw,omitempty"`
}

type a2aEnvelope struct {
	Protocol        string                 `json:"protocol"`
	ProtocolVersion string                 `json:"protocol_version"`
	MessageType     string                 `json:"message_type"`
	MessageID       string                 `json:"message_id"`
	SenderID        string                 `json:"sender_id,omitempty"`
	Timestamp       string                 `json:"timestamp"`
	Payload         map[string]interface{} `json:"payload"`
}

func NewClient(cfg Config) (*Client, error) {
	baseURL, err := normalizeBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxBytes := cfg.MaxResultBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxResultLen
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient, err = security.NewSSRFProtectedHTTPClientForURL(baseURL, timeout)
		if err != nil {
			return nil, fmt.Errorf("create EvoMap HTTP client: %w", err)
		}
	}
	if strings.TrimSpace(cfg.NodeSecret) != "" {
		security.RegisterSensitive(cfg.NodeSecret)
	}
	if strings.TrimSpace(cfg.APIKey) != "" {
		security.RegisterSensitive(cfg.APIKey)
	}
	return &Client{
		baseURL:        baseURL,
		nodeID:         strings.TrimSpace(cfg.NodeID),
		nodeSecret:     strings.TrimSpace(cfg.NodeSecret),
		apiKey:         strings.TrimSpace(cfg.APIKey),
		maxResultBytes: maxBytes,
		httpClient:     httpClient,
	}, nil
}

func normalizeBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = DefaultBaseURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid EvoMap base_url: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("EvoMap base_url must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("EvoMap base_url must include a host")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (c *Client) Status(ctx context.Context) (StatusResult, error) {
	raw, err := c.do(ctx, http.MethodGet, "/a2a/stats", nil, "")
	if err != nil {
		return StatusResult{}, err
	}
	var parsed struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if parsed.Status == "" {
		parsed.Status = "ok"
	}
	return StatusResult{Status: parsed.Status, Raw: raw}, nil
}

func (c *Client) RegisterNode(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	payload := map[string]interface{}{
		"capabilities": req.Capabilities,
	}
	if len(req.Metadata) > 0 {
		payload["metadata"] = req.Metadata
	}
	envelope, err := c.a2aEnvelope("hello", payload, false)
	if err != nil {
		return RegisterResponse{}, err
	}
	raw, err := c.do(ctx, http.MethodPost, "/a2a/hello", envelope, "")
	if err != nil {
		return RegisterResponse{}, err
	}
	var parsed RegisterResponse
	_ = json.Unmarshal(raw, &parsed)
	parsed.Raw = raw
	return parsed, nil
}

func (c *Client) FetchCapsules(ctx context.Context, req FetchRequest) (FetchResponse, error) {
	payload := map[string]interface{}{
		"problem": strings.TrimSpace(req.Problem),
		"query":   strings.TrimSpace(req.Query),
		"limit":   req.Limit,
	}
	if c.nodeSecret != "" {
		payload["node_secret"] = c.nodeSecret
	}
	if len(req.Signals) > 0 {
		payload["signals"] = req.Signals
	}
	envelope, err := c.a2aEnvelope("fetch", payload, true)
	if err != nil {
		return FetchResponse{}, err
	}
	raw, err := c.do(ctx, http.MethodPost, "/a2a/fetch", envelope, "")
	if err != nil {
		return FetchResponse{}, err
	}
	return FetchResponse{Raw: raw}, nil
}

func (c *Client) GetAsset(ctx context.Context, req AssetRequest) (AssetResponse, error) {
	assetID := strings.TrimSpace(req.AssetID)
	if assetID == "" {
		return AssetResponse{}, fmt.Errorf("asset_id is required")
	}
	payload := map[string]interface{}{
		"asset_id": assetID,
	}
	if c.nodeSecret != "" {
		payload["node_secret"] = c.nodeSecret
	}
	envelope, err := c.a2aEnvelope("asset", payload, true)
	if err != nil {
		return AssetResponse{}, err
	}
	raw, err := c.do(ctx, http.MethodPost, "/a2a/asset", envelope, "")
	if err != nil {
		return AssetResponse{}, err
	}
	return AssetResponse{Raw: raw}, nil
}

func (c *Client) KGQuery(ctx context.Context, req KGQueryRequest) (KGQueryResponse, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return KGQueryResponse{}, fmt.Errorf("EvoMap API key is not configured")
	}
	question := strings.TrimSpace(req.Question)
	query := strings.TrimSpace(req.Query)
	if question == "" {
		question = query
	}
	payload := map[string]interface{}{
		"question": question,
		"query":    query,
		"limit":    req.Limit,
	}
	raw, err := c.do(ctx, http.MethodPost, "/kg/query", payload, "Bearer "+c.apiKey)
	if err != nil {
		return KGQueryResponse{}, err
	}
	return KGQueryResponse{Raw: raw}, nil
}

func (c *Client) a2aEnvelope(messageType string, payload map[string]interface{}, requireSender bool) (a2aEnvelope, error) {
	senderID := strings.TrimSpace(c.nodeID)
	if requireSender && senderID == "" {
		return a2aEnvelope{}, fmt.Errorf("EvoMap node_id is required for %s requests", messageType)
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	messageID, err := newA2AMessageID()
	if err != nil {
		return a2aEnvelope{}, err
	}
	return a2aEnvelope{
		Protocol:        ProtocolName,
		ProtocolVersion: ProtocolVersion,
		MessageType:     messageType,
		MessageID:       messageID,
		SenderID:        senderID,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		Payload:         payload,
	}, nil
}

func newA2AMessageID() (string, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("create EvoMap message id: %w", err)
	}
	return fmt.Sprintf("msg_%d_%s", time.Now().UTC().UnixNano(), hex.EncodeToString(random)), nil
}

func (c *Client) do(ctx context.Context, method, path string, payload interface{}, authorization string) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode EvoMap request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create EvoMap request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(authorization) != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request EvoMap: %w", err)
	}
	defer resp.Body.Close()

	raw, err := readLimited(resp.Body, c.maxResultBytes)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("EvoMap request failed: %s", security.Scrub(msg))
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("EvoMap response was not valid JSON")
	}
	return json.RawMessage(raw), nil
}

func readLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxResultLen
	}
	raw, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read EvoMap response: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("EvoMap response exceeds configured result size limit (%d bytes)", maxBytes)
	}
	return raw, nil
}
