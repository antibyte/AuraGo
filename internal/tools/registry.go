package tools

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

type processSnapshot struct {
	pid       int
	startedAt time.Time
	info      *ProcessInfo
}

var signalProcess = func(process *os.Process, sig os.Signal) error {
	return process.Signal(sig)
}

var killProcess = func(process *os.Process) error {
	return process.Kill()
}

// maxOutputSize is the maximum bytes kept in a background process output buffer (1 MB).
const maxOutputSize = 1 << 20

// initialOutputSize keeps idle and low-output processes cheap while leaving
// enough room for typical command startup output.
const initialOutputSize = 4 << 10

// ProcessState represents the lifecycle state of a background process.
type ProcessState int

const (
	ProcessStateStarting   ProcessState = iota // Process is being set up
	ProcessStateRunning                        // Process is running
	ProcessStateTimedOut                       // Process was killed due to timeout
	ProcessStateExited                         // Process exited normally
	ProcessStateCrashed                        // Process exited with an error
	ProcessStateTerminated                     // Process was explicitly terminated
)

// ProcessInfo holds metadata about a running background process.
type ProcessInfo struct {
	PID          int
	Process      *os.Process
	StartedAt    time.Time
	Alive        bool
	State        ProcessState // Current lifecycle state
	ExitCode     int          // Exit code (if process has exited)
	TerminatedAt time.Time    // When the process ended
	TimedOut     bool         // Whether process was killed due to timeout
	ErrorReason  string       // Error description (if process crashed or was killed)
	mu           sync.Mutex   // Protects output ring and state fields
	outputRing   []byte       // fixed-size ring buffer for stdout/stderr
	ringStart    int          // logical start index inside outputRing
	ringLen      int          // number of bytes currently stored
}

// String returns a human-readable representation of the process state.
func (s ProcessState) String() string {
	switch s {
	case ProcessStateStarting:
		return "starting"
	case ProcessStateRunning:
		return "running"
	case ProcessStateTimedOut:
		return "timed_out"
	case ProcessStateExited:
		return "exited"
	case ProcessStateCrashed:
		return "crashed"
	case ProcessStateTerminated:
		return "terminated"
	default:
		return "unknown"
	}
}

// Write implements io.Writer so ProcessInfo can be used as cmd.Stdout/Stderr.
// It stores output in a fixed-size ring buffer; once the buffer is full the
// oldest bytes are overwritten by the newest data. This avoids repeated large
// allocations and keeps memory bounded.
func (p *ProcessInfo) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	originalLen := len(data)
	if originalLen == 0 {
		return 0, nil
	}
	if len(data) > maxOutputSize {
		// A single write larger than the buffer: keep only the tail.
		data = data[len(data)-maxOutputSize:]
	}

	required := p.ringLen + len(data)
	if required > maxOutputSize {
		required = maxOutputSize
	}
	if len(p.outputRing) < required {
		capacity := initialOutputSize
		for capacity < required && capacity < maxOutputSize {
			capacity <<= 1
		}
		if capacity > maxOutputSize {
			capacity = maxOutputSize
		}
		grown := make([]byte, capacity)
		if p.ringLen > 0 {
			oldCapacity := len(p.outputRing)
			if p.ringStart+p.ringLen <= oldCapacity {
				copy(grown, p.outputRing[p.ringStart:p.ringStart+p.ringLen])
			} else {
				tail := oldCapacity - p.ringStart
				copy(grown, p.outputRing[p.ringStart:])
				copy(grown[tail:], p.outputRing[:p.ringLen-tail])
			}
		}
		p.outputRing = grown
		p.ringStart = 0
	}
	capacity := len(p.outputRing)

	// If necessary, discard oldest data to make room.
	if free := capacity - p.ringLen; len(data) > free {
		discard := len(data) - free
		p.ringStart = (p.ringStart + discard) % capacity
		p.ringLen -= discard
	}

	for i, b := range data {
		pos := (p.ringStart + p.ringLen + i) % capacity
		p.outputRing[pos] = b
	}
	p.ringLen += len(data)
	return originalLen, nil
}

// ReadOutput returns the current contents of the output buffer.
func (p *ProcessInfo) ReadOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ringLen == 0 {
		return ""
	}
	capacity := len(p.outputRing)
	out := make([]byte, p.ringLen)
	if p.ringStart+p.ringLen <= capacity {
		copy(out, p.outputRing[p.ringStart:p.ringStart+p.ringLen])
	} else {
		tail := capacity - p.ringStart
		copy(out, p.outputRing[p.ringStart:])
		copy(out[tail:], p.outputRing[:p.ringLen-tail])
	}
	return string(out)
}

// OutputLen returns the number of bytes currently stored in the output ring.
func (p *ProcessInfo) OutputLen() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ringLen
}

// WriteSystemMessage writes a system/supervisor message to the output buffer.
// Unlike direct buffer access, this method applies the same truncation logic
// as Write() to ensure buffer size limits are respected.
// The message is prefixed with a newline for separation from process output.
func (p *ProcessInfo) WriteSystemMessage(msg string) error {
	data := []byte("\n" + msg)
	_, err := p.Write(data)
	return err
}

// WriteSystemMessageBytes writes raw system/supervisor message bytes to the output buffer.
// Unlike direct buffer access, this method applies the same truncation logic
// as Write() to ensure buffer size limits are respected.
func (p *ProcessInfo) WriteSystemMessageBytes(data []byte) error {
	_, err := p.Write(data)
	return err
}

