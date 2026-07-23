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
var teeveePlaylistURIAttrSingle = regexp.MustCompile(`URI='([^']+)'`)

func teeveeUnwrapProxiedStreamURL(raw string) string {
	for i := 0; i < 8; i++ {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return ""
		}
		if strings.HasPrefix(raw, teeveeStreamProxyPath+"?") {
			if parsed, err := url.Parse(raw); err == nil {
				if inner := strings.TrimSpace(parsed.Query().Get("url")); inner != "" {
					raw = inner
					continue
				}
			}
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		path := parsed.Path
		if path != teeveeStreamProxyPath && !strings.HasSuffix(path, teeveeStreamProxyPath) {
			return raw
		}
		inner := strings.TrimSpace(parsed.Query().Get("url"))
		if inner == "" {
			return raw
		}
		raw = inner
	}
	return raw
}

func teeveeHTTPClient(rawURL string) (*http.Client, error) {
	client, err := security.NewSSRFProtectedHTTPClientForURL(rawURL, 90*time.Second)
	if err != nil {
		return nil, err
	}
	if transport, ok := client.Transport.(*http.Transport); ok {
		cloned := transport.Clone()
		cloned.DisableCompression = true
		client.Transport = cloned
	}
	return client, nil
}

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
		upstreamURL := teeveeUnwrapProxiedStreamURL(rawURL)
		if upstreamURL == "" {
			jsonError(w, "invalid stream url", http.StatusBadRequest)
			return
		}
		if err := security.ValidateSSRF(upstreamURL); err != nil {
			jsonError(w, "stream URL is not allowed", http.StatusBadRequest)
			return
		}
		parsed, err := url.Parse(upstreamURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			jsonError(w, "invalid stream url", http.StatusBadRequest)
			return
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			jsonError(w, "only http and https streams are supported", http.StatusBadRequest)
			return
		}

		client, err := teeveeHTTPClient(upstreamURL)
		if err != nil {
			jsonError(w, "stream URL is not allowed", http.StatusBadRequest)
			return
		}
		upReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, nil)
		if err != nil {
			jsonError(w, "invalid stream url", http.StatusBadRequest)
			return
		}
		copyTeeVeeUpstreamHeaders(upReq, r)
		upReq.Header.Set("Accept-Encoding", "identity")

		resp, err := client.Do(upReq)
		if err != nil {
			jsonError(w, "upstream stream request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			jsonError(w, "upstream stream returned HTTP "+resp.Status, http.StatusBadGateway)
			return
		}

		teeveeSanitizeProxyResponseHeaders(resp)

		contentType := resp.Header.Get("Content-Type")
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", teeveeProxyContentType(contentType, upstreamURL))
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			return
		}

		if teeveeShouldRewritePlaylist(contentType, rawURL) {
			body, err := io.ReadAll(io.LimitReader(resp.Body, teeveeMaxPlaylistBytes))
			if err != nil {
				jsonError(w, "upstream playlist read failed", http.StatusBadGateway)
				return
			}
			body = rewriteTeeVeeHLSPlaylist(body, upstreamURL)
			contentType = "application/vnd.apple.mpegurl"
			w.Header().Set("Content-Type", teeveeProxyContentType(contentType, upstreamURL))
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}

		if resp.ContentLength > teeveeMaxSegmentProxySize {
			jsonError(w, "stream exceeds maximum proxy size", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", teeveeProxyContentType(contentType, upstreamURL))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		_, err = io.Copy(w, io.LimitReader(resp.Body, teeveeMaxSegmentProxySize))
		if err != nil {
			return
		}
	}
}

func teeveeSanitizeProxyResponseHeaders(upstream *http.Response) {
	for _, key := range []string{"Content-Encoding", "Content-Length", "Transfer-Encoding"} {
		upstream.Header.Del(key)
	}
}

func copyTeeVeeUpstreamHeaders(dst *http.Request, client *http.Request) {
	if accept := client.Header.Get("Accept"); accept != "" {
		dst.Header.Set("Accept", accept)
	}
	if rng := client.Header.Get("Range"); rng != "" {
		dst.Header.Set("Range", rng)
	}
	if userAgent := strings.TrimSpace(client.Header.Get("User-Agent")); userAgent != "" && len(userAgent) <= 512 {
		dst.Header.Set("User-Agent", userAgent)
	} else {
		dst.Header.Set("User-Agent", "AuraGo-TeeVee/1.0")
	}
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
		resolved := teeveeUnwrapProxiedStreamURL(teeveeResolvePlaylistURI(base, trim))
		out.WriteString(teeveeStreamProxyURL(resolved))
		out.WriteString("\n")
	}
	return out.Bytes()
}

func rewriteTeeVeePlaylistTagLine(base *url.URL, line string) string {
	line = teeveePlaylistURIAttr.ReplaceAllStringFunc(line, rewriteTeeVeeURIAttrMatch(base))
	line = teeveePlaylistURIAttrSingle.ReplaceAllStringFunc(line, rewriteTeeVeeURIAttrMatch(base))
	return line
}

func rewriteTeeVeeURIAttrMatch(base *url.URL) func(string) string {
	return func(match string) string {
		ref := ""
		if sub := teeveePlaylistURIAttr.FindStringSubmatch(match); len(sub) >= 2 {
			ref = sub[1]
		} else if sub := teeveePlaylistURIAttrSingle.FindStringSubmatch(match); len(sub) >= 2 {
			ref = sub[1]
		} else {
			return match
		}
		resolved := teeveeUnwrapProxiedStreamURL(teeveeResolvePlaylistURI(base, ref))
		proxied := teeveeStreamProxyURL(resolved)
		if strings.Contains(match, "URI='") {
			return `URI='` + proxied + `'`
		}
		return `URI="` + proxied + `"`
	}
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
