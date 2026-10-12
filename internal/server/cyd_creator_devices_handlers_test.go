package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/cydcreator"
)

// cydDeviceFixture is a CYD Creator test server whose devices reach AuraGo at
// http://192.168.1.5:8088, with one project and one succeeded build.
type cydDeviceFixture struct {
	*cydTestFixture
	project cydcreator.ProjectSummary
	build   cydcreator.Build
}

func newCYDDeviceFixture(t *testing.T) *cydDeviceFixture {
	t.Helper()
	f := newCYDTestServer(t)
	f.s.Cfg.Server.Host, f.s.Cfg.Server.Port = "192.168.1.5", 8088
	p := f.createProject(t, "Wetter")
	return &cydDeviceFixture{cydTestFixture: f, project: p, build: f.succeededBuild(t, p.ID)}
}

func (f *cydTestFixture) succeededBuild(t *testing.T, projectID string) cydcreator.Build {
	t.Helper()
	var out struct {
		Build cydcreator.Build `json:"build"`
	}
	f.json(t, f.admin, http.MethodPost, "projects/"+projectID+"/builds", `{}`, nil, http.StatusAccepted, &out)
	b, err := f.svc.WaitBuild(context.Background(), out.Build.ID)
	if err != nil || b.Status != cydcreator.BuildSucceeded {
		t.Fatalf("build = %+v, %v", b, err)
	}
	return b
}

// mint calls POST /projects/{id}/devices and returns the answer and the token.
func (f *cydDeviceFixture) mint(t *testing.T) (cydcreator.MintedDevice, cydcreator.DeviceConfig) {
	t.Helper()
	var minted cydcreator.MintedDevice
	f.json(t, f.admin, http.MethodPost, "projects/"+f.project.ID+"/devices", fmt.Sprintf(`{"build_id":%q}`, f.build.ID), nil, http.StatusCreated, &minted)
	cfg, err := cydcreator.DecodeDeviceConfig(minted.ConfigBlob)
	if err != nil {
		t.Fatalf("config blob: %v", err)
	}
	return minted, cfg
}

func (f *cydTestFixture) device(token, method, path, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, cydCreatorDevicePrefix+path, reader)
	r.RemoteAddr = "192.168.1.77:40000"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.s.handleCYDCreatorDevice(w, r)
	return w
}

func heartbeatBody(deviceID, buildID string) string {
	return fmt.Sprintf(`{"device_id":%q,"build_id":%q,"template_version":"1.0.0","uptime_s":5,"ip":"192.168.1.77","rssi":-60,"free_heap":90000,"reset_reason":"poweron","ota_state":"idle","ota_error":""}`, deviceID, buildID)
}

