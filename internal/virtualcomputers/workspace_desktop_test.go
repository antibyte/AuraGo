package virtualcomputers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type desktopTestTransport func(context.Context, string, string, interface{}, interface{}) error

func (f desktopTestTransport) Call(ctx context.Context, id, method string, p, out interface{}) error {
	return f(ctx, id, method, p, out)
}

func desktopTestManager(t *testing.T, transport WorkspaceTransport) (*WorkspaceManager, ToolConfig, WorkspaceIdentity, Workspace) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(WorkspaceControlPlaneStatus{ProtocolVersion: WorkspaceProtocolVersion, AssetFingerprint: WorkspaceAssetFingerprint()})
	}))
	t.Cleanup(s.Close)
	l, err := OpenLedger(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	m, err := NewWorkspaceManager(l, nil, WorkspaceManagerOptions{ClientFactory: func(ToolConfig) (*Client, error) { return NewClient(ClientConfig{BaseURL: s.URL}) }, TransportFactory: func(*Client) WorkspaceTransport { return transport }, ReconcileInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	cfg := ToolConfig{Enabled: true, ToolGate: true, NativeArtifactDir: t.TempDir(), AgentControl: AgentControlConfig{Enabled: true, NativeAppsEnabled: true, IdleTTLSeconds: 600, BrowserSessionsPerWorkspace: 1}}
	owner := WorkspaceIdentity{SessionID: "chat-a", MissionID: "mission-a", Actor: "agent"}
	now := time.Now().UTC()
	w := Workspace{ID: "workspace-a", MachineID: "vm-a", InstanceNonce: "instance-a", OwnerSessionID: owner.SessionID, MissionID: owner.MissionID, Template: "desktop", State: WorkspaceStateReady, ControlOwner: ControlOwnerAgent, Capabilities: []string{"desktop.native.v1", "desktop.control.v1"}, CreatedAt: now, UpdatedAt: now, LeaseExpiresAt: now.Add(time.Hour), MaxExpiresAt: now.Add(time.Hour)}
	if err = l.UpsertWorkspace(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	m.rememberConfig(cfg)
	return m, cfg, owner, w
}

func TestNativeDesktopGatesAndOwnership(t *testing.T) {
	for _, name := range []string{"disabled", "read_only", "foreign_chat", "foreign_mission", "python", "old_image", "expired", "unsupported"} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			m, cfg, owner, w := desktopTestManager(t, desktopTestTransport(func(context.Context, string, string, interface{}, interface{}) error { calls.Add(1); return nil }))
			op := "snapshot"
			switch name {
			case "disabled":
				cfg.AgentControl.NativeAppsEnabled = false
			case "read_only":
				cfg.ReadOnly = true
			case "foreign_chat":
				owner.SessionID = "other"
			case "foreign_mission":
				owner.MissionID = "other"
			case "python":
				w.Template = "python"
			case "old_image":
				w.Capabilities = nil
			case "expired":
				w.LeaseExpiresAt = time.Now().Add(-time.Second)
			case "unsupported":
				op = "sandbox"
			}
			if err := m.ledger.UpsertWorkspace(context.Background(), w); err != nil {
				t.Fatal(err)
			}
			if _, err := m.DesktopAction(context.Background(), cfg, owner, w.ID, DesktopActionRequest{Operation: op}); err == nil {
				t.Fatal("unauthorized action accepted")
			}
			if calls.Load() != 0 {
				t.Fatal("denial reached guest")
			}
		})
	}
}

