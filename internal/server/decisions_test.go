package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

type fakeDecisions struct {
	status    decisions.Status
	request   decisions.Request
	err       error
	calls     int
	policy    config.DecisionsConfig
	startGate *decisionsStartGate
}

// decisionsStartGate blocks the first Start after publishing a transitional
// status. A later Start returns nil and leaves that status alone, so a second
// HTTP handler that observed it would answer 202. Status stays readable.
type decisionsStartGate struct {
	mu      sync.Mutex
	release chan struct{}
	calls   int
}

func TestDecisionsDockerSelfIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, list string
		want       bool
	}{
		{"own container", selfListAlone, true},
		{"network provider is not proof", selfListShared("aurago", "aurago"), false},
		{"missing container", `[]`, false},
		{"invalid response", `{`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := newSelfIdentityServer(t, selfIdentityAPI{inspect: writeBody("{}"), list: writeBody(tc.list)})
			defer replaceContainerSelfProcFiles(map[string]string{})()
			id, err := resolveDecisionsDockerSelf(context.Background(), tools.DockerConfig{Host: s.Cfg.Docker.Host}, containerSelfSignals{ownID: selfContainerID})
			if tc.want && (err != nil || id != selfContainerID) {
				t.Fatalf("identity = %q, %v", id, err)
			}
			if !tc.want && (err == nil || id != "") {
				t.Fatalf("unproven container accepted: %q, %v", id, err)
			}
		})
	}
}

func (f *fakeDecisions) Status() decisions.Status {
	if f.startGate != nil {
		f.startGate.mu.Lock()
		defer f.startGate.mu.Unlock()
	}
	return f.status
}
func (f *fakeDecisions) Evaluate(ctx context.Context, r decisions.Request) (decisions.Response, error) {
	f.calls++
	f.request = r
	if err := ctx.Err(); err != nil {
		return decisions.Response{}, err
	}
	return decisions.Response{Calibration: "uncalibrated", Answers: map[string]decisions.Answer{"q": {Type: "noul", Decision: "yes", Confidence: 0.75, Probabilities: map[string]float64{"no": 0.25, "yes": 0.75}}}}, f.err
}
func (f *fakeDecisions) PrepareReconfigure(c config.DecisionsConfig) (func(context.Context) error, error) {
	f.policy = c
	return func(context.Context) error { return nil }, nil
}
func (f *fakeDecisions) AutoStart(context.Context) error { return nil }
func (f *fakeDecisions) Install(context.Context) error   { return nil }
func (f *fakeDecisions) Start(context.Context) error {
	if f.startGate == nil {
		return nil
	}
	f.startGate.mu.Lock()
	f.startGate.calls++
	if f.startGate.calls == 1 {
		f.status.State = "starting"
		f.status.Code = ""
		f.status.Ready = false
		release := f.startGate.release
		f.startGate.mu.Unlock()
		<-release
		return nil
	}
	f.startGate.mu.Unlock()
	return nil
}
func (f *fakeDecisions) Stop(context.Context) error     { return nil }
func (f *fakeDecisions) Shutdown(context.Context) error { return nil }

