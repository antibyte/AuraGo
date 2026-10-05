//go:build !remote_minimal

package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/remote"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

// End to end against the real hub: a current agent learns why its token is
// refused, including the spec text for tokens without a MAC key. The hub is
// not part of the remote_minimal build, hence the separate file.
func TestConnectShowsHubEnrollmentRefusals(t *testing.T) {
	isolateRemoteHome(t)
	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	hub := remote.NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var auth remote.RemoteMessage
		if err := conn.ReadJSON(&auth); err != nil {
			return
		}
		_ = hub.HandleEnrollment(conn, auth)
	}))
	t.Cleanup(srv.Close)
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	if _, err := remote.CreateEnrollment(db, remote.EnrollmentRecord{TokenHash: remote.DeriveEnrollmentLookupHash("keyless-token"), ExpiresAt: expires}); err != nil {
		t.Fatal(err)
	}
	usedID, err := hub.IssueEnrollmentToken("used-token", "", expires)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.MarkEnrollmentUsed(db, usedID, "other-device"); err != nil {
		t.Fatal(err)
	}
	for token, reason := range map[string]string{
		"keyless-token": "enrollment token predates the upgrade; create a new one",
		"used-token":    "enrollment token already used",
		"unknown-token": "invalid enrollment token",
	} {
		client := newConnectTestClient(t, clientConfig{SupervisorURL: url, EnrollToken: token})
		err := client.connect()
		if want := fmt.Sprintf("enrollment rejected (unverified): %q", reason); err == nil || err.Error() != want {
			t.Fatalf("%s: got %v, want %s", token, err, want)
		}
		if client.cfg.DeviceID != "" || client.cfg.SharedKey != "" || client.cfg.EnrollToken != token {
			t.Fatalf("%s: a refusal must not change config: %+v", token, client.cfg)
		}
		assertNoStoredConfig(t)
	}
}
