package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/phonon"
	"aurago/internal/tools"
)

func TestDesktopChessCommentaryUsesDefaultPhononTTSAndCaches(t *testing.T) {
	t.Parallel()

	s := &Server{
		Cfg: &config.Config{},
	}
	// Even if global TTS points at a cloud provider, chess commentary uses local default Phonon TTS.
	s.Cfg.TTS.Provider = "openai"
	s.Cfg.Server.UILanguage = "de"
	s.Cfg.Personality.CorePersonality = "servant"
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.VirtualDesktop.ReadOnly = true

	var calls atomic.Int32
	var gotCfg tools.TTSConfig
	var gotText string
	stubWAV := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")

	h := newDesktopChessCommentaryHandler(s, func(_ context.Context, cfg tools.TTSConfig, text string) ([]byte, string, error) {
		calls.Add(1)
		gotCfg = cfg
		gotText = text
		return stubWAV, ".wav", nil
	})

	body := `{"category":"knight_rim","piece":"n","square":"a3","lang":"de","seed":1}`
	req := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/desktop/chess/commentary", strings.NewReader(body))
	req.Header.Set("Origin", "https://aurago.local")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}

	var resp desktopChessCommentaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if gotCfg.Provider != "phonon" {
		t.Fatalf("TTS provider = %q, want phonon", gotCfg.Provider)
	}
	wantVoice := phonon.DefaultVoice("de", "servant")
	if gotCfg.Phonon.VoiceID != wantVoice {
		t.Fatalf("Phonon voice = %q, want default %q", gotCfg.Phonon.VoiceID, wantVoice)
	}
	if resp.Language != "de" || resp.Category != "knight_rim" {
		t.Fatalf("unexpected response metadata: %+v", resp)
	}
	if !strings.Contains(resp.Text, "a3") || resp.Text != gotText {
		t.Fatalf("commentary text = %q (synth %q), want interpolated square a3", resp.Text, gotText)
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Audio)
	if err != nil || string(decoded) != string(stubWAV) || resp.MIMEType != "audio/wav" {
		t.Fatalf("unexpected audio payload: mime=%q err=%v", resp.MIMEType, err)
	}

	// Second identical call serves from bounded memory cache without re-invoking Phonon.
	req2 := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/desktop/chess/commentary", strings.NewReader(body))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("expected cached audio with 1 synthesis call, got status=%d calls=%d", rec2.Code, calls.Load())
	}
}

func TestDesktopChessCommentaryAvoidsRepetitionAcrossCycle(t *testing.T) {
	t.Parallel()

	lines := desktopChessCommentaryLines("de", "opening_e4")
	if len(lines) < 5 {
		t.Fatalf("expected rich variant pool for opening_e4, got %d", len(lines))
	}
	seen := make([]int, 0, len(lines))
	seenSet := make(map[int]bool, len(lines))
	for step := 0; step < len(lines); step++ {
		idx, text := pickDesktopChessCommentaryLine("de", "opening_e4", "p", "e4", seen, uint32(step+1))
		if text == "" {
			t.Fatal("empty commentary text")
		}
		if seenSet[idx] {
			t.Fatalf("variant %d repeated before deck exhausted (seen=%v)", idx, seen)
		}
		seenSet[idx] = true
		seen = append(seen, idx)
	}
	// Wrap-around when all variants were heard must not repeat the immediate last variant.
	last := seen[len(seen)-1]
	nextIdx, _ := pickDesktopChessCommentaryLine("de", "opening_e4", "p", "e4", seen, 1)
	if nextIdx == last {
		t.Fatalf("wrap-around repeated immediate previous variant %d", last)
	}
}

func TestDesktopChessCommentaryRejectsUntrustedInputAndGracefullyDegradesAudio(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}}
	h := newDesktopChessCommentaryHandler(s, func(_ context.Context, _ tools.TTSConfig, _ string) ([]byte, string, error) {
		return nil, "", errors.New("phonon runtime missing")
	})

	// Reject GET
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://aurago.local/api/desktop/chess/commentary", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", rec.Code)
	}

	// Reject foreign Origin
	crossReq := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/desktop/chess/commentary", strings.NewReader(`{"category":"undo"}`))
	crossReq.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, crossReq)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want 403", rec.Code)
	}

	// Reject arbitrary text injection, unknown category, bad square, bad piece
	for _, bad := range []string{
		`{"category":"undo","text":"speak arbitrary user text"}`,
		`{"category":"unknown_category"}`,
		`{"category":"knight_rim","square":"z9"}`,
		`{"category":"piece_develop","piece":"dragon"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/desktop/chess/commentary", strings.NewReader(bad))
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("bad payload %s returned %d, want 400", bad, rec.Code)
		}
	}

	// Unsupported UI language falls back cleanly to English Phonon, and synthesis failure still returns text.
	req := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/desktop/chess/commentary", strings.NewReader(`{"category":"stalemate","lang":"ja"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fallback status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp desktopChessCommentaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode fallback response: %v", err)
	}
	if resp.Language != "en" || resp.Text == "" || !resp.AudioUnavailable || resp.Audio != "" {
		t.Fatalf("unexpected fallback response: %+v", resp)
	}
}

func TestDesktopChessCommentaryCatalogCoversAllPhononLanguages(t *testing.T) {
	t.Parallel()

	langs := []string{"de", "en", "es", "fr", "pt"}
	if len(desktopChessCommentaryDE) < 35 {
		t.Fatalf("expected >= 35 commentary categories, got %d", len(desktopChessCommentaryDE))
	}
	for category := range desktopChessCommentaryDE {
		for _, lang := range langs {
			lines := desktopChessCommentaryLines(lang, category)
			if len(lines) == 0 {
				t.Fatalf("language %s missing lines for category %s", lang, category)
			}
			for idx := range lines {
				_, formatted := pickDesktopChessCommentaryLine(lang, category, "n", "f3", nil, uint32(idx+1))
				if strings.TrimSpace(formatted) == "" {
					t.Fatalf("empty formatted line for %s/%s[%s]", lang, category, strconv.Itoa(idx))
				}
				if strings.Contains(formatted, "{square}") || strings.Contains(formatted, "{piece}") {
					t.Fatalf("unreplaced placeholder in %s/%s: %q", lang, category, formatted)
				}
				if err := phonon.ValidateText(formatted); err != nil {
					t.Fatalf("phonon text validation failed for %s/%s: %v", lang, category, err)
				}
			}
		}
	}
}