func decisionsServerFixture(t *testing.T) (*Server, *fakeDecisions) {
	t.Helper()
	base := t.TempDir()
	cfg := &config.Config{Decisions: config.DecisionsConfig{Enabled: true, Deployment: "native"}}
	cfg.Directories.WorkspaceDir = filepath.Join(base, "workspace")
	cfg.Directories.DataDir = filepath.Join(base, "data")
	cfg.Tools.DocumentCreator.OutputDir = filepath.Join(base, "documents")
	for _, path := range []string{cfg.Directories.WorkspaceDir, cfg.Directories.DataDir, cfg.Tools.DocumentCreator.OutputDir} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	fake := &fakeDecisions{status: decisions.Status{Enabled: true, Installed: true, Ready: true, State: "ready"}}
	return &Server{Cfg: cfg, Decisions: fake, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, fake
}

func TestDecisionsStartRejectsASecondStart(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	fake.startGate = &decisionsStartGate{release: make(chan struct{})}
	t.Cleanup(func() {
		select {
		case <-fake.startGate.release:
		default:
			close(fake.startGate.release)
		}
	})
	mux := http.NewServeMux()
	registerDecisionsRoutes(mux, s)
	post := func(action string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://localhost/api/decisions/"+action, nil)
		r.Header.Set("Origin", "http://localhost")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { firstDone <- post("start") }()
	deadline := time.Now().Add(2 * time.Second)
	for fake.Status().State != "starting" {
		if time.Now().After(deadline) {
			t.Fatal("first start did not publish a transitional state")
		}
		time.Sleep(time.Millisecond)
	}
	var first *httptest.ResponseRecorder
	select {
	case first = <-firstDone:
	case <-time.After(3 * time.Second):
		t.Fatal("first start did not answer")
	}
	if first.Code != http.StatusAccepted || strings.TrimSpace(first.Body.String()) != `{"status":"accepted"}` {
		t.Fatalf("first start = %d %s", first.Code, first.Body.String())
	}
	if fake.Status().State != "starting" {
		t.Fatalf("status before the second start = %q, want starting", fake.Status().State)
	}
	// The first Start is still blocked and the status a poll would accept is
	// already visible. A second handler must not turn that into 202.
	for _, action := range []string{"start", "install"} {
		second := post(action)
		if second.Code != http.StatusConflict {
			t.Fatalf("second %s = %d %s, want 409", action, second.Code, second.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["code"] != decisions.CodeLifecycleBusy {
			t.Fatalf("second %s code = %q, want lifecycle_busy", action, body["code"])
		}
	}
	fake.startGate.mu.Lock()
	calls := fake.startGate.calls
	fake.startGate.mu.Unlock()
	if calls != 1 {
		t.Fatalf("admission lock started %d Start calls, want 1", calls)
	}
}

func TestDecisionsAPIInputAndAccess(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	mux := http.NewServeMux()
	registerDecisionsRoutes(mux, s)
	valid := `{"state":"private input","questions":[{"id":"q","type":"noul","instructions":"Is it ready?"}]}`
	for _, tc := range []struct {
		method, action, body, origin, token string
		want                                int
	}{
		{"GET", "status", "", "", "", 200},
		{"POST", "evaluate", valid, "http://localhost", "", 200},
		{"POST", "test", "", "http://localhost", "", 200},
		{"GET", "evaluate", "", "http://localhost", "", 405},
		{"POST", "evaluate", valid, "http://attacker.invalid", "", 403},
		{"POST", "evaluate", valid, "", "", 403},
		{"POST", "evaluate", valid, "http://localhost", "invalid", 403},
		{"POST", "evaluate", valid + ` {}`, "http://localhost", "", 400},
		{"POST", "evaluate", `{"unknown":1}`, "http://localhost", "", 400},
		{"POST", "evaluate", `{"questions":[]}`, "http://localhost", "", 400},
		{"POST", "evaluate", `{"state":"` + strings.Repeat("x", 1<<20) + `"}`, "http://localhost", "", 400},
	} {
		r := httptest.NewRequest(tc.method, "http://localhost/api/decisions/"+tc.action, strings.NewReader(tc.body))
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s =%d want%d: %s", tc.method, tc.action, w.Code, tc.want, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private input") {
			t.Fatal("prompt echoed")
		}
	}
	if fake.calls != 2 {
		t.Fatalf("invalid request reached scorer: %d calls", fake.calls)
	}
	if !isAdminProtectedPath("/api/decisions/evaluate") {
		t.Fatal("missing bearer admin policy")
	}
	s.Cfg.Auth.Enabled = true
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/decisions/status", nil))
	if w.Code != 401 {
		t.Fatalf("unauthenticated status =%d", w.Code)
	}
}

func TestDecisionsBearerScopes(t *testing.T) {
	s, admin, desktop, read := newBearerSchemeTestServer(t)
	s.Cfg.Decisions.Enabled = true
	s.Decisions = &fakeDecisions{status: decisions.Status{State: "disabled"}}
	mux := http.NewServeMux()
	registerDecisionsRoutes(mux, s)
	for _, tc := range []struct {
		token string
		want  int
	}{{admin, 200}, {desktop, 403}, {read, 403}} {
		r := httptest.NewRequest("GET", "/api/decisions/status", nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		authMiddleware(s, mux).ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("scope gate =%d want%d: %s", w.Code, tc.want, w.Body.String())
		}
	}
}

func TestDecisionsRegisteredImagesStayWithinMediaRoots(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	db, err := tools.InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s.MediaRegistryDB = db
	var content bytes.Buffer
	if err := png.Encode(&content, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for index, path := range []string{filepath.Join(s.Cfg.Directories.DataDir, "image.png"), filepath.Join(t.TempDir(), "outside.png")} {
		if err := os.WriteFile(path, content.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		web := "/files/decisions-" + string(rune('a'+index)) + ".png"
		id, _, err := tools.RegisterMedia(db, tools.MediaItem{Filename: "image.png", FilePath: path, WebPath: web, MediaType: "image"})
		if err != nil {
			t.Fatal(err)
		}
		for _, ref := range []string{fmt.Sprintf("media:%d", id), web} {
			_, err := s.evaluateDecisions(context.Background(), decisionsAPIRequest{Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Red?"}}, Images: []string{ref}})
			if index == 0 && err != nil {
				t.Fatal(err)
			}
			if index == 1 && !errors.Is(err, errDecisionImage) {
				t.Fatal("registry granted access outside roots")
			}
		}
	}
	if fake.calls != 2 {
		t.Fatalf("wrong scorer calls: %d", fake.calls)
	}
}

func TestDecisionsImagesAreBoundedAndAuthorized(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, picture); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(s.Cfg.Directories.WorkspaceDir, "test.png")
	outside := filepath.Join(t.TempDir(), "outside.png")
	bad := filepath.Join(s.Cfg.Directories.WorkspaceDir, "bad.png")
	for _, path := range []string{good, outside} {
		if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(bad, []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	request := decisionsAPIRequest{State: "image state", Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Is it red?"}}, Images: []string{good}}
	if _, err := s.evaluateDecisions(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 || len(fake.request.Images) != 1 || !bytes.Equal(fake.request.Images[0].Data, buf.Bytes()) {
		t.Fatal("trusted bytes not passed to scorer")
	}
	for _, ref := range []string{outside, bad, "https://example.com/image.png", "media:42", "/files/arbitrary.png", filepath.Join(s.Cfg.Directories.WorkspaceDir, "..", "outside.png")} {
		request.Images = []string{ref}
		if _, err := s.evaluateDecisions(context.Background(), request); !errors.Is(err, errDecisionImage) {
			t.Fatalf("accepted unauthorized/invalid %q: %v", ref, err)
		}
	}
	request.Images = make([]string, 9)
	if _, err := s.evaluateDecisions(context.Background(), request); !errors.Is(err, errDecisionImage) {
		t.Fatalf("nine images accepted: %v", err)
	}
	if fake.calls != 1 {
		t.Fatal("invalid image reached scorer")
	}
	link := filepath.Join(s.Cfg.Directories.WorkspaceDir, "escape.png")
	if err := os.Symlink(outside, link); err == nil {
		request.Images = []string{link}
		if _, err := s.evaluateDecisions(context.Background(), request); !errors.Is(err, errDecisionImage) {
			t.Fatal("image symlink escaped workspace")
		}
	}
}

func TestDecisionsImageByteAndPixelBudgets(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(s.Cfg.Directories.WorkspaceDir, "large.png")
	if err := os.WriteFile(oversized, picture.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(oversized, decisionImageBytes+1); err != nil {
		t.Fatal(err)
	}
	tooManyPixels := append([]byte(nil), picture.Bytes()...)
	binary.BigEndian.PutUint32(tooManyPixels[16:20], decisionImagePixels+1)
	binary.BigEndian.PutUint32(tooManyPixels[29:33], crc32.ChecksumIEEE(tooManyPixels[12:29]))
	pixelPath := filepath.Join(s.Cfg.Directories.WorkspaceDir, "pixels.png")
	if err := os.WriteFile(pixelPath, tooManyPixels, 0600); err != nil {
		t.Fatal(err)
	}
	padded := filepath.Join(s.Cfg.Directories.WorkspaceDir, "padded.png")
	if err := os.WriteFile(padded, picture.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(padded, 9<<20); err != nil {
		t.Fatal(err)
	}
	for _, refs := range [][]string{{oversized}, {pixelPath}, {padded, padded, padded}} {
		_, err := s.evaluateDecisions(context.Background(), decisionsAPIRequest{Questions: []decisions.Question{{ID: "q", Type: "noul"}}, Images: refs})
		if !errors.Is(err, errDecisionImage) {
			t.Fatalf("image budget not enforced for %v: %v", refs, err)
		}
	}
	if fake.calls != 0 {
		t.Fatal("oversized images reached inference")
	}
}

func TestDecisionsWebPNormalizesWithoutNativeCodec(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	data, err := base64.StdEncoding.DecodeString("UklGRhwAAABXRUJQVlA4TA8AAAAvAAAAAAcQ/Y/+ByKi/wEA")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Cfg.Directories.WorkspaceDir, "red.webp")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = s.evaluateDecisions(context.Background(), decisionsAPIRequest{Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Red?"}}, Images: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.request.Images) != 1 || fake.request.Images[0].MIME != "image/png" {
		t.Fatal("WebP not normalized")
	}
	decoded, err := png.Decode(bytes.NewReader(fake.request.Images[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decoded.At(0, 0).RGBA()
	if r != 65535 || g != 0 || b != 0 {
		t.Fatal("image content changed")
	}
	// Small decoded images must not hide oversized aggregate source files.
	if err := os.Truncate(path, 9<<20); err != nil {
		t.Fatal(err)
	}
	_, err = s.evaluateDecisions(context.Background(), decisionsAPIRequest{Questions: []decisions.Question{{ID: "q", Type: "noul"}}, Images: []string{path, path, path}})
	if !errors.Is(err, errDecisionImage) || fake.calls != 1 {
		t.Fatalf("WebP source byte budget bypassed: calls=%d, error=%v", fake.calls, err)
	}
	limited := decisionImageBuffer{limit: 3}
	if _, err := limited.Write([]byte("four")); !errors.Is(err, errDecisionImage) || limited.Len() != 0 {
		t.Fatal("normalization exceeded byte budget")
	}
}

func TestDecisionsGIFUsesTheFirstFrameAsPNG(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	red := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.RGBA{R: 255, A: 255}})
	blue := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.RGBA{B: 255, A: 255}})
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{Image: []*image.Paletted{red, blue}, Delay: []int{0, 0}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Cfg.Directories.WorkspaceDir, "frames.gif")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := s.evaluateDecisions(context.Background(), decisionsAPIRequest{Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Red?"}}, Images: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.request.Images) != 1 || fake.request.Images[0].MIME != "image/png" || !bytes.HasPrefix(fake.request.Images[0].Data, []byte("\x89PNG")) {
		t.Fatal("GIF not reduced to one PNG frame")
	}
	decoded, err := png.Decode(bytes.NewReader(fake.request.Images[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decoded.At(0, 0).RGBA()
	if r != 65535 || g != 0 || b != 0 || fake.calls != 1 {
		t.Fatalf("first frame not kept: r=%d g=%d b=%d calls=%d", r, g, b, fake.calls)
	}

	oversized, oversizedFake := decisionsServerFixture(t)
	header := make([]byte, 13)
	copy(header, "GIF89a")
	binary.LittleEndian.PutUint16(header[6:8], 8192)
	binary.LittleEndian.PutUint16(header[8:10], 8192)
	pixelPath := filepath.Join(oversized.Cfg.Directories.WorkspaceDir, "huge.gif")
	if err := os.WriteFile(pixelPath, header, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = oversized.evaluateDecisions(context.Background(), decisionsAPIRequest{Questions: []decisions.Question{{ID: "q", Type: "noul"}}, Images: []string{pixelPath}})
	if !errors.Is(err, errDecisionImage) || oversizedFake.calls != 0 {
		t.Fatalf("oversized GIF logical screen accepted: calls=%d, error=%v", oversizedFake.calls, err)
	}
}

func TestDecisionsDisabledCancellationAndTechnicalErrors(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	request := flows.DecisionRequest{State: "state", Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Ready?"}}}
	s.Cfg.Decisions.Enabled = false
	if _, err := (flowDecisions{s}).Evaluate(context.Background(), request); err == nil || fake.calls != 0 {
		t.Fatal("disabled service reached model")
	}
	s.Cfg.Decisions.Enabled = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (flowDecisions{s}).Evaluate(ctx, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	fake.err = decisions.NewError(decisions.CodeCPUConsentRequired, nil)
	if _, err := (flowDecisions{s}).Evaluate(context.Background(), request); err == nil {
		t.Fatal("CPU denial became a decision")
	}
	for _, tc := range []struct{ code, flowCode string }{
		{decisions.CodeInferenceFailed, "FLOW_AI_UNAVAILABLE"},
		{decisions.CodeInvalidRuntimeResponse, "FLOW_AI_UNAVAILABLE"},
		{decisions.CodeRuntimeAttestationFailed, "FLOW_AI_UNAVAILABLE"},
		{decisions.CodeCandidateNotScorable, "FLOW_PARAM_INVALID"},
		{decisions.CodeQueueFull, "FLOW_NODE_FAILED"},
		{decisions.CodeRuntimeBusy, "FLOW_NODE_FAILED"},
	} {
		fake.err = decisions.NewError(tc.code, nil)
		_, err := (flowDecisions{s}).Evaluate(context.Background(), request)
		var nodeError *flows.NodeError
		if !errors.As(err, &nodeError) || nodeError.Code != tc.flowCode {
			t.Fatalf("flow mapping for %s: %v", tc.code, err)
		}
	}
	fake.err = decisions.NewError(decisions.CodeCPUConsentRequired, nil)
	for _, tc := range []struct {
		err    error
		status int
	}{
		{errDecisionImage, 400}, {context.DeadlineExceeded, 504}, {decisions.NewError("queue_full", nil), 429}, {fake.err, 503},
		{decisions.NewError(decisions.CodeCancelled, context.DeadlineExceeded), 504},
		{decisions.NewError(decisions.CodeCancelled, context.Canceled), 408},
		{decisions.NewError(decisions.CodeCandidateNotScorable, nil), 400},
	} {
		w := httptest.NewRecorder()
		writeDecisionsFailure(w, tc.err)
		if w.Code != tc.status {
			t.Fatalf("wrong error status: %d", w.Code)
		}
	}
}

func TestDecisionsLiveAvailabilityAndPolicyUpdate(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	env := newFlowCatalogEnv(s)
	if got := env.DecisionAvailability(); got.State != flows.AvailableState {
		t.Fatalf("%+v", got)
	}
	fake.status.Ready = false
	fake.status.Code = decisions.CodeCPUConsentRequired
	if got := env.DecisionAvailability(); got.Code != "DECISIONS_CPU_FALLBACK_DENIED" {
		t.Fatalf("stale availability: %+v", got)
	}
	previous := *s.Cfg
	previous.Decisions.CPUFallbackAllowed = true
	next := *s.Cfg
	next.Decisions.Enabled = false
	s.reconfigureDecisions(&previous, &next)
	if fake.policy.Enabled || fake.policy.CPUFallbackAllowed {
		t.Fatal("revoked policy not delivered")
	}
}

type slowDecisionsCleanup struct {
	*fakeDecisions
	started, release, finished chan struct{}
}

func (f *slowDecisionsCleanup) PrepareReconfigure(c config.DecisionsConfig) (func(context.Context) error, error) {
	f.policy = c
	return func(ctx context.Context) error {
		close(f.started)
		defer close(f.finished)
		select {
		case <-f.release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}, nil
}

func TestDecisionsRevocationDrainsOutsideConfigLock(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	slow := &slowDecisionsCleanup{fakeDecisions: fake, started: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
	s.Decisions = slow
	defer func() {
		close(slow.release)
		select {
		case <-slow.finished:
		case <-time.After(5 * time.Second):
			t.Error("cleanup did not finish")
		}
	}()
	next := *s.Cfg
	next.Decisions.Enabled = false
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(&next)
	s.CfgMu.Unlock()
	if fake.policy.Enabled || fake.policy.CPUFallbackAllowed {
		t.Fatal("revocation was not synchronous")
	}
	select {
	case <-slow.started:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not start")
	}
	read := make(chan struct{})
	go func() {
		s.CfgMu.RLock()
		s.CfgMu.RUnlock()
		close(read)
	}()
	select {
	case <-read:
	case <-time.After(time.Second):
		t.Fatal("runtime cleanup blocked config readers")
	}
}

func TestDecisionsConfigAPIRejectsInvalidAndPersistsRevocation(t *testing.T) {
	s := routerServerFixture(t)
	for _, allowed := range []bool{true, false} {
		body, _ := json.Marshal(map[string]any{"decisions": config.DecisionsConfig{Enabled: false, Deployment: "native", CPUFallbackAllowed: allowed}})
		w := httptest.NewRecorder()
		handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest("PUT", "/api/config", bytes.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("save: %d %s", w.Code, w.Body.String())
		}
		cfg, err := config.Load(s.Cfg.ConfigPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Decisions.CPUFallbackAllowed != allowed {
			t.Fatal("permission did not persist")
		}
	}
	before, _ := os.ReadFile(s.Cfg.ConfigPath)
	w := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"decisions":{"deployment":"remote"}}`)))
	if w.Code != 400 {
		t.Fatalf("invalid deployment accepted: %d %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(s.Cfg.ConfigPath)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid config was written")
	}
}
