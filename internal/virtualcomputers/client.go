package virtualcomputers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
)

type ClientConfig struct {
	BaseURL    string
	Token      string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

type RESTError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e RESTError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		body = http.StatusText(e.StatusCode)
	}
	return fmt.Sprintf("boringd %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, body)
}

func NewClient(cfg ClientConfig) (*Client, error) {
	rawURL := strings.TrimSpace(cfg.BaseURL)
	if rawURL == "" {
		rawURL = defaultBoringdURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse boringd url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("boringd url must include scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Client{baseURL: parsed, token: strings.TrimSpace(cfg.Token), httpClient: client}, nil
}

func (c *Client) Status(ctx context.Context) (map[string]interface{}, error) {
	var payload map[string]interface{}
	if err := c.doJSON(ctx, http.MethodGet, "/healthz", nil, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) ListMachines(ctx context.Context) ([]Machine, error) {
	var machines []Machine
	if err := c.doJSONFlexibleList(ctx, http.MethodGet, "/v1/machines", nil, "machines", &machines); err != nil {
		return nil, err
	}
	return machines, nil
}

func (c *Client) GetMachine(ctx context.Context, id string) (Machine, error) {
	var machine Machine
	if err := c.doJSON(ctx, http.MethodGet, "/v1/machines/"+pathEscapeID(id), nil, &machine); err != nil {
		return Machine{}, err
	}
	return machine, nil
}

func (c *Client) LaunchMachine(ctx context.Context, req LaunchMachineRequest) (Machine, error) {
	payload := launchMachineRequest{
		Template:      req.Template,
		Name:          req.Name,
		TTLSeconds:    clampTTL(req.TTLSeconds, MaxTTLSeconds),
		AllowInternet: req.AllowInternet,
		Volume:        firstString(req.Volumes),
		Persistent:    req.Persistent,
		Volumes:       req.Volumes,
		Metadata:      req.Metadata,
	}
	var machine Machine
	if err := c.doJSON(ctx, http.MethodPost, "/v1/machines", payload, &machine); err != nil {
		return Machine{}, err
	}
	return machine, nil
}

func (c *Client) DestroyMachine(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/v1/machines/"+pathEscapeID(id), nil, nil)
}

func (c *Client) ExtendMachine(ctx context.Context, id string, ttlSeconds int) (Machine, error) {
	var machine Machine
	payload := map[string]int{"ttl_seconds": clampTTL(ttlSeconds, MaxTTLSeconds)}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+pathEscapeID(id)+"/extend", payload, &machine); err != nil {
		return Machine{}, err
	}
	return machine, nil
}

func (c *Client) ForkMachine(ctx context.Context, id string, ttlSeconds int) (Machine, error) {
	_ = ttlSeconds
	var machine Machine
	if err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+pathEscapeID(id)+"/branch", nil, &machine); err != nil {
		return Machine{}, err
	}
	return machine, nil
}

func (c *Client) Exec(ctx context.Context, id string, req ExecRequest) (ExecResult, error) {
	var result ExecResult
	if err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+pathEscapeID(id)+"/exec", req, &result); err != nil {
		return ExecResult{}, err
	}
	return result, nil
}

func (c *Client) Screenshot(ctx context.Context, id string) (Screenshot, error) {
	var shot Screenshot
	if err := c.doJSON(ctx, http.MethodGet, "/v1/machines/"+pathEscapeID(id)+"/screenshot", nil, &shot); err != nil {
		return Screenshot{}, err
	}
	if shot.Base64 != "" && len(shot.Data) == 0 {
		if decoded, err := base64.StdEncoding.DecodeString(shot.Base64); err == nil {
			shot.Data = decoded
		}
	}
	return shot, nil
}

