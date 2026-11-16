package virtualcomputers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNativeDesktopRegressionBrowserPreservesRequestedTimeout(t *testing.T) {
	var remaining time.Duration
	m, cfg, owner, w := desktopTestManager(t, desktopTestTransport(func(ctx context.Context, id, method string, p, out interface{}) error {
		if method == "browser.wait" {
			deadline, ok := ctx.Deadline()
			if !ok {
				remaining = 24 * time.Hour
			} else {
				remaining = time.Until(deadline)
			}
			return errors.New("review stop after observing deadline")
		}
		return nil
	}))
	cfg.AgentControl.NativeAppsEnabled = false
	m.rememberConfig(cfg)
	_, _ = m.BrowserAction(context.Background(), cfg, owner, w.ID, BrowserActionRequest{Operation: "wait", TimeoutMS: 90000})
	if remaining < 89*time.Second {
		t.Fatalf("requested 90s browser wait while native apps disabled, but effective deadline is %v", remaining)
	}
}

func TestNativeDesktopRegressionDisableFailureKeepsOtherBrowsersUsable(t *testing.T) {
	var mu sync.Mutex
	failControls := false
	failedID := ""
	tr := desktopTestTransport(func(ctx context.Context, id, method string, p, out interface{}) error {
		mu.Lock()
		defer mu.Unlock()
		if failControls && method == "desktop.control" {
			if failedID == "" {
				failedID = id
			}
			if id == failedID {
				return errors.New("review: this guest is unreachable")
			}
		}
		return nil
	})
	m, cfg, owner, w := desktopTestManager(t, tr)
	workspaces := []Workspace{w}
	for _, id := range []string{"b", "c"} {
		other := w
		other.ID = "workspace-" + id
		other.MachineID = "vm-" + id
		if err := m.ledger.UpsertWorkspace(context.Background(), other); err != nil {
			t.Fatal(err)
		}
		workspaces = append(workspaces, other)
	}
	for _, w := range workspaces {
		_, _, finish, err := m.beginGUI(context.Background(), cfg, owner, w.ID, false, false)
		if err != nil {
			t.Fatal(err)
		}
		finish()
	}
	live := cfg
	live.AgentControl.NativeAppsEnabled = false
	m.SetGUIConfigSource(func() ToolConfig { return live })
	mu.Lock()
	failControls = true
	mu.Unlock()
	if err := m.ApplyGUIConfig(context.Background(), cfg, live); err == nil {
		t.Fatal("expected unreachable guest error")
	}
	mu.Lock()
	bad := failedID
	mu.Unlock()
	for _, w := range workspaces {
		if w.MachineID == bad {
			continue
		}
		_, _, finish, err := m.beginGUI(context.Background(), live, owner, w.ID, false, false)
		if err != nil {
			t.Errorf("healthy %s browser remains blocked after native disable failure in %s: %v", w.ID, bad, err)
		} else {
			finish()
		}
	}
	off := live
	live = cfg
	if err := m.ApplyGUIConfig(context.Background(), off, live); err == nil {
		t.Fatal("unreachable guest unexpectedly recovered")
	}
	for _, w := range workspaces {
		if w.MachineID == bad {
			continue
		}
		_, _, finish, err := m.beginGUI(context.Background(), live, owner, w.ID, false, false)
		if err != nil {
			t.Errorf("reenabling still leaves healthy %s blocked: %v", w.ID, err)
		} else {
			finish()
		}
	}
	mu.Lock()
	failControls = false
	mu.Unlock()
	if err := m.ApplyGUIConfig(context.Background(), live, live); err != nil {
		t.Fatal(err)
	}
	for _, w := range workspaces {
		_, _, finish, err := m.beginGUI(context.Background(), live, owner, w.ID, false, false)
		if err != nil {
			t.Fatalf("restored guest did not recover: %v", err)
		}
		finish()
	}

}

func nativeScreenshotTestClient(t *testing.T, m *WorkspaceManager) {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/screenshot") {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(b.Bytes())
			return
		}
		_ = json.NewEncoder(w).Encode(WorkspaceControlPlaneStatus{ProtocolVersion: WorkspaceProtocolVersion, AssetFingerprint: WorkspaceAssetFingerprint()})
	}))
	t.Cleanup(server.Close)
	m.clientFactory = func(ToolConfig) (*Client, error) { return NewClient(ClientConfig{BaseURL: server.URL}) }
}

func TestNativeDesktopRegressionTakeoverInvalidatesCaptureBeingPublished(t *testing.T) {
	commitReached := make(chan struct{})
	tr := desktopTestTransport(func(ctx context.Context, id, method string, p, out interface{}) error {
		switch method {
		case "desktop.screenshot_prepare":
			return json.Unmarshal([]byte(`{"image_ref":"review-capture","width":16,"height":16}`), out)
		case "desktop.screenshot_commit":
			// The successful response is available, but this caller loses the CPU before it returns.
			close(commitReached)
			<-ctx.Done()
			return nil
		}
		return nil
	})
	m, cfg, owner, w := desktopTestManager(t, tr)
	nativeScreenshotTestClient(t, m)
	result := make(chan map[string]interface{}, 1)
	actionErr := make(chan error, 1)
	go func() {
		r, err := m.DesktopAction(context.Background(), cfg, owner, w.ID, DesktopActionRequest{Operation: "screenshot", WindowID: "window-a"})
		result <- r
		actionErr <- err
	}()
	select {
	case <-commitReached:
	case <-time.After(time.Second * 5):
		t.Fatal("capture never reached publication boundary")
	}
	if _, err := m.TakeControl(context.Background(), WorkspaceIdentity{Admin: true}, w.ID, true); err != nil {
		t.Fatal(err)
	}
	r := <-result
	if err := <-actionErr; err != nil {
		t.Logf("capture returned cancellation: %v", err)
	}
	handle, _ := r["file_path"].(string)
	if path, err := m.ResolveDesktopImage(context.Background(), owner, handle); err == nil {
		if _, statErr := os.Stat(path); statErr == nil {
			t.Fatalf("after takeover acknowledged, cancelled capture is still readable by the agent: %s", handle)
		}
	}
}

