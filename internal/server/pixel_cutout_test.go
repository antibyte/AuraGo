package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/onnxrt"
	"aurago/internal/pixelmatte"
)

// pixelCutoutInstaller is a fake pixelmatte.Installer: nothing touches the
// disk or the network. EnsureRuntime/EnsureModel block on gate when it is set.
type pixelCutoutInstaller struct {
	mu           sync.Mutex
	runtimeReady bool
	models       map[string]bool
	runtimeCalls int
	modelCalls   int
	gate         chan struct{}
	entered      chan struct{}
}

func newPixelCutoutInstaller(ready bool) *pixelCutoutInstaller {
	f := &pixelCutoutInstaller{models: map[string]bool{}, entered: make(chan struct{}, 8), runtimeReady: ready}
	if ready {
		for _, spec := range pixelmatte.Models() {
			f.models[spec.Asset.ID] = true
		}
	}
	return f
}

func (f *pixelCutoutInstaller) ModelReady(spec onnxrt.AssetSpec, _ string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.models[spec.ID]
}

func (f *pixelCutoutInstaller) RuntimeReady(spec onnxrt.AssetSpec) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return "runtimes/" + spec.ID, f.runtimeReady
}

func (f *pixelCutoutInstaller) block(ctx context.Context) error {
	if f.gate == nil {
		return nil
	}
	f.entered <- struct{}{}
	select {
	case <-f.gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *pixelCutoutInstaller) EnsureRuntime(ctx context.Context, spec onnxrt.AssetSpec) (string, error) {
	f.mu.Lock()
	f.runtimeCalls++
	f.mu.Unlock()
	if err := f.block(ctx); err != nil {
		return "", err
	}
	f.mu.Lock()
	f.runtimeReady = true
	f.mu.Unlock()
	return "runtimes/" + spec.ID, nil
}

func (f *pixelCutoutInstaller) EnsureModel(ctx context.Context, spec onnxrt.AssetSpec, _ string) error {
	f.mu.Lock()
	f.modelCalls++
	f.mu.Unlock()
	if err := f.block(ctx); err != nil {
		return err
	}
	f.mu.Lock()
	f.models[spec.ID] = true
	f.mu.Unlock()
	return nil
}

func (f *pixelCutoutInstaller) calls() (runtimeCalls, modelCalls int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runtimeCalls, f.modelCalls
}

// pixelCutoutRunner is a fake pixelmatte.Runner: it answers with a gray PNG of
// the size of the request's image, or blocks / fails on demand.
type pixelCutoutRunner struct {
	mu      sync.Mutex
	calls   int
	last    pixelmatte.RunRequest
	started chan struct{}
	release chan struct{}
	err     error
	badSize bool
}

func (f *pixelCutoutRunner) Run(ctx context.Context, request pixelmatte.RunRequest) ([]byte, error) {
	f.mu.Lock()
	f.calls++
	f.last = request
	f.mu.Unlock()
	if f.started != nil {
		f.started <- struct{}{}
		select {
		case <-f.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	size, _, err := image.DecodeConfig(bytes.NewReader(request.Image))
	if err != nil {
		return nil, err
	}
	width, height := size.Width, size.Height
	if f.badSize {
		width++
	}
	matte := image.NewGray(image.Rect(0, 0, width, height))
	for i := range matte.Pix {
		matte.Pix[i] = uint8(i)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, matte); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func newPixelCutoutServer(t *testing.T, installer pixelmatte.Installer, runner pixelmatte.Runner, mutate func(*pixelmatte.Options)) *Server {
	t.Helper()
	s := &Server{Cfg: &config.Config{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	opts := pixelmatte.Options{
		DataDir: t.TempDir(), Executable: "aurago-test", Installer: installer, Runner: runner,
		GOOS: "windows", GOARCH: "amd64", Logger: s.Logger,
	}
	if mutate != nil {
		mutate(&opts)
	}
	pixelCutoutManagers.Store(s, pixelmatte.NewManager(opts))
	t.Cleanup(func() { pixelCutoutManagers.Delete(s) })
	return s
}

func pixelCutoutRequest(handler http.HandlerFunc, method, path string, body any) *httptest.ResponseRecorder {
	var reader io.Reader
	switch value := body.(type) {
	case nil:
	case string:
		reader = strings.NewReader(value)
	default:
		raw, _ := json.Marshal(value)
		reader = bytes.NewReader(raw)
	}
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(method, path, reader))
	return recorder
}

func pixelCutoutDecode(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON object: %v: %s", err, recorder.Body.String())
	}
	return body
}

func pixelCutoutSource(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 40), B: 100, A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

func TestPixelCutoutStatusShape(t *testing.T) {
	s := newPixelCutoutServer(t, newPixelCutoutInstaller(false), &pixelCutoutRunner{}, nil)
	recorder := pixelCutoutRequest(handlePixelCutoutStatus(s), http.MethodGet, "/api/pixel/cutout/status", nil)
	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status %d, content type %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	body := pixelCutoutDecode(t, recorder)
	if len(body) != 4 || body["available"] != true || body["runtime_ready"] != false || body["busy"] != false {
		t.Fatalf("body = %v", body)
	}
	models, _ := body["models"].([]any)
	if len(models) != 2 {
		t.Fatalf("models = %v", body["models"])
	}
	first := models[0].(map[string]any)
	if len(first) != 8 || first["id"] != "standard" || first["name"] != "ISNet general-use" || first["license"] != "Apache-2.0" ||
		first["size_bytes"] != float64(178648008) || first["ready"] != false || first["downloading"] != false ||
		first["progress"] != float64(0) || first["error"] != "" {
		t.Fatalf("first model = %v", first)
	}
	if second := models[1].(map[string]any); second["id"] != "precise" || second["license"] != "MIT" {
		t.Fatalf("second model = %v", second)
	}

	ready := newPixelCutoutServer(t, newPixelCutoutInstaller(true), &pixelCutoutRunner{}, nil)
	body = pixelCutoutDecode(t, pixelCutoutRequest(handlePixelCutoutStatus(ready), http.MethodGet, "/api/pixel/cutout/status", nil))
	model := body["models"].([]any)[1].(map[string]any)
	if body["runtime_ready"] != true || model["ready"] != true || model["progress"] != float64(1) {
		t.Fatalf("ready body = %v", body)
	}

	if post := pixelCutoutRequest(handlePixelCutoutStatus(s), http.MethodPost, "/api/pixel/cutout/status", "{}"); post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", post.Code)
	}
}

func TestPixelConfigReportsCutoutSupport(t *testing.T) {
	for goos, want := range map[string]bool{"windows": true, "plan9": false} {
		s := newPixelCutoutServer(t, newPixelCutoutInstaller(false), &pixelCutoutRunner{}, func(o *pixelmatte.Options) {
			o.GOOS, o.GOARCH = goos, "amd64"
		})
		recorder := httptest.NewRecorder()
		handlePixelConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/pixel/config", nil))
		if got := pixelCutoutDecode(t, recorder)["supports_cutout_ai"]; got != want {
			t.Errorf("%s: supports_cutout_ai = %v, want %v", goos, got, want)
		}
		status := pixelCutoutDecode(t, pixelCutoutRequest(handlePixelCutoutStatus(s), http.MethodGet, "/api/pixel/cutout/status", nil))
		if status["available"] != want {
			t.Errorf("%s: status available = %v, want %v", goos, status["available"], want)
		}
	}
}

func TestPixelCutoutPrepareIsSingleFlight(t *testing.T) {
	installer := newPixelCutoutInstaller(false)
	installer.gate = make(chan struct{})
	s := newPixelCutoutServer(t, installer, &pixelCutoutRunner{}, nil)
	prepare := handlePixelCutoutPrepare(s)

	first := pixelCutoutRequest(prepare, http.MethodPost, "/api/pixel/cutout/prepare", map[string]string{"model": "precise"})
	if first.Code != http.StatusAccepted || pixelCutoutDecode(t, first)["status"] != "downloading" {
		t.Fatalf("first prepare = %d %s", first.Code, first.Body.String())
	}
	<-installer.entered // the runtime download is running
	second := pixelCutoutRequest(prepare, http.MethodPost, "/api/pixel/cutout/prepare", map[string]string{"model": "precise"})
	if second.Code != http.StatusAccepted || pixelCutoutDecode(t, second)["status"] != "downloading" {
		t.Fatalf("second prepare = %d %s", second.Code, second.Body.String())
	}
	status := pixelCutoutDecode(t, pixelCutoutRequest(handlePixelCutoutStatus(s), http.MethodGet, "/api/pixel/cutout/status", nil))
	models := status["models"].([]any)
	if models[1].(map[string]any)["downloading"] != true || models[0].(map[string]any)["downloading"] != false {
		t.Fatalf("status while downloading = %v", status)
	}

	installer.gate <- struct{}{} // runtime done
	<-installer.entered          // model download running
	installer.gate <- struct{}{} // model done
	deadline := time.Now().Add(5 * time.Second)
	for {
		recorder := pixelCutoutRequest(prepare, http.MethodPost, "/api/pixel/cutout/prepare", map[string]string{"model": "precise"})
		if recorder.Code == http.StatusOK {
			if pixelCutoutDecode(t, recorder)["status"] != "ready" {
				t.Fatalf("ready prepare = %s", recorder.Body.String())
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("download never finished: %d %s", recorder.Code, recorder.Body.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	if runtimeCalls, modelCalls := installer.calls(); runtimeCalls != 1 || modelCalls != 1 {
		t.Fatalf("runtime downloaded %d times and model %d times, want once each", runtimeCalls, modelCalls)
	}
}

func TestPixelCutoutPrepareRejectsBadRequests(t *testing.T) {
	installer := newPixelCutoutInstaller(false)
	s := newPixelCutoutServer(t, installer, &pixelCutoutRunner{}, nil)
	prepare := handlePixelCutoutPrepare(s)
	for name, body := range map[string]any{"unknown model": map[string]string{"model": "huge"}, "no model": "{}", "not json": "nope"} {
		recorder := pixelCutoutRequest(prepare, http.MethodPost, "/api/pixel/cutout/prepare", body)
		if recorder.Code != http.StatusBadRequest || pixelCutoutDecode(t, recorder)["error"] == "" {
			t.Errorf("%s: %d %s", name, recorder.Code, recorder.Body.String())
		}
	}
	if recorder := pixelCutoutRequest(prepare, http.MethodGet, "/api/pixel/cutout/prepare", nil); recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET prepare = %d", recorder.Code)
	}
	if runtimeCalls, modelCalls := installer.calls(); runtimeCalls+modelCalls != 0 {
		t.Error("a bad request must not start a download")
	}

	unsupported := newPixelCutoutServer(t, newPixelCutoutInstaller(false), &pixelCutoutRunner{}, func(o *pixelmatte.Options) { o.GOOS = "plan9" })
	recorder := pixelCutoutRequest(handlePixelCutoutPrepare(unsupported), http.MethodPost, "/api/pixel/cutout/prepare", map[string]string{"model": "standard"})
	if recorder.Code != http.StatusConflict || pixelCutoutDecode(t, recorder)["status"] != "unavailable" {
		t.Errorf("unsupported platform: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPixelCutoutReturnsMatteOfTheSourceSize(t *testing.T) {
	runner := &pixelCutoutRunner{}
	s := newPixelCutoutServer(t, newPixelCutoutInstaller(true), runner, func(o *pixelmatte.Options) { o.Threads = 2 })
	recorder := pixelCutoutRequest(handlePixelCutout(s), http.MethodPost, "/api/pixel/cutout",
		map[string]string{"model": "precise", "source_data": pixelCutoutSource(t, 7, 5)})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	body := pixelCutoutDecode(t, recorder)
	if len(body) != 5 || body["status"] != "ok" || body["width"] != float64(7) || body["height"] != float64(5) {
		t.Fatalf("body keys/values = %v", body)
	}
	if _, ok := body["ms"].(float64); !ok {
		t.Fatalf("ms = %v", body["ms"])
	}
	mask, _ := body["mask"].(string)
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(mask, "data:image/png;base64,"))
	if err != nil || !strings.HasPrefix(mask, "data:image/png;base64,") {
		t.Fatalf("mask is not a PNG data URL: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if _, gray := decoded.(*image.Gray); !gray || decoded.Bounds().Dx() != 7 || decoded.Bounds().Dy() != 5 {
		t.Fatalf("mask is %T %v", decoded, decoded.Bounds())
	}
	if runner.last.Kind != pixelmatte.KindPrecise || runner.last.Threads != 2 || runner.last.Executable != "aurago-test" || len(runner.last.Image) == 0 {
		t.Fatalf("worker request = %+v", runner.last)
	}
}

func TestPixelCutoutNeedsPreparedModel(t *testing.T) {
	installer := newPixelCutoutInstaller(false)
	runner := &pixelCutoutRunner{}
	s := newPixelCutoutServer(t, installer, runner, nil)
	recorder := pixelCutoutRequest(handlePixelCutout(s), http.MethodPost, "/api/pixel/cutout",
		map[string]string{"model": "standard", "source_data": pixelCutoutSource(t, 4, 4)})
	if recorder.Code != http.StatusConflict || pixelCutoutDecode(t, recorder)["status"] != "not_ready" {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	if runner.calls != 0 {
		t.Fatal("the worker must not start")
	}
	if runtimeCalls, modelCalls := installer.calls(); runtimeCalls+modelCalls != 0 {
		t.Fatal("a cutout request must never download")
	}
}

func TestPixelCutoutAllowsOneRunAtATime(t *testing.T) {
	runner := &pixelCutoutRunner{started: make(chan struct{}, 1), release: make(chan struct{})}
	s := newPixelCutoutServer(t, newPixelCutoutInstaller(true), runner, nil)
	cutout := handlePixelCutout(s)
	body := map[string]string{"model": "standard", "source_data": pixelCutoutSource(t, 4, 4)}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- pixelCutoutRequest(cutout, http.MethodPost, "/api/pixel/cutout", body) }()
	<-runner.started
	status := pixelCutoutDecode(t, pixelCutoutRequest(handlePixelCutoutStatus(s), http.MethodGet, "/api/pixel/cutout/status", nil))
	if status["busy"] != true {
		t.Fatalf("status while running = %v", status)
	}
	busy := pixelCutoutRequest(cutout, http.MethodPost, "/api/pixel/cutout", body)
	if busy.Code != http.StatusTooManyRequests || pixelCutoutDecode(t, busy)["status"] != "busy" {
		t.Fatalf("second cutout = %d %s", busy.Code, busy.Body.String())
	}
	close(runner.release)
	if first := <-done; first.Code != http.StatusOK {
		t.Fatalf("first cutout = %d %s", first.Code, first.Body.String())
	}
}

func TestPixelCutoutRejectsBadInput(t *testing.T) {
	runner := &pixelCutoutRunner{}
	s := newPixelCutoutServer(t, newPixelCutoutInstaller(true), runner, nil)
	cutout := handlePixelCutout(s)
	valid := pixelCutoutSource(t, 4, 4)
	cases := map[string]any{
		"not json":       "nope",
		"no model":       map[string]string{"source_data": valid},
		"unknown model":  map[string]string{"model": "huge", "source_data": valid},
		"no source":      map[string]string{"model": "standard"},
		"bad base64":     map[string]string{"model": "standard", "source_data": "data:image/png;base64,@@@"},
		"not an image":   map[string]string{"model": "standard", "source_data": "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("hello"))},
		"gif is refused": map[string]string{"model": "standard", "source_data": "data:image/gif;base64,R0lGODlhAQABAAAAACw="},
	}
	for name, body := range cases {
		recorder := pixelCutoutRequest(cutout, http.MethodPost, "/api/pixel/cutout", body)
		if recorder.Code != http.StatusBadRequest || pixelCutoutDecode(t, recorder)["error"] == "" {
			t.Errorf("%s: %d %s", name, recorder.Code, recorder.Body.String())
		}
	}
	if recorder := pixelCutoutRequest(cutout, http.MethodGet, "/api/pixel/cutout", nil); recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET cutout = %d", recorder.Code)
	}
	if runner.calls != 0 {
		t.Fatalf("worker started %d times for bad input", runner.calls)
	}
}

func TestPixelCutoutFailuresDoNotLeakWorkerDetails(t *testing.T) {
	body := map[string]string{"model": "standard", "source_data": pixelCutoutSource(t, 4, 4)}

	failing := newPixelCutoutServer(t, newPixelCutoutInstaller(true), &pixelCutoutRunner{err: errors.New(`read model: open C:\secret\dir\model.onnx: denied`)}, nil)
	recorder := pixelCutoutRequest(handlePixelCutout(failing), http.MethodPost, "/api/pixel/cutout", body)
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("worker failure = %d %s", recorder.Code, recorder.Body.String())
	}

	wrongSize := newPixelCutoutServer(t, newPixelCutoutInstaller(true), &pixelCutoutRunner{badSize: true}, nil)
	recorder = pixelCutoutRequest(handlePixelCutout(wrongSize), http.MethodPost, "/api/pixel/cutout", body)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("matte of the wrong size = %d %s", recorder.Code, recorder.Body.String())
	}

	slow := newPixelCutoutServer(t, newPixelCutoutInstaller(true), &pixelCutoutRunner{started: make(chan struct{}, 1), release: make(chan struct{})},
		func(o *pixelmatte.Options) { o.Timeout = 30 * time.Millisecond })
	recorder = pixelCutoutRequest(handlePixelCutout(slow), http.MethodPost, "/api/pixel/cutout", body)
	if recorder.Code != http.StatusGatewayTimeout || pixelCutoutDecode(t, recorder)["status"] != "timeout" {
		t.Fatalf("timeout = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestPixelCutoutMutationsDenyReadonlyBeforeWork(t *testing.T) {
	installer := newPixelCutoutInstaller(false)
	runner := &pixelCutoutRunner{}
	s := newPixelCutoutServer(t, installer, runner, nil)
	s.Cfg.VirtualDesktop.ReadOnly = true
	for _, handler := range []http.HandlerFunc{handlePixelCutoutPrepare(s), handlePixelCutout(s)} {
		recorder := pixelCutoutRequest(handler, http.MethodPost, "/api/pixel/cutout", map[string]string{"model": "standard", "source_data": pixelCutoutSource(t, 4, 4)})
		if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "desktop_readonly") {
			t.Errorf("readonly mutation: %d %s", recorder.Code, recorder.Body.String())
		}
	}
	// Status stays readable.
	if recorder := pixelCutoutRequest(handlePixelCutoutStatus(s), http.MethodGet, "/api/pixel/cutout/status", nil); recorder.Code != http.StatusOK {
		t.Errorf("readonly status = %d", recorder.Code)
	}
	if runtimeCalls, modelCalls := installer.calls(); runtimeCalls+modelCalls+runner.calls != 0 {
		t.Error("nothing may start in a read-only desktop")
	}
}
