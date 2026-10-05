package remote

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

// ── ReadOnlySafe ────────────────────────────────────────────────────────────

func TestReadOnlySafe(t *testing.T) {
	safe := []string{
		OpSysinfo, OpFileRead, OpFileList,
		OpDesktopScreenshot, OpDesktopStreamStart, OpDesktopStreamStop, OpDesktopPermissionRequest,
		OpDesktopListDisplays, OpDesktopListWindows, OpDesktopActiveWindow, OpDesktopHostInfo,
		OpDesktopUITree, OpDesktopBrowserConnect, OpDesktopBrowserSnapshot, OpDesktopBrowserDisconnect,
	}
	for _, op := range safe {
		if !ReadOnlySafe(op) {
			t.Errorf("ReadOnlySafe(%q) = false; want true", op)
		}
	}
	unsafe := []string{
		OpFileWrite, OpFileDelete, OpFilePatch,
		OpShellExec, OpShellExecStream, OpShellSessionStart, OpShellSessionRead, OpShellSessionInput, OpShellSessionStop, OpShellSessionList,
		OpDesktopInput, OpDesktopUIAction, OpDesktopBrowserAction,
	}
	for _, op := range unsafe {
		if ReadOnlySafe(op) {
			t.Errorf("ReadOnlySafe(%q) = true; want false", op)
		}
	}
}

// ── Nonce & SharedKey ───────────────────────────────────────────────────────

func TestGenerateNonce(t *testing.T) {
	n1, err := GenerateNonce()
	if err != nil {
		t.Fatalf("GenerateNonce: %v", err)
	}
	if len(n1) != 32 { // 16 bytes → 32 hex chars
		t.Fatalf("nonce length = %d; want 32", len(n1))
	}
	n2, err := GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}
	if n1 == n2 {
		t.Error("two nonces are identical; expected unique values")
	}
}

func TestValidNonce(t *testing.T) {
	generated, err := GenerateNonce()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		nonce string
		want  bool
	}{
		{name: "generated", nonce: generated, want: true},
		{name: "lowercase hex", nonce: "0123456789abcdef0123456789abcdef", want: true},
		{name: "uppercase hex", nonce: "0123456789ABCDEF0123456789ABCDEF", want: false},
		{name: "empty", nonce: "", want: false},
		{name: "short", nonce: "0123456789abcdef0123456789abcde", want: false},
		{name: "long (sequence digit shifted in)", nonce: "2" + "0123456789abcdef0123456789abcdef", want: false},
		{name: "non-hex", nonce: "0123456789abcdef0123456789abcdeg", want: false},
		{name: "non-ascii (32 bytes)", nonce: "0123456789abcdef0123456789abcd" + "é", want: false},
	}
	for _, tt := range tests {
		if got := ValidNonce(tt.nonce); got != tt.want {
			t.Errorf("%s: ValidNonce(%q) = %v; want %v", tt.name, tt.nonce, got, tt.want)
		}
	}
}

func TestGenerateSharedKey(t *testing.T) {
	k, err := GenerateSharedKey()
	if err != nil {
		t.Fatalf("GenerateSharedKey: %v", err)
	}
	if len(k) != 64 { // 32 bytes → 64 hex chars
		t.Fatalf("shared key length = %d; want 64", len(k))
	}
}

// ── HMAC sign/verify ────────────────────────────────────────────────────────

func TestSignAndVerifyMessage(t *testing.T) {
	key, _ := GenerateSharedKey()

	msg := &RemoteMessage{
		Type:      MsgHeartbeat,
		DeviceID:  "test-device-1",
		MessageID: "msg-123",
		Sequence:  1,
		Nonce:     "deadbeef" + "deadbeef" + "deadbeef" + "deadbeef",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   []byte(`{"cpu_percent":42}`),
	}

	if err := SignMessage(msg, key); err != nil {
		t.Fatalf("SignMessage: %v", err)
	}
	if msg.HMAC == "" {
		t.Fatal("HMAC field is empty after signing")
	}

	ok, err := VerifyMessage(*msg, key)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if !ok {
		t.Error("VerifyMessage returned false for correctly-signed message")
	}
}

func TestVerifyMessageTamperedPayload(t *testing.T) {
	key, _ := GenerateSharedKey()

	msg := &RemoteMessage{
		Type:      MsgCommand,
		DeviceID:  "device-2",
		MessageID: "msg-456",
		Sequence:  5,
		Nonce:     "aabbccddaabbccddaabbccddaabbccdd",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   []byte(`{"op":"sysinfo"}`),
	}
	_ = SignMessage(msg, key)

	// tamper payload
	msg.Payload = []byte(`{"op":"shell_exec","cmd":"rm -rf /"}`)

	ok, err := VerifyMessage(*msg, key)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if ok {
		t.Error("VerifyMessage accepted tampered message")
	}
}

