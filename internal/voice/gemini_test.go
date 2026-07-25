package voice

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/realtimespeech"

	"github.com/gorilla/websocket"
)

type geminiTestRunner struct {
	executed    chan string
	cancel      atomic.Int32
	end         atomic.Int32
	internalEnd atomic.Int32
	internalWhy atomic.Value
}

func (r *geminiTestRunner) RunVoiceTurn(_ context.Context, _ CallContext, request string) (string, error) {
	r.executed <- request
	return "done", nil
}
func (r *geminiTestRunner) CancelVoiceTurn(string) { r.cancel.Add(1) }
func (r *geminiTestRunner) EndVoiceCall(string)    { r.end.Add(1) }
func (r *geminiTestRunner) EndVoiceCallInternal(_ string, reason string) {
	r.internalWhy.Store(reason)
	r.internalEnd.Add(1)
}

func TestGeminiLiveSetupAudioToolsInterruptAndResumption(t *testing.T) {
	var connections atomic.Int32
	serverErrors := make(chan string, 4)
	resumedSetup := make(chan bool, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErrors <- err.Error()
			return
		}
		defer conn.Close()
		index := connections.Add(1)
		var setup map[string]interface{}
		if err := conn.ReadJSON(&setup); err != nil {
			serverErrors <- err.Error()
			return
		}
		if index == 2 {
			body, _ := setup["setup"].(map[string]interface{})
			_, resumed := body["sessionResumption"]
			resumedSetup <- resumed
		}
		if err := conn.WriteJSON(map[string]interface{}{"setupComplete": map[string]interface{}{}}); err != nil {
			serverErrors <- err.Error()
			return
		}
		if index == 1 {
			_ = conn.WriteJSON(map[string]interface{}{"sessionResumptionUpdate": map[string]interface{}{"resumable": true, "newHandle": "resume-1"}})
			return
		}
		pcm := make([]byte, 480)
		for i := 0; i < len(pcm); i += 2 {
			binary.LittleEndian.PutUint16(pcm[i:i+2], uint16(int16(1000)))
		}
		_ = conn.WriteJSON(map[string]interface{}{"serverContent": map[string]interface{}{
			"inputTranscription": map[string]interface{}{"text": "Hallo"},
			"modelTurn": map[string]interface{}{"parts": []interface{}{map[string]interface{}{"inlineData": map[string]interface{}{
				"mimeType": "audio/pcm;rate=24000", "data": base64.StdEncoding.EncodeToString(pcm),
			}}}},
		}})
		_ = conn.WriteJSON(map[string]interface{}{"toolCall": map[string]interface{}{"functionCalls": []interface{}{
			map[string]interface{}{"id": "tool-1", "name": "aurago_execute", "args": map[string]interface{}{"request": "status"}},
			map[string]interface{}{"id": "tool-2", "name": "aurago_cancel_current_task", "args": map[string]interface{}{}},
			map[string]interface{}{"id": "tool-3", "name": "aurago_end_call", "args": map[string]interface{}{}},
		}}})
		for range 3 {
			var response map[string]interface{}
			if err := conn.ReadJSON(&response); err != nil {
				serverErrors <- err.Error()
				return
			}
		}
		_ = conn.WriteJSON(map[string]interface{}{"serverContent": map[string]interface{}{"interrupted": true}})
		<-r.Context().Done()
	}))
	defer server.Close()

	runner := &geminiTestRunner{executed: make(chan string, 1)}
	bridge := NewBridge(8)
	backend := &GeminiLiveBackend{
		Profile: config.RealtimeSpeechProfile{ID: "gemini", Enabled: true, Provider: realtimespeech.ProviderGemini, Model: "gemini-live-test", Voice: "Aoede", APIKey: "test-key"},
		Runner:  runner, WebSocketURL: "ws" + strings.TrimPrefix(server.URL, "http"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	session, err := backend.Start(ctx, CallContext{CallID: "call-1", AllowedTools: []string{}}, bridge)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	select {
	case resumed := <-resumedSetup:
		if !resumed {
			t.Fatal("reconnected setup omitted session resumption handle")
		}
	case errText := <-serverErrors:
		t.Fatal(errText)
	case <-ctx.Done():
		t.Fatal("Gemini session did not reconnect")
	}
	frame, err := bridge.NextSend(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if frame.SampleRate != 8000 || len(frame.Samples) == 0 {
		t.Fatalf("unexpected Gemini output frame: rate=%d samples=%d", frame.SampleRate, len(frame.Samples))
	}
	select {
	case request := <-runner.executed:
		if request != "status" {
			t.Fatalf("private execute request = %q", request)
		}
	case <-ctx.Done():
		t.Fatal("Gemini private tool was not executed")
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	foundTranscript, foundResumed, foundInterrupted := false, false, false
	for !(foundTranscript && foundResumed && foundInterrupted) {
		select {
		case event := <-session.Events():
			switch event.Type {
			case "transcript":
				foundTranscript = true
			case "session_resumed":
				foundResumed = true
			case "interrupted":
				foundInterrupted = true
			}
		case <-deadline.C:
			t.Fatalf("missing Gemini events transcript=%v resumed=%v interrupted=%v", foundTranscript, foundResumed, foundInterrupted)
		}
	}
	if runner.cancel.Load() != 1 || runner.end.Load() != 1 {
		t.Fatalf("private controls cancel=%d end=%d", runner.cancel.Load(), runner.end.Load())
	}
}

func TestGeminiLiveDoesNotClaimResumptionWithoutHandle(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		connections.Add(1)
		var setup map[string]interface{}
		if conn.ReadJSON(&setup) != nil {
			return
		}
		_ = conn.WriteJSON(map[string]interface{}{"setupComplete": map[string]interface{}{}})
	}))
	defer server.Close()

	backend := &GeminiLiveBackend{
		Profile:      config.RealtimeSpeechProfile{ID: "gemini", Enabled: true, Provider: realtimespeech.ProviderGemini, Model: "gemini-live-test", APIKey: "test-key"},
		Runner:       &geminiTestRunner{executed: make(chan string, 1)},
		WebSocketURL: "ws" + strings.TrimPrefix(server.URL, "http"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	session, err := backend.Start(ctx, CallContext{CallID: "call-no-handle"}, NewBridge(2))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	for {
		select {
		case event := <-session.Events():
			if event.Type == "session_resumed" {
				t.Fatal("session was reported resumed without a provider handle")
			}
			if event.Type == "voice_backend_error" {
				if got := connections.Load(); got != 1 {
					t.Fatalf("connections=%d, want no contextless reconnect", got)
				}
				return
			}
		case <-ctx.Done():
			t.Fatal("missing controlled backend error after connection loss")
		}
	}
}

func TestGeminiLiveSendsGreetingAsServerSideTextTurn(t *testing.T) {
	greetingFrame := make(chan map[string]interface{}, 1)
	serverErrors := make(chan error, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.Close()
		var setup map[string]interface{}
		if err := conn.ReadJSON(&setup); err != nil {
			serverErrors <- err
			return
		}
		if err := conn.WriteJSON(map[string]interface{}{"setupComplete": map[string]interface{}{}}); err != nil {
			serverErrors <- err
			return
		}
		var frame map[string]interface{}
		if err := conn.ReadJSON(&frame); err != nil {
			serverErrors <- err
			return
		}
		greetingFrame <- frame
		<-r.Context().Done()
	}))
	defer server.Close()

	backend := &GeminiLiveBackend{
		Profile: config.RealtimeSpeechProfile{
			ID: "gemini", Enabled: true, Provider: realtimespeech.ProviderGemini, Model: "gemini-live-test", Voice: "Kore", APIKey: "test-key",
		},
		Runner: &geminiTestRunner{executed: make(chan string, 1)}, WebSocketURL: "ws" + strings.TrimPrefix(server.URL, "http"),
		Greeting: "Guten Tag, hier ist AuraGo.", IdleTimeout: time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	session, err := backend.Start(ctx, CallContext{CallID: "call-greeting"}, NewBridge(2))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	select {
	case frame := <-greetingFrame:
		encoded, _ := json.Marshal(frame)
		if !strings.Contains(string(encoded), "clientContent") || !strings.Contains(string(encoded), backend.Greeting) {
			t.Fatalf("Gemini greeting frame = %s", encoded)
		}
	case err := <-serverErrors:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal("Gemini Live greeting was not sent after setup")
	}
}

func TestGeminiResumptionRequiresResumableHandleAndHonorsGoAway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &geminiLiveSession{
		ctx: ctx, cancel: cancel, events: make(chan VoiceEvent, 4), handle: "stale-handle",
	}
	if reconnect := session.handlePayload(map[string]interface{}{
		"sessionResumptionUpdate": map[string]interface{}{"resumable": false, "newHandle": "ignored"},
	}); reconnect {
		t.Fatal("resumption update unexpectedly requested reconnect")
	}
	session.stateMu.Lock()
	handle := session.handle
	session.stateMu.Unlock()
	if handle != "" {
		t.Fatalf("non-resumable handle retained as %q", handle)
	}
	if reconnect := session.handlePayload(map[string]interface{}{"goAway": map[string]interface{}{}}); !reconnect {
		t.Fatal("GoAway did not request proactive reconnect")
	}
}

