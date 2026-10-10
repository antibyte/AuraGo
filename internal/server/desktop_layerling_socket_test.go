package server

import (
	"aurago/internal/layerling"
	"context"
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLayerlingSocketOwnershipAndRevocation(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.Layerling.Enabled = true
	s.Cfg.VirtualDesktop.Layerling.AgentAccess = "write"
	s.Cfg.VirtualDesktop.AllowAgentControl = true
	s.Cfg.Tools.VirtualDesktop.Enabled = true
	mux := http.NewServeMux()
	h := registerLayerlingRoutes(mux, s)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	target := "ws" + strings.TrimPrefix(srv.URL, "http") + layerlingBase + "connect?window_id=one"
	headers := http.Header{"Origin": []string{"https://foreign.invalid"}}
	if c, _, err := websocket.DefaultDialer.Dial(target, headers); err == nil {
		c.Close()
		t.Fatal("foreign origin accepted")
	}
	headers.Set("Origin", srv.URL)
	conn, _, err := websocket.DefaultDialer.Dial(target, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var hello map[string]string
	if err = conn.ReadJSON(&hello); err != nil {
		t.Fatal(err)
	}
	id := hello["editor_id"]
	owner := layerling.WithOwner(context.Background(), h.broker, "local-desktop", func() bool { return true })
	foreign := layerling.WithOwner(context.Background(), h.broker, "other-owner", func() bool { return true })
	if _, err := layerling.Execute(foreign, "get_scene", id, json.RawMessage(`{}`)); err == nil {
		t.Fatal("foreign session accepted")
	}
	done := make(chan error, 1)
	go func() {
		_, err := layerling.Execute(owner, "create_shape", id, json.RawMessage(`{"kind":"box"}`))
		done <- err
	}()
	var command layerling.Command
	if err = conn.ReadJSON(&command); err != nil {
		t.Fatal(err)
	}
	s.CfgMu.Lock()
	s.Cfg.VirtualDesktop.Layerling.AgentAccess = "off"
	s.CfgMu.Unlock()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("revoked command succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("revocation did not stop command")
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("uncertain connection stayed open")
	}
	r := httptest.NewRequest("PUT", layerlingBase+"file?path=Documents/Layerling/revoked.lyl", strings.NewReader("bytes"))
	r.Header.Set("X-Layerling-Editor", id)
	r.Header.Set("X-Layerling-Command", command.ID)
	r.Header.Set("If-None-Match", "*")
	w := httptest.NewRecorder()
	h.serve(w, r)
	if w.Code != 403 {
		t.Fatalf("revoked file write: %d", w.Code)
	}
}

func TestLayerlingTokenScopesAndLogout(t *testing.T) {
	s, _, desktopToken, _ := newBearerSchemeTestServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.Layerling.Enabled = true
	s.Cfg.VirtualDesktop.Layerling.AgentAccess = "write"
	s.Cfg.VirtualDesktop.AllowAgentControl = true
	s.Cfg.Tools.VirtualDesktop.Enabled = true
	h := registerLayerlingRoutes(http.NewServeMux(), s)
	readToken, _, err := s.TokenManager.Create("CAD reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", layerlingBase+"connect", nil)
	r.Header.Set("Authorization", "Bearer "+readToken)
	if !h.allowed(r, false) || h.allowed(r, true) {
		t.Fatal("read token gained CAD mutation rights")
	}
	r.Header.Set("Authorization", "Bearer "+desktopToken)
	if !h.allowed(r, true) {
		t.Fatal("admin token blocked")
	}
	r.Header.Del("Authorization")
	if h.allowed(r, false) {
		t.Fatal("logged-out request accepted")
	}
}

func TestLayerlingLargeResultArtifact(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.VirtualDesktop.Layerling.Enabled = true
	s.Cfg.VirtualDesktop.Layerling.AgentAccess = "read"
	s.Cfg.VirtualDesktop.AllowAgentControl = true
	s.Cfg.Tools.VirtualDesktop.Enabled = true
	mux := http.NewServeMux()
	h := registerLayerlingRoutes(mux, s)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+layerlingBase+"connect?window_id=large", http.Header{"Origin": []string{srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var hello map[string]string
	if err = conn.ReadJSON(&hello); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"mesh": strings.Repeat("geometry", 10000)})
	go func() {
		var c layerling.Command
		if conn.ReadJSON(&c) == nil {
			conn.WriteJSON(layerling.Result{ID: c.ID, OK: true, Data: payload})
		}
	}()
	result, err := layerling.Execute(layerling.WithOwner(context.Background(), h.broker, "local-desktop", func() bool { return true }), "get_scene", hello["editor_id"], json.RawMessage(`{}`))
	if err != nil || len(result) > layerling.MaxResult || strings.Contains(string(result), "geometry") {
		t.Fatal("unbounded model result", err)
	}
	var ref struct {
		Artifact string `json:"artifact"`
	}
	json.Unmarshal(result, &ref)
	w := httptest.NewRecorder()
	h.serve(w, httptest.NewRequest("GET", ref.Artifact, nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/json" || w.Body.Len() != len(payload) {
		t.Fatal("artifact unavailable", w.Code)
	}
	h.mu.Lock()
	key := strings.TrimPrefix(ref.Artifact, layerlingBase+"artifact/")
	p := h.previews[key]
	p.owner = "foreign"
	h.previews[key] = p
	h.mu.Unlock()
	w = httptest.NewRecorder()
	h.serve(w, httptest.NewRequest("GET", ref.Artifact, nil))
	if w.Code != 404 {
		t.Fatal("foreign artifact leaked")
	}
	h.mu.Lock()
	p.owner = "local-desktop"
	p.expires = time.Now().Add(-time.Minute)
	h.previews[key] = p
	h.mu.Unlock()
	w = httptest.NewRecorder()
	h.serve(w, httptest.NewRequest("GET", ref.Artifact, nil))
	if w.Code != 404 {
		t.Fatal("expired artifact served")
	}
}
