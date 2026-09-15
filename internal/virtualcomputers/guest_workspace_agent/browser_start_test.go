package main

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestManagedBrowserSurvivesStartupRequest(t *testing.T) {
	var executable string
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", `C:/Program Files/Google/Chrome/Application/chrome.exe`} {
		if path, err := exec.LookPath(name); err == nil {
			executable = path
			break
		}
	}
	if executable == "" {
		t.Skip("Chrome or Chromium is required")
	}
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(executable),
		chromedp.NoSandbox, chromedp.UserDataDir(t.TempDir()))
	allocatorCtx, closeAllocator := chromedp.NewExecAllocator(context.Background(), options...)
	defer closeAllocator()
	requestCtx, endRequest := context.WithTimeout(context.Background(), 20*time.Second)
	defer endRequest()
	browserCtx, closeBrowser, err := startManagedBrowser(requestCtx, allocatorCtx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer closeBrowser()
	process := chromedp.FromContext(browserCtx).Browser.Process()
	endRequest()

	// A later RPC/tab inherits the existing process after the opening RPC ends.
	tabCtx, closeTab := chromedp.NewContext(browserCtx)
	defer closeTab()
	actionCtx, endAction := context.WithTimeout(tabCtx, 10*time.Second)
	defer endAction()
	var value int
	if err := chromedp.Run(actionCtx, chromedp.Evaluate("21 * 2", &value)); err != nil || value != 42 {
		t.Fatalf("browser did not survive startup: value=%d err=%v", value, err)
	}
	if chromedp.FromContext(tabCtx).Browser.Process() != process {
		t.Fatal("later tab allocated a second Chromium process")
	}
	closeTab()
	closeBrowser()
	select {
	case <-chromedp.FromContext(browserCtx).Browser.LostConnection:
	default:
		t.Fatal("browser connection remained open after close")
	}
}

func TestManagedBrowserRejectsCancelledStartup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := startManagedBrowser(ctx, context.Background(), ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled startup: %v", err)
	}
}
