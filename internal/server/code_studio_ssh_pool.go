package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/inventory"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	codeStudioSSHKeepalive = 30 * time.Second
	codeStudioSSHIdleClose = 10 * time.Minute
	codeStudioSSHExecSlots = 4 // short sessions; + 4 long + 1 SFTP stays below OpenSSH MaxSessions 10
	codeStudioSSHLongSlots = 4 // terminals + processes
)

type codeStudioSSHTarget struct {
	DeviceID, Name, Host string
	Port                 int
	User                 string
}

type codeStudioSSHDialFunc func(ctx context.Context, deviceID, trustFingerprint string) (*ssh.Client, codeStudioSSHTarget, error)

type codeStudioSSHSessionOpenError struct{ err error }

func (e codeStudioSSHSessionOpenError) Error() string { return "open SSH session: " + e.err.Error() }
func (e codeStudioSSHSessionOpenError) Unwrap() error { return e.err }

func codeStudioSSHRetrySessionOpen(err error) bool {
	var open codeStudioSSHSessionOpenError
	return errors.As(err, &open)
}

func codeStudioSSHRetryAlways(error) bool { return true }

func codeStudioSSHBroken(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||
		errors.Is(err, sftp.ErrSSHFxConnectionLost) || errors.Is(err, sftp.ErrSSHFxNoConnection) {
		return true
	}
	text := err.Error()
	return strings.Contains(text, "use of closed network connection") || strings.Contains(text, "connection lost")
}

type codeStudioSSHConn struct {
	target    codeStudioSSHTarget
	client    *ssh.Client
	sftp      *sftp.Client
	execSem   chan struct{}
	longSem   chan struct{}
	lastUsed  atomic.Int64
	done      chan struct{}
	closeOnce sync.Once
	rootsMu   sync.Mutex
	realRoots map[string]string
}

func (c *codeStudioSSHConn) dead() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *codeStudioSSHConn) touch()     { c.lastUsed.Store(time.Now().UnixNano()) }
func (c *codeStudioSSHConn) busy() bool { return len(c.longSem) > 0 || len(c.execSem) > 0 }

func (c *codeStudioSSHConn) close() {
	c.closeOnce.Do(func() { _ = c.sftp.Close(); _ = c.client.Close() })
}

// run executes cmdline in one session. stop (optional) kills it early (search output full).
func (c *codeStudioSSHConn) run(ctx context.Context, cmdline string, stdin []byte, stdout, stderr io.Writer, stop <-chan struct{}) (int, error) {
	select {
	case c.execSem <- struct{}{}:
	case <-ctx.Done():
		return -1, ctx.Err()
	}
	defer func() { <-c.execSem; c.touch() }()
	session, err := c.client.NewSession()
	if err != nil {
		return -1, codeStudioSSHSessionOpenError{err}
	}
	defer session.Close()
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}
	session.Stdout, session.Stderr = stdout, stderr
	if err := session.Start(cmdline); err != nil {
		return -1, codeStudioSSHSessionOpenError{err}
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- session.Wait() }()
	kill := func() {
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		select {
		case <-waitErr:
		case <-time.After(2 * time.Second):
		}
	}
	select {
	case err := <-waitErr:
		return codeStudioSSHExitCode(err)
	case <-stop:
		kill()
		return 0, nil
	case <-ctx.Done():
		kill()
		return -1, ctx.Err()
	}
}

func codeStudioSSHExitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus(), nil
	}
	var missing *ssh.ExitMissingError
	if errors.As(err, &missing) {
		return -1, nil
	}
	return -1, err
}

type codeStudioSSHPool struct {
	dial      codeStudioSSHDialFunc
	logger    *slog.Logger
	keepalive time.Duration
	idle      time.Duration
	mu        sync.Mutex
	conns     map[string]*codeStudioSSHConn
	dialLocks map[string]*sync.Mutex
	janitor   sync.Once
	stopOnce  sync.Once
	stop      chan struct{}
}

