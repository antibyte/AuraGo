package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	Reviewed   *bool
	Cameras    string
	Labels     string
	Zones      string
	Severity   string
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
	client := &http.Client{Timeout: 30 * time.Second, Transport: transport, CheckRedirect: security.SameOriginRedirect}
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
	if err := security.ValidateHTTPBaseURL(baseURL); err != nil {
		return "", err
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
// Each attempt is capped at 30 seconds on the caller context. A cancelled
// caller returns before another attempt starts.
func frigateRequest(ctx context.Context, cfg FrigateConfig, method, path string) ([]byte, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(attemptCtx, method, baseURL+path, nil)
		if err != nil {
			cancel()
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		if cfg.APIToken != "" && !cfg.InternalPort {
			req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
		}
		resp, err := getFrigateClient(cfg).Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("request failed: %w", err)
			if attempt < attempts && frigateRetryWait(ctx, attempt) {
				continue
			}
			if ctx.Err() != nil {
				return nil, 0, ctx.Err()
			}
			return nil, 0, lastErr
		}
		data, readErr := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		_ = resp.Body.Close()
		cancel()
		lastData = data
		lastCode = resp.StatusCode
		if readErr != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", readErr)
		}
		if attempt < attempts && frigateRetryableStatus(resp.StatusCode) {
			if frigateRetryWait(ctx, attempt) {
				continue
			}
			if ctx.Err() != nil {
				return nil, 0, ctx.Err()
			}
		}
		return data, resp.StatusCode, nil
	}
	return lastData, lastCode, lastErr
}

func frigateRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func frigateRetryWait(ctx context.Context, attempt int) bool {
	timer := time.NewTimer(time.Duration(attempt) * 50 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func frigateGetJSON(ctx context.Context, cfg FrigateConfig, path string) string {
	data, code, err := frigateRequest(ctx, cfg, http.MethodGet, path)
	if err != nil {
		return frigateJSONError("Frigate request failed: %s", security.Scrub(err.Error()))
	}
	if code < 200 || code >= 300 {
		return frigateJSONError("Frigate returned HTTP %d", code)
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

func addFrigateBoolIntQuery(q url.Values, key string, value *bool) {
	if value == nil {
		return
	}
	if *value {
		q.Set(key, "1")
		return
	}
	q.Set(key, "0")
}

func frigatePath(path string, q url.Values) string {
	if encoded := q.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

// FrigateStatus returns cameras, stats, and health data.
func FrigateStatus(ctx context.Context, cfg FrigateConfig) string {
	return frigateGetJSON(ctx, cfg, "/api/stats")
}

// FrigateCameras returns only the redacted cameras object from /api/config.
func FrigateCameras(ctx context.Context, cfg FrigateConfig) string {
	data, code, err := frigateRequest(ctx, cfg, http.MethodGet, "/api/config")
	if err != nil {
		return frigateJSONError("Frigate configuration request failed: %s", security.Scrub(err.Error()))
	}
	if code < 200 || code >= 300 {
		return frigateJSONError("Frigate configuration returned HTTP %d", code)
	}
	redacted, err := redactFrigateConfig(data, false)
	if err != nil {
		return frigateJSONError("Frigate configuration could not be safely redacted")
	}
	cameras, err := frigateCamerasObject(redacted)
	if err != nil {
		return frigateJSONError("Frigate configuration could not be safely redacted")
	}
	out, err := json.Marshal(struct {
		Status  string          `json:"status"`
		Cameras json.RawMessage `json:"cameras"`
	}{Status: "ok", Cameras: cameras})
	if err != nil {
		return frigateJSONError("Frigate configuration could not be safely redacted")
	}
	return string(out)
}

func frigateCamerasObject(redacted []byte) (json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(redacted, &doc); err != nil {
		return nil, err
	}
	raw, ok := doc["cameras"]
	if !ok || len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return json.RawMessage(`{}`), nil
	}
	trimmed := bytes.TrimSpace(raw)
	if trimmed[0] != '{' {
		return nil, fmt.Errorf("cameras is not an object")
	}
	return json.RawMessage(trimmed), nil
}

// FrigateEvents searches events with filters.
func FrigateEvents(ctx context.Context, cfg FrigateConfig, params FrigateEventParams) string {
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
	return frigateGetJSON(ctx, cfg, frigatePath("/api/events", q))
}

// FrigateEvent returns a single event by ID.
func FrigateEvent(ctx context.Context, cfg FrigateConfig, eventID string) string {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return frigateJSONError("event_id is required")
	}
	return frigateGetJSON(ctx, cfg, "/api/events/"+url.PathEscape(eventID))
}

// FrigateReviews queries review items.
func FrigateReviews(ctx context.Context, cfg FrigateConfig, params FrigateReviewParams) string {
	q := url.Values{}
	addFrigateQuery(q, "cameras", firstNonEmptyString(params.Cameras, params.Camera, cfg.DefaultCamera))
	addFrigateQuery(q, "labels", params.Labels)
	addFrigateQuery(q, "zones", params.Zones)
	addFrigateIntQuery(q, "after", params.After)
	addFrigateIntQuery(q, "before", params.Before)
	addFrigateBoolIntQuery(q, "reviewed", params.Reviewed)
	addFrigateQuery(q, "severity", params.Severity)
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		q.Set("offset", strconv.Itoa(params.Offset))
	}
	return frigateGetJSON(ctx, cfg, frigatePath("/api/review", q))
}

// FrigateReviewSummary returns review summary data.
func FrigateReviewSummary(ctx context.Context, cfg FrigateConfig, params FrigateReviewParams) string {
	q := url.Values{}
	addFrigateIntQuery(q, "after", params.After)
	addFrigateIntQuery(q, "before", params.Before)
	addFrigateQuery(q, "cameras", params.Cameras)
	addFrigateQuery(q, "labels", params.Labels)
	addFrigateQuery(q, "zones", params.Zones)
	return frigateGetJSON(ctx, cfg, frigatePath("/api/review/summary", q))
}

// FrigateReviewActivity returns review activity over time.
func FrigateReviewActivity(ctx context.Context, cfg FrigateConfig, params FrigateReviewParams) string {
	q := url.Values{}
	addFrigateIntQuery(q, "after", params.After)
	addFrigateIntQuery(q, "before", params.Before)
	addFrigateQuery(q, "cameras", params.Cameras)
	return frigateGetJSON(ctx, cfg, frigatePath("/api/review/activity/motion", q))
}

const maxFrigateMediaBytes int64 = 256 * 1024 * 1024

// FrigateMedia streams a snapshot, clip, latest frame, or exported recording
// into the dated frigate_media directory. store false keeps the byte count and
// deletes the temporary file. The body is never returned in memory.
func FrigateMedia(ctx context.Context, cfg FrigateConfig, operation string, params FrigateMediaParams, dataDir string, mediaDB *sql.DB, store bool) (FrigateMediaStoreResult, error) {
	result := FrigateMediaStoreResult{Status: "ok", Stored: false}
	path, err := frigateMediaPath(cfg, operation, params)
	if err != nil {
		return result, err
	}
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return result, fmt.Errorf("data directory is required")
	}
	now := time.Now().UTC()
	destDir := filepath.Join(dataDir, "frigate_media", now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return result, fmt.Errorf("create frigate media directory: %w", err)
	}
	tmpPath := filepath.Join(destDir, fmt.Sprintf(".frigate_%d.tmp", now.UnixNano()))
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return result, fmt.Errorf("create frigate media file: %w", err)
	}
	remove := true
	defer func() {
		if remove {
			_ = os.Remove(tmpPath)
		}
	}()

	resp, cancel, err := frigateOpen(ctx, cfg, path)
	if err != nil {
		_ = tmp.Close()
		return result, err
	}
	defer cancel()
	defer resp.Body.Close()

	n, head, sum, err := writeFrigateMediaLimited(tmp, resp.Body, maxFrigateMediaBytes)
	closeErr := tmp.Close()
	if err != nil {
		return result, err
	}
	if closeErr != nil {
		return result, fmt.Errorf("write frigate media: %w", closeErr)
	}
	contentType, ok := frigateAllowedMediaType(head, operation)
	if !ok {
		return result, fmt.Errorf("unsupported Frigate media type %q", contentType)
	}
	result.ContentType = contentType
	result.Bytes = int(n)
	if !store {
		result.Message = "Media fetched successfully but frigate.store_media is disabled."
		return result, nil
	}

	ext := frigateMediaExtension(contentType, operation)
	identity := firstNonEmptyString(params.EventID, params.Camera, "media")
	filename := fmt.Sprintf("%s_%s_%s%s", sanitizeMediaFileToken(operation), sanitizeMediaFileToken(identity), now.Format("150405.000000000"), ext)
	localPath := filepath.Join(destDir, filename)
	if err := os.Rename(tmpPath, localPath); err != nil {
		return result, fmt.Errorf("write frigate media: %w", err)
	}
	remove = false
	webPath := "/files/frigate_media/" + strings.ReplaceAll(filepath.ToSlash(filepath.Join(now.Format("2006"), now.Format("01"), now.Format("02"), filename)), " ", "%20")
	mediaType := frigateMediaType(ext, contentType)
	result.Stored = true
	result.LocalPath = localPath
	result.WebPath = webPath
	result.SHA256 = sum
	result.MediaType = mediaType
	result.Message = "Media stored successfully."
	if mediaDB != nil {
		mediaID, _, err := RegisterMedia(mediaDB, MediaItem{
			MediaType:   mediaType,
			SourceTool:  "frigate",
			Filename:    filename,
			FilePath:    localPath,
			WebPath:     webPath,
			FileSize:    n,
			Format:      strings.TrimPrefix(ext, "."),
			Provider:    "frigate",
			Description: fmt.Sprintf("Frigate %s media", operation),
			Tags:        []string{"frigate", operation},
			Hash:        sum,
		})
		if err != nil {
			return result, fmt.Errorf("register frigate media: %w", err)
		}
		result.MediaID = mediaID
	}
	return result, nil
}

