package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDesktopAgentChatOpticsRenderInTheBrowser drives the virtual desktop agent
// chat optics in a real browser: every desktop theme must mount the shared
// WebGL layer, paint it, keep it behind the messages and let the window tear it
// down again.
//
// AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopAgentChatOptics -timeout 10m
func TestDesktopAgentChatOpticsRenderInTheBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	// The fixture reproduces the window geometry the controller expects: a host
	// with a scrolling log and a message row on top of it.
	const fixture = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>agent chat optics</title>
<link rel="stylesheet" href="/css/theme-optics.css">
<style>
  html, body { margin: 0; height: 100%; background: #101018; }
  #host { position: relative; width: 760px; height: 520px; }
  .vd-chat-log {
    position: relative;
    width: 100%;
    height: 100%;
    overflow-y: auto;
    background: #15151f;
  }
  .vd-chat-message-row { position: relative; z-index: 1; height: 300px; color: #eee; }
</style>
</head>
<body data-theme="fruity">
  <div id="host">
    <div class="vd-chat-log">
      <div class="vd-chat-message-row">hello</div>
    </div>
  </div>
  <script src="/js/shared/theme-optics-core.js"></script>
  <script src="/js/desktop/apps/agent-chat-optics-recipes.js"></script>
  <script src="/js/desktop/apps/agent-chat-optics.js"></script>
  <script>
    window.opticsHandle = window.AuraDesktopChatOptics.mount(document.getElementById('host'));
  </script>
</body>
</html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(fixture))
			return
		}
		http.FileServer(http.FS(Content)).ServeHTTP(w, r)
	}))
	defer srv.Close()

	browser := newSmokeBrowser(t)
	p := browser.MustPage().Timeout(60 * time.Second)
	p.MustSetViewport(1024, 768, 1, false)
	p.MustNavigate(srv.URL).MustWaitLoad()
	p.MustWait(`() => !!window.AuraDesktopChatOptics && !!window.AuraDesktopChatOpticsRecipes && !!window.AuraThemeOptics && !!window.opticsHandle`)

	for _, theme := range desktopThemeIDsFromRegistry(t) {
		report := p.MustEval(`(theme) => {
			document.body.dataset.theme = theme;
			return new Promise(resolve => setTimeout(() => {
				const canvas = document.getElementById('aurago-desktop-chat-optics');
				if (!canvas) return resolve('no canvas | recipes=' + (window.AuraDesktopChatOpticsRecipes ? 'yes' : 'no'));
				const rect = canvas.getBoundingClientRect();
				if (!rect.width || !rect.height) return resolve('canvas has no size | err=' + (window.AuraThemeOptics ? window.AuraThemeOptics.lastError : ''));
				const gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
				if (!gl) return resolve('no webgl context');
				if (gl.isContextLost()) return resolve('webgl context lost');
				requestAnimationFrame(() => {
					const w = canvas.width;
					const h = canvas.height;
					const buf = new Uint8Array(w * h * 4);
					gl.readPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, buf);
					let maxAlpha = 0;
					let maxColour = 0;
					let lit = 0;
					for (let i = 0; i < buf.length; i += 4) {
						if (buf[i + 3] > maxAlpha) maxAlpha = buf[i + 3];
						const colour = Math.max(buf[i], buf[i + 1], buf[i + 2]);
						if (colour > maxColour) maxColour = colour;
						if (colour > 0 || buf[i + 3] > 0) lit++;
					}
					const style = getComputedStyle(canvas);
					const row = document.querySelector('.vd-chat-message-row');
					const rowStyle = row ? getComputedStyle(row) : null;
					resolve(JSON.stringify({
						painted: maxAlpha > 0 || maxColour > 0,
						alpha: maxAlpha,
						colour: maxColour,
						lit: lit,
						ready: canvas.classList.contains('is-ready'),
						parent: canvas.parentElement ? canvas.parentElement.className : '',
						opacity: style.opacity,
						display: style.display,
						pointerEvents: style.pointerEvents,
						rowZ: rowStyle ? rowStyle.zIndex : 'none',
						canvasZ: style.zIndex,
						err: window.AuraThemeOptics ? window.AuraThemeOptics.lastError : ''
					}));
				});
			}, 700));
		}`, theme).Str()

		if strings.HasPrefix(report, "no canvas") {
			t.Errorf("%s: desktop agent chat optics canvas is missing (%s)", theme, report)
			continue
		}
		if strings.HasPrefix(report, "no webgl") {
			t.Errorf("%s: desktop agent chat optics has no WebGL context (%s)", theme, report)
			continue
		}
		if strings.HasPrefix(report, "webgl context lost") {
			t.Errorf("%s: desktop agent chat optics lost its WebGL context (%s)", theme, report)
			continue
		}
		if strings.HasPrefix(report, "canvas has no size") {
			t.Errorf("%s: desktop agent chat optics canvas has no size (%s)", theme, report)
			continue
		}
		if !strings.Contains(report, `"painted":true`) {
			t.Errorf("%s: desktop agent chat optics never painted a pixel (%s)", theme, report)
			continue
		}
		if !strings.Contains(report, `"ready":true`) {
			t.Errorf("%s: desktop agent chat optics layer never revealed itself (%s)", theme, report)
		}
		if strings.Contains(report, `"pointerEvents":"auto"`) {
			t.Errorf("%s: desktop agent chat optics layer must not take pointer events (%s)", theme, report)
		}
		if !strings.Contains(report, `vd-chat-log`) {
			t.Errorf("%s: desktop agent chat optics canvas must live in the chat log (%s)", theme, report)
		}
		// The light belongs behind the messages.
		rowZ, canvasZ := opticsZIndexes(report, "rowZ")
		if rowZ <= canvasZ {
			t.Errorf("%s: desktop agent chat optics layer must sit behind the messages (row z=%d, canvas z=%d)", theme, rowZ, canvasZ)
		}
	}

	// Closing the window has to take the canvas and the WebGL context with it.
	tornDown := p.MustEval(`() => {
		window.opticsHandle.dispose();
		const canvas = document.getElementById('aurago-desktop-chat-optics');
		return JSON.stringify({ gone: !canvas });
	}`).Str()
	if !strings.Contains(tornDown, `"gone":true`) {
		t.Errorf("disposing the agent chat optics must remove the canvas, got %s", tornDown)
	}

	p.MustClose()
}
