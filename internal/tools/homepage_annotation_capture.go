package tools

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

//go:embed homepage_annotation_map.js
var homepageAnnotationMapJS string

const (
	homepageAnnotationTimeout   = 30 * time.Second
	homepageAnnotationMaxHeight = 12000
	homepageAnnotationQuality   = 80
)

var (
	ErrHomepageAnnotationBusy       = errors.New("another page capture is running")
	ErrHomepageAnnotationBrowser    = errors.New("headless browser unavailable")
	ErrHomepageAnnotationSandbox    = errors.New("browser sandbox is blocked by the host")
	ErrHomepageAnnotationTimeout    = errors.New("page did not load in time")
	ErrHomepageAnnotationBlocked    = errors.New("page address is not allowed")
	ErrHomepageAnnotationNavigation = errors.New("page could not be loaded")
)

// homepageAnnotationSlot allows one page capture at a time.
var homepageAnnotationSlot = make(chan struct{}, 1)

// HomepageAnnotationCaptureOptions describes one snapshot. AllowedOrigin is set
// only for the local preview: the capture proxy dials that one host:port
// directly, although the SSRF rules would refuse its loopback address.
type HomepageAnnotationCaptureOptions struct {
	URL           *url.URL
	AllowedOrigin *url.URL
	Width         int
	Height        int
	BrowserBin    string // tests only; empty uses rod's browser lookup
}

type HomepageAnnotationViewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type HomepageAnnotationPage struct {
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Truncated bool `json:"truncated"`
}

