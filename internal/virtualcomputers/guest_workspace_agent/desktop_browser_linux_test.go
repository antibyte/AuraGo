package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestDesktopBrowserRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name, method, body string
		status             int
	}{
		{"method", "GET", "", http.StatusMethodNotAllowed},
		{"malformed", "POST", "{", http.StatusBadRequest},
		{"extra-field", "POST", `{"url":"about:blank","command":"sh"}`, http.StatusBadRequest},
		{"trailing-json", "POST", `{"url":"about:blank"}{}`, http.StatusBadRequest},
		{"local-file", "POST", `{"url":"file:///etc/passwd"}`, http.StatusBadRequest},
		{"credentials", "POST", `{"url":"https://user:secret@example.com"}`, http.StatusBadRequest},
		{"too-large", "POST", `{"url":"https://example.com/` + strings.Repeat("x", 65536) + `"}`, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controller := &browserController{}
			response := httptest.NewRecorder()
			controller.handleDesktopBrowser(response, httptest.NewRequest(tc.method, "/open", strings.NewReader(tc.body)))
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if controller.browserCtx != nil {
				t.Fatal("invalid request started a browser")
			}
		})
	}
}

func TestDesktopBrowserLauncherSocketIsPrivateAndRemoved(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "browser.sock")
	closeServer, err := serveDesktopBrowser(&browserController{}, socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeServer)
	info, err := os.Stat(socket)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket permissions: info=%v err=%v", info, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := openDesktopBrowser(ctx, socket, []string{"https://example.com"}); err == nil {
		t.Fatal("cancelled desktop launch succeeded")
	}
	if err := openDesktopBrowser(context.Background(), socket, []string{"file:///etc/passwd"}); err == nil {
		t.Fatal("local file URL accepted")
	}
	closeServer()
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Fatalf("socket remained after shutdown: %v", err)
	}
}

// This check runs inside an isolated Debian rootfs with Xvfb, never against a
// developer's /run browser profile or /workspace directory.
func TestDesktopBrowserReusesAndReopensManagedChromium(t *testing.T) {
	if os.Getenv("AURAGO_DESKTOP_BROWSER_SMOKE") != "1" {
		t.Skip("requires an isolated desktop guest with Xvfb and Chromium")
	}
	controller := &browserController{}
	t.Cleanup(controller.close)
	socket := filepath.Join(t.TempDir(), "browser.sock")
	closeServer, err := serveDesktopBrowser(controller, socket)
	if err != nil {
		t.Fatal(err)
	}
	defer closeServer()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := openDesktopBrowser(ctx, socket, nil); err != nil {
		t.Fatal(err)
	}
	process := chromedp.FromContext(controller.browserCtx).Browser.Process()
	if err := openDesktopBrowser(ctx, socket, []string{"about:blank"}); err != nil {
		t.Fatal(err)
	}
	if chromedp.FromContext(controller.browserCtx).Browser.Process() != process {
		t.Fatal("desktop launcher started a second browser")
	}
	if err := controller.openFromDesktop(ctx, "http://127.0.0.1:1"); err == nil {
		t.Fatal("navigation to a closed port succeeded")
	}
	// Closing the browser's window must let the next menu launch restart it.
	closeCtx, cancelClose := context.WithTimeout(controller.browserCtx, 5*time.Second)
	err = chromedp.Cancel(closeCtx)
	cancelClose()
	if err != nil {
		t.Fatalf("close browser: %v", err)
	}
	select {
	case <-chromedp.FromContext(controller.browserCtx).Browser.LostConnection:
	case <-time.After(5 * time.Second):
		t.Fatal("browser did not close")
	}
	if err := openDesktopBrowser(ctx, socket, nil); err != nil {
		t.Fatal(err)
	}
	if chromedp.FromContext(controller.browserCtx).Browser.Process() == process {
		t.Fatal("closed browser was reused")
	}
}
