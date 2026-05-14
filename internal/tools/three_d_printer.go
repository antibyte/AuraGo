package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
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

	"aurago/internal/config"

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

type KlipperPrinter struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	APIKey         string `json:"-"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	WebcamName     string `json:"webcam_name,omitempty"`
}

type KlipperConfig struct {
	Enabled  bool             `json:"enabled"`
	Printers []KlipperPrinter `json:"printers"`
}

type ThreeDPrinterConfig struct {
	Enabled              bool                       `json:"enabled"`
	ReadOnly             bool                       `json:"readonly"`
	DefaultPrinter       string                     `json:"default_printer"`
	DataDir              string                     `json:"-"`
	MediaDB              *sql.DB                    `json:"-"`
	ElegooCentauriCarbon ElegooCentauriCarbonConfig `json:"elegoo_centauri_carbon"`
	Klipper              KlipperConfig              `json:"klipper"`
}

type ThreeDPrinterRequest struct {
	Operation         string `json:"operation"`
	Protocol          string `json:"protocol,omitempty"`
	PrinterID         string `json:"printer_id"`
	URL               string `json:"url,omitempty"`
	APIKey            string `json:"api_key,omitempty"`
	MainboardID       string `json:"mainboard_id,omitempty"`
	WebcamName        string `json:"webcam_name,omitempty"`
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
	MediaID     int64  `json:"media_id,omitempty"`
	Message     string `json:"message"`
}

type ResolvedThreeDPrinter struct {
	Protocol string
	ID       string
	Name     string
	URL      string
	Elegoo   *ElegooCentauriCarbonPrinter
	Klipper  *KlipperPrinter
}

func BuildThreeDPrinterRuntimeConfig(cfg *config.Config) ThreeDPrinterConfig {
	if cfg == nil {
		return ThreeDPrinterConfig{}
	}
	printers := make([]ElegooCentauriCarbonPrinter, 0, len(cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers))
	for _, printer := range cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers {
		printers = append(printers, ElegooCentauriCarbonPrinter{
			ID:             printer.ID,
			Name:           printer.Name,
			URL:            printer.URL,
			MainboardID:    printer.MainboardID,
			TimeoutSeconds: printer.TimeoutSeconds,
		})
	}
	klipperPrinters := make([]KlipperPrinter, 0, len(cfg.ThreeDPrinters.Klipper.Printers))
	for _, printer := range cfg.ThreeDPrinters.Klipper.Printers {
		klipperPrinters = append(klipperPrinters, KlipperPrinter{
			ID:             printer.ID,
			Name:           printer.Name,
			URL:            printer.URL,
			APIKey:         printer.APIKey,
			TimeoutSeconds: printer.TimeoutSeconds,
			WebcamName:     printer.WebcamName,
		})
	}
	return ThreeDPrinterConfig{
		Enabled:        cfg.ThreeDPrinters.Enabled,
		ReadOnly:       cfg.ThreeDPrinters.ReadOnly,
		DefaultPrinter: cfg.ThreeDPrinters.DefaultPrinter,
		DataDir:        cfg.Directories.DataDir,
		ElegooCentauriCarbon: ElegooCentauriCarbonConfig{
			Enabled:  cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled,
			Printers: printers,
		},
		Klipper: KlipperConfig{
			Enabled:  cfg.ThreeDPrinters.Klipper.Enabled,
			Printers: klipperPrinters,
		},
	}
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
		return threeDPrinterJSON(map[string]interface{}{
			"status":          "ok",
			"default_printer": cfg.DefaultPrinter,
			"elegoo_centauri_carbon": map[string]interface{}{
				"enabled":  cfg.ElegooCentauriCarbon.Enabled,
				"printers": cfg.ElegooCentauriCarbon.Printers,
			},
			"klipper": map[string]interface{}{
				"enabled":  cfg.Klipper.Enabled,
				"printers": cfg.Klipper.Printers,
			},
		})
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
		if printer.Klipper != nil {
			if operation == "test_connection" {
				return klipperCommandJSON(ctx, *printer.Klipper, http.MethodGet, "/server/info", nil, nil)
			}
			return klipperStatus(ctx, *printer.Klipper)
		}
		return ElegooCentauriCarbonStatus(ctx, *printer.Elegoo)
	case "attributes":
		if printer.Klipper != nil {
			return klipperCommandJSON(ctx, *printer.Klipper, http.MethodGet, "/printer/objects/list", nil, nil)
		}
		return elegooCentauriCarbonCommandJSON(ctx, *printer.Elegoo, 1, map[string]interface{}{})
	case "files":
		if printer.Klipper != nil {
			return klipperCommandJSON(ctx, *printer.Klipper, http.MethodGet, "/server/files/list", url.Values{"root": []string{"gcodes"}}, nil)
		}
		dir := strings.TrimSpace(req.Directory)
		if dir == "" {
			dir = "/local"
		}
		return elegooCentauriCarbonCommandJSON(ctx, *printer.Elegoo, 258, map[string]interface{}{"Url": dir})
	case "history":
		if printer.Klipper != nil {
			return klipperCommandJSON(ctx, *printer.Klipper, http.MethodGet, "/server/history/list", url.Values{"limit": []string{"20"}}, nil)
		}
		return elegooCentauriCarbonCommandJSON(ctx, *printer.Elegoo, 320, map[string]interface{}{})
	case "camera_url":
		streamURL, _, err := ResolveThreeDPrinterCameraURLs(ctx, printer)
		if err != nil {
			return threeDPrinterJSONError(err.Error())
		}
		return threeDPrinterJSON(map[string]interface{}{"status": "ok", "printer_id": printer.ID, "protocol": printer.Protocol, "url": streamURL})
	case "camera_snapshot":
		return executeThreeDPrinterSnapshot(ctx, cfg, printer)
	case "show_live_stream":
		streamURL, _, err := ResolveThreeDPrinterCameraURLs(ctx, printer)
		if err != nil {
			return threeDPrinterJSONError(err.Error())
		}
		proxyURL := "/api/3d-printers/" + url.PathEscape(printer.ID) + "/camera/stream"
		if err := ValidateThreeDPrinterStreamURL(printer.URL, streamURL); err != nil {
			return threeDPrinterJSON(map[string]interface{}{"status": "fallback", "message": err.Error(), "stream_url": streamURL})
		}
		return threeDPrinterJSON(map[string]interface{}{"status": "ok", "printer_id": printer.ID, "protocol": printer.Protocol, "stream_url": streamURL, "proxy_url": proxyURL, "mime_type": "multipart/x-mixed-replace"})
	case "start_print":
		filename := strings.TrimSpace(req.Filename)
		if filename == "" {
			return threeDPrinterJSONError("filename is required for start_print; the agent must not guess a file")
		}
		if printer.Klipper != nil {
			return klipperCommandJSON(ctx, *printer.Klipper, http.MethodPost, "/printer/print/start", url.Values{"filename": []string{filename}}, nil)
		}
		return elegooCentauriCarbonCommandJSON(ctx, *printer.Elegoo, 128, map[string]interface{}{
			"Filename":           filename,
			"StartLayer":         req.StartLayer,
			"Calibration_switch": boolAsInt(req.Calibration),
			"PrintPlatformType":  0,
			"Tlp_Switch":         boolAsInt(req.TimeLapse),
		})
	case "pause_print":
		if printer.Klipper != nil {
			return klipperCommandJSON(ctx, *printer.Klipper, http.MethodPost, "/printer/print/pause", nil, nil)
		}
		return elegooCentauriCarbonCommandJSON(ctx, *printer.Elegoo, 129, map[string]interface{}{})
	case "cancel_print":
		if printer.Klipper != nil {
			return klipperCommandJSON(ctx, *printer.Klipper, http.MethodPost, "/printer/print/cancel", nil, nil)
		}
		return elegooCentauriCarbonCommandJSON(ctx, *printer.Elegoo, 130, map[string]interface{}{})
	case "resume_print":
		if printer.Klipper != nil {
			return klipperCommandJSON(ctx, *printer.Klipper, http.MethodPost, "/printer/print/resume", nil, nil)
		}
		return elegooCentauriCarbonCommandJSON(ctx, *printer.Elegoo, 131, map[string]interface{}{})
	case "set_camera_light":
		if printer.Klipper != nil {
			return threeDPrinterJSONError("set_camera_light is not supported for Klipper in standard-actions mode")
		}
		if req.LightOn == nil {
			return threeDPrinterJSONError("light_on is required for set_camera_light")
		}
		return elegooCentauriCarbonCommandJSON(ctx, *printer.Elegoo, 403, map[string]interface{}{
			"LightStatus": map[string]interface{}{
				"SecondLight": *req.LightOn,
				"RgbLight":    []int{0, 0, 0},
			},
		})
	default:
		return threeDPrinterJSONError("unknown 3D printer operation")
	}
}

func ResolveThreeDPrinter(cfg ThreeDPrinterConfig, printerID string) (ResolvedThreeDPrinter, error) {
	id := strings.TrimSpace(printerID)
	if id == "" {
		id = strings.TrimSpace(cfg.DefaultPrinter)
	}

	if cfg.ElegooCentauriCarbon.Enabled {
		for _, printer := range cfg.ElegooCentauriCarbon.Printers {
			if id == "" || strings.EqualFold(printer.ID, id) || strings.EqualFold(printer.Name, id) {
				if strings.TrimSpace(printer.URL) == "" {
					return ResolvedThreeDPrinter{}, fmt.Errorf("printer %q has no url", printer.ID)
				}
				p := printer
				return ResolvedThreeDPrinter{Protocol: "elegoo_centauri_carbon", ID: printer.ID, Name: printer.Name, URL: printer.URL, Elegoo: &p}, nil
			}
		}
	}
	if cfg.Klipper.Enabled {
		for _, printer := range cfg.Klipper.Printers {
			if id == "" || strings.EqualFold(printer.ID, id) || strings.EqualFold(printer.Name, id) {
				if strings.TrimSpace(printer.URL) == "" {
					return ResolvedThreeDPrinter{}, fmt.Errorf("printer %q has no url", printer.ID)
				}
				p := printer
				return ResolvedThreeDPrinter{Protocol: "klipper", ID: printer.ID, Name: printer.Name, URL: printer.URL, Klipper: &p}, nil
			}
		}
	}
	if id == "" {
		return ResolvedThreeDPrinter{}, fmt.Errorf("no 3D printer is configured")
	}
	return ResolvedThreeDPrinter{}, fmt.Errorf("3D printer %q was not found", id)
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

func StoreThreeDPrinterMedia(dataDir string, mediaDB *sql.DB, printerID string, data []byte, contentType string) (ThreeDPrinterMediaResult, error) {
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
	absDataDir, err := filepath.Abs(strings.TrimSpace(dataDir))
	if err != nil {
		return result, fmt.Errorf("resolve data directory: %w", err)
	}
	dataDir = filepath.Clean(absDataDir)
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
	filename := fmt.Sprintf("snapshot_%s_%s%s", sanitizeMediaFileToken(printerID), now.Format("150405.000000000"), ext)
	localPath := filepath.Join(destDir, filename)
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return result, fmt.Errorf("write snapshot: %w", err)
	}
	webPath := "/files/3d_printer_media/" + strings.ReplaceAll(filepath.ToSlash(filepath.Join(now.Format("2006"), now.Format("01"), now.Format("02"), filename)), " ", "%20")
	result.Stored = true
	result.LocalPath = localPath
	result.WebPath = webPath
	result.SHA256 = hash
	if mediaDB != nil {
		mediaID, _, err := RegisterMedia(mediaDB, MediaItem{
			MediaType:   "image",
			SourceTool:  "three_d_printer",
			Filename:    filename,
			FilePath:    localPath,
			WebPath:     webPath,
			FileSize:    int64(len(data)),
			Format:      strings.TrimPrefix(ext, "."),
			Provider:    "three_d_printer",
			Description: fmt.Sprintf("3D printer snapshot for %s", printerID),
			Tags:        []string{"3d_printer", "snapshot", printerID},
			Hash:        hash,
		})
		if err != nil {
			return result, fmt.Errorf("register 3D printer media: %w", err)
		}
		result.MediaID = mediaID
	}
	return result, nil
}

func klipperStatus(ctx context.Context, printer KlipperPrinter) string {
	body := map[string]interface{}{
		"objects": map[string]interface{}{
			"webhooks":       nil,
			"print_stats":    nil,
			"toolhead":       []string{"position", "homed_axes"},
			"extruder":       []string{"temperature", "target"},
			"heater_bed":     []string{"temperature", "target"},
			"display_status": nil,
			"virtual_sdcard": nil,
		},
	}
	return klipperCommandJSON(ctx, printer, http.MethodPost, "/printer/objects/query", nil, body)
}

func klipperCommandJSON(ctx context.Context, printer KlipperPrinter, method, path string, query url.Values, body interface{}) string {
	resp, err := klipperCommand(ctx, printer, method, path, query, body)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	return threeDPrinterJSON(resp)
}

func klipperCommand(ctx context.Context, printer KlipperPrinter, method, path string, query url.Values, body interface{}) (map[string]interface{}, error) {
	timeout := time.Duration(printer.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint, err := klipperEndpoint(printer.URL, path, query)
	if err != nil {
		return nil, err
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode Moonraker request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("create Moonraker request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(printer.APIKey) != "" {
		req.Header.Set("X-Api-Key", strings.TrimSpace(printer.APIKey))
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Moonraker %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("Moonraker %s %s returned HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var decoded map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode Moonraker response: %w", err)
	}
	return decoded, nil
}

func klipperEndpoint(baseURL, path string, query url.Values) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid Klipper Moonraker URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("Klipper Moonraker URL must use http or https")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func ResolveThreeDPrinterCameraURLs(ctx context.Context, printer ResolvedThreeDPrinter) (string, string, error) {
	if printer.Elegoo != nil {
		streamURL, err := ElegooCentauriCarbonCameraURL(ctx, *printer.Elegoo)
		return streamURL, "", err
	}
	if printer.Klipper != nil {
		return KlipperCameraURLs(ctx, *printer.Klipper)
	}
	return "", "", fmt.Errorf("unknown 3D printer protocol")
}

func KlipperCameraURLs(ctx context.Context, printer KlipperPrinter) (string, string, error) {
	resp, err := klipperCommand(ctx, printer, http.MethodGet, "/server/webcams/list", nil, nil)
	if err != nil {
		return "", "", err
	}
	webcam, err := selectKlipperWebcam(resp, printer.WebcamName)
	if err != nil {
		return "", "", err
	}
	streamURL, _ := webcam["stream_url"].(string)
	snapshotURL, _ := webcam["snapshot_url"].(string)
	if strings.TrimSpace(streamURL) == "" {
		return "", "", fmt.Errorf("Klipper webcam stream_url was not found")
	}
	streamURL, err = resolvePrinterHTTPURL(printer.URL, streamURL)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(snapshotURL) != "" {
		snapshotURL, err = resolvePrinterHTTPURL(printer.URL, snapshotURL)
		if err != nil {
			return "", "", err
		}
	}
	return streamURL, snapshotURL, nil
}

func selectKlipperWebcam(resp map[string]interface{}, wantedName string) (map[string]interface{}, error) {
	var webcams []interface{}
	if result, ok := resp["result"].(map[string]interface{}); ok {
		if arr, ok := result["webcams"].([]interface{}); ok {
			webcams = arr
		}
	} else if arr, ok := resp["result"].([]interface{}); ok {
		webcams = arr
	}
	wantedName = strings.TrimSpace(wantedName)
	var firstEnabled map[string]interface{}
	for _, item := range webcams {
		webcam, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		enabled, hasEnabled := webcam["enabled"].(bool)
		if hasEnabled && !enabled {
			continue
		}
		name, _ := webcam["name"].(string)
		if wantedName != "" && strings.EqualFold(strings.TrimSpace(name), wantedName) {
			return webcam, nil
		}
		if firstEnabled == nil {
			firstEnabled = webcam
		}
	}
	if wantedName != "" {
		return nil, fmt.Errorf("Klipper webcam %q was not found", wantedName)
	}
	if firstEnabled == nil {
		return nil, fmt.Errorf("no enabled Klipper webcam was found")
	}
	return firstEnabled, nil
}

func resolvePrinterHTTPURL(baseURL, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid printer camera URL: %w", err)
	}
	if parsed.IsAbs() {
		return parsed.String(), nil
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid configured printer URL")
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		base.Scheme = "http"
	}
	return base.ResolveReference(parsed).String(), nil
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

func executeThreeDPrinterSnapshot(ctx context.Context, cfg ThreeDPrinterConfig, printer ResolvedThreeDPrinter) string {
	streamURL, snapshotURL, err := ResolveThreeDPrinterCameraURLs(ctx, printer)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	if err := ValidateThreeDPrinterStreamURL(printer.URL, streamURL); err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	fetchURL := streamURL
	if snapshotURL != "" {
		if err := ValidateThreeDPrinterStreamURL(printer.URL, snapshotURL); err != nil {
			return threeDPrinterJSONError(err.Error())
		}
		fetchURL = snapshotURL
	}
	data, contentType, err := FetchThreeDPrinterSnapshot(ctx, fetchURL)
	if err != nil {
		return threeDPrinterJSONError(err.Error())
	}
	result, err := StoreThreeDPrinterMedia(cfg.DataDir, cfg.MediaDB, printer.ID, data, contentType)
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
	var obj map[string]interface{}
	if json.Unmarshal(data, &obj) == nil {
		if _, ok := obj["status"]; !ok {
			obj["status"] = "ok"
			data, _ = json.Marshal(obj)
		}
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
