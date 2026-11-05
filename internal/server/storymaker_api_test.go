package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

// storymakerTestFFmpeg finds a usable ffmpeg for the render pre-checks (it is not started for a render).
func storymakerTestFFmpeg(t *testing.T) string {
	t.Helper()
	exe := "ffmpeg"
	if runtime.GOOS == "windows" {
		exe = "ffmpeg.exe"
	}
	if dir := os.Getenv("AURAGO_TEST_FFMPEG_DIR"); dir != "" {
		if p := filepath.Join(dir, exe); fileReadable(p) {
			return p
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		if m, _ := filepath.Glob(filepath.Join(local, "Microsoft", "WinGet", "Packages", "*FFmpeg*", "*", "bin", exe)); len(m) > 0 {
			return m[0]
		}
	}
	t.Skip("ffmpeg is not available (set AURAGO_TEST_FFMPEG_DIR)")
	return ""
}

func fileReadable(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular()
}

func storymakerAPIFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, readToken, _ := testDesktopPermissionServer(t)
	// The token of the app can read and write; the read token of the fixture only reads.
	writeToken, _, err := s.TokenManager.Create("storymaker rw", []string{desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopAppsHandlerTest(t, s)
	s.Logger = slog.Default()
	s.Cfg.Storymaker.Enabled = true
	if err := config.NormalizeStorymakerConfig(&s.Cfg.Storymaker); err != nil {
		t.Fatal(err)
	}
	s.storymakerUI = os.DirFS("../../ui")
	s.storymakerBrowsers = render.NewBrowsersWithProbe(t.TempDir(),
		func() (string, bool) { return "/fake/chrome", true },
		func(context.Context, string) (string, error) { return "FakeChrome/1", nil })
	t.Cleanup(s.closeStorymaker)
	return s, readToken, writeToken
}

func storymakerRequest(s *Server, token, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, storymakerAPIBase+path, body)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handleStorymaker(s).ServeHTTP(w, r)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, w.Body.String())
	}
}

type apiProject struct {
	Project storymaker.Project `json:"project"`
	Story   storymaker.Story   `json:"story"`
	ETag    string             `json:"etag"`
}

