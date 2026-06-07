package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/agodesk"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	promptsembed "aurago/prompts"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

const (
	agodeskMessageSource           = "agodesk_chat"
	agodeskControlMessageMaxBytes  = 256 * 1024
	agodeskDesktopResultMaxBytes   = 16 * 1024 * 1024
	agodeskWebSocketReadLimitBytes = agodeskDesktopResultMaxBytes
)

var agodeskUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     sameOriginOrNoOrigin,
}

type agodeskChatResult struct {
	Answer   string
	Metadata map[string]interface{}
}

var agodeskAgentChatRunner = runAgodeskAgentChat

type agodeskConnectionState struct {
	sessionID             string
	deviceID              string
	paired                bool
	readOnly              bool
	devMode               bool
	capabilities          map[string]struct{}
	activeRuns            map[string]agodeskActiveChatRun
	latestActiveRequestID string
	nextActiveRunSequence uint64
	mu                    sync.RWMutex
	writeMu               sync.Mutex
	activeMu              sync.Mutex
}

type agodeskActiveChatRun struct {
	conversationID string
	cancel         context.CancelFunc
	sequence       uint64
}

type agodeskDesktopBroker struct {
	hub      *remote.RemoteHub
	logger   *slog.Logger
	mu       sync.RWMutex
	sessions map[string]*agodeskDesktopSession
}

type agodeskDesktopSession struct {
	deviceID     string
	conn         *websocket.Conn
	state        *agodeskConnectionState
	capabilities map[string]struct{}
	pendingMu    sync.Mutex
	pending      map[string]chan agodeskDesktopCommandResult
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
		conn.SetReadLimit(agodeskWebSocketReadLimitBytes)

		serverCapabilities := agodeskServerCapabilities(s)
		state := agodeskConnectionState{
			sessionID:    "agodesk:temp:" + agodeskConnectionID(),
			paired:       false,
			devMode:      isExplicitAgodeskLoopbackDev(r),
			capabilities: make(map[string]struct{}),
			activeRuns:   make(map[string]agodeskActiveChatRun),
		}
		if state.devMode {
			state.sessionID = "agodesk:dev:" + agodeskConnectionID()
			state.paired = true
			state.capabilities = normalizeAgodeskCapabilities(serverCapabilities)
		}
		defer cleanupAgodeskConnection(s, &state)

		if err := writeAgodeskEnvelopeLocked(conn, &state, agodesk.TypeSystemConnected, agodesk.SystemConnectedPayload{
			ProtocolVersion: agodesk.ProtocolVersion,
			ServerVersion:   "aurago",
			SessionID:       state.sessionID,
			AuthRequired:    !state.devMode,
			PairingRequired: !state.devMode,
			Capabilities:    serverCapabilities,
		}); err != nil {
			return
		}

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			env, err := decodeAgodeskEnvelope(data)
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
		state.capabilities = normalizeAgodeskCapabilities(accepted.AdvertisedCapabilities)
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
	case agodesk.TypeChatSessionsList:
		payload, errPayload := decodeAgodeskPayload[agodesk.ChatSessionsListPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskChatSessionsList(s, conn, state, env.ID, payload)
	case agodesk.TypeChatSessionCreate:
		payload, errPayload := decodeAgodeskPayload[agodesk.ChatSessionCreatePayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskChatSessionCreate(s, conn, state, env.ID, payload)
	case agodesk.TypeChatSessionLoad:
		payload, errPayload := decodeAgodeskPayload[agodesk.ChatSessionLoadPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskChatSessionLoad(s, conn, state, env.ID, payload)
	case agodesk.TypeChatCancel:
		payload, errPayload := decodeAgodeskPayload[agodesk.ChatCancelPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskChatCancel(s, conn, state, env.ID, payload)
	case agodesk.TypeChatVoiceOutputStatus:
		payload, errPayload := decodeAgodeskPayload[agodesk.ChatVoiceOutputStatusPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskVoiceOutputStatus(s, conn, state, env.ID, payload)
	case agodesk.TypePersonaAssetsRequest:
		payload, errPayload := decodeAgodeskPayload[agodesk.PersonaAssetsRequestPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskPersonaAssetsRequest(s, conn, state, env.ID, payload)
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

func handleAgodeskChatSessionsList(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatSessionsListPayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.sessions.list")
	if !ok {
		return
	}
	if s == nil || s.ShortTermMem == nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, "short-term memory is not configured")
		return
	}
	limit := payload.Limit
	if limit <= 0 {
		limit = agodeskChatSessionLimit(s)
	}
	sessions, err := s.ShortTermMem.ListChatSessionsWithLimit(limit)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatSessions, agodesk.ChatSessionsPayload{
		SessionID: sessionID,
		Sessions:  agodeskChatSessionSummaries(sessions),
	})
}

func handleAgodeskChatSessionCreate(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatSessionCreatePayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.session.create")
	if !ok {
		return
	}
	if s == nil || s.ShortTermMem == nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, "short-term memory is not configured")
		return
	}
	session, err := s.ShortTermMem.CreateChatSessionWithLimit(agodeskChatSessionLimit(s))
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatSession, agodesk.ChatSessionPayload{
		SessionID:      sessionID,
		ConversationID: session.ID,
		Session:        agodeskChatSessionSummary(*session),
	})
}

func handleAgodeskChatSessionLoad(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatSessionLoadPayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.session.load")
	if !ok {
		return
	}
	conversationID := strings.TrimSpace(payload.ConversationID)
	if conversationID == "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, "chat.session.load conversation_id is required")
		return
	}
	if s == nil || s.ShortTermMem == nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, "short-term memory is not configured")
		return
	}
	session, err := s.ShortTermMem.GetChatSession(conversationID)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	if session == nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, "chat.session.load conversation_id was not found")
		return
	}
	messages, err := s.ShortTermMem.GetSessionMessages(conversationID)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	_ = s.ShortTermMem.TouchChatSession(conversationID)
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatSession, agodesk.ChatSessionPayload{
		SessionID:      sessionID,
		ConversationID: conversationID,
		Session:        agodeskChatSessionSummary(*session),
		Messages:       agodeskHistoryMessages(messages),
	})
}

