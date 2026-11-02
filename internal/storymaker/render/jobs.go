// Package render turns a Storymaker story into an MP4: the job manager, the per-job loopback
// listener the headless browser talks to, frame ingest, browser provisioning and the FFmpeg
// frame-input pipeline. The HTTP handlers live in internal/server; nothing here depends on it.
package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"github.com/google/uuid"
)

// Job kinds, statuses and phases (contract section 5).
const (
	KindRender = "render"

	StatusQueued      = "queued"
	StatusRunning     = "running"
	StatusSucceeded   = "succeeded"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
	StatusInterrupted = "interrupted"

	PhaseStarting  = "starting"
	PhaseCompiling = "compiling"
	PhaseMixing    = "mixing"
	PhaseRendering = "rendering"
	PhaseEncoding  = "encoding"
	PhaseWriting   = "writing"
)

// Stable job error codes (spec "Error handling"); interrupted is the code of a job that a restart cut off.
const (
	CodeInvalidStory       = "invalid_story"
	CodeBrowserUnavailable = "browser_unavailable"
	CodeFFmpegUnavailable  = "ffmpeg_unavailable"
	CodeRenderFailed       = "render_failed"
	CodeRenderTimeout      = "render_timeout"
	CodeCancelled          = "cancelled"
	CodeDisabled           = "storymaker_disabled"
	CodeBusy               = "busy"
	CodeCodeError          = "code_error"
	CodeInterrupted        = "interrupted"
)

const (
	// DefaultMaxQueued is the number of waiting render jobs; the next one is answered with ErrBusy.
	DefaultMaxQueued = 16
	// DefaultRetention is the number of finished jobs kept in jobs.json.
	DefaultRetention = 200

	progressNotifyEvery  = 150 * time.Millisecond
	progressPersistEvery = 3 * time.Second
	jobsFileVersion      = 1
)

// ErrBusy means the render queue already holds DefaultMaxQueued waiting jobs.
var ErrBusy = errors.New("storymaker: the render queue is full")

// ErrIdempotencyConflict means an Idempotency-Key was reused for a different request.
var ErrIdempotencyConflict = errors.New("storymaker: idempotency key reused with a different request")

// ErrClosed means the manager has been closed.
var ErrClosed = errors.New("storymaker: the job manager is closed")

// JobError is the error of a failed job and the error type RunFunc returns.
type JobError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *JobError) Error() string { return e.Code + ": " + e.Message }

// Fail builds a *JobError.
func Fail(code, format string, args ...any) *JobError {
	return &JobError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Output is the file a successful render wrote: its desktop path and size.
type Output struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Job is the public job document (contract section 5).
type Job struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Progress    float64    `json:"progress"`
	Phase       string     `json:"phase,omitempty"`
	FramesDone  int        `json:"frames_done"`
	FramesTotal int        `json:"frames_total"`
	Output      *Output    `json:"output,omitempty"`
	Error       *JobError  `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// Terminal reports whether the job has ended.
func (j Job) Terminal() bool {
	switch j.Status {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusInterrupted:
		return true
	}
	return false
}

// record is a job as the manager keeps and persists it. PrefixHash is the SHA-256 of the render
// prefix of the listener that served the job (never the prefix itself); KeyHash and Fingerprint
// implement Idempotency-Key.
type record struct {
	Job
	PrefixHash  string `json:"prefix_hash,omitempty"`
	KeyHash     string `json:"key_hash,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Rev         uint64 `json:"rev"`

	cancelRequested bool
	notifiedAt      time.Time
	persistedAt     time.Time
}

type diskState struct {
	Version int       `json:"version"`
	Jobs    []*record `json:"jobs"`
}

// RunFunc executes one job. It returns the output on success and a *JobError (or any error, which
// becomes render_failed) on failure. A cancelled ctx means the job was cancelled or the manager is
// closing; the function stops its processes and returns.
type RunFunc func(ctx context.Context, job Job, payload any, h *Handle) (Output, error)

