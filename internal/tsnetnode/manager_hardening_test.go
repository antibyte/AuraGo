package tsnetnode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"

	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
)

type testRecoveryFS struct {
	base      recoveryFileSystem
	lstat     func(string) (os.FileInfo, error)
	rename    func(string, string) error
	chmod     func(string, os.FileMode) error
	mkdir     func(string, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
	removeAll func(string) error
	writeFile func(string, []byte, os.FileMode) error
}

func (f *testRecoveryFS) Lstat(path string) (os.FileInfo, error) {
	if f.lstat != nil {
		return f.lstat(path)
	}
	return f.base.Lstat(path)
}

func (f *testRecoveryFS) Rename(oldPath, newPath string) error {
	if f.rename != nil {
		return f.rename(oldPath, newPath)
	}
	return f.base.Rename(oldPath, newPath)
}

func (f *testRecoveryFS) Chmod(path string, mode os.FileMode) error {
	if f.chmod != nil {
		return f.chmod(path, mode)
	}
	return f.base.Chmod(path, mode)
}

func (f *testRecoveryFS) Mkdir(path string, mode os.FileMode) error {
	if f.mkdir != nil {
		return f.mkdir(path, mode)
	}
	return f.base.Mkdir(path, mode)
}

func (f *testRecoveryFS) MkdirAll(path string, mode os.FileMode) error {
	if f.mkdirAll != nil {
		return f.mkdirAll(path, mode)
	}
	return f.base.MkdirAll(path, mode)
}

func (f *testRecoveryFS) RemoveAll(path string) error {
	if f.removeAll != nil {
		return f.removeAll(path)
	}
	return f.base.RemoveAll(path)
}

func (f *testRecoveryFS) WriteFile(path string, data []byte, mode os.FileMode) error {
	if f.writeFile != nil {
		return f.writeFile(path, data, mode)
	}
	return f.base.WriteFile(path, data, mode)
}

func (f *testRecoveryFS) ReadFile(path string) ([]byte, error) {
	return f.base.ReadFile(path)
}

func (f *testRecoveryFS) Remove(path string) error {
	return f.base.Remove(path)
}

type fakeNodeLocalClient struct {
	statuses        []*ipnstate.Status
	statusIndex     int
	getPrefsCalls   int
	startCalls      int
	startLoginCalls int
	lastAuthKey     string
	startErr        error
	startLoginErr   error
}

func (f *fakeNodeLocalClient) Status(context.Context) (*ipnstate.Status, error) {
	if len(f.statuses) == 0 {
		return nil, errors.New("no fake status configured")
	}
	index := f.statusIndex
	if index >= len(f.statuses) {
		index = len(f.statuses) - 1
	}
	f.statusIndex++
	return f.statuses[index], nil
}

func (f *fakeNodeLocalClient) GetPrefs(context.Context) (*ipn.Prefs, error) {
	f.getPrefsCalls++
	return ipn.NewPrefs(), nil
}

func (f *fakeNodeLocalClient) Start(_ context.Context, options ipn.Options) error {
	f.startCalls++
	f.lastAuthKey = options.AuthKey
	return f.startErr
}

func (f *fakeNodeLocalClient) StartLoginInteractive(context.Context) error {
	f.startLoginCalls++
	return f.startLoginErr
}

func TestAuthKeyForNodePriority(t *testing.T) {
	t.Setenv("TS_AUTHKEY", "tskey-auth-environment-value")

	cfg := &config.Config{}
	cfg.Tailscale.TsNet.AuthKey = "tskey-auth-shared-value"
	cfg.Tailscale.TsNet.AuthKeyMain = "tskey-auth-main-value"
	manager := NewManager(cfg, slog.Default())

	if key, source := manager.authKeyForNodeWithSource(NodeMain); key != "tskey-auth-main-value" || source != "node_vault" {
		t.Fatalf("main key = %q (%s), want node-specific Vault key", key, source)
	}
	if key, source := manager.authKeyForNodeWithSource(NodeManifest); key != "tskey-auth-shared-value" || source != "shared_vault" {
		t.Fatalf("manifest key = %q (%s), want shared Vault key", key, source)
	}

	cfgWithoutVault := &config.Config{}
	manager.UpdateConfig(cfgWithoutVault)
	if key, source := manager.authKeyForNodeWithSource(NodeSpaceAgent); key != "tskey-auth-environment-value" || source != "environment" {
		t.Fatalf("space-agent key = %q (%s), want environment fallback", key, source)
	}

	t.Setenv("TS_AUTHKEY", "")
	if key, source := manager.authKeyForNodeWithSource(NodeSpaceAgent); key != "" || source != "none" {
		t.Fatalf("empty key = %q (%s), want no credential", key, source)
	}
}

func TestOperationUsesImmutableConfigSnapshot(t *testing.T) {
	t.Setenv("TS_AUTHKEY", "tskey-auth-original-environment-value")
	original := &config.Config{}
	manager := NewManager(original, slog.Default())

	_, operationID, err := manager.beginOperation("start", NodeMain)
	if err != nil {
		t.Fatalf("beginOperation() error = %v", err)
	}
	updated := &config.Config{}
	updated.Tailscale.TsNet.AuthKeyMain = "tskey-auth-updated-value"
	manager.UpdateConfig(updated)
	t.Setenv("TS_AUTHKEY", "tskey-auth-updated-environment-value")

	if got := manager.authKeyForNode(NodeMain); got != "tskey-auth-original-environment-value" {
		t.Fatalf("active operation observed changed credential %q", got)
	}
	manager.finishOperation(operationID, nil)
	if got := manager.authKeyForNode(NodeMain); got != "tskey-auth-updated-value" {
		t.Fatalf("next operation observed credential %q, want updated value", got)
	}
}

func TestManagerDoesNotObserveExternalConfigMutation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Hostname = "snapshot-host"
	manager := NewManager(cfg, slog.Default())

	cfg.Tailscale.TsNet.Hostname = "externally-mutated-host"
	if got := manager.configSnapshot().Tailscale.TsNet.Hostname; got != "snapshot-host" {
		t.Fatalf("manager observed external config mutation: %q", got)
	}
	manager.UpdateConfig(cfg)
	if got := manager.configSnapshot().Tailscale.TsNet.Hostname; got != "externally-mutated-host" {
		t.Fatalf("UpdateConfig did not publish the new snapshot: %q", got)
	}
}

