package gamemaker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var slugPartPattern = regexp.MustCompile(`[^a-z0-9]+`)

type previewToken struct {
	ProjectID string
	JobID     string
	ExpiresAt time.Time
}

// Service owns Game Maker Studio persistence and job publication.
type Service struct {
	db         *sql.DB
	opts       Options
	stagingDir string
	blobDir    string

	mu          sync.RWMutex
	runner      Runner
	activeJobID string
	jobCancels  map[string]context.CancelFunc
	subscribers map[string]map[chan Event]struct{}
	tokens      map[string]previewToken
	previewJobs map[string]string
	skills      []SkillInfo
	skillsReady bool
}

var (
	defaultServiceMu sync.RWMutex
	defaultService   *Service
)

func SetDefaultService(service *Service) {
	defaultServiceMu.Lock()
	defer defaultServiceMu.Unlock()
	defaultService = service
}

func DefaultService() *Service {
	defaultServiceMu.RLock()
	defer defaultServiceMu.RUnlock()
	return defaultService
}

func NewService(opts Options) (*Service, error) {
	if strings.TrimSpace(opts.DBPath) == "" || strings.TrimSpace(opts.WorkspacePath) == "" {
		return nil, fmt.Errorf("game maker database and workspace paths are required")
	}
	if opts.MaxProjects <= 0 {
		opts.MaxProjects = 25
	}
	if opts.MaxFilesPerProject <= 0 {
		opts.MaxFilesPerProject = 250
	}
	if opts.MaxFileBytes <= 0 {
		opts.MaxFileBytes = 2 * 1024 * 1024
	}
	if opts.MaxProjectBytes <= 0 {
		opts.MaxProjectBytes = 100 * 1024 * 1024
	}
	if opts.JobTimeout <= 0 {
		opts.JobTimeout = 30 * time.Minute
	}
	staging, blobs, err := ensureStorageDirs(opts)
	if err != nil {
		return nil, err
	}
	db, err := openDatabase(opts.DBPath)
	if err != nil {
		return nil, err
	}
	service := &Service{
		db:          db,
		opts:        opts,
		stagingDir:  staging,
		blobDir:     blobs,
		jobCancels:  map[string]context.CancelFunc{},
		subscribers: map[string]map[chan Event]struct{}{},
		tokens:      map[string]previewToken{},
		previewJobs: map[string]string{},
	}
	return service, nil
}

func (s *Service) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.mu.Lock()
	for _, cancel := range s.jobCancels {
		cancel()
	}
	s.jobCancels = map[string]context.CancelFunc{}
	s.mu.Unlock()
	return s.db.Close()
}

func (s *Service) SetRunner(runner Runner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runner = runner
}

func (s *Service) SetSkillStatus(skills []SkillInfo, ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skills = append([]SkillInfo(nil), skills...)
	s.skillsReady = ready
}

func (s *Service) SkillStatus() ([]SkillInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]SkillInfo(nil), s.skills...), s.skillsReady
}

func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,slug,project_key,dimension,description,
		provider_id,model,use_image,use_music,status,current_revision,created_at,updated_at
		FROM gm_projects ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list game maker projects: %w", err)
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan game maker project: %w", err)
		}
		out = append(out, project)
	}
	return out, rows.Err()
}

func (s *Service) GetProject(ctx context.Context, id string) (Project, error) {
	project, err := scanProject(s.db.QueryRowContext(ctx, `SELECT id,name,slug,project_key,dimension,description,
		provider_id,model,use_image,use_music,status,current_revision,created_at,updated_at
		FROM gm_projects WHERE id=?`, strings.TrimSpace(id)))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("get game maker project: %w", err)
	}
	return project, nil
}

