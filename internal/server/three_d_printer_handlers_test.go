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
