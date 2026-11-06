package render

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/storymaker"
)

// testRenderToken is the token of the listener fixture: 32 hex characters, like NewPrefix gives.
const testRenderToken = "feedfacefeedfacefeedfacefeedface"

type listenerFixture struct {
	l      *Listener
	client *http.Client
	mu     sync.Mutex
	report []byte
	frames []string
	errors []PageError
}

func newListenerFixture(t *testing.T) *listenerFixture {
	t.Helper()
	fx := &listenerFixture{client: &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}}
	prefix, err := NewPrefix()
	if err != nil {
		t.Fatal(err)
	}
	snap := &Snapshot{
		Story:        []byte(`{"schema":"storymaker/1"}`),
		CatalogFiles: map[string][]byte{"catalog.json": []byte(`{"schema":"storymaker-catalog/1"}`), "rig/human.json": []byte(`{"id":"rig.human/1"}`)},
		Fonts:        []SnapshotFont{{Name: "default", Family: "Geist", Weight: 400, File: "/fonts/Geist-Regular.woff2", Data: []byte("wOF2font")}},
		CodeBundles:  map[string][]byte{"s3": []byte("self.__smScene=function(){};")},
		HostBundle:   []byte("/*host*/"), PlayerBundle: []byte("/*player*/"),
	}
	params := PageParams{Width: 1920, Height: 1080, FPS: 30, Transport: TransportPNG, FrameLimit: 27000, Token: testRenderToken}
	hooks := Hooks{
		Report: func(body []byte) ReportAnswer {
			fx.mu.Lock()
			fx.report = append([]byte(nil), body...)
			fx.mu.Unlock()
			return ReportAnswer{Start: true}
		},
		Frame: func(n int, ct string, data []byte) (FrameAnswer, int) {
			fx.mu.Lock()
			fx.frames = append(fx.frames, ct+"|"+string(data))
			fx.mu.Unlock()
			if n == 2 {
				return FrameAnswer{OK: true, Done: true}, http.StatusOK
			}
			return FrameAnswer{OK: true}, http.StatusOK
		},
		Error: func(e PageError) {
			fx.mu.Lock()
			fx.errors = append(fx.errors, e)
			fx.mu.Unlock()
		},
	}
	l, err := StartListener(prefix, snap, params, hooks)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	fx.l = l
	return fx
}

func (fx *listenerFixture) do(t *testing.T, method, rawURL string, body []byte, header map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	res, err := fx.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, rawURL, err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res, data
}

func TestListenerBindsLoopbackAndAnswers404OutsideTheBase(t *testing.T) {
	fx := newListenerFixture(t)
	if !strings.HasPrefix(fx.l.Origin(), "http://127.0.0.1:") {
		t.Fatalf("origin = %s", fx.l.Origin())
	}
	if len(fx.l.prefix) != 32 {
		t.Fatalf("prefix length = %d", len(fx.l.prefix))
	}
	other := strings.Repeat("0", 32)
	for _, p := range []string{
		"/", "/favicon.ico", "/" + fx.l.prefix, "/" + other + "/", "/" + other + "/boot.json",
		"/" + fx.l.prefix + "/../story.json", "/" + fx.l.prefix[:30] + "/", "/js/storymaker/host.bundle.js",
		"/" + fx.l.prefix + "/nothing", "/" + fx.l.prefix + "/catalog/missing.json", "/" + fx.l.prefix + "/catalog/../story.json",
		"/" + fx.l.prefix + "/fonts/Other.woff2", "/" + fx.l.prefix + "/scenes/nope.js", "/" + fx.l.prefix + "/story.json/",
		"/" + fx.l.prefix + "//story.json",
	} {
		res, _ := fx.do(t, http.MethodGet, fx.l.Origin()+p, nil, nil)
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", p, res.StatusCode)
		}
		if res.Header.Get("Access-Control-Allow-Origin") != "" && !strings.HasPrefix(p, "/"+fx.l.prefix) {
			t.Errorf("GET %s sent CORS headers outside the base", p)
		}
	}
	// A request that names another Host (DNS rebinding) is refused even with the right path.
	res, _ := fx.do(t, http.MethodGet, fx.l.Base()+"/boot.json", nil, map[string]string{"Host": "evil.example:80"})
	_ = res
	req, _ := http.NewRequest(http.MethodGet, fx.l.Base()+"/boot.json", nil)
	req.Host = "evil.example"
	rebound, err := fx.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = rebound.Body.Close()
	if rebound.StatusCode != http.StatusNotFound {
		t.Errorf("foreign Host header = %d, want 404", rebound.StatusCode)
	}
}

