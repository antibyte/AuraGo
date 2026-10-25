package ruview

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

const containerName = "aurago-ruview"

type nativeProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error // Published by closing done; readers must wait for done first.
	kill func() error
}

func configCheck(c *config.Config) error {
	return config.ValidateRuViewConfig(c.RuView, c.Runtime.IsDocker)
}

func (m *Manager) Start(ctx context.Context) error {
	return m.start(ctx, nil)
}

// warn logs without assuming a logger was supplied.
func (m *Manager) warn(msg string, args ...any) {
	if m.logger != nil {
		m.logger.Warn(msg, args...)
	}
}

// startCancelled reports a revoked start. Called with opMu held, right after
// the slow pull or verification, before anything is created.
func startCancelled(ctx, generation context.Context) error {
	if generation.Err() != nil {
		return ctxError(context.Canceled)
	}
	if err := ctx.Err(); err != nil {
		return ctxError(err)
	}
	return nil
}

// start runs one managed start. Startup carries its original generation so a
// preceding stop/config change wins.
//
// Lock use (lifecycle.go): opMu is taken to decide and claim the lifecycle
// slot in one mu section, released for the slow part (image pull, package
// verification) and taken again for the short create/launch and the commit. mu
// guards every state change. The deferred epilogue clears the slot under mu
// while opMu is still held, so a start that follows always sees a settled state.
func (m *Manager) start(ctx context.Context, expected context.Context) (startErr error) {
	if err := m.opMu.acquire(ctx, m.lockWait); err != nil {
		return err
	}
	// One mu section reads the generation, applies the gates and claims the
	// slot, so a Configure cannot land between the check and the claim and
	// leave a "starting" status behind a cancelled return. The config snapshot
	// is a lock-free read.
	m.mu.Lock()
	m.reapNativeLocked()
	generation := m.generation
	c := m.config()
	var refusal error
	alreadyStarted := false
	switch {
	case ctx.Err() != nil:
		refusal = ctxError(ctx.Err())
	case generation.Err() != nil || expected != nil && expected != generation:
		refusal = ctxError(context.Canceled)
	case !c.RuView.Enabled:
		refusal = fail(CodeDisabled, "RuView start requires enabled, writable integration", nil)
	case c.RuView.ReadOnly:
		refusal = fail(CodeReadOnly, "RuView start requires enabled, writable integration", nil)
	case c.RuView.Mode == "external":
		refusal = fail(CodeExternalRuntime, "external RuView is managed by its operator", nil)
	case m.op != "":
		refusal = fail(CodeBusy, "RuView is busy with another start or installation", nil)
	case m.managed && expected != nil:
		alreadyStarted = true
	case m.managed:
		refusal = fail(CodeAlreadyRunning, "RuView is already started; stop it before reconfiguration", nil)
	}
	if refusal != nil || alreadyStarted {
		m.mu.Unlock()
		m.opMu.release()
		return refusal // nil for alreadyStarted: the automatic start found it running
	}
	m.op = opStarting
	m.setPhaseLocked(PhasePreparing)
	m.status = Status{DataState: "starting"}
	m.startFailed = false
	m.observed = time.Time{}
	m.stopReason, m.stoppedAt = "", time.Time{}
	m.mu.Unlock()
	// The work is bound to the captured generation: Stop and Configure abort a
	// pull or a create by cancelling it, without waiting for any lock.
	ctx, cancel := context.WithCancel(ctx)
	stopBinding := context.AfterFunc(generation, cancel)
	holding := true // opMu is held until the slot is released below
	defer func() {
		m.endStart(generation, startErr)
		if holding {
			m.opMu.release()
		}
		stopBinding()
		cancel()
		// Hints run with no lock held (a cancelled start's cleanup stopped the runtime).
		m.flushStopNote()
		if startErr == nil {
			m.noteStop(true, c.RuView.Mode)
		}
	}()
	m.opMu.release()
	holding = false

	if err := configCheck(c); err != nil {
		return fail(CodeInvalidConfig, err.Error(), err)
	}
	dir, err := filepath.Abs(filepath.Join(c.Directories.DataDir, "ruview", "data"))
	if err != nil {
		return fail(CodeStartFailed, err.Error(), err)
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return fail(CodeStartFailed, err.Error(), err)
	}
	if i, err := os.Lstat(dir); err != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return fail(CodeStartFailed, "invalid RuView data directory", err)
	}
	token, err := m.managedToken()
	if err != nil {
		return fail(CodeStartFailed, err.Error(), err)
	}
	if c.RuView.Mode == "docker" {
		if err := m.ensureImage(ctx, c); err != nil { // slow, no lock held
			return err
		}
		if err := m.opMu.acquire(ctx, m.lockWait); err != nil {
			return err
		}
		holding = true
		if err := startCancelled(ctx, generation); err != nil {
			return err
		}
		m.setPhase(PhaseCreating)
		createCtx, cancelCreate := context.WithTimeout(ctx, createTimeout) // bounds how long opMu is held
		err := m.createContainer(createCtx, c, dir, token)
		cancelCreate()
		if err != nil {
			return err
		}
	} else {
		m.setPhase(PhaseVerifying)
		root, _, err := m.verifyNative(c) // hashes the package, no lock held
		if err != nil {
			return err
		}
		if err := m.opMu.acquire(ctx, m.lockWait); err != nil {
			return err
		}
		holding = true
		if err := startCancelled(ctx, generation); err != nil {
			return err
		}
		m.setPhase(PhaseCreating)
		if err := m.launchNative(c, root, dir, token); err != nil {
			return err
		}
	}
	// Commit: opMu is held. Ownership is recorded even when the start was
	// revoked meanwhile, so the cleanup below can find and stop what it made.
	m.mu.Lock()
	m.managed = true
	m.runningConfig = c
	m.runSeq++
	m.runID = m.runSeq
	current := generation == m.generation && generation.Err() == nil && ctx.Err() == nil
	if current {
		m.setPhaseLocked(PhaseWaitingReady)
	}
	m.mu.Unlock()
	if !current {
		cleanup, cancelCleanup := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelCleanup()
		if err := m.stopRuntime(cleanup, ""); err != nil { // keeps the reason Configure or Stop recorded
			return fmt.Errorf("RuView start cancelled; cleanup failed: %w", err)
		}
		return ctxError(context.Canceled)
	}
	return nil
}

