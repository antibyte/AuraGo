package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"aurago/internal/cydcreator"
)

func (f *cydTestFixture) do(token, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, cydCreatorAPIPrefix+path, body)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.s.handleCYDCreator(w, r)
	return w
}

func (f *cydTestFixture) json(t *testing.T, token, method, path, body string, headers map[string]string, wantStatus int, out any) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	w := f.do(token, method, path, reader, headers)
	if w.Code != wantStatus {
		t.Fatalf("%s %s = %d %s, want %d", method, path, w.Code, w.Body.String(), wantStatus)
	}
	if out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, path, w.Body.String(), err)
		}
	}
}

type cydErrorBody struct {
	Error   string         `json:"error"`
	Code    string         `json:"code"`
	Details map[string]any `json:"details"`
}

func wantCYDError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) cydErrorBody {
	t.Helper()
	var body cydErrorBody
	if w.Code != status || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Code != code || body.Error == "" {
		t.Fatalf("answer = %d %s, want %d %s", w.Code, w.Body.String(), status, code)
	}
	return body
}

func (f *cydTestFixture) createProject(t *testing.T, name string) cydcreator.ProjectSummary {
	t.Helper()
	var out struct {
		Project cydcreator.ProjectSummary `json:"project"`
	}
	f.json(t, f.admin, http.MethodPost, "projects", fmt.Sprintf(`{"name":%q,"board":"esp32-2432s028r","theme":"night"}`, name), nil, http.StatusCreated, &out)
	return out.Project
}

func TestCYDCreatorRoutesEnforceDesktopScopes(t *testing.T) {
	f := newCYDTestServer(t)
	if w := f.do(f.read, http.MethodGet, "capabilities", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("read token capabilities = %d", w.Code)
	}
	for _, token := range []string{f.read, f.write} {
		w := f.do(token, http.MethodPost, "projects", strings.NewReader(`{"name":"X","board":"esp32-2432s028r"}`), nil)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_scope_required") {
			t.Fatalf("non-admin token write = %d %s", w.Code, w.Body.String())
		}
	}
	if w := f.do("", http.MethodGet, "projects", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated read = %d", w.Code)
	}
	p := f.createProject(t, "Scopes")
	wantCYDError(t, f.do(f.read, http.MethodPost, "builds/ccb_missing/decode-backtrace", strings.NewReader(`{"addresses":["0x1"]}`), nil),
		http.StatusNotFound, cydcreator.CodeNotFound)
	if w := f.do(f.read, http.MethodPost, "builds/ccb_missing/cancel", nil, nil); w.Code != http.StatusForbidden {
		t.Fatalf("read token cancel = %d", w.Code)
	}
	if w := f.do(f.read, http.MethodGet, "projects/"+p.ID, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("read token project = %d", w.Code)
	}
}

func TestCYDCreatorBearerScopeMatchesTheRouteTable(t *testing.T) {
	f := newCYDTestServer(t)
	for _, route := range cydCreatorRoutes(f.s) {
		path := route.Pattern
		path = strings.ReplaceAll(path, "{path...}", "src/app/app_main.cpp")
		for _, param := range []string{"{id}", "{asset}", "{name}"} {
			path = strings.ReplaceAll(path, param, "x1")
		}
		want := desktopScopeRead
		if route.Write || route.Stop {
			want = desktopScopeAdmin
		}
		if got := cydCreatorBearerScope(route.Method, cydCreatorAPIPrefix+path); got != want {
			t.Errorf("%s %s: bearer scope %s, want %s", route.Method, route.Pattern, got, want)
		}
		readOK := validRouteBearer(f.s, f.read, cydCreatorAPIPrefix+path, route.Method)
		adminOK := validRouteBearer(f.s, f.admin, cydCreatorAPIPrefix+path, route.Method)
		if !adminOK || readOK != (want == desktopScopeRead) {
			t.Errorf("%s %s: validRouteBearer read=%v admin=%v", route.Method, route.Pattern, readOK, adminOK)
		}
	}
}

