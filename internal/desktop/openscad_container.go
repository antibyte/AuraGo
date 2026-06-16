package desktop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	openSCADContainerName                       = "aurago-openscad"
	defaultOpenSCADImage                        = "openscad/openscad:latest"
	openSCADJobsInContainer                     = "/jobs"
	defaultOpenSCADMemoryMB                     = 2048
	defaultOpenSCADCPUCores                     = 2
	defaultOpenSCADPidsLimit                    = 512
	defaultOpenSCADAutoStop                     = 20 * time.Minute
	defaultOpenSCADRenderTimeout                = 120 * time.Second
	defaultOpenSCADMaxRenderTimeout             = 600 * time.Second
	defaultOpenSCADMaxSourceKB                  = 512
	defaultOpenSCADMaxOutputMB                  = 100
	defaultOpenSCADJobRetentionDays             = 7
	openSCADJobsRootMode            os.FileMode = 0o755
	// The official OpenSCAD image may execute as a UID different from AuraGo's host UID.
	openSCADJobDirMode     os.FileMode = 0o1777
	openSCADSourceFileMode os.FileMode = 0o644
)

var (
	openSCADDefineNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	openSCADModelNamePattern  = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	openSCADExportSet         = map[string]struct{}{
		"png": {}, "stl": {}, "3mf": {}, "off": {}, "amf": {}, "dxf": {}, "svg": {}, "pdf": {}, "csg": {}, "echo": {},
	}
	openSCAD2DOnlyExportSet = map[string]struct{}{
		"dxf": {}, "svg": {}, "pdf": {},
	}
)

type OpenSCADContainerService struct {
	cfg           Config
	logger        *slog.Logger
	docker        CodeContainerDocker
	mu            sync.RWMutex
	renderMu      sync.Mutex
	state         ContainerState
	lastActivity  time.Time
	stopTimer     *time.Timer
	autoStopAfter time.Duration
}

type OpenSCADDefine struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type OpenSCADRenderRequest struct {
	SourceSCAD     string           `json:"source_scad"`
	ModelName      string           `json:"model_name,omitempty"`
	Exports        []string         `json:"exports,omitempty"`
	Defines        []OpenSCADDefine `json:"defines,omitempty"`
	RenderMode     string           `json:"render_mode,omitempty"`
	TimeoutSeconds int              `json:"timeout_seconds,omitempty"`
	SaveToDesktop  bool             `json:"save_to_desktop,omitempty"`
}

