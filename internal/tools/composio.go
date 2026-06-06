package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const (
	defaultComposioBaseURL        = "https://backend.composio.dev/api/v3.1"
	defaultComposioMaxResultBytes = 262144
)

type ComposioClientConfig struct {
	BaseURL        string
	APIKey         string
	Timeout        time.Duration
	MaxResultBytes int64
	CacheTTL       time.Duration
	HTTPClient     *http.Client
}

type ComposioClient struct {
	baseURL        string
	apiKey         string
	maxResultBytes int64
	client         *http.Client
	cacheTTL       time.Duration
	cacheMu        sync.Mutex
	cache          map[string]composioCacheEntry
}

type composioCacheEntry struct {
	expiresAt time.Time
	body      []byte
}

type ComposioListQuery struct {
	Query  string
	Cursor string
	Limit  int
}

type ComposioToolQuery struct {
	ComposioListQuery
	ToolkitSlug string
}

type ComposioListPage[T any] struct {
	Items      []T
	NextCursor string
	Total      int
	Raw        json.RawMessage
}

type composioToolkitCategory struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	ID   string `json:"id"`
}

type composioToolkitMeta struct {
	Description string                    `json:"description,omitempty"`
	Logo        string                    `json:"logo,omitempty"`
	Categories  []composioToolkitCategory `json:"categories,omitempty"`
}

type composioToolkitRef struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Logo string `json:"logo,omitempty"`
}

type composioToolMeta struct {
	Description string `json:"description,omitempty"`
}

type ComposioToolkit struct {
	Slug        string              `json:"slug"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    string              `json:"category"`
	Logo        string              `json:"logo"`
	Meta        composioToolkitMeta `json:"meta,omitempty"`
}

type ComposioToolInfo struct {
	Slug             string             `json:"slug"`
	Name             string             `json:"name"`
	DisplayName      string             `json:"display_name,omitempty"`
	Description      string             `json:"description"`
	HumanDescription string             `json:"human_description,omitempty"`
	ToolkitSlug      string             `json:"toolkit_slug"`
	Toolkit          composioToolkitRef `json:"toolkit"`
	Meta             composioToolMeta   `json:"meta,omitempty"`
	InputParameters  json.RawMessage    `json:"input_parameters"`
}

type ComposioAuthConfig struct {
	ID                string             `json:"id"`
	UUID              string             `json:"uuid,omitempty"`
	ToolkitSlug       string             `json:"toolkit_slug"`
	Toolkit           composioToolkitRef `json:"toolkit"`
	Name              string             `json:"name"`
	Type              string             `json:"type"`
	Status            string             `json:"status"`
	Enabled           bool               `json:"enabled"`
	IsComposioManaged bool               `json:"is_composio_managed"`
}

type ComposioConnectedAccount struct {
	ID          string             `json:"id"`
	ToolkitSlug string             `json:"toolkit_slug"`
	Toolkit     composioToolkitRef `json:"toolkit"`
	UserID      string             `json:"user_id"`
	Status      string             `json:"status"`
}

type ComposioConnectLinkRequest struct {
	ToolkitSlug  string `json:"toolkit_slug,omitempty"`
	AuthConfigID string `json:"auth_config_id,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	CallbackURL  string `json:"callback_url,omitempty"`
}

type ComposioConnectLink struct {
	ID          string          `json:"id,omitempty"`
	RedirectURL string          `json:"redirect_url,omitempty"`
	Link        string          `json:"link,omitempty"`
	Raw         json.RawMessage `json:"raw,omitempty"`
}

type ComposioExecuteRequest struct {
	ToolSlug           string                 `json:"tool_slug"`
	ToolkitSlug        string                 `json:"toolkit_slug,omitempty"`
	ConnectedAccountID string                 `json:"connected_account_id,omitempty"`
	UserID             string                 `json:"user_id,omitempty"`
	Arguments          map[string]interface{} `json:"arguments,omitempty"`
	Text               string                 `json:"text,omitempty"`
}

type ComposioExecuteResult struct {
	Raw json.RawMessage `json:"raw"`
}

