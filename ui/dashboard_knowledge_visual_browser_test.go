package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// TestDashboardKnowledgeGraphVisualBrowserSmoke renders the real dashboard
// knowledge-graph visual (WebGL 3D constellation and 2D canvas fallback) with
// mock graph data and verifies both renderers paint without errors, survive a
// view-mode switch, honor reduced motion and react to theme changes.
func TestDashboardKnowledgeGraphVisualBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}

	widgets := normalizeAssetText(mustReadUIFile(t, "js/dashboard/widgets-knowledge.js"))

	html := `<!doctype html><html data-theme="dark"><head><meta charset="utf-8">
<style>
  html { background: #0b1220; }
  #knowledge-graph-visual { width: 900px; height: 520px; }
</style>
</head><body>
<div id="knowledge-graph-mode"></div>
<div id="knowledge-graph-caption"></div>
<button type="button" id="knowledge-graph-reset" class="is-hidden"></button>
<div id="knowledge-graph-legend"></div>
<div id="knowledge-graph-view-toggle" role="group">
  <button type="button" data-kg-view="2d">2D</button>
  <button type="button" data-kg-view="3d">3D</button>
</div>
<div id="knowledge-graph-visual"></div>
<div id="kgDetailOverlay"><div id="kgDetailBody"></div></div>
<script>
window.__errors = [];
addEventListener('error', e => __errors.push(String((e && e.message) || e)));
window.t = (key) => key;
window.cv = () => '';
window.API = { get: async () => ({}) };
window.truncate = (s, n) => { s = String(s); return s.length > n ? s.slice(0, n - 1) + '…' : s; };
window.esc = (s) => String(s);
window.escapeHtml = (s) => String(s);
window.escapeJsString = (s) => String(s);
window.showToast = () => {};
window.showAlert = async () => {};
window.showConfirm = async () => true;
window.loadTabKnowledge = async () => {};
window.KnowledgeGraphState = {
  nodes: [], edges: [], importantNodes: [], importantEdges: [], stats: null,
  focusNodeId: '', focusPayload: null, editingNodeId: '', editingEdgeKey: '',
  showAll: false, filterType: '', filterSource: '', modalKind: '', modalNodeId: '', modalTriggerEl: null
};
</script>
<script src="/js/vendor/three.min.js"></script>
<script src="/js/vendor/force-graph.min.js"></script>
<script src="/js/vendor/3d-force-graph.min.js"></script>
<script src="/widgets-knowledge.js"></script>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(html))
		case "/widgets-knowledge.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(widgets))
		default:
			http.FileServer(http.FS(Content)).ServeHTTP(w, r)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	url, err := l.Launch()
	if err != nil {
		t.Skipf("headless browser launch failed: %v", err)
	}
	defer func() { l.Kill(); l.Cleanup() }()
	b := rod.New().Context(ctx).ControlURL(url).MustConnect()
	defer b.Close()
	p := b.MustPage(server.URL)
	defer p.Close()
	waitCtx, waitCancel := context.WithTimeout(ctx, 20*time.Second)
	defer waitCancel()
	ready := false
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		if p.Context(waitCtx).MustEval(`() => typeof renderKnowledgeGraphVisual === 'function' && typeof ForceGraph3D === 'function' && typeof ForceGraph === 'function'`).Bool() {
			ready = true
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !ready {
		t.Logf("typeof renderKnowledgeGraphVisual=%v ForceGraph=%v ForceGraph3D=%v THREE=%v",
			p.MustEval(`() => typeof renderKnowledgeGraphVisual`).Str(),
			p.MustEval(`() => typeof ForceGraph`).Str(),
			p.MustEval(`() => typeof ForceGraph3D`).Str(),
			p.MustEval(`() => typeof THREE`).Str())
		t.Logf("page errors: %s", p.MustEval(`() => window.__errors.join(' | ')`).Str())
		t.Fatal("knowledge graph visual scripts did not become ready")
	}

	seed := `() => {
    // Cold start in 2D, like a returning user with a stored 2D preference.
    localStorage.setItem('aurago.dashboard.kgview.v1', '2d');
    const types = ['device', 'service', 'person', 'software', 'concept', 'network'];
    KnowledgeGraphState.nodes = Array.from({ length: 12 }, (_, i) => ({
        id: 'n' + i,
        label: 'Node ' + i,
        importance_score: 100 - i * 7,
        properties: { type: types[i % types.length], source: 'smoke' },
    }));
    KnowledgeGraphState.edges = [];
    for (let i = 1; i < 12; i++) {
        KnowledgeGraphState.edges.push({ source: 'n' + (i % 4), target: 'n' + i, relation: 'link' + i });
    }
    renderKnowledgeGraphVisual();
    return true;
}`
	if !p.MustEval(seed).Bool() {
		t.Fatal("seeding the knowledge graph visual failed")
	}

	litPixels := `() => {
    const c = document.getElementById('knowledge-graph-visual').querySelector('canvas');
    const g = c.getContext('2d');
    const d = g.getImageData(0, 0, c.width, c.height).data;
    let lit = 0;
    for (let i = 3; i < d.length; i += 4) if (d[i] > 0) lit++;
    return lit;
}`

	// Cold-started 2D must paint immediately (regression: undefined node positions
	// during the first paint used to kill the render loop and leave the canvas blank).
	p.Timeout(15 * time.Second).MustWait(`() => {
    const wrap = document.getElementById('knowledge-graph-visual');
    return !!(wrap && wrap._kgRenderer === '2d' && wrap._forceGraph && wrap.querySelector('canvas'));
}`)
	time.Sleep(1500 * time.Millisecond)
	if lit := p.MustEval(litPixels).Int(); lit < 500 {
		t.Fatalf("cold-started 2D canvas painted only %d lit pixels", lit)
	}
	if errors := p.MustEval(`() => window.__errors.join(' | ')`).Str(); errors != "" {
		t.Fatalf("cold-started 2D view raised page errors: %s", errors)
	}

	// 3D view: WebGL constellation with starfield and glow nodes.
	if !p.MustEval(`() => { setKnowledgeGraphViewMode('3d'); return true; }`).Bool() {
		t.Fatal("switching to 3D failed")
	}
	p.Timeout(20 * time.Second).MustWait(`() => {
    const wrap = document.getElementById('knowledge-graph-visual');
    return !!(wrap && wrap._kgRenderer === '3d' && wrap._forceGraph3d && wrap.querySelector('canvas'));
}`)
	time.Sleep(3 * time.Second)
	if calls := p.MustEval(`() => document.getElementById('knowledge-graph-visual')._forceGraph3d.renderer().info.render.calls`).Int(); calls < 1 {
		t.Fatalf("3D renderer produced no draw calls: %d", calls)
	}
	if !p.MustEval(`() => !!document.getElementById('knowledge-graph-visual')._kgStars3d`).Bool() {
		t.Fatal("3D starfield backdrop missing")
	}
	if errors := p.MustEval(`() => window.__errors.join(' | ')`).Str(); errors != "" {
		t.Fatalf("3D view raised page errors: %s", errors)
	}

	// Switching back to 2D disposes the WebGL instance and paints the canvas view.
	if !p.MustEval(`() => { setKnowledgeGraphViewMode('2d'); return true; }`).Bool() {
		t.Fatal("switching to 2D failed")
	}
	p.Timeout(15 * time.Second).MustWait(`() => {
    const wrap = document.getElementById('knowledge-graph-visual');
    return !!(wrap && wrap._kgRenderer === '2d' && wrap._forceGraph && !wrap._forceGraph3d && wrap.querySelector('canvas'));
}`)
	time.Sleep(1500 * time.Millisecond)
	if lit := p.MustEval(litPixels).Int(); lit < 500 {
		t.Fatalf("2D canvas painted only %d lit pixels", lit)
	}

	// Reduced motion + theme change must re-render both modes without errors.
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
		t.Fatal(err)
	}
	if !p.MustEval(`() => { setKnowledgeGraphViewMode('3d'); return true; }`).Bool() {
		t.Fatal("switching back to 3D failed")
	}
	p.Timeout(15 * time.Second).MustWait(`() => {
    const wrap = document.getElementById('knowledge-graph-visual');
    return !!(wrap && wrap._kgRenderer === '3d' && wrap._forceGraph3d);
}`)
	time.Sleep(1200 * time.Millisecond)
	if autoRotate := p.MustEval(`() => document.getElementById('knowledge-graph-visual')._forceGraph3d.controls().autoRotate`).Bool(); autoRotate {
		t.Fatal("reduced motion must disable auto-rotation")
	}
	if !p.MustEval(`() => {
    document.documentElement.setAttribute('data-theme', 'light');
    dispatchEvent(new CustomEvent('aurago:themechange'));
    return true;
}`).Bool() {
		t.Fatal("theme change dispatch failed")
	}
	p.Timeout(15 * time.Second).MustWait(`() => {
    const wrap = document.getElementById('knowledge-graph-visual');
    return !!(wrap && wrap._kgRenderer === '3d' && wrap._forceGraph3d);
}`)
	time.Sleep(800 * time.Millisecond)
	if errors := p.MustEval(`() => window.__errors.join(' | ')`).Str(); errors != "" {
		t.Fatalf("knowledge graph visual raised page errors: %s", errors)
	}

	// The caption must not leak the unavailable note while 3D actually works.
	if caption := p.MustEval(`() => document.getElementById('knowledge-graph-caption').textContent`).Str(); strings.Contains(caption, "knowledge_visual_3d_unavailable") {
		t.Fatalf("unexpected 3D-unavailable note in caption: %q", caption)
	}
}
