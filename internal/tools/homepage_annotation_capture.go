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
	ErrHomepageAnnotationTimeout    = errors.New("page did not load in time")
	ErrHomepageAnnotationBlocked    = errors.New("page address is not allowed")
	ErrHomepageAnnotationNavigation = errors.New("page could not be loaded")
)

// homepageAnnotationSlot allows one page capture at a time.
var homepageAnnotationSlot = make(chan struct{}, 1)

// HomepageAnnotationCaptureOptions describes one snapshot. AllowedOrigin is set
// only for the local preview, whose loopback address the SSRF guard blocks.
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

	l := launcher.New().Context(ctx).Headless(true).Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp")
	if opts.BrowserBin != "" {
		l = l.Bin(opts.BrowserBin)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
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
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width: opts.Width, Height: opts.Height, DeviceScaleFactor: 1, Mobile: opts.Width <= 480,
	}); err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	}
	if err := security.GuardRodPageWithOrigin(page, opts.AllowedOrigin); err != nil {
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationBrowser, err)
	}
	if err := page.Navigate(opts.URL.String()); err != nil {
		if strings.Contains(err.Error(), "BLOCKED_BY_CLIENT") {
			return nil, fmt.Errorf("%w: %v", ErrHomepageAnnotationBlocked, err)
		}
		return nil, homepageAnnotationError(ctx, ErrHomepageAnnotationNavigation, err)
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

func homepageAnnotationError(ctx context.Context, kind, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrHomepageAnnotationTimeout, err)
	}
	return fmt.Errorf("%w: %v", kind, err)
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
