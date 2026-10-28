package rustfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
	"github.com/minio/minio-go/v7"
)

type memoryVault struct {
	mu     sync.Mutex
	values map[string]string
}

func (v *memoryVault) ReadSecret(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	value, ok := v.values[key]
	if !ok {
		return "", security.ErrSecretNotFound
	}
	return value, nil
}
func (v *memoryVault) WriteSecret(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.values == nil {
		v.values = map[string]string{}
	}
	v.values[key] = value
	return nil
}
func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.S3.Enabled = true
	cfg.S3.Provider = config.S3ProviderRustFS
	cfg.RustFS = config.RustFSConfig{Enabled: true, Bucket: "aurago", AgentAccess: true, Permissions: config.RustFSPermissions{AllowRead: true, AllowWrite: true, AllowDelete: true, AllowedLocations: []config.RustFSLocation{{Bucket: "aurago", Prefixes: []string{"agent/"}}}}}
	cfg.Docker.Enabled = true
	return cfg
}
func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func closeManager(t *testing.T, m *Manager) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := m.Shutdown(ctx); err != nil {
		t.Error(err)
	}
}
func TestPermissionIntersection(t *testing.T) {
	a := config.RustFSPermissions{AllowRead: true, AllowWrite: true, AllowedLocations: []config.RustFSLocation{{Bucket: "aurago", Prefixes: []string{"agent/", "other/"}}}}
	b := config.RustFSPermissions{AllowRead: true, AllowDelete: true, AllowedLocations: []config.RustFSLocation{{Bucket: "aurago", Prefixes: []string{"agent/private/", "other"}}, {Bucket: "foreign", Prefixes: []string{""}}}}
	p := intersectPermissions(a, b)
	for _, test := range []struct {
		bucket, key, access string
		want                bool
	}{{"aurago", "agent/private/x", "read", true}, {"aurago", "agent/x", "read", false}, {"aurago", "other/x", "read", true}, {"foreign", "x", "read", false}, {"aurago", "agent/private/x", "write", false}, {"aurago", "agent/private/x", "delete", false}} {
		if got := p.Allows(test.bucket, test.key, test.access); got != test.want {
			t.Errorf("%+v: %v", test, got)
		}
	}
	p.AllowedLocations[0].Prefixes[0] = "changed"
	if a.AllowedLocations[0].Prefixes[0] != "agent/" {
		t.Fatal("intersection aliases source")
	}
}
func TestPolicyUsesSeparatedFamiliesAndLiteralBoundaries(t *testing.T) {
	cfg := testConfig()
	var p policyDocument
	if err := json.Unmarshal(generatePolicy(effectivePermissions(cfg)), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Statement) < 4 || p.Statement[0].Action[0] != "admin:*" || p.Statement[1].Action[0] != "sts:*" {
		t.Fatal("missing separate administrative denies")
	}
	for _, s := range p.Statement {
		for _, a := range s.Action {
			if s.Effect == "Allow" && (a == "s3:*" || a == "s3:ListAllMyBuckets") {
				t.Fatal("overbroad grant")
			}
		}
		for _, r := range s.Resource {
			if strings.Contains(r, "/agent*") {
				t.Fatal("subtree boundary lost")
			}
		}
	}
	cfg.S3.ReadOnly = true
	p = policyDocument{}
	_ = json.Unmarshal(generatePolicy(effectivePermissions(cfg)), &p)
	for _, s := range p.Statement {
		for _, a := range s.Action {
			if a == "s3:PutObject" || a == "s3:DeleteObject" {
				t.Fatal("readonly policy permits mutation")
			}
		}
	}
	cfg.RustFS.AgentAccess = false
	_ = json.Unmarshal(generatePolicy(effectivePermissions(cfg)), &p)
	if len(p.Statement) != 2 {
		t.Fatal("disabled access kept S3 allows")
	}
}
func TestExternalLeaseFencesProviderAndPublication(t *testing.T) {
	cfg := testConfig()
	cfg.S3.Provider = config.S3ProviderExternal
	cfg.S3.Endpoint = "https://storage.example"
	cfg.S3.AccessKey = "unit-access"
	cfg.S3.SecretKey = "unit-secret"
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), nil, nil, defaultNames())
	defer closeManager(t, m)
	lease, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	unchanged := copyConfig(cfg)
	unchanged.Server.Host = "changed-unrelated"
	m.Configure(unchanged)
	if err = lease.Check(); err != nil {
		t.Fatal("unrelated config cancelled lease")
	}
	changed := copyConfig(cfg)
	changed.S3.Provider = config.S3ProviderRustFS
	m.Configure(changed)
	if err = lease.Check(); ErrorCode(err) != "stale_target" {
		t.Fatalf("stale lease: %v", err)
	}
	published := false
	if err = lease.Publish(func() error { published = true; return nil }); err == nil || published {
		t.Fatal("stale publication ran")
	}
	if _, err = m.AcquireS3(context.Background(), cfg, true); ErrorCode(err) != "stale_target" {
		t.Fatal("old provider snapshot accepted")
	}
}
func TestNativeAdminSignsAndRejectsRedirect(t *testing.T) {
	var received bool
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received = true }))
	defer foreign.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") || !strings.Contains(r.Header.Get("Authorization"), "/us-east-1/s3/aws4_request") {
			t.Error("missing S3 SigV4")
		}
		if r.URL.Path != "/rustfs/admin/v3/add-user" {
			t.Error("wrong native prefix")
		}
		http.Redirect(w, r, foreign.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	admin, err := newAdminClient(server.URL, "unit-access", "unit-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.native(context.Background(), http.MethodPut, "add-user", nil, map[string]string{"secretKey": "unit-secret"}, nil); err == nil {
		t.Fatal("redirect accepted")
	}
	if received {
		t.Fatal("credentialed redirect reached foreign origin")
	}
}
func TestStateFailsClosedWithoutRootAndDoesNotReplaceCorruption(t *testing.T) {
	v := &memoryVault{values: map[string]string{stateKey: "{}"}}
	if _, err := loadState(v); ErrorCode(err) != "recovery_required" {
		t.Fatal("missing root accepted")
	}
	if got, _ := v.ReadSecret(stateKey); got != "{}" {
		t.Fatal("corrupt state replaced")
	}
	s := newState()
	registerState(s)
	if strings.Contains(security.Scrub(s.RootSecret), s.RootSecret) {
		t.Fatal("root not registered")
	}
}
func TestShutdownCancelsLease(t *testing.T) {
	cfg := testConfig()
	cfg.S3.Provider = config.S3ProviderExternal
	cfg.S3.AccessKey = "unit-access"
	cfg.S3.SecretKey = "unit-secret"
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), nil, nil, defaultNames())
	lease, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	closeManager(t, m)
	select {
	case <-lease.Context.Done():
	case <-time.After(time.Second):
		t.Fatal("lease cancellation timed out")
	}
	if !errors.Is(lease.Context.Err(), context.Canceled) {
		t.Fatal("lease survived shutdown")
	}
}

