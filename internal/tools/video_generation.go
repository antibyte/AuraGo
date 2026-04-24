package tools

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/uid"
)

var videoGenHTTPClient = security.NewSSRFProtectedHTTPClient(12 * time.Minute)

const (
	defaultMiniMaxVideoModel = "Hailuo-2.3-768P"
	defaultGoogleVideoModel  = "veo-3.1-generate-preview"
)

// VideoGenParams holds the parameters for the generate_video tool call.
type VideoGenParams struct {
	Prompt          string   `json:"prompt"`
	NegativePrompt  string   `json:"negative_prompt,omitempty"`
	Model           string   `json:"model,omitempty"`
	DurationSeconds int      `json:"duration_seconds,omitempty"`
	Resolution      string   `json:"resolution,omitempty"`
	AspectRatio     string   `json:"aspect_ratio,omitempty"`
	FirstFrameImage string   `json:"first_frame_image,omitempty"`
	LastFrameImage  string   `json:"last_frame_image,omitempty"`
	ReferenceImages []string `json:"reference_images,omitempty"`
}

// VideoGenResult holds the result of a video generation.
type VideoGenResult struct {
	Status       string  `json:"status"`
	Filename     string  `json:"filename,omitempty"`
	FilePath     string  `json:"file_path,omitempty"`
	WebPath      string  `json:"web_path,omitempty"`
	DurationMs   int64   `json:"duration_ms,omitempty"`
	Provider     string  `json:"provider,omitempty"`
	Model        string  `json:"model,omitempty"`
	Format       string  `json:"format,omitempty"`
	FileSize     int64   `json:"file_size,omitempty"`
	MediaID      int64   `json:"media_id,omitempty"`
	TaskID       string  `json:"task_id,omitempty"`
	CostEstimate float64 `json:"cost_estimate,omitempty"`
	Message      string  `json:"message,omitempty"`
	Error        string  `json:"error,omitempty"`
}

type videoDailyCounter struct {
	mu    sync.Mutex
	date  string
	count int
}

var videoCounter = &videoDailyCounter{}

func videoCounterReserve(maxDaily int) (int, bool) {
	videoCounter.mu.Lock()
	defer videoCounter.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if videoCounter.date != today {
		videoCounter.date = today
		videoCounter.count = 0
	}
	if maxDaily > 0 && videoCounter.count >= maxDaily {
		return videoCounter.count, false
	}
	videoCounter.count++
	return videoCounter.count, true
}

func videoCounterRelease() {
	videoCounter.mu.Lock()
	defer videoCounter.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if videoCounter.date == today && videoCounter.count > 0 {
		videoCounter.count--
	}
}

// VideoCounterGet returns the current daily video generation count.
func VideoCounterGet() int {
	videoCounter.mu.Lock()
	defer videoCounter.mu.Unlock()
	if videoCounter.date != time.Now().Format("2006-01-02") {
		return 0
	}
	return videoCounter.count
}

// GenerateVideo is the main entry point for the generate_video tool.
func GenerateVideo(ctx context.Context, cfg *config.Config, mediaDB *sql.DB, logger *slog.Logger, params VideoGenParams) string {
	return mustJSON(GenerateVideoResult(ctx, cfg, mediaDB, logger, params))
}

// VideoResultToJSON serialises a VideoGenResult to JSON.
func VideoResultToJSON(r VideoGenResult) string {
	return mustJSON(r)
}