func handleAgodeskChatCancel(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatCancelPayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.cancel")
	if !ok {
		return
	}
	conversationID := strings.TrimSpace(payload.ConversationID)
	activeRequestID := strings.TrimSpace(payload.RequestID)
	run, activeRequestID, found := agodeskFindActiveRun(state, activeRequestID, conversationID)
	if found {
		if conversationID == "" {
			conversationID = run.conversationID
		}
		if run.cancel != nil {
			run.cancel()
		}
	}
	if found && conversationID != "" {
		agent.InterruptSession(conversationID)
	}
	status := "not_active"
	if found {
		status = "cancelled"
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatCancelled, agodesk.ChatCancelledPayload{
		SessionID:      sessionID,
		ConversationID: conversationID,
		RequestID:      activeRequestID,
		Status:         status,
	})
}

func handleAgodeskVoiceOutputStatus(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatVoiceOutputStatusPayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.voice_output.status")
	if !ok {
		return
	}
	speakerMode := payload.SpeakerMode
	mode := strings.ToLower(strings.TrimSpace(payload.Mode))
	switch mode {
	case "on", "enabled", "speaker":
		speakerMode = true
	case "off", "disabled", "muted":
		speakerMode = false
	}
	agent.SetVoiceMode(speakerMode)
	if s != nil && s.Logger != nil {
		s.Logger.Info("AgoDesk voice output status updated", "speaker_mode", speakerMode)
	}
	if mode == "" {
		if speakerMode {
			mode = "on"
		} else {
			mode = "off"
		}
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatVoiceOutputStatus, agodesk.ChatVoiceOutputStatusPayload{
		SessionID:      sessionID,
		ConversationID: strings.TrimSpace(payload.ConversationID),
		SpeakerMode:    speakerMode,
		Mode:           mode,
		Reason:         strings.TrimSpace(payload.Reason),
		Status:         "ok",
	})
}

func validateAgodeskTransportSession(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID, payloadSessionID, messageName string) (string, bool) {
	paired := false
	stateSessionID := ""
	if state != nil {
		state.mu.RLock()
		paired = state.paired
		stateSessionID = state.sessionID
		state.mu.RUnlock()
	}
	if !paired {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorPairingRequired, "Pairing is required before "+messageName+" is accepted.")
		return "", false
	}
	sessionID := strings.TrimSpace(payloadSessionID)
	if sessionID == "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, messageName+" session_id is required")
		return "", false
	}
	if sessionID != stateSessionID {
		if s != nil && s.Logger != nil {
			s.Logger.Warn("agodesk transport session mismatch", "request_id", requestID, "message_type", messageName, "payload_session_id", sessionID, "active_session_id", stateSessionID)
		}
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, messageName+" session_id does not match the active agodesk session")
		return "", false
	}
	return sessionID, true
}

