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

func TestSessionStartPayloadCarriesFileAccessMetadata(t *testing.T) {
	env, err := NewEnvelope(TypeSessionStart, SessionStartPayload{
		ClientVersion:      "agodesk-test",
		ClientCapabilities: []string{"remote.files.read", "remote.files.write"},
		FileAccess: &FileAccessPayload{
			Enabled:       true,
			MaxReadBytes:  8 * 1024 * 1024,
			MaxWriteBytes: 4 * 1024 * 1024,
			Roots: []FileAccessRoot{
				{
					RootID:      "workspace",
					Label:       "Workspace",
					PathDisplay: "~/Projects/AuraGo",
					Permissions: []string{"read", "write"},
				},
			},
		},
		Host: SessionStartHost{Hostname: "AGODESK", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}

	var payload SessionStartPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal session.start payload: %v", err)
	}
	if payload.FileAccess == nil || !payload.FileAccess.Enabled {
		t.Fatalf("file_access missing or disabled: %+v", payload.FileAccess)
	}
	if payload.FileAccess.MaxReadBytes != 8*1024*1024 || payload.FileAccess.MaxWriteBytes != 4*1024*1024 {
		t.Fatalf("file_access limits = %+v", payload.FileAccess)
	}
	if len(payload.FileAccess.Roots) != 1 || payload.FileAccess.Roots[0].RootID != "workspace" {
		t.Fatalf("file_access roots = %+v", payload.FileAccess.Roots)
	}
	if got := payload.FileAccess.Roots[0].Permissions; len(got) != 2 || got[0] != "read" || got[1] != "write" {
		t.Fatalf("permissions = %#v, want read/write", got)
	}
}

func TestNewPersonaAssetsPayloadUsesCoreAvatarAndIcon(t *testing.T) {
	payload := NewPersonaAssetsPayload("agodesk:dev:1", "friend", true, "Friendly and supportive.")

	if payload.SessionID != "agodesk:dev:1" {
		t.Fatalf("session_id = %q, want agodesk:dev:1", payload.SessionID)
	}
	if payload.Persona != "friend" || payload.IconKey != "friend" {
		t.Fatalf("persona payload = %+v, want friend persona and icon key", payload)
	}
	if payload.AvatarImageURL != "/img/personas/friend.png?v="+PersonaAssetVersion {
		t.Fatalf("avatar_image_url = %q", payload.AvatarImageURL)
	}
	if payload.IconURL != "/img/persona-icons/friend.png?v="+PersonaAssetVersion {
		t.Fatalf("icon_url = %q", payload.IconURL)
	}
	if payload.PersonaPrompt != "Friendly and supportive." {
		t.Fatalf("persona_prompt = %q, want trimmed prompt", payload.PersonaPrompt)
	}
}

func TestNewPersonaAssetsRequestBuildsClientEnvelope(t *testing.T) {
	env, err := NewPersonaAssetsRequest(" agodesk:dev:1 ")
	if err != nil {
		t.Fatalf("NewPersonaAssetsRequest: %v", err)
	}
	if env.Type != TypePersonaAssetsRequest {
		t.Fatalf("type = %q, want %q", env.Type, TypePersonaAssetsRequest)
	}
	var payload PersonaAssetsRequestPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal request payload: %v", err)
	}
	if payload.SessionID != "agodesk:dev:1" {
		t.Fatalf("session_id = %q, want trimmed session", payload.SessionID)
	}
}

func TestNewPersonaAssetsPayloadFallsBackForCustomPersona(t *testing.T) {
	payload := NewPersonaAssetsPayload("agodesk:dev:1", "lab-assistant", false, "  Custom lab tone.  ")

	if payload.Persona != "lab-assistant" {
		t.Fatalf("persona = %q, want original active persona name", payload.Persona)
	}
	if payload.IconKey != "custom" {
		t.Fatalf("icon_key = %q, want custom fallback", payload.IconKey)
	}
	if !strings.Contains(payload.AvatarImageURL, "/img/personas/custom.png") || !strings.Contains(payload.IconURL, "/img/persona-icons/custom.png") {
		t.Fatalf("custom asset urls not returned: %+v", payload)
	}
	if payload.PersonaPrompt != "Custom lab tone." {
		t.Fatalf("persona_prompt = %q, want trimmed custom prompt", payload.PersonaPrompt)
	}
}