// GenerateVideoResult runs video generation and returns the structured result.
func GenerateVideoResult(ctx context.Context, cfg *config.Config, mediaDB *sql.DB, logger *slog.Logger, params VideoGenParams) VideoGenResult {
	if params.Prompt == "" {
		return VideoGenResult{Status: "error", Error: "'prompt' is required for video generation."}
	}
	if logger == nil {
		logger = slog.Default()
	}

	providerType := strings.ToLower(cfg.VideoGeneration.ProviderType)
	apiKey := cfg.VideoGeneration.APIKey
	model := cfg.VideoGeneration.ResolvedModel
	if params.Model != "" {
		model = params.Model
	}
	params = applyVideoDefaults(cfg, params)

	logger.Info("Video generation requested", "provider_type", providerType, "prompt_len", len(params.Prompt), "duration_seconds", params.DurationSeconds)

	if apiKey == "" {
		return VideoGenResult{Status: "error", Error: "Video generation provider not configured. Set a provider in Settings > Video Generation."}
	}
	if cfg.VideoGeneration.MaxDaily > 0 {
		count, allowed := videoCounterReserve(cfg.VideoGeneration.MaxDaily)
		if !allowed {
			return VideoGenResult{Status: "error", Error: fmt.Sprintf("Daily video generation limit reached (%d/%d). Try again tomorrow or increase the limit in settings.", count, cfg.VideoGeneration.MaxDaily)}
		}
	}

	videoDir := filepath.Join(cfg.Directories.DataDir, "generated_videos")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		videoCounterRelease()
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to create generated_videos directory: %v", err)}
	}

	var result VideoGenResult
	switch providerType {
	case "minimax":
		result = generateVideoMiniMax(ctx, cfg.VideoGeneration.BaseURL, apiKey, model, params, cfg.VideoGeneration.PollIntervalSeconds, cfg.VideoGeneration.TimeoutSeconds, videoDir)
	case "google", "google_veo":
		result = generateVideoGoogle(ctx, cfg.VideoGeneration.BaseURL, apiKey, model, params, cfg.VideoGeneration.PollIntervalSeconds, cfg.VideoGeneration.TimeoutSeconds, videoDir)
	default:
		videoCounterRelease()
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Unknown video generation provider type: %q. Supported: minimax, google", providerType)}
	}

	if result.Status != "ok" {
		videoCounterRelease()
		return result
	}

	if result.CostEstimate == 0 {
		switch providerType {
		case "minimax":
			result.CostEstimate = 0.20
		case "google", "google_veo":
			result.CostEstimate = 0.50
		}
	}

	if mediaDB != nil {
		fileHash, _ := ComputeMediaFileHash(result.FilePath)
		regID, _, regErr := RegisterMedia(mediaDB, MediaItem{
			MediaType:    "video",
			SourceTool:   "generate_video",
			Filename:     result.Filename,
			FilePath:     result.FilePath,
			WebPath:      result.WebPath,
			FileSize:     result.FileSize,
			Format:       result.Format,
			Provider:     result.Provider,
			Model:        result.Model,
			Prompt:       params.Prompt,
			Description:  truncateString(params.Prompt, 140),
			DurationMs:   result.DurationMs,
			CostEstimate: result.CostEstimate,
			Tags:         []string{"auto-generated", "video"},
			Hash:         fileHash,
		})
		if regErr != nil {
			logger.Warn("Failed to register video in media registry", "error", regErr)
		} else {
			result.MediaID = regID
		}
	}

	return result
}

func applyVideoDefaults(cfg *config.Config, params VideoGenParams) VideoGenParams {
	if params.DurationSeconds <= 0 {
		params.DurationSeconds = cfg.VideoGeneration.DefaultDurationSeconds
	}
	if params.DurationSeconds <= 0 {
		params.DurationSeconds = 6
	}
	if params.Resolution == "" {
		params.Resolution = cfg.VideoGeneration.DefaultResolution
	}
	if params.Resolution == "" {
		params.Resolution = "768P"
	}
	if params.AspectRatio == "" {
		params.AspectRatio = cfg.VideoGeneration.DefaultAspectRatio
	}
	if params.AspectRatio == "" {
		params.AspectRatio = "16:9"
	}
	return params
}