func TestNativeDesktopTakeoverDrainsAndFencesQueuedBrowser(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	var browserCalls atomic.Int32
	tr := desktopTestTransport(func(ctx context.Context, id, method string, p, out interface{}) error {
		if method == "desktop.type" {
			close(started)
			<-ctx.Done()
			close(cancelled)
			<-release
			return ctx.Err()
		}
		if method == "browser.click" {
			browserCalls.Add(1)
		}
		return nil
	})
	m, cfg, owner, w := desktopTestManager(t, tr)
	first := make(chan error, 1)
	go func() {
		_, err := m.DesktopAction(context.Background(), cfg, owner, w.ID, DesktopActionRequest{Operation: "type", ElementRef: "@s1:e1", Text: "private input"})
		first <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("input did not start")
	}
	second := make(chan error, 1)
	go func() {
		_, err := m.BrowserAction(context.Background(), cfg, owner, w.ID, BrowserActionRequest{Operation: "click"})
		second <- err
	}()
	taken := make(chan error, 1)
	go func() {
		_, err := m.TakeControl(context.Background(), WorkspaceIdentity{Admin: true}, w.ID, true)
		taken <- err
	}()
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("takeover did not cancel input")
	}
	select {
	case <-taken:
		t.Fatal("takeover confirmed before input stopped")
	default:
	}
	close(release)
	if err := <-taken; err != nil {
		t.Fatal(err)
	}
	if err := <-first; err == nil {
		t.Fatal("cancelled input succeeded")
	}
	if err := <-second; err == nil {
		t.Fatal("queued input survived takeover")
	}
	if browserCalls.Load() != 0 {
		t.Fatal("browser received a late click")
	}
	events, err := m.ledger.ListWorkspaceEvents(context.Background(), w.ID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(events)
	if strings.Contains(string(raw), "private input") {
		t.Fatal("input leaked to activity log")
	}
}

func TestNativeDesktopLiveRevocationAndIndependentWorkspaces(t *testing.T) {
	m, cfg, owner, w := desktopTestManager(t, desktopTestTransport(func(context.Context, string, string, interface{}, interface{}) error { return nil }))
	ctx, _, finish, err := m.beginGUI(context.Background(), cfg, owner, w.ID, true, true)
	if err != nil {
		t.Fatal(err)
	}
	w2 := w
	w2.ID = "workspace-b"
	w2.MachineID = "vm-b"
	if err = m.ledger.UpsertWorkspace(context.Background(), w2); err != nil {
		t.Fatal(err)
	}
	_, _, finish2, err := m.beginGUI(context.Background(), cfg, owner, w2.ID, true, true)
	if err != nil {
		t.Fatal(err)
	}
	finish2()
	live := cfg
	live.AgentControl.NativeAppsEnabled = false
	m.SetGUIConfigSource(func() ToolConfig { return live })
	done := make(chan error, 1)
	go func() { done <- m.ApplyGUIConfig(context.Background(), cfg, live) }()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("disable did not cancel")
	}
	finish()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = m.beginGUI(context.Background(), cfg, owner, w.ID, true, true); err == nil {
		t.Fatal("stale feature snapshot bypassed live disable")
	}
}

func TestNativeDesktopFingerprintAndPinnedInstaller(t *testing.T) {
	data, err := workspaceAssets.ReadFile("guest_workspace_agent/install-native-desktop.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, pin := range []string{"3537152", "3407558", "ca77b6b75034eeb12e537dd8c0a4f2a3424119253f38b7e6bd79ba0d2fef1264", "438990a703fbeb19775596c341c3701435e80adbfa1b2f64b82ff17567d5573b", "sha256sum -c", "ge 2.52"} {
		if !strings.Contains(string(data), pin) {
			t.Errorf("missing pin %s", pin)
		}
	}
	script := (SetupManager{}).installScript()
	probe := strings.Index(script, "verify-native-image.sh\" \"${DESKTOP_CANDIDATE}")
	activate := strings.Index(script, "mv -f \"${DESKTOP_CANDIDATE}\"")
	if probe < 0 || activate < probe {
		t.Fatal("desktop candidate activates before real guest verification")
	}
}

