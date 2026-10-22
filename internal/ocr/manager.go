package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type upload struct {
	file    File
	owner   string
	created time.Time
}

type Manager struct {
	mu              sync.Mutex
	lifecycle       sync.Mutex
	root            string
	policy          Policy
	runtime         Runtime
	status          Status
	uploads         map[string]upload
	jobs            map[string]*Job
	queue           chan string
	storage         int64
	now             func() time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	closed          bool
	manualStop      bool
	generation      uint64
	operationCancel context.CancelFunc
}

func New(root string, policy Policy, runtime Runtime) (*Manager, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if info, err := os.Lstat(root); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInvalid
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		// Only this service's flat UUID files are eligible for restart cleanup.
		for _, entry := range entries {
			stem := strings.Split(entry.Name(), ".")[0]
			if len(stem) == 32 && strings.Trim(stem, "0123456789abcdef") == "" && !entry.IsDir() {
				if err := os.Remove(filepath.Join(root, entry.Name())); err != nil {
					return nil, err
				}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	if policy.InstanceKey == "" {
		policy.InstanceKey = fmt.Sprintf("%x", sha256.Sum256([]byte(root)))
	}
	m := &Manager{root: root, policy: policy, runtime: runtime, uploads: map[string]upload{}, jobs: map[string]*Job{},
		queue: make(chan string, 100), now: time.Now, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	m.status = Status{State: "not_installed", Version: Version, ModelRevision: ModelRevision}
	if !policy.Enabled {
		m.status.State = "disabled"
	}
	go m.run()
	return m, nil
}

func (m *Manager) Policy() Policy { m.mu.Lock(); defer m.mu.Unlock(); return m.policy }

// CleanupDisabled revokes orphan parser resources at startup without a manual stop.
func (m *Manager) CleanupDisabled(ctx context.Context) error {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	disabled := !m.policy.Enabled && !m.closed
	m.mu.Unlock()
	if disabled && m.runtime != nil {
		return m.runtime.Stop(ctx)
	}
	return nil
}
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	s.Queued = len(m.queue)
	return s
}

// Configure revokes running work synchronously; cleanup runs outside config locks.
func (m *Manager) Configure(policy Policy) {
	m.mu.Lock()
	policy.InstanceKey = m.policy.InstanceKey
	if m.policy == policy {
		m.mu.Unlock()
		return
	}
	m.policy = policy
	m.generation++
	generation := m.generation
	m.status.Ready = false
	m.status.State = "stopped"
	m.status.Code = ""
	if !policy.Enabled {
		m.status.State = "disabled"
	}
	m.cancelJobsLocked()
	if m.operationCancel != nil {
		m.operationCancel()
	}
	m.mu.Unlock()
	go func() {
		m.lifecycle.Lock()
		m.mu.Lock()
		currentBeforeCleanup := m.generation == generation && !m.closed
		m.mu.Unlock()
		if !currentBeforeCleanup {
			m.lifecycle.Unlock()
			return
		}
		ctx, cancel := context.WithTimeout(m.ctx, 15*time.Second)
		defer cancel()
		if m.runtime != nil {
			_ = m.runtime.Stop(ctx)
		}
		m.lifecycle.Unlock()
		m.mu.Lock()
		current := m.generation == generation && !m.closed && !m.manualStop
		m.mu.Unlock()
		if current && policy.Enabled {
			_ = m.start(m.ctx, false, generation)
		}
	}()
}

func (m *Manager) Start(parent context.Context, install bool) error {
	return m.start(parent, install, 0)
}
func (m *Manager) start(parent context.Context, install bool, expectedGeneration uint64) error {
	if !m.lifecycle.TryLock() {
		return ErrBusy
	}
	defer m.lifecycle.Unlock()
	m.mu.Lock()
	if m.closed || !m.policy.Enabled {
		m.mu.Unlock()
		return ErrDisabled
	}
	if expectedGeneration != 0 && (m.generation != expectedGeneration || m.manualStop) {
		m.mu.Unlock()
		return context.Canceled
	}
	if m.runtime == nil {
		m.mu.Unlock()
		return ErrNotReady
	}
	if m.status.Ready {
		m.mu.Unlock()
		return nil
	}
	m.manualStop = false
	m.generation++
	generation := m.generation
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	m.operationCancel = cancel
	m.status.Ready = false
	m.status.Code = ""
	m.status.State = "starting"
	if install {
		m.status.State = "installing"
	}
	m.mu.Unlock()
	defer cancel()
	err := m.runtime.Start(ctx, install)
	m.mu.Lock()
	defer m.mu.Unlock()
	if generation != m.generation || m.closed {
		return context.Canceled
	}
	m.operationCancel = nil
	if err != nil {
		m.status.State, m.status.Code = "error", errorCode(err)
		if errors.Is(err, ErrNotReady) {
			m.status.State = "not_installed"
		}
		return err
	}
	m.status.State, m.status.Ready, m.status.Installed = "ready", true, true
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	m.manualStop = true
	m.generation++
	m.status.Ready = false
	m.status.State = "stopped"
	m.status.Code = ""
	if !m.policy.Enabled {
		m.status.State = "disabled"
	}
	m.cancelJobsLocked()
	if m.operationCancel != nil {
		m.operationCancel()
	}
	m.mu.Unlock()
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	if m.runtime != nil {
		err := m.runtime.Stop(ctx)
		if err != nil {
			m.mu.Lock()
			m.status.State = "error"
			m.status.Code = "ocr_stop_failed"
			m.mu.Unlock()
		}
		return err
	}
	return nil
}

func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.cancel()
	err := m.Stop(ctx)
	select {
	case <-m.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		m.removeJobFilesLocked(j)
	}
	for key, u := range m.uploads {
		_ = os.Remove(u.file.path)
		delete(m.uploads, key)
	}
	return err
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, ErrDisabled):
		return "ocr_disabled"
	case errors.Is(err, ErrNotReady):
		return "ocr_not_ready"
	case errors.Is(err, ErrNotFound):
		return "ocr_not_found"
	case errors.Is(err, ErrInvalid):
		return "ocr_invalid_input"
	case errors.Is(err, ErrLimit):
		return "ocr_limit_exceeded"
	case errors.Is(err, context.DeadlineExceeded):
		return "ocr_timeout"
	case errors.Is(err, context.Canceled):
		return "ocr_cancelled"
	default:
		return "ocr_runtime_failed"
	}
}