func createStorymakerProject(t *testing.T, s *Server, token string) apiProject {
	t.Helper()
	w := storymakerRequest(s, token, http.MethodPost, "/projects", strings.NewReader(`{"title":"Was ist eine Firewall?"}`), nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var p apiProject
	decodeBody(t, w, &p)
	if w.Header().Get("ETag") != p.ETag || !storymaker.ValidProjectID(p.Project.ID) {
		t.Fatalf("create answer: %+v header %q", p, w.Header().Get("ETag"))
	}
	return p
}

func TestStorymakerGatesDisabledScopeAndReadOnly(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)

	// No credentials, wrong scope.
	if w := storymakerRequest(s, "", http.MethodGet, "/projects", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous GET /projects = %d", w.Code)
	}
	if w := storymakerRequest(s, readToken, http.MethodPost, "/projects", strings.NewReader(`{"title":"x"}`), nil); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_scope_required") {
		t.Fatalf("read token POST = %d %s", w.Code, w.Body.String())
	}

	// Read-only: mutations refuse with the shared middleware's code, reads work.
	project := createStorymakerProject(t, s, writeToken)
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/projects", `{"title":"y"}`},
		{http.MethodDelete, "/projects/" + project.Project.ID, ""},
		{http.MethodPut, "/projects/" + project.Project.ID + "/story", "{}"},
		{http.MethodPut, "/projects/" + project.Project.ID + "/scenes/s1/code", "x"},
		{http.MethodPost, "/projects/" + project.Project.ID + "/render", ""},
	} {
		w := storymakerRequest(s, writeToken, tc.method, tc.path, strings.NewReader(tc.body), map[string]string{"If-Match": project.ETag, "If-None-Match": "*"})
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_readonly") {
			t.Errorf("read-only %s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+project.Project.ID, nil, nil); w.Code != http.StatusOK {
		t.Fatalf("read-only GET project = %d %s", w.Code, w.Body.String())
	}
	var st struct {
		ReadOnly bool `json:"read_only"`
	}
	w := storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	decodeBody(t, w, &st)
	if !st.ReadOnly {
		t.Errorf("status read_only = false: %s", w.Body.String())
	}
	ro.VirtualDesktop.ReadOnly = false
	s.replaceConfigSnapshot(&ro)

	// Disabled: every endpoint refuses with storymaker_disabled, the status says so.
	off := *s.ConfigSnapshot()
	off.Storymaker.Enabled = false
	s.replaceConfigSnapshot(&off)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/projects"}, {http.MethodPost, "/projects"}, {http.MethodGet, "/projects/" + project.Project.ID},
		{http.MethodGet, "/player"}, {http.MethodGet, "/jobs/" + project.Project.ID}, {http.MethodGet, "/projects/" + project.Project.ID + "/events"},
	} {
		w := storymakerRequest(s, writeToken, tc.method, tc.path, strings.NewReader("{}"), nil)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), `"code":"storymaker_disabled"`) {
			t.Errorf("disabled %s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	w = storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Errorf("disabled status = %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerCreateWithLanguage(t *testing.T) {
	t.Setenv("AURAGO_PHONON_DIR", t.TempDir())
	s, readToken, writeToken := storymakerAPIFixture(t)
	s.Cfg.TTS.Provider = "sanotts"

	// sanotts is the active provider, TTSReady says yes for it and it speaks de.
	w := storymakerRequest(s, writeToken, http.MethodPost, "/projects", strings.NewReader(`{"title":"Mit Stimme","language":"de"}`), nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create with language = %d %s", w.Code, w.Body.String())
	}
	var created apiProject
	decodeBody(t, w, &created)
	if created.Story.Voice == nil || created.Story.Voice.Provider != "sanotts" || storymaker.LanguageBase(created.Story.Voice.Language) != "de" {
		t.Fatalf("resolved voice = %+v", created.Story.Voice)
	}

	// Without a language the voice stays null, as in phase 1.
	w = storymakerRequest(s, writeToken, http.MethodPost, "/projects", strings.NewReader(`{"title":"Ohne Stimme"}`), nil)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"voice":null`) {
		t.Fatalf("create without language = %d %s", w.Code, w.Body.String())
	}

	// Nothing ready speaks hi: sanotts does not list it, phonon has no files in this folder, piper,
	// supertonic and speech_lab are off and the cloud providers have no key.
	w = storymakerRequest(s, writeToken, http.MethodPost, "/projects", strings.NewReader(`{"title":"Hindi","language":"hi"}`), nil)
	var refused struct {
		Code      string   `json:"code"`
		Languages []string `json:"languages"`
	}
	decodeBody(t, w, &refused)
	if w.Code != http.StatusUnprocessableEntity || refused.Code != "narration_unavailable" {
		t.Fatalf("create in hi = %d %s", w.Code, w.Body.String())
	}
	hasGerman := false
	for _, language := range refused.Languages {
		if language == "*" || language == "hi" {
			t.Errorf("languages hold %q: %v", language, refused.Languages)
		}
		hasGerman = hasGerman || language == "de"
	}
	if !hasGerman {
		t.Errorf("languages miss de, which sanotts speaks: %v", refused.Languages)
	}

	// The refused create left nothing behind: only the first two projects exist.
	var list struct {
		Projects []storymaker.Project `json:"projects"`
	}
	decodeBody(t, storymakerRequest(s, readToken, http.MethodGet, "/projects", nil, nil), &list)
	if len(list.Projects) != 2 {
		t.Errorf("projects after the refusal = %d", len(list.Projects))
	}
}

func TestStorymakerStatusShowsNarrationLimits(t *testing.T) {
	s, readToken, _ := storymakerAPIFixture(t)
	w := storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	var st struct {
		Limits map[string]float64 `json:"limits"`
	}
	decodeBody(t, w, &st)
	// 128 is the narrate queue of newStorymakerRuntime. 100000 and 20000 are the defaults of
	// StorymakerNarrationConfig, which the fixture leaves unchanged.
	for key, want := range map[string]float64{"max_queued_narrations": 128, "max_daily_chars": 100000, "max_job_chars": 20000} {
		if st.Limits[key] != want {
			t.Errorf("limits.%s = %v, want %v", key, st.Limits[key], want)
		}
	}
}

func TestStorymakerProjectCRUD(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	if p.Project.Aspect != "16:9" || p.Project.FPS != 30 || len(p.Story.Scenes) != 1 || p.Story.Scenes[0].Environment == "" {
		t.Fatalf("created project = %+v", p)
	}
	// Invalid create bodies.
	for _, body := range []string{`{}`, `{"title":"x","aspect":"4:3"}`, `{"title":"x","fps":60}`, `{"title":"x","extra":1}`, `nope`} {
		if w := storymakerRequest(s, writeToken, http.MethodPost, "/projects", strings.NewReader(body), nil); w.Code != http.StatusBadRequest {
			t.Errorf("create %s = %d %s", body, w.Code, w.Body.String())
		}
	}
	other := storymakerRequest(s, writeToken, http.MethodPost, "/projects", strings.NewReader(`{"title":"Vertical","aspect":"9:16","fps":25}`), nil)
	if other.Code != http.StatusCreated {
		t.Fatalf("vertical create = %d %s", other.Code, other.Body.String())
	}

	w := storymakerRequest(s, readToken, http.MethodGet, "/projects", nil, nil)
	var list struct {
		Projects []storymaker.Project `json:"projects"`
	}
	decodeBody(t, w, &list)
	if w.Code != http.StatusOK || len(list.Projects) != 2 {
		t.Fatalf("list = %d %s", w.Code, w.Body.String())
	}

	w = storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil)
	var got struct {
		Project    storymaker.Project           `json:"project"`
		Story      *storymaker.Story            `json:"story"`
		ETag       string                       `json:"etag"`
		Issues     []storymaker.Issue           `json:"issues"`
		CodeScenes []storymaker.CodeSceneStatus `json:"code_scenes"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusOK || got.ETag != p.ETag || got.Story == nil || got.Project.ID != p.Project.ID || got.CodeScenes == nil || got.Issues == nil {
		t.Fatalf("get = %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("ETag") != p.ETag {
		t.Errorf("ETag header = %q", w.Header().Get("ETag"))
	}

	// Unknown and malformed ids.
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/33333333-3333-4333-8333-333333333333", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown project = %d", w.Code)
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/..%2f..", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("bad id = %d", w.Code)
	}

	if w := storymakerRequest(s, writeToken, http.MethodDelete, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("deleted project GET = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodDelete, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d", w.Code)
	}
}

func TestStorymakerStoryPutETagAndValidation(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	path := "/projects/" + p.Project.ID + "/story"
	story, err := os.ReadFile("../storymaker/testdata/stories/firewall.json")
	if err != nil {
		t.Fatal(err)
	}
	story = bytes.ReplaceAll(story, []byte("\r\n"), []byte("\n"))

	if w := storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), nil); w.Code != http.StatusPreconditionRequired {
		t.Fatalf("PUT without If-Match = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), map[string]string{"If-Match": "W/\"weak\""}); w.Code != http.StatusBadRequest {
		t.Fatalf("PUT with a malformed If-Match = %d", w.Code)
	}
	stale := `"` + strings.Repeat("0", 64) + `"`
	w := storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), map[string]string{"If-Match": stale})
	if w.Code != http.StatusPreconditionFailed || w.Header().Get("ETag") != p.ETag {
		t.Fatalf("stale PUT = %d etag %q: %s", w.Code, w.Header().Get("ETag"), w.Body.String())
	}

	w = storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), map[string]string{"If-Match": p.ETag})
	var saved struct {
		ETag     string                      `json:"etag"`
		Issues   []storymaker.Issue          `json:"issues"`
		Defaults []storymaker.DefaultApplied `json:"defaults_applied"`
		Story    storymaker.Story            `json:"story"`
	}
	decodeBody(t, w, &saved)
	if w.Code != http.StatusOK || saved.ETag == "" || saved.ETag == p.ETag || w.Header().Get("ETag") != saved.ETag || len(saved.Story.Scenes) != 3 {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	if saved.Defaults == nil || saved.Issues == nil {
		t.Fatalf("answer lacks defaults_applied or issues: %s", w.Body.String())
	}
	for _, is := range saved.Issues {
		if is.Severity == storymaker.SeverityError && is.Code != storymaker.CodeCodeError {
			t.Errorf("firewall.json gives an error issue: %+v", is)
		}
	}
	// s3 is a code scene without source: reported as code_error, the story is saved anyway.
	foundCode := false
	for _, is := range saved.Issues {
		foundCode = foundCode || (is.Code == storymaker.CodeCodeError && is.SceneID == "s3")
	}
	if !foundCode {
		t.Errorf("no code_error issue for the code scene without source: %+v", saved.Issues)
	}
	// The old version is stale now.
	if w := storymakerRequest(s, writeToken, http.MethodPut, path, bytes.NewReader(story), map[string]string{"If-Match": p.ETag}); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("second PUT with the old etag = %d", w.Code)
	}
	var again struct {
		ETag  string `json:"etag"`
		Story struct {
			Title string `json:"title"`
		} `json:"story"`
	}
	decodeBody(t, storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil), &again)
	if again.ETag != saved.ETag || again.Story.Title != "Was ist eine Firewall?" {
		t.Fatalf("GET after save: %+v", again)
	}

	// A story with a bad reference is saved with error issues, so a draft is not lost.
	bad := `{"schema":"storymaker/1","title":"Draft","scenes":[{"id":"s1","environment":"env.nowhere"}]}`
	w = storymakerRequest(s, writeToken, http.MethodPut, path, strings.NewReader(bad), map[string]string{"If-Match": saved.ETag})
	var draft struct {
		ETag   string             `json:"etag"`
		Issues []storymaker.Issue `json:"issues"`
	}
	decodeBody(t, w, &draft)
	if w.Code != http.StatusOK || len(draft.Issues) == 0 || draft.Issues[0].Code != storymaker.CodeUnknownAsset {
		t.Fatalf("draft PUT = %d %s", w.Code, w.Body.String())
	}

	// Decode errors are 422 and nothing is written.
	for name, body := range map[string]string{
		"syntax":   `{"schema":`,
		"unknown":  `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1","narrtion":"typo"}]}`,
		"wrong":    `{"schema":"storymaker/2","title":"x","scenes":[]}`,
		"trailing": `{"schema":"storymaker/1","title":"x","scenes":[{"id":"s1"}]} {}`,
	} {
		w := storymakerRequest(s, writeToken, http.MethodPut, path, strings.NewReader(body), map[string]string{"If-Match": draft.ETag})
		var e struct {
			Error  string             `json:"error"`
			Code   string             `json:"code"`
			Issues []storymaker.Issue `json:"issues"`
		}
		decodeBody(t, w, &e)
		if w.Code != http.StatusUnprocessableEntity || e.Error != "invalid_story" || e.Code != "invalid_story" || len(e.Issues) == 0 {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	var current struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil), &current)
	if current.ETag != draft.ETag {
		t.Fatal("a refused story changed story.json")
	}

	// Too large.
	huge := `{"schema":"storymaker/1","title":"` + strings.Repeat("x", 1<<20) + `","scenes":[{"id":"s1"}]}`
	if w := storymakerRequest(s, writeToken, http.MethodPut, path, strings.NewReader(huge), map[string]string{"If-Match": draft.ETag}); w.Code != http.StatusRequestEntityTooLarge || !strings.Contains(w.Body.String(), "too_large") {
		t.Fatalf("huge story = %d %.200s", w.Code, w.Body.String())
	}
}