type OpenSCADFile struct {
	Name        string `json:"name"`
	Format      string `json:"format"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	DownloadURL string `json:"download_url"`
	PreviewURL  string `json:"preview_url"`
	SavedPath   string `json:"saved_path,omitempty"`
}

type OpenSCADRenderResult struct {
	JobID        string         `json:"job_id"`
	ModelName    string         `json:"model_name"`
	Files        []OpenSCADFile `json:"files"`
	SourcePath   string         `json:"source_path"`
	SourceSCAD   string         `json:"source_scad,omitempty"`
	ExitCode     int            `json:"exit_code"`
	DurationMS   int64          `json:"duration_ms"`
	Stdout       string         `json:"stdout,omitempty"`
	Stderr       string         `json:"stderr,omitempty"`
	SavedPaths   []string       `json:"saved_paths,omitempty"`
	DownloadBase string         `json:"download_base"`
	CreatedAt    time.Time      `json:"created_at"`
}

type OpenSCADStatus struct {
	Enabled                 bool                   `json:"enabled"`
	State                   string                 `json:"state"`
	Running                 bool                   `json:"running"`
	ContainerID             string                 `json:"container_id,omitempty"`
	Image                   string                 `json:"image"`
	JobsHostPath            string                 `json:"jobs_host_path"`
	JobsContainerPath       string                 `json:"jobs_container_path"`
	AutoStopMinutes         int                    `json:"auto_stop_minutes"`
	MaxConcurrentJobs       int                    `json:"max_concurrent_jobs"`
	DefaultExports          []string               `json:"default_exports"`
	RenderTimeoutSeconds    int                    `json:"render_timeout_seconds"`
	MaxRenderTimeoutSeconds int                    `json:"max_render_timeout_seconds"`
	LastActivity            time.Time              `json:"last_activity,omitempty"`
	Resources               CodeContainerResources `json:"resources"`
	Error                   string                 `json:"error,omitempty"`
}

func NewOpenSCADContainerService(cfg Config, logger *slog.Logger) *OpenSCADContainerService {
	if logger == nil {
		logger = slog.Default()
	}
	return &OpenSCADContainerService{
		cfg:           cfg,
		logger:        logger,
		docker:        missingCodeContainerDocker{},
		state:         StateStopped,
		autoStopAfter: defaultOpenSCADAutoStop,
	}
}

func (s *OpenSCADContainerService) SetDockerClient(docker CodeContainerDocker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if docker == nil {
		s.docker = missingCodeContainerDocker{}
		return
	}
	s.docker = docker
}

func (s *OpenSCADContainerService) EnsureInstalled(ctx context.Context) error {
	return s.ensureStarted(ctx, false)
}

func (s *OpenSCADContainerService) EnsureStarted(ctx context.Context) error {
	return s.ensureStarted(ctx, true)
}

func (s *OpenSCADContainerService) ensureStarted(ctx context.Context, keepRunning bool) error {
	if !s.cfg.OpenSCAD.Enabled {
		return fmt.Errorf("openscad is disabled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateStarting {
		return fmt.Errorf("openscad container is already starting")
	}
	jobsRoot, err := s.ensureJobsRootLocked()
	if err != nil {
		s.state = StateError
		return err
	}
	containerID, running, err := s.findContainerLocked(ctx)
	if err != nil {
		s.state = StateError
		return err
	}
	if containerID != "" && !running {
		if err := s.docker.ContainerAction(ctx, containerID, "start"); err != nil {
			s.state = StateError
			return fmt.Errorf("start openscad container: %w", err)
		}
		if err := s.waitForRunningLocked(ctx, containerID); err != nil {
			s.state = StateError
			return err
		}
		running = true
	}
	if containerID == "" {
		s.state = StateStarting
		image := openSCADImage(s.cfg.OpenSCAD)
		if err := s.docker.EnsureImage(ctx, image); err != nil {
			s.state = StateError
			return fmt.Errorf("prepare openscad image %s: %w", image, err)
		}
		containerID, err = s.createOpenSCADContainerLocked(ctx, image, jobsRoot)
		if err != nil {
			s.state = StateError
			return err
		}
		running = true
	}
	if running {
		if err := s.probeRuntimeLocked(ctx, containerID); err != nil {
			s.state = StateError
			return err
		}
	}
	if !keepRunning {
		if err := s.docker.ContainerAction(ctx, containerID, "stop"); err != nil {
			s.state = StateError
			return fmt.Errorf("stop openscad container after install validation: %w", err)
		}
		s.state = StateStopped
		return nil
	}
	s.state = StateRunning
	s.touchLocked()
	return nil
}

func (s *OpenSCADContainerService) createOpenSCADContainerLocked(ctx context.Context, image, jobsRoot string) (string, error) {
	createdID, err := s.docker.CreateContainer(ctx, CodeDockerCreateRequest{
		Name:        openSCADContainerName,
		Image:       image,
		Ports:       map[string]string{},
		Volumes:     []string{jobsRoot + ":" + openSCADJobsInContainer},
		Cmd:         []string{"sleep", "infinity"},
		Restart:     "no",
		NetworkMode: "none",
		SecurityOpt: []string{"no-new-privileges:true"},
		CapDrop:     []string{"ALL"},
		Resources:   openSCADResourcesPtr(s.cfg.OpenSCAD),
	})
	if err != nil {
		return "", fmt.Errorf("create openscad container: %w", err)
	}
	if createdID == "" {
		createdID = openSCADContainerName
	}
	if err := s.docker.ContainerAction(ctx, createdID, "start"); err != nil {
		return "", fmt.Errorf("start openscad container: %w", err)
	}
	if err := s.waitForRunningLocked(ctx, createdID); err != nil {
		return "", err
	}
	return createdID, nil
}

func (s *OpenSCADContainerService) probeRuntimeLocked(ctx context.Context, containerID string) error {
	result, err := s.docker.ExecContainer(ctx, containerID, []string{"sh", "-lc", "command -v openscad >/dev/null 2>&1 && command -v xvfb-run >/dev/null 2>&1"}, "", 30*time.Second)
	if err != nil {
		return fmt.Errorf("check openscad runtime tools: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("openscad container is missing openscad or xvfb-run: %s", strings.TrimSpace(result.Output))
	}
	return nil
}

func (s *OpenSCADContainerService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopTimer != nil {
		s.stopTimer.Stop()
		s.stopTimer = nil
	}
	containerID, running, err := s.findContainerLocked(ctx)
	if err != nil {
		return err
	}
	if containerID != "" && running {
		if err := s.docker.ContainerAction(ctx, containerID, "stop"); err != nil {
			return fmt.Errorf("stop openscad container: %w", err)
		}
	}
	s.state = StateStopped
	return nil
}

func (s *OpenSCADContainerService) Remove(ctx context.Context, deleteData bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopTimer != nil {
		s.stopTimer.Stop()
		s.stopTimer = nil
	}
	containerID, _, err := s.findContainerLocked(ctx)
	if err != nil {
		return err
	}
	if containerID != "" {
		if err := s.docker.ContainerAction(ctx, containerID, "remove"); err != nil {
			return fmt.Errorf("remove openscad container: %w", err)
		}
	}
	if deleteData {
		if err := os.RemoveAll(s.jobsRootLocked()); err != nil {
			return fmt.Errorf("remove openscad jobs: %w", err)
		}
	}
	s.state = StateStopped
	return nil
}

func (s *OpenSCADContainerService) Render(ctx context.Context, req OpenSCADRenderRequest) (OpenSCADRenderResult, error) {
	if err := s.validateRenderRequest(&req); err != nil {
		return OpenSCADRenderResult{}, err
	}
	s.renderMu.Lock()
	defer s.renderMu.Unlock()
	if err := s.EnsureStarted(ctx); err != nil {
		return OpenSCADRenderResult{}, err
	}
	containerID, err := s.ContainerID(ctx)
	if err != nil {
		return OpenSCADRenderResult{}, err
	}
	jobsRoot, err := s.ensureJobsRoot()
	if err != nil {
		return OpenSCADRenderResult{}, err
	}
	jobID := newOpenSCADJobID()
	modelName := safeOpenSCADModelName(req.ModelName)
	jobDir := filepath.Join(jobsRoot, jobID)
	if err := os.MkdirAll(jobDir, openSCADJobDirMode); err != nil {
		return OpenSCADRenderResult{}, fmt.Errorf("create openscad job directory: %w", err)
	}
	if err := os.Chmod(jobDir, openSCADJobDirMode); err != nil {
		return OpenSCADRenderResult{}, fmt.Errorf("prepare openscad job directory permissions: %w", err)
	}
	sourcePath := filepath.Join(jobDir, "model.scad")
	if err := os.WriteFile(sourcePath, []byte(req.SourceSCAD), openSCADSourceFileMode); err != nil {
		return OpenSCADRenderResult{}, fmt.Errorf("write model.scad: %w", err)
	}
	if err := os.Chmod(sourcePath, openSCADSourceFileMode); err != nil {
		return OpenSCADRenderResult{}, fmt.Errorf("prepare model.scad permissions: %w", err)
	}
	start := time.Now()
	timeout := openSCADRenderTimeout(req.TimeoutSeconds, s.cfg.OpenSCAD)
	var combinedOutput strings.Builder
	var skippedExports []string
	result := OpenSCADRenderResult{
		JobID:        jobID,
		ModelName:    modelName,
		SourcePath:   sourcePath,
		SourceSCAD:   req.SourceSCAD,
		DownloadBase: "/api/openscad/jobs/" + jobID + "/files/",
		CreatedAt:    start.UTC(),
	}
	for _, export := range req.Exports {
		cmd, filename := buildOpenSCADCommand(jobID, modelName, export, req)
		execResult, execErr := s.docker.ExecContainer(ctx, containerID, cmd, "", timeout)
		execOutput := strings.TrimSpace(execResult.Output)
		if execErr != nil {
			result.ExitCode = execResult.ExitCode
			result.DurationMS = time.Since(start).Milliseconds()
			appendOpenSCADOutput(&combinedOutput, execResult.Output)
			result.Stderr = truncateOpenSCADOutput(combinedOutput.String())
			_ = s.writeJobMetadata(jobDir, result)
			return result, fmt.Errorf("run openscad export %s: %w", export, execErr)
		}
		if execResult.ExitCode != 0 {
			if openSCADExportFailedBecause3DObject(export, execOutput) {
				skippedExports = append(skippedExports, openSCADSkipped2DExportMessage(export))
				continue
			}
			result.ExitCode = execResult.ExitCode
			result.DurationMS = time.Since(start).Milliseconds()
			appendOpenSCADOutput(&combinedOutput, execResult.Output)
			result.Stderr = truncateOpenSCADOutput(combinedOutput.String())
			_ = s.writeJobMetadata(jobDir, result)
			return result, fmt.Errorf("openscad export %s failed: %s", export, strings.TrimSpace(result.Stderr))
		}
		appendOpenSCADOutput(&combinedOutput, execResult.Output)
		file, err := s.outputFile(jobDir, jobID, filename, export)
		if err != nil {
			result.DurationMS = time.Since(start).Milliseconds()
			result.Stderr = truncateOpenSCADOutput(combinedOutput.String())
			_ = s.writeJobMetadata(jobDir, result)
			return result, err
		}
		result.Files = append(result.Files, file)
	}
	result.DurationMS = time.Since(start).Milliseconds()
	if len(result.Files) == 0 && len(skippedExports) > 0 {
		result.ExitCode = 1
		result.Stderr = truncateOpenSCADOutput(strings.Join(skippedExports, "\n"))
		_ = s.writeJobMetadata(jobDir, result)
		return result, fmt.Errorf("%s", strings.Join(skippedExports, "\n"))
	}
	result.ExitCode = 0
	result.Stdout = truncateOpenSCADOutput(combinedOutput.String())
	if len(skippedExports) > 0 {
		result.Stderr = truncateOpenSCADOutput(strings.Join(skippedExports, "\n"))
	}
	if err := s.enforceOutputLimit(result.Files); err != nil {
		result.Stderr = err.Error()
		_ = s.writeJobMetadata(jobDir, result)
		return result, err
	}
	if err := s.writeJobMetadata(jobDir, result); err != nil {
		return OpenSCADRenderResult{}, err
	}
	s.mu.Lock()
	s.touchLocked()
	s.pruneOldOpenSCADJobs(jobsRoot, jobID)
	return result, nil
}

func (s *OpenSCADContainerService) Job(ctx context.Context, jobID string) (OpenSCADRenderResult, error) {
	_ = ctx
	jobDir, err := s.safeJobDir(jobID)
	if err != nil {
		return OpenSCADRenderResult{}, err
	}
	data, err := os.ReadFile(filepath.Join(jobDir, "job.json"))
	if err != nil {
		return OpenSCADRenderResult{}, fmt.Errorf("read openscad job metadata: %w", err)
	}
	var result OpenSCADRenderResult
	if err := json.Unmarshal(data, &result); err != nil {
		return OpenSCADRenderResult{}, fmt.Errorf("decode openscad job metadata: %w", err)
	}
	return result, nil
}

func (s *OpenSCADContainerService) JobFile(jobID, filename string) (string, OpenSCADFile, error) {
	jobDir, err := s.safeJobDir(jobID)
	if err != nil {
		return "", OpenSCADFile{}, err
	}
	name := safeOpenSCADFilename(filename)
	if name == "" || name == "job.json" || name == "model.scad" {
		return "", OpenSCADFile{}, fmt.Errorf("invalid openscad job filename")
	}
	path := filepath.Join(jobDir, name)
	file, err := s.describeOutputFile(path, jobID, name, strings.TrimPrefix(filepath.Ext(name), "."))
	if err != nil {
		return "", OpenSCADFile{}, err
	}
	return path, file, nil
}

func (s *OpenSCADContainerService) SaveJob(ctx context.Context, desktopSvc *Service, jobID string) (OpenSCADRenderResult, error) {
	if desktopSvc == nil {
		return OpenSCADRenderResult{}, fmt.Errorf("desktop service is required")
	}
	result, err := s.Job(ctx, jobID)
	if err != nil {
		return OpenSCADRenderResult{}, err
	}
	jobDir, err := s.safeJobDir(jobID)
	if err != nil {
		return OpenSCADRenderResult{}, err
	}
	base := "Documents/OpenSCAD/" + safeOpenSCADModelName(result.ModelName) + "-" + jobID
	var saved []string
	for i := range result.Files {
		filePath := filepath.Join(jobDir, result.Files[i].Name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return result, fmt.Errorf("read openscad output %s: %w", result.Files[i].Name, err)
		}
		rel := path.Join(base, result.Files[i].Name)
		if err := desktopSvc.WriteFileBytes(ctx, rel, data, SourceAgent); err != nil {
			return result, fmt.Errorf("save openscad output %s: %w", result.Files[i].Name, err)
		}
		result.Files[i].SavedPath = rel
		saved = append(saved, rel)
	}
	sourceData, err := os.ReadFile(filepath.Join(jobDir, "model.scad"))
	if err == nil {
		rel := path.Join(base, "model.scad")
		if err := desktopSvc.WriteFileBytes(ctx, rel, sourceData, SourceAgent); err != nil {
			return result, fmt.Errorf("save openscad source: %w", err)
		}
		saved = append(saved, rel)
	}
	result.SavedPaths = saved
	if err := s.writeJobMetadata(jobDir, result); err != nil {
		return result, err
	}
	return result, nil
}

func (s *OpenSCADContainerService) ContainerID(ctx context.Context) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	containerID, _, err := s.findContainerLocked(ctx)
	if err != nil {
		return "", err
	}
	if containerID == "" {
		return "", fmt.Errorf("openscad container not found")
	}
	return containerID, nil
}

func (s *OpenSCADContainerService) Status(ctx context.Context) OpenSCADStatus {
	s.mu.RLock()
	status := OpenSCADStatus{
		Enabled:                 s.cfg.OpenSCAD.Enabled,
		State:                   s.state.String(),
		Running:                 s.state == StateRunning,
		Image:                   openSCADImage(s.cfg.OpenSCAD),
		JobsHostPath:            s.jobsRootLocked(),
		JobsContainerPath:       openSCADJobsInContainer,
		AutoStopMinutes:         openSCADAutoStopMinutes(s.cfg.OpenSCAD),
		MaxConcurrentJobs:       openSCADMaxConcurrentJobs(s.cfg.OpenSCAD),
		DefaultExports:          openSCADDefaultExports(s.cfg.OpenSCAD),
		RenderTimeoutSeconds:    int(openSCADDefaultTimeout(s.cfg.OpenSCAD).Seconds()),
		MaxRenderTimeoutSeconds: int(openSCADMaxTimeout(s.cfg.OpenSCAD).Seconds()),
		LastActivity:            s.lastActivity,
		Resources:               openSCADResources(s.cfg.OpenSCAD),
	}
	s.mu.RUnlock()
	containerID, running, err := s.findContainer(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.ContainerID = containerID
	status.Running = running
	if running {
		status.State = StateRunning.String()
	}
	return status
}

func (s *OpenSCADContainerService) validateRenderRequest(req *OpenSCADRenderRequest) error {
	req.SourceSCAD = strings.TrimSpace(req.SourceSCAD)
	if req.SourceSCAD == "" {
		return fmt.Errorf("source_scad is required")
	}
	if len(req.SourceSCAD) > openSCADMaxSourceBytes(s.cfg.OpenSCAD) {
		return fmt.Errorf("source_scad exceeds %d KiB", openSCADMaxSourceKB(s.cfg.OpenSCAD))
	}
	if len(req.Exports) == 0 {
		req.Exports = openSCADDefaultExports(s.cfg.OpenSCAD)
	}
	seen := map[string]struct{}{}
	exports := make([]string, 0, len(req.Exports))
	for _, item := range req.Exports {
		export := strings.ToLower(strings.TrimSpace(item))
		if _, ok := openSCADExportSet[export]; !ok {
			return fmt.Errorf("unsupported openscad export %q", item)
		}
		if _, ok := seen[export]; ok {
			continue
		}
		seen[export] = struct{}{}
		exports = append(exports, export)
	}
	req.Exports = exports
	for _, define := range req.Defines {
		if !openSCADDefineNamePattern.MatchString(strings.TrimSpace(define.Name)) {
			return fmt.Errorf("invalid define name %q", define.Name)
		}
		if len(define.Value) > 4096 {
			return fmt.Errorf("define %s value is too large", define.Name)
		}
	}
	mode := strings.ToLower(strings.TrimSpace(req.RenderMode))
	switch mode {
	case "", "render", "preview":
		req.RenderMode = mode
	default:
		return fmt.Errorf("unsupported render_mode %q", req.RenderMode)
	}
	return nil
}

func (s *OpenSCADContainerService) ensureJobsRoot() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ensureJobsRootLocked()
}

func (s *OpenSCADContainerService) ensureJobsRootLocked() (string, error) {
	root := s.jobsRootLocked()
	if err := os.MkdirAll(root, openSCADJobsRootMode); err != nil {
		return "", fmt.Errorf("create openscad jobs root: %w", err)
	}
	if err := os.Chmod(root, openSCADJobsRootMode); err != nil {
		return "", fmt.Errorf("prepare openscad jobs root permissions: %w", err)
	}
	return root, nil
}

func (s *OpenSCADContainerService) jobsRootLocked() string {
	dataDir := strings.TrimSpace(s.cfg.DataDir)
	if dataDir == "" {
		dataDir = filepath.Join(filepath.Dir(s.cfg.DBPath), "data")
	}
	return filepath.Join(dataDir, "openscad", "jobs")
}

func (s *OpenSCADContainerService) safeJobDir(jobID string) (string, error) {
	jobID = strings.TrimSpace(jobID)
	if !strings.HasPrefix(jobID, "oscad-") || strings.ContainsAny(jobID, `/\`) || strings.Contains(jobID, "..") {
		return "", fmt.Errorf("invalid openscad job id")
	}
	root, err := s.ensureJobsRoot()
	if err != nil {
		return "", err
	}
	jobDir := filepath.Join(root, jobID)
	rel, err := filepath.Rel(root, jobDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("openscad job path escapes jobs root")
	}
	return jobDir, nil
}

func (s *OpenSCADContainerService) outputFile(jobDir, jobID, filename, format string) (OpenSCADFile, error) {
	path := filepath.Join(jobDir, filename)
	return s.describeOutputFile(path, jobID, filename, format)
}

func (s *OpenSCADContainerService) describeOutputFile(filePath, jobID, filename, format string) (OpenSCADFile, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return OpenSCADFile{}, fmt.Errorf("openscad output %s missing: %w", filename, err)
	}
	if info.IsDir() {
		return OpenSCADFile{}, fmt.Errorf("openscad output %s is a directory", filename)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return OpenSCADFile{}, fmt.Errorf("read openscad output %s: %w", filename, err)
	}
	sum := sha256.Sum256(data)
	return OpenSCADFile{
		Name:        filename,
		Format:      strings.ToLower(format),
		Size:        info.Size(),
		SHA256:      hex.EncodeToString(sum[:]),
		DownloadURL: "/api/openscad/jobs/" + jobID + "/files/" + filename + "?download=1",
		PreviewURL:  "/api/openscad/jobs/" + jobID + "/files/" + filename,
	}, nil
}

func (s *OpenSCADContainerService) enforceOutputLimit(files []OpenSCADFile) error {
	var total int64
	for _, file := range files {
		total += file.Size
	}
	maxBytes := int64(openSCADMaxOutputMB(s.cfg.OpenSCAD)) * 1024 * 1024
	if maxBytes > 0 && total > maxBytes {
		return fmt.Errorf("openscad outputs exceed %d MiB", openSCADMaxOutputMB(s.cfg.OpenSCAD))
	}
	return nil
}

func (s *OpenSCADContainerService) writeJobMetadata(jobDir string, result OpenSCADRenderResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal openscad job metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "job.json"), data, 0o600); err != nil {
		return fmt.Errorf("write openscad job metadata: %w", err)
	}
	return nil
}

func (s *OpenSCADContainerService) findContainer(ctx context.Context) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.findContainerLocked(ctx)
}

func (s *OpenSCADContainerService) findContainerLocked(ctx context.Context) (string, bool, error) {
	if s.docker == nil {
		return "", false, fmt.Errorf("Docker backend not configured")
	}
	containers, err := s.docker.ListContainers(ctx, true)
	if err != nil {
		return "", false, fmt.Errorf("list docker containers: %w", err)
	}
	for _, container := range containers {
		if !openSCADContainerMatches(container) {
			continue
		}
		inspect, err := s.docker.InspectContainer(ctx, container.ID)
		if err != nil {
			return "", false, fmt.Errorf("inspect openscad container: %w", err)
		}
		return container.ID, inspect.State.Running, nil
	}
	return "", false, nil
}

func (s *OpenSCADContainerService) waitForRunningLocked(ctx context.Context, containerID string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for openscad container to start")
		case <-ticker.C:
			inspect, err := s.docker.InspectContainer(ctx, containerID)
			if err == nil && inspect.State.Running {
				return nil
			}
		}
	}
}

func (s *OpenSCADContainerService) touchLocked() {
	s.lastActivity = time.Now()
	s.resetAutoStopTimerLocked()
}

func (s *OpenSCADContainerService) resetAutoStopTimerLocked() {
	if s.stopTimer != nil {
		s.stopTimer.Stop()
	}
	timeout := s.autoStopAfter
	if s.cfg.OpenSCAD.AutoStopMinutes > 0 {
		timeout = time.Duration(s.cfg.OpenSCAD.AutoStopMinutes) * time.Minute
	}
	if timeout <= 0 {
		s.stopTimer = nil
		return
	}
	s.stopTimer = time.AfterFunc(timeout, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Stop(ctx); err != nil && s.logger != nil {
			s.logger.Warn("openscad auto-stop failed", "error", err)
		}
	})
}

func buildOpenSCADCommand(jobID, modelName, export string, req OpenSCADRenderRequest) ([]string, string) {
	ext := export
	if ext == "echo" {
		ext = "txt"
	}
	filename := modelName + "." + ext
	outputPath := path.Join(openSCADJobsInContainer, jobID, filename)
	sourcePath := path.Join(openSCADJobsInContainer, jobID, "model.scad")
	cmd := []string{"openscad"}
	if export == "png" {
		cmd = []string{"xvfb-run", "-a", "openscad"}
	}
	switch strings.ToLower(strings.TrimSpace(req.RenderMode)) {
	case "preview":
		cmd = append(cmd, "--preview")
	case "render", "":
		cmd = append(cmd, "--render")
	}
	for _, define := range req.Defines {
		cmd = append(cmd, "-D", strings.TrimSpace(define.Name)+"="+define.Value)
	}
	cmd = append(cmd, "-o", outputPath, sourcePath)
	return cmd, filename
}

func appendOpenSCADOutput(builder *strings.Builder, output string) {
	if builder == nil || strings.TrimSpace(output) == "" {
		return
	}
	builder.WriteString(output)
	if !strings.HasSuffix(output, "\n") {
		builder.WriteByte('\n')
	}
}

func openSCADExportFailedBecause3DObject(export, output string) bool {
	if _, ok := openSCAD2DOnlyExportSet[strings.ToLower(strings.TrimSpace(export))]; !ok {
		return false
	}
	normalized := strings.ToLower(output)
	return strings.Contains(normalized, "not a 2d object") ||
		strings.Contains(normalized, "top level object is a 3d object")
}

func openSCADSkipped2DExportMessage(export string) string {
	export = strings.ToLower(strings.TrimSpace(export))
	return fmt.Sprintf("Skipped %s export: OpenSCAD %s export requires a 2D top-level object; the current model is 3D. Use PNG/STL/3MF/OFF/AMF/CSG for 3D models or wrap a 2D design/projection before exporting %s.", export, strings.ToUpper(export), export)
}

func openSCADContainerMatches(container CodeDockerContainer) bool {
	if container.ID == openSCADContainerName {
		return true
	}
	for _, name := range container.Names {
		if strings.TrimPrefix(name, "/") == openSCADContainerName {
			return true
		}
	}
	return false
}

func openSCADImage(cfg OpenSCADConfig) string {
	image := strings.TrimSpace(cfg.Image)
	if image == "" {
		return defaultOpenSCADImage
	}
	return image
}

func openSCADResources(cfg OpenSCADConfig) CodeContainerResources {
	return CodeContainerResources{
		MemoryMB:  openSCADMemoryMB(cfg),
		CPUCores:  openSCADCPUCores(cfg),
		PidsLimit: defaultOpenSCADPidsLimit,
	}
}

func openSCADResourcesPtr(cfg OpenSCADConfig) *CodeContainerResources {
	resources := openSCADResources(cfg)
	return &resources
}

func openSCADMemoryMB(cfg OpenSCADConfig) int {
	if cfg.MaxMemoryMB > 0 {
		return cfg.MaxMemoryMB
	}
	return defaultOpenSCADMemoryMB
}

func openSCADCPUCores(cfg OpenSCADConfig) int {
	if cfg.MaxCPUCores > 0 {
		return cfg.MaxCPUCores
	}
	return defaultOpenSCADCPUCores
}

func openSCADMaxConcurrentJobs(cfg OpenSCADConfig) int {
	if cfg.MaxConcurrentJobs > 0 {
		return cfg.MaxConcurrentJobs
	}
	return 1
}

func openSCADAutoStopMinutes(cfg OpenSCADConfig) int {
	if cfg.AutoStopMinutes > 0 {
		return cfg.AutoStopMinutes
	}
	return int(defaultOpenSCADAutoStop.Minutes())
}

func openSCADDefaultExports(cfg OpenSCADConfig) []string {
	if len(cfg.DefaultExports) == 0 {
		return []string{"png", "stl"}
	}
	out := append([]string(nil), cfg.DefaultExports...)
	for i := range out {
		out[i] = strings.ToLower(strings.TrimSpace(out[i]))
	}
	sort.Strings(out)
	return out
}

func openSCADMaxSourceKB(cfg OpenSCADConfig) int {
	if cfg.MaxSourceKB > 0 {
		return cfg.MaxSourceKB
	}
	return defaultOpenSCADMaxSourceKB
}

func openSCADMaxSourceBytes(cfg OpenSCADConfig) int {
	return openSCADMaxSourceKB(cfg) * 1024
}

func openSCADMaxOutputMB(cfg OpenSCADConfig) int {
	if cfg.MaxOutputMB > 0 {
		return cfg.MaxOutputMB
	}
	return defaultOpenSCADMaxOutputMB
}

func openSCADDefaultTimeout(cfg OpenSCADConfig) time.Duration {
	if cfg.RenderTimeoutSeconds > 0 {
		return time.Duration(cfg.RenderTimeoutSeconds) * time.Second
	}
	return defaultOpenSCADRenderTimeout
}

func openSCADMaxTimeout(cfg OpenSCADConfig) time.Duration {
	if cfg.MaxRenderTimeoutSeconds > 0 {
		return time.Duration(cfg.MaxRenderTimeoutSeconds) * time.Second
	}
	return defaultOpenSCADMaxRenderTimeout
}

func openSCADRenderTimeout(requested int, cfg OpenSCADConfig) time.Duration {
	timeout := openSCADDefaultTimeout(cfg)
	if requested > 0 {
		timeout = time.Duration(requested) * time.Second
	}
	maxTimeout := openSCADMaxTimeout(cfg)
	if timeout > maxTimeout {
		timeout = maxTimeout
	}
	return timeout
}

func safeOpenSCADModelName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		name = "model"
	}
	name = strings.Trim(openSCADModelNamePattern.ReplaceAllString(name, "-"), ".-_")
	if name == "" {
		name = "model"
	}
	if len(name) > 80 {
		name = strings.TrimRight(name[:80], ".-_")
		if name == "" {
			name = "model"
		}
	}
	return name
}

func safeOpenSCADFilename(value string) string {
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	name = openSCADModelNamePattern.ReplaceAllString(name, "-")
	name = strings.Trim(name, ".-_")
	if name == "" || strings.Contains(name, "..") {
		return ""
	}
	return name
}

func newOpenSCADJobID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("oscad-%d", time.Now().UTC().UnixNano())
	}
	return "oscad-" + hex.EncodeToString(b[:])
}

func openSCADJobRetentionDays(cfg OpenSCADConfig) int {
	if cfg.JobRetentionDays <= 0 {
		return defaultOpenSCADJobRetentionDays
	}
	return cfg.JobRetentionDays
}

func openSCADJobDirCreatedAt(jobDir string) time.Time {
	data, err := os.ReadFile(filepath.Join(jobDir, "job.json"))
	if err == nil {
		var meta OpenSCADRenderResult
		if json.Unmarshal(data, &meta) == nil && !meta.CreatedAt.IsZero() {
			return meta.CreatedAt.UTC()
		}
	}
	fi, err := os.Stat(jobDir)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime().UTC()
}

func (s *OpenSCADContainerService) pruneOldOpenSCADJobs(jobsRoot, keepJobID string) {
	days := openSCADJobRetentionDays(s.cfg.OpenSCAD)
	if days <= 0 {
		return
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	entries, err := os.ReadDir(jobsRoot)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if !ent.IsDir() || !strings.HasPrefix(ent.Name(), "oscad-") {
			continue
		}
		if ent.Name() == keepJobID {
			continue
		}
		created := openSCADJobDirCreatedAt(filepath.Join(jobsRoot, ent.Name()))
		if created.IsZero() || created.Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(jobsRoot, ent.Name()))
		}
	}
}

func truncateOpenSCADOutput(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 6000 {
		return value
	}
	return value[:6000] + "\n...[truncated]"
}