type fixtureEngine struct {
	mu       sync.Mutex
	requests []string
	request  func(context.Context, string, string, any, any) (int, error)
}

func (e *fixtureEngine) DoJSON(ctx context.Context, method, path string, body, result any) (int, error) {
	e.mu.Lock()
	e.requests = append(e.requests, method+" "+path)
	e.mu.Unlock()
	if e.request != nil {
		return e.request(ctx, method, path, body, result)
	}
	return 0, fmt.Errorf("no fixture response")
}
func (e *fixtureEngine) HTTPClient() *http.Client { return http.DefaultClient }
func (e *fixtureEngine) Host() string             { return dockerutil.DefaultHost() }
func (e *fixtureEngine) CloseIdleConnections()    {}
func TestDisabledManagerMakesNoBackgroundDockerRequests(t *testing.T) {
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	engine := &fixtureEngine{}
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), engine, nil, defaultNames())
	defer closeManager(t, m)
	m.observe(context.Background())
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if len(engine.requests) != 0 {
		t.Fatal("unconfigured manager touched Docker")
	}
}
func TestLifecycleRequiresExplicitInstall(t *testing.T) {
	m := newManager(context.Background(), testConfig(), &memoryVault{}, quietLogger(), &fixtureEngine{}, nil, defaultNames())
	defer closeManager(t, m)
	for _, action := range []Action{ActionStart, ActionRecreate} {
		if _, err := m.Begin(action); ErrorCode(err) != "not_installed" {
			t.Errorf("%s did not require install: %v", action, err)
		}
	}
}
func TestInactiveRustFSLifecycleDoesNotInvalidateExternalLease(t *testing.T) {
	cfg := testConfig()
	cfg.S3.Provider = config.S3ProviderExternal
	cfg.S3.AccessKey = "unit-access"
	cfg.S3.SecretKey = "unit-secret"
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), &fixtureEngine{}, nil, defaultNames())
	defer closeManager(t, m)
	lease, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	m.mu.Lock()
	m.revokeLocked()
	m.mu.Unlock()
	if err = lease.Check(); err != nil {
		t.Fatal("managed lifecycle revoked external lease")
	}
	changed := copyConfig(cfg)
	changed.RustFS.Permissions.AllowRead = false
	changed.Docker.ReadOnly = true
	m.Configure(changed)
	if err = lease.Check(); err != nil {
		t.Fatal("inactive managed settings revoked external lease")
	}
	if !m.Availability(changed).Available {
		t.Fatal("external availability blocked by managed lifecycle")
	}
}
func TestStaleExternalPermissionSnapshotCannotAcquire(t *testing.T) {
	cfg := testConfig()
	cfg.S3.Provider = config.S3ProviderExternal
	cfg.S3.AccessKey = "unit-access"
	cfg.S3.SecretKey = "unit-secret"
	cfg.Agent.AllowFilesystemWrite = true
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), &fixtureEngine{}, nil, defaultNames())
	defer closeManager(t, m)
	next := copyConfig(cfg)
	next.S3.ReadOnly = true
	next.Agent.AllowFilesystemWrite = false
	m.Configure(next)
	if _, err := m.AcquireS3(context.Background(), cfg, true); ErrorCode(err) != "stale_target" {
		t.Fatal("stale permissive snapshot admitted")
	}
	lease, err := m.AcquireS3(context.Background(), next, true)
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
}
func TestPreemptedLifecycleCannotPublishReady(t *testing.T) {
	cfg := testConfig()
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), &fixtureEngine{}, nil, defaultNames())
	defer closeManager(t, m)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.mu.Lock()
	old := &job{ctx: ctx, cancel: cancel, generation: m.generation, done: make(chan struct{})}
	m.active = old
	m.mu.Unlock()
	entered := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	go func() { close(entered); <-release; result <- m.publishReady(old, cfg) }()
	<-entered
	if err := m.acquireControl(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Begin(ActionStop); err != nil {
		m.releaseControl()
		t.Fatal(err)
	}
	close(release)
	if err := <-result; ErrorCode(err) != "stale_target" {
		t.Fatal("preempted ready publication accepted")
	}
	if m.Status().Ready {
		t.Fatal("preempted job cleared admission fence")
	}
	m.releaseControl()
}
func TestManagementAuthorityCanBeReenabledWithoutOldMutation(t *testing.T) {
	cfg := testConfig()
	cfg.Docker.ReadOnly = true
	engine := &fixtureEngine{}
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), engine, nil, defaultNames())
	defer closeManager(t, m)
	m.mu.Lock()
	m.state = newState()
	m.state.Provisioned = true
	m.state.InitialBucket = cfg.RustFS.Bucket
	m.mu.Unlock()
	next := copyConfig(cfg)
	next.Docker.ReadOnly = false
	if err := m.PrepareReconfigure(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if strings.Contains(strings.Join(engine.requests, ";"), "POST ") || strings.Contains(strings.Join(engine.requests, ";"), "DELETE ") {
		t.Fatal("reenable used old revoked Docker authority")
	}
}
func TestInitialBucketLockedBeforeRevocation(t *testing.T) {
	cfg := testConfig()
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), &fixtureEngine{}, nil, defaultNames())
	defer closeManager(t, m)
	m.mu.Lock()
	m.state.Provisioned = true
	m.state.InitialBucket = "aurago"
	generation := m.generation
	m.mu.Unlock()
	next := copyConfig(cfg)
	next.RustFS.Bucket = "another-bucket"
	if err := m.PrepareReconfigure(context.Background(), next); ErrorCode(err) != "bucket_locked" {
		t.Fatal("initialized bucket changed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.generation != generation {
		t.Fatal("invalid bucket edit revoked valid authority")
	}
}
func TestAdminDoesNotReplaySameOriginMutationRedirect(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	a, err := newAdminClient(server.URL, "unit-access", "unit-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	code, err := a.native(context.Background(), http.MethodPut, "add-user", nil, map[string]string{"secretKey": "unit-secret"}, nil)
	if err == nil || code != http.StatusTemporaryRedirect || requests != 1 {
		t.Fatal("admin mutation redirect replayed")
	}
}
func TestTypedNilVaultDoesNotPanic(t *testing.T) {
	m := NewManager(context.Background(), &config.Config{}, (*security.Vault)(nil), quietLogger())
	defer closeManager(t, m)
	if m.Status().Reason != "vault_unavailable" {
		t.Fatal("typed nil Vault not rejected")
	}
}

func TestConfirmedOriginalConfigRecoversPreparedUnpublishedTransition(t *testing.T) {
	cfg := testConfig()
	cfg.S3.Provider = config.S3ProviderExternal
	cfg.S3.AccessKey = "unit-access"
	cfg.S3.SecretKey = "unit-secret"
	engine := &fixtureEngine{}
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), engine, nil, defaultNames())
	defer closeManager(t, m)
	lease, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	next := copyConfig(cfg)
	next.S3.ReadOnly = true
	if err = m.PrepareReconfigure(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	m.observe(context.Background())
	if _, err = m.AcquireS3(context.Background(), cfg, true); ErrorCode(err) != "permission_pending" {
		t.Fatal("unpublished preparation restored authority")
	}
	m.Configure(cfg)
	restored, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal("confirmed original snapshot did not restore external admission")
	}
	restored.Release()
	if lease.Check() == nil {
		t.Fatal("old cancelled lease revived")
	}
}
func TestUnknownOutcomeDoesNotAutomaticallyRetry(t *testing.T) {
	cfg := testConfig()
	engine := &fixtureEngine{}
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), engine, nil, defaultNames())
	defer closeManager(t, m)
	m.mu.Lock()
	m.state = newState()
	m.state.Provisioned = true
	m.state.DesiredRunning = true
	m.uncertain = true
	m.mu.Unlock()
	m.observe(context.Background())
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if len(engine.requests) != 0 {
		t.Fatal("unknown mutation was automatically retried")
	}
}

