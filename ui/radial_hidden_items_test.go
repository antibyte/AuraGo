package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"
)

func TestRadialHiddenItemsOverrideTheItemDisplay(t *testing.T) {
	css, err := os.ReadFile("shared-components.css")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?s)\.radial-item\.is-hidden\s*\{[^}]*display:\s*none`).Match(css) {
		t.Fatal("shared-components.css needs .radial-item.is-hidden { display: none }: .radial-item outranks .is-hidden")
	}
}

func TestRadialLogoutStaysHiddenWithoutAuthBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	files := http.FileServer(http.Dir("."))
	mux.Handle("/shared-utilities.css", files)
	mux.Handle("/shared-components.css", files)
	mux.HandleFunc("/radial", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><link rel="stylesheet" href="/shared-utilities.css"><link rel="stylesheet" href="/shared-components.css">
<nav class="radial-menu open"><div class="radial-items">
<a href="/" class="radial-item" style="--radial-index:1"><span class="radial-item-label">Chat</span><span class="radial-item-icon">c</span></a>
<button id="radialLogout" type="button" class="radial-item radial-item-button is-hidden" style="--radial-index:2"><span class="radial-item-label">Logout</span><span class="radial-item-icon">l</span></button>
</div></nav>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(30 * time.Second)
	defer page.Close()
	page.MustNavigate(srv.URL + "/radial").MustWaitLoad()
	page.MustEval(`() => {
		const logout = document.getElementById('radialLogout');
		const hidden = getComputedStyle(logout).display;
		if (hidden !== 'none') throw Error('hidden logout renders as ' + hidden);
		logout.classList.remove('is-hidden');
		const shown = getComputedStyle(logout).display;
		if (shown !== 'flex') throw Error('revealed logout renders as ' + shown);
	}`)
}