type miniMaxVideoCreateResponse struct {
	TaskID   string `json:"task_id"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	Data map[string]interface{} `json:"data"`
}

type miniMaxVideoStatusResponse struct {
	Status       string `json:"status"`
	FileID       string `json:"file_id"`
	ErrorMessage string `json:"error_message"`
	BaseResp     struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
	Data map[string]interface{} `json:"data"`
}

type miniMaxFileRetrieveResponse struct {
	File struct {
		DownloadURL string `json:"download_url"`
	} `json:"file"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

func generateVideoMiniMax(ctx context.Context, baseURL, apiKey, model string, params VideoGenParams, pollIntervalSeconds, timeoutSeconds int, videoDir string) VideoGenResult {
	if model == "" {
		model = defaultMiniMaxVideoModel
	}
	apiModel, displayModel := miniMaxVideoModelForAPI(model)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.minimax.io/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/video_generation")

	payload := map[string]interface{}{
		"prompt":     params.Prompt,
		"model":      apiModel,
		"duration":   params.DurationSeconds,
		"resolution": strings.ToUpper(params.Resolution),
	}
	if params.AspectRatio != "" {
		payload["aspect_ratio"] = params.AspectRatio
	}
	if params.NegativePrompt != "" {
		payload["negative_prompt"] = params.NegativePrompt
	}
	if params.FirstFrameImage != "" {
		payload["first_frame_image"] = params.FirstFrameImage
	}
	if params.LastFrameImage != "" {
		payload["last_frame_image"] = params.LastFrameImage
	}
	if len(params.ReferenceImages) > 0 {
		payload["subject_reference"] = []map[string]interface{}{{
			"type":  "character",
			"image": params.ReferenceImages,
		}}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to build MiniMax request: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/video_generation", bytes.NewReader(bodyBytes))
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to create MiniMax request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := videoGenHTTPClient.Do(req)
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("MiniMax API request failed: %v", err)}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("MiniMax API returned status %d: %s", resp.StatusCode, truncateString(string(respBody), 500))}
	}

	var createResp miniMaxVideoCreateResponse
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to parse MiniMax response: %v", err)}
	}
	if createResp.BaseResp.StatusCode != 0 {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("MiniMax API error: %s (code %d)", createResp.BaseResp.StatusMsg, createResp.BaseResp.StatusCode)}
	}
	taskID := firstNonEmpty(createResp.TaskID, stringFromAny(createResp.Data["task_id"]))
	if taskID == "" {
		return VideoGenResult{Status: "error", Error: "MiniMax did not return a task_id"}
	}

	fileID, err := pollMiniMaxVideo(ctx, baseURL, apiKey, taskID, pollIntervalSeconds, timeoutSeconds)
	if err != nil {
		return VideoGenResult{Status: "error", TaskID: taskID, Error: err.Error()}
	}

	downloadURL, err := retrieveMiniMaxVideoURL(ctx, baseURL, apiKey, fileID)
	if err != nil {
		return VideoGenResult{Status: "error", TaskID: taskID, Error: err.Error()}
	}

	filename := fmt.Sprintf("video_%s.mp4", uid.New()[:8])
	filePath := filepath.Join(videoDir, filename)
	if err := downloadFileWithHeaders(ctx, downloadURL, filePath, nil); err != nil {
		return VideoGenResult{Status: "error", TaskID: taskID, Error: fmt.Sprintf("Failed to download MiniMax video: %v", err)}
	}
	fileSize := fileSizeOrZero(filePath)

	return VideoGenResult{
		Status:     "ok",
		Filename:   filename,
		FilePath:   filePath,
		WebPath:    "/files/generated_videos/" + filename,
		DurationMs: int64(params.DurationSeconds) * 1000,
		Provider:   "minimax",
		Model:      displayModel,
		Format:     "mp4",
		FileSize:   fileSize,
		TaskID:     taskID,
		Message:    fmt.Sprintf("Video generated successfully with MiniMax (%ds).", params.DurationSeconds),
	}
}

func miniMaxVideoModelForAPI(model string) (apiModel, displayModel string) {
	switch strings.TrimSpace(model) {
	case "", defaultMiniMaxVideoModel:
		return "MiniMax-Hailuo-2.3", defaultMiniMaxVideoModel
	default:
		return model, model
	}
}