func (c *Client) Upload(ctx context.Context, id, remotePath string, content []byte) (map[string]interface{}, error) {
	var payload map[string]interface{}
	headers := map[string]string{
		"Content-Type": "application/octet-stream",
		"X-Filename":   pathpkg.Base(remotePath),
	}
	if err := c.do(ctx, http.MethodPost, "/v1/machines/"+pathEscapeID(id)+"/upload", bytes.NewReader(content), headers, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) Download(ctx context.Context, id, remotePath string) ([]byte, string, error) {
	if c == nil || c.baseURL == nil {
		return nil, "", fmt.Errorf("client is not configured")
	}
	u := *c.baseURL
	u.Path = joinURLPath(u.Path, "/v1/machines/"+pathEscapeID(id)+"/download")
	query := u.Query()
	query.Set("path", remotePath)
	u.RawQuery = query.Encode()
	req, err := c.newRequest(ctx, http.MethodGet, u.String(), nil, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("boringd download request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", c.responseError(req.Method, req.URL.RequestURI(), resp)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024*1024))
	if err != nil {
		return nil, "", fmt.Errorf("read boringd download: %w", err)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

func (c *Client) ListTemplates(ctx context.Context) ([]Template, error) {
	var templates []Template
	if err := c.doJSONFlexibleList(ctx, http.MethodGet, "/v1/templates", nil, "templates", &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

func (c *Client) Publish(ctx context.Context, id string) (map[string]interface{}, error) {
	var payload map[string]interface{}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+pathEscapeID(id)+"/publish", nil, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) ListVolumes(ctx context.Context) ([]Volume, error) {
	var volumes []Volume
	if err := c.doJSONFlexibleList(ctx, http.MethodGet, "/v1/volumes", nil, "volumes", &volumes); err != nil {
		return nil, err
	}
	return volumes, nil
}

func (c *Client) CreateVolume(ctx context.Context, name string, sizeBytes int64) (Volume, error) {
	var volume Volume
	payload := map[string]interface{}{"name": name, "size_bytes": sizeBytes}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/volumes", payload, &volume); err != nil {
		return Volume{}, err
	}
	return volume, nil
}

func (c *Client) SaveMachine(ctx context.Context, id, name string) (map[string]interface{}, error) {
	var payload map[string]interface{}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+pathEscapeID(id)+"/save", map[string]string{"name": name}, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) RunAgentTask(ctx context.Context, id, channel, instruction string) (map[string]interface{}, error) {
	channel = strings.Trim(channel, "/")
	if channel == "" {
		channel = "agent"
	}
	var payload map[string]interface{}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+pathEscapeID(id)+"/"+channel, map[string]string{"instruction": instruction}, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) WebSocketURL(machineID, channel string) (string, http.Header, error) {
	if c == nil || c.baseURL == nil {
		return "", nil, fmt.Errorf("client is not configured")
	}
	u := *c.baseURL
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = joinURLPath(u.Path, "/v1/machines/"+pathEscapeID(machineID)+"/"+strings.Trim(channel, "/"))
	header := http.Header{}
	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}
	return u.String(), header, nil
}

func (c *Client) PreviewTargetURL(machineID string, port int, suffix string) (*url.URL, error) {
	if c == nil || c.baseURL == nil {
		return nil, fmt.Errorf("client is not configured")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid preview port")
	}
	u := *c.baseURL
	u.Path = joinURLPath(u.Path, "/v1/machines/"+pathEscapeID(machineID)+"/web/"+strconv.Itoa(port)+"/"+strings.TrimLeft(suffix, "/"))
	return &u, nil
}

func (c *Client) doJSON(ctx context.Context, method, apiPath string, in interface{}, out interface{}) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode boringd request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	return c.do(ctx, method, apiPath, body, map[string]string{"Content-Type": "application/json"}, out)
}

func (c *Client) do(ctx context.Context, method, apiPath string, body io.Reader, headers map[string]string, out interface{}) error {
	req, err := c.newRequest(ctx, method, apiPath, body, headers)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("boringd %s %s: %w", method, apiPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.responseError(method, apiPath, resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	prefix, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return fmt.Errorf("read boringd response: %w", err)
	}
	reader := io.MultiReader(bytes.NewReader(prefix), resp.Body)
	if err := json.NewDecoder(reader).Decode(out); err != nil {
		return c.decodeError(method, apiPath, resp, prefix, err)
	}
	return nil
}

func (c *Client) doJSONFlexibleList(ctx context.Context, method, apiPath string, in interface{}, field string, out interface{}) error {
	var raw json.RawMessage
	if err := c.doJSON(ctx, method, apiPath, in, &raw); err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(trimmed, out)
	}
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &wrapped); err != nil {
		return fmt.Errorf("decode boringd list response: %w", err)
	}
	if payload, ok := wrapped[field]; ok {
		return json.Unmarshal(payload, out)
	}
	return json.Unmarshal(trimmed, out)
}

func (c *Client) newRequest(ctx context.Context, method, apiPath string, body io.Reader, headers map[string]string) (*http.Request, error) {
	if c == nil || c.baseURL == nil {
		return nil, fmt.Errorf("client is not configured")
	}
	u := *c.baseURL
	if strings.HasPrefix(apiPath, "http://") || strings.HasPrefix(apiPath, "https://") {
		parsed, err := url.Parse(apiPath)
		if err != nil {
			return nil, err
		}
		u = *parsed
	} else {
		u.Path = joinURLPath(u.Path, apiPath)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create boringd request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range headers {
		if strings.TrimSpace(v) != "" {
			req.Header.Set(k, v)
		}
	}
	return req, nil
}

func (c *Client) responseError(method, apiPath string, resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	body := redactSecrets(string(data), c.token)
	return RESTError{Method: method, Path: apiPath, StatusCode: resp.StatusCode, Body: body}
}

func (c *Client) decodeError(method, apiPath string, resp *http.Response, prefix []byte, err error) error {
	contentType := resp.Header.Get("Content-Type")
	sample := compactBodySample(redactSecrets(string(prefix), c.token))
	if looksNonJSONResponse(contentType, prefix) {
		return fmt.Errorf("decode boringd response: non-JSON response from boringd %s %s (status %d, content-type %q, body starts %q); check virtual_computers.control_plane.boringd_url points to boringd, not AuraGo UI or a redirect: %w", method, apiPath, resp.StatusCode, contentType, sample, err)
	}
	return fmt.Errorf("decode boringd response: %w", err)
}

func looksNonJSONResponse(contentType string, prefix []byte) bool {
	trimmed := bytes.TrimSpace(prefix)
	if len(trimmed) > 0 && trimmed[0] == '<' {
		return true
	}
	contentType = strings.ToLower(contentType)
	return contentType != "" && !strings.Contains(contentType, "json")
}

func compactBodySample(sample string) string {
	sample = strings.Join(strings.Fields(sample), " ")
	if len(sample) > 200 {
		return sample[:200] + "..."
	}
	return sample
}

func clampTTL(ttl, maxTTL int) int {
	if maxTTL <= 0 || maxTTL > MaxTTLSeconds {
		maxTTL = MaxTTLSeconds
	}
	if ttl <= 0 {
		ttl = DefaultTTLSeconds
	}
	if ttl < MinTTLSeconds {
		return MinTTLSeconds
	}
	if ttl > maxTTL {
		return maxTTL
	}
	return ttl
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func pathEscapeID(id string) string {
	return url.PathEscape(sanitizeID(id))
}

func sanitizeID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.ReplaceAll(id, "/", "")
	id = strings.ReplaceAll(id, "\\", "")
	id = strings.ReplaceAll(id, "..", ".")
	return id
}

func joinURLPath(basePath, apiPath string) string {
	if strings.TrimSpace(basePath) == "" {
		basePath = "/"
	}
	if strings.TrimSpace(apiPath) == "" {
		return basePath
	}
	joined := pathpkg.Join("/", basePath, apiPath)
	if strings.HasSuffix(apiPath, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return joined
}

func PreviewProxyPath(machineID string, port int, suffix string) string {
	cleanSuffix := strings.TrimLeft(suffix, "/")
	if cleanSuffix != "" {
		cleanSuffix = "/" + cleanSuffix
	}
	return "/api/virtual-computers/machines/" + sanitizeID(machineID) + "/web/" + strconv.Itoa(port) + cleanSuffix
}

func redactSecrets(value string, secrets ...string) string {
	out := value
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		out = strings.ReplaceAll(out, secret, "<redacted>")
	}
	return out
}