func TestNativeDesktopToolRequiresExplicitCoordinatesAndValues(t *testing.T) {
	args := map[string]interface{}{"operation": "click_xy", "workspace_id": "w", "window_id": "window", "image_ref": "image", "x": 0, "y": 0}
	if err := validateDesktopToolFields(args); err != nil {
		t.Fatal(err)
	}
	delete(args, "x")
	if validateDesktopToolFields(args) == nil {
		t.Fatal("missing x became an implicit click at zero")
	}
	args = map[string]interface{}{"operation": "set_value", "workspace_id": "w", "element_ref": "@s1:e1", "value": ""}
	if err := validateDesktopToolFields(args); err != nil {
		t.Fatal("intentional empty value rejected", err)
	}
	args["value"] = nil
	if validateDesktopToolFields(args) == nil {
		t.Fatal("null value became an implicit field clear")
	}
}

func TestNativeDesktopImagesArePrivateFreshAndRevoked(t *testing.T) {
	m, cfg, owner, w := desktopTestManager(t, desktopTestTransport(func(context.Context, string, string, interface{}, interface{}) error { return nil }))
	path := filepath.Join(cfg.NativeArtifactDir, "capture-test.png")
	if err := os.WriteFile(path, []byte("private fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	g := m.guiFor(w.ID)
	g.image = &workspaceDesktopImage{ref: "image-a", path: path, nonce: w.InstanceNonce, created: time.Now()}
	handle := "native-desktop:image-a"
	if _, err := m.ResolveDesktopImage(context.Background(), owner, handle); err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []WorkspaceIdentity{{SessionID: "other", MissionID: owner.MissionID}, {SessionID: owner.SessionID, MissionID: "other"}} {
		if _, err := m.ResolveDesktopImage(context.Background(), foreign, handle); err == nil {
			t.Fatal("private capture crossed its owner boundary")
		}
	}
	g.image.created = time.Now().Add(-31 * time.Second)
	if _, err := m.ResolveDesktopImage(context.Background(), owner, handle); err == nil {
		t.Fatal("expired capture resolved")
	}
	g.image.created = time.Now()
	if _, err := m.TakeControl(context.Background(), WorkspaceIdentity{Admin: true}, w.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("capture survived human takeover")
	}
	if _, err := m.ResolveDesktopImage(context.Background(), owner, handle); err == nil {
		t.Fatal("revoked handle still resolved")
	}
}

func TestNativeDesktopDisableCancelsEveryWorkspaceBeforeGuestAcknowledgement(t *testing.T) {
	fail := false
	m, cfg, owner, w := desktopTestManager(t, desktopTestTransport(func(context.Context, string, string, interface{}, interface{}) error {
		if fail {
			return errors.New("guest unreachable")
		}
		return nil
	}))
	first, _, finish, err := m.beginGUI(context.Background(), cfg, owner, w.ID, true, true)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	w.ID = "second"
	w.MachineID = "vm-second"
	if err = m.ledger.UpsertWorkspace(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	second, _, finish2, err := m.beginGUI(context.Background(), cfg, owner, w.ID, true, true)
	if err != nil {
		t.Fatal(err)
	}
	defer finish2()
	fail = true
	live := cfg
	live.AgentControl.NativeAppsEnabled = false
	if m.ApplyGUIConfig(context.Background(), cfg, live) == nil {
		t.Fatal("unconfirmed cancellation acknowledged")
	}
	if first.Err() == nil || second.Err() == nil {
		t.Fatal("an unreachable guest delayed cancellation of another workspace")
	}
}

func TestNativeDesktopCloseCanTerminateUnreachableGuest(t *testing.T) {
	m, cfg, owner, w := desktopTestManager(t, desktopTestTransport(func(context.Context, string, string, interface{}, interface{}) error {
		return errors.New("guest unreachable")
	}))
	if err := m.CloseWorkspace(context.Background(), cfg, owner, w.ID); err != nil {
		t.Fatal(err)
	}
	stored, _, err := m.ledger.GetWorkspace(context.Background(), w.ID)
	if err != nil || stored.State != WorkspaceStateClosed {
		t.Fatalf("close did not finish: %v %v", stored.State, err)
	}
}