func TestConcurrentConfigUpdatesAndStatusUseValueSnapshots(t *testing.T) {
	manager := NewManager(&config.Config{}, slog.Default())
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for index := 0; index < 200; index++ {
			cfg := &config.Config{}
			cfg.Tailscale.TsNet.Enabled = index%2 == 0
			cfg.Tailscale.TsNet.Hostname = fmt.Sprintf("node-%d", index)
			cfg.Tailscale.TsNet.AuthKeyMain = fmt.Sprintf("tskey-auth-%d", index)
			manager.UpdateConfig(cfg)
		}
	}()
	for worker := 0; worker < 2; worker++ {
		go func() {
			defer wg.Done()
			for index := 0; index < 200; index++ {
				status := manager.GetStatus()
				if status.Nodes == nil {
					t.Error("status omitted node snapshots")
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestReauthenticateRejectsUnconfiguredNodeBeforeOperation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Enabled = true
	manager := NewManager(cfg, slog.Default())

	if _, err := manager.BeginReauthenticate(NodeManifest, "normal", false, http.NotFoundHandler()); err == nil ||
		classifyError(err) != ErrorNodeNotConfigured {
		t.Fatalf("BeginReauthenticate() error = %v, want %s", err, ErrorNodeNotConfigured)
	}
	if operation := manager.operationSnapshot(); operation != nil {
		t.Fatalf("disabled node registered operation %+v", operation)
	}
}

func TestConcurrentOperationsAreRejected(t *testing.T) {
	manager := NewManager(&config.Config{}, slog.Default())
	_, operationID, err := manager.beginOperation("start", NodeMain)
	if err != nil {
		t.Fatalf("beginOperation() error = %v", err)
	}
	defer manager.finishOperation(operationID, nil)

	if _, _, err := manager.beginOperation("reauth", NodeManifest); err == nil || classifyError(err) != ErrorOperationConflict {
		t.Fatalf("second beginOperation() error = %v, want %s", err, ErrorOperationConflict)
	}
}

func TestAuthenticateNodeHandlesLoginStatesPerNode(t *testing.T) {
	expiry := time.Now().Add(-time.Hour)
	for _, test := range []struct {
		name    string
		initial *ipnstate.Status
	}{
		{
			name:    "NoState",
			initial: &ipnstate.Status{BackendState: fmt.Sprint(ipn.NoState)},
		},
		{
			name:    "NeedsLogin",
			initial: &ipnstate.Status{BackendState: fmt.Sprint(ipn.NeedsLogin)},
		},
		{
			name: "expired node key",
			initial: &ipnstate.Status{
				BackendState: fmt.Sprint(ipn.Running),
				HaveNodeKey:  true,
				Self:         &ipnstate.PeerStatus{Expired: true, KeyExpiry: &expiry},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeNodeLocalClient{statuses: []*ipnstate.Status{
				test.initial,
				{BackendState: fmt.Sprint(ipn.NeedsLogin)},
				{BackendState: fmt.Sprint(ipn.Running), HaveNodeKey: true, Self: &ipnstate.PeerStatus{}},
			}}
			manager := NewManager(&config.Config{}, slog.Default())
			manager.localClientForServer = func(*tsnet.Server) (nodeLocalClient, error) { return client, nil }

			if err := manager.authenticateNode(context.Background(), NodeMain, &tsnet.Server{}, "tskey-auth-test-value", false); err != nil {
				t.Fatalf("authenticateNode() error = %v", err)
			}
			if client.getPrefsCalls != 1 || client.startCalls != 1 || client.startLoginCalls != 1 {
				t.Fatalf("calls = prefs:%d start:%d interactive:%d, want 1 each", client.getPrefsCalls, client.startCalls, client.startLoginCalls)
			}
			if client.lastAuthKey != "tskey-auth-test-value" {
				t.Fatalf("Start auth key = %q", client.lastAuthKey)
			}
		})
	}
}

func TestAuthenticateNodeLeavesHealthyStateUntouched(t *testing.T) {
	client := &fakeNodeLocalClient{statuses: []*ipnstate.Status{{
		BackendState: fmt.Sprint(ipn.Running),
		HaveNodeKey:  true,
		Self:         &ipnstate.PeerStatus{},
	}}}
	manager := NewManager(&config.Config{}, slog.Default())
	manager.localClientForServer = func(*tsnet.Server) (nodeLocalClient, error) { return client, nil }

	if err := manager.authenticateNode(context.Background(), NodeMain, &tsnet.Server{}, "", false); err != nil {
		t.Fatalf("healthy authenticateNode() error = %v", err)
	}
	if client.getPrefsCalls != 0 || client.startCalls != 0 || client.startLoginCalls != 0 {
		t.Fatalf("healthy node was reauthenticated: prefs:%d start:%d interactive:%d", client.getPrefsCalls, client.startCalls, client.startLoginCalls)
	}
}

func TestEnsureNodeAuthenticatedWaitsForTsnetStartup(t *testing.T) {
	client := &fakeNodeLocalClient{}
	manager := NewManager(&config.Config{}, slog.Default())
	manager.localClientForServer = func(*tsnet.Server) (nodeLocalClient, error) { return client, nil }
	manager.upForServer = func(context.Context, *tsnet.Server) (*ipnstate.Status, error) {
		return &ipnstate.Status{
			BackendState: fmt.Sprint(ipn.Running),
			HaveNodeKey:  true,
			Self:         &ipnstate.PeerStatus{},
		}, nil
	}

	if err := manager.ensureNodeAuthenticated(context.Background(), NodeMain, &tsnet.Server{}, "stale-auth-key"); err != nil {
		t.Fatalf("ensureNodeAuthenticated() error = %v", err)
	}
	if client.getPrefsCalls != 0 || client.startCalls != 0 || client.startLoginCalls != 0 {
		t.Fatalf("startup readiness attempted a second authentication: prefs:%d start:%d interactive:%d", client.getPrefsCalls, client.startCalls, client.startLoginCalls)
	}
}

func TestAuthenticateNodeClassifiesRejectedAuthKey(t *testing.T) {
	client := &fakeNodeLocalClient{statuses: []*ipnstate.Status{
		{BackendState: fmt.Sprint(ipn.NeedsLogin)},
		{BackendState: fmt.Sprint(ipn.NeedsLogin)},
		{BackendState: fmt.Sprint(ipn.NeedsLogin), Health: []string{"API key does not exist"}},
	}}
	manager := NewManager(&config.Config{}, slog.Default())
	manager.localClientForServer = func(*tsnet.Server) (nodeLocalClient, error) { return client, nil }

	err := manager.authenticateNode(context.Background(), NodeManifest, &tsnet.Server{}, "tskey-auth-rejected-value", false)
	if err == nil || classifyError(err) != ErrorAuthKeyRejected {
		t.Fatalf("authenticateNode() error = %v, want %s", err, ErrorAuthKeyRejected)
	}
}

func TestAuthenticateNodeDoesNotMisclassifyConnectionFailureAsRejectedKey(t *testing.T) {
	client := &fakeNodeLocalClient{
		statuses: []*ipnstate.Status{{BackendState: fmt.Sprint(ipn.NeedsLogin)}},
		startErr: errors.New("connection refused"),
	}
	manager := NewManager(&config.Config{}, slog.Default())
	manager.localClientForServer = func(*tsnet.Server) (nodeLocalClient, error) { return client, nil }

	err := manager.authenticateNode(context.Background(), NodeMain, &tsnet.Server{}, "tskey-auth-valid-value", false)
	if err == nil || classifyError(err) != ErrorBackendUnavailable {
		t.Fatalf("authenticateNode() error = %v, want %s", err, ErrorBackendUnavailable)
	}
}

func TestValidateRecoveryStatePathRejectsUnsafeTargetsAndSymlinks(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.VolumeName(workingDir) + string(os.PathSeparator),
		workingDir,
	} {
		if _, err := validateRecoveryStatePath(path); err == nil {
			t.Fatalf("validateRecoveryStatePath(%q) succeeded for unsafe target", path)
		}
	}

	root := t.TempDir()
	realParent := filepath.Join(root, "real")
	if err := os.Mkdir(realParent, 0o700); err != nil {
		t.Fatal(err)
	}
	linkParent := filepath.Join(root, "link")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	if _, err := validateRecoveryStatePath(filepath.Join(linkParent, "state")); err == nil {
		t.Fatal("validateRecoveryStatePath() accepted a path containing a symlink")
	}
}

func TestCleanupRecoveryBackupOnlyRemovesMarkedSibling(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "tsnet")
	backupDir := stateDir + ".recovery-1234"
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "state"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeRecoveryMarker(stateDir, backupDir); err != nil {
		t.Fatalf("writeRecoveryMarker() error = %v", err)
	}
	if err := cleanupRecoveryBackup(stateDir); err != nil {
		t.Fatalf("cleanupRecoveryBackup() error = %v", err)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatalf("backup still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(stateDir + recoveryMarkerSuffix); !os.IsNotExist(err) {
		t.Fatalf("marker still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(stateDir); err != nil {
		t.Fatalf("active state directory was removed: %v", err)
	}
}

func TestSafeErrorMessageDoesNotExposeCredentialOrPath(t *testing.T) {
	secret := "tskey-auth-do-not-expose"
	hostPath := filepath.Join("private", "tsnet", "state")
	err := errors.New("auth key " + secret + " rejected while reading " + hostPath)
	message := safeErrorMessage(err)
	if strings.Contains(message, secret) || strings.Contains(message, hostPath) {
		t.Fatalf("safe error leaked sensitive input: %q", message)
	}
	if message != "Tailscale rejected the configured auth key" {
		t.Fatalf("safe error = %q", message)
	}
}

func TestStatusNeverExposesCredentialsOrStatePaths(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.StateDir = filepath.Join("private", "tsnet-state-sentinel")
	cfg.Tailscale.TsNet.AuthKey = "tskey-auth-shared-status-secret"
	cfg.Tailscale.TsNet.AuthKeyMain = "tskey-auth-main-status-secret"
	manager := NewManager(cfg, slog.Default())

	data, err := json.Marshal(manager.GetStatus())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		cfg.Tailscale.TsNet.StateDir,
		cfg.Tailscale.TsNet.AuthKey,
		cfg.Tailscale.TsNet.AuthKeyMain,
	} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("status exposed %q: %s", forbidden, data)
		}
	}
}

