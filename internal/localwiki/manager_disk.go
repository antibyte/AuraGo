package localwiki

import "time"

const (
	// diskProbeInterval is how often the background probe re-measures the
	// free space of the storage directory.
	diskProbeInterval = 10 * time.Second
	// diskProbeTimeout is how long a measurement may take before Status
	// reports the free space as unknown (an unreachable network share).
	diskProbeTimeout = 5 * time.Second
)

// diskReading is the last free-space measurement Status reports. Status never
// touches the storage directory itself: a hung network share would block every
// status request.
type diskReading struct {
	dir  string
	free int64  // -1 when the measurement failed
	seq  uint64 // 0 until the first measurement was recorded
}

// probeLoop measures the free space of the configured storage directory when
// it starts, when signalProbe asks for it (storage directory changed, an
// operation or Delete ended) and every diskProbeInterval.
func (m *Manager) probeLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.diskProbeEvery)
	defer ticker.Stop()
	for {
		m.probeDisk()
		select {
		case <-m.lifecycleCtx.Done():
			return
		case <-ticker.C:
		case <-m.probeNow:
		}
	}
}

func (m *Manager) signalProbe() {
	select {
	case m.probeNow <- struct{}{}:
	default:
	}
}

// probeDisk starts one measurement of the configured storage directory unless
// one for that directory is still running or the integration is off (nothing
// can be installed then; Configure asks for a measurement when it is switched
// on). The measuring goroutine is not tracked by wg: it may hang on an
// unreachable share, and it only records its result, so Shutdown does not
// wait for it.
func (m *Manager) probeDisk() {
	m.mu.Lock()
	dir := m.settings.DataDir
	if _, running := m.diskInFlight[dir]; running || m.shuttingDown || !m.settings.Enabled {
		m.mu.Unlock()
		return
	}
	m.diskSeq++
	seq := m.diskSeq
	m.diskInFlight[dir] = time.Now()
	m.mu.Unlock()
	go func() {
		free, err := m.freeDisk(dir)
		if err != nil {
			free = -1
		}
		m.mu.Lock()
		delete(m.diskInFlight, dir)
		m.recordFreeLocked(dir, free, seq)
		m.mu.Unlock()
	}()
}

// recordFreeLocked stores a measurement unless one taken later was recorded
// already. seq orders measurements by the time they were started (probes) or
// recorded (a download's own check). The caller holds mu.
func (m *Manager) recordFreeLocked(dir string, free int64, seq uint64) {
	if seq < m.disk.seq {
		return
	}
	m.disk = diskReading{dir: dir, free: free, seq: seq}
}

// freeBytesLocked is the free space Status reports for dir: the last
// measurement, or -1 when there is none for dir yet, it failed, or a newer
// measurement has been running longer than diskProbeTimeout. The caller holds
// mu.
func (m *Manager) freeBytesLocked(dir string) int64 {
	if m.disk.seq == 0 || m.disk.dir != dir {
		return -1
	}
	if started, running := m.diskInFlight[dir]; running && time.Since(started) > m.diskProbeLimit {
		return -1
	}
	return m.disk.free
}
