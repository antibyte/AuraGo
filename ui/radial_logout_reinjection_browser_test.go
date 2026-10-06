package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// radialLogoutLayout models how a page gets its radial menu:
//   - static: the anchor is in the HTML and initShared injects the menu;
//   - plans: like static, then the page injects the menu again on
//     DOMContentLoaded (ui/js/plans/main.js);
//   - desktop-*: shared-core.js is deferred and a dynamically loaded bundle
//     creates the anchor and injects the menu later
//     (ensureDesktopRadialMenuAnchor). "fast" injects before the auth status
//     arrives, "slow" after it.
type radialLogoutLayout struct {
	name                       string
	desktop                    bool
	reinjectOnDOMContentLoaded bool
	statusDelay, bundleDelay   time.Duration
}

func TestRadialLogoutWorksAfterTheMenuIsReinjectedBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	layouts := []radialLogoutLayout{
		{name: "static"},
		{name: "plans", reinjectOnDOMContentLoaded: true},
		{name: "desktop-fast-bundle", desktop: true, statusDelay: 400 * time.Millisecond},
		{name: "desktop-slow-bundle", desktop: true, bundleDelay: 400 * time.Millisecond},
	}
	browser := newSmokeBrowser(t)
	for _, layout := range layouts {
		for _, auth := range []bool{false, true} {
			layout, auth := layout, auth
			t.Run(fmt.Sprintf("%s/auth=%v", layout.name, auth), func(t *testing.T) {
				srv := httptest.NewServer(radialLogoutFixture(layout, auth))
				defer srv.Close()
				page := browser.MustPage().Timeout(30 * time.Second)
				defer page.Close()
				page.MustNavigate(srv.URL + "/page").MustWaitLoad()
				got := page.MustEval(`async (auth) => {
					const sleep = ms => new Promise(r => setTimeout(r, ms));
					const loaded = Date.now() + 10000;
					while (!(window.__authStatusSeen && window.__layoutDone) && Date.now() < loaded) await sleep(25);
					if (!(window.__authStatusSeen && window.__layoutDone)) return 'page never finished loading';
					let el = null;
					const settled = Date.now() + (auth ? 5000 : 500);
					while (Date.now() < settled) {
						el = document.getElementById('radialLogout');
						if (auth && el && !el.classList.contains('is-hidden')) break;
						await sleep(25);
					}
					if (!el) return 'no radialLogout';
					window.__logouts = 0;
					window.performLogout = () => { window.__logouts++; };
					document.getElementById('radialMenu').classList.add('open');
					const display = getComputedStyle(el).display;
					el.dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true}));
					// Exactly one logout per click: initLogoutLinks never binds twice.
					return 'display=' + display + ' bound=' + (el.dataset.logoutBound === 'true') + ' logoutsPerClick=' + window.__logouts;
				}`, auth).Str()
				want := "display=none"
				ok := strings.HasPrefix(got, want+" ")
				if auth {
					want = "display=flex bound=true logoutsPerClick=1"
					ok = got == want
				}
				if !ok {
					t.Fatalf("%s with auth=%v: Logout %s, want %s", layout.name, auth, got, want)
				}
			})
		}
	}
}

func radialLogoutFixture(layout radialLogoutLayout, auth bool) http.Handler {
	mux := http.NewServeMux()
	files := http.FileServer(http.Dir("."))
	mux.Handle("/shared-utilities.css", files)
	mux.Handle("/shared-components.css", files)
	mux.Handle("/js/", files)
	mux.HandleFunc("/api/auth/status", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(layout.statusDelay)
		w.Header().Set("Content-Type", "application/json")
		if auth {
			fmt.Fprint(w, `{"enabled":true,"authenticated":true,"expires_in_seconds":86400}`)
		} else {
			fmt.Fprint(w, `{"enabled":false}`)
		}
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/loader.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, `(function(){var s=document.createElement('script');s.src='/bundle.js';document.head.appendChild(s);})();`)
	})
	mux.HandleFunc("/bundle.js", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(layout.bundleDelay)
		w.Header().Set("Content-Type", "text/javascript")
		// The order of ensureDesktopRadialMenuAnchor in desktop-foundation.js.
		fmt.Fprint(w, `(function(){var a=document.createElement('div');a.id='radialMenuAnchor';document.body.appendChild(a);`+
			`if(typeof injectRadialMenu==='function')injectRadialMenu();if(typeof initRadialMenu==='function')initRadialMenu();window.__layoutDone=true;})();`)
	})
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		var b strings.Builder
		// Marks when the auth status response has been handed to checkAuth.
		b.WriteString(`<!doctype html><html><head><script>(function(){var f=window.fetch.bind(window);window.fetch=function(u,o){var p=f(u,o);` +
			`if(String(u).indexOf('/api/auth/status')>=0){p.then(function(){setTimeout(function(){window.__authStatusSeen=true;},50);},function(){window.__authStatusSeen=true;});}return p;};})();</script>`)
		b.WriteString(`<link rel="stylesheet" href="/shared-utilities.css"><link rel="stylesheet" href="/shared-components.css">`)
		if layout.desktop {
			b.WriteString(`<script defer src="/js/shared/shared-core.js"></script><script defer src="/loader.js"></script></head><body>`)
		} else {
			b.WriteString(`<script src="/js/shared/shared-core.js"></script></head><body><div id="radialMenuAnchor"></div><script>document.addEventListener('DOMContentLoaded',function(){`)
			if layout.reinjectOnDOMContentLoaded {
				b.WriteString(`injectRadialMenu();`) // ui/js/plans/main.js after initShared
			}
			b.WriteString(`window.__layoutDone=true;});</script>`)
		}
		b.WriteString(`</body></html>`)
		fmt.Fprint(w, b.String())
	})
	return mux
}
