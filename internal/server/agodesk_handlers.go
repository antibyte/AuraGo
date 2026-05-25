package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
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
	mu        sync.RWMutex
	writeMu   sync.Mutex
}

type agodeskDesktopBroker struct {
	hub      *remote.RemoteHub
	logger   *slog.Logger
	mu       sync.RWMutex
	sessions map[string]*agodeskDesktopSession
}

type agodeskDesktopSession struct {
	deviceID  string
	conn      *websocket.Conn
	state     *agodeskConnectionState
	pendingMu sync.Mutex
	pending   map[string]chan agodeskDesktopCommandResult
}

type agodeskDesktopCommandResult struct {
	payload agodesk.DesktopResultPayload
	err     error
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
		defer cleanupAgodeskConnection(s, &state)

		if err := writeAgodeskEnvelopeLocked(conn, &state, agodesk.TypeSystemConnected, agodesk.SystemConnectedPayload{
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
				_ = writeAgodeskEnvelopeLocked(conn, &state, agodesk.TypeChatError, agodesk.ChatErrorPayload{
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
		_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeSystemPong, map[string]string{})
	case agodesk.TypeSessionStart:
		payload, errPayload := decodeAgodeskPayload[agodesk.SessionStartPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		accepted, errCode, errMsg := acceptAgodeskSessionStart(s, r, env.ID, payload)
		if errCode != "" {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, errCode, errMsg)
			return true
		}
		state.mu.Lock()
		state.sessionID = accepted.SessionID
		state.deviceID = accepted.DeviceID
		state.readOnly = accepted.ReadOnly
		state.paired = accepted.Approved
		state.mu.Unlock()
		registerAgodeskDesktopSession(s, conn, state, accepted)
		_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeSessionAccepted, accepted)
	case agodesk.TypeChatMessage:
		payload, errPayload := decodeAgodeskPayload[agodesk.ChatMessagePayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		go handleAgodeskChatMessage(s, r, conn, state, env.ID, payload)
	case agodesk.TypeDesktopResult:
		payload, errPayload := decodeAgodeskPayload[agodesk.DesktopResultPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		deviceID, paired := agodeskStateDevice(state)
		if !paired || deviceID == "" {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorPairingRequired, "Pairing is required before desktop results are accepted.")
			return true
		}
		broker := ensureAgodeskDesktopBroker(s)
		if broker == nil || !broker.HandleResult(deviceID, payload) {
			if s != nil && s.Logger != nil {
				s.Logger.Warn("Unknown agodesk desktop result", "device_id", deviceID, "command_id", payload.CommandID)
			}
		}
		return true
	default:
		return true
	}
	return true
}

func handleAgodeskChatMessage(s *Server, r *http.Request, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatMessagePayload) {
	paired := false
	stateSessionID := ""
	if state != nil {
		state.mu.RLock()
		paired = state.paired
		stateSessionID = state.sessionID
		state.mu.RUnlock()
	}
	if !paired {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorPairingRequired, "Pairing is required before chat messages are accepted.")
		return
	}
	message := strings.TrimSpace(payload.Text)
	if message == "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, "chat.message text is required")
		return
	}
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, "chat.message session_id is required")
		return
	}
	if sessionID != stateSessionID {
		if s != nil && s.Logger != nil {
			s.Logger.Warn("agodesk chat session mismatch", "request_id", requestID, "payload_session_id", sessionID, "active_session_id", stateSessionID)
		}
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, "chat.message session_id does not match the active agodesk session")
		return
	}
	unlockSession := lockSessionRequest(sessionID)
	defer unlockSession()

	answer, err := agodeskAgentChatRunner(s, r, sessionID, message)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatResponse, agodesk.ChatResponsePayload{
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
		"Desktop screenshots are available through remote_control desktop_screenshot when the agodesk client is connected.",
		"Desktop input requires local approval in the agodesk remote-control banner; the backend cannot approve or bypass that local control session.",
		"Desktop streaming is not available in this backend version.",
	}, "\n")
}

func ensureAgodeskDesktopBroker(s *Server) *agodeskDesktopBroker {
	if s == nil || s.RemoteHub == nil {
		return nil
	}
	s.agodeskDesktopMu.Lock()
	defer s.agodeskDesktopMu.Unlock()
	if s.agodeskDesktop != nil {
		return s.agodeskDesktop
	}
	broker := &agodeskDesktopBroker{
		hub:      s.RemoteHub,
		logger:   s.Logger,
		sessions: make(map[string]*agodeskDesktopSession),
	}
	s.agodeskDesktop = broker
	s.RemoteHub.RegisterCommandTransport("agodesk", broker)
	return broker
}

func registerAgodeskDesktopSession(s *Server, conn *websocket.Conn, state *agodeskConnectionState, accepted agodesk.SessionAcceptedPayload) {
	if !accepted.Approved || strings.TrimSpace(accepted.DeviceID) == "" {
		return
	}
	broker := ensureAgodeskDesktopBroker(s)
	if broker == nil {
		return
	}
	broker.RegisterSession(accepted.DeviceID, conn, state)
	if s != nil && s.RemoteHub != nil && s.RemoteHub.DB() != nil {
		_ = remote.UpdateDeviceStatus(s.RemoteHub.DB(), accepted.DeviceID, "connected")
	}
}

func cleanupAgodeskConnection(s *Server, state *agodeskConnectionState) {
	deviceID, paired := agodeskStateDevice(state)
	if !paired || deviceID == "" {
		return
	}
	removed := false
	if broker := ensureAgodeskDesktopBroker(s); broker != nil {
		removed = broker.UnregisterSession(deviceID, state)
	}
	if removed && s != nil && s.RemoteHub != nil && s.RemoteHub.DB() != nil {
		_ = remote.UpdateDeviceStatus(s.RemoteHub.DB(), deviceID, "offline")
	}
}

