package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/fileutil"
	"aurago/internal/uid"
	"aurago/internal/videostudio"
)

const (
	videoStudioJobRetention  = 200
	videoStudioIdemRetention = 500
	// ponytail: one worker per server; add bounded concurrency if queue latency warrants it.
	videoStudioQueueSize = 128
	// ponytail: 256 project lock stripes bound lock memory; raise only if measured contention warrants it.
	videoStudioProjectLockStripeCount = 256
)

var (
	errVideoStudioProjectSizeLimit = errors.New("project_size_limit")
	errVideoStudioAssetSizeLimit   = errors.New("asset_size_limit")
	errVideoStudioGenerationImport = errors.New("generation_import_failed")
)

const videoStudioUploadReadWindow = 15 * time.Minute
const videoStudioUploadIdleWindow = 30 * time.Second

type videoStudioArtifact struct {
	Name        string `json:"name"`
	Path        string `json:"path,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	Size        int64  `json:"size"`
	privatePath string `json:"-"`
}

type videoStudioJob struct {
	ID                    string                 `json:"id"`
	ProjectID             string                 `json:"project_id"`
	Kind                  string                 `json:"kind"`
	Status                string                 `json:"status"`
	Progress              float64                `json:"progress"`
	CreatedAt             time.Time              `json:"created_at"`
	StartedAt             *time.Time             `json:"started_at,omitempty"`
	FinishedAt            *time.Time             `json:"finished_at,omitempty"`
	Artifact              *videoStudioArtifact   `json:"artifact,omitempty"`
	Result                map[string]interface{} `json:"result,omitempty"`
	Error                 string                 `json:"error,omitempty"`
	Message               string                 `json:"message,omitempty"`
	ExternalStatusUnknown bool                   `json:"external_status_unknown,omitempty"`
	KeyHash               string                 `json:"-"`
	Fingerprint           string                 `json:"-"`
	progressPersistedAt   time.Time              `json:"-"`
}

type videoStudioMediaRecord struct {
	ProjectID string            `json:"project_id"`
	Asset     videostudio.Asset `json:"asset"`
	Path      string            `json:"path"`
	SHA256    string            `json:"sha256"`
	CreatedAt time.Time         `json:"created_at"`
}

type videoStudioIdempotency struct {
	ProjectID   string `json:"project_id"`
	KeyHash     string `json:"key_hash"`
	Fingerprint string `json:"fingerprint"`
	JobID       string `json:"job_id"`
}

type videoStudioDiskState struct {
	Version     int                      `json:"version"`
	Jobs        []*videoStudioJob        `json:"jobs"`
	Media       []videoStudioMediaRecord `json:"media"`
	Idempotency []videoStudioIdempotency `json:"idempotency"`
}

type videoStudioWork struct {
	kind           string
	projectID      string
	project        videostudio.Project
	revision       string
	assetID        string
	mediaPath      string
	request        videoStudioJobRequest
	admittedConfig *config.Config
	admittedEpoch  uint64
}

type videoStudioJobRequest struct {
	AssetID           string   `json:"asset_id,omitempty"`
	Prompt            string   `json:"prompt,omitempty"`
	NegativePrompt    string   `json:"negative_prompt,omitempty"`
	DurationSeconds   int      `json:"duration_seconds,omitempty"`
	Resolution        string   `json:"resolution,omitempty"`
	AspectRatio       string   `json:"aspect_ratio,omitempty"`
	FirstFrameAssetID string   `json:"first_frame_asset_id,omitempty"`
	LastFrameAssetID  string   `json:"last_frame_asset_id,omitempty"`
	ReferenceAssetIDs []string `json:"reference_asset_ids,omitempty"`
	IdempotencyKey    string   `json:"idempotency_key,omitempty"`
}

type videoStudioManager struct {
	server         *Server
	ctx            context.Context
	cancel         context.CancelFunc
	dataDir        string
	path           string
	previewDir     string
	mu             sync.Mutex
	jobs           map[string]*videoStudioJob
	media          map[string]map[string]videoStudioMediaRecord
	idem           map[string]videoStudioIdempotency
	works          map[string]*videoStudioWork
	active         map[string]context.CancelFunc
	projectMu      [videoStudioProjectLockStripeCount]sync.Mutex
	queue          chan string
	closed         bool
	configChanging bool
	epoch          uint64
	wg             sync.WaitGroup
}

func (s *Server) videoStudioManager() (*videoStudioManager, error) {
	if s == nil {
		return nil, fmt.Errorf("video studio unavailable")
	}
	s.videoStudioMu.Lock()
	defer s.videoStudioMu.Unlock()
	if s.videoStudioClosed {
		return nil, fmt.Errorf("video studio is shutting down")
	}
	if s.videoStudioConfigRevoking.Load() {
		return nil, fmt.Errorf("video studio configuration is changing")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || strings.TrimSpace(cfg.Directories.DataDir) == "" {
		return nil, fmt.Errorf("video studio data directory is unavailable")
	}
	dataDir := normalizedVideoStudioRoot(cfg.Directories.DataDir)
	if s.videoStudio != nil {
		if sameVideoStudioRoot(s.videoStudio.dataDir, dataDir) {
			return s.videoStudio, nil
		}
		old := s.videoStudio
		s.videoStudio = nil
		old.close()
	}
	manager, err := newVideoStudioManager(s, dataDir)
	if err != nil {
		return nil, err
	}
	s.videoStudio = manager
	return manager, nil
}

func (s *Server) beginVideoStudioConfigChange() {
	if s == nil {
		return
	}
	s.videoStudioMu.Lock()
	s.videoStudioConfigRevoking.Store(true)
	manager := s.videoStudio
	s.videoStudioMu.Unlock()
	if manager != nil {
		manager.cancelAll()
	}
}

func (s *Server) finishVideoStudioConfigChange() {
	if s == nil {
		return
	}
	s.videoStudioMu.Lock()
	manager := s.videoStudio
	s.videoStudioMu.Unlock()
	s.videoStudioConfigRevoking.Store(false)
	if manager != nil {
		manager.finishConfigChange()
	}
}

func (s *Server) closeVideoStudioManager() {
	if s == nil {
		return
	}
	s.videoStudioMu.Lock()
	s.videoStudioClosed = true
	manager := s.videoStudio
	s.videoStudio = nil
	s.videoStudioMu.Unlock()
	if manager != nil {
		manager.close()
	}
}

func normalizedVideoStudioRoot(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(path)
}

func sameVideoStudioRoot(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func videoStudioConfigRootsChanged(previous, next *config.Config) bool {
	if previous == nil || next == nil {
		return false
	}
	if previous.VideoStudio != next.VideoStudio {
		return true
	}
	return !sameVideoStudioRoot(previous.Directories.DataDir, next.Directories.DataDir) ||
		!sameVideoStudioRoot(previous.Directories.WorkspaceDir, next.Directories.WorkspaceDir) ||
		!sameVideoStudioRoot(previous.VirtualDesktop.WorkspaceDir, next.VirtualDesktop.WorkspaceDir) ||
		!sameVideoStudioRoot(previous.Tools.DocumentCreator.OutputDir, next.Tools.DocumentCreator.OutputDir) ||
		previous.VideoGeneration != next.VideoGeneration ||
		previous.VirtualDesktop.Enabled != next.VirtualDesktop.Enabled ||
		previous.VirtualDesktop.ReadOnly != next.VirtualDesktop.ReadOnly
}

func beginVideoStudioUploadReadWindow(w http.ResponseWriter, body io.ReadCloser) (io.ReadCloser, func(), error) {
	controller := http.NewResponseController(w)
	started := time.Now()
	maxEnd := started.Add(videoStudioUploadReadWindow)
	setIdleDeadline := func() error {
		deadline := time.Now().Add(videoStudioUploadIdleWindow)
		if deadline.After(maxEnd) {
			deadline = maxEnd
		}
		err := controller.SetReadDeadline(deadline)
		if errors.Is(err, http.ErrNotSupported) {
			return nil
		}
		return err
	}
	if err := setIdleDeadline(); err != nil {
		return nil, nil, fmt.Errorf("extend upload read deadline: %w", err)
	}
	if err := controller.SetWriteDeadline(maxEnd.Add(videoStudioUploadIdleWindow)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return nil, nil, fmt.Errorf("extend upload response deadline: %w", err)
	}
	wrapped := videoStudioDeadlineReader{ReadCloser: body, renew: setIdleDeadline}
	reset := func() {
		deadline := time.Now().Add(videoStudioUploadIdleWindow)
		_ = controller.SetReadDeadline(deadline)
		_ = controller.SetWriteDeadline(deadline)
	}
	return wrapped, reset, nil
}

type videoStudioDeadlineReader struct {
	io.ReadCloser
	renew func() error
}

func (r videoStudioDeadlineReader) Read(p []byte) (int, error) {
	if err := r.renew(); err != nil {
		return 0, err
	}
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		_ = r.renew()
	}
	return n, err
}

func newVideoStudioManager(s *Server, dataDir string) (*videoStudioManager, error) {
	root := filepath.Join(dataDir, "video_studio")
	previewDir := filepath.Join(root, "previews")
	if err := os.MkdirAll(previewDir, 0o700); err != nil {
		return nil, fmt.Errorf("create private video studio storage: %w", err)
	}
	_ = os.Chmod(root, 0o700)
	_ = os.Chmod(previewDir, 0o700)
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	m := &videoStudioManager{
		server: s, ctx: ctx, cancel: cancel, dataDir: normalizedVideoStudioRoot(dataDir),
		path: filepath.Join(root, "jobs.json"), previewDir: previewDir,
		jobs: make(map[string]*videoStudioJob), media: make(map[string]map[string]videoStudioMediaRecord),
		idem: make(map[string]videoStudioIdempotency), works: make(map[string]*videoStudioWork),
		active: make(map[string]context.CancelFunc),
		queue:  make(chan string, videoStudioQueueSize),
	}
	if err := m.load(); err != nil {
		cancel()
		return nil, err
	}
	m.wg.Add(1)
	go m.worker()
	return m, nil
}

func (m *videoStudioManager) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read video studio job state: %w", err)
	}
	if err == nil {
		var state videoStudioDiskState
		if err := json.Unmarshal(data, &state); err != nil || state.Version != 1 {
			return fmt.Errorf("decode video studio job state")
		}
		for _, job := range state.Jobs {
			if job == nil || job.ID == "" || !validVideoStudioProjectID(job.ProjectID) {
				continue
			}
			if job.Status == "queued" || job.Status == "running" {
				wasRunning := job.Status == "running"
				job.Status = "interrupted"
				job.Error = "interrupted"
				if wasRunning && job.Kind == "generate" && job.ExternalStatusUnknown {
					job.Message = "The server restarted while the provider may still have been processing this generation. Check the provider before starting another paid request."
				} else {
					job.Message = "The server restarted before this job completed. Start a new job to try again."
				}
				finished := time.Now().UTC()
				job.FinishedAt = &finished
			}
			m.jobs[job.ID] = job
		}
		for _, record := range state.Media {
			if validVideoStudioProjectID(record.ProjectID) && validVideoStudioAssetID(record.Asset.ID) && videoStudioMediaPathValid(record.Path, record.Asset.ID) {
				if m.media[record.ProjectID] == nil {
					m.media[record.ProjectID] = make(map[string]videoStudioMediaRecord)
				}
				m.media[record.ProjectID][record.Asset.ID] = record
			}
		}
		for _, item := range state.Idempotency {
			if item.ProjectID != "" && item.KeyHash != "" && item.JobID != "" {
				m.idem[videoStudioIdemMapKey(item.ProjectID, item.KeyHash)] = item
			}
		}
	}
	return m.persistLocked()
}

func (m *videoStudioManager) persistLocked() error {
	state := videoStudioDiskState{Version: 1}
	for _, job := range m.jobs {
		copyJob := *job
		copyJob.KeyHash = ""
		copyJob.Fingerprint = ""
		if copyJob.Artifact != nil {
			artifact := *copyJob.Artifact
			artifact.privatePath = ""
			copyJob.Artifact = &artifact
		}
		state.Jobs = append(state.Jobs, &copyJob)
	}
	for _, records := range m.media {
		for _, record := range records {
			state.Media = append(state.Media, record)
		}
	}
	for _, item := range m.idem {
		state.Idempotency = append(state.Idempotency, item)
	}
	encoded, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode video studio state: %w", err)
	}
	if err := config.WriteFileAtomic(m.path, encoded, 0o600); err != nil {
		return fmt.Errorf("persist video studio state: %w", err)
	}
	return nil
}

func (m *videoStudioManager) projectLock(projectID string) *sync.Mutex {
	stripe := crc32.ChecksumIEEE([]byte(projectID)) % uint32(len(m.projectMu))
	return &m.projectMu[stripe]
}

func (m *videoStudioManager) admissionEpoch() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.epoch
}

func (m *videoStudioManager) finishConfigChange() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.configChanging = false
	m.mu.Unlock()
}

func (m *videoStudioManager) enqueue(projectID, kind, fingerprint, key string, work *videoStudioWork) (*videoStudioJob, bool, error) {
	keyHash := videoStudioHash(key)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, false, fmt.Errorf("video_studio_shutdown")
	}
	if work != nil && work.admittedConfig != nil && (m.configChanging || m.server.videoStudioConfigRevoking.Load() || m.epoch != work.admittedEpoch || videoStudioConfigRootsChanged(work.admittedConfig, m.server.ConfigSnapshot())) {
		return nil, false, fmt.Errorf("configuration_changed")
	}
	idemKey := videoStudioIdemMapKey(projectID, keyHash)
	if previous, ok := m.idem[idemKey]; ok {
		if previous.Fingerprint != fingerprint {
			return nil, false, fmt.Errorf("idempotency_conflict")
		}
		job := m.jobs[previous.JobID]
		if job != nil {
			copyJob := *job
			return &copyJob, true, nil
		}
	}
	if kind == "generate" {
		projectAssets := len(m.media[projectID])
		pendingGenerations := 0
		for _, existing := range m.jobs {
			if existing.ProjectID == projectID && existing.Kind == "generate" &&
				(existing.Status == "queued" || existing.Status == "running") {
				pendingGenerations++
			}
		}
		if projectAssets+pendingGenerations >= videostudio.MaxAssetsPerProject {
			return nil, false, fmt.Errorf("asset_count_limit")
		}
	}
	if len(m.queue) >= cap(m.queue) {
		return nil, false, fmt.Errorf("video_studio_queue_full")
	}
	job := &videoStudioJob{
		ID: uidNew(), ProjectID: projectID, Kind: kind, Status: "queued", CreatedAt: time.Now().UTC(),
		KeyHash: keyHash, Fingerprint: fingerprint,
	}
	if work != nil {
		work.projectID = projectID
		work.kind = kind
		m.works[job.ID] = work
	}
	m.jobs[job.ID] = job
	m.idem[idemKey] = videoStudioIdempotency{ProjectID: projectID, KeyHash: keyHash, Fingerprint: fingerprint, JobID: job.ID}
	if err := m.persistLocked(); err != nil {
		delete(m.jobs, job.ID)
		delete(m.works, job.ID)
		delete(m.idem, idemKey)
		return nil, false, err
	}
	m.queue <- job.ID
	copyJob := *job
	return &copyJob, false, nil
}

func (m *videoStudioManager) worker() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case id := <-m.queue:
			m.runJob(id)
		}
	}
}

func (m *videoStudioManager) runJob(id string) {
	m.mu.Lock()
	job, work := m.jobs[id], m.works[id]
	if m.closed || m.configChanging || m.server.videoStudioConfigRevoking.Load() || m.ctx.Err() != nil || job == nil || job.Status != "queued" || work == nil {
		m.mu.Unlock()
		return
	}
	cfg := m.server.ConfigSnapshot()
	if cfg == nil || !cfg.VideoStudio.Enabled || !cfg.VirtualDesktop.Enabled || cfg.VideoStudio.ReadOnly || cfg.VirtualDesktop.ReadOnly {
		job.Status = "cancelled"
		job.Error = "action_revoked"
		job.Message = "Video Studio or Desktop permissions changed before the job started."
		finished := time.Now().UTC()
		job.FinishedAt = &finished
		delete(m.works, id)
		_ = m.persistLocked()
		m.mu.Unlock()
		return
	}
	started := time.Now().UTC()
	job.Status, job.StartedAt, job.Progress = "running", &started, 0.01
	if err := m.persistLocked(); err != nil {
		job.Status = "failed"
		job.Error = "state_write_failed"
		job.Message = "The job could not be recorded safely."
		finished := time.Now().UTC()
		job.FinishedAt = &finished
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	timeout := time.Duration(cfg.VideoStudio.RenderTimeoutSeconds) * time.Second
	if work.kind == "generate" {
		timeout = time.Duration(cfg.VideoGeneration.TimeoutSeconds+120) * time.Second
		if timeout < 5*time.Minute {
			timeout = 5 * time.Minute
		}
	}
	ctx, cancel, err := m.server.beginDesktopBackgroundRun(timeout)
	if err != nil {
		m.finish(id, "cancelled", "action_revoked", "The desktop action was revoked before processing.", nil, nil)
		return
	}
	jobCtx, jobCancel := context.WithCancel(ctx)
	stopManagerCancel := context.AfterFunc(m.ctx, jobCancel)
	stop := func() { stopManagerCancel(); jobCancel(); cancel() }
	m.mu.Lock()
	if m.closed || m.ctx.Err() != nil || m.jobs[id] != job || job.Status != "running" {
		m.mu.Unlock()
		stop()
		return
	}
	m.active[id] = stop
	m.mu.Unlock()
	jobCtx = m.publicationContext(jobCtx, id)
	artifact, result, runErr := m.process(jobCtx, job, work, cfg)
	m.mu.Lock()
	delete(m.active, id)
	m.mu.Unlock()
	if runErr != nil {
		if m.server != nil && m.server.Logger != nil {
			m.server.Logger.Error("Video Studio background job failed", "job_id", id, "kind", work.kind, "error", runErr)
		}
		status, code, message := videoStudioJobFailure(work.kind, runErr, jobCtx)
		if work.kind == "generate" {
			if failedJob := m.job(id); failedJob != nil && failedJob.ExternalStatusUnknown {
				message = "The provider may still be processing this request. Check provider status before starting another paid request."
			}
		}
		stop()
		m.finish(id, status, code, message, nil, nil)
		return
	}
	stop()
	m.finish(id, "succeeded", "", "", artifact, result)
}

func (m *videoStudioManager) publicationContext(ctx context.Context, jobID string) context.Context {
	m.mu.Lock()
	epoch := m.epoch
	m.mu.Unlock()
	return fileutil.WithPublicationGate(ctx, func(commit func() error) error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.closed || m.configChanging || m.server.videoStudioConfigRevoking.Load() || m.epoch != epoch || ctx.Err() != nil {
			return context.Canceled
		}
		if jobID != "" {
			job := m.jobs[jobID]
			if job == nil || job.Status != "running" {
				return context.Canceled
			}
		}
		cfg := m.server.ConfigSnapshot()
		if cfg == nil || !cfg.VideoStudio.Enabled || cfg.VideoStudio.ReadOnly || !cfg.VirtualDesktop.Enabled || cfg.VirtualDesktop.ReadOnly {
			return context.Canceled
		}
		return fileutil.PublishContext(ctx, commit)
	})
}

func (m *videoStudioManager) finish(id, status, code, message string, artifact *videoStudioArtifact, result map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil {
		return
	}
	if job.Status == "cancelled" {
		return
	}
	job.Status, job.Error, job.Message = status, code, message
	if status == "succeeded" {
		job.ExternalStatusUnknown = false
	}
	job.Artifact, job.Result = artifact, result
	if status == "succeeded" {
		job.Progress = 1
	}
	finished := time.Now().UTC()
	job.FinishedAt = &finished
	delete(m.works, id)
	_ = m.persistLocked()
	trimmed := m.trimJobsLocked()
	if trimmed {
		_ = m.persistLocked()
	}
}

func (m *videoStudioManager) markExternalStatusUnknownForWork(id string, work *videoStudioWork) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if m.closed || m.configChanging || m.server.videoStudioConfigRevoking.Load() || m.ctx.Err() != nil || job == nil || job.Status != "running" || work == nil ||
		m.epoch != work.admittedEpoch || videoStudioConfigRootsChanged(work.admittedConfig, m.server.ConfigSnapshot()) {
		return context.Canceled
	}
	job.ExternalStatusUnknown = true
	return m.persistLocked()
}

func (m *videoStudioManager) markExternalStatusResolved(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil || job.Status != "running" {
		return context.Canceled
	}
	job.ExternalStatusUnknown = false
	return m.persistLocked()
}

func (m *videoStudioManager) updateProgress(id string, progress float64) {
	if progress != progress {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 0.99 {
		progress = 0.99
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if m.closed || job == nil || job.Status != "running" || progress <= job.Progress {
		return
	}
	previous := job.Progress
	job.Progress = progress
	if progress-previous < 0.02 && time.Since(job.progressPersistedAt) < 5*time.Second {
		return
	}
	if err := m.persistLocked(); err == nil {
		job.progressPersistedAt = time.Now()
	}
}

func (m *videoStudioManager) trimJobsLocked() bool {
	if len(m.jobs) <= videoStudioJobRetention {
		return false
	}
	ordered := make([]*videoStudioJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		ordered = append(ordered, job)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CreatedAt.Before(ordered[j].CreatedAt) })
	remove := len(ordered) - videoStudioJobRetention
	for _, job := range ordered[:remove] {
		if job.Status == "queued" || job.Status == "running" {
			continue
		}
		delete(m.jobs, job.ID)
		for key, item := range m.idem {
			if item.JobID == job.ID {
				delete(m.idem, key)
			}
		}
		_ = os.Remove(filepath.Join(m.previewDir, job.ProjectID, job.ID+".mp4"))
	}
	if len(m.idem) > videoStudioIdemRetention {
		for key, item := range m.idem {
			if m.jobs[item.JobID] == nil {
				delete(m.idem, key)
			}
		}
	}
	return true
}

func (m *videoStudioManager) job(id string) *videoStudioJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job := m.jobs[id]; job != nil {
		copyJob := *job
		if job.Artifact != nil {
			artifact := *job.Artifact
			artifact.privatePath = ""
			copyJob.Artifact = &artifact
		}
		return &copyJob
	}
	return nil
}

func (m *videoStudioManager) pendingProbeJob(projectID, assetID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, work := range m.works {
		if work != nil && work.kind == "probe" && work.projectID == projectID && work.assetID == assetID {
			if job := m.jobs[id]; job != nil && (job.Status == "queued" || job.Status == "running") {
				return id
			}
		}
	}
	return ""
}

func (m *videoStudioManager) jobAsset(jobID string) *videostudio.Asset {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[jobID]
	if job == nil || job.Result == nil {
		return nil
	}
	value, ok := job.Result["asset"]
	if !ok {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var asset videostudio.Asset
	if json.Unmarshal(data, &asset) != nil || asset.ID == "" {
		return nil
	}
	return &asset
}

func (m *videoStudioManager) jobsForProject(projectID string) []*videoStudioJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*videoStudioJob
	for _, job := range m.jobs {
		if job.ProjectID == projectID {
			copyJob := *job
			if job.Artifact != nil {
				artifact := *job.Artifact
				artifact.privatePath = ""
				copyJob.Artifact = &artifact
			}
			out = append(out, &copyJob)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (m *videoStudioManager) previewJobIDs(projectID, assetID, exceptID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for _, job := range m.jobs {
		if job.ID == exceptID || job.ProjectID != projectID || job.Kind != "preview" || job.Status != "succeeded" || job.Artifact == nil || job.Result == nil {
			continue
		}
		if value, ok := job.Result["asset_id"].(string); ok && value == assetID {
			ids = append(ids, job.ID)
		}
	}
	return ids
}

func (m *videoStudioManager) projectStorageUsage(ctx context.Context, svc *desktop.Service, projectID string) (int64, int64, error) {
	desktopBytes, err := svc.GetDirectorySize(ctx, videoStudioProjectPath(projectID))
	if err != nil {
		return 0, 0, err
	}
	previewBytes, err := videoStudioPrivateTreeSize(filepath.Join(m.previewDir, projectID))
	if err != nil {
		return 0, 0, err
	}
	if previewBytes > int64(^uint64(0)>>1)-desktopBytes {
		return 0, 0, fmt.Errorf("project storage size overflows")
	}
	return desktopBytes, previewBytes, nil
}

func (m *videoStudioManager) projectStorageSize(ctx context.Context, svc *desktop.Service, projectID string) (int64, error) {
	desktopBytes, previewBytes, err := m.projectStorageUsage(ctx, svc, projectID)
	if err != nil {
		return 0, err
	}
	return desktopBytes + previewBytes, nil
}

func (m *videoStudioManager) expirePreviewJobs(projectID, assetID string, ids []string) {
	if len(ids) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		job := m.jobs[id]
		if job == nil || job.ProjectID != projectID || job.Kind != "preview" || job.Status != "succeeded" || job.Result == nil {
			continue
		}
		if value, ok := job.Result["asset_id"].(string); !ok || value != assetID {
			continue
		}
		job.Artifact = nil
		job.Message = "This cached preview was replaced by a newer preview for the same asset."
	}
	_ = m.persistLocked()
}

func (m *videoStudioManager) mediaForProject(projectID string) []videoStudioMediaRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []videoStudioMediaRecord
	for _, record := range m.media[projectID] {
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (m *videoStudioManager) removeMedia(projectID, assetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.media[projectID]
	previous, exists := items[assetID]
	if !exists {
		return nil
	}
	delete(items, assetID)
	if len(items) == 0 {
		delete(m.media, projectID)
	}
	if err := m.persistLocked(); err != nil {
		if items == nil {
			items = make(map[string]videoStudioMediaRecord)
		}
		items[assetID] = previous
		m.media[projectID] = items
		return err
	}
	return nil
}

func (m *videoStudioManager) removeProject(projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.media, projectID)
	_ = os.RemoveAll(filepath.Join(m.previewDir, projectID))
	_ = m.persistLocked()
}

func (m *videoStudioManager) cancelJob(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	job := m.jobs[id]
	if job == nil || job.Status == "succeeded" || job.Status == "failed" || job.Status == "cancelled" || job.Status == "interrupted" {
		return false
	}
	job.Status = "cancelled"
	job.Error = "cancelled"
	job.Message = "The job was cancelled."
	if job.Kind == "generate" && job.ExternalStatusUnknown {
		job.Message = "Polling stopped, but the provider may still complete this request. Check provider status before starting another paid request."
	}
	finished := time.Now().UTC()
	job.FinishedAt = &finished
	delete(m.works, id)
	if cancel := m.active[id]; cancel != nil {
		cancel()
	}
	_ = m.persistLocked()
	return true
}

func (m *videoStudioManager) cancelProject(projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, job := range m.jobs {
		if job.ProjectID != projectID || job.Status == "succeeded" || job.Status == "failed" || job.Status == "cancelled" || job.Status == "interrupted" {
			continue
		}
		job.Status, job.Error, job.Message = "cancelled", "project_deleted", "The project was deleted."
		if job.Kind == "generate" && job.ExternalStatusUnknown {
			job.Message = "The project was deleted and polling stopped, but the provider may still complete this request."
		}
		finished := time.Now().UTC()
		job.FinishedAt = &finished
		delete(m.works, id)
		if cancel := m.active[id]; cancel != nil {
			cancel()
		}
	}
	_ = m.persistLocked()
}

func (m *videoStudioManager) cancelAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.configChanging = true
	m.epoch++
	for id, job := range m.jobs {
		if job.Status != "queued" && job.Status != "running" {
			continue
		}
		job.Status, job.Error, job.Message = "cancelled", "configuration_changed", "Video Studio or Desktop permissions changed."
		if job.Kind == "generate" && job.ExternalStatusUnknown {
			job.Message = "Configuration changed and polling stopped, but the provider may still complete this request. Check provider status before retrying."
		}
		finished := time.Now().UTC()
		job.FinishedAt = &finished
		delete(m.works, id)
		if cancel := m.active[id]; cancel != nil {
			cancel()
		}
	}
	_ = m.persistLocked()
	m.mu.Unlock()
}

func (m *videoStudioManager) close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.epoch++
	for id, job := range m.jobs {
		if job.Status != "queued" && job.Status != "running" {
			continue
		}
		job.Status, job.Error, job.Message = "cancelled", "server_shutdown", "The server is shutting down."
		if job.Kind == "generate" && job.ExternalStatusUnknown {
			job.Message = "The server stopped polling, but the provider may still complete this request. Check provider status before retrying."
		}
		finished := time.Now().UTC()
		job.FinishedAt = &finished
		delete(m.works, id)
	}
	for _, cancel := range m.active {
		cancel()
	}
	m.cancel()
	_ = m.persistLocked()
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		if m.server != nil && m.server.Logger != nil {
			m.server.Logger.Warn("video studio worker did not stop before shutdown timeout")
		}
	}
}

func videoStudioJobFailure(kind string, err error, ctx context.Context) (string, string, string) {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return "cancelled", "cancelled", "The job was cancelled or revoked."
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "failed", "timeout", "The job exceeded its configured time limit."
	}
	if errors.Is(err, errVideoStudioProjectSizeLimit) {
		return "failed", "project_size_limit", "The project storage limit has been reached. Free space in this project before continuing."
	}
	if errors.Is(err, errVideoStudioAssetSizeLimit) {
		return "failed", "asset_size_limit", "The generated video exceeds the configured per-asset size limit."
	}
	switch kind {
	case "probe":
		return "failed", "probe_failed", "Media inspection failed. Check that the file is supported and FFmpeg is configured."
	case "preview":
		return "failed", "preview_failed", "A preview could not be created."
	case "render":
		return "failed", "render_failed", "The project could not be rendered."
	case "generate":
		if errors.Is(err, errVideoStudioGenerationImport) {
			return "failed", "generation_import_failed", "The provider returned a video, but AuraGo could not import it into this project."
		}
		return "failed", "generation_failed", "The configured video provider could not complete this request."
	default:
		return "failed", "job_failed", "The video job failed."
	}
}

func videoStudioHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func videoStudioIdemMapKey(projectID, keyHash string) string { return projectID + ":" + keyHash }

func uidNew() string { return uid.New() }

func validVideoStudioProjectID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for i, r := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func validVideoStudioAssetID(value string) bool { return validVideoStudioProjectID(value) }

func videoStudioMediaPathValid(value, assetID string) bool {
	if !validVideoStudioAssetID(assetID) || !strings.HasPrefix(value, "media/"+assetID+"_") {
		return false
	}
	base := strings.TrimPrefix(value, "media/")
	if strings.ContainsAny(base, `/\\`) || strings.Contains(base, "..") {
		return false
	}
	return len(base) <= 180
}

func safeVideoStudioFilename(raw string) string {
	base := filepath.Base(strings.ReplaceAll(strings.TrimSpace(raw), `\\`, "/"))
	if base == "." || base == "/" || base == "" {
		base = "media"
	}
	var out strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			out.WriteRune(r)
		} else {
			out.WriteByte('_')
		}
		if out.Len() >= 120 {
			break
		}
	}
	value := strings.Trim(out.String(), "._-")
	if value == "" {
		value = "media"
	}
	if ext := filepath.Ext(value); len(ext) > 12 {
		value = strings.TrimSuffix(value, ext)
	}
	return value
}

func videoStudioFileVersion(data []byte) string { return desktop.NoteVersion(data) }

func videoStudioProjectPath(id string) string { return "Documents/Video Studio/" + id }

func readVideoStudioProject(ctx context.Context, svc *desktop.Service, projectPath string) (videostudio.Project, []byte, string, error) {
	file, entry, _, err := svc.OpenPreviewFile(ctx, projectPath+"/project.json")
	if err != nil {
		return videostudio.Project{}, nil, "", err
	}
	defer file.Close()
	const projectJSONMaxBytes = 8 << 20
	if entry.Size > projectJSONMaxBytes {
		return videostudio.Project{}, nil, "", fmt.Errorf("project file exceeds the JSON size limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, projectJSONMaxBytes+1))
	if err != nil {
		return videostudio.Project{}, nil, "", err
	}
	if len(data) > projectJSONMaxBytes {
		return videostudio.Project{}, nil, "", fmt.Errorf("project file exceeds the JSON size limit")
	}
	var project videostudio.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return videostudio.Project{}, nil, "", fmt.Errorf("decode project: %w", err)
	}
	return project, data, desktop.NoteVersion(data), nil
}

func videoStudioLog(logger *slog.Logger, message string, attrs ...any) {
	if logger != nil {
		logger.Error(message, attrs...)
	}
}