func TestVerifyMessageWrongKey(t *testing.T) {
	key1, _ := GenerateSharedKey()
	key2, _ := GenerateSharedKey()

	msg := &RemoteMessage{
		Type:      MsgResult,
		DeviceID:  "device-3",
		MessageID: "msg-789",
		Sequence:  10,
		Nonce:     "11223344556677881122334455667788",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   []byte(`{"status":"ok"}`),
	}
	_ = SignMessage(msg, key1)

	ok, _ := VerifyMessage(*msg, key2)
	if ok {
		t.Error("VerifyMessage accepted message signed with different key")
	}
}

// hex.DecodeString("") yields a zero-length key without error, and HMAC with an
// empty key is computable by anyone, so both directions must refuse it.
func TestSignAndVerifyRejectEmptyKey(t *testing.T) {
	msg := &RemoteMessage{
		Type:      MsgCommand,
		DeviceID:  "dev-1",
		MessageID: "msg-1",
		Sequence:  1,
		Nonce:     "0123456789abcdef0123456789abcdef",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   []byte(`{"cmd_id":"x"}`),
	}
	if err := SignMessage(msg, ""); err == nil {
		t.Fatal("SignMessage must refuse an empty key")
	}

	mac := hmac.New(sha256.New, nil)
	mac.Write([]byte(hmacData(msg)))
	msg.HMAC = hex.EncodeToString(mac.Sum(nil))
	ok, err := VerifyMessage(*msg, "")
	if err == nil || ok {
		t.Fatalf("VerifyMessage must refuse an empty key even for an empty-key HMAC: ok=%v err=%v", ok, err)
	}
}

func TestSignMessageInvalidKey(t *testing.T) {
	msg := &RemoteMessage{Type: MsgAck, DeviceID: "d"}
	if err := SignMessage(msg, "not-hex!"); err == nil {
		t.Error("expected error for invalid hex key")
	}
}

// ── NewMessage ──────────────────────────────────────────────────────────────

func TestNewMessageSigned(t *testing.T) {
	key, _ := GenerateSharedKey()
	msg, err := NewMessage(MsgHeartbeat, "dev-1", key, 1, HeartbeatPayload{CPUPercent: 55})
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.Type != MsgHeartbeat {
		t.Errorf("Type = %q; want %q", msg.Type, MsgHeartbeat)
	}
	if msg.HMAC == "" {
		t.Error("HMAC should be set for signed message")
	}
	ok, _ := VerifyMessage(*msg, key)
	if !ok {
		t.Error("NewMessage produced unverifiable signature")
	}
}

func TestNewMessageUnsigned(t *testing.T) {
	msg, err := NewMessage(MsgAuth, "dev-enroll", "", 0, AuthPayload{Version: "1.0"})
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.HMAC != "" {
		t.Error("HMAC should be empty for unsigned enrollment message")
	}
}

func TestNewAuthResponseMessageSigned(t *testing.T) {
	bootstrapKey := DeriveEnrollmentAuthKey("remote_bootstrap_token")
	msg, err := NewAuthResponseMessage("dev-enroll", bootstrapKey, AuthResponsePayload{
		Status:        "enrolled",
		DeviceID:      "dev-enroll",
		SharedKey:     "shared-key",
		MaxFileSizeMB: DefaultMaxFileSizeMB,
	})
	if err != nil {
		t.Fatalf("NewAuthResponseMessage: %v", err)
	}
	if msg.HMAC == "" {
		t.Fatal("expected signed auth response")
	}
	ok, err := VerifyMessage(*msg, bootstrapKey)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if !ok {
		t.Fatal("expected auth response signature to verify")
	}
}

func TestNewAuthResponseMessageUnsigned(t *testing.T) {
	msg, err := NewAuthResponseMessage("", "", AuthResponsePayload{Status: "pending"})
	if err != nil {
		t.Fatalf("NewAuthResponseMessage: %v", err)
	}
	if msg.HMAC != "" {
		t.Fatal("expected unsigned auth response when no signing key is available")
	}
}

// ── Timestamp validation ────────────────────────────────────────────────────

func TestValidateTimestamp_Current(t *testing.T) {
	ts := time.Now().UTC().Format(time.RFC3339)
	if err := ValidateTimestamp(ts); err != nil {
		t.Errorf("ValidateTimestamp rejected current time: %v", err)
	}
}

func TestValidateTimestamp_Expired(t *testing.T) {
	ts := time.Now().Add(-20 * time.Minute).UTC().Format(time.RFC3339)
	if err := ValidateTimestamp(ts); err == nil {
		t.Error("ValidateTimestamp accepted timestamp 20 minutes in the past")
	}
}

func TestValidateTimestamp_Future(t *testing.T) {
	ts := time.Now().Add(20 * time.Minute).UTC().Format(time.RFC3339)
	if err := ValidateTimestamp(ts); err == nil {
		t.Error("ValidateTimestamp accepted timestamp 20 minutes in the future")
	}
}

func TestValidateTimestamp_Invalid(t *testing.T) {
	if err := ValidateTimestamp("not a timestamp"); err == nil {
		t.Error("ValidateTimestamp accepted malformed string")
	}
}
