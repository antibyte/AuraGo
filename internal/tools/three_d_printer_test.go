package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
