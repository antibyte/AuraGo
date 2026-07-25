package voice

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/realtimespeech"

	"github.com/gorilla/websocket"
)

const (
	defaultGeminiLiveWebSocketURL = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1alpha.GenerativeService.BidiGenerateContent"
	geminiMaxMessageBytes         = 8 * 1024 * 1024
	geminiMaxAudioBytes           = 4 * 1024 * 1024
)

type GeminiLiveBackend struct {
	Profile           config.RealtimeSpeechProfile
	Runner            VoiceActionRunner
	WebSocketURL      string
	Dialer            *websocket.Dialer
	AgentProviderID   string
	SystemInstruction string
	Greeting          string
	FailureMessage    string
	GoodbyeMessage    string
	IdleTimeout       time.Duration
	TestNoTools       bool
}

func (b *GeminiLiveBackend) Start(ctx context.Context, call CallContext, audio DuplexAudio) (VoiceSession, error) {
	if b.Runner == nil {
		return nil, fmt.Errorf("Gemini Live voice runner is required")
	}
	if !b.Profile.Enabled || !strings.EqualFold(b.Profile.Provider, realtimespeech.ProviderGemini) || strings.TrimSpace(b.Profile.APIKey) == "" {
		return nil, fmt.Errorf("Gemini Live profile is unavailable")
	}
	if b.IdleTimeout <= 0 {
		b.IdleTimeout = 2 * time.Minute
	}
	call.AgentProviderID = b.AgentProviderID
	call.AdditionalPrompt = b.SystemInstruction
	call.Greeting = b.Greeting
	call.FailureMessage = b.FailureMessage
	call.GoodbyeMessage = b.GoodbyeMessage
	sessionCtx, cancel := context.WithCancel(ctx)
	session := &geminiLiveSession{
		ctx: sessionCtx, cancel: cancel, call: call, audio: audio, backend: b,
		events: make(chan VoiceEvent, 64), activitySignal: make(chan struct{}, 1),
		turnCompleteSignal: make(chan struct{}, 1),
	}
	session.outputResampler, _ = NewResampler(24000, 8000)
	conn, err := session.connect("")
	if err != nil {
		cancel()
		return nil, err
	}
	session.setConnection(conn)
	session.wg.Add(3)
	go session.inputLoop()
	go session.readLoop()
	go session.idleLoop()
	go func() {
		session.wg.Wait()
		close(session.events)
	}()
	return session, nil
}

type geminiLiveSession struct {
	ctx                context.Context
	cancel             context.CancelFunc
	call               CallContext
	audio              DuplexAudio
	backend            *GeminiLiveBackend
	events             chan VoiceEvent
	connMu             sync.RWMutex
	conn               *websocket.Conn
	writeMu            sync.Mutex
	stateMu            sync.Mutex
	activity           bool
	handle             string
	wg                 sync.WaitGroup
	closeOnce          sync.Once
	outputResampler    *Resampler
	activitySignal     chan struct{}
	turnCompleteSignal chan struct{}
	busyTasks          atomic.Int32
	failureOnce        sync.Once
}

func (s *geminiLiveSession) connect(resumeHandle string) (*websocket.Conn, error) {
	endpoint := strings.TrimSpace(s.backend.WebSocketURL)
	if endpoint == "" {
		endpoint = defaultGeminiLiveWebSocketURL
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse Gemini Live WebSocket URL: %w", err)
	}
	query := parsed.Query()
	query.Set("key", s.backend.Profile.APIKey)
	parsed.RawQuery = query.Encode()
	dialer := s.backend.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	conn, response, err := dialer.DialContext(s.ctx, parsed.String(), http.Header{})
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("Gemini Live WebSocket rejected with HTTP %d", response.StatusCode)
		}
		return nil, fmt.Errorf("connect Gemini Live WebSocket: %w", err)
	}
	conn.SetReadLimit(geminiMaxMessageBytes)
	setup := realtimespeech.GeminiSIPSessionSetupWithInstruction(s.backend.Profile, s.backend.SystemInstruction)
	if s.backend.TestNoTools {
		setup = realtimespeech.GeminiSIPTestSessionSetup(s.backend.Profile, s.backend.SystemInstruction)
	}
	if resumeHandle != "" {
		setup["sessionResumption"] = map[string]interface{}{"handle": resumeHandle}
	}
	if err := conn.WriteJSON(map[string]interface{}{"setup": setup}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send Gemini Live setup: %w", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	for {
		var payload map[string]interface{}
		if err := conn.ReadJSON(&payload); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("receive Gemini Live setup: %w", err)
		}
		if providerError, ok := payload["error"].(map[string]interface{}); ok {
			_ = conn.Close()
			return nil, fmt.Errorf("Gemini Live setup rejected: %v", providerError["status"])
		}
		if _, ok := payload["setupComplete"]; ok {
			break
		}
		if _, ok := payload["setup_complete"]; ok {
			break
		}
	}
	_ = conn.SetReadDeadline(time.Time{})
	if resumeHandle == "" && strings.TrimSpace(s.backend.Greeting) != "" {
		if err := conn.WriteJSON(geminiTextTurn("Greet the caller now with exactly this meaning, naturally and in the configured voice: " + s.backend.Greeting)); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("send Gemini Live greeting: %w", err)
		}
	}
	return conn, nil
}

