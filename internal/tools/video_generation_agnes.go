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
	width, height, err := agnesVideoDimensions(params.Resolution, params.AspectRatio)
	if err != nil {
		return VideoGenResult{Status: "error", Error: err.Error()}
	}
	frameCount, frameRate, err := agnesVideoFrameSettings(params.DurationSeconds)
	if err != nil {
		return VideoGenResult{Status: "error", Error: err.Error()}
	}
	firstImage, keyframes, err := agnesVideoImageInputs(params)
	if err != nil {
		return VideoGenResult{Status: "error", Error: err.Error()}
	}

	apiBase, resultEndpoint := agnesVideoEndpoints(baseURL)

	payload := map[string]interface{}{
		"model":      model,
		"prompt":     params.Prompt,
		"height":     height,
		"width":      width,
		"num_frames": frameCount,
		"frame_rate": frameRate,
	}
	if params.NegativePrompt != "" {
		payload["negative_prompt"] = params.NegativePrompt
	}
	if firstImage != "" {
		payload["image"] = firstImage
	}
	if len(keyframes) > 1 {
		payload["extra_body"] = map[string]interface{}{
			"image": keyframes,
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

	durationSeconds := float64(params.DurationSeconds)
	if state.Seconds > 0 {
		durationSeconds = state.Seconds
	}
	durationMs := int64(durationSeconds*1000 + 0.5)
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

func agnesVideoDimensions(resolution, aspectRatio string) (width, height int, err error) {
	type dimensions struct {
		width  int
		height int
	}
	table := map[string]map[string]dimensions{
		"480p": {
			"16:9": {854, 480}, "9:16": {480, 854}, "1:1": {480, 480}, "4:3": {640, 480}, "3:4": {480, 640},
		},
		"720p": {
			"16:9": {1280, 720}, "9:16": {720, 1280}, "1:1": {720, 720}, "4:3": {960, 720}, "3:4": {720, 960},
		},
		"768p": {
			"16:9": {1366, 768}, "9:16": {768, 1366}, "1:1": {768, 768}, "4:3": {1024, 768}, "3:4": {768, 1024},
		},
		"1080p": {
			"16:9": {1920, 1080}, "9:16": {1080, 1920}, "1:1": {1080, 1080}, "4:3": {1440, 1080}, "3:4": {1080, 1440},
		},
	}
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	aspectRatio = strings.TrimSpace(aspectRatio)
	byAspect, ok := table[resolution]
	if !ok {
		return 0, 0, fmt.Errorf("Agnes AI does not support video resolution %q; use 480p, 720p, 768p, or 1080p", resolution)
	}
	size, ok := byAspect[aspectRatio]
	if !ok {
		return 0, 0, fmt.Errorf("Agnes AI does not support video aspect ratio %q; use 16:9, 9:16, 1:1, 4:3, or 3:4", aspectRatio)
	}
	return size.width, size.height, nil
}

func agnesVideoFrameCount(seconds int) int {
	if seconds < 1 {
		return 0
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

func agnesVideoFrameSettings(seconds int) (frameCount int, frameRate float64, err error) {
	if seconds < 1 || seconds > 30 {
		return 0, 0, fmt.Errorf("Agnes AI video duration must be between 1 and 30 seconds")
	}
	frameCount = agnesVideoFrameCount(seconds)
	frameRate = 24
	if seconds > 18 {
		frameRate = float64(frameCount) / float64(seconds)
	}
	return frameCount, frameRate, nil
}

func agnesVideoImageInputs(params VideoGenParams) (firstImage string, keyframes []string, err error) {
	firstImage = strings.TrimSpace(params.FirstFrameImage)
	lastImage := strings.TrimSpace(params.LastFrameImage)
	references := make([]string, 0, len(params.ReferenceImages))
	for i, imageURL := range params.ReferenceImages {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			return "", nil, fmt.Errorf("Agnes AI reference_images[%d] must not be empty", i)
		}
		if err := validatePublicImageURL(imageURL); err != nil {
			return "", nil, fmt.Errorf("Agnes AI reference_images[%d] must be a publicly reachable HTTP(S) URL: %w", i, err)
		}
		references = append(references, imageURL)
	}
	if firstImage != "" {
		if err := validatePublicImageURL(firstImage); err != nil {
			return "", nil, fmt.Errorf("Agnes AI first_frame_image must be a publicly reachable HTTP(S) URL: %w", err)
		}
	}
	if lastImage != "" {
		if firstImage == "" {
			return "", nil, fmt.Errorf("Agnes AI last_frame_image requires first_frame_image")
		}
		if err := validatePublicImageURL(lastImage); err != nil {
			return "", nil, fmt.Errorf("Agnes AI last_frame_image must be a publicly reachable HTTP(S) URL: %w", err)
		}
	}

	if firstImage == "" {
		if len(references) == 1 {
			return "", nil, fmt.Errorf("Agnes AI requires at least two reference_images for keyframe mode; use first_frame_image for a single image")
		}
		if len(references) >= 2 {
			return "", references, nil
		}
		return "", nil, nil
	}
	if len(references) == 0 && lastImage == "" {
		return firstImage, nil, nil
	}
	keyframes = append(keyframes, firstImage)
	keyframes = append(keyframes, references...)
	if lastImage != "" {
		keyframes = append(keyframes, lastImage)
	}
	return firstImage, keyframes, nil
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
