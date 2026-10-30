package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

const desktopBrowserSocket = runtimeRoot + "/desktop-browser.sock"

// Desktop launchers ask the existing controller to open/focus its browser.
// Running Chromium directly as root either fails or creates an unmanaged browser.
func serveDesktopBrowser(browser *browserController, socket string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	server := &http.Server{
		Handler:           http.HandlerFunc(browser.handleDesktopBrowser),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      40 * time.Second,
		IdleTimeout:       5 * time.Second,
		MaxHeaderBytes:    4096,
	}
	go func() { _ = server.Serve(listener) }()
	return func() { _ = server.Close(); _ = listener.Close() }, nil
}

func openDesktopBrowser(ctx context.Context, socket string, urls []string) error {
	if len(urls) == 0 {
		urls = []string{""}
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	for _, rawURL := range urls {
		if rawURL != "" {
			if err := validateBrowserURL(rawURL); err != nil {
				return err
			}
		}
		body, _ := json.Marshal(map[string]string{"url": rawURL})
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://desktop/open", strings.NewReader(string(body)))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("contact managed desktop browser: %w", err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			return fmt.Errorf("open managed desktop browser: %s", response.Status)
		}
	}
	return nil
}

func (b *browserController) handleDesktopBrowser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/open" {
		http.Error(w, "POST /open required", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		URL string `json:"url"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || decoder.Decode(new(interface{})) != io.EOF {
		http.Error(w, "invalid browser request", http.StatusBadRequest)
		return
	}
	if request.URL != "" && validateBrowserURL(request.URL) != nil {
		http.Error(w, "browser URL must use http or https", http.StatusBadRequest)
		return
	}
	if err := b.openFromDesktop(r.Context(), request.URL); err != nil {
		log.Printf("desktop browser launch failed: %v", err)
		http.Error(w, "managed desktop browser could not open", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (b *browserController) openFromDesktop(ctx context.Context, rawURL string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.browserCtx != nil {
		select {
		case <-chromedp.FromContext(b.browserCtx).Browser.LostConnection:
			b.closeLocked()
		default:
		}
	}
	if b.session.State != "open" {
		return b.openLocked(ctx, browserRequest{URL: rawURL})
	}
	actionCtx, cancel := context.WithTimeout(b.tabCtx, 30*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	if rawURL != "" {
		if err := chromedp.Do(actionCtx, chromedp.Navigate(rawURL)); err != nil {
			return err
		}
		b.session.URLOrigin = origin(rawURL)
	}
	_, err := chromedp.CallBrowser(actionCtx, target.ActivateTarget, target.ActivateTargetParams{TargetID: target.ID(b.session.ActivePageID)})
	b.session.UpdatedAt = time.Now().UTC()
	return err
}
