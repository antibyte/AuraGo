package remote

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
	"aurago/internal/testutil"

	"github.com/gorilla/websocket"
)

func enrollmentTestSocket(t *testing.T, hub *RemoteHub) func(*RemoteMessage) AuthResponsePayload {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		var message RemoteMessage
		if err := conn.ReadJSON(&message); err != nil {
			t.Errorf("read enrollment: %v", err)
			return
		}
		if err := hub.HandleEnrollment(conn, message); err != nil {
			t.Errorf("handle enrollment: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return func(message *RemoteMessage) AuthResponsePayload {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if err := conn.WriteJSON(message); err != nil {
			t.Fatal(err)
		}
		var response RemoteMessage
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatal(err)
		}
		var payload AuthResponsePayload
		if err := json.Unmarshal(response.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		return payload
	}
}

func TestManualApprovalIssuesOneTimeToken(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vault, err := security.NewVault(strings.Repeat("c", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	hub := NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	hub.AutoApprove = true
	exchange := enrollmentTestSocket(t, hub)
	unauthenticated, _ := NewMessage(MsgAuth, "", "", 1, AuthPayload{Hostname: "pending"})
	pending := exchange(unauthenticated)
	if pending.Status != "pending" {
		t.Fatalf("address authenticated a device: %+v", pending)
	}
	token, _, err := hub.ApproveDevice(pending.DeviceID)
	if err != nil || token == "" {
		t.Fatalf("approval: %v", err)
	}
	if _, _, err := hub.ApproveDevice(pending.DeviceID); err == nil {
		t.Fatal("approval repeated")
	}
	key := DeriveEnrollmentAuthKey(token)
	for attempt := 0; attempt < 2; attempt++ {
		message, _ := NewMessage(MsgAuth, "", key, 1, AuthPayload{TokenHash: key, Hostname: "pending"})
		response := exchange(message)
		want := "enrolled"
		if attempt == 1 {
			want = "rejected"
		}
		if response.Status != want {
			t.Fatalf("attempt %d: %s", attempt, response.Status)
		}
	}
}

func TestEnrollmentTokenSingleUseAndReconnectReplay(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	hub := NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	if _, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: DeriveEnrollmentAuthKey(token), DeviceName: "test", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	message, err := NewMessage(MsgAuth, "", DeriveEnrollmentAuthKey(token), 1, AuthPayload{TokenHash: DeriveEnrollmentAuthKey(token), Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	first := exchange(message)
	if first.Status != "enrolled" || first.DeviceID == "" {
		t.Fatalf("first enrollment = %+v", first)
	}
	second, err := NewMessage(MsgAuth, "", DeriveEnrollmentAuthKey(token), 2, AuthPayload{TokenHash: DeriveEnrollmentAuthKey(token), Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := exchange(second); got.Status != "rejected" {
		t.Fatalf("consumed token response = %+v", got)
	}
	devices, err := ListDevices(db)
	if err != nil || len(devices) != 1 {
		t.Fatalf("devices after consumed token = %d, %v", len(devices), err)
	}

	reconnect, err := NewMessage(MsgAuth, first.DeviceID, first.SharedKey, 3, AuthPayload{DeviceID: first.DeviceID, Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := exchange(reconnect); got.Status != "authenticated" {
		t.Fatalf("fresh reconnect = %+v", got)
	}
	if got := exchange(reconnect); got.Status != "rejected" {
		t.Fatalf("replayed reconnect = %+v", got)
	}
}

// shiftSequenceIntoNonce moves the last sequence digit into the nonce. The HMAC
// still verifies because hmacData joins the two fields without a delimiter.
func shiftSequenceIntoNonce(t *testing.T, msg *RemoteMessage, key string) *RemoteMessage {
	t.Helper()
	if msg.Sequence != 12 {
		t.Fatalf("shift helper expects sequence 12, got %d", msg.Sequence)
	}
	shifted := *msg
	shifted.Sequence = 1
	shifted.Nonce = "2" + msg.Nonce
	if ok, err := VerifyMessage(shifted, key); err != nil || !ok {
		t.Fatalf("shifted frame is expected to keep a valid HMAC (delimiter-free encoding): ok=%v err=%v", ok, err)
	}
	return &shifted
}

func TestHandleEnrollmentRejectsSequenceShiftedNonce(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	hub := NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	exchange := enrollmentTestSocket(t, hub)
	bootstrapKey := DeriveEnrollmentAuthKey("fresh-admin-token")
	if _, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: bootstrapKey, DeviceName: "test", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}

	enroll, err := NewMessage(MsgAuth, "", bootstrapKey, 12, AuthPayload{TokenHash: bootstrapKey, Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := exchange(shiftSequenceIntoNonce(t, enroll, bootstrapKey)); got.Status != "rejected" {
		t.Fatalf("shifted enrollment = %+v", got)
	}
	enrolled := exchange(enroll)
	if enrolled.Status != "enrolled" || enrolled.DeviceID == "" {
		t.Fatalf("original enrollment after rejected shift = %+v", enrolled)
	}

	reconnect, err := NewMessage(MsgAuth, enrolled.DeviceID, enrolled.SharedKey, 12, AuthPayload{DeviceID: enrolled.DeviceID, Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := exchange(shiftSequenceIntoNonce(t, reconnect, enrolled.SharedKey)); got.Status != "rejected" {
		t.Fatalf("shifted reconnect = %+v", got)
	}
	if got := exchange(reconnect); got.Status != "authenticated" {
		t.Fatalf("original reconnect after rejected shift = %+v", got)
	}
}

func TestEnrollmentVaultFailureLeavesTokenUnused(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "missing", "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	hub := NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil)))
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	id, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: DeriveEnrollmentAuthKey(token), DeviceName: "test", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	message, err := NewMessage(MsgAuth, "", DeriveEnrollmentAuthKey(token), 1, AuthPayload{TokenHash: DeriveEnrollmentAuthKey(token), Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := exchange(message); got.Status != "rejected" {
		t.Fatalf("vault failure response = %+v", got)
	}
	enrollment, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentAuthKey(token))
	if err != nil || enrollment.ID != id || enrollment.Used {
		t.Fatalf("enrollment consumed after vault failure: %+v, %v", enrollment, err)
	}
	devices, err := ListDevices(db)
	if err != nil || len(devices) != 0 {
		t.Fatalf("orphan devices after vault failure = %d, %v", len(devices), err)
	}
}
