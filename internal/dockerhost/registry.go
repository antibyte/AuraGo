package dockerhost

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerhost/transport"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

// SecretVault is the part of the AuraGo vault the registry needs.
type SecretVault interface {
	ReadSecret(key string) (string, error)
	WriteSecret(key, value string) error
	DeleteSecret(key string) error
}

// SSHSecretKey names the vault entry of a host's SSH private key or password.
func SSHSecretKey(id string) string { return "docker_host_ssh_" + id }

// TLSSecretKey names the vault entry of a host's TLS material (JSON ca/cert/key).
func TLSSecretKey(id string) string { return "docker_host_tls_" + id }

// LocalEndpointFunc returns the configured docker.host ("" = dockerutil.DefaultHost()).
type LocalEndpointFunc func() string

const (
	localHostName     = "Local"
	clientTimeout     = 30 * time.Second
	dialTimeout       = 5 * time.Second
	sshSocketOpenWait = 10 * time.Second
	testBudget        = 30 * time.Second
)

// secretBundle is everything the vault holds for one host.
type secretBundle struct {
	ssh string
	tls *transport.TLSMaterial
}

func (b secretBundle) tlsMaterial() transport.TLSMaterial {
	if b.tls == nil {
		return transport.TLSMaterial{}
	}
	return *b.tls
}

// tlsJSON is the vault form of the TLS material; "" means no entry.
func (b secretBundle) tlsJSON() string {
	if b.tls == nil {
		return ""
	}
	raw, _ := json.Marshal(b.tls)
	return string(raw)
}

// String and GoString keep the SSH secret out of a stray %v or %+v.
func (b secretBundle) String() string {
	return fmt.Sprintf("secretBundle{ssh:%s tls:%s}", setOrUnset(b.ssh != ""), setOrUnset(b.tls != nil))
}

func (b secretBundle) GoString() string { return b.String() }

func setOrUnset(set bool) string {
	if set {
		return "set"
	}
	return "unset"
}

type cachedClient struct {
	fingerprint string
	client      *dockerutil.Client
}

// clientBuildAttempts bounds how often clientFor rebuilds a client that an
// Update or Delete overtook while it was being built.
const clientBuildAttempts = 3

// errRegistryClosed is returned for a client requested after Close.
var errRegistryClosed = errors.New("the Docker host registry is closed")

// Registry is the list of Docker hosts: the built-in local host plus the stored
// remote hosts, their secrets in the vault, one cached Engine client per host
// and the last connection test result of each.
type Registry struct {
	store         *Store
	vault         SecretVault
	localEndpoint LocalEndpointFunc

	// now and dialSSH are fields so tests can drive the SSH login protection
	// without a clock or an SSH server.
	now     func() time.Time
	dialSSH func(ctx context.Context, target transport.SSHTarget, socketOpenTimeout time.Duration) (net.Conn, error)

	// writeMu serializes Create, Update and Delete, which hold it across vault and
	// store I/O. mu guards only the caches below and is never held across I/O.
	writeMu sync.Mutex

	mu         sync.Mutex
	clients    map[string]cachedClient
	status     map[string]HostStatus
	secretFlag map[string]bool
	guards     map[string]*sshGuard // SSH hosts: login breaker and dial gate
	generation map[string]uint64    // bumped by forget; a client built across a bump is discarded
	closed     bool
}

// NewRegistry returns a registry over a store and a vault. localEndpoint may be
// nil (the OS default Engine endpoint is used).
func NewRegistry(store *Store, vault SecretVault, localEndpoint LocalEndpointFunc) *Registry {
	return &Registry{
		store:         store,
		vault:         vault,
		localEndpoint: localEndpoint,
		now:           time.Now,
		dialSSH:       transport.DialEngineOverSSH,
		clients:       map[string]cachedClient{},
		status:        map[string]HostStatus{},
		secretFlag:    map[string]bool{},
		guards:        map[string]*sshGuard{},
		generation:    map[string]uint64{},
	}
}

func (r *Registry) localHost() Host {
	endpoint := ""
	if r.localEndpoint != nil {
		endpoint = r.localEndpoint()
	}
	return Host{
		ID:             LocalHostID,
		Name:           localHostName,
		Kind:           KindLocal,
		Address:        dockerutil.NormalizeHost(endpoint),
		StacksDir:      DefaultStacksDir,
		HostSocketPath: DefaultHostSocketPath,
		Color:          "blue",
		Sort:           -1,
	}
}

