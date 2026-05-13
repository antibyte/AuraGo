package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type ElegooCentauriCarbonPrinter struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	MainboardID    string `json:"mainboard_id,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type ElegooCentauriCarbonConfig struct {
	Enabled  bool                          `json:"enabled"`
	Printers []ElegooCentauriCarbonPrinter `json:"printers"`
}

type ThreeDPrinterConfig struct {
	Enabled              bool                       `json:"enabled"`
	ReadOnly             bool                       `json:"readonly"`
	DefaultPrinter       string                     `json:"default_printer"`
	DataDir              string                     `json:"-"`
	ElegooCentauriCarbon ElegooCentauriCarbonConfig `json:"elegoo_centauri_carbon"`
}

type ThreeDPrinterRequest struct {
	Operation         string `json:"operation"`
	PrinterID         string `json:"printer_id"`
	URL               string `json:"url,omitempty"`
	MainboardID       string `json:"mainboard_id,omitempty"`
	TimeoutSeconds    int    `json:"timeout_seconds,omitempty"`
	Filename          string `json:"filename"`
	Directory         string `json:"directory"`
	Prompt            string `json:"prompt"`
	LightOn           *bool  `json:"light_on,omitempty"`
	StartLayer        int    `json:"start_layer"`
	Calibration       bool   `json:"calibration"`
	TimeLapse         bool   `json:"time_lapse"`
	ShowInChat        bool   `json:"show_in_chat"`
	InternalStreamURL string `json:"-"`
}

type ThreeDPrinterMediaResult struct {
	Status      string `json:"status"`
	ContentType string `json:"content_type"`
	Bytes       int    `json:"bytes"`
	Stored      bool   `json:"stored"`
	LocalPath   string `json:"local_path,omitempty"`
	WebPath     string `json:"web_path,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Message     string `json:"message"`
}

func ExecuteThreeDPrinter(ctx context.Context, cfg ThreeDPrinterConfig, req ThreeDPrinterRequest) string {
	if !cfg.Enabled {
		return threeDPrinterJSONError("3D printer integration is not enabled")
	}
	operation := strings.ToLower(strings.TrimSpace(req.Operation))
	if operation == "" {
		operation = "status"
	}
	if operation == "list_printers" {
		return threeDPrinterJSON(map[string]interface{}{"status": "ok", "printers": cfg.ElegooCentauriCarbon.Printers, "default_printer": cfg.DefaultPrinter})
	}
	if cfg.ReadOnly && threeDPrinterMutates(operation) {
		return threeDPrinterJSONError("3D printer integration is in read-only mode")
	}
	printer, err := ResolveThreeDPrinter(cfg, req.PrinterID)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}

	switch operation {
	case "test_connection", "status":
		return ElegooCentauriCarbonStatus(ctx, printer)
	case "attributes":
		return elegooCentauriCarbonCommandJSON(ctx, printer, 1, map[string]interface{}{})
	case "files":
		dir := strings.TrimSpace(req.Directory)
		if dir == "" {
			dir = "/local"
		}
		return elegooCentauriCarbonCommandJSON(ctx, printer, 258, map[string]interface{}{"Url": dir})
	case "history":
		return elegooCentauriCarbonCommandJSON(ctx, printer, 320, map[string]interface{}{})
	case "camera_url":
		streamURL, err := ElegooCentauriCarbonCameraURL(ctx, printer)
		if err != nil {
			return threeDPrinterJSONError(err.Error())
		}
		return threeDPrinterJSON(map[string]interface{}{"status": "ok", "printer_id": printer.ID, "url": streamURL})
	case "camera_snapshot":
		return executeThreeDPrinterSnapshot(ctx, cfg, printer)
	case "show_live_stream":
		streamURL, err := ElegooCentauriCarbonCameraURL(ctx, printer)
		if err != nil {
			return threeDPrinterJSONError(err.Error())
		}
		proxyURL := "/api/3d-printers/" + url.PathEscape(printer.ID) + "/camera/stream"
		if err := ValidateThreeDPrinterStreamURL(printer.URL, streamURL); err != nil {
			return threeDPrinterJSON(map[string]interface{}{"status": "fallback", "message": err.Error(), "stream_url": streamURL})
		}
		return threeDPrinterJSON(map[string]interface{}{"status": "ok", "printer_id": printer.ID, "stream_url": streamURL, "proxy_url": proxyURL, "mime_type": "multipart/x-mixed-replace"})
	case "start_print":
		filename := strings.TrimSpace(req.Filename)
		if filename == "" {
			return threeDPrinterJSONError("filename is required for start_print; the agent must not guess a file")
		}
		return elegooCentauriCarbonCommandJSON(ctx, printer, 128, map[string]interface{}{
			"Filename":           filename,
			"StartLayer":         req.StartLayer,
			"Calibration_switch": boolAsInt(req.Calibration),
			"PrintPlatformType":  0,
			"Tlp_Switch":         boolAsInt(req.TimeLapse),
		})
	case "pause_print":
		return elegooCentauriCarbonCommandJSON(ctx, printer, 129, map[string]interface{}{})
	case "cancel_print":
		return elegooCentauriCarbonCommandJSON(ctx, printer, 130, map[string]interface{}{})
	case "resume_print":
		return elegooCentauriCarbonCommandJSON(ctx, printer, 131, map[string]interface{}{})
	case "set_camera_light":
		if req.LightOn == nil {
			return threeDPrinterJSONError("light_on is required for set_camera_light")
		}
		return elegooCentauriCarbonCommandJSON(ctx, printer, 403, map[string]interface{}{
			"LightStatus": map[string]interface{}{
				"SecondLight": *req.LightOn,
				"RgbLight":    []int{0, 0, 0},
			},
		})
	default:
		return threeDPrinterJSONError("unknown 3D printer operation")
	}
}

