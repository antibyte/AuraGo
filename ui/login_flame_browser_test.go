package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestLoginFlameKeepsFormInteractiveBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var translations map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/common/de.json"), &translations); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"systemLang": "de", "totpEnabled": true, "i18n": translations})
	if err != nil {
		t.Fatal(err)
	}
	html := renderUIPageWith(t, "login.html", uiPageOptions{
		Lang: "de", TemplateDataJSON: string(data),
	})
	mux := http.NewServeMux()
	var authenticated atomic.Bool
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/api/auth/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"enabled":true,"authenticated":%t,"totp_enabled":true}`, authenticated.Load())
	})
	mux.HandleFunc("/auth/webauthn/client.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
	})
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Password string `json:"password"`
			TOTPCode string `json:"totp_code"`
		}
		if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&body) != nil || body.Password != "fixture-login" || body.TOTPCode != "123456" {
			http.Error(w, "invalid fixture credentials", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		authenticated.Store(true)
		fmt.Fprint(w, `{"ok":true,"redirect":"/authenticated"}`)
	})
	mux.HandleFunc("/authenticated", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Fixture login complete")
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	browser := newWebGLSmokeBrowser(t)
	for _, mode := range []string{"browser", "capture-api", "no-webgl", "reduced-motion"} {
		t.Run(mode, func(t *testing.T) {
			authenticated.Store(false)
			page := browser.MustPage().Timeout(40 * time.Second)
			defer page.Close()
			page.MustSetViewport(1114, 634, 1, false)
			page.MustEvalOnNewDocument(`(() => {
				window.__loginCaptureCalls = 0;
				if (` + fmt.Sprintf("%q", mode) + ` === 'capture-api') {
					CanvasRenderingContext2D.prototype.drawElementImage = function() { window.__loginCaptureCalls++; };
					HTMLCanvasElement.prototype.requestPaint = function() { window.__loginCaptureCalls++; };
				}
				if (` + fmt.Sprintf("%q", mode) + ` === 'no-webgl') {
					const getContext = HTMLCanvasElement.prototype.getContext;
					HTMLCanvasElement.prototype.getContext = function(type, ...args) {
						return /webgl/.test(type) ? null : getContext.call(this, type, ...args);
					};
				}
			})()`)
			motion := "no-preference"
			if mode == "reduced-motion" {
				motion = "reduce"
			}
			if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: motion}}}).Call(page); err != nil {
				t.Fatal(err)
			}
			page.MustNavigate(server.URL + "/login").MustWaitLoad()
			page.MustEval(`async () => {
				window.loginFlameFixture = (await import('/js/login/flame-wrap.js?v=test')).default;
				await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
			}`)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				page.MustScreenshot(filepath.Join(dir, "login-"+mode+".png"))
			}
			check := `() => {
				const card = document.getElementById('main-content');
				if (card.closest('canvas, [aria-hidden="true"]')) throw Error('login form hidden inside decorative canvas');
				if (card.getBoundingClientRect().height < 200) throw Error('login card collapsed');
				for (const id of ['password', 'totpCode', 'btnLogin']) {
					const control = document.getElementById(id), box = control.getBoundingClientRect();
					if (box.width < 40 || box.height < 30 || box.left < 0 || box.right > innerWidth || box.top < 0 || box.bottom > innerHeight) throw Error(id + ' not visible');
					const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
					if (!control.contains(hit)) throw Error(id + ' cannot receive clicks');
				}
				if (window.__loginCaptureCalls) throw Error('decorative effect captured login controls');
			}`
			page.MustEval(check)
			if mode == "browser" || mode == "capture-api" {
				page.MustEval(`() => { if (!window.loginFlameFixture || !document.getElementById('flame-output').classList.contains('is-active')) throw Error('flame renderer did not start'); }`)
			}
			page.MustSetViewport(390, 844, 1, true)
			page.MustEval(`async () => {
				document.documentElement.setAttribute('data-theme', 'light');
				window.dispatchEvent(new CustomEvent('aurago:themechange', {detail: {theme: 'light'}}));
				await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
			}`)
			page.MustEval(check)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				page.MustScreenshot(filepath.Join(dir, "login-"+mode+"-mobile-light.png"))
			}
			page.MustElement("#password").MustInput("fixture-login")
			page.MustElement("#totpCode").MustInput("123456")
			if mode == "capture-api" {
				page.MustEval(`() => window.loginFlameFixture.destroy()`)
				page.MustEval(check)
			}
			page.MustElement("#btnLogin").MustClick()
			page.MustWait(`() => location.pathname === '/authenticated'`)
		})
	}
}
