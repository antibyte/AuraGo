package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"github.com/gorilla/websocket"
)

func TestThreeDPrinterWidgetReadOnlyStatus(t *testing.T) {
	var calls atomic.Int32
	device := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		var request struct {
			Data struct {
				Cmd       int
				RequestID string
			}
		}
		if err := conn.ReadJSON(&request); err != nil {
			t.Error(err)
			return
		}
		calls.Add(1)
		if request.Data.Cmd != 0 {
			t.Errorf("widget sent mutating/non-status command %d", request.Data.Cmd)
			return
		}
		_ = conn.WriteJSON(map[string]interface{}{"Data": map[string]interface{}{"RequestID": request.Data.RequestID, "Data": map[string]int{"Ack": 0}}})
		_ = conn.WriteJSON(map[string]interface{}{"Status": map[string]interface{}{"PrintInfo": map[string]int{"Progress": 42, "CurrentTicks": 60, "TotalTicks": 180}}})
	}))
	defer device.Close()
	cfg := &config.Config{}
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.ReadOnly = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers = []config.ElegooCentauriCarbonPrinterConfig{{ID: "lab", Name: "Lab", URL: "ws" + strings.TrimPrefix(device.URL, "http"), TimeoutSeconds: 2}}
	s := &Server{Cfg: cfg}
	handler := handleThreeDPrinterWidgetStatus(s)
	call := func(method, query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/api/3d-printers/status"+query, nil))
		return rec
	}
	list := call("GET", "")
	if list.Code != 200 || calls.Load() != 0 || strings.Contains(list.Body.String(), device.URL) || !strings.Contains(list.Body.String(), `"id":"lab"`) {
		t.Fatalf("unsafe list: %s", list.Body.String())
	}
	result := call("GET", "?printer_id=lab&operation=cancel_print")
	if result.Code != 200 || calls.Load() != 1 || !strings.Contains(result.Body.String(), `"Progress":42`) {
		t.Fatalf("status: %s", result.Body.String())
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal(result.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if result.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("status must not be cached")
	}
	if got := call("GET", "?printer_id=missing").Code; got != 404 {
		t.Fatalf("unknown ID: %d", got)
	}
	if got := call("POST", "?printer_id=lab").Code; got != 405 {
		t.Fatalf("POST: %d", got)
	}
	cfg.ThreeDPrinters.Enabled = false
	if got := call("GET", "?printer_id=lab").Code; got != 403 {
		t.Fatalf("disabled: %d", got)
	}
	if calls.Load() != 1 {
		t.Fatal("rejected requests reached device")
	}
}

func TestThreeDPrinterWidgetRequiresSession(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = "0123456789abcdef0123456789abcdef"
	s.Cfg.Auth.PasswordHash = "configured"
	token, err := issueDesktopEmbedToken(s.Cfg.Auth.SessionSecret, "Widgets/printer-camera/index.html", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	authMiddleware(s, handleThreeDPrinterWidgetStatus(s)).ServeHTTP(rec, httptest.NewRequest("GET", "/api/3d-printers/status?desktop_token="+token, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("widget embed token must not grant printer status access: %d", rec.Code)
	}
}