func (s *geminiLiveSession) inputLoop() {
	defer s.wg.Done()
	detector := NewActivityDetector(20, 100, 500, 100)
	resampler, _ := NewResampler(8000, 16000)
	for {
		select {
		case <-s.ctx.Done():
			return
		case frame := <-s.audio.Receive():
			if frame.SampleRate != 8000 {
				s.fail(fmt.Sprintf("Gemini input requires 8 kHz telephone PCM, got %d", frame.SampleRate), true)
				return
			}
			started, utterance := detector.Push(frame.Samples)
			if started {
				s.signalActivity()
				s.audio.FlushOutput()
				s.setActivity(true)
				s.emit("barge_in", "", nil)
			}
			if s.activityEnabled() {
				pcm := resampler.Process(frame.Samples)
				data := make([]byte, len(pcm)*2)
				for i, sample := range pcm {
					binary.LittleEndian.PutUint16(data[i*2:i*2+2], uint16(sample))
				}
				_ = s.writeJSON(map[string]interface{}{"realtimeInput": map[string]interface{}{
					"audio": map[string]interface{}{"data": base64.StdEncoding.EncodeToString(data), "mimeType": "audio/pcm;rate=16000"},
				}})
			}
			if utterance != nil {
				s.signalActivity()
				s.setActivity(false)
			}
		}
	}
}

func (s *geminiLiveSession) readLoop() {
	defer s.wg.Done()
	defer s.cancel()
	for {
		conn := s.connection()
		if conn == nil {
			return
		}
		var payload map[string]interface{}
		if err := conn.ReadJSON(&payload); err != nil {
			if s.ctx.Err() != nil {
				return
			}
			if !s.reconnect() {
				s.fail("Gemini Live connection could not be resumed", false)
				return
			}
			continue
		}
		if s.handlePayload(payload) && !s.reconnect() {
			s.fail("Gemini Live connection could not be resumed before shutdown", false)
			return
		}
	}
}

func (s *geminiLiveSession) reconnect() bool {
	s.stateMu.Lock()
	handle := s.handle
	s.stateMu.Unlock()
	if strings.TrimSpace(handle) == "" {
		return false
	}
	for attempt := 1; attempt <= 5 && s.ctx.Err() == nil; attempt++ {
		timer := time.NewTimer(time.Duration(1<<min(attempt-1, 4)) * time.Second)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
		conn, err := s.connect(handle)
		if err == nil {
			s.setConnection(conn)
			s.emit("session_resumed", "", nil)
			return true
		}
	}
	return false
}