// endStart releases the lifecycle slot and records a failure. A start that was
// revoked (the generation moved on) does not overwrite the status Stop or
// Configure wrote. A start the caller gave up on (cancelled with a generation
// that is still current) is not a failure: the "starting" placeholder is
// dropped and no error is recorded. Any other failure is sticky (see
// Manager.startFailed) and is logged raw, server side only.
func (m *Manager) endStart(generation context.Context, startErr error) {
	m.mu.Lock()
	m.op = ""
	if startErr == nil {
		m.mu.Unlock()
		return
	}
	m.clearPhaseLocked()
	e := AsError(startErr)
	current := generation == m.generation && generation.Err() == nil
	record := current && e.Code != CodeCancelled
	switch {
	case record:
		m.recordFailureLocked(startErr)
	case current:
		m.status = Status{}
	}
	m.mu.Unlock()
	if record {
		m.warn("RuView start failed", "error_code", e.Code, "error", rawText(startErr))
	}
}

// recordFailureLocked stores a failed start or install as the status error.
// The failure is sticky: a later probe or an unrelated Configure keeps it while
// nothing runs, and an explicit Start, Stop or Install or a runtime-relevant
// Configure replaces it. Called with mu held.
func (m *Manager) recordFailureLocked(err error) {
	e := AsError(err)
	m.status = Status{DataState: "disconnected", Error: legacyText(err), ErrorCode: e.Code, ErrorDetail: e.Detail}
	m.observed = time.Time{}
	m.startFailed = true
}

