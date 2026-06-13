package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/agodesk"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/warnings"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
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
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		if !strings.HasPrefix(transportSessionID, "agodesk:dev:") {
			t.Fatalf("transportSessionID = %q, want agodesk dev session", transportSessionID)
		}
		if conversationID != transportSessionID {
			t.Fatalf("conversationID = %q, want old-client fallback %q", conversationID, transportSessionID)
		}
		if requestID == "" {
			t.Fatal("requestID missing")
		}
		if message != "hello" {
			t.Fatalf("message = %q, want hello", message)
		}
		return agodeskChatResult{Answer: "agent says hello"}, nil
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

func TestAgodeskWebSocketStripsDoneTagFromChatResponse(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		return agodeskChatResult{Answer: "TTS-Test erfolgreich abgeschlossen - Audio wurde generiert und ist abspielbereit. <done/>"}, nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: connectedPayload.SessionID,
		Text:      "tts test",
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
	if strings.Contains(strings.ToLower(payload.Text), "<done") {
		t.Fatalf("chat response leaked done tag: %q", payload.Text)
	}
	want := "TTS-Test erfolgreich abgeschlossen - Audio wurde generiert und ist abspielbereit."
	if payload.Text != want {
		t.Fatalf("chat response text = %q, want %q", payload.Text, want)
	}
}

func TestAgodeskAttachmentCapabilitiesRequireWorkspaceAndSigningSecret(t *testing.T) {
	noUpload := agodeskServerCapabilities(&Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	})
	if agodeskTestContainsString(noUpload, "chat.media_upload") || agodeskTestContainsString(noUpload, "chat.attachments") {
		t.Fatalf("attachment capabilities advertised without workspace/signing secret: %v", noUpload)
	}

	cfg := &config.Config{}
	cfg.Auth.SessionSecret = "test-secret"
	cfg.Directories.WorkspaceDir = t.TempDir()
	enabled := agodeskServerCapabilities(&Server{
		Cfg:    cfg,
		Logger: slog.Default(),
	})
	for _, want := range []string{"chat.media_upload", "chat.attachments"} {
		if !agodeskTestContainsString(enabled, want) {
			t.Fatalf("capabilities missing %s: %v", want, enabled)
		}
	}
}

