package render

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// egressGuard is the CDP half of the render page's egress control (spec step 2, control 3). Together
// with the page's CSP and the strict capture proxy (controls 1 and 2) it forms three independent
// layers; each one alone keeps the page on the render base.
//
// Measured on Chrome 156 (plan task B10, notes "Egress"):
//   - Fetch.enable is not available on a dedicated worker's own CDP session ("'Fetch.enable' wasn't
//     found"), but the page session's interception also pauses the requests dedicated workers make
//     (fetch, XMLHttpRequest, importScripts).
//   - Pausing a request that carries a frame costs about 35 ms (the browser hands the whole body to
//     the DevTools agent): the catch-all interception halves the rendering rate (20 fps to 10 fps).
//     Enabling Fetch without pausing anything costs nothing.
//   - Network.setBlockedURLs with urlPatterns blocks requests natively, on the page session and on
//     worker sessions, without pausing them. It does not cover top-level navigations.
//
// The default (hybrid) mode therefore pauses only navigations (Fetch, resource type Document: a few
// requests per job) and blocks everything else natively on the page and on every worker target that
// auto-attaches (held until its block list is installed). When the browser lacks block patterns
// (Chrome before 126) the guard falls back to the catch-all interception, which is slower and covers
// the same requests. WebSocket handshakes are outside both mechanisms; the CSP and the proxy stop
// them. A popup opened by window.open is another target; the sandbox keeps it from opening (no
// allow-popups) and the guard closes any extra page target as a second barrier.
type egressGuard struct {
	base string // http://127.0.0.1:<port>/<prefix>, no trailing slash
	// full forces the catch-all interception (tests compare it).
	full bool
	// mode records the mechanism in use: "hybrid" or "fetch".
	mode atomic.Value

	blocked      atomic.Int64
	seen         atomic.Int64 // requests the guard paused and decided
	extra        atomic.Int64 // extra page targets that were closed
	workers      atomic.Int64 // worker targets guarded
	workerFailed atomic.Int64 // worker targets that could not be guarded and were closed
	mu           sync.Mutex
	hosts        []string // scheme://host of the first blocked requests, never the render base
}

func newEgressGuard(base string) *egressGuard { return &egressGuard{base: base} }

// allowed decides one paused request. Navigations (documents and frames) must lie below the base;
// other requests may also be blob: or data: URLs, which never leave the browser.
func (g *egressGuard) allowed(rawURL string, resource proto.NetworkResourceType) bool {
	if strings.HasPrefix(rawURL, g.base+"/") {
		return true
	}
	if resource == proto.NetworkResourceTypeDocument {
		return false
	}
	return strings.HasPrefix(rawURL, "blob:") || strings.HasPrefix(rawURL, "data:")
}

// Mode returns "hybrid" or "fetch" once the guard is installed.
func (g *egressGuard) Mode() string {
	m, _ := g.mode.Load().(string)
	return m
}

// Blocked is the number of paused requests the guard failed.
func (g *egressGuard) Blocked() int64 { return g.blocked.Load() }

// Seen is the number of requests the guard paused.
func (g *egressGuard) Seen() int64 { return g.seen.Load() }

// BlockedHosts returns the scheme://host of the first blocked requests.
func (g *egressGuard) BlockedHosts() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.hosts...)
}

func (g *egressGuard) noteBlocked(rawURL string) {
	g.blocked.Add(1)
	host := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		host = u.Scheme + "://" + u.Host
	} else if len(host) > 40 {
		host = host[:40]
	}
	g.mu.Lock()
	if len(g.hosts) < 16 {
		g.hosts = append(g.hosts, host)
	}
	g.mu.Unlock()
}

// pausedLite reads only what the decision needs from Fetch.requestPaused. The full event carries
// the request body (a frame is up to several hundred kilobytes), which the guard never looks at.
type pausedLite struct {
	RequestID    proto.FetchRequestID      `json:"requestId"`
	ResourceType proto.NetworkResourceType `json:"resourceType"`
	Request      struct {
		URL string `json:"url"`
	} `json:"request"`
}

func (pausedLite) ProtoEvent() string { return "Fetch.requestPaused" }

// sessionClient sends CDP commands on one target session of the browser connection.
type sessionClient struct {
	b   *rod.Browser
	sid proto.TargetSessionID
	ctx context.Context
}

func (c sessionClient) Call(ctx context.Context, _ string, method string, params interface{}) ([]byte, error) {
	return c.b.Call(ctx, string(c.sid), method, params)
}
func (c sessionClient) GetSessionID() proto.TargetSessionID { return c.sid }
func (c sessionClient) GetContext() context.Context         { return c.ctx }

