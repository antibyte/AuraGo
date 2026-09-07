package ui

import (
	"strings"
	"testing"
)

func TestDesktopWidgetPersistI18n(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	if strings.Count(shell, "message: t('desktop.widget_update_failed')") < 3 {
		t.Fatal("widget persist toasts must localize desktop.widget_update_failed three times")
	}
	if strings.Contains(shell, "message: err.message") {
		t.Fatal("widget persist toasts still dump err.message")
	}
	if !strings.Contains(shell, "throw new Error('HTTP')") {
		t.Fatal("weather fetch must throw the HTTP sentinel without a status")
	}
	if strings.Contains(shell, "'HTTP ' + res.status") {
		t.Fatal("weather fetch still concatenates HTTP with a status")
	}
}