func frigateMediaPath(cfg FrigateConfig, operation string, params FrigateMediaParams) (string, error) {
	switch operation {
	case "event_snapshot":
		if strings.TrimSpace(params.EventID) == "" {
			return "", fmt.Errorf("event_id is required")
		}
		return "/api/events/" + url.PathEscape(params.EventID) + "/snapshot.jpg", nil
	case "event_clip":
		if strings.TrimSpace(params.EventID) == "" {
			return "", fmt.Errorf("event_id is required")
		}
		return "/api/events/" + url.PathEscape(params.EventID) + "/clip.mp4", nil
	case "latest_frame":
		camera := firstNonEmptyString(params.Camera, cfg.DefaultCamera)
		if strings.TrimSpace(camera) == "" {
			return "", fmt.Errorf("camera is required")
		}
		return "/api/" + url.PathEscape(camera) + "/latest.jpg", nil
	case "export_recording":
		camera := firstNonEmptyString(params.Camera, cfg.DefaultCamera)
		if strings.TrimSpace(camera) == "" || strings.TrimSpace(params.StartTime) == "" || strings.TrimSpace(params.EndTime) == "" {
			return "", fmt.Errorf("camera, start_time, and end_time are required")
		}
		return "/api/" + url.PathEscape(camera) + "/start/" + url.PathEscape(strings.TrimSpace(params.StartTime)) + "/end/" + url.PathEscape(strings.TrimSpace(params.EndTime)) + "/clip.mp4", nil
	default:
		return "", fmt.Errorf("unsupported media operation %q", operation)
	}
}