func newCodeStudioSSHPool(dial codeStudioSSHDialFunc, logger *slog.Logger) *codeStudioSSHPool {
	return &codeStudioSSHPool{dial: dial, logger: logger, keepalive: codeStudioSSHKeepalive, idle: codeStudioSSHIdleClose,
		conns: map[string]*codeStudioSSHConn{}, dialLocks: map[string]*sync.Mutex{}, stop: make(chan struct{})}
}

func (p *codeStudioSSHPool) dialLock(deviceID string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dialLocks[deviceID] == nil {
		p.dialLocks[deviceID] = &sync.Mutex{}
	}
	return p.dialLocks[deviceID]
}

// get returns the pooled connection or dials one. trust is only used for a new dial.
func (p *codeStudioSSHPool) get(ctx context.Context, deviceID, trust string) (*codeStudioSSHConn, error) {
	p.janitor.Do(func() { go p.reap() })
	lock := p.dialLock(deviceID)
	lock.Lock()
	defer lock.Unlock()
	p.mu.Lock()
	if c := p.conns[deviceID]; c != nil && !c.dead() {
		p.mu.Unlock()
		c.touch()
		return c, nil
	}
	p.mu.Unlock()
	client, target, err := p.dial(ctx, deviceID, trust)
	if err != nil {
		return nil, err
	}
	sc, err := sftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		return nil, &codeStudioRemoteError{Status: http.StatusBadGateway, Code: "sftp_unavailable",
			Message: "SFTP is not available on " + target.Host + ": " + err.Error(), Err: err}
	}
	c := &codeStudioSSHConn{target: target, client: client, sftp: sc, execSem: make(chan struct{}, codeStudioSSHExecSlots),
		longSem: make(chan struct{}, codeStudioSSHLongSlots), done: make(chan struct{}), realRoots: map[string]string{}}
	c.touch()
	go func() { _ = client.Wait(); close(c.done); c.close() }()
	go p.keepAlive(c)
	p.mu.Lock()
	if old := p.conns[deviceID]; old != nil {
		old.close()
	}
	p.conns[deviceID] = c
	p.mu.Unlock()
	return c, nil
}

func (p *codeStudioSSHPool) keepAlive(c *codeStudioSSHConn) {
	ticker := time.NewTicker(p.keepalive)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-p.stop:
			return
		case <-ticker.C:
			reply := make(chan error, 1)
			go func() { _, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil); reply <- err }()
			select {
			case err := <-reply:
				if err != nil {
					c.close()
					return
				}
			case <-time.After(15 * time.Second):
				c.close()
				return
			case <-c.done:
				return
			}
		}
	}
}

func (p *codeStudioSSHPool) reap() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			return
		case now := <-ticker.C:
			p.closeIdle(now)
		}
	}
}

func (p *codeStudioSSHPool) closeIdle(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, c := range p.conns {
		if c.dead() || (!c.busy() && now.Sub(time.Unix(0, c.lastUsed.Load())) > p.idle) {
			c.close()
			delete(p.conns, id)
		}
	}
}

func (p *codeStudioSSHPool) drop(deviceID string, c *codeStudioSSHConn) {
	p.mu.Lock()
	if p.conns[deviceID] == c {
		delete(p.conns, deviceID)
	}
	p.mu.Unlock()
	c.close()
}

// release closes an idle connection (workspace forgotten); busy ones are left to the reaper.
func (p *codeStudioSSHPool) release(deviceID string) {
	p.mu.Lock()
	c := p.conns[deviceID]
	if c != nil && !c.busy() {
		delete(p.conns, deviceID)
	}
	p.mu.Unlock()
	if c != nil && !c.busy() {
		c.close()
	}
}

func (p *codeStudioSSHPool) connected(deviceID string) (codeStudioSSHTarget, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c := p.conns[deviceID]; c != nil && !c.dead() {
		return c.target, true
	}
	return codeStudioSSHTarget{}, false
}

