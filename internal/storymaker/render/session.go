package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// sessionOptions describes one render browser session.
type sessionOptions struct {
	Bin     string
	Profile string
	// ProxyAddr is the strict capture proxy; empty runs without a proxy (the egress test of the CDP
	// guard on its own).
	ProxyAddr string
	// Base is the render base the egress guard lets through.
	Base          string
	Width, Height int
	// NoGuard skips the CDP egress guard. Only the tests use it, to show that the guard (and the proxy)
	// are what stop a leak and to measure what they cost.
	NoGuard bool
	// GuardFull forces the catch-all request interception instead of the hybrid guard (tests, and
	// browsers without block patterns).
	GuardFull bool
}

// browserPage is the part of a headless browser tab the render job uses; tests replace it with a
// stand-in that talks to the listener over HTTP.
type browserPage interface {
	// Navigate opens url; it returns once the navigation committed.
	Navigate(ctx context.Context, url string) error
	// Status returns window.__smStatus as JSON ("" when unavailable).
	Status(ctx context.Context) string
	// WaitReady waits for the screenshot transport's ready phase.
	WaitReady(ctx context.Context) error
	// Capture renders frame n through window.__smRenderFrame and returns a PNG screenshot.
	Capture(ctx context.Context, n int) ([]byte, error)
	// Crashed is closed when the tab crashed.
	Crashed() <-chan struct{}
	// Close stops the browser and removes its profile.
	Close()
}

// pageOpener starts the browser; tests replace it.
type pageOpener func(ctx context.Context, o sessionOptions) (browserPage, error)

// session is the production browserPage: a launched Chrome with the egress guard on its tab.
type session struct {
	l       *launcher.Launcher
	browser *rod.Browser
	page    *rod.Page
	guard   *egressGuard
	crashed chan struct{}
	opts    sessionOptions
}

func openSession(ctx context.Context, o sessionOptions) (browserPage, error) {
	s, err := openRodSession(ctx, o)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func openRodSession(ctx context.Context, o sessionOptions) (*session, error) {
	if err := os.MkdirAll(o.Profile, 0o700); err != nil {
		return nil, err
	}
	l := newLauncher(ctx, launchOptions{Bin: o.Bin, UserDataDir: o.Profile, ProxyAddr: o.ProxyAddr, WindowSize: [2]int{o.Width, o.Height}})
	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch the browser: %w", err)
	}
	s := &session{l: l, opts: o, guard: newEgressGuard(o.Base), crashed: make(chan struct{})}
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		s.closeProcess()
		return nil, fmt.Errorf("connect to the browser: %w", err)
	}
	s.browser = browser
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("open a tab: %w", err)
	}
	s.page = page
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{Width: o.Width, Height: o.Height, DeviceScaleFactor: 1, Mobile: false}); err != nil {
		s.Close()
		return nil, fmt.Errorf("size the tab: %w", err)
	}
	s.guard.full = o.GuardFull
	if !o.NoGuard {
		if err := s.guard.install(ctx, browser, page); err != nil {
			s.Close()
			return nil, fmt.Errorf("install the egress guard: %w", err)
		}
	}
	go page.EachEvent(func(*proto.InspectorTargetCrashed) bool {
		select {
		case <-s.crashed:
		default:
			close(s.crashed)
		}
		return true
	})()
	return s, nil
}

func (s *session) Navigate(ctx context.Context, url string) error {
	return s.page.Context(ctx).Navigate(url)
}

func (s *session) Status(ctx context.Context) string {
	res, err := s.page.Timeout(3 * time.Second).Eval(`() => JSON.stringify(window.__smStatus || null)`)
	if err != nil {
		return ""
	}
	return res.Value.Str()
}

func (s *session) WaitReady(ctx context.Context) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		var status struct {
			Phase   string `json:"phase"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if raw := s.Status(ctx); raw != "" && raw != "null" {
			_ = json.Unmarshal([]byte(raw), &status)
		}
		switch status.Phase {
		case "ready":
			return nil
		case "error", "refused", "stopped":
			return fmt.Errorf("the page stopped (%s): %s", status.Error, status.Message)
		}
		if time.Now().After(deadline) {
			return errors.New("the page never became ready for screenshots")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(40 * time.Millisecond):
		}
	}
}

func (s *session) Capture(ctx context.Context, n int) ([]byte, error) {
	p := s.page.Context(ctx).Timeout(30 * time.Second)
	if _, err := p.Eval(`(n) => window.__smRenderFrame(n)`, n); err != nil {
		return nil, err
	}
	return p.Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
		Clip:   &proto.PageViewport{X: 0, Y: 0, Width: float64(s.opts.Width), Height: float64(s.opts.Height), Scale: 1},
	})
}

func (s *session) Crashed() <-chan struct{} { return s.crashed }

// Guard exposes the egress guard to the tests.
func (s *session) Guard() *egressGuard { return s.guard }

func (s *session) Close() {
	if s.browser != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = s.browser.Context(closeCtx).Close()
		cancel()
	}
	s.closeProcess()
}

func (s *session) closeProcess() {
	s.l.Kill()
	s.l.Cleanup()
}
