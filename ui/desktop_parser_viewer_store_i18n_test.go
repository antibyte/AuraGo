package ui

import (
	"strings"
	"testing"
)

func TestDesktopParserViewerStoreI18n(t *testing.T) {
	t.Parallel()

	homepage := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	if !strings.Contains(homepage, "t('desktop.chat_request_failed')") {
		t.Fatal("homepage missing parser throw must localize desktop.chat_request_failed")
	}
	if strings.Contains(homepage, "Chat stream parser not loaded") {
		t.Fatal("homepage still hardcodes Chat stream parser not loaded")
	}

	viewer := readDesktopAssetText(t, "js/desktop/apps/viewer.js")
	if !strings.Contains(viewer, "t('viewer.error')") {
		t.Fatal("viewer markdown-it fallback must localize viewer.error")
	}
	if strings.Contains(viewer, "markdown-it not loaded") {
		t.Fatal("viewer still hardcodes markdown-it not loaded")
	}

	store := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	if !strings.Contains(store, "vd-store-frame-error-msg\">${esc(t('desktop.load_failed'))}") {
		t.Fatal("store container-app frame must localize desktop.load_failed")
	}
	if strings.Contains(store, "vd-store-frame-error-msg\">${esc(err.message)}") {
		t.Fatal("store container-app frame still dumps err.message")
	}
}
