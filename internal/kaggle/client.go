package kaggle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
)

const apiBase = "https://api.kaggle.com/v1/"

type Credentials struct{ Mode, Token, Username, Key string }
type Client struct {
	http        *http.Client
	base        string
	credentials Credentials
}
type APIError struct {
	Status     int
	RetryAfter time.Duration
	Uncertain  bool
}

func (e *APIError) Error() string {
	if e.Uncertain {
		return "Kaggle outcome is unknown; this mutation was not retried"
	}
	switch e.Status {
	case 401:
		return "Kaggle credentials are invalid or expired"
	case 403:
		return "Kaggle denied access; check resource access and competition participation on Kaggle"
	case 429:
		return "Kaggle quota or rate limit reached"
	}
	return fmt.Sprintf("Kaggle request failed (HTTP %d)", e.Status)
}

func NewClient(c Credentials, timeout time.Duration) (*Client, error) {
	if c.Mode == "token" && c.Token == "" {
		return nil, errors.New("Kaggle API token is missing")
	}
	if c.Mode == "legacy" && (c.Username == "" || c.Key == "") {
		return nil, errors.New("Kaggle legacy username and key are required")
	}
	if c.Mode != "token" && c.Mode != "legacy" {
		return nil, errors.New("unsupported Kaggle authentication mode")
	}
	security.RegisterSensitive(c.Token)
	security.RegisterSensitive(c.Key)
	h := security.NewSSRFProtectedHTTPClientSameOrigin(timeout)
	// Download redirects are handled separately, without credentials.
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{http: h, base: apiBase, credentials: c}, nil
}

func (c *Client) request(ctx context.Context, op Operation, body any) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode Kaggle request: %w", err)
	}
	if len(b) > 20<<20 {
		return nil, errors.New("Kaggle request too large")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+op.Service+"/"+op.Method, bytes.NewReader(b))
	if err != nil {
		return nil, errors.New("invalid Kaggle request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AuraGo-Kaggle/1 SDK-contract/0.1.38")
	if c.credentials.Mode == "token" {
		req.Header.Set("Authorization", "Bearer "+c.credentials.Token)
	} else {
		req.SetBasicAuth(c.credentials.Username, c.credentials.Key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &APIError{Uncertain: op.Grant != "read" && op.Grant != "download"}
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		seconds, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		if seconds > 300 {
			seconds = 300
		}
		return nil, &APIError{Status: resp.StatusCode, RetryAfter: time.Duration(seconds) * time.Second, Uncertain: resp.StatusCode >= 500 && op.Grant != "read" && op.Grant != "download"}
	}
	return resp, nil
}

func (c *Client) Call(ctx context.Context, op Operation, body any) (json.RawMessage, error) {
	resp, err := c.request(ctx, op, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Uncertain: op.Grant != "read"}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (16<<20)+1))
	if err != nil || len(b) > 16<<20 {
		return nil, &APIError{Uncertain: op.Grant != "read"}
	}
	if len(b) == 0 {
		b = []byte("{}")
	}
	if !json.Valid(b) {
		return nil, &APIError{Uncertain: op.Grant != "read"}
	}
	var outcome map[string]json.RawMessage
	if json.Unmarshal(b, &outcome) == nil {
		for _, key := range []string{"error", "errorMessage"} {
			if v := outcome[key]; len(v) > 0 && string(v) != "null" && string(v) != `""` {
				return nil, errors.New("Kaggle rejected the requested operation")
			}
		}
	}
	return b, nil
}

func (c *Client) Identity(ctx context.Context) (string, error) {
	if c.credentials.Mode == "legacy" {
		op, _ := Lookup("notebooks", "get_accelerator_quota_statistics")
		if _, err := c.Call(ctx, op, struct{}{}); err != nil {
			return "", err
		}
		return c.credentials.Username, nil
	}
	op := operations["internal/introspect_token"]
	b, err := c.Call(ctx, op, map[string]string{"token": c.credentials.Token})
	if err != nil {
		return "", err
	}
	var v struct {
		Active   bool   `json:"active"`
		Username string `json:"username"`
	}
	if json.Unmarshal(b, &v) != nil || !v.Active || v.Username == "" {
		return "", errors.New("Kaggle token has no active account identity")
	}
	return v.Username, nil
}

// StorageClient returns an uncredentialed, DNS-pinned public HTTPS client.
func storageClient(ctx context.Context, location string, timeout time.Duration) (*http.Client, error) {
	security.RegisterSensitive(location)
	u, err := url.Parse(location)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" {
		return nil, errors.New("Kaggle returned an invalid storage URL")
	}
	h, err := security.NewSSRFProtectedHTTPClientForURLContext(ctx, location, timeout)
	if err != nil {
		return nil, errors.New("Kaggle storage URL was rejected")
	}
	previous := h.CheckRedirect
	h.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		security.RegisterSensitive(req.URL.String())
		req.Header.Del("Authorization")
		req.Header.Del("Cookie")
		if req.URL.Scheme != "https" || req.URL.User != nil {
			return errors.New("invalid storage redirect")
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	return h, nil
}

// Preserve int64 remote identities through generic response processing.
func decodeResponse(raw []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	return d.Decode(value)
}

func cleanOutput(raw json.RawMessage) json.RawMessage {
	var v any
	if decodeResponse(raw, &v) != nil {
		return json.RawMessage(`{}`)
	}
	var clean func(any) any
	clean = func(x any) any {
		switch a := x.(type) {
		case map[string]any:
			for k, v := range a {
				lower := strings.ToLower(k)
				if lower == "token" || lower == "accesstoken" || lower == "refreshtoken" || strings.Contains(lower, "createurl") || strings.Contains(lower, "downloadurl") || strings.Contains(lower, "signedurl") {
					delete(a, k)
				} else {
					a[k] = clean(v)
				}
			}
		case []any:
			for i, v := range a {
				a[i] = clean(v)
			}
		case string:
			return security.Scrub(a)
		}
		return x
	}
	v = clean(v)
	b, _ := json.Marshal(v)
	return b
}
