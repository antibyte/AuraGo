package remote

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// enrollmentExchange serves HandleEnrollment on a test socket and returns a
// function that sends one auth frame and returns the supervisor's reply.
func enrollmentExchange(t *testing.T, hub *RemoteHub) func(*RemoteMessage) (RemoteMessage, AuthResponsePayload) {
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
	return func(message *RemoteMessage) (RemoteMessage, AuthResponsePayload) {
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
		return response, payload
	}
}

func enrollmentTestSocket(t *testing.T, hub *RemoteHub) func(*RemoteMessage) AuthResponsePayload {
	t.Helper()
	exchange := enrollmentExchange(t, hub)
	return func(message *RemoteMessage) AuthResponsePayload {
		t.Helper()
		_, payload := exchange(message)
		return payload
	}
}

// newEnrollmentTestHub returns a hub backed by a fresh database and a working
// vault.
func newEnrollmentTestHub(t *testing.T) (*RemoteHub, *sql.DB, *security.Vault) {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil))), db, vault
}

// newBrokenVaultTestHub returns a hub whose vault cannot be read or written.
func newBrokenVaultTestHub(t *testing.T) (*RemoteHub, *sql.DB) {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "missing", "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil))), db
}

// issueTestEnrollment issues token through the hub, as the server does.
func issueTestEnrollment(t *testing.T, hub *RemoteHub, token string) string {
	t.Helper()
	id, err := hub.IssueEnrollmentToken(token, "test", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("IssueEnrollmentToken: %v", err)
	}
	return id
}

// enrollmentFrame is the auth frame a current agent sends for token: the
// lookup hash in the clear, signed with the separately derived MAC key.
func enrollmentFrame(t *testing.T, token string, seq uint64) *RemoteMessage {
	t.Helper()
	message, err := NewMessage(MsgAuth, "", DeriveEnrollmentAuthKey(token), seq, AuthPayload{
		KDF:       EnrollmentKDFVersion,
		TokenHash: DeriveEnrollmentLookupHash(token),
		Hostname:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func plainSHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func assertEnrollmentUnused(t *testing.T, db *sql.DB, token, wantID string) {
	t.Helper()
	enrollment, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentLookupHash(token))
	if err != nil || enrollment.ID != wantID || enrollment.Used {
		t.Fatalf("enrollment must stay unused: %+v, %v", enrollment, err)
	}
}

func assertNoDevices(t *testing.T, db *sql.DB) {
	t.Helper()
	devices, err := ListDevices(db)
	if err != nil || len(devices) != 0 {
		t.Fatalf("devices = %d, %v; want none", len(devices), err)
	}
}

func assertNoEnrollmentRows(t *testing.T, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM remote_enrollments`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("remote_enrollments has %d rows; want none", count)
	}
}

// assertStoredAsLookupHashOnly checks the database keeps only the lookup hash
// for token and the vault holds its MAC key.
func assertStoredAsLookupHashOnly(t *testing.T, db *sql.DB, vault *security.Vault, token string) string {
	t.Helper()
	enrollment, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentLookupHash(token))
	if err != nil {
		t.Fatalf("enrollment not stored under its lookup hash: %v", err)
	}
	for name, value := range map[string]string{
		"raw token":  token,
		"plain hash": plainSHA256Hex(token),
		"MAC key":    DeriveEnrollmentAuthKey(token),
	} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM remote_enrollments WHERE token_hash = ?`, value).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("remote_enrollments must not hold the %s", name)
		}
	}
	key, err := vault.ReadSecret(enrollmentAuthKeyName(enrollment.ID))
	if err != nil || key != DeriveEnrollmentAuthKey(token) {
		t.Fatalf("vault MAC key for enrollment %s: %v", enrollment.ID, err)
	}
	return enrollment.ID
}

func TestManualApprovalIssuesOneTimeToken(t *testing.T) {
	hub, db, vault := newEnrollmentTestHub(t)
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
	enrollmentID := assertStoredAsLookupHashOnly(t, db, vault, token)
	for attempt := 0; attempt < 2; attempt++ {
		response := exchange(enrollmentFrame(t, token, uint64(attempt+1)))
		want := "enrolled"
		if attempt == 1 {
			want = "rejected"
		}
		if response.Status != want {
			t.Fatalf("attempt %d: %s", attempt, response.Status)
		}
	}
	if _, err := vault.ReadSecret(enrollmentAuthKeyName(enrollmentID)); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("consumed token's MAC key must be deleted, got %v", err)
	}
}

