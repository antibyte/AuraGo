package dockerhost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"aurago/internal/dockerhost/transport"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

// memVault is an in-memory SecretVault. Writes to keys with a prefix in
// failWritePrefix fail.
type memVault struct {
	mu              sync.Mutex
	secrets         map[string]string
	failWritePrefix string
}

func newMemVault() *memVault { return &memVault{secrets: map[string]string{}} }

func (v *memVault) ReadSecret(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	value, ok := v.secrets[key]
	if !ok {
		return "", security.ErrSecretNotFound
	}
	return value, nil
}

func (v *memVault) WriteSecret(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.failWritePrefix != "" && strings.HasPrefix(key, v.failWritePrefix) {
		return io.ErrClosedPipe
	}
	v.secrets[key] = value
	return nil
}

func (v *memVault) DeleteSecret(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.secrets[key]; !ok {
		return security.ErrSecretNotFound
	}
	delete(v.secrets, key)
	return nil
}

func (v *memVault) has(key string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := v.secrets[key]
	return ok
}

func newTestRegistry(t *testing.T, localEndpoint string) (*Registry, *memVault) {
	t.Helper()
	store, _ := openTestStore(t)
	vault := newMemVault()
	reg := NewRegistry(store, vault, func() string { return localEndpoint })
	t.Cleanup(reg.Close)
	return reg, vault
}

func tcpHostFor(fe *fakeEngine, name string) Host {
	return Host{Name: name, Kind: KindTCP, Address: fe.host(), Port: fe.port(), Color: "teal"}
}

func TestRegistryLocalHostIsSynthesizedAndFixed(t *testing.T) {
	ctx := context.Background()
	reg, _ := newTestRegistry(t, "tcp://127.0.0.1:1")

	hosts, err := reg.Hosts(ctx)
	if err != nil || len(hosts) != 1 {
		t.Fatalf("Hosts = (%v, %v), want only the local host", hosts, err)
	}
	local := hosts[0]
	if local.ID != LocalHostID || local.Name != "Local" || local.Kind != KindLocal || local.Address != "tcp://127.0.0.1:1" ||
		local.StacksDir != DefaultStacksDir || local.HostSocketPath != DefaultHostSocketPath || local.Color != "blue" || local.Sort != -1 {
		t.Fatalf("local host = %+v", local)
	}
	if _, err := reg.Update(ctx, LocalHostID, local, SecretInput{}); !errors.Is(err, ErrLocalHostFixed) {
		t.Fatalf("Update(local) = %v, want ErrLocalHostFixed", err)
	}
	if err := reg.Delete(ctx, LocalHostID); !errors.Is(err, ErrLocalHostFixed) {
		t.Fatalf("Delete(local) = %v, want ErrLocalHostFixed", err)
	}
	if reg.SecretSet(LocalHostID) {
		t.Fatal("the local host has no secrets")
	}

	// An empty docker.host falls back to the OS default endpoint.
	fallback := NewRegistry(reg.store, newMemVault(), nil)
	if got, _ := fallback.Host(ctx, LocalHostID); got.Address != dockerutil.DefaultHost() {
		t.Fatalf("local address without docker.host = %q, want %q", got.Address, dockerutil.DefaultHost())
	}
}

