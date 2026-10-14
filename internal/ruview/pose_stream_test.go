package ruview

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestPoseStreamProxyUpgradeAndQueryBoundary(t *testing.T) {
	var calls atomic.Int32
	m, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/api/v1/stream/pose" || r.URL.RawQuery != "" {
			t.Error("pose stream received an unexpected path or query")
		}
		if r.Header.Get("Authorization") != "Bearer test-ruview-private-token" ||
			r.Header.Get("Cookie") != "" || r.Header.Get("X-Api-Key") != "" {
			t.Error("pose stream did not isolate browser credentials from Vault authentication")
		}
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade upstream pose stream: %v", err)
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteJSON(map[string]string{"type": "pose_data"}); err != nil {
			t.Errorf("write upstream pose frame: %v", err)
			return
		}
		_, _, _ = conn.ReadMessage()
	})
	for _, query := range []string{
		"zone_ids=zone_1", "min_confidence=0.3", "max_fps=30",
		"token=fixture", "ticket=fixture", "url=http://example.invalid",
	} {
		w := httptest.NewRecorder()
		m.Proxy(w, httptest.NewRequest("GET", "/?"+query, nil), "/api/v1/stream/pose")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("pose query %q: status %d, want 400", query, w.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("rejected pose query reached upstream")
	}
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.Proxy(w, r, "/api/v1/stream/pose")
	}))
	defer proxy.Close()
	headers := http.Header{
		"Cookie":        {"session=fixture"},
		"Authorization": {"Bearer browser-fixture"},
		"X-Api-Key":     {"browser-fixture"},
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(proxy.URL, "http")+"/api/v1/stream/pose", headers)
	if err != nil {
		t.Fatalf("connect pose stream through proxy: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var frame map[string]string
	if err := conn.ReadJSON(&frame); err != nil || frame["type"] != "pose_data" {
		t.Fatalf("read proxied pose frame: %v, %v", frame, err)
	}
}