var (
	interceptCatchAll  = []*proto.FetchRequestPattern{{URLPattern: "*", RequestStage: proto.FetchRequestStageRequest}}
	interceptDocuments = []*proto.FetchRequestPattern{{URLPattern: "*", ResourceType: proto.NetworkResourceTypeDocument, RequestStage: proto.FetchRequestStageRequest}}
)

// blockPatterns is the native block list: the render base is allowed, every other http(s) and ws(s)
// URL is blocked. blob: and data: URLs match none of the patterns.
func (g *egressGuard) blockPatterns() []map[string]any {
	return []map[string]any{
		{"urlPattern": g.base + "/*", "block": false},
		{"urlPattern": "http://*:*/*", "block": true},
		{"urlPattern": "https://*:*/*", "block": true},
		{"urlPattern": "ws://*:*/*", "block": true},
		{"urlPattern": "wss://*:*/*", "block": true},
	}
}

// blockOutside installs the native block list on one target session.
func (g *egressGuard) blockOutside(c sessionClient) error {
	if err := (proto.NetworkEnable{}).Call(c); err != nil {
		return err
	}
	_, err := c.Call(c.ctx, "", "Network.setBlockedURLs", map[string]any{"urlPatterns": g.blockPatterns()})
	return err
}

// install guards the page and starts the decision loop. It must run before the page navigates.
func (g *egressGuard) install(ctx context.Context, browser *rod.Browser, page *rod.Page) error {
	b := browser.Context(ctx)
	events := b.Event()
	hybrid := false
	if !g.full {
		hybrid = g.blockOutside(sessionClient{b: b, sid: page.SessionID, ctx: ctx}) == nil
	}
	patterns, mode := interceptCatchAll, "fetch"
	if hybrid {
		patterns, mode = interceptDocuments, "hybrid"
	}
	g.mode.Store(mode)
	if err := (proto.FetchEnable{Patterns: patterns}).Call(page); err != nil {
		return err
	}
	if hybrid {
		if err := (proto.TargetSetAutoAttach{AutoAttach: true, WaitForDebuggerOnStart: true, Flatten: true}).Call(page); err != nil {
			return err
		}
	}
	if err := (proto.TargetSetDiscoverTargets{Discover: true}).Call(b); err != nil {
		return err
	}
	ours := page.TargetID
	paused := (pausedLite{}).ProtoEvent()
	created := (proto.TargetTargetCreated{}).ProtoEvent()
	attached := (proto.TargetAttachedToTarget{}).ProtoEvent()
	go func() {
		for msg := range events {
			switch msg.Method {
			case paused:
				var ev pausedLite
				if !msg.Load(&ev) {
					continue
				}
				client := sessionClient{b: b, sid: msg.SessionID, ctx: ctx}
				g.seen.Add(1)
				// Replies run off the loop so a slow command cannot hold the next event back.
				go func() {
					if g.allowed(ev.Request.URL, ev.ResourceType) {
						_ = (proto.FetchContinueRequest{RequestID: ev.RequestID}).Call(client)
						return
					}
					g.noteBlocked(ev.Request.URL)
					_ = (proto.FetchFailRequest{RequestID: ev.RequestID, ErrorReason: proto.NetworkErrorReasonBlockedByClient}).Call(client)
				}()
			case created:
				var ev proto.TargetTargetCreated
				if !msg.Load(&ev) || ev.TargetInfo == nil {
					continue
				}
				if ev.TargetInfo.Type == "page" && ev.TargetInfo.TargetID != ours {
					g.extra.Add(1)
					go func(id proto.TargetTargetID) { _, _ = (proto.TargetCloseTarget{TargetID: id}).Call(b) }(ev.TargetInfo.TargetID)
				}
			case attached:
				var ev proto.TargetAttachedToTarget
				if !msg.Load(&ev) {
					continue
				}
				go g.guardWorker(ctx, b, ev)
			}
		}
	}()
	return nil
}

// guardWorker installs the block list on a freshly attached worker (and on workers it starts) and
// lets it run. A target that cannot be guarded is closed.
func (g *egressGuard) guardWorker(ctx context.Context, b *rod.Browser, ev proto.TargetAttachedToTarget) {
	client := sessionClient{b: b, sid: ev.SessionID, ctx: ctx}
	err := g.blockOutside(client)
	if err == nil {
		err = (proto.TargetSetAutoAttach{AutoAttach: true, WaitForDebuggerOnStart: true, Flatten: true}).Call(client)
	}
	if err != nil {
		g.workerFailed.Add(1)
		if ev.TargetInfo != nil {
			_, _ = (proto.TargetCloseTarget{TargetID: ev.TargetInfo.TargetID}).Call(b)
		}
		return
	}
	g.workers.Add(1)
	if ev.WaitingForDebugger {
		_ = (proto.RuntimeRunIfWaitingForDebugger{}).Call(client)
	}
}