func pollMiniMaxVideo(ctx context.Context, baseURL, apiKey, taskID string, pollIntervalSeconds, timeoutSeconds int) (string, error) {
	deadline := time.Now().Add(videoTimeout(timeoutSeconds))
	interval := videoPollInterval(pollIntervalSeconds)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("MiniMax video generation timed out after %s", videoTimeout(timeoutSeconds))
		}
		if err := sleepWithContext(ctx, interval); err != nil {
			return "", err
		}

		req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/query/video_generation?task_id="+url.QueryEscape(taskID), nil)
		if err != nil {
			return "", fmt.Errorf("create MiniMax status request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := videoGenHTTPClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("MiniMax status request failed: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("MiniMax status returned %d: %s", resp.StatusCode, truncateString(string(body), 500))
		}
		var status miniMaxVideoStatusResponse
		if err := json.Unmarshal(body, &status); err != nil {
			return "", fmt.Errorf("parse MiniMax status response: %w", err)
		}
		if status.BaseResp.StatusCode != 0 {
			return "", fmt.Errorf("MiniMax status error: %s (code %d)", status.BaseResp.StatusMsg, status.BaseResp.StatusCode)
		}
		state := strings.ToLower(status.Status)
		switch state {
		case "success", "succeeded", "done":
			fileID := firstNonEmpty(status.FileID, stringFromAny(status.Data["file_id"]))
			if fileID == "" {
				return "", fmt.Errorf("MiniMax completed without file_id")
			}
			return fileID, nil
		case "fail", "failed", "error":
			msg := status.ErrorMessage
			if msg == "" {
				msg = stringFromAny(status.Data["error_message"])
			}
			if msg == "" {
				msg = "unknown error"
			}
			return "", fmt.Errorf("MiniMax video generation failed: %s", msg)
		}
	}
}

func retrieveMiniMaxVideoURL(ctx context.Context, baseURL, apiKey, fileID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/files/retrieve?file_id="+url.QueryEscape(fileID), nil)
	if err != nil {
		return "", fmt.Errorf("create MiniMax retrieve request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := videoGenHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("MiniMax retrieve request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MiniMax retrieve returned %d: %s", resp.StatusCode, truncateString(string(body), 500))
	}
	var retrieve miniMaxFileRetrieveResponse
	if err := json.Unmarshal(body, &retrieve); err != nil {
		return "", fmt.Errorf("parse MiniMax retrieve response: %w", err)
	}
	if retrieve.BaseResp.StatusCode != 0 {
		return "", fmt.Errorf("MiniMax retrieve error: %s (code %d)", retrieve.BaseResp.StatusMsg, retrieve.BaseResp.StatusCode)
	}
	if retrieve.File.DownloadURL == "" {
		return "", fmt.Errorf("MiniMax retrieve response did not include a download_url")
	}
	return retrieve.File.DownloadURL, nil
}

type googleVeoOperation struct {
	Name     string                 `json:"name"`
	Done     bool                   `json:"done"`
	Error    *googleVeoError        `json:"error,omitempty"`
	Response map[string]interface{} `json:"response,omitempty"`
}

type googleVeoError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func generateVideoGoogle(ctx context.Context, baseURL, apiKey, model string, params VideoGenParams, pollIntervalSeconds, timeoutSeconds int, videoDir string) VideoGenResult {
	if model == "" {
		model = defaultGoogleVideoModel
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	payload := buildGoogleVideoPayload(params)
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to build Google Veo request: %v", err)}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/models/%s:predictLongRunning", baseURL, model), bytes.NewReader(bodyBytes))
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to create Google Veo request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := videoGenHTTPClient.Do(req)
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Google Veo API request failed: %v", err)}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Google Veo API returned status %d: %s", resp.StatusCode, truncateString(string(respBody), 500))}
	}

	var operation googleVeoOperation
	if err := json.Unmarshal(respBody, &operation); err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to parse Google Veo operation: %v", err)}
	}
	if operation.Name == "" {
		return VideoGenResult{Status: "error", Error: "Google Veo did not return an operation name"}
	}

	done, err := pollGoogleVideo(ctx, baseURL, apiKey, operation.Name, pollIntervalSeconds, timeoutSeconds)
	if err != nil {
		return VideoGenResult{Status: "error", TaskID: operation.Name, Error: err.Error()}
	}

	filename := fmt.Sprintf("video_%s.mp4", uid.New()[:8])
	filePath := filepath.Join(videoDir, filename)
	if err := saveGoogleVideoOutput(ctx, apiKey, done.Response, filePath); err != nil {
		return VideoGenResult{Status: "error", TaskID: operation.Name, Error: err.Error()}
	}
	fileSize := fileSizeOrZero(filePath)

	return VideoGenResult{
		Status:     "ok",
		Filename:   filename,
		FilePath:   filePath,
		WebPath:    "/files/generated_videos/" + filename,
		DurationMs: int64(params.DurationSeconds) * 1000,
		Provider:   "google",
		Model:      model,
		Format:     "mp4",
		FileSize:   fileSize,
		TaskID:     operation.Name,
		Message:    fmt.Sprintf("Video generated successfully with Google Veo (%ds).", params.DurationSeconds),
	}
}