func TestCYDCreatorGatesDisabledReadOnlyAndUnknownRoutes(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Gates")
	var build struct {
		Build cydcreator.Build `json:"build"`
	}
	f.json(t, f.admin, http.MethodPost, "projects/"+p.ID+"/builds", `{}`, nil, http.StatusAccepted, &build)
	if _, err := f.svc.WaitBuild(context.Background(), build.Build.ID); err != nil {
		t.Fatal(err)
	}

	wantCYDError(t, f.do(f.read, http.MethodGet, "nothing/here", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)
	wantCYDError(t, f.do(f.read, http.MethodGet, "projects/bad%20id", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)
	wantCYDError(t, f.do(f.admin, http.MethodPut, "projects", nil, nil), http.StatusMethodNotAllowed, cydcreator.CodeBadRequest)

	f.s.Cfg.VirtualDesktop.ReadOnly = true
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects", strings.NewReader(`{"name":"X","board":"esp32-2432s028r"}`), nil),
		http.StatusForbidden, cydcreator.CodeReadOnly)
	for _, path := range []string{"projects", "projects/" + p.ID + "/layout", "builds/" + build.Build.ID + "/artifacts/merged.bin"} {
		if w := f.do(f.read, http.MethodGet, path, nil, nil); w.Code != http.StatusOK {
			t.Errorf("read-only GET %s = %d", path, w.Code)
		}
	}
	if w := f.do(f.read, http.MethodPost, "builds/"+build.Build.ID+"/decode-backtrace", strings.NewReader(`{"addresses":["0x400d1234"]}`), nil); w.Code != http.StatusOK {
		t.Errorf("read-only decode-backtrace = %d %s", w.Code, w.Body.String())
	}
	if w := f.do(f.admin, http.MethodPost, "builds/"+build.Build.ID+"/cancel", nil, nil); w.Code != http.StatusAccepted {
		t.Errorf("read-only cancel = %d %s", w.Code, w.Body.String())
	}
	var caps cydcreator.Capabilities
	f.json(t, f.read, http.MethodGet, "capabilities", "", nil, http.StatusOK, &caps)
	if !caps.ReadOnly || !caps.Enabled {
		t.Fatalf("capabilities enabled=%v readonly=%v", caps.Enabled, caps.ReadOnly)
	}
	f.s.Cfg.VirtualDesktop.ReadOnly = false

	f.s.Cfg.CYDCreator.Enabled = false
	wantCYDError(t, f.do(f.read, http.MethodGet, "capabilities", nil, nil), http.StatusServiceUnavailable, cydcreator.CodeDisabled)
	f.s.Cfg.CYDCreator.Enabled = true
}

func TestCYDCreatorSessionWritesNeedTheSameOrigin(t *testing.T) {
	f := newCYDTestServer(t)
	f.s.Cfg.Auth.Enabled = false
	body := `{"name":"Origin","board":"esp32-2432s028r"}`
	wantCYDError(t, f.do("", http.MethodPost, "projects", strings.NewReader(body), map[string]string{"Origin": "https://evil.example"}),
		http.StatusForbidden, cydcreator.CodeBadRequest)
	if w := f.do("", http.MethodPost, "projects", strings.NewReader(body), map[string]string{"Origin": "http://example.com"}); w.Code != http.StatusCreated {
		t.Fatalf("same-origin session write = %d %s", w.Code, w.Body.String())
	}
}

func TestCYDCreatorProjectAndLayoutAPI(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Wetter")
	if p.Slug != "wetter" || p.Version != 1 {
		t.Fatalf("created = %+v", p)
	}
	var list struct {
		Projects []cydcreator.ProjectSummary `json:"projects"`
	}
	f.json(t, f.read, http.MethodGet, "projects", "", nil, http.StatusOK, &list)
	if len(list.Projects) != 1 || list.Projects[0].ID != p.ID {
		t.Fatalf("list = %+v", list)
	}
	var renamed struct {
		Project cydcreator.ProjectSummary `json:"project"`
	}
	f.json(t, f.admin, http.MethodPatch, "projects/"+p.ID, `{"name":"Wetter 2"}`, nil, http.StatusOK, &renamed)
	if renamed.Project.Name != "Wetter 2" || renamed.Project.Version != 2 {
		t.Fatalf("renamed = %+v", renamed.Project)
	}
	wantCYDError(t, f.do(f.admin, http.MethodPatch, "projects/"+p.ID, strings.NewReader(`{"title":"x"}`), nil), http.StatusBadRequest, cydcreator.CodeBadRequest)

	var layout struct {
		Version int64              `json:"version"`
		Project cydcreator.Project `json:"project"`
		Issues  []cydcreator.Issue `json:"issues"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID+"/layout", "", nil, http.StatusOK, &layout)
	if layout.Version != 2 || layout.Issues == nil || layout.Project.Name != "Wetter 2" {
		t.Fatalf("layout = v%d %+v", layout.Version, layout.Issues)
	}
	payload, _ := json.Marshal(map[string]any{"project": layout.Project})
	wantCYDError(t, f.do(f.admin, http.MethodPut, "projects/"+p.ID+"/layout", bytes.NewReader(payload), nil), http.StatusBadRequest, cydcreator.CodeBadRequest)
	var saved struct {
		Version int64              `json:"version"`
		Issues  []cydcreator.Issue `json:"issues"`
	}
	f.json(t, f.admin, http.MethodPut, "projects/"+p.ID+"/layout", string(payload), map[string]string{"If-Match": `"2"`}, http.StatusOK, &saved)
	if saved.Version != 3 || saved.Issues == nil {
		t.Fatalf("saved = %+v", saved)
	}
	conflict := wantCYDError(t, f.do(f.admin, http.MethodPut, "projects/"+p.ID+"/layout", bytes.NewReader(payload), map[string]string{"If-Match": "2"}),
		http.StatusConflict, cydcreator.CodeProjectConflict)
	if conflict.Details["version"] != float64(3) {
		t.Fatalf("conflict details = %v", conflict.Details)
	}
	wantCYDError(t, f.do(f.admin, http.MethodPut, "projects/"+p.ID+"/layout", strings.NewReader(`{"project":{"format":7}}`), map[string]string{"If-Match": "3"}),
		http.StatusUnprocessableEntity, cydcreator.CodeInvalidLayout)
	wantCYDError(t, f.do(f.admin, http.MethodPut, "projects/"+p.ID+"/layout", strings.NewReader(`{"layout":{}}`), map[string]string{"If-Match": "3"}),
		http.StatusUnprocessableEntity, cydcreator.CodeInvalidLayout)

	if w := f.do(f.admin, http.MethodDelete, "projects/"+p.ID, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	wantCYDError(t, f.do(f.read, http.MethodGet, "projects/"+p.ID, nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)
}

func TestCYDCreatorFileAPI(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Files")
	base := "projects/" + p.ID + "/files"
	var listing struct {
		Files []cydcreator.FileInfo `json:"files"`
	}
	f.json(t, f.read, http.MethodGet, base, "", nil, http.StatusOK, &listing)
	paths := map[string]cydcreator.FileInfo{}
	for _, fi := range listing.Files {
		paths[fi.Path] = fi
	}
	if !paths["src/app/app_main.cpp"].Editable || !paths["src/generated/ui.cpp"].Generated {
		t.Fatalf("listing = %+v", listing.Files)
	}
	w := f.do(f.read, http.MethodGet, base+"/src/generated/ui.cpp", nil, nil)
	if w.Code != http.StatusOK || w.Body.String() != "// Files\n" || w.Header().Get("X-CYD-SHA256") != sha256HexForTest([]byte("// Files\n")) ||
		!strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("generated file = %d %q %v", w.Code, w.Body.String(), w.Header())
	}
	wantCYDError(t, f.do(f.admin, http.MethodPut, base+"/src/app/new.cpp", strings.NewReader("int a;\n"), nil), http.StatusBadRequest, cydcreator.CodeBadRequest)
	var written struct {
		SHA256 string `json:"sha256"`
	}
	f.json(t, f.admin, http.MethodPut, base+"/src/app/new.cpp", "int a;\n", map[string]string{"If-Match": "*"}, http.StatusOK, &written)
	if written.SHA256 != sha256HexForTest([]byte("int a;\n")) {
		t.Fatalf("sha = %q", written.SHA256)
	}
	conflict := wantCYDError(t, f.do(f.admin, http.MethodPut, base+"/src/app/new.cpp", strings.NewReader("int b;\n"), map[string]string{"If-Match": "*"}),
		http.StatusConflict, cydcreator.CodeFileConflict)
	if conflict.Details["sha256"] != written.SHA256 {
		t.Fatalf("conflict details = %v", conflict.Details)
	}
	wantCYDError(t, f.do(f.admin, http.MethodPut, base+"/src/generated/ui.cpp", strings.NewReader("x"), map[string]string{"If-Match": "*"}),
		http.StatusForbidden, cydcreator.CodeFileReadOnly)
	big := strings.Repeat("a", cydcreator.MaxAppFileBytes+1)
	wantCYDError(t, f.do(f.admin, http.MethodPut, base+"/src/app/big.cpp", strings.NewReader(big), map[string]string{"If-Match": "*"}),
		http.StatusRequestEntityTooLarge, cydcreator.CodeFileLimit)
	if w := f.do(f.admin, http.MethodDelete, base+"/src/app/new.cpp", nil, map[string]string{"If-Match": written.SHA256}); w.Code != http.StatusNoContent {
		t.Fatalf("delete file = %d %s", w.Code, w.Body.String())
	}
	if _, err := f.svc.WriteFile(context.Background(), p.ID, "src/app/locked.cpp", []byte("x"), "*", cydcreator.OriginUser); err != nil {
		t.Fatal(err)
	}
	cydLockProjectForTest(t, f, p.ID, "ccj_0001")
	locked := wantCYDError(t, f.do(f.admin, http.MethodPut, base+"/src/app/locked.cpp", strings.NewReader("y"), map[string]string{"If-Match": "*"}),
		http.StatusLocked, cydcreator.CodeProjectLocked)
	if locked.Details["job_id"] != "ccj_0001" {
		t.Fatalf("locked details = %v", locked.Details)
	}
}

func pngForTest(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func multipartBody(t *testing.T, field, name string, data []byte) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = mw.Close()
	return &buf, mw.FormDataContentType()
}

func TestCYDCreatorAssetAPI(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Assets")
	data := pngForTest(t, 16, 16)
	body, contentType := multipartBody(t, "file", "Sun Icon.png", data)
	var uploaded struct {
		Asset   cydcreator.Asset `json:"asset"`
		Version int64            `json:"version"`
	}
	w := f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/assets", body, map[string]string{"Content-Type": contentType})
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &uploaded) != nil {
		t.Fatalf("upload = %d %s", w.Code, w.Body.String())
	}
	if uploaded.Asset.File != "assets/sun_icon.png" || uploaded.Asset.W != 16 || uploaded.Version != 2 {
		t.Fatalf("uploaded = %+v", uploaded)
	}
	got := f.do(f.read, http.MethodGet, "projects/"+p.ID+"/assets/"+uploaded.Asset.ID, nil, nil)
	if got.Code != http.StatusOK || got.Header().Get("Content-Type") != "image/png" || !bytes.Equal(got.Body.Bytes(), data) {
		t.Fatalf("asset = %d %s", got.Code, got.Header())
	}
	big, bigType := multipartBody(t, "file", "big.png", make([]byte, cydcreator.MaxAssetUploadBytes+1))
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/assets", big, map[string]string{"Content-Type": bigType}),
		http.StatusRequestEntityTooLarge, cydcreator.CodeAssetTooLarge)
	other, otherType := multipartBody(t, "image", "x.png", data)
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/assets", other, map[string]string{"Content-Type": otherType}),
		http.StatusBadRequest, cydcreator.CodeBadRequest)
	var deleted struct {
		Version int64 `json:"version"`
	}
	f.json(t, f.admin, http.MethodDelete, "projects/"+p.ID+"/assets/"+uploaded.Asset.ID, "", nil, http.StatusOK, &deleted)
	if deleted.Version != 3 {
		t.Fatalf("version after delete = %d", deleted.Version)
	}
}

func TestCYDCreatorBuildAPI(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Builds")
	var started struct {
		Build cydcreator.Build `json:"build"`
	}
	f.json(t, f.admin, http.MethodPost, "projects/"+p.ID+"/builds", "{}", nil, http.StatusAccepted, &started)
	if started.Build.Status != cydcreator.BuildQueued {
		t.Fatalf("started = %+v", started.Build)
	}
	if _, err := f.svc.WaitBuild(context.Background(), started.Build.ID); err != nil {
		t.Fatal(err)
	}
	var one struct {
		Build cydcreator.Build `json:"build"`
	}
	f.json(t, f.read, http.MethodGet, "builds/"+started.Build.ID, "", nil, http.StatusOK, &one)
	if one.Build.Status != cydcreator.BuildSucceeded || len(one.Build.Artifacts) != 4 {
		t.Fatalf("build = %+v", one.Build)
	}
	var list struct {
		Builds []cydcreator.Build `json:"builds"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID+"/builds", "", nil, http.StatusOK, &list)
	if len(list.Builds) != 1 {
		t.Fatalf("builds = %+v", list.Builds)
	}
	logResp := f.do(f.read, http.MethodGet, "builds/"+started.Build.ID+"/log", nil, nil)
	if logResp.Code != http.StatusOK || !strings.Contains(logResp.Body.String(), "Building esp32-2432s028r") {
		t.Fatalf("log = %d %q", logResp.Code, logResp.Body.String())
	}
	art := f.do(f.read, http.MethodGet, "builds/"+started.Build.ID+"/artifacts/merged.bin", nil, nil)
	if art.Code != http.StatusOK || art.Body.String() != "merged" || art.Header().Get("X-CYD-SHA256") != sha256HexForTest([]byte("merged")) ||
		art.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(art.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("artefact = %d %v", art.Code, art.Header())
	}
	wantCYDError(t, f.do(f.read, http.MethodGet, "builds/"+started.Build.ID+"/artifacts/firmware.elf", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)
	var frames struct {
		Frames []cydcreator.Frame `json:"frames"`
	}
	f.json(t, f.read, http.MethodPost, "builds/"+started.Build.ID+"/decode-backtrace", `{"addresses":["0x400d1234","0x400d0001"]}`, nil, http.StatusOK, &frames)
	if len(frames.Frames) != 2 || frames.Frames[0].Line != 7 {
		t.Fatalf("frames = %+v", frames)
	}
	addrs := make([]string, 65)
	for i := range addrs {
		addrs[i] = "0x1"
	}
	payload, _ := json.Marshal(map[string]any{"addresses": addrs})
	wantCYDError(t, f.do(f.read, http.MethodPost, "builds/"+started.Build.ID+"/decode-backtrace", bytes.NewReader(payload), nil),
		http.StatusBadRequest, cydcreator.CodeBadRequest)
	var cancelled struct {
		Build cydcreator.Build `json:"build"`
	}
	f.json(t, f.admin, http.MethodPost, "builds/"+started.Build.ID+"/cancel", "", nil, http.StatusAccepted, &cancelled)
	if cancelled.Build.Status != cydcreator.BuildSucceeded {
		t.Fatalf("cancel of a finished build changed it: %+v", cancelled.Build)
	}

	f.builder.mu.Lock()
	f.builder.status = cydcreator.ToolchainStatus{DockerAvailable: true, Image: "test:1"}
	f.builder.mu.Unlock()
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/builds", strings.NewReader("{}"), nil), http.StatusConflict, cydcreator.CodeImageMissing)
	var pulled struct {
		Toolchain cydcreator.ToolchainStatus `json:"toolchain"`
	}
	f.json(t, f.admin, http.MethodPost, "toolchain/pull", "", nil, http.StatusAccepted, &pulled)
	if pulled.Toolchain.Image != "test:1" {
		t.Fatalf("pull answer = %+v", pulled)
	}
}

func TestCYDCreatorErrorEnvelope(t *testing.T) {
	f := newCYDTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/desktop/cyd-creator/x", nil)
	w := httptest.NewRecorder()
	f.s.cydError(w, r, errors.New("disk I/O error at /secret/path"))
	body := wantCYDError(t, w, http.StatusInternalServerError, cydcreator.CodeInternal)
	if strings.Contains(body.Error, "/secret/path") {
		t.Fatalf("internal details leaked: %q", body.Error)
	}
	clientOnly := []string{cydcreator.CodeFlashWrongChip, cydcreator.CodeFlashPortBusy, cydcreator.CodeFlashDisconnected, cydcreator.CodeSerialUnsupported}
	for _, code := range cydcreator.ErrorCodes() {
		if _, ok := cydCreatorHTTPStatus[code]; !ok && !slices.Contains(clientOnly, code) {
			t.Errorf("%s has no HTTP status", code)
		}
	}
	w = httptest.NewRecorder()
	f.s.cydError(w, r, fmt.Errorf("wrapped: %w", cydcreator.NewError(cydcreator.CodeProjectLocked, "locked", map[string]any{"job_id": "ccj_1"})))
	if wantCYDError(t, w, http.StatusLocked, cydcreator.CodeProjectLocked).Details["job_id"] != "ccj_1" {
		t.Fatal("details lost")
	}
}

// The tests below pin requirements collected in the plan 2 reviews
// (capabilities overlay, permission model, body caps, envelope details).

// cydTestRoutePath fills a route pattern with valid parameter values.
func cydTestRoutePath(pattern string) string {
	path := strings.ReplaceAll(pattern, "{path...}", "src/app/app_main.cpp")
	for _, param := range []string{"{id}", "{asset}", "{name}"} {
		path = strings.ReplaceAll(path, param, "x1")
	}
	return path
}

// cydTestProjectDir is the on-disk directory of a project of the fixture
// (newCYDTestServer keeps the desktop workspace next to the data directory).
func cydTestProjectDir(f *cydTestFixture, slug string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(f.dbPath)), "desktop", "CYD", slug)
}

