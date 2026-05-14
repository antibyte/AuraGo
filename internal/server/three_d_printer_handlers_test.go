package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"

	"github.com/gorilla/websocket"
)

func TestHandleThreeDPrinterSnapshotStoresImageFromConfiguredPrinterHost(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(testJPEG(t))
	}))
	defer imageServer.Close()

	wsURL, closeWS := mockThreeDPrinterCameraURLServer(t, imageServer.URL+"/snapshot.jpg")
	defer closeWS()

	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.DefaultPrinter = "lab"
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers = []config.ElegooCentauriCarbonPrinterConfig{{
		ID:  "lab",
		URL: wsURL,
	}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/3d-printers/lab/camera/snapshot", nil)
	rec := httptest.NewRecorder()
	handleThreeDPrinterCameraSnapshot(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if body["status"] != "ok" || body["web_path"] == "" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if !strings.HasPrefix(body["web_path"].(string), "/files/3d_printer_media/") {
		t.Fatalf("web_path = %q", body["web_path"])
	}
}

func TestHandleThreeDPrinterStreamRejectsMismatchedCameraHost(t *testing.T) {
	wsURL, closeWS := mockThreeDPrinterCameraURLServer(t, "http://203.0.113.10/video")
	defer closeWS()

	cfg := &config.Config{}
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.DefaultPrinter = "lab"
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers = []config.ElegooCentauriCarbonPrinterConfig{{
		ID:  "lab",
		URL: wsURL,
	}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/3d-printers/lab/camera/stream", nil)
	rec := httptest.NewRecorder()
	handleThreeDPrinterCameraStream(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

func TestHandleThreeDPrinterTestSupportsAdHocKlipperPrinter(t *testing.T) {
	moonraker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/info" {
			t.Fatalf("path = %s, want /server/info", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": map[string]interface{}{"klippy_state": "ready"}})
	}))
	defer moonraker.Close()

	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	body := strings.NewReader(`{"operation":"test_connection","protocol":"klipper","printer_id":"voron","url":"` + moonraker.URL + `","timeout_seconds":2}`)
	req := httptest.NewRequest(http.MethodPost, "/api/3d-printers/test", body)
	rec := httptest.NewRecorder()

	handleThreeDPrinterTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"klippy_state":"ready"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleThreeDPrinterTestRejectsMalformedJSON(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/3d-printers/test", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	handleThreeDPrinterTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "invalid json") {
		t.Fatalf("expected invalid JSON error, got: %s", rec.Body.String())
	}
}

func TestHandleThreeDPrinterSnapshotStoresImageFromKlipperSnapshotURL(t *testing.T) {
	moonraker := newKlipperCameraServer(t, "/snapshot.jpg", testJPEG(t))
	defer moonraker.Close()

	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.DefaultPrinter = "voron"
	cfg.ThreeDPrinters.Klipper.Enabled = true
	cfg.ThreeDPrinters.Klipper.Printers = []config.KlipperPrinterConfig{{
		ID:  "voron",
		URL: moonraker.URL,
	}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/3d-printers/voron/camera/snapshot", nil)
	rec := httptest.NewRecorder()
	handleThreeDPrinterCameraSnapshot(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"/files/3d_printer_media/`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleThreeDPrinterStreamAllowsKlipperCameraSameHostDifferentPort(t *testing.T) {
	stream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(testJPEG(t))
	}))
	defer stream.Close()

	moonraker := newKlipperCameraServerWithStream(t, stream.URL+"/stream.jpg", "")
	defer moonraker.Close()

	cfg := &config.Config{}
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.DefaultPrinter = "voron"
	cfg.ThreeDPrinters.Klipper.Enabled = true
	cfg.ThreeDPrinters.Klipper.Printers = []config.KlipperPrinterConfig{{
		ID:  "voron",
		URL: moonraker.URL,
	}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/3d-printers/voron/camera/stream", nil)
	rec := httptest.NewRecorder()
	handleThreeDPrinterCameraStream(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/jpeg") {
		t.Fatalf("content type = %q, want image/jpeg", ct)
	}
}

func TestHandleThreeDPrinterStreamRejectsKlipperCameraDifferentHost(t *testing.T) {
	moonraker := newKlipperCameraServerWithStream(t, "http://203.0.113.11:8080/webcam", "")
	defer moonraker.Close()

	cfg := &config.Config{}
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.DefaultPrinter = "voron"
	cfg.ThreeDPrinters.Klipper.Enabled = true
	cfg.ThreeDPrinters.Klipper.Printers = []config.KlipperPrinterConfig{{
		ID:  "voron",
		URL: moonraker.URL,
	}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/3d-printers/voron/camera/stream", nil)
	rec := httptest.NewRecorder()
	handleThreeDPrinterCameraStream(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
}

func mockThreeDPrinterCameraURLServer(t *testing.T, cameraURL string) (string, func()) {
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
		data := payload["Data"].(map[string]interface{})
		if got := int(data["Cmd"].(float64)); got != 386 {
			t.Fatalf("Cmd = %d, want 386", got)
		}
		requestID := data["RequestID"].(string)
		if err := conn.WriteJSON(map[string]interface{}{
			"Data": map[string]interface{}{
				"Cmd":       386,
				"RequestID": requestID,
				"Data": map[string]interface{}{
					"Url": cameraURL,
				},
			},
			"Topic": "sdcp/response/mainboard",
		}); err != nil {
			t.Fatalf("WriteJSON error = %v", err)
		}
	}))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket"
	return wsURL, server.Close
}

func newKlipperCameraServer(t *testing.T, snapshotPath string, snapshot []byte) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server/webcams/list":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"result": map[string]interface{}{
					"webcams": []map[string]interface{}{
						{"name": "default", "enabled": true, "stream_url": "/webcam", "snapshot_url": snapshotPath},
					},
				},
			})
		case snapshotPath:
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(snapshot)
		default:
			t.Fatalf("unexpected klipper camera path: %s", r.URL.Path)
		}
	}))
	return server
}

func newKlipperCameraServerWithStream(t *testing.T, streamURL string, snapshotURL string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/webcams/list" {
			t.Fatalf("unexpected klipper camera path: %s", r.URL.Path)
		}
		webcam := map[string]interface{}{"name": "default", "enabled": true, "stream_url": streamURL}
		if snapshotURL != "" {
			webcam["snapshot_url"] = snapshotURL
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{"webcams": []map[string]interface{}{webcam}},
		})
	}))
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode error = %v", err)
	}
	if len(buf.Bytes()) == 0 {
		t.Fatal(fmt.Errorf("empty jpeg"))
	}
	_ = context.Background()
	return buf.Bytes()
}
