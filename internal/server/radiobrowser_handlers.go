package server

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// radioBrowserServers is the ordered list of public radio-browser.info API
// hosts. "all.api.radio-browser.info" is the official round-robin entry point;
// the regional mirrors act as failover targets when a single node returns 5xx
// or is unreachable (e.g. de1 returning 503).
var radioBrowserServers = []string{
	"all.api.radio-browser.info",
	"de1.api.radio-browser.info",
	"fi1.api.radio-browser.info",
	"nl1.api.radio-browser.info",
	"at1.api.radio-browser.info",
}

const radioBrowserPathPrefix = "/api/radio-browser"

// handleRadioBrowserProxy proxies read-only radio-browser.info JSON requests
// through the AuraGo backend so the browser never makes cross-origin calls
// (which fail CORS and break when a single API node is unavailable). It tries
// the preferred server first and falls back to the remaining mirrors on
// network errors or 5xx responses.
func handleRadioBrowserProxy(s *Server) http.HandlerFunc {
	client := &http.Client{Timeout: 12 * time.Second}
	var mu sync.Mutex
	preferred := 0

	tryServer := func(w http.ResponseWriter, r *http.Request, apiPath, host string) (bool, error) {
		target := "https://" + host + apiPath
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
		if err != nil {
			return false, err
		}
		// radio-browser.info asks consumers to send a descriptive User-Agent.
		req.Header.Set("User-Agent", "AuraGo-Radio/1.0 (https://github.com/antibyte/AuraGo)")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		// 5xx means this node is unhealthy — try the next mirror.
		if resp.StatusCode >= 500 {
			io.Copy(io.Discard, resp.Body)
			return false, nil
		}

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		if cache := resp.Header.Get("Cache-Control"); cache != "" {
			w.Header().Set("Cache-Control", cache)
		}
		w.WriteHeader(resp.StatusCode)
		_, err = io.Copy(w, resp.Body)
		return true, err
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		apiPath := strings.TrimPrefix(r.URL.Path, radioBrowserPathPrefix)
		if !strings.HasPrefix(apiPath, "/json/") {
			jsonError(w, "Invalid radio-browser path", http.StatusBadRequest)
			return
		}

		mu.Lock()
		start := preferred
		mu.Unlock()

		var lastErr error
		for i := 0; i < len(radioBrowserServers); i++ {
			idx := (start + i) % len(radioBrowserServers)
			host := radioBrowserServers[idx]
			ok, err := tryServer(w, r, apiPath, host)
			if err != nil {
				lastErr = err
				continue
			}
			if ok {
				// Remember the healthy server for the next request.
				mu.Lock()
				preferred = idx
				mu.Unlock()
				return
			}
		}

		s.Logger.Warn("All radio-browser mirrors failed", "error", lastErr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		msg := "Radio Browser unavailable"
		if lastErr != nil {
			msg = lastErr.Error()
		}
		_, _ = w.Write([]byte(`{"status":"error","message":` + jsonString(msg) + `}`))
	}
}

// jsonString returns a minimal JSON-encoded string. Kept local to avoid pulling
// in encoding/json just for an error envelope.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
