package server

import (
	"bytes"
	"testing"
)

func TestInjectDesktopSDKChannelHTMLRunsBeforeAppScripts(t *testing.T) {
	t.Parallel()

	source := []byte(`<!doctype html><html><head><meta charset="utf-8"><script src="app.js"></script></head><body></body></html>`)
	got := injectDesktopSDKChannelHTML(source)
	marker := bytes.Index(got, []byte(desktopSDKChannelMarker))
	appScript := bytes.Index(got, []byte(`<script src="app.js">`))
	if marker < 0 || appScript < 0 || marker >= appScript {
		t.Fatalf("SDK channel bootstrap must precede application scripts: %s", got)
	}
	for _, want := range []string{
		"__aurago_sdk_channel",
		"__aurago_sdk_original_hash",
		"history.replaceState",
		"aurago.desktop.channel.challenge",
		"aurago.desktop.channel.handshake",
		"event.source.postMessage",
		"event.origin,[channel.port2]",
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("injected SDK channel bootstrap missing %q", want)
		}
	}
}

func TestInjectDesktopSDKChannelHTMLIsIdempotentAndHandlesMinimalDocuments(t *testing.T) {
	t.Parallel()

	for _, source := range [][]byte{
		[]byte(`<html><head><title>App</title></head><body>ready</body></html>`),
		[]byte(`<script>window.appReady=true</script>`),
		[]byte(`<main>ready</main>`),
	} {
		first := injectDesktopSDKChannelHTML(source)
		second := injectDesktopSDKChannelHTML(first)
		if !bytes.Equal(first, second) {
			t.Fatalf("injection must be idempotent:\nfirst:  %s\nsecond: %s", first, second)
		}
		if bytes.Count(first, []byte(desktopSDKChannelMarker)) != 1 {
			t.Fatalf("expected one SDK channel marker, got %s", first)
		}
	}
}

func TestInjectDesktopSDKChannelHTMLPrecedesScriptBeforeHead(t *testing.T) {
	source := []byte(`<!doctype html><script src="early.js"></script><html><head><title>App</title></head></html>`)
	got := injectDesktopSDKChannelHTML(source)
	marker := bytes.Index(got, []byte(desktopSDKChannelMarker))
	if !bytes.HasPrefix(got, []byte(`<!doctype html>`)) || marker < 0 || marker >= bytes.Index(got, []byte(`<script src="early.js">`)) {
		t.Fatal("bootstrap must preserve the doctype and run before the first application script")
	}
}
