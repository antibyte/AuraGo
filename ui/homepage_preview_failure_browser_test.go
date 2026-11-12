package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomepageStudioFailedLocalPreviewBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/previewed" {
			_, _ = w.Write([]byte("<!doctype html><html><body><h1>Published website</h1></body></html>"))
			return
		}
		_, _ = w.Write([]byte(`<!doctype html><html><body><div id="hp-host" style="width:1200px;height:700px"></div></body></html>`))
	}))
	defer origin.Close()
	page := newSmokeBrowser(t).MustPage(origin.URL)
	defer page.MustClose()
	page.MustSetViewport(1440, 900, 1, false)
	for _, name := range []string{"preview", "sites", "history", "annotate", ""} {
		path := "js/desktop/apps/homepage-studio"
		if name != "" {
			path += "-" + name
		}
		if err := page.AddScriptTag("", readDesktopAssetText(t, path+".js")); err != nil {
			t.Fatal(err)
		}
	}
	if err := page.AddStyleTag("", readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")); err != nil {
		t.Fatal(err)
	}
	mount := strings.Replace(homepageStudioSmokeMount, "preview_url: window.location.origin + '/previewed',", "preview_url: window.location.origin + '/previewed', local_preview: { available: !!window.__hpLocalAvailable, http_status: window.__hpLocalAvailable ? 200 : 404 },", 1)
	mount = strings.Replace(mount, "web_container: { running: false }", "web_container: { running: true }", 1)
	page.MustEval(mount, homepageStudioSmokeSites)
	assertUnavailable := func() {
		t.Helper()
		page.MustWait(`() => document.querySelector('.vd-hp-status-dot.offline') !== null`)
		result := page.MustEval(`() => {
 const placeholder = document.querySelector('.vd-hp-preview-placeholder');
 const rect = placeholder.getBoundingClientRect();
 const hit = document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2);
 return { frames: document.querySelectorAll('.vd-hp-preview-iframe').length,
  visible: rect.width > 0 && rect.height > 0 && placeholder.contains(hit),
  rect: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
  hit: hit ? hit.outerHTML.slice(0, 300) : '',
  frameDisplay: getComputedStyle(document.querySelector('.vd-hp-preview-frame')).display,
  status: document.querySelector('.vd-hp-status-pill').textContent,
  externalDisabled: document.querySelector('[id^="hp-external-"]').disabled };
}`)
		if result.Get("frames").Int() != 0 || !result.Get("visible").Bool() || !result.Get("externalDisabled").Bool() || !strings.Contains(result.Get("status").Str(), "404") {
			t.Fatalf("failed publication must show a visible explanation: %s", result.JSON("", ""))
		}
	}
	assertUnavailable()
	page.MustEval(`() => { window.__hpLocalAvailable = true; document.querySelector('[id^="hp-refresh-"]').click(); }`)
	page.MustWait(`() => document.querySelector('.vd-hp-preview-iframe') !== null && document.querySelector('.vd-hp-status-dot.online') !== null`)
	frame := page.MustElement(".vd-hp-preview-iframe").MustFrame()
	if text := frame.MustElement("h1").MustText(); text != "Published website" {
		t.Fatalf("recovered preview rendered %q", text)
	}
	page.MustEval(`() => { window.__hpLocalAvailable = false; document.querySelector('[id^="hp-refresh-"]').click(); }`)
	assertUnavailable()
}