// EnqueueRequest describes a job to queue.
type EnqueueRequest struct {
	ProjectID string
	// Key is the Idempotency-Key header (optional); Fingerprint identifies the request (project id and
	// story etag): the same key with another fingerprint is an ErrIdempotencyConflict.
	Key         string
	Fingerprint string
	// Payload travels to the RunFunc unchanged. It is kept in memory only: a job that was queued
	// when the server stopped is interrupted, so it never needs to be persisted.
	Payload any
}

// ManagerOptions configures NewManager.
type ManagerOptions struct {
	// Dir is the storymaker data folder (<DataDir>/storymaker); jobs.json lives in it and the
	// staging/ folder below it is emptied at start.
	Dir       string
	Run       RunFunc
	MaxQueued int
	Retention int
	Logger    *slog.Logger
	// NewID and Now are for tests.
	NewID func() string
	Now   func() time.Time
}

// Manager runs render jobs one at a time from a bounded queue and persists them in jobs.json.
type Manager struct {
	opts   ManagerOptions
	path   string
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	jobs    map[string]*record
	payload map[string]any
	queue   []string
	wake    chan struct{}
	active  string
	cancels map[string]context.CancelFunc
	subs    map[int]*subscriber
	nextSub int
	rev     uint64
	closed  bool
}

type subscriber struct {
	project string // "" = every project
	ch      chan struct{}
}

// NewManager loads jobs.json (jobs that were queued or running become interrupted), empties the
// staging folder and starts the worker.
func NewManager(opts ManagerOptions) (*Manager, error) {
	if opts.Dir == "" || opts.Run == nil {
		return nil, errors.New("storymaker: job manager needs a folder and a run function")
	}
	if opts.MaxQueued <= 0 {
		opts.MaxQueued = DefaultMaxQueued
	}
	if opts.Retention <= 0 {
		opts.Retention = DefaultRetention
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.NewID == nil {
		opts.NewID = uuid.NewString
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if err := os.MkdirAll(opts.Dir, 0o700); err != nil {
		return nil, fmt.Errorf("storymaker: create job folder: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		opts: opts, path: filepath.Join(opts.Dir, "jobs.json"), ctx: ctx, cancel: cancel,
		jobs: map[string]*record{}, payload: map[string]any{}, wake: make(chan struct{}, 1), cancels: map[string]context.CancelFunc{}, subs: map[int]*subscriber{},
	}
	if err := m.load(); err != nil {
		cancel()
		return nil, err
	}
	clearStaging(StagingRoot(opts.Dir))
	m.wg.Add(1)
	go m.worker()
	return m, nil
}

// StagingRoot is the folder under dir that holds the per-job temporary files (Chrome profile and
// the MP4 before it is published).
func StagingRoot(dir string) string { return filepath.Join(dir, "staging") }

func clearStaging(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(root, e.Name()))
	}
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storymaker: read jobs.json: %w", err)
	}
	if err == nil {
		var st diskState
		if err := json.Unmarshal(data, &st); err != nil || st.Version != jobsFileVersion {
			// A damaged file must not lock the feature out: keep it aside and start empty.
			m.opts.Logger.Warn("[Storymaker] jobs.json is unreadable and was set aside")
			_ = os.Rename(m.path, m.path+".bad")
		} else {
			for _, r := range st.Jobs {
				if r == nil || !validJobID(r.ID) || r.Kind != KindRender {
					continue
				}
				if r.Status == StatusQueued || r.Status == StatusRunning {
					now := m.opts.Now()
					r.Status = StatusInterrupted
					r.Error = &JobError{Code: CodeInterrupted, Message: "The server restarted before this render finished. Start the export again."}
					r.FinishedAt = &now
					r.UpdatedAt = now
				}
				if r.Rev > m.rev {
					m.rev = r.Rev
				}
				m.jobs[r.ID] = r
			}
		}
	}
	return m.persistLocked()
}

