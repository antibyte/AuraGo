package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/uid"
	"aurago/internal/videostudio"
)

const videoStudioAPIBase = "/api/desktop/video-studio"
const videoStudioJSONLimit = 8 << 20

type videoStudioProjectItem struct {
	ID          string              `json:"id"`
	Project     videostudio.Project `json:"project"`
	DesktopPath string              `json:"desktop_path"`
	ETag        string              `json:"etag"`
	UpdatedAt   time.Time           `json:"updated_at,omitempty"`
}

type videoStudioMediaItem struct {
	Asset    videostudio.Asset `json:"asset"`
	MediaURL string            `json:"media_url"`
	State    string            `json:"state"`
	JobID    string            `json:"job_id,omitempty"`
}

func handleVideoStudio(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == videoStudioAPIBase || r.URL.Path == videoStudioAPIBase+"/" {
			writeVideoStudioError(w, http.StatusNotFound, "not_found", "Video Studio endpoint not found.")
			return
		}
		if !strings.HasPrefix(r.URL.Path, videoStudioAPIBase+"/") {
			writeVideoStudioError(w, http.StatusNotFound, "not_found", "Video Studio endpoint not found.")
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, videoStudioAPIBase), "/"), "/")
		if len(parts) == 1 && parts[0] == "status" {
			handleVideoStudioStatus(s, w, r)
			return
		}
		if len(parts) == 1 && parts[0] == "projects" {
			handleVideoStudioProjects(s, w, r)
			return
		}
		if len(parts) >= 2 && parts[0] == "projects" {
			if !validVideoStudioProjectID(parts[1]) {
				writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
				return
			}
			switch {
			case len(parts) == 2:
				handleVideoStudioProject(s, w, r, parts[1])
			case len(parts) == 3 && parts[2] == "media":
				handleVideoStudioMedia(s, w, r, parts[1])
			case len(parts) == 4 && parts[2] == "media":
				handleVideoStudioMediaFile(s, w, r, parts[1], parts[3])
			case len(parts) == 3 && parts[2] == "jobs":
				if r.Method == http.MethodGet {
					handleVideoStudioJobList(s, w, r, parts[1])
				} else {
					handleVideoStudioJobCreate(s, w, r, parts[1])
				}
			case len(parts) == 4 && parts[2] == "previews":
				handleVideoStudioPreviewFile(s, w, r, parts[1], parts[3])
			case len(parts) == 4 && parts[2] == "exports":
				handleVideoStudioExportFile(s, w, r, parts[1], parts[3])
			default:
				writeVideoStudioError(w, http.StatusNotFound, "not_found", "Video Studio endpoint not found.")
			}
			return
		}
		if len(parts) == 1 && parts[0] == "jobs" && r.Method == http.MethodGet {
			handleVideoStudioJobList(s, w, r, "")
			return
		}
		if len(parts) == 2 && parts[0] == "jobs" {
			if !validVideoStudioProjectID(parts[1]) {
				writeVideoStudioError(w, http.StatusNotFound, "job_not_found", "Job not found.")
				return
			}
			if r.Method == http.MethodGet {
				handleVideoStudioJobGet(s, w, r, parts[1])
				return
			}
		}
		if len(parts) == 3 && parts[0] == "jobs" && parts[2] == "cancel" && r.Method == http.MethodPost {
			if !validVideoStudioProjectID(parts[1]) {
				writeVideoStudioError(w, http.StatusNotFound, "job_not_found", "Job not found.")
				return
			}
			handleVideoStudioJobCancel(s, w, r, parts[1])
			return
		}
		writeVideoStudioError(w, http.StatusNotFound, "not_found", "Video Studio endpoint not found.")
	}
}

func normalizedVideoStudioConfig(s *Server) (*config.Config, error) {
	if s == nil {
		return nil, fmt.Errorf("server unavailable")
	}
	return normalizeVideoStudioConfigSnapshot(s.ConfigSnapshot())
}

func normalizeVideoStudioConfigSnapshot(cfg *config.Config) (*config.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration unavailable")
	}
	copyCfg := *cfg
	if err := config.NormalizeVideoStudioConfig(&copyCfg.VideoStudio); err != nil {
		return nil, err
	}
	return &copyCfg, nil
}

