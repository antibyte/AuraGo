package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/agodesk"
	"aurago/internal/config"
	"aurago/internal/remote"
	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

func TestAgodeskWebSocketSendsConnectedAndPong(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()

	connected := readAgodeskTestEnvelope(t, conn)
	if connected.Type != agodesk.TypeSystemConnected {
		t.Fatalf("first message type = %q, want %q", connected.Type, agodesk.TypeSystemConnected)
	}
	var payload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &payload)
	if payload.ProtocolVersion != agodesk.ProtocolVersion {
		t.Fatalf("protocol_version = %q, want %q", payload.ProtocolVersion, agodesk.ProtocolVersion)
	}
	if !payload.AuthRequired || !payload.PairingRequired {
		t.Fatalf("auth flags = auth:%v pairing:%v, want both true", payload.AuthRequired, payload.PairingRequired)
	}
	if payload.SessionID == "" {
		t.Fatal("system.connected did not include a temporary session_id")
	}

	ping, err := agodesk.NewEnvelope(agodesk.TypeSystemPing, map[string]string{})
	if err != nil {
		t.Fatalf("NewEnvelope ping: %v", err)
	}
	if err := conn.WriteJSON(ping); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	pong := readAgodeskTestEnvelope(t, conn)
	if pong.Type != agodesk.TypeSystemPong {
		t.Fatalf("pong type = %q, want %q", pong.Type, agodesk.TypeSystemPong)
	}
}

func TestAgodeskWebSocketRouteBypassesSessionAuthForPairingHandshake(t *testing.T) {
	if !isAuthBypassed("/api/agodesk/ws") {
		t.Fatal("/api/agodesk/ws must bypass session auth so agodesk can perform its own pairing handshake")
	}
}

