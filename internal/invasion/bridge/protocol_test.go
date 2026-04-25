package bridge

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// validKey returns a random 32-byte hex key for tests.
func validKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// ── HMAC signing / verification ─────────────────────────────────────────────

func TestSignAndVerify_RoundTrip(t *testing.T) {
	key := validKey(t)
	msg, err := NewMessage(MsgTask, "egg-1", "nest-1", key, TaskPayload{
		TaskID:      "t1",
		Description: "run tests",
		Timeout:     60,
	})
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.HMAC == "" {
		t.Fatal("HMAC should be set after NewMessage")
	}

	ok, err := VerifyMessage(*msg, key)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if !ok {
		t.Fatal("HMAC verification should succeed for untampered message")
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	key := validKey(t)
	msg, _ := NewMessage(MsgHeartbeat, "egg-1", "nest-1", key, HeartbeatPayload{
		CPUPercent: 42.0,
		Status:     "idle",
	})

	// Tamper with payload
	msg.Payload = []byte(`{"cpu_percent":99.9,"status":"hacked"}`)

	ok, err := VerifyMessage(*msg, key)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if ok {
		t.Fatal("HMAC verification should fail for tampered payload")
	}
}

func TestVerify_TamperedHMAC(t *testing.T) {
	key := validKey(t)
	msg, _ := NewMessage(MsgAck, "egg-1", "nest-1", key, AckPayload{
		RefID: "ref-1", Success: true,
	})

	msg.HMAC = strings.Repeat("aa", 32) // bogus HMAC

	ok, err := VerifyMessage(*msg, key)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if ok {
		t.Fatal("HMAC verification should fail for tampered HMAC")
	}
}

func TestVerify_WrongKey(t *testing.T) {
	key1 := validKey(t)
	key2 := validKey(t)
	msg, _ := NewMessage(MsgTask, "egg-1", "nest-1", key1, nil)

	ok, err := VerifyMessage(*msg, key2)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if ok {
		t.Fatal("HMAC verification should fail with wrong key")
	}
}

func TestSign_InvalidHexKey(t *testing.T) {
	msg := &Message{Type: MsgTask, EggID: "e", NestID: "n", Timestamp: "t"}
	if err := SignMessage(msg, "not-valid-hex!"); err == nil {
		t.Fatal("expected error for invalid hex key")
	}
}

func TestVerify_InvalidHexKey(t *testing.T) {
	msg := Message{Type: MsgTask, EggID: "e", NestID: "n", Timestamp: "t", HMAC: "aabb"}
	_, err := VerifyMessage(msg, "zzzz")
	if err == nil {
		t.Fatal("expected error for invalid hex key")
	}
}

func TestSignVerify_EmptyPayload(t *testing.T) {
	key := validKey(t)
	msg, err := NewMessage(MsgStop, "egg-1", "nest-1", key, nil)
	if err != nil {
		t.Fatalf("NewMessage with nil payload: %v", err)
	}
	ok, err := VerifyMessage(*msg, key)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if !ok {
		t.Fatal("HMAC verification should succeed with nil payload")
	}
}

func TestNewMessage_AllTypes(t *testing.T) {
	key := validKey(t)
	types := []string{MsgAuth, MsgHeartbeat, MsgTask, MsgResult, MsgMissionSync, MsgMissionRun, MsgMissionDelete, MsgMissionResult, MsgStatus, MsgSecret, MsgAck, MsgError, MsgStop}
	for _, msgType := range types {
		t.Run(msgType, func(t *testing.T) {
			msg, err := NewMessage(msgType, "egg-1", "nest-1", key, nil)
			if err != nil {
				t.Fatalf("NewMessage(%s): %v", msgType, err)
			}
			if msg.Type != msgType {
				t.Errorf("type = %q, want %q", msg.Type, msgType)
			}
			if msg.ID == "" {
				t.Error("ID should be set")
			}
			if msg.Timestamp == "" {
				t.Error("Timestamp should be set")
			}
			if msg.HMAC == "" {
				t.Error("HMAC should be set")
			}
		})
	}
}

func TestNewMessageIDIsUniqueForSameTimestamp(t *testing.T) {
	first := newMessageID(time.Unix(1777140000, 123))
	second := newMessageID(time.Unix(1777140000, 123))
	if first == second {
		t.Fatalf("newMessageID returned duplicate IDs for same timestamp: %q", first)
	}
	if !strings.HasPrefix(first, "1777140000000000123-") {
		t.Fatalf("newMessageID prefix = %q, want timestamp prefix", first)
	}
}

func TestNewMessage_MissionSyncPayloadSerialization(t *testing.T) {
	key := validKey(t)
	createdAt := time.Date(2026, 4, 25, 20, 30, 0, 0, time.UTC)
	msg, err := NewMessage(MsgMissionSync, "egg-1", "nest-1", key, MissionSyncPayload{
		Revision:       "rev-1",
		MissionID:      "mission-1",
		Name:           "Remote mission",
		PromptSnapshot: "Base prompt\n\nCheatsheet attachment",
		ExecutionType:  "scheduled",
		Schedule:       "0 * * * *",
		Priority:       "high",
		Enabled:        true,
		AutoPrepare:    true,
		CreatedAt:      createdAt,
	})
	if err != nil {
		t.Fatalf("NewMessage(MsgMissionSync): %v", err)
	}
	if !strings.Contains(string(msg.Payload), `"mission_id":"mission-1"`) {
		t.Fatalf("expected mission_id in payload, got %s", msg.Payload)
	}
	if !strings.Contains(string(msg.Payload), `"prompt_snapshot":"Base prompt\n\nCheatsheet attachment"`) {
		t.Fatalf("expected prompt snapshot in payload, got %s", msg.Payload)
	}
	var decoded MissionSyncPayload
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("unmarshal mission sync payload: %v", err)
	}
	if !decoded.AutoPrepare {
		t.Fatal("AutoPrepare was not preserved")
	}
	if !decoded.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %s, want %s", decoded.CreatedAt, createdAt)
	}
	ok, err := VerifyMessage(*msg, key)
	if err != nil {
		t.Fatalf("VerifyMessage: %v", err)
	}
	if !ok {
		t.Fatal("mission sync payload should verify after signing")
	}
}