// Hosts returns the local host first, then the stored hosts.
func (r *Registry) Hosts(ctx context.Context) ([]Host, error) {
	stored, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return append([]Host{r.localHost()}, stored...), nil
}

// Host returns the local host or one stored host (ErrHostNotFound).
func (r *Registry) Host(ctx context.Context, id string) (Host, error) {
	if id == LocalHostID {
		return r.localHost(), nil
	}
	return r.store.Get(ctx, id)
}

// Create validates and stores a host and its secrets. A failure while writing
// the secrets removes the host again.
func (r *Registry) Create(ctx context.Context, h Host, sec SecretInput) (Host, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	h = h.Normalized()
	h.ID = ""
	if h.Color == "" {
		h.Color = r.nextColor(ctx)
	}
	if err := checkLocalName(h.Name); err != nil {
		return Host{}, err
	}
	if err := ValidateHost(h); err != nil {
		return Host{}, err
	}
	next, err := resolveSecrets(h, secretBundle{}, sec)
	if err != nil {
		return Host{}, err
	}
	created, err := r.store.Create(ctx, h)
	if err != nil {
		return Host{}, err
	}
	if _, err := r.applySecrets(created.ID, secretBundle{}, next); err != nil {
		if delErr := r.store.Delete(ctx, created.ID); delErr != nil {
			err = errors.Join(err, fmt.Errorf("the half-created host could not be removed again: %w", delErr))
		}
		return Host{}, err
	}
	return created, nil
}

// Update replaces a stored host and merges the secret input into its stored
// secrets. The cached client, the cached status and the SSH login pause are
// dropped. A stored SSH secret only carries over while the address, port and
// user stay the same; otherwise the SSH secret must be entered again (a
// ValidationError for ssh_secret), so it is never sent to another machine.
func (r *Registry) Update(ctx context.Context, id string, h Host, sec SecretInput) (Host, error) {
	if id == LocalHostID {
		return Host{}, ErrLocalHostFixed
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	current, err := r.store.Get(ctx, id)
	if err != nil {
		return Host{}, err
	}
	h = h.Normalized()
	h.ID = id
	if h.Color == "" {
		h.Color = current.Color
	}
	if err := checkLocalName(h.Name); err != nil {
		return Host{}, err
	}
	if err := ValidateHost(h); err != nil {
		return Host{}, err
	}
	prev, err := r.loadBundle(id)
	if err != nil {
		return Host{}, err
	}
	base, err := reusableSecrets(current, h, prev, sec)
	if err != nil {
		return Host{}, err
	}
	next, err := resolveSecrets(h, base, sec)
	if err != nil {
		return Host{}, err
	}
	undo, err := r.applySecrets(id, prev, next)
	if err != nil {
		return Host{}, err
	}
	updated, err := r.store.Update(ctx, h)
	if err != nil {
		undo()
		return Host{}, err
	}
	r.forget(id)
	return updated, nil
}

// Delete removes a stored host, its cached client and its vault secrets.
func (r *Registry) Delete(ctx context.Context, id string) error {
	if id == LocalHostID {
		return ErrLocalHostFixed
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if err := r.store.Delete(ctx, id); err != nil {
		return err
	}
	r.forget(id)
	if r.vault == nil {
		return nil
	}
	var errs []error
	for _, key := range []string{SSHSecretKey(id), TLSSecretKey(id)} {
		if err := r.vault.DeleteSecret(key); err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("host removed, but its stored secrets could not be deleted: %w", errors.Join(errs...))
	}
	return nil
}

// SecretSet reports whether the vault holds any secret for the host. The answer
// is remembered until the host is updated or deleted.
func (r *Registry) SecretSet(id string) bool {
	if id == LocalHostID || r.vault == nil {
		return false
	}
	r.mu.Lock()
	set, known := r.secretFlag[id]
	r.mu.Unlock()
	if known {
		return set
	}
	definitive := true // a vault error says nothing about the secret, so it is not remembered
	for _, key := range []string{SSHSecretKey(id), TLSSecretKey(id)} {
		value, err := r.vault.ReadSecret(key)
		if err == nil && value != "" {
			set = true
			break
		}
		if err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			definitive = false
		}
	}
	if set || definitive {
		r.mu.Lock()
		r.secretFlag[id] = set
		r.mu.Unlock()
	}
	return set
}

// Client returns the Engine client of a host. It is cached per host and rebuilt
// when the host's connection settings change.
func (r *Registry) Client(ctx context.Context, id string) (*dockerutil.Client, error) {
	return r.clientFor(ctx, id)
}

// clientFor returns the cached client of a host or builds one. The host's
// generation is read before the host record and the vault, and a built client
// is only cached when the generation is still the same: an Update, a secret
// change or a Delete that ran in between (forget bumps the generation) makes
// the client stale, so it is closed and rebuilt from the new state.
func (r *Registry) clientFor(ctx context.Context, id string) (*dockerutil.Client, error) {
	for attempt := 0; attempt < clientBuildAttempts; attempt++ {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return nil, errRegistryClosed
		}
		generation := r.generation[id]
		r.mu.Unlock()

		h, err := r.Host(ctx, id)
		if err != nil {
			return nil, err
		}
		fingerprint := hostFingerprint(h)
		r.mu.Lock()
		cached, ok := r.clients[id]
		var guard *sshGuard
		if h.Kind == KindSSH {
			guard = r.guardLocked(id)
		}
		r.mu.Unlock()
		if ok && cached.fingerprint == fingerprint {
			return cached.client, nil
		}

		bundle, err := r.bundleForClient(h)
		if err != nil {
			return nil, err
		}
		client, err := r.newClient(h, bundle, guard)
		if err != nil {
			return nil, err
		}

		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			client.CloseIdleConnections()
			return nil, errRegistryClosed
		}
		if r.generation[id] != generation {
			r.mu.Unlock()
			client.CloseIdleConnections() // built from state that has been replaced
			continue
		}
		if existing, found := r.clients[id]; found {
			if existing.fingerprint == fingerprint { // another goroutine built it first
				r.mu.Unlock()
				client.CloseIdleConnections()
				return existing.client, nil
			}
			existing.client.CloseIdleConnections() // the host's settings changed
		}
		r.clients[id] = cachedClient{fingerprint: fingerprint, client: client}
		r.mu.Unlock()
		return client, nil
	}
	return nil, errors.New("the host was changed while its connection was being prepared; try again")
}

