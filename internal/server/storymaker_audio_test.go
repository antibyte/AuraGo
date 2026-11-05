package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/phonon"
	"aurago/internal/realtimespeech"
	"aurago/internal/storymaker"
	"aurago/internal/tools"
)

// storymakerClosedPort returns a local port nothing listens on.
func storymakerClosedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func storymakerVoiceIDs(rows []storymaker.VoiceProvider) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

// assertStorymakerVoiceNotesAreFacts fails on a word that judges the sound. Nobody has listened to
// these voices yet, so a note may name a limit and never a quality (ledger D20).
func assertStorymakerVoiceNotesAreFacts(t *testing.T, rows []storymaker.VoiceProvider) {
	t.Helper()
	for _, row := range rows {
		note := strings.ToLower(row.Note)
		for _, word := range []string{"good", "great", "best", "natural", "clear", "pleasant", "recommend", "quality", "sounds"} {
			if strings.Contains(note, word) {
				t.Errorf("%s: the note %q judges the sound (%q)", row.ID, row.Note, word)
			}
		}
	}
}

func TestStorymakerVoicesEndpoint(t *testing.T) {
	t.Setenv("AURAGO_PHONON_DIR", t.TempDir())
	s, readToken, _ := storymakerAPIFixture(t)

	var fetches atomic.Int32
	xai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		_, _ = w.Write([]byte(`{"voices":[{"voice_id":"eve","name":"Eve"}]}`))
	}))
	t.Cleanup(xai.Close)

	const openAIKey, xaiKey = "sk-fixture-openai-secret", "xai-fixture-secret"
	s.Cfg.TTS.Provider = "openai"
	s.Cfg.TTS.OpenAI.APIKey = openAIKey
	s.Cfg.TTS.XAI.APIKey = xaiKey

	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	rt.voices.client = &realtimespeech.Client{HTTPClient: xai.Client(), XAIBaseURL: xai.URL}
	rt.voices.now = func() time.Time { return now }

	get := func() ([]storymaker.VoiceProvider, *httptest.ResponseRecorder) {
		t.Helper()
		w := storymakerRequest(s, readToken, http.MethodGet, "/voices", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /voices = %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Providers []storymaker.VoiceProvider `json:"providers"`
		}
		decodeBody(t, w, &body)
		return body.Providers, w
	}

	rows, w := get()
	// The active provider, then the local ids in list order, then the cloud ids sorted.
	// google is absent, because it is not the active provider.
	want := []string{"openai", "phonon", "piper", "sanotts", "supertonic", "speech_lab", "elevenlabs", "minimax", "mistral", "resemble", "stepfun", "xai"}
	if got := storymakerVoiceIDs(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("provider order = %v", got)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	for _, secret := range []string{openAIKey, xaiKey} {
		if strings.Contains(w.Body.String(), secret) {
			t.Errorf("the answer holds the secret %q", secret)
		}
	}
	for i, row := range rows {
		if row.TestedLanguages == nil {
			t.Errorf("%s: tested_languages is null", row.ID)
		}
		if row.Active != (i == 0) {
			t.Errorf("%s: active = %v", row.ID, row.Active)
		}
	}
	assertStorymakerVoiceNotesAreFacts(t, rows)

	// 13 is the length of the frozen list in tools.OpenAITTSVoices.
	if !rows[0].Ready || len(rows[0].Voices) != 13 {
		t.Errorf("openai row = %+v", rows[0])
	}
	last := rows[len(rows)-1]
	if !last.Ready || len(last.Voices) != 1 || last.Voices[0].ID != "eve" {
		t.Errorf("xai row = %+v", last)
	}

	// The list is fetched once in 5 minutes.
	if fetches.Load() != 1 {
		t.Fatalf("first answer fetched %d times", fetches.Load())
	}
	now = now.Add(5*time.Minute - time.Second)
	get()
	if fetches.Load() != 1 {
		t.Errorf("a second inside the window fetched again: %d", fetches.Load())
	}
	now = now.Add(time.Second)
	get()
	if fetches.Load() != 2 {
		t.Errorf("an expired list was not fetched again: %d", fetches.Load())
	}

	// Read-only changes nothing for this route.
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	get()

	// An active google comes first and is listed once.
	google := *s.ConfigSnapshot()
	google.VirtualDesktop.ReadOnly = false
	google.TTS.Provider = "google"
	s.replaceConfigSnapshot(&google)
	rows, _ = get()
	want = []string{"google", "phonon", "piper", "sanotts", "supertonic", "speech_lab", "elevenlabs", "minimax", "mistral", "openai", "resemble", "stepfun", "xai"}
	if got := storymakerVoiceIDs(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("provider order with google active = %v", got)
	}
	if !rows[0].Active || rows[0].ChunkLimit != 200 {
		t.Errorf("google row = %+v", rows[0])
	}
}

