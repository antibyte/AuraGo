package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestElegooCentauriCarbonStatusSendsSDCPCommand(t *testing.T) {
	wsURL, closeServer := mockElegooWebSocket(t, func(t *testing.T, payload map[string]interface{}, conn *websocket.Conn) {
		data := payload["Data"].(map[string]interface{})
		if got := int(data["Cmd"].(float64)); got != 0 {
			t.Fatalf("Cmd = %d, want 0", got)
		}
		if strings.TrimSpace(data["RequestID"].(string)) == "" {
			t.Fatal("RequestID should be populated")
		}
		if got := int(data["From"].(float64)); got != 1 {
			t.Fatalf("From = %d, want 1", got)
		}
		if err := conn.WriteJSON(map[string]interface{}{
			"Status": map[string]interface{}{
				"PrintInfo": map[string]interface{}{"Status": 13, "Progress": 42},
			},
			"Topic": "sdcp/status/mainboard",
		}); err != nil {
			t.Fatalf("WriteJSON error = %v", err)
		}
	})
	defer closeServer()

	out := ElegooCentauriCarbonStatus(context.Background(), ElegooCentauriCarbonPrinter{
		ID:             "lab",
		URL:            wsURL,
		TimeoutSeconds: 2,
	})
	if !strings.Contains(out, `"Progress":42`) {
		t.Fatalf("unexpected status output: %s", out)
	}
}

func TestThreeDPrinterExecuteBlocksMutationsInReadOnlyMode(t *testing.T) {
	cfg := ThreeDPrinterConfig{
		Enabled:        true,
		ReadOnly:       true,
		DefaultPrinter: "lab",
		ElegooCentauriCarbon: ElegooCentauriCarbonConfig{
			Enabled: true,
			Printers: []ElegooCentauriCarbonPrinter{{
				ID:  "lab",
				URL: "ws://192.168.1.50/websocket",
			}},
		},
	}
	out := ExecuteThreeDPrinter(context.Background(), cfg, ThreeDPrinterRequest{Operation: "pause_print", PrinterID: "lab"})
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(strings.ToLower(out), "read-only") {
		t.Fatalf("expected read-only error, got: %s", out)
	}
}

func TestThreeDPrinterStartPrintRequiresExplicitFilename(t *testing.T) {
	cfg := ThreeDPrinterConfig{
		Enabled:        true,
		ReadOnly:       false,
		DefaultPrinter: "lab",
		ElegooCentauriCarbon: ElegooCentauriCarbonConfig{
			Enabled: true,
			Printers: []ElegooCentauriCarbonPrinter{{
				ID:  "lab",
				URL: "ws://192.168.1.50/websocket",
			}},
		},
	}
	out := ExecuteThreeDPrinter(context.Background(), cfg, ThreeDPrinterRequest{Operation: "start_print", PrinterID: "lab"})
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(strings.ToLower(out), "filename") {
		t.Fatalf("expected filename error, got: %s", out)
	}
}

func TestThreeDPrinterListPrintersReturnsConfiguredPrinters(t *testing.T) {
	cfg := ThreeDPrinterConfig{
		Enabled:        true,
		DefaultPrinter: "lab",
		ElegooCentauriCarbon: ElegooCentauriCarbonConfig{
			Enabled:  true,
			Printers: []ElegooCentauriCarbonPrinter{{ID: "lab", URL: "ws://192.168.1.50/websocket"}},
		},
		Klipper: KlipperConfig{
			Enabled:  true,
			Printers: []KlipperPrinter{{ID: "voron", URL: "http://192.168.1.60:7125"}},
		},
	}
	out := ExecuteThreeDPrinter(context.Background(), cfg, ThreeDPrinterRequest{Operation: "list_printers"})
	if !strings.Contains(out, `"default_printer":"lab"`) || !strings.Contains(out, `"id":"voron"`) {
		t.Fatalf("unexpected list output: %s", out)
	}
}

