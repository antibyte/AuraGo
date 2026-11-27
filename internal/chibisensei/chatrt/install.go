package chatrt

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"aurago/internal/security"
)

const (
	installTimeout  = 60 * time.Minute
	verifyTimeout   = 2 * time.Minute
	diskHeadroomMul = 4
	diskHeadroom    = 64 << 20

	errorHandleDiskFull syscall.Errno = 39  // ERROR_HANDLE_DISK_FULL
	errorDiskFull       syscall.Errno = 112 // ERROR_DISK_FULL
)

// installOp is the one running download. Its fields are guarded by Manager.mu.
type installOp struct {
	done      chan struct{}
	doneBytes map[string]int64
	err       error
}

func (op *installOp) doneTotal() int64 {
	var total int64
	for _, n := range op.doneBytes {
		total += n
	}
	return total
}

// Installed reports whether the install set is complete on disk. It never touches the network; the first call
// verifies the files (hashing the archives) and the result is cached until an install or a backend change.
func (m *Manager) Installed() bool {
	cfg := m.cfg()
	if !cfg.Enabled {
		return false
	}
	res := m.resolve(m.life)
	if res.unsupported {
		return false
	}
	m.mu.Lock()
	if m.inst != nil {
		m.mu.Unlock()
		return false // a verify would wait for the download's file lock
	}
	if m.installed != nil {
		v := *m.installed
		m.mu.Unlock()
		return v
	}
	m.mu.Unlock()

	m.verifyMu.Lock()
	defer m.verifyMu.Unlock()
	m.mu.Lock()
	if m.installed != nil {
		v := *m.installed
		m.mu.Unlock()
		return v
	}
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(m.life, verifyTimeout)
	defer cancel()
	ok := m.launcher.Verify(ctx, cfg.CacheDir, res.assets)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inst == nil && m.installed == nil && m.life.Err() == nil {
		m.installed = &ok
	}
	return ok
}

// Install starts or joins the download and returns when it ends.
func (m *Manager) Install(ctx context.Context) error {
	op, err := m.beginInstall(ctx)
	if err != nil {
		return err
	}
	select {
	case <-op.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return op.err
}

// BeginInstall registers the download (state "installing") and returns a channel that closes when it ends; the
// caller reads the result from Status. A second call while installing returns the same channel; with everything
// installed it returns a closed channel. ctx cancels the download, so pass a context that outlives the request
// for a background install.
func (m *Manager) BeginInstall(ctx context.Context) (<-chan struct{}, error) {
	op, err := m.beginInstall(ctx)
	if err != nil {
		return nil, err
	}
	return op.done, nil
}

func (m *Manager) beginInstall(ctx context.Context) (*installOp, error) {
	if err := m.precheck(); err != nil {
		return nil, err
	}
	res := m.resolve(ctx)
	if res.unsupported {
		return nil, ErrUnsupported
	}
	if m.Installed() {
		done := make(chan struct{})
		close(done)
		return &installOp{done: done}, nil
	}
	cfg := m.cfg()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrClosed
	}
	if m.inst != nil {
		op := m.inst
		m.mu.Unlock()
		return op, nil
	}
	op := &installOp{done: make(chan struct{}), doneBytes: map[string]int64{}}
	m.inst = op
	m.lastErrCode = ""
	m.gpuFailed = false
	m.failedFor = ""
	m.installed = nil
	m.wg.Add(1)
	m.mu.Unlock()

	runCtx, cancel := context.WithTimeout(m.life, installTimeout)
	stopAfter := context.AfterFunc(ctx, cancel)
	go func() {
		defer m.wg.Done()
		defer cancel()
		defer stopAfter()
		err := m.runInstall(runCtx, cfg, res, op)
		m.mu.Lock()
		if err == nil {
			ok := true
			m.installed = &ok
		} else {
			ok := false
			m.installed = &ok
			m.lastErrCode = installErrorCode(err)
			op.err = err
			m.log.Warn("Chibi Sensei chat runtime install failed", "code", m.lastErrCode, "error", security.Scrub(err.Error()))
		}
		m.inst = nil
		m.mu.Unlock()
		close(op.done)
	}()
	return op, nil
}

// precheck answers the states that exclude an install.
func (m *Manager) precheck() error {
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return ErrClosed
	}
	if !m.cfg().Enabled {
		return ErrDisabled
	}
	return nil
}

func (m *Manager) runInstall(ctx context.Context, cfg Config, res resolution, op *installOp) error {
	need := res.totalBytes()*diskHeadroomMul + diskHeadroom
	if free, err := m.freeDisk(filepath.Dir(cfg.CacheDir)); err == nil && free < need {
		return syscall.ENOSPC
	}
	return m.launcher.Install(ctx, cfg.CacheDir, res.assets, func(assetID string, done int64) {
		m.mu.Lock()
		op.doneBytes[assetID] = done
		m.mu.Unlock()
	})
}

// installErrorCode maps an install error to the code the status shows: disk_full, checksum_mismatch or
// download_failed. The raw error only goes to the log.
func installErrorCode(err error) string {
	if isNoSpace(err) {
		return "disk_full"
	}
	msg := err.Error()
	if strings.Contains(msg, "SHA-256") || strings.Contains(msg, "content length") || strings.Contains(msg, "received") {
		return "checksum_mismatch"
	}
	return "download_failed"
}

// isNoSpace reports whether err means the disk is full: ENOSPC or EDQUOT, or ERROR_DISK_FULL and
// ERROR_HANDLE_DISK_FULL on Windows. The error number decides; the English messages are a fallback for errors
// without a number.
func isNoSpace(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == syscall.ENOSPC || errno == syscall.EDQUOT {
			return true
		}
		return runtime.GOOS == "windows" && (errno == errorDiskFull || errno == errorHandleDiskFull)
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no space left") || strings.Contains(msg, "not enough space on the disk") || strings.Contains(msg, "disk quota exceeded")
}