func (m *Manager) Submit(owner string, ids []string, mode string) (Job, error) {
	return m.submit(owner, ids, mode, false)
}
func (m *Manager) SubmitAutomatic(owner string, fileID string) (Job, error) {
	return m.submit(owner, []string{fileID}, "auto", true)
}
func (m *Manager) submit(owner string, ids []string, mode string, automatic bool) (Job, error) {
	if mode == "" {
		mode = "auto"
	}
	if owner == "" || len(ids) == 0 || len(ids) > MaxFiles || (mode != "auto" && mode != "ocr") {
		return Job{}, ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	if m.closed || !m.policy.Enabled {
		return Job{}, ErrDisabled
	}
	if !m.status.Ready {
		return Job{}, ErrNotReady
	}
	files := make([]File, 0, len(ids))
	seen := map[string]bool{}
	for _, key := range ids {
		u, ok := m.uploads[key]
		if !ok || u.owner != owner || seen[key] {
			return Job{}, ErrNotFound
		}
		seen[key] = true
		f := u.file
		f.State = "queued"
		files = append(files, f)
	}
	if automatic {
		for _, previous := range m.jobs {
			if previous.owner == owner && previous.Mode == "auto" && len(previous.Files) == 1 &&
				previous.Files[0].digest == files[0].digest &&
				(previous.State == "queued" || previous.State == "running" || previous.State == "completed") {
				_ = os.Remove(files[0].path)
				m.storage -= files[0].Size
				delete(m.uploads, files[0].ID)
				return snapshot(previous), nil
			}
		}
	}
	if len(m.queue) == cap(m.queue) {
		return Job{}, ErrBusy
	}
	j := &Job{ID: id(), State: "queued", Mode: mode, CreatedAt: m.now(), Files: files, owner: owner, cancel: func() {}}
	// The worker acquires its own context so cancellation is observable before dequeue.
	for _, key := range ids {
		delete(m.uploads, key)
	}
	m.jobs[j.ID] = j
	m.queue <- j.ID
	return snapshot(j), nil
}

func snapshot(j *Job) Job {
	result := *j
	result.Files = append([]File(nil), j.Files...)
	result.cancel = nil
	return result
}
func (m *Manager) List(owner string) []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	out := []Job{}
	for _, j := range m.jobs {
		if j.owner == owner {
			out = append(out, snapshot(j))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}
func (m *Manager) Get(owner, key string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	j, ok := m.jobs[key]
	if !ok || j.owner != owner {
		return Job{}, ErrNotFound
	}
	return snapshot(j), nil
}
func (m *Manager) Cancel(owner, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[key]
	if !ok || j.owner != owner {
		return ErrNotFound
	}
	m.cancelJobLocked(j)
	return nil
}
func (m *Manager) cancelJobLocked(j *Job) {
	if j.FinishedAt != nil {
		return
	}
	j.cancel()
	j.State = "cancelled"
	running := false
	for i := range j.Files {
		if j.Files[i].State == "running" {
			running = true
		}
		if j.Files[i].State == "queued" {
			j.Files[i].State = "cancelled"
		}
	}
	if !running {
		now := m.now()
		j.FinishedAt = &now
	}
}
func (m *Manager) cancelJobsLocked() {
	for _, j := range m.jobs {
		m.cancelJobLocked(j)
	}
}
func (m *Manager) pruneLocked() {
	now := m.now()
	for key, u := range m.uploads {
		if now.Sub(u.created) > time.Hour {
			_ = os.Remove(u.file.path)
			m.storage -= u.file.Size
			delete(m.uploads, key)
		}
	}
	for key, j := range m.jobs {
		if j.FinishedAt != nil && now.Sub(*j.FinishedAt) >= Retention {
			m.removeJobFilesLocked(j)
			delete(m.jobs, key)
		}
	}
}
func (m *Manager) removeJobFilesLocked(j *Job) {
	for _, f := range j.Files {
		for _, path := range []string{f.path, filepath.Join(m.root, f.ID+".json")} {
			if info, err := os.Stat(path); err == nil {
				if os.Remove(path) == nil {
					m.storage -= info.Size()
				}
			}
		}
	}
}
func (m *Manager) Result(owner, key, fileID string) (File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	j, ok := m.jobs[key]
	if !ok || j.owner != owner {
		return File{}, ErrNotFound
	}
	for _, f := range j.Files {
		if f.ID != fileID {
			continue
		}
		if f.State == "queued" || f.State == "running" {
			return File{}, ErrBusy
		}
		if f.Error == "ocr_storage_failed" {
			return f, nil
		}
		data, err := os.ReadFile(filepath.Join(m.root, f.ID+".json"))
		if errors.Is(err, os.ErrNotExist) {
			return f, nil
		}
		if err != nil {
			return File{}, err
		}
		if int64(len(data)) > MaxResultBytes {
			return File{}, ErrLimit
		}
		if err := json.Unmarshal(data, &f); err != nil {
			return File{}, err
		}
		return f, nil
	}
	return File{}, ErrNotFound
}

func (m *Manager) Source(owner, key, fileID string) (*os.File, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	j, ok := m.jobs[key]
	if !ok || j.owner != owner {
		return nil, "", ErrNotFound
	}
	for _, f := range j.Files {
		if f.ID == fileID {
			r, err := os.Open(f.path)
			return r, f.MIME, err
		}
	}
	return nil, "", ErrNotFound
}

func (m *Manager) run() {
	defer close(m.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			m.pruneLocked()
			m.mu.Unlock()
		case key := <-m.queue:
			m.process(key)
		}
	}
}
func (m *Manager) process(key string) {
	m.mu.Lock()
	j := m.jobs[key]
	if j == nil || j.State != "queued" {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	j.cancel = cancel
	j.State = "running"
	m.mu.Unlock()
	defer cancel()
	for index := range j.Files {
		m.mu.Lock()
		if ctx.Err() != nil || !m.status.Ready || j.State == "cancelled" {
			if !m.status.Ready && j.State != "cancelled" {
				for n := index; n < len(j.Files); n++ {
					if j.Files[n].State == "queued" {
						j.Files[n].State = "error"
						j.Files[n].Error = "ocr_not_ready"
					}
				}
			}
			m.mu.Unlock()
			break
		}
		f := j.Files[index]
		j.Files[index].State = "running"
		mode := j.Mode
		m.mu.Unlock()
		var pages []Page
		var size int64
		fileCtx, stop := context.WithTimeout(ctx, FileTimeout)
		err := m.runtime.Convert(fileCtx, f.path, f.MIME, mode, func(page Page) error {
			if fileCtx.Err() != nil {
				return fileCtx.Err()
			}
			if page.Index != len(pages) || len(pages) >= MaxPages {
				return ErrInvalid
			}
			encoded, err := json.Marshal(page)
			if err != nil {
				return err
			}
			size += int64(len(encoded))
			if size > MaxResultBytes-(64<<10) {
				return ErrLimit
			}
			pages = append(pages, page)
			m.mu.Lock()
			j.Files[index].PageCount = len(pages)
			m.mu.Unlock()
			return nil
		})
		if fileCtx.Err() != nil {
			err = fileCtx.Err()
		}
		stop()
		f.Pages, f.PageCount, f.State = pages, len(pages), "completed"
		if err == nil && len(pages) == 0 {
			err = ErrInvalid
		}
		for _, p := range pages {
			if p.Source == "error" && err == nil {
				err = fmt.Errorf("page failed")
			}
		}
		if err != nil {
			f.State, f.Error = "error", errorCode(err)
			if len(pages) > 0 {
				f.State = "partial"
			}
		}
		m.mu.Lock()
		if errors.Is(err, ErrNotReady) {
			m.status.Ready = false
			m.status.State = "error"
			m.status.Code = "ocr_not_ready"
		}
		if j.State == "cancelled" {
			f.State = "cancelled"
		}
		encoded, marshalErr := json.Marshal(f)
		if marshalErr == nil && int64(len(encoded))+m.storage <= MaxStorageBytes {
			target := filepath.Join(m.root, f.ID+".json")
			if writeErr := os.WriteFile(target, encoded, 0600); writeErr == nil {
				m.storage += int64(len(encoded))
			} else {
				if removeErr := os.Remove(target); removeErr != nil {
					if info, statErr := os.Stat(target); statErr == nil {
						m.storage += info.Size()
					}
				}
				f.State, f.Error = "error", "ocr_storage_failed"
			}
		} else {
			f.State, f.Error = "error", "ocr_limit_exceeded"
		}
		f.Pages = nil
		j.Files[index] = f
		m.mu.Unlock()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if j.State != "cancelled" {
		j.State = "error"
		successes, partial := 0, false
		for _, f := range j.Files {
			if f.State == "completed" {
				successes++
			}
			partial = partial || f.State == "partial"
		}
		if successes == len(j.Files) {
			j.State = "completed"
		} else if successes > 0 || partial {
			j.State = "partial"
		}
	}
	if j.FinishedAt == nil {
		now := m.now()
		j.FinishedAt = &now
	}
}