func (m *Manager) managedToken() (string, error) {
	if m.vault == nil {
		return "", fmt.Errorf("RuView requires the secrets Vault")
	}
	if token, err := m.vault.ReadSecret(ManagedTokenKey); err == nil && len(token) >= 32 {
		security.RegisterSensitive(token)
		return token, nil
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b[:])
	security.RegisterSensitive(token)
	if err := m.vault.WriteSecret(ManagedTokenKey, token); err != nil {
		return "", err
	}
	return token, nil
}

func runtimeArgs(c *config.Config, dir, bind string, port int) []string {
	udpPort := c.RuView.UDPPort
	if c.RuView.Mode == "docker" {
		udpPort = 5005
	}
	args := []string{"--source", c.RuView.Source, "--http-port", strconv.Itoa(port), "--ws-port", strconv.Itoa(port + 1), "--udp-port", strconv.Itoa(udpPort), "--bind-addr", bind, "--data-dir", dir, "--no-edge-registry"}
	return args
}

func runtimeEnv(c *config.Config, token string, bind string) []string {
	env := []string{"RUVIEW_API_TOKEN=" + token, "RUVIEW_BIND_ADDR=" + bind, "SENSING_ALLOWED_HOSTS=localhost,127.0.0.1,aurago-ruview", "RUST_LOG=off"}
	if c.RuView.UDPEnabled {
		if c.RuView.Mode == "native" {
			bind = c.RuView.UDPBind
		}
		env = append(env, "RUVIEW_UDP_ALLOW="+strings.Join(c.RuView.UDPAllow, ","), "RUVIEW_UDP_BIND="+bind)
	} else {
		env = append(env, "RUVIEW_UDP_BIND=127.0.0.1")
	}
	return env
}

// verifyNative hashes the installed package. It is slow for large files, so
// the caller holds no lock.
func (m *Manager) verifyNative(c *config.Config) (root string, a NativeArtifact, err error) {
	a, err = nativeArtifact()
	if err != nil {
		return "", a, err // already typed platform_unsupported
	}
	root, err = filepath.Abs(filepath.Join(c.Directories.DataDir, "ruview", "runtime", Revision, a.Platform))
	if err != nil {
		return "", a, fail(CodeStartFailed, err.Error(), err)
	}
	for _, file := range a.Files {
		if err := verifyFile(root, file); err != nil {
			return "", a, fail(CodeNotInstalled, "RuView native package is not installed or failed verification", err)
		}
	}
	return root, a, nil
}

// launchNative starts the verified package. The caller holds opMu.
func (m *Manager) launchNative(c *config.Config, root, dir, token string) error {
	name := "sensing-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	// Accepted risk: verifyNative hashed the package without a lock, a moment
	// before this exec, so a binary replaced in between is not detected. Doing so
	// needs write access to AuraGo's data directory, which is as trusted as the
	// vault and the managed token that sit beside it.
	cmd := exec.Command(filepath.Join(root, name), runtimeArgs(c, dir, "127.0.0.1", c.RuView.HTTPPort)...)
	cmd.Dir = dir
	// Keep provider/Vault environment out of the sensing process. mac_wifi is
	// found only in this verified package; OS utilities retain their system path.
	path := root + string(os.PathListSeparator) + "/usr/bin:/bin:/usr/sbin:/sbin"
	if runtime.GOOS == "windows" {
		path = root + ";" + filepath.Join(os.Getenv("SystemRoot"), "System32")
	}
	cmd.Env = append([]string{"PATH=" + path, "SystemRoot=" + os.Getenv("SystemRoot"), "HOME=" + dir, "TMPDIR=" + os.TempDir(), "TEMP=" + os.TempDir()}, runtimeEnv(c, token, "127.0.0.1")...)
	p, err := launchNative(cmd)
	if err != nil {
		return fail(CodeStartFailed, err.Error(), err)
	}
	m.mu.Lock()
	m.process = p
	m.mu.Unlock()
	return nil
}