func fixtureResult(result any, value any) {
	data, _ := json.Marshal(value)
	_ = json.Unmarshal(data, result)
}
func TestMissingInitializedVolumeRequiresRecoveryWithoutCreate(t *testing.T) {
	engine := &fixtureEngine{request: func(ctx context.Context, method, path string, body, result any) (int, error) {
		if method != http.MethodGet {
			t.Fatal("missing initialized volume was recreated")
		}
		return http.StatusNotFound, fmt.Errorf("not found")
	}}
	m := newManager(context.Background(), testConfig(), &memoryVault{}, quietLogger(), engine, nil, resourceNames{container: "volume-test"})
	defer closeManager(t, m)
	s := newState()
	s.Provisioned = true
	if _, err := m.ensureVolume(context.Background(), "retained-volume", "data", "original-created", s); ErrorCode(err) != "recovery_required" {
		t.Fatal("missing initialized volume accepted")
	}
}
func TestOwnedPreviousImageCanStopWithoutGrantingReadiness(t *testing.T) {
	cfg := testConfig()
	s := newState()
	s.EngineID = "fixture-engine"
	s.ContainerID = "recorded-container"
	engine := &fixtureEngine{}
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), engine, nil, resourceNames{container: "old-image-test"})
	defer closeManager(t, m)
	var observed containerInfo
	observed.ID = s.ContainerID
	observed.Config.Image = "rustfs/rustfs:previous-qualified-pin"
	observed.Config.Labels = m.labels("runtime", s)
	engine.request = func(ctx context.Context, method, path string, body, result any) (int, error) {
		if method != http.MethodGet {
			t.Fatal("stopped container required mutation")
		}
		if path == "info" {
			fixtureResult(result, map[string]any{"ID": s.EngineID, "OSType": "linux"})
		} else {
			fixtureResult(result, observed)
		}
		return http.StatusOK, nil
	}
	if err := m.stopRuntime(context.Background(), s, false); err != nil {
		t.Fatal(err)
	}
	if err := m.validateRuntimeSpec(observed, true); ErrorCode(err) != "runtime_spec_changed" {
		t.Fatal("old image granted current data readiness")
	}
}
func TestUnknownMutationIsNeverReplayed(t *testing.T) {
	requests := 0
	engine := &fixtureEngine{request: func(ctx context.Context, method, path string, body, result any) (int, error) {
		requests++
		return 0, fmt.Errorf("lost response")
	}}
	m := newManager(context.Background(), testConfig(), &memoryVault{}, quietLogger(), engine, nil, resourceNames{container: "mutation-test"})
	defer closeManager(t, m)
	if err := m.mutate(context.Background(), http.MethodPost, "containers/create", nil, nil); ErrorCode(err) != "outcome_unknown" {
		t.Fatal("lost mutation not classified unknown")
	}
	if requests != 1 {
		t.Fatal("mutation replayed")
	}
}