func NewComposioClient(cfg ComposioClientConfig) *ComposioClient {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultComposioBaseURL
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	maxBytes := cfg.MaxResultBytes
	if maxBytes <= 0 {
		maxBytes = defaultComposioMaxResultBytes
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &ComposioClient{
		baseURL:        baseURL,
		apiKey:         strings.TrimSpace(cfg.APIKey),
		maxResultBytes: maxBytes,
		client:         httpClient,
		cacheTTL:       cfg.CacheTTL,
		cache:          map[string]composioCacheEntry{},
	}
}

func NewComposioClientFromConfig(cfg config.ComposioConfig) *ComposioClient {
	return NewComposioClient(ComposioClientConfigFromConfig(cfg))
}

func ComposioClientConfigFromConfig(cfg config.ComposioConfig) ComposioClientConfig {
	return ComposioClientConfig{
		BaseURL:        cfg.BaseURL,
		APIKey:         cfg.APIKey,
		Timeout:        time.Duration(cfg.RequestTimeoutSeconds) * time.Second,
		MaxResultBytes: int64(cfg.MaxResultBytes),
		CacheTTL:       time.Duration(cfg.CacheTTLSeconds) * time.Second,
	}
}

func (c *ComposioClient) ListToolkits(ctx context.Context, q ComposioListQuery) (ComposioListPage[ComposioToolkit], error) {
	values := listQueryValues(q)
	body, err := c.do(ctx, http.MethodGet, "/toolkits", values, nil)
	if err != nil {
		return ComposioListPage[ComposioToolkit]{}, err
	}
	page, err := decodeComposioList[ComposioToolkit](body)
	if err != nil {
		return page, err
	}
	for i := range page.Items {
		page.Items[i].normalize()
	}
	return page, nil
}

func (c *ComposioClient) ListTools(ctx context.Context, q ComposioToolQuery) (ComposioListPage[ComposioToolInfo], error) {
	values := listQueryValues(q.ComposioListQuery)
	if strings.TrimSpace(q.ToolkitSlug) != "" {
		values.Set("toolkit_slug", strings.TrimSpace(q.ToolkitSlug))
	}
	body, err := c.do(ctx, http.MethodGet, "/tools", values, nil)
	if err != nil {
		return ComposioListPage[ComposioToolInfo]{}, err
	}
	page, err := decodeComposioList[ComposioToolInfo](body)
	if err != nil {
		return page, err
	}
	for i := range page.Items {
		page.Items[i].normalize()
	}
	return page, nil
}

func (c *ComposioClient) GetTool(ctx context.Context, slug string) (ComposioToolInfo, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ComposioToolInfo{}, fmt.Errorf("composio tool slug is required")
	}
	body, err := c.do(ctx, http.MethodGet, "/tools/"+url.PathEscape(slug), nil, nil)
	if err != nil {
		return ComposioToolInfo{}, err
	}
	toolInfo, err := decodeComposioObject[ComposioToolInfo](body)
	if err != nil {
		return ComposioToolInfo{}, err
	}
	toolInfo.normalize()
	return toolInfo, nil
}

func (c *ComposioClient) ListAuthConfigs(ctx context.Context, toolkitSlug string) (ComposioListPage[ComposioAuthConfig], error) {
	values := url.Values{}
	if strings.TrimSpace(toolkitSlug) != "" {
		values.Set("toolkit_slug", strings.TrimSpace(toolkitSlug))
	}
	body, err := c.do(ctx, http.MethodGet, "/auth_configs", values, nil)
	if err != nil {
		return ComposioListPage[ComposioAuthConfig]{}, err
	}
	page, err := decodeComposioList[ComposioAuthConfig](body)
	if err != nil {
		return page, err
	}
	for i := range page.Items {
		page.Items[i].normalize()
	}
	return page, nil
}