func registerAgodeskActiveRun(state *agodeskConnectionState, requestID, conversationID string, cancel context.CancelFunc) {
	if state == nil || strings.TrimSpace(requestID) == "" {
		return
	}
	state.activeMu.Lock()
	defer state.activeMu.Unlock()
	if state.activeRuns == nil {
		state.activeRuns = make(map[string]agodeskActiveChatRun)
	}
	state.nextActiveRunSequence++
	state.activeRuns[requestID] = agodeskActiveChatRun{
		conversationID: strings.TrimSpace(conversationID),
		cancel:         cancel,
		sequence:       state.nextActiveRunSequence,
	}
	state.latestActiveRequestID = requestID
}

func unregisterAgodeskActiveRun(state *agodeskConnectionState, requestID string) {
	if state == nil || strings.TrimSpace(requestID) == "" {
		return
	}
	state.activeMu.Lock()
	defer state.activeMu.Unlock()
	delete(state.activeRuns, requestID)
	if state.latestActiveRequestID == requestID {
		state.latestActiveRequestID = ""
		var latestSeq uint64
		for id, run := range state.activeRuns {
			if run.sequence > latestSeq {
				latestSeq = run.sequence
				state.latestActiveRequestID = id
			}
		}
	}
}

func agodeskFindActiveRun(state *agodeskConnectionState, requestID, conversationID string) (agodeskActiveChatRun, string, bool) {
	if state == nil {
		return agodeskActiveChatRun{}, "", false
	}
	requestID = strings.TrimSpace(requestID)
	conversationID = strings.TrimSpace(conversationID)
	state.activeMu.Lock()
	defer state.activeMu.Unlock()
	if requestID != "" {
		run, ok := state.activeRuns[requestID]
		if ok && (conversationID == "" || conversationID == run.conversationID) {
			return run, requestID, true
		}
		return agodeskActiveChatRun{}, requestID, false
	}
	if conversationID != "" {
		for id, run := range state.activeRuns {
			if run.conversationID == conversationID {
				return run, id, true
			}
		}
		return agodeskActiveChatRun{}, "", false
	}
	if state.latestActiveRequestID != "" {
		run, ok := state.activeRuns[state.latestActiveRequestID]
		if ok {
			return run, state.latestActiveRequestID, true
		}
	}
	return agodeskActiveChatRun{}, "", false
}

func agodeskChatSessionLimit(s *Server) int {
	if s == nil || s.Cfg == nil {
		return memory.MaxChatSessions
	}
	s.CfgMu.RLock()
	limit := s.Cfg.Consolidation.ChatSessionLimit
	s.CfgMu.RUnlock()
	if limit <= 0 {
		return memory.MaxChatSessions
	}
	return limit
}

func agodeskChatSessionSummaries(sessions []memory.ChatSession) []agodesk.ChatSessionSummary {
	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].LastActiveAt != sessions[j].LastActiveAt {
			return sessions[i].LastActiveAt > sessions[j].LastActiveAt
		}
		if sessions[i].CreatedAt != sessions[j].CreatedAt {
			return sessions[i].CreatedAt > sessions[j].CreatedAt
		}
		return sessions[i].ID > sessions[j].ID
	})
	summaries := make([]agodesk.ChatSessionSummary, 0, len(sessions))
	for _, session := range sessions {
		summaries = append(summaries, agodeskChatSessionSummary(session))
	}
	return summaries
}

func agodeskChatSessionSummary(session memory.ChatSession) agodesk.ChatSessionSummary {
	return agodesk.ChatSessionSummary{
		ID:           strings.TrimSpace(session.ID),
		Preview:      strings.TrimSpace(session.Preview),
		CreatedAt:    strings.TrimSpace(session.CreatedAt),
		LastActiveAt: strings.TrimSpace(session.LastActiveAt),
		MessageCount: session.MessageCount,
	}
}

func agodeskHistoryMessages(messages []memory.HistoryMessage) []agodesk.ChatHistoryMessagePayload {
	out := make([]agodesk.ChatHistoryMessagePayload, 0, len(messages))
	for _, msg := range messages {
		if msg.IsInternal {
			continue
		}
		out = append(out, agodesk.ChatHistoryMessagePayload{
			Role:      strings.TrimSpace(msg.Role),
			Content:   strings.TrimSpace(msg.Content),
			Timestamp: strings.TrimSpace(msg.Timestamp),
		})
	}
	return out
}

func agodeskServerCapabilities(s *Server) []string {
	capabilities := append([]string(nil), agodesk.DefaultCapabilities...)
	if agodeskServerTTSConfigured(s) {
		capabilities = append(capabilities, "chat.voice_output")
	}
	return capabilities
}

func agodeskServerTTSConfigured(s *Server) bool {
	if s == nil || s.Cfg == nil {
		return false
	}
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	return agodeskTTSConfigured(&cfg)
}

func agodeskTTSConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.TTS.Provider))
	if provider == "" && cfg.TTS.Piper.Enabled {
		provider = "piper"
	}
	switch provider {
	case "google":
		return true
	case "elevenlabs":
		return strings.TrimSpace(cfg.TTS.ElevenLabs.APIKey) != ""
	case "minimax":
		return strings.TrimSpace(cfg.TTS.MiniMax.APIKey) != ""
	case "piper":
		return cfg.TTS.Piper.Enabled
	default:
		return false
	}
}

func handleAgodeskPersonaAssetsRequest(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.PersonaAssetsRequestPayload) {
	paired := false
	stateSessionID := ""
	if state != nil {
		state.mu.RLock()
		paired = state.paired
		stateSessionID = state.sessionID
		state.mu.RUnlock()
	}
	if !paired {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorPairingRequired, "Pairing is required before persona assets are available.")
		return
	}
	sessionID := strings.TrimSpace(payload.SessionID)
	if sessionID == "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, "persona.assets.request session_id is required")
		return
	}
	if sessionID != stateSessionID {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, "persona.assets.request session_id does not match the active agodesk session")
		return
	}
	persona := "custom"
	promptsDir := ""
	if s != nil && s.Cfg != nil {
		s.CfgMu.RLock()
		persona = strings.TrimSpace(s.Cfg.Personality.CorePersonality)
		promptsDir = strings.TrimSpace(s.Cfg.Directories.PromptsDir)
		s.CfgMu.RUnlock()
	}
	if persona == "" {
		persona = "custom"
	}
	personaPrompt := loadAgodeskPersonaPrompt(promptsDir, persona)
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypePersonaAssets, agodesk.NewPersonaAssetsPayload(sessionID, persona, isCorePersonality(persona), personaPrompt))
}

func loadAgodeskPersonaPrompt(promptsDir, personaName string) string {
	persona := strings.TrimSpace(personaName)
	if !isValidPersonalityName(persona) {
		return ""
	}
	if promptsDir != "" {
		if data, err := os.ReadFile(filepath.Join(promptsDir, "personalities", persona+".md")); err == nil {
			return stripAgodeskPersonaFrontMatter(data)
		}
	}
	if data, err := promptsembed.FS.ReadFile("personalities/" + persona + ".md"); err == nil {
		return stripAgodeskPersonaFrontMatter(data)
	}
	return ""
}

func stripAgodeskPersonaFrontMatter(data []byte) string {
	body := strings.TrimSpace(string(data))
	if !strings.HasPrefix(body, "---") {
		return body
	}
	rest := body[3:]
	if idx := strings.Index(rest, "\n---"); idx != -1 {
		return strings.TrimSpace(rest[idx+4:])
	}
	return body
}

func handleAgodeskChatMessage(s *Server, r *http.Request, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatMessagePayload) {
	deviceID := ""
	if state != nil {
		state.mu.RLock()
		deviceID = state.deviceID
		state.mu.RUnlock()
	}
	message := strings.TrimSpace(payload.Text)
	if message == "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, "chat.message text is required")
		return
	}
	transportSessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.message")
	if !ok {
		return
	}
	conversationID, ok := resolveAgodeskConversationID(s, conn, state, requestID, transportSessionID, strings.TrimSpace(payload.ConversationID))
	if !ok {
		return
	}
	unlockSession := lockSessionRequest(conversationID)
	defer unlockSession()

	chatCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	registerAgodeskActiveRun(state, requestID, conversationID, cancel)
	defer unregisterAgodeskActiveRun(state, requestID)

	initialPlan, hasInitialPlan := sendAgodeskCurrentPlanSnapshot(s, conn, state, requestID, transportSessionID, conversationID)
	voiceOutput := payload.VoiceOutput && agodeskStateHasCapability(state, "chat.voice_output")
	result, err := agodeskAgentChatRunner(s, r.WithContext(chatCtx), conn, state, requestID, transportSessionID, conversationID, deviceID, message, voiceOutput)
	if err != nil {
		if chatCtx.Err() != nil {
			return
		}
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return
	}
	metadata := map[string]interface{}{
		"source": agodeskMessageSource,
	}
	for key, value := range result.Metadata {
		metadata[key] = value
	}
	if !agodeskStateHasCapability(state, "chat.agent_metadata") {
		delete(metadata, "agent_mood")
	}
	if hasInitialPlan && agodeskStateHasCapability(state, "chat.plan_updates") {
		if _, ok := metadata["plan"]; !ok {
			metadata["plan"] = initialPlan
		}
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatResponse, agodesk.ChatResponsePayload{
		SessionID:      transportSessionID,
		ConversationID: conversationID,
		RequestID:      requestID,
		Text:           strings.TrimSpace(result.Answer),
		Role:           "assistant",
		Metadata:       metadata,
	})
}