func TestElegooCentauriCarbonMutationAndInfoCommandsUseExpectedSDCPCommands(t *testing.T) {
	tests := []struct {
		name      string
		req       ThreeDPrinterRequest
		wantCmd   int
		wantField string
	}{
		{name: "attributes", req: ThreeDPrinterRequest{Operation: "attributes"}, wantCmd: sdcpCmdAttributes},
		{name: "history", req: ThreeDPrinterRequest{Operation: "history"}, wantCmd: sdcpCmdHistory},
		{name: "files custom dir", req: ThreeDPrinterRequest{Operation: "files", Directory: "/usb"}, wantCmd: sdcpCmdFiles, wantField: "/usb"},
		{name: "pause", req: ThreeDPrinterRequest{Operation: "pause_print"}, wantCmd: sdcpCmdPausePrint},
		{name: "resume", req: ThreeDPrinterRequest{Operation: "resume_print"}, wantCmd: sdcpCmdResumePrint},
		{name: "cancel", req: ThreeDPrinterRequest{Operation: "cancel_print"}, wantCmd: sdcpCmdCancelPrint},
		{name: "camera light", req: ThreeDPrinterRequest{Operation: "set_camera_light", LightOn: boolPtr(true)}, wantCmd: sdcpCmdCameraLight},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wsURL, closeServer := mockElegooWebSocket(t, func(t *testing.T, payload map[string]interface{}, conn *websocket.Conn) {
				data := payload["Data"].(map[string]interface{})
				if got := int(data["Cmd"].(float64)); got != tt.wantCmd {
					t.Fatalf("Cmd = %d, want %d", got, tt.wantCmd)
				}
				if tt.wantField != "" {
					cmdData := data["Data"].(map[string]interface{})
					if got := cmdData["Url"]; got != tt.wantField {
						t.Fatalf("Url = %v, want %q", got, tt.wantField)
					}
				}
				requestID := data["RequestID"].(string)
				if err := conn.WriteJSON(map[string]interface{}{
					"Data": map[string]interface{}{
						"Cmd":       tt.wantCmd,
						"RequestID": requestID,
						"Ack":       true,
					},
				}); err != nil {
					t.Fatalf("WriteJSON error = %v", err)
				}
			})
			defer closeServer()

			cfg := ThreeDPrinterConfig{
				Enabled:        true,
				ReadOnly:       false,
				DefaultPrinter: "lab",
				ElegooCentauriCarbon: ElegooCentauriCarbonConfig{
					Enabled:  true,
					Printers: []ElegooCentauriCarbonPrinter{{ID: "lab", URL: wsURL, TimeoutSeconds: 2}},
				},
			}
			tt.req.PrinterID = "lab"
			out := ExecuteThreeDPrinter(context.Background(), cfg, tt.req)
			if !strings.Contains(out, `"status":"ok"`) {
				t.Fatalf("unexpected output: %s", out)
			}
		})
	}
}

func TestValidateThreeDPrinterStreamURLRequiresConfiguredHost(t *testing.T) {
	if err := ValidateThreeDPrinterStreamURL("ws://192.168.1.50/websocket", "http://192.168.1.50:8080/video"); err != nil {
		t.Fatalf("expected matching host to pass: %v", err)
	}
	if err := ValidateThreeDPrinterStreamURL("ws://192.168.1.50/websocket", "http://192.168.1.99:8080/video"); err == nil {
		t.Fatal("expected mismatched stream host to fail")
	}
	if err := ValidateThreeDPrinterStreamURL("ws://192.168.1.50/websocket", "rtsp://192.168.1.50/live"); err == nil {
		t.Fatal("expected unsupported stream scheme to fail")
	}
}