func TestAgodeskWebSocketInvalidJSONReturnsChatError(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"id":`)); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatError {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatError)
	}
	var payload agodesk.ChatErrorPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.Code != agodesk.ErrorInvalidMessage {
		t.Fatalf("error code = %q, want %q", payload.Code, agodesk.ErrorInvalidMessage)
	}
}

func TestAgodeskInsecureLoopbackDevRequiresLoopbackRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/agodesk/ws?insecure_loopback=1", nil)
	req.RemoteAddr = "203.0.113.10:5555"
	if isExplicitAgodeskLoopbackDev(req) {
		t.Fatal("insecure_loopback=1 must not enable dev mode for non-loopback clients")
	}
}

func TestAgodeskWebSocketRequiresPairingForNormalChat(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: "agodesk-temp",
		Text:      "hello",
		Role:      "user",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write chat: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatError {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatError)
	}
	var payload agodesk.ChatErrorPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.Code != agodesk.ErrorPairingRequired {
		t.Fatalf("error code = %q, want %q", payload.Code, agodesk.ErrorPairingRequired)
	}
	if payload.RequestID != msg.ID {
		t.Fatalf("request_id = %q, want %q", payload.RequestID, msg.ID)
	}
}

func TestAgodeskWebSocketAllowsExplicitLoopbackDevChat(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, sessionID, message string) (string, error) {
		if !strings.HasPrefix(sessionID, "agodesk:dev:") {
			t.Fatalf("sessionID = %q, want agodesk dev session", sessionID)
		}
		if message != "hello" {
			t.Fatalf("message = %q, want hello", message)
		}
		return "agent says hello", nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)
	if connectedPayload.AuthRequired || connectedPayload.PairingRequired {
		t.Fatalf("dev auth flags = auth:%v pairing:%v, want false/false", connectedPayload.AuthRequired, connectedPayload.PairingRequired)
	}

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: connectedPayload.SessionID,
		Text:      "hello",
		Role:      "user",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write chat: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatResponse {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatResponse)
	}
	var payload agodesk.ChatResponsePayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.RequestID != msg.ID || payload.Text != "agent says hello" || payload.Role != "assistant" {
		t.Fatalf("chat response payload = %+v", payload)
	}
}

func TestAgodeskWebSocketRejectsMismatchedChatSessionID(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	oldRunner := agodeskAgentChatRunner
	runnerCalled := make(chan struct{}, 1)
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, sessionID, message string) (string, error) {
		runnerCalled <- struct{}{}
		return "runner should not run", nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: connectedPayload.SessionID + "-stale",
		Text:      "hello",
		Role:      "user",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write chat: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	select {
	case <-runnerCalled:
		t.Fatal("agodesk runner should not be called for mismatched chat session")
	default:
	}
	if resp.Type != agodesk.TypeChatError {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatError)
	}
	var payload agodesk.ChatErrorPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.Code != agodesk.ErrorSessionNotFound {
		t.Fatalf("error code = %q, want %q", payload.Code, agodesk.ErrorSessionNotFound)
	}
	if payload.RequestID != msg.ID {
		t.Fatalf("request_id = %q, want %q", payload.RequestID, msg.ID)
	}
}

func TestAgodeskWebSocketPongsWhileChatMessageInFlight(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	oldRunner := agodeskAgentChatRunner
	runnerStarted := make(chan struct{})
	releaseRunner := make(chan struct{})
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, sessionID, message string) (string, error) {
		close(runnerStarted)
		<-releaseRunner
		return "slow answer", nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: connectedPayload.SessionID,
		Text:      "slow hello",
		Role:      "user",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write chat: %v", err)
	}
	select {
	case <-runnerStarted:
	case <-time.After(time.Second):
		t.Fatal("agodesk chat runner did not start")
	}

	ping, err := agodesk.NewEnvelope(agodesk.TypeSystemPing, map[string]string{})
	if err != nil {
		t.Fatalf("NewEnvelope ping: %v", err)
	}
	if err := conn.WriteJSON(ping); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	pong := readAgodeskTestEnvelope(t, conn)
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
	}
	if pong.Type != agodesk.TypeSystemPong {
		t.Fatalf("response while chat in flight = %q, want %q", pong.Type, agodesk.TypeSystemPong)
	}

	close(releaseRunner)
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatResponse {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatResponse)
	}
	var payload agodesk.ChatResponsePayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.RequestID != msg.ID || payload.Text != "slow answer" {
		t.Fatalf("chat response payload = %+v", payload)
	}
}

func TestAgodeskPersonaAssetsRequestReturnsActivePersonaAvatarIconAndPrompt(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.Cfg.Personality.CorePersonality = "punk"
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()

	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)
	if !agodeskTestContainsString(connectedPayload.Capabilities, "persona.assets") {
		t.Fatalf("system.connected capabilities = %v, want persona.assets", connectedPayload.Capabilities)
	}

	req, err := agodesk.NewEnvelope(agodesk.TypePersonaAssetsRequest, agodesk.PersonaAssetsRequestPayload{
		SessionID: connectedPayload.SessionID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope persona assets: %v", err)
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write persona.assets.request: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypePersonaAssets {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypePersonaAssets)
	}
	var payload agodesk.PersonaAssetsPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.SessionID != connectedPayload.SessionID || payload.Persona != "punk" || payload.IconKey != "punk" {
		t.Fatalf("persona assets payload = %+v", payload)
	}
	if payload.AvatarImageURL != "/img/personas/punk.png?v="+agodesk.PersonaAssetVersion {
		t.Fatalf("avatar_image_url = %q", payload.AvatarImageURL)
	}
	if payload.IconURL != "/img/persona-icons/punk.png?v="+agodesk.PersonaAssetVersion {
		t.Fatalf("icon_url = %q", payload.IconURL)
	}
	if !strings.Contains(payload.PersonaPrompt, "# Core Personality: Punk") {
		t.Fatalf("persona_prompt missing punk markdown body: %q", payload.PersonaPrompt)
	}
	if strings.Contains(payload.PersonaPrompt, "anchor_traits:") {
		t.Fatalf("persona_prompt should not include YAML front matter: %q", payload.PersonaPrompt)
	}
}

func TestAgodeskSessionStartWithPairingTokenCreatesRemoteDevice(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	token := "remote_test_pairing_token"
	enrollID, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256(token),
		DeviceName: "desktop-pc",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)

	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion: "0.1.0",
		PairingToken:  token,
		Host: agodesk.SessionStartHost{
			Hostname: "DESKTOP-PC",
			OS:       "windows",
			Arch:     "amd64",
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write session.start: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeSessionAccepted {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeSessionAccepted)
	}
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, resp, &accepted)
	if !accepted.Approved || accepted.DeviceID == "" || accepted.SessionID != "agodesk:"+accepted.DeviceID {
		t.Fatalf("accepted payload = %+v", accepted)
	}
	if accepted.SharedKey == "" {
		t.Fatal("fresh pairing did not return a shared key for client secure storage")
	}

	device, err := remote.GetDevice(s.RemoteHub.DB(), accepted.DeviceID)
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if device.Name != "desktop-pc" || device.Hostname != "DESKTOP-PC" || device.OS != "windows" || device.Arch != "amd64" {
		t.Fatalf("device record = %+v", device)
	}
	if !agodeskTestContainsString(device.Tags, "agodesk") {
		t.Fatalf("device tags = %v, want agodesk", device.Tags)
	}
	secret, err := s.Vault.ReadSecret("remote_shared_key_" + accepted.DeviceID)
	if err != nil {
		t.Fatalf("ReadSecret shared key: %v", err)
	}
	if secret != accepted.SharedKey {
		t.Fatal("vault shared key does not match accepted payload")
	}
	enrollment, err := remote.GetEnrollmentByTokenHash(s.RemoteHub.DB(), hashSHA256(token))
	if err != nil {
		t.Fatalf("GetEnrollmentByTokenHash: %v", err)
	}
	if !enrollment.Used || enrollment.UsedByDevice != accepted.DeviceID || enrollment.ID != enrollID {
		t.Fatalf("enrollment after pairing = %+v", enrollment)
	}
}

func TestAgodeskSessionAcceptedAdvertisesNegotiatedClientCapabilities(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	token := "agodesk-advertised-capabilities-token"
	if _, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256(token),
		DeviceName: "desktop-pc",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)
	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion: "0.1.0",
		PairingToken:  token,
		ClientCapabilities: []string{
			"chat.full_response",
			"remote.desktop.capture",
			"remote.desktop.discovery",
			"remote.desktop.browser",
		},
		Host: agodesk.SessionStartHost{Hostname: "DESKTOP-PC", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write session.start: %v", err)
	}

	resp := readAgodeskTestEnvelope(t, conn)
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, resp, &accepted)
	for _, want := range []string{"chat.full_response", "remote.desktop.capture", "remote.desktop.discovery", "remote.desktop.browser"} {
		if !agodeskTestContainsString(accepted.AdvertisedCapabilities, want) {
			t.Fatalf("advertised_capabilities = %v, missing %s", accepted.AdvertisedCapabilities, want)
		}
	}
	if agodeskTestContainsString(accepted.AdvertisedCapabilities, "remote.desktop.input") {
		t.Fatalf("advertised_capabilities should not include omitted client capability: %v", accepted.AdvertisedCapabilities)
	}
	if len(accepted.Capabilities) == 0 {
		t.Fatal("legacy capabilities should still be populated for old clients")
	}
}

func TestAgodeskSessionStartReconnectRequiresSharedKeyProof(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	sharedKey := "0123456789abcdef0123456789abcdef"
	deviceID, err := remote.CreateDevice(s.RemoteHub.DB(), remote.DeviceRecord{
		Name:          "desktop-pc",
		Hostname:      "DESKTOP-PC",
		OS:            "windows",
		Arch:          "amd64",
		Status:        "approved",
		SharedKeyHash: hashSHA256(sharedKey),
		Tags:          []string{"agodesk"},
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if err := s.Vault.WriteSecret("remote_shared_key_"+deviceID, sharedKey); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)

	startID := "session-start-reconnect"
	proof, err := agodesk.NewSharedKeyProof(sharedKey, startID, deviceID, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewSharedKeyProof: %v", err)
	}
	startPayload := agodesk.SessionStartPayload{
		ClientVersion:  "0.1.0",
		DeviceID:       deviceID,
		SharedKeyProof: &proof,
		Host: agodesk.SessionStartHost{
			Hostname: "DESKTOP-PC",
			OS:       "windows",
			Arch:     "amd64",
		},
	}
	rawPayload, _ := json.Marshal(startPayload)
	start := agodesk.Envelope{
		ID:        startID,
		Type:      agodesk.TypeSessionStart,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:   rawPayload,
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write reconnect session.start: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeSessionAccepted {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeSessionAccepted)
	}
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, resp, &accepted)
	if accepted.DeviceID != deviceID || accepted.SessionID != "agodesk:"+deviceID || !accepted.Approved {
		t.Fatalf("accepted reconnect payload = %+v", accepted)
	}
	if accepted.SharedKey != "" {
		t.Fatal("reconnect must not echo the existing shared key")
	}
}

func TestAgodeskSessionStartReconnectAllowsOfflinePairedDevice(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	sharedKey := "fedcba9876543210fedcba9876543210"
	deviceID, err := remote.CreateDevice(s.RemoteHub.DB(), remote.DeviceRecord{
		Name:          "desktop-pc",
		Hostname:      "DESKTOP-PC",
		OS:            "windows",
		Arch:          "amd64",
		Status:        "offline",
		SharedKeyHash: hashSHA256(sharedKey),
		Tags:          []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if err := s.Vault.WriteSecret("remote_shared_key_"+deviceID, sharedKey); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)

	startID := "session-start-offline-reconnect"
	proof, err := agodesk.NewSharedKeyProof(sharedKey, startID, deviceID, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewSharedKeyProof: %v", err)
	}
	startPayload := agodesk.SessionStartPayload{
		ClientVersion:  "0.1.0",
		DeviceID:       deviceID,
		SharedKeyProof: &proof,
		Host: agodesk.SessionStartHost{
			Hostname: "DESKTOP-PC",
			OS:       "windows",
			Arch:     "amd64",
		},
	}
	rawPayload, _ := json.Marshal(startPayload)
	start := agodesk.Envelope{
		ID:        startID,
		Type:      agodesk.TypeSessionStart,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:   rawPayload,
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write reconnect session.start: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeSessionAccepted {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeSessionAccepted)
	}
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, resp, &accepted)
	if accepted.DeviceID != deviceID || accepted.SessionID != "agodesk:"+deviceID || !accepted.Approved {
		t.Fatalf("accepted reconnect payload = %+v", accepted)
	}
}

func TestAgodeskDesktopCommandRoutesThroughPairedSocket(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	token := "desktop-command-pairing-token"
	if _, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256(token),
		DeviceName: "desktop-pc",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)
	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion:      "0.1.0",
		PairingToken:       token,
		ClientCapabilities: []string{"remote.desktop.capture", "remote.desktop.permission_request", "remote.desktop.input"},
		Host:               agodesk.SessionStartHost{Hostname: "DESKTOP-PC", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write session.start: %v", err)
	}
	acceptedEnvelope := readAgodeskTestEnvelope(t, conn)
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, acceptedEnvelope, &accepted)
	if accepted.DeviceID == "" {
		t.Fatalf("accepted payload missing device id: %+v", accepted)
	}
	if !s.RemoteHub.IsConnected(accepted.DeviceID) {
		t.Fatal("paired agodesk device should be connected through RemoteHub")
	}

	type commandResult struct {
		result remote.ResultPayload
		err    error
	}
	resultCh := make(chan commandResult, 1)
	go func() {
		result, err := s.RemoteHub.SendCommand(accepted.DeviceID, remote.CommandPayload{
			Operation: remote.OpDesktopScreenshot,
			Args:      map[string]interface{}{"display_id": "display-0", "format": "png"},
		}, 2*time.Second)
		resultCh <- commandResult{result: result, err: err}
	}()

	cmdEnvelope := readAgodeskTestEnvelope(t, conn)
	if cmdEnvelope.Type != agodesk.TypeDesktopCommand {
		t.Fatalf("command envelope type = %q, want %q", cmdEnvelope.Type, agodesk.TypeDesktopCommand)
	}
	var cmd agodesk.DesktopCommandPayload
	decodeAgodeskTestPayload(t, cmdEnvelope, &cmd)
	if cmd.CommandID == "" || cmd.Operation != remote.OpDesktopScreenshot {
		t.Fatalf("desktop command payload = %+v", cmd)
	}
	resultEnvelope, err := agodesk.NewEnvelope(agodesk.TypeDesktopResult, agodesk.DesktopResultPayload{
		CommandID: cmd.CommandID,
		OK:        true,
		Data: map[string]interface{}{
			"source":     "display",
			"display_id": "display-0",
			"format":     "png",
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope desktop.result: %v", err)
	}
	if err := conn.WriteJSON(resultEnvelope); err != nil {
		t.Fatalf("write desktop.result: %v", err)
	}

	select {
	case got := <-resultCh:
		if got.err != nil {
			t.Fatalf("SendCommand returned error: %v", got.err)
		}
		if got.result.Status != "ok" || !strings.Contains(got.result.Output, `"display_id":"display-0"`) {
			t.Fatalf("desktop command result = %+v", got.result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for routed desktop result")
	}

	unknown, err := agodesk.NewEnvelope(agodesk.TypeDesktopResult, agodesk.DesktopResultPayload{
		CommandID: "unknown-command",
		OK:        true,
	})
	if err != nil {
		t.Fatalf("NewEnvelope unknown desktop.result: %v", err)
	}
	if err := conn.WriteJSON(unknown); err != nil {
		t.Fatalf("write unknown desktop.result: %v", err)
	}
	ping, err := agodesk.NewEnvelope(agodesk.TypeSystemPing, map[string]string{})
	if err != nil {
		t.Fatalf("NewEnvelope ping: %v", err)
	}
	if err := conn.WriteJSON(ping); err != nil {
		t.Fatalf("write ping after unknown result: %v", err)
	}
	pong := readAgodeskTestEnvelope(t, conn)
	if pong.Type != agodesk.TypeSystemPong {
		t.Fatalf("response after unknown desktop.result = %q, want pong", pong.Type)
	}
}

func TestAgodeskDesktopCommandWithoutClientCapabilityFailsFast(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	token := "desktop-command-no-capability-token"
	if _, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256(token),
		DeviceName: "chat-only-pc",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)
	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion:      "0.1.0",
		PairingToken:       token,
		ClientCapabilities: []string{"chat.full_response"},
		Host:               agodesk.SessionStartHost{Hostname: "CHAT-PC", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write session.start: %v", err)
	}
	acceptedEnvelope := readAgodeskTestEnvelope(t, conn)
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, acceptedEnvelope, &accepted)
	if accepted.DeviceID == "" {
		t.Fatalf("accepted payload missing device id: %+v", accepted)
	}

	result, err := s.RemoteHub.SendCommand(accepted.DeviceID, remote.CommandPayload{
		Operation: remote.OpDesktopScreenshot,
		Args:      map[string]interface{}{"format": "png"},
	}, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("SendCommand returned transport error: %v", err)
	}
	if result.Status != "error" || !strings.Contains(result.Error, agodesk.ErrorUnsupportedCapability) {
		t.Fatalf("desktop command result = %+v, want unsupported capability error", result)
	}

	permission, err := s.RemoteHub.SendCommand(accepted.DeviceID, remote.CommandPayload{
		Operation: remote.OpDesktopPermissionRequest,
	}, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("SendCommand permission request returned transport error: %v", err)
	}
	if permission.Status != "error" || !strings.Contains(permission.Error, "remote.desktop.permission_request") {
		t.Fatalf("permission command result = %+v, want permission capability error", permission)
	}
}

func TestAgodeskFileCommandsRequireClientCapabilities(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	token := "agodesk-file-no-capability-token"
	if _, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256(token),
		DeviceName: "chat-only-file-pc",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)
	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion:      "0.1.0",
		PairingToken:       token,
		ClientCapabilities: []string{"chat.full_response"},
		Host:               agodesk.SessionStartHost{Hostname: "FILES-PC", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write session.start: %v", err)
	}
	acceptedEnvelope := readAgodeskTestEnvelope(t, conn)
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, acceptedEnvelope, &accepted)

	for _, tc := range []struct {
		op          string
		wantCap     string
		description string
	}{
		{op: remote.OpFileRead, wantCap: "remote.files.read", description: "file_read"},
		{op: remote.OpFileList, wantCap: "remote.files.read", description: "file_list"},
		{op: remote.OpFileWrite, wantCap: "remote.files.write", description: "file_write"},
	} {
		result, err := s.RemoteHub.SendCommand(accepted.DeviceID, remote.CommandPayload{
			Operation: tc.op,
			Args:      map[string]interface{}{"path": "src/main.go"},
		}, 25*time.Millisecond)
		if err != nil {
			t.Fatalf("%s SendCommand returned transport error: %v", tc.description, err)
		}
		if result.Status != "error" || !strings.Contains(result.Error, agodesk.ErrorUnsupportedCapability) || !strings.Contains(result.Error, tc.wantCap) {
			t.Fatalf("%s result = %+v, want unsupported capability %s", tc.description, result, tc.wantCap)
		}
	}
}

func TestAgodeskComputerUseCommandsRequireClientCapabilities(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	conn, cleanup, accepted := pairAgodeskTestClient(t, s, "agodesk-computer-use-no-cap-token", []string{"chat.full_response"})
	defer cleanup()
	_ = conn

	for _, tc := range []struct {
		op      string
		wantCap string
	}{
		{op: remote.OpDesktopListDisplays, wantCap: "remote.desktop.discovery"},
		{op: remote.OpDesktopListWindows, wantCap: "remote.desktop.discovery"},
		{op: remote.OpDesktopActiveWindow, wantCap: "remote.desktop.discovery"},
		{op: remote.OpDesktopHostInfo, wantCap: "remote.desktop.discovery"},
		{op: remote.OpDesktopUITree, wantCap: "remote.desktop.ui_automation"},
		{op: remote.OpDesktopUIAction, wantCap: "remote.desktop.ui_automation"},
		{op: remote.OpDesktopBrowserConnect, wantCap: "remote.desktop.browser"},
		{op: remote.OpDesktopBrowserSnapshot, wantCap: "remote.desktop.browser"},
		{op: remote.OpDesktopBrowserAction, wantCap: "remote.desktop.browser"},
		{op: remote.OpDesktopBrowserDisconnect, wantCap: "remote.desktop.browser"},
	} {
		result, err := s.RemoteHub.SendCommand(accepted.DeviceID, remote.CommandPayload{
			Operation: tc.op,
			Args:      map[string]interface{}{"window_id": "win-1", "action": "click"},
		}, 25*time.Millisecond)
		if err != nil {
			t.Fatalf("%s SendCommand returned transport error: %v", tc.op, err)
		}
		if result.Status != "error" || !strings.Contains(result.Error, agodesk.ErrorUnsupportedCapability) || !strings.Contains(result.Error, tc.wantCap) {
			t.Fatalf("%s result = %+v, want unsupported capability %s", tc.op, result, tc.wantCap)
		}
	}
}

func TestAgodeskFileDesktopResultsNormalizeRemoteOutput(t *testing.T) {
	success := true
	read := agodeskDesktopResultToRemoteResult("read-1", agodesk.DesktopResultPayload{
		CommandID: "read-1",
		OK:        true,
		Data: map[string]interface{}{
			"content":  "package main\n",
			"encoding": "utf-8",
		},
	})
	if read.Status != "ok" || read.Output != "package main\n" {
		t.Fatalf("file_read normalized result = %+v", read)
	}

	list := agodeskDesktopResultToRemoteResult("list-1", agodesk.DesktopResultPayload{
		CommandID: "list-1",
		OK:        true,
		Data: map[string]interface{}{
			"files": []interface{}{
				map[string]interface{}{"name": "main.go", "type": "file"},
			},
		},
	})
	if list.Status != "ok" || !strings.Contains(list.Output, `"name":"main.go"`) || strings.Contains(list.Output, `"files"`) {
		t.Fatalf("file_list normalized result = %+v", list)
	}

	activeWindow := agodeskDesktopResultToRemoteResult("active-1", agodesk.DesktopResultPayload{
		CommandID: "active-1",
		Success:   &success,
		Status:    "ok",
		Data: map[string]interface{}{
			"id":           "win-1",
			"title":        "Notepad",
			"display_id":   "display-0",
			"process_name": "notepad.exe",
		},
	})
	if activeWindow.Status != "ok" || !strings.Contains(activeWindow.Output, `"id":"win-1"`) || !strings.Contains(activeWindow.Output, `"process_name":"notepad.exe"`) {
		t.Fatalf("computer-use normalized result = %+v", activeWindow)
	}
}

func TestAgodeskComputerUseDesktopResultAcceptsSuccessField(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	conn, cleanup, accepted := pairAgodeskTestClient(t, s, "agodesk-computer-use-success-token", []string{"remote.desktop.discovery"})
	defer cleanup()

	type commandResult struct {
		result remote.ResultPayload
		err    error
	}
	resultCh := make(chan commandResult, 1)
	go func() {
		result, err := s.RemoteHub.SendCommand(accepted.DeviceID, remote.CommandPayload{
			Operation: remote.OpDesktopActiveWindow,
		}, 2*time.Second)
		resultCh <- commandResult{result: result, err: err}
	}()

	cmdEnvelope := readAgodeskTestEnvelope(t, conn)
	if cmdEnvelope.Type != agodesk.TypeDesktopCommand {
		t.Fatalf("command envelope type = %q, want %q", cmdEnvelope.Type, agodesk.TypeDesktopCommand)
	}
	var cmd agodesk.DesktopCommandPayload
	decodeAgodeskTestPayload(t, cmdEnvelope, &cmd)
	resultEnvelope, err := agodesk.NewEnvelope(agodesk.TypeDesktopResult, map[string]interface{}{
		"command_id": cmd.CommandID,
		"success":    true,
		"status":     "ok",
		"data": map[string]interface{}{
			"id":           "win-123",
			"title":        "Editor",
			"process_name": "editor.exe",
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope desktop.result: %v", err)
	}
	if err := conn.WriteJSON(resultEnvelope); err != nil {
		t.Fatalf("write desktop.result: %v", err)
	}

	select {
	case got := <-resultCh:
		if got.err != nil {
			t.Fatalf("SendCommand returned error: %v", got.err)
		}
		if got.result.Status != "ok" || !strings.Contains(got.result.Output, `"id":"win-123"`) {
			t.Fatalf("desktop command result = %+v", got.result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for routed desktop result")
	}
}

func TestAgodeskChatMessageCommandSendsServerPushResponse(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	token := "agodesk-chat-push-token"
	if _, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256(token),
		DeviceName: "agodesk-chat",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)
	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion:      "test-client",
		PairingToken:       token,
		ClientCapabilities: []string{"chat.full_response"},
		Host:               agodesk.SessionStartHost{Hostname: "AGOCHAT-PC", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write session.start: %v", err)
	}
	acceptedEnvelope := readAgodeskTestEnvelope(t, conn)
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, acceptedEnvelope, &accepted)

	result, err := s.RemoteHub.SendCommand(accepted.DeviceID, remote.CommandPayload{
		CommandID: "chat-push-1",
		Operation: "agodesk_chat_message",
		Args: map[string]interface{}{
			"message": "mission update",
		},
	}, time.Second)
	if err != nil {
		t.Fatalf("send chat command: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("result = %+v, want ok", result)
	}

	push := readAgodeskTestEnvelope(t, conn)
	if push.Type != agodesk.TypeChatResponse {
		t.Fatalf("push type = %q, want %q", push.Type, agodesk.TypeChatResponse)
	}
	var payload agodesk.ChatResponsePayload
	decodeAgodeskTestPayload(t, push, &payload)
	if payload.SessionID != accepted.SessionID || payload.RequestID != "chat-push-1" || payload.Text != "mission update" || payload.Role != "assistant" {
		t.Fatalf("push payload = %+v", payload)
	}
	if payload.Metadata["server_push"] != true {
		t.Fatalf("push metadata = %#v, want server_push=true", payload.Metadata)
	}
}

func TestAgodeskDesktopResultAcceptsLargeScreenshotPayload(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	token := "desktop-command-large-screenshot-token"
	if _, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256(token),
		DeviceName: "large-screenshot-pc",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)
	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion:      "0.1.0",
		PairingToken:       token,
		ClientCapabilities: []string{"remote.desktop.capture"},
		Host:               agodesk.SessionStartHost{Hostname: "BIG-SHOT-PC", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write session.start: %v", err)
	}
	acceptedEnvelope := readAgodeskTestEnvelope(t, conn)
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, acceptedEnvelope, &accepted)

	type commandResult struct {
		result remote.ResultPayload
		err    error
	}
	resultCh := make(chan commandResult, 1)
	go func() {
		result, err := s.RemoteHub.SendCommand(accepted.DeviceID, remote.CommandPayload{
			Operation: remote.OpDesktopScreenshot,
			Args:      map[string]interface{}{"format": "png"},
		}, 2*time.Second)
		resultCh <- commandResult{result: result, err: err}
	}()

	cmdEnvelope := readAgodeskTestEnvelope(t, conn)
	var cmd agodesk.DesktopCommandPayload
	decodeAgodeskTestPayload(t, cmdEnvelope, &cmd)
	if cmd.CommandID == "" {
		t.Fatalf("desktop command payload = %+v", cmd)
	}
	largeImage := strings.Repeat("a", 420*1024)
	resultEnvelope, err := agodesk.NewEnvelope(agodesk.TypeDesktopResult, agodesk.DesktopResultPayload{
		CommandID: cmd.CommandID,
		OK:        true,
		Data: map[string]interface{}{
			"source":      "display",
			"display_id":  "display-0",
			"format":      "png",
			"mime":        "image/png",
			"data_base64": largeImage,
		},
	})
	if err != nil {
		t.Fatalf("NewEnvelope desktop.result: %v", err)
	}
	raw, _ := json.Marshal(resultEnvelope)
	if len(raw) <= 256*1024 {
		t.Fatalf("test payload = %d bytes, want above old agodesk limit", len(raw))
	}
	if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		t.Fatalf("write large desktop.result: %v", err)
	}

	select {
	case got := <-resultCh:
		if got.err != nil {
			t.Fatalf("SendCommand returned error: %v", got.err)
		}
		if got.result.Status != "ok" || !strings.Contains(got.result.Output, `"display_id":"display-0"`) {
			t.Fatalf("large desktop command result = %+v", got.result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for large desktop result")
	}
}

func TestAgodeskRejectsLargeNonDesktopMessage(t *testing.T) {
	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: "agodesk:test",
		Text:      strings.Repeat("x", 300*1024),
		Role:      "user",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.message: %v", err)
	}
	raw, _ := json.Marshal(msg)
	if len(raw) <= agodeskControlMessageMaxBytes {
		t.Fatalf("test payload = %d bytes, want above control limit", len(raw))
	}
	if _, err := decodeAgodeskEnvelope(raw); err == nil || !strings.Contains(err.Error(), "message too large") {
		t.Fatalf("decodeAgodeskEnvelope error = %v, want message too large", err)
	}
}

func newAgodeskHandlerTestServer() *Server {
	return &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
}

func newAgodeskPairingTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := remote.InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("remote InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	cfg := &config.Config{}
	hub := remote.NewRemoteHub(db, vault, slog.Default())
	return &Server{
		Cfg:       cfg,
		Logger:    slog.Default(),
		Vault:     vault,
		RemoteHub: hub,
	}
}

func pairAgodeskTestClient(t *testing.T, s *Server, token string, clientCapabilities []string) (*websocket.Conn, func(), agodesk.SessionAcceptedPayload) {
	t.Helper()
	if _, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256(token),
		DeviceName: "agodesk-test-client",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	_ = readAgodeskTestEnvelope(t, conn)
	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion:      "0.1.0",
		PairingToken:       token,
		ClientCapabilities: clientCapabilities,
		Host:               agodesk.SessionStartHost{Hostname: "AGODESK-PC", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		cleanup()
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		cleanup()
		t.Fatalf("write session.start: %v", err)
	}
	acceptedEnvelope := readAgodeskTestEnvelope(t, conn)
	var accepted agodesk.SessionAcceptedPayload
	decodeAgodeskTestPayload(t, acceptedEnvelope, &accepted)
	if accepted.DeviceID == "" {
		cleanup()
		t.Fatalf("accepted payload missing device id: %+v", accepted)
	}
	return conn, cleanup, accepted
}

func dialAgodeskTestWebSocket(t *testing.T, s *Server, path string) (*websocket.Conn, func()) {
	t.Helper()
	srv := httptest.NewServer(handleAgodeskWebSocket(s))
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial agodesk websocket: %v", err)
	}
	return conn, func() {
		_ = conn.Close()
		srv.Close()
	}
}

func readAgodeskTestEnvelope(t *testing.T, conn *websocket.Conn) agodesk.Envelope {
	t.Helper()
	var env agodesk.Envelope
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatalf("read envelope: %v", err)
	}
	return env
}

func decodeAgodeskTestPayload(t *testing.T, env agodesk.Envelope, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(env.Payload, target); err != nil {
		t.Fatalf("decode %s payload: %v", env.Type, err)
	}
}

func agodeskTestContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
