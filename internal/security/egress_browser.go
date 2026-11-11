package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const (
	egressMaxBrowsers     = 4
	egressDefaultLifetime = 2 * time.Minute
	egressCheckTimeout    = 5 * time.Second
	// egressSlotWaitCap bounds the wait for a free slot when the caller's context
	// has no deadline of its own.
	egressSlotWaitCap = 2 * time.Minute

	// Defaults per browser life. A heavy page with media and many third-party
	// hosts stays well inside them; hitting one ends the browser (ErrEgressLimit).
	egressDefaultMaxBytes       = 256 << 20
	egressDefaultMaxConnections = 2000
)

var (
	// ErrEgressProxyDied is the reason a browser ended when its egress proxy stopped.
	ErrEgressProxyDied = errors.New("egress proxy stopped; the page was cut off")
	// ErrEgressBypassed means the path check did not come back through the proxy.
	ErrEgressBypassed = errors.New("browser traffic does not go through the egress proxy")
	// ErrEgressLifetime is the reason a browser ended when its lifetime ran out.
	ErrEgressLifetime = errors.New("egress browser lifetime ended")
	// ErrEgressBusy means no browser slot became free in time.
	ErrEgressBusy = errors.New("too many egress browsers are running")
	// ErrEgressLimit is the reason a browser ended when the proxy hit its byte or
	// connection limit: what the page loaded after that is incomplete.
	ErrEgressLimit = errors.New("egress proxy limit reached; the page was cut off")
)

// egressSlots allows at most egressMaxBrowsers egress browsers at once.
var egressSlots = make(chan struct{}, egressMaxBrowsers)

// The two seams below let unit tests run the helper without Chrome.
var (
	// egressStart launches Chrome and connects to it. stop kills it.
	egressStart = startEgressChrome
	// egressProbe opens checkURL from a blank page and returns the value of the
	// X-AuraGo-Egress response header.
	egressProbe = probeEgressViaPage
)

// EgressBrowserOptions configures LaunchEgressBrowser.
type EgressBrowserOptions struct {
	// AllowedOrigin is the one origin (a loopback preview) the proxy may dial
	// without the SSRF check. Nil: public addresses only.
	AllowedOrigin *url.URL
	// Lifetime bounds the browser; zero means 2 minutes.
	Lifetime time.Duration
	// Bin is the browser binary; empty uses rod's lookup.
	Bin string
	// MaxBytes and MaxConnections bound the proxy over the browser's life; zero
	// selects 256 MiB and 2000 connections. Hitting either ends the browser with
	// ErrEgressLimit.
	MaxBytes       int64
	MaxConnections int
	// Sandbox true removes --no-sandbox even inside a container.
	Sandbox bool
}

// EgressBrowser is a headless Chrome whose whole network traffic goes through
// its own capture proxy.
type EgressBrowser struct {
	Browser *rod.Browser
	// Done is closed when the browser, the proxy or the lifetime ends.
	Done <-chan struct{}

	done      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	proxy     *CaptureProxy
	profile   string
	stop      func()
	closeOnce sync.Once

	mu      sync.Mutex
	err     error
	closing bool
}

// LaunchEgressBrowser starts a capture proxy and a fresh Chrome with a fresh
// profile behind it, and proves with a path check that the browser's traffic
// really uses the proxy. The caller must Close the result.
func LaunchEgressBrowser(ctx context.Context, opts EgressBrowserOptions) (*EgressBrowser, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	lifetime := opts.Lifetime
	if lifetime <= 0 {
		lifetime = egressDefaultLifetime
	}
	if err := acquireEgressSlot(ctx); err != nil {
		return nil, err
	}

	bctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	eb := &EgressBrowser{Done: done, done: done, ctx: bctx, cancel: cancel}
	// From here on every failure goes through shutdown, which releases the slot.
	fail := func(err error) (*EgressBrowser, error) {
		eb.shutdown()
		return nil, err
	}

	nonce, err := randomHex(16)
	if err != nil {
		return fail(fmt.Errorf("egress nonce: %w", err))
	}
	maxBytes, maxConns := opts.MaxBytes, opts.MaxConnections
	if maxBytes <= 0 {
		maxBytes = egressDefaultMaxBytes
	}
	if maxConns <= 0 {
		maxConns = egressDefaultMaxConnections
	}
	proxy, err := StartCaptureProxy(opts.AllowedOrigin, CaptureProxyLimits{
		MaxConcurrent: 16, MaxConnections: maxConns, MaxBytes: maxBytes,
	}, withEgressCheck(nonce))
	if err != nil {
		return fail(fmt.Errorf("egress proxy: %w", err))
	}
	eb.proxy = proxy

	profile, err := os.MkdirTemp("", "aurago-egress-*")
	if err != nil {
		return fail(fmt.Errorf("egress profile directory: %w", err))
	}
	eb.profile = profile

	browser, stop, err := egressStart(bctx, newEgressLauncher(bctx, proxy.Addr(), profile, opts))
	if err != nil {
		return fail(err)
	}
	eb.Browser, eb.stop = browser, stop

	if err := verifyEgressPath(bctx, browser, nonce); err != nil {
		select {
		case <-proxy.Done():
			err = ErrEgressProxyDied
		default:
		}
		return fail(err)
	}

	go eb.watch(lifetime)
	if observe := egressObserver(); observe != nil {
		observe(eb)
	}
	return eb, nil
}

// Close stops the browser and the proxy and removes the profile. It is safe to
// call more than once.
func (b *EgressBrowser) Close() error {
	b.shutdown()
	return nil
}

