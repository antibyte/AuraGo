package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/agodesk"
	"aurago/internal/remote"

	"github.com/gorilla/websocket"
)

const (
	agodeskMessageSource   = "agodesk_chat"
	agodeskMaxMessageBytes = 256 * 1024
)

var agodeskUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     sameOriginOrNoOrigin,
}

var agodeskAgentChatRunner = runAgodeskAgentChat

type agodeskConnectionState struct {
	sessionID string
	deviceID  string
	paired    bool
	readOnly  bool
	devMode   bool
}

func handleAgodeskWebSocket(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := agodeskUpgrader.Upgrade(w, r, nil)
		if err != nil {
			if s != nil && s.Logger != nil {
				s.Logger.Error("agodesk WebSocket upgrade failed", "error", err)
			}
			return
		}
		defer conn.Close()
		conn.SetReadLimit(agodeskMaxMessageBytes)

		state := agodeskConnectionState{
			sessionID: "agodesk:temp:" + agodeskConnectionID(),
			paired:    false,
			devMode:   isExplicitAgodeskLoopbackDev(r),
		}
		if state.devMode {
			state.sessionID = "agodesk:dev:" + agodeskConnectionID()
			state.paired = true
		}

		if err := writeAgodeskEnvelope(conn, agodesk.TypeSystemConnected, agodesk.SystemConnectedPayload{
			ProtocolVersion: agodesk.ProtocolVersion,
			ServerVersion:   "aurago",
			SessionID:       state.sessionID,
			AuthRequired:    !state.devMode,
			PairingRequired: !state.devMode,
			Capabilities:    agodesk.DefaultCapabilities,
		}); err != nil {
			return
		}

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			env, err := agodesk.DecodeEnvelope(data, agodeskMaxMessageBytes)
			if err != nil {
				_ = writeAgodeskEnvelope(conn, agodesk.TypeChatError, agodesk.ChatErrorPayload{
					Code:    agodesk.ErrorInvalidMessage,
					Message: "Invalid agodesk message: " + err.Error(),
				})
				continue
			}
			if !handleAgodeskEnvelope(s, r, conn, &state, env) {
				return
			}
		}
	}
}

