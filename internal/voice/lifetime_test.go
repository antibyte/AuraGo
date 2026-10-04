package voice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/realtimespeech"
	"github.com/gorilla/websocket"
)

func TestGeminiBurstPlaybackIsPacedAndPreservesEveryFrame(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	bridge := NewBridge(25)
	s := &geminiLiveSession{ctx: ctx, cancel: cancel, audio: bridge, output: make(chan PCMFrame, 1500)}
	samples := make([]int16, 45*160)
	for i := range samples {
		samples[i] = int16(i)
	}
	if !s.queueOutput(samples) {
		t.Fatal("normal burst rejected")
	}
	start := time.Now()
	s.wg.Add(1)
	go s.outputLoop()
	defer func() { cancel(); s.wg.Wait() }()
	for frameIndex := 0; frameIndex < 45; frameIndex++ {
		frame, err := bridge.NextSend(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(frame.Samples) != 160 {
			t.Fatal("unframed audio")
		}
		for i, sample := range frame.Samples {
			if sample != int16(frameIndex*160+i) {
				t.Fatal("burst lost or reordered samples")
			}
		}
	}
	if time.Since(start) < 800*time.Millisecond {
		t.Fatal("audio burst was sent without pacing")
	}
	if !s.queueOutput(make([]int16, 1600)) {
		t.Fatal("second burst rejected")
	}
	s.flushOutput()
	waitCtx, stop := context.WithTimeout(ctx, 70*time.Millisecond)
	defer stop()
	if _, err := bridge.NextSend(waitCtx); err == nil {
		t.Fatal("interrupted output continued")
	}
	if s.queueOutput(make([]int16, 1501*160)) {
		t.Fatal("oversized burst exceeded memory bound")
	}
}

type drainingVoiceRunner struct {
	testVoiceRunner
	started, cancelled, release, finished chan struct{}
}

func (r *drainingVoiceRunner) RunVoiceTurn(ctx context.Context, _ CallContext, _ string) (string, error) {
	close(r.started)
	<-ctx.Done()
	close(r.cancelled)
	<-r.release
	close(r.finished)
	return "late final transcript", nil
}

func TestVoiceSessionCloseWaitsForActiveTurn(t *testing.T) {
	for _, kind := range []string{"classic", "gemini"} {
		t.Run(kind, func(t *testing.T) {
			runner := &drainingVoiceRunner{started: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
			var once sync.Once
			release := func() { once.Do(func() { close(runner.release) }) }
			defer release()
			var session VoiceSession
			var err error
			if kind == "classic" {
				backend := &ClassicBackend{Recognizer: testRecognizer{text: "hello"}, Synthesizer: testSynthesizer{}, Runner: runner}
				session, err = backend.Start(context.Background(), CallContext{CallID: "drain"}, NewBridge(25))
				if err != nil {
					t.Fatal(err)
				}
				session.(*classicSession).startUtterance(make([]int16, 320), 8000)
			} else {
				upgrader := websocket.Upgrader{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					conn, err := upgrader.Upgrade(w, r, nil)
					if err != nil {
						return
					}
					defer conn.Close()
					var setup any
					if conn.ReadJSON(&setup) != nil {
						return
					}
					_ = conn.WriteJSON(map[string]any{"setupComplete": map[string]any{}})
					_ = conn.WriteJSON(map[string]any{"toolCall": map[string]any{"functionCalls": []any{map[string]any{"name": "aurago_execute", "id": "turn", "args": map[string]any{"request": "hello"}}}}})
					for {
						if _, _, err := conn.ReadMessage(); err != nil {
							return
						}
					}
				}))
				defer server.Close()
				backend := &GeminiLiveBackend{Profile: config.RealtimeSpeechProfile{Enabled: true, Provider: realtimespeech.ProviderGemini, APIKey: "fixture"}, Runner: runner, WebSocketURL: "ws" + strings.TrimPrefix(server.URL, "http")}
				session, err = backend.Start(context.Background(), CallContext{CallID: "drain"}, NewBridge(25))
				if err != nil {
					t.Fatal(err)
				}
			}
			defer func() { release(); session.Close() }()
			select {
			case <-runner.started:
			case <-time.After(time.Second):
				t.Fatal("turn did not start")
			}
			closed := make(chan struct{})
			go func() { session.Close(); close(closed) }()
			select {
			case <-runner.cancelled:
			case <-time.After(time.Second):
				t.Fatal("turn did not cancel")
			}
			select {
			case <-closed:
				t.Fatal("Close returned before transcript producer exited")
			default:
			}
			release()
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("Close did not join turn")
			}
			select {
			case <-runner.finished:
			default:
				t.Fatal("producer survived Close")
			}
		})
	}
}

func TestGeminiCloseRejectsLateReconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &geminiLiveSession{ctx: ctx, cancel: cancel, events: make(chan VoiceEvent, 4), handle: "resume"}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var setup any
		if conn.ReadJSON(&setup) != nil {
			return
		}
		close(started)
		<-release
		_ = conn.WriteJSON(map[string]any{"setupComplete": map[string]any{}})
	}))
	defer func() { unblock(); server.Close() }()
	s.backend = &GeminiLiveBackend{Profile: config.RealtimeSpeechProfile{APIKey: "fixture"}, WebSocketURL: "ws" + strings.TrimPrefix(server.URL, "http")}
	done := make(chan bool, 1)
	go func() { done <- s.reconnect() }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("reconnect did not enter setup")
	}
	s.Close()
	unblock()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("closed session resumed")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not close pending setup socket")
	}
	if s.connection() != nil {
		t.Fatal("late socket installed")
	}
}
