package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/speechlab"
)

func newSystemWorldVoiceTestServer(t *testing.T) *Server {
	t.Helper()
	s := newDesktopOfficeTestServer(t)
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "memory.db"), s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	s.ShortTermMem = stm
	if err := stm.InitNotesTables(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return s
}

func TestSystemWorldVoiceSourcesAreReadOnly(t *testing.T) {
	const phrase = "Eine kleine Erinnerung an den blauen Himmel."
	for _, source := range []string{"core", "memory", "notes", "desktop", "chat"} {
		t.Run(source, func(t *testing.T) {
			s := newSystemWorldVoiceTestServer(t)
			var err error
			switch source {
			case "core":
				_, err = s.ShortTermMem.AddCoreMemoryFact(phrase)
			case "memory":
				err = s.ShortTermMem.UpsertMemoryMeta("tower-test")
				s.LongTermMem = &lifecycleVectorDB{docs: map[string]string{"tower-test": phrase}}
			case "notes":
				_, err = s.ShortTermMem.AddNote("general", "Test", phrase, 1, "")
			case "desktop":
				var svc *desktop.Service
				svc, _, err = s.getDesktopService(context.Background())
				if err == nil {
					_, err = svc.CreateNote(context.Background(), "Test", phrase, "", desktop.SourceUser)
				}
			case "chat":
				_, err = s.ShortTermMem.InsertMessage("default", "assistant", phrase, false, false)
			}
			if err != nil {
				t.Fatal(err)
			}
			// Hidden turns must never become an alternative source.
			for _, m := range []struct {
				session, role string
				internal      bool
			}{
				{"default", "system", false}, {"default", "tool", false}, {"default", "user", true}, {"heartbeat", "assistant", false}, {"space-agent-bridge", "user", false},
			} {
				if _, err := s.ShortTermMem.InsertMessage(m.session, m.role, "PRIVATE INTERNAL TEXT MUST NOT BE SPOKEN.", false, m.internal); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := s.ShortTermMem.GetAllMemoryMeta(1, 0)
			for i := 0; i < 4; i++ {
				if got := systemWorldVoiceText(context.Background(), s); got != phrase {
					t.Fatalf("source %s: %q", source, got)
				}
			}
			after, _ := s.ShortTermMem.GetAllMemoryMeta(1, 0)
			if len(before) > 0 && (after[0].AccessCount != before[0].AccessCount || after[0].LastAccessed != before[0].LastAccessed) {
				t.Fatal("Sampling mutated memory access metadata")
			}
			if source == "desktop" {
				result, err := s.DesktopService.SearchNotes(context.Background(), desktop.NotesQuery{Limit: 1})
				if err != nil || len(result.Notes) != 1 {
					t.Fatal(err)
				}
				note, err := s.DesktopService.ReadNote(context.Background(), result.Notes[0].Path)
				if err != nil || note.Content != phrase {
					t.Fatal("Sampling modified note")
				}
			}
		})
	}
}

func TestSystemWorldVoiceExcerpt(t *testing.T) {
	const phrase = "Die Pflanzen wachsen langsam zum Fenster."
	secret := "tower-test-sensitive-value"
	security.RegisterSensitive(secret)
	for i := 0; i < 15; i++ {
		got := systemWorldVoiceExcerpt("<think>PRIVATE INNER THOUGHT.</think>\n\x60\x60\x60\nDO NOT READ THIS CODE.\n\x60\x60\x60\n**" + phrase + "**")
		if got != phrase {
			t.Fatalf("markup/thinking/code leaked: %q", got)
		}
		if text := systemWorldVoiceExcerpt("Der Schlüssel lautet " + secret + "."); text != "" {
			t.Fatal("secret leaked")
		}
	}
	long := strings.Repeat("Übergrößenträger ", 40)
	if got := systemWorldVoiceExcerpt(long); !utf8.ValidString(got) || utf8.RuneCountInString(got) > 180 {
		t.Fatal("invalid excerpt bounds")
	}
	if got := systemWorldVoiceExcerpt("abc\xff"); got != "" {
		t.Fatal("accepted invalid UTF-8")
	}
	s := newSystemWorldVoiceTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := systemWorldVoiceText(ctx, s); got != "" {
		t.Fatal("cancelled sampling")
	}
}

func TestSystemWorldVoiceUsesActiveTTS(t *testing.T) {
	for _, provider := range []string{"supertonic", "speech_lab"} {
		t.Run(provider, func(t *testing.T) {
			s := newSystemWorldVoiceTestServer(t)
			const phrase = "Die Sterne leuchten heute besonders hell."
			if _, err := s.ShortTermMem.AddCoreMemoryFact(phrase); err != nil {
				t.Fatal(err)
			}
			wav := make([]byte, 46)
			copy(wav, "RIFF")
			binary.LittleEndian.PutUint32(wav[4:], 38)
			copy(wav[8:], "WAVEfmt ")
			binary.LittleEndian.PutUint32(wav[16:], 16)
			binary.LittleEndian.PutUint16(wav[20:], 1)
			binary.LittleEndian.PutUint16(wav[22:], 1)
			binary.LittleEndian.PutUint32(wav[24:], 16000)
			binary.LittleEndian.PutUint32(wav[28:], 32000)
			binary.LittleEndian.PutUint16(wav[32:], 2)
			binary.LittleEndian.PutUint16(wav[34:], 16)
			copy(wav[36:], "data")
			binary.LittleEndian.PutUint32(wav[40:], 2)
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/ready" {
					_ = json.NewEncoder(w).Encode(speechlab.Ready{Ready: true, TTSOK: true, TTSID: "active-engine", Voice: "ActiveVoice"})
					return
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				field := "text"
				if provider == "speech_lab" {
					field = "input"
				}
				if body[field] != phrase || body["voice"] != "ActiveVoice" {
					t.Errorf("wrong active voice/text: %#v", body)
				}
				calls.Add(1)
				w.Header().Set("Content-Type", "audio/wav")
				w.Header().Set("X-S2S-TTS-ID", "active-engine")
				w.Header().Set("X-S2S-Voice", "ActiveVoice")
				_, _ = w.Write(wav)
			}))
			defer upstream.Close()
			s.Cfg.TTS.Provider = "supertonic"
			s.Cfg.TTS.Supertonic.URL = upstream.URL
			s.Cfg.TTS.Supertonic.Voice = "ActiveVoice"
			s.Cfg.TTS.Supertonic.ResponseFormat = "wav"
			if provider == "speech_lab" {
				// Active Speech Lab output overrides the global provider and ignores legacy voice fields.
				s.Cfg.SpeechLab = config.SpeechLabConfig{Enabled: true, BaseURL: upstream.URL, ChatOutputEnabled: true, Voice: "Obsolete", Language: "de"}
			}
			h := handleSystemWorldVoice(s)
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest("POST", "/api/desktop/system-world/voice", nil))
			if rec.Code != 200 || rec.Header().Get("Content-Type") != "audio/wav" || rec.Body.Len() != len(wav) || calls.Load() != 1 {
				t.Fatalf("voice response: %d %s calls=%d", rec.Code, rec.Body.String(), calls.Load())
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("voice may be cached")
			}
			if _, err := os.Stat(filepath.Join(s.Cfg.Directories.DataDir, "tts")); !os.IsNotExist(err) {
				t.Fatal("speech created persistent media")
			}
			rec = httptest.NewRecorder()
			h(rec, httptest.NewRequest("POST", "/api/desktop/system-world/voice", nil))
			if rec.Code != 429 || calls.Load() != 1 {
				t.Fatal("missing request cooldown")
			}
		})
	}
}

