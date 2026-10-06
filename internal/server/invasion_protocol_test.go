package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
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

// invasionKeyFixture serves the real egg WebSocket handshake against a temp
// vault holding the given egg_shared_<nest>[suffix] entries.
type invasionKeyFixture struct {
	s         *Server
	vault     *security.Vault
	hub       *bridge.EggHub
	eggID     string
	nestID    string
	wsURL     string
	connected chan struct{}
}

func newInvasionKeyFixture(t *testing.T, secrets map[string]string) invasionKeyFixture {
	t.Helper()
	db := setupInvasionTestDB(t)
	eggID, err := invasion.CreateEgg(db, invasion.EggRecord{Name: "fixture", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	nestID, err := invasion.CreateNest(db, invasion.NestRecord{Name: "fixture", Active: true, EggID: eggID})
	if err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	for suffix, value := range secrets {
		if err := vault.WriteSecret("egg_shared_"+nestID+suffix, value); err != nil {
			t.Fatal(err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := bridge.NewEggHub(logger)
	s := &Server{InvasionDB: db, Vault: vault, EggHub: hub, Logger: logger}
	connected := make(chan struct{}, 8)
	hub.OnConnect = func(string, string) { connected <- struct{}{} }
	httpServer := httptest.NewServer(handleInvasionWebSocket(s))
	t.Cleanup(httpServer.Close)
	return invasionKeyFixture{
		s: s, vault: vault, hub: hub, eggID: eggID, nestID: nestID,
		wsURL:     "ws" + strings.TrimPrefix(httpServer.URL, "http"),
		connected: connected,
	}
}

func (f invasionKeyFixture) secret(t *testing.T, suffix string) string {
	t.Helper()
	value, err := f.vault.ReadSecret("egg_shared_" + f.nestID + suffix)
	if errors.Is(err, security.ErrSecretNotFound) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// handshake runs one raw egg authentication and reports whether the master
// acknowledged it under key.
func (f invasionKeyFixture) handshake(t *testing.T, key string) error {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(f.wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var challenge bridge.Message
	if err := conn.ReadJSON(&challenge); err != nil {
		t.Fatal(err)
	}
	session, err := bridge.NewSession(challenge.Session, f.eggID, f.nestID, "egg")
	if err != nil {
		t.Fatal(err)
	}
	auth, err := bridge.NewMessage(bridge.MsgAuth, f.eggID, f.nestID, key, bridge.AuthPayload{Version: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Prepare(auth, key); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(auth); err != nil {
		t.Fatal(err)
	}
	var ack bridge.Message
	if err := conn.ReadJSON(&ack); err != nil {
		return err
	}
	return session.Accept(ack, key, "")
}

func (f invasionKeyFixture) waitConnected(t *testing.T) {
	t.Helper()
	select {
	case <-f.connected:
	case <-time.After(2 * time.Second):
		t.Fatal("egg was not registered")
	}
}

// startEgg runs a real EggClient with the given OnRekey until the test ends.
func (f invasionKeyFixture) startEgg(t *testing.T, key string, onRekey func(string, int) error) *bridge.EggClient {
	t.Helper()
	client := bridge.NewEggClient(f.wsURL, f.eggID, f.nestID, key, "fixture", slog.New(slog.NewTextHandler(io.Discard, nil)))
	client.OnRekey = onRekey
	done := make(chan struct{})
	go func() { defer close(done); client.Start() }()
	t.Cleanup(func() {
		client.Stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("egg client did not stop")
		}
	})
	f.waitConnected(t)
	return client
}

func (f invasionKeyFixture) rotate(t *testing.T, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/invasion/nests/"+f.nestID+"/rotate-key", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleInvasionNestRotateKey(f.s).ServeHTTP(rec, req)
	return rec
}

func TestInvasionHandshakeAcceptsAndPromotesNextKeyCandidate(t *testing.T) {
	oldKey, newKey := strings.Repeat("1", 64), strings.Repeat("2", 64)
	f := newInvasionKeyFixture(t, map[string]string{"": oldKey, "_next": newKey})
	if err := f.handshake(t, newKey); err != nil {
		t.Fatalf("egg holding the staged key must authenticate: %v", err)
	}
	f.waitConnected(t)
	if got := f.secret(t, ""); got != newKey {
		t.Fatal("the _next candidate must be promoted to the current key")
	}
	if f.secret(t, "_next") != "" || f.secret(t, "_prev") != "" {
		t.Fatal("promotion must remove the other candidates")
	}
	if err := f.handshake(t, oldKey); err == nil {
		t.Fatal("the superseded key must no longer authenticate")
	}
}

func rotatedAt(ago time.Duration) string {
	return time.Now().Add(-ago).UTC().Format(time.RFC3339)
}

func TestInvasionHandshakeAcceptsPreviousKeyOnceAndBoundsIt(t *testing.T) {
	oldKey, newKey := strings.Repeat("1", 64), strings.Repeat("2", 64)

	t.Run("previous key reconnects once within the window and is promoted", func(t *testing.T) {
		f := newInvasionKeyFixture(t, map[string]string{"": newKey, "_prev": oldKey, "_prev_at": rotatedAt(10 * time.Minute)})
		if err := f.handshake(t, oldKey); err != nil {
			t.Fatalf("egg that missed the rotation must reconnect once: %v", err)
		}
		f.waitConnected(t)
		if f.secret(t, "") != oldKey || f.secret(t, "_prev") != "" || f.secret(t, "_prev_at") != "" {
			t.Fatal("the matching previous key must become current and consume _prev")
		}
		if err := f.handshake(t, newKey); err == nil {
			t.Fatal("the slot is single-use: the replaced key must not authenticate afterwards")
		}
	})

	t.Run("current-key handshake retires the previous key", func(t *testing.T) {
		f := newInvasionKeyFixture(t, map[string]string{"": newKey, "_prev": oldKey, "_prev_at": rotatedAt(time.Minute)})
		if err := f.handshake(t, newKey); err != nil {
			t.Fatal(err)
		}
		f.waitConnected(t)
		if f.secret(t, "") != newKey || f.secret(t, "_prev") != "" || f.secret(t, "_prev_at") != "" {
			t.Fatal("a handshake under the current key must delete _prev")
		}
		if err := f.handshake(t, oldKey); err == nil {
			t.Fatal("the previous key must not authenticate after the egg proved the current key")
		}
	})

	for name, prevAt := range map[string]string{
		"expired after the grace window": rotatedAt(eggPrevKeyGrace + time.Minute),
		"undated":                        "",
		"unparseable":                    "yesterday",
		"dated in the future":            rotatedAt(-2 * time.Hour),
	} {
		t.Run(name, func(t *testing.T) {
			secrets := map[string]string{"": newKey, "_prev": oldKey}
			if prevAt != "" {
				secrets["_prev_at"] = prevAt
			}
			f := newInvasionKeyFixture(t, secrets)
			if err := f.handshake(t, oldKey); err == nil {
				t.Fatal("a previous key outside the window must not authenticate")
			}
			if f.secret(t, "") != newKey || f.secret(t, "_prev") != "" || f.secret(t, "_prev_at") != "" {
				t.Fatal("an expired previous key must be deleted and the current key kept")
			}
			if err := f.handshake(t, newKey); err != nil {
				t.Fatalf("the current key keeps working: %v", err)
			}
		})
	}
}

// Re-hatching is the operator's revocation path: it must drop every rotation
// candidate, not only replace the current key.
func TestInvasionRehatchRevokesRotationCandidates(t *testing.T) {
	oldKey, prevKey := strings.Repeat("1", 64), strings.Repeat("4", 64)
	f := newInvasionKeyFixture(t, map[string]string{"": oldKey, "_prev": prevKey, "_prev_at": rotatedAt(time.Minute)})
	f.startEgg(t, oldKey, func(string, int) error { return errors.New("read-only vault") })
	if rec := f.rotate(t, context.Background()); rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body %s", rec.Code, rec.Body.String())
	}
	stale := f.secret(t, "_next")
	if stale == "" {
		t.Fatal("the rejected rotation should have left a staged key")
	}
	fresh := strings.Repeat("3", 64)
	if err := f.s.storeEggSharedKey(f.nestID, fresh); err != nil {
		t.Fatal(err)
	}
	if f.secret(t, "") != fresh || f.secret(t, "_next") != "" || f.secret(t, "_prev") != "" || f.secret(t, "_prev_at") != "" {
		t.Fatal("re-hatch must store only the fresh key")
	}
	for _, revoked := range []string{stale, prevKey, oldKey} {
		if err := f.handshake(t, revoked); err == nil {
			t.Fatal("a key revoked by re-hatch must not authenticate")
		}
	}
	if f.secret(t, "") != fresh {
		t.Fatal("failed handshakes must not displace the hatched key")
	}
	if err := f.handshake(t, fresh); err != nil {
		t.Fatalf("the freshly hatched key must authenticate: %v", err)
	}
}

func TestInvasionRotateKeyCommitsAfterEggPersistedAndAcked(t *testing.T) {
	oldKey := strings.Repeat("1", 64)
	f := newInvasionKeyFixture(t, map[string]string{"": oldKey})
	persisted := make(chan string, 1)
	client := f.startEgg(t, oldKey, func(key string, _ int) error {
		persisted <- key
		return nil
	})
	rec := f.rotate(t, context.Background())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var newKey string
	select {
	case newKey = <-persisted:
	default:
		t.Fatal("the egg must persist before the master commits")
	}
	if newKey == oldKey || f.secret(t, "") != newKey || client.SharedKeySnapshot() != newKey {
		t.Fatal("master vault and egg must both hold the rotated key")
	}
	if f.secret(t, "_prev") != oldKey || f.secret(t, "_next") != "" {
		t.Fatal("commit must keep the old key as _prev and drop the staged _next")
	}
	at, err := time.Parse(time.RFC3339, f.secret(t, "_prev_at"))
	if err != nil || time.Since(at) > time.Minute || time.Since(at) < -time.Minute {
		t.Fatalf("commit must date _prev in the same write: %q, %v", f.secret(t, "_prev_at"), err)
	}
	if got := f.hub.GetConnection(f.nestID).SharedKey; got != newKey {
		t.Fatal("the live connection must use the committed key")
	}
}

func TestInvasionRotateKeyKeepsPreviousKeyWhenEggRejects(t *testing.T) {
	oldKey := strings.Repeat("1", 64)
	f := newInvasionKeyFixture(t, map[string]string{"": oldKey})
	client := f.startEgg(t, oldKey, func(string, int) error { return errors.New("read-only vault") })
	rec := f.rotate(t, context.Background())
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "reconciled at its next connection") {
		t.Fatalf("status = %d, want 502 with the reconcile notice; body %s", rec.Code, rec.Body.String())
	}
	if f.secret(t, "") != oldKey || client.SharedKeySnapshot() != oldKey || f.hub.GetConnection(f.nestID).SharedKey != oldKey {
		t.Fatal("a rejected rotation must leave the previous key active on both sides")
	}
	if f.secret(t, "_next") == "" {
		t.Fatal("the staged candidate stays until a handshake or the next rotation replaces it")
	}
}

func TestInvasionRotateKeyRecoversWhenAckIsLost(t *testing.T) {
	oldKey := strings.Repeat("1", 64)
	f := newInvasionKeyFixture(t, map[string]string{"": oldKey})
	release := make(chan error, 1)
	persisted := make(chan string, 1)
	client := f.startEgg(t, oldKey, func(key string, _ int) error {
		if err := <-release; err != nil {
			return err
		}
		persisted <- key
		return nil
	})
	defer func() {
		select {
		case release <- errors.New("test ended"):
		default:
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	rec := f.rotate(t, ctx)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body %s", rec.Code, rec.Body.String())
	}
	staged := f.secret(t, "_next")
	if staged == "" || f.secret(t, "") != oldKey {
		t.Fatal("an unconfirmed rotation keeps the previous key current and the candidate staged")
	}
	// The egg may still adopt the staged key, so a second rotation must not
	// replace the candidate.
	if rec := f.rotate(t, context.Background()); rec.Code != http.StatusConflict {
		t.Fatalf("second rotation status = %d, want 409; body %s", rec.Code, rec.Body.String())
	}
	if f.secret(t, "_next") != staged {
		t.Fatal("a refused rotation must not touch the staged candidate")
	}

	// The egg now persists and switches; its new-key ack fails verification
	// under the rolled-back key, it reconnects, and the handshake promotes _next.
	release <- nil
	if got := <-persisted; got != staged {
		t.Fatal("egg persisted a different key than the master staged")
	}
	f.waitConnected(t)
	deadline := time.Now().Add(2 * time.Second)
	for f.secret(t, "") != staged && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if f.secret(t, "") != staged || f.secret(t, "_next") != "" || client.SharedKeySnapshot() != staged {
		t.Fatal("the reconnect handshake must promote the key the egg persisted")
	}
}