func TestStorymakerPhononRowNeedsReadyAndVoicePath(t *testing.T) {
	missing := errors.New("missing")
	for _, tc := range []struct {
		name               string
		readyErr, voiceErr error
		ready, install     bool
	}{
		{"everything is there", nil, nil, true, false},
		{"no runtime", missing, nil, false, true},
		{"no voice", nil, missing, false, true},
	} {
		ready, install := storymakerPhononRule(true, tc.readyErr, tc.voiceErr)
		if ready != tc.ready || install != tc.install {
			t.Errorf("%s: ready %v, needs_install %v", tc.name, ready, install)
		}
	}

	// An empty resource folder. TTSReady alone says yes for phonon (tts_config.go:166-167), so
	// ready:false below can only come from phonon.Ready and phonon.VoicePath.
	t.Setenv("AURAGO_PHONON_DIR", t.TempDir())
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Language = "de"
	withPhonon := *cfg
	withPhonon.TTS.Provider = "phonon"
	if !tools.TTSReady(&withPhonon, tools.TTSRouteTool) {
		t.Fatal("TTSReady no longer says yes for phonon, so this test proves nothing")
	}
	row := storymakerPhononRow(cfg)
	if row.Ready || !row.NeedsInstall || row.Note == "" {
		t.Fatalf("phonon without files = %+v", row)
	}
	if !reflect.DeepEqual(row.Languages, []string{"de", "en", "es", "fr", "pt"}) || row.ChunkLimit != 2000 || row.TestedLanguages == nil {
		t.Errorf("phonon row = %+v", row)
	}
	if row.DefaultVoice != phonon.DefaultVoice("de", "") {
		t.Errorf("phonon default voice = %q", row.DefaultVoice)
	}
	if len(row.Voices) == 0 {
		t.Fatal("the phonon catalog is empty")
	}
	if row.Voices[0].Installed == nil || *row.Voices[0].Installed {
		t.Errorf("a voice in an empty folder must say it is not installed: %+v", row.Voices[0])
	}
}

func TestStorymakerPiperListFailureIsNotReady(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Piper.Enabled = true
	cfg.TTS.Piper.ContainerPort = storymakerClosedPort(t)
	row := storymakerPiperRow(context.Background(), cfg)
	if row.Ready || !reflect.DeepEqual(row.Languages, []string{"*"}) || row.Note == "" {
		t.Fatalf("piper with a failing list = %+v", row)
	}
	// A working list: the languages are the bases of its locales, sorted and without repeats.
	got := storymakerPiperLanguages([]tools.WyomingVoice{{Languages: []string{"en_US", "en_GB"}}, {Languages: []string{"de_DE"}}})
	if !reflect.DeepEqual(got, []string{"de", "en"}) {
		t.Errorf("piper languages = %v", got)
	}
}

func TestStorymakerSanoListsNoHindiOrChinese(t *testing.T) {
	row := storymakerSanoRow(&config.Config{})
	for _, language := range []string{"hi", "zh"} {
		if slices.Contains(row.Languages, language) {
			t.Errorf("sanotts lists %s, but sanotts.Voice speaks it in English", language)
		}
	}
	if !row.Ready || row.ChunkLimit != 2000 || !slices.Contains(row.Languages, "de") {
		t.Errorf("sanotts row = %+v", row)
	}
}

func TestStorymakerVoiceNotesHoldFactsOnly(t *testing.T) {
	t.Setenv("AURAGO_PHONON_DIR", t.TempDir())
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.TTS.Piper.Enabled = true
	cfg.TTS.Piper.ContainerPort = storymakerClosedPort(t)
	cfg.TTS.Supertonic.URL = "http://127.0.0.1:" + strconv.Itoa(storymakerClosedPort(t))
	rows := (&Server{}).storymakerLocalVoiceRows(context.Background(), cfg)
	if got := storymakerVoiceIDs(rows); !reflect.DeepEqual(got, []string{"phonon", "piper", "sanotts", "supertonic", "speech_lab"}) {
		t.Fatalf("local rows = %v", got)
	}
	// Here phonon has no files, piper and supertonic have failing lists and sanotts has its limit.
	// speech_lab is switched off, so it has nothing to report.
	for _, row := range rows[:4] {
		if row.Note == "" {
			t.Errorf("%s has no note", row.ID)
		}
	}
	assertStorymakerVoiceNotesAreFacts(t, rows)
}