func TestGeminiProviderErrorPlaysFailureBeforeErrorEvent(t *testing.T) {
	serverErrors := make(chan error, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.Close()
		var setup map[string]interface{}
		if err := conn.ReadJSON(&setup); err != nil {
			serverErrors <- err
			return
		}
		if err := conn.WriteJSON(map[string]interface{}{"setupComplete": map[string]interface{}{}}); err != nil {
			serverErrors <- err
			return
		}
		if err := conn.WriteJSON(map[string]interface{}{"error": map[string]interface{}{"status": "UNAVAILABLE"}}); err != nil {
			serverErrors <- err
			return
		}
		var failureTurn map[string]interface{}
		if err := conn.ReadJSON(&failureTurn); err != nil {
			serverErrors <- err
			return
		}
		encoded, _ := json.Marshal(failureTurn)
		if !strings.Contains(string(encoded), "Technischer Fehler") {
			serverErrors <- fmt.Errorf("failure turn = %s", encoded)
			return
		}
		pcm := make([]byte, 480)
		for i := 0; i < len(pcm); i += 2 {
			binary.LittleEndian.PutUint16(pcm[i:i+2], uint16(int16(750)))
		}
		if err := conn.WriteJSON(map[string]interface{}{"serverContent": map[string]interface{}{
			"modelTurn": map[string]interface{}{"parts": []interface{}{map[string]interface{}{"inlineData": map[string]interface{}{
				"mimeType": "audio/pcm;rate=24000", "data": base64.StdEncoding.EncodeToString(pcm),
			}}}},
			"turnComplete": true,
		}}); err != nil {
			serverErrors <- err
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	bridge := NewBridge(4)
	backend := &GeminiLiveBackend{
		Profile: config.RealtimeSpeechProfile{
			ID: "gemini", Enabled: true, Provider: realtimespeech.ProviderGemini, Model: "gemini-live-test", APIKey: "test-key",
		},
		Runner: &geminiTestRunner{executed: make(chan string, 1)}, WebSocketURL: "ws" + strings.TrimPrefix(server.URL, "http"),
		FailureMessage: "Technischer Fehler.", IdleTimeout: time.Second,
	}
	session, err := backend.Start(ctx, CallContext{CallID: "call-provider-error"}, bridge)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	frame, err := bridge.NextSend(ctx)
	if err != nil {
		t.Fatalf("failure announcement audio was not delivered: %v", err)
	}
	if frame.SampleRate != 8000 || len(frame.Samples) == 0 {
		t.Fatalf("failure audio frame = %+v", frame)
	}
	for {
		select {
		case err := <-serverErrors:
			t.Fatal(err)
		case event := <-session.Events():
			if event.Type == "voice_backend_error" {
				return
			}
		case <-ctx.Done():
			t.Fatal("missing backend error after failure announcement")
		}
	}
}

func TestGeminiInactivityPausesDuringToolWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := &geminiTestRunner{executed: make(chan string, 1)}
	session := &geminiLiveSession{
		ctx: ctx, cancel: cancel, backend: &GeminiLiveBackend{Runner: runner, IdleTimeout: 20 * time.Millisecond},
		call: CallContext{CallID: "call-busy"}, events: make(chan VoiceEvent, 4),
		activitySignal: make(chan struct{}, 1), turnCompleteSignal: make(chan struct{}, 1),
	}
	session.busyTasks.Store(1)
	session.signalActivity()
	session.wg.Add(1)
	go session.idleLoop()
	time.Sleep(60 * time.Millisecond)
	if runner.internalEnd.Load() != 0 {
		t.Fatal("inactivity ended Gemini call while tool work was active")
	}
	session.busyTasks.Store(0)
	session.signalActivity()
	deadline := time.Now().Add(time.Second)
	for runner.internalEnd.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if runner.internalEnd.Load() != 1 || runner.end.Load() != 0 || runner.internalWhy.Load() != "inactivity_timeout" {
		t.Fatalf("Gemini inactivity did not resume, internal ends=%d reason=%v agent ends=%d", runner.internalEnd.Load(), runner.internalWhy.Load(), runner.end.Load())
	}
	cancel()
	session.wg.Wait()
}
