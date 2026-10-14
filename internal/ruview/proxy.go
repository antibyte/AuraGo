package ruview

import (
	"aurago/internal/security"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

// Proxy is used only after AuraGo authentication, Desktop scope and CSRF checks.
// Live integration policy is repeated here so callers cannot widen its grants.
func (m *Manager) Proxy(w http.ResponseWriter, r *http.Request, p string) {
	allowed, write, stream := Endpoint(r.Method, p)
	c := m.config()
	if !allowed {
		http.Error(w, "RuView endpoint is unavailable", http.StatusNotFound)
		return
	}
	if !c.RuView.Enabled || write && c.RuView.ReadOnly && !IsStop(r.Method, p) {
		http.Error(w, "RuView operation is disabled", http.StatusForbidden)
		return
	}
	if r.URL.RawQuery != "" {
		if len(r.URL.RawQuery) > 2048 {
			http.Error(w, "RuView query exceeds limit", 400)
			return
		}
		q := r.URL.Query()
		for key := range q {
			if key == "view" && len(q[key]) == 1 && (q.Get(key) == "raw" || q.Get(key) == "refined" || q.Get(key) == "both") {
				continue
			}
			if key != "limit" && key != "offset" && key != "since" && key != "node_id" && key != "window_ms" {
				http.Error(w, "RuView query parameter is unavailable", 400)
				return
			}
		}
	}
	if write && r.Body != nil {
		data, err := io.ReadAll(io.LimitReader(r.Body, 65537))
		if err != nil || len(data) > 65536 {
			http.Error(w, "RuView request exceeds limit", 400)
			return
		}
		var value any = map[string]any{}
		if len(data) > 0 {
			if json.Unmarshal(data, &value) != nil || !safeControlBody(value, 0) {
				http.Error(w, "Invalid RuView control payload", 400)
				return
			}
		}
		if p == "/api/v1/recording/start" {
			body, ok := value.(map[string]any)
			if !ok || body == nil {
				http.Error(w, "Invalid RuView recording payload", 400)
				return
			}
			if _, timed := body["duration_secs"]; timed {
				http.Error(w, "Timed RuView recording is unavailable; start and stop manually", 400)
				return
			}
			prefix := "rec"
			if raw, exists := body["id"]; exists {
				id, valid := raw.(string)
				if !valid || len(id) > 96 || !resourceID.MatchString(id) {
					http.Error(w, "Invalid RuView recording identifier", 400)
					return
				}
				prefix = id
			}
			// Upstream joins the ID to its recording directory and truncates existing files.
			body["id"] = prefix + "_" + rand.Text()
			data, _ = json.Marshal(body)
			r.Header.Set("Content-Type", "application/json")
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
	}
	if p == "/api/v1/recording/start" && c.RuView.Mode != "external" {
		if storageLimited(c) {
			http.Error(w, "RuView storage budget reached", 507)
			return
		}
	}
	client, target, token, err := m.client(c)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer client.CloseIdleConnections()
	ctx, done := m.bound(r.Context())
	defer done()
	if !stream {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}
	clone := r.Clone(ctx)
	clone.URL.Path = p
	clone.URL.RawPath = ""
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = p
			pr.Out.URL.RawPath = ""
			pr.Out.Host = target.Host
			headers := make(http.Header)
			for _, name := range []string{"Accept", "Content-Type", "Connection", "Upgrade", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Extensions"} {
				if value := pr.Out.Header.Get(name); value != "" {
					headers.Set(name, value)
				}
			}
			pr.Out.Header = headers
			if token != "" {
				pr.Out.Header.Set("Authorization", "Bearer "+token)
			}
		},
		Transport: client.Transport,
		ModifyResponse: func(resp *http.Response) error {
			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				return fmt.Errorf("RuView redirects are disabled")
			}
			for _, header := range []string{"Set-Cookie", "Location", "Access-Control-Allow-Origin", "Access-Control-Allow-Credentials", "Content-Security-Policy", "Service-Worker-Allowed"} {
				resp.Header.Del(header)
			}
			resp.Header.Set("Cache-Control", "no-store")
			if !stream {
				defer resp.Body.Close()
				data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
				if err != nil || len(data) > maxResponseBytes || !json.Valid(data) {
					return fmt.Errorf("invalid RuView JSON response")
				}
				data = []byte(security.Scrub(string(data)))
				resp.Body = io.NopCloser(bytes.NewReader(data))
				resp.ContentLength = int64(len(data))
				resp.Header.Set("Content-Length", fmt.Sprint(len(data)))
				resp.Header.Set("Content-Type", "application/json")
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "RuView upstream unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, clone)
}

func safeControlBody(value any, depth int) bool {
	if depth > 8 {
		return false
	}
	switch v := value.(type) {
	case map[string]any:
		for k, x := range v {
			lower := strings.ToLower(k)
			if strings.Contains(lower, "path") || strings.Contains(lower, "url") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "directory") {
				return false
			}
			if !safeControlBody(x, depth+1) {
				return false
			}
		}
	case []any:
		if len(v) > 256 {
			return false
		}
		for _, x := range v {
			if !safeControlBody(x, depth+1) {
				return false
			}
		}
	case string:
		if len(v) > 512 || strings.ContainsAny(v, "\x00\r\n") {
			return false
		}
	}
	return true
}