func TestListenerServesTheRenderPageWithTheExactHeaders(t *testing.T) {
	fx := newListenerFixture(t)
	res, body := fx.do(t, http.MethodGet, fx.l.Base()+"/", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	base := fx.l.Base()
	wantCSP := "sandbox allow-scripts; default-src 'none'; script-src " + base + "/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src " + base + "/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"
	if got := res.Header.Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("CSP = %q\nwant %q", got, wantCSP)
	}
	if got := res.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q", got)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := res.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO = %q", got)
	}
	want := `<!doctype html><html data-sm-mode="render" data-sm-base="` + base + `"><head><meta charset="utf-8"></head>` +
		`<body><canvas id="sm-stage" width="1920" height="1080"></canvas><script src="` + base + `/js/storymaker/host.bundle.js"></script></body></html>`
	if string(body) != want {
		t.Errorf("page:\n%s\nwant:\n%s", body, want)
	}
	if strings.Contains(string(body), "<style") || strings.Contains(string(body), "onload") || strings.Contains(string(body), "<script>") {
		t.Error("the render page carries inline script or style")
	}
	// HEAD works without a body.
	head, hb := fx.do(t, http.MethodHead, base+"/", nil, nil)
	if head.StatusCode != http.StatusOK || len(hb) != 0 {
		t.Errorf("HEAD = %d with %d body bytes", head.StatusCode, len(hb))
	}
}

func TestListenerServesTheSnapshotOnly(t *testing.T) {
	fx := newListenerFixture(t)
	base := fx.l.Base()
	_, boot := fx.do(t, http.MethodGet, base+"/boot.json", nil, nil)
	var doc map[string]any
	if err := json.Unmarshal(boot, &doc); err != nil {
		t.Fatalf("boot.json: %v\n%s", err, boot)
	}
	checks := map[string]any{"mode": "render", "fps": float64(30), "width": float64(1920), "height": float64(1080), "transport": "png",
		"frame_limit": float64(27000), "story_url": base + "/story.json", "catalog_url": base + "/catalog/catalog.json", "assets_base": base + "/catalog/"}
	for k, want := range checks {
		if doc[k] != want {
			t.Errorf("boot.%s = %v, want %v", k, doc[k], want)
		}
	}
	bundles, _ := doc["code_bundles"].(map[string]any)
	if bundles["s3"] != base+"/scenes/s3.js" {
		t.Errorf("code_bundles = %v", doc["code_bundles"])
	}
	fonts, _ := doc["fonts"].([]any)
	if len(fonts) != 1 {
		t.Fatalf("fonts = %v", doc["fonts"])
	}
	font := fonts[0].(map[string]any)
	if font["name"] != "default" || font["family"] != "Geist" || font["weight"] != float64(400) || font["url"] != base+"/fonts/Geist-Regular.woff2" {
		t.Errorf("font = %v", font)
	}
	if _, ok := doc["audio_info"].(map[string]any); !ok {
		t.Errorf("audio_info = %v", doc["audio_info"])
	}

	for path, want := range map[string]string{
		"/story.json": `{"schema":"storymaker/1"}`, "/catalog/catalog.json": `{"schema":"storymaker-catalog/1"}`,
		"/catalog/rig/human.json": `{"id":"rig.human/1"}`, "/fonts/Geist-Regular.woff2": "wOF2font", "/scenes/s3.js": "self.__smScene=function(){};",
		"/js/storymaker/host.bundle.js": "/*host*/", "/js/storymaker/player.bundle.js": "/*player*/",
	} {
		res, body := fx.do(t, http.MethodGet, base+path, nil, nil)
		if res.StatusCode != http.StatusOK || string(body) != want {
			t.Errorf("GET %s = %d %q", path, res.StatusCode, body)
		}
		if res.Header.Get("Access-Control-Allow-Origin") != "*" || res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("GET %s lacks the response headers: %v", path, res.Header)
		}
	}
	if res, _ := fx.do(t, http.MethodGet, base+"/scenes/s3.js", nil, nil); !strings.HasPrefix(res.Header.Get("Content-Type"), "text/javascript") {
		t.Errorf("bundle Content-Type = %q", res.Header.Get("Content-Type"))
	}
	if res, _ := fx.do(t, http.MethodGet, base+"/fonts/Geist-Regular.woff2", nil, nil); res.Header.Get("Content-Type") != "font/woff2" {
		t.Errorf("font Content-Type = %q", res.Header.Get("Content-Type"))
	}
	// Static routes reject POST.
	if res, _ := fx.do(t, http.MethodPost, base+"/story.json", []byte("x"), nil); res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST story.json = %d", res.StatusCode)
	}
}

