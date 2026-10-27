package dockerhost

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/dockerhost/transport"
)

// sshDialLog replaces Registry.dialSSH: it records every login attempt (address
// and secret) and answers with err.
type sshDialLog struct {
	mu    sync.Mutex
	err   error
	calls []string
}

func (d *sshDialLog) dial(_ context.Context, target transport.SSHTarget, _ time.Duration) (net.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, target.Host+"|"+string(target.Secret))
	return nil, d.err
}

func (d *sshDialLog) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}

func (d *sshDialLog) last() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.calls) == 0 {
		return ""
	}
	return d.calls[len(d.calls)-1]
}

func newSSHTestRegistry(t *testing.T, dialErr error) (*Registry, *memVault, *sshDialLog, *fakeClock) {
	t.Helper()
	reg, vault := newTestRegistry(t, "")
	log := &sshDialLog{err: dialErr}
	clock := newFakeClock()
	reg.dialSSH = log.dial
	reg.now = clock.Now
	return reg, vault, log, clock
}

func validationOf(err error) (field, message string) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Field, ve.Message
	}
	return "", ""
}

func piHost() Host {
	return Host{Name: "pi", Kind: KindSSH, Address: "pi.lan", SSHUser: "pi", Color: "green"}
}

func TestRegistrySSHAuthFailurePausesLoginsUntilTestNowOrUpdate(t *testing.T) {
	ctx := context.Background()
	reg, _, log, _ := newSSHTestRegistry(t, errBadPassword)
	wrong := "wrong"
	host, err := reg.Create(ctx, piHost(), SecretInput{SSHSecret: &wrong})
	if err != nil {
		t.Fatal(err)
	}

	// The first test logs in once and fails with auth.
	if status := reg.Test(ctx, host.ID); status.State != StateError || status.Code != CodeAuth || log.count() != 1 {
		t.Fatalf("first Test = %+v after %d logins, want error auth after 1", status, log.count())
	}
	// Further tests and Engine requests (the 5 s container poll) do not log in.
	status := reg.Test(ctx, host.ID)
	if status.Code != CodeAuth || !strings.Contains(status.Message, "SSH login paused after an authentication failure; next attempt in 2m0s") {
		t.Fatalf("Test while paused = %+v, want the pause message with code auth", status)
	}
	client, err := reg.Client(ctx, host.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, listErr := ListContainers(ctx, client)
	if listErr == nil || StatusOf(listErr) != 0 || ClassifyError(listErr) != CodeAuth { // the API answers host_unreachable, reason auth
		t.Fatalf("ListContainers while paused = %v (status %d, class %q), want a dial failure classified as auth", listErr, StatusOf(listErr), ClassifyError(listErr))
	}
	if log.count() != 1 {
		t.Fatalf("%d logins were attempted while the host was paused, want only the first", log.count())
	}

	// TestDraft is a manual action: it dials even while paused and leaves the pause as it is.
	if draft := reg.TestDraft(ctx, host, SecretInput{}); draft.Code != CodeAuth || log.count() != 2 {
		t.Fatalf("TestDraft = %+v after %d logins, want a real attempt (2 logins)", draft, log.count())
	}
	if reg.Test(ctx, host.ID); log.count() != 2 {
		t.Fatal("TestDraft must not lift the pause")
	}

	// TestNow lifts the pause for exactly one attempt, which fails and pauses again.
	if status := reg.TestNow(ctx, host.ID); status.Code != CodeAuth || log.count() != 3 {
		t.Fatalf("TestNow = %+v after %d logins, want one more login", status, log.count())
	}
	reg.Test(ctx, host.ID)
	if log.count() != 3 {
		t.Fatalf("Test after TestNow made a login (%d), want the pause back", log.count())
	}

	// Editing the host (new credentials) forgets the pause.
	fixed := "fixed"
	if _, err := reg.Update(ctx, host.ID, host, SecretInput{SSHSecret: &fixed}); err != nil {
		t.Fatal(err)
	}
	reg.Test(ctx, host.ID)
	if log.count() != 4 || log.last() != "pi.lan|fixed" {
		t.Fatalf("Test after Update = %d logins, last %q; want a fresh login with the new secret", log.count(), log.last())
	}
}

func TestRegistryStoredSSHSecretDoesNotFollowAChangedHostIdentity(t *testing.T) {
	ctx := context.Background()
	reg, vault, log, _ := newSSHTestRegistry(t, errors.New("dial tcp: connection refused"))
	secret := "s3cret"
	created, err := reg.Create(ctx, piHost(), SecretInput{SSHSecret: &secret})
	if err != nil {
		t.Fatal(err)
	}

	moves := map[string]func(*Host){
		"address": func(h *Host) { h.Address = "other.lan" },
		"port":    func(h *Host) { h.Port = 2222 },
		"user":    func(h *Host) { h.SSHUser = "root" },
	}
	for name, move := range moves {
		edit := created
		move(&edit)
		_, err := reg.Update(ctx, created.ID, edit, SecretInput{})
		if field, message := validationOf(err); field != "ssh_secret" || message != errSSHSecretIdentity {
			t.Fatalf("Update with a changed %s = %v, want the ssh_secret re-entry error", name, err)
		}
		if vault.secrets[SSHSecretKey(created.ID)] != secret {
			t.Fatalf("a rejected Update (%s) must leave the vault as it was", name)
		}
		before := log.count()
		draft := reg.TestDraft(ctx, edit, SecretInput{})
		if draft.State != StateError || !strings.Contains(draft.Message, errSSHSecretIdentity) || log.count() != before {
			t.Fatalf("TestDraft with a changed %s = %+v (%d new logins); want the same error and no login", name, draft, log.count()-before)
		}
	}
	// A host that was not SSH has no stored SSH secret to carry over.
	viaTCP := created
	viaTCP.Kind = KindTCP
	if _, err := reg.Update(ctx, created.ID, viaTCP, SecretInput{}); err != nil {
		t.Fatal(err)
	}
	back := viaTCP
	back.Kind, back.SSHUser = KindSSH, "pi"
	if field, _ := validationOf(func() error { _, err := reg.Update(ctx, created.ID, back, SecretInput{}); return err }()); field != "ssh_secret" {
		t.Fatal("tcp -> ssh without a secret must ask for one")
	}
	if _, err := reg.Update(ctx, created.ID, back, SecretInput{SSHSecret: &secret}); err != nil {
		t.Fatal(err)
	}

	// The same machine and login (spelling, default port) keeps the stored secret.
	same := created
	same.Address, same.Port, same.Name = "PI.lan", 22, "pi renamed"
	if _, err := reg.Update(ctx, created.ID, same, SecretInput{}); err != nil || vault.secrets[SSHSecretKey(created.ID)] != secret {
		t.Fatalf("Update of the same identity = %v, stored %q; want the secret kept", err, vault.secrets[SSHSecretKey(created.ID)])
	}
	before := log.count()
	reg.TestDraft(ctx, same, SecretInput{})
	if log.count() != before+1 || log.last() != "PI.lan|"+secret {
		t.Fatalf("TestDraft of the same identity dialed %q, want the stored secret used", log.last())
	}

	// A newly typed secret moves the host.
	moved := same
	moved.Address = "other.lan"
	newSecret := "n3w"
	if _, err := reg.Update(ctx, created.ID, moved, SecretInput{SSHSecret: &newSecret}); err != nil || vault.secrets[SSHSecretKey(created.ID)] != newSecret {
		t.Fatalf("Update with a new secret = %v, stored %q", err, vault.secrets[SSHSecretKey(created.ID)])
	}
}

func TestRegistryClientForDiscardsAClientBuiltAcrossASecretChange(t *testing.T) {
	ctx := context.Background()
	reg, vault, log, _ := newSSHTestRegistry(t, errors.New("dial tcp: connection refused"))
	oldSecret, newSecret := "old", "new"
	host, err := reg.Create(ctx, piHost(), SecretInput{SSHSecret: &oldSecret})
	if err != nil {
		t.Fatal(err)
	}

	// The secret is replaced right after clientFor read the old one from the vault.
	var armed atomic.Bool
	armed.Store(true)
	vault.onRead = func(key string) {
		if key == SSHSecretKey(host.ID) && armed.CompareAndSwap(true, false) {
			if _, err := reg.Update(ctx, host.ID, host, SecretInput{SSHSecret: &newSecret}); err != nil {
				t.Errorf("Update during the build: %v", err)
			}
		}
	}
	client, err := reg.Client(ctx, host.ID)
	if err != nil {
		t.Fatal(err)
	}
	reg.Test(ctx, host.ID)
	if log.count() != 1 || log.last() != "pi.lan|new" {
		t.Fatalf("the client logged in as %q (%d logins), want only the replaced secret %q", log.last(), log.count(), "pi.lan|new")
	}
	if again, _ := reg.Client(ctx, host.ID); again != client {
		t.Fatal("the rebuilt client must be the cached one")
	}

	// A closed registry builds nothing.
	reg.Close()
	if _, err := reg.Client(ctx, host.ID); !errors.Is(err, errRegistryClosed) {
		t.Fatalf("Client after Close = %v, want errRegistryClosed", err)
	}
}
