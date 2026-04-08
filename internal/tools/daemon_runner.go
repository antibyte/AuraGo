package tools

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// daemonLogMaxBytes is the maximum size of a per-daemon log file before truncation.
const daemonLogMaxBytes = 5 * 1024 * 1024 // 5 MB

// daemonStopGracePeriod is the default time to wait for a daemon to exit after a stop command.
const daemonStopGracePeriod = 10 * time.Second

// DaemonRunner manages the lifecycle of a single daemon skill process.
type DaemonRunner struct {
	mu    sync.Mutex
	logMu sync.Mutex // guards appendDaemonLog / rotateDaemonLog

	skillID   string
	skillName string
	config    DaemonManifest
	manifest  SkillManifest

	// Process state
	stdinPipe io.WriteCloser
	pid       int
	cancel    context.CancelFunc
	status    DaemonStatus
	startedAt time.Time

	// Restart tracking
	restartCount    int
	lastRestartTime time.Time

	// Activity tracking
	lastActivity time.Time
	lastError    string

	// Wake-up stats
	wakeUpCount     int
	suppressedCount int
	lastWakeUp      time.Time

	// Auto-disabled by circuit breaker
	autoDisabled bool

	// Dependencies
	workspaceDir string
	skillsDir    string
	registry     *ProcessRegistry
	logDir       string
	logger       *slog.Logger

	// Wake-up channel: DaemonRunner sends wake-up messages here.
	// DaemonSupervisor reads from this channel.
	wakeCh chan<- daemonWakeEvent
}

// daemonWakeEvent is sent from a DaemonRunner to the supervisor when a wake_agent message arrives.
type daemonWakeEvent struct {
	SkillID   string
	SkillName string
	Message   DaemonMessage
	Timestamp time.Time
}

// DaemonRunnerConfig holds the parameters needed to create a DaemonRunner.
type DaemonRunnerConfig struct {
	SkillID      string
	SkillName    string
	Config       DaemonManifest
	Manifest     SkillManifest
	WorkspaceDir string
	SkillsDir    string
	Registry     *ProcessRegistry
	LogDir       string
	Logger       *slog.Logger
	WakeCh       chan<- daemonWakeEvent
}

// NewDaemonRunner creates a new DaemonRunner for the given skill.
func NewDaemonRunner(cfg DaemonRunnerConfig) *DaemonRunner {
	// Note: ApplyDefaults is already called by DaemonSupervisor.startRunner before
	// constructing the config; do not call it again here to avoid overwriting
	// user-provided defaults with zero-value fields.
	return &DaemonRunner{
		skillID:      cfg.SkillID,
		skillName:    cfg.SkillName,
		config:       cfg.Config,
		manifest:     cfg.Manifest,
		status:       DaemonStopped,
		workspaceDir: cfg.WorkspaceDir,
		skillsDir:    cfg.SkillsDir,
		registry:     cfg.Registry,
		logDir:       cfg.LogDir,
		logger:       cfg.Logger.With("daemon", cfg.SkillName, "skill_id", cfg.SkillID),
		wakeCh:       cfg.WakeCh,
	}
}

// Status returns the current daemon status under lock.
func (r *DaemonRunner) Status() DaemonStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

// State returns a snapshot of the daemon's runtime state.
func (r *DaemonRunner) State() DaemonState {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := DaemonState{
		SkillID:         r.skillID,
		SkillName:       r.skillName,
		Status:          r.status,
		RestartCount:    r.restartCount,
		WakeUpCount:     r.wakeUpCount,
		SuppressedCount: r.suppressedCount,
		LastError:       r.lastError,
		AutoDisabled:    r.autoDisabled,
	}
	if !r.startedAt.IsZero() {
		t := r.startedAt
		s.StartedAt = &t
		s.PID = r.processPID()
	}
	if !r.lastWakeUp.IsZero() {
		t := r.lastWakeUp
		s.LastWakeUp = &t
	}
	if !r.lastActivity.IsZero() {
		t := r.lastActivity
		s.LastActivity = &t
	}
	return s
}

// Start spawns the daemon process and begins monitoring its output.
func (r *DaemonRunner) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == DaemonRunning || r.status == DaemonStarting {
		return fmt.Errorf("daemon %q is already %s", r.skillName, r.status)
	}
	if r.autoDisabled {
		return fmt.Errorf("daemon %q is auto-disabled by circuit breaker; re-enable via UI first", r.skillName)
	}

	return r.startLocked()
}