func TestCYDCreatorDeviceDesktopRoutes(t *testing.T) {
	f := newCYDDeviceFixture(t)
	minted, cfg := f.mint(t)
	if minted.ConfigOffset != 3997696 || len(minted.ConfigBlob) != cydcreator.ConfigSize || minted.Device.Status != cydcreator.DevicePending {
		t.Fatalf("minted = %+v", minted.Device)
	}
	if cfg.URL != "http://192.168.1.5:8088" || cfg.CertSHA256 != "" || cfg.DeviceID != minted.Device.ID || !strings.HasPrefix(cfg.Token, "ccdt_") {
		t.Fatalf("config = %+v", cfg)
	}

	var list struct {
		Devices []cydcreator.Device `json:"devices"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+f.project.ID+"/devices", "", nil, http.StatusOK, &list)
	if len(list.Devices) != 1 || list.Devices[0].ID != minted.Device.ID {
		t.Fatalf("list = %+v", list.Devices)
	}
	if strings.Contains(f.do(f.read, http.MethodGet, "projects/"+f.project.ID+"/devices", nil, nil).Body.String(), cfg.Token) {
		t.Fatal("the device list leaks the token")
	}
	for _, token := range []string{f.read, f.write} {
		w := f.do(token, http.MethodPost, "projects/"+f.project.ID+"/devices", strings.NewReader(`{"build_id":"`+f.build.ID+`"}`), nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("mint with a non-admin token = %d %s", w.Code, w.Body.String())
		}
	}

	var updated struct {
		Device cydcreator.Device `json:"device"`
	}
	f.json(t, f.admin, http.MethodPatch, "devices/"+minted.Device.ID, `{"name":"Flur"}`, nil, http.StatusOK, &updated)
	if updated.Device.Name != "Flur" {
		t.Fatalf("rename = %+v", updated.Device)
	}
	wantCYDError(t, f.do(f.admin, http.MethodPatch, "devices/"+minted.Device.ID, strings.NewReader(`{"color":"red"}`), nil),
		http.StatusBadRequest, cydcreator.CodeBadRequest)

	var caps cydcreator.Capabilities
	f.json(t, f.read, http.MethodGet, "capabilities", "", nil, http.StatusOK, &caps)
	if caps.OTA.PublicURL != "http://192.168.1.5:8088" || !caps.OTA.Insecure || !caps.OTA.Enabled {
		t.Fatalf("capabilities ota = %+v", caps.OTA)
	}

	f.s.Cfg.VirtualDesktop.ReadOnly = true
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+f.project.ID+"/devices", strings.NewReader(`{"build_id":"`+f.build.ID+`"}`), nil),
		http.StatusForbidden, cydcreator.CodeReadOnly)
	wantCYDError(t, f.do(f.admin, http.MethodDelete, "devices/"+minted.Device.ID, nil, nil), http.StatusForbidden, cydcreator.CodeReadOnly)
	f.json(t, f.read, http.MethodGet, "projects/"+f.project.ID+"/devices", "", nil, http.StatusOK, &list)
	f.s.Cfg.VirtualDesktop.ReadOnly = false

	if w := f.do(f.admin, http.MethodDelete, "devices/"+minted.Device.ID, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d %s", w.Code, w.Body.String())
	}
	wantCYDError(t, f.do(f.admin, http.MethodDelete, "devices/"+minted.Device.ID, nil, nil), http.StatusNotFound, cydcreator.CodeDeviceNotFound)

	limits := f.svc.Limits()
	limits.OTA.Enabled = false
	f.svc.UpdateLimits(limits)
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+f.project.ID+"/devices", strings.NewReader(`{"build_id":"`+f.build.ID+`"}`), nil),
		http.StatusServiceUnavailable, cydcreator.CodeOTADisabled)
}

func TestCYDCreatorDeviceEndpointRoutes(t *testing.T) {
	f := newCYDDeviceFixture(t)
	minted, cfg := f.mint(t)
	id := minted.Device.ID

	w := f.device(cfg.Token, http.MethodPost, "heartbeat", heartbeatBody(id, f.build.ID))
	var hb cydcreator.HeartbeatResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &hb) != nil || hb.IntervalS != 60 || hb.Update != nil {
		t.Fatalf("heartbeat = %d %s", w.Code, w.Body.String())
	}
	second := f.succeededBuild(t, f.project.ID)
	f.json(t, f.admin, http.MethodPatch, "devices/"+id, fmt.Sprintf(`{"target_build_id":%q}`, second.ID), nil, http.StatusOK, nil)
	w = f.device(cfg.Token, http.MethodPost, "heartbeat", heartbeatBody(id, f.build.ID))
	if err := json.Unmarshal(w.Body.Bytes(), &hb); err != nil || hb.Update == nil ||
		hb.Update.URL != "/api/cyd-creator/device/firmware/"+second.ID {
		t.Fatalf("update offer = %d %s", w.Code, w.Body.String())
	}

	w = f.device(cfg.Token, http.MethodGet, "firmware/"+second.ID, "")
	if w.Code != http.StatusOK || w.Body.String() != "firmware" || w.Header().Get("Content-Length") != fmt.Sprint(hb.Update.Size) ||
		w.Header().Get("X-CYD-SHA256") != hb.Update.SHA256 || w.Header().Get("Content-Type") != "application/octet-stream" ||
		!strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("firmware = %d %q %v", w.Code, w.Body.String(), w.Header())
	}
	wantCYDError(t, f.device(cfg.Token, http.MethodGet, "firmware/"+f.build.ID, ""), http.StatusNotFound, cydcreator.CodeNotFound)
	f.device(cfg.Token, http.MethodGet, "firmware/"+second.ID, "")
	f.device(cfg.Token, http.MethodGet, "firmware/"+second.ID, "")
	wantCYDError(t, f.device(cfg.Token, http.MethodGet, "firmware/"+second.ID, ""), http.StatusTooManyRequests, cydcreator.CodeRateLimited)

	if w := f.device(cfg.Token, http.MethodPost, "ota-result", fmt.Sprintf(`{"build_id":%q,"ok":false,"error":"timeout"}`, second.ID)); w.Code != http.StatusNoContent {
		t.Fatalf("ota-result = %d %s", w.Code, w.Body.String())
	}
	wantCYDError(t, f.device("", http.MethodPost, "heartbeat", heartbeatBody(id, f.build.ID)), http.StatusUnauthorized, cydcreator.CodeUnauthorized)
	wantCYDError(t, f.device(f.admin, http.MethodPost, "heartbeat", heartbeatBody(id, f.build.ID)), http.StatusUnauthorized, cydcreator.CodeUnauthorized)
	wantCYDError(t, f.device(cfg.Token, http.MethodPost, "heartbeat", `{"device_id":"x","bogus":1}`), http.StatusBadRequest, cydcreator.CodeBadRequest)
	wantCYDError(t, f.device(cfg.Token, http.MethodGet, "heartbeat", ""), http.StatusMethodNotAllowed, cydcreator.CodeBadRequest)
	wantCYDError(t, f.device(cfg.Token, http.MethodGet, "nothing", ""), http.StatusNotFound, cydcreator.CodeNotFound)

	limits := f.svc.Limits()
	limits.OTA.Enabled = false
	f.svc.UpdateLimits(limits)
	wantCYDError(t, f.device(cfg.Token, http.MethodPost, "heartbeat", heartbeatBody(id, f.build.ID)), http.StatusServiceUnavailable, cydcreator.CodeOTADisabled)
	f.s.Cfg.CYDCreator.Enabled = false
	wantCYDError(t, f.device(cfg.Token, http.MethodPost, "heartbeat", heartbeatBody(id, f.build.ID)), http.StatusServiceUnavailable, cydcreator.CodeDisabled)
}

// cydUnreadBody is a request body that must never be read.
type cydUnreadBody struct {
	t    *testing.T
	name string
}

func (b cydUnreadBody) Read([]byte) (int, error) {
	b.t.Errorf("%s: the request body was read", b.name)
	return 0, io.EOF
}

// TestCYDCreatorDeviceRoutesRefuseMissingTokensBeforeTheBody pins the device
// gate: /api/cyd-creator/device/ bypasses session auth, so every device route
// answers 401 to a request without a well-formed device bearer token (none,
// API tokens of every desktop scope, a session cookie, malformed tokens)
// before the route is dispatched or the body is read.
func TestCYDCreatorDeviceRoutesRefuseMissingTokensBeforeTheBody(t *testing.T) {
	f := newCYDDeviceFixture(t)
	minted, cfg := f.mint(t)
	mux := http.NewServeMux()
	registerCYDCreatorRoutes(mux, f.s)
	middleware := authMiddleware(f.s, mux)
	hex64 := strings.Repeat("ab", 32)
	session := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(f.s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))}
	credentials := map[string]func(*http.Request){
		"no credentials":       func(*http.Request) {},
		"desktop read token":   func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+f.read) },
		"desktop write token":  func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+f.write) },
		"desktop admin token":  func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+f.admin) },
		"desktop session":      func(r *http.Request) { r.AddCookie(session) },
		"short device token":   func(r *http.Request) { r.Header.Set("Authorization", "Bearer ccdt_"+hex64[:63]) },
		"long device token":    func(r *http.Request) { r.Header.Set("Authorization", "Bearer ccdt_"+hex64+"a") },
		"upper-case hex":       func(r *http.Request) { r.Header.Set("Authorization", "Bearer ccdt_"+strings.ToUpper(hex64)) },
		"other prefix":         func(r *http.Request) { r.Header.Set("Authorization", "Bearer ccdx_"+hex64) },
		"prefix only":          func(r *http.Request) { r.Header.Set("Authorization", "Bearer ccdt_") },
		"two credentials":      func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+cfg.Token+" "+cfg.Token) },
		"basic scheme":         func(r *http.Request) { r.Header.Set("Authorization", "Basic "+cfg.Token) },
		"token in a cookie":    func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "token", Value: cfg.Token}) },
		"token in the query":   func(r *http.Request) { r.URL.RawQuery = "token=" + cfg.Token },
		"token without scheme": func(r *http.Request) { r.Header.Set("Authorization", cfg.Token) },
	}
	routes := cydCreatorDeviceRoutes(f.s)
	if len(routes) == 0 {
		t.Fatal("no device routes")
	}
	for _, route := range routes {
		path := cydCreatorDevicePrefix + strings.ReplaceAll(route.Pattern, "{id}", f.build.ID)
		for name, apply := range credentials {
			for _, via := range []struct {
				name    string
				handler http.Handler
			}{{"handler", http.HandlerFunc(f.s.handleCYDCreatorDevice)}, {"auth middleware", middleware}} {
				label := fmt.Sprintf("%s %s, %s, %s", route.Method, route.Pattern, name, via.name)
				r := httptest.NewRequest(route.Method, path, nil)
				r.Body = io.NopCloser(cydUnreadBody{t: t, name: label})
				r.RemoteAddr = "192.168.1.77:40000"
				apply(r)
				w := httptest.NewRecorder()
				via.handler.ServeHTTP(w, r)
				var body cydErrorBody
				if w.Code != http.StatusUnauthorized || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Code != cydcreator.CodeUnauthorized {
					t.Errorf("%s = %d %s, want 401 %s", label, w.Code, w.Body.String(), cydcreator.CodeUnauthorized)
				}
			}
		}
	}
	// The gate passes the real token; the device still works afterwards.
	if w := f.device(cfg.Token, http.MethodPost, "heartbeat", heartbeatBody(minted.Device.ID, f.build.ID)); w.Code != http.StatusOK {
		t.Fatalf("heartbeat with the device token = %d %s", w.Code, w.Body.String())
	}
}

func TestCYDCreatorDeviceTokensStayInTheirLane(t *testing.T) {
	f := newCYDDeviceFixture(t)
	_, cfg := f.mint(t)
	if w := f.do(cfg.Token, http.MethodGet, "projects", nil, nil); w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Fatalf("device token on the desktop API = %d", w.Code)
	}
	if validRouteBearer(f.s, cfg.Token, cydCreatorAPIPrefix+"projects", http.MethodGet) {
		t.Fatal("a device token must not pass validRouteBearer")
	}
	if _, ok := f.s.TokenManager.Validate(cfg.Token, ""); ok {
		t.Fatal("device tokens must not live in the API token store")
	}
	if err := validateTokenScopes([]string{cydcreator.TokenScopeDevice}); err == nil || !strings.Contains(err.Error(), "minted by CYD Creator") {
		t.Fatalf("POST /api/tokens with the device scope: %v", err)
	}
	if err := validateTokenScopes([]string{desktopScopeRead, cydcreator.TokenScopeDevice}); err == nil {
		t.Fatal("the device scope must not be mixed into an API token")
	}
}

func TestCYDCreatorDeviceEndpointsBypassOnlyTheirSessionAuth(t *testing.T) {
	f := newCYDDeviceFixture(t)
	minted, cfg := f.mint(t)
	mux := http.NewServeMux()
	registerCYDCreatorRoutes(mux, f.s)
	handler := authMiddleware(f.s, mux)
	serve := func(method, path, token, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "192.168.1.77:40000"
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := serve(http.MethodPost, "/api/cyd-creator/device/heartbeat", cfg.Token, heartbeatBody(minted.Device.ID, f.build.ID)); w.Code != http.StatusOK {
		t.Fatalf("heartbeat through the auth middleware = %d %s", w.Code, w.Body.String())
	}
	wantCYDError(t, serve(http.MethodPost, "/api/cyd-creator/device/heartbeat", "", "{}"), http.StatusUnauthorized, cydcreator.CodeUnauthorized)
	if w := serve(http.MethodGet, "/api/desktop/cyd-creator/projects", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("desktop API without a session = %d", w.Code)
	}
	if w := serve(http.MethodGet, "/api/desktop/cyd-creator/projects", cfg.Token, ""); w.Code != http.StatusForbidden {
		t.Fatalf("desktop API with a device token through the auth middleware = %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/cyd-creator/device/heartbeat", "/api/cyd-creator/device/firmware/ccb_1", "/api/cyd-creator/device/ota-result"} {
		if !isAuthBypassed(path) {
			t.Errorf("%s must bypass the session check", path)
		}
	}
	for _, path := range []string{"/api/cyd-creator/device", "/api/cyd-creator/devicex", "/api/cyd-creator/", "/api/desktop/cyd-creator/projects", "/api/cyd-creator/devices/x"} {
		if isAuthBypassed(path) {
			t.Errorf("%s must keep the session check", path)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/api/cyd-creator/device/firmware/ccb_1", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	if gzipRequestEligible(r) {
		t.Fatal("device downloads must stay uncompressed with Content-Length")
	}
}

func TestCYDCreatorExportAndImportRoutes(t *testing.T) {
	f := newCYDDeviceFixture(t)
	w := f.do(f.read, http.MethodGet, "projects/"+f.project.ID+"/export", nil, nil)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/zip" ||
		w.Header().Get("Content-Disposition") != `attachment; filename=wetter.zip` {
		t.Fatalf("export = %d %v", w.Code, w.Header())
	}
	archive := w.Body.Bytes()
	if _, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive))); err != nil {
		t.Fatalf("export is not a zip: %v", err)
	}

	upload := func(token string, data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, _ := mw.CreateFormFile("file", "wetter.zip")
		_, _ = part.Write(data)
		_ = mw.Close()
		return f.do(token, http.MethodPost, "import", &body, map[string]string{"Content-Type": mw.FormDataContentType()})
	}
	var imported struct {
		Project cydcreator.ProjectSummary `json:"project"`
	}
	w = upload(f.admin, archive)
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &imported) != nil || imported.Project.Slug != "wetter-2" {
		t.Fatalf("import = %d %s", w.Code, w.Body.String())
	}
	body := wantCYDError(t, upload(f.admin, []byte("not a zip")), http.StatusUnprocessableEntity, cydcreator.CodeImportInvalid)
	if body.Details["reason"] != "not_zip" {
		t.Fatalf("details = %v", body.Details)
	}
	if w := upload(f.read, archive); w.Code != http.StatusForbidden {
		t.Fatalf("import with a read token = %d", w.Code)
	}
	f.s.Cfg.VirtualDesktop.ReadOnly = true
	wantCYDError(t, upload(f.admin, archive), http.StatusForbidden, cydcreator.CodeReadOnly)
	if w := f.do(f.read, http.MethodGet, "projects/"+f.project.ID+"/export", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("export while read-only = %d", w.Code)
	}
}
