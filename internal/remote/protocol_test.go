package remote

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
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

// IsShellOperation must cover every Op* constant whose value starts with
// "shell_", including ones added later, and nothing else. The constants are
// read from protocol.go itself.
func TestIsShellOperationCoversEveryShellOp(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "protocol.go", nil, 0)
	if err != nil {
		t.Fatalf("parse protocol.go: %v", err)
	}
	shellOps := 0
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec := spec.(*ast.ValueSpec)
			for i, name := range valueSpec.Names {
				if !strings.HasPrefix(name.Name, "Op") || i >= len(valueSpec.Values) {
					continue
				}
				lit, ok := valueSpec.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", name.Name, err)
				}
				isShell := strings.HasPrefix(value, "shell_")
				if isShell {
					shellOps++
				}
				if IsShellOperation(value) != isShell {
					t.Errorf("IsShellOperation(%s = %q) = %v, want %v", name.Name, value, !isShell, isShell)
				}
			}
		}
	}
	if shellOps < 7 {
		t.Fatalf("found %d shell_ operations in protocol.go, want at least 7", shellOps)
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

// ── Canonical HMAC form ─────────────────────────────────────────────────────

func signedTestFrame(t *testing.T, key string, seq uint64) *RemoteMessage {
	t.Helper()
	msg, err := NewMessage(MsgCommand, "dev-1", key, seq, CommandPayload{CommandID: "cmd-1", Operation: OpSysinfo})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// Under the undelimited form a frame signed with sequence 12 and nonce N also
// verified as sequence 1 with nonce "2"+N; bytes could move across any field
// boundary. The version 2 form length-prefixes every field, so no shift
// verifies.
func TestCanonicalHMACRejectsFieldBoundaryShifts(t *testing.T) {
	key, _ := GenerateSharedKey()
	frame := signedTestFrame(t, key, 12)
	if frame.Version != FrameVersion {
		t.Fatalf("NewMessage version = %d, want %d", frame.Version, FrameVersion)
	}

	shifts := map[string]func(m *RemoteMessage){
		"sequence digit into nonce": func(m *RemoteMessage) { m.Sequence = 1; m.Nonce = "2" + m.Nonce },
		"nonce char into sequence": func(m *RemoteMessage) {
			m.Sequence = 120
			m.Nonce = m.Nonce[1:]
		},
		"type into device id":   func(m *RemoteMessage) { m.Type = "comman"; m.DeviceID = "d" + m.DeviceID },
		"device id into msg id": func(m *RemoteMessage) { m.DeviceID = "dev-"; m.MessageID = "1" + m.MessageID },
		"timestamp into payload": func(m *RemoteMessage) {
			m.Payload = append([]byte(m.Timestamp[len(m.Timestamp)-1:]), m.Payload...)
			m.Timestamp = m.Timestamp[:len(m.Timestamp)-1]
		},
		"version into type": func(m *RemoteMessage) { m.Version = 0; m.Type = "2" + m.Type },
	}
	for name, shift := range shifts {
		shifted := *frame
		shift(&shifted)
		if bytes.Equal(hmacData(&shifted), hmacData(frame)) {
			t.Fatalf("%s: canonical form is not injective", name)
		}
		if ok, _ := VerifyMessage(shifted, key); ok {
			t.Fatalf("%s: shifted frame still verifies", name)
		}
	}
	if ok, err := VerifyMessage(*frame, key); err != nil || !ok {
		t.Fatalf("original frame must verify: ok=%v err=%v", ok, err)
	}
}

// The encoding is part of the wire contract between agent and supervisor.
func TestCanonicalHMACEncoding(t *testing.T) {
	msg := &RemoteMessage{
		Version:   2,
		Type:      MsgAck,
		DeviceID:  "dev",
		MessageID: "m1",
		Sequence:  12,
		Nonce:     "0123456789abcdef0123456789abcdef",
		Timestamp: "2026-10-05T12:00:00Z",
		Payload:   []byte(`{"a":1}`),
	}
	want := "1:2" + "3:ack" + "3:dev" + "2:m1" + "2:12" + "32:0123456789abcdef0123456789abcdef" + "20:2026-10-05T12:00:00Z" + `7:{"a":1}`
	if got := string(hmacData(msg)); got != want {
		t.Fatalf("hmacData = %q\nwant      %q", got, want)
	}
}

func TestVersion2FramesRoundTrip(t *testing.T) {
	key, _ := GenerateSharedKey()
	frame := signedTestFrame(t, key, 7)
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"v":2`) {
		t.Fatalf("wire frame must carry its version: %s", raw)
	}
	var decoded RemoteMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyMessage(decoded, key); err != nil || !ok {
		t.Fatalf("decoded v2 frame must verify: ok=%v err=%v", ok, err)
	}
	unsigned, err := NewMessage(MsgAuth, "", "", 1, AuthPayload{Hostname: "h"})
	if err != nil || unsigned.Version != FrameVersion {
		t.Fatalf("unsigned frames carry the version too: %+v, %v", unsigned, err)
	}
}

// Frames without the current version are refused outright, including ones
// carrying a valid HMAC in the old undelimited form.
func TestVerifyMessageRejectsUnsupportedFrameVersions(t *testing.T) {
	key, _ := GenerateSharedKey()
	keyBytes, _ := hex.DecodeString(key)

	// The old form, spelled out here so this test does not depend on the
	// supervisor-only legacyHMACData.
	legacy := *signedTestFrame(t, key, 3)
	legacy.Version = 0
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(legacy.Type + legacy.DeviceID + legacy.MessageID + "3" + legacy.Nonce + legacy.Timestamp + string(legacy.Payload)))
	legacy.HMAC = hex.EncodeToString(mac.Sum(nil))
	if ok, err := VerifyMessage(legacy, key); ok || !errors.Is(err, ErrUnsupportedFrameVersion) {
		t.Fatalf("unversioned frame: ok=%v err=%v", ok, err)
	}

	future := *signedTestFrame(t, key, 4)
	future.Version = FrameVersion + 1
	if ok, err := VerifyMessage(future, key); ok || !errors.Is(err, ErrUnsupportedFrameVersion) {
		t.Fatalf("relabelled frame: ok=%v err=%v", ok, err)
	}

	if err := SignMessage(&RemoteMessage{Version: 1, Type: MsgAck}, key); !errors.Is(err, ErrUnsupportedFrameVersion) {
		t.Fatalf("SignMessage must refuse other versions, got %v", err)
	}
	unversioned := &RemoteMessage{Type: MsgAck, DeviceID: "d", Nonce: "0123456789abcdef0123456789abcdef"}
	if err := SignMessage(unversioned, key); err != nil || unversioned.Version != FrameVersion {
		t.Fatalf("SignMessage signs an unversioned frame as version %d: version=%d err=%v", FrameVersion, unversioned.Version, err)
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

// The agent's first frame carries the lookup hash in the clear, so the MAC key
// that signs the enrollment answer must not be computable from it.
func TestEnrollmentLookupHashDoesNotRevealAuthKey(t *testing.T) {
	const token = "remote_0123456789abcdef0123456789abcdef"
	lookup := DeriveEnrollmentLookupHash(token)
	authKey := DeriveEnrollmentAuthKey(token)
	plain := sha256.Sum256([]byte(token))
	plainHex := hex.EncodeToString(plain[:])

	if lookup == authKey {
		t.Fatal("lookup hash and MAC key must differ")
	}
	if lookup == plainHex || authKey == plainHex {
		t.Fatal("neither derivation may be the plain SHA-256 of the token")
	}
	for name, derived := range map[string]string{"lookup": lookup, "auth": authKey} {
		if len(derived) != 64 {
			t.Fatalf("%s derivation has %d hex chars, want 64", name, len(derived))
		}
	}
	if _, err := decodeSharedKey(authKey); err != nil {
		t.Fatalf("MAC key must be a usable HMAC key: %v", err)
	}
	// Agent and supervisor must derive the same values: pin the wire contract.
	wantLookup := sha256.Sum256([]byte("aurago-remote-enroll-lookup\n" + token))
	wantAuth := sha256.Sum256([]byte("aurago-remote-enroll-auth\n" + token))
	if lookup != hex.EncodeToString(wantLookup[:]) || authKey != hex.EncodeToString(wantAuth[:]) {
		t.Fatal("enrollment derivations changed; agent and supervisor would disagree")
	}
	if DeriveEnrollmentLookupHash(token+"x") == lookup || DeriveEnrollmentAuthKey(token+"x") == authKey {
		t.Fatal("derivations must depend on the token")
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
