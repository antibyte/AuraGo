package rustfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

type reconfigureVault struct {
	inner      *memoryVault
	mu         sync.Mutex
	failWrites int
}

func (v *reconfigureVault) ReadSecret(key string) (string, error) {
	return v.inner.ReadSecret(key)
}

func (v *reconfigureVault) WriteSecret(key, value string) error {
	v.mu.Lock()
	if v.failWrites > 0 {
		v.failWrites--
		v.mu.Unlock()
		return errors.New("injected vault write failure")
	}
	v.mu.Unlock()
	return v.inner.WriteSecret(key, value)
}

type reconfigureFixture struct {
	mu               sync.Mutex
	state            persistedState
	policy           string
	childPolicy      string
	childStatus      string
	confirmReadFails bool
	unknownDisable   bool
	updateCalls      int
	disableCalls     int
	enableCalls      int
	httpRequests     []string
	server           *httptest.Server
	vault            *reconfigureVault
	engine           *fixtureEngine
	manager          *Manager
	config           *config.Config
	names            resourceNames
}

func newReconfigureFixture(t *testing.T) *reconfigureFixture {
	t.Helper()
	cfg := testConfig()
	f := &reconfigureFixture{config: cfg, childStatus: "on", names: resourceNames{"rustfs-test", "rustfs-seed-test", "rustfs-data-test", "rustfs-credentials-test"}}
	f.state = newState()
	f.state.InitialBucket = cfg.RustFS.Bucket
	f.state.EngineID = "engine-test"
	f.state.ContainerID = "rustfs-container-test"
	f.state.DataCreated = "data-created"
	f.state.CredentialsCreated = "credentials-created"
	f.state.Seeded = true
	f.state.DesiredRunning = true
	f.state.Provisioned = true
	f.state.PolicyHash = policyHash(generatePolicy(effectivePermissions(cfg)))
	f.policy = string(generatePolicy(effectivePermissions(cfg)))
	f.childPolicy = f.policy
	f.vault = &reconfigureVault{inner: &memoryVault{}}
	if err := f.vault.inner.WriteSecret(stateKey, mustJSON(t, f.state)); err != nil {
		t.Fatal(err)
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	f.state.Endpoint = f.server.URL
	if err := f.vault.inner.WriteSecret(stateKey, mustJSON(t, f.state)); err != nil {
		t.Fatal(err)
	}
	f.engine = &fixtureEngine{request: f.engineRequest}
	f.manager = newManager(context.Background(), cfg, f.vault, quietLogger(), f.engine, f.server.Client(), f.names)
	t.Cleanup(func() { closeManager(t, f.manager) })
	return f
}

func (f *reconfigureFixture) engineRequest(_ context.Context, method, path string, _ any, result any) (int, error) {
	f.mu.Lock()
	s := f.state
	f.mu.Unlock()
	var value any
	switch {
	case method == http.MethodGet && path == "info":
		value = map[string]any{"ID": s.EngineID, "OSType": "linux"}
	case method == http.MethodGet && path == "containers/"+url.PathEscape(f.names.container)+"/json":
		port := strconv.Itoa(f.server.Listener.Addr().(*net.TCPAddr).Port)
		value = map[string]any{
			"Id":         s.ContainerID,
			"Config":     map[string]any{"Image": Image, "User": "10001:10001", "Labels": f.labels(s, "runtime")},
			"State":      map[string]any{"Running": true},
			"HostConfig": map[string]any{"NetworkMode": "bridge", "Privileged": false},
			"Mounts": []map[string]any{
				{"Type": "volume", "Name": f.names.data, "Source": f.names.data, "Destination": "/data", "RW": true},
				{"Type": "volume", "Name": f.names.credentials, "Source": f.names.credentials, "Destination": "/run/aurago-rustfs", "RW": false},
			},
			"NetworkSettings": map[string]any{"Ports": map[string]any{"9000/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": port}}}},
		}
	case method == http.MethodGet && path == "images/"+url.PathEscape(Image)+"/json":
		value = map[string]any{"Id": "sha256:rustfs-fixture"}
	case method == http.MethodGet && path == "volumes/"+url.PathEscape(f.names.data):
		value = map[string]any{"Name": f.names.data, "CreatedAt": s.DataCreated, "Mountpoint": "C:/rustfs-data-test", "Labels": f.labels(s, "data")}
	case method == http.MethodGet && path == "volumes/"+url.PathEscape(f.names.credentials):
		value = map[string]any{"Name": f.names.credentials, "CreatedAt": s.CredentialsCreated, "Mountpoint": "C:/rustfs-credentials-test", "Labels": f.labels(s, "credentials")}
	default:
		return http.StatusNotFound, fmt.Errorf("unexpected Docker fixture request %s %s", method, path)
	}
	if result != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return 0, err
		}
		if err = json.Unmarshal(data, result); err != nil {
			return 0, err
		}
	}
	return http.StatusOK, nil
}