func buildGoogleVideoPayload(params VideoGenParams) map[string]interface{} {
	instance := map[string]interface{}{"prompt": params.Prompt}
	if params.FirstFrameImage != "" {
		instance["image"] = googleVideoImagePart(params.FirstFrameImage)
	}
	payload := map[string]interface{}{
		"instances": []map[string]interface{}{instance},
		"parameters": map[string]interface{}{
			"durationSeconds": params.DurationSeconds,
			"aspectRatio":     params.AspectRatio,
			"sampleCount":     1,
		},
	}
	if params.NegativePrompt != "" {
		payload["parameters"].(map[string]interface{})["negativePrompt"] = params.NegativePrompt
	}
	return payload
}

func googleVideoImagePart(value string) map[string]interface{} {
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return map[string]interface{}{"uri": value}
	}
	return map[string]interface{}{"bytesBase64Encoded": value, "mimeType": "image/png"}
}

func pollGoogleVideo(ctx context.Context, baseURL, apiKey, operationName string, pollIntervalSeconds, timeoutSeconds int) (googleVeoOperation, error) {
	deadline := time.Now().Add(videoTimeout(timeoutSeconds))
	interval := videoPollInterval(pollIntervalSeconds)
	for {
		if time.Now().After(deadline) {
			return googleVeoOperation{}, fmt.Errorf("Google Veo generation timed out after %s", videoTimeout(timeoutSeconds))
		}
		if err := sleepWithContext(ctx, interval); err != nil {
			return googleVeoOperation{}, err
		}

		req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/"+strings.TrimPrefix(operationName, "/"), nil)
		if err != nil {
			return googleVeoOperation{}, fmt.Errorf("create Google Veo status request: %w", err)
		}
		req.Header.Set("x-goog-api-key", apiKey)
		resp, err := videoGenHTTPClient.Do(req)
		if err != nil {
			return googleVeoOperation{}, fmt.Errorf("Google Veo status request failed: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return googleVeoOperation{}, fmt.Errorf("Google Veo status returned %d: %s", resp.StatusCode, truncateString(string(body), 500))
		}
		var op googleVeoOperation
		if err := json.Unmarshal(body, &op); err != nil {
			return googleVeoOperation{}, fmt.Errorf("parse Google Veo status response: %w", err)
		}
		if op.Error != nil {
			return googleVeoOperation{}, fmt.Errorf("Google Veo generation failed: %s (code %d)", op.Error.Message, op.Error.Code)
		}
		if op.Done {
			return op, nil
		}
	}
}

func saveGoogleVideoOutput(ctx context.Context, apiKey string, response map[string]interface{}, filePath string) error {
	video := findGoogleVideoObject(response)
	if len(video) == 0 {
		return fmt.Errorf("Google Veo completed without video output")
	}
	if data := firstNonEmpty(stringFromAny(video["bytesBase64Encoded"]), stringFromAny(video["data"])); data != "" {
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return fmt.Errorf("decode Google Veo video data: %w", err)
		}
		return os.WriteFile(filePath, decoded, 0644)
	}
	uri := firstNonEmpty(stringFromAny(video["uri"]), stringFromAny(video["downloadUri"]), stringFromAny(video["url"]))
	if uri == "" {
		return fmt.Errorf("Google Veo video output did not include bytes or a URI")
	}
	return downloadFileWithHeaders(ctx, uri, filePath, map[string]string{"x-goog-api-key": apiKey})
}

