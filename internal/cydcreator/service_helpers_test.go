package cydcreator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// testClock advances one second per call so stored times are unique and
// ordered.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(time.Second)
	return c.t
}

// fakeBuilder implements Builder. build decides the outcome of each build;
// the default succeeds with consistent artefacts.
type fakeBuilder struct {
	mu        sync.Mutex
	status    ToolchainStatus
	requests  []BuildRequest
	build     func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error)
	pullCalls int
	pull      func(ctx context.Context) error
	a2l       []Addr2LineRequest
	frames    []Frame
}

func newFakeBuilder() *fakeBuilder {
	return &fakeBuilder{status: ToolchainStatus{DockerAvailable: true, Image: "test/cyd-builder:1", Present: true}}
}

func (f *fakeBuilder) Status(context.Context) ToolchainStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func (f *fakeBuilder) setStatus(st ToolchainStatus) {
	f.mu.Lock()
	f.status = st
	f.mu.Unlock()
}

func (f *fakeBuilder) Pull(ctx context.Context) error {
	f.mu.Lock()
	f.pullCalls++
	pull := f.pull
	f.mu.Unlock()
	if pull != nil {
		return pull(ctx)
	}
	return nil
}

func (f *fakeBuilder) Build(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	build := f.build
	f.mu.Unlock()
	if build != nil {
		return build(ctx, req, onLog)
	}
	onLog("Compiling .pio/build/" + req.Env + "/src/app/app_main.cpp.o")
	return successOutput(req), nil
}

func (f *fakeBuilder) Addr2Line(_ context.Context, req Addr2LineRequest) ([]Frame, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.a2l = append(f.a2l, req)
	return f.frames, nil
}

func (f *fakeBuilder) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeBuilder) request(i int) BuildRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[i]
}

// successOutput returns artefacts whose result.json hashes match.
func successOutput(req BuildRequest) BuildOutput {
	files := map[string][]byte{
		ArtifactMergedBin:   []byte("merged-" + req.BuildID),
		ArtifactFirmwareBin: []byte("firmware-" + req.BuildID),
		ArtifactFirmwareELF: []byte("elf-" + req.BuildID),
	}
	result := BuildResultFile{OK: true, Env: req.Env, Chip: ChipESP32, DurationMS: 1234, Warnings: 1,
		Flash: BuildResultUsage{Used: 1000, Total: 1966080}, RAM: BuildResultUsage{Used: 200, Total: 327680},
		Files: map[string]BuildResultArtifact{}}
	for name, data := range files {
		result.Files[name] = BuildResultArtifact{Size: int64(len(data)), SHA256: sha256Hex(data)}
	}
	return BuildOutput{ExitCode: 0, Result: result, Files: files,
		Log: "Compiling src/app/app_main.cpp\nsrc/app/app_main.cpp:3:1: warning: unused variable 'x'\n"}
}

// fakeGenerate validates like the real generator and returns a small tree
// that contains the app files and one line per asset.
func fakeGenerate(in GenerateInput) (GenerateOutput, error) {
	sizes := map[string]int64{}
	for _, a := range in.Project.Assets {
		bpp := int64(2)
		if a.Alpha {
			bpp = 3
		}
		sizes[a.ID] = int64(a.W) * int64(a.H) * bpp
	}
	issues := Validate(in.Project, ValidateOptions{AssetSizes: sizes})
	if HasErrors(issues) {
		return GenerateOutput{Issues: issues}, NewError(CodeInvalidLayout, "the layout has errors", map[string]any{"issues": issues})
	}
	files := map[string][]byte{
		"platformio.ini":              []byte("[env:" + in.Project.Board + "]\n"),
		"partitions.csv":              []byte("# partitions\n"),
		"include/board_config.h":      []byte("#define CYD_BOARD_ID \"" + in.Project.Board + "\"\n"),
		"src/generated/ui.cpp":        []byte("// " + in.Project.Name + "\n"),
		"lib/cyd_template/src/cyd.h":  []byte("#pragma once\n"),
		"src/generated/ui_assets.cpp": []byte(assetList(in.Assets)),
	}
	for rel, data := range in.AppFiles {
		files[rel] = data
	}
	return GenerateOutput{Files: files, Issues: issues}, nil
}

func assetList(assets map[string][]byte) string {
	ids := make([]string, 0, len(assets))
	for id, data := range assets {
		ids = append(ids, fmt.Sprintf("%s:%d", id, len(data)))
	}
	sort.Strings(ids)
	return "// assets " + strings.Join(ids, ",") + "\n"
}

type testServiceOptions struct {
	limits   ServiceLimits
	generate func(GenerateInput) (GenerateOutput, error)
	dir      string
	noWorker bool
}

type testEnv struct {
	svc     *Service
	builder *fakeBuilder
	dir     string
	cfg     ServiceConfig
}

func newTestService(t *testing.T, opts ...testServiceOptions) *testEnv {
	t.Helper()
	var o testServiceOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	dir := o.dir
	if dir == "" {
		dir = t.TempDir()
	}
	builder := newFakeBuilder()
	generate := o.generate
	if generate == nil {
		generate = fakeGenerate
	}
	cfg := ServiceConfig{
		WorkspaceDir: filepath.Join(dir, "workspace"),
		DataDir:      filepath.Join(dir, "data"),
		DBPath:       filepath.Join(dir, "data", "cyd_creator.db"),
		Builder:      builder,
		Generate:     generate,
		IDs:          &SeqIDs{},
		Now:          (&testClock{t: time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)}).Now,
		Limits:       o.limits,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return &testEnv{svc: svc, builder: builder, dir: dir, cfg: cfg}
}

func wantCode(t *testing.T, err error, code string) *Error {
	t.Helper()
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != code {
		t.Fatalf("error = %v, want %s", err, code)
	}
	return typed
}

func pngBytes(t *testing.T, w, h int, transparent bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	if transparent {
		img.Set(0, 0, color.NRGBA{A: 0})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// collectEvents drains a subscription until n events of the wanted types
// arrived or the timeout passed.
func collectEvents(t *testing.T, ch <-chan Event, n int, timeout time.Duration, types ...string) []Event {
	t.Helper()
	want := map[string]bool{}
	for _, typ := range types {
		want[typ] = true
	}
	var out []Event
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			if len(want) == 0 || want[ev.Type] {
				out = append(out, ev)
			}
		case <-deadline:
			t.Fatalf("got %d of %d events before the timeout: %+v", len(out), n, out)
		}
	}
	return out
}