func TestStorymakerCodeScenes(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	base := "/projects/" + p.Project.ID + "/scenes/s3/"
	src, err := os.ReadFile("../storymaker/testdata/scenes/s3.js")
	if err != nil {
		t.Fatal(err)
	}
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))

	if w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(src), nil); w.Code != http.StatusPreconditionRequired {
		t.Fatalf("code PUT without a precondition = %d", w.Code)
	}
	w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(src), map[string]string{"If-None-Match": "*"})
	var res struct {
		ETag   string                   `json:"etag"`
		OK     bool                     `json:"ok"`
		Errors []storymaker.BundleError `json:"errors"`
	}
	decodeBody(t, w, &res)
	if w.Code != http.StatusOK || !res.OK || res.ETag == "" || res.Errors == nil || len(res.Errors) != 0 {
		t.Fatalf("code PUT = %d %s", w.Code, w.Body.String())
	}
	// Creating again conflicts.
	if w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(src), map[string]string{"If-None-Match": "*"}); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("second create = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(src), map[string]string{"If-Match": `"` + strings.Repeat("a", 64) + `"`}); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale code PUT = %d", w.Code)
	}

	got := storymakerRequest(s, readToken, http.MethodGet, base+"code", nil, nil)
	if got.Code != http.StatusOK || got.Body.String() != string(src) || got.Header().Get("ETag") != res.ETag ||
		!strings.HasPrefix(got.Header().Get("Content-Type"), "text/javascript") || !strings.Contains(got.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("code GET = %d etag %q headers %v", got.Code, got.Header().Get("ETag"), got.Header())
	}
	bundle := storymakerRequest(s, readToken, http.MethodGet, base+"bundle", nil, nil)
	if bundle.Code != http.StatusOK || !strings.Contains(bundle.Body.String(), "__smScene") {
		t.Fatalf("bundle GET = %d %.200s", bundle.Code, bundle.Body.String())
	}

	// A syntax error saves the source, reports the position and removes the bundle.
	broken := []byte("export default function scene(api) {\n  api.duration(5;\n}\n")
	w = storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(broken), map[string]string{"If-Match": res.ETag})
	decodeBody(t, w, &res)
	if w.Code != http.StatusOK || res.OK || len(res.Errors) == 0 || res.Errors[0].Line != 2 {
		t.Fatalf("broken code PUT = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, base+"bundle", nil, nil); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "bundle_not_found") {
		t.Fatalf("bundle after a failed save = %d %s", w.Code, w.Body.String())
	}
	var project struct {
		CodeScenes []storymaker.CodeSceneStatus `json:"code_scenes"`
	}
	decodeBody(t, storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID, nil, nil), &project)
	if len(project.CodeScenes) != 1 || project.CodeScenes[0].OK || len(project.CodeScenes[0].Errors) == 0 {
		t.Fatalf("code_scenes = %+v", project.CodeScenes)
	}

	// Limits and names.
	big := bytes.Repeat([]byte("a"), 64<<10+1)
	if w := storymakerRequest(s, writeToken, http.MethodPut, base+"code", bytes.NewReader(big), map[string]string{"If-Match": res.ETag}); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized code = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodPut, "/projects/"+p.Project.ID+"/scenes/Bad_Name/code", bytes.NewReader(src), map[string]string{"If-None-Match": "*"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid scene id = %d", w.Code)
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+p.Project.ID+"/scenes/nope/code", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing code = %d", w.Code)
	}
}