func (p *codeStudioSSHPool) closeAll() {
	p.stopOnce.Do(func() { close(p.stop) })
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, c := range p.conns {
		c.close()
		delete(p.conns, id)
	}
}

// withConn runs fn and, when retryable(err) and the connection is dead/broken, redials once.
func (p *codeStudioSSHPool) withConn(ctx context.Context, deviceID string, retryable func(error) bool, fn func(*codeStudioSSHConn) error) error {
	c, err := p.get(ctx, deviceID, "")
	if err != nil {
		return err
	}
	err = fn(c)
	if err == nil || retryable == nil || !retryable(err) || !(c.dead() || codeStudioSSHBroken(err)) {
		return err
	}
	p.drop(deviceID, c)
	c, err = p.get(ctx, deviceID, "")
	if err != nil {
		return err
	}
	return fn(c)
}

// ---- per-server remote state ----

type codeStudioRemote struct {
	pool    *codeStudioSSHPool
	touchMu sync.Mutex
	touched map[string]time.Time
}

func newCodeStudioRemote(dial codeStudioSSHDialFunc, logger *slog.Logger) *codeStudioRemote {
	return &codeStudioRemote{pool: newCodeStudioSSHPool(dial, logger), touched: map[string]time.Time{}}
}

// shouldTouch throttles last_used_at writes to one per workspace per 5 minutes.
func (r *codeStudioRemote) shouldTouch(id string, now time.Time) bool {
	r.touchMu.Lock()
	defer r.touchMu.Unlock()
	if now.Sub(r.touched[id]) < 5*time.Minute {
		return false
	}
	r.touched[id] = now
	return true
}

var codeStudioRemotes sync.Map // *Server -> *codeStudioRemote

func (h codeStudioHandlers) remoteState() *codeStudioRemote {
	if h.remote != nil {
		return h.remote
	}
	if v, ok := codeStudioRemotes.Load(h.server); ok {
		return v.(*codeStudioRemote)
	}
	v, _ := codeStudioRemotes.LoadOrStore(h.server, newCodeStudioRemote(codeStudioDefaultSSHDialer(h.server), h.codeStudioLogger()))
	return v.(*codeStudioRemote)
}

func codeStudioDefaultSSHDialer(s *Server) codeStudioSSHDialFunc {
	return func(ctx context.Context, deviceID, trust string) (*ssh.Client, codeStudioSSHTarget, error) {
		if s == nil || s.InventoryDB == nil {
			return nil, codeStudioSSHTarget{}, &codeStudioRemoteError{Status: http.StatusServiceUnavailable,
				Code: "inventory_unavailable", Message: "inventory database not configured"}
		}
		access, err := desktop.ResolveSSHDevice(s.InventoryDB, s.Vault, deviceID)
		if errors.Is(err, desktop.ErrSSHDeviceNotFound) {
			return nil, codeStudioSSHTarget{}, &codeStudioRemoteError{Status: http.StatusNotFound, Code: "device_not_found", Message: "device not found"}
		}
		if err != nil {
			return nil, codeStudioSSHTarget{}, &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "ssh_access_invalid", Message: err.Error()}
		}
		target := codeStudioSSHTarget{DeviceID: access.DeviceID, Name: access.Name, Host: access.Host, Port: access.Port, User: access.Username}
		client, err := desktop.DialSSH(ctx, access, desktop.SSHDialOptions{TrustFingerprint: trust, Logger: s.Logger})
		if err != nil {
			return nil, target, codeStudioClassifyDialError(err, target)
		}
		return client, target, nil
	}
}