func TestListenerAnswersCORSPreflight(t *testing.T) {
	fx := newListenerFixture(t)
	for _, p := range []string{"/frame", "/report", "/error", "/boot.json"} {
		res, _ := fx.do(t, http.MethodOptions, fx.l.Base()+p, nil, map[string]string{"Origin": "null", "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "content-type, x-sm-frame"})
		if res.StatusCode != http.StatusNoContent {
			t.Errorf("OPTIONS %s = %d", p, res.StatusCode)
		}
		h := res.Header
		if h.Get("Access-Control-Allow-Origin") != "*" || h.Get("Access-Control-Allow-Methods") != "GET, POST" ||
			h.Get("Access-Control-Allow-Headers") != "Content-Type, X-SM-Frame" || h.Get("Access-Control-Max-Age") != "600" {
			t.Errorf("OPTIONS %s headers = %v", p, h)
		}
	}
	// A preflight outside the base is still a 404 without CORS headers.
	res, _ := fx.do(t, http.MethodOptions, fx.l.Origin()+"/frame", nil, nil)
	if res.StatusCode != http.StatusNotFound || res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("OPTIONS outside the base = %d %v", res.StatusCode, res.Header)
	}
}

func TestListenerReportFrameAndErrorEndpoints(t *testing.T) {
	fx := newListenerFixture(t)
	base := fx.l.Base()

	if res, _ := fx.do(t, http.MethodGet, base+"/report", nil, nil); res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET report = %d", res.StatusCode)
	}
	res, body := fx.do(t, http.MethodPost, base+"/report?t="+testRenderToken, []byte(`{"frame_count":3}`), map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"start":true`) {
		t.Errorf("report = %d %s", res.StatusCode, body)
	}
	fx.mu.Lock()
	gotReport := string(fx.report)
	fx.mu.Unlock()
	if gotReport != `{"frame_count":3}` {
		t.Errorf("report body = %q", gotReport)
	}
	big := bytes.Repeat([]byte("x"), maxReportBytes+1)
	if res, body := fx.do(t, http.MethodPost, base+"/report?t="+testRenderToken, big, nil); res.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), "8 MiB") {
		t.Errorf("oversized report = %d %s", res.StatusCode, body)
	}
	// 2 MiB is over the old cap of 1 MiB and inside the new one.
	if res, _ := fx.do(t, http.MethodPost, base+"/report?t="+testRenderToken, bytes.Repeat([]byte("x"), 2<<20), nil); res.StatusCode != http.StatusOK {
		t.Errorf("2 MiB report = %d", res.StatusCode)
	}

	// Frames: the header is required, the hook sees content type and bytes, and the answer is JSON.
	if res, _ := fx.do(t, http.MethodPost, base+"/frame?t="+testRenderToken, []byte("f"), map[string]string{"Content-Type": "image/png"}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("frame without header = %d", res.StatusCode)
	}
	res, body = fx.do(t, http.MethodPost, base+"/frame?t="+testRenderToken, []byte("frame-0"), map[string]string{"Content-Type": "image/png", "X-SM-Frame": "0"})
	if res.StatusCode != http.StatusOK || string(body) != `{"ok":true}` {
		t.Errorf("frame 0 = %d %s", res.StatusCode, body)
	}
	res, body = fx.do(t, http.MethodPost, base+"/frame?t="+testRenderToken, []byte("frame-2"), map[string]string{"Content-Type": "image/png", "X-SM-Frame": "2"})
	if !strings.Contains(string(body), `"done":true`) {
		t.Errorf("last frame answer = %s", body)
	}
	fx.mu.Lock()
	if len(fx.frames) != 2 || fx.frames[0] != "image/png|frame-0" {
		t.Errorf("frames seen = %v", fx.frames)
	}
	fx.mu.Unlock()
	tooBig := bytes.Repeat([]byte("x"), int(MaxFrameBytes(IngestConfig{Width: 1920, Height: 1080, Transport: TransportPNG}))+1)
	res, body = fx.do(t, http.MethodPost, base+"/frame?t="+testRenderToken, tooBig, map[string]string{"Content-Type": "image/png", "X-SM-Frame": "5"})
	if res.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(string(body), `"stop":true`) {
		t.Errorf("oversized frame = %d %s", res.StatusCode, body)
	}

	// This message quotes the token, and the page error check below pins it without the token.
	res, body = fx.do(t, http.MethodPost, base+"/error?t="+testRenderToken, []byte(`{"code":"code_error","message":"scene s3 threw`+testRenderToken+`","scene_id":"s3"}`), map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusOK {
		t.Errorf("error endpoint = %d %s", res.StatusCode, body)
	}
	fx.do(t, http.MethodPost, base+"/error?t="+testRenderToken, []byte(`not json`), nil)
	fx.mu.Lock()
	defer fx.mu.Unlock()
	if len(fx.errors) < 3 {
		t.Fatalf("errors seen = %+v", fx.errors)
	}
	// The oversized frame and the missing header were reported as failures too.
	if fx.errors[len(fx.errors)-2] != (PageError{Code: "code_error", Message: "scene s3 threw", SceneID: "s3"}) {
		t.Errorf("page error = %+v", fx.errors[len(fx.errors)-2])
	}
	if last := fx.errors[len(fx.errors)-1]; last.Code != "render_failed" {
		t.Errorf("unreadable error body mapped to %+v", last)
	}
}

func bootToken(t *testing.T, fx *listenerFixture) string {
	t.Helper()
	res, body := fx.do(t, http.MethodGet, fx.l.Base()+"/boot.json", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("boot.json = %d", res.StatusCode)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	token, _ := doc["token"].(string)
	return token
}

func TestListenerServesTheTokenOnTheFirstGetOnly(t *testing.T) {
	fx := newListenerFixture(t)
	if head, _ := fx.do(t, http.MethodHead, fx.l.Base()+"/boot.json", nil, nil); head.StatusCode != http.StatusOK {
		t.Fatalf("HEAD boot.json = %d", head.StatusCode)
	}
	if got := bootToken(t, fx); got != testRenderToken {
		t.Fatalf("first GET after a HEAD: token = %q", got)
	}
	if got := bootToken(t, fx); got != "" {
		t.Fatalf("second GET: token = %q", got)
	}
}

func TestListenerRefusesARequestWithoutTheToken(t *testing.T) {
	fx := newListenerFixture(t)
	base := fx.l.Base()
	jsonHeader := map[string]string{"Content-Type": "application/json"}
	for _, q := range []string{"", "?t=", "?t=" + strings.Repeat("0", 32), "?t=" + testRenderToken[:31]} {
		res, body := fx.do(t, http.MethodPost, base+"/report"+q, []byte(`{"frame_count":3}`), jsonHeader)
		if res.StatusCode != http.StatusForbidden || string(body) != `{"error":"bad_token"}` {
			t.Errorf("report %q = %d %s", q, res.StatusCode, body)
		}
		res, body = fx.do(t, http.MethodPost, base+"/frame"+q, []byte("f"), map[string]string{"Content-Type": "text/plain", "X-SM-Frame": "0"})
		if res.StatusCode != http.StatusForbidden || string(body) != `{"stop":true,"error":"bad_token"}` {
			t.Errorf("frame %q = %d %s", q, res.StatusCode, body)
		}
		res, body = fx.do(t, http.MethodPost, base+"/error"+q, []byte(`{"code":"code_error","message":"forged"}`), jsonHeader)
		if res.StatusCode != http.StatusForbidden || string(body) != `{"error":"bad_token"}` {
			t.Errorf("error %q = %d %s", q, res.StatusCode, body)
		}
	}
	fx.mu.Lock()
	report, frames, errs := fx.report, len(fx.frames), len(fx.errors)
	fx.mu.Unlock()
	if report != nil || frames != 0 || errs != 0 {
		t.Fatalf("a hook ran: report %q, %d frames, %d errors", report, frames, errs)
	}
	// The one-shot stays free: the page's own report still reaches the hook.
	res, body := fx.do(t, http.MethodPost, base+"/report?t="+testRenderToken, []byte(`{"frame_count":3}`), jsonHeader)
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"start":true`) {
		t.Fatalf("report with the token = %d %s", res.StatusCode, body)
	}
}