// guardLocked returns the SSH login guard of a host; callers hold r.mu.
func (r *Registry) guardLocked(id string) *sshGuard {
	guard, ok := r.guards[id]
	if !ok {
		guard = newSSHGuard(r.now)
		r.guards[id] = guard
	}
	return guard
}

// Status returns the last connection test result of a host; State "unknown"
// before the first test.
func (r *Registry) Status(id string) HostStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if status, ok := r.status[id]; ok {
		return status
	}
	return HostStatus{State: StateUnknown}
}

// Test checks the connection of a stored host (GET /version, then /info) and
// caches the result.
//
// An SSH host whose login was rejected is not dialed again while its pause runs
// (see sshGuard); the test then reports the pause as an auth failure. Use
// TestNow for a deliberate attempt.
func (r *Registry) Test(ctx context.Context, id string) HostStatus {
	ctx, cancel := context.WithTimeout(ctx, testBudget)
	defer cancel()
	var status HostStatus
	client, err := r.clientFor(ctx, id)
	switch {
	case errors.Is(err, ErrHostNotFound):
		return errorStatus(err) // an unknown id is not cached
	case err != nil:
		status = errorStatus(err)
	default:
		status = probeHost(ctx, client)
	}
	r.mu.Lock()
	r.status[id] = status
	r.mu.Unlock()
	return status
}

// TestNow is Test as a deliberate user action (the Test and Retry buttons): it
// first lifts the running SSH login pause of the host, so exactly one login is
// attempted even after a rejected password. The failure series stays: if that
// attempt is rejected too, the next pause is the next one of the series (2 min,
// 4 min, ...), not 2 min again. A successful login, Update and Delete reset the
// series. Background tests use Test.
func (r *Registry) TestNow(ctx context.Context, id string) HostStatus {
	r.mu.Lock()
	if guard, ok := r.guards[id]; ok {
		guard.breaker.lift()
	}
	r.mu.Unlock()
	return r.Test(ctx, id)
}