func handleVideoStudioStatus(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVideoStudioMethodError(w)
		return
	}
	if !requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead) {
		return
	}
	cfg, err := normalizedVideoStudioConfig(s)
	if err != nil {
		writeVideoStudioError(w, http.StatusInternalServerError, "configuration_invalid", "Video Studio configuration is invalid.")
		return
	}
	runtime := videostudio.CheckRuntime(r.Context(), cfg.VideoStudio.FFmpegPath)
	provider := strings.ToLower(cfg.VideoGeneration.ProviderType)
	model := cfg.VideoGeneration.ResolvedModel
	if model == "" {
		model = cfg.VideoGeneration.Model
	}
	configured := cfg.VideoGeneration.Enabled && strings.TrimSpace(cfg.VideoGeneration.APIKey) != "" && provider != ""
	issue := ""
	switch {
	case !cfg.VideoStudio.Enabled:
		issue = "disabled"
	case !cfg.VirtualDesktop.Enabled:
		issue = "desktop_disabled"
	case !runtime.Ready:
		issue = "ffmpeg_unavailable"
	}
	limits := map[string]interface{}{
		"max_asset_size_bytes":   int64(cfg.VideoStudio.MaxAssetSizeMB) << 20,
		"max_project_size_bytes": int64(cfg.VideoStudio.MaxProjectSizeMB) << 20,
		"max_duration_frames":    videostudio.MaxDurationFrames,
		"fps":                    videostudio.FramesPerSecond,
		"max_tracks":             map[string]int{"video": videostudio.MaxVideoTracks, "audio": videostudio.MaxAudioTracks, "overlay": videostudio.MaxOverlayTracks},
		"max_clips":              videostudio.MaxClipsPerProject,
		"max_render_assets":      videostudio.MaxRenderInputs,
		"canvas_sizes": []map[string]int{
			{"width": 1280, "height": 720}, {"width": 1920, "height": 1080},
			{"width": 720, "height": 1280}, {"width": 1080, "height": 1920},
			{"width": 720, "height": 720}, {"width": 1080, "height": 1080},
		},
	}
	generation := map[string]interface{}{
		"enabled":           cfg.VideoGeneration.Enabled,
		"configured":        configured,
		"provider":          provider,
		"model":             model,
		"durations_seconds": videoStudioDurationOptions(cfg, "", false),
		"image_modes":       videoStudioImageModes(provider),
		"budget_blocked":    s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("video_generation"),
	}
	w.Header().Set("Cache-Control", "no-store")
	writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":         cfg.VideoStudio.Enabled,
		"read_only":       cfg.VideoStudio.ReadOnly || cfg.VirtualDesktop.ReadOnly,
		"desktop_enabled": cfg.VirtualDesktop.Enabled,
		"ffmpeg_ready":    runtime.Ready,
		"ffmpeg_version":  runtime.Version,
		"issue":           issue,
		"generation":      generation,
		"limits":          limits,
	})
}

func videoStudioDurationOptions(cfg *config.Config, resolution string, hasReferences bool) []int {
	provider := strings.ToLower(cfg.VideoGeneration.ProviderType)
	resolution = strings.ToLower(firstNonEmptyVideoStudio(resolution, cfg.VideoGeneration.DefaultResolution))
	switch provider {
	case "google", "google_veo":
		if strings.Contains(resolution, "1080") || hasReferences {
			return []int{8}
		}
		return []int{4, 6, 8}
	case "minimax":
		model := strings.ToLower(cfg.VideoGeneration.ResolvedModel)
		if model == "" {
			model = strings.ToLower(cfg.VideoGeneration.Model)
		}
		if strings.Contains(resolution, "1080") || (!strings.Contains(model, "hailuo") && model != "") {
			return []int{6}
		}
		return []int{6, 10}
	case "agnes":
		values := make([]int, 30)
		for i := range values {
			values[i] = i + 1
		}
		return values
	default:
		return []int{}
	}
}

func videoStudioImageModes(provider string) []string {
	switch strings.ToLower(provider) {
	case "minimax":
		return []string{"first_frame"}
	case "google", "google_veo":
		return []string{"first_frame"}
	default:
		return []string{}
	}
}

