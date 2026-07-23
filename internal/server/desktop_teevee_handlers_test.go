package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRewriteTeeVeeHLSPlaylistRewritesRelativeAndAbsolute(t *testing.T) {
	t.Parallel()

	base := "http://cdn.example.com/live/ch.m3u8"
	body := []byte("#EXTM3U\n#EXTINF:1,\nseg1.ts\nhttp://other.example.com/alt.m3u8\n")
	out := rewriteTeeVeeHLSPlaylist(body, base)
	text := string(out)
	if !strings.Contains(text, "/api/desktop/teevee/stream?url=") {
		t.Fatalf("expected proxy URLs in playlist: %s", text)
	}
	if strings.Contains(text, "seg1.ts\n") && !strings.Contains(text, "cdn.example.com") {
		t.Fatalf("relative segment should resolve against base: %s", text)
	}
}

func TestRewriteTeeVeeHLSPlaylistRewritesURIAttributes(t *testing.T) {
	t.Parallel()

	base := "https://cdn.example.com/master.m3u8"
	body := []byte("#EXTM3U\n#EXT-X-MEDIA:TYPE=AUDIO,URI=\"audio/eng.m3u8\"\n")
	out := rewriteTeeVeeHLSPlaylist(body, base)
	text := string(out)
	if !strings.Contains(text, `URI="/api/desktop/teevee/stream?url=`) {
		t.Fatalf("expected proxied URI attribute: %s", text)
	}
	if strings.Contains(text, `URI="audio/eng.m3u8"`) {
		t.Fatal("raw URI must be rewritten")
	}
}

func TestTeeVeeStreamProxyURLEncodes(t *testing.T) {
	t.Parallel()

	raw := "http://example.com/stream.m3u8?x=1"
	got := teeveeStreamProxyURL(raw)
	if !strings.HasPrefix(got, "/api/desktop/teevee/stream?url=") {
		t.Fatalf("unexpected proxy url: %s", got)
	}
	_, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTeeVeeUnwrapProxiedStreamURL(t *testing.T) {
	t.Parallel()

	inner := "http://customized-cdn.net/invalidurlstream1/streamPlaylist.m3u8"
	once := teeveeStreamProxyURL(inner)
	nested := "https://aurago.example/api/desktop/teevee/stream?url=" + url.QueryEscape("https://aurago.example"+once)
	if got := teeveeUnwrapProxiedStreamURL(nested); got != inner {
		t.Fatalf("nested unwrap: got %q want %q", got, inner)
	}
	if got := teeveeUnwrapProxiedStreamURL(once); got != inner {
		t.Fatalf("single proxy path unwrap: got %q want %q", got, inner)
	}
}

func TestRewriteTeeVeeHLSPlaylistDoesNotDoubleWrapProxyURL(t *testing.T) {
	t.Parallel()

	inner := "http://cdn.example.com/live/ch.m3u8"
	proxied := "https://aurago.example/api/desktop/teevee/stream?url=" + url.QueryEscape(inner)
	body := []byte("#EXTM3U\n" + proxied + "\n")
	out := rewriteTeeVeeHLSPlaylist(body, inner)
	text := string(out)
	if strings.Contains(text, url.QueryEscape(url.QueryEscape(inner))) {
		t.Fatalf("playlist must not nest proxy urls: %s", text)
	}
	if !strings.Contains(text, url.QueryEscape(inner)) {
		t.Fatalf("expected single-encoded upstream in proxy url: %s", text)
	}
}

func TestCopyTeeVeeUpstreamHeadersUsesBoundedBrowserUserAgent(t *testing.T) {
	t.Parallel()

	clientReq, err := http.NewRequest(http.MethodGet, "https://aurago.example/api/desktop/teevee/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	clientReq.Header.Set("User-Agent", "Browser UA")
	upstreamReq, err := http.NewRequest(http.MethodGet, "https://cdn.example.com/live.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}

	copyTeeVeeUpstreamHeaders(upstreamReq, clientReq)
	if got := upstreamReq.Header.Get("User-Agent"); got != "Browser UA" {
		t.Fatalf("User-Agent = %q, want forwarded browser value", got)
	}

	clientReq.Header.Set("User-Agent", strings.Repeat("x", 513))
	copyTeeVeeUpstreamHeaders(upstreamReq, clientReq)
	if got := upstreamReq.Header.Get("User-Agent"); got != "AuraGo-TeeVee/1.0" {
		t.Fatalf("oversized User-Agent must use safe fallback, got %q", got)
	}
}