// startLocked spawns the process. Caller must hold r.mu.
func (r *DaemonRunner) startLocked() error {
	r.status = DaemonStarting
	r.logger.Info("Starting daemon")

	absExecPath := filepath.Join(r.skillsDir, r.manifest.Executable)
	if _, err := os.Stat(absExecPath); os.IsNotExist(err) {
		r.status = DaemonStopped
		return fmt.Errorf("daemon executable not found: %s", absExecPath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	// Apply max runtime limit if configured
	if r.config.MaxRuntimeHours > 0 {
		var runtimeCancel context.CancelFunc
		ctx, runtimeCancel = context.WithTimeout(ctx, time.Duration(r.config.MaxRuntimeHours)*time.Hour)
		origCancel := r.cancel
		r.cancel = func() { runtimeCancel(); origCancel() }
	}

	cmd := buildSkillCommand(ctx, r.workspaceDir, r.manifest, absExecPath)
	cmd.Dir = r.workspaceDir

	// Inject daemon-specific environment variables
	cmd.Env = append(os.Environ(),
		"AURAGO_DAEMON=1",
		fmt.Sprintf("AURAGO_SKILL_ID=%s", r.skillID),
		fmt.Sprintf("AURAGO_SKILL_NAME=%s", r.skillName),
		fmt.Sprintf("AURAGO_WAKE_RATE_LIMIT=%d", r.config.WakeRateLimitSeconds),
	)
	for k, v := range r.config.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		r.status = DaemonStopped
		r.cancel()
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdinPipe.Close()
		r.status = DaemonStopped
		r.cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdinPipe.Close()
		r.status = DaemonStopped
		r.cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		r.status = DaemonStopped
		r.cancel()
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	pid := cmd.Process.Pid
	// Close old stdin pipe before overwriting (handles restart case).
	if r.stdinPipe != nil {
		_ = r.stdinPipe.Close()
	}
	r.stdinPipe = stdinPipe
	r.pid = pid
	r.startedAt = time.Now()
	r.lastActivity = r.startedAt
	r.status = DaemonRunning

	// Register in process registry for KillAll on shutdown
	procInfo := &ProcessInfo{
		PID:       pid,
		Process:   cmd.Process,
		Output:    &bytes.Buffer{},
		StartedAt: r.startedAt,
		Alive:     true,
	}
	if r.registry != nil {
		r.registry.Register(procInfo)
	}

	r.logger.Info("Daemon started", "pid", pid)

	// Start monitor goroutines
	go r.monitorStdout(stdoutPipe, procInfo)
	go r.monitorStderr(stderrPipe, procInfo)
	go r.waitProcess(cmd, procInfo, cancel)
	go r.healthLoop(ctx, cmd.Process)

	return nil
}

// Stop sends a graceful stop command, waits for the grace period, then kills the process.
func (r *DaemonRunner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopLocked("user_requested")
}

// stopLocked sends stop and kills if needed. Caller must hold r.mu.
func (r *DaemonRunner) stopLocked(reason string) error {
	if r.status == DaemonCrashed {
		r.status = DaemonStopped
		r.logger.Info("Stopped daemon during crash recovery, preventing pending restart", "reason", reason)
		return nil
	}
	if r.status != DaemonRunning && r.status != DaemonStarting {
		return nil
	}

	r.logger.Info("Stopping daemon", "reason", reason)

	// Send stop command via stdin
	if r.stdinPipe != nil {
		cmd := NewStopCommand(reason, int(daemonStopGracePeriod.Seconds()))
		if data, err := EncodeDaemonCommand(cmd); err == nil {
			data = append(data, '\n')
			_, _ = r.stdinPipe.Write(data)
		}
	}

	// Cancel the context — this will cause cmd.Wait to return
	if r.cancel != nil {
		r.cancel()
	}

	r.status = DaemonStopped
	r.pid = 0
	return nil
}

// Disable marks the daemon as auto-disabled by the circuit breaker.
func (r *DaemonRunner) Disable(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.autoDisabled = true
	r.lastError = reason
	_ = r.stopLocked("auto_disabled: " + reason)
	r.status = DaemonDisabled
	r.logger.Warn("Daemon auto-disabled", "reason", reason)
}

// Reenable clears the auto-disabled flag, allowing the daemon to be started again.
func (r *DaemonRunner) Reenable() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.autoDisabled = false
	if r.status == DaemonDisabled {
		r.status = DaemonStopped
	}
	r.restartCount = 0
	r.logger.Info("Daemon re-enabled")
}

// IsAutoDisabled returns whether the daemon was auto-disabled by the circuit breaker.
func (r *DaemonRunner) IsAutoDisabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.autoDisabled
}