func codeStudioClassifyDialError(err error, target codeStudioSSHTarget) error {
	var hk *desktop.SSHHostKeyError
	if errors.As(err, &hk) {
		msg := "Unknown SSH host key for " + hk.Host
		if hk.Code == desktop.SSHHostKeyMismatch {
			msg = "The SSH host key of " + hk.Host + " changed. Check the host, then fix ~/.ssh/known_hosts on the AuraGo server."
		}
		return &codeStudioRemoteError{Status: http.StatusConflict, Code: hk.Code, Message: msg,
			Host: hk.Host, KeyType: hk.KeyType, Fingerprint: hk.Fingerprint, Err: err}
	}
	if strings.Contains(err.Error(), "unable to authenticate") {
		return &codeStudioRemoteError{Status: http.StatusBadGateway, Code: "ssh_auth_failed",
			Message: "SSH authentication failed for " + target.User + "@" + target.Host, Err: err}
	}
	return &codeStudioRemoteError{Status: http.StatusBadGateway, Code: "ssh_unreachable",
		Message: "SSH connection to " + target.Host + " failed: " + err.Error(), Err: err}
}

// codeStudioRemoteCallerAllowed: SSH workspaces are remote access; bearer tokens need desktop:admin.
func codeStudioRemoteCallerAllowed(s *Server, r *http.Request) bool {
	raw, isBearer := bearerCredential(r.Header.Get("Authorization"))
	if !isBearer {
		return true // cookie/no-auth callers already passed requirePermission
	}
	tm := s.currentTokenManager()
	if tm == nil || raw == "" {
		return false
	}
	_, ok := tm.Validate(raw, desktopScopeAdmin)
	return ok
}

func codeStudioRemoteScopeError() error {
	return &codeStudioRemoteError{Status: http.StatusForbidden, Code: "remote_scope_required",
		Message: "SSH workspaces need a session or a desktop:admin token"}
}

// codeStudioBackendByID resolves a workspace id without any request: ""/"local" is the
// container, anything else a saved SSH workspace. The SSH connection is NOT dialed here;
// it opens lazily on the first operation (host key 409 / auth errors surface there).
// An unknown id returns an error matching errCodeStudioWorkspaceNotFound. Callers that
// act for an HTTP request must apply codeStudioRemoteCallerAllowed first (backendFor does).
func (h *codeStudioHandlers) codeStudioBackendByID(ctx context.Context, id string) (codeStudioBackend, error) {
	id = strings.TrimSpace(id)
	if id == "" || id == codeStudioLocalWorkspaceID {
		return h.localBackend(ctx)
	}
	if !codeStudioWorkspaceIDPattern.MatchString(id) {
		return nil, &codeStudioRemoteError{Status: http.StatusBadRequest, Code: "invalid_workspace", Message: "invalid workspace id"}
	}
	store, err := h.workspaceStore(ctx)
	if err != nil {
		return nil, codeStudioUnavailableError{err}
	}
	rec, err := store.Get(ctx, id)
	if errors.Is(err, errCodeStudioWorkspaceRecordNotFound) {
		return nil, codeStudioWorkspaceNotFound()
	}
	if err != nil {
		return nil, err
	}
	remote := h.remoteState()
	if remote.shouldTouch(rec.ID, time.Now()) {
		_ = store.Touch(ctx, rec.ID, nil)
	}
	return newCodeStudioSSHBackend(rec, remote.pool, h.codeStudioSSHTargetFor(remote, rec.DeviceID)), nil
}

// codeStudioSSHTargetFor describes the device for Info() without dialing or reading the vault.
func (h codeStudioHandlers) codeStudioSSHTargetFor(remote *codeStudioRemote, deviceID string) codeStudioSSHTarget {
	if target, ok := remote.pool.connected(deviceID); ok {
		return target
	}
	target := codeStudioSSHTarget{DeviceID: deviceID}
	if h.server != nil && h.server.InventoryDB != nil {
		if device, err := inventory.GetDeviceByID(h.server.InventoryDB, deviceID); err == nil {
			a := desktop.DescribeSSHDevice(h.server.InventoryDB, device)
			target = codeStudioSSHTarget{DeviceID: deviceID, Name: a.Name, Host: a.Host, Port: a.Port, User: a.Username}
		}
	}
	return target
}
