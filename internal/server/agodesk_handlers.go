package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/agodesk"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/warnings"
	promptsembed "aurago/prompts"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

const (
	agodeskMessageSource           = "agodesk_chat"
	agodeskControlMessageMaxBytes  = 256 * 1024
	agodeskDesktopResultMaxBytes   = 16 * 1024 * 1024
	agodeskWebSocketReadLimitBytes = agodeskDesktopResultMaxBytes
	agodeskMediaAssetTokenTTL      = 15 * time.Minute
	agodeskMediaAssetExpParam      = "agodesk_exp"
	agodeskMediaAssetSigParam      = "agodesk_sig"
	agodeskFileAccessInlineLimit   = 8 * 1024 * 1024
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

var agodeskDoneTagPattern = regexp.MustCompile(`(?i)<\s*/?\s*done\s*/?\s*>`)

var errAgodeskAgentTimeout = errors.New("agent request timed out")

type agodeskConnectionState struct {
	sessionID             string
	deviceID              string
	paired                bool
	readOnly              bool
	devMode               bool
	capabilities          map[string]struct{}
	fileAccess            *agodesk.FileAccessPayload
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
		state.fileAccess = normalizeAgodeskFileAccessPayload(payload.FileAccess)
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
	case agodesk.TypeChatAttachmentPrepare:
		payload, errPayload := decodeAgodeskPayload[agodesk.ChatAttachmentPreparePayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskAttachmentPrepare(s, conn, state, env.ID, payload)
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
	case agodesk.TypeIntegrationsWebhostsList:
		payload, errPayload := decodeAgodeskPayload[agodesk.IntegrationsWebhostsListPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskIntegrationsWebhostsList(s, r, conn, state, env.ID, payload)
	case agodesk.TypeSystemWarningsList:
		payload, errPayload := decodeAgodeskPayload[agodesk.SystemWarningsListPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskSystemWarningsList(s, conn, state, env.ID, payload)
	case agodesk.TypeSystemWarningAcknowledge:
		payload, errPayload := decodeAgodeskPayload[agodesk.SystemWarningAcknowledgePayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskSystemWarningAcknowledge(s, conn, state, env.ID, payload)
	case agodesk.TypePersonaAssetsRequest:
		payload, errPayload := decodeAgodeskPayload[agodesk.PersonaAssetsRequestPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskPersonaAssetsRequest(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderCatalogList:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderCatalogListPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderCatalogList(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderCatalogDetail:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderCatalogDetailPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderCatalogDetail(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProvidersList:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProvidersListPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProvidersList(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderGet:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderGetPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderGet(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderUpsert:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderUpsertPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderUpsert(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderDelete:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderDeletePayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderDelete(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderTest:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderTestPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderTest(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderOAuthStart:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderOAuthStartPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderOAuthStart(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderOAuthComplete:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderOAuthCompletePayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderOAuthComplete(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderOAuthStatusRequest:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderOAuthStatusRequestPayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderOAuthStatus(s, conn, state, env.ID, payload)
	case agodesk.TypeConfigProviderOAuthRevoke:
		payload, errPayload := decodeAgodeskPayload[agodesk.ConfigProviderOAuthRevokePayload](env)
		if errPayload != nil {
			_ = writeAgodeskErrorLocked(conn, state, env.ID, agodesk.ErrorInvalidMessage, errPayload.Error())
			return true
		}
		handleAgodeskProviderOAuthRevoke(s, conn, state, env.ID, payload)
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
	if !ok || !validateAgodeskCapability(conn, state, requestID, "chat.sessions", "chat.sessions.list") {
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
	if !ok || !validateAgodeskCapability(conn, state, requestID, "chat.sessions", "chat.session.create") {
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
	if !ok || !validateAgodeskCapability(conn, state, requestID, "chat.sessions", "chat.session.load") {
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
		Messages:       agodeskHistoryMessages(messages, agodeskHistoryAttachmentMap(s, messages)),
	})
}

func handleAgodeskChatCancel(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatCancelPayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.cancel")
	if !ok || !validateAgodeskCapability(conn, state, requestID, "chat.cancel", "chat.cancel") {
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
	if found {
		emitAgodeskRunActivity(conn, state, sessionID, conversationID, activeRequestID, "cancelled", "Agent cancelled", s.Logger)
	}
}

func handleAgodeskVoiceOutputStatus(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatVoiceOutputStatusPayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.voice_output.status")
	if !ok || !validateAgodeskCapability(conn, state, requestID, "chat.voice_output_status", "chat.voice_output.status") {
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

func handleAgodeskIntegrationsWebhostsList(s *Server, r *http.Request, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.IntegrationsWebhostsListPayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "integrations.webhosts.list")
	if !ok || !validateAgodeskCapability(conn, state, requestID, "integrations.webhosts", "integrations.webhosts.list") {
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeIntegrationsWebhosts, agodesk.IntegrationsWebhostsPayload{
		SessionID: sessionID,
		Status:    "ok",
		Webhosts:  agodeskWebhostIntegrationPayloads(integrationWebhostsForRequest(s, r)),
	})
}

func handleAgodeskSystemWarningsList(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.SystemWarningsListPayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "system.warnings.list")
	if !ok || !validateAgodeskCapability(conn, state, requestID, "system.warnings", "system.warnings.list") {
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeSystemWarnings, agodeskSystemWarningsPayload(s, sessionID))
}

func handleAgodeskSystemWarningAcknowledge(s *Server, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.SystemWarningAcknowledgePayload) {
	sessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "system.warning.acknowledge")
	if !ok || !validateAgodeskCapability(conn, state, requestID, "system.warnings", "system.warning.acknowledge") {
		return
	}
	if s == nil || s.WarningsRegistry == nil {
		_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeSystemWarnings, agodeskSystemWarningsPayload(s, sessionID))
		return
	}
	if payload.All {
		s.WarningsRegistry.AcknowledgeAll()
	} else if id := strings.TrimSpace(payload.ID); id != "" {
		if !s.WarningsRegistry.Acknowledge(id) {
			_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, "system.warning.acknowledge id was not found")
			return
		}
	} else {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, "system.warning.acknowledge requires id or all:true")
		return
	}
	_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeSystemWarnings, agodeskSystemWarningsPayload(s, sessionID))
	broadcastAgodeskSystemWarningsExcept(s, state)
	if s != nil && s.SSE != nil {
		total, unack := s.WarningsRegistry.Count()
		s.SSE.BroadcastType(EventSystemWarning, map[string]interface{}{
			"total":          total,
			"unacknowledged": unack,
		})
	}
}

func validateAgodeskCapability(conn *websocket.Conn, state *agodeskConnectionState, requestID, capability, messageName string) bool {
	if agodeskStateHasCapability(state, capability) {
		return true
	}
	_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorUnsupportedCapability, messageName+" requires "+capability)
	return false
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

func agodeskHistoryMessages(messages []memory.HistoryMessage, attachmentMaps ...map[int64][]agodesk.ChatAttachmentItem) []agodesk.ChatHistoryMessagePayload {
	attachmentsByMessageID := map[int64][]agodesk.ChatAttachmentItem{}
	if len(attachmentMaps) > 0 && attachmentMaps[0] != nil {
		attachmentsByMessageID = attachmentMaps[0]
	}
	out := make([]agodesk.ChatHistoryMessagePayload, 0, len(messages))
	for _, msg := range messages {
		if msg.IsInternal {
			continue
		}
		out = append(out, agodesk.ChatHistoryMessagePayload{
			Role:        strings.TrimSpace(msg.Role),
			Content:     stripAgodeskAttachmentBlock(strings.TrimSpace(msg.Content)),
			Timestamp:   strings.TrimSpace(msg.Timestamp),
			Attachments: append([]agodesk.ChatAttachmentItem(nil), attachmentsByMessageID[msg.ID]...),
		})
	}
	return out
}

func agodeskHistoryAttachmentMap(s *Server, messages []memory.HistoryMessage) map[int64][]agodesk.ChatAttachmentItem {
	out := map[int64][]agodesk.ChatAttachmentItem{}
	if s == nil || s.ShortTermMem == nil || len(messages) == 0 {
		return out
	}
	ids := make([]int64, 0, len(messages))
	for _, msg := range messages {
		if msg.ID > 0 && !msg.IsInternal {
			ids = append(ids, msg.ID)
		}
	}
	recordsByMessage, err := s.ShortTermMem.ListAgoDeskAttachmentsForMessages(ids)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("Failed to load agodesk chat attachments for history", "error", err)
		}
		return out
	}
	for messageID, records := range recordsByMessage {
		items := make([]agodesk.ChatAttachmentItem, 0, len(records))
		for _, record := range records {
			items = append(items, agodeskChatAttachmentItem(s, record))
		}
		out[messageID] = items
	}
	return out
}

func agodeskWebhostIntegrationPayloads(webhosts []webhostIntegration) []agodesk.WebhostIntegrationPayload {
	out := make([]agodesk.WebhostIntegrationPayload, 0, len(webhosts))
	for _, item := range webhosts {
		out = append(out, agodesk.WebhostIntegrationPayload{
			ID:          strings.TrimSpace(item.ID),
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
			Status:      strings.TrimSpace(item.Status),
			URL:         strings.TrimSpace(item.URL),
			Icon:        strings.TrimSpace(item.Icon),
		})
	}
	return out
}

func agodeskSystemWarningsPayload(s *Server, sessionID string) agodesk.SystemWarningsPayload {
	payload := agodesk.SystemWarningsPayload{
		SessionID: strings.TrimSpace(sessionID),
		Warnings:  []agodesk.SystemWarningPayload{},
	}
	if s == nil || s.WarningsRegistry == nil {
		return payload
	}
	all := s.WarningsRegistry.Warnings()
	total, unack := s.WarningsRegistry.Count()
	payload.Total = total
	payload.Unacknowledged = unack
	payload.Warnings = make([]agodesk.SystemWarningPayload, 0, len(all))
	for _, warning := range all {
		payload.Warnings = append(payload.Warnings, agodeskSystemWarningPayload(warning))
	}
	return payload
}

func agodeskSystemWarningPayload(warning warnings.Warning) agodesk.SystemWarningPayload {
	timestamp := ""
	if !warning.Timestamp.IsZero() {
		timestamp = warning.Timestamp.UTC().Format(time.RFC3339)
	}
	return agodesk.SystemWarningPayload{
		ID:           strings.TrimSpace(warning.ID),
		Severity:     strings.TrimSpace(warning.Severity),
		Title:        strings.TrimSpace(warning.Title),
		Description:  strings.TrimSpace(warning.Description),
		Category:     strings.TrimSpace(warning.Category),
		Timestamp:    timestamp,
		Acknowledged: warning.Acknowledged,
	}
}

func broadcastAgodeskSystemWarnings(s *Server) {
	broadcastAgodeskSystemWarningsExcept(s, nil)
}

func broadcastAgodeskSystemWarningsExcept(s *Server, skipState *agodeskConnectionState) {
	broker := currentAgodeskDesktopBroker(s)
	if broker == nil {
		return
	}
	for _, session := range broker.sessionsWithCapability("system.warnings") {
		if skipState != nil && session != nil && session.state == skipState {
			continue
		}
		sessionID := agodeskSessionTransportID(session)
		_ = writeAgodeskEnvelopeLocked(session.conn, session.state, agodesk.TypeSystemWarnings, agodeskSystemWarningsPayload(s, sessionID))
	}
}

func currentAgodeskDesktopBroker(s *Server) *agodeskDesktopBroker {
	if s == nil {
		return nil
	}
	s.agodeskDesktopMu.Lock()
	defer s.agodeskDesktopMu.Unlock()
	return s.agodeskDesktop
}

func agodeskServerCapabilities(s *Server) []string {
	capabilities := append([]string(nil), agodesk.DefaultCapabilities...)
	if agodeskRemoteShellEnabled(s) {
		capabilities = append(capabilities, "remote.shell.exec", "remote.shell.session")
	}
	if agodeskServerTTSConfigured(s) {
		capabilities = append(capabilities, "chat.voice_output")
	}
	if agodeskAttachmentUploadsEnabled(s) {
		capabilities = append(capabilities, "chat.media_upload", "chat.attachments")
	}
	if agodeskProviderManagementReadable(s) {
		capabilities = append(capabilities, agodesk.CapabilityConfigProvidersRead)
		if agodeskProviderManagementWritable(s) {
			capabilities = append(capabilities, agodesk.CapabilityConfigProvidersWrite, agodesk.CapabilityConfigProvidersOAuth)
		}
	}
	return capabilities
}

func agodeskRemoteShellEnabled(s *Server) bool {
	if s == nil || s.Cfg == nil {
		return false
	}
	s.CfgMu.RLock()
	enabled := s.Cfg.Agent.AllowRemoteShell
	s.CfgMu.RUnlock()
	return enabled
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
	case "supertonic":
		return strings.TrimSpace(cfg.TTS.Supertonic.URL) != ""
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
	if !validateAgodeskCapability(conn, state, requestID, "persona.assets", "persona.assets.request") {
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

func sanitizeAgodeskChatResponseText(text string) string {
	text = security.StripThinkingTags(text)
	text = agodeskDoneTagPattern.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func handleAgodeskChatMessage(s *Server, r *http.Request, conn *websocket.Conn, state *agodeskConnectionState, requestID string, payload agodesk.ChatMessagePayload) {
	deviceID := ""
	if state != nil {
		state.mu.RLock()
		deviceID = state.deviceID
		state.mu.RUnlock()
	}
	message := strings.TrimSpace(payload.Text)
	if message == "" && len(payload.Attachments) == 0 {
		_ = writeAgodeskErrorLocked(conn, state, requestID, agodesk.ErrorInvalidMessage, "chat.message text or attachments are required")
		return
	}
	transportSessionID, ok := validateAgodeskTransportSession(s, conn, state, requestID, payload.SessionID, "chat.message")
	if !ok {
		return
	}
	if len(payload.Attachments) > 0 && !validateAgodeskCapability(conn, state, requestID, "chat.attachments", "chat.message attachments") {
		return
	}
	conversationID, ok := resolveAgodeskConversationID(s, conn, state, requestID, transportSessionID, strings.TrimSpace(payload.ConversationID))
	if !ok {
		return
	}
	attachmentRecords, attachmentItems, attachmentErrCode, attachmentErrMsg := agodeskResolveChatAttachments(s, state, transportSessionID, conversationID, payload.Attachments)
	if attachmentErrCode != "" {
		_ = writeAgodeskErrorLocked(conn, state, requestID, attachmentErrCode, attachmentErrMsg)
		return
	}
	message = buildAgodeskMessageWithAttachments(s, message, attachmentRecords)
	unlockSession := lockSessionRequest(conversationID)
	defer unlockSession()

	chatCtx, cancel := context.WithCancel(r.Context())
	chatCtx = contextWithAgodeskAttachmentBinding(chatCtx, s, conversationID, attachmentRecords)
	defer cancel()
	registerAgodeskActiveRun(state, requestID, conversationID, cancel)
	defer unregisterAgodeskActiveRun(state, requestID)

	initialPlan, hasInitialPlan := sendAgodeskCurrentPlanSnapshot(s, conn, state, requestID, transportSessionID, conversationID)
	voiceOutput := payload.VoiceOutput && agodeskStateHasCapability(state, "chat.voice_output")
	if len(attachmentItems) > 0 {
		_ = writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeChatAttachmentAccepted, agodesk.ChatAttachmentAcceptedPayload{
			SessionID:      transportSessionID,
			ConversationID: conversationID,
			Attachments:    attachmentItems,
		})
	}
	result, err := agodeskAgentChatRunner(s, r.WithContext(chatCtx), conn, state, requestID, transportSessionID, conversationID, deviceID, message, voiceOutput)
	if err != nil {
		if chatCtx.Err() != nil {
			return
		}
		code := agodesk.ErrorInternal
		if errors.Is(err, errAgodeskAgentTimeout) {
			code = agodesk.ErrorAgentTimeout
		}
		_ = writeAgodeskErrorLocked(conn, state, requestID, code, err.Error())
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
		Text:           sanitizeAgodeskChatResponseText(result.Answer),
		Role:           "assistant",
		Metadata:       metadata,
	})
}

func handleAgodeskTTSAsset(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		filename := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/agodesk/tts/"))
		if !isSafeAgodeskTTSFilename(filename) {
			http.NotFound(w, r)
			return
		}
		dataDir := ""
		if s != nil && s.Cfg != nil {
			s.CfgMu.RLock()
			dataDir = s.Cfg.Directories.DataDir
			s.CfgMu.RUnlock()
		}
		ttsDir := tools.TTSAudioDir(dataDir)
		target := filepath.Join(ttsDir, filename)
		if !pathStaysWithinDir(ttsDir, target) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", chatVoiceAudioMIMEType(filename))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, target)
	}
}

func handleAgodeskMediaAsset(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !verifyAgodeskMediaAssetSignature(s, r, time.Now()) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		bucket, relPath, ok := agodeskMediaAssetRequestPath(r)
		if !ok {
			http.NotFound(w, r)
			return
		}
		root, ok := agodeskMediaAssetRoot(s, bucket)
		if !ok {
			http.NotFound(w, r)
			return
		}
		target := filepath.Join(root, relPath)
		if !pathStaysWithinDir(root, target) {
			http.NotFound(w, r)
			return
		}
		if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(target))); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "private, max-age=900")
		if bucket == "documents" {
			filename := filepath.Base(target)
			disposition := "attachment"
			if r.URL.Query().Get("inline") == "1" {
				disposition = "inline"
			}
			w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, filename))
		}
		http.ServeFile(w, r, target)
	}
}

func verifyAgodeskMediaAssetSignature(s *Server, r *http.Request, now time.Time) bool {
	secret := agodeskMediaAssetSigningSecret(s)
	if secret == "" || r == nil || r.URL == nil {
		return false
	}
	q := r.URL.Query()
	expRaw := strings.TrimSpace(q.Get(agodeskMediaAssetExpParam))
	sig := strings.TrimSpace(q.Get(agodeskMediaAssetSigParam))
	if expRaw == "" || sig == "" {
		return false
	}
	expires, err := strconv.ParseInt(expRaw, 10, 64)
	if err != nil || expires <= 0 || now.Unix() > expires {
		return false
	}
	q.Del(agodeskMediaAssetSigParam)
	expected := agodeskMediaAssetSignature(secret, agodeskMediaAssetSignatureMaterial(r.URL.EscapedPath(), q))
	return hmac.Equal([]byte(strings.ToLower(sig)), []byte(expected))
}

func signAgodeskMediaAssetPath(s *Server, pathValue string, now time.Time) string {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" || !strings.HasPrefix(pathValue, "/api/agodesk/media/") {
		return pathValue
	}
	secret := agodeskMediaAssetSigningSecret(s)
	if secret == "" {
		return pathValue
	}
	parsed, err := url.Parse(pathValue)
	if err != nil || parsed.Path == "" {
		return pathValue
	}
	q := parsed.Query()
	q.Set(agodeskMediaAssetExpParam, strconv.FormatInt(now.Add(agodeskMediaAssetTokenTTL).Unix(), 10))
	q.Del(agodeskMediaAssetSigParam)
	sig := agodeskMediaAssetSignature(secret, agodeskMediaAssetSignatureMaterial(parsed.EscapedPath(), q))
	q.Set(agodeskMediaAssetSigParam, sig)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func agodeskMediaAssetSigningSecret(s *Server) string {
	if s == nil || s.Cfg == nil {
		return ""
	}
	s.CfgMu.RLock()
	secret := strings.TrimSpace(s.Cfg.Auth.SessionSecret)
	s.CfgMu.RUnlock()
	return secret
}

func agodeskMediaAssetSignature(secret, material string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(material))
	return hex.EncodeToString(mac.Sum(nil))
}

func agodeskMediaAssetSignatureMaterial(escapedPath string, q url.Values) string {
	return escapedPath + "\n" + q.Encode()
}

func agodeskMediaAssetRequestPath(r *http.Request) (string, string, bool) {
	if r == nil || r.URL == nil {
		return "", "", false
	}
	rel := strings.TrimPrefix(r.URL.EscapedPath(), "/api/agodesk/media/")
	if rel == "" || rel == r.URL.EscapedPath() {
		return "", "", false
	}
	rel, err := url.PathUnescape(rel)
	if err != nil {
		return "", "", false
	}
	bucket, rest, ok := strings.Cut(rel, "/")
	bucket = strings.TrimSpace(bucket)
	if !ok || !agodeskMediaBucketAllowed(bucket) || strings.TrimSpace(rest) == "" {
		return "", "", false
	}
	for _, segment := range strings.Split(rest, "/") {
		if segment == ".." || strings.Contains(segment, "\x00") {
			return "", "", false
		}
	}
	clean := pathpkg.Clean("/" + rest)
	if clean == "/" || strings.Contains(clean, "\x00") {
		return "", "", false
	}
	clean = strings.TrimPrefix(clean, "/")
	if strings.HasPrefix(clean, "../") || clean == ".." {
		return "", "", false
	}
	return bucket, filepath.FromSlash(clean), true
}

func agodeskMediaAssetRoot(s *Server, bucket string) (string, bool) {
	if s == nil || s.Cfg == nil {
		return "", false
	}
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	switch bucket {
	case "images":
		workspaceDir := strings.TrimSpace(cfg.Directories.WorkspaceDir)
		if workspaceDir == "" {
			return "", false
		}
		return filepath.Join(workspaceDir, "images"), true
	case "attachments":
		workspaceDir := strings.TrimSpace(cfg.Directories.WorkspaceDir)
		if workspaceDir == "" {
			return "", false
		}
		return filepath.Join(workspaceDir, "attachments"), true
	}
	dataDir := strings.TrimSpace(cfg.Directories.DataDir)
	if dataDir == "" {
		return "", false
	}
	switch bucket {
	case "generated_images":
		return filepath.Join(dataDir, "generated_images"), true
	case "generated_videos":
		return filepath.Join(dataDir, "generated_videos"), true
	case "audio":
		return filepath.Join(dataDir, "audio"), true
	case "documents":
		docDir := strings.TrimSpace(cfg.Tools.DocumentCreator.OutputDir)
		if docDir == "" {
			docDir = filepath.Join(dataDir, "documents")
		}
		return docDir, true
	case "downloads":
		downloadDir, err := tools.ResolveVideoDownloadDir(&cfg)
		if err != nil || strings.TrimSpace(downloadDir) == "" {
			downloadDir = filepath.Join(dataDir, "downloads")
		}
		return downloadDir, true
	default:
		return "", false
	}
}

func isSafeAgodeskTTSFilename(filename string) bool {
	if filename == "" || filename != filepath.Base(filename) {
		return false
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, `\`) || strings.Contains(filename, "..") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp3", ".wav", ".ogg", ".flac":
		return true
	default:
		return false
	}
}

func pathStaysWithinDir(root, target string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
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
	name := strings.TrimSpace(enrollment.DeviceName)
	if name == "" {
		name = strings.TrimSpace(payload.Host.Hostname)
	}
	if name == "" {
		name = "agodesk"
	}
	readOnly := s.RemoteHub.DefaultReadOnly
	serverCapabilities := agodeskServerCapabilitiesForDevice(s, readOnly)
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
	if s.Vault == nil {
		_ = remote.DeleteDevice(s.RemoteHub.DB(), deviceID)
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorInternal, "failed to store agodesk shared key"
	}
	if err := s.Vault.WriteSecret("remote_shared_key_"+deviceID, sharedKey); err != nil {
		if s.Logger != nil {
			s.Logger.Error("Failed to store agodesk shared key", "device_id", deviceID, "error", err)
		}
		_ = remote.DeleteDevice(s.RemoteHub.DB(), deviceID)
		return agodesk.SessionAcceptedPayload{}, agodesk.ErrorInternal, "failed to store agodesk shared key"
	}
	_ = remote.MarkEnrollmentUsed(s.RemoteHub.DB(), enrollment.ID, deviceID)
	advertised := agodesk.NegotiateCapabilities(payload.ClientCapabilities, serverCapabilities)
	return agodesk.SessionAcceptedPayload{
		SessionID:              "agodesk:" + deviceID,
		DeviceID:               deviceID,
		Approved:               true,
		ReadOnly:               readOnly,
		Capabilities:           serverCapabilities,
		AdvertisedCapabilities: advertised,
		SharedKey:              sharedKey,
		AttachmentLimits:       agodeskAttachmentLimitsForAccepted(s, advertised),
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
	serverCapabilities := agodeskServerCapabilitiesForDevice(s, device.ReadOnly)
	device.Hostname = strings.TrimSpace(payload.Host.Hostname)
	device.OS = strings.TrimSpace(payload.Host.OS)
	device.Arch = strings.TrimSpace(payload.Host.Arch)
	if err := remote.UpdateDevice(s.RemoteHub.DB(), device); err != nil && s.Logger != nil {
		s.Logger.Warn("Failed to update agodesk reconnect device metadata", "device_id", deviceID, "error", err)
	}
	advertised := agodesk.NegotiateCapabilities(payload.ClientCapabilities, serverCapabilities)
	return agodesk.SessionAcceptedPayload{
		SessionID:              "agodesk:" + deviceID,
		DeviceID:               deviceID,
		Approved:               true,
		ReadOnly:               device.ReadOnly,
		Capabilities:           serverCapabilities,
		AdvertisedCapabilities: advertised,
		AttachmentLimits:       agodeskAttachmentLimitsForAccepted(s, advertised),
	}, "", ""
}

type agodeskChatBroker struct {
	agent.FeedbackBroker
	server         *Server
	conn           *websocket.Conn
	state          *agodeskConnectionState
	sessionID      string
	conversationID string
	requestID      string
	logger         *slog.Logger

	mu             sync.RWMutex
	latestPlan     interface{}
	latestPlanSeen bool
	emittedAudio   map[string]struct{}
}

func (b *agodeskChatBroker) Send(event, message string) {
	if event == "plan_update" {
		b.capturePlanUpdate(message)
	}
	if event == "audio" {
		if agodeskAudioMessageIsTTS(message) {
			b.emitAudio(message)
			return
		}
		if b.emitMedia(event, message) {
			return
		}
		if b.FeedbackBroker != nil {
			b.FeedbackBroker.Send(event, message)
		}
		return
	}
	if b.emitMedia(event, message) {
		return
	}
	if b.FeedbackBroker != nil {
		b.FeedbackBroker.Send(event, message)
	}
}

func (b *agodeskChatBroker) SendTyped(eventType string, payload interface{}) bool {
	forwarded := false
	if b != nil && b.FeedbackBroker != nil {
		if typed, ok := b.FeedbackBroker.(agent.TypedFeedbackBroker); ok {
			forwarded = typed.SendTyped(eventType, payload)
		}
		if !forwarded {
			if raw, err := json.Marshal(struct {
				Type    string      `json:"type"`
				Payload interface{} `json:"payload"`
			}{
				Type:    eventType,
				Payload: payload,
			}); err == nil {
				b.FeedbackBroker.SendJSON(string(raw))
				forwarded = true
			}
		}
	}
	if eventType != "agent_action" {
		return forwarded
	}
	action, ok := agodeskAgentActionFromTypedPayload(payload)
	if !ok {
		return forwarded
	}
	_ = b.emitAgentActionActivity(action)
	return true
}

func agodeskAgentActionFromTypedPayload(payload interface{}) (agent.AgentActionEvent, bool) {
	switch value := payload.(type) {
	case agent.AgentActionEvent:
		return value, true
	case *agent.AgentActionEvent:
		if value != nil {
			return *value, true
		}
	}
	return agent.AgentActionEvent{}, false
}

func (b *agodeskChatBroker) emitAgentActionActivity(action agent.AgentActionEvent) bool {
	if b == nil || !agodeskStateHasCapability(b.state, "chat.agent_activity") {
		return false
	}
	payload := b.agentActionActivityPayload(action)
	if strings.TrimSpace(payload.ActivityID) == "" {
		return false
	}
	if err := writeAgodeskEnvelopeLocked(b.conn, b.state, agodesk.TypeAgentActivity, payload); err != nil {
		if b.logger != nil {
			b.logger.Warn("Failed to emit agodesk agent activity", "session_id", b.sessionID, "activity_id", payload.ActivityID, "error", err)
		}
		return false
	}
	return true
}

func (b *agodeskChatBroker) agentActionActivityPayload(action agent.AgentActionEvent) agodesk.AgentActivityPayload {
	requestID := strings.TrimSpace(b.requestID)
	parentID := ""
	if requestID != "" {
		parentID = "agent:" + requestID
	}
	title := agodeskAgentActivityTitle(action)
	phase := agodeskAgentActivityPhase(action.State)
	return agodesk.AgentActivityPayload{
		ActivityID:       strings.TrimSpace(action.ID),
		ParentActivityID: parentID,
		SessionID:        strings.TrimSpace(b.sessionID),
		ConversationID:   strings.TrimSpace(b.conversationID),
		RequestID:        requestID,
		CommandID:        strings.TrimSpace(action.CorrelationID),
		Kind:             agodeskAgentActivityKind(action),
		Phase:            phase,
		Title:            title,
		Summary:          agodeskAgentActivitySummary(title, phase, action),
		Risk:             agodeskAgentActivityRisk(action),
	}
}

func agodeskAgentActivityPhase(state string) string {
	switch strings.TrimSpace(state) {
	case string(agent.AgentActionStateProposed), string(agent.AgentActionStateAccepted):
		return "queued"
	case string(agent.AgentActionStateStarted):
		return "started"
	case string(agent.AgentActionStateNeedsHumanApproval):
		return "waiting_approval"
	case string(agent.AgentActionStateSucceeded), string(agent.AgentActionStateSanitized):
		return "completed"
	case string(agent.AgentActionStateFailed), string(agent.AgentActionStateBlocked):
		return "failed"
	case string(agent.AgentActionStateCancelled):
		return "cancelled"
	default:
		return "queued"
	}
}

func agodeskAgentActivityKind(action agent.AgentActionEvent) string {
	text := strings.ToLower(strings.TrimSpace(action.ToolName + " " + action.Operation))
	switch {
	case strings.Contains(text, "shell") || strings.Contains(text, "execute_command"):
		return "shell"
	case strings.Contains(text, "browser"):
		return "browser"
	case strings.Contains(text, "desktop"):
		return "desktop"
	case strings.Contains(text, "file_search") || strings.Contains(text, "workspace_search") || strings.Contains(text, "search"):
		return "search"
	case strings.Contains(text, "file") && (strings.Contains(text, "write") || strings.Contains(text, "edit") || strings.Contains(text, "patch")):
		return "file_edit"
	case strings.Contains(text, "file") || strings.Contains(text, "read"):
		return "file_read"
	default:
		return "tool"
	}
}

func agodeskAgentActivityTitle(action agent.AgentActionEvent) string {
	tool := strings.TrimSpace(action.ToolName)
	subject := strings.TrimSpace(action.Subject)
	if tool == "" {
		tool = "tool"
	}
	if strings.EqualFold(tool, "activate_agent_skill") && subject != "" {
		return "Skill: " + subject
	}
	return tool
}

func agodeskAgentActivitySummary(title, phase string, action agent.AgentActionEvent) string {
	state := phase
	switch phase {
	case "queued":
		state = "queued"
	case "started":
		state = "started"
	case "waiting_approval":
		state = "waiting for approval"
	case "completed":
		state = "completed"
	case "failed":
		state = "failed"
	case "cancelled":
		state = "cancelled"
	}
	summary := fmt.Sprintf("%s %s", title, state)
	if action.DurationMS > 0 && (phase == "completed" || phase == "failed" || phase == "cancelled") {
		summary = fmt.Sprintf("%s in %d ms", summary, action.DurationMS)
	}
	return agodeskTruncateActivityText(security.Scrub(summary), 240)
}

func agodeskAgentActivityRisk(action agent.AgentActionEvent) string {
	text := strings.ToLower(strings.TrimSpace(action.ToolName + " " + action.Operation))
	switch {
	case strings.Contains(text, "delete") || strings.Contains(text, "revoke"):
		return "dangerous"
	case strings.Contains(text, "shell") || strings.Contains(text, "write") || strings.Contains(text, "edit") || strings.Contains(text, "patch") || strings.Contains(text, "input") || strings.Contains(text, "action"):
		return "write"
	default:
		return "read"
	}
}

func agodeskTruncateActivityText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return strings.TrimSpace(value[:limit-3]) + "..."
}

func emitAgodeskRunActivity(conn *websocket.Conn, state *agodeskConnectionState, sessionID, conversationID, requestID, phase, summary string, logger *slog.Logger) bool {
	requestID = strings.TrimSpace(requestID)
	if conn == nil || requestID == "" || !agodeskStateHasCapability(state, "chat.agent_activity") {
		return false
	}
	payload := agodesk.AgentActivityPayload{
		ActivityID:     "agent:" + requestID,
		SessionID:      strings.TrimSpace(sessionID),
		ConversationID: strings.TrimSpace(conversationID),
		RequestID:      requestID,
		Kind:           "agent",
		Phase:          strings.TrimSpace(phase),
		Title:          "Agent",
		Summary:        agodeskTruncateActivityText(security.Scrub(summary), 240),
		Risk:           "read",
	}
	if payload.Phase == "" {
		payload.Phase = "started"
	}
	if strings.TrimSpace(payload.Summary) == "" {
		payload.Summary = "Agent " + payload.Phase
	}
	if err := writeAgodeskEnvelopeLocked(conn, state, agodesk.TypeAgentActivity, payload); err != nil {
		if logger != nil {
			logger.Warn("Failed to emit agodesk run activity", "session_id", sessionID, "request_id", requestID, "phase", phase, "error", err)
		}
		return false
	}
	return true
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
	audioPath := agodeskStringField(raw, "path", "url")
	filename := agodeskStringField(raw, "filename", "file_name")
	payload := agodesk.ChatAudioPayload{
		SessionID:      strings.TrimSpace(b.sessionID),
		ConversationID: strings.TrimSpace(b.conversationID),
		RequestID:      strings.TrimSpace(b.requestID),
		Path:           agodeskChatAudioPath(audioPath, filename),
		Title:          agodeskStringField(raw, "title"),
		MimeType:       agodeskStringField(raw, "mime_type", "content_type"),
		Filename:       filename,
	}
	if metadata, ok := raw["metadata"].(map[string]interface{}); ok {
		payload.Metadata = metadata
	}
	if payload.Path == "" && payload.Filename == "" {
		return
	}
	if !b.markAudioEmitted(agodeskChatAudioDedupeKey(payload)) {
		return
	}
	_ = writeAgodeskEnvelopeLocked(b.conn, b.state, agodesk.TypeChatAudio, payload)
}

func (b *agodeskChatBroker) emitMedia(event, message string) bool {
	if b == nil || !agodeskStateHasCapability(b.state, "chat.media_events") {
		return false
	}
	payload, ok := agodeskChatMediaPayload(b.server, event, message, b.sessionID, b.conversationID, b.requestID, b.logger)
	if !ok {
		return false
	}
	_ = writeAgodeskEnvelopeLocked(b.conn, b.state, agodesk.TypeChatMedia, payload)
	return true
}

func agodeskAudioMessageIsTTS(message string) bool {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(message), &raw); err != nil {
		return false
	}
	pathValue := agodeskStringField(raw, "path", "url")
	if strings.HasPrefix(pathValue, "/tts/") || strings.HasPrefix(pathValue, "/api/agodesk/tts/") {
		return true
	}
	title := strings.ToLower(agodeskStringField(raw, "title"))
	filename := agodeskStringField(raw, "filename", "file_name")
	return pathValue == "" && isSafeAgodeskTTSFilename(filename) && strings.Contains(title, "tts")
}

func agodeskChatMediaPayload(s *Server, event, message, sessionID, conversationID, requestID string, logger *slog.Logger) (agodesk.ChatMediaPayload, bool) {
	kind := agodeskMediaKindForEvent(event)
	if kind == "" {
		return agodesk.ChatMediaPayload{}, false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(message), &raw); err != nil {
		if logger != nil {
			logger.Warn("Failed to decode agodesk media payload", "event", event, "session_id", conversationID, "error", err)
		}
		return agodesk.ChatMediaPayload{}, false
	}
	payload := agodesk.ChatMediaPayload{
		SessionID:      strings.TrimSpace(sessionID),
		ConversationID: strings.TrimSpace(conversationID),
		RequestID:      strings.TrimSpace(requestID),
		Kind:           kind,
		Path:           agodeskRewriteMediaPath(s, agodeskStringField(raw, "path", "web_path")),
		PreviewURL:     agodeskRewriteMediaPath(s, agodeskStringField(raw, "preview_url")),
		URL:            agodeskStringField(raw, "url"),
		EmbedURL:       agodeskStringField(raw, "embed_url"),
		VideoID:        agodeskStringField(raw, "video_id"),
		Title:          agodeskStringField(raw, "title"),
		Caption:        agodeskStringField(raw, "caption"),
		MimeType:       agodeskStringField(raw, "mime_type", "content_type"),
		Filename:       agodeskStringField(raw, "filename", "file_name"),
		Format:         agodeskStringField(raw, "format"),
		Provider:       agodeskStringField(raw, "provider"),
		StartSeconds:   agodeskIntField(raw, "start_seconds", "start"),
		DurationMs:     agodeskInt64Field(raw, "duration_ms"),
	}
	if metadata, ok := raw["metadata"].(map[string]interface{}); ok {
		payload.Metadata = metadata
	}
	if mediaType := agodeskStringField(raw, "media_type"); mediaType != "" {
		if payload.Metadata == nil {
			payload.Metadata = map[string]interface{}{}
		}
		payload.Metadata["media_type"] = mediaType
	}
	payload.OpenMode = agodeskMediaOpenMode(payload)
	if payload.Path == "" && payload.PreviewURL == "" && payload.URL == "" && payload.EmbedURL == "" {
		return agodesk.ChatMediaPayload{}, false
	}
	return payload, true
}

func agodeskMediaKindForEvent(event string) string {
	switch strings.TrimSpace(event) {
	case "image":
		return "image"
	case "audio":
		return "audio"
	case "video":
		return "video"
	case "document":
		return "document"
	case "stl":
		return "stl"
	case "live_stream":
		return "live_stream"
	case "youtube_video":
		return "youtube_video"
	case "link":
		return "link"
	default:
		return ""
	}
}

func agodeskMediaOpenMode(payload agodesk.ChatMediaPayload) string {
	switch payload.Kind {
	case "link":
		return "external"
	case "document":
		if strings.TrimSpace(payload.PreviewURL) != "" {
			return "inline"
		}
		return "folder"
	default:
		return "inline"
	}
}

func agodeskRewriteMediaPath(s *Server, pathValue string) string {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return ""
	}
	parsed, err := url.Parse(pathValue)
	if err != nil || !strings.HasPrefix(parsed.Path, "/files/") {
		return pathValue
	}
	rel := strings.TrimPrefix(parsed.Path, "/files/")
	bucket, rest, ok := strings.Cut(rel, "/")
	if !ok || !agodeskMediaBucketAllowed(bucket) || strings.TrimSpace(rest) == "" {
		return pathValue
	}
	rewritten := "/api/agodesk/media/" + bucket + "/" + rest
	if parsed.RawQuery != "" {
		rewritten += "?" + parsed.RawQuery
	}
	return signAgodeskMediaAssetPath(s, rewritten, time.Now())
}

func agodeskMediaBucketAllowed(bucket string) bool {
	switch strings.TrimSpace(bucket) {
	case "generated_images", "generated_videos", "audio", "documents", "downloads", "images", "attachments":
		return true
	default:
		return false
	}
}

func agodeskIntField(raw map[string]interface{}, names ...string) int {
	return int(agodeskInt64Field(raw, names...))
}

func agodeskInt64Field(raw map[string]interface{}, names ...string) int64 {
	for _, name := range names {
		value, ok := raw[name]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case int:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		case json.Number:
			if n, err := v.Int64(); err == nil {
				return n
			}
		default:
			var out int64
			if _, err := fmt.Sscanf(strings.TrimSpace(fmt.Sprint(value)), "%d", &out); err == nil {
				return out
			}
		}
	}
	return 0
}

func (b *agodeskChatBroker) markAudioEmitted(key string) bool {
	if b == nil || key == "" {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.emittedAudio == nil {
		b.emittedAudio = make(map[string]struct{})
	}
	if _, exists := b.emittedAudio[key]; exists {
		return false
	}
	b.emittedAudio[key] = struct{}{}
	return true
}

func agodeskChatAudioDedupeKey(payload agodesk.ChatAudioPayload) string {
	pathValue := strings.TrimSpace(payload.Path)
	filename := strings.TrimSpace(payload.Filename)
	if pathValue == "" && filename == "" {
		return ""
	}
	return strings.Join([]string{
		strings.TrimSpace(payload.ConversationID),
		strings.TrimSpace(payload.RequestID),
		pathValue,
		filename,
	}, "\x00")
}

func agodeskChatAudioPath(pathValue, filename string) string {
	pathValue = strings.TrimSpace(pathValue)
	filename = strings.TrimSpace(filename)
	if strings.HasPrefix(pathValue, "/tts/") {
		if name := strings.TrimPrefix(pathValue, "/tts/"); isSafeAgodeskTTSFilename(name) {
			return "/api/agodesk/tts/" + name
		}
	}
	if pathValue == "" && isSafeAgodeskTTSFilename(filename) {
		return "/api/agodesk/tts/" + filename
	}
	return pathValue
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
		SessionID:             conversationID,
		MessageSource:         agodeskMessageSource,
		AdditionalPrompt:      buildAgodeskAgentContext(deviceID, agodeskStateFileAccess(state)),
		PersistedMessage:      stripAgodeskAttachmentBlock(message),
		VoiceOutputActive:     voiceOutput,
		OnUserMessageInserted: agodeskAttachmentBindingCallback(r.Context()),
		PrepareSessionMessages: func(messages []memory.HistoryMessage, currentMessageID int64) []memory.HistoryMessage {
			return agodeskMessagesWithAttachmentContext(s, messages, currentMessageID)
		},
	})
	if err != nil {
		return agodeskChatResult{}, err
	}
	replyBroker := &desktopReplyBroker{FeedbackBroker: NewSSEBrokerAdapterWithSession(s.SSE, conversationID)}
	broker := &agodeskChatBroker{
		FeedbackBroker: replyBroker,
		server:         s,
		conn:           conn,
		state:          state,
		sessionID:      transportSessionID,
		conversationID: conversationID,
		requestID:      requestID,
		logger:         s.Logger,
	}
	emitAgodeskRunActivity(conn, state, transportSessionID, conversationID, requestID, "started", "Agent started", s.Logger)
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
		emitAgodeskRunActivity(conn, state, transportSessionID, conversationID, requestID, "failed", "Agent timed out", s.Logger)
		return agodeskChatResult{}, errAgodeskAgentTimeout
	}
	if loopErr != nil {
		emitAgodeskRunActivity(conn, state, transportSessionID, conversationID, requestID, "failed", "Agent failed", s.Logger)
	} else {
		emitAgodeskRunActivity(conn, state, transportSessionID, conversationID, requestID, "completed", "Agent completed", s.Logger)
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
		Answer:   sanitizeAgodeskChatResponseText(answer),
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

func buildAgodeskAgentContext(deviceID string, fileAccess *agodesk.FileAccessPayload) string {
	lines := []string{
		"The user is chatting from agodesk, a paired desktop companion running on a remote PC.",
		"When the user asks about that remote PC, prefer the remote_control tool for available device operations and respect read-only policy.",
		"Desktop screenshots, display/window discovery, active-window metadata, UI tree reads, and browser snapshots are available through remote_control when the agodesk client advertises the matching capabilities.",
		"If desktop screenshot or permission requests return UNSUPPORTED_CAPABILITY, explain that the client is connected for chat but does not advertise remote-control support.",
		"Desktop input, UI actions, and browser actions require local approval in the agodesk remote-control banner; the backend cannot approve or bypass that local control session.",
		"Desktop streaming is not available in this backend version.",
		"Explicit chat attachments uploaded by the user are listed inside <agodesk_attachments> in the current or prior user messages with agent_workspace/workdir/attachments/... paths. Use those local uploaded files directly; do not use remote.files or remote_control file operations for them unless the user separately asks to access files on the paired PC.",
		"Ignore generic files directly under agent_workspace/workdir/attachments/ unless they are listed inside <agodesk_attachments>; AgoDesk chat uploads use agent_path entries under attachments/agodesk/.",
	}
	if id := strings.TrimSpace(deviceID); id != "" {
		lines = append(lines,
			fmt.Sprintf("Paired agodesk remote_control device_id: %q. Always pass this device_id on remote_control operations for the user's PC.", id),
		)
	}
	lines = append(lines, agodeskFileAccessAgentContextLines(fileAccess)...)
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

func agodeskStateFileAccess(state *agodeskConnectionState) *agodesk.FileAccessPayload {
	if state == nil {
		return nil
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	return copyAgodeskFileAccessPayload(state.fileAccess)
}

func normalizeAgodeskFileAccessPayload(payload *agodesk.FileAccessPayload) *agodesk.FileAccessPayload {
	if payload == nil {
		return nil
	}
	out := &agodesk.FileAccessPayload{
		Enabled:       payload.Enabled,
		MaxReadBytes:  normalizeAgodeskFileAccessLimit(payload.MaxReadBytes),
		MaxWriteBytes: normalizeAgodeskFileAccessLimit(payload.MaxWriteBytes),
	}
	for _, root := range payload.Roots {
		rootID := strings.TrimSpace(root.RootID)
		if rootID == "" {
			continue
		}
		out.Roots = append(out.Roots, agodesk.FileAccessRoot{
			RootID:      rootID,
			Label:       strings.TrimSpace(root.Label),
			PathDisplay: strings.TrimSpace(root.PathDisplay),
			Permissions: normalizeAgodeskFileAccessPermissions(root.Permissions),
		})
		if len(out.Roots) >= 32 {
			break
		}
	}
	return out
}

func copyAgodeskFileAccessPayload(payload *agodesk.FileAccessPayload) *agodesk.FileAccessPayload {
	if payload == nil {
		return nil
	}
	out := *payload
	if payload.Roots != nil {
		out.Roots = make([]agodesk.FileAccessRoot, len(payload.Roots))
		for i, root := range payload.Roots {
			out.Roots[i] = root
			if root.Permissions != nil {
				out.Roots[i].Permissions = append([]string(nil), root.Permissions...)
			}
		}
	}
	return &out
}

func normalizeAgodeskFileAccessLimit(limit int64) int64 {
	if limit <= 0 || limit > agodeskFileAccessInlineLimit {
		return agodeskFileAccessInlineLimit
	}
	return limit
}

func normalizeAgodeskFileAccessPermissions(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "read", "write":
		default:
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func agodeskFileAccessAgentContextLines(fileAccess *agodesk.FileAccessPayload) []string {
	if fileAccess == nil {
		return nil
	}
	if !fileAccess.Enabled {
		return []string{"AgoDesk local file access is disabled for this client session."}
	}
	lines := []string{
		fmt.Sprintf("AgoDesk local file access is enabled. Inline read/write payload limits are max_read_bytes=%d and max_write_bytes=%d.", fileAccess.MaxReadBytes, fileAccess.MaxWriteBytes),
		"Use remote_control read_file, list_files, and write_file with root_id when the user asks to access files on the paired PC.",
		"When searching files on the paired agodesk PC, use remote_control file_search (grep / grep_recursive / find) scoped to the granted roots; agodesk uses a fast local index (fff).",
	}
	if len(fileAccess.Roots) == 0 {
		lines = append(lines, "The client did not report named file-access roots; ask the user to configure AgoDesk file-access roots before using root_id-based file commands.")
		return lines
	}
	lines = append(lines, "Available AgoDesk file-access roots:")
	for _, root := range fileAccess.Roots {
		permissions := strings.Join(root.Permissions, ",")
		if permissions == "" {
			permissions = "none"
		}
		label := root.Label
		if label == "" {
			label = root.RootID
		}
		pathDisplay := root.PathDisplay
		if pathDisplay == "" {
			pathDisplay = "(path hidden by client)"
		}
		lines = append(lines, fmt.Sprintf("- root_id=%q label=%q path_display=%q permissions=%q", root.RootID, label, pathDisplay, permissions))
	}
	return lines
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
	if limitedCmd, denied := applyAgodeskFileAccessLimits(session.state, cmd); denied != nil {
		return *denied, nil
	} else {
		cmd = limitedCmd
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

func applyAgodeskFileAccessLimits(state *agodeskConnectionState, cmd remote.CommandPayload) (remote.CommandPayload, *remote.ResultPayload) {
	switch cmd.Operation {
	case remote.OpFileRead, remote.OpFileList, remote.OpFileWrite, remote.OpFilePatch, remote.OpFileSearch:
	default:
		return cmd, nil
	}
	fileAccess := agodeskStateFileAccess(state)
	if fileAccess == nil {
		return cmd, nil
	}
	if !fileAccess.Enabled {
		return cmd, &remote.ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "error",
			Error:     "FILE_ACCESS_DISABLED: agodesk local file access is disabled",
			ErrorCode: "FILE_ACCESS_DISABLED",
		}
	}
	if denied := validateAgodeskFileAccessRoot(cmd, fileAccess); denied != nil {
		return cmd, denied
	}
	limited := cmd
	limited.Args = copyRemoteCommandArgs(cmd.Args)
	switch cmd.Operation {
	case remote.OpFileRead:
		applyAgodeskCommandByteLimit(limited.Args, fileAccess.MaxReadBytes)
	case remote.OpFileWrite, remote.OpFilePatch:
		applyAgodeskCommandByteLimit(limited.Args, fileAccess.MaxWriteBytes)
		if cmd.Operation == remote.OpFileWrite && exceedsAgodeskWriteLimit(limited.Args["content"], fileAccess.MaxWriteBytes) {
			return cmd, &remote.ResultPayload{
				CommandID: cmd.CommandID,
				Status:    "error",
				Error:     "FILE_TOO_LARGE: content exceeds agodesk file_access.max_write_bytes",
				ErrorCode: "FILE_TOO_LARGE",
			}
		}
		if cmd.Operation == remote.OpFilePatch && exceedsAgodeskPatchLimit(limited.Args["patches"], fileAccess.MaxWriteBytes) {
			return cmd, &remote.ResultPayload{
				CommandID: cmd.CommandID,
				Status:    "error",
				Error:     "FILE_TOO_LARGE: patches exceed agodesk file_access.max_write_bytes",
				ErrorCode: "FILE_TOO_LARGE",
			}
		}
	}
	return limited, nil
}

func validateAgodeskFileAccessRoot(cmd remote.CommandPayload, fileAccess *agodesk.FileAccessPayload) *remote.ResultPayload {
	rootID := strings.TrimSpace(fmt.Sprint(cmd.Args["root_id"]))
	if rootID == "" || rootID == "<nil>" {
		return nil
	}
	wantPermission := "read"
	if cmd.Operation == remote.OpFileWrite || cmd.Operation == remote.OpFilePatch {
		wantPermission = "write"
	}
	for _, root := range fileAccess.Roots {
		if root.RootID != rootID {
			continue
		}
		for _, permission := range root.Permissions {
			if permission == wantPermission {
				return nil
			}
		}
		return &remote.ResultPayload{
			CommandID: cmd.CommandID,
			Status:    "error",
			Error:     fmt.Sprintf("FILE_ACCESS_DENIED: root_id %q does not allow %s", rootID, wantPermission),
			ErrorCode: "FILE_ACCESS_DENIED",
		}
	}
	return &remote.ResultPayload{
		CommandID: cmd.CommandID,
		Status:    "error",
		Error:     fmt.Sprintf("FILE_ACCESS_DENIED: unknown agodesk file-access root_id %q", rootID),
		ErrorCode: "FILE_ACCESS_DENIED",
	}
}

func copyRemoteCommandArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(args)+1)
	for key, value := range args {
		out[key] = value
	}
	return out
}

func applyAgodeskCommandByteLimit(args map[string]interface{}, limit int64) {
	if limit <= 0 {
		return
	}
	current := agodeskInt64Field(args, "max_bytes", "max_read_bytes", "max_write_bytes")
	if current <= 0 || current > limit {
		args["max_bytes"] = limit
	}
}

func exceedsAgodeskWriteLimit(content interface{}, limit int64) bool {
	if limit <= 0 || content == nil {
		return false
	}
	return int64(len([]byte(fmt.Sprint(content)))) > limit
}

func exceedsAgodeskPatchLimit(patches interface{}, limit int64) bool {
	if limit <= 0 || patches == nil {
		return false
	}
	raw, err := json.Marshal(patches)
	if err != nil {
		return int64(len([]byte(fmt.Sprint(patches)))) > limit
	}
	return int64(len(raw)) > limit
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

func (b *agodeskDesktopBroker) sessionsWithCapability(capability string) []*agodeskDesktopSession {
	if b == nil || strings.TrimSpace(capability) == "" {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	sessions := make([]*agodeskDesktopSession, 0, len(b.sessions))
	for _, session := range b.sessions {
		if session != nil && session.hasCapability(capability) {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func agodeskSessionTransportID(session *agodeskDesktopSession) string {
	if session == nil || session.state == nil {
		return ""
	}
	session.state.mu.RLock()
	defer session.state.mu.RUnlock()
	return strings.TrimSpace(session.state.sessionID)
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
	case remote.OpFileRead, remote.OpFileList, remote.OpFileSearch:
		return "remote.files.read"
	case remote.OpFileWrite, remote.OpFilePatch:
		return "remote.files.write"
	case remote.OpShellExec:
		return "remote.shell.exec"
	case remote.OpShellSessionStart, remote.OpShellSessionRead, remote.OpShellSessionInput, remote.OpShellSessionStop, remote.OpShellSessionList:
		return "remote.shell.session"
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