func owner(c *config.Config) string {
	root, _ := filepath.Abs(c.Directories.DataDir)
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:16])
}

// containerState is what the Engine reports about the managed container.
// Known is false when the Engine omitted the State object.
type containerState struct {
	ID       string
	Running  bool
	Status   string
	ExitCode int
	Known    bool
}

// inspectOwned looks up the managed container. found is false when none
// exists. A container with this name that is not ours is a typed
// container_conflict error: it must be neither stopped nor removed.
func inspectOwned(ctx context.Context, d *dockerutil.Client, c *config.Config) (st containerState, found bool, err error) {
	var info struct {
		ID     string `json:"Id"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		State *struct {
			Running  bool   `json:"Running"`
			Status   string `json:"Status"`
			ExitCode int    `json:"ExitCode"`
		} `json:"State"`
	}
	code, err := d.DoJSON(ctx, "GET", "containers/"+containerName+"/json", nil, &info)
	if code == http.StatusNotFound {
		return containerState{}, false, nil
	}
	if err != nil {
		return containerState{}, false, dockerFail(ctx, fmt.Errorf("inspect RuView container: %w", err), code)
	}
	if info.ID == "" || info.Config.Labels["aurago.managed"] != "ruview" || info.Config.Labels["aurago.ruview.owner"] != owner(c) {
		return containerState{}, false, fail(CodeContainerConflict, "a container named aurago-ruview belongs to another workload", nil)
	}
	st = containerState{ID: info.ID}
	if info.State != nil {
		st.Known, st.Running, st.Status, st.ExitCode = true, info.State.Running, info.State.Status, info.State.ExitCode
	}
	return st, true, nil
}

func ownedContainer(ctx context.Context, d *dockerutil.Client, c *config.Config) (string, error) {
	st, found, err := inspectOwned(ctx, d, c)
	if err != nil || !found {
		return "", err
	}
	return st.ID, nil
}

// dockerFail types a failed Engine call: a revoked context is cancelled or
// timeout, no HTTP answer (code 0) is docker_unavailable, anything else is a
// failed start.
func dockerFail(ctx context.Context, err error, code int) *Error {
	if ctx != nil && ctx.Err() != nil {
		return ctxError(ctx.Err())
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ctxError(err)
	}
	if code == 0 {
		return fail(CodeDockerUnavailable, err.Error(), err)
	}
	return fail(CodeStartFailed, err.Error(), err)
}

// pullFail types a failed image pull. Registry refusals are image_denied.
func pullFail(ctx context.Context, status int, message string, cause error) *Error {
	if ctx.Err() != nil {
		return ctxError(ctx.Err())
	}
	code := CodeImagePullFailed
	lower := strings.ToLower(message)
	if status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound ||
		strings.Contains(lower, "denied") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "manifest unknown") {
		code = CodeImageDenied
	}
	return fail(code, message, cause)
}

// ensureImage pulls the pinned image when it is missing. It is the slow part
// of a Docker start and must run without opMu.
func (m *Manager) ensureImage(ctx context.Context, c *config.Config) error {
	if !c.Docker.Enabled || c.Docker.ReadOnly {
		return fail(CodeDockerReadOnly, "managed RuView requires writable Docker integration", nil)
	}
	image := Artifacts().DockerImage
	if !strings.Contains(image, "@sha256:") {
		return fail(CodeArtifactMissing, "no verified RuView Docker digest in this release", nil)
	}
	d := dockerutil.NewClient(c.Docker.Host, 0)
	defer d.CloseIdleConnections()
	code, err := d.DoJSON(ctx, "GET", "images/"+url.PathEscape(image)+"/json", nil, nil)
	if code == http.StatusOK {
		return nil
	}
	if code == 0 {
		return dockerFail(ctx, err, 0)
	}
	m.setPhase(PhasePullingImage)
	req, err := http.NewRequestWithContext(ctx, "POST", dockerutil.Endpoint("images/create?fromImage="+url.QueryEscape(image)), nil)
	if err != nil {
		return fail(CodeStartFailed, err.Error(), err)
	}
	resp, err := d.HTTPClientWithTimeout(10 * time.Minute).Do(req)
	if err != nil {
		return dockerFail(ctx, err, 0)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		detail := dockerutil.SanitizeOneLine(security.Scrub(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body))), 512)
		return pullFail(ctx, resp.StatusCode, fmt.Sprintf("RuView image pull returned HTTP %d: %s", resp.StatusCode, detail), fmt.Errorf("HTTP %d: %s", resp.StatusCode, detail))
	}
	if err = dockerutil.DrainJSONMessages(resp.Body); err != nil {
		return pullFail(ctx, 0, "pull RuView image: "+err.Error(), err)
	}
	return nil
}

// createContainer replaces any stale managed container and starts a new one.
// The caller holds opMu; every call is short or aborted by ctx.
func (m *Manager) createContainer(ctx context.Context, c *config.Config, dir, token string) error {
	image := Artifacts().DockerImage
	d := dockerutil.NewClient(c.Docker.Host, 0)
	defer d.CloseIdleConnections()
	id, err := ownedContainer(ctx, d, c)
	if err != nil {
		return err
	}
	if id != "" {
		if code, stopErr := d.DoJSON(ctx, "POST", "containers/"+id+"/stop?t=10", nil, nil); stopErr != nil && code != 304 && code != 404 {
			return dockerFail(ctx, stopErr, code)
		}
		if code, delErr := d.DoJSON(ctx, "DELETE", "containers/"+id, nil, nil); delErr != nil && code != 404 {
			return dockerFail(ctx, delErr, code)
		}
	}
	host := map[string]any{"ReadonlyRootfs": true, "CapDrop": []string{"ALL"}, "SecurityOpt": []string{"no-new-privileges:true"}, "Memory": int64(2 << 30), "NanoCpus": int64(2e9), "PidsLimit": 128, "Init": true, "Tmpfs": map[string]string{"/tmp": "rw,noexec,nosuid,size=128m"}, "RestartPolicy": map[string]string{"Name": "no"}, "LogConfig": map[string]any{"Type": "none"}}
	ports := map[string][]map[string]string{}
	if c.Runtime.IsDocker {
		network, mount, err := placement(ctx, d, dir)
		if err != nil {
			if ctx.Err() != nil {
				return ctxError(ctx.Err())
			}
			return fail(CodeStartFailed, err.Error(), err)
		}
		host["NetworkMode"] = network
		host["Mounts"] = []any{mount}
	} else {
		host["Binds"] = []string{dockerutil.FormatBindMount(dir, "/data")}
		ports["3000/tcp"] = []map[string]string{{"HostIp": "127.0.0.1", "HostPort": strconv.Itoa(c.RuView.HTTPPort)}}
	}
	if c.RuView.UDPEnabled {
		ports["5005/udp"] = []map[string]string{{"HostIp": c.RuView.UDPBind, "HostPort": strconv.Itoa(c.RuView.UDPPort)}}
	}
	host["PortBindings"] = ports
	payload := map[string]any{"Image": image, "Entrypoint": []string{"/app/sensing-server"}, "Cmd": runtimeArgs(c, "/data", "0.0.0.0", 3000), "Env": runtimeEnv(c, token, "0.0.0.0"), "Labels": map[string]string{"aurago.managed": "ruview", "aurago.ruview.owner": owner(c), "org.opencontainers.image.revision": Revision}, "ExposedPorts": map[string]any{"3000/tcp": struct{}{}, "5005/udp": struct{}{}}, "HostConfig": host}
	var created struct {
		ID string `json:"Id"`
	}
	if code, err := d.DoJSON(ctx, "POST", "containers/create?name="+containerName, payload, &created); err != nil {
		return dockerFail(ctx, err, code)
	}
	if created.ID == "" {
		return fail(CodeStartFailed, "RuView container creation returned no identity", nil)
	}
	code, err := d.DoJSON(ctx, "POST", "containers/"+created.ID+"/start", nil, nil)
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// force: the start may have half-succeeded, and a running container nobody tracks must not stay.
		_, _ = d.DoJSON(cleanup, "DELETE", "containers/"+created.ID+"?force=1", nil, nil)
		return dockerFail(ctx, err, code)
	}
	return nil
}

func placement(ctx context.Context, d *dockerutil.Client, dir string) (string, map[string]any, error) {
	self := dockerutil.OwnContainerID(os.ReadFile)
	if self == "" {
		hostname, _ := os.Hostname()
		self = dockerutil.DefaultContainerHostname(hostname)
	}
	if self == "" {
		return "", nil, fmt.Errorf("cannot identify the AuraGo container")
	}
	var i struct {
		NetworkSettings struct{ Networks map[string]any }
		Mounts          []struct{ Type, Source, Destination, Name string }
	}
	if _, err := d.DoJSON(ctx, "GET", "containers/"+url.PathEscape(self)+"/json", nil, &i); err != nil {
		return "", nil, err
	}
	var networks []string
	for n := range i.NetworkSettings.Networks {
		if !strings.Contains(n, "docker-control") && !strings.Contains(n, "docker_control") && n != "bridge" && n != "host" {
			networks = append(networks, n)
		}
	}
	sort.Strings(networks)
	if len(networks) == 0 {
		return "", nil, fmt.Errorf("RuView needs an AuraGo application network")
	}
	for _, mount := range i.Mounts {
		rel, e := filepath.Rel(mount.Destination, dir)
		if e != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if mount.Type == "bind" {
			return networks[0], map[string]any{"Type": "bind", "Source": filepath.Join(mount.Source, rel), "Target": "/data"}, nil
		}
		if mount.Type == "volume" && mount.Name != "" {
			return networks[0], map[string]any{"Type": "volume", "Source": mount.Name, "Target": "/data", "VolumeOptions": map[string]any{"Subpath": filepath.ToSlash(rel)}}, nil
		}
	}
	return "", nil, fmt.Errorf("RuView data directory is outside AuraGo's Docker data mount")
}

// Stop cancels whatever is starting or running and stops the runtime.
func (m *Manager) Stop(ctx context.Context) error {
	return m.stopWithReason(ctx, StopUser)
}

// stopWithReason cancels the generation first, which aborts a pull, a
// download or a probe in flight, and only then takes opMu. Because a pull never
// holds opMu, Stop works while a start is still downloading (M29). The start
// then ends with CodeCancelled.
func (m *Manager) stopWithReason(ctx context.Context, reason string) error {
	m.mu.Lock()
	m.cancel()
	m.mu.Unlock()
	// The hint is flushed by the deferred call below, after opMu is released.
	defer m.flushStopNote()
	// Gives up with the caller's context instead of hanging.
	if err := m.opMu.acquire(ctx, 0); err != nil {
		return err
	}
	defer m.opMu.release()
	return m.stopRuntime(ctx, reason)
}

// stopRuntime requires opMu; the caller's context is independent of generation.
// reason is recorded as Status.StopReason on a confirmed stop; "" keeps the
// reason Configure or Stop already recorded (start cleanup).
func (m *Manager) stopRuntime(ctx context.Context, reason string) error {
	m.mu.Lock()
	m.cancel()
	m.status = Status{DataState: "stopping"}
	m.observed = time.Time{}
	c := m.runningConfig
	p := m.process
	m.mu.Unlock()
	stopErr := func() error {
		if c == nil {
			c = m.config()
			if p == nil && (!c.RuView.Enabled || c.RuView.Mode != "docker" || !c.Docker.Enabled) {
				return nil
			}
		}
		if p != nil {
			select {
			case <-p.done:
			default:
				if err := p.kill(); err != nil {
					select {
					case <-p.done:
					default:
						return fmt.Errorf("stop RuView process: %w", err)
					}
				}
				// The wait is bounded even for a caller without a deadline: a
				// process that ignores the kill must end in stop_failed, not hang
				// every later Start and Stop behind opMu.
				wait := time.NewTimer(m.stopWait)
				defer wait.Stop()
				select {
				case <-p.done:
				case <-ctx.Done():
					return fmt.Errorf("wait for RuView process: %w", ctx.Err())
				case <-wait.C:
					return fmt.Errorf("RuView process did not end within %s after it was killed", m.stopWait)
				}
			}
		}
		if c.RuView.Mode == "docker" {
			return m.stopContainer(ctx, c)
		}
		return nil
	}()
	mode := c.RuView.Mode
	m.mu.Lock()
	m.cancel()
	m.generation, m.cancel = context.WithCancel(context.Background())
	m.observed = time.Time{}
	m.clearPhaseLocked()
	m.startFailed = false // an explicit stop replaces an earlier start failure
	if stopErr != nil {
		m.status = Status{DataState: "disconnected", Error: CodeStopFailed, ErrorCode: CodeStopFailed, ErrorDetail: scrubDetail(stopErr.Error(), maxDetailBytes)}
	} else {
		m.process = nil
		m.managed = false
		m.runningConfig = nil
		m.status = Status{DataState: "stopped"}
		if reason != "" {
			m.stopReason, m.stoppedAt = reason, time.Now().UTC()
		}
	}
	// The hint is emitted by flushStopNote once the caller released opMu.
	m.stopNote = stopNote{set: true, confirmed: stopErr == nil, mode: mode}
	m.mu.Unlock()
	if stopErr != nil {
		m.warn("RuView stop was not confirmed", "error", security.Scrub(stopErr.Error()))
		return fail(CodeStopFailed, stopErr.Error(), stopErr)
	}
	return nil
}

// stopNote is the stop outcome waiting to become a hint (guarded by mu).
type stopNote struct {
	set, confirmed bool
	mode           string
}

// flushStopNote emits the hint for the last stopRuntime. Callers run it after
// releasing opMu, so the hint callback (a database write) never delays a Start
// or Stop. Concurrent stops may share one flush: emitHint is edge-triggered on
// state, so the last outcome wins.
func (m *Manager) flushStopNote() {
	m.mu.Lock()
	note := m.stopNote
	m.stopNote = stopNote{}
	m.mu.Unlock()
	if note.set {
		m.noteStop(note.confirmed, note.mode)
	}
}

// stopContainer stops the managed container and removes it (M23), so a stopped
// service leaves nothing behind. A container that is not ours is left alone:
// there is nothing of ours to stop, and Start reports container_conflict.
func (m *Manager) stopContainer(ctx context.Context, c *config.Config) error {
	d := dockerutil.NewClient(c.Docker.Host, 0)
	defer d.CloseIdleConnections()
	id, err := ownedContainer(ctx, d, c)
	if err != nil {
		if AsError(err).Code == CodeContainerConflict {
			m.warn("RuView container name belongs to another workload; nothing of ours to stop")
			return nil
		}
		return err
	}
	if id == "" {
		return nil
	}
	if code, err := d.DoJSON(ctx, "POST", "containers/"+id+"/stop?t=10", nil, nil); err != nil && code != 304 && code != 404 {
		return err
	}
	// The stop is confirmed. A failed removal is not fatal: the next start
	// removes a stale container before it creates a new one.
	if code, err := d.DoJSON(ctx, "DELETE", "containers/"+id, nil, nil); err != nil && code != 404 {
		m.warn("RuView container could not be removed after stopping", "error", scrubDetail(err.Error(), maxDetailBytes))
	}
	return nil
}