func (c *ComposioClient) ListConnectedAccounts(ctx context.Context, toolkitSlug, userID string) (ComposioListPage[ComposioConnectedAccount], error) {
	values := url.Values{}
	if strings.TrimSpace(toolkitSlug) != "" {
		values.Set("toolkit_slugs", strings.TrimSpace(toolkitSlug))
	}
	if strings.TrimSpace(userID) != "" {
		values.Set("user_ids", strings.TrimSpace(userID))
	}
	body, err := c.do(ctx, http.MethodGet, "/connected_accounts", values, nil)
	if err != nil {
		return ComposioListPage[ComposioConnectedAccount]{}, err
	}
	page, err := decodeComposioList[ComposioConnectedAccount](body)
	if err != nil {
		return page, err
	}
	for i := range page.Items {
		page.Items[i].normalize()
	}
	return page, nil
}

func (c *ComposioClient) CreateConnectLink(ctx context.Context, req ComposioConnectLinkRequest) (ComposioConnectLink, error) {
	body, err := c.do(ctx, http.MethodPost, "/connected_accounts/link", nil, req)
	if err != nil {
		return ComposioConnectLink{}, err
	}
	link, err := decodeComposioObject[ComposioConnectLink](body)
	if err != nil {
		return ComposioConnectLink{}, err
	}
	link.Raw = append(json.RawMessage(nil), body...)
	return link, nil
}

func (c *ComposioClient) ExecuteTool(ctx context.Context, req ComposioExecuteRequest) (ComposioExecuteResult, error) {
	req.ToolSlug = strings.TrimSpace(req.ToolSlug)
	if req.ToolSlug == "" {
		return ComposioExecuteResult{}, fmt.Errorf("composio tool slug is required")
	}
	payload := map[string]interface{}{}
	if req.Arguments != nil {
		payload["arguments"] = req.Arguments
	} else {
		payload["arguments"] = map[string]interface{}{}
	}
	if strings.TrimSpace(req.ConnectedAccountID) != "" {
		payload["connected_account_id"] = strings.TrimSpace(req.ConnectedAccountID)
	}
	if strings.TrimSpace(req.UserID) != "" {
		payload["user_id"] = strings.TrimSpace(req.UserID)
	}
	if strings.TrimSpace(req.Text) != "" {
		payload["text"] = strings.TrimSpace(req.Text)
	}
	body, err := c.do(ctx, http.MethodPost, "/tools/execute/"+url.PathEscape(req.ToolSlug), nil, payload)
	if err != nil {
		return ComposioExecuteResult{}, err
	}
	return ComposioExecuteResult{Raw: append(json.RawMessage(nil), body...)}, nil
}

func (c *ComposioClient) do(ctx context.Context, method, path string, values url.Values, payload interface{}) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("composio client is nil")
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("composio API key is not configured")
	}
	requestURL := c.baseURL + path
	if len(values) > 0 {
		requestURL += "?" + values.Encode()
	}
	cacheKey := method + " " + requestURL
	if method == http.MethodGet {
		if body, ok := c.getCached(cacheKey); ok {
			return body, nil
		}
	}

	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal composio request: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create composio request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("accept", "application/json")
	if payload != nil {
		req.Header.Set("content-type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("composio request failed: %s", security.Scrub(err.Error()))
	}
	defer resp.Body.Close()

	body, err := readComposioBody(resp.Body, c.maxResultBytes)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("composio API returned HTTP %d: %s", resp.StatusCode, composioErrorPreview(body))
	}
	if method == http.MethodGet {
		c.setCached(cacheKey, body)
	}
	return body, nil
}

func (c *ComposioClient) getCached(key string) ([]byte, bool) {
	if c.cacheTTL <= 0 {
		return nil, false
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	entry, ok := c.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(c.cache, key)
		return nil, false
	}
	return append([]byte(nil), entry.body...), true
}

func (c *ComposioClient) setCached(key string, body []byte) {
	if c.cacheTTL <= 0 {
		return
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache[key] = composioCacheEntry{
		expiresAt: time.Now().Add(c.cacheTTL),
		body:      append([]byte(nil), body...),
	}
}

func readComposioBody(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = defaultComposioMaxResultBytes
	}
	body, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read composio response: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("composio response exceeds composio result size limit (%d bytes)", maxBytes)
	}
	return body, nil
}