func TestListenerKeepsThePrefixAndTheTokenOutOfAPageError(t *testing.T) {
	fx := newListenerFixture(t)
	base := fx.l.Base()
	// Chrome's text for a refused importScripts can name the URL, and the URL holds the prefix.
	sent, err := json.Marshal(PageError{Code: "code_error", Message: "refused " + base + "/scenes/s3.js?t=" + testRenderToken, SceneID: fx.l.prefix})
	if err != nil {
		t.Fatal(err)
	}
	res, body := fx.do(t, http.MethodPost, base+"/error?t="+testRenderToken, sent, map[string]string{"Content-Type": "application/json"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("error endpoint = %d %s", res.StatusCode, body)
	}
	fx.mu.Lock()
	defer fx.mu.Unlock()
	want := PageError{Code: "code_error", Message: "refused " + fx.l.Origin() + "//scenes/s3.js?t="}
	if len(fx.errors) != 1 || fx.errors[0] != want {
		t.Fatalf("errors seen = %+v, want %+v", fx.errors, want)
	}
}

func TestStartListenerRefusesAShortToken(t *testing.T) {
	prefix, err := NewPrefix()
	if err != nil {
		t.Fatal(err)
	}
	params := PageParams{Width: 1920, Height: 1080, FPS: 30, Transport: TransportPNG}
	for _, token := range []string{"", strings.Repeat("a", 31)} {
		params.Token = token
		if l, err := StartListener(prefix, &Snapshot{}, params, Hooks{}); err == nil {
			_ = l.Close()
			t.Errorf("token %q accepted", token)
		}
	}
	params.Token = testRenderToken
	l, err := StartListener(prefix, &Snapshot{}, params, Hooks{})
	if err != nil {
		t.Fatalf("a 32 character token was refused: %v", err)
	}
	_ = l.Close()
}

func TestBootCarriesWorkerFieldsOnlyWhenSet(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	read := func(params PageParams) map[string]any {
		t.Helper()
		prefix, err := NewPrefix()
		if err != nil {
			t.Fatal(err)
		}
		token, err := NewPrefix() // 32 hex characters, the size StartListener asks of a token
		if err != nil {
			t.Fatal(err)
		}
		params.Width, params.Height, params.FPS, params.Transport, params.FrameLimit, params.Token = 1920, 1080, 30, TransportPNG, 27000, token
		l, err := StartListener(prefix, &Snapshot{}, params, Hooks{})
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		res, err := client.Get(l.Base() + "/boot.json")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var doc map[string]any
		if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
			t.Fatal(err)
		}
		return doc
	}
	absent := read(PageParams{})
	for _, key := range []string{"workers", "chunk", "pre_roll"} {
		if _, ok := absent[key]; ok {
			t.Errorf("boot.%s is present without a value: %v", key, absent[key])
		}
	}
	set := read(PageParams{Workers: 4, Chunk: 8, PreRoll: 2})
	if set["workers"] != float64(4) || set["chunk"] != float64(8) || set["pre_roll"] != float64(2) {
		t.Errorf("boot = %v", set)
	}
}

