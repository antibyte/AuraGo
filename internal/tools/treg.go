package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const tregAPI = "https://treg.to"
const tregJSONLimit = 4 << 20

// TregClient shares the public-only transport with other native integrations.
// URLs, credentials, cost and idempotency headers are never tool arguments.
type TregClient struct {
	baseURL string
	http    *http.Client
	token   string
	// Authorize is re-evaluated immediately before each request, including polls.
	Authorize      func() (config.TregConfig, error)
	WorkspaceDir   string
	DataDir        string
	SessionID      string
	MediaDB        *sql.DB
	ValidateUpload func(string) error
}

type TregEndpoint struct {
	ID          string          `json:"id"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Name        string          `json:"name"`
	Provider    string          `json:"provider"`
	Summary     string          `json:"summary"`
	Kind        string          `json:"kind"`
	Platform    string          `json:"platform"`
	StrictQuery bool            `json:"strict_query"`
	Input       TregInput       `json:"input"`
	Cost        json.RawMessage `json:"cost"`
	Async       *tregAsync      `json:"async,omitempty"`
}

type TregInput struct {
	PathParams  map[string]json.RawMessage `json:"pathParams"`
	QueryParams map[string]json.RawMessage `json:"queryParams"`
	Body        map[string]json.RawMessage `json:"body"`
	BodyType    string                     `json:"bodyType"`
	StrictQuery bool                       `json:"strict_query"`
	Note        string                     `json:"note"`
}

type tregParameters struct {
	Path    map[string]any  `json:"path"`
	Query   map[string]any  `json:"query"`
	Body    json.RawMessage `json:"body"`
	Form    map[string]any  `json:"form"`
	Uploads []tregUpload    `json:"uploads"`
}

type TregResult struct {
	Status          string      `json:"status"`
	EndpointID      string      `json:"endpoint_id,omitempty"`
	CallID          string      `json:"call_id,omitempty"`
	IdempotencyKey  string      `json:"idempotency_key,omitempty"`
	HTTPStatus      int         `json:"http_status,omitempty"`
	ReservedMicro   *int64      `json:"reserved_micro"`
	ChargedMicro    *int64      `json:"charged_micro"`
	HeaderCostMicro *int64      `json:"header_cost_micro"`
	Continuation    string      `json:"continuation,omitempty"`
	RetryAfter      int         `json:"retry_after_seconds,omitempty"`
	Data            any         `json:"data,omitempty"`
	Media           []MediaItem `json:"media,omitempty"`
	Message         string      `json:"message,omitempty"`
}

type TregError struct {
	Status string
	Code   int
	Text   string
}

func (e *TregError) Error() string { return e.Text }

func NewTregClient(token string) (*TregClient, error) {
	client, err := security.NewStrictPublicHTTPClientForURL(tregAPI, 90*time.Second)
	if err != nil {
		return nil, fmt.Errorf("create treg transport: %w", err)
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	security.RegisterSensitive(token)
	return &TregClient{baseURL: tregAPI, http: client, token: token}, nil
}

func (c *TregClient) policy() (config.TregConfig, error) {
	if c.Authorize != nil {
		return c.Authorize()
	}
	return config.TregConfig{}, &TregError{Status: "policy_denied", Text: "treg authorization is unavailable"}
}

func (c *TregClient) grant(id, operation string, endpoint *TregEndpoint) (config.TregConfig, error) {
	cfg, err := c.policy()
	if err != nil {
		return cfg, err
	}
	g, err := cfg.Grant(id, operation)
	if err != nil {
		return cfg, &TregError{Status: "policy_denied", Text: err.Error()}
	}
	if endpoint != nil && (endpoint.ID != id || endpoint.Method != g.Method || endpoint.Path != g.Path) {
		return cfg, &TregError{Status: "policy_denied", Text: "treg endpoint contract changed; approve its current method and path again"}
	}
	return cfg, nil
}

func tregUSD(micro int64) string { return fmt.Sprintf("%d.%06d", micro/1_000_000, micro%1_000_000) }

func tregRandomID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(id[:])
}

func (c *TregClient) request(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType, key string, endpoint *TregEndpoint, operation string) (*http.Response, error) {
	cfg, err := c.policy()
	if endpoint != nil {
		cfg, err = c.grant(endpoint.ID, operation, endpoint)
	}
	if err != nil {
		return nil, err
	}
	if c.token == "" {
		return nil, &TregError{Status: "needs_setup", Text: "Store an organization-scoped treg token in Settings"}
	}
	target := c.baseURL + path
	if len(query) != 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("build treg request: %w", err)
	}
	req.Header.Set("X-Treg-Token", c.token)
	req.Header.Set("Accept", "application/json, image/*, audio/*, video/*")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if strings.HasPrefix(path, "/call/") {
		req.Header.Set("X-Treg-Route-Max-Cost", tregUSD(cfg.MaxCallCostMicro))
		req.Header.Set("Idempotency-Key", key)
	}
	// A new connection and non-replayable body prevent net/http's stale-connection
	// retry even with Idempotency-Key. An uncertain paid call is never resubmitted.
	req.Close = true
	req.GetBody = nil
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("treg transport did not confirm an outcome: %w", err)
	}
	return resp, nil
}

func tregReadJSON(resp *http.Response, out any) error {
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, tregJSONLimit+1))
	if err != nil {
		return fmt.Errorf("read treg response: %w", err)
	}
	if len(b) > tregJSONLimit {
		return fmt.Errorf("treg response exceeds 4 MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status := "error"
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			status = "needs_setup"
		}
		return &TregError{Status: status, Code: resp.StatusCode, Text: fmt.Sprintf("treg HTTP %d (authentication, budget or provider request rejected)", resp.StatusCode)}
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("decode treg response: %w", err)
	}
	return nil
}

func (c *TregClient) get(ctx context.Context, path string, query url.Values, out any) error {
	resp, err := c.request(ctx, http.MethodGet, path, query, nil, "", "", nil, "")
	if err != nil {
		return err
	}
	return tregReadJSON(resp, out)
}

func (c *TregClient) Search(ctx context.Context, query string, limit int) (any, error) {
	if len(query) > 500 {
		return nil, fmt.Errorf("treg search is limited to 500 characters")
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	var result any
	err := c.get(ctx, "/catalog/search", url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}, &result)
	return result, err
}

func (c *TregClient) Endpoint(ctx context.Context, id string) (TregEndpoint, any, error) {
	var descriptor struct {
		Endpoint TregEndpoint `json:"endpoint"`
	}
	if !config.ValidTregEndpointID(id) {
		return descriptor.Endpoint, nil, fmt.Errorf("invalid treg endpoint ID")
	}
	var raw json.RawMessage
	err := c.get(ctx, "/catalog/endpoints/"+id, nil, &raw)
	if err == nil {
		err = json.Unmarshal(raw, &descriptor)
	}
	if err == nil && (descriptor.Endpoint.ID != id || descriptor.Endpoint.Method == "" || descriptor.Endpoint.Path == "") {
		err = fmt.Errorf("treg returned an incomplete or mismatched endpoint contract")
	}
	return descriptor.Endpoint, raw, err
}

func (c *TregClient) organization(ctx context.Context) (string, error) {
	var me struct {
		OrgID string `json:"org_id"`
	}
	if err := c.get(ctx, "/auth/me", nil, &me); err != nil {
		return "", err
	}
	if me.OrgID == "" || len(me.OrgID) > 200 || strings.ContainsAny(me.OrgID, "/\\?#%\r\n") {
		return "", &TregError{Status: "needs_setup", Text: "treg requires an organization-scoped token; identity tokens are not supported"}
	}
	return me.OrgID, nil
}

func (c *TregClient) Balance(ctx context.Context) (any, error) {
	org, err := c.organization(ctx)
	if err != nil {
		return nil, err
	}
	var balance map[string]any
	if err := c.get(ctx, "/orgs/"+org+"/balance", nil, &balance); err != nil {
		return nil, err
	}
	return map[string]any{"org_id": org, "balance_micro": balance["balance_micro"], "holds_micro": balance["holds_micro"]}, nil
}

func (c *TregClient) Resources(ctx context.Context, provider, kind string) (any, error) {
	if len(provider)+len(kind) > 300 {
		return nil, fmt.Errorf("resource filters are too long")
	}
	org, err := c.organization(ctx)
	if err != nil {
		return nil, err
	}
	var result any
	err = c.get(ctx, "/orgs/"+org+"/provider-resources", url.Values{"provider": {provider}, "kind": {kind}}, &result)
	return result, err
}

// Accounting exposes receipt metadata only, never archived request/response bodies.
func (c *TregClient) Accounting(ctx context.Context, ref string) (any, error) {
	if !config.ValidTregEndpointID(ref) {
		return nil, fmt.Errorf("invalid treg call ID")
	}
	if _, err := c.organization(ctx); err != nil {
		return nil, err
	}
	var raw map[string]any
	resp, err := c.request(ctx, http.MethodGet, "/calls/"+ref, nil, nil, "", "", nil, "")
	if err != nil {
		return nil, err
	}
	missingAudit := resp.StatusCode == http.StatusNotFound
	if missingAudit {
		resp.StatusCode = http.StatusOK
	} // The documented 404 body may still contain durable ledger entries.
	if err := tregReadJSON(resp, &raw); err != nil {
		return nil, err
	}
	result := map[string]any{"reference": ref, "audit_missing": missingAudit, "ledger": raw["ledger"]}
	for _, key := range []string{"call_id", "call_ref", "status", "outcome", "reserved_micro", "charged_micro", "refunded_micro", "created_at", "settled_at"} {
		result[key] = raw[key]
	}
	if missingAudit {
		result["message"] = "An absent audit row does not prove that the call was not sent or billed. Inspect the ledger; missing amounts are unknown."
	}
	return result, nil
}

func (c *TregClient) Call(ctx context.Context, id, operation, parameters string) (*TregResult, error) {
	if _, err := c.grant(id, operation, nil); err != nil {
		return nil, err
	}
	ep, _, err := c.Endpoint(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := c.grant(id, operation, &ep); err != nil {
		return nil, err
	}
	query, body, contentType, err := c.encodeParameters(ep, parameters)
	if err != nil {
		return nil, err
	}
	if _, err := c.organization(ctx); err != nil {
		return nil, err
	}
	key := tregRandomID()
	resp, err := c.request(ctx, ep.Method, "/call/"+id, query, body, contentType, key, &ep, operation)
	if err != nil {
		if _, denied := err.(*TregError); denied {
			return nil, err
		}
		return &TregResult{Status: "unknown", EndpointID: id, IdempotencyKey: key, Message: "No confirmed outcome. Do not repeat this call; inspect treg call history using the idempotency key."}, nil
	}
	result := &TregResult{EndpointID: id, IdempotencyKey: key}
	if err := c.readCallResponse(resp, result); err != nil {
		result.Status, result.Message = "unknown", err.Error()+"; do not repeat the call"
		return result, nil
	}
	if result.Status == "success" || result.Status == "pending" {
		if err := c.acceptTask(ctx, &ep, operation, resp.Header, result); err != nil {
			result.Status, result.Message = "unknown", err.Error()+"; inspect call accounting, do not resubmit"
		}
	}
	return result, nil
}

func tregMicro(value string) *int64 {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 {
		return nil
	}
	return &n
}

func (c *TregClient) readCallResponse(resp *http.Response, result *TregResult) error {
	defer resp.Body.Close()
	result.HTTPStatus, result.CallID = resp.StatusCode, resp.Header.Get("X-Treg-Call-Id")
	result.HeaderCostMicro = tregMicro(resp.Header.Get("X-Treg-Cost-Micro"))
	result.RetryAfter, _ = strconv.Atoi(resp.Header.Get("Retry-After"))
	result.Status = "success"
	if resp.StatusCode == 202 {
		result.Status = "pending"
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || resp.Header.Get("X-Treg-Error") == "1" {
		result.Status = "error"
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			result.Status = "needs_setup"
		}
		if resp.StatusCode >= 500 {
			result.Status = "unknown"
		}
	}
	typ, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if result.Status == "success" && tregMediaType(typ) != "" {
		item, err := c.storeMedia(resp.Body, typ)
		if err != nil {
			return err
		}
		result.Media = []MediaItem{item}
	} else {
		b, err := io.ReadAll(io.LimitReader(resp.Body, tregJSONLimit+1))
		if err != nil {
			return fmt.Errorf("read treg call response: %w", err)
		}
		if len(b) > tregJSONLimit {
			return fmt.Errorf("treg call response exceeds 4 MiB")
		}
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		if len(b) != 0 && d.Decode(&result.Data) != nil {
			result.Data = string(b)
		}
	}
	return nil
}

func (c *TregClient) encodeParameters(ep TregEndpoint, raw string) (url.Values, io.Reader, string, error) {
	ep.Input.StrictQuery = ep.Input.StrictQuery || ep.StrictQuery
	if len(raw) > 1<<20 {
		return nil, nil, "", fmt.Errorf("treg parameters exceed 1 MiB")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	if !strings.HasPrefix(raw, "{") {
		return nil, nil, "", fmt.Errorf("parameters must be one JSON object")
	}
	var p tregParameters
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(&p); err != nil {
		return nil, nil, "", fmt.Errorf("invalid treg parameters: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, nil, "", fmt.Errorf("parameters must be one JSON object")
	}
	query := url.Values{}
	for _, part := range []struct {
		values map[string]any
		schema map[string]json.RawMessage
	}{{p.Path, ep.Input.PathParams}, {p.Query, ep.Input.QueryParams}} {
		for name, rule := range part.schema {
			var field struct {
				Required bool `json:"required"`
			}
			_ = json.Unmarshal(rule, &field)
			if field.Required && part.values[name] == nil {
				return nil, nil, "", fmt.Errorf("missing required parameter %s", name)
			}
		}
		for name, value := range part.values {
			if _, exists := query[name]; exists {
				return nil, nil, "", fmt.Errorf("duplicate path/query parameter %s", name)
			}
			if _, declared := part.schema[name]; !declared {
				return nil, nil, "", fmt.Errorf("undeclared path/query parameter %s", name)
			}
			values, err := tregValues(value)
			if err != nil || (ep.Input.StrictQuery && len(values) != 1) {
				return nil, nil, "", fmt.Errorf("invalid query parameter %s", name)
			}
			query[name] = values
		}
	}
	if ep.Input.StrictQuery && (len(p.Body) != 0 || len(p.Form) != 0 || len(p.Uploads) != 0) {
		return nil, nil, "", fmt.Errorf("this endpoint accepts query parameters only")
	}
	if len(p.Uploads) != 0 || len(p.Form) != 0 || ep.Input.BodyType == "multipart" {
		if len(p.Body) != 0 {
			return nil, nil, "", fmt.Errorf("body cannot be combined with form/uploads")
		}
		return c.encodeForm(ep, p, query)
	}
	if len(p.Body) == 0 {
		return query, nil, "", nil
	}
	if ep.Input.BodyType != "" && ep.Input.BodyType != "json" {
		return nil, nil, "", fmt.Errorf("endpoint requires %s input", ep.Input.BodyType)
	}
	return query, bytes.NewReader(p.Body), "application/json", nil
}

func tregValues(v any) ([]string, error) {
	switch v := v.(type) {
	case string:
		return []string{v}, nil
	case json.Number:
		return []string{string(v)}, nil
	case bool:
		return []string{strconv.FormatBool(v)}, nil
	case []any:
		var values []string
		for _, item := range v {
			s, err := tregValues(item)
			if err != nil || len(s) != 1 {
				return nil, fmt.Errorf("non-scalar form value")
			}
			values = append(values, s[0])
		}
		return values, nil
	default:
		return nil, fmt.Errorf("expected a scalar or an array of scalars")
	}
}
