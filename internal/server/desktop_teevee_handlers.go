package server

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"aurago/internal/security"
)

const (
	teeveeStreamProxyPath     = "/api/desktop/teevee/stream"
	teeveeMaxPlaylistBytes    = 4 << 20
	teeveeMaxSegmentProxySize = 64 << 20
)

var teeveePlaylistURIAttr = regexp.MustCompile(`URI="([^"]+)"`)

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

		if teeveeShouldRewritePlaylist(contentType, rawURL) {
			body, err := io.ReadAll(io.LimitReader(resp.Body, teeveeMaxPlaylistBytes))
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadGateway)
				return
			}
			body = rewriteTeeVeeHLSPlaylist(body, rawURL)
			contentType = "application/vnd.apple.mpegurl"
			w.Header().Set("Content-Type", teeveeProxyContentType(contentType, rawURL))
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}

		if resp.ContentLength > teeveeMaxSegmentProxySize {
			jsonError(w, "stream exceeds maximum proxy size", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", teeveeProxyContentType(contentType, rawURL))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		if resp.ContentLength > 0 {
			w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
		}
		w.WriteHeader(http.StatusOK)
		_, err = io.Copy(w, io.LimitReader(resp.Body, teeveeMaxSegmentProxySize))
		if err != nil {
			return
		}
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

func teeveeShouldRewritePlaylist(contentType, rawURL string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "mpegurl") || strings.Contains(ct, "m3u8") {
		return true
	}
	return strings.Contains(strings.ToLower(rawURL), ".m3u8")
}

func teeveeLooksLikeHLSPlaylist(contentType, rawURL string, body []byte) bool {
	if teeveeShouldRewritePlaylist(contentType, rawURL) {
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
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if trim == "" {
			out.WriteString("\n")
			continue
		}
		if strings.HasPrefix(trim, "#") {
			out.WriteString(rewriteTeeVeePlaylistTagLine(base, line))
			out.WriteString("\n")
			continue
		}
		resolved := teeveeResolvePlaylistURI(base, trim)
		out.WriteString(teeveeStreamProxyURL(resolved))
		out.WriteString("\n")
	}
	return out.Bytes()
}

func rewriteTeeVeePlaylistTagLine(base *url.URL, line string) string {
	return teeveePlaylistURIAttr.ReplaceAllStringFunc(line, func(match string) string {
		sub := teeveePlaylistURIAttr.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		resolved := teeveeResolvePlaylistURI(base, sub[1])
		return `URI="` + teeveeStreamProxyURL(resolved) + `"`
	})
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