func TestCYDCreatorCapabilitiesOverlayConfigSessionAndProblem(t *testing.T) {
	f := newCYDTestServer(t)
	var caps cydcreator.Capabilities
	f.json(t, f.admin, http.MethodGet, "capabilities", "", nil, http.StatusOK, &caps)
	if !caps.Enabled || caps.ReadOnly {
		t.Fatalf("admin capabilities enabled=%v readonly=%v, want true/false", caps.Enabled, caps.ReadOnly)
	}
	caps = cydcreator.Capabilities{}
	f.json(t, f.read, http.MethodGet, "capabilities", "", nil, http.StatusOK, &caps)
	if !caps.ReadOnly {
		t.Fatal("a desktop:read token cannot write, so its capabilities must say readonly")
	}
	f.s.Cfg.Auth.Enabled = false
	caps = cydcreator.Capabilities{}
	f.json(t, "", http.MethodGet, "capabilities", "", nil, http.StatusOK, &caps)
	if caps.ReadOnly {
		t.Fatal("a session on a writable desktop got readonly capabilities")
	}
	f.s.Cfg.VirtualDesktop.ReadOnly = true
	caps = cydcreator.Capabilities{}
	f.json(t, "", http.MethodGet, "capabilities", "", nil, http.StatusOK, &caps)
	if !caps.ReadOnly {
		t.Fatal("a read-only desktop must report readonly capabilities")
	}
	f.s.Cfg.VirtualDesktop.ReadOnly = false
	f.s.Cfg.Auth.Enabled = true

	// Refused or failed at startup: no service, the generic hint goes along.
	svc := f.s.CYDCreator
	f.s.CYDCreator, f.s.cydCreatorProblem = nil, cydCreatorStartFailed
	w := f.do(f.read, http.MethodGet, "capabilities", nil, nil)
	refused := wantCYDError(t, w, http.StatusServiceUnavailable, cydcreator.CodeDisabled)
	if refused.Details["problem"] != cydCreatorStartFailed || refused.Details["restart_required"] != true || strings.Contains(w.Body.String(), filepath.Dir(f.dbPath)) {
		t.Fatalf("failed-start answer = %s", w.Body.String())
	}
	// Switched on at runtime without a restart: say so, not "switched off".
	f.s.cydCreatorProblem = ""
	w = f.do(f.read, http.MethodGet, "capabilities", nil, nil)
	pending := wantCYDError(t, w, http.StatusServiceUnavailable, cydcreator.CodeDisabled)
	if pending.Details["restart_required"] != true || !strings.Contains(pending.Error, "restart") || pending.Details["problem"] != nil {
		t.Fatalf("switched-on answer = %s", w.Body.String())
	}
	// The live configuration would be refused again: the refusal, no restart hint.
	workspace := filepath.Join(t.TempDir(), "virtual_desktop")
	f.s.Cfg.VirtualDesktop.WorkspaceDir = workspace
	f.s.Cfg.SQLite.CYDCreatorPath = filepath.Join(workspace, "cyd.db")
	w = f.do(f.read, http.MethodGet, "capabilities", nil, nil)
	storage := wantCYDError(t, w, http.StatusServiceUnavailable, cydcreator.CodeDisabled)
	if storage.Details["problem"] != "sqlite.cyd_creator_path lies inside the virtual desktop workspace" || storage.Details["restart_required"] != nil ||
		strings.Contains(w.Body.String(), "virtual_desktop"+string(filepath.Separator)) {
		t.Fatalf("refused answer = %s", w.Body.String())
	}
	f.s.Cfg.VirtualDesktop.WorkspaceDir, f.s.Cfg.SQLite.CYDCreatorPath = "", ""
	f.s.CYDCreator, f.s.cydCreatorProblem = svc, ""

	// Switched off: no hint.
	f.s.Cfg.CYDCreator.Enabled = false
	off := wantCYDError(t, f.do(f.read, http.MethodGet, "capabilities", nil, nil), http.StatusServiceUnavailable, cydcreator.CodeDisabled)
	if _, ok := off.Details["problem"]; ok {
		t.Fatalf("switched-off answer carries a problem: %v", off.Details)
	}
}