func TestTimedOutListenerIsClosedWhenItArrivesLate(t *testing.T) {
	release := make(chan struct{})
	listener := newBlockingListener("late.tailnet.test:443")
	_, err := listenTLSWithFunction(
		context.Background(),
		&tsnet.Server{},
		":443",
		10*time.Millisecond,
		false,
		func(_ *tsnet.Server, _ string, _ time.Duration) (net.Listener, error) {
			<-release
			return listener, nil
		},
	)
	if err == nil || classifyError(err) != ErrorTimeout {
		t.Fatalf("listenTLSWithFunction() error = %v, want timeout", err)
	}
	close(release)
	select {
	case <-listener.closed:
	case <-time.After(time.Second):
		t.Fatal("late listener was not closed")
	}
}

func TestFunnelModeCreatesExactlyOneFunnelListener(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Funnel = true
	manager := NewManager(cfg, slog.Default())
	listener := newBlockingListener("funnel.tailnet.test:443")
	funnelCalls := 0
	tlsCalls := 0
	manager.listenFunnelForNode = func(_ context.Context, _ *tsnet.Server, addr string, _ time.Duration, _ bool) (net.Listener, error) {
		funnelCalls++
		if addr != ":443" {
			t.Fatalf("Funnel address = %q, want :443", addr)
		}
		return listener, nil
	}
	manager.listenTLSForNode = func(context.Context, *tsnet.Server, string, time.Duration, bool) (net.Listener, error) {
		tlsCalls++
		return nil, errors.New("TLS listener must not be created in Funnel mode")
	}

	if err := manager.startMainListener(context.Background(), &tsnet.Server{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})); err != nil {
		t.Fatalf("startMainListener() error = %v", err)
	}
	if funnelCalls != 1 || tlsCalls != 0 {
		t.Fatalf("listener calls = Funnel:%d TLS:%d, want Funnel:1 TLS:0", funnelCalls, tlsCalls)
	}
	if err := manager.stopMainListener(context.Background()); err != nil {
		t.Fatalf("stopMainListener() error = %v", err)
	}
}

