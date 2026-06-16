package server

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"
)

const teeveeStreamProxyPath = "/api/desktop/teevee/stream"

func handleDesktopTeeVeeStream(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
		if rawURL == "" {
			jsonError(w, "url query parameter is required", http.StatusBadRequest)
			return
		}
		if err := security.ValidateSSRF(rawURL); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			jsonError(w, "invalid stream url", http.StatusBadRequest)
			return
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			jsonError(w, "only http and https streams are supported", http.StatusBadRequest)
			return
		}

		client, err := security.NewSSRFProtectedHTTPClientForURL(rawURL, 90*time.Second)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		upReq, err := http.NewRequestWithContext(r.Context(), r.Method, rawURL, nil)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		copyTeeVeeUpstreamHeaders(upReq, r)

		resp, err := client.Do(upReq)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			jsonError(w, "upstream stream returned HTTP "+resp.Status, http.StatusBadGateway)
			return
		}

		contentType := resp.Header.Get("Content-Type")
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", teeveeProxyContentType(contentType, rawURL))
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			return
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadGateway)
			return
		}

		if teeveeLooksLikeHLSPlaylist(contentType, rawURL, body) {
			body = rewriteTeeVeeHLSPlaylist(body, rawURL)
			contentType = "application/vnd.apple.mpegurl"
		}

		w.Header().Set("Content-Type", teeveeProxyContentType(contentType, rawURL))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

func copyTeeVeeUpstreamHeaders(dst *http.Request, client *http.Request) {
	if accept := client.Header.Get("Accept"); accept != "" {
		dst.Header.Set("Accept", accept)
	}
	if rng := client.Header.Get("Range"); rng != "" {
		dst.Header.Set("Range", rng)
	}
	dst.Header.Set("User-Agent", "AuraGo-TeeVee/1.0")
}

func teeveeProxyContentType(header, rawURL string) string {
	if strings.TrimSpace(header) != "" {
		return header
	}
	if strings.Contains(strings.ToLower(rawURL), ".m3u8") {
		return "application/vnd.apple.mpegurl"
	}
	return "application/octet-stream"
}

func teeveeLooksLikeHLSPlaylist(contentType, rawURL string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "mpegurl") || strings.Contains(ct, "m3u8") {
		return true
	}
	if strings.Contains(strings.ToLower(rawURL), ".m3u8") {
		return true
	}
	trim := bytes.TrimSpace(body)
	return bytes.HasPrefix(trim, []byte("#EXTM3U"))
}

func rewriteTeeVeeHLSPlaylist(body []byte, baseURL string) []byte {
	base, err := url.Parse(baseURL)
	if err != nil {
		return body
	}
	var out bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			out.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "#") {
			out.WriteString(line)
			out.WriteString("\n")
			continue
		}
		resolved := teeveeResolvePlaylistURI(base, line)
		out.WriteString(teeveeStreamProxyURL(resolved))
		out.WriteString("\n")
	}
	return out.Bytes()
}

func teeveeResolvePlaylistURI(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	return base.ResolveReference(parsed).String()
}

func teeveeStreamProxyURL(raw string) string {
	return teeveeStreamProxyPath + "?url=" + url.QueryEscape(raw)
}

// teeveeStreamProxyPathForTests exposes the path constant to ui/server tests.
func teeveeStreamProxyPathForTests() string { return teeveeStreamProxyPath }