func TestStorymakerStatus(t *testing.T) {
	s, readToken, _ := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	w := storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	var st struct {
		Enabled bool `json:"enabled"`
		FFmpeg  struct {
			Ready   bool   `json:"ready"`
			Version string `json:"version"`
		} `json:"ffmpeg"`
		Browser struct {
			Ready  bool   `json:"ready"`
			Source string `json:"source"`
		} `json:"browser"`
		Limits         map[string]float64 `json:"limits"`
		Transport      string             `json:"transport"`
		CatalogVersion string             `json:"catalog_version"`
	}
	decodeBody(t, w, &st)
	if w.Code != http.StatusOK || !st.Enabled || !st.FFmpeg.Ready || !st.Browser.Ready || st.Browser.Source != "system" {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	if st.Limits["max_duration_seconds"] != 900 || st.Limits["max_scenes"] != 200 || st.Transport != render.DefaultTransport || len(st.CatalogVersion) != 12 {
		t.Errorf("status values: %+v", st)
	}
	if strings.ContainsAny(st.FFmpeg.Version, " 	") || strings.Contains(st.FFmpeg.Version, "Copyright") {
		t.Errorf("ffmpeg.version must be the version token, got %q", st.FFmpeg.Version)
	}
	// The configured transport and duration show up.
	s.Cfg.Storymaker.Render.Transport = "screenshot"
	s.Cfg.Storymaker.Render.MaxDurationSeconds = 120
	w = storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	decodeBody(t, w, &st)
	if st.Transport != "screenshot" || st.Limits["max_duration_seconds"] != 120 {
		t.Errorf("configured status: %s", w.Body.String())
	}
	// An FFmpeg that is not there is reported with its issue.
	s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-ffmpeg")
	w = storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
	if !strings.Contains(w.Body.String(), `"ffmpeg":{"issue":`) && !strings.Contains(w.Body.String(), `"issue"`) {
		t.Errorf("missing ffmpeg is not explained: %s", w.Body.String())
	}
}

// renderFixture prepares a project with a valid story for the render endpoint tests.
func renderFixture(t *testing.T, run render.RunFunc) (*Server, string, apiProject) {
	t.Helper()
	s, _, writeToken := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	s.storymakerRunOverride = run
	p := createStorymakerProject(t, s, writeToken)
	return s, writeToken, p
}

func TestStorymakerRenderEndpointChecksAndQueue(t *testing.T) {
	var started atomic.Int32
	release := make(chan struct{})
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		started.Add(1)
		h.SetPhase(render.PhaseRendering)
		h.SetFrames(1, 10)
		select {
		case <-release:
			return render.Output{Path: "Documents/Storymaker/" + job.ProjectID + "/exports/x.mp4", Size: 3}, nil
		case <-ctx.Done():
			return render.Output{}, ctx.Err()
		}
	})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	path := "/projects/" + p.Project.ID + "/render"

	if w := storymakerRequest(s, token, http.MethodPost, path, nil, nil); w.Code != http.StatusPreconditionRequired {
		t.Fatalf("render without If-Match = %d %s", w.Code, w.Body.String())
	}
	stale := `"` + strings.Repeat("0", 64) + `"`
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": stale}); w.Code != http.StatusPreconditionFailed || w.Header().Get("ETag") != p.ETag {
		t.Fatalf("stale render = %d etag %q", w.Code, w.Header().Get("ETag"))
	}
	if w := storymakerRequest(s, token, http.MethodPost, "/projects/33333333-3333-4333-8333-333333333333/render", nil, map[string]string{"If-Match": p.ETag}); w.Code != http.StatusNotFound {
		t.Fatalf("render of an unknown project = %d", w.Code)
	}
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": p.ETag, "Idempotency-Key": "bad key with spaces"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad idempotency key = %d", w.Code)
	}

	// An invalid story is refused with its issues.
	bad := `{"schema":"storymaker/1","title":"Bad","scenes":[{"id":"s1","environment":"env.nowhere"}]}`
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(bad), map[string]string{"If-Match": p.ETag})
	var saved struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &saved)
	w = storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": saved.ETag})
	var refused struct {
		Code   string             `json:"code"`
		Issues []storymaker.Issue `json:"issues"`
	}
	decodeBody(t, w, &refused)
	if w.Code != http.StatusUnprocessableEntity || refused.Code != "invalid_story" || len(refused.Issues) == 0 {
		t.Fatalf("invalid story render = %d %s", w.Code, w.Body.String())
	}
	good := `{"schema":"storymaker/1","title":"Good","scenes":[{"id":"s1","narration":"Hallo."}]}`
	w = storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(good), map[string]string{"If-Match": saved.ETag})
	decodeBody(t, w, &saved)

	// Missing FFmpeg and missing browser are capability errors.
	s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-ffmpeg")
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": saved.ETag}); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ffmpeg_unavailable") {
		t.Fatalf("no ffmpeg = %d %s", w.Code, w.Body.String())
	}
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	s.Cfg.Storymaker.Render.BrowserPath = filepath.Join(t.TempDir(), "no-chrome")
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": saved.ETag}); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "browser_unavailable") {
		t.Fatalf("no browser = %d %s", w.Code, w.Body.String())
	}
	s.Cfg.Storymaker.Render.BrowserPath = ""

	// A valid request queues a job; the same Idempotency-Key returns it again.
	headers := map[string]string{"If-Match": saved.ETag, "Idempotency-Key": "export-1"}
	w = storymakerRequest(s, token, http.MethodPost, path, nil, headers)
	var accepted struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &accepted)
	if w.Code != http.StatusAccepted || accepted.Job.ID == "" || accepted.Job.ProjectID != p.Project.ID || accepted.Job.Kind != "render" {
		t.Fatalf("render = %d %s", w.Code, w.Body.String())
	}
	var replay struct {
		Job render.Job `json:"job"`
	}
	w = storymakerRequest(s, token, http.MethodPost, path, nil, headers)
	decodeBody(t, w, &replay)
	if w.Code != http.StatusAccepted || replay.Job.ID != accepted.Job.ID {
		t.Fatalf("idempotent replay = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": saved.ETag, "Idempotency-Key": "export-1-other"}); w.Code != http.StatusAccepted {
		t.Fatalf("second job = %d", w.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for started.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	// GET job, list, cancel.
	w = storymakerRequest(s, token, http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil)
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusOK || got.Job.Status != "running" || got.Job.Phase != "rendering" || got.Job.FramesTotal != 10 {
		t.Fatalf("job = %d %s", w.Code, w.Body.String())
	}
	if w := storymakerRequest(s, token, http.MethodGet, "/jobs/00000000-0000-4000-8000-000000000000", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown job = %d", w.Code)
	}
	w = storymakerRequest(s, token, http.MethodGet, "/projects/"+p.Project.ID+"/jobs", nil, nil)
	var listed struct {
		Jobs []render.Job `json:"jobs"`
	}
	decodeBody(t, w, &listed)
	if len(listed.Jobs) != 2 {
		t.Fatalf("job list = %s", w.Body.String())
	}

	// Cancel works while the desktop is read-only (desktopStop).
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	if w := storymakerRequest(s, token, http.MethodPost, path, nil, headers); w.Code != http.StatusForbidden {
		t.Fatalf("render while read-only = %d", w.Code)
	}
	w = storymakerRequest(s, token, http.MethodPost, "/jobs/"+accepted.Job.ID+"/cancel", nil, nil)
	decodeBody(t, w, &got)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel while read-only = %d %s", w.Code, w.Body.String())
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w = storymakerRequest(s, token, http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil)
		decodeBody(t, w, &got)
		if got.Job.Status == "cancelled" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got.Job.Status != "cancelled" {
		t.Fatalf("job after cancel = %+v", got.Job)
	}
	ro.VirtualDesktop.ReadOnly = false
	s.replaceConfigSnapshot(&ro)
	close(release)
}

func TestStorymakerRenderQueueIsBounded(t *testing.T) {
	release := make(chan struct{})
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return render.Output{}, nil
	})
	defer close(release)
	path := "/projects/" + p.Project.ID + "/render"
	busy := 0
	for i := 0; i < render.DefaultMaxQueued+3; i++ {
		w := storymakerRequest(s, token, http.MethodPost, path, nil, map[string]string{"If-Match": p.ETag})
		if w.Code == http.StatusTooManyRequests {
			if !strings.Contains(w.Body.String(), `"code":"busy"`) {
				t.Fatalf("429 without the busy code: %s", w.Body.String())
			}
			busy++
			continue
		}
		if w.Code != http.StatusAccepted {
			t.Fatalf("request %d = %d %s", i, w.Code, w.Body.String())
		}
	}
	// One job runs, sixteen wait, the rest is refused.
	if busy < 2 || busy > 3 {
		t.Fatalf("%d requests were refused as busy", busy)
	}
}