func TestKlipperStatusSendsMoonrakerObjectsQuery(t *testing.T) {
	var gotMethod, gotPath string
	server := mockKlipperHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if r.Header.Get("X-Api-Key") != "moon-key" {
			t.Fatalf("X-Api-Key = %q, want moon-key", r.Header.Get("X-Api-Key"))
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		objects, ok := body["objects"].(map[string]interface{})
		if !ok {
			t.Fatalf("objects missing print_stats: %#v", body)
		}
		if _, ok := objects["print_stats"]; !ok {
			t.Fatalf("objects missing print_stats: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{"status": map[string]interface{}{"print_stats": map[string]interface{}{"state": "printing"}}},
		})
	})
	defer server.Close()

	cfg := klipperOnlyConfig(server.URL)
	cfg.Klipper.Printers[0].APIKey = "moon-key"
	out := ExecuteThreeDPrinter(context.Background(), cfg, ThreeDPrinterRequest{Operation: "status", PrinterID: "voron"})
	if gotMethod != http.MethodPost || gotPath != "/printer/objects/query" {
		t.Fatalf("request = %s %s, want POST /printer/objects/query", gotMethod, gotPath)
	}
	if !strings.Contains(out, `"state":"printing"`) {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("expected top-level ok status, got: %s", out)
	}
	if strings.Contains(out, "moon-key") {
		t.Fatalf("tool output leaked API key: %s", out)
	}
}

func TestKlipperFilesUsesGcodesRoot(t *testing.T) {
	var rawQuery string
	server := mockKlipperHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/files/list" {
			t.Fatalf("path = %s, want /server/files/list", r.URL.Path)
		}
		rawQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": []map[string]interface{}{{"path": "cube.gcode"}}})
	})
	defer server.Close()

	out := ExecuteThreeDPrinter(context.Background(), klipperOnlyConfig(server.URL), ThreeDPrinterRequest{Operation: "files", PrinterID: "voron"})
	values, _ := url.ParseQuery(rawQuery)
	if values.Get("root") != "gcodes" {
		t.Fatalf("root query = %q, want gcodes", values.Get("root"))
	}
	if !strings.Contains(out, "cube.gcode") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestKlipperStartPrintRequiresFilenameAndCallsMoonraker(t *testing.T) {
	var gotPath, gotFilename string
	server := mockKlipperHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotFilename = r.URL.Query().Get("filename")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
	})
	defer server.Close()

	out := ExecuteThreeDPrinter(context.Background(), klipperOnlyConfig(server.URL), ThreeDPrinterRequest{
		Operation: "start_print",
		PrinterID: "voron",
		Filename:  "calibration.gcode",
	})
	if gotPath != "/printer/print/start" || gotFilename != "calibration.gcode" {
		t.Fatalf("request = %s filename=%q", gotPath, gotFilename)
	}
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestKlipperMutationAndInfoCommandsUseExpectedMoonrakerPaths(t *testing.T) {
	tests := []struct {
		operation  string
		wantMethod string
		wantPath   string
	}{
		{operation: "attributes", wantMethod: http.MethodGet, wantPath: "/printer/objects/list"},
		{operation: "history", wantMethod: http.MethodGet, wantPath: "/server/history/list"},
		{operation: "pause_print", wantMethod: http.MethodPost, wantPath: "/printer/print/pause"},
		{operation: "resume_print", wantMethod: http.MethodPost, wantPath: "/printer/print/resume"},
		{operation: "cancel_print", wantMethod: http.MethodPost, wantPath: "/printer/print/cancel"},
	}
	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			var gotMethod, gotPath string
			server := mockKlipperHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
			})
			defer server.Close()

			out := ExecuteThreeDPrinter(context.Background(), klipperOnlyConfig(server.URL), ThreeDPrinterRequest{Operation: tt.operation, PrinterID: "voron"})
			if gotMethod != tt.wantMethod || gotPath != tt.wantPath {
				t.Fatalf("request = %s %s, want %s %s", gotMethod, gotPath, tt.wantMethod, tt.wantPath)
			}
			if !strings.Contains(out, `"status":"ok"`) {
				t.Fatalf("unexpected output: %s", out)
			}
		})
	}
}

