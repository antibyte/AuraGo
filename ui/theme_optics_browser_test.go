package ui

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestChatThemeOpticsRenderInTheBrowser opens the real chat page in a headless
// browser and checks that every themed chat actually paints its optical
// backdrop: the canvas exists, the WebGL context is live, the shader compiled
// and the layer ends up behind the messages.
//
// The Standard theme is the one exception on purpose: it promises a plain,
// noise-free look, so it must not get a backdrop.
func TestChatThemeOpticsRenderInTheBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	page := readDesktopAssetText(t, "index.html")
	var themes []string
	for _, m := range regexp.MustCompile(`class="chat-theme-option" data-theme="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		themes = append(themes, m[1])
	}
	if len(themes) < 10 {
		t.Fatalf("expected the chat theme picker options in index.html, got %v", themes)
	}

	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(page, "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	// The optics controller needs the theme effect loader, the shared engine
	// and the recipes, exactly as the page loads them.
	fixture := `<script src="/js/shared/lazy-assets.js"></script>` +
		`<script src="/js/shared/shared-core.js"></script>` +
		`<script src="/js/chat/theme-effects.js"></script>` +
		`<script src="/js/shared/theme-optics-core.js"></script>` +
		`<script src="/js/chat/theme-optics-recipes.js"></script>` +
		`<script src="/js/chat/theme-optics.js"></script>`
	html = strings.Replace(html, "</body>", fixture+"</body>", 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(html))
		case strings.HasPrefix(r.URL.Path, "/api/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"enabled":false}`))
		default:
			http.FileServer(http.FS(Content)).ServeHTTP(w, r)
		}
	}))
	defer srv.Close()

	browser := newSmokeBrowser(t)
	p := browser.MustPage().Timeout(60 * time.Second)
	p.MustSetViewport(1440, 900, 1, false)
	p.MustNavigate(srv.URL).MustWaitLoad()
	p.MustWait(`() => !!window.AuraChatThemeEffects && !!window.AuraChatThemeOptics && !!window.AuraThemeOptics`)

	for _, theme := range themes {
		report := p.MustEval(`(theme) => {
			document.documentElement.dataset.theme = theme;
			window.dispatchEvent(new CustomEvent('aurago:themechange', { detail: { theme } }));
			return new Promise(resolve => setTimeout(() => {
				const canvas = document.getElementById('aurago-chat-optics');
				if (!canvas) {
					const probe = document.createElement('canvas');
					const probeGl = probe.getContext('webgl') || probe.getContext('experimental-webgl');
					resolve('no canvas | webgl=' + (probeGl ? 'yes' : 'no') +
						' core=' + (window.AuraThemeOptics ? 'yes' : 'no') +
						' recipes=' + (window.AuraChatThemeOptics ? 'yes' : 'no') +
						' runtime=' + (window.AuraChatThemeOpticsRuntime ? window.AuraChatThemeOpticsRuntime.getTheme() : 'no') +
						' chatbox=' + (document.getElementById('chat-box') ? 'yes' : 'no'));
				}
				const rect = canvas.getBoundingClientRect();
				if (!rect.width || !rect.height) return resolve('canvas has no size | err=' + (window.AuraThemeOptics ? window.AuraThemeOptics.lastError : ''));
				const gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
				if (!gl) return resolve('no webgl context');
				if (gl.isContextLost()) return resolve('webgl context lost');
				// The drawing buffer is dropped after the compositor took it, so the
				// pixels have to be read inside the frame the engine painted.
				requestAnimationFrame(() => {
					const w = canvas.width;
					const h = canvas.height;
					const buf = new Uint8Array(w * h * 4);
					gl.readPixels(0, 0, w, h, gl.RGBA, gl.UNSIGNED_BYTE, buf);
					// A faint theme paints into the colour channels long before
					// the alpha channel survives the 8 bit rounding, so every
					// channel counts as a painted pixel.
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
					const content = document.getElementById('chat-content');
					const contentStyle = content ? getComputedStyle(content) : null;
					resolve(JSON.stringify({
						painted: maxAlpha > 0 || maxColour > 0,
						alpha: maxAlpha,
						colour: maxColour,
						lit: lit,
						ready: canvas.classList.contains('is-ready'),
						opacity: style.opacity,
						display: style.display,
						pointerEvents: style.pointerEvents,
						contentZ: contentStyle ? contentStyle.zIndex : 'none',
						canvasZ: style.zIndex,
						err: window.AuraThemeOptics ? window.AuraThemeOptics.lastError : ''
					}));
				});
			}, 700));
		}`, theme).Str()

		if theme == "dark" {
			// The Standard theme must stay plain.
			if !strings.Contains(report, "no canvas") {
				t.Errorf("Standard chat theme must not mount an optics canvas, got %s", report)
			}
			continue
		}

		if strings.HasPrefix(report, "no canvas") {
			t.Errorf("%s: theme optics canvas is missing (%s)", theme, report)
			continue
		}
		if strings.HasPrefix(report, "no webgl") {
			t.Errorf("%s: theme optics has no WebGL context (%s)", theme, report)
			continue
		}
		if strings.HasPrefix(report, "webgl context lost") {
			t.Errorf("%s: theme optics lost its WebGL context (%s)", theme, report)
			continue
		}
		if strings.HasPrefix(report, "canvas has no size") {
			t.Errorf("%s: theme optics canvas has no size (%s)", theme, report)
			continue
		}
		if !strings.Contains(report, `"painted":true`) {
			t.Errorf("%s: theme optics never painted a pixel (%s)", theme, report)
			continue
		}
		if !strings.Contains(report, `"ready":true`) {
			t.Errorf("%s: theme optics layer never revealed itself (%s)", theme, report)
		}
		if strings.Contains(report, `"pointerEvents":"auto"`) {
			t.Errorf("%s: theme optics layer must not take pointer events (%s)", theme, report)
		}
		// The layer belongs behind the conversation, never on top of it.
		if !strings.Contains(report, `"contentZ":"`) {
			t.Errorf("%s: chat content has no stacking order to compare against (%s)", theme, report)
			continue
		}
		contentZ, canvasZ := opticsZIndexes(report, "contentZ")
		if contentZ <= canvasZ {
			t.Errorf("%s: theme optics layer must sit behind the chat content (content z=%d, canvas z=%d)", theme, contentZ, canvasZ)
		}
	}
	p.MustClose()
}

// opticsZIndexes pulls the content and canvas stacking order out of the browser
// report. Auto counts as 0, because an unlayered element paints below any
// stacked sibling.
func opticsZIndexes(report string, contentKey string) (content int, canvas int) {
	content, canvas = -1, -1
	for _, field := range []struct {
		key string
		dst *int
	}{
		{contentKey, &content},
		{"canvasZ", &canvas},
	} {
		marker := `"` + field.key + `":"`
		idx := strings.Index(report, marker)
		if idx < 0 {
			continue
		}
		rest := report[idx+len(marker):]
		end := strings.IndexAny(rest, `",}`)
		if end < 0 {
			continue
		}
		value := rest[:end]
		if value == "auto" {
			*field.dst = 0
			continue
		}
		if n, err := strconv.Atoi(value); err == nil {
			*field.dst = n
		}
	}
	return content, canvas
}