// frigateOpen returns a successful GET response. The caller closes the body and
// calls cancel after the body has been consumed.
func frigateOpen(ctx context.Context, cfg FrigateConfig, path string) (*http.Response, context.CancelFunc, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.APIToken != "" {
		security.RegisterSensitive(cfg.APIToken)
	}
	baseURL, err := frigateBaseURL(cfg)
	if err != nil {
		return nil, nil, err
	}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, baseURL+path, nil)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("failed to create request: %w", err)
		}
		if cfg.APIToken != "" && !cfg.InternalPort {
			req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
		}
		resp, err := getFrigateClient(cfg).Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("request failed: %w", err)
			if attempt < 3 && frigateRetryWait(ctx, attempt) {
				continue
			}
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			return nil, nil, lastErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			code := resp.StatusCode
			_ = resp.Body.Close()
			cancel()
			if attempt < 3 && frigateRetryableStatus(code) && frigateRetryWait(ctx, attempt) {
				continue
			}
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			return nil, nil, fmt.Errorf("Frigate returned HTTP %d", code)
		}
		return resp, cancel, nil
	}
	return nil, nil, lastErr
}

func writeFrigateMediaLimited(dst io.Writer, src io.Reader, limit int64) (int64, []byte, string, error) {
	hash := sha256.New()
	head := &frigateHeadWriter{limit: 512}
	n, err := io.Copy(io.MultiWriter(dst, hash, head), io.LimitReader(src, limit+1))
	if err != nil {
		return n, head.buf, "", err
	}
	if n > limit {
		return n, head.buf, "", fmt.Errorf("Frigate media exceeds %d bytes", limit)
	}
	if n == 0 {
		return 0, head.buf, "", fmt.Errorf("media data is empty")
	}
	return n, head.buf, hex.EncodeToString(hash.Sum(nil)), nil
}

type frigateHeadWriter struct {
	buf   []byte
	limit int
}

func (w *frigateHeadWriter) Write(p []byte) (int, error) {
	if len(w.buf) < w.limit {
		take := w.limit - len(w.buf)
		if take > len(p) {
			take = len(p)
		}
		w.buf = append(w.buf, p[:take]...)
	}
	return len(p), nil
}

func frigateAllowedMediaType(head []byte, operation string) (string, bool) {
	base := canonicalMediaType(http.DetectContentType(head))
	switch base {
	case "image/jpeg", "image/png", "image/webp", "video/mp4":
		return base, true
	}
	if base == "application/octet-stream" && (operation == "event_clip" || operation == "export_recording") && frigateFtypPrefix(head) {
		return "video/mp4", true
	}
	if base == "" {
		base = "application/octet-stream"
	}
	return base, false
}

func canonicalMediaType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

func frigateFtypPrefix(head []byte) bool {
	if len(head) < 8 {
		return false
	}
	if string(head[4:8]) != "ftyp" {
		return false
	}
	size := binary.BigEndian.Uint32(head[:4])
	return size >= 8 && size%4 == 0
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
	filename := fmt.Sprintf("%s_%s_%s%s", sanitizeMediaFileToken(operation), sanitizeMediaFileToken(identity), now.Format("150405.000000000"), ext)
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

func sanitizeMediaFileToken(value string) string {
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
func FrigateRecordingsSummary(ctx context.Context, cfg FrigateConfig, params FrigateMediaParams) string {
	camera := firstNonEmptyString(params.Camera, cfg.DefaultCamera)
	if strings.TrimSpace(camera) == "" {
		return frigateJSONError("camera is required")
	}
	q := url.Values{}
	addFrigateQuery(q, "start", params.StartTime)
	addFrigateQuery(q, "end", params.EndTime)
	return frigateGetJSON(ctx, cfg, frigatePath("/api/"+url.PathEscape(camera)+"/recordings/summary", q))
}

// FrigateConfigRead reads Frigate config.
func FrigateConfigRead(ctx context.Context, cfg FrigateConfig, raw bool) string {
	path := "/api/config"
	if raw {
		path += "/raw"
	}
	data, code, err := frigateRequest(ctx, cfg, http.MethodGet, path)
	if err != nil {
		return frigateJSONError("Frigate configuration request failed: %s", security.Scrub(err.Error()))
	}
	if code < 200 || code >= 300 {
		return frigateJSONError("Frigate configuration returned HTTP %d", code)
	}
	redacted, err := redactFrigateConfig(data, raw)
	if err != nil {
		return frigateJSONError("Frigate configuration could not be safely redacted")
	}
	return string(redacted)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