func TestApproveDeviceVaultFailureCreatesNoToken(t *testing.T) {
	hub, db := newBrokenVaultTestHub(t)
	pendingID, err := CreateDevice(db, DeviceRecord{Name: "pending", Hostname: "pending", Status: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	if token, _, err := hub.ApproveDevice(pendingID); err == nil || token != "" {
		t.Fatalf("approval must fail when the MAC key cannot be stored: token=%q err=%v", token, err)
	}
	assertNoEnrollmentRows(t, db)
	device, err := GetDevice(db, pendingID)
	if err != nil || device.Status != "pending" {
		t.Fatalf("pending observation must survive a failed approval: %+v, %v", device, err)
	}
}

func TestIssueEnrollmentTokenVaultFailureCreatesNoToken(t *testing.T) {
	hub, db := newBrokenVaultTestHub(t)
	if _, err := hub.IssueEnrollmentToken("fresh-admin-token", "test", time.Now().Add(time.Hour).UTC().Format(time.RFC3339)); err == nil {
		t.Fatal("token issuance must fail when the MAC key cannot be stored")
	}
	assertNoEnrollmentRows(t, db)
}

func TestEnrollmentTokenSingleUseAndReconnectReplay(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	issueTestEnrollment(t, hub, token)
	first := exchange(enrollmentFrame(t, token, 1))
	if first.Status != "enrolled" || first.DeviceID == "" {
		t.Fatalf("first enrollment = %+v", first)
	}
	if got := exchange(enrollmentFrame(t, token, 2)); got.Status != "rejected" {
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

// The lookup hash travels in the clear. A frame signed with it must not enroll,
// and the refusal must not be signed with anything the requester could use.
func TestEnrollmentRequiresAuthKeyNotLookupHash(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	exchange := enrollmentExchange(t, hub)
	token := "fresh-admin-token"
	id := issueTestEnrollment(t, hub, token)

	lookup := DeriveEnrollmentLookupHash(token)
	forged, err := NewMessage(MsgAuth, "", lookup, 1, AuthPayload{KDF: EnrollmentKDFVersion, TokenHash: lookup, Hostname: "attacker"})
	if err != nil {
		t.Fatal(err)
	}
	response, payload := exchange(forged)
	if payload.Status != "rejected" || payload.SharedKey != "" || payload.DeviceID != "" {
		t.Fatalf("frame signed with the lookup hash = %+v", payload)
	}
	if ok, _ := VerifyMessage(response, lookup); ok {
		t.Fatal("the refusal must not be signed with the lookup hash")
	}
	assertEnrollmentUnused(t, db, token, id)
	assertNoDevices(t, db)

	response, payload = exchange(enrollmentFrame(t, token, 2))
	if payload.Status != "enrolled" || payload.DeviceID == "" || payload.SharedKey == "" {
		t.Fatalf("frame signed with the MAC key = %+v", payload)
	}
	if ok, err := VerifyMessage(response, DeriveEnrollmentAuthKey(token)); err != nil || !ok {
		t.Fatalf("enrolled answer must be signed with the MAC key: ok=%v err=%v", ok, err)
	}
	if ok, _ := VerifyMessage(response, lookup); ok {
		t.Fatal("enrolled answer must not verify under the lookup hash")
	}
}

// A row without a vault MAC key was created before the key split, stored the
// plain token hash, and must not be accepted under any key.
func TestEnrollmentRejectsTokenWithoutVaultKey(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	id, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: DeriveEnrollmentLookupHash(token), DeviceName: "test", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	got := exchange(enrollmentFrame(t, token, 1))
	if got.Status != "rejected" || got.Message != "enrollment token predates the upgrade; create a new one" {
		t.Fatalf("row without vault key = %+v", got)
	}
	assertEnrollmentUnused(t, db, token, id)
	assertNoDevices(t, db)
}

// Agents that predate the key split send no kdf field and the plain token hash,
// signed with that same hash. They get a clear refusal and the token is left
// alone.
func TestEnrollmentRejectsFrameWithoutKDF(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	id := issueTestEnrollment(t, hub, token)

	preUpgrade, err := NewMessage(MsgAuth, "", plainSHA256Hex(token), 1, AuthPayload{TokenHash: plainSHA256Hex(token), Hostname: "old-agent"})
	if err != nil {
		t.Fatal(err)
	}
	withoutKDF, err := NewMessage(MsgAuth, "", DeriveEnrollmentAuthKey(token), 2, AuthPayload{TokenHash: DeriveEnrollmentLookupHash(token), Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for name, frame := range map[string]*RemoteMessage{"pre-upgrade agent": preUpgrade, "lookup hash without kdf": withoutKDF} {
		got := exchange(frame)
		if got.Status != "rejected" || got.Message != "enrollment token predates the upgrade; create a new one" {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	assertEnrollmentUnused(t, db, token, id)
	assertNoDevices(t, db)
	if got := exchange(enrollmentFrame(t, token, 3)); got.Status != "enrolled" {
		t.Fatalf("token must still enroll a current agent after the refusals: %+v", got)
	}
}

func TestFinalizeEnrollmentRemovesVaultKey(t *testing.T) {
	hub, db, vault := newEnrollmentTestHub(t)
	token := "fresh-admin-token"
	id := issueTestEnrollment(t, hub, token)
	deviceID, err := CreateDevice(db, DeviceRecord{Name: "test", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if err := finalizeEnrollment(db, vault, id, deviceID); err != nil {
		t.Fatalf("finalizeEnrollment: %v", err)
	}
	if _, err := vault.ReadSecret(enrollmentAuthKeyName(id)); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("finalized enrollment's MAC key must be deleted, got %v", err)
	}
	enrollment, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentLookupHash(token))
	if err != nil || !enrollment.Used || enrollment.UsedByDevice != deviceID {
		t.Fatalf("enrollment after finalize = %+v, %v", enrollment, err)
	}
}

func TestCleanExpiredEnrollmentsRemovesOrphanedVaultKeys(t *testing.T) {
	hub, db, vault := newEnrollmentTestHub(t)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)

	live := issueTestEnrollment(t, hub, "live-token")
	expired, err := hub.IssueEnrollmentToken("expired-token", "test", past)
	if err != nil {
		t.Fatal(err)
	}
	consumed := issueTestEnrollment(t, hub, "consumed-token")
	if err := MarkEnrollmentUsed(db, consumed, "agodesk-device"); err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret(enrollmentAuthKeyName("deleted-row"), DeriveEnrollmentAuthKey("gone")); err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("remote_shared_key_device-1", strings.Repeat("ab", 32)); err != nil {
		t.Fatal(err)
	}

	if err := CleanExpiredEnrollments(db, vault); err != nil {
		t.Fatalf("CleanExpiredEnrollments: %v", err)
	}

	for _, id := range []string{expired, consumed, "deleted-row"} {
		if _, err := vault.ReadSecret(enrollmentAuthKeyName(id)); !errors.Is(err, security.ErrSecretNotFound) {
			t.Fatalf("MAC key of dead enrollment %s must be deleted, got %v", id, err)
		}
	}
	if _, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentLookupHash("expired-token")); err == nil {
		t.Fatal("expired enrollment row must be deleted")
	}
	if key, err := vault.ReadSecret(enrollmentAuthKeyName(live)); err != nil || key != DeriveEnrollmentAuthKey("live-token") {
		t.Fatalf("live enrollment's MAC key must be kept: %v", err)
	}
	if _, err := vault.ReadSecret("remote_shared_key_device-1"); err != nil {
		t.Fatalf("unrelated vault entries must be kept: %v", err)
	}
	if _, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentLookupHash("live-token")); err != nil {
		t.Fatalf("live enrollment row must be kept: %v", err)
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
	hub, _, _ := newEnrollmentTestHub(t)
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	issueTestEnrollment(t, hub, token)

	enroll := enrollmentFrame(t, token, 12)
	if got := exchange(shiftSequenceIntoNonce(t, enroll, DeriveEnrollmentAuthKey(token))); got.Status != "rejected" {
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

// A vault that cannot be used must leave the token unused and no device behind,
// whether it fails when the MAC key is read or when the new device key is
// stored.
func TestEnrollmentVaultFailureLeavesTokenUnused(t *testing.T) {
	hub, db := newBrokenVaultTestHub(t)
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	id, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: DeriveEnrollmentLookupHash(token), DeviceName: "test", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	if got := exchange(enrollmentFrame(t, token, 1)); got.Status != "rejected" {
		t.Fatalf("vault read failure response = %+v", got)
	}
	assertEnrollmentUnused(t, db, token, id)
	assertNoDevices(t, db)

	// The MAC key was read; storing the device key fails.
	serverConn, clientConn, cleanup := newWebSocketPairForHubTest(t)
	defer cleanup()
	done := make(chan error, 1)
	go func() {
		done <- hub.completeEnrollment(serverConn, AuthPayload{Hostname: "test"}, id, "test", DeriveEnrollmentAuthKey(token))
	}()
	var response RemoteMessage
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := clientConn.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("completeEnrollment: %v", err)
	}
	var payload AuthResponsePayload
	if err := json.Unmarshal(response.Payload, &payload); err != nil || payload.Status != "rejected" || payload.SharedKey != "" {
		t.Fatalf("vault write failure response = %+v, %v", payload, err)
	}
	assertEnrollmentUnused(t, db, token, id)
	assertNoDevices(t, db)
}