// IsAlive returns whether the process is still marked as running.
func (p *ProcessInfo) IsAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Alive
}

// GetState returns the current process state.
func (p *ProcessInfo) GetState() ProcessState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.State
}

// GetExitCode returns the process exit code if available.
func (p *ProcessInfo) GetExitCode() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ExitCode
}

// GetTerminatedAt returns when the process terminated.
func (p *ProcessInfo) GetTerminatedAt() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.TerminatedAt
}

// GetErrorReason returns the error reason if the process failed.
func (p *ProcessInfo) GetErrorReason() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ErrorReason
}

// ProcessRegistry is a thread-safe registry for background processes.
type ProcessRegistry struct {
	mu        sync.RWMutex
	processes map[int]*ProcessInfo
	logger    *slog.Logger
}

// NewProcessRegistry creates a new empty process registry.
func NewProcessRegistry(logger *slog.Logger) *ProcessRegistry {
	return &ProcessRegistry{
		processes: make(map[int]*ProcessInfo),
		logger:    logger,
	}
}

// Register adds a process to the registry.
func (r *ProcessRegistry) Register(info *ProcessInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.processes[info.PID] = info
	r.logger.Info("Registered background process", "pid", info.PID)
}

// Get retrieves a process by PID.
func (r *ProcessRegistry) Get(pid int) (*ProcessInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.processes[pid]
	return info, ok
}

// Remove removes a process from the registry.
func (r *ProcessRegistry) Remove(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.processes, pid)
	r.logger.Info("Removed process from registry", "pid", pid)
}

// Terminate stops a specific process by PID and removes it from the registry.
// It is safe to call even if the process has already exited or is being cleaned up
// by superviseBackgroundProcess.
func (r *ProcessRegistry) Terminate(pid int) error {
	// Acquire r.mu only to look up the process info pointer; release before
	// sending the signal so the lock is not held during a potentially-blocking
	// OS call (avoids lock-contention with KillAll and superviseBackgroundProcess).
	r.mu.Lock()
	info, ok := r.processes[pid]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("process %d not found", pid)
	}

	// Send signal under info.mu only (fine-grained lock).
	info.mu.Lock()
	wasAlive := info.Alive
	var terminateErr error
	if wasAlive && info.Process != nil {
		if err := signalProcess(info.Process, os.Interrupt); err != nil {
			if killErr := killProcess(info.Process); killErr != nil {
				terminateErr = fmt.Errorf("interrupt process %d: %w; kill fallback failed: %v", pid, err, killErr)
				r.logger.Warn("Failed to terminate background process", "pid", pid, "signal_error", err, "kill_error", killErr)
			} else {
				r.logger.Warn("Interrupt failed; killed background process", "pid", pid, "signal_error", err)
			}
		}
	}
	// Only update state if process was still alive (not already exited by superviseBackgroundProcess)
	if wasAlive {
		info.Alive = false
		info.State = ProcessStateTerminated
		info.TerminatedAt = time.Now()
		info.ErrorReason = "explicitly terminated"
	}
	info.mu.Unlock()

	// Remove from registry under r.mu after signal is sent.
	// Check if process is still in registry before deleting (superviseBackgroundProcess
	// may have already removed it).
	r.mu.Lock()
	_, stillRegistered := r.processes[pid]
	if stillRegistered {
		delete(r.processes, pid)
		r.logger.Info("Terminated and removed process", "pid", pid)
	} else {
		r.logger.Info("Process already removed by supervisor", "pid", pid)
	}
	r.mu.Unlock()
	return terminateErr
}

// List returns a summary of all registered processes.
func (r *ProcessRegistry) List() []map[string]interface{} {
	snapshots := r.snapshotProcesses()
	var result []map[string]interface{}
	for _, snapshot := range snapshots {
		alive := snapshot.info.IsAlive()
		result = append(result, map[string]interface{}{
			"pid":     snapshot.pid,
			"alive":   alive,
			"uptime":  fmt.Sprintf("%.0fs", time.Since(snapshot.startedAt).Seconds()),
			"started": snapshot.startedAt.Format(time.RFC3339),
		})
	}
	if result == nil {
		result = []map[string]interface{}{}
	}
	return result
}

func (r *ProcessRegistry) snapshotProcesses() []processSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshots := make([]processSnapshot, 0, len(r.processes))
	for pid, info := range r.processes {
		snapshots = append(snapshots, processSnapshot{pid: pid, startedAt: info.StartedAt, info: info})
	}
	return snapshots
}

// KillAll terminates all registered background processes.
func (r *ProcessRegistry) KillAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for pid, info := range r.processes {
		info.mu.Lock()
		alive := info.Alive
		if alive && info.Process != nil {
			r.logger.Warn("Killing orphaned background process", "pid", pid)
			if err := killProcess(info.Process); err != nil {
				r.logger.Warn("Failed to kill orphaned background process", "pid", pid, "error", err)
			}
			info.Alive = false
			info.State = ProcessStateTerminated
			info.TerminatedAt = time.Now()
			info.ErrorReason = "orphaned process killed"
		}
		info.mu.Unlock()
	}
	r.processes = make(map[int]*ProcessInfo)
	r.logger.Info("All background processes cleaned up")
}