func TestStartListenerRejectsWeakPrefixes(t *testing.T) {
	for _, prefix := range []string{"", "short", strings.Repeat("a/", 20), strings.Repeat("a", 31)} {
		if l, err := StartListener(prefix, &Snapshot{}, PageParams{Token: testRenderToken}, Hooks{}); err == nil {
			_ = l.Close()
			t.Errorf("prefix %q accepted", prefix)
		}
	}
}

func TestListenerServesFallbackFacesWithTheirType(t *testing.T) {
	prefix, err := NewPrefix()
	if err != nil {
		t.Fatal(err)
	}
	snap := &Snapshot{Fonts: []SnapshotFont{
		{Name: "default", Family: "Geist", Weight: 400, File: "/fonts/Geist-Regular.woff2", Data: []byte("woff2")},
		{Name: "sm-deva", Family: "SMDeva", Weight: 400, File: "/fonts/qr/NotoSansDevanagari.ttf", Data: []byte("ttf"), Fallback: true, WeightRange: "100 900"},
		{Name: "sm-cjk", Family: "SMCJK", Weight: 400, File: "/fonts/qr/NotoSansCJKsc.otf", Data: []byte("otf"), Fallback: true, WeightRange: "400", MaxBytes: 17 << 20},
		{Name: "sm-greek", Family: "SMGreek", Weight: 400, File: "/fonts/Greek.woff2", Data: []byte("greek"), Fallback: true, WeightRange: "300 700", UnicodeRange: "U+0370-03FF"},
	}}
	l, err := StartListener(prefix, snap, PageParams{Width: 1920, Height: 1080, FPS: 30, Transport: TransportPNG, Token: testRenderToken}, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	fx := &listenerFixture{l: l, client: &http.Client{Timeout: 5 * time.Second}}
	base := l.Base()

	_, body := fx.do(t, http.MethodGet, base+"/boot.json", nil, nil)
	var doc struct {
		Fonts []bootFont `json:"fonts"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	want := []bootFont{
		{Name: "default", Family: "Geist", Weight: 400, URL: base + "/fonts/Geist-Regular.woff2"},
		{Name: "sm-deva", Family: "SMDeva", Weight: 400, URL: base + "/fonts/qr/NotoSansDevanagari.ttf", Fallback: true, WeightRange: "100 900"},
		{Name: "sm-cjk", Family: "SMCJK", Weight: 400, URL: base + "/fonts/qr/NotoSansCJKsc.otf", Fallback: true, WeightRange: "400", MaxBytes: 17 << 20},
		{Name: "sm-greek", Family: "SMGreek", Weight: 400, URL: base + "/fonts/Greek.woff2", Fallback: true, WeightRange: "300 700", UnicodeRange: "U+0370-03FF"},
	}
	if len(doc.Fonts) != len(want) {
		t.Fatalf("boot.fonts = %+v", doc.Fonts)
	}
	for i := range want {
		if doc.Fonts[i] != want[i] {
			t.Errorf("boot.fonts[%d] = %+v, want %+v", i, doc.Fonts[i], want[i])
		}
	}
	for file, ct := range map[string]string{
		"/fonts/Geist-Regular.woff2":       "font/woff2",
		"/fonts/qr/NotoSansDevanagari.ttf": "font/ttf",
		"/fonts/qr/NotoSansCJKsc.otf":      "font/otf",
	} {
		res, _ := fx.do(t, http.MethodGet, base+file, nil, nil)
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != ct {
			t.Errorf("GET %s = %d %q, want %q", file, res.StatusCode, res.Header.Get("Content-Type"), ct)
		}
	}
}

func TestSnapshotHoldsGlyphsAndFallbackFaces(t *testing.T) {
	ui := os.DirFS(uiDirForTests)
	cat, err := storymaker.LoadCatalog(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	run := &renderRun{r: &Runner{deps: Deps{UI: ui, Catalog: cat}}, plan: &Plan{StoryJSON: []byte(`{}`)}}
	snap, err := run.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.CatalogFiles["glyphs.json"]) == 0 {
		t.Error("glyphs.json is not in the snapshot")
	}
	fallback := map[string]SnapshotFont{}
	for _, f := range snap.Fonts {
		if f.Fallback {
			fallback[f.Name] = f
		}
	}
	// 17 MiB is the one named exception to the 8 MiB cap (frozen section 6, Font caps).
	cjk, ok := fallback["sm-cjk"]
	if !ok || cjk.Family != "SMCJK" || cjk.Weight != 400 || cjk.WeightRange != "400" || cjk.MaxBytes != 17<<20 || cjk.File != "/fonts/qr/NotoSansCJKsc.otf" || len(cjk.Data) == 0 {
		t.Errorf("sm-cjk = %+v (ok %v, %d bytes)", cjk.Name, ok, len(cjk.Data))
	}
	greek, ok := fallback["sm-greek"]
	if !ok || greek.UnicodeRange != "U+0370-03FF" || greek.WeightRange != "300 700" || greek.Weight != 400 {
		t.Errorf("sm-greek = %q %q %d (ok %v)", greek.UnicodeRange, greek.WeightRange, greek.Weight, ok)
	}
	// Geist is a catalog font family, so its faces load through catalog.fonts only.
	if _, dup := fallback["geist"]; dup {
		t.Error("the geist face is listed as a fallback")
	}
}

func TestListenerCloseStopsServing(t *testing.T) {
	fx := newListenerFixture(t)
	url := fx.l.Base() + "/boot.json"
	if res, _ := fx.do(t, http.MethodGet, url, nil, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	_ = fx.l.Close()
	if _, err := fx.client.Get(url); err == nil {
		t.Fatal("the listener still answers after Close")
	}
}

// /start needs the token, answers 404 until a report was accepted, holds at most startHold and then
// answers pending, start or the failure of the start.
func TestListenerDeferredStartRoute(t *testing.T) {
	old := startHold
	startHold = 150 * time.Millisecond
	t.Cleanup(func() { startHold = old })
	const token = "0123456789abcdef0123456789abcdef"
	listen := func(run *renderRun) (*listenerFixture, string) {
		t.Helper()
		prefix, err := NewPrefix()
		if err != nil {
			t.Fatal(err)
		}
		l, err := StartListener(prefix, &Snapshot{}, PageParams{Width: 1920, Height: 1080, FPS: 30, Transport: TransportPNG, FrameLimit: 900, Token: token}, Hooks{Start: run.onStart})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = l.Close() })
		client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
		return &listenerFixture{l: l, client: client}, l.Base() + "/start"
	}

	run := &renderRun{startCh: make(chan struct{})}
	fx, start := listen(run)
	if res, body := fx.do(t, http.MethodGet, start+"?t=wrong", nil, nil); res.StatusCode != http.StatusForbidden || !strings.Contains(string(body), `"error":"bad_token"`) {
		t.Errorf("wrong token = %d %s", res.StatusCode, body)
	}
	if res, _ := fx.do(t, http.MethodPost, start+"?t="+token, nil, nil); res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /start = %d", res.StatusCode)
	}
	if res, _ := fx.do(t, http.MethodGet, start+"?t="+token, nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("/start before a report = %d, want 404", res.StatusCode)
	}

	// A report was accepted and the start still runs: the call holds, then answers pending.
	run.mu.Lock()
	run.frames = 30
	run.mu.Unlock()
	began := time.Now()
	res, body := fx.do(t, http.MethodGet, start+"?t="+token, nil, nil)
	if res.StatusCode != http.StatusOK || string(body) != `{"pending":true}` {
		t.Errorf("held /start = %d %s", res.StatusCode, body)
	}
	if held := time.Since(began); held < startHold {
		t.Errorf("/start answered after %s, it must hold %s", held, startHold)
	}

	// A hold far longer than the client's 5 s timeout: the answer comes when the start is done, not at the end of the hold.
	startHold = 10 * time.Second
	time.AfterFunc(50*time.Millisecond, func() { close(run.startCh) })
	res, body = fx.do(t, http.MethodGet, start+"?t="+token, nil, nil)
	if res.StatusCode != http.StatusOK || string(body) != `{"start":true}` {
		t.Errorf("/start after the start = %d %s", res.StatusCode, body)
	}
	if _, body := fx.do(t, http.MethodGet, start+"?t="+token, nil, nil); string(body) != `{"start":true}` {
		t.Errorf("a repeated /start = %s", body)
	}
	if fx.l.Counts()["GET start"] == 0 {
		t.Errorf("counts = %v, want the route start", fx.l.Counts())
	}

	failed := &renderRun{startCh: make(chan struct{}), frames: 30, startErr: Fail(CodeFFmpegUnavailable, "no encoder")}
	close(failed.startCh)
	fx2, start2 := listen(failed)
	res, body = fx2.do(t, http.MethodGet, start2+"?t="+token, nil, nil)
	if res.StatusCode != http.StatusOK || string(body) != `{"error":"ffmpeg_unavailable","message":"no encoder"}` {
		t.Errorf("failed start = %d %s", res.StatusCode, body)
	}
}