func TestRegistryTCPHostLifecycle(t *testing.T) {
	ctx := context.Background()
	fe := newFakeEngine(t)
	fe.withFixtures()
	reg, vault := newTestRegistry(t, "")

	created, err := reg.Create(ctx, tcpHostFor(fe, "NAS"), SecretInput{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.StacksDir != DefaultStacksDir || created.HostSocketPath != DefaultHostSocketPath {
		t.Fatalf("defaults not applied: %+v", created)
	}
	if status := reg.Status(created.ID); status.State != StateUnknown {
		t.Fatalf("Status before a test = %+v, want unknown", status)
	}
	if reg.SecretSet(created.ID) || len(vault.secrets) != 0 {
		t.Fatal("a plain TCP host stores no secrets")
	}

	status := reg.Test(ctx, created.ID)
	if status.State != StateOK || status.EngineVersion != "27.3.1" || status.APIVersion != "1.45" ||
		status.OS != "Debian GNU/Linux 12 (bookworm)" || status.Arch != "x86_64" || status.CheckedAt.IsZero() {
		t.Fatalf("Test = %+v, want ok with versions, OS and architecture from /info", status)
	}
	if cached := reg.Status(created.ID); cached != status {
		t.Fatalf("Status = %+v, want the cached %+v", cached, status)
	}

	first, err := reg.Client(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second, _ := reg.Client(ctx, created.ID); second != first {
		t.Fatal("the client of an unchanged host must be cached")
	}
	if first.Host() != "tcp://127.0.0.1:"+strconv.Itoa(fe.port()) {
		t.Fatalf("client name = %q", first.Host())
	}
	containers, err := ListContainers(ctx, first)
	if err != nil || len(containers) != 3 {
		t.Fatalf("ListContainers through the registry client = (%d, %v)", len(containers), err)
	}

	// Editing the connection settings drops the client and the status.
	edit := created
	edit.Name = "NAS 2"
	edit.Port = fe.port() // same engine, but the stored record changes
	edit.Color = "coral"
	updated, err := reg.Update(ctx, created.ID, edit, SecretInput{})
	if err != nil || updated.Name != "NAS 2" || updated.Color != "coral" {
		t.Fatalf("Update = (%+v, %v)", updated, err)
	}
	if status := reg.Status(created.ID); status.State != StateUnknown {
		t.Fatalf("Status after Update = %+v, want unknown", status)
	}
	if rebuilt, _ := reg.Client(ctx, created.ID); rebuilt == first {
		t.Fatal("Update must drop the cached client")
	}

	if err := reg.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := reg.Host(ctx, created.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("Host after Delete = %v, want ErrHostNotFound", err)
	}
	if err := reg.Delete(ctx, created.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("second Delete = %v, want ErrHostNotFound", err)
	}
	if st := reg.Test(ctx, created.ID); st.State != StateError {
		t.Fatalf("Test of an unknown id = %+v, want an error", st)
	}
	if got := reg.Status(created.ID); got.State != StateUnknown {
		t.Fatalf("an unknown id must not be cached, got %+v", got)
	}
}

func TestRegistryTLSHostVerifiesTheEngineAgainstTheStoredCA(t *testing.T) {
	ctx := context.Background()
	fe := newFakeTLSEngine(t)
	reg, vault := newTestRegistry(t, "")

	host := Host{Name: "secure", Kind: KindTLS, Address: fe.host(), Port: fe.port(), Color: "purple"}
	trusted, err := reg.Create(ctx, host, SecretInput{TLS: &transport.TLSMaterial{CA: fe.caPEM()}})
	if err != nil {
		t.Fatalf("Create with CA: %v", err)
	}
	if !reg.SecretSet(trusted.ID) || !vault.has(TLSSecretKey(trusted.ID)) {
		t.Fatal("the CA must be stored in the vault under docker_host_tls_<id>")
	}
	var stored transport.TLSMaterial
	if err := json.Unmarshal([]byte(vault.secrets[TLSSecretKey(trusted.ID)]), &stored); err != nil || stored.CA == "" {
		t.Fatalf("stored TLS material = %q, %v", vault.secrets[TLSSecretKey(trusted.ID)], err)
	}
	if status := reg.Test(ctx, trusted.ID); status.State != StateOK {
		t.Fatalf("Test with the engine CA = %+v, want ok", status)
	}

	// Without the CA the self-signed engine certificate is refused, never ignored.
	host.Name = "untrusted"
	untrusted, err := reg.Create(ctx, host, SecretInput{})
	if err != nil {
		t.Fatalf("Create without CA: %v", err)
	}
	if status := reg.Test(ctx, untrusted.ID); status.State != StateError || status.Code != CodeTLS {
		t.Fatalf("Test without the CA = %+v, want error code tls", status)
	}

	// Switching to plain TCP removes the TLS entry from the vault.
	switched := trusted
	switched.Kind = KindTCP
	if _, err := reg.Update(ctx, trusted.ID, switched, SecretInput{}); err != nil {
		t.Fatalf("Update to tcp: %v", err)
	}
	if vault.has(TLSSecretKey(trusted.ID)) || reg.SecretSet(trusted.ID) {
		t.Fatal("a host that no longer uses TLS must not keep TLS material")
	}
}

func TestRegistrySecretRules(t *testing.T) {
	ctx := context.Background()
	reg, vault := newTestRegistry(t, "")
	secret := "p4ssw0rd"
	wantField := func(err error, field string) {
		t.Helper()
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != field {
			t.Fatalf("error = %v, want a ValidationError for %q", err, field)
		}
	}

	ssh := Host{Name: "pi", Kind: KindSSH, Address: "10.0.0.7", SSHUser: "pi", Color: "green"}
	_, err := reg.Create(ctx, ssh, SecretInput{})
	wantField(err, "ssh_secret")
	blank := "  "
	_, err = reg.Create(ctx, ssh, SecretInput{SSHSecret: &blank})
	wantField(err, "ssh_secret")

	created, err := reg.Create(ctx, ssh, SecretInput{SSHSecret: &secret})
	if err != nil {
		t.Fatalf("Create ssh: %v", err)
	}
	if vault.secrets[SSHSecretKey(created.ID)] != secret || !reg.SecretSet(created.ID) {
		t.Fatal("the SSH secret must be in the vault under docker_host_ssh_<id>")
	}
	client, err := reg.Client(ctx, created.ID)
	if err != nil || client.Host() != "ssh://pi@10.0.0.7:22" {
		t.Fatalf("ssh client = (%v, %v); it must build without dialing and name no secret", client, err)
	}

	// Updating without secret input keeps the stored secret; Clear removes it.
	renamed := created
	renamed.Name = "pi-2"
	if _, err := reg.Update(ctx, created.ID, renamed, SecretInput{}); err != nil || vault.secrets[SSHSecretKey(created.ID)] != secret {
		t.Fatalf("Update without secret input = %v, stored %q; want the secret kept", err, vault.secrets[SSHSecretKey(created.ID)])
	}
	_, err = reg.Update(ctx, created.ID, renamed, SecretInput{Clear: true})
	wantField(err, "ssh_secret")
	if vault.secrets[SSHSecretKey(created.ID)] != secret {
		t.Fatal("a rejected Update must leave the vault as it was")
	}

	// Other validation: names, kinds, TLS material.
	_, err = reg.Create(ctx, Host{Name: "LOCAL", Kind: KindTCP, Address: "10.0.0.8", Color: "gray"}, SecretInput{})
	wantField(err, "name")
	_, err = reg.Create(ctx, Host{Name: "mtls-box", Kind: KindMTLS, Address: "10.0.0.9", Color: "gray"}, SecretInput{})
	wantField(err, "tls")
	_, err = reg.Create(ctx, Host{Name: "bad-ca", Kind: KindTLS, Address: "10.0.0.9", Color: "gray"}, SecretInput{TLS: &transport.TLSMaterial{CA: "not a certificate"}})
	wantField(err, "tls")
	_, err = reg.Create(ctx, Host{Name: "local-kind", Kind: KindLocal, Address: "x", Color: "gray"}, SecretInput{})
	wantField(err, "kind")

	// Delete removes the vault entries.
	if err := reg.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if vault.has(SSHSecretKey(created.ID)) {
		t.Fatal("Delete must remove the host's vault entries")
	}
}

func TestRegistryCreateRollsBackWhenTheVaultWriteFails(t *testing.T) {
	ctx := context.Background()
	reg, vault := newTestRegistry(t, "")
	vault.failWritePrefix = "docker_host_ssh_"
	secret := "p4ssw0rd"
	ssh := Host{Name: "pi", Kind: KindSSH, Address: "10.0.0.7", SSHUser: "pi", Color: "green"}
	if _, err := reg.Create(ctx, ssh, SecretInput{SSHSecret: &secret}); err == nil {
		t.Fatal("Create succeeded although the vault refused the secret")
	}
	if hosts, _ := reg.store.List(ctx); len(hosts) != 0 {
		t.Fatalf("a host whose secret could not be stored must not remain: %+v", hosts)
	}
}

func TestRegistryTestDraftDoesNotSaveAndClassifiesFailures(t *testing.T) {
	ctx := context.Background()
	fe := newFakeEngine(t)
	reg, _ := newTestRegistry(t, "")

	if status := reg.TestDraft(ctx, tcpHostFor(fe, "draft"), SecretInput{}); status.State != StateOK || status.EngineVersion != "27.3.1" {
		t.Fatalf("TestDraft of a reachable engine = %+v", status)
	}
	if hosts, _ := reg.store.List(ctx); len(hosts) != 0 {
		t.Fatalf("TestDraft must not save the host: %+v", hosts)
	}

	// A port with nothing listening: refused, not a hang.
	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	refusedPort := closed.Addr().(*net.TCPAddr).Port
	_ = closed.Close()
	dead := Host{Name: "dead", Kind: KindTCP, Address: "127.0.0.1", Port: refusedPort, Color: "gray"}
	status := reg.TestDraft(ctx, dead, SecretInput{})
	if status.State != StateError || status.Code != CodeRefused {
		t.Fatalf("TestDraft of a closed port = %+v, want error code refused", status)
	}

	// An invalid draft is reported as a status, not a panic.
	if status := reg.TestDraft(ctx, Host{Name: "x", Kind: KindTCP, Address: "tcp://x"}, SecretInput{}); status.State != StateError || !strings.Contains(status.Message, "address") {
		t.Fatalf("TestDraft of an invalid draft = %+v", status)
	}

	// A draft that names a stored host reuses its stored secrets.
	tlsEngine := newFakeTLSEngine(t)
	saved, err := reg.Create(ctx, Host{Name: "secure", Kind: KindTLS, Address: tlsEngine.host(), Port: tlsEngine.port(), Color: "purple"},
		SecretInput{TLS: &transport.TLSMaterial{CA: tlsEngine.caPEM()}})
	if err != nil {
		t.Fatal(err)
	}
	if status := reg.TestDraft(ctx, saved, SecretInput{}); status.State != StateOK {
		t.Fatalf("TestDraft of a stored host without secret input = %+v, want the stored CA used", status)
	}
}

func TestRegistryTestCachesUnreachableAndOldEngineStatus(t *testing.T) {
	ctx := context.Background()
	reg, _ := newTestRegistry(t, "")

	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	refusedPort := closed.Addr().(*net.TCPAddr).Port
	_ = closed.Close()
	dead, err := reg.Create(ctx, Host{Name: "dead", Kind: KindTCP, Address: "127.0.0.1", Port: refusedPort, Color: "gray"}, SecretInput{})
	if err != nil {
		t.Fatal(err)
	}
	status := reg.Test(ctx, dead.ID)
	if status.State != StateError || status.Code != CodeRefused || status.Message == "" {
		t.Fatalf("Test of a closed port = %+v, want error code refused with a message", status)
	}
	if reg.Status(dead.ID) != status {
		t.Fatal("a failed test must be cached too")
	}

	old := newFakeEngine(t)
	old.setVersion("1.40", "19.03.0")
	legacy, err := reg.Create(ctx, tcpHostFor(old, "legacy"), SecretInput{})
	if err != nil {
		t.Fatal(err)
	}
	if status := reg.Test(ctx, legacy.ID); status.State != StateError || status.Code != CodeAPITooOld {
		t.Fatalf("Test of an API 1.40 engine = %+v, want error code api_too_old", status)
	}
}