func (f *reconfigureFixture) labels(s persistedState, role string) map[string]string {
	labels := dockerutil.ManagedLabels(dockerutil.RustFSOwner, "rustfs", role, "")
	labels["aurago.instance"] = s.Instance
	return labels
}

func (f *reconfigureFixture) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.httpRequests = append(f.httpRequests, r.Method+" "+r.URL.Path)
	f.mu.Unlock()
	if r.URL.Path == "/health/ready" {
		writeFixtureJSON(w, map[string]any{"ready": true})
		return
	}
	if r.Method == http.MethodHead && strings.TrimSuffix(r.URL.Path, "/") == "/"+f.config.RustFS.Bucket {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/rustfs/admin/v3/") {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/rustfs/admin/v3/")
	switch {
	case r.Method == http.MethodGet && name == "user-info":
		writeFixtureJSON(w, map[string]string{"policyName": f.state.PolicyName, "status": "enabled"})
	case r.Method == http.MethodGet && name == "info-canned-policy":
		f.mu.Lock()
		policy := f.policy
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"policy":%s}`, policy)
	case r.Method == http.MethodGet && name == "info-service-account":
		f.mu.Lock()
		if f.confirmReadFails {
			f.confirmReadFails = false
			f.mu.Unlock()
			http.Error(w, "temporary confirmation read failure", http.StatusServiceUnavailable)
			return
		}
		status, policy := f.childStatus, f.childPolicy
		f.mu.Unlock()
		writeFixtureJSON(w, map[string]any{"parentUser": f.state.ParentAccess, "accountStatus": status, "impliedPolicy": false, "policy": policy, "description": "AuraGo managed S3 " + f.state.Instance})
	case r.Method == http.MethodGet && name == "account/info":
		writeFixtureJSON(w, map[string]string{"identity_type": "service-account"})
	case r.Method == http.MethodGet && name == "list-users":
		http.Error(w, "denied", http.StatusForbidden)
	case r.Method == http.MethodPut && name == "add-canned-policy":
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.policy = string(data)
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPut && name == "set-user-or-group-policy":
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodPost && name == "update-service-account":
		var body struct {
			NewStatus string          `json:"newStatus"`
			NewPolicy json.RawMessage `json:"newPolicy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.updateCalls++
		if body.NewStatus == "off" {
			f.disableCalls++
			f.childStatus = "off"
			if f.unknownDisable {
				f.unknownDisable = false
				f.mu.Unlock()
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					return
				}
				connection, _, err := hijacker.Hijack()
				if err == nil {
					_ = connection.Close()
				}
				return
			}
			f.confirmReadFails = true
		} else {
			f.enableCalls++
			f.childStatus = body.NewStatus
			if len(body.NewPolicy) > 0 {
				f.childPolicy = string(body.NewPolicy)
			}
		}
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func writeFixtureJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func waitReconfigureOperation(t *testing.T, m *Manager, operation Operation) error {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, ok := m.Operation(operation.ID)
		if !ok {
			return errors.New("operation disappeared")
		}
		switch current.State {
		case "succeeded":
			return nil
		case "failed", "cancelled", "unknown":
			m.mu.Lock()
			err := m.jobs[operation.ID].err
			m.mu.Unlock()
			return err
		}
		select {
		case <-deadline.C:
			return errors.New("operation timed out")
		case <-ticker.C:
		}
	}
}

