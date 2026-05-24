package agodesk

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecodeEnvelopeValidatesRequiredFieldsAndSize(t *testing.T) {
	_, err := DecodeEnvelope([]byte(`{"id":"","type":"chat.message","timestamp":"2026-05-24T12:00:00Z","payload":{}}`), 4096)
	if err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("DecodeEnvelope missing id error = %v, want id validation", err)
	}

	_, err = DecodeEnvelope([]byte(`{"id":"msg-1","type":"chat.message","timestamp":"not-time","payload":{}}`), 4096)
	if err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("DecodeEnvelope bad timestamp error = %v, want timestamp validation", err)
	}

	_, err = DecodeEnvelope([]byte(`{"id":"msg-1","type":"chat.message","timestamp":"2026-05-24T12:00:00Z","payload":{}}`), 8)
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("DecodeEnvelope oversize error = %v, want size validation", err)
	}
}

func TestNewEnvelopeCarriesPayload(t *testing.T) {
	env, err := NewEnvelope(TypeSystemPong, map[string]string{"ok": "yes"})
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}
	if env.ID == "" || env.Type != TypeSystemPong || env.Timestamp == "" {
		t.Fatalf("NewEnvelope missing envelope fields: %+v", env)
	}

	var payload map[string]string
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if payload["ok"] != "yes" {
		t.Fatalf("payload ok = %q, want yes", payload["ok"])
	}
}

func TestSharedKeyProofVerifiesEnvelopeBoundHMAC(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	proof, err := NewSharedKeyProof("0123456789abcdef", "session-start-1", "device-1", now)
	if err != nil {
		t.Fatalf("NewSharedKeyProof: %v", err)
	}
	if !VerifySharedKeyProof("0123456789abcdef", "session-start-1", "device-1", proof, now.Add(time.Second), time.Minute) {
		t.Fatal("proof should verify for matching envelope and device")
	}
	if VerifySharedKeyProof("0123456789abcdef", "session-start-2", "device-1", proof, now.Add(time.Second), time.Minute) {
		t.Fatal("proof verified for a different envelope id")
	}
	if VerifySharedKeyProof("0123456789abcdef", "session-start-1", "device-2", proof, now.Add(time.Second), time.Minute) {
		t.Fatal("proof verified for a different device id")
	}
	if VerifySharedKeyProof("0123456789abcdef", "session-start-1", "device-1", proof, now.Add(10*time.Minute), time.Minute) {
		t.Fatal("proof verified outside allowed clock skew")
	}
}