func TestCYDCreatorEveryMutatingRouteNeedsTheWritePermission(t *testing.T) {
	f := newCYDTestServer(t)
	routes := cydCreatorRoutes(f.s)
	for _, route := range routes {
		mutating := route.Method != http.MethodGet && route.Method != http.MethodHead && route.Pattern != "builds/{id}/decode-backtrace"
		if mutating != (route.Write || route.Stop) {
			t.Errorf("%s %s: mutating=%v but Write=%v Stop=%v", route.Method, route.Pattern, mutating, route.Write, route.Stop)
		}
		if !mutating {
			continue
		}
		for name, token := range map[string]string{"desktop:read": f.read, "desktop:write": f.write} {
			w := f.do(token, route.Method, cydTestRoutePath(route.Pattern), strings.NewReader("{}"), nil)
			if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_scope_required") {
				t.Errorf("%s %s with a %s token = %d %s", route.Method, route.Pattern, name, w.Code, w.Body.String())
			}
		}
	}

	f.s.Cfg.VirtualDesktop.ReadOnly = true
	for _, route := range routes {
		w := f.do(f.admin, route.Method, cydTestRoutePath(route.Pattern), strings.NewReader("{}"), nil)
		switch {
		case route.Write:
			wantCYDError(t, w, http.StatusForbidden, cydcreator.CodeReadOnly)
		case strings.Contains(w.Body.String(), cydcreator.CodeReadOnly) || strings.Contains(w.Body.String(), "desktop_readonly"):
			t.Errorf("%s %s refused on a read-only desktop: %d %s", route.Method, route.Pattern, w.Code, w.Body.String())
		}
	}
	f.s.Cfg.VirtualDesktop.ReadOnly = false

	f.s.Cfg.Auth.Enabled = false
	for _, route := range routes {
		if route.Method == http.MethodGet || route.Method == http.MethodHead {
			continue
		}
		for _, origin := range []string{"https://evil.example", ""} {
			headers := map[string]string{}
			if origin != "" {
				headers["Origin"] = origin
			}
			wantCYDError(t, f.do("", route.Method, cydTestRoutePath(route.Pattern), strings.NewReader("{}"), headers),
				http.StatusForbidden, cydcreator.CodeBadRequest)
		}
	}
}

