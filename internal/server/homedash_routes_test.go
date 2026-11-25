package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/homedash"
)

func TestHomeDashRoutesRequireDesktopAdmin(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	if w := env.do(t, "", http.MethodGet, "bootstrap", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.read, http.MethodGet, "bootstrap", nil); w.Code != http.StatusForbidden || hdCode(t, w) != "desktop_scope_required" {
		t.Fatalf("desktop:read = %d %s", w.Code, w.Body.String())
	}
	w := env.do(t, env.admin, http.MethodGet, "bootstrap", nil)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("admin = %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	var boot struct {
		Status         string   `json:"status"`
		APIVersion     int      `json:"api_version"`
		StreamProtocol int      `json:"stream_protocol"`
		CatalogHash    string   `json:"catalog_hash"`
		Templates      []string `json:"templates"`
		Limits         struct {
			Subscriptions int `json:"subscriptions"`
		} `json:"limits"`
	}
	hdDecode(t, w, &boot)
	if boot.Status != "ok" || boot.APIVersion != 1 || boot.StreamProtocol != 1 || boot.CatalogHash != homedash.Catalog().Hash ||
		len(boot.Templates) != 1 || boot.Templates[0] != "system" || boot.Limits.Subscriptions != 300 {
		t.Fatalf("bootstrap = %+v", boot)
	}
}

func TestHomeDashRouteTableAnswers404And405(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	if w := env.do(t, env.admin, http.MethodGet, "kiosk/devices", nil); w.Code != http.StatusNotFound || hdCode(t, w) != "not_found" {
		t.Fatalf("unknown route = %d %s", w.Code, w.Body.String())
	}
	w := env.do(t, env.admin, http.MethodDelete, "bootstrap", nil)
	if w.Code != http.StatusMethodNotAllowed || hdCode(t, w) != "method_not_allowed" || w.Header().Get("Allow") != "GET" {
		t.Fatalf("405 = %d %v %s", w.Code, w.Header(), w.Body.String())
	}
}

func TestHomeDashUnavailableAnswers503(t *testing.T) {
	for name, mutate := range map[string]func(*config.Config){
		"homedash off": func(c *config.Config) { c.HomeDash.Enabled = false },
		"egg mode":     func(c *config.Config) { c.EggMode.Enabled = true },
		"no desktop":   func(c *config.Config) { c.VirtualDesktop.Enabled = false },
	} {
		env := newHomeDashTestEnv(t, mutate)
		if w := env.do(t, env.admin, http.MethodGet, "bootstrap", nil); w.Code != http.StatusServiceUnavailable || hdCode(t, w) != "homedash_disabled" {
			t.Fatalf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	env := newHomeDashTestEnv(t, nil)
	env.s.closeHomeDash()
	if w := env.do(t, env.admin, http.MethodGet, "bootstrap", nil); w.Code != http.StatusServiceUnavailable || hdCode(t, w) != "storage_unavailable" {
		t.Fatalf("no service: %d %s", w.Code, w.Body.String())
	}
}

func TestHomeDashWidgetCatalogETag(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	w := env.do(t, env.admin, http.MethodGet, "widget-catalog", nil)
	etag := `"` + homedash.Catalog().Hash + `"`
	if w.Code != http.StatusOK || w.Header().Get("ETag") != etag {
		t.Fatalf("catalog = %d %v", w.Code, w.Header())
	}
	var body struct {
		Catalog homedash.WidgetCatalog `json:"catalog"`
	}
	hdDecode(t, w, &body)
	if body.Catalog.Hash != homedash.Catalog().Hash || len(body.Catalog.Widgets) != 24 {
		t.Fatalf("catalog hash %q, %d widgets", body.Catalog.Hash, len(body.Catalog.Widgets))
	}
	req := httptestRequestWithHeader(t, env, "widget-catalog", "If-None-Match", etag)
	if req.Code != http.StatusNotModified || req.Body.Len() != 0 {
		t.Fatalf("If-None-Match = %d %q", req.Code, req.Body.String())
	}
}

func TestHomeDashStatusTableCoversCoreCodes(t *testing.T) {
	want := map[string]int{
		homedash.CodeBadRequest: 400, homedash.CodeInvalidArgs: 400, homedash.CodeUnsupportedCommand: 400,
		homedash.CodeReadOnly: 403, homedash.CodeForbiddenEntity: 403, homedash.CodeViewOnly: 403,
		homedash.CodeDashboardNotFound: 404, homedash.CodeEntityNotFound: 404, homedash.CodeStreamNotFound: 404,
		homedash.CodeRevisionNotFound: 404, homeDashCodeNotFound: 404, homeDashCodeMethodNotAllowed: 405,
		homedash.CodeRevisionConflict: 409, homedash.CodeConfirmationRequired: 409, homedash.CodeSchemaTooNew: 409,
		homedash.CodeSlugTaken: 409, homeDashCodeBodyTooLarge: 413, homedash.CodeInvalidDocument: 422,
		homedash.CodeLimitExceeded: 422, homedash.CodeRateLimited: 429, homedash.CodeTooManyStreams: 429,
		homedash.CodeCommandFailed: 502, homedash.CodeSourceError: 502, homedash.CodeHomeDashDisabled: 503,
		homedash.CodeSourceUnavailable: 503, homedash.CodeHubClosed: 503, homeDashCodeStorageUnavailable: 503,
		homeDashCodeTimeout: 504, homeDashCodeInternal: 500,
	}
	for code, status := range want {
		if got := homeDashStatus(code); got != status {
			t.Errorf("%s = %d, want %d", code, got, status)
		}
	}
}

func httptestRequestWithHeader(t *testing.T, env *homeDashTestEnv, path, key, value string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, homeDashAPIPrefix+path, nil)
	r.Header.Set("Authorization", "Bearer "+env.admin)
	r.Header.Set(key, value)
	w := httptest.NewRecorder()
	handleHomeDash(env.s).ServeHTTP(w, r)
	return w
}

func TestHomeDashPrincipalIgnoresUnvalidatedCookieWhenLoginIsOff(t *testing.T) {
	req := func(cookie string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/homedash/bootstrap", nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie})
		return r
	}
	off := &config.Config{}
	a, b := homeDashPrincipal(off, req("random-one")), homeDashPrincipal(off, req("random-two"))
	if a.Key != "local-desktop" || b.Key != "local-desktop" {
		t.Errorf("auth off: keys %q %q, want local-desktop", a.Key, b.Key)
	}
	on := &config.Config{}
	on.Auth.Enabled = true
	if k := homeDashPrincipal(on, req("random-one")).Key; !strings.HasPrefix(k, "s:") {
		t.Errorf("auth on: key %q, want s: prefix", k)
	}
	if k := homeDashPrincipal(nil, req("random-one")).Key; k != "local-desktop" {
		t.Errorf("nil config: key %q", k)
	}
}
