package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"aurago/internal/uid"
)

const (
	defaultAgnesVideoModel = "agnes-video-2.5-flash"
	AgnesVideoMinSeconds   = 4
	AgnesVideoMaxSeconds   = 12
)

type agnesVideoState struct {
	TaskID  string
	VideoID string
	Status  string
	URL     string
	Error   string
	Model   string
	Size    string
	Seconds float64
}

func AgnesVideoModel(model string) string {
	model = strings.TrimSpace(model)
	switch strings.ToLower(model) {
	case "agnes-video-2.5", defaultAgnesVideoModel:
		return strings.ToLower(model)
	case "agnes-video-v2.0":
		return defaultAgnesVideoModel
	}
	if strings.HasPrefix(strings.ToLower(model), "agnes-video-") {
		return model
	}
	return defaultAgnesVideoModel
}

func agnesVideoSizes(model string) []string {
	if AgnesVideoModel(model) == defaultAgnesVideoModel {
		return []string{"720P"}
	}
	return []string{"720P", "1080P", "1K", "2K"}
}

func generateVideoAgnes(ctx context.Context, baseURL, apiKey, model string, params VideoGenParams, pollIntervalSeconds, timeoutSeconds int, videoDir string) VideoGenResult {
	model = AgnesVideoModel(model)
	payload, err := agnesVideoPayload(model, params)
	if err != nil {
		return VideoGenResult{Status: "error", Error: err.Error()}
	}

	apiBase, resultEndpoint := agnesVideoEndpoints(baseURL)
	body, err := json.Marshal(payload)
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to marshal Agnes AI video request: %v", err)}
	}
	reached := &atomic.Bool{}
	req, err := http.NewRequestWithContext(withVideoProviderCreateTrace(ctx, reached), http.MethodPost, apiBase+"/videos", bytes.NewReader(body))
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to create Agnes AI video request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := videoGenHTTPClient.Do(req)
	if err != nil {
		return videoCreateTransportFailure(err, reached, "Agnes AI video request failed: %v")
	}
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return videoCreateHTTPFailure(resp.StatusCode, fmt.Sprintf("Agnes AI video API returned status %d: %s", resp.StatusCode, truncateString(string(respBody), 500)))
	}
	if readErr != nil {
		return videoTaskOutcomeUnknown("")
	}

	state, err := parseAgnesVideoState(respBody)
	if err != nil {
		return videoTaskOutcomeUnknown("")
	}
	if state.failed() {
		return VideoGenResult{Status: "error", Error: "Agnes AI reported that generation failed.", TaskID: firstNonEmpty(state.VideoID, state.TaskID), ProviderFailure: true}
	}
	if state.TaskID == "" && state.VideoID == "" {
		return videoTaskOutcomeUnknown("")
	}
	if state.URL == "" {
		state.Model = model
		state, err = pollAgnesVideo(ctx, apiBase, resultEndpoint, apiKey, state, pollIntervalSeconds, timeoutSeconds)
		if err != nil {
			taskID := firstNonEmpty(state.VideoID, state.TaskID)
			if state.failed() {
				return VideoGenResult{Status: "error", Error: "Agnes AI reported that generation failed.", TaskID: taskID, ProviderFailure: true}
			}
			return videoTaskOutcomeUnknown(taskID)
		}
	}

	filename := fmt.Sprintf("video_%s.mp4", uid.New()[:8])
	filePath := filepath.Join(videoDir, filename)
	if err := downloadFileWithHeaders(ctx, state.URL, filePath, nil); err != nil {
		return videoTaskOutcomeUnknown(firstNonEmpty(state.VideoID, state.TaskID))
	}

	durationSeconds := float64(params.DurationSeconds)
	if state.Seconds > 0 {
		durationSeconds = state.Seconds
	}
	durationMs := int64(durationSeconds*1000 + 0.5)
	if state.Model != "" {
		model = state.Model
	}
	outputSize := firstNonEmpty(state.Size, payload.Size)
	return VideoGenResult{
		Status:     "ok",
		Filename:   filename,
		FilePath:   filePath,
		WebPath:    "/files/generated_videos/" + filename,
		DurationMs: durationMs,
		Provider:   "agnes",
		Model:      model,
		Size:       outputSize,
		Format:     "mp4",
		FileSize:   fileSizeOrZero(filePath),
		TaskID:     firstNonEmpty(state.VideoID, state.TaskID),
		Message:    fmt.Sprintf("Video generated successfully with Agnes AI (%ss).", strconv.FormatFloat(durationSeconds, 'f', -1, 64)),
	}
}