func TestThreeDPrinterExecuteBlocksKlipperMutationsInReadOnlyMode(t *testing.T) {
	called := false
	server := mockKlipperHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	cfg := klipperOnlyConfig(server.URL)
	cfg.ReadOnly = true
	out := ExecuteThreeDPrinter(context.Background(), cfg, ThreeDPrinterRequest{Operation: "pause_print", PrinterID: "voron"})
	if called {
		t.Fatal("read-only mutation contacted Moonraker")
	}
	if !strings.Contains(out, `"status":"error"`) || !strings.Contains(strings.ToLower(out), "read-only") {
		t.Fatalf("expected read-only error, got: %s", out)
	}
}

func TestKlipperCameraURLSelectsConfiguredWebcam(t *testing.T) {
	server := mockKlipperHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/webcams/list" {
			t.Fatalf("path = %s, want /server/webcams/list", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"webcams": []map[string]interface{}{
					{"name": "bed", "enabled": true, "stream_url": "/webcam/bed"},
					{"name": "toolhead", "enabled": true, "stream_url": "/webcam/toolhead", "snapshot_url": "/snapshot/toolhead.jpg"},
				},
			},
		})
	})
	defer server.Close()

	cfg := klipperOnlyConfig(server.URL)
	cfg.Klipper.Printers[0].WebcamName = "toolhead"
	out := ExecuteThreeDPrinter(context.Background(), cfg, ThreeDPrinterRequest{Operation: "camera_url", PrinterID: "voron"})
	if !strings.Contains(out, server.URL+"/webcam/toolhead") {
		t.Fatalf("unexpected output: %s", out)
	}
	if !strings.Contains(out, `"/api/3d-printers/voron/camera/stream"`) {
		t.Fatalf("camera_url output missing same-origin proxy URL: %s", out)
	}
}

func TestElegooCentauriCarbonCameraURLPrefersSchemaURL(t *testing.T) {
	wsURL, closeServer := mockElegooWebSocket(t, func(t *testing.T, payload map[string]interface{}, conn *websocket.Conn) {
		data := payload["Data"].(map[string]interface{})
		requestID := data["RequestID"].(string)
		if err := conn.WriteJSON(map[string]interface{}{
			"Url": "http://192.168.1.50/not-camera",
			"Data": map[string]interface{}{
				"RequestID": requestID,
				"Data": map[string]interface{}{
					"Url": "http://192.168.1.50/camera-stream",
				},
			},
		}); err != nil {
			t.Fatalf("WriteJSON error = %v", err)
		}
	})
	defer closeServer()

	got, err := ElegooCentauriCarbonCameraURL(context.Background(), ElegooCentauriCarbonPrinter{ID: "lab", URL: wsURL, TimeoutSeconds: 2})
	if err != nil {
		t.Fatalf("ElegooCentauriCarbonCameraURL error = %v", err)
	}
	if got != "http://192.168.1.50/camera-stream" {
		t.Fatalf("camera URL = %q, want schema URL", got)
	}
}

func TestElegooCentauriCarbonCameraURLNormalizesSchemelessVideoURL(t *testing.T) {
	wsURL, closeServer := mockElegooWebSocket(t, func(t *testing.T, payload map[string]interface{}, conn *websocket.Conn) {
		data := payload["Data"].(map[string]interface{})
		requestID := data["RequestID"].(string)
		if err := conn.WriteJSON(map[string]interface{}{
			"Data": map[string]interface{}{
				"RequestID": requestID,
				"VideoUrl":  "192.168.6.181:3031/video",
			},
		}); err != nil {
			t.Fatalf("WriteJSON error = %v", err)
		}
	})
	defer closeServer()

	got, err := ElegooCentauriCarbonCameraURL(context.Background(), ElegooCentauriCarbonPrinter{ID: "lab", URL: wsURL, TimeoutSeconds: 2})
	if err != nil {
		t.Fatalf("ElegooCentauriCarbonCameraURL error = %v", err)
	}
	if got != "http://192.168.6.181:3031/video" {
		t.Fatalf("camera URL = %q, want normalized VideoUrl", got)
	}
}

