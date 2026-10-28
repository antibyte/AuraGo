package ruview

import (
	"context"
	"fmt"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

// Status.Runtime: where the managed (or external) service is in its life.
const (
	RuntimeExternal        = "external"
	RuntimeNotStarted      = "not_started"
	RuntimeStarting        = "starting"
	RuntimeRunning         = "running"
	RuntimeStopping        = "stopping"
	RuntimeStopped         = "stopped"
	RuntimeExited          = "exited"
	RuntimeStopUnconfirmed = "stop_unconfirmed"
)

// Status.Phase: the step of a long start or install.
const (
	PhasePreparing    = "preparing"
	PhasePullingImage = "pulling_image"
	PhaseCreating     = "creating"
	PhaseWaitingReady = "waiting_ready"
	PhaseDownloading  = "downloading"
	PhaseVerifying    = "verifying"
)

// Status.StopReason: why the managed runtime is not running.
const (
	StopUser           = "user"
	StopConfigChanged  = "config_changed"
	StopProcessExit    = "process_exit"
	StopAuraGoShutdown = "aurago_shutdown"
)

const (
	// Values of Manager.op, the lifecycle slot (see the lock protocol below).
	opStarting   = "starting"
	opInstalling = "installing"

	// waitingReadyTimeout ends PhaseWaitingReady when no probe reported Ready.
	waitingReadyTimeout = 30 * time.Second
)

func Runtimes() []string {
	return []string{RuntimeExternal, RuntimeNotStarted, RuntimeStarting, RuntimeRunning, RuntimeStopping, RuntimeStopped, RuntimeExited, RuntimeStopUnconfirmed}
}

func Phases() []string {
	return []string{PhasePreparing, PhasePullingImage, PhaseCreating, PhaseWaitingReady, PhaseDownloading, PhaseVerifying}
}

func StopReasons() []string {
	return []string{StopUser, StopConfigChanged, StopProcessExit, StopAuraGoShutdown}
}

// Lock protocol (M29). Order is always opMu -> mu, never the reverse; hintMu is
// taken with neither of them held.
//
//   - mu guards every Manager field. It is never held across I/O (the config
//     snapshot is a lock-free atomic read, so reading it under mu is fine).
//   - op (guarded by mu) is the single slot for long lifecycle work: a start or
//     an install. It is claimed only while opMu is held, in the same mu section
//     that validates the generation, and nobody waits for it: a second Start or
//     Install returns CodeBusy.
//   - opMu (an opLock) serializes short state changes: reaping, container
//     inspect/remove/create/start, process launch and stopRuntime. It is never
//     held across an image pull, a native download or file hashing, so Stop can
//     always take it. Stop first cancels the generation (which aborts the pull,
//     download or probe in flight) and only then takes opMu. Every wait for
//     opMu ends with the caller's context, and Start/Install/watch also give up
//     after lockWait (CodeBusy), so a stuck holder never hangs a request.
//     The longest holders are bounded: the create phase by createTimeout, a
//     stop by its context and stopWait for the process to end.
//   - emitHint, noteStop and noteStorage run with neither mu nor opMu held:
//     stopRuntime only records the outcome (stopNote) and its caller emits the
//     hint through flushStopNote after releasing opMu. hintMu serializes a hint
//     state change together with its callback, so deliveries keep the order of
//     the state changes. The callback must not call the manager.
//   - One start owns the slot from its claim until the deferred epilogue in
//     start clears it; every state change in between is made under mu. A Stop
//     during a start cancels the generation, so the start ends with
//     CodeCancelled and does not overwrite the status Stop wrote.
//   - runID (guarded by mu) names the start whose commit made the runtime
//     managed. Work that inspected the runtime without a lock (watch) only acts
//     on a result when runID is unchanged, so a Stop followed by a Start with an
//     identical configuration is not mistaken for the old runtime.

const (
	// defaultLockWait (Manager.lockWait) is how long Start, Install and watch wait for opMu before they
	// answer CodeBusy. Stop waits for its context only.
	defaultLockWait = 10 * time.Second
	// createTimeout bounds the part of a start that holds opMu (inspect, stop,
	// remove, create, start of the container).
	createTimeout = 60 * time.Second
	// defaultStopWait (Manager.stopWait) bounds the wait for a killed native process to end.
	defaultStopWait = 15 * time.Second
	// watchStopTimeout bounds the stop watch performs after a configuration change.
	watchStopTimeout = 30 * time.Second
)

// opLock is a mutex whose waiters can give up. It is a one-slot channel; the
// zero value (nil) is not usable, New creates it.
type opLock chan struct{}

// acquire takes the lock, or gives up when ctx ends (cancelled/timeout) or,
// when wait > 0, after wait (busy).
func (l opLock) acquire(ctx context.Context, wait time.Duration) error {
	select {
	case l <- struct{}{}:
		return nil
	default:
	}
	var timeout <-chan time.Time
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		timeout = timer.C
	}
	select {
	case l <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctxError(ctx.Err())
	case <-timeout:
		return fail(CodeBusy, "RuView is busy with another start, stop or installation", nil)
	}
}