func composioErrorPreview(body []byte) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err == nil {
		for _, key := range []string{"message", "error", "detail"} {
			if val, ok := payload[key]; ok {
				switch typed := val.(type) {
				case string:
					return security.Scrub(strings.TrimSpace(typed))
				default:
					raw, _ := json.Marshal(typed)
					return security.Scrub(strings.TrimSpace(string(raw)))
				}
			}
		}
	}
	preview := strings.TrimSpace(string(body))
	if len(preview) > 500 {
		preview = preview[:500]
	}
	return security.Scrub(preview)
}

func listQueryValues(q ComposioListQuery) url.Values {
	values := url.Values{}
	if strings.TrimSpace(q.Query) != "" {
		query := strings.TrimSpace(q.Query)
		values.Set("q", query)
		values.Set("query", query)
	}
	if strings.TrimSpace(q.Cursor) != "" {
		values.Set("cursor", strings.TrimSpace(q.Cursor))
	}
	if q.Limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", q.Limit))
	}
	return values
}

type composioListEnvelope struct {
	Items      json.RawMessage `json:"items"`
	Data       json.RawMessage `json:"data"`
	Results    json.RawMessage `json:"results"`
	NextCursor string          `json:"next_cursor"`
	Cursor     string          `json:"cursor"`
	Total      int             `json:"total"`
}

func decodeComposioList[T any](body []byte) (ComposioListPage[T], error) {
	body = bytes.TrimSpace(body)
	page := ComposioListPage[T]{Raw: append(json.RawMessage(nil), body...)}
	if len(body) == 0 {
		return page, nil
	}
	if body[0] == '[' {
		if err := json.Unmarshal(body, &page.Items); err != nil {
			return page, fmt.Errorf("decode composio list: %w", err)
		}
		return page, nil
	}
	var env composioListEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return page, fmt.Errorf("decode composio list envelope: %w", err)
	}
	items := firstRawArray(env.Items, env.Data, env.Results)
	if len(items) == 0 || string(items) == "null" {
		return page, nil
	}
	if err := json.Unmarshal(items, &page.Items); err != nil {
		return page, fmt.Errorf("decode composio list items: %w", err)
	}
	page.NextCursor = strings.TrimSpace(env.NextCursor)
	if page.NextCursor == "" {
		page.NextCursor = strings.TrimSpace(env.Cursor)
	}
	page.Total = env.Total
	return page, nil
}

func decodeComposioObject[T any](body []byte) (T, error) {
	var zero T
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return zero, fmt.Errorf("empty composio response")
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Item   json.RawMessage `json:"item"`
		Result json.RawMessage `json:"result"`
	}
	if body[0] == '{' {
		if err := json.Unmarshal(body, &env); err == nil {
			for _, raw := range []json.RawMessage{env.Data, env.Item, env.Result} {
				if len(raw) > 0 && string(raw) != "null" {
					var value T
					if err := json.Unmarshal(raw, &value); err == nil {
						return value, nil
					}
				}
			}
		}
	}
	var value T
	if err := json.Unmarshal(body, &value); err != nil {
		return zero, fmt.Errorf("decode composio object: %w", err)
	}
	return value, nil
}

func firstRawArray(candidates ...json.RawMessage) json.RawMessage {
	for _, raw := range candidates {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || string(raw) == "null" {
			continue
		}
		if raw[0] == '[' {
			return raw
		}
		var nested composioListEnvelope
		if err := json.Unmarshal(raw, &nested); err == nil {
			if arr := firstRawArray(nested.Items, nested.Data, nested.Results); len(arr) > 0 {
				return arr
			}
		}
	}
	return nil
}

func (t *ComposioToolkit) normalize() {
	if t == nil {
		return
	}
	if strings.TrimSpace(t.Description) == "" {
		t.Description = strings.TrimSpace(t.Meta.Description)
	}
	if strings.TrimSpace(t.Logo) == "" {
		t.Logo = strings.TrimSpace(t.Meta.Logo)
	}
	if strings.TrimSpace(t.Category) == "" {
		for _, category := range t.Meta.Categories {
			if strings.TrimSpace(category.Name) != "" {
				t.Category = strings.TrimSpace(category.Name)
				return
			}
			if strings.TrimSpace(category.Slug) != "" {
				t.Category = strings.TrimSpace(category.Slug)
				return
			}
			if strings.TrimSpace(category.ID) != "" {
				t.Category = strings.TrimSpace(category.ID)
				return
			}
		}
	}
}