func findGoogleVideoObject(root map[string]interface{}) map[string]interface{} {
	if root == nil {
		return nil
	}
	if generated, ok := root["generatedVideos"].([]interface{}); ok && len(generated) > 0 {
		if item, ok := generated[0].(map[string]interface{}); ok {
			if video, ok := item["video"].(map[string]interface{}); ok {
				return video
			}
			return item
		}
	}
	if videos, ok := root["videos"].([]interface{}); ok && len(videos) > 0 {
		if video, ok := videos[0].(map[string]interface{}); ok {
			return video
		}
	}
	if video, ok := root["video"].(map[string]interface{}); ok {
		return video
	}
	return nil
}

func downloadFileWithHeaders(ctx context.Context, rawURL, dest string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := videoGenHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func videoPollInterval(seconds int) time.Duration {
	if seconds < 0 {
		return 0
	}
	if seconds == 0 {
		seconds = 10
	}
	return time.Duration(seconds) * time.Second
}

func videoTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = 600
	}
	return time.Duration(seconds) * time.Second
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func stringFromAny(v interface{}) string {
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fileSizeOrZero(path string) int64 {
	stat, err := os.Stat(path)
	if err != nil || stat == nil {
		return 0
	}
	return stat.Size()
}

// TestVideoConnection performs a lightweight API connectivity test without generating video.
func TestVideoConnection(ctx context.Context, provider, apiKey, baseURL string) (bool, string) {
	switch strings.ToLower(provider) {
	case "minimax":
		baseURL = strings.TrimRight(baseURL, "/")
		if baseURL == "" {
			baseURL = "https://api.minimax.io/v1"
		}
		baseURL = strings.TrimSuffix(baseURL, "/video_generation")
		reqBody := map[string]interface{}{
			"model":      "MiniMax-Hailuo-2.3",
			"duration":   6,
			"resolution": "768P",
		}
		bodyBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/video_generation", bytes.NewReader(bodyBytes))
		if err != nil {
			return false, fmt.Sprintf("Request creation failed: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := videoGenHTTPClient.Do(req)
		if err != nil {
			return false, fmt.Sprintf("Connection failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return false, "Authentication failed - check your API key"
		}
		if resp.StatusCode < 500 {
			return true, "Connection successful"
		}
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Sprintf("API returned status %d: %s", resp.StatusCode, truncateString(string(body), 200))
	case "google", "google_veo":
		baseURL = strings.TrimRight(baseURL, "/")
		if baseURL == "" {
			baseURL = "https://generativelanguage.googleapis.com/v1beta"
		}
		req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/models", nil)
		if err != nil {
			return false, fmt.Sprintf("Request creation failed: %v", err)
		}
		req.Header.Set("x-goog-api-key", apiKey)
		resp, err := videoGenHTTPClient.Do(req)
		if err != nil {
			return false, fmt.Sprintf("Connection failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true, "Connection successful"
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return false, "Authentication failed - check your API key"
		}
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Sprintf("API returned status %d: %s", resp.StatusCode, truncateString(string(body), 200))
	default:
		return false, fmt.Sprintf("Unsupported video provider: %s", provider)
	}
}