// Err says why the browser ended: nil after a plain Close, ErrEgressProxyDied,
// ErrEgressLimit or ErrEgressLifetime otherwise.
func (b *EgressBrowser) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

// Wrap adds the reason the browser ended, if there is one, to a failure the
// caller saw. A caller that gets a non-nil cause must return it and never
// partial content.
func (b *EgressBrowser) Wrap(err error) error {
	if err == nil {
		return nil
	}
	if cause := b.Err(); cause != nil && !errors.Is(err, cause) {
		return fmt.Errorf("%w: %v", cause, err)
	}
	return err
}

// ProxyDialed lists the destinations (host:port) the proxy connected to.
func (b *EgressBrowser) ProxyDialed() []string { return b.proxy.Dialed() }

// ProxyRefused lists the destinations (host:port) the proxy refused.
func (b *EgressBrowser) ProxyRefused() []string { return b.proxy.Refused() }

func (b *EgressBrowser) end(err error) {
	b.mu.Lock()
	if !b.closing && b.err == nil {
		b.err = err
	}
	b.mu.Unlock()
	b.shutdown()
}

func (b *EgressBrowser) watch(lifetime time.Duration) {
	timer := time.NewTimer(lifetime)
	defer timer.Stop()
	select {
	case <-b.proxy.Done():
		b.end(ErrEgressProxyDied)
	case <-b.proxy.Cut():
		b.end(ErrEgressLimit)
	case <-timer.C:
		b.end(ErrEgressLifetime)
	case <-b.ctx.Done():
		b.shutdown() // the caller's context ended
	case <-b.done:
	}
}

// shutdown is the one teardown. It runs once, whatever started it.
func (b *EgressBrowser) shutdown() {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closing = true
		b.mu.Unlock()
		if b.stop != nil {
			b.stop()
		}
		b.cancel()
		if b.proxy != nil {
			_ = b.proxy.Close()
		}
		if b.profile != "" {
			_ = os.RemoveAll(b.profile)
		}
		<-egressSlots
		close(b.done)
	})
}

func acquireEgressSlot(ctx context.Context) error {
	wait := time.NewTimer(egressSlotWaitCap)
	defer wait.Stop()
	select {
	case egressSlots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrEgressBusy, ctx.Err())
	case <-wait.C:
		return ErrEgressBusy
	}
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newEgressLauncher(ctx context.Context, proxyAddr, profile string, opts EgressBrowserOptions) *launcher.Launcher {
	l := launcher.New().Context(ctx).Headless(true).UserDataDir(profile).
		// Chrome 156 ignores the force- spelling for STUN, so set both.
		Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Set("webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Proxy("http://"+proxyAddr).
		// Chrome bypasses a proxy for loopback unless told otherwise.
		Set("proxy-bypass-list", "<-loopback>").
		// rod already sets disable-background-networking and disable-sync; these
		// stop Chrome's own update and default-browser traffic, and the features
		// list keeps rod's own two entries.
		Set("disable-component-update").
		Set("no-default-browser-check").
		Set("disable-features", "site-per-process", "TranslateUI", "OptimizationHints", "MediaRouter")
	if opts.Bin != "" {
		l = l.Bin(opts.Bin)
	}
	if opts.Sandbox {
		l = l.NoSandbox(false)
	}
	return l
}

func startEgressChrome(ctx context.Context, l *launcher.Launcher) (*rod.Browser, func(), error) {
	controlURL, err := l.Launch()
	if err != nil {
		return nil, nil, fmt.Errorf("launch browser: %w", err)
	}
	kill := func() { l.Kill(); l.Cleanup() }
	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		kill()
		return nil, nil, fmt.Errorf("connect browser: %w", err)
	}
	stop := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = browser.Context(closeCtx).Close()
		kill()
	}
	return browser, stop, nil
}

// verifyEgressPath loads the proxy's own check host from a blank page. The
// proxy answers it without dialling; a browser that reaches anything else, or
// nothing, did not go through the proxy.
func verifyEgressPath(ctx context.Context, b *rod.Browser, nonce string) error {
	checkCtx, cancel := context.WithTimeout(ctx, egressCheckTimeout)
	defer cancel()
	got, err := egressProbe(checkCtx, b, "http://"+captureEgressCheckHost+"/"+nonce)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrEgressBypassed, err)
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(nonce)) != 1 {
		return fmt.Errorf("%w: the check answered without the nonce", ErrEgressBypassed)
	}
	return nil
}

func probeEgressViaPage(ctx context.Context, b *rod.Browser, checkURL string) (string, error) {
	page, err := b.Context(ctx).Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return "", err
	}
	defer func() { _ = page.Close() }()
	res, err := page.Eval(`(u) => fetch(u, {cache: 'no-store'}).then((r) => r.headers.get('X-AuraGo-Egress') || '')`, checkURL)
	if err != nil {
		return "", err
	}
	return res.Value.Str(), nil
}

var (
	egressObserverMu sync.Mutex
	egressObserverFn func(*EgressBrowser)
)

func egressObserver() func(*EgressBrowser) {
	egressObserverMu.Lock()
	defer egressObserverMu.Unlock()
	return egressObserverFn
}

// SetEgressLaunchObserver makes fn see every EgressBrowser that
// LaunchEgressBrowser returns, so a test can read the proxy log of a browser
// that a tool launched internally. Tests only. The returned function restores
// the previous observer.
func SetEgressLaunchObserver(fn func(*EgressBrowser)) (restore func()) {
	egressObserverMu.Lock()
	previous := egressObserverFn
	egressObserverFn = fn
	egressObserverMu.Unlock()
	return func() {
		egressObserverMu.Lock()
		egressObserverFn = previous
		egressObserverMu.Unlock()
	}
}