func TestStorymakerEventsStreamJobChanges(t *testing.T) {
	step := make(chan struct{})
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		h.SetPhase(render.PhaseRendering)
		select {
		case <-step:
		case <-ctx.Done():
			return render.Output{}, ctx.Err()
		}
		return render.Output{Path: "Documents/Storymaker/x/exports/a.mp4", Size: 9}, nil
	})
	srv := httptest.NewServer(handleStorymaker(s))
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+storymakerAPIBase+"/projects/"+p.Project.ID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("events = %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	events := make(chan render.Job, 32)
	go func() {
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		var name string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: ") && name == "job":
				var d struct {
					Job render.Job `json:"job"`
				}
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &d) == nil {
					events <- d.Job
				}
			}
		}
		close(events)
	}()

	w := storymakerRequest(s, token, http.MethodPost, "/projects/"+p.Project.ID+"/render", nil, map[string]string{"If-Match": p.ETag})
	if w.Code != http.StatusAccepted {
		t.Fatalf("render = %d %s", w.Code, w.Body.String())
	}
	var accepted struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &accepted)
	seenRunning := false
	waitFor := func(pred func(render.Job) bool) render.Job {
		t.Helper()
		for {
			select {
			case j, ok := <-events:
				if !ok {
					t.Fatal("the event stream ended")
				}
				if j.ID == accepted.Job.ID && pred(j) {
					return j
				}
			case <-time.After(10 * time.Second):
				t.Fatal("no matching event")
			}
		}
	}
	waitFor(func(j render.Job) bool {
		seenRunning = j.Status == "running" && j.Phase == "rendering"
		return seenRunning
	})
	close(step)
	done := waitFor(func(j render.Job) bool { return j.Status == "succeeded" })
	if !seenRunning || done.Output == nil || done.Output.Size != 9 || done.Progress != 1 {
		t.Fatalf("final event = %+v", done)
	}
	cancel()

	// A new subscriber gets the latest job of the project at once.
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+storymakerAPIBase+"/projects/"+p.Project.ID+"/events", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	res2, err := http.DefaultClient.Do(req2.WithContext(ctx2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	reader := bufio.NewReader(res2.Body)
	found := false
	for i := 0; i < 8 && !found; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		found = strings.Contains(line, accepted.Job.ID) && strings.Contains(line, `"succeeded"`)
	}
	if !found {
		t.Fatal("a new subscriber did not receive the finished job")
	}
}

func TestStorymakerDeleteProjectCancelsItsJobs(t *testing.T) {
	started := make(chan struct{}, 1)
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		started <- struct{}{}
		<-ctx.Done()
		return render.Output{}, ctx.Err()
	})
	w := storymakerRequest(s, token, http.MethodPost, "/projects/"+p.Project.ID+"/render", nil, map[string]string{"If-Match": p.ETag})
	var accepted struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &accepted)
	<-started
	if w := storymakerRequest(s, token, http.MethodDelete, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var got struct {
			Job render.Job `json:"job"`
		}
		decodeBody(t, storymakerRequest(s, token, http.MethodGet, "/jobs/"+accepted.Job.ID, nil, nil), &got)
		if got.Job.Status == "cancelled" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the job of a deleted project kept running")
}

func TestStorymakerPlayerHostPage(t *testing.T) {
	s, readToken, _ := storymakerAPIFixture(t)
	r := httptest.NewRequest(http.MethodGet, storymakerAPIBase+"/player", nil)
	r.Host = "aurago.example:8443"
	r.Header.Set("Authorization", "Bearer "+readToken)
	w := httptest.NewRecorder()
	// Through the security middleware, as in production.
	securityHeadersMiddleware(handleStorymaker(s), false, false).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("player = %d %s", w.Code, w.Body.String())
	}
	origin := "http://aurago.example:8443"
	wantCSP := "sandbox allow-scripts; default-src 'none'; script-src " + origin + "/js/storymaker/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src " + origin + "/css/storymaker-player.css; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'"
	h := w.Header()
	if h.Get("Content-Security-Policy") != wantCSP {
		t.Errorf("CSP =\n%s\nwant\n%s", h.Get("Content-Security-Policy"), wantCSP)
	}
	if h.Get("Referrer-Policy") != "no-referrer" || !strings.Contains(h.Get("Cache-Control"), "no-store") || h.Get("X-Frame-Options") == "DENY" || h.Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Errorf("headers: Referrer-Policy %q Cache-Control %q X-Frame-Options %q", h.Get("Referrer-Policy"), h.Get("Cache-Control"), h.Get("X-Frame-Options"))
	}
	if strings.Contains(h.Get("Content-Security-Policy"), "unsafe-inline") || strings.Contains(h.Get("Content-Security-Policy"), "unsafe-eval") {
		t.Error("the player CSP allows inline or eval")
	}
	wantPage := `<!doctype html><html data-sm-mode="app" data-sm-base="` + origin + `"><head><meta charset="utf-8"><link rel="stylesheet" href="` + origin + `/css/storymaker-player.css"></head>` +
		`<body><canvas id="sm-stage"></canvas><script src="` + origin + `/js/storymaker/host.bundle.js"></script></body></html>`
	if w.Body.String() != wantPage {
		t.Errorf("page:\n%s\nwant:\n%s", w.Body.String(), wantPage)
	}
	if strings.Contains(w.Body.String(), "<style") || strings.Contains(strings.ReplaceAll(w.Body.String(), `<script src=`, ""), "<script") {
		t.Error("inline script or style in the player page")
	}

	// HEAD answers the headers only; a hostile Host is refused; anonymous callers are refused.
	hr := httptest.NewRequest(http.MethodHead, storymakerAPIBase+"/player", nil)
	hr.Header.Set("Authorization", "Bearer "+readToken)
	hw := httptest.NewRecorder()
	handleStorymaker(s).ServeHTTP(hw, hr)
	if hw.Code != http.StatusOK || hw.Body.Len() != 0 {
		t.Errorf("HEAD = %d with %d bytes", hw.Code, hw.Body.Len())
	}
	evil := httptest.NewRequest(http.MethodGet, storymakerAPIBase+"/player", nil)
	evil.Host = "evil.example/js/storymaker/ <script>"
	evil.Header.Set("Authorization", "Bearer "+readToken)
	ew := httptest.NewRecorder()
	handleStorymaker(s).ServeHTTP(ew, evil)
	if ew.Code != http.StatusBadRequest {
		t.Errorf("hostile Host = %d", ew.Code)
	}
	if w := storymakerRequest(s, "", http.MethodGet, "/player", nil, nil); w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous player = %d", w.Code)
	}
	// Behind a trusted proxy that says https.
	if !strings.HasPrefix(StorymakerPlayerCSP("https://a.example"), "sandbox allow-scripts;") {
		t.Error("CSP builder")
	}
}

