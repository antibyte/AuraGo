package main

import (
	"context"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
)

// startManagedBrowser allocates Chrome on its session context, never on an RPC
// deadline. Cancelling the first Run context would otherwise kill the process.
func startManagedBrowser(requestCtx, allocatorCtx context.Context, downloadDir string) (context.Context, context.CancelFunc, error) {
	if err := requestCtx.Err(); err != nil {
		return nil, nil, err
	}
	browserCtx, closeBrowser := chromedp.NewContext(allocatorCtx)
	startupCtx, cancelStartup := context.WithTimeout(requestCtx, 30*time.Second)
	stopCancellation := context.AfterFunc(startupCtx, closeBrowser)
	err := chromedp.Do(browserCtx, chromedp.Func(func(ctx context.Context, _ *chromedp.Target) error {
		eventsEnabled := true
		_, err := chromedp.CallBrowser(ctx, browser.SetDownloadBehavior, browser.SetDownloadBehaviorParams{
			Behavior: browser.SetDownloadBehaviorBehaviorAllow, DownloadPath: downloadDir, EventsEnabled: &eventsEnabled,
		})
		return err
	}))
	stopped := stopCancellation()
	startupErr := startupCtx.Err()
	cancelStartup()
	if err != nil || !stopped {
		closeBrowser()
		if startupErr != nil {
			err = startupErr
		}
		return nil, nil, err
	}
	return browserCtx, sync.OnceFunc(func() {
		defer closeBrowser() // Force cleanup if the graceful close fails or times out.
		// Cancel is not idempotent after another caller or a closed browser has
		// already ended the session. A desktop window may disappear at any time.
		select {
		case <-browserCtx.Done():
			return
		case <-chromedp.FromContext(browserCtx).Browser.LostConnection:
			return
		default:
		}
		// Let Chrome stop its child processes and profile writers before the
		// caller removes the profile. Killing only the parent can race cleanup.
		shutdownCtx, cancelShutdown := context.WithTimeout(browserCtx, 5*time.Second)
		defer cancelShutdown()
		_ = chromedp.Cancel(shutdownCtx)
	}), nil
}