func TestNativeDesktopRegressionManagerCloseRemovesPrivateImages(t *testing.T) {
	tr := desktopTestTransport(func(ctx context.Context, id, method string, p, out interface{}) error {
		if method == "desktop.screenshot_prepare" {
			return json.Unmarshal([]byte(`{"image_ref":"review-close","width":16,"height":16}`), out)
		}
		return nil
	})
	m, cfg, owner, w := desktopTestManager(t, tr)
	nativeScreenshotTestClient(t, m)
	result, err := m.DesktopAction(context.Background(), cfg, owner, w.ID, DesktopActionRequest{Operation: "screenshot", WindowID: "window-a"})
	if err != nil {
		t.Fatal(err)
	}
	path, err := m.ResolveDesktopImage(context.Background(), owner, result["file_path"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("private capture remains after manager shutdown; process exit also removes its only cleanup timer")
	}
}

func TestNativeDesktopStartupRemovesOrphansWithoutTouchingOtherFiles(t *testing.T) {
	m, cfg, _, _ := desktopTestManager(t, desktopTestTransport(func(context.Context, string, string, interface{}, interface{}) error { return nil }))
	orphan := filepath.Join(cfg.NativeArtifactDir, "capture-orphan.png")
	keep := filepath.Join(cfg.NativeArtifactDir, "unrelated.txt")
	for _, path := range []string{orphan, keep} {
		if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	nested := filepath.Join(cfg.NativeArtifactDir, "capture-directory.png")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	m.SetGUIConfigSource(func() ToolConfig { return cfg })
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("startup retained orphaned capture")
	}
	for _, path := range []string{keep, nested} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("startup removed an unowned path: %v", err)
		}
	}
	live := filepath.Join(cfg.NativeArtifactDir, "capture-live.png")
	if err := os.WriteFile(live, []byte("current capture"), 0600); err != nil {
		t.Fatal(err)
	}
	m.SetGUIConfigSource(func() ToolConfig { return cfg })
	if _, err := os.Stat(live); err != nil {
		t.Fatal("config rebinding deleted current capture")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	// A new manager represents restart after an abrupt exit left an untracked file.
	next, nextCfg, _, _ := desktopTestManager(t, desktopTestTransport(func(context.Context, string, string, interface{}, interface{}) error { return nil }))
	nextCfg.NativeArtifactDir = cfg.NativeArtifactDir
	next.SetGUIConfigSource(func() ToolConfig { return nextCfg })
	if _, err := os.Stat(live); !os.IsNotExist(err) {
		t.Fatal("restart retained orphaned capture")
	}
}

func TestNativeDesktopCleanupDoesNotFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.png")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "capture-link.png")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := removeOrphanDesktopCaptures(dir); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "keep" {
		t.Fatal("capture cleanup followed a symlink")
	}
	linkedDir := filepath.Join(t.TempDir(), "capture-dir")
	if err := os.Symlink(dir, linkedDir); err != nil {
		t.Fatal(err)
	}
	if err := removeOrphanDesktopCaptures(linkedDir); err == nil {
		t.Fatal("symlinked capture directory accepted")
	}
}

func TestNativeDesktopSlowRevocationDoesNotHoldOtherWorkspaces(t *testing.T) {
	var revoking atomic.Bool
	var first sync.Once
	badID := ""
	started := make(chan struct{})
	healthy := make(chan struct{}, 1)
	resume := make(chan struct{})
	tr := desktopTestTransport(func(ctx context.Context, id, method string, _, _ interface{}) error {
		if method != "desktop.control" || !revoking.Load() {
			return nil
		}
		first.Do(func() { badID = id; close(started) })
		if id == badID {
			select {
			case <-resume:
			case <-ctx.Done():
			}
			return errors.New("unreachable guest")
		}
		healthy <- struct{}{}
		return nil
	})
	m, cfg, owner, a := desktopTestManager(t, tr)
	b := a
	b.ID, b.MachineID = "workspace-b", "vm-b"
	if err := m.ledger.UpsertWorkspace(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	for _, w := range []Workspace{a, b} {
		_, _, finish, err := m.beginGUI(context.Background(), cfg, owner, w.ID, false, false)
		if err != nil {
			t.Fatal(err)
		}
		finish()
	}
	live := cfg
	live.AgentControl.NativeAppsEnabled = false
	m.SetGUIConfigSource(func() ToolConfig { return live })
	revoking.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- m.ApplyGUIConfig(ctx, cfg, live) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("barrier did not start")
	}
	select {
	case <-healthy:
	case <-ctx.Done():
		t.Fatal("slow guest prevented healthy guest revocation")
	}
	revoking.Store(false)
	good := a
	if good.MachineID == badID {
		good = b
	}
	_, _, finish, err := m.beginGUI(ctx, live, owner, good.ID, false, false)
	if err != nil {
		t.Fatalf("healthy browser stayed blocked: %v", err)
	}
	finish()
	select {
	case <-done:
		t.Fatal("slow guest was not awaited")
	default:
	}
	close(resume)
	if err := <-done; err == nil {
		t.Fatal("unreachable guest error was lost")
	}
}
