package tools

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
)

const videoDownloadContainerDir = "/downloads"
const maxVideoSearchDescriptionLength = 200

type VideoDownloadRequest struct {
	Operation string
	URL       string
	Query     string
	Format    string
	Quality   string
}

type videoDownloadResult struct {
	Status        string                 `json:"status"`
	Operation     string                 `json:"operation"`
	Mode          string                 `json:"mode,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Results       []videoSearchResult    `json:"results,omitempty"`
	Info          map[string]interface{} `json:"info,omitempty"`
	FilePath      string                 `json:"file_path,omitempty"`
	WebPath       string                 `json:"web_path,omitempty"`
	MediaID       int64                  `json:"media_id,omitempty"`
	MediaType     string                 `json:"media_type,omitempty"`
	FileSizeBytes int64                  `json:"file_size_bytes,omitempty"`
	Transcript    string                 `json:"transcript,omitempty"`
	STTCost       float64                `json:"stt_cost,omitempty"`
}

type videoSearchResult struct {
	ID          string  `json:"id,omitempty"`
	Title       string  `json:"title,omitempty"`
	URL         string  `json:"url,omitempty"`
	Uploader    string  `json:"uploader,omitempty"`
	Duration    float64 `json:"duration,omitempty"`
	ViewCount   int64   `json:"view_count,omitempty"`
	Description string  `json:"description,omitempty"`
	Thumbnail   string  `json:"thumbnail,omitempty"`
}

// DispatchVideoDownload executes the video_download tool using yt-dlp. Docker mode is the default;
// native mode requires yt-dlp to be installed on the host or configured via tools.video_download.yt_dlp_path.
func DispatchVideoDownload(ctx context.Context, cfg *config.Config, mediaDB *sql.DB, req VideoDownloadRequest, logger *slog.Logger) string {
	if cfg == nil {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Message: "config is required"})
	}
	op := strings.ToLower(strings.TrimSpace(req.Operation))
	if op == "" {
		op = "info"
	}
	if cfg.Tools.VideoDownload.ReadOnly && (op == "download" || op == "transcribe") {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: op, Message: "video_download is in read-only mode; only search and info are allowed"})
	}

	switch op {
	case "search":
		return videoDownloadSearch(ctx, cfg, req, logger)
	case "info":
		return videoDownloadInfo(ctx, cfg, req, logger)
	case "download":
		return videoDownloadFile(ctx, cfg, mediaDB, req, false, logger)
	case "transcribe":
		return videoDownloadFile(ctx, cfg, mediaDB, req, true, logger)
	default:
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: op, Message: "unknown operation; use search, info, download, or transcribe"})
	}
}

func videoDownloadSearch(ctx context.Context, cfg *config.Config, req VideoDownloadRequest, logger *slog.Logger) string {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: "search", Message: "query is required for search"})
	}
	maxResults := cfg.Tools.VideoDownload.MaxSearchResults
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 25 {
		maxResults = 25
	}
	args := []string{"--dump-json", "--flat-playlist", "--no-warnings", fmt.Sprintf("ytsearch%d:%s", maxResults, query)}
	stdout, stderr, err := runYtDlp(ctx, cfg, args, logger)
	if err != nil {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: "search", Mode: videoDownloadMode(cfg), Message: commandErrMessage(err, stderr)})
	}
	items := parseYtDlpSearch(stdout)
	return videoDownloadJSON(videoDownloadResult{Status: "ok", Operation: "search", Mode: videoDownloadMode(cfg), Results: items})
}

func videoDownloadInfo(ctx context.Context, cfg *config.Config, req VideoDownloadRequest, logger *slog.Logger) string {
	videoURL := strings.TrimSpace(req.URL)
	if videoURL == "" {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: "info", Message: "url is required for info"})
	}
	info, stderr, err := fetchYtDlpInfo(ctx, cfg, videoURL, logger)
	if err != nil {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: "info", Mode: videoDownloadMode(cfg), Message: commandErrMessage(err, stderr)})
	}
	return videoDownloadJSON(videoDownloadResult{Status: "ok", Operation: "info", Mode: videoDownloadMode(cfg), Info: compactVideoInfo(info)})
}

func videoDownloadFile(ctx context.Context, cfg *config.Config, mediaDB *sql.DB, req VideoDownloadRequest, transcribe bool, logger *slog.Logger) string {
	videoURL := strings.TrimSpace(req.URL)
	if videoURL == "" {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: req.Operation, Message: "url is required"})
	}
	downloadDir, err := resolveVideoDownloadDir(cfg)
	if err != nil {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: req.Operation, Message: err.Error()})
	}
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: req.Operation, Message: fmt.Sprintf("create download directory: %v", err)})
	}

	formatMode := strings.ToLower(strings.TrimSpace(req.Format))
	if transcribe && formatMode == "" {
		formatMode = "audio"
	}
	outputDir := downloadDir
	if videoDownloadMode(cfg) == "docker" {
		outputDir = videoDownloadContainerDir
	}
	args := buildYtDlpDownloadArgs(cfg, req, videoURL, formatMode, outputDir)
	stdout, stderr, err := runYtDlp(ctx, cfg, args, logger)
	if err != nil {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: req.Operation, Mode: videoDownloadMode(cfg), Message: commandErrMessage(err, stderr)})
	}
	filePath, err := resolveDownloadedFilePath(cfg, downloadDir, stdout)
	if err != nil {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: req.Operation, Mode: videoDownloadMode(cfg), Message: err.Error()})
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: req.Operation, Mode: videoDownloadMode(cfg), Message: fmt.Sprintf("stat downloaded file: %v", err)})
	}
	if err := enforceVideoDownloadSize(cfg, info.Size()); err != nil {
		_ = os.Remove(filePath)
		return videoDownloadJSON(videoDownloadResult{Status: "error", Operation: req.Operation, Mode: videoDownloadMode(cfg), Message: err.Error()})
	}
	mediaID, webPath, mediaType := registerVideoDownloadMedia(mediaDB, filePath, info.Size())

	result := videoDownloadResult{
		Status:        "ok",
		Operation:     req.Operation,
		Mode:          videoDownloadMode(cfg),
		FilePath:      filePath,
		WebPath:       webPath,
		MediaID:       mediaID,
		MediaType:     mediaType,
		FileSizeBytes: info.Size(),
	}
	if transcribe {
		transcript, cost, err := TranscribeAudioFile(filePath, cfg)
		if err != nil {
			result.Status = "partial"
			result.Message = fmt.Sprintf("download succeeded but transcription failed: %v", err)
			return videoDownloadJSON(result)
		}
		result.Transcript = transcript
		result.STTCost = cost
	}
	return videoDownloadJSON(result)
}

func fetchYtDlpInfo(ctx context.Context, cfg *config.Config, videoURL string, logger *slog.Logger) (map[string]interface{}, string, error) {
	stdout, stderr, err := runYtDlp(ctx, cfg, []string{"--dump-json", "--no-download", "--no-playlist", "--no-warnings", videoURL}, logger)
	if err != nil {
		return nil, stderr, err
	}
	var info map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &info); err != nil {
		return nil, stderr, fmt.Errorf("parse yt-dlp info JSON: %w", err)
	}
	return info, stderr, nil
}

func buildYtDlpDownloadArgs(cfg *config.Config, req VideoDownloadRequest, videoURL, formatMode, outputDir string) []string {
	format := strings.TrimSpace(req.Format)
	quality := strings.ToLower(strings.TrimSpace(req.Quality))
	if format == "" || formatMode == "video" || formatMode == "audio" {
		format = strings.TrimSpace(cfg.Tools.VideoDownload.DefaultFormat)
	}
	if format == "" {
		format = "best"
	}
	if quality == "medium" && formatMode != "audio" && (format == "best" || format == "") {
		format = "bestvideo[height<=720]+bestaudio/best[height<=720]/best"
	} else if quality == "low" && formatMode != "audio" && (format == "best" || format == "") {
		format = "bestvideo[height<=480]+bestaudio/best[height<=480]/best"
	}

	if strings.TrimSpace(outputDir) == "" {
		outputDir = videoDownloadContainerDir
	}
	outputTemplate := strings.TrimRight(filepath.ToSlash(filepath.Clean(outputDir)), "/") + "/%(title).200B [%(id)s].%(ext)s"
	args := []string{"--no-playlist", "--no-progress", "--newline", "--restrict-filenames", "--print", "after_move:filepath", "-o", outputTemplate}
	if formatMode == "audio" || format == "bestaudio" {
		args = append(args, "-f", "bestaudio/best", "-x", "--audio-format", "mp3")
	} else {
		args = append(args, "-f", format)
	}
	args = append(args, videoURL)
	return args
}

func runYtDlp(ctx context.Context, cfg *config.Config, args []string, logger *slog.Logger) (string, string, error) {
	timeout := time.Duration(cfg.Tools.VideoDownload.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if videoDownloadMode(cfg) == "native" {
		return nativeRunYtDlp(cmdCtx, cfg, args)
	}
	return dockerRunYtDlp(cmdCtx, cfg, args, logger)
}

func nativeRunYtDlp(ctx context.Context, cfg *config.Config, args []string) (string, string, error) {
	binary := strings.TrimSpace(cfg.Tools.VideoDownload.YtDlpPath)
	if binary == "" {
		var err error
		binary, err = exec.LookPath("yt-dlp")
		if err != nil {
			return "", "", fmt.Errorf("native mode requires yt-dlp installed on the host or tools.video_download.yt_dlp_path configured: %w", err)
		}
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	if downloadDir, err := resolveVideoDownloadDir(cfg); err == nil {
		cmd.Dir = downloadDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return stdout.String(), stderr.String(), ctx.Err()
		}
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

func dockerRunYtDlp(ctx context.Context, cfg *config.Config, args []string, logger *slog.Logger) (string, string, error) {
	image := strings.TrimSpace(cfg.Tools.VideoDownload.ContainerImage)
	if image == "" {
		image = "ghcr.io/jauderho/yt-dlp:latest"
	}
	dockerCfg := DockerConfig{Host: cfg.Docker.Host, WorkspaceDir: cfg.Directories.WorkspaceDir}
	if err := DockerPing(dockerCfg.Host); err != nil {
		return "", "", fmt.Errorf("Docker is required for video_download docker mode: %w", err)
	}
	if cfg.Tools.VideoDownload.AutoPull {
		if err := PullImageWait(ctx, dockerCfg, image, logger); err != nil {
			return "", "", err
		}
	}
	downloadDir, err := resolveVideoDownloadDir(cfg)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", "", fmt.Errorf("create download directory: %w", err)
	}

	name := fmt.Sprintf("aurago_yt_dlp_%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"Image":      image,
		"Cmd":        args,
		"WorkingDir": videoDownloadContainerDir,
		"HostConfig": map[string]interface{}{
			"Binds":       []string{downloadDir + ":" + videoDownloadContainerDir},
			"NetworkMode": "bridge",
		},
	}
	payload, _ := json.Marshal(body)
	data, code, err := dockerRequestContext(ctx, dockerCfg, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), string(payload))
	if err != nil {
		return "", "", fmt.Errorf("create yt-dlp container: %w", err)
	}
	if code != http.StatusCreated {
		return "", "", fmt.Errorf("create yt-dlp container: %s", dockerBodyErr(code, data))
	}
	var createResp struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(data, &createResp); err != nil || createResp.ID == "" {
		return "", "", fmt.Errorf("parse Docker container create response: %w", err)
	}
	containerID := createResp.ID
	defer func() {
		_, _, _ = dockerRequestContext(context.Background(), dockerCfg, http.MethodDelete, "/containers/"+url.PathEscape(containerID)+"?force=true&v=true", "")
	}()

	data, code, err = dockerRequestContext(ctx, dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/start", "")
	if err != nil {
		return "", "", fmt.Errorf("start yt-dlp container: %w", err)
	}
	if code != http.StatusNoContent && code != http.StatusNotModified {
		return "", "", fmt.Errorf("start yt-dlp container: %s", dockerBodyErr(code, data))
	}

	data, code, err = dockerRequestContext(ctx, dockerCfg, http.MethodPost, "/containers/"+url.PathEscape(containerID)+"/wait?condition=not-running", "")
	if err != nil {
		return "", "", fmt.Errorf("wait for yt-dlp container: %w", err)
	}
	if code != http.StatusOK {
		return "", "", fmt.Errorf("wait for yt-dlp container: %s", dockerBodyErr(code, data))
	}
	var waitResp struct {
		StatusCode int `json:"StatusCode"`
	}
	_ = json.Unmarshal(data, &waitResp)
	logs, _, logErr := dockerRequestContext(context.Background(), dockerCfg, http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/logs?stdout=true&stderr=true", "")
	stdout, stderr := splitDockerLogStreams(logs)
	if logErr != nil {
		stderr += "\n" + logErr.Error()
	}
	if waitResp.StatusCode != 0 {
		return stdout, stderr, fmt.Errorf("yt-dlp exited with status %d", waitResp.StatusCode)
	}
	return stdout, stderr, nil
}

func dockerRequestContext(ctx context.Context, cfg DockerConfig, method, endpoint, body string) ([]byte, int, error) {
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://localhost/"+dockerAPIVersion+endpoint, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := getPullDockerClient(cfg).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseSize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

func splitDockerLogStreams(raw []byte) (string, string) {
	if len(raw) < 8 {
		return string(raw), ""
	}
	var stdout, stderr strings.Builder
	for len(raw) >= 8 {
		streamType := raw[0]
		size := int(uint32(raw[4])<<24 | uint32(raw[5])<<16 | uint32(raw[6])<<8 | uint32(raw[7]))
		raw = raw[8:]
		if size <= 0 || size > len(raw) {
			return string(raw), ""
		}
		chunk := string(raw[:size])
		if streamType == 2 {
			stderr.WriteString(chunk)
		} else {
			stdout.WriteString(chunk)
		}
		raw = raw[size:]
	}
	return stdout.String(), stderr.String()
}

func parseYtDlpSearch(output string) []videoSearchResult {
	var results []videoSearchResult
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var raw map[string]interface{}
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		item := videoSearchResult{
			ID:          stringFromMap(raw, "id"),
			Title:       stringFromMap(raw, "title"),
			URL:         stringFromMap(raw, "url"),
			Uploader:    stringFromMap(raw, "uploader"),
			Duration:    floatFromMap(raw, "duration"),
			ViewCount:   intFromMap(raw, "view_count"),
			Description: truncateVideoSearchDescription(stringFromMap(raw, "description")),
			Thumbnail:   stringFromMap(raw, "thumbnail"),
		}
		if item.URL == "" && item.ID != "" {
			item.URL = "https://www.youtube.com/watch?v=" + item.ID
		}
		results = append(results, item)
	}
	return results
}

func truncateVideoSearchDescription(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxVideoSearchDescriptionLength {
		return value
	}
	return strings.TrimSpace(value[:maxVideoSearchDescriptionLength]) + "..."
}

func compactVideoInfo(info map[string]interface{}) map[string]interface{} {
	keys := []string{"id", "title", "webpage_url", "uploader", "channel", "duration", "view_count", "upload_date", "thumbnail", "description", "filesize", "filesize_approx", "ext"}
	out := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		if val, ok := info[key]; ok {
			out[key] = val
		}
	}
	return out
}

func resolveVideoDownloadDir(cfg *config.Config) (string, error) {
	dir := strings.TrimSpace(cfg.Tools.VideoDownload.DownloadDir)
	if dir == "" {
		dir = filepath.Join(cfg.Directories.DataDir, "downloads")
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir), nil
	}
	base := strings.TrimSpace(cfg.Directories.WorkspaceDir)
	if base == "" {
		return "", fmt.Errorf("workspace_dir is not configured")
	}
	root := detectFilesystemProjectRoot(base)
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(dir))), nil
}

// ResolveVideoDownloadDir returns the host directory used by the video_download tool.
func ResolveVideoDownloadDir(cfg *config.Config) (string, error) {
	return resolveVideoDownloadDir(cfg)
}

func resolveDownloadedFilePath(cfg *config.Config, downloadDir, stdout string) (string, error) {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		candidate := line
		if videoDownloadMode(cfg) == "docker" && strings.HasPrefix(filepath.ToSlash(candidate), videoDownloadContainerDir+"/") {
			candidate = filepath.Join(downloadDir, strings.TrimPrefix(filepath.ToSlash(candidate), videoDownloadContainerDir+"/"))
		}
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(downloadDir, candidate)
		}
		var err error
		candidate, err = ensurePathInsideDir(downloadDir, candidate)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("yt-dlp did not report a downloaded file path")
}

func ensurePathInsideDir(baseDir, candidate string) (string, error) {
	baseAbs, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", fmt.Errorf("resolve download directory: %w", err)
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", fmt.Errorf("resolve downloaded file path: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return "", fmt.Errorf("compare downloaded file path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("resolved file path escapes download directory")
	}
	return candidateAbs, nil
}

func enforceVideoDownloadSize(cfg *config.Config, size int64) error {
	limitMB := cfg.Tools.VideoDownload.MaxFileSizeMB
	if limitMB <= 0 {
		return nil
	}
	limit := int64(limitMB) * 1024 * 1024
	if size > limit {
		return fmt.Errorf("downloaded file size %d bytes exceeds max_file_size_mb=%d", size, limitMB)
	}
	return nil
}

func registerVideoDownloadMedia(db *sql.DB, filePath string, size int64) (int64, string, string) {
	filename := filepath.Base(filePath)
	mediaType := inferMediaType(filename, filePath)
	webPath := "/files/downloads/" + url.PathEscape(filename)
	if db == nil {
		return 0, webPath, mediaType
	}
	hash, _ := ComputeMediaFileHash(filePath)
	id, _, err := RegisterMedia(db, MediaItem{
		MediaType:   mediaType,
		SourceTool:  "video_download",
		Filename:    filename,
		FilePath:    filePath,
		WebPath:     webPath,
		FileSize:    size,
		Format:      strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), "."),
		Description: "Downloaded with yt-dlp",
		Tags:        []string{"yt-dlp", "download"},
		Hash:        hash,
	})
	if err != nil {
		return 0, webPath, mediaType
	}
	return id, webPath, mediaType
}

func videoDownloadMode(cfg *config.Config) string {
	mode := strings.ToLower(strings.TrimSpace(cfg.Tools.VideoDownload.Mode))
	if mode == "native" {
		return "native"
	}
	return "docker"
}

func commandErrMessage(err error, stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return err.Error()
	}
	return err.Error() + ": " + stderr
}

func videoDownloadJSON(result videoDownloadResult) string {
	b, err := json.Marshal(result)
	if err != nil {
		fallback, fallbackErr := json.Marshal(videoDownloadResult{Status: "error", Message: fmt.Sprintf("encode video_download result: %v", err)})
		if fallbackErr != nil {
			return `{"status":"error","message":"encode video_download result failed"}`
		}
		return string(fallback)
	}
	return string(b)
}

func stringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func floatFromMap(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

func intFromMap(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}