func resolveAgodeskConversationID(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID, transportSessionID, conversationID string) (string, bool) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		if agodeskStateHasCapability(state, "chat.sessions") && !agodeskStateIsDevMode(state) {
			if s == nil || s.ShortTermMem == nil {
				_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, "short-term memory is not configured")
				return "", false
			}
			session, err := s.ShortTermMem.CreateChatSessionWithLimit(agodeskChatSessionLimit(s))
			if err != nil {
				_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
				return "", false
			}
			return session.ID, true
		}
		return strings.TrimSpace(transportSessionID), true
	}
	if s == nil || s.ShortTermMem == nil {
		return conversationID, true
	}
	session, err := s.ShortTermMem.GetChatSession(conversationID)
	if err != nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInternal, err.Error())
		return "", false
	}
	if session == nil {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorSessionNotFound, "chat.message conversation_id was not found")
		return "", false
	}
	return conversationID, true
}

func agodeskStateIsDevMode(state *agodeskConnectionState) bool {
	if state == nil {
		return false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.devMode
}

func sendAgodeskCurrentPlanSnapshot(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID, sessionID, conversationID string) (interface{}, bool) {
	if !agodeskStateHasCapability(state, "chat.plan_updates") || s == nil || s.ShortTermMem == nil {
		return nil, false
	}
	plan, err := s.ShortTermMem.GetSessionPlan(conversationID)
	if err != nil || plan == nil {
		if err != nil && s.Logger != nil {
			s.Logger.Warn("Failed to fetch agodesk session plan", "session_id", conversationID, "error", err)
		}
		return nil, false
	}
	raw, planValue, ok := agodeskPlanSnapshotPayload(plan)
	if !ok {
		return nil, false
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatPlanUpdate, agodesk.ChatPlanUpdatePayload{
		SessionID:      sessionID,
		ConversationID: conversationID,
		RequestID:      requestID,
		Plan:           raw,
	})
	return planValue, true
}

func agodeskPlanSnapshotPayload(plan interface{}) (json.RawMessage, interface{}, bool) {
	raw, err := json.Marshal(plan)
	if err != nil {
		return nil, nil, false
	}
	if len(raw) == 0 {
		raw = json.RawMessage(`null`)
	}
	var planValue interface{}
	if err := json.Unmarshal(raw, &planValue); err != nil {
		return nil, nil, false
	}
	return raw, planValue, true
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
	serverCapabilities := agodeskServerCapabilities(s)
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
		SessionID:              "agodesk:" + deviceID,
		DeviceID:               deviceID,
		Approved:               true,
		ReadOnly:               readOnly,
		Capabilities:           serverCapabilities,
		AdvertisedCapabilities: agodesk.NegotiateCapabilities(payload.ClientCapabilities, serverCapabilities),
		SharedKey:              sharedKey,
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
	if device.Status != "approved" && device.Status != "connected" && device.Status != "offline" {
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorDeviceNotApproved, "device is not paired"
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
	serverCapabilities := agodeskServerCapabilities(s)
	device.Hostname = strings.TrimSpace(payload.Host.Hostname)
	device.OS = strings.TrimSpace(payload.Host.OS)
	device.Arch = strings.TrimSpace(payload.Host.Arch)
	if err := remote.UpdateDevice(s.RemoteHub.DB(), device); err != nil && s.Logger != nil {
		s.Logger.Warn("Failed to update agodesk reconnect device metadata", "device_id", deviceID, "error", err)
	}
	return agodesk.SessionAcceptedPayload{
		SessionID:              "agodesk:" + deviceID,
		DeviceID:               deviceID,
		Approved:               true,
		ReadOnly:               device.ReadOnly,
		Capabilities:           serverCapabilities,
		AdvertisedCapabilities: agodesk.NegotiateCapabilities(payload.ClientCapabilities, serverCapabilities),
	}, "", ""
}

type agodeskChatBroker struct {
	agent.FeedbackBroker
	conn           *websocket.Conn
	state          *agodeskConnectionState
	sessionID      string
	conversationID string
	requestID      string
	logger         *slog.Logger

	mu             sync.RWMutex
	latestPlan     interface{}
	latestPlanSeen bool
}

func (b *agodeskChatBroker) Send(event, message string) {
	if event == "plan_update" {
		b.capturePlanUpdate(message)
	}
	if event == "audio" {
		b.emitAudio(message)
	}
	if b.FeedbackBroker != nil {
		b.FeedbackBroker.Send(event, message)
	}
}

func (b *agodeskChatBroker) capturePlanUpdate(message string) {
	if b == nil {
		return
	}
	var payload struct {
		Plan json.RawMessage `json:"plan"`
	}
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		if b.logger != nil {
			b.logger.Warn("Failed to decode agodesk plan_update payload", "session_id", b.sessionID, "error", err)
		}
		return
	}
	if len(payload.Plan) == 0 {
		payload.Plan = json.RawMessage(`null`)
	}
	var planValue interface{}
	if err := json.Unmarshal(payload.Plan, &planValue); err != nil {
		if b.logger != nil {
			b.logger.Warn("Failed to decode agodesk plan snapshot", "session_id", b.sessionID, "error", err)
		}
		return
	}
	b.mu.Lock()
	b.latestPlan = planValue
	b.latestPlanSeen = true
	b.mu.Unlock()
	if agodeskStateHasCapability(b.state, "chat.plan_updates") {
		_ = writeAgodeskEnvelopeLocked(b.conn, b.state, agodesk.TypeChatPlanUpdate, agodesk.ChatPlanUpdatePayload{
			SessionID:      b.sessionID,
			ConversationID: b.conversationID,
			RequestID:      b.requestID,
			Plan:           payload.Plan,
		})
	}
}

func (b *agodeskChatBroker) emitAudio(message string) {
	if b == nil || !agodeskStateHasCapability(b.state, "chat.audio_events") {
		return
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(message), &raw); err != nil {
		if b.logger != nil {
			b.logger.Warn("Failed to decode agodesk audio payload", "session_id", b.conversationID, "error", err)
		}
		return
	}
	payload := agodesk.ChatAudioPayload{
		SessionID:      strings.TrimSpace(b.sessionID),
		ConversationID: strings.TrimSpace(b.conversationID),
		RequestID:      strings.TrimSpace(b.requestID),
		Path:           agodeskStringField(raw, "path", "url"),
		Title:          agodeskStringField(raw, "title"),
		MimeType:       agodeskStringField(raw, "mime_type", "content_type"),
		Filename:       agodeskStringField(raw, "filename", "file_name"),
	}
	if metadata, ok := raw["metadata"].(map[string]interface{}); ok {
		payload.Metadata = metadata
	}
	if payload.Path == "" && payload.Filename == "" {
		return
	}
	_ = writeAgodeskEnvelopeLocked(b.conn, b.state, agodesk.TypeChatAudio, payload)
}

func agodeskStringField(raw map[string]interface{}, names ...string) string {
	for _, name := range names {
		value, ok := raw[name]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func (b *agodeskChatBroker) latestPlanSnapshot() (interface{}, bool) {
	if b == nil {
		return nil, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.latestPlan, b.latestPlanSeen
}

func runAgodeskAgentChat(s *Server, r *http.Request, conn *websocket.Conn, state *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
	if s == nil {
		return agodeskChatResult{}, fmt.Errorf("server not configured")
	}
	turn, err := prepareDesktopAgentTurnWithOptions(r.Context(), s, message, desktopChatContext{}, false, desktopAgentTurnOptions{
		SessionID:         conversationID,
		MessageSource:     agodeskMessageSource,
		AdditionalPrompt:  buildAgodeskAgentContext(deviceID),
		VoiceOutputActive: voiceOutput,
	})
	if err != nil {
		return agodeskChatResult{}, err
	}
	replyBroker := &desktopReplyBroker{FeedbackBroker: NewSSEBrokerAdapterWithSession(s.SSE, conversationID)}
	broker := &agodeskChatBroker{
		FeedbackBroker: replyBroker,
		conn:           conn,
		state:          state,
		sessionID:      transportSessionID,
		conversationID: conversationID,
		requestID:      requestID,
		logger:         s.Logger,
	}
	ctx, cancel := context.WithTimeout(r.Context(), desktopChatAgentTurnTimeout)
	defer cancel()
	var resp openai.ChatCompletionResponse
	var loopErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, loopErr = agent.ExecuteAgentLoop(ctx, turn.req, turn.runCfg, false, broker)
		if loopErr != nil {
			broker.Send("error_recovery", chatCompletionErrorMessage(desktopUILanguage(s), loopErr))
		}
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return agodeskChatResult{}, fmt.Errorf("agent request timed out")
	}
	answer := strings.TrimSpace(replyBroker.finalResponse)
	if answer == "" && len(resp.Choices) > 0 {
		answer = strings.TrimSpace(resp.Choices[0].Message.Content)
	}
	touchDesktopChatSessionMetadata(s, conversationID)
	metadata := make(map[string]interface{})
	if agodeskStateHasCapability(state, "chat.agent_metadata") {
		mood := buildAgodeskAgentMoodMetadata(s)
		if len(mood) > 0 {
			metadata["agent_mood"] = mood
		}
	}
	if agodeskStateHasCapability(state, "chat.plan_updates") {
		if plan, ok := broker.latestPlanSnapshot(); ok {
			metadata["plan"] = plan
		}
	}
	return agodeskChatResult{
		Answer:   security.StripThinkingTags(answer),
		Metadata: metadata,
	}, nil
}

func buildAgodeskAgentMoodMetadata(s *Server) map[string]interface{} {
	if s == nil || s.ShortTermMem == nil {
		return nil
	}
	if latest, err := s.ShortTermMem.GetLatestEmotion(); err == nil && latest != nil {
		mood := strings.TrimSpace(latest.PrimaryMood)
		metadata := map[string]interface{}{
			"mood":                       mood,
			"primary_mood":               mood,
			"valence":                    latest.Valence,
			"arousal":                    latest.Arousal,
			"confidence":                 latest.Confidence,
			"recommended_response_style": strings.TrimSpace(latest.RecommendedResponseStyle),
			"source":                     "emotion_history",
			"timestamp":                  strings.TrimSpace(latest.Timestamp),
		}
		if secondary := strings.TrimSpace(latest.SecondaryMood); secondary != "" {
			metadata["secondary_mood"] = secondary
		}
		if description := sanitizeEmotionPreview(latest.Description, 220); description != "" {
			metadata["description"] = description
		}
		return metadata
	}
	mood := strings.TrimSpace(string(s.ShortTermMem.GetCurrentMood()))
	if mood == "" {
		return nil
	}
	return map[string]interface{}{
		"mood":         mood,
		"primary_mood": mood,
		"source":       "mood_log",
	}
}

func buildAgodeskAgentContext(deviceID string) string {
	lines := []string{
		"The user is chatting from agodesk, a paired desktop companion running on a remote PC.",
		"When the user asks about that remote PC, prefer the remote_control tool for available device operations and respect read-only policy.",
		"Desktop screenshots, display/window discovery, active-window metadata, UI tree reads, and browser snapshots are available through remote_control when the agodesk client advertises the matching capabilities.",
		"If desktop screenshot or permission requests return UNSUPPORTED_CAPABILITY, explain that the client is connected for chat but does not advertise remote-control support.",
		"Desktop input, UI actions, and browser actions require local approval in the agodesk remote-control banner; the backend cannot approve or bypass that local control session.",
		"Desktop streaming is not available in this backend version.",
	}
	if id := strings.TrimSpace(deviceID); id != "" {
		lines = append(lines,
			fmt.Sprintf("Paired agodesk remote_control device_id: %q. Always pass this device_id on remote_control operations for the user's PC.", id),
		)
	}
	return strings.Join(lines, "\n")
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
	broker.RegisterSession(accepted.DeviceID, conn, state, accepted.AdvertisedCapabilities)
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

func agodeskStateHasCapability(state *agodeskConnectionState, capability string) bool {
	if state == nil || strings.TrimSpace(capability) == "" {
		return false
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	_, ok := state.capabilities[capability]
	return ok
}

func (b *agodeskDesktopBroker) RegisterSession(deviceID string, conn *websocket.Conn, state *agodeskConnectionState, clientCapabilities []string) {
	if b == nil || strings.TrimSpace(deviceID) == "" || conn == nil || state == nil {
		return
	}
	session := &agodeskDesktopSession{
		deviceID:     deviceID,
		conn:         conn,
		state:        state,
		capabilities: normalizeAgodeskCapabilities(clientCapabilities),
		pending:      make(map[string]chan agodeskDesktopCommandResult),
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
	if cmd.Operation == remote.OpAgoDeskChatMessage {
		return session.sendChatMessage(cmd)
	}
	requiredCapability := agodeskDesktopCapabilityForOperation(cmd.Operation)
	if requiredCapability == "" {
		return remote.ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "error",
			Error:     fmt.Sprintf("%s: agodesk desktop transport does not support operation %q", agodesk.ErrorUnsupportedCapability, cmd.Operation),
		}, nil
	}
	if !session.hasCapability(requiredCapability) {
		return remote.ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "error",
			Error:     fmt.Sprintf("%s: agodesk client does not advertise %s for %s", agodesk.ErrorUnsupportedCapability, requiredCapability, cmd.Operation),
		}, nil
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

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-resultCh:
		if result.err != nil {
			return remote.ResultPayload{CommandID: cmd.CommandID, Status: "error", Error: result.err.Error()}, nil
		}
		return agodeskDesktopResultToRemoteResult(cmd.CommandID, result.payload), nil
	case <-timer.C:
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

func (s *agodeskDesktopSession) sendChatMessage(cmd remote.CommandPayload) (remote.ResultPayload, error) {
	rawMessage, ok := cmd.Args["message"]
	if !ok {
		return remote.ResultPayload{CommandID: cmd.CommandID, Status: "error", Error: "message is required"}, nil
	}
	message := strings.TrimSpace(fmt.Sprint(rawMessage))
	if message == "" {
		return remote.ResultPayload{CommandID: cmd.CommandID, Status: "error", Error: "message is required"}, nil
	}
	if !s.hasCapability("chat.full_response") {
		return remote.ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "error",
			Error:     fmt.Sprintf("%s: agodesk client does not advertise chat.full_response", agodesk.ErrorUnsupportedCapability),
		}, nil
	}
	sessionID := ""
	if s.state != nil {
		s.state.mu.RLock()
		sessionID = s.state.sessionID
		s.state.mu.RUnlock()
	}
	if sessionID == "" {
		sessionID = "agodesk:" + s.deviceID
	}
	conversationID := strings.TrimSpace(fmt.Sprint(cmd.Args["conversation_id"]))
	if conversationID == "<nil>" {
		conversationID = ""
	}
	if err := writeAgodeskEnvelopeLocked(s.conn, s.state, agodesk.TypeChatResponse, agodesk.ChatResponsePayload{
		SessionID:      sessionID,
		ConversationID: conversationID,
		RequestID:      cmd.CommandID,
		Text:           message,
		Role:           "assistant",
		Metadata: map[string]interface{}{
			"source":      "aurago_agent",
			"server_push": true,
		},
	}); err != nil {
		return remote.ResultPayload{}, fmt.Errorf("send agodesk chat message: %w", err)
	}
	return remote.ResultPayload{
		CommandID: cmd.CommandID,
		Status:    "ok",
		Output:    `{"sent":true}`,
	}, nil
}

func (s *agodeskDesktopSession) hasCapability(capability string) bool {
	if s == nil || strings.TrimSpace(capability) == "" {
		return false
	}
	_, ok := s.capabilities[capability]
	return ok
}

func agodeskDesktopCapabilityForOperation(operation string) string {
	switch operation {
	case remote.OpDesktopScreenshot:
		return "remote.desktop.capture"
	case remote.OpDesktopPermissionRequest:
		return "remote.desktop.permission_request"
	case remote.OpDesktopInput:
		return "remote.desktop.input"
	case remote.OpDesktopListDisplays, remote.OpDesktopListWindows, remote.OpDesktopActiveWindow, remote.OpDesktopHostInfo:
		return "remote.desktop.discovery"
	case remote.OpDesktopUITree, remote.OpDesktopUIAction:
		return "remote.desktop.ui_automation"
	case remote.OpDesktopBrowserConnect, remote.OpDesktopBrowserSnapshot, remote.OpDesktopBrowserAction, remote.OpDesktopBrowserDisconnect:
		return "remote.desktop.browser"
	case remote.OpFileRead, remote.OpFileList:
		return "remote.files.read"
	case remote.OpFileWrite:
		return "remote.files.write"
	default:
		return ""
	}
}

func normalizeAgodeskCapabilities(capabilities []string) map[string]struct{} {
	normalized := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		capability = strings.TrimSpace(capability)
		if capability != "" {
			normalized[capability] = struct{}{}
		}
	}
	return normalized
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
	if !payload.Succeeded() {
		status = "error"
	}
	output := ""
	if payload.Data != nil {
		if content, ok := payload.Data["content"].(string); ok {
			output = content
		} else if files, ok := payload.Data["files"]; ok {
			if data, err := json.Marshal(files); err == nil {
				output = string(data)
			}
		} else {
			if data, err := json.Marshal(payload.Data); err == nil {
				output = string(data)
			}
		}
	}
	if payload.CommandID != "" {
		commandID = payload.CommandID
	}
	errorMessage := payload.Error
	if errorMessage == "" {
		errorMessage = payload.ErrorCode
	}
	return remote.ResultPayload{
		CommandID: commandID,
		Status:    status,
		Output:    output,
		Error:     errorMessage,
	}
}

func decodeAgodeskEnvelope(data []byte) (agodesk.Envelope, error) {
	env, err := agodesk.DecodeEnvelope(data, agodeskWebSocketReadLimitBytes)
	if err != nil {
		return agodesk.Envelope{}, err
	}
	if env.Type != agodesk.TypeDesktopResult && len(data) > agodeskControlMessageMaxBytes {
		return agodesk.Envelope{}, fmt.Errorf("message too large: %d bytes exceeds %d for %s", len(data), agodeskControlMessageMaxBytes, env.Type)
	}
	return env, nil
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