func TestSystemWorldVoicePermissionsAndCancel(t *testing.T) {
	s := newSystemWorldVoiceTestServer(t)
	s.Cfg.TTS.Provider = "supertonic"
	s.Cfg.TTS.Supertonic.URL = "http://127.0.0.1:1"
	for _, tc := range []struct {
		method, token string
		auth          bool
		want          int
	}{
		{"GET", "", false, 405}, {"POST", "bad", false, 403}, {"POST", "", true, 401},
	} {
		s.Cfg.Auth.Enabled = tc.auth
		r := httptest.NewRequest(tc.method, "/api/desktop/system-world/voice", nil)
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		handleSystemWorldVoice(s)(w, r)
		if w.Code != tc.want {
			t.Fatalf("gate: %d want %d", w.Code, tc.want)
		}
	}
	// Valid read/write tokens still cannot read owner memory through audio.
	gated, read, write := testDesktopPermissionServer(t)
	for _, token := range []string{read, write} {
		r := httptest.NewRequest("POST", "/api/desktop/system-world/voice", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handleSystemWorldVoice(gated)(w, r)
		if w.Code != 403 {
			t.Fatal("scoped token gained global memory access")
		}
	}
	s.Cfg.Auth.Enabled = false
	w := httptest.NewRecorder()
	handleSystemWorldVoice(s)(w, httptest.NewRequest("POST", "/api/desktop/system-world/voice", nil))
	if w.Code != 204 {
		t.Fatalf("empty sources: %d", w.Code)
	}
	if _, err := s.ShortTermMem.AddCoreMemoryFact("Die Erinnerung bleibt unverändert bestehen."); err != nil {
		t.Fatal(err)
	}
	started, finished := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
		close(finished)
	}))
	defer upstream.Close()
	s.Cfg.TTS.Supertonic.URL = upstream.URL
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	h := handleSystemWorldVoice(s)
	go func() {
		defer close(done)
		h(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/desktop/system-world/voice", nil).WithContext(ctx))
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("TTS did not start")
	}
	w = httptest.NewRecorder()
	h(w, httptest.NewRequest("POST", "/api/desktop/system-world/voice", nil))
	if w.Code != 429 {
		t.Fatal("concurrent synthesis allowed")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not cancel")
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not cancel")
	}
}