func waitConfiguredStart(t *testing.T, m *Manager) error {
	t.Helper()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		m.mu.Lock()
		var latest *job
		for _, candidate := range m.jobs {
			if candidate.Action == ActionStart && (latest == nil || candidate.Created.After(latest.Created)) {
				latest = candidate
			}
		}
		if latest != nil {
			state, err := latest.State, latest.err
			m.mu.Unlock()
			switch state {
			case "succeeded":
				return nil
			case "failed", "cancelled", "unknown":
				return err
			}
		} else {
			m.mu.Unlock()
		}
		select {
		case <-deadline.C:
			return errors.New("configured Start timed out")
		case <-ticker.C:
		}
	}
}

func TestReconfigureRevocationReadFailureSurvivesRestartAndExplicitStart(t *testing.T) {
	f := newReconfigureFixture(t)
	next := copyConfig(f.config)
	next.S3.ReadOnly = true
	if err := f.manager.PrepareReconfigure(context.Background(), next); ErrorCode(err) != "admin_rejected" {
		t.Fatalf("PrepareReconfigure error = %q (%v), want confirmation read rejection; Docker requests: %v; HTTP requests: %v", ErrorCode(err), err, f.engine.requests, f.httpRequests)
	}
	stored, err := f.vault.ReadSecret(stateKey)
	if err != nil {
		t.Fatal(err)
	}
	var pending persistedState
	if err = json.Unmarshal([]byte(stored), &pending); err != nil {
		t.Fatal(err)
	}
	if !pending.IAMPending || pending.PolicyHash != f.state.PolicyHash || pending.PendingPolicyHash != policyHash(generatePolicy(effectivePermissions(next))) {
		t.Fatal("revocation intent was not durably recorded with the prior policy proof")
	}
	f.manager.Configure(f.config)
	unrelated := copyConfig(f.config)
	unrelated.Agent.AllowFilesystemWrite = true
	if err = f.manager.PrepareReconfigure(context.Background(), unrelated); err != nil {
		t.Fatalf("unrelated config preparation: %v", err)
	}
	f.manager.Configure(unrelated)
	f.manager.observe(context.Background())
	f.mu.Lock()
	updates := f.updateCalls
	f.mu.Unlock()
	if updates != 1 {
		t.Fatalf("rollback/background reconciliation replayed IAM mutation: %d updates", updates)
	}
	if _, err = f.manager.AcquireS3(context.Background(), f.config, true); ErrorCode(err) != "permission_pending" {
		t.Fatalf("pending IAM authority admitted S3 lease: %v", err)
	}
	closeManager(t, f.manager)

	restarted := newManager(context.Background(), f.config, f.vault, quietLogger(), f.engine, f.server.Client(), f.names)
	t.Cleanup(func() { closeManager(t, restarted) })
	if !restarted.state.IAMPending || restarted.state.PolicyHash != f.state.PolicyHash {
		t.Fatal("manager restart lost the durable IAM recovery state")
	}
	if err = restarted.inspectEstablishedIAM(context.Background(), f.config, restarted.state, false); ErrorCode(err) != "recovery_required" {
		t.Fatal("background inspection accepted pending IAM state")
	}
	operation, err := restarted.Begin(ActionStart)
	if err != nil {
		t.Fatal(err)
	}
	if err = waitReconfigureOperation(t, restarted, operation); err != nil {
		t.Fatalf("explicit Start did not recover the owned IAM state: %s", ErrorCode(err))
	}
	if restarted.state.IAMPending || restarted.state.PendingPolicyHash != "" || restarted.state.PolicyHash != policyHash(generatePolicy(effectivePermissions(f.config))) {
		t.Fatal("successful explicit recovery did not clear the pending marker")
	}
	if !restarted.Status().Ready || restarted.Status().AccessState != "ready" {
		t.Fatal("recovered IAM did not restore readiness")
	}
}