func ResolveThreeDPrinter(cfg ThreeDPrinterConfig, printerID string) (ElegooCentauriCarbonPrinter, error) {
	if !cfg.ElegooCentauriCarbon.Enabled {
		return ElegooCentauriCarbonPrinter{}, fmt.Errorf("Elegoo Centauri Carbon integration is not enabled")
	}
	id := strings.TrimSpace(printerID)
	if id == "" {
		id = strings.TrimSpace(cfg.DefaultPrinter)
	}
	for _, printer := range cfg.ElegooCentauriCarbon.Printers {
		if id == "" || strings.EqualFold(printer.ID, id) || strings.EqualFold(printer.Name, id) {
			if strings.TrimSpace(printer.URL) == "" {
				return ElegooCentauriCarbonPrinter{}, fmt.Errorf("printer %q has no url", printer.ID)
			}
			return printer, nil
		}
	}
	if id == "" {
		return ElegooCentauriCarbonPrinter{}, fmt.Errorf("no 3D printer is configured")
	}
	return ElegooCentauriCarbonPrinter{}, fmt.Errorf("3D printer %q was not found", id)
}

func ElegooCentauriCarbonStatus(ctx context.Context, printer ElegooCentauriCarbonPrinter) string {
	return elegooCentauriCarbonCommandJSON(ctx, printer, 0, map[string]interface{}{})
}

func ElegooCentauriCarbonCameraURL(ctx context.Context, printer ElegooCentauriCarbonPrinter) (string, error) {
	resp, err := elegooCentauriCarbonCommand(ctx, printer, 386, map[string]interface{}{"Enable": 1})
	if err != nil {
		return "", err
	}
	streamURL := findHTTPURL(resp)
	if streamURL == "" {
		encoded, _ := json.Marshal(resp)
		return "", fmt.Errorf("camera stream URL was not found in printer response: %s", string(encoded))
	}
	return streamURL, nil
}

func ValidateThreeDPrinterStreamURL(printerURL, streamURL string) error {
	printerParsed, err := url.Parse(strings.TrimSpace(printerURL))
	if err != nil || printerParsed.Hostname() == "" {
		return fmt.Errorf("invalid configured printer URL")
	}
	streamParsed, err := url.Parse(strings.TrimSpace(streamURL))
	if err != nil || streamParsed.Hostname() == "" {
		return fmt.Errorf("invalid printer stream URL")
	}
	if streamParsed.Scheme != "http" && streamParsed.Scheme != "https" {
		return fmt.Errorf("camera stream uses unsupported scheme %q; only HTTP/MJPEG can be proxied without transcoding", streamParsed.Scheme)
	}
	if !strings.EqualFold(printerParsed.Hostname(), streamParsed.Hostname()) {
		return fmt.Errorf("camera stream host %q does not match configured printer host %q", streamParsed.Hostname(), printerParsed.Hostname())
	}
	return nil
}

func FetchThreeDPrinterSnapshot(ctx context.Context, streamURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create snapshot request: %w", err)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch camera stream: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("camera stream returned HTTP %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	mediaType, params, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		data, err := readHTTPResponseBody(resp.Body, 8*1024*1024)
		if err != nil {
			return nil, "", fmt.Errorf("read snapshot: %w", err)
		}
		return data, http.DetectContentType(data), nil
	case strings.EqualFold(mediaType, "multipart/x-mixed-replace"):
		boundary := params["boundary"]
		if boundary == "" {
			return nil, "", fmt.Errorf("MJPEG stream is missing multipart boundary")
		}
		reader := multipart.NewReader(resp.Body, boundary)
		part, err := reader.NextPart()
		if err != nil {
			return nil, "", fmt.Errorf("read first MJPEG frame: %w", err)
		}
		defer part.Close()
		data, err := readHTTPResponseBody(part, 8*1024*1024)
		if err != nil {
			return nil, "", fmt.Errorf("read first MJPEG frame: %w", err)
		}
		return data, http.DetectContentType(data), nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, "", fmt.Errorf("unsupported camera content type %q: %s", contentType, strings.TrimSpace(string(body)))
	}
}

