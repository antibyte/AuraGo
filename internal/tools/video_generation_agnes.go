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
	"strconv"
	"strings"
	"time"

	"aurago/internal/uid"
)

type agnesVideoState struct {
	TaskID  string
	VideoID string
	Status  string
	URL     string
	Error   string
	Model   string
	Seconds float64
}

func generateVideoAgnes(ctx context.Context, baseURL, apiKey, model string, params VideoGenParams, pollIntervalSeconds, timeoutSeconds int, videoDir string) VideoGenResult {
	if strings.TrimSpace(model) == "" {
		model = defaultAgnesVideoModel
	}
	apiBase, resultEndpoint := agnesVideoEndpoints(baseURL)
	width, height := agnesVideoDimensions(params.Resolution, params.AspectRatio)

	payload := map[string]interface{}{
		"model":      model,
		"prompt":     params.Prompt,
		"height":     height,
		"width":      width,
		"num_frames": agnesVideoFrameCount(params.DurationSeconds),
		"frame_rate": 24,
	}
	if params.NegativePrompt != "" {
		payload["negative_prompt"] = params.NegativePrompt
	}
	images := make([]string, 0, 2+len(params.ReferenceImages))
	if params.FirstFrameImage != "" {
		images = append(images, params.FirstFrameImage)
		payload["image"] = params.FirstFrameImage
	}
	if params.LastFrameImage != "" {
		images = append(images, params.LastFrameImage)
	}
	images = append(images, params.ReferenceImages...)
	if len(images) > 1 {
		payload["extra_body"] = map[string]interface{}{
			"image": images,
			"mode":  "keyframes",
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to marshal Agnes AI video request: %v", err)}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/videos", bytes.NewReader(body))
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to create Agnes AI video request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := videoGenHTTPClient.Do(req)
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Agnes AI video request failed: %v", err)}
	}
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	resp.Body.Close()
	if readErr != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to read Agnes AI video response: %v", readErr)}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Agnes AI video API returned status %d: %s", resp.StatusCode, truncateString(string(respBody), 500))}
	}

	state, err := parseAgnesVideoState(respBody)
	if err != nil {
		return VideoGenResult{Status: "error", Error: fmt.Sprintf("Failed to parse Agnes AI video response: %v", err)}
	}
	if state.TaskID == "" && state.VideoID == "" {
		return VideoGenResult{Status: "error", Error: "Agnes AI did not return a task_id or video_id"}
	}
	if state.URL == "" {
		state, err = pollAgnesVideo(ctx, apiBase, resultEndpoint, apiKey, state, pollIntervalSeconds, timeoutSeconds)
		if err != nil {
			return VideoGenResult{Status: "error", TaskID: firstNonEmpty(state.TaskID, state.VideoID), Error: err.Error()}
		}
	}

	filename := fmt.Sprintf("video_%s.mp4", uid.New()[:8])
	filePath := filepath.Join(videoDir, filename)
	if err := downloadFileWithHeaders(ctx, state.URL, filePath, nil); err != nil {
		return VideoGenResult{Status: "error", TaskID: firstNonEmpty(state.TaskID, state.VideoID), Error: fmt.Sprintf("Failed to download Agnes AI video: %v", err)}
	}

	durationMs := int64(params.DurationSeconds) * 1000
	if state.Seconds > 0 {
		durationMs = int64(state.Seconds * 1000)
	}
	if state.Model != "" {
		model = state.Model
	}
	return VideoGenResult{
		Status:     "ok",
		Filename:   filename,
		FilePath:   filePath,
		WebPath:    "/files/generated_videos/" + filename,
		DurationMs: durationMs,
		Provider:   "agnes",
		Model:      model,
		Format:     "mp4",
		FileSize:   fileSizeOrZero(filePath),
		TaskID:     firstNonEmpty(state.TaskID, state.VideoID),
		Message:    fmt.Sprintf("Video generated successfully with Agnes AI (%ds).", params.DurationSeconds),
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

func agnesVideoDimensions(resolution, aspectRatio string) (width, height int) {
	shortSide := 768
	longSide := 1152
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "480p":
		shortSide, longSide = 480, 720
	case "720p":
		shortSide, longSide = 720, 1080
	case "1080p":
		shortSide, longSide = 1080, 1620
	}

	switch strings.TrimSpace(aspectRatio) {
	case "9:16", "3:4":
		return shortSide, longSide
	case "1:1":
		return shortSide, shortSide
	case "4:3":
		return longSide, shortSide
	default:
		return longSide, shortSide
	}
}

func agnesVideoFrameCount(seconds int) int {
	if seconds <= 0 {
		seconds = 6
	}
	frames := seconds*24 + 1
	if frames > 441 {
		return 441
	}
	if frames < 9 {
		return 9
	}
	return frames
}

func pollAgnesVideo(ctx context.Context, apiBase, resultEndpoint, apiKey string, state agnesVideoState, pollIntervalSeconds, timeoutSeconds int) (agnesVideoState, error) {
	deadline := time.Now().Add(videoTimeout(timeoutSeconds))
	interval := videoPollInterval(pollIntervalSeconds)

	for {
		if time.Now().After(deadline) {
			return state, fmt.Errorf("Agnes AI video generation timed out after %s", videoTimeout(timeoutSeconds))
		}
		if err := sleepWithContext(ctx, interval); err != nil {
			return state, err
		}

		statusURL := ""
		if state.VideoID != "" {
			statusURL = resultEndpoint + "?video_id=" + url.QueryEscape(state.VideoID)
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
		state.Error = firstNonEmpty(state.Error, stringFromAny(object["error"]), stringFromAny(object["error_message"]))
		if errorObject, ok := object["error"].(map[string]interface{}); ok {
			state.Error = firstNonEmpty(state.Error, stringFromAny(errorObject["message"]), stringFromAny(errorObject["detail"]))
		}
		if state.Seconds == 0 {
			if seconds, ok := object["seconds"].(float64); ok {
				state.Seconds = seconds
			} else if seconds, err := strconv.ParseFloat(stringFromAny(object["seconds"]), 64); err == nil {
				state.Seconds = seconds
			}
		}
	}
	return state, nil
}