func TestStopRuntimeClosesChildOnlyResources(t *testing.T) {
	manager := NewManager(&config.Config{}, slog.Default())
	listener := newBlockingListener("manifest.tailnet.test:443")
	manager.mu.Lock()
	manager.manifest = childResourceState{
		Generation: 7,
		Listener:   listener,
		State:      "ready",
	}
	manager.mu.Unlock()

	if err := manager.stopRuntime(context.Background()); err != nil {
		t.Fatalf("stopRuntime() error = %v", err)
	}
	select {
	case <-listener.closed:
	case <-time.After(time.Second):
		t.Fatal("child-only listener was not closed")
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.manifest.Listener != nil || manager.manifest.State != "" {
		t.Fatalf("child resources remained published: %+v", manager.manifest)
	}
}

func TestLateChildServeFailureCannotClearNewGeneration(t *testing.T) {
	manager := NewManager(&config.Config{}, slog.Default())
	oldServer := &http.Server{}
	newServer := &http.Server{}
	manager.mu.Lock()
	manager.manifest = childResourceState{
		Generation: 12,
		Server:     newServer,
		State:      "ready",
	}
	manager.mu.Unlock()

	manager.handleChildServeExit(NodeManifest, 11, oldServer, errors.New("old generation failed"))

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.manifest.Generation != 12 || manager.manifest.Server != newServer || manager.manifest.State != "ready" {
		t.Fatalf("late generation changed current resources: %+v", manager.manifest)
	}
}

func TestRecoveryMkdirFailureRestoresOriginalState(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "tsnet-state")
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(stateDir, "machine-key")
	if err := os.WriteFile(sentinel, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.StateDir = stateDir
	cfg.Tailscale.TsNet.AuthKeyMain = "tskey-auth-recovery-test-value"
	manager := NewManager(cfg, slog.Default())
	manager.setNodeError(NodeMain, errors.New("corrupt state"))
	base := osRecoveryFileSystem{}
	manager.recoveryFS = &testRecoveryFS{
		base: base,
		mkdirAll: func(path string, mode os.FileMode) error {
			if samePath(path, stateDir) {
				return errors.New("injected mkdir failure")
			}
			return base.MkdirAll(path, mode)
		},
	}

	err := manager.recoverNodeState(context.Background(), NodeMain, http.NotFoundHandler())
	if err == nil || classifyError(err) != ErrorStateCorrupt {
		t.Fatalf("recoverNodeState() error = %v, want %s", err, ErrorStateCorrupt)
	}
	data, readErr := os.ReadFile(sentinel)
	if readErr != nil {
		t.Fatalf("original state was not restored: %v", readErr)
	}
	if string(data) != "original" {
		t.Fatalf("restored state = %q", data)
	}
}

func TestRecoveryRollbackFailureRetainsBackup(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	backupDir := filepath.Join(root, "state.recovery-1")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	base := osRecoveryFileSystem{}
	manager := NewManager(&config.Config{}, slog.Default())
	manager.recoveryFS = &testRecoveryFS{
		base: base,
		rename: func(oldPath, newPath string) error {
			if samePath(oldPath, backupDir) {
				return errors.New("injected rollback rename failure")
			}
			return base.Rename(oldPath, newPath)
		},
	}

	if err := manager.restoreRecoveryBackup(stateDir, backupDir, 0o700); err == nil {
		t.Fatal("restoreRecoveryBackup() unexpectedly succeeded")
	}
	if _, err := os.Stat(backupDir); err != nil {
		t.Fatalf("backup was not retained after failed rollback: %v", err)
	}
}

func TestRecoveryRollbackRemoveAndChmodFailuresPreserveState(t *testing.T) {
	t.Run("remove", func(t *testing.T) {
		root := t.TempDir()
		stateDir := filepath.Join(root, "state")
		backupDir := filepath.Join(root, "state.recovery-1")
		if err := os.MkdirAll(stateDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(backupDir, 0o700); err != nil {
			t.Fatal(err)
		}
		base := osRecoveryFileSystem{}
		manager := NewManager(&config.Config{}, slog.Default())
		manager.recoveryFS = &testRecoveryFS{
			base: base,
			removeAll: func(path string) error {
				return errors.New("injected remove failure")
			},
		}
		if err := manager.restoreRecoveryBackup(stateDir, backupDir, 0o700); err == nil {
			t.Fatal("restoreRecoveryBackup() unexpectedly succeeded")
		}
		if _, err := os.Stat(backupDir); err != nil {
			t.Fatalf("backup was lost after remove failure: %v", err)
		}
	})

	t.Run("chmod", func(t *testing.T) {
		root := t.TempDir()
		stateDir := filepath.Join(root, "state")
		backupDir := filepath.Join(root, "state.recovery-1")
		if err := os.MkdirAll(backupDir, 0o700); err != nil {
			t.Fatal(err)
		}
		base := osRecoveryFileSystem{}
		manager := NewManager(&config.Config{}, slog.Default())
		manager.recoveryFS = &testRecoveryFS{
			base: base,
			chmod: func(path string, mode os.FileMode) error {
				return errors.New("injected chmod failure")
			},
		}
		if err := manager.restoreRecoveryBackup(stateDir, backupDir, 0o750); err == nil {
			t.Fatal("restoreRecoveryBackup() unexpectedly succeeded")
		}
		if _, err := os.Stat(backupDir); err != nil {
			t.Fatalf("backup was lost after chmod failure: %v", err)
		}
	})
}

func TestTsnetCallbackLogsSuppressSecretsAndRawBackendMessages(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	manager := NewManager(&config.Config{}, logger)
	callback := manager.makeLoginAwareLogFunc(NodeMain, true)
	secret := "tskey-auth-callback-secret"
	nodeKey := "nodekey:callback-secret"
	statePath := filepath.Join("private", "tsnet-state")
	loginURL := "https://login.tailscale.com/a/secret-login-code"

	callback("backend failed auth=%s node=%s state=%s", secret, nodeKey, statePath)
	callback("To authenticate, visit: %s", loginURL)

	logged := output.String()
	for _, forbidden := range []string{secret, nodeKey, statePath, loginURL, "backend failed"} {
		if strings.Contains(logged, forbidden) {
			t.Fatalf("callback log exposed %q: %s", forbidden, logged)
		}
	}
	status := manager.GetStatus()
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{secret, nodeKey, statePath} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("status exposed %q: %s", forbidden, encoded)
		}
	}
	if status.Nodes[NodeMain].LoginURL != loginURL {
		t.Fatalf("login URL was not retained in node status: %q", status.Nodes[NodeMain].LoginURL)
	}
}