func StoreThreeDPrinterMedia(dataDir, printerID string, data []byte, contentType string) (ThreeDPrinterMediaResult, error) {
	result := ThreeDPrinterMediaResult{
		Status:      "ok",
		ContentType: contentType,
		Bytes:       len(data),
		Message:     "Snapshot stored successfully.",
	}
	if strings.TrimSpace(dataDir) == "" {
		return result, fmt.Errorf("data directory is required")
	}
	if len(data) == 0 {
		return result, fmt.Errorf("snapshot data is empty")
	}
	now := time.Now().UTC()
	relDir := filepath.Join("3d_printer_media", now.Format("2006"), now.Format("01"), now.Format("02"))
	destDir := filepath.Join(dataDir, relDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return result, fmt.Errorf("create 3D printer media directory: %w", err)
	}
	hashBytes := sha256.Sum256(data)
	hash := hex.EncodeToString(hashBytes[:])
	ext := ".jpg"
	if strings.HasPrefix(contentType, "image/png") {
		ext = ".png"
	} else if strings.HasPrefix(contentType, "image/webp") {
		ext = ".webp"
	}
	filename := fmt.Sprintf("snapshot_%s_%s%s", sanitizeFrigateFileToken(printerID), now.Format("150405.000000000"), ext)
	localPath := filepath.Join(destDir, filename)
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return result, fmt.Errorf("write snapshot: %w", err)
	}
	result.Stored = true
	result.LocalPath = localPath
	result.WebPath = "/files/3d_printer_media/" + strings.ReplaceAll(filepath.ToSlash(filepath.Join(now.Format("2006"), now.Format("01"), now.Format("02"), filename)), " ", "%20")
	result.SHA256 = hash
	return result, nil
}

func elegooCentauriCarbonCommandJSON(ctx context.Context, printer ElegooCentauriCarbonPrinter, cmd int, data map[string]interface{}) string {
	resp, err := elegooCentauriCarbonCommand(ctx, printer, cmd, data)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	return threeDPrinterJSON(resp)
}

func elegooCentauriCarbonCommand(ctx context.Context, printer ElegooCentauriCarbonPrinter, cmd int, data map[string]interface{}) (map[string]interface{}, error) {
	timeout := time.Duration(printer.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, strings.TrimSpace(printer.URL), nil)
	if err != nil {
		return nil, fmt.Errorf("connect Elegoo Centauri Carbon websocket: %w", err)
	}
	defer conn.Close()
	requestID := randomRequestID()
	payload := map[string]interface{}{
		"Id": "",
		"Data": map[string]interface{}{
			"Cmd":         cmd,
			"Data":        data,
			"RequestID":   requestID,
			"MainboardID": printer.MainboardID,
			"TimeStamp":   time.Now().UnixMilli(),
			"From":        1,
		},
	}
	if err := conn.WriteJSON(payload); err != nil {
		return nil, fmt.Errorf("send SDCP command %d: %w", cmd, err)
	}
	for {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		var resp map[string]interface{}
		if err := conn.ReadJSON(&resp); err != nil {
			return nil, fmt.Errorf("read SDCP response for command %d: %w", cmd, err)
		}
		if responseMatches(resp, requestID) || cmd == 0 || cmd == 1 {
			return resp, nil
		}
	}
}

func responseMatches(resp map[string]interface{}, requestID string) bool {
	data, ok := resp["Data"].(map[string]interface{})
	if !ok {
		return false
	}
	got, _ := data["RequestID"].(string)
	return got != "" && got == requestID
}

func findHTTPURL(value interface{}) string {
	switch v := value.(type) {
	case string:
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
			return v
		}
	case map[string]interface{}:
		for _, item := range v {
			if found := findHTTPURL(item); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, item := range v {
			if found := findHTTPURL(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func executeThreeDPrinterSnapshot(ctx context.Context, cfg ThreeDPrinterConfig, printer ElegooCentauriCarbonPrinter) string {
	streamURL, err := ElegooCentauriCarbonCameraURL(ctx, printer)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	if err := ValidateThreeDPrinterStreamURL(printer.URL, streamURL); err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	data, contentType, err := FetchThreeDPrinterSnapshot(ctx, streamURL)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	result, err := StoreThreeDPrinterMedia(cfg.DataDir, printer.ID, data, contentType)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	return threeDPrinterJSON(result)
}

func threeDPrinterMutates(operation string) bool {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "start_print", "pause_print", "resume_print", "cancel_print", "set_camera_light":
		return true
	default:
		return false
	}
}

func threeDPrinterJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	if bytes.Contains(data, []byte(`"status"`)) {
		return string(data)
	}
	var obj map[string]interface{}
	if json.Unmarshal(data, &obj) == nil {
		obj["status"] = "ok"
		data, _ = json.Marshal(obj)
	}
	return string(data)
}

func threeDPrinterJSONError(message string) string {
	data, _ := json.Marshal(map[string]interface{}{"status": "error", "message": message})
	return string(data)
}

func randomRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

func boolAsInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