func handleAgodeskEnvelope(s *Server, r *http.Request, conn *websocket.Conn, state *agodeskConnectionState, env agodesk.Envelope) bool {
	switch env.Type {
	case agodesk.TypeSystemPing:
		_ = writeAgodeskEnvelope(conn, agodesk.TypeSystemPong, map[string]string{})
	case agodesk.TypeSessionStart:
		payload, errPayload := decodeAgodeskPayload[agodesk.SessionStartPayload](env)
		if errPayload != nil {
			_ = writeAgodeskError(conn, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		accepted, errCode, errMsg := acceptAgodeskSessionStart(s, r, env.ID, payload)
		if errCode != "" {
			_ = writeAgodeskError(conn, env.ID, errCode, errMsg)
			return true
		}
		state.sessionID = accepted.SessionID
		state.deviceID = accepted.DeviceID
		state.readOnly = accepted.ReadOnly
		state.paired = accepted.Approved
		_ = writeAgodeskEnvelope(conn, agodesk.TypeSessionAccepted, accepted)
	case agodesk.TypeChatMessage:
		payload, errPayload := decodeAgodeskPayload[agodesk.ChatMessagePayload](env)
		if errPayload != nil {
			_ = writeAgodeskError(conn, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskChatMessage(s, r, conn, state, env.ID, payload)
	case agodesk.TypeDesktopResult:
		// Desktop-control transport is defined in v1 but command routing is feature-gated.
		return true
	default:
		return true
	}
	return true
}

func handleAgodeskChatMessage(s *Server, r *http.Request, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatMessagePayload) {
	if state == nil || !state.paired {
		_ = writeAgodeskError(conn, requestID, agodesk.ErrorPairingRequired, "Pairing is required before chat messages are accepted.")
		return
	}
	message := strings.TrimSpace(payload.Text)
	if message == "" {
		_ = writeAgodeskError(conn, requestID, agodesk.ErrorInvalidMessage, "chat.message text is required")
		return
	}
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" || sessionID != state.sessionID {
		sessionID = state.sessionID
	}
	unlockSession := lockSessionRequest(sessionID)
	defer unlockSession()

	answer, err := agodeskAgentChatRunner(s, r, sessionID, message)
	if err != nil {
		_ = writeAgodeskError(conn, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	_ = writeAgodeskEnvelope(conn, agodesk.TypeChatResponse, agodesk.ChatResponsePayload{
		SessionID: sessionID,
		RequestID: requestID,
		Text:      strings.TrimSpace(answer),
		Role:      "assistant",
		Metadata: map[string]interface{}{
			"source": agodeskMessageSource,
		},
	})
}

func acceptAgodeskSessionStart(s *Server, r *http.Request, requestID string, payload agodesk.SessionStartPayload) (agodesk.SessionAcceptedPayload, string, string) {
	if s == nil || s.RemoteHub == nil || s.RemoteHub.DB() == nil {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorPairingRequired, "RemoteHub pairing is not available."
	}
	if deviceID := strings.TrimSpace(payload.DeviceID); deviceID != "" && strings.TrimSpace(payload.PairingToken) == "" {
		return acceptAgodeskDeviceReconnect(s, requestID, payload, deviceID)
	}
	if strings.TrimSpace(payload.PairingToken) == "" {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorPairingRequired, "pairing_token is required for this agodesk session."
	}

	tokenHash := hashSHA256(strings.TrimSpace(payload.PairingToken))
	enrollment, err := remote.GetEnrollmentByTokenHash(s.RemoteHub.DB(), tokenHash)
	if err != nil {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorAuthFailed, "invalid pairing token"
	}
	if enrollment.Used {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorAuthFailed, "pairing token has already been used"
	}
	if expiry, err := time.Parse(time.RFC3339, enrollment.ExpiresAt); err == nil && time.Now().After(expiry) {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorAuthFailed, "pairing token expired"
	}

	sharedKey, err := remote.GenerateSharedKey()
	if err != nil {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorInternal, "failed to generate shared key"
	}
	name := strings.TrimSpace(enrollment.DeviceName)
	if name == "" {
		name = strings.TrimSpace(payload.Host.Hostname)
	}
	if name == "" {
		name = "agodesk"
	}
	readOnly := s.RemoteHub.DefaultReadOnly
	deviceID, err := remote.CreateDevice(s.RemoteHub.DB(), remote.DeviceRecord{
		Name:          name,
		Hostname:      strings.TrimSpace(payload.Host.Hostname),
		OS:            strings.TrimSpace(payload.Host.OS),
		Arch:          strings.TrimSpace(payload.Host.Arch),
		IPAddress:     agodeskClientIP(r, payload.Host.IP),
		Status:        "approved",
		ReadOnly:      readOnly,
		SharedKeyHash: hashSHA256(sharedKey),
		Tags:          []string{"agodesk", "desktop-client"},
	})
	if err != nil {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorInternal, "failed to register agodesk device"
	}
	if s.Vault != nil {
		if err := s.Vault.WriteSecret("remote_shared_key_"+deviceID, sharedKey); err != nil && s.Logger != nil {
			s.Logger.Error("Failed to store agodesk shared key", "device_id", deviceID, "error", err)
		}
	}
	_ = remote.MarkEnrollmentUsed(s.RemoteHub.DB(), enrollment.ID, deviceID)
	return agodesk.SessionAcceptedPayload{
		SessionID:    "agodesk:" + deviceID,
		DeviceID:     deviceID,
		Approved:     true,
		ReadOnly:     readOnly,
		Capabilities: agodesk.DefaultCapabilities,
		SharedKey:    sharedKey,
	}, "", ""
}

func acceptAgodeskDeviceReconnect(s *Server, requestID string, payload agodesk.SessionStartPayload, deviceID string) (agodesk.SessionAcceptedPayload, string, string) {
	device, err := remote.GetDevice(s.RemoteHub.DB(), deviceID)
	if err != nil {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorAuthFailed, "unknown agodesk device"
	}
	if device.Status == "revoked" {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorDeviceNotApproved, "device has been revoked"
	}
	if device.Status != "approved" && device.Status != "connected" {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorDeviceNotApproved, "device is not approved"
	}
	if payload.SharedKeyProof == nil {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorAuthRequired, "shared_key_proof is required for reconnect"
	}
	if s.Vault == nil {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorInternal, "vault is not available"
	}
	sharedKey, err := s.Vault.ReadSecret("remote_shared_key_" + deviceID)
	if err != nil {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorAuthFailed, "missing shared key"
	}
	if device.SharedKeyHash != "" && !strings.EqualFold(device.SharedKeyHash, hashSHA256(sharedKey)) {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorAuthFailed, "shared key hash mismatch"
	}
	if !agodesk.VerifySharedKeyProof(sharedKey, requestID, deviceID, *payload.SharedKeyProof, time.Now().UTC(), 5*time.Minute) {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorAuthFailed, "invalid shared key proof"
	}
	device.Hostname = strings.TrimSpace(payload.Host.Hostname)
	device.OS = strings.TrimSpace(payload.Host.OS)
	device.Arch = strings.TrimSpace(payload.Host.Arch)
	if err := remote.UpdateDevice(s.RemoteHub.DB(), device); err != nil && s.Logger != nil {
		s.Logger.Warn("Failed to update agodesk reconnect device metadata", "device_id", deviceID, "error", err)
	}
	return agodesk.SessionAcceptedPayload{
		SessionID:    "agodesk:" + deviceID,
		DeviceID:     deviceID,
		Approved:     true,
		ReadOnly:     device.ReadOnly,
		Capabilities: agodesk.DefaultCapabilities,
	}, "", ""
}

func runAgodeskAgentChat(s *Server, r *http.Request, sessionID, message string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("server not configured")
	}
	turn, err := prepareDesktopAgentTurnWithOptions(r.Context(), s, message, desktopChatContext{}, false, desktopAgentTurnOptions{
		SessionID:        sessionID,
		MessageSource:    agodeskMessageSource,
		AdditionalPrompt: buildAgodeskAgentContext(),
	})
	if err != nil {
		return "", err
	}
	broker := &desktopReplyBroker{FeedbackBroker: NewSSEBrokerAdapterWithSession(s.SSE, sessionID)}
	ctx, cancel := context.WithTimeout(r.Context(), desktopChatAgentTurnTimeout)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := agent.ExecuteAgentLoop(ctx, turn.req, turn.runCfg, false, broker); err != nil {
			broker.Send("error_recovery", chatCompletionErrorMessage(desktopUILanguage(s), err))
		}
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return "", fmt.Errorf("agent request timed out")
	}
	answer := strings.TrimSpace(broker.finalResponse)
	if answer == "" {
		answer = latestDesktopAssistantMessage(s.ShortTermMem, sessionID)
	}
	return strings.TrimSpace(answer), nil
}

func buildAgodeskAgentContext() string {
	return strings.Join([]string{
		"The user is chatting from agodesk, a paired desktop companion running on a remote PC.",
		"When the user asks about that remote PC, prefer the remote_control tool for available device operations and respect read-only policy.",
		"Desktop capture and desktop input are planned agodesk capabilities. Do not claim they are available unless the connected client advertises and successfully executes those commands.",
	}, "\n")
}

func decodeAgodeskPayload[T any](env agodesk.Envelope) (T, error) {
	var payload T
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return payload, fmt.Errorf("invalid %s payload: %w", env.Type, err)
	}
	return payload, nil
}

func writeAgodeskError(conn *websocket.Conn, requestID, code, message string) error {
	return writeAgodeskEnvelope(conn, agodesk.TypeChatError, agodesk.ChatErrorPayload{
		RequestID: requestID,
		Code:      code,
		Message:   message,
	})
}

func writeAgodeskEnvelope(conn *websocket.Conn, messageType agodesk.MessageType, payload interface{}) error {
	env, err := agodesk.NewEnvelope(messageType, payload)
	if err != nil {
		return err
	}
	return conn.WriteJSON(env)
}

func isExplicitAgodeskLoopbackDev(r *http.Request) bool {
	if r == nil || r.URL.Query().Get("insecure_loopback") != "1" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func agodeskConnectionID() string {
	env, err := agodesk.NewEnvelope(agodesk.TypeSystemPong, map[string]string{})
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return env.ID
}

func agodeskClientIP(r *http.Request, fallback string) string {
	if ip := strings.TrimSpace(fallback); ip != "" {
		return ip
	}
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