// IncrementSuppressed increments the suppressed wake-up counter.
func (r *DaemonRunner) IncrementSuppressed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suppressedCount++
}

// IncrementWakeUp records a successful wake-up.
func (r *DaemonRunner) IncrementWakeUp() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wakeUpCount++
	now := time.Now()
	r.lastWakeUp = now
}

// processPID returns the PID if running, 0 otherwise. Caller must hold r.mu.
func (r *DaemonRunner) processPID() int {
	if r.status == DaemonRunning {
		return r.pid
	}
	return 0
}

// monitorStdout reads daemon stdout line by line and dispatches IPC messages.
func (r *DaemonRunner) monitorStdout(pipe io.ReadCloser, procInfo *ProcessInfo) {
	defer pipe.Close()
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024) // up to 256 KB per line

	for scanner.Scan() {
		line := scanner.Text()
		r.mu.Lock()
		r.lastActivity = time.Now()
		r.mu.Unlock()

		// Write to process info buffer (for log retrieval)
		_, _ = procInfo.Write([]byte(line + "\n"))

		// Write to daemon log file
		r.appendDaemonLog(line)

		// Try to parse as IPC message
		msg, err := ParseDaemonMessage(line)
		if err != nil {
			// Not a valid IPC message — treat as plain log output
			r.logger.Debug("Daemon stdout (plain)", "line", truncateString(line, 200))
			continue
		}

		r.handleMessage(msg)
	}

	if err := scanner.Err(); err != nil {
		r.logger.Debug("Daemon stdout scanner error", "error", err)
	}
}

// monitorStderr reads daemon stderr and logs it.
func (r *DaemonRunner) monitorStderr(pipe io.ReadCloser, procInfo *ProcessInfo) {
	defer pipe.Close()
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Text()
		r.mu.Lock()
		r.lastActivity = time.Now()
		r.mu.Unlock()

		_, _ = procInfo.Write([]byte("[stderr] " + line + "\n"))
		r.appendDaemonLog("[stderr] " + line)
		r.logger.Debug("Daemon stderr", "line", truncateString(line, 200))
	}
}