func TestStorymakerCapabilityAndOperations(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	c := desktopCapabilities{s}
	if !c.HasCapability("storymaker") {
		t.Fatal("storymaker capability is off although the feature is on")
	}
	off := *s.ConfigSnapshot()
	off.Storymaker.Enabled = false
	s.replaceConfigSnapshot(&off)
	if c.HasCapability("storymaker") {
		t.Fatal("storymaker capability is on although the feature is off")
	}
	off.Storymaker.Enabled = true
	off.VirtualDesktop.Enabled = false
	s.replaceConfigSnapshot(&off)
	if c.HasCapability("storymaker") {
		t.Fatal("storymaker capability is on without the virtual desktop")
	}

	cancel := httptest.NewRequest(http.MethodPost, storymakerAPIBase+"/jobs/11111111-1111-4111-8111-111111111111/cancel", nil)
	if desktopRequestOperation(cancel) != desktopStop {
		t.Error("job cancel is not a desktopStop operation")
	}
	for _, path := range []string{"/projects", "/projects/x/render", "/jobs/x"} {
		r := httptest.NewRequest(http.MethodPost, storymakerAPIBase+path, nil)
		if desktopRequestOperation(r) != desktopWrite {
			t.Errorf("POST %s is not a write operation", path)
		}
	}
}

func TestStorymakerConfigValidationRejectsBadValuesBeforeWrite(t *testing.T) {
	for _, setup := range []bool{true, false} {
		name := "config"
		if setup {
			name = "setup"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			original := []byte("server:\n  host: 127.0.0.1\nstorymaker:\n  enabled: true\n")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			cfg.ConfigPath = path
			s := &Server{Cfg: cfg, Logger: slog.Default()}
			for _, patch := range []string{`{"storymaker":{"render":{"timeout_seconds":5}}}`, `{"storymaker":{"render":{"transport":"carrier-pigeon"}}}`, `{"storymaker":{"render":{"browser_path":"relative/chrome"}}}`} {
				if setup {
					var m map[string]interface{}
					if err := json.Unmarshal([]byte(patch), &m); err != nil {
						t.Fatal(err)
					}
					if _, err := applyConfigPatch(s, m); err == nil || !strings.Contains(err.Error(), "storymaker:") {
						t.Fatalf("patch %s: %v", patch, err)
					}
				} else {
					rec := httptest.NewRecorder()
					handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
					if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "storymaker:") {
						t.Fatalf("patch %s: %d %s", patch, rec.Code, rec.Body.String())
					}
				}
			}
			stored, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(stored, original) {
				t.Fatalf("a rejected patch changed the config file: %v", err)
			}
		})
	}
}

func TestStorymakerStoreOverTheDesktopWorkspace(t *testing.T) {
	s, _, writeToken := storymakerAPIFixture(t)
	// Any authorised request resolves the desktop service and points the store at it.
	p := createStorymakerProject(t, s, writeToken)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	st := rt.store
	ctx := context.Background()
	dir := "Documents/Storymaker/" + p.Project.ID

	// ReadFile / WriteFile with etags.
	data, etag, err := st.ReadFile(ctx, dir+"/story.json")
	if err != nil || etag != storymaker.ETag(data) {
		t.Fatalf("ReadFile = %v %q", err, etag)
	}
	if _, err := st.WriteFile(ctx, dir+"/story.json", []byte("{}"), `"`+strings.Repeat("1", 64)+`"`); !errors.Is(err, storymaker.ErrConflict) {
		t.Fatalf("stale WriteFile = %v", err)
	}
	if _, err := st.WriteFile(ctx, dir+"/story.json", []byte("{}"), storymaker.MustNotExist); !errors.Is(err, storymaker.ErrConflict) {
		t.Fatalf("MustNotExist on an existing file = %v", err)
	}
	newTag, err := st.WriteFile(ctx, dir+"/extra/notes.txt", []byte("hi"), storymaker.MustNotExist)
	if err != nil || newTag != storymaker.ETag([]byte("hi")) {
		t.Fatalf("create in a new folder = %v %q", err, newTag)
	}
	if _, err := st.WriteFile(ctx, dir+"/extra/notes.txt", []byte("hello"), newTag); err != nil {
		t.Fatalf("WriteFile with the current etag: %v", err)
	}
	if _, err := st.WriteFile(ctx, dir+"/extra/missing.txt", []byte("x"), newTag); !errors.Is(err, storymaker.ErrConflict) {
		t.Fatalf("etag for a file that does not exist = %v", err)
	}
	if _, _, err := st.ReadFile(ctx, dir+"/extra/none.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadFile of a missing file = %v", err)
	}

	// List: base names, sorted; a missing folder is (nil, nil).
	names, err := st.List(ctx, dir)
	if err != nil || len(names) < 3 {
		t.Fatalf("List = %v %v", names, err)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("List is not sorted: %v", names)
		}
	}
	if names, err := st.List(ctx, dir+"/nope"); err != nil || names != nil {
		t.Fatalf("List of a missing folder = %v %v", names, err)
	}

	// Remove: a folder tree goes, a missing path is fine.
	if err := st.Remove(ctx, dir+"/extra"); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove(ctx, dir+"/extra"); err != nil {
		t.Fatalf("Remove of a missing path = %v", err)
	}
	if _, _, err := st.ReadFile(ctx, dir+"/extra/notes.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file survived its folder: %v", err)
	}

	// Publisher: create-only streams.
	if err := st.WriteStream(ctx, dir+"/exports/a.mp4", strings.NewReader("video"), 1<<20); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteStream(ctx, dir+"/exports/a.mp4", strings.NewReader("again"), 1<<20); err == nil {
		t.Fatal("WriteStream replaced an existing file")
	}
	if err := st.WriteStream(ctx, dir+"/exports/b.mp4", strings.NewReader("too long"), 3); err == nil {
		t.Fatal("WriteStream ignored its size budget")
	}
	if err := st.WriteBytes(ctx, dir+"/exports/job.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteBytes(ctx, dir+"/exports/job.json", []byte("{\"x\":1}")); err != nil {
		t.Fatalf("WriteBytes replace: %v", err)
	}
}