// TestDraft checks the connection of a host that is not saved. Secrets the
// input leaves out come from the stored host when h.ID names one, except a
// stored SSH secret when the address, port or user differs from the stored host
// (the same rule as Update). The client is not cached, is closed afterwards and
// neither consults nor changes the SSH login pause: every click on the form's
// Test button is one deliberate attempt, so it is unguarded by design.
//
// A draft that cannot be tested at all (an invalid field, a missing or
// unusable secret) is returned as the *ValidationError, like Create and Update
// report it; only the outcome of a real connection attempt, or a failure to
// prepare one, is a HostStatus.
func (r *Registry) TestDraft(ctx context.Context, h Host, sec SecretInput) (HostStatus, error) {
	h = h.Normalized()
	if h.Color == "" {
		h.Color = Colors[0]
	}
	if err := ValidateHost(h); err != nil {
		return HostStatus{}, err
	}
	var prev secretBundle
	if h.ID != "" && h.ID != LocalHostID {
		if stored, err := r.store.Get(ctx, h.ID); err == nil {
			if prev, err = r.loadBundle(h.ID); err != nil {
				return errorStatus(err), nil
			}
			if prev, err = reusableSecrets(stored, h, prev, sec); err != nil {
				return HostStatus{}, err
			}
		}
	}
	bundle, err := resolveSecrets(h, prev, sec)
	if err != nil {
		return HostStatus{}, err
	}
	client, err := r.newClient(h, bundle, nil)
	if err != nil {
		return errorStatus(err), nil
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, testBudget)
	defer cancel()
	return probeHost(ctx, client), nil
}

// Close closes the idle connections of the cached clients, which also ends the
// SSH sessions they hold. A closed registry builds no further clients.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	for id, cached := range r.clients {
		cached.client.CloseIdleConnections()
		delete(r.clients, id)
	}
}

// forget drops everything cached for a host: the client, the status, the SSH
// login pause and gate. It bumps the host's generation so that a client that is
// being built right now (possibly from the secret that was just replaced or
// removed) is discarded instead of cached.
func (r *Registry) forget(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generation[id]++
	if guard, ok := r.guards[id]; ok {
		guard.breaker.reset()
		delete(r.guards, id)
	}
	if cached, ok := r.clients[id]; ok {
		cached.client.CloseIdleConnections()
		delete(r.clients, id)
	}
	delete(r.status, id)
	delete(r.secretFlag, id)
}

func (r *Registry) nextColor(ctx context.Context) string {
	stored, err := r.store.List(ctx)
	if err != nil {
		return Colors[1]
	}
	return Colors[(len(stored)+1)%len(Colors)]
}

func checkLocalName(name string) error {
	if strings.EqualFold(strings.TrimSpace(name), localHostName) {
		return invalid("name", "another host already uses this name")
	}
	return nil
}

// errSSHSecretIdentity is what an edit that moves an SSH host gets back.
const errSSHSecretIdentity = "Enter the SSH password or key again: the address, port or user changed."

// sshSecretCarriesOver reports whether the SSH secret stored for the stored host
// may be used for the edited one: only for the same SSH machine and login (the
// address compared bare and ignoring case, the effective port and the user).
func sshSecretCarriesOver(stored, next Host) bool {
	return stored.Kind == KindSSH && next.Kind == KindSSH &&
		sameAddress(stored.Address, next.Address) &&
		effectivePort(stored) == effectivePort(next) &&
		stored.SSHUser == next.SSHUser
}

func sameAddress(a, b string) bool {
	a, b = bareAddress(strings.TrimSpace(a)), bareAddress(strings.TrimSpace(b))
	if ipA, ipB := net.ParseIP(a), net.ParseIP(b); ipA != nil && ipB != nil {
		return ipA.Equal(ipB)
	}
	return strings.EqualFold(a, b)
}

// reusableSecrets returns the stored secrets an edit of the stored host into
// next may build on. A stored SSH secret is dropped when it would reach a
// different machine or login than the one it was stored for; the edit must then
// bring a new one, else the ssh_secret ValidationError is returned. TLS
// material keeps merging: it only signs or verifies and is never sent as such.
func reusableSecrets(stored, next Host, prev secretBundle, in SecretInput) (secretBundle, error) {
	if next.Kind != KindSSH || prev.ssh == "" || sshSecretCarriesOver(stored, next) {
		return prev, nil
	}
	prev.ssh = ""
	if in.SSHSecret == nil || strings.TrimSpace(*in.SSHSecret) == "" {
		return prev, invalid("ssh_secret", "%s", errSSHSecretIdentity)
	}
	return prev, nil
}

