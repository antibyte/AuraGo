package ui

import (
	"strings"
	"testing"
)

func TestDesktopStoreStartI18n(t *testing.T) {
	t.Parallel()

	preview := readDesktopAssetText(t, "js/desktop/apps/store-terminal-preview.js")
	if !strings.Contains(preview, "vd-store-frame-error-msg\">${esc(t('desktop.load_failed'))}") {
		t.Fatal("store terminal-preview frame must localize desktop.load_failed")
	}
	if strings.Contains(preview, "vd-store-frame-error-msg\">${esc(err.message)}") {
		t.Fatal("store terminal-preview frame still dumps err.message")
	}
	if !strings.Contains(preview, "message: t('desktop.load_failed')") {
		t.Fatal("store terminal-preview start toast must localize desktop.load_failed")
	}
	if strings.Contains(preview, "message: startErr.message") {
		t.Fatal("store terminal-preview start toast still dumps startErr.message")
	}

	store := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	if strings.Count(store, "message: t('desktop.load_failed')") < 2 {
		t.Fatal("store start toast and external-open must localize desktop.load_failed")
	}
	if strings.Contains(store, "message: startErr.message") {
		t.Fatal("store container start toast still dumps startErr.message")
	}
	if strings.Contains(store, "message: err.message") {
		t.Fatal("store external-open still dumps err.message")
	}
}
