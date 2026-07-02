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

func TestProviderManagementProtocolPayloadsRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name string
		got  MessageType
		want string
	}{
		{"catalog list", TypeConfigProviderCatalogList, "config.provider.catalog.list"},
		{"catalog detail", TypeConfigProviderCatalogDetail, "config.provider.catalog.detail"},
		{"catalog", TypeConfigProviderCatalog, "config.provider.catalog"},
		{"providers list", TypeConfigProvidersList, "config.providers.list"},
		{"providers", TypeConfigProviders, "config.providers"},
		{"provider get", TypeConfigProviderGet, "config.provider.get"},
		{"provider", TypeConfigProvider, "config.provider"},
		{"provider upsert", TypeConfigProviderUpsert, "config.provider.upsert"},
		{"provider delete", TypeConfigProviderDelete, "config.provider.delete"},
		{"provider test", TypeConfigProviderTest, "config.provider.test"},
		{"provider test result", TypeConfigProviderTestResult, "config.provider.test_result"},
		{"oauth start", TypeConfigProviderOAuthStart, "config.provider.oauth.start"},
		{"oauth started", TypeConfigProviderOAuthStarted, "config.provider.oauth.started"},
		{"oauth complete", TypeConfigProviderOAuthComplete, "config.provider.oauth.complete"},
		{"oauth status request", TypeConfigProviderOAuthStatusRequest, "config.provider.oauth.status"},
		{"oauth status", TypeConfigProviderOAuthStatus, "config.provider.oauth.status"},
		{"oauth revoke", TypeConfigProviderOAuthRevoke, "config.provider.oauth.revoke"},
	} {
		if string(tt.got) != tt.want {
			t.Fatalf("%s message type = %q, want %q", tt.name, tt.got, tt.want)
		}
	}

	upsert, err := NewEnvelope(TypeConfigProviderUpsert, ConfigProviderUpsertPayload{
		SessionID: "agodesk:dev-1",
		Mode:      "update",
		Provider: ConfigProviderEntryPayload{
			ID:            "google",
			Name:          "Google",
			Type:          "google",
			BaseURL:       "https://generativelanguage.googleapis.com/v1beta/openai",
			Model:         "gemini-2.5-flash",
			AuthType:      "oauth2",
			OAuthAuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			OAuthTokenURL: "https://oauth2.googleapis.com/token",
			OAuthClientID: "client-id",
			OAuthScopes:   "openid profile email",
			Capabilities:  &ProviderCapabilitiesPayload{Auto: true, ToolCalling: true},
		},
		Secrets: ConfigProviderSecretOpsPayload{
			APIKey:            SecretOperationPayload{Op: "clear"},
			OAuthClientSecret: SecretOperationPayload{Op: "keep"},
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope upsert: %v", err)
	}
	var upsertPayload ConfigProviderUpsertPayload
	if err := json.Unmarshal(upsert.Payload, &upsertPayload); err != nil {
		t.Fatalf("unmarshal upsert: %v", err)
	}
	if upsertPayload.Provider.ID != "google" || upsertPayload.Secrets.APIKey.Op != "clear" || upsertPayload.Secrets.OAuthClientSecret.Op != "keep" {
		t.Fatalf("upsert payload = %+v", upsertPayload)
	}

	started, err := NewEnvelope(TypeConfigProviderOAuthStarted, ConfigProviderOAuthStartedPayload{
		SessionID:   "agodesk:dev-1",
		ProviderID:  "google",
		AuthURL:     "https://accounts.example/auth?state=state-1",
		Mode:        "agodesk_loopback",
		OAuthState:  "state-1",
		ExpiresAt:   "2026-06-25T12:10:00Z",
		RedirectURI: "http://127.0.0.1:8088/callback",
	})
	if err != nil {
		t.Fatalf("NewEnvelope oauth.started: %v", err)
	}
	var startedPayload ConfigProviderOAuthStartedPayload
	if err := json.Unmarshal(started.Payload, &startedPayload); err != nil {
		t.Fatalf("unmarshal oauth.started: %v", err)
	}
	if startedPayload.Mode != "agodesk_loopback" || startedPayload.OAuthState != "state-1" {
		t.Fatalf("oauth.started payload = %+v", startedPayload)
	}

	catalog, err := NewEnvelope(TypeConfigProviderCatalog, ConfigProviderCatalogPayload{
		SessionID: "agodesk:dev-1",
		Status:    "ok",
		Providers: []ProviderCatalogProviderPayload{{
			ID:               "google",
			AuraProviderType: "google",
			Name:             "Google",
			DefaultModel:     "gemini-2.5-flash",
			OAuthSetup: &ProviderCatalogOAuthSetupPayload{
				Flow:         "authorization_code_pkce",
				ClientID:     "public-client-id",
				CallbackPort: 8088,
				CallbackPath: "/oauth/callback",
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewEnvelope provider catalog: %v", err)
	}
	var catalogPayload ConfigProviderCatalogPayload
	if err := json.Unmarshal(catalog.Payload, &catalogPayload); err != nil {
		t.Fatalf("unmarshal provider catalog: %v", err)
	}
	if len(catalogPayload.Providers) != 1 || catalogPayload.Providers[0].OAuthSetup == nil || catalogPayload.Providers[0].OAuthSetup.CallbackPort != 8088 || catalogPayload.Providers[0].OAuthSetup.ClientID != "public-client-id" {
		t.Fatalf("catalog payload = %+v", catalogPayload)
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

func TestDefaultCapabilitiesIncludeComputerUseFeatures(t *testing.T) {
	if containsAgodeskTestString(DefaultCapabilities, "chat.streaming") {
		t.Fatalf("DefaultCapabilities must not advertise unimplemented chat.streaming: %v", DefaultCapabilities)
	}
	for _, want := range []string{
		"chat.agent_metadata",
		"chat.plan_updates",
		"chat.sessions",
		"chat.cancel",
		"chat.audio_events",
		"chat.media_events",
		"chat.voice_output_status",
		"integrations.webhosts",
		"system.warnings",
		"remote.desktop.capture",
		"remote.desktop.permission_request",
		"remote.desktop.input",
		"remote.desktop.discovery",
		"remote.desktop.ui_automation",
		"remote.desktop.browser",
	} {
		if !containsAgodeskTestString(DefaultCapabilities, want) {
			t.Fatalf("DefaultCapabilities missing %s: %v", want, DefaultCapabilities)
		}
	}
}

func TestMediaIntegrationsAndWarningsProtocolPayloadsRoundTrip(t *testing.T) {
	media, err := NewEnvelope(TypeChatMedia, ChatMediaPayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		RequestID:      "req-1",
		Kind:           "youtube_video",
		URL:            "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		EmbedURL:       "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ",
		VideoID:        "dQw4w9WgXcQ",
		Title:          "Demo",
		Provider:       "youtube",
		StartSeconds:   12,
		OpenMode:       "inline",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.media: %v", err)
	}
	var mediaPayload ChatMediaPayload
	if err := json.Unmarshal(media.Payload, &mediaPayload); err != nil {
		t.Fatalf("unmarshal chat.media: %v", err)
	}
	if mediaPayload.Kind != "youtube_video" || mediaPayload.VideoID != "dQw4w9WgXcQ" || mediaPayload.StartSeconds != 12 || mediaPayload.OpenMode != "inline" {
		t.Fatalf("media payload = %+v", mediaPayload)
	}

	webhosts, err := NewEnvelope(TypeIntegrationsWebhosts, IntegrationsWebhostsPayload{
		SessionID: "agodesk:dev-1",
		Status:    "ok",
		Webhosts: []WebhostIntegrationPayload{{
			ID:          "virtual_desktop",
			Name:        "Virtual Desktop",
			Description: "Browser-based virtual desktop",
			Status:      "running",
			URL:         "/desktop",
			Icon:        "expand",
		}},
	})
	if err != nil {
		t.Fatalf("NewEnvelope integrations.webhosts: %v", err)
	}
	var webhostsPayload IntegrationsWebhostsPayload
	if err := json.Unmarshal(webhosts.Payload, &webhostsPayload); err != nil {
		t.Fatalf("unmarshal integrations.webhosts: %v", err)
	}
	if webhostsPayload.SessionID != "agodesk:dev-1" || webhostsPayload.Status != "ok" || len(webhostsPayload.Webhosts) != 1 || webhostsPayload.Webhosts[0].ID != "virtual_desktop" {
		t.Fatalf("webhosts payload = %+v", webhostsPayload)
	}

	warnings, err := NewEnvelope(TypeSystemWarnings, SystemWarningsPayload{
		SessionID: "agodesk:dev-1",
		Warnings: []SystemWarningPayload{{
			ID:          "warn-1",
			Severity:    "warning",
			Title:       "Test warning",
			Description: "Something needs attention",
			Category:    "system",
			Timestamp:   "2026-06-07T12:00:00Z",
		}},
		Total:          1,
		Unacknowledged: 1,
	})
	if err != nil {
		t.Fatalf("NewEnvelope system.warnings: %v", err)
	}
	var warningsPayload SystemWarningsPayload
	if err := json.Unmarshal(warnings.Payload, &warningsPayload); err != nil {
		t.Fatalf("unmarshal system.warnings: %v", err)
	}
	if warningsPayload.Total != 1 || warningsPayload.Unacknowledged != 1 || warningsPayload.Warnings[0].ID != "warn-1" {
		t.Fatalf("warnings payload = %+v", warningsPayload)
	}
}

func TestChatSessionProtocolPayloadsCarryConversationID(t *testing.T) {
	msg, err := NewEnvelope(TypeChatMessage, ChatMessagePayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		Text:           "hello",
		Role:           "user",
		VoiceOutput:    true,
		Attachments: []ChatAttachmentItem{{
			AttachmentID: "att-1",
			Filename:     "diagram.png",
			MimeType:     "image/png",
			SizeBytes:    1234,
			Path:         "/api/agodesk/media/attachments/agodesk/sess-1/att-1/diagram.png",
			Kind:         "image",
		}},
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.message: %v", err)
	}
	var msgPayload ChatMessagePayload
	if err := json.Unmarshal(msg.Payload, &msgPayload); err != nil {
		t.Fatalf("unmarshal chat.message: %v", err)
	}
	if msgPayload.SessionID != "agodesk:dev-1" || msgPayload.ConversationID != "sess-1" || !msgPayload.VoiceOutput {
		t.Fatalf("chat.message payload = %+v", msgPayload)
	}
	if len(msgPayload.Attachments) != 1 || msgPayload.Attachments[0].AttachmentID != "att-1" || msgPayload.Attachments[0].Kind != "image" {
		t.Fatalf("chat.message attachments = %+v", msgPayload.Attachments)
	}

	resp, err := NewEnvelope(TypeChatResponse, ChatResponsePayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		RequestID:      "req-1",
		Text:           "hi",
		Role:           "assistant",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.response: %v", err)
	}
	var respPayload ChatResponsePayload
	if err := json.Unmarshal(resp.Payload, &respPayload); err != nil {
		t.Fatalf("unmarshal chat.response: %v", err)
	}
	if respPayload.ConversationID != "sess-1" {
		t.Fatalf("response conversation_id = %q, want sess-1", respPayload.ConversationID)
	}
}

func TestChatSessionManagementPayloadsRoundTrip(t *testing.T) {
	list, err := NewEnvelope(TypeChatSessionsList, ChatSessionsListPayload{SessionID: "agodesk:dev-1"})
	if err != nil {
		t.Fatalf("NewEnvelope sessions.list: %v", err)
	}
	if list.Type != TypeChatSessionsList {
		t.Fatalf("list type = %q", list.Type)
	}

	session := ChatSessionSummary{
		ID:           "sess-1",
		Preview:      "Hello",
		CreatedAt:    "2026-06-07T10:00:00Z",
		LastActiveAt: "2026-06-07T10:01:00Z",
		MessageCount: 2,
	}
	loaded, err := NewEnvelope(TypeChatSession, ChatSessionPayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		Session:        session,
		Messages: []ChatHistoryMessagePayload{
			{
				Role:      "user",
				Content:   "Hello",
				Timestamp: "2026-06-07T10:00:00Z",
				Attachments: []ChatAttachmentItem{{
					AttachmentID: "att-history",
					Filename:     "notes.txt",
					MimeType:     "text/plain",
					SizeBytes:    42,
					Path:         "/api/agodesk/media/attachments/agodesk/sess-1/att-history/notes.txt",
					Kind:         "text",
				}},
			},
			{Role: "assistant", Content: "Hi", Timestamp: "2026-06-07T10:01:00Z"},
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.session: %v", err)
	}
	var payload ChatSessionPayload
	if err := json.Unmarshal(loaded.Payload, &payload); err != nil {
		t.Fatalf("unmarshal chat.session: %v", err)
	}
	if payload.ConversationID != "sess-1" || payload.Session.ID != "sess-1" || len(payload.Messages) != 2 {
		t.Fatalf("chat.session payload = %+v", payload)
	}
	if len(payload.Messages[0].Attachments) != 1 || payload.Messages[0].Attachments[0].AttachmentID != "att-history" {
		t.Fatalf("chat.session attachments = %+v", payload.Messages[0].Attachments)
	}
}

func TestChatAttachmentProtocolPayloadsRoundTrip(t *testing.T) {
	prepared, err := NewEnvelope(TypeChatAttachmentPrepared, ChatAttachmentPreparedPayload{
		SessionID:    "agodesk:dev-1",
		PrepareID:    "prep-1",
		AttachmentID: "att-1",
		UploadURL:    "/api/agodesk/media/upload/att-1?agodesk_exp=1&agodesk_sig=sig",
		Method:       "POST",
		UploadField:  "file",
		ExpiresAt:    "2026-06-07T10:05:00Z",
		MaxBytes:     8 * 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("NewEnvelope prepared: %v", err)
	}
	var preparedPayload ChatAttachmentPreparedPayload
	if err := json.Unmarshal(prepared.Payload, &preparedPayload); err != nil {
		t.Fatalf("unmarshal prepared: %v", err)
	}
	if preparedPayload.PrepareID != "prep-1" || preparedPayload.AttachmentID != "att-1" || preparedPayload.Method != "POST" || preparedPayload.UploadField != "file" {
		t.Fatalf("prepared payload = %+v", preparedPayload)
	}

	accepted, err := NewEnvelope(TypeChatAttachmentAccepted, ChatAttachmentAcceptedPayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		Attachments: []ChatAttachmentItem{{
			AttachmentID: "att-1",
			Filename:     "diagram.png",
			MimeType:     "image/png",
			SizeBytes:    1234,
			Path:         "/api/agodesk/media/attachments/agodesk/sess-1/att-1/diagram.png",
			Kind:         "image",
		}},
	})
	if err != nil {
		t.Fatalf("NewEnvelope accepted: %v", err)
	}
	var acceptedPayload ChatAttachmentAcceptedPayload
	if err := json.Unmarshal(accepted.Payload, &acceptedPayload); err != nil {
		t.Fatalf("unmarshal accepted: %v", err)
	}
	if acceptedPayload.ConversationID != "sess-1" || len(acceptedPayload.Attachments) != 1 || acceptedPayload.Attachments[0].AttachmentID != "att-1" {
		t.Fatalf("accepted payload = %+v", acceptedPayload)
	}
}

func TestChatCancelAndAudioPayloadsRoundTrip(t *testing.T) {
	cancelled, err := NewEnvelope(TypeChatCancelled, ChatCancelledPayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		RequestID:      "req-1",
		Status:         "cancelled",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.cancelled: %v", err)
	}
	var cancelPayload ChatCancelledPayload
	if err := json.Unmarshal(cancelled.Payload, &cancelPayload); err != nil {
		t.Fatalf("unmarshal chat.cancelled: %v", err)
	}
	if cancelPayload.ConversationID != "sess-1" || cancelPayload.RequestID != "req-1" || cancelPayload.Status != "cancelled" {
		t.Fatalf("cancelled payload = %+v", cancelPayload)
	}

	audio, err := NewEnvelope(TypeChatAudio, ChatAudioPayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		RequestID:      "req-1",
		Path:           "/tts/answer.mp3",
		Title:          "TTS Audio",
		MimeType:       "audio/mpeg",
		Filename:       "answer.mp3",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.audio: %v", err)
	}
	var audioPayload ChatAudioPayload
	if err := json.Unmarshal(audio.Payload, &audioPayload); err != nil {
		t.Fatalf("unmarshal chat.audio: %v", err)
	}
	if audioPayload.Path != "/tts/answer.mp3" || audioPayload.ConversationID != "sess-1" {
		t.Fatalf("audio payload = %+v", audioPayload)
	}
}

func TestChatVoiceOutputStatusPayloadRoundTrip(t *testing.T) {
	env, err := NewEnvelope(TypeChatVoiceOutputStatus, ChatVoiceOutputStatusPayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		SpeakerMode:    false,
		Mode:           "off",
		Reason:         "user_disabled",
		Status:         "ok",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.voice_output.status: %v", err)
	}
	var payload ChatVoiceOutputStatusPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal chat.voice_output.status: %v", err)
	}
	if payload.SessionID != "agodesk:dev-1" || payload.ConversationID != "sess-1" || payload.SpeakerMode || payload.Mode != "off" || payload.Reason != "user_disabled" || payload.Status != "ok" {
		t.Fatalf("voice output status payload = %+v", payload)
	}
}

func TestChatPlanUpdatePayloadRoundTripsPlanAndNull(t *testing.T) {
	env, err := NewEnvelope(TypeChatPlanUpdate, ChatPlanUpdatePayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		RequestID:      "req-1",
		Plan:           json.RawMessage(`{"title":"Deploy site","tasks":[{"title":"Build","status":"in_progress"}],"progress_pct":40}`),
	})
	if err != nil {
		t.Fatalf("NewEnvelope plan update: %v", err)
	}

	var payload ChatPlanUpdatePayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal chat plan update payload: %v", err)
	}
	if payload.SessionID != "agodesk:dev-1" || payload.ConversationID != "sess-1" || payload.RequestID != "req-1" {
		t.Fatalf("payload ids = %+v", payload)
	}
	var plan map[string]interface{}
	if err := json.Unmarshal(payload.Plan, &plan); err != nil {
		t.Fatalf("unmarshal plan raw json: %v", err)
	}
	if plan["title"] != "Deploy site" {
		t.Fatalf("plan title = %q, want Deploy site", plan["title"])
	}

	nullEnv, err := NewEnvelope(TypeChatPlanUpdate, ChatPlanUpdatePayload{
		SessionID: "agodesk:dev-1",
		RequestID: "req-2",
		Plan:      json.RawMessage(`null`),
	})
	if err != nil {
		t.Fatalf("NewEnvelope null plan update: %v", err)
	}
	var nullPayload ChatPlanUpdatePayload
	if err := json.Unmarshal(nullEnv.Payload, &nullPayload); err != nil {
		t.Fatalf("unmarshal null chat plan update payload: %v", err)
	}
	if string(nullPayload.Plan) != "null" {
		t.Fatalf("null plan raw = %s, want null", string(nullPayload.Plan))
	}
}

func TestChatChunkPayloadMetadataIsOptional(t *testing.T) {
	env, err := NewEnvelope(TypeChatChunk, ChatChunkPayload{
		SessionID:      "agodesk:dev-1",
		ConversationID: "sess-1",
		RequestID:      "req-1",
		Delta:          "hello",
		Sequence:       1,
		Metadata: map[string]interface{}{
			"agent_mood": map[string]interface{}{"mood": "focused"},
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope chunk: %v", err)
	}

	var payload ChatChunkPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal chunk payload: %v", err)
	}
	if payload.Metadata == nil {
		t.Fatal("chunk metadata missing")
	}
	if payload.ConversationID != "sess-1" {
		t.Fatalf("chunk conversation_id = %q, want sess-1", payload.ConversationID)
	}
	if _, ok := payload.Metadata["agent_mood"].(map[string]interface{}); !ok {
		t.Fatalf("agent_mood metadata = %#v", payload.Metadata["agent_mood"])
	}
}

func TestSessionAcceptedPayloadCarriesAdvertisedCapabilities(t *testing.T) {
	env, err := NewEnvelope(TypeSessionAccepted, SessionAcceptedPayload{
		SessionID:              "agodesk:dev-1",
		DeviceID:               "dev-1",
		Approved:               true,
		Capabilities:           []string{"chat.full_response", "remote.desktop.capture", "remote.desktop.discovery"},
		AdvertisedCapabilities: []string{"remote.desktop.capture", "remote.desktop.discovery"},
		AttachmentLimits: &AttachmentLimitsPayload{
			MaxFileBytes:  8 * 1024 * 1024,
			MaxFiles:      5,
			MaxTotalBytes: 24 * 1024 * 1024,
			AllowedMime:   []string{"image/*", "text/*", "application/pdf"},
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(env.Payload, &raw); err != nil {
		t.Fatalf("unmarshal session.accepted payload: %v", err)
	}
	advertised, ok := raw["advertised_capabilities"].([]interface{})
	if !ok {
		t.Fatalf("advertised_capabilities missing from JSON payload: %#v", raw)
	}
	if len(advertised) != 2 || advertised[0] != "remote.desktop.capture" || advertised[1] != "remote.desktop.discovery" {
		t.Fatalf("advertised_capabilities = %#v", advertised)
	}
	limits, ok := raw["attachment_limits"].(map[string]interface{})
	if !ok {
		t.Fatalf("attachment_limits missing from JSON payload: %#v", raw)
	}
	if limits["max_files"] != float64(5) || limits["max_file_bytes"] != float64(8*1024*1024) {
		t.Fatalf("attachment_limits = %#v", limits)
	}
}

func TestNegotiateCapabilitiesIntersectsClientAndServerCapabilities(t *testing.T) {
	got := NegotiateCapabilities(
		[]string{"chat.full_response", "remote.desktop.capture", "remote.desktop.browser", "unknown.cap"},
		[]string{"remote.desktop.capture", "chat.full_response", "persona.assets"},
	)
	want := []string{"chat.full_response", "remote.desktop.capture"}
	if len(got) != len(want) {
		t.Fatalf("negotiated capabilities = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("negotiated capabilities = %v, want %v", got, want)
		}
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

func containsAgodeskTestString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