func (s *Service) CreateProject(ctx context.Context, req CreateProjectRequest) (Project, error) {
	if !s.opts.Enabled {
		return Project{}, ErrDisabled
	}
	if s.opts.ReadOnly || !s.opts.AllowCreate {
		return Project{}, ErrReadOnly
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Dimension = strings.ToLower(strings.TrimSpace(req.Dimension))
	if req.Name == "" || req.Description == "" {
		return Project{}, fmt.Errorf("project name and description are required")
	}
	if req.Dimension != "2d" && req.Dimension != "3d" {
		return Project{}, fmt.Errorf("dimension must be 2d or 3d")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gm_projects`).Scan(&count); err != nil {
		return Project{}, fmt.Errorf("count game maker projects: %w", err)
	}
	if count >= s.opts.MaxProjects {
		return Project{}, fmt.Errorf("game maker project limit reached")
	}
	slug, err := s.availableSlug(ctx, req.Name)
	if err != nil {
		return Project{}, err
	}
	now := time.Now().UTC()
	project := Project{
		ID:                 randomID("project"),
		Name:               req.Name,
		Slug:               slug,
		ProjectKey:         projectKey(slug),
		Dimension:          req.Dimension,
		Description:        req.Description,
		ProviderID:         strings.TrimSpace(req.ProviderID),
		Model:              strings.TrimSpace(req.Model),
		UseImageGeneration: req.UseImageGeneration && s.opts.AllowMediaGeneration,
		UseMusicGeneration: req.UseMusicGeneration && s.opts.AllowMediaGeneration,
		Status:             "draft",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO gm_projects
		(id,name,slug,project_key,dimension,description,provider_id,model,use_image,use_music,status,current_revision,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		project.ID, project.Name, project.Slug, project.ProjectKey, project.Dimension, project.Description,
		project.ProviderID, project.Model, boolInt(project.UseImageGeneration), boolInt(project.UseMusicGeneration),
		project.Status, 0, now, now)
	if err != nil {
		return Project{}, fmt.Errorf("create game maker project: %w", err)
	}
	if _, err := s.appendMessage(ctx, project.ID, "", "user", project.Description); err != nil {
		return Project{}, err
	}
	_, _ = s.emit(ctx, project.ID, "", "project_created", map[string]any{"project": project})
	return project, nil
}

func (s *Service) UpdateProject(ctx context.Context, id string, req UpdateProjectRequest) (Project, error) {
	if !s.opts.Enabled {
		return Project{}, ErrDisabled
	}
	if s.opts.ReadOnly || !s.opts.AllowEdit {
		return Project{}, ErrReadOnly
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Project{}, fmt.Errorf("project name is required")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE gm_projects SET name=?,updated_at=? WHERE id=?`, name, time.Now().UTC(), id)
	if err != nil {
		return Project{}, fmt.Errorf("rename game maker project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Project{}, ErrNotFound
	}
	project, err := s.GetProject(ctx, id)
	if err == nil {
		_, _ = s.emit(ctx, id, "", "project_updated", map[string]any{"name": name})
	}
	return project, err
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	if !s.opts.Enabled {
		return ErrDisabled
	}
	if s.opts.ReadOnly || !s.opts.AllowDelete {
		return ErrReadOnly
	}
	project, err := s.GetProject(ctx, id)
	if err != nil {
		return err
	}
	s.mu.RLock()
	for jobID := range s.jobCancels {
		job, getErr := s.GetJob(ctx, jobID)
		if getErr == nil && job.ProjectID == id {
			s.mu.RUnlock()
			return ErrBusy
		}
	}
	s.mu.RUnlock()
	projectDir := filepath.Join(s.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	if err := rejectSymlinkComponents(s.opts.WorkspacePath, projectDir); err != nil {
		return err
	}
	if err := os.RemoveAll(projectDir); err != nil {
		return fmt.Errorf("remove game maker project files: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM gm_projects WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete game maker project: %w", err)
	}
	return nil
}

func (s *Service) StartJob(ctx context.Context, projectID string, req StartJobRequest) (Job, error) {
	if !s.opts.Enabled {
		return Job{}, ErrDisabled
	}
	if s.opts.ReadOnly || !s.opts.AllowEdit {
		return Job{}, ErrReadOnly
	}
	_, ready := s.SkillStatus()
	if !ready {
		return Job{}, ErrSkillsUnusable
	}
	project, err := s.GetProject(ctx, projectID)
	if err != nil {
		return Job{}, err
	}
	imageGeneration := project.UseImageGeneration
	musicGeneration := project.UseMusicGeneration
	if req.ImageGeneration != nil {
		imageGeneration = *req.ImageGeneration && s.opts.AllowMediaGeneration
	}
	if req.MusicGeneration != nil {
		musicGeneration = *req.MusicGeneration && s.opts.AllowMediaGeneration
	}
	prompt := strings.TrimSpace(req.Prompt)
	kind := "create"
	if project.CurrentRevision > 0 {
		kind = "edit"
		if prompt == "" {
			return Job{}, fmt.Errorf("change request is required")
		}
	} else if prompt == "" {
		prompt = project.Description
	}
	providerID := firstNonEmpty(req.ProviderID, project.ProviderID)
	model := firstNonEmpty(req.Model, project.Model)
	project.ProviderID = providerID
	project.Model = model
	project.UseImageGeneration = imageGeneration
	project.UseMusicGeneration = musicGeneration
	if _, err := s.db.ExecContext(ctx, `UPDATE gm_projects
		SET provider_id=?,model=?,use_image=?,use_music=?,updated_at=? WHERE id=?`,
		providerID, model, boolInt(imageGeneration), boolInt(musicGeneration), time.Now().UTC(), project.ID); err != nil {
		return Job{}, fmt.Errorf("update game maker job preferences: %w", err)
	}
	now := time.Now().UTC()
	job := Job{
		ID:           randomID("job"),
		ProjectID:    project.ID,
		Kind:         kind,
		Prompt:       prompt,
		Status:       "queued",
		Phase:        "queued",
		ProviderID:   providerID,
		Model:        model,
		BaseRevision: project.CurrentRevision,
		CreatedAt:    now,
	}

	s.mu.Lock()
	if s.activeJobID != "" {
		s.mu.Unlock()
		return Job{}, ErrBusy
	}
	jobCtx, cancel := context.WithTimeout(context.Background(), s.opts.JobTimeout)
	s.activeJobID = job.ID
	s.jobCancels[job.ID] = cancel
	s.mu.Unlock()

	_, err = s.db.ExecContext(ctx, `INSERT INTO gm_jobs
		(id,project_id,kind,prompt,status,phase,provider_id,model,base_revision,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, job.ID, job.ProjectID, job.Kind, job.Prompt, job.Status, job.Phase,
		job.ProviderID, job.Model, job.BaseRevision, job.CreatedAt)
	if err != nil {
		s.releaseJob(job.ID)
		return Job{}, fmt.Errorf("create game maker job: %w", err)
	}
	_, _ = s.appendMessage(ctx, project.ID, job.ID, "user", prompt)
	_, _ = s.emit(ctx, project.ID, job.ID, "job_status", map[string]any{"status": "queued", "job": job})
	go s.executeJob(jobCtx, job, project)
	return job, nil
}

func (s *Service) executeJob(ctx context.Context, job Job, project Project) {
	defer s.releaseJob(job.ID)
	stage := filepath.Join(s.stagingDir, job.ID)
	_ = os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0o750); err != nil {
		s.terminateJob(job, ctx, err)
		return
	}
	defer os.RemoveAll(stage)

	if err := s.updateJobPhase(ctx, &job, "planning"); err != nil {
		s.terminateJob(job, ctx, err)
		return
	}
	if job.BaseRevision > 0 {
		if err := s.copyPublishedProject(project, stage); err != nil {
			s.terminateJob(job, ctx, err)
			return
		}
	} else if err := WriteScaffold(stage, project); err != nil {
		s.terminateJob(job, ctx, err)
		return
	}
	if err := s.updateJobPhase(ctx, &job, "building"); err != nil {
		s.terminateJob(job, ctx, err)
		return
	}
	s.mu.RLock()
	runner := s.runner
	s.mu.RUnlock()
	if runner == nil {
		s.terminateJob(job, ctx, fmt.Errorf("game maker agent runner is unavailable"))
		return
	}
	if err := runner.RunGameMakerJob(ctx, JobRun{Job: job, Project: project}); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			s.cancelledJob(job, ctx.Err())
			return
		}
		s.terminateJob(job, ctx, err)
		return
	}
	if err := s.updateJobPhase(ctx, &job, "validating"); err != nil {
		s.terminateJob(job, ctx, err)
		return
	}
	result := s.BuildJob(ctx, job.ID)
	if !result.OK {
		s.terminateJob(job, ctx, fmt.Errorf("game validation failed: %s", diagnosticsText(result.Diagnostics)))
		return
	}
	if err := s.updateJobPhase(ctx, &job, "polishing"); err != nil {
		s.terminateJob(job, ctx, err)
		return
	}
	revision, err := s.publish(stage, project, job, "agent", job.Prompt)
	if err != nil {
		s.terminateJob(job, ctx, err)
		return
	}
	now := time.Now().UTC()
	job.Status = "ready"
	job.Phase = "ready"
	job.ResultRevision = revision.Number
	job.FinishedAt = &now
	s.mu.Lock()
	_, _ = s.db.Exec(`UPDATE gm_jobs SET status='ready',phase='ready',result_revision=?,finished_at=? WHERE id=?`,
		revision.Number, now, job.ID)
	s.releaseJobLocked(job.ID)
	s.mu.Unlock()
	_, _ = s.appendMessage(context.Background(), project.ID, job.ID, "assistant",
		fmt.Sprintf("Playable revision %d is ready.", revision.Number))
	_, _ = s.emit(context.Background(), project.ID, job.ID, "preview_reload",
		map[string]any{"revision": revision.Number})
	_, _ = s.emit(context.Background(), project.ID, job.ID, "revision",
		map[string]any{"revision": revision})
	_, _ = s.emit(context.Background(), project.ID, job.ID, "job_status",
		map[string]any{"status": "ready", "job": job})
}

func (s *Service) CancelJob(ctx context.Context, id string) error {
	s.mu.RLock()
	cancel := s.jobCancels[id]
	s.mu.RUnlock()
	if cancel == nil {
		return ErrNotFound
	}
	cancel()
	return nil
}

func (s *Service) GetJob(ctx context.Context, id string) (Job, error) {
	job, err := scanJob(s.db.QueryRowContext(ctx, `SELECT id,project_id,kind,prompt,status,phase,
		provider_id,model,error,base_revision,result_revision,created_at,started_at,finished_at
		FROM gm_jobs WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("get game maker job: %w", err)
	}
	return job, nil
}

func (s *Service) updateJobPhase(ctx context.Context, job *Job, phase string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now := time.Now().UTC()
	job.Status = phase
	job.Phase = phase
	if job.StartedAt == nil {
		job.StartedAt = &now
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE gm_jobs SET status=?,phase=?,started_at=COALESCE(started_at,?) WHERE id=?`,
		phase, phase, now, job.ID); err != nil {
		return fmt.Errorf("update game maker job phase: %w", err)
	}
	_, _ = s.emit(ctx, job.ProjectID, job.ID, "phase", map[string]any{"phase": phase})
	return nil
}

func (s *Service) failJob(job Job, failure error) {
	now := time.Now().UTC()
	message := strings.TrimSpace(failure.Error())
	s.mu.Lock()
	_, _ = s.db.Exec(`UPDATE gm_jobs SET status='failed',phase='failed',error=?,finished_at=? WHERE id=?`,
		message, now, job.ID)
	_, _ = s.db.Exec(`UPDATE gm_projects SET status='failed',updated_at=? WHERE id=?`, now, job.ProjectID)
	s.releaseJobLocked(job.ID)
	s.mu.Unlock()
	_, _ = s.emit(context.Background(), job.ProjectID, job.ID, "diagnostic",
		map[string]any{"level": "error", "message": message})
	_, _ = s.emit(context.Background(), job.ProjectID, job.ID, "job_status",
		map[string]any{"status": "failed", "error": message})
}

func (s *Service) terminateJob(job Job, ctx context.Context, failure error) {
	if ctx != nil && (errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		s.cancelledJob(job, ctx.Err())
		return
	}
	s.failJob(job, failure)
}

func (s *Service) cancelledJob(job Job, cause error) {
	now := time.Now().UTC()
	message := "Game creation was cancelled."
	if errors.Is(cause, context.DeadlineExceeded) {
		message = "Game creation exceeded its time limit."
	}
	s.mu.Lock()
	_, _ = s.db.Exec(`UPDATE gm_jobs SET status='cancelled',phase='cancelled',error=?,finished_at=? WHERE id=?`,
		message, now, job.ID)
	s.releaseJobLocked(job.ID)
	s.mu.Unlock()
	_, _ = s.emit(context.Background(), job.ProjectID, job.ID, "job_status",
		map[string]any{"status": "cancelled", "error": message})
}

func (s *Service) releaseJob(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseJobLocked(id)
}

func (s *Service) releaseJobLocked(id string) {
	if cancel := s.jobCancels[id]; cancel != nil {
		cancel()
		delete(s.jobCancels, id)
		for projectID, jobID := range s.previewJobs {
			if jobID == id {
				delete(s.previewJobs, projectID)
			}
		}
	}
	if s.activeJobID == id {
		s.activeJobID = ""
	}
}

func (s *Service) JobDirectory(jobID string) (string, error) {
	job, err := s.GetJob(context.Background(), jobID)
	if err != nil {
		return "", err
	}
	s.mu.RLock()
	_, active := s.jobCancels[jobID]
	s.mu.RUnlock()
	if !active {
		return "", fmt.Errorf("game maker job is not active")
	}
	stage := filepath.Join(s.stagingDir, job.ID)
	if err := rejectSymlinkComponents(s.stagingDir, stage); err != nil {
		return "", err
	}
	return stage, nil
}

func (s *Service) ReadJobFile(ctx context.Context, jobID, rawPath string) (string, error) {
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return "", err
	}
	path, _, err := secureJoin(stage, rawPath, true)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read game maker file: %w", err)
	}
	if int64(len(data)) > s.opts.MaxFileBytes {
		return "", fmt.Errorf("game maker file exceeds configured limit")
	}
	return string(data), nil
}

func (s *Service) WriteJobFile(ctx context.Context, jobID, rawPath, content string) error {
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return err
	}
	path, rel, err := secureJoin(stage, rawPath, false)
	if err != nil {
		return err
	}
	if int64(len(content)) > s.opts.MaxFileBytes {
		return fmt.Errorf("game maker file exceeds configured limit")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create game maker file directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gm-write-*")
	if err != nil {
		return fmt.Errorf("create game maker temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = io.WriteString(tmp, content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write game maker temporary file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync game maker temporary file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close game maker temporary file: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("publish game maker file: %w", err)
	}
	job, _ := s.GetJob(ctx, jobID)
	_, _ = s.emit(ctx, job.ProjectID, jobID, "file_changed", map[string]any{"path": rel})
	return nil
}

func (s *Service) StoreJobAsset(ctx context.Context, jobID, rawPath, kind, generator, provenance string, data []byte) (string, error) {
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return "", err
	}
	rel, err := safeRelativePath(rawPath, false)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(strings.ToLower(rel), "assets/") {
		return "", fmt.Errorf("%w: generated assets must be stored under assets/", ErrInvalidPath)
	}
	if int64(len(data)) > s.opts.MaxFileBytes {
		return "", fmt.Errorf("game maker asset exceeds configured file limit")
	}
	path, _, err := secureJoin(stage, rel, false)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("create game maker asset directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gm-asset-*")
	if err != nil {
		return "", fmt.Errorf("create game maker asset temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write game maker asset: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close game maker asset: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("publish game maker asset: %w", err)
	}
	sum := sha256Bytes(data)
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO gm_assets(project_id,job_id,path,kind,generator,provenance,content_hash,created_at)
		VALUES(?,?,?,?,?,?,?,?)`, job.ProjectID, jobID, rel, kind, generator, provenance, sum, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("record game maker asset: %w", err)
	}
	_, _ = s.emit(ctx, job.ProjectID, jobID, "asset_changed", map[string]any{
		"path": rel, "kind": kind, "generator": generator, "provenance": provenance,
	})
	return rel, nil
}

func (s *Service) ProjectForJob(ctx context.Context, jobID string) (Project, Job, error) {
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return Project{}, Job{}, err
	}
	project, err := s.GetProject(ctx, job.ProjectID)
	return project, job, err
}

func (s *Service) ListJobFiles(ctx context.Context, jobID string) ([]string, error) {
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return nil, err
	}
	var paths []string
	err = filepath.WalkDir(stage, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		if len(paths) > s.opts.MaxFilesPerProject {
			return fmt.Errorf("game maker file count exceeds configured limit")
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

func (s *Service) appendMessage(ctx context.Context, projectID, jobID, role, content string) (Message, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `INSERT INTO gm_messages(project_id,job_id,role,content,created_at)
		VALUES(?,?,?,?,?)`, projectID, jobID, role, content, now)
	if err != nil {
		return Message{}, fmt.Errorf("append game maker message: %w", err)
	}
	id, _ := result.LastInsertId()
	return Message{ID: id, ProjectID: projectID, JobID: jobID, Role: role, Content: content, CreatedAt: now}, nil
}

func (s *Service) ListMessages(ctx context.Context, projectID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,project_id,job_id,role,content,created_at
		FROM gm_messages WHERE project_id=? ORDER BY id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list game maker messages: %w", err)
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.ProjectID, &message.JobID, &message.Role, &message.Content, &message.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, message)
	}
	return out, rows.Err()
}

func (s *Service) RecordAgentMessage(ctx context.Context, projectID, jobID, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	_, err := s.appendMessage(ctx, projectID, jobID, "assistant", content)
	return err
}

func (s *Service) availableSlug(ctx context.Context, name string) (string, error) {
	base := strings.Trim(slugPartPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "game"
	}
	if len(base) > 48 {
		base = strings.Trim(base[:48], "-")
	}
	for i := 0; i < 1000; i++ {
		slug := base
		if i > 0 {
			slug = fmt.Sprintf("%s-%d", base, i+1)
		}
		var exists int
		err := s.db.QueryRowContext(ctx, `SELECT 1 FROM gm_projects WHERE slug=?`, slug).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return slug, nil
		}
		if err != nil {
			return "", fmt.Errorf("check game maker slug: %w", err)
		}
	}
	return "", fmt.Errorf("could not allocate project slug")
}

func (s *Service) copyPublishedProject(project Project, stage string) error {
	source := filepath.Join(s.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	return copyTree(source, stage, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes)
}

func copyTree(source, destination string, maxFiles int, maxBytes int64) error {
	var files int
	var total int64
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink in project", ErrInvalidPath)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		files++
		total += info.Size()
		if files > maxFiles || total > maxBytes {
			return fmt.Errorf("game maker project exceeds configured limits")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o640)
	})
}

func randomID(prefix string) string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(raw[:])
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func diagnosticsText(diagnostics []Diagnostic) string {
	var parts []string
	for _, diagnostic := range diagnostics {
		parts = append(parts, diagnostic.Message)
	}
	return strings.Join(parts, "; ")
}

func eventJSON(payload map[string]any) string {
	if payload == nil {
		payload = map[string]any{}
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func sha256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