type HomepageAnnotationElement struct {
	I        int               `json:"i"`
	Selector string            `json:"selector"`
	Tag      string            `json:"tag"`
	Role     string            `json:"role,omitempty"`
	Text     string            `json:"text,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
	Box      [4]float64        `json:"box"`
	Parent   int               `json:"parent"`
	Tab      bool              `json:"tab,omitempty"`
}

type HomepageAnnotationSnapshot struct {
	URL      string                      `json:"url"`
	Path     string                      `json:"path"`
	Viewport HomepageAnnotationViewport  `json:"viewport"`
	Page     HomepageAnnotationPage      `json:"page"`
	Image    string                      `json:"image"`
	Elements []HomepageAnnotationElement `json:"elements"`
}

type homepageAnnotationMap struct {
	Width    int                         `json:"width"`
	Height   int                         `json:"height"`
	Elements []HomepageAnnotationElement `json:"elements"`
}

// CaptureHomepageAnnotationSnapshot renders the page in a fresh headless
// browser and returns a JPEG of the page plus its visible element map.
func CaptureHomepageAnnotationSnapshot(ctx context.Context, opts HomepageAnnotationCaptureOptions) (*HomepageAnnotationSnapshot, error) {
	if opts.URL == nil || opts.Width <= 0 || opts.Height <= 0 {
		return nil, fmt.Errorf("capture options are incomplete")
	}
	if opts.AllowedOrigin == nil {
		if err := security.ValidateSSRF(opts.URL.String()); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrHomepageAnnotationBlocked, err)
		}
	}
	select {
	case homepageAnnotationSlot <- struct{}{}:
	default:
		return nil, ErrHomepageAnnotationBusy
	}
	defer func() { <-homepageAnnotationSlot }()

	ctx, cancel := context.WithTimeout(ctx, homepageAnnotationTimeout)
	defer cancel()

	// Every byte the browser sends goes through this proxy, which refuses any
	// destination but the allowed origin and public addresses. A CDP request
	// hook sees only the page target; service workers, WebSockets and
	// prerenders would slip past it.
	proxy, err := security.StartCaptureProxy(opts.AllowedOrigin, security.CaptureProxyLimits{
		MaxConcurrent: 16, MaxConnections: 600, MaxBytes: 64 << 20,
	})
	if err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	}
	defer func() { _ = proxy.Close() }()

	l := launcher.New().Context(ctx).Headless(true).
		// Chrome 156 ignores the force- spelling for STUN (verified: a page
		// still sent UDP binding requests to loopback), so set both.
		Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Set("webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Proxy(proxy.Addr()).
		// Chrome bypasses a proxy for loopback unless told otherwise; the
		// preview itself lives on loopback.
		Set("proxy-bypass-list", "<-loopback>")
	if opts.BrowserBin != "" {
		l = l.Bin(opts.BrowserBin)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return nil, homepageAnnotationLaunchError(ctx, err)
	}
	defer func() { l.Kill(); l.Cleanup() }()

	browser := rod.New().Context(ctx).ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = browser.Context(closeCtx).Close()
	}()

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	}
	// Mobile stays off: with it Chrome lays a page without a width=device-width
	// meta out at 980 px, while the studio preview is a plain iframe of exactly
	// opts.Width and the screenshot is clipped to that width.
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width: opts.Width, Height: opts.Height, DeviceScaleFactor: 1, Mobile: false,
	}); err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	}
	if err := page.Navigate(opts.URL.String()); err != nil {
		return nil, homepageAnnotationNavigateError(ctx, err)
	}
	if err := page.WaitLoad(); err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationNavigation, err)
	}
	// Late images and web fonts get a short, bounded settle time.
	_ = page.Timeout(4 * time.Second).WaitIdle(4 * time.Second)

	mapped, err := evaluateHomepageAnnotationMap(page)
	if err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationNavigation, err)
	}
	height := mapped.Height
	if height <= 0 {
		height = opts.Height
	}
	truncated := height > homepageAnnotationMaxHeight
	if truncated {
		height = homepageAnnotationMaxHeight
	}
	quality := homepageAnnotationQuality
	// Clip + CaptureBeyondViewport keeps the viewport size, so 100vh sections
	// render as on screen (rod's fullPage mode would resize the viewport).
	shot, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format:                proto.PageCaptureScreenshotFormatJpeg,
		Quality:               &quality,
		Clip:                  &proto.PageViewport{X: 0, Y: 0, Width: float64(opts.Width), Height: float64(height), Scale: 1},
		CaptureBeyondViewport: true,
	})
	if err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationNavigation, err)
	}
	return &HomepageAnnotationSnapshot{
		URL:      opts.URL.String(),
		Viewport: HomepageAnnotationViewport{Width: opts.Width, Height: opts.Height},
		Page:     HomepageAnnotationPage{Width: opts.Width, Height: height, Truncated: truncated},
		Image:    "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(shot),
		Elements: clipHomepageAnnotationElements(mapped.Elements, float64(height)),
	}, nil
}

func evaluateHomepageAnnotationMap(page *rod.Page) (homepageAnnotationMap, error) {
	var mapped homepageAnnotationMap
	res, err := page.Eval(homepageAnnotationMapJS)
	if err != nil {
		return mapped, err
	}
	if err := json.Unmarshal([]byte(res.Value.JSON("", "")), &mapped); err != nil {
		return mapped, fmt.Errorf("decode element map: %w", err)
	}
	return mapped, nil
}

// homepageAnnotationNavigateError maps a failed navigation. The capture proxy
// answers a refused destination with a failed tunnel (HTTPS) or a 403 page
// (plain HTTP); a slow or silent server surfaces as a Chrome timeout.
func homepageAnnotationNavigateError(ctx context.Context, err error) error {
	text := err.Error()
	switch {
	case strings.Contains(text, "TIMED_OUT"):
		return fmt.Errorf("%w: %v", ErrHomepageAnnotationTimeout, err)
	case strings.Contains(text, "BLOCKED_BY_CLIENT"), strings.Contains(text, "TUNNEL_CONNECTION_FAILED"):
		return fmt.Errorf("%w: %v", ErrHomepageAnnotationBlocked, err)
	}
	return homepageAnnotationError(ctx, ErrHomepageAnnotationNavigation, err)
}

func homepageAnnotationError(ctx context.Context, kind, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrHomepageAnnotationTimeout, err)
	}
	return fmt.Errorf("%w: %v", kind, err)
}

// homepageAnnotationLaunchError names a blocked Chromium sandbox: Ubuntu 23.10+
// refuses user namespaces to programs without an AppArmor profile and Chromium
// then dies with "No usable sandbox". install.sh and update.sh add that profile.
func homepageAnnotationLaunchError(ctx context.Context, err error) error {
	mapped := homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	if errors.Is(mapped, ErrHomepageAnnotationTimeout) || !strings.Contains(err.Error(), "No usable sandbox") {
		return mapped
	}
	return fmt.Errorf("%w: %w: %v", ErrHomepageAnnotationSandbox, ErrHomepageAnnotationBrowser, err)
}

// clipHomepageAnnotationElements drops elements outside the captured height,
// clips boxes at the bottom edge and rewrites indexes and parent links.
func clipHomepageAnnotationElements(elements []HomepageAnnotationElement, height float64) []HomepageAnnotationElement {
	kept := make([]HomepageAnnotationElement, 0, len(elements))
	index := make(map[int]int, len(elements))
	for _, el := range elements {
		if el.Box[1] >= height || el.Box[1]+el.Box[3] <= 0 || el.Box[2] <= 0 || el.Box[3] <= 0 {
			continue
		}
		if el.Box[1]+el.Box[3] > height {
			el.Box[3] = height - el.Box[1]
		}
		next := len(kept)
		index[el.I] = next
		if parent, ok := index[el.Parent]; ok {
			el.Parent = parent
		} else {
			el.Parent = -1
		}
		el.I = next
		kept = append(kept, el)
	}
	return kept
}