func validJobID(id string) bool {
	u, err := uuid.Parse(id)
	return err == nil && u.String() == id
}

func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// HashPrefix returns the hash a job stores in place of its render prefix.
func HashPrefix(prefix string) string { return hashHex("storymaker-render-prefix\x00" + prefix) }

func (m *Manager) persistLocked() error {
	st := diskState{Version: jobsFileVersion}
	for _, r := range m.jobs {
		st.Jobs = append(st.Jobs, r)
	}
	sort.Slice(st.Jobs, func(i, j int) bool {
		if !st.Jobs[i].CreatedAt.Equal(st.Jobs[j].CreatedAt) {
			return st.Jobs[i].CreatedAt.Before(st.Jobs[j].CreatedAt)
		}
		return st.Jobs[i].ID < st.Jobs[j].ID
	})
	data, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return fmt.Errorf("storymaker: encode jobs: %w", err)
	}
	if err := writeAtomic(m.path, data, 0o600); err != nil {
		return fmt.Errorf("storymaker: persist jobs: %w", err)
	}
	now := m.opts.Now()
	for _, r := range m.jobs {
		r.persistedAt = now
	}
	return nil
}

// writeAtomic writes data to path through a temp file in the same folder.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if err := tmp.Chmod(perm); err != nil && !errors.Is(err, os.ErrPermission) {
		// Windows ignores most mode bits; other failures are not fatal for a state file.
		_ = err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := fileutil.Rename(name, path); err != nil {
		return err
	}
	ok = true
	return nil
}

// Enqueue queues a render. With an Idempotency-Key that was seen for this project and request it
// returns the earlier job and replayed true. ErrBusy when DefaultMaxQueued jobs are waiting.
func (m *Manager) Enqueue(req EnqueueRequest) (job Job, replayed bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Job{}, false, ErrClosed
	}
	keyHash := ""
	if req.Key != "" {
		keyHash = hashHex(req.ProjectID + "\x00" + req.Key)
		for _, r := range m.jobs {
			if r.KeyHash == keyHash {
				if r.Fingerprint != req.Fingerprint {
					return Job{}, false, ErrIdempotencyConflict
				}
				return r.Job, true, nil
			}
		}
	}
	if len(m.queue) >= m.opts.MaxQueued {
		return Job{}, false, ErrBusy
	}
	now := m.opts.Now()
	r := &record{
		Job: Job{
			ID: m.opts.NewID(), ProjectID: req.ProjectID, Kind: KindRender, Status: StatusQueued,
			CreatedAt: now, UpdatedAt: now,
		},
		KeyHash: keyHash, Fingerprint: req.Fingerprint,
	}
	m.jobs[r.ID] = r
	m.payload[r.ID] = req.Payload
	m.queue = append(m.queue, r.ID)
	m.touchLocked(r)
	m.pruneLocked()
	if err := m.persistLocked(); err != nil {
		delete(m.jobs, r.ID)
		delete(m.payload, r.ID)
		m.queue = m.queue[:len(m.queue)-1]
		return Job{}, false, err
	}
	m.notifyLocked(r)
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return r.Job, false, nil
}

// touchLocked stamps UpdatedAt and the revision of r.
func (m *Manager) touchLocked(r *record) {
	m.rev++
	r.Rev = m.rev
	r.UpdatedAt = m.opts.Now()
}

func (m *Manager) pruneLocked() {
	var finished []*record
	for _, r := range m.jobs {
		if r.Terminal() {
			finished = append(finished, r)
		}
	}
	if len(finished) <= m.opts.Retention {
		return
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].CreatedAt.Before(finished[j].CreatedAt) })
	for _, r := range finished[:len(finished)-m.opts.Retention] {
		delete(m.jobs, r.ID)
	}
}

func (m *Manager) notifyLocked(r *record) {
	r.notifiedAt = m.opts.Now()
	for _, s := range m.subs {
		if s.project != "" && s.project != r.ProjectID {
			continue
		}
		select {
		case s.ch <- struct{}{}:
		default:
		}
	}
}