func (s *geminiLiveSession) handlePayload(payload map[string]interface{}) (reconnectRequested bool) {
	if _, ok := payload["error"]; ok {
		s.fail("Gemini Live provider error", true)
		return false
	}
	if update := mapValue(payload, "sessionResumptionUpdate", "session_resumption_update"); update != nil {
		handle := stringValue(update, "newHandle", "new_handle")
		resumable := boolValue(update, "resumable")
		s.stateMu.Lock()
		if resumable && handle != "" {
			s.handle = handle
		} else if !resumable {
			s.handle = ""
		}
		s.stateMu.Unlock()
		if resumable && handle != "" {
			s.emit("session_resumption", "", map[string]any{"available": true})
		}
	}
	if mapValue(payload, "goAway", "go_away") != nil {
		return true
	}
	if content := mapValue(payload, "serverContent", "server_content"); content != nil {
		for _, pair := range []struct{ key, alt, direction string }{
			{"inputTranscription", "input_transcription", "input"},
			{"outputTranscription", "output_transcription", "output"},
		} {
			if transcript := mapValue(content, pair.key, pair.alt); transcript != nil {
				if text := stringValue(transcript, "text"); text != "" {
					s.signalActivity()
					s.emit("transcript", "", map[string]any{"direction": pair.direction, "text": text})
				}
			}
		}
		turn := mapValue(content, "modelTurn", "model_turn")
		if parts, ok := turn["parts"].([]interface{}); ok {
			for _, rawPart := range parts {
				part, _ := rawPart.(map[string]interface{})
				inline := mapValue(part, "inlineData", "inline_data")
				mime := stringValue(inline, "mimeType", "mime_type")
				if inline == nil || !strings.HasPrefix(mime, "audio/") {
					continue
				}
				encoded := stringValue(inline, "data")
				if len(encoded) > base64.StdEncoding.EncodedLen(geminiMaxAudioBytes) {
					s.fail("Gemini Live audio frame exceeds the safety limit", true)
					return false
				}
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil || len(decoded)%2 != 0 || len(decoded) > geminiMaxAudioBytes {
					continue
				}
				samples := make([]int16, len(decoded)/2)
				for i := range samples {
					samples[i] = int16(binary.LittleEndian.Uint16(decoded[i*2 : i*2+2]))
				}
				s.signalActivity()
				_ = s.audio.Send(s.ctx, PCMFrame{Samples: s.outputResampler.Process(samples), SampleRate: 8000})
			}
		}
		if boolValue(content, "interrupted") {
			s.audio.FlushOutput()
			s.emit("interrupted", "", nil)
		}
		if boolValue(content, "turnComplete", "turn_complete") {
			s.signalTurnComplete()
			s.emit("turn_complete", "", nil)
		}
	}
	if toolCall := mapValue(payload, "toolCall", "tool_call"); toolCall != nil {
		calls, _ := firstValue(toolCall, "functionCalls", "function_calls").([]interface{})
		for _, raw := range calls {
			call, _ := raw.(map[string]interface{})
			s.busyTasks.Add(1)
			s.signalActivity()
			go func() {
				defer func() {
					s.busyTasks.Add(-1)
					s.signalActivity()
				}()
				s.handleToolCall(call)
			}()
		}
	}
	return false
}

func (s *geminiLiveSession) handleToolCall(providerCall map[string]interface{}) {
	s.signalActivity()
	name := stringValue(providerCall, "name")
	id := stringValue(providerCall, "id")
	args, _ := providerCall["args"].(map[string]interface{})
	if args == nil {
		args, _ = providerCall["arguments"].(map[string]interface{})
	}
	response := map[string]interface{}{"status": "completed"}
	switch name {
	case "aurago_execute":
		request, _ := args["request"].(string)
		result, err := s.backend.Runner.RunVoiceTurn(s.ctx, s.call, request)
		if err != nil {
			response = map[string]interface{}{"status": "error", "message": "AuraGo action failed"}
		} else {
			response["result"] = result
		}
	case "aurago_cancel_current_task":
		s.backend.Runner.CancelVoiceTurn(s.call.CallID)
		response["result"] = "cancelled"
	case "aurago_end_call":
		s.backend.Runner.EndVoiceCall(s.call.CallID)
		response["result"] = "call ending"
	default:
		response = map[string]interface{}{"status": "error", "message": "unsupported private tool"}
	}
	_ = s.writeJSON(map[string]interface{}{"toolResponse": map[string]interface{}{
		"functionResponses": []map[string]interface{}{{"id": id, "name": name, "response": response}},
	}})
}

func (s *geminiLiveSession) idleLoop() {
	defer s.wg.Done()
	timer := time.NewTimer(s.backend.IdleTimeout)
	defer timer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.activitySignal:
			if s.busyTasks.Load() > 0 {
				stopVoiceIdleTimer(timer)
			} else {
				resetVoiceIdleTimer(timer, s.backend.IdleTimeout)
			}
		case <-timer.C:
			if s.busyTasks.Load() > 0 {
				continue
			}
			if message := strings.TrimSpace(s.backend.GoodbyeMessage); message != "" {
				_ = s.sendTextTurnAndWait("Say this brief farewell now and do nothing else: "+message, 3*time.Second)
			}
			s.emit("inactivity_timeout", "", nil)
			s.backend.Runner.EndVoiceCall(s.call.CallID)
			return
		}
	}
}