func TestCYDCreatorJSONBodiesAreCapped(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Caps")
	padded := `{"name":"` + strings.Repeat("a", cydSmallBodyLimit) + `"}`
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "projects"},
		{http.MethodPatch, "projects/" + p.ID},
		{http.MethodPost, "projects/" + p.ID + "/builds"},
		{http.MethodPost, "builds/ccb_x1/decode-backtrace"},
		{http.MethodPost, "builds/ccb_x1/cancel"},
		{http.MethodPost, "toolchain/pull"},
	} {
		wantCYDError(t, f.do(f.admin, route.method, route.path, strings.NewReader(padded), nil), http.StatusRequestEntityTooLarge, cydcreator.CodeBadRequest)
	}
	huge := `{"project":"` + strings.Repeat("a", cydcreator.MaxLayoutBytes+cydLayoutBodySlop) + `"}`
	wantCYDError(t, f.do(f.admin, http.MethodPut, "projects/"+p.ID+"/layout", strings.NewReader(huge), map[string]string{"If-Match": "1"}),
		http.StatusUnprocessableEntity, cydcreator.CodeInvalidLayout)
}

func TestCYDCreatorJSONBodiesHoldOneStrictValue(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Strict")
	for _, body := range []string{`{"name":"A"} {"name":"B"}`, `{"name":"A"}x`, `{"name":"A","title":"x"}`, `{"name":7}`, `[`} {
		refused := wantCYDError(t, f.do(f.admin, http.MethodPatch, "projects/"+p.ID, strings.NewReader(body), nil), http.StatusBadRequest, cydcreator.CodeBadRequest)
		if strings.Contains(refused.Error, "json") || strings.Contains(refused.Error, "Go ") || strings.Contains(refused.Error, "title") {
			t.Errorf("body %q: answer echoes decoder text: %q", body, refused.Error)
		}
	}
	wantCYDError(t, f.do(f.read, http.MethodPost, "builds/ccb_x1/decode-backtrace", strings.NewReader(`{"addresses":[]}{}`), nil),
		http.StatusBadRequest, cydcreator.CodeBadRequest)
	f.json(t, f.admin, http.MethodPatch, "projects/"+p.ID, "{\"name\":\"B\"}\n\t ", nil, http.StatusOK, nil)

	var layout struct {
		Version int64              `json:"version"`
		Project cydcreator.Project `json:"project"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID+"/layout", "", nil, http.StatusOK, &layout)
	payload, _ := json.Marshal(map[string]any{"project": layout.Project})
	ifMatch := map[string]string{"If-Match": fmt.Sprint(layout.Version)}
	wantCYDError(t, f.do(f.admin, http.MethodPut, "projects/"+p.ID+"/layout", strings.NewReader(string(payload)+`{"project":{}}`), ifMatch),
		http.StatusUnprocessableEntity, cydcreator.CodeInvalidLayout)
	f.json(t, f.admin, http.MethodPut, "projects/"+p.ID+"/layout", string(payload)+"\n", ifMatch, http.StatusOK, nil)
}

func TestCYDCreatorMethodNotAllowedListsTheAllowedMethods(t *testing.T) {
	f := newCYDTestServer(t)
	for path, want := range map[string]string{
		"projects":                    "GET, HEAD, POST",
		"projects/x1":                 "DELETE, GET, HEAD, PATCH",
		"builds/x1/cancel":            "POST",
		"projects/x1/files/src/a.cpp": "DELETE, GET, HEAD, PUT",
	} {
		w := f.do(f.admin, http.MethodOptions, path, nil, nil)
		wantCYDError(t, w, http.StatusMethodNotAllowed, cydcreator.CodeBadRequest)
		if got := w.Header().Get("Allow"); got != want {
			t.Errorf("OPTIONS %s: Allow %q, want %q", path, got, want)
		}
	}
}

// cydCountingReader counts the bytes a handler pulled from a request body.
type cydCountingReader struct {
	r io.Reader
	n int64
}

func (c *cydCountingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func TestCYDCreatorAssetUploadIsCappedBeforeReadingAndServedSafely(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Upload")
	huge, hugeType := multipartBody(t, "file", "huge.png", make([]byte, 16<<20))
	counter := &cydCountingReader{r: huge}
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/assets", counter, map[string]string{"Content-Type": hugeType}),
		http.StatusRequestEntityTooLarge, cydcreator.CodeAssetTooLarge)
	if limit := int64(cydcreator.MaxAssetUploadBytes + cydSmallBodyLimit + 64<<10); counter.n > limit {
		t.Fatalf("the upload read %d bytes of a 16 MiB body, want at most %d", counter.n, limit)
	}

	data := pngForTest(t, 8, 8)
	body, contentType := multipartBody(t, "file", "Dot.png", data)
	var uploaded struct {
		Asset cydcreator.Asset `json:"asset"`
	}
	w := f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/assets", body, map[string]string{"Content-Type": contentType})
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &uploaded) != nil {
		t.Fatalf("upload = %d %s", w.Code, w.Body.String())
	}
	got := f.do(f.read, http.MethodGet, "projects/"+p.ID+"/assets/"+uploaded.Asset.ID, nil, nil)
	if got.Code != http.StatusOK || got.Header().Get("X-Content-Type-Options") != "nosniff" || got.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("asset = %d %v", got.Code, got.Header())
	}
	f.json(t, f.admin, http.MethodDelete, "projects/"+p.ID+"/assets/"+uploaded.Asset.ID, "", nil, http.StatusOK, nil)
	if _, err := os.Stat(filepath.Join(cydTestProjectDir(f, p.Slug), filepath.FromSlash(uploaded.Asset.File))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("asset file after delete: %v", err)
	}
	wantCYDError(t, f.do(f.read, http.MethodGet, "projects/"+p.ID+"/assets/", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)
	wantCYDError(t, f.do(f.read, http.MethodGet, "builds/ccb_x1/artifacts/", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)

	var started struct {
		Build cydcreator.Build `json:"build"`
	}
	f.json(t, f.admin, http.MethodPost, "projects/"+p.ID+"/builds", "{}", nil, http.StatusAccepted, &started)
	if _, err := f.svc.WaitBuild(context.Background(), started.Build.ID); err != nil {
		t.Fatal(err)
	}
	for name, wantType := range map[string]string{"merged.bin": "application/octet-stream", "result.json": "application/json", "build.log": "text/plain; charset=utf-8"} {
		art := f.do(f.read, http.MethodGet, "builds/"+started.Build.ID+"/artifacts/"+name, nil, nil)
		if art.Code != http.StatusOK || art.Header().Get("Content-Type") != wantType || art.Header().Get("X-Content-Type-Options") != "nosniff" ||
			!strings.HasPrefix(art.Header().Get("Content-Disposition"), "attachment") {
			t.Errorf("artefact %s = %d %v", name, art.Code, art.Header())
		}
	}
	if w := f.do(f.admin, http.MethodDelete, "projects/"+p.ID, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete project = %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(f.dbPath), "cyd_creator", "artifacts", started.Build.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("artefact directory after delete: %v", err)
	}
}

// cydCloseCounter is an opened asset or artefact that counts Close calls.
type cydCloseCounter struct {
	*bytes.Reader
	closed int
}

func (c *cydCloseCounter) Close() error {
	c.closed++
	return nil
}

func TestCYDCreatorServedReadersAreAlwaysClosed(t *testing.T) {
	for _, tc := range []struct {
		name, method, rangeHeader string
		status                    int
	}{
		{"get", http.MethodGet, "", http.StatusOK},
		{"head", http.MethodHead, "", http.StatusOK},
		{"range", http.MethodGet, "bytes=2-3", http.StatusPartialContent},
		{"unsatisfiable range", http.MethodGet, "bytes=99-", http.StatusRequestedRangeNotSatisfiable},
	} {
		file := &cydCloseCounter{Reader: bytes.NewReader([]byte("merged"))}
		r := httptest.NewRequest(tc.method, cydCreatorAPIPrefix+"builds/ccb_1/artifacts/merged.bin", nil)
		if tc.rangeHeader != "" {
			r.Header.Set("Range", tc.rangeHeader)
		}
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/octet-stream")
		cydServeReader(w, r, "merged.bin", file)
		if w.Code != tc.status || file.closed != 1 {
			t.Errorf("%s: status %d, closed %d times; want %d and once", tc.name, w.Code, file.closed, tc.status)
		}
	}
}

func TestCYDCreatorIfMatchIgnoresQuotesAndLetterCase(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Etags")
	base := "projects/" + p.ID + "/files/src/app/etag.cpp"
	var written struct {
		SHA256 string `json:"sha256"`
	}
	f.json(t, f.admin, http.MethodPut, base, "int a;\n", map[string]string{"If-Match": `"*"`}, http.StatusOK, &written)
	f.json(t, f.admin, http.MethodPut, base, "int b;\n", map[string]string{"If-Match": `"` + strings.ToUpper(written.SHA256) + `"`}, http.StatusOK, &written)
	if w := f.do(f.admin, http.MethodDelete, base, nil, map[string]string{"If-Match": `W/"` + strings.ToUpper(written.SHA256) + `"`}); w.Code != http.StatusNoContent {
		t.Fatalf("delete with a quoted upper-case SHA = %d %s", w.Code, w.Body.String())
	}
}

func TestCYDCreatorEnvelopeCarriesTheServiceDetails(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Details")
	dir := cydTestProjectDir(f, p.Slug)

	// Letter-case clash: CYD_FILE_CONFLICT names the existing file.
	f.json(t, f.admin, http.MethodPut, "projects/"+p.ID+"/files/src/app/Clash.cpp", "int a;\n", map[string]string{"If-Match": "*"}, http.StatusOK, nil)
	clash := wantCYDError(t, f.do(f.admin, http.MethodPut, "projects/"+p.ID+"/files/src/app/clash.cpp", strings.NewReader("int b;\n"), map[string]string{"If-Match": "*"}),
		http.StatusConflict, cydcreator.CodeFileConflict)
	if clash.Details["existing"] != "src/app/Clash.cpp" {
		t.Fatalf("clash details = %v", clash.Details)
	}

	// A layout entry with a broken asset path: CYD_INVALID_LAYOUT names the asset.
	layoutPath := filepath.Join(dir, cydcreator.ProjectFile)
	original, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(original, &doc); err != nil {
		t.Fatal(err)
	}
	doc["assets"] = []any{map[string]any{"id": "a_bad", "file": "../secret.png", "w": 1, "h": 1}}
	broken, _ := json.Marshal(doc)
	if err := os.WriteFile(layoutPath, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	bad := wantCYDError(t, f.do(f.read, http.MethodGet, "projects/"+p.ID+"/assets/a_bad", nil, nil), http.StatusUnprocessableEntity, cydcreator.CodeInvalidLayout)
	if bad.Details["asset"] != "a_bad" {
		t.Fatalf("asset details = %v", bad.Details)
	}

	// A missing project file: CYD_INVALID_LAYOUT with reason "missing".
	if err := os.Remove(layoutPath); err != nil {
		t.Fatal(err)
	}
	missing := wantCYDError(t, f.do(f.read, http.MethodGet, "projects/"+p.ID+"/layout", nil, nil), http.StatusUnprocessableEntity, cydcreator.CodeInvalidLayout)
	if missing.Details["reason"] != "missing" || strings.Contains(missing.Error, dir) {
		t.Fatalf("missing layout = %+v", missing)
	}
	_ = os.WriteFile(layoutPath, original, 0o600)

	r := httptest.NewRequest(http.MethodGet, cydCreatorAPIPrefix+"x", nil)
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{cydcreator.NewError(cydcreator.CodeBusy, "a build of this project is queued or running", nil), http.StatusConflict, cydcreator.CodeBusy},
		{cydcreator.NewError(cydcreator.CodeImageMissing, "pull the toolchain first", nil), http.StatusConflict, cydcreator.CodeImageMissing},
		{cydcreator.NewError(cydcreator.CodeAssetTooLarge, "too large", map[string]any{"asset": "a_1", "reason": "file_size"}), http.StatusRequestEntityTooLarge, cydcreator.CodeAssetTooLarge},
	} {
		w := httptest.NewRecorder()
		f.s.cydError(w, r, tc.err)
		wantCYDError(t, w, tc.status, tc.code)
	}

	// A typed CYD_INTERNAL keeps a generic message and drops its details.
	w := httptest.NewRecorder()
	f.s.cydError(w, r, cydcreator.NewError(cydcreator.CodeInternal, "open C:/data/x.db failed", map[string]any{"path": "C:/data/x.db"}))
	internal := wantCYDError(t, w, http.StatusInternalServerError, cydcreator.CodeInternal)
	if strings.Contains(w.Body.String(), "x.db") || len(internal.Details) != 0 {
		t.Fatalf("typed internal error leaked: %s", w.Body.String())
	}
}

// cydTestDB opens a second connection to the fixture database (tests only).
func cydTestDB(t *testing.T, f *cydTestFixture) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", f.dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