// Get returns a copy of the job.
func (m *Manager) Get(id string) (Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.jobs[id]
	if !ok {
		return Job{}, false
	}
	return r.Job, true
}

// List returns the jobs of a project (all jobs for ""), newest first.
func (m *Manager) List(projectID string) []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Job
	for _, r := range m.jobs {
		if projectID == "" || r.ProjectID == projectID {
			out = append(out, r.Job)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out
}

// PrefixHash returns the stored hash of the render prefix of a job ("" until the listener started).
func (m *Manager) PrefixHash(id string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.jobs[id]; ok {
		return r.PrefixHash
	}
	return ""
}

// Subscribe returns a channel that receives a signal after every job change of the project ("" for
// all projects) and a function that ends the subscription. Signals coalesce; call ChangedSince for
// the jobs themselves.
func (m *Manager) Subscribe(projectID string) (<-chan struct{}, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextSub
	m.nextSub++
	s := &subscriber{project: projectID, ch: make(chan struct{}, 1)}
	m.subs[id] = s
	return s.ch, func() {
		m.mu.Lock()
		delete(m.subs, id)
		m.mu.Unlock()
	}
}

// Revision returns the current change counter.
func (m *Manager) Revision() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rev
}

// ChangedSince returns the jobs of the project that changed after revision rev and the highest
// revision among them (rev itself when nothing changed).
func (m *Manager) ChangedSince(projectID string, rev uint64) ([]Job, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var changed []*record
	for _, r := range m.jobs {
		if (projectID == "" || r.ProjectID == projectID) && r.Rev > rev {
			changed = append(changed, r)
		}
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Rev < changed[j].Rev })
	out := make([]Job, 0, len(changed))
	for _, r := range changed {
		out = append(out, r.Job)
		if r.Rev > rev {
			rev = r.Rev
		}
	}
	return out, rev
}

// Cancel cancels a queued or running job and returns its state. A finished job is returned as is.
func (m *Manager) Cancel(id string) (Job, bool) {
	m.mu.Lock()
	r, ok := m.jobs[id]
	if !ok {
		m.mu.Unlock()
		return Job{}, false
	}
	switch r.Status {
	case StatusQueued:
		m.removeFromQueueLocked(id)
		delete(m.payload, id)
		m.finishLocked(r, StatusCancelled, &JobError{Code: CodeCancelled, Message: "The export was cancelled."}, nil)
		_ = m.persistLocked()
	case StatusRunning:
		r.cancelRequested = true
		if cancel := m.cancels[id]; cancel != nil {
			cancel()
		}
	}
	job := r.Job
	m.mu.Unlock()
	return job, true
}

// CancelProject cancels every unfinished job of a project (a deleted project).
func (m *Manager) CancelProject(projectID string) {
	m.mu.Lock()
	var ids []string
	for id, r := range m.jobs {
		if r.ProjectID == projectID && !r.Terminal() {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Cancel(id)
	}
}

func (m *Manager) removeFromQueueLocked(id string) {
	for i, q := range m.queue {
		if q == id {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			return
		}
	}
}

func (m *Manager) finishLocked(r *record, status string, jobErr *JobError, out *Output) {
	now := m.opts.Now()
	r.Status = status
	r.Error = jobErr
	r.Output = out
	r.FinishedAt = &now
	if status == StatusSucceeded {
		r.Progress = 1
	}
	m.touchLocked(r)
	m.notifyLocked(r)
}

// Close cancels the running job (it ends as interrupted), marks the queued ones interrupted and
// waits for the worker.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.cancel()
	for _, id := range m.queue {
		if r := m.jobs[id]; r != nil {
			m.finishLocked(r, StatusInterrupted, &JobError{Code: CodeInterrupted, Message: "The server stopped before this render started."}, nil)
		}
	}
	m.queue = nil
	m.payload = map[string]any{}
	_ = m.persistLocked()
	m.mu.Unlock()
	m.wg.Wait()
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return
		}
		var id string
		if len(m.queue) > 0 {
			id = m.queue[0]
			m.queue = m.queue[1:]
		}
		m.mu.Unlock()
		if id == "" {
			select {
			case <-m.ctx.Done():
				return
			case <-m.wake:
			}
			continue
		}
		m.runJob(id)
	}
}