func (l opLock) release() {
	select {
	case <-l:
	default:
		panic("ruview: opMu released while it was not held")
	}
}

// runtimeLocked derives Status.Runtime. Called with mu held; the first match wins.
func (m *Manager) runtimeLocked(mode string, running bool) string {
	switch {
	case mode == "external":
		return RuntimeExternal
	case m.status.DataState == "stopping":
		return RuntimeStopping
	case m.status.Error == CodeStopFailed:
		return RuntimeStopUnconfirmed
	case m.op == opStarting || m.phase == PhaseWaitingReady && running:
		return RuntimeStarting
	case running:
		return RuntimeRunning
	case m.status.Error == CodeProcessExited:
		return RuntimeExited
	case m.stopReason != "":
		return RuntimeStopped
	default:
		return RuntimeNotStarted
	}
}

// setPhaseLocked enters a phase. Called with mu held.
func (m *Manager) setPhaseLocked(phase string) {
	m.phase, m.phaseSince = phase, time.Now().UTC()
}

// clearPhaseLocked leaves any phase. Called with mu held.
func (m *Manager) clearPhaseLocked() {
	m.phase, m.phaseSince = "", time.Time{}
}

// setPhase enters a phase. Called without mu.
func (m *Manager) setPhase(phase string) {
	m.mu.Lock()
	m.setPhaseLocked(phase)
	m.mu.Unlock()
}

// markExitedLocked records that the managed runtime ended without being asked
// to (M24): ownership is dropped, the generation is revoked so streams and
// probes of the dead service end, and the lifecycle error stays until an
// explicit Start or Stop. Called with mu held (and opMu by lifecycle callers).
func (m *Manager) markExitedLocked(detail string) {
	m.process = nil
	m.managed = false
	m.runningConfig = nil
	m.cancel()
	m.generation, m.cancel = context.WithCancel(context.Background())
	m.status = Status{DataState: "disconnected", Error: CodeProcessExited, ErrorCode: CodeProcessExited, ErrorDetail: detail}
	m.observed = time.Time{}
	m.clearPhaseLocked()
	m.stopReason, m.stoppedAt = StopProcessExit, time.Now().UTC()
	m.startFailed = false
}

// exitDetail describes how a native process ended without leaking paths.
func exitDetail(err error) string {
	if err == nil {
		return "process ended"
	}
	if text := scrubDetail(err.Error(), maxDetailBytes); text != "" {
		return text
	}
	return "process ended"
}

// inspectExit reports whether the managed Docker container ended on its own
// (M24). Native exits are noticed through the process itself. The Engine is
// asked at most every dockerEvery; a transport error or an unknown state is
// not an exit. Called with neither lock held.
func (m *Manager) inspectExit(ctx context.Context, running *config.Config) (exited bool, detail string) {
	if running == nil || running.RuView.Mode != "docker" {
		return false, ""
	}
	m.mu.Lock()
	if !m.dockerAt.IsZero() && time.Since(m.dockerAt) < m.dockerEvery {
		m.mu.Unlock()
		return false, ""
	}
	m.dockerAt = time.Now()
	m.mu.Unlock()
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	d := dockerutil.NewClient(running.Docker.Host, 0)
	defer d.CloseIdleConnections()
	state, found, err := inspectOwned(bounded, d, running)
	switch {
	case err != nil:
		if AsError(err).Code == CodeContainerConflict {
			return true, "container was replaced by another workload"
		}
		return false, ""
	case !found:
		return true, "container was removed"
	case state.Known && !state.Running:
		return true, fmt.Sprintf("container exited with code %d", state.ExitCode)
	}
	return false, ""
}