func TestStorymakerSettingsRefuseWhenSwitchedOffOrReadOnly(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	st, err := s.storymakerSettings(context.Background())
	if err != nil || st.FFmpegPath == "" || st.MaxDurationSeconds != 900 || st.Limits.MaxDurationSeconds != 900 || st.TimeoutSeconds != 3600 {
		t.Fatalf("settings = %+v, %v", st, err)
	}
	off := *s.ConfigSnapshot()
	off.Storymaker.Enabled = false
	s.replaceConfigSnapshot(&off)
	var je *render.JobError
	if _, err := s.storymakerSettings(context.Background()); !errors.As(err, &je) || je.Code != render.CodeDisabled {
		t.Fatalf("disabled: %v", err)
	}
	off.Storymaker.Enabled, off.VirtualDesktop.ReadOnly = true, true
	s.replaceConfigSnapshot(&off)
	if _, err := s.storymakerSettings(context.Background()); !errors.As(err, &je) || je.Code != render.CodeRenderFailed || !strings.Contains(je.Message, "read-only") {
		t.Fatalf("read-only: %v", err)
	}
}

// storymakerBlockingRun is a job function that holds its job until it is cancelled.
func storymakerBlockingRun(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
	<-ctx.Done()
	return render.Output{}, ctx.Err()
}

// storymakerQueueFixture makes a server whose jobs run the given functions. Tests put jobs straight
// on the managers, so no FFmpeg or browser is needed.
func storymakerQueueFixture(t *testing.T, renderRun, audioRun render.RunFunc) (*Server, *storymakerRuntime, string, apiProject) {
	t.Helper()
	s, _, writeToken := storymakerAPIFixture(t)
	s.storymakerRunOverride = renderRun
	s.storymakerAudioRunOverride = audioRun
	p := createStorymakerProject(t, s, writeToken)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	return s, rt, writeToken, p
}

func enqueueStorymakerJob(t *testing.T, m *render.Manager, projectID, kind string) render.Job {
	t.Helper()
	job, _, err := m.Enqueue(render.EnqueueRequest{ProjectID: projectID, Kind: kind})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func waitStorymakerManagerJob(t *testing.T, m *render.Manager, id string) render.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if job, ok := m.Get(id); ok && job.Terminal() {
			return job
		}
		time.Sleep(5 * time.Millisecond)
	}
	job, _ := m.Get(id)
	t.Fatalf("job %s did not end: %+v", id, job)
	return render.Job{}
}

// openStorymakerEvents opens the event stream of a project and returns its jobs in the order they arrive.
func openStorymakerEvents(t *testing.T, s *Server, token, projectID string) <-chan render.Job {
	t.Helper()
	srv := httptest.NewServer(handleStorymaker(s))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+storymakerAPIBase+"/projects/"+projectID+"/events", nil)
	if err != nil {
		cancel()
		srv.Close()
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		srv.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		res.Body.Close()
		srv.Close()
	})
	events := make(chan render.Job, 64)
	go func() {
		defer close(events)
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		var name string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: ") && name == "job":
				var d struct {
					Job render.Job `json:"job"`
				}
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &d) == nil {
					events <- d.Job
				}
			}
		}
	}()
	return events
}

func TestStorymakerRouterServesBothQueues(t *testing.T) {
	s, rt, token, p := storymakerQueueFixture(t, storymakerBlockingRun, storymakerBlockingRun)
	// CreatedAt orders the merged list; the pauses keep the three stamps apart on a coarse clock.
	older := enqueueStorymakerJob(t, rt.jobs, p.Project.ID, "")
	time.Sleep(20 * time.Millisecond)
	narrate := enqueueStorymakerJob(t, rt.narrate, p.Project.ID, render.KindNarrate)
	time.Sleep(20 * time.Millisecond)
	newer := enqueueStorymakerJob(t, rt.jobs, p.Project.ID, "")

	w := storymakerRequest(s, token, http.MethodGet, "/jobs/"+narrate.ID, nil, nil)
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusOK || got.Job.ID != narrate.ID || got.Job.Kind != render.KindNarrate {
		t.Fatalf("GET narrate job = %d %s", w.Code, w.Body.String())
	}

	w = storymakerRequest(s, token, http.MethodGet, "/projects/"+p.Project.ID+"/jobs", nil, nil)
	var listed struct {
		Jobs []render.Job `json:"jobs"`
	}
	decodeBody(t, w, &listed)
	ids := make([]string, 0, len(listed.Jobs))
	for _, job := range listed.Jobs {
		ids = append(ids, job.ID)
	}
	want := []string{newer.ID, narrate.ID, older.ID}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("merged list = %v, want %v", ids, want)
	}

	if w := storymakerRequest(s, token, http.MethodPost, "/jobs/"+narrate.ID+"/cancel", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("cancel = %d %s", w.Code, w.Body.String())
	}
	if job := waitStorymakerManagerJob(t, rt.narrate, narrate.ID); job.Status != render.StatusCancelled {
		t.Fatalf("cancelled narrate job = %+v", job)
	}
}

func TestStorymakerDeleteProjectCancelsBothQueues(t *testing.T) {
	started := make(chan struct{}, 2)
	// The pause after the cancel stands for a job that still holds a file. The delete has to outlast it.
	run := func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		started <- struct{}{}
		<-ctx.Done()
		time.Sleep(200 * time.Millisecond)
		return render.Output{}, ctx.Err()
	}
	s, rt, token, p := storymakerQueueFixture(t, run, run)
	renderJob := enqueueStorymakerJob(t, rt.jobs, p.Project.ID, "")
	narrateJob := enqueueStorymakerJob(t, rt.narrate, p.Project.ID, render.KindNarrate)
	<-started
	<-started
	if w := storymakerRequest(s, token, http.MethodDelete, "/projects/"+p.Project.ID, nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	// No polling: both jobs have ended by the time the delete answers.
	if job, _ := rt.jobs.Get(renderJob.ID); job.Status != render.StatusCancelled {
		t.Fatalf("render job when the delete answered = %+v", job)
	}
	if job, _ := rt.narrate.Get(narrateJob.ID); job.Status != render.StatusCancelled {
		t.Fatalf("narrate job when the delete answered = %+v", job)
	}
}