func agnesVideoEndpoints(rawBaseURL string) (apiBase, resultEndpoint string) {
	apiBase = strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
	if apiBase == "" {
		apiBase = "https://apihub.agnes-ai.com/v1"
	}
	apiBase = strings.TrimSuffix(apiBase, "/videos")
	if !strings.HasSuffix(apiBase, "/v1") {
		apiBase += "/v1"
	}
	resultBase := strings.TrimSuffix(apiBase, "/v1")
	return apiBase, resultBase + "/agnesapi"
}

func normalizeAgnesVideoSize(size string) string {
	switch size = strings.ToUpper(strings.TrimSpace(size)); size {
	case "", "480P", "768P":
		return "720P"
	default:
		return size
	}
}

type agnesVideoRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Mode        string   `json:"mode"`
	Seconds     string   `json:"seconds"`
	Size        string   `json:"size"`
	AspectRatio string   `json:"aspect_ratio"`
	FirstFrame  string   `json:"first_frame,omitempty"`
	LastFrame   string   `json:"last_frame,omitempty"`
	Images      []string `json:"images,omitempty"`
}

func agnesVideoPayload(model string, params VideoGenParams) (agnesVideoRequest, error) {
	payload := agnesVideoRequest{
		Model:       model,
		Prompt:      params.Prompt,
		Mode:        "text",
		Seconds:     strconv.Itoa(params.DurationSeconds),
		Size:        normalizeAgnesVideoSize(params.Resolution),
		AspectRatio: strings.TrimSpace(params.AspectRatio),
		FirstFrame:  strings.TrimSpace(params.FirstFrameImage),
		LastFrame:   strings.TrimSpace(params.LastFrameImage),
	}
	if params.DurationSeconds < AgnesVideoMinSeconds || params.DurationSeconds > AgnesVideoMaxSeconds {
		return payload, fmt.Errorf("Agnes AI video duration must be between %d and %d seconds", AgnesVideoMinSeconds, AgnesVideoMaxSeconds)
	}
	sizes := agnesVideoSizes(model)
	if !slices.Contains(sizes, payload.Size) {
		return payload, fmt.Errorf("Agnes AI model %s supports these video resolutions: %s", model, strings.Join(sizes, ", "))
	}
	if !slices.Contains([]string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}, payload.AspectRatio) {
		return payload, fmt.Errorf("Agnes AI does not support video aspect ratio %q", payload.AspectRatio)
	}
	if negative := strings.TrimSpace(params.NegativePrompt); negative != "" {
		payload.Prompt += "\nAvoid: " + negative
	}
	if len(params.ReferenceImages) > 0 {
		if payload.FirstFrame != "" || payload.LastFrame != "" {
			return payload, fmt.Errorf("Agnes AI reference_images cannot be combined with first_frame_image or last_frame_image")
		}
		maxImages := 8
		if model == defaultAgnesVideoModel {
			maxImages = 5
		}
		if len(params.ReferenceImages) > maxImages {
			return payload, fmt.Errorf("Agnes AI model %s supports at most %d reference images", model, maxImages)
		}
		payload.Mode = "reference"
		payload.Images = make([]string, 0, len(params.ReferenceImages))
		for i, image := range params.ReferenceImages {
			image = strings.TrimSpace(image)
			if err := validatePublicImageURL(image); err != nil {
				return payload, fmt.Errorf("Agnes AI reference_images[%d] must be a publicly reachable HTTP(S) URL: %w", i, err)
			}
			payload.Images = append(payload.Images, image)
		}
	} else if payload.FirstFrame != "" || payload.LastFrame != "" {
		payload.Mode = "keyframe"
		if payload.FirstFrame != "" {
			if err := validatePublicImageURL(payload.FirstFrame); err != nil {
				return payload, fmt.Errorf("Agnes AI first_frame_image must be a publicly reachable HTTP(S) URL: %w", err)
			}
		}
		if payload.LastFrame != "" {
			if err := validatePublicImageURL(payload.LastFrame); err != nil {
				return payload, fmt.Errorf("Agnes AI last_frame_image must be a publicly reachable HTTP(S) URL: %w", err)
			}
		}
	}
	return payload, nil
}