func TestSamePermissionSaveRetryReadVerifiesPendingChild(t *testing.T) {
	f := newReconfigureFixture(t)
	if _, err := f.manager.Preflight(context.Background()); err != nil {
		t.Fatalf("initial Preflight: %v", err)
	}
	next := copyConfig(f.config)
	next.S3.ReadOnly = true
	if err := f.manager.PrepareReconfigure(context.Background(), next); ErrorCode(err) != "admin_rejected" {
		t.Fatalf("initial PrepareReconfigure error = %q", ErrorCode(err))
	}
	f.manager.Configure(f.config)
	if err := f.manager.PrepareReconfigure(context.Background(), next); err != nil {
		t.Fatalf("same permission save retry did not read-verify the pending child: %s", ErrorCode(err))
	}
	f.mu.Lock()
	disables, status := f.disableCalls, f.childStatus
	f.mu.Unlock()
	if disables != 1 || status != "off" {
		t.Fatalf("verified-off child was mutated again: disables=%d status=%s", disables, status)
	}
	stored, err := f.vault.ReadSecret(stateKey)
	if err != nil {
		t.Fatal(err)
	}
	var state persistedState
	if err = json.Unmarshal([]byte(stored), &state); err != nil {
		t.Fatal(err)
	}
	if !state.IAMPending || state.PendingPolicyHash != policyHash(generatePolicy(effectivePermissions(next))) {
		t.Fatal("same permission save replaced or cleared the pending recovery anchor")
	}
	f.manager.Configure(next)
	if err = waitConfiguredStart(t, f.manager); err != nil {
		t.Fatalf("confirmed permission publication did not reconcile automatically: %s", ErrorCode(err))
	}
	f.manager.mu.Lock()
	pending := f.manager.state.IAMPending
	f.manager.mu.Unlock()
	if pending || !f.manager.Status().Ready {
		t.Fatal("confirmed permission publication left IAM pending")
	}
}

func TestPendingIAMDoesNotAutoReconcileOnUnrelatedConfigAfterRestart(t *testing.T) {
	f := newReconfigureFixture(t)
	next := copyConfig(f.config)
	next.S3.ReadOnly = true
	if err := f.manager.PrepareReconfigure(context.Background(), next); ErrorCode(err) != "admin_rejected" {
		t.Fatalf("PrepareReconfigure error = %q", ErrorCode(err))
	}
	closeManager(t, f.manager)

	restarted := newManager(context.Background(), next, f.vault, quietLogger(), f.engine, f.server.Client(), f.names)
	t.Cleanup(func() { closeManager(t, restarted) })
	if _, err := restarted.Preflight(context.Background()); ErrorCode(err) != "recovery_required" {
		t.Fatalf("pending startup Preflight error = %q", ErrorCode(err))
	}
	unrelated := copyConfig(next)
	unrelated.Agent.AllowFilesystemWrite = true
	if err := restarted.PrepareReconfigure(context.Background(), unrelated); err != nil {
		t.Fatalf("unrelated config preparation: %v", err)
	}
	restarted.Configure(unrelated)
	restarted.observe(context.Background())
	time.Sleep(100 * time.Millisecond)
	restarted.mu.Lock()
	active := restarted.active
	jobCount := len(restarted.jobs)
	restarted.mu.Unlock()
	f.mu.Lock()
	updates := f.updateCalls
	f.mu.Unlock()
	if active != nil || jobCount != 0 || updates != 1 {
		t.Fatalf("unrelated config save initiated IAM recovery: active=%v jobs=%d updates=%d", active != nil, jobCount, updates)
	}
}

func TestReconfigureVaultFailureDoesNotMutateChild(t *testing.T) {
	f := newReconfigureFixture(t)
	f.vault.mu.Lock()
	f.vault.failWrites = 1
	f.vault.mu.Unlock()
	next := copyConfig(f.config)
	next.S3.ReadOnly = true
	err := f.manager.PrepareReconfigure(context.Background(), next)
	if ErrorCode(err) != "state_save_failed" {
		t.Fatalf("PrepareReconfigure error = %q (%v), want state_save_failed; Docker requests: %v", ErrorCode(err), err, f.engine.requests)
	}
	f.mu.Lock()
	updates, status := f.updateCalls, f.childStatus
	f.mu.Unlock()
	if updates != 0 || status != "on" {
		t.Fatalf("IAM changed without a durable intent: updates=%d status=%s", updates, status)
	}
	stored, err := f.vault.ReadSecret(stateKey)
	if err != nil {
		t.Fatal(err)
	}
	var state persistedState
	if err = json.Unmarshal([]byte(stored), &state); err != nil {
		t.Fatal(err)
	}
	if state.IAMPending {
		t.Fatal("failed Vault write changed durable state")
	}
}