func (t *ComposioToolInfo) normalize() {
	if t == nil {
		return
	}
	if strings.TrimSpace(t.ToolkitSlug) == "" {
		t.ToolkitSlug = strings.TrimSpace(t.Toolkit.Slug)
	}
	if strings.TrimSpace(t.Description) == "" {
		t.Description = strings.TrimSpace(firstNonEmptyComposioString(t.HumanDescription, t.Meta.Description))
	}
	if strings.TrimSpace(t.Name) == "" {
		t.Name = strings.TrimSpace(t.DisplayName)
	}
}

func (a *ComposioAuthConfig) normalize() {
	if a == nil {
		return
	}
	if strings.TrimSpace(a.ID) == "" {
		a.ID = strings.TrimSpace(a.UUID)
	}
	if strings.TrimSpace(a.ToolkitSlug) == "" {
		a.ToolkitSlug = strings.TrimSpace(a.Toolkit.Slug)
	}
	if !a.Enabled && strings.EqualFold(strings.TrimSpace(a.Status), "ENABLED") {
		a.Enabled = true
	}
}

func (a *ComposioConnectedAccount) normalize() {
	if a == nil {
		return
	}
	if strings.TrimSpace(a.ToolkitSlug) == "" {
		a.ToolkitSlug = strings.TrimSpace(a.Toolkit.Slug)
	}
}

type ComposioPolicyConfig struct {
	Enabled                   bool
	ReadOnly                  bool
	AllowDestructive          bool
	AllowNaturalLanguageInput bool
	Toolkits                  []ComposioToolkitPolicy
}

type ComposioToolkitPolicy struct {
	Slug                        string
	Enabled                     bool
	PreferredConnectedAccountID string
	ReadOnly                    *bool
	AllowDestructive            *bool
	AllowNaturalLanguageInput   *bool
	AllowedToolSlugs            []string
	BlockedToolSlugs            []string
}

type ComposioPolicyDecision struct {
	Allowed     bool   `json:"allowed"`
	Reason      string `json:"reason,omitempty"`
	ReadOnly    bool   `json:"read_only"`
	Destructive bool   `json:"destructive"`
	ToolkitSlug string `json:"toolkit_slug,omitempty"`
	ToolSlug    string `json:"tool_slug,omitempty"`
}

func ComposioPolicyFromConfig(cfg config.ComposioConfig) ComposioPolicyConfig {
	policy := ComposioPolicyConfig{
		Enabled:                   cfg.Enabled,
		ReadOnly:                  cfg.ReadOnly,
		AllowDestructive:          cfg.AllowDestructive,
		AllowNaturalLanguageInput: cfg.AllowNaturalLanguageInput,
		Toolkits:                  make([]ComposioToolkitPolicy, 0, len(cfg.Toolkits)),
	}
	for _, tk := range cfg.Toolkits {
		policy.Toolkits = append(policy.Toolkits, ComposioToolkitPolicy{
			Slug:                        tk.Slug,
			Enabled:                     tk.Enabled,
			PreferredConnectedAccountID: tk.PreferredConnectedAccountID,
			ReadOnly:                    tk.ReadOnly,
			AllowDestructive:            tk.AllowDestructive,
			AllowNaturalLanguageInput:   tk.AllowNaturalLanguageInput,
			AllowedToolSlugs:            append([]string(nil), tk.AllowedToolSlugs...),
			BlockedToolSlugs:            append([]string(nil), tk.BlockedToolSlugs...),
		})
	}
	return policy
}