// resolveSecrets merges the input into the stored secrets for the kind of h and
// checks the result: an SSH host needs a secret, a TLS host needs usable PEM
// material. Secrets that the kind does not use are dropped.
func resolveSecrets(h Host, stored secretBundle, in SecretInput) (secretBundle, error) {
	out := stored
	if in.Clear {
		out = secretBundle{}
	}
	if in.SSHSecret != nil && strings.TrimSpace(*in.SSHSecret) != "" {
		out.ssh = *in.SSHSecret
	}
	if in.TLS != nil {
		merged := out.tlsMaterial()
		if v := strings.TrimSpace(in.TLS.CA); v != "" {
			merged.CA = v
		}
		if v := strings.TrimSpace(in.TLS.Cert); v != "" {
			merged.Cert = v
		}
		if v := strings.TrimSpace(in.TLS.Key); v != "" {
			merged.Key = v
		}
		out.tls = &merged
	}
	switch h.Kind {
	case KindSSH:
		out.tls = nil
		if strings.TrimSpace(out.ssh) == "" {
			return secretBundle{}, invalid("ssh_secret", "an SSH password or private key is required")
		}
	case KindTLS, KindMTLS:
		out.ssh = ""
		material := out.tlsMaterial()
		if h.Kind == KindTLS {
			material.Cert, material.Key = "", "" // server-only TLS never presents a client certificate
		}
		if _, err := transport.TLSClientConfig(h.Kind, material); err != nil {
			return secretBundle{}, invalid("tls", "%s", err.Error())
		}
		out.tls = &material
		if material == (transport.TLSMaterial{}) {
			out.tls = nil // system roots: nothing to store
		}
	default:
		out = secretBundle{}
	}
	return out, nil
}

// errTLSMaterialUnreadable reports a vault entry that is not valid JSON.
var errTLSMaterialUnreadable = errors.New("the stored TLS material of this host is unreadable")

func (r *Registry) readSSH(id string) (string, error) {
	if r.vault == nil {
		return "", errors.New("vault is not available")
	}
	value, err := r.vault.ReadSecret(SSHSecretKey(id))
	if errors.Is(err, security.ErrSecretNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read SSH secret: %w", err)
	}
	return value, nil
}

func (r *Registry) readTLS(id string) (*transport.TLSMaterial, error) {
	if r.vault == nil {
		return nil, errors.New("vault is not available")
	}
	raw, err := r.vault.ReadSecret(TLSSecretKey(id))
	if errors.Is(err, security.ErrSecretNotFound) || (err == nil && raw == "") {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read TLS material: %w", err)
	}
	var material transport.TLSMaterial
	if err := json.Unmarshal([]byte(raw), &material); err != nil {
		return nil, errTLSMaterialUnreadable
	}
	return &material, nil
}

// loadBundle reads both secrets of a host for an update. Unreadable TLS
// material counts as empty so that the update can replace it.
func (r *Registry) loadBundle(id string) (secretBundle, error) {
	var b secretBundle
	if r.vault == nil {
		return b, nil
	}
	ssh, err := r.readSSH(id)
	if err != nil {
		return b, err
	}
	material, err := r.readTLS(id)
	if err != nil && !errors.Is(err, errTLSMaterialUnreadable) {
		return b, err
	}
	b.ssh, b.tls = ssh, material
	return b, nil
}

// bundleForClient reads the secret a host's transport uses.
func (r *Registry) bundleForClient(h Host) (secretBundle, error) {
	var b secretBundle
	switch h.Kind {
	case KindSSH:
		secret, err := r.readSSH(h.ID)
		if err != nil {
			return b, err
		}
		b.ssh = secret
	case KindTLS, KindMTLS:
		material, err := r.readTLS(h.ID)
		if err != nil {
			if errors.Is(err, errTLSMaterialUnreadable) {
				return b, withCode(CodeTLS, err)
			}
			return b, err
		}
		b.tls = material
	}
	return b, nil
}

