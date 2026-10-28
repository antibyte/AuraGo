package ui

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestChatThemesKeepTheOpenRadialMenuAboveItsBackdropBrowser opens the radial
// menu on the real Chat DOM and stylesheets in every chat theme and hit-tests
// the trigger and every item. A theme that lowers the menu below the blurring
// backdrop leaves the page blurred with nothing clickable (Galaxy did).
func TestChatThemesKeepTheOpenRadialMenuAboveItsBackdropBrowser(t *testing.T) {
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
	html = strings.Replace(html, "</head>", `<style>*,*::before,*::after{transition:none!important;animation:none!important}</style>`+
		`<script src="/js/shared/shared-core.js"></script></head>`, 1)
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
	for _, size := range [][2]int{{1440, 900}, {390, 844}} {
		p := browser.MustPage().Timeout(60 * time.Second)
		p.MustSetViewport(size[0], size[1], 1, size[0] < 768)
		p.MustNavigate(srv.URL).MustWaitLoad()
		p.MustWait(`() => !!document.getElementById('radialMenu')`)
		for _, theme := range themes {
			blocked := p.MustEval(`(theme) => {
				document.documentElement.dataset.theme = theme;
				const menu = document.getElementById('radialMenu'), trigger = document.getElementById('radialTrigger');
				if (!menu.classList.contains('open')) trigger.click();
				const name = el => el ? (el.id || el.getAttribute('href') || String(el.className)) : 'nothing';
				const targets = [trigger, ...menu.querySelectorAll('.radial-item:not(.is-hidden)')];
				const blocked = targets.flatMap(el => {
					const r = el.getBoundingClientRect(), x = r.left + r.width / 2, y = r.top + r.height / 2;
					if (!r.width || !r.height || x < 0 || y < 0 || x >= innerWidth || y >= innerHeight) return [];
					const top = document.elementFromPoint(x, y);
					return top && (el === top || el.contains(top)) ? [] : [name(el) + ' under ' + name(top)];
				});
				// The blurring backdrop must still cover the rest of the page.
				const rects = targets.map(el => el.getBoundingClientRect()), backdrop = document.getElementById('radialBackdrop');
				for (let i = 0; i < 5; i++) for (let j = 0; j < 5; j++) {
					const x = innerWidth * (i + 0.5) / 5, y = innerHeight * (j + 0.5) / 5;
					if (rects.some(r => x >= r.left && x <= r.right && y >= r.top && y <= r.bottom)) continue;
					const top = document.elementFromPoint(x, y);
					if (top !== backdrop && !menu.contains(top)) blocked.push(name(top) + ' above the backdrop');
				}
				trigger.click();
				return [...new Set(blocked)].join('; ');
			}`, theme).Str()
			if blocked != "" {
				t.Errorf("%s at %dx%d: open radial menu is not clickable: %s", theme, size[0], size[1], blocked)
			}
		}
		p.MustClose()
	}
}
