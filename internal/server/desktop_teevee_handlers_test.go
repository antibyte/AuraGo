package server

import (
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