package tools

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
)

// FrigateConfig holds the Frigate NVR connection parameters.
type FrigateConfig struct {
	URL           string
	APIToken      string
	InternalPort  bool
	Insecure      bool
	DefaultCamera string
	ReadOnly      bool
}

// FrigateEventParams contains filters for event queries.
type FrigateEventParams struct {
	Camera      string
	EventID     string
	Label       string
	Zone        string
	After       int64
	Before      int64
	MinScore    float64
	HasClip     *bool
	HasSnapshot *bool
	Limit       int
	Offset      int
}

// FrigateReviewParams contains filters for review queries.
type FrigateReviewParams struct {
	Camera     string
	After      int64
	Before     int64
	Limit      int
	Offset     int
	InProgress *bool
	Cameras    string
	Labels     string
	Zones      string
}

// FrigateMediaParams contains parameters for image/video operations.
type FrigateMediaParams struct {
	Camera    string
	EventID   string
	StartTime string
	EndTime   string
	Playback  string
}

// FrigateMediaStoreResult describes a stored Frigate media asset.
type FrigateMediaStoreResult struct {
	Status      string `json:"status"`
	ContentType string `json:"content_type"`
	Bytes       int    `json:"bytes"`
	Stored      bool   `json:"stored"`
	LocalPath   string `json:"local_path,omitempty"`
	WebPath     string `json:"web_path,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	MediaID     int64  `json:"media_id,omitempty"`
	MediaType   string `json:"media_type,omitempty"`
	Message     string `json:"message"`
}

var frigateClientCache sync.Map

func getFrigateClient(cfg FrigateConfig) *http.Client {
	baseURL, err := frigateBaseURL(cfg)
	if err != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	}
	cacheKey := baseURL + "|" + strconv.FormatBool(cfg.Insecure)
	if cached, ok := frigateClientCache.Load(cacheKey); ok {
		return cached.(*http.Client)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.Insecure}
	client := &http.Client{Timeout: 30 * time.Second, Transport: transport}
	actual, _ := frigateClientCache.LoadOrStore(cacheKey, client)
	return actual.(*http.Client)
}

func frigateJSONError(format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	b, _ := json.Marshal(map[string]interface{}{"status": "error", "message": msg})
	return string(b)
}

func frigateJSONResult(raw []byte) string {
	if json.Valid(raw) {
		return string(raw)
	}
	b, _ := json.Marshal(map[string]interface{}{"status": "ok", "data": string(raw)})
	return string(b)
}

func frigateBaseURL(cfg FrigateConfig) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("frigate url is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid frigate url %q: %w", cfg.URL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid frigate url scheme %q: use http or https", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid frigate url %q: host is required", cfg.URL)
	}
	return baseURL, nil
}

// frigateRequest performs a generic HTTP request against the Frigate API.
func frigateRequest(cfg FrigateConfig, method, path string) ([]byte, int, error) {
	if cfg.APIToken != "" {
		security.RegisterSensitive(cfg.APIToken)
	}
	baseURL, err := frigateBaseURL(cfg)
	if err != nil {
		return nil, 0, err
	}
	var lastData []byte
	var lastCode int
	var lastErr error
	attempts := 1
	if strings.EqualFold(method, http.MethodGet) {
		attempts = 3
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequest(method, baseURL+path, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		if cfg.APIToken != "" && !cfg.InternalPort {
			req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		resp, err := getFrigateClient(cfg).Do(req.WithContext(ctx))
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if attempt < attempts {
				time.Sleep(time.Duration(attempt) * 50 * time.Millisecond)
				continue
			}
			return nil, 0, lastErr
		}
		data, readErr := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		_ = resp.Body.Close()
		lastData = data
		lastCode = resp.StatusCode
		if readErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", readErr)
		}
		if attempt < attempts && frigateRetryableStatus(resp.StatusCode) {
			time.Sleep(time.Duration(attempt) * 50 * time.Millisecond)
			continue
		}
		return data, resp.StatusCode, nil
	}
	return lastData, lastCode, lastErr
}

func frigateRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func frigateGetJSON(cfg FrigateConfig, path string) string {
	data, code, err := frigateRequest(cfg, http.MethodGet, path)
	if err != nil {
		return frigateJSONError("Frigate request failed: %v", err)
	}
	if code < 200 || code >= 300 {
		return frigateJSONError("Frigate returned HTTP %d: %s", code, string(data))
	}
	return frigateJSONResult(data)
}

func addFrigateQuery(q url.Values, key, value string) {
	if strings.TrimSpace(value) != "" {
		q.Set(key, strings.TrimSpace(value))
	}
}

func addFrigateIntQuery(q url.Values, key string, value int64) {
	if value > 0 {
		q.Set(key, strconv.FormatInt(value, 10))
	}
}

func addFrigateBoolQuery(q url.Values, key string, value *bool) {
	if value != nil {
		q.Set(key, strconv.FormatBool(*value))
	}
}

func frigatePath(path string, q url.Values) string {
	if encoded := q.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

// FrigateStatus returns cameras, stats, and health data.
func FrigateStatus(cfg FrigateConfig) string {
	return frigateGetJSON(cfg, "/api/stats")
}

// FrigateCameras lists configured cameras and capabilities.
func FrigateCameras(cfg FrigateConfig) string {
	return frigateGetJSON(cfg, "/api/config")
}

// FrigateEvents searches events with filters.
func FrigateEvents(cfg FrigateConfig, params FrigateEventParams) string {
	q := url.Values{}
	addFrigateQuery(q, "camera", firstNonEmptyString(params.Camera, cfg.DefaultCamera))
	addFrigateQuery(q, "label", params.Label)
	addFrigateQuery(q, "zone", params.Zone)
	addFrigateIntQuery(q, "after", params.After)
	addFrigateIntQuery(q, "before", params.Before)
	if params.MinScore > 0 {
		q.Set("min_score", strconv.FormatFloat(params.MinScore, 'f', -1, 64))
	}
	addFrigateBoolQuery(q, "has_clip", params.HasClip)
	addFrigateBoolQuery(q, "has_snapshot", params.HasSnapshot)
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		q.Set("offset", strconv.Itoa(params.Offset))
	}
	return frigateGetJSON(cfg, frigatePath("/api/events", q))
}

// FrigateEvent returns a single event by ID.
func FrigateEvent(cfg FrigateConfig, eventID string) string {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return frigateJSONError("event_id is required")
	}
	return frigateGetJSON(cfg, "/api/events/"+url.PathEscape(eventID))
}

// FrigateReviews queries review items.
func FrigateReviews(cfg FrigateConfig, params FrigateReviewParams) string {
	q := url.Values{}
	addFrigateQuery(q, "camera", firstNonEmptyString(params.Camera, cfg.DefaultCamera))
	addFrigateIntQuery(q, "after", params.After)
	addFrigateIntQuery(q, "before", params.Before)
	addFrigateBoolQuery(q, "in_progress", params.InProgress)
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		q.Set("offset", strconv.Itoa(params.Offset))
	}
	return frigateGetJSON(cfg, frigatePath("/api/review", q))
}

// FrigateReviewSummary returns review summary data.
func FrigateReviewSummary(cfg FrigateConfig, params FrigateReviewParams) string {
	q := url.Values{}
	addFrigateIntQuery(q, "after", params.After)
	addFrigateIntQuery(q, "before", params.Before)
	addFrigateQuery(q, "cameras", params.Cameras)
	addFrigateQuery(q, "labels", params.Labels)
	addFrigateQuery(q, "zones", params.Zones)
	return frigateGetJSON(cfg, frigatePath("/api/review/summary", q))
}

// FrigateReviewActivity returns review activity over time.
func FrigateReviewActivity(cfg FrigateConfig, params FrigateReviewParams) string {
	q := url.Values{}
	addFrigateIntQuery(q, "after", params.After)
	addFrigateIntQuery(q, "before", params.Before)
	addFrigateQuery(q, "cameras", params.Cameras)
	addFrigateBoolQuery(q, "in_progress", params.InProgress)
	return frigateGetJSON(cfg, frigatePath("/api/review/activity", q))
}

// FrigateMedia fetches snapshots, clips, latest frames, or exported recording data.
func FrigateMedia(cfg FrigateConfig, operation string, params FrigateMediaParams) ([]byte, string, error) {
	var path string
	switch operation {
	case "event_snapshot":
		if strings.TrimSpace(params.EventID) == "" {
			return nil, "", fmt.Errorf("event_id is required")
		}
		path = "/api/events/" + url.PathEscape(params.EventID) + "/snapshot.jpg"
	case "event_clip":
		if strings.TrimSpace(params.EventID) == "" {
			return nil, "", fmt.Errorf("event_id is required")
		}
		path = "/api/events/" + url.PathEscape(params.EventID) + "/clip.mp4"
	case "latest_frame":
		camera := firstNonEmptyString(params.Camera, cfg.DefaultCamera)
		if strings.TrimSpace(camera) == "" {
			return nil, "", fmt.Errorf("camera is required")
		}
		path = "/api/" + url.PathEscape(camera) + "/latest.jpg"
	case "export_recording":
		camera := firstNonEmptyString(params.Camera, cfg.DefaultCamera)
		if strings.TrimSpace(camera) == "" || strings.TrimSpace(params.StartTime) == "" || strings.TrimSpace(params.EndTime) == "" {
			return nil, "", fmt.Errorf("camera, start_time, and end_time are required")
		}
		q := url.Values{}
		q.Set("start", params.StartTime)
		q.Set("end", params.EndTime)
		addFrigateQuery(q, "playback", params.Playback)
		path = frigatePath("/api/"+url.PathEscape(camera)+"/recordings/export", q)
	default:
		return nil, "", fmt.Errorf("unsupported media operation %q", operation)
	}
	data, code, err := frigateRequest(cfg, http.MethodGet, path)
	if err != nil {
		return nil, "", err
	}
	if code < 200 || code >= 300 {
		return nil, "", fmt.Errorf("Frigate returned HTTP %d: %s", code, string(data))
	}
	return data, http.DetectContentType(data), nil
}

// StoreFrigateMedia persists a Frigate media response in the AuraGo data directory.
func StoreFrigateMedia(dataDir string, mediaDB *sql.DB, operation string, params FrigateMediaParams, data []byte, contentType string) (FrigateMediaStoreResult, error) {
	result := FrigateMediaStoreResult{
		Status:      "ok",
		ContentType: contentType,
		Bytes:       len(data),
		Stored:      false,
		Message:     "Media fetched successfully but was not stored.",
	}
	if len(data) == 0 {
		return result, fmt.Errorf("media data is empty")
	}
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return result, fmt.Errorf("data directory is required")
	}
	now := time.Now().UTC()
	relDir := filepath.Join("frigate_media", now.Format("2006"), now.Format("01"), now.Format("02"))
	destDir := filepath.Join(dataDir, relDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return result, fmt.Errorf("create frigate media directory: %w", err)
	}
	hashBytes := sha256.Sum256(data)
	hash := hex.EncodeToString(hashBytes[:])
	ext := frigateMediaExtension(contentType, operation)
	identity := firstNonEmptyString(params.EventID, params.Camera, "media")
	filename := fmt.Sprintf("%s_%s_%s%s", sanitizeFrigateFileToken(operation), sanitizeFrigateFileToken(identity), now.Format("150405.000000000"), ext)
	localPath := filepath.Join(destDir, filename)
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return result, fmt.Errorf("write frigate media: %w", err)
	}
	webPath := "/files/frigate_media/" + strings.ReplaceAll(filepath.ToSlash(filepath.Join(now.Format("2006"), now.Format("01"), now.Format("02"), filename)), " ", "%20")
	mediaType := frigateMediaType(ext, contentType)
	result.Stored = true
	result.LocalPath = localPath
	result.WebPath = webPath
	result.SHA256 = hash
	result.MediaType = mediaType
	result.Message = "Media stored successfully."
	if mediaDB != nil {
		mediaID, _, err := RegisterMedia(mediaDB, MediaItem{
			MediaType:   mediaType,
			SourceTool:  "frigate",
			Filename:    filename,
			FilePath:    localPath,
			WebPath:     webPath,
			FileSize:    int64(len(data)),
			Format:      strings.TrimPrefix(ext, "."),
			Provider:    "frigate",
			Description: fmt.Sprintf("Frigate %s media", operation),
			Tags:        []string{"frigate", operation},
			Hash:        hash,
		})
		if err != nil {
			return result, fmt.Errorf("register frigate media: %w", err)
		}
		result.MediaID = mediaID
	}
	return result, nil
}

func frigateMediaExtension(contentType, operation string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	}
	switch operation {
	case "event_clip", "export_recording":
		return ".mp4"
	default:
		return ".jpg"
	}
}

func frigateMediaType(ext, contentType string) string {
	if strings.HasPrefix(strings.ToLower(contentType), "video/") || ext == ".mp4" {
		return "video"
	}
	return "image"
}

func sanitizeFrigateFileToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "media"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		allowed := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if allowed {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_-.")
	if out == "" {
		return "media"
	}
	if len(out) > 80 {
		out = out[:80]
		out = strings.Trim(out, "_-.")
	}
	if out == "" {
		return "media"
	}
	return out
}

// FrigateRecordingsSummary returns hourly recording availability.
func FrigateRecordingsSummary(cfg FrigateConfig, params FrigateMediaParams) string {
	camera := firstNonEmptyString(params.Camera, cfg.DefaultCamera)
	if strings.TrimSpace(camera) == "" {
		return frigateJSONError("camera is required")
	}
	q := url.Values{}
	addFrigateQuery(q, "start", params.StartTime)
	addFrigateQuery(q, "end", params.EndTime)
	return frigateGetJSON(cfg, frigatePath("/api/"+url.PathEscape(camera)+"/recordings/summary", q))
}

// FrigateConfigRead reads Frigate config.
func FrigateConfigRead(cfg FrigateConfig, raw bool) string {
	if raw {
		return frigateGetJSON(cfg, "/api/config/raw")
	}
	return frigateGetJSON(cfg, "/api/config")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