func TestAgodeskAttachmentPrepareUploadAndTextlessChatMessage(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.Cfg.Auth.SessionSecret = "test-secret"
	s.Cfg.Directories.WorkspaceDir = t.TempDir()
	sess, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}

	capturedMessage := make(chan string, 1)
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		capturedMessage <- message
		return agodeskChatResult{Answer: "saw attachment"}, nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	httpSrv := httptest.NewServer(agodeskAttachmentTestMux(s))
	defer httpSrv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpSrv.URL, "http")+"/api/agodesk/ws?insecure_loopback=1", nil)
	if err != nil {
		t.Fatalf("dial agodesk websocket: %v", err)
	}
	defer conn.Close()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)
	for _, want := range []string{"chat.media_upload", "chat.attachments"} {
		if !agodeskTestContainsString(connectedPayload.Capabilities, want) {
			t.Fatalf("system.connected capabilities missing %s: %v", want, connectedPayload.Capabilities)
		}
	}

	body := []byte("hello attachment")
	sum := sha256.Sum256(body)
	prepareReq, err := agodesk.NewEnvelope(agodesk.TypeChatAttachmentPrepare, agodesk.ChatAttachmentPreparePayload{
		SessionID:      connectedPayload.SessionID,
		ConversationID: sess.ID,
		Filename:       "note.txt",
		MimeType:       "text/plain",
		SizeBytes:      int64(len(body)),
		SHA256:         hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatalf("NewEnvelope prepare: %v", err)
	}
	if err := conn.WriteJSON(prepareReq); err != nil {
		t.Fatalf("write prepare: %v", err)
	}
	preparedEnv := readAgodeskTestEnvelope(t, conn)
	if preparedEnv.Type != agodesk.TypeChatAttachmentPrepared {
		t.Fatalf("prepared type = %q, want %q", preparedEnv.Type, agodesk.TypeChatAttachmentPrepared)
	}
	var prepared agodesk.ChatAttachmentPreparedPayload
	decodeAgodeskTestPayload(t, preparedEnv, &prepared)
	if prepared.AttachmentID == "" || prepared.UploadURL == "" || prepared.UploadField != "file" || prepared.MaxBytes != agodeskAttachmentMaxFileBytes {
		t.Fatalf("prepared payload = %+v", prepared)
	}

	uploadStatus, uploadPayload := postAgodeskAttachmentTestUpload(t, httpSrv.URL+prepared.UploadURL, "file", "note.txt", body)
	if uploadStatus != http.StatusCreated {
		t.Fatalf("upload status = %d payload = %s", uploadStatus, uploadPayload)
	}
	if !strings.Contains(uploadPayload, `"attachment_id":"`+prepared.AttachmentID+`"`) || !strings.Contains(uploadPayload, `"status":"ready"`) {
		t.Fatalf("upload payload = %s", uploadPayload)
	}
	var uploadResp agodeskAttachmentUploadResponse
	if err := json.Unmarshal([]byte(uploadPayload), &uploadResp); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	downloadResp, err := http.Get(httpSrv.URL + uploadResp.Path)
	if err != nil {
		t.Fatalf("download uploaded attachment: %v", err)
	}
	downloaded := new(bytes.Buffer)
	_, _ = downloaded.ReadFrom(downloadResp.Body)
	_ = downloadResp.Body.Close()
	if downloadResp.StatusCode != http.StatusOK || downloaded.String() != string(body) {
		t.Fatalf("download status/body = %d/%q, want 200/%q", downloadResp.StatusCode, downloaded.String(), string(body))
	}

	msgReq, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID:      connectedPayload.SessionID,
		ConversationID: sess.ID,
		Role:           "user",
		Attachments: []agodesk.ChatAttachmentItem{{
			AttachmentID: prepared.AttachmentID,
		}},
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.message: %v", err)
	}
	if err := conn.WriteJSON(msgReq); err != nil {
		t.Fatalf("write chat.message: %v", err)
	}
	acceptedEnv := readAgodeskTestEnvelope(t, conn)
	if acceptedEnv.Type != agodesk.TypeChatAttachmentAccepted {
		t.Fatalf("accepted type = %q, want %q", acceptedEnv.Type, agodesk.TypeChatAttachmentAccepted)
	}
	var accepted agodesk.ChatAttachmentAcceptedPayload
	decodeAgodeskTestPayload(t, acceptedEnv, &accepted)
	if len(accepted.Attachments) != 1 || accepted.Attachments[0].AttachmentID != prepared.AttachmentID || accepted.Attachments[0].Kind != "text" {
		t.Fatalf("accepted payload = %+v", accepted)
	}
	if accepted.Attachments[0].Metadata["storage_filename"] != "note.txt" {
		t.Fatalf("accepted attachment metadata = %+v, want storage_filename note.txt", accepted.Attachments[0].Metadata)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatResponse {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatResponse)
	}
	select {
	case got := <-capturedMessage:
		for _, want := range []string{"note.txt", "agent_workspace/workdir/attachments/agodesk/", prepared.AttachmentID} {
			if !strings.Contains(got, want) {
				t.Fatalf("runner message missing %q in:\n%s", want, got)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("runner did not receive attachment message")
	}
}

func TestAgodeskChatMessageRejectsAttachmentsWithoutCapability(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.ShortTermMem = newAgodeskTestMemory(t)
	sess, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	msgReq, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID:      connectedPayload.SessionID,
		ConversationID: sess.ID,
		Role:           "user",
		Attachments: []agodesk.ChatAttachmentItem{{
			AttachmentID: "att-missing",
		}},
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.message: %v", err)
	}
	if err := conn.WriteJSON(msgReq); err != nil {
		t.Fatalf("write chat.message: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatError {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatError)
	}
	var payload agodesk.ChatErrorPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.Code != agodesk.ErrorUnsupportedCapability {
		t.Fatalf("error code = %q, want %q", payload.Code, agodesk.ErrorUnsupportedCapability)
	}
}

func TestAgodeskHistoryMessagesAttachUploadedItemsAndStripInternalBlock(t *testing.T) {
	msgs := []memory.HistoryMessage{{
		ID:                    11,
		ChatCompletionMessage: websocketTestChatMessage("user", "Please inspect.\n\n<agodesk_attachments>\n- note.txt | text/plain | 12 bytes | agent_workspace/workdir/attachments/agodesk/sess-1/att-1/note.txt\n</agodesk_attachments>"),
	}}
	attachments := map[int64][]agodesk.ChatAttachmentItem{
		11: {{
			AttachmentID: "att-1",
			Filename:     "note.txt",
			MimeType:     "text/plain",
			SizeBytes:    12,
			Path:         "/api/agodesk/media/attachments/agodesk/sess-1/att-1/note.txt",
			Kind:         "text",
		}},
	}

	out := agodeskHistoryMessages(msgs, attachments)
	if len(out) != 1 {
		t.Fatalf("history messages = %+v", out)
	}
	if out[0].Content != "Please inspect." {
		t.Fatalf("content = %q, want stripped display text", out[0].Content)
	}
	if len(out[0].Attachments) != 1 || out[0].Attachments[0].AttachmentID != "att-1" {
		t.Fatalf("attachments = %+v", out[0].Attachments)
	}
}

func TestAgodeskAttachmentBindingContextBindsRecordsToInsertedMessage(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.ShortTermMem = newAgodeskTestMemory(t)
	sess, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	record := memory.AgoDeskAttachmentRecord{
		AttachmentID:       "att-bind",
		TransportSessionID: "agodesk:dev-1",
		ConversationID:     sess.ID,
		Filename:           "note.txt",
		MimeType:           "text/plain",
		Kind:               "text",
		DeclaredSizeBytes:  4,
		ExpiresAt:          time.Now().Add(time.Minute),
	}
	if err := s.ShortTermMem.PrepareAgoDeskAttachment(record); err != nil {
		t.Fatalf("PrepareAgoDeskAttachment: %v", err)
	}
	uploaded, err := s.ShortTermMem.MarkAgoDeskAttachmentUploaded(record.AttachmentID, 4, "sha", "attachments/agodesk/"+sess.ID+"/att-bind/note.txt", "text", "text/plain")
	if err != nil {
		t.Fatalf("MarkAgoDeskAttachmentUploaded: %v", err)
	}
	messageID, err := s.ShortTermMem.InsertMessage(sess.ID, "user", "with file", false, false)
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	ctx := contextWithAgodeskAttachmentBinding(context.Background(), s, sess.ID, []memory.AgoDeskAttachmentRecord{*uploaded})
	callback := agodeskAttachmentBindingCallback(ctx)
	if callback == nil {
		t.Fatal("binding callback missing")
	}
	if err := callback(messageID); err != nil {
		t.Fatalf("binding callback: %v", err)
	}
	byMessage, err := s.ShortTermMem.ListAgoDeskAttachmentsForMessages([]int64{messageID})
	if err != nil {
		t.Fatalf("ListAgoDeskAttachmentsForMessages: %v", err)
	}
	if len(byMessage[messageID]) != 1 || byMessage[messageID][0].Status != memory.AgoDeskAttachmentStatusAccepted {
		t.Fatalf("bound attachments = %+v", byMessage)
	}
}

func TestAgodeskAttachmentTurnPersistsVisibleTextButKeepsLLMContext(t *testing.T) {
	s := newTestDesktopChatServer(t)
	s.Cfg.LLM.ProviderType = "openrouter"
	s.Cfg.LLM.Multimodal = true
	sess, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	record := memory.AgoDeskAttachmentRecord{
		AttachmentID:       "att-maja",
		TransportSessionID: "agodesk:dev-1",
		ConversationID:     sess.ID,
		Filename:           "maja.png",
		MimeType:           "image/png",
		Kind:               "image",
		DeclaredSizeBytes:  218432,
		ExpiresAt:          time.Now().Add(time.Minute),
	}
	if err := s.ShortTermMem.PrepareAgoDeskAttachment(record); err != nil {
		t.Fatalf("PrepareAgoDeskAttachment: %v", err)
	}
	uploaded, err := s.ShortTermMem.MarkAgoDeskAttachmentUploaded(record.AttachmentID, record.DeclaredSizeBytes, "sha", "attachments/agodesk/"+sess.ID+"/att-maja/maja.png", "image", "image/png")
	if err != nil {
		t.Fatalf("MarkAgoDeskAttachmentUploaded: %v", err)
	}
	attachmentPath := filepath.Join(s.Cfg.Directories.WorkspaceDir, filepath.FromSlash(uploaded.RelativePath))
	if err := os.MkdirAll(filepath.Dir(attachmentPath), 0o755); err != nil {
		t.Fatalf("mkdir attachment dir: %v", err)
	}
	f, err := os.Create(attachmentPath)
	if err != nil {
		t.Fatalf("create attachment image: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatalf("encode attachment image: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close attachment image: %v", err)
	}

	visibleText := "konvertiere nach jpg und sende es"
	agentPrompt := buildAgodeskMessageWithAttachments(s, visibleText, []memory.AgoDeskAttachmentRecord{*uploaded})
	ctx := contextWithAgodeskAttachmentBinding(context.Background(), s, sess.ID, []memory.AgoDeskAttachmentRecord{*uploaded})
	turn, err := prepareDesktopAgentTurnWithOptions(ctx, s, agentPrompt, desktopChatContext{}, false, desktopAgentTurnOptions{
		SessionID:             sess.ID,
		MessageSource:         agodeskMessageSource,
		PersistedMessage:      stripAgodeskAttachmentBlock(agentPrompt),
		OnUserMessageInserted: agodeskAttachmentBindingCallback(ctx),
		PrepareSessionMessages: func(messages []memory.HistoryMessage, currentMessageID int64) []memory.HistoryMessage {
			return agodeskMessagesWithAttachmentContext(s, messages, currentMessageID)
		},
	})
	if err != nil {
		t.Fatalf("prepareDesktopAgentTurnWithOptions first turn: %v", err)
	}
	history, err := s.ShortTermMem.GetSessionMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetSessionMessages first turn: %v", err)
	}
	if len(history) != 1 || history[0].Content != visibleText {
		t.Fatalf("visible history = %+v, want one clean user message %q", history, visibleText)
	}
	if strings.Contains(history[0].Content, "<agodesk_attachments>") || strings.Contains(history[0].Content, "agent_workspace/workdir/attachments") {
		t.Fatalf("visible history leaked agodesk attachment context: %q", history[0].Content)
	}
	if len(turn.req.Messages) == 0 {
		t.Fatal("prepared first request has no messages")
	}
	firstCurrent := turn.req.Messages[len(turn.req.Messages)-1]
	if len(firstCurrent.MultiContent) == 0 || firstCurrent.Content != "" {
		t.Fatalf("current LLM request was not promoted to multimodal: %+v", firstCurrent)
	}
	firstText := firstCurrent.MultiContent[0].Text
	if !strings.Contains(firstText, "<agodesk_attachments>") ||
		!strings.Contains(firstText, "attachment_id: att-maja") ||
		!strings.Contains(firstText, "filename: maja.png") ||
		!strings.Contains(firstText, "agent_path: agent_workspace/workdir/attachments/agodesk/") ||
		!strings.Contains(firstText, "Use agent_path for file operations") {
		t.Fatalf("current LLM request missing agodesk attachment context: %q", firstText)
	}
	byMessage, err := s.ShortTermMem.ListAgoDeskAttachmentsForMessages([]int64{history[0].ID})
	if err != nil {
		t.Fatalf("ListAgoDeskAttachmentsForMessages: %v", err)
	}
	if len(byMessage[history[0].ID]) != 1 || byMessage[history[0].ID][0].Status != memory.AgoDeskAttachmentStatusAccepted {
		t.Fatalf("bound attachments after first turn = %+v", byMessage)
	}

	secondPrompt := "sende es nochmal"
	turn, err = prepareDesktopAgentTurnWithOptions(context.Background(), s, secondPrompt, desktopChatContext{}, false, desktopAgentTurnOptions{
		SessionID:        sess.ID,
		MessageSource:    agodeskMessageSource,
		PersistedMessage: secondPrompt,
		PrepareSessionMessages: func(messages []memory.HistoryMessage, currentMessageID int64) []memory.HistoryMessage {
			return agodeskMessagesWithAttachmentContext(s, messages, currentMessageID)
		},
	})
	if err != nil {
		t.Fatalf("prepareDesktopAgentTurnWithOptions second turn: %v", err)
	}
	history, err = s.ShortTermMem.GetSessionMessages(sess.ID)
	if err != nil {
		t.Fatalf("GetSessionMessages second turn: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("history length after second turn = %d, want 2", len(history))
	}
	for _, msg := range history {
		if strings.Contains(msg.Content, "<agodesk_attachments>") || strings.Contains(msg.Content, "agent_workspace/workdir/attachments") {
			t.Fatalf("visible history leaked agodesk attachment context after second turn: %+v", history)
		}
	}
	var sawRehydratedPriorAttachment bool
	for _, msg := range turn.req.Messages {
		text := msg.Content
		if text == "" && len(msg.MultiContent) > 0 {
			text = msg.MultiContent[0].Text
		}
		if strings.Contains(text, "<agodesk_attachments>") &&
			strings.Contains(text, "attachment_id: att-maja") &&
			strings.Contains(text, "agent_path: agent_workspace/workdir/attachments/agodesk/") {
			sawRehydratedPriorAttachment = true
		}
	}
	if !sawRehydratedPriorAttachment {
		t.Fatalf("second LLM request did not rehydrate prior agodesk attachment context: %+v", turn.req.Messages)
	}
	secondCurrent := turn.req.Messages[len(turn.req.Messages)-1]
	if secondCurrent.Content != secondPrompt {
		t.Fatalf("second current request = %q, want %q", secondCurrent.Content, secondPrompt)
	}
}

func TestAgodeskWebSocketSessionCreateListAndLoadFiltersInternalMessages(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.ShortTermMem = newAgodeskTestMemory(t)

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	createReq, err := agodesk.NewEnvelope(agodesk.TypeChatSessionCreate, agodesk.ChatSessionCreatePayload{
		SessionID: connectedPayload.SessionID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.session.create: %v", err)
	}
	if err := conn.WriteJSON(createReq); err != nil {
		t.Fatalf("write create session: %v", err)
	}
	created := readAgodeskTestEnvelope(t, conn)
	if created.Type != agodesk.TypeChatSession {
		t.Fatalf("created type = %q, want %q", created.Type, agodesk.TypeChatSession)
	}
	var createdPayload agodesk.ChatSessionPayload
	decodeAgodeskTestPayload(t, created, &createdPayload)
	if createdPayload.ConversationID == "" || !strings.HasPrefix(createdPayload.ConversationID, "sess-") {
		t.Fatalf("created conversation_id = %q", createdPayload.ConversationID)
	}

	if _, err := s.ShortTermMem.InsertMessage(createdPayload.ConversationID, "user", "visible user", false, false); err != nil {
		t.Fatalf("insert visible user: %v", err)
	}
	if _, err := s.ShortTermMem.InsertMessage(createdPayload.ConversationID, "assistant", "visible assistant", false, false); err != nil {
		t.Fatalf("insert visible assistant: %v", err)
	}
	if _, err := s.ShortTermMem.InsertMessage(createdPayload.ConversationID, "assistant", "hidden tool result", false, true); err != nil {
		t.Fatalf("insert internal assistant: %v", err)
	}
	if err := s.ShortTermMem.UpdateChatSessionPreview(createdPayload.ConversationID); err != nil {
		t.Fatalf("UpdateChatSessionPreview: %v", err)
	}

	listReq, err := agodesk.NewEnvelope(agodesk.TypeChatSessionsList, agodesk.ChatSessionsListPayload{
		SessionID: connectedPayload.SessionID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.sessions.list: %v", err)
	}
	if err := conn.WriteJSON(listReq); err != nil {
		t.Fatalf("write list sessions: %v", err)
	}
	listed := readAgodeskTestEnvelope(t, conn)
	if listed.Type != agodesk.TypeChatSessions {
		t.Fatalf("listed type = %q, want %q", listed.Type, agodesk.TypeChatSessions)
	}
	var listedPayload agodesk.ChatSessionsPayload
	decodeAgodeskTestPayload(t, listed, &listedPayload)
	if len(listedPayload.Sessions) == 0 || listedPayload.Sessions[0].ID != createdPayload.ConversationID {
		t.Fatalf("listed sessions = %+v, want created session first", listedPayload.Sessions)
	}

	loadReq, err := agodesk.NewEnvelope(agodesk.TypeChatSessionLoad, agodesk.ChatSessionLoadPayload{
		SessionID:      connectedPayload.SessionID,
		ConversationID: createdPayload.ConversationID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat.session.load: %v", err)
	}
	if err := conn.WriteJSON(loadReq); err != nil {
		t.Fatalf("write load session: %v", err)
	}
	loaded := readAgodeskTestEnvelope(t, conn)
	if loaded.Type != agodesk.TypeChatSession {
		t.Fatalf("loaded type = %q, want %q", loaded.Type, agodesk.TypeChatSession)
	}
	var loadedPayload agodesk.ChatSessionPayload
	decodeAgodeskTestPayload(t, loaded, &loadedPayload)
	if loadedPayload.ConversationID != createdPayload.ConversationID || len(loadedPayload.Messages) != 2 {
		t.Fatalf("loaded payload = %+v, want two visible messages", loadedPayload)
	}
	for _, msg := range loadedPayload.Messages {
		if strings.Contains(msg.Content, "hidden") {
			t.Fatalf("loaded internal message: %+v", loadedPayload.Messages)
		}
	}
}

func TestAgodeskChatMessageUsesConversationIDAndVoiceOutput(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.ShortTermMem = newAgodeskTestMemory(t)
	s.Cfg.TTS.Provider = "google"
	sess, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		if conversationID != sess.ID {
			t.Fatalf("conversationID = %q, want %q", conversationID, sess.ID)
		}
		if !strings.HasPrefix(transportSessionID, "agodesk:dev:") {
			t.Fatalf("transportSessionID = %q, want agodesk dev session", transportSessionID)
		}
		if !voiceOutput {
			t.Fatal("voiceOutput = false, want true")
		}
		return agodeskChatResult{Answer: "conversation answer"}, nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID:      connectedPayload.SessionID,
		ConversationID: sess.ID,
		Text:           "hello from session",
		Role:           "user",
		VoiceOutput:    true,
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
	if payload.SessionID != connectedPayload.SessionID || payload.ConversationID != sess.ID || payload.Text != "conversation answer" {
		t.Fatalf("chat response payload = %+v", payload)
	}
}

func TestAgodeskChatCancelCancelsActiveConversationRun(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.ShortTermMem = newAgodeskTestMemory(t)
	sess, err := s.ShortTermMem.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	started := make(chan struct{}, 1)
	cancelled := make(chan string, 1)
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, r *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		started <- struct{}{}
		<-r.Context().Done()
		cancelled <- conversationID
		return agodeskChatResult{}, r.Context().Err()
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID:      connectedPayload.SessionID,
		ConversationID: sess.ID,
		Text:           "long run",
		Role:           "user",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write chat: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("agodesk runner did not start")
	}

	badCancel, err := agodesk.NewEnvelope(agodesk.TypeChatCancel, agodesk.ChatCancelPayload{
		SessionID:      connectedPayload.SessionID + "-stale",
		ConversationID: sess.ID,
		RequestID:      msg.ID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope bad cancel: %v", err)
	}
	if err := conn.WriteJSON(badCancel); err != nil {
		t.Fatalf("write bad cancel: %v", err)
	}
	badResp := readAgodeskTestEnvelope(t, conn)
	if badResp.Type != agodesk.TypeChatError {
		t.Fatalf("bad cancel response = %q, want %q", badResp.Type, agodesk.TypeChatError)
	}

	cancelReq, err := agodesk.NewEnvelope(agodesk.TypeChatCancel, agodesk.ChatCancelPayload{
		SessionID:      connectedPayload.SessionID,
		ConversationID: sess.ID,
		RequestID:      msg.ID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope cancel: %v", err)
	}
	if err := conn.WriteJSON(cancelReq); err != nil {
		t.Fatalf("write cancel: %v", err)
	}
	cancelResp := readAgodeskTestEnvelope(t, conn)
	if cancelResp.Type != agodesk.TypeChatCancelled {
		t.Fatalf("cancel response = %q, want %q", cancelResp.Type, agodesk.TypeChatCancelled)
	}
	var cancelPayload agodesk.ChatCancelledPayload
	decodeAgodeskTestPayload(t, cancelResp, &cancelPayload)
	if cancelPayload.SessionID != connectedPayload.SessionID || cancelPayload.ConversationID != sess.ID || cancelPayload.RequestID != msg.ID {
		t.Fatalf("cancel payload = %+v", cancelPayload)
	}
	select {
	case got := <-cancelled:
		if got != sess.ID {
			t.Fatalf("cancelled conversation = %q, want %q", got, sess.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("runner context was not cancelled")
	}
}

func TestAgodeskVoiceOutputStatusUpdatesSpeakerMode(t *testing.T) {
	oldVoiceMode := agent.GetVoiceMode()
	agent.SetVoiceMode(true)
	t.Cleanup(func() { agent.SetVoiceMode(oldVoiceMode) })

	s := newAgodeskHandlerTestServer()
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatVoiceOutputStatus, agodesk.ChatVoiceOutputStatusPayload{
		SessionID:   connectedPayload.SessionID,
		SpeakerMode: false,
		Mode:        "off",
		Reason:      "user_disabled",
	})
	if err != nil {
		t.Fatalf("NewEnvelope voice status: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write voice status: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatVoiceOutputStatus {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatVoiceOutputStatus)
	}
	var payload agodesk.ChatVoiceOutputStatusPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.SessionID != connectedPayload.SessionID || payload.SpeakerMode || payload.Mode != "off" || payload.Status != "ok" {
		t.Fatalf("voice status ack payload = %+v", payload)
	}
	if agent.GetVoiceMode() {
		t.Fatal("voice mode remained enabled after AgoDesk status update")
	}
}

func TestAgodeskWebSocketRejectsFeatureMessagesWithoutCapability(t *testing.T) {
	tests := []struct {
		name    string
		msgType agodesk.MessageType
		payload func(sessionID string) interface{}
	}{
		{
			name:    "sessions_list",
			msgType: agodesk.TypeChatSessionsList,
			payload: func(sessionID string) interface{} {
				return agodesk.ChatSessionsListPayload{SessionID: sessionID}
			},
		},
		{
			name:    "session_create",
			msgType: agodesk.TypeChatSessionCreate,
			payload: func(sessionID string) interface{} {
				return agodesk.ChatSessionCreatePayload{SessionID: sessionID}
			},
		},
		{
			name:    "session_load",
			msgType: agodesk.TypeChatSessionLoad,
			payload: func(sessionID string) interface{} {
				return agodesk.ChatSessionLoadPayload{SessionID: sessionID, ConversationID: "sess-test"}
			},
		},
		{
			name:    "cancel",
			msgType: agodesk.TypeChatCancel,
			payload: func(sessionID string) interface{} {
				return agodesk.ChatCancelPayload{SessionID: sessionID, RequestID: "req-test"}
			},
		},
		{
			name:    "voice_output_status",
			msgType: agodesk.TypeChatVoiceOutputStatus,
			payload: func(sessionID string) interface{} {
				return agodesk.ChatVoiceOutputStatusPayload{SessionID: sessionID, Mode: "off"}
			},
		},
		{
			name:    "persona_assets",
			msgType: agodesk.TypePersonaAssetsRequest,
			payload: func(sessionID string) interface{} {
				return agodesk.PersonaAssetsRequestPayload{SessionID: sessionID}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAgodeskPairingTestServer(t)
			s.ShortTermMem = newAgodeskTestMemory(t)
			conn, cleanup, accepted := pairAgodeskTestClient(t, s, "capability-"+tt.name, []string{"chat.full_response"})
			defer cleanup()
			env, err := agodesk.NewEnvelope(tt.msgType, tt.payload(accepted.SessionID))
			if err != nil {
				t.Fatalf("NewEnvelope %s: %v", tt.msgType, err)
			}
			if err := conn.WriteJSON(env); err != nil {
				t.Fatalf("write %s: %v", tt.msgType, err)
			}
			resp := readAgodeskTestEnvelope(t, conn)
			if resp.Type != agodesk.TypeChatError {
				t.Fatalf("response type = %q, want chat.error", resp.Type)
			}
			var payload agodesk.ChatErrorPayload
			decodeAgodeskTestPayload(t, resp, &payload)
			if payload.Code != agodesk.ErrorUnsupportedCapability {
				t.Fatalf("error payload = %+v, want unsupported capability", payload)
			}
		})
	}
}

func TestAgodeskWebSocketChatTimeoutUsesAgentTimeoutCode(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		return agodeskChatResult{}, errAgodeskAgentTimeout
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)
	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: connectedPayload.SessionID,
		Text:      "timeout",
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
		t.Fatalf("response type = %q, want chat.error", resp.Type)
	}
	var payload agodesk.ChatErrorPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.Code != agodesk.ErrorAgentTimeout {
		t.Fatalf("error payload = %+v, want AGENT_TIMEOUT", payload)
	}
}

func TestAgodeskChatBrokerEmitsAudioOnlyWithCapability(t *testing.T) {
	withAudioState := &agodeskConnectionState{
		sessionID:    "agodesk:dev-1",
		paired:       true,
		capabilities: normalizeAgodeskCapabilities([]string{"chat.audio_events"}),
	}
	audioEnv := readAgodeskBrokerAudioEnvelope(t, withAudioState)
	if audioEnv.Type != agodesk.TypeChatAudio {
		t.Fatalf("audio envelope type = %q, want %q", audioEnv.Type, agodesk.TypeChatAudio)
	}
	var audioPayload agodesk.ChatAudioPayload
	decodeAgodeskTestPayload(t, audioEnv, &audioPayload)
	if audioPayload.SessionID != "agodesk:dev-1" || audioPayload.ConversationID != "sess-1" || audioPayload.RequestID != "req-1" || audioPayload.Path != "/api/agodesk/tts/a.mp3" {
		t.Fatalf("audio payload = %+v", audioPayload)
	}

	withoutAudioState := &agodeskConnectionState{
		sessionID:    "agodesk:dev-1",
		paired:       true,
		capabilities: normalizeAgodeskCapabilities([]string{"chat.full_response"}),
	}
	if env := readAgodeskBrokerAudioEnvelope(t, withoutAudioState); env.Type != "" {
		t.Fatalf("unexpected audio envelope without capability: %+v", env)
	}
}

func TestAgodeskChatBrokerDeduplicatesAudioAndDoesNotForwardToSSE(t *testing.T) {
	state := &agodeskConnectionState{
		sessionID:    "agodesk:dev-1",
		paired:       true,
		capabilities: normalizeAgodeskCapabilities([]string{"chat.audio_events"}),
	}
	forwarded := &agodeskForwardCaptureBroker{}
	payload := `{"path":"/tts/a.mp3","title":"TTS Audio","mime_type":"audio/mpeg","filename":"a.mp3"}`
	envs := readAgodeskBrokerAudioEnvelopes(t, state, forwarded, payload, payload)
	if len(envs) != 1 {
		t.Fatalf("audio envelope count = %d, want 1", len(envs))
	}
	forwardedEvents := forwarded.Events()
	if len(forwardedEvents) != 0 {
		t.Fatalf("audio should not be forwarded to SSE broker, got events %v", forwardedEvents)
	}
}

func TestAgodeskChatBrokerEmitsMediaOnlyWithCapabilityAndRewritesFiles(t *testing.T) {
	state := &agodeskConnectionState{
		sessionID:    "agodesk:dev-1",
		paired:       true,
		capabilities: normalizeAgodeskCapabilities([]string{"chat.media_events"}),
	}
	envs := readAgodeskBrokerEventEnvelopes(t, state, nil, agodeskBrokerTestEvent{
		event:   "image",
		message: `{"path":"/files/images/cat.png","caption":"Cat picture"}`,
	})
	if len(envs) != 1 {
		t.Fatalf("media envelope count = %d, want 1", len(envs))
	}
	if envs[0].Type != agodesk.TypeChatMedia {
		t.Fatalf("media envelope type = %q, want %q", envs[0].Type, agodesk.TypeChatMedia)
	}
	var payload agodesk.ChatMediaPayload
	decodeAgodeskTestPayload(t, envs[0], &payload)
	if payload.Kind != "image" || payload.Path != "/api/agodesk/media/images/cat.png" || payload.Caption != "Cat picture" || payload.OpenMode != "inline" {
		t.Fatalf("media payload = %+v", payload)
	}

	withoutCapability := &agodeskConnectionState{
		sessionID:    "agodesk:dev-1",
		paired:       true,
		capabilities: normalizeAgodeskCapabilities([]string{"chat.full_response"}),
	}
	envs = readAgodeskBrokerEventEnvelopes(t, withoutCapability, nil, agodeskBrokerTestEvent{
		event:   "document",
		message: `{"path":"/files/documents/report.pdf","preview_url":"/files/documents/report.pdf?inline=1","title":"Report","mime_type":"application/pdf","filename":"report.pdf","format":"pdf"}`,
	})
	if len(envs) != 0 {
		t.Fatalf("unexpected media envelopes without capability: %+v", envs)
	}
}

func TestAgodeskChatBrokerKeepsTTSAudioSeparateFromMedia(t *testing.T) {
	state := &agodeskConnectionState{
		sessionID:    "agodesk:dev-1",
		paired:       true,
		capabilities: normalizeAgodeskCapabilities([]string{"chat.audio_events", "chat.media_events"}),
	}
	envs := readAgodeskBrokerEventEnvelopes(t, state, nil,
		agodeskBrokerTestEvent{
			event:   "audio",
			message: `{"path":"/tts/voice.mp3","title":"TTS Audio","mime_type":"audio/mpeg","filename":"voice.mp3"}`,
		},
		agodeskBrokerTestEvent{
			event:   "audio",
			message: `{"path":"/files/audio/song.mp3","title":"Song","mime_type":"audio/mpeg","filename":"song.mp3"}`,
		},
		agodeskBrokerTestEvent{
			event:   "youtube_video",
			message: `{"url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=12s","embed_url":"https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?start=12","video_id":"dQw4w9WgXcQ","title":"Demo","start_seconds":12,"provider":"youtube"}`,
		},
	)
	if len(envs) != 3 {
		t.Fatalf("envelope count = %d, want 3: %+v", len(envs), envs)
	}
	if envs[0].Type != agodesk.TypeChatAudio {
		t.Fatalf("first envelope type = %q, want chat.audio", envs[0].Type)
	}
	var ttsPayload agodesk.ChatAudioPayload
	decodeAgodeskTestPayload(t, envs[0], &ttsPayload)
	if ttsPayload.Path != "/api/agodesk/tts/voice.mp3" {
		t.Fatalf("tts path = %q, want agodesk tts asset path", ttsPayload.Path)
	}
	if envs[1].Type != agodesk.TypeChatMedia {
		t.Fatalf("second envelope type = %q, want chat.media", envs[1].Type)
	}
	var audioPayload agodesk.ChatMediaPayload
	decodeAgodeskTestPayload(t, envs[1], &audioPayload)
	if audioPayload.Kind != "audio" || audioPayload.Path != "/api/agodesk/media/audio/song.mp3" || audioPayload.OpenMode != "inline" {
		t.Fatalf("audio media payload = %+v", audioPayload)
	}
	var youtubePayload agodesk.ChatMediaPayload
	decodeAgodeskTestPayload(t, envs[2], &youtubePayload)
	if youtubePayload.Kind != "youtube_video" || youtubePayload.VideoID != "dQw4w9WgXcQ" || youtubePayload.StartSeconds != 12 || youtubePayload.OpenMode != "inline" {
		t.Fatalf("youtube media payload = %+v", youtubePayload)
	}
}

func TestAgodeskChatBrokerForwardsNonTTSAudioWithoutMediaCapability(t *testing.T) {
	state := &agodeskConnectionState{
		sessionID:    "agodesk:dev-1",
		paired:       true,
		capabilities: normalizeAgodeskCapabilities([]string{"chat.audio_events"}),
	}
	forwarded := &agodeskForwardCaptureBroker{}
	envs := readAgodeskBrokerEventEnvelopes(t, state, forwarded, agodeskBrokerTestEvent{
		event:   "audio",
		message: `{"path":"/files/audio/song.mp3","title":"Song","mime_type":"audio/mpeg","filename":"song.mp3"}`,
	})
	if len(envs) != 0 {
		t.Fatalf("unexpected websocket envelopes without media capability: %+v", envs)
	}
	forwardedEvents := forwarded.Events()
	if len(forwardedEvents) != 1 || forwardedEvents[0] != "audio" {
		t.Fatalf("forwarded events = %v, want [audio]", forwardedEvents)
	}
}

func TestAgodeskTTSAssetBypassesSessionAuthAndServesCachedAudio(t *testing.T) {
	dataDir := t.TempDir()
	ttsDir := filepath.Join(dataDir, "tts")
	if err := os.MkdirAll(ttsDir, 0o755); err != nil {
		t.Fatalf("mkdir tts dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ttsDir, "voice.mp3"), []byte("mp3-data"), 0o644); err != nil {
		t.Fatalf("write tts file: %v", err)
	}
	s := newAgodeskHandlerTestServer()
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = "test-secret"
	s.Cfg.Directories.DataDir = dataDir

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agodesk/tts/", handleAgodeskTTSAsset(s))
	req := httptest.NewRequest(http.MethodGet, "/api/agodesk/tts/voice.mp3", nil)
	rec := httptest.NewRecorder()
	authMiddleware(s, mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q; want 200 without web session", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "mp3-data" {
		t.Fatalf("body = %q, want mp3-data", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "audio/mpeg") {
		t.Fatalf("Content-Type = %q, want audio/mpeg", ct)
	}
}

func TestAgodeskMediaAssetRequiresSignedURLForAllowedFiles(t *testing.T) {
	dataDir := t.TempDir()
	workspaceDir := t.TempDir()
	audioDir := filepath.Join(dataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatalf("mkdir audio dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(audioDir, "song.mp3"), []byte("song-data"), 0o644); err != nil {
		t.Fatalf("write audio file: %v", err)
	}
	imageDir := filepath.Join(workspaceDir, "images")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatalf("mkdir image dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "cat.jpeg"), []byte("image-data"), 0o644); err != nil {
		t.Fatalf("write image file: %v", err)
	}
	s := newAgodeskHandlerTestServer()
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.PasswordHash = "configured"
	s.Cfg.Auth.SessionSecret = "test-secret"
	s.Cfg.Directories.DataDir = dataDir
	s.Cfg.Directories.WorkspaceDir = workspaceDir

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agodesk/media/", handleAgodeskMediaAsset(s))
	req := httptest.NewRequest(http.MethodGet, "/api/agodesk/media/audio/song.mp3", nil)
	rec := httptest.NewRecorder()
	authMiddleware(s, mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned status = %d, body = %q; want 401", rec.Code, rec.Body.String())
	}

	signedPath := signAgodeskMediaAssetPath(s, "/api/agodesk/media/audio/song.mp3", time.Now())
	req = httptest.NewRequest(http.MethodGet, signedPath, nil)
	rec = httptest.NewRecorder()
	authMiddleware(s, mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("signed status = %d, body = %q; want 200 without web session", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "song-data" {
		t.Fatalf("body = %q, want song-data", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "audio/") {
		t.Fatalf("Content-Type = %q, want audio type", ct)
	}

	signedPath = signAgodeskMediaAssetPath(s, "/api/agodesk/media/images/cat.jpeg", time.Now())
	req = httptest.NewRequest(http.MethodGet, signedPath, nil)
	rec = httptest.NewRecorder()
	authMiddleware(s, mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("signed image status = %d, body = %q; want 200 without web session", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "image-data" {
		t.Fatalf("image body = %q, want image-data", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Fatalf("image Content-Type = %q, want image type", ct)
	}

	traversalReq := httptest.NewRequest(http.MethodGet, "/api/agodesk/media/audio/%2e%2e/secret.txt", nil)
	traversalRec := httptest.NewRecorder()
	authMiddleware(s, mux).ServeHTTP(traversalRec, traversalReq)
	if traversalRec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned traversal status = %d, want 401", traversalRec.Code)
	}
}

func TestAgodeskChatMediaPayloadSignsRewrittenAssetPaths(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.Cfg.Auth.SessionSecret = "test-secret"
	payload, ok := agodeskChatMediaPayload(s, "document", `{"path":"/files/documents/report.pdf","preview_url":"/files/documents/report.pdf?inline=1","title":"Report"}`, "agodesk:dev-1", "sess-1", "req-1", slog.Default())
	if !ok {
		t.Fatal("agodeskChatMediaPayload returned ok=false")
	}
	if !strings.HasPrefix(payload.Path, "/api/agodesk/media/documents/report.pdf?") {
		t.Fatalf("signed path = %q, want agodesk media path with query", payload.Path)
	}
	if !strings.Contains(payload.Path, agodeskMediaAssetExpParam+"=") || !strings.Contains(payload.Path, agodeskMediaAssetSigParam+"=") {
		t.Fatalf("signed path missing token params: %q", payload.Path)
	}
	req := httptest.NewRequest(http.MethodGet, payload.PreviewURL, nil)
	if !verifyAgodeskMediaAssetSignature(s, req, time.Now()) {
		t.Fatalf("preview_url signature did not verify: %q", payload.PreviewURL)
	}
	if req.URL.Query().Get("inline") != "1" {
		t.Fatalf("preview_url lost inline query: %q", payload.PreviewURL)
	}

	imagePayload, ok := agodeskChatMediaPayload(s, "image", `{"web_path":"/files/images/img_test.jpeg","caption":"Workspace image"}`, "agodesk:dev-1", "sess-1", "req-1", slog.Default())
	if !ok {
		t.Fatal("image agodeskChatMediaPayload returned ok=false")
	}
	if !strings.HasPrefix(imagePayload.Path, "/api/agodesk/media/images/img_test.jpeg?") {
		t.Fatalf("signed image path = %q, want agodesk images media path with query", imagePayload.Path)
	}
	imageReq := httptest.NewRequest(http.MethodGet, imagePayload.Path, nil)
	if !verifyAgodeskMediaAssetSignature(s, imageReq, time.Now()) {
		t.Fatalf("image path signature did not verify: %q", imagePayload.Path)
	}
}

func TestAgodeskWebSocketReturnsIntegrationWebhosts(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.Cfg.VirtualDesktop.Enabled = true
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	req, err := agodesk.NewEnvelope(agodesk.TypeIntegrationsWebhostsList, agodesk.IntegrationsWebhostsListPayload{
		SessionID: connectedPayload.SessionID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope integrations list: %v", err)
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatalf("write integrations list: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeIntegrationsWebhosts {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeIntegrationsWebhosts)
	}
	var payload agodesk.IntegrationsWebhostsPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.SessionID != connectedPayload.SessionID || payload.Status != "ok" {
		t.Fatalf("webhosts payload ids/status = %+v", payload)
	}
	found := false
	for _, item := range payload.Webhosts {
		if item.ID == "virtual_desktop" && item.URL == "/desktop" {
			found = true
		}
	}
	if !found {
		t.Fatalf("webhosts missing virtual_desktop: %+v", payload.Webhosts)
	}
}

func TestAgodeskWebSocketListsAndAcknowledgesSystemWarnings(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.WarningsRegistry = warnings.NewRegistry()
	s.WarningsRegistry.Add(warnings.Warning{
		ID:          "warn-1",
		Severity:    warnings.SeverityWarning,
		Title:       "Test warning",
		Description: "Something needs attention",
		Category:    warnings.CategorySystem,
	})
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

	listReq, err := agodesk.NewEnvelope(agodesk.TypeSystemWarningsList, agodesk.SystemWarningsListPayload{
		SessionID: connectedPayload.SessionID,
	})
	if err != nil {
		t.Fatalf("NewEnvelope warnings list: %v", err)
	}
	if err := conn.WriteJSON(listReq); err != nil {
		t.Fatalf("write warnings list: %v", err)
	}
	listResp := readAgodeskTestEnvelope(t, conn)
	if listResp.Type != agodesk.TypeSystemWarnings {
		t.Fatalf("list response type = %q, want %q", listResp.Type, agodesk.TypeSystemWarnings)
	}
	var listPayload agodesk.SystemWarningsPayload
	decodeAgodeskTestPayload(t, listResp, &listPayload)
	if listPayload.Total != 1 || listPayload.Unacknowledged != 1 || len(listPayload.Warnings) != 1 || listPayload.Warnings[0].ID != "warn-1" {
		t.Fatalf("warnings list payload = %+v", listPayload)
	}

	ackReq, err := agodesk.NewEnvelope(agodesk.TypeSystemWarningAcknowledge, agodesk.SystemWarningAcknowledgePayload{
		SessionID: connectedPayload.SessionID,
		ID:        "warn-1",
	})
	if err != nil {
		t.Fatalf("NewEnvelope warnings ack: %v", err)
	}
	if err := conn.WriteJSON(ackReq); err != nil {
		t.Fatalf("write warnings ack: %v", err)
	}
	ackResp := readAgodeskTestEnvelope(t, conn)
	if ackResp.Type != agodesk.TypeSystemWarnings {
		t.Fatalf("ack response type = %q, want %q", ackResp.Type, agodesk.TypeSystemWarnings)
	}
	var ackPayload agodesk.SystemWarningsPayload
	decodeAgodeskTestPayload(t, ackResp, &ackPayload)
	if ackPayload.Unacknowledged != 0 || !ackPayload.Warnings[0].Acknowledged {
		t.Fatalf("warnings ack payload = %+v", ackPayload)
	}

	badReq, err := agodesk.NewEnvelope(agodesk.TypeSystemWarningsList, agodesk.SystemWarningsListPayload{
		SessionID: connectedPayload.SessionID + "-wrong",
	})
	if err != nil {
		t.Fatalf("NewEnvelope bad warnings list: %v", err)
	}
	if err := conn.WriteJSON(badReq); err != nil {
		t.Fatalf("write bad warnings list: %v", err)
	}
	badResp := readAgodeskTestEnvelope(t, conn)
	if badResp.Type != agodesk.TypeChatError {
		t.Fatalf("bad session response type = %q, want chat.error", badResp.Type)
	}
	var errPayload agodesk.ChatErrorPayload
	decodeAgodeskTestPayload(t, badResp, &errPayload)
	if errPayload.Code != agodesk.ErrorSessionNotFound {
		t.Fatalf("bad session error = %+v", errPayload)
	}
}

func TestAgodeskWebSocketWarningAcknowledgeDoesNotEchoBroadcastToRequester(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	s.WarningsRegistry = warnings.NewRegistry()
	s.WarningsRegistry.Add(warnings.Warning{
		ID:          "warn-1",
		Severity:    warnings.SeverityWarning,
		Title:       "Test warning",
		Description: "Something needs attention",
		Category:    warnings.CategorySystem,
	})

	conn, cleanup, accepted := pairAgodeskTestClient(t, s, "warnings-token", []string{"system.warnings"})
	defer cleanup()
	ackReq, err := agodesk.NewEnvelope(agodesk.TypeSystemWarningAcknowledge, agodesk.SystemWarningAcknowledgePayload{
		SessionID: accepted.SessionID,
		ID:        "warn-1",
	})
	if err != nil {
		t.Fatalf("NewEnvelope warnings ack: %v", err)
	}
	if err := conn.WriteJSON(ackReq); err != nil {
		t.Fatalf("write warnings ack: %v", err)
	}
	ackResp := readAgodeskTestEnvelope(t, conn)
	if ackResp.Type != agodesk.TypeSystemWarnings {
		t.Fatalf("ack response type = %q, want %q", ackResp.Type, agodesk.TypeSystemWarnings)
	}
	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer conn.SetReadDeadline(time.Time{})
	var duplicate agodesk.Envelope
	if err := conn.ReadJSON(&duplicate); err == nil {
		t.Fatalf("unexpected duplicate warning snapshot after ack: %+v", duplicate)
	}
}

func TestBuildAgodeskAgentMoodMetadataUsesSanitizedLatestEmotion(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.ShortTermMem = newAgodeskTestMemory(t)
	if err := s.ShortTermMem.InsertEmotionStateHistory(memory.EmotionState{
		Description:              "<think>hidden</think> I feel calm and ready to help.",
		PrimaryMood:              memory.MoodFocused,
		SecondaryMood:            "steady",
		Valence:                  0.2,
		Arousal:                  0.3,
		Confidence:               0.8,
		Cause:                    "private cause",
		Source:                   "llm_structured",
		RecommendedResponseStyle: "calm_and_precise",
	}, "test trigger"); err != nil {
		t.Fatalf("InsertEmotionStateHistory: %v", err)
	}

	metadata := buildAgodeskAgentMoodMetadata(s)
	if metadata["mood"] != "focused" || metadata["primary_mood"] != "focused" {
		t.Fatalf("mood metadata = %#v", metadata)
	}
	if metadata["secondary_mood"] != "steady" || metadata["recommended_response_style"] != "calm_and_precise" {
		t.Fatalf("emotion nuance metadata = %#v", metadata)
	}
	if metadata["description"] != "I feel calm and ready to help." {
		t.Fatalf("description = %q, want sanitized emotion", metadata["description"])
	}
	if _, ok := metadata["cause"]; ok {
		t.Fatalf("agent_mood must not expose cause: %#v", metadata)
	}
	if metadata["source"] != "emotion_history" {
		t.Fatalf("source = %q, want emotion_history", metadata["source"])
	}
}

func TestAgodeskWebSocketChatResponseCarriesAgentMoodMetadata(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		return agodeskChatResult{
			Answer: "mood answer",
			Metadata: map[string]interface{}{
				"agent_mood": map[string]interface{}{
					"mood":         "focused",
					"primary_mood": "focused",
				},
			},
		}, nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)

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
	var payload agodesk.ChatResponsePayload
	decodeAgodeskTestPayload(t, resp, &payload)
	mood, ok := payload.Metadata["agent_mood"].(map[string]interface{})
	if !ok {
		t.Fatalf("agent_mood metadata missing: %#v", payload.Metadata)
	}
	if mood["mood"] != "focused" {
		t.Fatalf("agent_mood = %#v", mood)
	}
}

func TestAgodeskWebSocketSendsActivePlanUpdateWhenCapabilityAdvertised(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	s.ShortTermMem = newAgodeskTestMemory(t)
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		return agodeskChatResult{Answer: "plan answer"}, nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws?insecure_loopback=1")
	defer cleanup()
	connected := readAgodeskTestEnvelope(t, conn)
	var connectedPayload agodesk.SystemConnectedPayload
	decodeAgodeskTestPayload(t, connected, &connectedPayload)
	plan := createAgodeskActiveTestPlan(t, s.ShortTermMem, connectedPayload.SessionID)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: connectedPayload.SessionID,
		Text:      "show plan",
		Role:      "user",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write chat: %v", err)
	}

	update := readAgodeskTestEnvelope(t, conn)
	if update.Type != agodesk.TypeChatPlanUpdate {
		t.Fatalf("first response type = %q, want %q", update.Type, agodesk.TypeChatPlanUpdate)
	}
	var updatePayload agodesk.ChatPlanUpdatePayload
	decodeAgodeskTestPayload(t, update, &updatePayload)
	if updatePayload.RequestID != msg.ID || updatePayload.SessionID != connectedPayload.SessionID {
		t.Fatalf("plan update payload ids = %+v", updatePayload)
	}
	var planPayload map[string]interface{}
	if err := json.Unmarshal(updatePayload.Plan, &planPayload); err != nil {
		t.Fatalf("unmarshal plan update plan: %v", err)
	}
	if planPayload["id"] != plan.ID || planPayload["title"] != "AgoDesk Plan" {
		t.Fatalf("plan update plan = %#v", planPayload)
	}

	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatResponse {
		t.Fatalf("final response type = %q, want %q", resp.Type, agodesk.TypeChatResponse)
	}
	var responsePayload agodesk.ChatResponsePayload
	decodeAgodeskTestPayload(t, resp, &responsePayload)
	if _, ok := responsePayload.Metadata["plan"].(map[string]interface{}); !ok {
		t.Fatalf("final response metadata missing plan snapshot: %#v", responsePayload.Metadata)
	}
}

func TestAgodeskWebSocketDoesNotSendPlanUpdateWithoutCapability(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	s.ShortTermMem = newAgodeskTestMemory(t)
	oldRunner := agodeskAgentChatRunner
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		return agodeskChatResult{Answer: "plain answer"}, nil
	}
	t.Cleanup(func() { agodeskAgentChatRunner = oldRunner })

	conn, cleanup, accepted := pairAgodeskTestClient(t, s, "plan-token", []string{"chat.full_response"})
	defer cleanup()
	createAgodeskActiveTestPlan(t, s.ShortTermMem, accepted.SessionID)

	msg, err := agodesk.NewEnvelope(agodesk.TypeChatMessage, agodesk.ChatMessagePayload{
		SessionID: accepted.SessionID,
		Text:      "show plan",
		Role:      "user",
	})
	if err != nil {
		t.Fatalf("NewEnvelope chat: %v", err)
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write chat: %v", err)
	}

	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type == agodesk.TypeChatPlanUpdate {
		t.Fatal("server sent chat.plan_update even though client did not advertise chat.plan_updates")
	}
	if resp.Type != agodesk.TypeChatResponse {
		t.Fatalf("response type = %q, want %q", resp.Type, agodesk.TypeChatResponse)
	}
	var responsePayload agodesk.ChatResponsePayload
	decodeAgodeskTestPayload(t, resp, &responsePayload)
	if _, ok := responsePayload.Metadata["plan"]; ok {
		t.Fatalf("final response should not include plan metadata without capability: %#v", responsePayload.Metadata)
	}
}

func TestAgodeskWebSocketRejectsMismatchedChatSessionID(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	oldRunner := agodeskAgentChatRunner
	runnerCalled := make(chan struct{}, 1)
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		runnerCalled <- struct{}{}
		return agodeskChatResult{Answer: "runner should not run"}, nil
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
	agodeskAgentChatRunner = func(_ *Server, _ *http.Request, _ *websocket.Conn, _ *agodeskConnectionState, requestID, transportSessionID, conversationID, deviceID, message string, voiceOutput bool) (agodeskChatResult, error) {
		close(runnerStarted)
		<-releaseRunner
		return agodeskChatResult{Answer: "slow answer"}, nil
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

func TestAgodeskSessionStartFailsWhenSharedKeyCannotBeStored(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	s.Vault = nil
	if _, err := remote.CreateEnrollment(s.RemoteHub.DB(), remote.EnrollmentRecord{
		TokenHash:  hashSHA256("vault-fail-token"),
		DeviceName: "agodesk-vault-fail",
		ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("CreateEnrollment: %v", err)
	}
	conn, cleanup := dialAgodeskTestWebSocket(t, s, "/api/agodesk/ws")
	defer cleanup()
	_ = readAgodeskTestEnvelope(t, conn)
	start, err := agodesk.NewEnvelope(agodesk.TypeSessionStart, agodesk.SessionStartPayload{
		ClientVersion: "0.1.0",
		PairingToken:  "vault-fail-token",
		Host:          agodesk.SessionStartHost{Hostname: "AGODESK-PC", OS: "windows", Arch: "amd64"},
	})
	if err != nil {
		t.Fatalf("NewEnvelope session.start: %v", err)
	}
	if err := conn.WriteJSON(start); err != nil {
		t.Fatalf("write session.start: %v", err)
	}
	resp := readAgodeskTestEnvelope(t, conn)
	if resp.Type != agodesk.TypeChatError {
		t.Fatalf("response type = %q, want chat.error", resp.Type)
	}
	var payload agodesk.ChatErrorPayload
	decodeAgodeskTestPayload(t, resp, &payload)
	if payload.Code != agodesk.ErrorInternal {
		t.Fatalf("error payload = %+v, want INTERNAL_ERROR", payload)
	}
	devices, err := remote.ListDevices(s.RemoteHub.DB())
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 0 {
		t.Fatalf("vault failure left registered devices behind: %+v", devices)
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
		{op: remote.OpFileSearch, wantCap: "remote.files.read", description: "file_search"},
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

func TestAgodeskFileAccessSessionStateFeedsAgentContext(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	fileAccess := &agodesk.FileAccessPayload{
		Enabled:       true,
		MaxReadBytes:  4096,
		MaxWriteBytes: 2048,
		Roots: []agodesk.FileAccessRoot{
			{
				RootID:      "workspace",
				Label:       "Workspace",
				PathDisplay: "C:\\Users\\Andi\\Workspace",
				Permissions: []string{"read", "write", "execute", "read"},
			},
		},
	}
	_, cleanup, accepted := pairAgodeskTestClientWithFileAccess(t, s, "agodesk-file-access-token", []string{"remote.files.read", "remote.files.write"}, fileAccess)
	defer cleanup()

	session := ensureAgodeskDesktopBroker(s).session(accepted.DeviceID)
	if session == nil {
		t.Fatal("agodesk session was not registered")
	}
	stored := agodeskStateFileAccess(session.state)
	if stored == nil || !stored.Enabled {
		t.Fatalf("stored file_access = %+v, want enabled payload", stored)
	}
	if stored.MaxReadBytes != 4096 || stored.MaxWriteBytes != 2048 {
		t.Fatalf("stored file_access limits = %+v", stored)
	}
	if len(stored.Roots) != 1 || stored.Roots[0].RootID != "workspace" {
		t.Fatalf("stored file_access roots = %+v", stored.Roots)
	}
	if fmt.Sprint(stored.Roots[0].Permissions) != "[read write]" {
		t.Fatalf("stored permissions = %+v, want read/write only once", stored.Roots[0].Permissions)
	}
	contextText := buildAgodeskAgentContext(accepted.DeviceID, stored)
	for _, want := range []string{
		`root_id="workspace"`,
		`max_read_bytes=4096`,
		`max_write_bytes=2048`,
		`permissions="read,write"`,
		`remote_control file_search`,
		`grep_recursive`,
	} {
		if !strings.Contains(contextText, want) {
			t.Fatalf("agent context missing %q in:\n%s", want, contextText)
		}
	}
}

func TestAgodeskFileAccessLimitsApplyToFileCommands(t *testing.T) {
	state := &agodeskConnectionState{
		fileAccess: normalizeAgodeskFileAccessPayload(&agodesk.FileAccessPayload{
			Enabled:       true,
			MaxReadBytes:  1024,
			MaxWriteBytes: 5,
			Roots: []agodesk.FileAccessRoot{
				{RootID: "workspace", Permissions: []string{"read", "write"}},
				{RootID: "readonly", Permissions: []string{"read"}},
				{RootID: "writeonly", Permissions: []string{"write"}},
			},
		}),
	}

	read, denied := applyAgodeskFileAccessLimits(state, remote.CommandPayload{
		CommandID: "read-1",
		Operation: remote.OpFileRead,
		Args: map[string]interface{}{
			"path":      "notes.txt",
			"root_id":   "workspace",
			"max_bytes": int64(4096),
		},
	})
	if denied != nil {
		t.Fatalf("read denied: %+v", denied)
	}
	if got := read.Args["max_bytes"]; got != int64(1024) {
		t.Fatalf("read max_bytes = %#v, want 1024", got)
	}

	search, denied := applyAgodeskFileAccessLimits(state, remote.CommandPayload{
		CommandID: "search-1",
		Operation: remote.OpFileSearch,
		Args: map[string]interface{}{
			"root_id":   "readonly",
			"operation": "grep_recursive",
			"pattern":   "TODO",
			"glob":      "**/*.go",
		},
	})
	if denied != nil {
		t.Fatalf("search denied: %+v", denied)
	}
	if got := search.Args["root_id"]; got != "readonly" {
		t.Fatalf("search root_id = %#v, want readonly", got)
	}

	_, denied = applyAgodeskFileAccessLimits(state, remote.CommandPayload{
		CommandID: "search-2",
		Operation: remote.OpFileSearch,
		Args: map[string]interface{}{
			"root_id":   "writeonly",
			"operation": "find",
			"glob":      "**/*.go",
		},
	})
	if denied == nil || denied.ErrorCode != "FILE_ACCESS_DENIED" {
		t.Fatalf("writeonly search denied = %+v, want FILE_ACCESS_DENIED", denied)
	}

	_, denied = applyAgodeskFileAccessLimits(state, remote.CommandPayload{
		CommandID: "search-3",
		Operation: remote.OpFileSearch,
		Args: map[string]interface{}{
			"root_id":   "missing",
			"operation": "grep",
			"pattern":   "TODO",
			"path":      "main.go",
		},
	})
	if denied == nil || denied.ErrorCode != "FILE_ACCESS_DENIED" {
		t.Fatalf("unknown-root search denied = %+v, want FILE_ACCESS_DENIED", denied)
	}

	_, denied = applyAgodeskFileAccessLimits(state, remote.CommandPayload{
		CommandID: "write-1",
		Operation: remote.OpFileWrite,
		Args: map[string]interface{}{
			"path":    "notes.txt",
			"root_id": "readonly",
			"content": "ok",
		},
	})
	if denied == nil || denied.ErrorCode != "FILE_ACCESS_DENIED" {
		t.Fatalf("readonly write denied = %+v, want FILE_ACCESS_DENIED", denied)
	}

	_, denied = applyAgodeskFileAccessLimits(state, remote.CommandPayload{
		CommandID: "write-2",
		Operation: remote.OpFileWrite,
		Args: map[string]interface{}{
			"path":    "notes.txt",
			"root_id": "workspace",
			"content": "too large",
		},
	})
	if denied == nil || denied.ErrorCode != "FILE_TOO_LARGE" {
		t.Fatalf("large write denied = %+v, want FILE_TOO_LARGE", denied)
	}

	disabledState := &agodeskConnectionState{fileAccess: &agodesk.FileAccessPayload{Enabled: false}}
	_, denied = applyAgodeskFileAccessLimits(disabledState, remote.CommandPayload{CommandID: "list-1", Operation: remote.OpFileList, Args: map[string]interface{}{"path": "."}})
	if denied == nil || denied.ErrorCode != "FILE_ACCESS_DISABLED" {
		t.Fatalf("disabled file access denied = %+v, want FILE_ACCESS_DISABLED", denied)
	}

	_, denied = applyAgodeskFileAccessLimits(disabledState, remote.CommandPayload{
		CommandID: "search-disabled",
		Operation: remote.OpFileSearch,
		Args:      map[string]interface{}{"root_id": "workspace", "operation": "find", "glob": "**/*.go"},
	})
	if denied == nil || denied.ErrorCode != "FILE_ACCESS_DISABLED" {
		t.Fatalf("disabled file search denied = %+v, want FILE_ACCESS_DISABLED", denied)
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
			"message":         "mission update",
			"conversation_id": "sess-push",
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
	if payload.ConversationID != "sess-push" {
		t.Fatalf("conversation_id = %q, want sess-push", payload.ConversationID)
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

func TestAgodeskActiveRunFallbackUsesMostRecentlyRegisteredRun(t *testing.T) {
	state := &agodeskConnectionState{}
	for i := 0; i < 100; i++ {
		registerAgodeskActiveRun(state, fmt.Sprintf("req-%03d", i), fmt.Sprintf("sess-%03d", i), func() {})
	}

	unregisterAgodeskActiveRun(state, "req-099")
	run, requestID, found := agodeskFindActiveRun(state, "", "")
	if !found {
		t.Fatal("fallback active run not found")
	}
	if requestID != "req-098" || run.conversationID != "sess-098" {
		t.Fatalf("fallback active run = request %q conversation %q, want req-098/sess-098", requestID, run.conversationID)
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

func newAgodeskTestMemory(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	stm, err := memory.NewSQLiteMemory(":memory:", slog.Default())
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() {
		_ = stm.Close()
	})
	return stm
}

func createAgodeskActiveTestPlan(t *testing.T, stm *memory.SQLiteMemory, sessionID string) *memory.Plan {
	t.Helper()
	plan, err := stm.CreatePlan(sessionID, "AgoDesk Plan", "desc", "request", 1, []memory.PlanTaskInput{
		{Title: "First task", Description: "Do the first task", Kind: "manual"},
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, memory.PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}
	return plan
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
	return pairAgodeskTestClientWithFileAccess(t, s, token, clientCapabilities, nil)
}

func pairAgodeskTestClientWithFileAccess(t *testing.T, s *Server, token string, clientCapabilities []string, fileAccess *agodesk.FileAccessPayload) (*websocket.Conn, func(), agodesk.SessionAcceptedPayload) {
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
		FileAccess:         fileAccess,
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

func agodeskAttachmentTestMux(s *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agodesk/ws", handleAgodeskWebSocket(s))
	mux.HandleFunc("/api/agodesk/media/upload/", handleAgodeskAttachmentUpload(s))
	mux.HandleFunc("/api/agodesk/media/", handleAgodeskMediaAsset(s))
	return mux
}

func postAgodeskAttachmentTestUpload(t *testing.T, uploadURL, fieldName, filename string, data []byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write multipart body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, uploadURL, &body)
	if err != nil {
		t.Fatalf("NewRequest upload: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.String()
}

func websocketTestChatMessage(role, content string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{Role: role, Content: content}
}

func readAgodeskBrokerAudioEnvelope(t *testing.T, state *agodeskConnectionState) agodesk.Envelope {
	t.Helper()
	envs := readAgodeskBrokerAudioEnvelopes(t, state, nil, `{"path":"/tts/a.mp3","title":"TTS Audio","mime_type":"audio/mpeg","filename":"a.mp3"}`)
	if len(envs) == 0 {
		return agodesk.Envelope{}
	}
	return envs[0]
}

func readAgodeskBrokerAudioEnvelopes(t *testing.T, state *agodeskConnectionState, feedback agent.FeedbackBroker, messages ...string) []agodesk.Envelope {
	t.Helper()
	events := make([]agodeskBrokerTestEvent, 0, len(messages))
	for _, message := range messages {
		events = append(events, agodeskBrokerTestEvent{event: "audio", message: message})
	}
	return readAgodeskBrokerEventEnvelopes(t, state, feedback, events...)
}

type agodeskBrokerTestEvent struct {
	event   string
	message string
}

func readAgodeskBrokerEventEnvelopes(t *testing.T, state *agodeskConnectionState, feedback agent.FeedbackBroker, events ...agodeskBrokerTestEvent) []agodesk.Envelope {
	t.Helper()
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := agodeskUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade broker websocket: %v", err)
			return
		}
		defer conn.Close()
		close(ready)
		broker := &agodeskChatBroker{
			conn:           conn,
			state:          state,
			sessionID:      "agodesk:dev-1",
			conversationID: "sess-1",
			requestID:      "req-1",
			logger:         slog.Default(),
			FeedbackBroker: feedback,
		}
		for _, event := range events {
			broker.Send(event.event, event.message)
		}
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial broker websocket: %v", err)
	}
	defer conn.Close()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("broker websocket did not become ready")
	}
	var envs []agodesk.Envelope
	for {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		var env agodesk.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			return envs
		}
		envs = append(envs, env)
	}
}

type agodeskForwardCaptureBroker struct {
	mu     sync.Mutex
	events []string
}

func (b *agodeskForwardCaptureBroker) Send(event, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
}

func (b *agodeskForwardCaptureBroker) Events() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.events...)
}

func (b *agodeskForwardCaptureBroker) SendJSON(string) {}

func (b *agodeskForwardCaptureBroker) SendLLMStreamDelta(string, string, string, int, string) {}

func (b *agodeskForwardCaptureBroker) SendLLMStreamDone(string) {}

func (b *agodeskForwardCaptureBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}

func (b *agodeskForwardCaptureBroker) SendThinkingBlock(string, string, string) {}

func (b *agodeskForwardCaptureBroker) Scrub(s string) string { return s }

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