func TestElegooCentauriCarbonCommandStopsAfterTooManyUnrelatedResponses(t *testing.T) {
	wsURL, closeServer := mockElegooWebSocket(t, func(t *testing.T, payload map[string]interface{}, conn *websocket.Conn) {
		for i := 0; i < 51; i++ {
			if err := conn.WriteJSON(map[string]interface{}{
				"Data": map[string]interface{}{
					"RequestID": "other-request",
				},
			}); err != nil {
				t.Fatalf("WriteJSON error = %v", err)
			}
		}
	})
	defer closeServer()

	_, err := elegooCentauriCarbonCommand(context.Background(), ElegooCentauriCarbonPrinter{ID: "lab", URL: wsURL, TimeoutSeconds: 5}, sdcpCmdCameraURL, map[string]interface{}{})
	if err == nil || !strings.Contains(err.Error(), "unrelated") {
		t.Fatalf("err = %v, want unrelated response limit error", err)
	}
}

func TestStoreThreeDPrinterMediaWritesSafeFileAndRegistersMedia(t *testing.T) {
	dataDir := t.TempDir()
	db := initThreeDPrinterMediaTestDB(t)

	result, err := StoreThreeDPrinterMedia(dataDir, db, `door/../../event 1`, []byte{0xff, 0xd8, 0xff, 0xdb}, "image/jpeg")
	if err != nil {
		t.Fatalf("StoreThreeDPrinterMedia error = %v", err)
	}
	if !result.Stored {
		t.Fatal("expected stored result")
	}
	if result.LocalPath == "" || !strings.HasPrefix(result.LocalPath, filepath.Join(dataDir, "3d_printer_media")) {
		t.Fatalf("local path %q should stay inside data/3d_printer_media", result.LocalPath)
	}
	if strings.Contains(filepath.Base(result.LocalPath), "..") || strings.Contains(filepath.Base(result.LocalPath), "/") || strings.Contains(filepath.Base(result.LocalPath), `\`) {
		t.Fatalf("unsafe filename %q", filepath.Base(result.LocalPath))
	}
	if result.WebPath == "" || !strings.HasPrefix(result.WebPath, "/files/3d_printer_media/") {
		t.Fatalf("web path = %q, want /files/3d_printer_media prefix", result.WebPath)
	}
	if result.SHA256 == "" {
		t.Fatal("expected sha256")
	}
	if result.MediaID == 0 {
		t.Fatal("expected media registry id")
	}
	if _, err := os.Stat(result.LocalPath); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func TestStoreThreeDPrinterMediaRejectsInvalidInputs(t *testing.T) {
	if _, err := StoreThreeDPrinterMedia("", nil, "lab", []byte{1}, "image/jpeg"); err == nil || !strings.Contains(err.Error(), "data directory") {
		t.Fatalf("empty data dir err = %v, want data directory error", err)
	}
	if _, err := StoreThreeDPrinterMedia(t.TempDir(), nil, "lab", nil, "image/jpeg"); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty data err = %v, want empty data error", err)
	}
}

func initThreeDPrinterMediaTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatalf("InitMediaRegistryDB error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func klipperOnlyConfig(baseURL string) ThreeDPrinterConfig {
	return ThreeDPrinterConfig{
		Enabled:        true,
		ReadOnly:       false,
		DefaultPrinter: "voron",
		Klipper: KlipperConfig{
			Enabled: true,
			Printers: []KlipperPrinter{{
				ID:             "voron",
				Name:           "Voron 2.4",
				URL:            baseURL,
				TimeoutSeconds: 2,
			}},
		},
	}
}

func mockKlipperHTTPServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
}

func mockElegooWebSocket(t *testing.T, handler func(*testing.T, map[string]interface{}, *websocket.Conn)) (string, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error = %v", err)
		}
		defer conn.Close()
		var payload map[string]interface{}
		if err := conn.ReadJSON(&payload); err != nil {
			t.Fatalf("ReadJSON error = %v", err)
		}
		encoded, _ := json.Marshal(payload)
		if !json.Valid(encoded) {
			t.Fatalf("invalid payload: %#v", payload)
		}
		handler(t, payload, conn)
	}))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	return wsURL, server.Close
}