func TestIAMReconcileVaultFailurePrecedesPrivilegeMutation(t *testing.T) {
	f := newReconfigureFixture(t)
	f.vault.mu.Lock()
	f.vault.failWrites = 1
	f.vault.mu.Unlock()
	s := f.manager.state
	err := f.manager.reconcileIAM(context.Background(), f.config, &s)
	if ErrorCode(err) != "state_save_failed" {
		t.Fatalf("reconcileIAM error = %q, want state_save_failed", ErrorCode(err))
	}
	f.mu.Lock()
	updates, status := f.updateCalls, f.childStatus
	f.mu.Unlock()
	if updates != 0 || status != "on" {
		t.Fatalf("IAM changed without a durable reconciliation intent: updates=%d status=%s", updates, status)
	}
}

func TestUnknownRevocationStaysLatchedUntilReadVerifiedStart(t *testing.T) {
	f := newReconfigureFixture(t)
	f.mu.Lock()
	f.unknownDisable = true
	f.mu.Unlock()
	next := copyConfig(f.config)
	next.S3.ReadOnly = true
	if err := f.manager.PrepareReconfigure(context.Background(), next); ErrorCode(err) != "outcome_unknown" {
		t.Fatalf("PrepareReconfigure error = %q (%v), want outcome_unknown; Docker requests: %v", ErrorCode(err), err, f.engine.requests)
	}
	f.manager.Configure(f.config)
	f.manager.observe(context.Background())
	f.mu.Lock()
	updates := f.updateCalls
	f.mu.Unlock()
	if updates != 1 {
		t.Fatalf("unknown mutation was automatically replayed: %d updates", updates)
	}
	closeManager(t, f.manager)

	restarted := newManager(context.Background(), f.config, f.vault, quietLogger(), f.engine, f.server.Client(), f.names)
	t.Cleanup(func() { closeManager(t, restarted) })
	restarted.observe(context.Background())
	f.mu.Lock()
	updates = f.updateCalls
	f.mu.Unlock()
	if updates != 1 {
		t.Fatalf("restart automatically replayed unknown mutation: %d updates", updates)
	}
	operation, err := restarted.Begin(ActionStart)
	if err != nil {
		t.Fatal(err)
	}
	if err = waitReconfigureOperation(t, restarted, operation); err != nil {
		t.Fatalf("read-verified explicit recovery failed: %s", ErrorCode(err))
	}
	f.mu.Lock()
	disables, enables := f.disableCalls, f.enableCalls
	f.mu.Unlock()
	if disables != 1 || enables != 1 {
		t.Fatalf("verified off state was replayed instead of reconciled: disables=%d enables=%d", disables, enables)
	}
}

func TestPendingIAMRejectsForeignPolicyDuringRecovery(t *testing.T) {
	f := newReconfigureFixture(t)
	next := copyConfig(f.config)
	next.S3.ReadOnly = true
	if err := f.manager.PrepareReconfigure(context.Background(), next); ErrorCode(err) != "admin_rejected" {
		t.Fatalf("PrepareReconfigure error = %q (%v); Docker requests: %v", ErrorCode(err), err, f.engine.requests)
	}
	closeManager(t, f.manager)
	f.mu.Lock()
	f.childPolicy = `{"Version":"2012-10-17","Statement":[]}`
	f.mu.Unlock()
	restarted := newManager(context.Background(), f.config, f.vault, quietLogger(), f.engine, f.server.Client(), f.names)
	t.Cleanup(func() { closeManager(t, restarted) })
	operation, err := restarted.Begin(ActionStart)
	if err != nil {
		t.Fatal(err)
	}
	if err = waitReconfigureOperation(t, restarted, operation); ErrorCode(err) != "recovery_required" {
		t.Fatalf("foreign policy recovery error = %q", ErrorCode(err))
	}
	f.mu.Lock()
	updates := f.updateCalls
	f.mu.Unlock()
	if updates != 1 {
		t.Fatalf("foreign IAM was mutated during recovery: %d updates", updates)
	}
	f.mu.Lock()
	f.childPolicy = f.policy
	f.childStatus = "starting"
	f.mu.Unlock()
	if err = restarted.inspectEstablishedIAM(context.Background(), f.config, restarted.state, true); ErrorCode(err) != "recovery_required" {
		t.Fatalf("unknown service-account status was accepted during recovery: %q", ErrorCode(err))
	}
}