func firstNonEmptyVideoStudio(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func handleVideoStudioProjects(s *Server, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		svc, manager, cfg, ok := videoStudioReadAccess(s, w, r)
		if !ok {
			return
		}
		_ = manager
		entries, err := svc.ListFiles(r.Context(), "Documents/Video Studio")
		if errors.Is(err, os.ErrNotExist) {
			writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"projects": []videoStudioProjectItem{}})
			return
		}
		if err != nil {
			writeVideoStudioError(w, http.StatusInternalServerError, "project_list_failed", "Projects could not be listed.")
			return
		}
		items := make([]videoStudioProjectItem, 0, len(entries))
		for _, entry := range entries {
			if entry.Type != "directory" || !validVideoStudioProjectID(entry.Name) {
				continue
			}
			project, body, _, readErr := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(entry.Name))
			if readErr != nil {
				continue
			}
			project, readErr = manager.canonicalizeProject(entry.Name, project)
			if readErr == nil {
				readErr = validateVideoStudioProject(project)
			}
			if readErr != nil || !cfg.VideoStudio.Enabled {
				continue
			}
			items = append(items, videoStudioProjectItem{ID: entry.Name, Project: project, DesktopPath: videoStudioProjectPath(entry.Name), ETag: desktop.NoteVersion(body), UpdatedAt: entry.ModTime})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
		writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"projects": items})
	case http.MethodPost:
		svc, _, _, ok := videoStudioMutationAccess(s, w, r, desktopScopeWrite)
		if !ok {
			return
		}
		var body struct {
			Name   string `json:"name"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		}
		if err := decodeVideoStudioJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_request", "Project settings are invalid.")
			return
		}
		project := videostudio.Project{Version: 1, Name: strings.TrimSpace(body.Name), Width: body.Width, Height: body.Height, FPS: videostudio.FramesPerSecond, Assets: []videostudio.Asset{}, Tracks: []videostudio.Track{}}
		if project.Width == 0 {
			project.Width = 1920
		}
		if project.Height == 0 {
			project.Height = 1080
		}
		if err := videostudio.Validate(project); err != nil {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_project", "Project settings do not meet the supported canvas and timeline limits.")
			return
		}
		id := uid.New()
		projectRoot := "Documents/Video Studio"
		if err := svc.CreateDirectory(r.Context(), projectRoot, desktop.SourceUser); err != nil {
			writeVideoStudioError(w, http.StatusInternalServerError, "project_create_failed", "Project storage could not be created.")
			return
		}
		projectPath := videoStudioProjectPath(id)
		if err := svc.CreateDirectory(r.Context(), projectPath+"/media", desktop.SourceUser); err != nil {
			writeVideoStudioError(w, http.StatusInternalServerError, "project_create_failed", "Project storage could not be created.")
			return
		}
		if err := svc.CreateDirectory(r.Context(), projectPath+"/exports", desktop.SourceUser); err != nil {
			_ = svc.DeletePath(r.Context(), projectPath, desktop.SourceUser)
			writeVideoStudioError(w, http.StatusInternalServerError, "project_create_failed", "Project export storage could not be created.")
			return
		}
		data, _ := json.Marshal(project)
		entry, err := svc.WriteFileBytesConditional(r.Context(), projectPath+"/project.json", data, desktop.SourceUser, func(state desktop.FileWriteState) error {
			if state.Exists {
				return fmt.Errorf("project already exists")
			}
			return nil
		})
		if err != nil {
			_ = svc.DeletePath(r.Context(), projectPath, desktop.SourceUser)
			writeVideoStudioError(w, http.StatusInternalServerError, "project_create_failed", "Project settings could not be saved.")
			return
		}
		w.Header().Set("ETag", desktop.NoteVersion(data))
		writeVideoStudioJSON(w, http.StatusCreated, videoStudioProjectItem{ID: id, Project: project, DesktopPath: projectPath, ETag: desktop.NoteVersion(data), UpdatedAt: entry.Modified})
	default:
		writeVideoStudioMethodError(w)
	}
}

func handleVideoStudioProject(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		svc, manager, _, ok := videoStudioReadAccess(s, w, r)
		if !ok {
			return
		}
		project, _, version, err := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(id))
		if err != nil {
			writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
			return
		}
		project, err = manager.canonicalizeProject(id, project)
		if err == nil {
			err = validateVideoStudioProject(project)
		}
		if err != nil {
			writeVideoStudioError(w, http.StatusConflict, "project_asset_invalid", "Project media references could not be verified.")
			return
		}
		w.Header().Set("ETag", version)
		writeVideoStudioJSON(w, http.StatusOK, videoStudioProjectItem{ID: id, Project: project, DesktopPath: videoStudioProjectPath(id), ETag: version})
	case http.MethodPut:
		svc, manager, cfg, ok := videoStudioMutationAccess(s, w, r, desktopScopeWrite)
		if !ok {
			return
		}
		if strings.TrimSpace(r.Header.Get("If-Match")) == "" {
			writeVideoStudioError(w, http.StatusPreconditionRequired, "precondition_required", "Save requires the current project ETag in If-Match.")
			return
		}
		precondition, err := desktopFilePrecondition(r)
		if err != nil {
			writeVideoStudioFileError(w, err)
			return
		}
		var project videostudio.Project
		if err := decodeVideoStudioJSON(w, r, &project, videoStudioJSONLimit); err != nil {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_project", "Project document is invalid or too large.")
			return
		}
		lock := manager.projectLock(id)
		lock.Lock()
		defer lock.Unlock()
		project, err = manager.canonicalizeProject(id, project)
		if err != nil {
			writeVideoStudioError(w, http.StatusBadRequest, "project_asset_invalid", "Project may only reference media imported into this project.")
			return
		}
		if err := videostudio.Validate(project); err != nil {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_project", "Project timeline exceeds supported editing limits.")
			return
		}
		data, err := json.Marshal(project)
		if err != nil || int64(len(data)) > int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
			writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "project_size_limit", "Project document exceeds the configured project limit.")
			return
		}
		used, err := manager.projectStorageSize(r.Context(), svc, id)
		if err != nil {
			writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
			return
		}
		_, oldData, _, readErr := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(id))
		if readErr != nil {
			writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
			return
		}
		newSize := used - int64(len(oldData)) + int64(len(data))
		if newSize > int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 {
			writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "project_size_limit", "Project storage exceeds the configured project limit.")
			return
		}
		entry, err := svc.WriteFileBytesConditional(r.Context(), videoStudioProjectPath(id)+"/project.json", data, desktop.SourceUser, precondition)
		if err != nil {
			writeVideoStudioFileError(w, err)
			return
		}
		version := desktop.NoteVersion(data)
		w.Header().Set("ETag", version)
		writeVideoStudioJSON(w, http.StatusOK, videoStudioProjectItem{ID: id, Project: project, DesktopPath: videoStudioProjectPath(id), ETag: version, UpdatedAt: entry.Modified})
	case http.MethodDelete:
		svc, manager, _, ok := videoStudioMutationAccess(s, w, r, desktopScopeWrite)
		if !ok {
			return
		}
		lock := manager.projectLock(id)
		lock.Lock()
		manager.cancelProject(id)
		err := svc.DeletePath(r.Context(), videoStudioProjectPath(id), desktop.SourceUser)
		if err == nil {
			manager.removeProject(id)
		}
		lock.Unlock()
		if err != nil {
			writeVideoStudioError(w, http.StatusNotFound, "project_delete_failed", "Project could not be deleted.")
			return
		}
		writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"deleted": true, "id": id})
	default:
		writeVideoStudioMethodError(w)
	}
}

func handleVideoStudioMedia(s *Server, w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method == http.MethodGet {
		svc, manager, _, ok := videoStudioReadAccess(s, w, r)
		if !ok {
			return
		}
		if _, _, _, err := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(projectID)); err != nil {
			writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
			return
		}
		records := manager.mediaForProject(projectID)
		items := make([]videoStudioMediaItem, 0, len(records))
		for _, record := range records {
			state, jobID := "ready", ""
			if record.Asset.Kind == "" {
				state = "processing"
				jobID = manager.pendingProbeJob(projectID, record.Asset.ID)
			}
			items = append(items, videoStudioMediaItem{Asset: record.Asset, MediaURL: fmt.Sprintf("%s/projects/%s/media/%s", videoStudioAPIBase, projectID, record.Asset.ID), State: state, JobID: jobID})
		}
		writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"media": items})
		return
	}
	if r.Method != http.MethodPost {
		writeVideoStudioMethodError(w)
		return
	}
	if !authenticateDesktopPermission(s, w, r, desktopScopeWrite) {
		return
	}
	cfg, err := normalizedVideoStudioConfig(s)
	if err != nil {
		writeVideoStudioError(w, http.StatusInternalServerError, "configuration_invalid", "Video Studio configuration is invalid.")
		return
	}
	if !videoStudioEnabled(cfg, w, true) {
		return
	}
	if !checkDesktopOperation(s, w, r, desktopWrite) {
		return
	}
	svc, _, err := s.getDesktopService(r.Context())
	if err != nil {
		writeVideoStudioError(w, http.StatusServiceUnavailable, "desktop_unavailable", "Virtual Desktop storage is unavailable.")
		return
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		writeVideoStudioError(w, http.StatusInternalServerError, "storage_unavailable", "Video Studio storage is unavailable.")
		return
	}
	admittedConfig := s.ConfigSnapshot()
	if videoStudioConfigRootsChanged(admittedConfig, cfg) {
		writeVideoStudioError(w, http.StatusConflict, "configuration_changed", "Video Studio configuration changed before the import started.")
		return
	}
	admittedEpoch := manager.admissionEpoch()
	r = r.WithContext(manager.publicationContext(r.Context(), ""))
	if _, _, _, err := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(projectID)); err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
		return
	}
	key, err := videoStudioIdempotencyKey(r)
	if err != nil {
		writeVideoStudioError(w, http.StatusPreconditionRequired, "idempotency_key_required", "Media imports require an Idempotency-Key header.")
		return
	}
	maxAsset := int64(cfg.VideoStudio.MaxAssetSizeMB) << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxAsset+(1<<20))
	var name string
	var source io.Reader
	var sourceCloser io.Closer
	var multipartReader *multipart.Reader
	var uploadPart *multipart.Part
	var expectedSize int64 = -1
	var restoreUploadDeadline func()
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType == "application/json" {
		var body struct {
			SourcePath string `json:"source_path"`
		}
		if err := decodeVideoStudioJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil || strings.TrimSpace(body.SourcePath) == "" {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_import", "A Desktop source_path is required.")
			return
		}
		file, entry, _, err := svc.OpenPreviewFile(r.Context(), body.SourcePath)
		if err != nil {
			writeVideoStudioError(w, http.StatusBadRequest, "import_source_invalid", "The Desktop source file could not be opened.")
			return
		}
		name, source, sourceCloser, expectedSize = entry.Name, file, file, entry.Size
	} else if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		r.Body, restoreUploadDeadline, err = beginVideoStudioUploadReadWindow(w, r.Body)
		if err != nil {
			writeVideoStudioError(w, http.StatusRequestTimeout, "upload_timeout_setup_failed", "Upload could not establish a bounded read window.")
			return
		}
		defer restoreUploadDeadline()
		reader, err := r.MultipartReader()
		if err != nil {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_upload", "Upload must use multipart/form-data.")
			return
		}
		part, err := reader.NextPart()
		if err != nil || part.FormName() != "file" || part.FileName() == "" {
			if part != nil {
				_ = part.Close()
			}
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_upload", "Upload must include one file field named file.")
			return
		}
		multipartReader, uploadPart = reader, part
		name, source, sourceCloser = part.FileName(), part, part
	} else {
		writeVideoStudioError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Use multipart file upload or a Desktop source_path.")
		return
	}
	if sourceCloser != nil {
		defer sourceCloser.Close()
	}
	if expectedSize > maxAsset {
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "asset_size_limit", "Media file exceeds the configured asset limit.")
		return
	}
	assetID := uid.New()
	name = safeVideoStudioFilename(name)
	mediaPath := "media/" + assetID + "_" + name
	projectLock := manager.projectLock(projectID)
	projectLock.Lock()
	defer projectLock.Unlock()
	used, err := manager.projectStorageSize(r.Context(), svc, projectID)
	if err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
		return
	}
	if len(manager.mediaForProject(projectID)) >= videostudio.MaxAssetsPerProject {
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "asset_count_limit", "Project already contains the maximum number of media assets.")
		return
	}
	remaining := int64(cfg.VideoStudio.MaxProjectSizeMB)<<20 - used
	if remaining <= 0 {
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "project_size_limit", "Project storage is full.")
		return
	}
	if remaining < maxAsset {
		maxAsset = remaining
	}
	if expectedSize > maxAsset {
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "project_size_limit", "Media file exceeds the remaining project storage limit.")
		return
	}
	_ = svc.CreateDirectory(r.Context(), videoStudioProjectPath(projectID)+"/media", desktop.SourceUser)
	hasher := sha256.New()
	_, err = svc.WriteFileStreamConditional(r.Context(), videoStudioProjectPath(projectID)+"/"+mediaPath, io.TeeReader(source, hasher), maxAsset, desktop.SourceUser, nil)
	if err != nil {
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "media_upload_failed", "Media file could not be stored within the configured limits.")
		return
	}
	if uploadPart != nil {
		_ = uploadPart.Close()
		if _, extraErr := multipartReader.NextPart(); extraErr != io.EOF {
			_ = svc.DeletePath(r.Context(), videoStudioProjectPath(projectID)+"/"+mediaPath, desktop.SourceUser)
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_upload", "Upload must contain exactly one file.")
			return
		}
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	record := videoStudioMediaRecord{ProjectID: projectID, Asset: videostudio.Asset{ID: assetID, Name: name, Path: mediaPath}, Path: mediaPath, SHA256: digest, CreatedAt: time.Now().UTC()}
	if err := manager.recordMedia(record); err != nil {
		_ = svc.DeletePath(r.Context(), videoStudioProjectPath(projectID)+"/"+mediaPath, desktop.SourceUser)
		writeVideoStudioError(w, http.StatusInternalServerError, "media_manifest_failed", "Media metadata could not be recorded.")
		return
	}
	fingerprint := "import:" + name + ":" + digest
	work := &videoStudioWork{assetID: assetID, mediaPath: mediaPath, admittedConfig: admittedConfig, admittedEpoch: admittedEpoch}
	job, duplicate, err := manager.enqueue(projectID, "probe", fingerprint, key, work)
	if err != nil {
		if cleanupErr := manager.removeMedia(projectID, assetID); cleanupErr != nil {
			writeVideoStudioError(w, http.StatusInternalServerError, "media_cleanup_failed", "The media import failed and its cleanup could not be recorded safely.")
			return
		}
		_ = svc.DeletePath(r.Context(), videoStudioProjectPath(projectID)+"/"+mediaPath, desktop.SourceUser)
		writeVideoStudioEnqueueError(w, err)
		return
	}
	if duplicate {
		if cleanupErr := manager.removeMedia(projectID, assetID); cleanupErr != nil {
			writeVideoStudioError(w, http.StatusInternalServerError, "media_cleanup_failed", "The duplicate media import could not be cleaned up safely.")
			return
		}
		_ = svc.DeletePath(r.Context(), videoStudioProjectPath(projectID)+"/"+mediaPath, desktop.SourceUser)
		if existingAsset := manager.jobAsset(job.ID); existingAsset != nil {
			if existingRecord, exists := manager.mediaRecord(projectID, existingAsset.ID); exists {
				record = existingRecord
			}
		}
	}
	writeVideoStudioJSON(w, http.StatusAccepted, map[string]interface{}{"asset": record.Asset, "media_url": fmt.Sprintf("%s/projects/%s/media/%s", videoStudioAPIBase, projectID, record.Asset.ID), "job": job})
}

func handleVideoStudioMediaFile(s *Server, w http.ResponseWriter, r *http.Request, projectID, assetID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeVideoStudioMethodError(w)
		return
	}
	if !validVideoStudioAssetID(assetID) {
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	svc, manager, _, ok := videoStudioReadAccess(s, w, r)
	if !ok {
		return
	}
	record, ok := manager.mediaRecord(projectID, assetID)
	if !ok || record.Asset.Kind == "" || !videoStudioMediaPathValid(record.Path, assetID) {
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	file, entry, contentType, err := svc.OpenPreviewFile(r.Context(), videoStudioProjectPath(projectID)+"/"+record.Path)
	if err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Media not found.")
		return
	}
	defer file.Close()
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, record.Asset.Name, entry.ModTime, file)
}

func handleVideoStudioJobCreate(s *Server, w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		writeVideoStudioMethodError(w)
		return
	}
	if !authenticateDesktopPermission(s, w, r, desktopScopeWrite) {
		return
	}
	admittedConfig := s.ConfigSnapshot()
	cfg, err := normalizeVideoStudioConfigSnapshot(admittedConfig)
	if err != nil {
		writeVideoStudioError(w, 500, "configuration_invalid", "Video Studio configuration is invalid.")
		return
	}
	if !videoStudioEnabled(cfg, w, true) {
		return
	}
	var request videoStudioJobRequest
	var body struct {
		Kind              string   `json:"kind"`
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
	if err := decodeVideoStudioJSON(w, r, &body, desktopSmallJSONBodyLimit); err != nil {
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_job", "Job request is invalid.")
		return
	}
	request = videoStudioJobRequest{AssetID: body.AssetID, Prompt: body.Prompt, NegativePrompt: body.NegativePrompt, DurationSeconds: body.DurationSeconds, Resolution: body.Resolution, AspectRatio: body.AspectRatio, FirstFrameAssetID: body.FirstFrameAssetID, LastFrameAssetID: body.LastFrameAssetID, ReferenceAssetIDs: body.ReferenceAssetIDs, IdempotencyKey: body.IdempotencyKey}
	if request.IdempotencyKey == "" {
		request.IdempotencyKey = r.Header.Get("Idempotency-Key")
	}
	key, err := videoStudioIdempotencyKeyValue(request.IdempotencyKey)
	if err != nil {
		writeVideoStudioError(w, http.StatusPreconditionRequired, "idempotency_key_required", "Jobs require an Idempotency-Key header.")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(body.Kind))
	if kind != "render" && kind != "preview" && kind != "probe" && kind != "generate" {
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_job_kind", "Supported jobs are probe, preview, render, and generate.")
		return
	}
	if kind == "generate" && !authenticateDesktopPermission(s, w, r, desktopScopeAdmin) {
		return
	}
	if !checkDesktopOperation(s, w, r, desktopWrite) {
		return
	}
	svc, _, err := s.getDesktopService(r.Context())
	if err != nil {
		writeVideoStudioError(w, http.StatusServiceUnavailable, "desktop_unavailable", "Virtual Desktop storage is unavailable.")
		return
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		writeVideoStudioError(w, 500, "storage_unavailable", "Video Studio storage is unavailable.")
		return
	}
	r = r.WithContext(manager.publicationContext(r.Context(), ""))
	var renderPrecondition desktop.FileWritePrecondition
	if kind == "render" {
		if strings.TrimSpace(r.Header.Get("If-Match")) == "" {
			writeVideoStudioError(w, http.StatusPreconditionRequired, "precondition_required", "Render requires the current saved project ETag in If-Match.")
			return
		}
		renderPrecondition, err = desktopFilePrecondition(r)
		if err != nil {
			writeVideoStudioFileError(w, err)
			return
		}
	}
	projectLock := manager.projectLock(projectID)
	projectLock.Lock()
	defer projectLock.Unlock()
	project, projectBytes, revision, err := readVideoStudioProject(r.Context(), svc, videoStudioProjectPath(projectID))
	if err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "project_not_found", "Project not found.")
		return
	}
	if renderPrecondition != nil {
		if err := renderPrecondition(desktop.FileWriteState{Exists: true, Data: projectBytes, Version: revision}); err != nil {
			writeVideoStudioFileError(w, err)
			return
		}
	}
	project, err = manager.canonicalizeProject(projectID, project)
	if err != nil {
		writeVideoStudioError(w, http.StatusConflict, "project_asset_invalid", "Project media references could not be verified.")
		return
	}
	if err := videostudio.Validate(project); err != nil {
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_project", "Project timeline exceeds supported editing limits.")
		return
	}
	if kind == "generate" {
		if !cfg.VideoGeneration.Enabled || cfg.VideoGeneration.APIKey == "" || cfg.VideoGeneration.ProviderType == "" {
			writeVideoStudioError(w, http.StatusConflict, "generation_unavailable", "Video generation is not configured.")
			return
		}
		if s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("video_generation") {
			writeVideoStudioError(w, http.StatusTooManyRequests, "budget_blocked", "Video generation is blocked by the current budget policy.")
			return
		}
		if strings.TrimSpace(request.Prompt) == "" || len([]rune(request.Prompt)) > 4000 {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_prompt", "Prompt must contain 1 to 4000 characters.")
			return
		}
		if request.DurationSeconds == 0 {
			request.DurationSeconds = cfg.VideoGeneration.DefaultDurationSeconds
		}
		if request.DurationSeconds <= 0 {
			request.DurationSeconds = 6
		}
		if request.Resolution == "" {
			request.Resolution = cfg.VideoGeneration.DefaultResolution
		}
		if request.AspectRatio == "" {
			request.AspectRatio = cfg.VideoGeneration.DefaultAspectRatio
		}
		if !containsVideoStudioInt(videoStudioDurationOptions(cfg, request.Resolution, len(request.ReferenceAssetIDs) > 0), request.DurationSeconds) {
			writeVideoStudioError(w, http.StatusBadRequest, "unsupported_duration", "Requested duration is not available for the configured provider and settings.")
			return
		}
		provider := strings.ToLower(cfg.VideoGeneration.ProviderType)
		if provider == "agnes" && (request.FirstFrameAssetID != "" || request.LastFrameAssetID != "" || len(request.ReferenceAssetIDs) != 0) {
			writeVideoStudioError(w, http.StatusBadRequest, "unsupported_image_mode", "This provider accepts text-only generation in Video Studio.")
			return
		}
		if (provider == "google" || provider == "google_veo" || provider == "minimax") && (request.LastFrameAssetID != "" || len(request.ReferenceAssetIDs) != 0) {
			writeVideoStudioError(w, http.StatusBadRequest, "unsupported_image_mode", "Video Studio currently supports only one first-frame image for this provider.")
			return
		}
		if provider != "agnes" && provider != "google" && provider != "google_veo" && provider != "minimax" {
			writeVideoStudioError(w, http.StatusConflict, "generation_unavailable", "The configured provider has no supported Video Studio generation mode.")
			return
		}
		if request.FirstFrameAssetID != "" {
			if !validVideoStudioAssetID(request.FirstFrameAssetID) {
				writeVideoStudioError(w, http.StatusBadRequest, "invalid_image_asset", "Generation images must be imported project assets.")
				return
			}
			record, ok := manager.mediaRecord(projectID, request.FirstFrameAssetID)
			if !ok || record.Asset.Kind != videostudio.AssetImage {
				writeVideoStudioError(w, http.StatusBadRequest, "invalid_image_asset", "Generation images must be imported image assets in this project.")
				return
			}
		}
	}
	if kind == "preview" || kind == "probe" {
		if !validVideoStudioAssetID(request.AssetID) {
			writeVideoStudioError(w, http.StatusBadRequest, "invalid_asset_id", "A project asset_id is required.")
			return
		}
		record, ok := manager.mediaRecord(projectID, request.AssetID)
		if !ok {
			writeVideoStudioError(w, http.StatusNotFound, "media_not_found", "Project media was not found.")
			return
		}
		if kind == "preview" && record.Asset.Kind != videostudio.AssetVideo {
			writeVideoStudioError(w, http.StatusBadRequest, "preview_requires_video", "Previews are available for video assets.")
			return
		}
	}
	work := &videoStudioWork{project: project, revision: revision, request: request}
	work.admittedConfig = admittedConfig
	work.admittedEpoch = manager.admissionEpoch()
	if kind == "probe" {
		record, _ := manager.mediaRecord(projectID, request.AssetID)
		work.assetID, work.mediaPath = request.AssetID, record.Path
	}
	keyFree := request
	keyFree.IdempotencyKey = ""
	canonicalRequest, _ := json.Marshal(struct {
		Kind     string                `json:"kind"`
		Revision string                `json:"revision"`
		Request  videoStudioJobRequest `json:"request"`
	}{kind, revision, keyFree})
	fingerprint := videoStudioHash(string(canonicalRequest))
	job, _, err := manager.enqueue(projectID, kind, fingerprint, key, work)
	if err != nil {
		writeVideoStudioEnqueueError(w, err)
		return
	}
	writeVideoStudioJSON(w, http.StatusAccepted, map[string]interface{}{"job": job})
}

func handleVideoStudioJobList(s *Server, w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodGet {
		writeVideoStudioMethodError(w)
		return
	}
	_, manager, _, ok := videoStudioReadAccess(s, w, r)
	if !ok {
		return
	}
	if projectID == "" {
		projectID = r.URL.Query().Get("project_id")
	}
	if projectID != "" && !validVideoStudioProjectID(projectID) {
		writeVideoStudioError(w, http.StatusBadRequest, "invalid_project_id", "Project id is invalid.")
		return
	}
	var jobs []*videoStudioJob
	if projectID == "" {
		manager.mu.Lock()
		for _, job := range manager.jobs {
			copyJob := *job
			if job.Artifact != nil {
				artifact := *job.Artifact
				artifact.privatePath = ""
				copyJob.Artifact = &artifact
			}
			jobs = append(jobs, &copyJob)
		}
		manager.mu.Unlock()
		sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt.After(jobs[j].CreatedAt) })
	} else {
		jobs = manager.jobsForProject(projectID)
	}
	writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"jobs": jobs})
}

func handleVideoStudioJobGet(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeVideoStudioMethodError(w)
		return
	}
	_, manager, _, ok := videoStudioReadAccess(s, w, r)
	if !ok {
		return
	}
	job := manager.job(id)
	if job == nil {
		writeVideoStudioError(w, http.StatusNotFound, "job_not_found", "Job not found.")
		return
	}
	writeVideoStudioJSON(w, http.StatusOK, job)
}

func handleVideoStudioJobCancel(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeVideoStudioMethodError(w)
		return
	}
	if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopStop) {
		return
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		writeVideoStudioError(w, 500, "storage_unavailable", "Video Studio storage is unavailable.")
		return
	}
	job := manager.job(id)
	if job == nil {
		writeVideoStudioError(w, http.StatusNotFound, "job_not_found", "Job not found.")
		return
	}
	manager.cancelJob(id)
	job = manager.job(id)
	writeVideoStudioJSON(w, http.StatusOK, map[string]interface{}{"job": job})
}

func handleVideoStudioPreviewFile(s *Server, w http.ResponseWriter, r *http.Request, projectID, jobID string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeVideoStudioMethodError(w)
		return
	}
	if !validVideoStudioProjectID(jobID) {
		writeVideoStudioError(w, http.StatusNotFound, "preview_not_found", "Preview not found.")
		return
	}
	_, manager, _, ok := videoStudioReadAccess(s, w, r)
	if !ok {
		return
	}
	job := manager.job(jobID)
	if job == nil || job.ProjectID != projectID || job.Kind != "preview" || job.Status != "succeeded" || job.Artifact == nil {
		writeVideoStudioError(w, http.StatusNotFound, "preview_not_found", "Preview not found.")
		return
	}
	path := filepath.Join(manager.previewDir, projectID, jobID+".mp4")
	serveVideoStudioLocalFile(w, r, path, "video/mp4", "preview.mp4")
}

func handleVideoStudioExportFile(s *Server, w http.ResponseWriter, r *http.Request, projectID, filename string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeVideoStudioMethodError(w)
		return
	}
	if !strings.HasSuffix(filename, ".mp4") || !validVideoStudioProjectID(strings.TrimSuffix(filename, ".mp4")) {
		writeVideoStudioError(w, http.StatusNotFound, "export_not_found", "Export not found.")
		return
	}
	_, manager, _, ok := videoStudioReadAccess(s, w, r)
	if !ok {
		return
	}
	jobID := strings.TrimSuffix(filename, ".mp4")
	job := manager.job(jobID)
	if job == nil || job.ProjectID != projectID || job.Kind != "render" || job.Status != "succeeded" || job.Artifact == nil || job.Artifact.Path != "exports/"+filename {
		writeVideoStudioError(w, http.StatusNotFound, "export_not_found", "Export not found.")
		return
	}
	svc, _, err := s.getDesktopService(r.Context())
	if err != nil {
		writeVideoStudioError(w, 503, "desktop_unavailable", "Virtual Desktop storage is unavailable.")
		return
	}
	file, entry, contentType, err := svc.OpenPreviewFile(r.Context(), videoStudioProjectPath(projectID)+"/"+job.Artifact.Path)
	if err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "export_not_found", "Export not found.")
		return
	}
	defer file.Close()
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, job.Artifact.Name, entry.ModTime, file)
}

func serveVideoStudioLocalFile(w http.ResponseWriter, r *http.Request, path, contentType, name string) {
	file, err := os.Open(path)
	if err != nil {
		writeVideoStudioError(w, http.StatusNotFound, "preview_not_found", "Preview not found.")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeVideoStudioError(w, http.StatusNotFound, "preview_not_found", "Preview not found.")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func videoStudioReadAccess(s *Server, w http.ResponseWriter, r *http.Request) (*desktop.Service, *videoStudioManager, *config.Config, bool) {
	if !requireDesktopOperation(s, w, r, desktopScopeRead, desktopRead) {
		return nil, nil, nil, false
	}
	cfg, err := normalizedVideoStudioConfig(s)
	if err != nil {
		writeVideoStudioError(w, 500, "configuration_invalid", "Video Studio configuration is invalid.")
		return nil, nil, nil, false
	}
	if !videoStudioEnabled(cfg, w, false) {
		return nil, nil, nil, false
	}
	svc, _, err := s.getDesktopService(r.Context())
	if err != nil {
		writeVideoStudioError(w, 503, "desktop_unavailable", "Virtual Desktop storage is unavailable.")
		return nil, nil, nil, false
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		writeVideoStudioError(w, 500, "storage_unavailable", "Video Studio storage is unavailable.")
		return nil, nil, nil, false
	}
	return svc, manager, cfg, true
}

func videoStudioMutationAccess(s *Server, w http.ResponseWriter, r *http.Request, scope string) (*desktop.Service, *videoStudioManager, *config.Config, bool) {
	if !requireDesktopOperation(s, w, r, scope, desktopWrite) {
		return nil, nil, nil, false
	}
	cfg, err := normalizedVideoStudioConfig(s)
	if err != nil {
		writeVideoStudioError(w, 500, "configuration_invalid", "Video Studio configuration is invalid.")
		return nil, nil, nil, false
	}
	if !videoStudioEnabled(cfg, w, true) {
		return nil, nil, nil, false
	}
	svc, _, err := s.getDesktopService(r.Context())
	if err != nil {
		writeVideoStudioError(w, 503, "desktop_unavailable", "Virtual Desktop storage is unavailable.")
		return nil, nil, nil, false
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		writeVideoStudioError(w, 500, "storage_unavailable", "Video Studio storage is unavailable.")
		return nil, nil, nil, false
	}
	*r = *r.WithContext(manager.publicationContext(r.Context(), ""))
	return svc, manager, cfg, true
}

func videoStudioEnabled(cfg *config.Config, w http.ResponseWriter, mutation bool) bool {
	if !cfg.VideoStudio.Enabled {
		writeVideoStudioError(w, http.StatusServiceUnavailable, "video_studio_disabled", "Video Studio is disabled in configuration.")
		return false
	}
	if !cfg.VirtualDesktop.Enabled {
		writeVideoStudioError(w, http.StatusServiceUnavailable, "desktop_disabled", "Virtual Desktop is disabled in configuration.")
		return false
	}
	if mutation && cfg.VideoStudio.ReadOnly {
		writeDesktopPolicyError(w, "video_studio_readonly", "Video Studio is read-only.")
		return false
	}
	return true
}

func videoStudioIdempotencyKey(r *http.Request) (string, error) {
	return videoStudioIdempotencyKeyValue(r.Header.Get("Idempotency-Key"))
}
func videoStudioIdempotencyKeyValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "\r\n\x00") {
		return "", fmt.Errorf("idempotency key is required")
	}
	return value, nil
}

func decodeVideoStudioJSON(w http.ResponseWriter, r *http.Request, dst interface{}, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = videoStudioJSONLimit
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}

func containsVideoStudioInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func writeVideoStudioEnqueueError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "asset_count_limit":
		writeVideoStudioError(w, http.StatusRequestEntityTooLarge, "asset_count_limit", "Project already contains the maximum number of media assets.")
	case "idempotency_conflict":
		writeVideoStudioError(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key is already bound to a different request.")
	case "video_studio_queue_full":
		writeVideoStudioError(w, http.StatusTooManyRequests, "job_queue_full", "Video Studio is busy. Try again later.")
	case "video_studio_shutdown":
		writeVideoStudioError(w, http.StatusServiceUnavailable, "video_studio_shutdown", "Video Studio is shutting down.")
	default:
		writeVideoStudioError(w, http.StatusInternalServerError, "job_enqueue_failed", "Video Studio job could not be queued.")
	}
}

func writeVideoStudioFileError(w http.ResponseWriter, err error) {
	var conflict *desktopFileConflict
	if errors.As(err, &conflict) {
		if conflict.Code == "file_precondition_required" {
			writeVideoStudioError(w, http.StatusPreconditionRequired, "precondition_required", "Current project version is required.")
			return
		}
		writeDesktopFileError(w, err)
		return
	}
	writeVideoStudioError(w, http.StatusBadRequest, "project_save_failed", "Project could not be saved.")
}

func writeVideoStudioMethodError(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, HEAD, POST, PUT, DELETE")
	writeVideoStudioError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method is not allowed for this endpoint.")
}

func writeVideoStudioJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeVideoStudioError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "code": code, "message": message})
}
