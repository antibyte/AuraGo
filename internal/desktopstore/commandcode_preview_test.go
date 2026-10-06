package desktopstore

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCommandCodePreviewPreservesGuestAuthWithoutAuraGoCredentials(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the CommandCode preview integration test")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			_ = conn.WriteJSON(r.Header)
			return
		}
		w.Header().Add("Set-Cookie", "guest_session=logged-in; Path=/; HttpOnly")
		_ = json.NewEncoder(w).Encode(r.Header)
	}))
	defer upstream.Close()
	_, files, err := commandCodeBuildContext()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "preview.cjs")
	if err := os.WriteFile(script, files["commandcode-preview.js"], 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_, port, _ := net.SplitHostPort(address)
	_ = listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, script)
	cmd.Env = append(os.Environ(), "COMMANDCODE_PREVIEW_HOST=127.0.0.1", "COMMANDCODE_PREVIEW_PORT="+port,
		"COMMANDCODE_PREVIEW_TARGET="+upstream.URL, "COMMANDCODE_PREVIEW_TARGET_FILE="+filepath.Join(dir, "target"),
		"COMMANDCODE_PREVIEW_CANDIDATE_PORTS=")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); _ = cmd.Wait() }()
	client := &http.Client{Timeout: time.Second}
	base := "http://" + address
	for {
		resp, err := client.Get(base + "/__commandcode_preview_status")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("preview did not start")
		case <-time.After(20 * time.Millisecond):
		}
	}
	for _, auth := range []string{"Basic Z3Vlc3Q6cGFzcw==", "Bearer guest-access-token"} {
		for _, socket := range []bool{false, true} {
			header := http.Header{
				"Cookie":        {"aurago_session=parent; AuRaGo_custom=private; __Host-aurago-preview=grant; guest_session=login; theme=dark"},
				"Authorization": {auth}, "X-Csrf-Token": {"guest-csrf"},
				"X-Internal-Token": {"private"}, "X-Internal-Followup": {"private"},
				"X-Aurago-Agodesk-Dev-Token": {"private"}, "Proxy-Authorization": {"private"},
			}
			var got http.Header
			if socket {
				conn, _, err := websocket.DefaultDialer.Dial("ws://"+address+"/socket", header)
				if err != nil {
					t.Fatal(err)
				}
				_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				err = conn.ReadJSON(&got)
				_ = conn.Close()
				if err != nil {
					t.Fatal(err)
				}
			} else {
				req, _ := http.NewRequest(http.MethodGet, base+"/app", nil)
				req.Header = header
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				err = json.NewDecoder(resp.Body).Decode(&got)
				_ = resp.Body.Close()
				if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Set-Cookie"), "guest_session=logged-in") {
					t.Fatalf("guest response was not preserved: status=%d err=%v", resp.StatusCode, err)
				}
			}
			if got.Get("Cookie") != "guest_session=login; theme=dark" || got.Get("Authorization") != auth || got.Get("X-Csrf-Token") != "guest-csrf" {
				t.Fatalf("guest auth changed (websocket=%v): %v", socket, got)
			}
			for _, key := range []string{"X-Internal-Token", "X-Internal-Followup", "X-Aurago-Agodesk-Dev-Token", "Proxy-Authorization"} {
				if got.Get(key) != "" {
					t.Errorf("reserved header %s forwarded (websocket=%v)", key, socket)
				}
			}
		}
	}
}