// applySecrets writes the difference between prev and next to the vault and
// returns a function that puts the vault back as it was. A failure part-way
// restores what was already written.
func (r *Registry) applySecrets(id string, prev, next secretBundle) (func(), error) {
	var undo []func()
	rollback := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}
	steps := []struct{ key, before, after string }{
		{SSHSecretKey(id), prev.ssh, next.ssh},
		{TLSSecretKey(id), prev.tlsJSON(), next.tlsJSON()},
	}
	for _, step := range steps {
		if step.before == step.after {
			continue
		}
		if r.vault == nil {
			rollback()
			return nil, errors.New("vault is not available")
		}
		if err := r.putSecret(step.key, step.after); err != nil {
			rollback()
			return nil, fmt.Errorf("store host secret: %w", err)
		}
		key, before := step.key, step.before
		undo = append(undo, func() { _ = r.putSecret(key, before) })
	}
	r.mu.Lock()
	delete(r.secretFlag, id)
	r.mu.Unlock()
	return rollback, nil
}

// putSecret writes a vault entry, or deletes it for an empty value.
func (r *Registry) putSecret(key, value string) error {
	if value == "" {
		if err := r.vault.DeleteSecret(key); err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			return err
		}
		return nil
	}
	return r.vault.WriteSecret(key, value)
}

// hostFingerprint changes whenever a setting that shapes the connection
// changes, so a cached client is replaced.
func hostFingerprint(h Host) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		h.Kind, h.Address, strconv.Itoa(effectivePort(h)), h.HostSocketPath, h.SSHUser,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// endpointLabel names the connection without any secret; it is what
// Client.Host reports.
func endpointLabel(h Host) string {
	switch h.Kind {
	case KindLocal:
		return h.Address
	case KindSSH:
		return "ssh://" + h.SSHUser + "@" + net.JoinHostPort(bareAddress(h.Address), strconv.Itoa(effectivePort(h)))
	default:
		return h.Kind + "://" + net.JoinHostPort(bareAddress(h.Address), strconv.Itoa(effectivePort(h)))
	}
}

// newClient builds the Engine client of a host from its settings and secrets.
// Nothing is dialed until the first request. guard (SSH only, may be nil)
// pauses logins after authentication failures and lets one login run at a time.
func (r *Registry) newClient(h Host, b secretBundle, guard *sshGuard) (*dockerutil.Client, error) {
	var dial func(ctx context.Context) (net.Conn, error)
	var probeTimeout time.Duration
	hostPort := net.JoinHostPort(bareAddress(h.Address), strconv.Itoa(effectivePort(h)))
	switch h.Kind {
	case KindLocal:
		endpoint := h.Address
		dial = func(ctx context.Context) (net.Conn, error) { return dockerutil.DialContext(ctx, endpoint) }
	case KindTCP:
		dialer := &net.Dialer{Timeout: dialTimeout}
		dial = func(ctx context.Context) (net.Conn, error) { return dialer.DialContext(ctx, "tcp", hostPort) }
	case KindTLS, KindMTLS:
		config, err := transport.TLSClientConfig(h.Kind, b.tlsMaterial())
		if err != nil {
			return nil, withCode(CodeTLS, err)
		}
		config.ServerName = bareAddress(h.Address)
		dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: dialTimeout}, Config: config}
		dial = func(ctx context.Context) (net.Conn, error) { return dialer.DialContext(ctx, "tcp", hostPort) }
	case KindSSH:
		if strings.TrimSpace(b.ssh) == "" {
			return nil, withCode(CodeAuth, errors.New("no SSH password or private key is stored for this host"))
		}
		target := transport.SSHTarget{
			Host:   bareAddress(h.Address),
			Port:   effectivePort(h),
			User:   h.SSHUser,
			Secret: []byte(b.ssh),
			Socket: h.HostSocketPath,
		}
		login := func(ctx context.Context) (net.Conn, error) {
			return r.dialSSH(ctx, target, sshSocketOpenWait)
		}
		dial = login
		if guard != nil {
			dial = func(ctx context.Context) (net.Conn, error) { return guard.dial(ctx, login) }
		}
		probeTimeout = transport.SSHProbeTimeout
	default:
		return nil, fmt.Errorf("unsupported host kind %q", h.Kind)
	}
	return dockerutil.NewClientWithDialer(endpointLabel(h), dial, clientTimeout, probeTimeout), nil
}
