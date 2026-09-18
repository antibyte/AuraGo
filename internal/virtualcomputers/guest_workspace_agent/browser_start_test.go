package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestManagedBrowserSurvivesStartupRequest(t *testing.T) {
	var executable string
	// Hosted runners provide CHROME_BIN for their supported stable browser;
	// their separate Chromium snapshot can have different startup requirements.
	for _, name := range []string{os.Getenv("CHROME_BIN"), "google-chrome", "google-chrome-stable", "chromium", "chromium-browser", `C:/Program Files/Google/Chrome/Application/chrome.exe`} {
		if path, err := exec.LookPath(name); err == nil {
			executable = path
			break
		}
	}
	if executable == "" {
		t.Skip("Chrome or Chromium is required")
	}
	t.Logf("browser executable: %s", executable)
	var output bytes.Buffer
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(executable),
		chromedp.NoSandbox, chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.UserDataDir(t.TempDir()), chromedp.CombinedOutput(&output))
	allocatorCtx, closeAllocator := chromedp.NewExecAllocator(context.Background(), options...)
	defer closeAllocator()
	requestCtx, endRequest := context.WithTimeout(context.Background(), 20*time.Second)
	defer endRequest()
	browserCtx, closeBrowser, err := startManagedBrowser(requestCtx, allocatorCtx, t.TempDir())
	if err != nil {
		closeAllocator()
		t.Fatalf("start %s: %v\n%s", executable, err, output.String())
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