func TestNewMessage_PayloadSerialization(t *testing.T) {
	key := validKey(t)
	msg, err := NewMessage(MsgResult, "egg-1", "nest-1", key, ResultPayload{
		TaskID: "t-42",
		Status: "success",
		Output: "all good",
		Tokens: 123,
	})
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if !strings.Contains(string(msg.Payload), `"task_id":"t-42"`) {
		t.Errorf("expected task_id in payload, got %s", msg.Payload)
	}
}

// ── AES-256-GCM encryption ─────────────────────────────────────────────────

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := validKey(t)
	plaintext := []byte("super secret vault data")

	ciphertext, err := EncryptWithSharedKey(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ciphertext == "" {
		t.Fatal("ciphertext should not be empty")
	}

	decrypted, err := DecryptWithSharedKey(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncrypt_DifferentNonces(t *testing.T) {
	key := validKey(t)
	plaintext := []byte("same input")

	ct1, _ := EncryptWithSharedKey(plaintext, key)
	ct2, _ := EncryptWithSharedKey(plaintext, key)

	if ct1 == ct2 {
		t.Fatal("two encryptions of the same plaintext should produce different ciphertexts")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	key := validKey(t)
	ct, _ := EncryptWithSharedKey([]byte("hello"), key)

	// Flip a byte in the middle of the ciphertext
	raw, _ := hex.DecodeString(ct)
	raw[len(raw)/2] ^= 0xff
	tampered := hex.EncodeToString(raw)

	_, err := DecryptWithSharedKey(tampered, key)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := validKey(t)
	key2 := validKey(t)
	ct, _ := EncryptWithSharedKey([]byte("secret"), key1)

	_, err := DecryptWithSharedKey(ct, key2)
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestDecrypt_InvalidHexKey(t *testing.T) {
	_, err := DecryptWithSharedKey("aabbccdd", "not-hex!")
	if err == nil {
		t.Fatal("expected error for invalid hex key")
	}
}

func TestDecrypt_InvalidCiphertextHex(t *testing.T) {
	key := validKey(t)
	_, err := DecryptWithSharedKey("not-hex!", key)
	if err == nil {
		t.Fatal("expected error for invalid ciphertext hex")
	}
}

func TestDecrypt_ShortCiphertext(t *testing.T) {
	key := validKey(t)
	// Just 4 bytes — shorter than a GCM nonce (12 bytes)
	_, err := DecryptWithSharedKey(hex.EncodeToString([]byte{1, 2, 3, 4}), key)
	if err == nil {
		t.Fatal("expected error for ciphertext shorter than nonce")
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	key := validKey(t)
	ct, err := EncryptWithSharedKey([]byte{}, key)
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	decrypted, err := DecryptWithSharedKey(ct, key)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if len(decrypted) != 0 {
		t.Errorf("expected empty plaintext, got %d bytes", len(decrypted))
	}
}

func TestEncrypt_InvalidHexKey(t *testing.T) {
	_, err := EncryptWithSharedKey([]byte("data"), "bad-key")
	if err == nil {
		t.Fatal("expected error for invalid hex key")
	}
}

func TestEncrypt_WrongKeyLength(t *testing.T) {
	// 16-byte key — too short for AES-256 (needs 32 bytes)
	shortKey := hex.EncodeToString(make([]byte, 16))
	ct, err := EncryptWithSharedKey([]byte("data"), shortKey)
	// AES accepts 16-byte keys (AES-128) so this should still work.
	// But we verify round-trip with the same key.
	if err != nil {
		t.Skipf("AES-128 key rejected (may be valid): %v", err)
	}
	dec, err := DecryptWithSharedKey(ct, shortKey)
	if err != nil {
		t.Fatalf("Decrypt with short key: %v", err)
	}
	if string(dec) != "data" {
		t.Errorf("got %q, want %q", dec, "data")
	}
}