func agodeskStateDevice(state *agodeskConnectionState) (string, bool) {
	if state == nil {
		return "", false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.deviceID, state.paired
}

func (b *agodeskDesktopBroker) RegisterSession(deviceID string, conn *websocket.Conn, state *agodeskConnectionState) {
	if b == nil || strings.TrimSpace(deviceID) == "" || conn == nil || state == nil {
		return
	}
	session := &agodeskDesktopSession{
		deviceID: deviceID,
		conn:     conn,
		state:    state,
		pending:  make(map[string]chan agodeskDesktopCommandResult),
	}
	var old *agodeskDesktopSession
	b.mu.Lock()
	if b.sessions == nil {
		b.sessions = make(map[string]*agodeskDesktopSession)
	}
	old = b.sessions[deviceID]
	b.sessions[deviceID] = session
	b.mu.Unlock()
	if old != nil && old != session {
		old.failPending(fmt.Errorf("agodesk session replaced"))
		if old.conn != nil && old.conn != conn {
			_ = old.conn.Close()
		}
	}
}

func (b *agodeskDesktopBroker) UnregisterSession(deviceID string, state *agodeskConnectionState) bool {
	if b == nil || strings.TrimSpace(deviceID) == "" {
		return false
	}
	var session *agodeskDesktopSession
	b.mu.Lock()
	current := b.sessions[deviceID]
	if current != nil && (state == nil || current.state == state) {
		session = current
		delete(b.sessions, deviceID)
	}
	b.mu.Unlock()
	if session != nil {
		session.failPending(fmt.Errorf("agodesk session disconnected"))
		return true
	}
	return false
}

func (b *agodeskDesktopBroker) IsConnected(deviceID string) bool {
	return b.session(deviceID) != nil
}

func (b *agodeskDesktopBroker) SendCommand(deviceID string, cmd remote.CommandPayload, timeout time.Duration) (remote.ResultPayload, error) {
	session := b.session(deviceID)
	if session == nil {
		return remote.ResultPayload{}, fmt.Errorf("no active agodesk session for device %s", deviceID)
	}
	resultCh := make(chan agodeskDesktopCommandResult, 1)
	session.pendingMu.Lock()
	session.pending[cmd.CommandID] = resultCh
	session.pendingMu.Unlock()
	defer session.removePending(cmd.CommandID)

	if err := writeAgodeskEnvelopeLocked(session.conn, session.state, agodesk.TypeDesktopCommand, agodesk.DesktopCommandPayload{
		CommandID: cmd.CommandID,
		Operation: cmd.Operation,
		Params:    cmd.Args,
	}); err != nil {
		return remote.ResultPayload{}, fmt.Errorf("send agodesk desktop command: %w", err)
	}

	select {
	case result := <-resultCh:
		if result.err != nil {
			return remote.ResultPayload{CommandID: cmd.CommandID, Status: "error", Error: result.err.Error()}, nil
		}
		return agodeskDesktopResultToRemoteResult(cmd.CommandID, result.payload), nil
	case <-time.After(timeout):
		return remote.ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "timeout",
			Error:     fmt.Sprintf("command timed out after %v", timeout),
		}, nil
	}
}

func (b *agodeskDesktopBroker) HandleResult(deviceID string, payload agodesk.DesktopResultPayload) bool {
	session := b.session(deviceID)
	if session == nil || strings.TrimSpace(payload.CommandID) == "" {
		return false
	}
	session.pendingMu.Lock()
	ch, ok := session.pending[payload.CommandID]
	session.pendingMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- agodeskDesktopCommandResult{payload: payload}:
	default:
	}
	return true
}

func (b *agodeskDesktopBroker) session(deviceID string) *agodeskDesktopSession {
	if b == nil || strings.TrimSpace(deviceID) == "" {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sessions[deviceID]
}

func (s *agodeskDesktopSession) removePending(commandID string) {
	if s == nil || strings.TrimSpace(commandID) == "" {
		return
	}
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	delete(s.pending, commandID)
}

func (s *agodeskDesktopSession) failPending(err error) {
	if s == nil {
		return
	}
	s.pendingMu.Lock()
	pending := s.pending
	s.pending = make(map[string]chan agodeskDesktopCommandResult)
	s.pendingMu.Unlock()
	for _, ch := range pending {
		select {
		case ch <- agodeskDesktopCommandResult{err: err}:
		default:
		}
	}
}

func agodeskDesktopResultToRemoteResult(commandID string, payload agodesk.DesktopResultPayload) remote.ResultPayload {
	status := "ok"
	if !payload.OK {
		status = "error"
	}
	output := ""
	if payload.Data != nil {
		if data, err := json.Marshal(payload.Data); err == nil {
			output = string(data)
		}
	}
	if payload.CommandID != "" {
		commandID = payload.CommandID
	}
	return remote.ResultPayload{
		CommandID: commandID,
		Status:    status,
		Output:    output,
		Error:     payload.Error,
	}
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

func writeAgodeskErrorLocked(conn *websocket.Conn, state *agodeskConnectionState, requestID, code, message string) error {
	return writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatError, agodesk.ChatErrorPayload{
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

func writeAgodeskEnvelopeLocked(conn *websocket.Conn, state *agodeskConnectionState, messageType agodesk.MessageType, payload interface{}) error {
	if state == nil {
		return writeAgodeskEnvelope(conn, messageType, payload)
	}
	state.writeMu.Lock()
	defer state.writeMu.Unlock()
	return writeAgodeskEnvelope(conn, messageType, payload)
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