func TestStorymakerEventsDeliverBothQueues(t *testing.T) {
	s, rt, token, p := storymakerQueueFixture(t, storymakerBlockingRun, storymakerBlockingRun)
	events := openStorymakerEvents(t, s, token, p.Project.ID)
	renderJob := enqueueStorymakerJob(t, rt.jobs, p.Project.ID, "")
	narrateJob := enqueueStorymakerJob(t, rt.narrate, p.Project.ID, render.KindNarrate)
	seen := map[string]bool{}
	timeout := time.After(10 * time.Second)
	for !seen[renderJob.ID] || !seen[narrateJob.ID] {
		select {
		case job, ok := <-events:
			if !ok {
				t.Fatal("the event stream ended")
			}
			seen[job.ID] = true
		case <-timeout:
			t.Fatalf("events seen: %v", seen)
		}
	}
}

func TestStorymakerEventsFirstListKeepsARunningRender(t *testing.T) {
	started := make(chan struct{}, 1)
	renderRun := func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		started <- struct{}{}
		<-ctx.Done()
		return render.Output{}, ctx.Err()
	}
	audioRun := func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		return render.Output{}, nil
	}
	s, rt, token, p := storymakerQueueFixture(t, renderRun, audioRun)
	running := enqueueStorymakerJob(t, rt.jobs, p.Project.ID, "")
	<-started
	var finished []string
	for i := 0; i < 6; i++ {
		time.Sleep(20 * time.Millisecond)
		job := enqueueStorymakerJob(t, rt.narrate, p.Project.ID, render.KindNarrate)
		waitStorymakerManagerJob(t, rt.narrate, job.ID)
		finished = append(finished, job.ID)
	}
	// Merged newest first: six finished narrate jobs, then the render. The latest five are the newest
	// narrate jobs, and the unfinished rule adds the render. The stream sends oldest first, so the
	// render comes first and the oldest narrate job stays out.
	want := append([]string{running.ID}, finished[1:]...)
	events := openStorymakerEvents(t, s, token, p.Project.ID)
	got := make([]string, 0, len(want))
	timeout := time.After(10 * time.Second)
	for len(got) < len(want) {
		select {
		case job, ok := <-events:
			if !ok {
				t.Fatalf("the event stream ended after %d jobs", len(got))
			}
			got = append(got, job.ID)
		case <-timeout:
			t.Fatalf("first list so far = %v, want %v", got, want)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("first list = %v, want %v", got, want)
	}
}

func TestStorymakerFailingSecondManagerClosesTheFirst(t *testing.T) {
	dir := t.TempDir()
	// A file where the narrate folder belongs makes os.MkdirAll fail in the second NewManager.
	if err := os.WriteFile(filepath.Join(dir, "narrate"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var opened []*render.Manager
	open := func(o render.ManagerOptions) (*render.Manager, error) {
		m, err := render.NewManager(o)
		if err == nil {
			opened = append(opened, m)
		}
		return m, err
	}
	if _, _, err := openStorymakerManagers(dir, storymakerBlockingRun, storymakerBlockingRun, slog.Default(), open); err == nil {
		t.Fatal("the second manager did not fail")
	}
	if len(opened) != 1 {
		t.Fatalf("%d managers opened, want 1", len(opened))
	}
	if _, _, err := opened[0].Enqueue(render.EnqueueRequest{ProjectID: "x"}); !errors.Is(err, render.ErrClosed) {
		t.Fatalf("the first manager after the failure: %v, want ErrClosed", err)
	}
}

func TestStorymakerRenderOptions(t *testing.T) {
	plans := make(chan *render.Plan, 8)
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		plans <- payload.(*render.Plan)
		return render.Output{Path: "Documents/Storymaker/" + job.ProjectID + "/exports/x.mp4", Size: 3}, nil
	})
	path := "/projects/" + p.Project.ID + "/render"
	headers := func(key string) map[string]string {
		return map[string]string{"If-Match": p.ETag, "Idempotency-Key": key}
	}
	nextPlan := func() *render.Plan {
		t.Helper()
		select {
		case plan := <-plans:
			return plan
		case <-time.After(10 * time.Second):
			t.Fatal("the job did not run")
			return nil
		}
	}

	// A strict body: unknown fields, a bad quality and more than 16 KiB are 400.
	for name, body := range map[string]string{
		"unknown field": `{"handoff":true}`,
		"bad quality":   `{"quality":"ultra"}`,
		"too large":     `{"srt":true}` + strings.Repeat(" ", 16<<10),
	} {
		w := storymakerRequest(s, token, http.MethodPost, path, strings.NewReader(body), headers("bad"))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"code":"invalid_request"`) {
			t.Errorf("%s = %d %s", name, w.Code, w.Body.String())
		}
	}

	// An empty body gives the defaults.
	w := storymakerRequest(s, token, http.MethodPost, path, nil, headers("one"))
	var first struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &first)
	if w.Code != http.StatusAccepted || first.Job.ID == "" {
		t.Fatalf("empty body = %d %s", w.Code, w.Body.String())
	}
	if plan := nextPlan(); plan.Options != (render.RenderOptions{Quality: "standard"}) {
		t.Errorf("default options = %+v", plan.Options)
	}

	// The same key returns the job with the same options and is 409 with other options.
	w = storymakerRequest(s, token, http.MethodPost, path, nil, headers("one"))
	var replay struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &replay)
	if w.Code != http.StatusAccepted || replay.Job.ID != first.Job.ID {
		t.Errorf("replay = %d %s", w.Code, w.Body.String())
	}
	w = storymakerRequest(s, token, http.MethodPost, path, strings.NewReader(`{"srt":true}`), headers("one"))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "idempotency_conflict") {
		t.Errorf("same key, other options = %d %s", w.Code, w.Body.String())
	}

	// The options reach the snapshot, and the override reaches the story there.
	body := `{"burn_in_captions":false,"srt":true,"vtt":true,"stems":true,"quality":"high"}`
	w = storymakerRequest(s, token, http.MethodPost, path, strings.NewReader(body), headers("two"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("full options = %d %s", w.Code, w.Body.String())
	}
	plan := nextPlan()
	o := plan.Options
	if o.BurnInCaptions == nil || *o.BurnInCaptions || !o.SRT || !o.VTT || !o.Stems || o.Quality != "high" || plan.Story.Style.Captions {
		t.Errorf("plan options = %+v, captions %v", o, plan.Story.Style.Captions)
	}
}
