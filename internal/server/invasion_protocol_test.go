package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/security"
	"github.com/gorilla/websocket"
)

func TestInvasionChallengeRejectsReplayedAuthAndLegacyPeers(t *testing.T) {
	db := setupInvasionTestDB(t)
	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{Name: "fixture", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	nestID, err := invasion.CreateNest(db, invasion.NestRecord{Name: "fixture", Active: true, EggID: eggID})
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64)
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("egg_shared_"+nestID, key); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := bridge.NewEggHub(logger)
	s := &Server{InvasionDB: db, Vault: vault, EggHub: hub, Logger: logger}
	connected := make(chan struct{}, 4)
	hub.OnConnect = func(string, string) { connected <- struct{}{} }
	httpServer := httptest.NewServer(handleInvasionWebSocket(s))
	defer httpServer.Close()
	dial := func() (*websocket.Conn, bridge.Message) {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var challenge bridge.Message
		if err := conn.ReadJSON(&challenge); err != nil {
			t.Fatal(err)
		}
		var notice bridge.ErrorPayload
		if json.Unmarshal(challenge.Payload, &notice) != nil || challenge.Type != bridge.MsgError || !strings.Contains(notice.Message, "invasion_protocol_upgrade_required") {
			t.Fatal("legacy reader would not see upgrade notice")
		}
		return conn, challenge
	}
	first, challenge := dial()
	session, err := bridge.NewSession(challenge.Session, eggID, nestID, "egg")
	if err != nil {
		t.Fatal(err)
	}
	auth, _ := bridge.NewMessage(bridge.MsgAuth, eggID, nestID, key, bridge.AuthPayload{Version: "fixture"})
	if err := session.Prepare(auth, key); err != nil {
		t.Fatal(err)
	}
	if err := first.WriteJSON(auth); err != nil {
		t.Fatal(err)
	}
	var ack bridge.Message
	if err := first.ReadJSON(&ack); err != nil {
		t.Fatal(err)
	}
	if err := session.Accept(ack, key, ""); err != nil {
		t.Fatal(err)
	}
	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		t.Fatal("not registered")
	}
	original := hub.GetConnection(nestID)
	replay, otherChallenge := dial()
	if challenge.Session == otherChallenge.Session {
		t.Fatal("challenge reused")
	}
	if err := replay.WriteJSON(auth); err != nil {
		t.Fatal(err)
	}
	if err := replay.ReadJSON(&ack); err == nil {
		t.Fatal("replayed auth accepted")
	}
	if hub.GetConnection(nestID) != original {
		t.Fatal("auth replay replaced valid connection")
	}
	legacy, _ := dial()
	old := *auth
	old.Protocol = 0
	_ = legacy.WriteJSON(&old)
	err = legacy.ReadJSON(&ack)
	if err == nil || !strings.Contains(err.Error(), "invasion_protocol_upgrade_required") {
		t.Fatalf("missing explicit upgrade error: %v", err)
	}
	// A current EggClient can complete the same real server handshake.
	client := bridge.NewEggClient("ws"+strings.TrimPrefix(httpServer.URL, "http"), eggID, nestID, key, "fixture", logger)
	done := make(chan struct{})
	go func() { defer close(done); client.Start() }()
	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		client.Stop()
		t.Fatal("current egg handshake failed")
	}
	client.Stop()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("client did not stop")
	}
}