func (s *geminiLiveSession) signalActivity() {
	select {
	case s.activitySignal <- struct{}{}:
	default:
	}
}

func (s *geminiLiveSession) fail(message string, announce bool) {
	s.failureOnce.Do(func() {
		failure := strings.TrimSpace(s.call.FailureMessage)
		if !announce || failure == "" || s.connection() == nil {
			s.emit("voice_backend_error", message, nil)
			return
		}
		s.drainTurnComplete()
		if err := s.writeJSON(geminiTextTurn("Say this brief technical failure message now and do nothing else: " + failure)); err != nil {
			s.emit("voice_backend_error", message, nil)
			return
		}
		go func() {
			s.waitForTurnComplete(3 * time.Second)
			s.emit("voice_backend_error", message, nil)
		}()
	})
}

func (s *geminiLiveSession) sendTextTurnAndWait(text string, timeout time.Duration) error {
	s.drainTurnComplete()
	if err := s.writeJSON(geminiTextTurn(text)); err != nil {
		return err
	}
	s.waitForTurnComplete(timeout)
	return nil
}

func (s *geminiLiveSession) waitForTurnComplete(timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-s.ctx.Done():
	case <-s.turnCompleteSignal:
	case <-timer.C:
	}
}

func (s *geminiLiveSession) drainTurnComplete() {
	for {
		select {
		case <-s.turnCompleteSignal:
		default:
			return
		}
	}
}

func (s *geminiLiveSession) signalTurnComplete() {
	select {
	case s.turnCompleteSignal <- struct{}{}:
	default:
	}
}

func geminiTextTurn(text string) map[string]interface{} {
	return map[string]interface{}{"clientContent": map[string]interface{}{
		"turns":        []map[string]interface{}{{"role": "user", "parts": []map[string]string{{"text": text}}}},
		"turnComplete": true,
	}}
}

func (s *geminiLiveSession) Interrupt() {
	s.audio.FlushOutput()
	s.setActivity(true)
}

func (s *geminiLiveSession) Events() <-chan VoiceEvent { return s.events }

func (s *geminiLiveSession) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		s.connMu.Lock()
		if s.conn != nil {
			_ = s.conn.Close()
		}
		s.conn = nil
		s.connMu.Unlock()
	})
	return nil
}

func (s *geminiLiveSession) setActivity(active bool) {
	s.stateMu.Lock()
	if s.activity == active {
		s.stateMu.Unlock()
		return
	}
	s.activity = active
	s.stateMu.Unlock()
	if active {
		_ = s.writeJSON(map[string]interface{}{"realtimeInput": map[string]interface{}{"activityStart": map[string]interface{}{}}})
	} else {
		_ = s.writeJSON(map[string]interface{}{"realtimeInput": map[string]interface{}{"activityEnd": map[string]interface{}{}}})
	}
}

func (s *geminiLiveSession) activityEnabled() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.activity
}

func (s *geminiLiveSession) writeJSON(value interface{}) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	conn := s.connection()
	if conn == nil {
		return fmt.Errorf("Gemini Live connection is unavailable")
	}
	return conn.WriteJSON(value)
}

func (s *geminiLiveSession) connection() *websocket.Conn {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.conn
}

func (s *geminiLiveSession) setConnection(conn *websocket.Conn) {
	s.connMu.Lock()
	old := s.conn
	s.conn = conn
	s.connMu.Unlock()
	if old != nil && old != conn {
		_ = old.Close()
	}
}

func (s *geminiLiveSession) emit(eventType, message string, data map[string]any) {
	select {
	case s.events <- VoiceEvent{Type: eventType, Message: message, Data: data, Timestamp: time.Now().UTC()}:
	default:
	}
}

func mapValue(value map[string]interface{}, keys ...string) map[string]interface{} {
	for _, key := range keys {
		if nested, ok := value[key].(map[string]interface{}); ok {
			return nested
		}
	}
	return nil
}

func firstValue(value map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if nested, ok := value[key]; ok {
			return nested
		}
	}
	return nil
}

func stringValue(value map[string]interface{}, keys ...string) string {
	if value == nil {
		return ""
	}
	for _, key := range keys {
		if text, ok := value[key].(string); ok {
			return text
		}
	}
	return ""
}

func boolValue(value map[string]interface{}, keys ...string) bool {
	if value == nil {
		return false
	}
	for _, key := range keys {
		if flag, ok := value[key].(bool); ok {
			return flag
		}
	}
	return false
}