func (s agnesVideoState) failed() bool {
	switch strings.ToLower(s.Status) {
	case "failed", "error", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func pollAgnesVideo(ctx context.Context, apiBase, resultEndpoint, apiKey string, state agnesVideoState, pollIntervalSeconds, timeoutSeconds int) (agnesVideoState, error) {
	deadline := time.Now().Add(videoTimeout(timeoutSeconds))
	interval := videoPollInterval(pollIntervalSeconds)
	model := AgnesVideoModel(state.Model)

	for {
		if time.Now().After(deadline) {
			return state, fmt.Errorf("Agnes AI video generation timed out after %s", videoTimeout(timeoutSeconds))
		}
		if err := sleepWithContext(ctx, interval); err != nil {
			return state, err
		}

		statusURL := ""
		if state.VideoID != "" {
			statusURL = resultEndpoint + "?video_id=" + url.QueryEscape(state.VideoID) + "&model_name=" + url.QueryEscape(model)
		} else {
			statusURL = apiBase + "/videos/" + url.PathEscape(state.TaskID)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
		if err != nil {
			return state, fmt.Errorf("create Agnes AI video status request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := videoGenHTTPClient.Do(req)
		if err != nil {
			return state, fmt.Errorf("Agnes AI video status request failed: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		resp.Body.Close()
		if readErr != nil {
			return state, fmt.Errorf("read Agnes AI video status response: %w", readErr)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return state, fmt.Errorf("Agnes AI video status returned %d: %s", resp.StatusCode, truncateString(string(body), 500))
		}
		next, err := parseAgnesVideoState(body)
		if err != nil {
			return state, fmt.Errorf("parse Agnes AI video status response: %w", err)
		}
		next.TaskID = firstNonEmpty(next.TaskID, state.TaskID)
		next.VideoID = firstNonEmpty(next.VideoID, state.VideoID)
		next.Status = firstNonEmpty(next.Status, state.Status)
		next.URL = firstNonEmpty(next.URL, state.URL)
		next.Error = firstNonEmpty(next.Error, state.Error)
		next.Model = firstNonEmpty(next.Model, state.Model)
		next.Size = firstNonEmpty(next.Size, state.Size)
		if next.Seconds == 0 {
			next.Seconds = state.Seconds
		}
		state = next

		switch strings.ToLower(state.Status) {
		case "failed", "error", "cancelled", "canceled":
			message := state.Error
			if message == "" {
				message = "provider reported " + state.Status
			}
			return state, fmt.Errorf("Agnes AI video generation failed: %s", message)
		case "completed", "succeeded", "success":
			if state.URL == "" {
				return state, fmt.Errorf("Agnes AI completed without a video URL")
			}
			return state, nil
		default:
			if state.URL != "" {
				return state, nil
			}
		}
	}
}

func parseAgnesVideoState(body []byte) (agnesVideoState, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return agnesVideoState{}, err
	}
	objects := []map[string]interface{}{root}
	for _, key := range []string{"data", "result", "video"} {
		if nested, ok := root[key].(map[string]interface{}); ok {
			objects = append(objects, nested)
		}
	}

	var state agnesVideoState
	for _, object := range objects {
		state.TaskID = firstNonEmpty(state.TaskID, stringFromAny(object["task_id"]), stringFromAny(object["taskId"]), stringFromAny(object["id"]))
		state.VideoID = firstNonEmpty(state.VideoID, stringFromAny(object["video_id"]), stringFromAny(object["videoId"]))
		state.Status = firstNonEmpty(state.Status, stringFromAny(object["status"]), stringFromAny(object["state"]))
		state.URL = firstNonEmpty(state.URL, stringFromAny(object["url"]), stringFromAny(object["video_url"]), stringFromAny(object["download_url"]))
		state.Model = firstNonEmpty(state.Model, stringFromAny(object["model"]), stringFromAny(object["model_name"]))
		state.Size = firstNonEmpty(state.Size, stringFromAny(object["size"]))
		state.Error = firstNonEmpty(state.Error, stringFromAny(object["error"]), stringFromAny(object["error_message"]))
		if errorObject, ok := object["error"].(map[string]interface{}); ok {
			state.Error = firstNonEmpty(state.Error, stringFromAny(errorObject["message"]), stringFromAny(errorObject["detail"]), stringFromAny(errorObject["code"]))
		}
		if state.Seconds == 0 {
			if seconds, ok := object["seconds"].(float64); ok {
				state.Seconds = seconds
			} else if seconds, err := strconv.ParseFloat(stringFromAny(object["seconds"]), 64); err == nil {
				state.Seconds = seconds
			}
		}
		if metadata, ok := object["metadata"].(map[string]interface{}); ok {
			state.URL = firstNonEmpty(state.URL, stringFromAny(metadata["url"]), stringFromAny(metadata["video_url"]), stringFromAny(metadata["download_url"]))
			if sizeMapping, ok := metadata["size_mapping"].(map[string]interface{}); ok && state.Size == "" {
				width := positiveIntegerFromAny(sizeMapping["width"])
				height := positiveIntegerFromAny(sizeMapping["height"])
				if width > 0 && height > 0 {
					state.Size = fmt.Sprintf("%dx%d", width, height)
				}
			}
		}
	}
	return state, nil
}

func positiveIntegerFromAny(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case json.Number:
		parsed, _ := strconv.Atoi(typed.String())
		if parsed > 0 {
			return parsed
		}
	default:
		parsed, _ := strconv.Atoi(stringFromAny(value))
		if parsed > 0 {
			return parsed
		}
	}
	return 0
}
