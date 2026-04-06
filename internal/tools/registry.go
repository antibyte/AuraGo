package tools

import (
	"bytes"
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

// ProcessInfo holds metadata about a running background process.
type ProcessInfo struct {
	PID       int
	Process   *os.Process
	Output    *bytes.Buffer
	StartedAt time.Time
	Alive     bool
	mu        sync.Mutex // Protects Output writes
}

// Write implements io.Writer so ProcessInfo can be used as cmd.Stdout/Stderr.
// Drops data silently once the buffer exceeds maxOutputSize to prevent OOM.
func (p *ProcessInfo) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Output.Len()+len(data) > maxOutputSize {
		// Discard oldest half to make room, keeping the tail
		b := p.Output.Bytes()
		half := len(b) / 2
		p.Output.Reset()
		p.Output.Write(b[half:])
	}
	return p.Output.Write(data)
}

// ReadOutput returns the current contents of the output buffer.
func (p *ProcessInfo) ReadOutput() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Output.String()
}

// IsAlive returns whether the process is still marked as running.
func (p *ProcessInfo) IsAlive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Alive
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
func (r *ProcessRegistry) Terminate(pid int) error {
	r.mu.Lock()
	info, ok := r.processes[pid]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("process %d not found", pid)
	}
	// Check and update under info.mu first
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
	info.Alive = false
	info.mu.Unlock()
	// Now remove from registry under r.mu
	delete(r.processes, pid)
	r.mu.Unlock()
	r.logger.Info("Terminated and removed process", "pid", pid)
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
		}
		info.mu.Unlock()
	}
	r.processes = make(map[int]*ProcessInfo)
	r.logger.Info("All background processes cleaned up")
}
