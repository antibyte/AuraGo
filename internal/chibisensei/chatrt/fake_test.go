package chatrt

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/llamacpp"
)

// fakeLauncher is the in-package test double of Launcher. Its assets are the real install set of its platform.
type fakeLauncher struct {
	mu       sync.Mutex
	goos     string
	goarch   string
	hw       llamacpp.HardwareInfo
	verified bool

	installFn func(ctx context.Context, progress func(string, int64)) error
	launchFn  func(ctx context.Context, spec LaunchSpec) (Process, error)

	hardwareCalls int
	installCalls  int
	launches      []LaunchSpec
	procs         []*fakeProc
}

func newFakeLauncher(verified bool) *fakeLauncher {
	return &fakeLauncher{goos: "linux", goarch: "amd64", verified: verified}
}

func (f *fakeLauncher) Platform() (string, string) { return f.goos, f.goarch }

func (f *fakeLauncher) Hardware(ctx context.Context) llamacpp.HardwareInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hardwareCalls++
	if ctx.Err() != nil { // like the real probe: a cancelled context finds nothing
		return llamacpp.HardwareInfo{}
	}
	return f.hw
}

func (f *fakeLauncher) Assets(backend string) ([]llamacpp.AssetSpec, bool) {
	return installSet(f.goos, f.goarch, backend)
}

func (f *fakeLauncher) Verify(context.Context, string, []llamacpp.AssetSpec) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.verified
}

func (f *fakeLauncher) Install(ctx context.Context, _ string, _ []llamacpp.AssetSpec, progress func(string, int64)) error {
	f.mu.Lock()
	f.installCalls++
	fn := f.installFn
	f.mu.Unlock()
	if fn != nil {
		if err := fn(ctx, progress); err != nil {
			return err
		}
	}
	f.mu.Lock()
	f.verified = true
	f.mu.Unlock()
	return nil
}

func (f *fakeLauncher) Launch(ctx context.Context, spec LaunchSpec) (Process, error) {
	f.mu.Lock()
	f.launches = append(f.launches, spec)
	fn := f.launchFn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, spec)
	}
	return f.newProc(false), nil
}

func (f *fakeLauncher) newProc(gpu bool) *fakeProc {
	p := &fakeProc{gpu: gpu, done: make(chan struct{})}
	f.mu.Lock()
	f.procs = append(f.procs, p)
	f.mu.Unlock()
	return p
}

func (f *fakeLauncher) launchSpecs() []LaunchSpec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]LaunchSpec(nil), f.launches...)
}

func (f *fakeLauncher) installs() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.installCalls
}

// fakeProc answers a scripted reply; chatFn replaces the whole answer.
type fakeProc struct {
	gpu    bool
	done   chan struct{}
	once   sync.Once
	chatFn func(ctx context.Context, req ChatRequest, onDelta func(string)) (ChatStats, error)

	mu      sync.Mutex
	stopped int
}

func (p *fakeProc) Chat(ctx context.Context, req ChatRequest, onDelta func(string)) (ChatStats, error) {
	if p.chatFn != nil {
		return p.chatFn(ctx, req, onDelta)
	}
	onDelta("ok")
	return ChatStats{FinishReason: "stop", PromptTokens: 3, CompletionTokens: 1}, nil
}
func (p *fakeProc) GPUVerified() bool     { return p.gpu }
func (p *fakeProc) Done() <-chan struct{} { return p.done }
func (p *fakeProc) Stop() error {
	p.once.Do(func() {
		p.mu.Lock()
		p.stopped++
		p.mu.Unlock()
		close(p.done)
	})
	return nil
}
func (p *fakeProc) isStopped() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// fixture bundles a manager with its fakes.
type fixture struct {
	t     *testing.T
	m     *Manager
	l     *fakeLauncher
	dir   string
	mu    sync.Mutex
	cfg   Config
	clock time.Time
}

func newFixture(t *testing.T, l *fakeLauncher) *fixture {
	t.Helper()
	dir := t.TempDir()
	fx := &fixture{t: t, l: l, dir: dir, clock: time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC),
		cfg: Config{Enabled: true, Backend: "cpu", IdleMinutes: 10, ContextSize: 4096, CacheDir: filepath.Join(dir, "runtime")}}
	fx.m = NewManager(fx.config, l, slog.New(slog.NewTextHandler(io.Discard, nil)))
	fx.m.now = fx.now
	fx.m.freeDisk = func(string) (int64, error) { return 1 << 40, nil }
	t.Cleanup(fx.m.Close)
	return fx
}

func (fx *fixture) config() Config {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	return fx.cfg
}

func (fx *fixture) setConfig(edit func(*Config)) {
	fx.mu.Lock()
	edit(&fx.cfg)
	fx.mu.Unlock()
}

func (fx *fixture) now() time.Time {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	return fx.clock
}

func (fx *fixture) advance(d time.Duration) {
	fx.mu.Lock()
	fx.clock = fx.clock.Add(d)
	fx.mu.Unlock()
}

// model writes a small file and returns its reference.
func (fx *fixture) model(key string) ModelRef {
	fx.t.Helper()
	path := filepath.Join(fx.dir, strings.NewReplacer(":", "_", "/", "_").Replace(key)+".gguf")
	data := []byte("GGUF-test-" + key)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fx.t.Fatal(err)
	}
	return ModelRef{Key: key, Path: path, Bytes: int64(len(data))}
}

func (fx *fixture) acquire(key string) *Instance {
	fx.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lease, err := fx.m.Acquire(ctx, fx.model(key))
	if err != nil {
		fx.t.Fatalf("Acquire(%s): %v", key, err)
	}
	return lease
}

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}
