//go:build !remote_minimal

package remote

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
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

// The sweep compares RFC3339 strings, so expiry times are stored in UTC
// whatever offset the caller used; an unparsable expiry creates no token.
func TestIssueEnrollmentTokenNormalizesExpiryToUTC(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	if _, err := hub.IssueEnrollmentToken("offset-token", "test", "2099-01-01T05:00:00+02:00"); err != nil {
		t.Fatal(err)
	}
	enrollment, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentLookupHash("offset-token"))
	if err != nil || enrollment.ExpiresAt != "2099-01-01T03:00:00Z" {
		t.Fatalf("expiry must be stored in UTC: %+v, %v", enrollment, err)
	}
	if _, err := hub.IssueEnrollmentToken("bad-expiry-token", "test", "tomorrow"); err == nil {
		t.Fatal("an unparsable expiry must be refused")
	}
	if _, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentLookupHash("bad-expiry-token")); err == nil {
		t.Fatal("no token may be created with an unparsable expiry")
	}

	// A token that expired an hour ago, given in a +02:00 offset, is swept.
	past := time.Now().Add(-time.Hour).In(time.FixedZone("plus2", 2*60*60)).Format(time.RFC3339)
	if _, err := hub.IssueEnrollmentToken("expired-offset-token", "test", past); err != nil {
		t.Fatal(err)
	}
	if err := hub.SweepExpiredEnrollments(); err != nil {
		t.Fatal(err)
	}
	if _, err := GetEnrollmentByTokenHash(db, DeriveEnrollmentLookupHash("expired-offset-token")); err == nil {
		t.Fatal("an expired token given with an offset must be swept")
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

// A row stored under the lookup hash but without a vault MAC key (the vault
// entry was lost, or the row was written by something other than
// IssueEnrollmentToken) must not be accepted under any key.
func TestEnrollmentRejectsTokenWithoutVaultKey(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	id, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: DeriveEnrollmentLookupHash(token), DeviceName: "test", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	got := exchange(enrollmentFrame(t, token, 1))
	if got.Status != "rejected" || got.Message != "enrollment token has no key on this supervisor; create a new one" {
		t.Fatalf("row without vault key = %+v", got)
	}
	assertEnrollmentUnused(t, db, token, id)
	assertNoDevices(t, db)
}

// Tokens issued before the key split are stored under the plain SHA-256 of the
// token, which a current agent's lookup hash never matches. They are refused as
// not found, with wording that covers the pre-upgrade case.
func TestEnrollmentRefusesPreUpgradeRowAsNotFound(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	exchange := enrollmentTestSocket(t, hub)
	token := "remote_0123456789abcdef0123456789abcdef"
	id, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: plainSHA256Hex(token), DeviceName: "test", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	got := exchange(enrollmentFrame(t, token, 1))
	if got.Status != "rejected" || got.Message != "invalid or pre-upgrade enrollment token; create a new one" {
		t.Fatalf("pre-upgrade row = %+v", got)
	}
	enrollment, err := GetEnrollmentByTokenHash(db, plainSHA256Hex(token))
	if err != nil || enrollment.ID != id || enrollment.Used {
		t.Fatalf("pre-upgrade row must stay untouched: %+v, %v", enrollment, err)
	}
	assertNoDevices(t, db)
}

// Agents that predate the key split send no kdf field and the plain token hash,
// signed with that same hash (TestHandleEnrollmentRefusesPreUpgradeFrames
// covers their unversioned frames). Any enrollment frame without kdf 2 gets a
// clear refusal and the token is left alone.
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
	for name, frame := range map[string]*RemoteMessage{"plain token hash without kdf": preUpgrade, "lookup hash without kdf": withoutKDF} {
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

	if err := hub.SweepExpiredEnrollments(); err != nil {
		t.Fatalf("SweepExpiredEnrollments: %v", err)
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

// A reconnect frame that fails verification is refused unsigned: a signed
// refusal would hand an unauthenticated requester a frame only the supervisor
// can make. Refusals echo the request nonce only when it is well formed.
func TestReconnectRefusalsAreUnsignedUntilVerifiedAndEchoOnlyValidNonces(t *testing.T) {
	hub, _, _ := newEnrollmentTestHub(t)
	exchange := enrollmentExchange(t, hub)
	token := "fresh-admin-token"
	issueTestEnrollment(t, hub, token)
	_, enrolled := exchange(enrollmentFrame(t, token, 1))
	if enrolled.Status != "enrolled" {
		t.Fatalf("enrollment = %+v", enrolled)
	}
	hub.Unregister(enrolled.DeviceID)

	forged, err := NewMessage(MsgAuth, enrolled.DeviceID, strings.Repeat("ef", 32), 2, AuthPayload{DeviceID: enrolled.DeviceID})
	if err != nil {
		t.Fatal(err)
	}
	response, refused := exchange(forged)
	if refused.Status != "rejected" || refused.Message != "authentication failed" || response.HMAC != "" {
		t.Fatalf("failed reconnect must be refused unsigned: %+v (hmac %q)", refused, response.HMAC)
	}
	if hub.IsConnected(enrolled.DeviceID) {
		t.Fatal("a failed reconnect must not register a connection")
	}

	valid := strings.Repeat("0123456789abcdef", 2)
	for name, nonce := range map[string]string{
		"empty":     "",
		"short":     "xyz",
		"uppercase": strings.ToUpper("abcdef" + valid[6:]),
		"33 chars":  valid + "0",
	} {
		frame, err := NewMessage(MsgAuth, enrolled.DeviceID, enrolled.SharedKey, 3, AuthPayload{DeviceID: enrolled.DeviceID})
		if err != nil {
			t.Fatal(err)
		}
		frame.Nonce = nonce
		if err := SignMessage(frame, enrolled.SharedKey); err != nil {
			t.Fatal(err)
		}
		response, refused := exchange(frame)
		if refused.Status != "rejected" || refused.Message != "stale or replayed authentication" || refused.RequestNonce != "" {
			t.Fatalf("%s nonce: refusal must not echo it: %+v", name, refused)
		}
		if ok, err := VerifyMessage(response, enrolled.SharedKey); err != nil || !ok {
			t.Fatalf("%s nonce: a refusal to a verified frame stays signed: ok=%v err=%v", name, ok, err)
		}
	}
}

// Every answer to an auth frame echoes that frame's nonce under the HMAC, so the
// agent can tell it answers this request and not an earlier one.
func TestAuthResponsesEchoRequestNonce(t *testing.T) {
	hub, _, _ := newEnrollmentTestHub(t)
	exchange := enrollmentExchange(t, hub)
	token := "fresh-admin-token"
	issueTestEnrollment(t, hub, token)

	enroll := enrollmentFrame(t, token, 1)
	response, enrolled := exchange(enroll)
	if enrolled.Status != "enrolled" || enrolled.RequestNonce != enroll.Nonce {
		t.Fatalf("enrolled answer must echo the auth nonce %s: %+v", enroll.Nonce, enrolled)
	}
	if ok, err := VerifyMessage(response, DeriveEnrollmentAuthKey(token)); err != nil || !ok {
		t.Fatalf("enrolled answer must be signed: ok=%v err=%v", ok, err)
	}

	reconnect, err := NewMessage(MsgAuth, enrolled.DeviceID, enrolled.SharedKey, 2, AuthPayload{DeviceID: enrolled.DeviceID, Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	response, authenticated := exchange(reconnect)
	if authenticated.Status != "authenticated" || authenticated.RequestNonce != reconnect.Nonce {
		t.Fatalf("authenticated answer must echo the auth nonce %s: %+v", reconnect.Nonce, authenticated)
	}
	if ok, err := VerifyMessage(response, enrolled.SharedKey); err != nil || !ok {
		t.Fatalf("authenticated answer must be signed: ok=%v err=%v", ok, err)
	}

	again := enrollmentFrame(t, token, 3)
	if _, refused := exchange(again); refused.Status != "rejected" || refused.RequestNonce != again.Nonce {
		t.Fatalf("refusals echo the auth nonce too: %+v", refused)
	}
}

// shiftSequenceIntoNonce moves the last sequence digit into the nonce. Under
// the old undelimited HMAC form the copy still verified; under the canonical
// form it must not.
func shiftSequenceIntoNonce(t *testing.T, msg *RemoteMessage, key string) *RemoteMessage {
	t.Helper()
	if msg.Sequence != 12 {
		t.Fatalf("shift helper expects sequence 12, got %d", msg.Sequence)
	}
	shifted := *msg
	shifted.Sequence = 1
	shifted.Nonce = "2" + msg.Nonce
	if ok, _ := VerifyMessage(shifted, key); ok {
		t.Fatal("a sequence digit shifted into the nonce must not keep a valid HMAC")
	}
	return &shifted
}

// legacySigned re-signs msg in the pre-version-2 form, as an agent built before
// the canonical HMAC form would.
func legacySigned(t *testing.T, msg *RemoteMessage, key string) *RemoteMessage {
	t.Helper()
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		t.Fatal(err)
	}
	legacy := *msg
	legacy.Version = 0
	legacy.HMAC = ""
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write(legacyHMACData(&legacy))
	legacy.HMAC = hex.EncodeToString(mac.Sum(nil))
	return &legacy
}

func verifiesLegacy(msg RemoteMessage, key string) bool {
	keyBytes, err := hex.DecodeString(key)
	if err != nil || msg.HMAC == "" {
		return false
	}
	expected := msg.HMAC
	msg.HMAC = ""
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write(legacyHMACData(&msg))
	return hmac.Equal([]byte(expected), []byte(hex.EncodeToString(mac.Sum(nil))))
}

// Agents built before the canonical HMAC form send unversioned frames. Nothing
// in them is verified or granted. An enrollment frame gets a refusal in the old
// form signed with the plain token hash that agent verifies with; a reconnect
// naming a known device gets one signed with that device's key. Both let the
// old agent show the reason. Every other unversioned frame is refused unsigned.
func TestHandleEnrollmentRefusesPreUpgradeFrames(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	exchange := enrollmentExchange(t, hub)
	token := "fresh-admin-token"
	id := issueTestEnrollment(t, hub, token)

	plainHash := plainSHA256Hex(token)
	oldEnroll, err := NewMessage(MsgAuth, "", plainHash, 1, AuthPayload{TokenHash: plainHash, Hostname: "old-agent"})
	if err != nil {
		t.Fatal(err)
	}
	oldEnroll = legacySigned(t, oldEnroll, plainHash)
	response, refused := exchange(oldEnroll)
	// The binary is what is outdated, whatever the token's age, so the reason
	// names the agent download (which also issues a new token).
	if refused.Status != "rejected" || refused.Message != "agent predates the supervisor upgrade; download the agent again from AuraGo (Remote Control) — that also issues a new token" || refused.RequestNonce != oldEnroll.Nonce {
		t.Fatalf("pre-upgrade enrollment = %+v", refused)
	}
	var rawRefusal map[string]any
	if err := json.Unmarshal(response.Payload, &rawRefusal); err != nil || len(rawRefusal) != 3 {
		t.Fatalf("a legacy refusal carries only status, message and request_nonce: %s", response.Payload)
	}
	if response.Version != 0 || !verifiesLegacy(response, plainHash) || !ValidNonce(response.Nonce) || ValidateTimestamp(response.Timestamp) != nil {
		t.Fatalf("the refusal must be a fresh frame an old agent can verify: %+v", response)
	}
	if ok, _ := VerifyMessage(response, plainHash); ok {
		t.Fatal("the legacy refusal must not pass current verification")
	}
	assertEnrollmentUnused(t, db, token, id)
	assertNoDevices(t, db)

	// A current frame enrolls; its device then reconnects with an old frame.
	_, enrolled := exchange(enrollmentFrame(t, token, 2))
	if enrolled.Status != "enrolled" {
		t.Fatalf("current enrollment = %+v", enrolled)
	}
	hub.Unregister(enrolled.DeviceID)
	before, err := GetDevice(db, enrolled.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	oldReconnect, err := NewMessage(MsgAuth, enrolled.DeviceID, enrolled.SharedKey, 3, AuthPayload{DeviceID: enrolled.DeviceID, Hostname: "old-agent"})
	if err != nil {
		t.Fatal(err)
	}
	oldReconnect = legacySigned(t, oldReconnect, enrolled.SharedKey)
	response, refused = exchange(oldReconnect)
	if refused.Status != "rejected" || refused.Message != preUpgradeReconnectMessage || refused.RequestNonce != oldReconnect.Nonce ||
		refused.DeviceID != "" || refused.SharedKey != "" || refused.ReadOnly != nil || refused.AllowedPaths != nil || refused.MaxFileSizeMB != 0 {
		t.Fatalf("unversioned reconnect of a known device = %+v", refused)
	}
	if response.Version != 0 || !verifiesLegacy(response, enrolled.SharedKey) || !ValidNonce(response.Nonce) || ValidateTimestamp(response.Timestamp) != nil {
		t.Fatalf("the refusal must be a fresh frame the old agent can verify with its device key: %+v", response)
	}
	if ok, _ := VerifyMessage(response, enrolled.SharedKey); ok {
		t.Fatal("the legacy refusal must not pass current verification")
	}
	if hub.IsConnected(enrolled.DeviceID) {
		t.Fatal("an unversioned reconnect must not register a connection")
	}
	if after, err := GetDevice(db, enrolled.DeviceID); err != nil || after.Status != before.Status || after.LastSeen != before.LastSeen {
		t.Fatalf("an unversioned reconnect must not touch the device: before %+v, after %+v, %v", before, after, err)
	}

	// Unknown and revoked devices get the unsigned refusal.
	revokedID, err := CreateDevice(db, DeviceRecord{Name: "revoked", Status: "revoked"})
	if err != nil {
		t.Fatal(err)
	}
	if err := hub.vault.WriteSecret("remote_shared_key_"+revokedID, strings.Repeat("cd", 32)); err != nil {
		t.Fatal(err)
	}
	for name, deviceID := range map[string]string{"unknown": "no-such-device", "revoked": revokedID} {
		frame, err := NewMessage(MsgAuth, deviceID, strings.Repeat("cd", 32), 4, AuthPayload{DeviceID: deviceID})
		if err != nil {
			t.Fatal(err)
		}
		response, refused = exchange(legacySigned(t, frame, strings.Repeat("cd", 32)))
		if refused.Status != "rejected" || refused.Message != "unsupported frame version" || response.HMAC != "" {
			t.Fatalf("unversioned reconnect of a %s device = %+v (hmac %q)", name, refused, response.HMAC)
		}
	}

	oldKnock, err := NewMessage(MsgAuth, "", "", 4, AuthPayload{Hostname: "old-knock"})
	if err != nil {
		t.Fatal(err)
	}
	oldKnock.Version = 0
	if _, refused = exchange(oldKnock); refused.Status != "rejected" || refused.Message != "unsupported frame version" {
		t.Fatalf("unversioned tokenless knock = %+v", refused)
	}
	var pending int
	if err := db.QueryRow(`SELECT COUNT(*) FROM remote_devices WHERE status='pending'`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("an unversioned knock must not create a pending device: %d, %v", pending, err)
	}
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

// newCorruptVaultTestHub returns a hub whose vault file exists but cannot be
// decrypted, so every read fails with an error other than ErrSecretNotFound.
func newCorruptVaultTestHub(t *testing.T) (*RemoteHub, *sql.DB) {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	if err := os.WriteFile(vaultPath, []byte(strings.Repeat("not a vault ", 8)), 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := security.NewVault(strings.Repeat("a", 64), vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	return NewRemoteHub(db, vault, slog.New(slog.NewTextHandler(io.Discard, nil))), db
}

// An unsigned enrollment frame is refused before the vault is touched.
func TestUnsignedEnrollmentFrameRefusedBeforeVaultRead(t *testing.T) {
	hub, db := newCorruptVaultTestHub(t)
	exchange := enrollmentTestSocket(t, hub)
	token := "fresh-admin-token"
	if _, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: DeriveEnrollmentLookupHash(token), ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	unsigned, err := NewMessage(MsgAuth, "", "", 1, AuthPayload{KDF: EnrollmentKDFVersion, TokenHash: DeriveEnrollmentLookupHash(token)})
	if err != nil {
		t.Fatal(err)
	}
	if got := exchange(unsigned); got.Status != "rejected" || got.Message != "HMAC required for token enrollment" {
		t.Fatalf("unsigned enrollment frame = %+v", got)
	}
}

// A vault that cannot be used must leave the token unused and no device behind,
// whether it fails when the MAC key is read or when the new device key is
// stored.
func TestEnrollmentVaultFailureLeavesTokenUnused(t *testing.T) {
	token := "fresh-admin-token"
	expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	for name, newHub := range map[string]func(*testing.T) (*RemoteHub, *sql.DB){
		"missing vault directory": newBrokenVaultTestHub,
		"corrupt vault file":      newCorruptVaultTestHub,
	} {
		t.Run("read fails: "+name, func(t *testing.T) {
			hub, db := newHub(t)
			exchange := enrollmentTestSocket(t, hub)
			id, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: DeriveEnrollmentLookupHash(token), DeviceName: "test", ExpiresAt: expires})
			if err != nil {
				t.Fatal(err)
			}
			// A read error is not a missing key: it must not be reported as a
			// pre-upgrade token.
			if got := exchange(enrollmentFrame(t, token, 1)); got.Status != "rejected" || got.Message != "credential storage unavailable" {
				t.Fatalf("vault read failure response = %+v", got)
			}
			assertEnrollmentUnused(t, db, token, id)
			assertNoDevices(t, db)
		})
	}

	hub, db := newBrokenVaultTestHub(t)
	id, err := CreateEnrollment(db, EnrollmentRecord{TokenHash: DeriveEnrollmentLookupHash(token), DeviceName: "test", ExpiresAt: expires})
	if err != nil {
		t.Fatal(err)
	}

	// The MAC key was read; storing the device key fails.
	serverConn, clientConn, cleanup := newWebSocketPairForHubTest(t)
	defer cleanup()
	done := make(chan error, 1)
	go func() {
		done <- hub.completeEnrollment(serverConn, strings.Repeat("0", 32), AuthPayload{Hostname: "test"}, id, "test", DeriveEnrollmentAuthKey(token))
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