// handleMessage dispatches a parsed IPC message.
func (r *DaemonRunner) handleMessage(msg DaemonMessage) {
	switch msg.Type {
	case DaemonMsgWakeAgent:
		r.logger.Info("Daemon requests wake-up", "message", truncateString(msg.Message, 200), "severity", msg.Severity)
		if r.wakeCh != nil {
			select {
			case r.wakeCh <- daemonWakeEvent{
				SkillID:   r.skillID,
				SkillName: r.skillName,
				Message:   msg,
				Timestamp: time.Now(),
			}:
			default:
				r.logger.Warn("Wake-up channel full, dropping event")
			}
		}

	case DaemonMsgLog:
		level := slog.LevelInfo
		switch strings.ToLower(msg.Level) {
		case "debug":
			level = slog.LevelDebug
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
		r.logger.Log(context.Background(), level, "Daemon log", "msg", truncateString(msg.Message, 500))

	case DaemonMsgHeartbeat:
		r.mu.Lock()
		r.lastActivity = time.Now()
		r.mu.Unlock()

	case DaemonMsgError:
		r.mu.Lock()
		r.lastError = msg.Message
		r.mu.Unlock()
		r.logger.Warn("Daemon reported error", "message", truncateString(msg.Message, 500), "fatal", msg.Fatal)
		if msg.Fatal {
			r.logger.Error("Daemon reported fatal error, stopping", "message", msg.Message)
			r.mu.Lock()
			_ = r.stopLocked("fatal_error")
			r.mu.Unlock()
		}

	case DaemonMsgShutdown:
		r.logger.Info("Daemon requested shutdown", "reason", msg.Reason)
		r.mu.Lock()
		_ = r.stopLocked("skill_requested: " + msg.Reason)
		r.mu.Unlock()

	case DaemonMsgMetric:
		r.logger.Debug("Daemon metric", "data", truncateString(string(msg.Data), 500))
	}
}

// waitProcess waits for the daemon process to exit and handles crash recovery.
func (r *DaemonRunner) waitProcess(cmd interface{ Wait() error }, procInfo *ProcessInfo, cancel context.CancelFunc) {
	err := cmd.Wait()
	cancel() // ensure context is canceled

	procInfo.mu.Lock()
	procInfo.Alive = false
	procInfo.mu.Unlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status == DaemonStopped || r.status == DaemonDisabled {
		// Intentional stop — no restart
		r.logger.Info("Daemon process exited after stop")
		return
	}

	// Unexpected exit
	exitMsg := "daemon process exited unexpectedly"
	if err != nil {
		exitMsg = fmt.Sprintf("daemon process exited with error: %v", err)
	}
	r.lastError = exitMsg
	r.logger.Warn(exitMsg)

	// Attempt restart if configured
	if r.config.RestartOnCrash && r.canRestart() {
		r.restartCount++
		r.lastRestartTime = time.Now()
		r.status = DaemonCrashed
		r.logger.Info("Scheduling daemon restart", "attempt", r.restartCount, "max", r.config.MaxRestartAttempts)

		// Restart after a brief delay (outside lock)
		go func() {
			time.Sleep(2 * time.Second)
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.status == DaemonCrashed && !r.autoDisabled {
				if err := r.startLocked(); err != nil {
					r.logger.Error("Failed to restart daemon", "error", err)
					r.status = DaemonCrashed
				}
			}
		}()
	} else {
		r.status = DaemonCrashed
		if !r.config.RestartOnCrash {
			r.logger.Info("Restart disabled, daemon staying crashed")
		} else {
			r.logger.Warn("Max restart attempts exceeded, daemon staying crashed",
				"restarts", r.restartCount, "max", r.config.MaxRestartAttempts)
		}
	}
}

// canRestart checks if a restart is allowed based on cooldown and attempt limits.
// Caller must hold r.mu.
func (r *DaemonRunner) canRestart() bool {
	if r.restartCount >= r.config.MaxRestartAttempts {
		// Reset counter if outside cooldown window
		cooldown := time.Duration(r.config.RestartCooldownSeconds) * time.Second
		if time.Since(r.lastRestartTime) > cooldown {
			r.restartCount = 0
			return true
		}
		return false
	}
	return true
}

// healthLoop periodically checks if the daemon process is still alive.
func (r *DaemonRunner) healthLoop(ctx context.Context, process *os.Process) {
	interval := time.Duration(r.config.HealthCheckIntervalSeconds) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !r.isProcessAlive(process) {
				r.logger.Warn("Health check: daemon process not alive")
				// waitProcess goroutine will handle the crash
				return
			}
		}
	}
}

// isProcessAlive checks whether the process is still running.
func (r *DaemonRunner) isProcessAlive(process *os.Process) bool {
	if process == nil {
		return false
	}
	if runtime.GOOS == "windows" {
		// On Windows, Signal(0) is not supported. Check via FindProcess.
		p, err := os.FindProcess(process.Pid)
		if err != nil {
			return false
		}
		// FindProcess always succeeds on Windows; rely on waitProcess for actual exit detection
		_ = p
		return true
	}
	// On Unix, Signal(0) checks if process exists without sending a real signal
	err := process.Signal(signalZero)
	return err == nil
}

// appendDaemonLog appends a log line to the daemon's log file with rotation.
func (r *DaemonRunner) appendDaemonLog(line string) {
	if r.logDir == "" {
		return
	}

	r.logMu.Lock()
	defer r.logMu.Unlock()

	logPath := filepath.Join(r.logDir, r.skillID+".log")

	// Check file size for rotation
	if info, err := os.Stat(logPath); err == nil && info.Size() > daemonLogMaxBytes {
		r.rotateDaemonLog(logPath)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format(time.RFC3339), line)
}

// rotateDaemonLog truncates the log file, keeping the last half.
func (r *DaemonRunner) rotateDaemonLog(logPath string) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return
	}
	half := len(data) / 2
	// Find the next newline after the halfway point to avoid splitting a line
	for i := half; i < len(data); i++ {
		if data[i] == '\n' {
			half = i + 1
			break
		}
	}
	// Write to a temp file first, then atomically replace to avoid data loss on crash.
	tmpPath := logPath + ".tmp"
	if err := os.WriteFile(tmpPath, data[half:], 0644); err != nil {
		r.logger.Warn("Failed to write rotated daemon log", "path", tmpPath, "error", err)
		return
	}
	if err := os.Rename(tmpPath, logPath); err != nil {
		r.logger.Warn("Failed to replace daemon log after rotation", "path", logPath, "error", err)
		_ = os.Remove(tmpPath)
	}
}

// truncateString is defined in missions_v2.go — reused here.