func EvaluateComposioToolPolicy(cfg ComposioPolicyConfig, tool ComposioToolInfo) ComposioPolicyDecision {
	slug := normalizeComposioSlug(tool.Slug)
	toolkitSlug := normalizeComposioSlug(firstNonEmptyComposioString(tool.ToolkitSlug, tool.Toolkit.Slug))
	decision := ComposioPolicyDecision{
		ReadOnly:    cfg.ReadOnly,
		Destructive: isComposioDestructiveSlug(slug),
		ToolkitSlug: toolkitSlug,
		ToolSlug:    slug,
	}
	if !cfg.Enabled {
		decision.Reason = "composio is not enabled"
		return decision
	}
	if toolkitSlug == "" {
		decision.Reason = "composio toolkit slug is required"
		return decision
	}
	toolkit, ok := cfg.findToolkit(toolkitSlug)
	if !ok || !toolkit.Enabled {
		decision.Reason = fmt.Sprintf("composio toolkit %q is not enabled", toolkitSlug)
		return decision
	}
	readOnly := cfg.ReadOnly
	if toolkit.ReadOnly != nil {
		readOnly = *toolkit.ReadOnly
	}
	allowDestructive := cfg.AllowDestructive
	if toolkit.AllowDestructive != nil {
		allowDestructive = *toolkit.AllowDestructive
	}
	decision.ReadOnly = readOnly

	if slugInList(slug, toolkit.BlockedToolSlugs) {
		decision.Reason = fmt.Sprintf("composio tool %q is explicitly blocked", slug)
		return decision
	}
	if decision.Destructive && !allowDestructive {
		decision.Reason = "composio destructive tools are disabled"
		return decision
	}
	if len(toolkit.AllowedToolSlugs) > 0 {
		if slugInList(slug, toolkit.AllowedToolSlugs) {
			decision.Allowed = true
			return decision
		}
		decision.Reason = fmt.Sprintf("composio tool %q is not in the toolkit allowlist", slug)
		return decision
	}
	if readOnly && !isComposioClearlyReadOnlySlug(slug) {
		decision.Reason = "composio is in read-only mode and this tool is not clearly read-only"
		return decision
	}
	decision.Allowed = true
	return decision
}

func (cfg ComposioPolicyConfig) findToolkit(slug string) (ComposioToolkitPolicy, bool) {
	slug = normalizeComposioSlug(slug)
	for _, tk := range cfg.Toolkits {
		if normalizeComposioSlug(tk.Slug) == slug {
			return tk, true
		}
	}
	return ComposioToolkitPolicy{}, false
}

func ComposioToolkitAllowsNaturalLanguage(cfg ComposioPolicyConfig, toolkitSlug string) bool {
	toolkit, ok := cfg.findToolkit(toolkitSlug)
	if ok && toolkit.AllowNaturalLanguageInput != nil {
		return *toolkit.AllowNaturalLanguageInput
	}
	return cfg.AllowNaturalLanguageInput
}

func ComposioPreferredConnectedAccount(cfg ComposioPolicyConfig, toolkitSlug string) string {
	toolkit, ok := cfg.findToolkit(toolkitSlug)
	if !ok {
		return ""
	}
	return strings.TrimSpace(toolkit.PreferredConnectedAccountID)
}

func normalizeComposioSlug(slug string) string {
	return strings.ToUpper(strings.TrimSpace(slug))
}

func slugInList(slug string, list []string) bool {
	slug = normalizeComposioSlug(slug)
	for _, entry := range list {
		if normalizeComposioSlug(entry) == slug {
			return true
		}
	}
	return false
}

func firstNonEmptyComposioString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

var composioSlugSplitter = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func composioSlugTokens(slug string) []string {
	raw := composioSlugSplitter.Split(strings.ToLower(strings.TrimSpace(slug)), -1)
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func isComposioDestructiveSlug(slug string) bool {
	for _, token := range composioSlugTokens(slug) {
		switch token {
		case "delete", "remove", "revoke", "disable", "purge", "drop", "destroy", "truncate", "erase", "wipe":
			return true
		}
	}
	return false
}

func isComposioClearlyReadOnlySlug(slug string) bool {
	tokens := composioSlugTokens(slug)
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		switch token {
		case "create", "update", "edit", "send", "post", "put", "patch", "write", "upload", "move", "copy", "rename", "archive", "invite", "share", "execute", "run", "import", "trigger", "submit", "approve", "merge", "close":
			return false
		}
	}
	for _, token := range tokens {
		switch token {
		case "get", "list", "search", "read", "fetch", "retrieve", "query", "find", "lookup", "describe", "view", "check", "status", "inspect":
			return true
		}
	}
	return false
}