func (m *Manager) runJob(id string) {
	m.mu.Lock()
	r := m.jobs[id]
	if r == nil || r.Status != StatusQueued {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancels[id] = cancel
	m.active = id
	now := m.opts.Now()
	r.Status = StatusRunning
	r.StartedAt = &now
	r.Phase = PhaseStarting
	m.touchLocked(r)
	_ = m.persistLocked()
	m.notifyLocked(r)
	job := r.Job
	payload := m.payload[id]
	m.mu.Unlock()

	out, err := m.safeRun(ctx, job, payload, &Handle{m: m, id: id})
	cancel()

	m.mu.Lock()
	delete(m.cancels, id)
	delete(m.payload, id)
	m.active = ""
	switch {
	case err == nil:
		o := out
		m.finishLocked(r, StatusSucceeded, nil, &o)
	case r.cancelRequested && !m.closed:
		m.finishLocked(r, StatusCancelled, &JobError{Code: CodeCancelled, Message: "The export was cancelled."}, nil)
	case m.closed && ctx.Err() != nil:
		m.finishLocked(r, StatusInterrupted, &JobError{Code: CodeInterrupted, Message: "The server stopped before this render finished. Start the export again."}, nil)
	default:
		var je *JobError
		if !errors.As(err, &je) {
			je = &JobError{Code: CodeRenderFailed, Message: clip(err.Error(), 400)}
		}
		m.finishLocked(r, StatusFailed, je, nil)
	}
	m.pruneLocked()
	_ = m.persistLocked()
	m.mu.Unlock()
}

func (m *Manager) safeRun(ctx context.Context, job Job, payload any, h *Handle) (out Output, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			m.opts.Logger.Error("[Storymaker] Recovered from a panic in a render job", "panic", rec)
			err = &JobError{Code: CodeRenderFailed, Message: "The render stopped unexpectedly."}
		}
	}()
	return m.opts.Run(ctx, job, payload, h)
}

// Handle lets a running RunFunc report its progress.
type Handle struct {
	m  *Manager
	id string
}

func (h *Handle) update(fn func(r *record), state bool) {
	m := h.m
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.jobs[h.id]
	if r == nil || r.Status != StatusRunning {
		return
	}
	fn(r)
	now := m.opts.Now()
	if !state && now.Sub(r.notifiedAt) < progressNotifyEvery {
		return
	}
	m.touchLocked(r)
	m.notifyLocked(r)
	if state || now.Sub(r.persistedAt) >= progressPersistEvery {
		_ = m.persistLocked()
	}
}

// SetPhase records the phase (a state change: it is persisted and announced at once).
func (h *Handle) SetPhase(phase string) {
	h.update(func(r *record) { r.Phase = phase }, true)
}

// SetFrames records the frame counters; progress is frames done over frames total, scaled into
// the rendering part of the job (0.05 .. 0.95), the rest is start-up and publishing.
func (h *Handle) SetFrames(done, total int) {
	h.update(func(r *record) {
		r.FramesDone, r.FramesTotal = done, total
		if total > 0 {
			r.Progress = 0.05 + 0.90*float64(done)/float64(total)
		}
	}, false)
}

// SetPrefixHash stores the hash of the render prefix of this job's listener.
func (h *Handle) SetPrefixHash(hash string) {
	h.update(func(r *record) { r.PrefixHash = hash }, true)
}

// SetProgress sets the progress directly (0..1).
func (h *Handle) SetProgress(p float64) {
	h.update(func(r *record) {
		if p < 0 {
			p = 0
		}
		if p > 1 {
			p = 1
		}
		r.Progress = p
	}, false)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