func TestPreparedBarrierBlocksNewAndLateLifecyclePublication(t *testing.T) {
	cfg := testConfig()
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), &fixtureEngine{}, nil, resourceNames{container: "prepared-test"})
	defer closeManager(t, m)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.mu.Lock()
	m.prepared = true
	m.state.Provisioned = true
	m.state.Instance = "owned"
	j := &job{ctx: ctx, cancel: cancel, generation: m.generation}
	m.active = j
	m.mu.Unlock()
	if _, err := m.Begin(ActionStart); ErrorCode(err) != "configuration_pending" {
		t.Fatal("prepared transition admitted old-config start")
	}
	if err := m.checkJob(j, cfg); ErrorCode(err) != "stale_target" {
		t.Fatal("prepared transition admitted old IAM job")
	}
	if err := m.publishReady(j, cfg); ErrorCode(err) != "stale_target" {
		t.Fatal("prepared transition admitted late ready publication")
	}
	m.mu.Lock()
	m.active = nil
	m.mu.Unlock()
}

func TestExternalSelectedStillTracksRuntimePlacement(t *testing.T) {
	cfg := testConfig()
	cfg.S3.Provider = config.S3ProviderExternal
	cfg.S3.AccessKey = "unit-access"
	cfg.S3.SecretKey = "unit-secret"
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), &fixtureEngine{}, nil, resourceNames{container: "host-test"})
	defer closeManager(t, m)
	lease, err := m.AcquireS3(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	next := copyConfig(cfg)
	next.Docker.Host = "tcp://127.0.0.1:2375"
	if configFingerprint(next) == configFingerprint(cfg) {
		t.Fatal("inactive managed Engine change ignored")
	}
	m.Configure(next)
	m.mu.Lock()
	updated := m.cfg.Docker.Host == next.Docker.Host
	m.mu.Unlock()
	if !updated {
		t.Fatal("runtime kept stale Engine configuration")
	}
	if lease.Check() != nil {
		t.Fatal("runtime placement change revoked external data lease")
	}
}
func TestManagedMakeBucketDoesNotRetryUnavailableMutation(t *testing.T) {
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts++
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "<Error><Code>ServiceUnavailable</Code><Message>fixture</Message></Error>")
	}))
	defer server.Close()
	client, err := newDataClient(server.URL, "unit-access", "unit-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = client.MakeBucket(ctx, "fixture-bucket", minio.MakeBucketOptions{Region: Region}); err == nil || puts != 1 {
		t.Fatalf("MakeBucket mutation attempts: %d", puts)
	}
}

func TestNativeTCPPlacementRequiresIndependentProof(t *testing.T) {
	cfg := testConfig()
	cfg.Docker.Host = "tcp://127.0.0.1:2375"
	engine := &fixtureEngine{}
	m := newManager(context.Background(), cfg, &memoryVault{}, quietLogger(), engine, nil, resourceNames{container: "tcp-placement"})
	defer closeManager(t, m) // A fake local-transport host mismatch also fails before trusting any port binding.
	engine.request = func(ctx context.Context, method, path string, body, result any) (int, error) {
		fixtureResult(result, map[string]string{"ID": "fixture-engine", "OSType": "linux"})
		return http.StatusOK, nil
	}
	if _, err := m.provePlacement(context.Background(), cfg, persistedState{}); ErrorCode(err) != "unsupported_placement" {
		t.Fatal("native TCP Engine treated as proven local")
	}
}
