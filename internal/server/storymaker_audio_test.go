package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/phonon"
	"aurago/internal/realtimespeech"
	"aurago/internal/security"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/storymaker/storymakertest"
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

// storymakerAudioProject makes a project whose story has this narration and voice.
func storymakerAudioProject(t *testing.T, s *Server, token, narration string, voice *storymaker.Voice) string {
	t.Helper()
	p := createStorymakerProject(t, s, token)
	story := p.Story
	story.Voice = voice
	story.Scenes[0].Narration = narration
	data, err := storymaker.CanonicalJSON(story)
	if err != nil {
		t.Fatal(err)
	}
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", bytes.NewReader(data), map[string]string{"If-Match": p.ETag})
	if w.Code != http.StatusOK {
		t.Fatalf("save story = %d %s", w.Code, w.Body.String())
	}
	return p.Project.ID
}

func TestStorymakerAudioRoutes(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	id := createStorymakerProject(t, s, writeToken).Project.ID
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	hash := strings.Repeat("a", 64)
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i)
	}
	env := storymaker.Envelope{V: 1, DurationMS: 100, Rate: 100, Levels: make(storymaker.Levels, 10)}
	if err := rt.projects.WritePiece(ctx, id, hash, "wav", data, env); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(s.ConfigSnapshot().VirtualDesktop.WorkspaceDir, "Documents", "Storymaker", id, "audio", "narration")
	base := "/projects/" + id + "/audio/"

	// The whole file, with the fixed headers of the route.
	w := storymakerRequest(s, readToken, http.MethodGet, base+"narration/"+hash+".wav", nil, nil)
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatalf("full GET = %d, %d bytes", w.Code, w.Body.Len())
	}
	for header, want := range map[string]string{
		"Content-Type":            "audio/wav",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "sandbox; default-src 'none'",
		"Cache-Control":           "private, max-age=86400, immutable",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	// A range.
	w = storymakerRequest(s, readToken, http.MethodGet, base+"narration/"+hash+".wav", nil, map[string]string{"Range": "bytes=10-19"})
	if w.Code != http.StatusPartialContent || !bytes.Equal(w.Body.Bytes(), data[10:20]) || w.Header().Get("Content-Range") != "bytes 10-19/1000" {
		t.Errorf("range GET = %d %q, %d bytes", w.Code, w.Header().Get("Content-Range"), w.Body.Len())
	}

	// HEAD sends the length and no body.
	w = storymakerRequest(s, readToken, http.MethodHead, base+"narration/"+hash+".wav", nil, nil)
	if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Content-Length") != "1000" {
		t.Errorf("HEAD = %d, %d bytes, length %q", w.Code, w.Body.Len(), w.Header().Get("Content-Length"))
	}

	// A file over the 50 MB limit of Store.ReadFile still serves, through Store.Open.
	big := strings.Repeat("c", 64)
	f, err := os.Create(filepath.Join(folder, big+".mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(51 << 20); err != nil { // 51 MiB is 53477376 bytes
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := rt.store.ReadFile(ctx, "Documents/Storymaker/"+id+"/audio/narration/"+big+".mp3"); err == nil {
		t.Fatal("ReadFile took the big file, so this case proves nothing")
	}
	// The last 6 bytes: 53477376 - 6 = 53477370.
	w = storymakerRequest(s, readToken, http.MethodGet, base+"narration/"+big+".mp3", nil, map[string]string{"Range": "bytes=53477370-53477375"})
	if w.Code != http.StatusPartialContent || w.Body.Len() != 6 || w.Header().Get("Content-Type") != "audio/mpeg" {
		t.Errorf("big file = %d, %d bytes, %q", w.Code, w.Body.Len(), w.Header().Get("Content-Type"))
	}

	// Only wav and mp3 files with a lower-case hash name, in two folders. The envelope exists on
	// disk, so its 404 comes from the route rule.
	if _, err := os.Stat(filepath.Join(folder, hash+".env.json")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"exports/" + hash + ".wav",
		"narration/" + hash + ".env.json",
		"narration/" + hash + ".meta.json",
		"narration/" + hash + ".pending.json",
		"narration/" + strings.ToUpper(hash) + ".wav",
		"narration/" + hash + ".ogg",
		"narration/" + strings.Repeat("b", 64) + ".wav",
		"music/" + hash + ".mp3",
	} {
		w := storymakerRequest(s, readToken, http.MethodGet, base+path, nil, nil)
		if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"code":"audio_not_found"`) {
			t.Errorf("%s = %d %s", path, w.Code, w.Body.String())
		}
	}

	w = storymakerRequest(s, writeToken, http.MethodPost, base+"narration/"+hash+".wav", nil, nil)
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST = %d, Allow %q", w.Code, w.Header().Get("Allow"))
	}
}

func TestStorymakerNarrationAnswer(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	id := storymakerAudioProject(t, s, writeToken, "This is a short story about a firewall.", &storymaker.Voice{Provider: "sanotts", Language: "de"})

	w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+id+"/narration", nil, nil)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("narration = %d %q %s", w.Code, w.Header().Get("Cache-Control"), w.Body.String())
	}
	var got storymaker.AudioAnswer
	decodeBody(t, w, &got)
	// The text is one sentence of 39 code points (8 words, width 39, so nothing merges or splits) and
	// nothing is stored yet: one missing block, an empty narration map and no music.
	if len(got.Missing) != 1 || got.Missing[0].BlockID != "s1:n" || got.Missing[0].Chars != 39 || got.Chars.Total != 39 || got.Chars.Missing != 39 {
		t.Errorf("missing = %+v, chars = %+v", got.Missing, got.Chars)
	}
	if got.Narration == nil || len(got.Narration) != 0 || got.Music != nil || got.MusicIssue != "" {
		t.Errorf("answer = %s", w.Body.String())
	}

	unknown := storymakerRequest(s, readToken, http.MethodGet, "/projects/33333333-3333-4333-8333-333333333333/narration", nil, nil)
	if unknown.Code != http.StatusNotFound || !strings.Contains(unknown.Body.String(), `"code":"project_not_found"`) {
		t.Errorf("unknown project = %d %s", unknown.Code, unknown.Body.String())
	}

	// A story.json damaged by a hand edit.
	file := filepath.Join(s.ConfigSnapshot().VirtualDesktop.WorkspaceDir, "Documents", "Storymaker", id, "story.json")
	if err := os.WriteFile(file, []byte(`{"schema": "storymaker/1", "scenes": [`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/projects/" + id + "/narration"},
		{http.MethodPost, "/projects/" + id + "/narrate"},
	} {
		w := storymakerRequest(s, writeToken, tc.method, tc.path, nil, nil)
		if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), `"code":"invalid_story"`) {
			t.Errorf("damaged story %s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

// A Phonon voice that is not installed fails in the job, so the route refuses it first and the alternatives never name it.
// Settings offers every Piper voice, so that flag decides nothing.
func TestStorymakerVoiceListCheckKnowsInstalledPhononVoices(t *testing.T) {
	yes, no := true, false
	voices := []storymaker.VoiceOption{
		{ID: "absent", Language: "fr", Installed: &no},
		{ID: "there", Language: "fr", Installed: &yes},
		{ID: "unsaid", Language: "fr"},
	}
	phononRow := storymaker.VoiceProvider{ID: "phonon", Voices: voices}
	for voice, want := range map[string]bool{"absent": false, "there": true, "unsaid": true, "": true, "nobody": false} {
		if got := storymakerVoiceOnList(phononRow, voice); got != want {
			t.Errorf("phonon voice %q on the list = %v, want %v", voice, got, want)
		}
	}
	if got := storymakerVoiceAlternatives(phononRow, "fr"); !reflect.DeepEqual(got, []string{"there", "unsaid"}) {
		t.Errorf("phonon alternatives = %v", got)
	}
	piperRow := storymaker.VoiceProvider{ID: "piper", Voices: voices}
	if !storymakerVoiceOnList(piperRow, "absent") {
		t.Error("the piper flag refused a voice")
	}
	if got := storymakerVoiceAlternatives(piperRow, "fr"); !reflect.DeepEqual(got, []string{"absent", "there", "unsaid"}) {
		t.Errorf("piper alternatives = %v", got)
	}
}

func TestStorymakerNarrateQueuesOneJob(t *testing.T) {
	s, _, writeToken := storymakerAPIFixture(t)
	release := make(chan struct{})
	// Registered after the fixture's cleanup, so it runs first and the manager can close.
	t.Cleanup(func() { close(release) })
	s.storymakerAudioRunOverride = func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return render.Output{}, nil
	}
	post := func(id string) (*httptest.ResponseRecorder, *render.Job) {
		t.Helper()
		w := storymakerRequest(s, writeToken, http.MethodPost, "/projects/"+id+"/narrate", nil, nil)
		var body struct {
			Job *render.Job `json:"job"`
		}
		decodeBody(t, w, &body)
		return w, body.Job
	}

	id := storymakerAudioProject(t, s, writeToken, "This is a short story about a firewall.", &storymaker.Voice{Provider: "sanotts", Language: "de"})
	w, first := post(id)
	if w.Code != http.StatusAccepted || first == nil || first.Kind != render.KindNarrate || first.ProjectID != id {
		t.Fatalf("new job = %d %s", w.Code, w.Body.String())
	}
	w, second := post(id)
	if w.Code != http.StatusOK || second == nil || second.ID != first.ID {
		t.Fatalf("existing job = %d %s", w.Code, w.Body.String())
	}

	// A story with nothing to say has nothing missing.
	empty := createStorymakerProject(t, s, writeToken).Project.ID
	w, none := post(empty)
	if w.Code != http.StatusOK || none != nil || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("nothing missing = %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerNarrateRefusals(t *testing.T) {
	t.Setenv("AURAGO_PHONON_DIR", t.TempDir())
	s, _, writeToken := storymakerAPIFixture(t)
	s.Cfg.TTS.OpenAI.APIKey = "sk-fixture"

	type refusal struct {
		Code         string   `json:"code"`
		Message      string   `json:"message"`
		Alternatives []string `json:"alternatives"`
		Limit        int      `json:"limit"`
		Missing      int      `json:"missing"`
	}
	refuse := func(t *testing.T, narration string, voice *storymaker.Voice) refusal {
		t.Helper()
		id := storymakerAudioProject(t, s, writeToken, narration, voice)
		w := storymakerRequest(s, writeToken, http.MethodPost, "/projects/"+id+"/narrate", nil, nil)
		if w.Code != http.StatusConflict {
			t.Fatalf("narrate = %d %s", w.Code, w.Body.String())
		}
		var got refusal
		decodeBody(t, w, &got)
		return got
	}
	const short = "This is a short story about a firewall."
	// One sentence of 151 code points: under the 400-rune limit of openai and alone in its block, so
	// it stays one piece and missing counts all 151.
	long := strings.Repeat("a", 150) + "."

	t.Run("no voice names the first block", func(t *testing.T) {
		got := refuse(t, short, nil)
		if got.Code != "voice_required" || !strings.Contains(got.Message, "s1:n") {
			t.Errorf("refusal = %+v", got)
		}
	})
	t.Run("provider not ready", func(t *testing.T) {
		// Piper is switched off in this config, so TTSReady says no.
		got := refuse(t, short, &storymaker.Voice{Provider: "piper", Voice: "de_DE-thorsten-high", Language: "de"})
		if got.Code != "narration_unavailable" || !strings.Contains(got.Message, "piper") {
			t.Errorf("refusal = %+v", got)
		}
	})
	t.Run("inactive google", func(t *testing.T) {
		got := refuse(t, short, &storymaker.Voice{Provider: "google", Language: "de"})
		if got.Code != "narration_unavailable" || !strings.Contains(got.Message, "google") {
			t.Errorf("refusal = %+v", got)
		}
	})
	t.Run("voice not on the list", func(t *testing.T) {
		got := refuse(t, short, &storymaker.Voice{Provider: "openai", Voice: "nobody", Language: "en"})
		// The openai list has 13 voices and the answer is capped at 5.
		if got.Code != "voice_unknown" || !strings.Contains(got.Message, "s1:n") || len(got.Alternatives) != 5 {
			t.Fatalf("refusal = %+v", got)
		}
		for _, voice := range got.Alternatives {
			if !slices.Contains(tools.OpenAITTSVoices(), voice) {
				t.Errorf("alternative %q is not an openai voice", voice)
			}
		}
	})
	t.Run("job cap", func(t *testing.T) {
		s.Cfg.Storymaker.Narration.MaxJobChars = 100
		got := refuse(t, long, &storymaker.Voice{Provider: "openai", Voice: "alloy", Language: "en"})
		if got.Code != "budget_exceeded" || got.Limit != 100 || got.Missing != 151 {
			t.Errorf("refusal = %+v", got)
		}
	})
	t.Run("daily cap", func(t *testing.T) {
		s.Cfg.Storymaker.Narration.MaxJobChars = 20000
		s.Cfg.Storymaker.Narration.MaxDailyChars = 1000
		rt, err := s.storymakerRuntime()
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(storymakerDir(rt.dataDir), "narrate")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		usage := fmt.Sprintf(`{"v":1,"date":%q,"chars":950}`, time.Now().Format("2006-01-02"))
		if err := os.WriteFile(filepath.Join(dir, "usage.json"), []byte(usage), 0o600); err != nil {
			t.Fatal(err)
		}
		// 950 used today plus 151 missing is 1101, over the daily 1000. The job cap of 20000 is not reached.
		got := refuse(t, long, &storymaker.Voice{Provider: "openai", Voice: "alloy", Language: "en"})
		if got.Code != "budget_exceeded" || got.Limit != 1000 || got.Missing != 151 {
			t.Errorf("refusal = %+v", got)
		}
		// A damaged file is a full count, so the route refuses as the job would.
		if err := os.WriteFile(filepath.Join(dir, "usage.json"), []byte(`{"chars":`), 0o600); err != nil {
			t.Fatal(err)
		}
		got = refuse(t, short, &storymaker.Voice{Provider: "openai", Voice: "alloy", Language: "en"})
		if got.Code != "budget_exceeded" || got.Limit != 1000 || got.Missing != 39 {
			t.Errorf("refusal with a damaged file = %+v", got)
		}
	})
	t.Run("read-only", func(t *testing.T) {
		id := storymakerAudioProject(t, s, writeToken, short, &storymaker.Voice{Provider: "sanotts", Language: "de"})
		ro := *s.ConfigSnapshot()
		ro.VirtualDesktop.ReadOnly = true
		s.replaceConfigSnapshot(&ro)
		w := storymakerRequest(s, writeToken, http.MethodPost, "/projects/"+id+"/narrate", nil, nil)
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "desktop_readonly") {
			t.Errorf("read-only narrate = %d %s", w.Code, w.Body.String())
		}
	})
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

// musicRouteWAV builds a silent 8 kHz mono PCM16 WAV. The route reads its header only. The job's meter reads silence, so
// the meta keeps no level, and the tests that compare a whole MusicMeta hold with or without FFmpeg.
func musicRouteWAV(seconds float64) []byte {
	const rate = 8000
	frames := int(seconds * rate)
	data := make([]byte, 44+frames*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+frames*2))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(frames*2))
	return data
}

func writeMusicSource(t *testing.T, s *Server, rel string, data []byte) {
	t.Helper()
	target := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func musicRouteStory(t *testing.T, s *Server, token string, p *apiProject, music string) {
	t.Helper()
	doc := `{"schema":"storymaker/1","title":"Music","music":` + music + `,"scenes":[{"id":"s1"}]}`
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(doc), map[string]string{"If-Match": p.ETag})
	var saved struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &saved)
	if w.Code != http.StatusOK {
		t.Fatalf("save story: %d %s", w.Code, w.Body.String())
	}
	p.ETag = saved.ETag
}

func postMusic(s *Server, token, id, body string) *httptest.ResponseRecorder {
	return storymakerRequest(s, token, http.MethodPost, "/projects/"+id+"/music", strings.NewReader(body), nil)
}

func TestStorymakerMusicRouteRefusesBadSources(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, token)
	id := p.Project.ID

	// The bytes decide, never the name: OggS is neither WAV nor MP3.
	writeMusicSource(t, s, "Documents/theme.ogg", append([]byte("OggS"), make([]byte, 64)...))
	// One byte over the 1 GiB limit. Truncate writes nothing, so this takes no time.
	writeMusicSource(t, s, "Documents/big.wav", musicRouteWAV(0.1))
	big, err := os.OpenFile(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Documents", "big.wav"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := big.Truncate(1<<30 + 1); err != nil {
		t.Fatal(err)
	}
	if err := big.Close(); err != nil {
		t.Fatal(err)
	}
	// 0.5 s is under the 1 s minimum.
	writeMusicSource(t, s, "Documents/short.wav", musicRouteWAV(0.5))

	type answer struct {
		Code       string `json:"code"`
		MusicIssue string `json:"music_issue"`
	}
	for _, file := range []string{"theme.ogg", "big.wav", "short.wav"} {
		musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/`+file+`"}`)
		w := postMusic(s, token, id, "")
		var got answer
		decodeBody(t, w, &got)
		if w.Code != http.StatusUnprocessableEntity || got.Code != "invalid_music_file" || got.MusicIssue != "" {
			t.Fatalf("%s: %d %s", file, w.Code, w.Body.String())
		}
	}

	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/gone.wav"}`)
	w := postMusic(s, token, id, "")
	var got answer
	decodeBody(t, w, &got)
	if w.Code != http.StatusUnprocessableEntity || got.Code != "invalid_music_file" || got.MusicIssue != "source_missing" {
		t.Fatalf("missing source: %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerMusicRouteGeneratedMode(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, token)
	id := p.Project.ID
	musicRouteStory(t, s, token, &p, `{"mode":"generated","mood":"calm-ambient"}`)
	code := func(w *httptest.ResponseRecorder) string {
		t.Helper()
		var got struct {
			Code string `json:"code"`
		}
		decodeBody(t, w, &got)
		return got.Code
	}

	// Empty means 0. The range is 1000 to max_duration_seconds*1000.
	tooLong := fmt.Sprintf(`{"duration_ms":%d}`, s.Cfg.Storymaker.Render.MaxDurationSeconds*1000+1)
	for _, body := range []string{"", `{"duration_ms":999}`, tooLong} {
		w := postMusic(s, token, id, body)
		if w.Code != http.StatusBadRequest || code(w) != "invalid_argument" {
			t.Fatalf("body %q: %d %s", body, w.Code, w.Body.String())
		}
	}
	if w := postMusic(s, token, id, `{"seconds":5}`); w.Code != http.StatusBadRequest || code(w) != "invalid_request" {
		t.Fatalf("unknown field: %d %s", w.Code, w.Body.String())
	}

	// Only the Generate click asks for a generation. Any other call queues nothing, and the panel offers the button.
	w := postMusic(s, token, id, `{"duration_ms":15000}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("no click: %d %s", w.Code, w.Body.String())
	}

	// The fixture configures no music provider.
	w = postMusic(s, token, id, `{"duration_ms":15000,"generate":true}`)
	if w.Code != http.StatusConflict || code(w) != "music_unavailable" {
		t.Fatalf("not configured: %d %s", w.Code, w.Body.String())
	}

	// A stored file with its meta needs no job, whatever the provider state is.
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	if err := storymakertest.SeedMusic(context.Background(), rt.projects, id, key, 2); err != nil {
		t.Fatal(err)
	}
	w = postMusic(s, token, id, `{"duration_ms":15000,"generate":true}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("stored key: %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerMusicRouteQueuesAFileSource(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, token)
	writeMusicSource(t, s, "Documents/theme.wav", musicRouteWAV(2))
	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/theme.wav"}`)
	w := postMusic(s, token, p.Project.ID, "")
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusAccepted || got.Job.Kind != "music" || got.Job.ProjectID != p.Project.ID {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerStatusReportsMusicConfigured(t *testing.T) {
	s, readToken, _ := storymakerAPIFixture(t)
	read := func() bool {
		t.Helper()
		w := storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
		var got struct {
			MusicConfigured bool `json:"music_configured"`
		}
		decodeBody(t, w, &got)
		if w.Code != http.StatusOK {
			t.Fatalf("status: %d %s", w.Code, w.Body.String())
		}
		return got.MusicConfigured
	}
	if read() {
		t.Fatal("music_configured is true with no provider")
	}
	s.Cfg.MusicGeneration.Enabled = true
	s.Cfg.MusicGeneration.APIKey = "test-key"
	if !read() {
		t.Fatal("music_configured is false with a provider")
	}
}

func waitStorymakerMusicJob(t *testing.T, rt *storymakerRuntime, id string) render.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if job, ok := rt.narrate.Get(id); ok && job.Terminal() {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the music job %s did not end", id)
	return render.Job{}
}

// queueMusicJob skips POST /music, because these tests reach job checks that the route stops first.
func queueMusicJob(t *testing.T, rt *storymakerRuntime, id string) render.Job {
	t.Helper()
	job, _, err := rt.narrate.Enqueue(render.EnqueueRequest{ProjectID: id, Kind: render.KindMusic, Coalesce: true, Payload: storymakerMusicPayload{ProjectID: id, DurationMS: 15000}})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func musicFileFixture(t *testing.T, seconds float64) (*Server, string, string, *storymakerRuntime, string) {
	t.Helper()
	s, _, token := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, token)
	writeMusicSource(t, s, "Documents/theme.wav", musicRouteWAV(seconds))
	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/theme.wav"}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	key, err := s.storymakerMusicKey(context.Background(), &storymaker.Music{Mode: "file", File: "Documents/theme.wav"})
	if err != nil || key == "" {
		t.Fatalf("music key = %q, %v", key, err)
	}
	return s, token, p.Project.ID, rt, key
}

func TestStorymakerMusicTrackSourceRules(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	createStorymakerProject(t, s, token) // binds the store to the desktop service
	db, err := tools.InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s.MediaRegistryDB = db
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		t.Fatal(err)
	}
	audioDir := filepath.Join(s.Cfg.Directories.DataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(audioDir, "song.wav")
	outside := filepath.Join(s.Cfg.Directories.DataDir, "secret.wav")
	for _, file := range []string{inside, outside} {
		if err := os.WriteFile(file, musicRouteWAV(2), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	register := func(mediaType, file string) string {
		t.Helper()
		id, _, err := tools.RegisterMedia(db, tools.MediaItem{MediaType: mediaType, SourceTool: "test", Filename: filepath.Base(file), FilePath: file})
		if err != nil {
			t.Fatal(err)
		}
		return strconv.FormatInt(id, 10)
	}
	open := func(track string) error {
		src, _, err := s.storymakerMusicSourceOpen(context.Background(), rt, cfg, &storymaker.Music{Mode: "file", Track: track})
		if err == nil {
			_ = src.Close()
		}
		return err
	}
	if err := open(register("music", inside)); err != nil {
		t.Fatalf("a music track in the audio folder: %v", err)
	}
	var refusal *storymakerMusicRefusal
	if err := open(register("audio", inside)); !errors.As(err, &refusal) {
		t.Fatalf("another media type: %v", err)
	}
	if err := open(register("music", outside)); !errors.As(err, &refusal) {
		t.Fatalf("a path outside the audio folder: %v", err)
	}
	if err := open("999999"); !errors.Is(err, storymaker.ErrMusicSourceMissing) {
		t.Fatalf("an unknown track: %v", err)
	}
	// A plain os.Open would follow this link. Store.Open refuses it, because it leaves the Music mount.
	link := filepath.Join(audioDir, "link.wav")
	if err := os.Symlink(outside, link); err != nil {
		t.Logf("this box cannot create a symlink, so that case is not run: %v", err)
		return
	}
	if err := open(register("music", link)); err == nil {
		t.Fatal("a symlink out of the audio folder was opened, so the source did not go through Store.Open")
	}
}

func TestStorymakerMusicJobCopiesAFileAndWritesMeta(t *testing.T) {
	s, token, id, rt, key := musicFileFixture(t, 2)
	ctx := context.Background()
	w := postMusic(s, token, id, "")
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusAccepted {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
	done := waitStorymakerMusicJob(t, rt, got.Job.ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	// ProbeAudio over the stored copy: 2 s of WAV.
	meta, ext, err := rt.projects.MusicInfo(ctx, id, key)
	if err != nil || ext != "wav" || meta != (storymaker.MusicMeta{V: 1, Ext: "wav", DurationMS: 2000}) {
		t.Fatalf("music info = %+v %q %v", meta, ext, err)
	}
	stored, _, err := rt.projects.OpenAudio(ctx, id, "music", key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()
	data, err := io.ReadAll(stored)
	if err != nil || !bytes.Equal(data, musicRouteWAV(2)) {
		t.Fatalf("the stored copy differs from the source: %d bytes, %v", len(data), err)
	}
	if w := postMusic(s, token, id, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("a second call: %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerMusicStoreCountsAPathConflictAsSuccess(t *testing.T) {
	_, _, id, rt, _ := musicFileFixture(t, 2)
	ctx := context.Background()
	key := strings.Repeat("a", 64)
	first := musicRouteWAV(2)
	if err := storymakerStoreMusic(ctx, rt, id, key, "wav", bytes.NewReader(first)); err != nil {
		t.Fatal(err)
	}
	if err := storymakerStoreMusic(ctx, rt, id, key, "wav", bytes.NewReader(musicRouteWAV(3))); err != nil {
		t.Fatalf("a second copy under the same key: %v", err)
	}
	stored, _, err := rt.projects.OpenAudio(ctx, id, "music", key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()
	data, err := io.ReadAll(stored)
	if err != nil || !bytes.Equal(data, first) {
		t.Fatalf("the first copy was replaced: %d bytes, %v", len(data), err)
	}
}

func TestStorymakerMusicJobWritesMetaForAStoredCopy(t *testing.T) {
	s, token, id, rt, key := musicFileFixture(t, 2)
	ctx := context.Background()
	// The stored copy lasts 3 s and the source 2 s, so 3000 proves the job probed the copy and left it in place.
	if err := rt.projects.WriteMusic(ctx, id, key, "wav", musicRouteWAV(3)); err != nil {
		t.Fatal(err)
	}
	w := postMusic(s, token, id, "")
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusAccepted {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
	done := waitStorymakerMusicJob(t, rt, got.Job.ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	meta, _, err := rt.projects.MusicInfo(ctx, id, key)
	if err != nil || meta.V != 1 || meta.DurationMS != 3000 {
		t.Fatalf("meta = %+v, %v", meta, err)
	}
}

func TestStorymakerMusicJobRemovesACopyOfTheWrongLength(t *testing.T) {
	_, _, id, rt, key := musicFileFixture(t, 0.5)
	done := waitStorymakerMusicJob(t, rt, queueMusicJob(t, rt, id).ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	meta, ext, err := rt.projects.MusicInfo(context.Background(), id, key)
	if err != nil || meta.V != 0 || ext != "" {
		t.Fatalf("the copy of 500 ms stayed: %+v %q %v", meta, ext, err)
	}
	for ms, want := range map[int]bool{999: false, 1000: true, 3600000: true, 3600001: false} {
		if storymakerMusicDurationOK(ms) != want {
			t.Errorf("storymakerMusicDurationOK(%d) != %v", ms, want)
		}
	}
}

func TestStorymakerMusicJobStopsWhenReadOnly(t *testing.T) {
	s, _, id, rt, key := musicFileFixture(t, 2)
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	done := waitStorymakerMusicJob(t, rt, queueMusicJob(t, rt, id).ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed || !strings.Contains(done.Error.Message, "read-only") {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if _, ext, err := rt.projects.MusicInfo(context.Background(), id, key); err != nil || ext != "" {
		t.Fatalf("a read-only stop stored %q, %v", ext, err)
	}
}

type generatedMusicFixture struct {
	s      *Server
	token  string
	id     string
	key    string
	rt     *storymakerRuntime
	seam   *storymakerMusicSeam
	calls  atomic.Int32
	params tools.MusicGenParams
}

func (fx *generatedMusicFixture) audioFile(t *testing.T, name string, seconds float64) string {
	t.Helper()
	dir := filepath.Join(fx.s.Cfg.Directories.DataDir, "audio")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, musicRouteWAV(seconds), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

// newGeneratedMusicFixture has a configured provider whose stand-in returns a 20 s WAV with no DurationMs.
func newGeneratedMusicFixture(t *testing.T) *generatedMusicFixture {
	t.Helper()
	s, _, token := storymakerAPIFixture(t)
	s.Cfg.MusicGeneration.Enabled = true
	s.Cfg.MusicGeneration.APIKey = "test-key"
	p := createStorymakerProject(t, s, token)
	musicRouteStory(t, s, token, &p, `{"mode":"generated","mood":"calm-ambient"}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	fx := &generatedMusicFixture{s: s, token: token, id: p.Project.ID, key: storymaker.MusicKeyGenerated("calm-ambient", ""), rt: rt}
	file := fx.audioFile(t, "generated.wav", 20)
	fx.seam = &storymakerMusicSeam{generate: func(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
		fx.calls.Add(1)
		fx.params = params
		return tools.MusicGenResult{Status: "ok", FilePath: file, CostEstimate: 0.06}
	}}
	s.storymakerMusicOverride = fx.seam
	return fx
}

func (fx *generatedMusicFixture) post(t *testing.T, body string) render.Job {
	t.Helper()
	w := postMusic(fx.s, fx.token, fx.id, body)
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusAccepted {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
	return got.Job
}

// queue skips POST /music, as queueMusicJob does, and sets the key a Generate click would pin.
func (fx *generatedMusicFixture) queue(t *testing.T, generateKey string) render.Job {
	t.Helper()
	job, _, err := fx.rt.narrate.Enqueue(render.EnqueueRequest{ProjectID: fx.id, Kind: render.KindMusic, Coalesce: true, Payload: storymakerMusicPayload{ProjectID: fx.id, DurationMS: 15000, GenerateKey: generateKey}})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func musicTestTracker(t *testing.T, limit float64, enforcement string) *budget.Tracker {
	t.Helper()
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = limit
	cfg.Budget.Enforcement = enforcement
	cfg.Budget.WarningThreshold = 0.8
	tracker := budget.NewTracker(cfg, slog.Default(), t.TempDir())
	t.Cleanup(tracker.Flush)
	return tracker
}

func TestStorymakerMusicSeconds(t *testing.T) {
	for _, row := range []struct {
		durationMS, profileMax int
		want                   float64
	}{
		{15000, 0, 17},     // ceil(15) + 2
		{1000, 0, 10},      // 1 + 2 = 3, raised to the 10 s minimum
		{300000, 240, 240}, // 300 + 2 = 302, cut to the profile limit
		{900000, 0, 600},   // 900 + 2 = 902, cut to 600 without a profile
		{900000, 900, 600}, // a profile above 600 does not raise the limit
	} {
		if got := storymakerMusicSeconds(row.durationMS, row.profileMax); got != row.want {
			t.Errorf("storymakerMusicSeconds(%d, %d) = %v, want %v", row.durationMS, row.profileMax, got, row.want)
		}
	}
}

func TestStorymakerMusicGeneratedAsksForTheProfileLimitAndProbesTheResult(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	fx.s.Cfg.MusicGeneration.Provider = config.LocalMusicProviderID
	fx.seam.profileMax = 240
	// A 300 s film asks for 300 + 2 = 302 s, which is above the 240 s profile limit.
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":300000,"generate":true}`).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if fx.params.DurationSeconds != 240 || !fx.params.Instrumental || fx.params.Prompt != storymaker.MusicPrompt("calm-ambient", "") {
		t.Fatalf("params = %+v", fx.params)
	}
	// The result has no DurationMs, so the meta comes from ProbeAudio over the 20 s file.
	meta, ext, err := fx.rt.projects.MusicInfo(context.Background(), fx.id, fx.key)
	if err != nil || ext != "wav" || meta != (storymaker.MusicMeta{V: 1, Ext: "wav", DurationMS: 20000}) {
		t.Fatalf("music info = %+v %q %v", meta, ext, err)
	}
}

func TestStorymakerMusicGeneratedRefusedByTheBudget(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	tracker := musicTestTracker(t, 1, "full")
	tracker.RecordCostForCategory("chat", 2) // 2 USD over a limit of 1 USD: exceeded, and "full" blocks every category
	fx.s.BudgetTracker = tracker
	w := postMusic(fx.s, fx.token, fx.id, `{"duration_ms":15000,"generate":true}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"code":"budget_exceeded"`) {
		t.Fatalf("route: %d %s", w.Code, w.Body.String())
	}
	done := waitStorymakerMusicJob(t, fx.rt, fx.queue(t, fx.key).ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeBudgetExceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if fx.calls.Load() != 0 {
		t.Fatalf("the provider was called %d times", fx.calls.Load())
	}
}

func TestStorymakerMusicGeneratedNeverPaysWithoutAClickOnThisKey(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	// No click: the route queues nothing, also with a provider configured.
	w := postMusic(fx.s, fx.token, fx.id, `{"duration_ms":15000}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("no click: %d %s", w.Code, w.Body.String())
	}
	// A file job that finds a generated story when it runs has no key. A click whose mood or prompt changed while it waited
	// has an older one.
	for _, generateKey := range []string{"", strings.Repeat("a", 64)} {
		done := waitStorymakerMusicJob(t, fx.rt, fx.queue(t, generateKey).ID)
		if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed || !strings.Contains(done.Error.Message, "Click Generate") {
			t.Fatalf("key %q: job = %+v %+v", generateKey, done, done.Error)
		}
	}
	if fx.calls.Load() != 0 {
		t.Fatalf("the provider was called %d times", fx.calls.Load())
	}
}

func TestStorymakerMusicGeneratedStopsWhenReadOnly(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	started := make(chan struct{})
	fx.seam.generate = func(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
		close(started)
		<-ctx.Done()
		return tools.MusicGenResult{Status: "error", Error: ctx.Err().Error()}
	}
	job := fx.post(t, `{"duration_ms":15000,"generate":true}`)
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("the provider call did not start")
	}
	ro := *fx.s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	fx.s.replaceConfigSnapshot(&ro)
	// The check right before a provider call sees the new state.
	if je := fx.s.storymakerMusicPreflight(); je == nil || je.Code != render.CodeMusicFailed || !strings.Contains(je.Message, "read-only") {
		t.Fatalf("preflight = %+v", je)
	}
	// The call in flight is cancelled and stores nothing.
	done := waitStorymakerMusicJob(t, fx.rt, job.ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed || !strings.Contains(done.Error.Message, "read-only") {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if _, ext, err := fx.rt.projects.MusicInfo(context.Background(), fx.id, fx.key); err != nil || ext != "" {
		t.Fatalf("stored %q, %v", ext, err)
	}
}

func TestStorymakerMusicRerunReadsPendingWithoutAProviderCall(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	ctx := context.Background()
	fx.audioFile(t, "left.wav", 5)
	target, err := fx.rt.projects.AudioPath(fx.id, "music", fx.key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	pending := path.Join(path.Dir(target), fx.key+".pending.json")
	if err := fx.rt.store.WriteBytes(ctx, pending, []byte(`{"path":"left.wav"}`)); err != nil {
		t.Fatal(err)
	}
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if fx.calls.Load() != 0 {
		t.Fatalf("the provider was called %d times", fx.calls.Load())
	}
	// 5000 is the 5 s of left.wav, not the 20 s the stand-in would return.
	meta, _, err := fx.rt.projects.MusicInfo(ctx, fx.id, fx.key)
	if err != nil || meta.DurationMS != 5000 {
		t.Fatalf("meta = %+v, %v", meta, err)
	}
	if _, _, err := fx.rt.store.ReadFile(ctx, pending); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the pending file stayed: %v", err)
	}
}

func TestStorymakerMusicBadGeneratedFileDoesNotBlockTheNextGenerate(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	ctx := context.Background()
	good := fx.seam.generate
	bad := fx.audioFile(t, "bad.wav", 1)
	if err := os.WriteFile(bad, []byte("this is not audio at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	fx.seam.generate = func(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
		fx.calls.Add(1)
		return tools.MusicGenResult{Status: "ok", FilePath: bad, CostEstimate: 0.06}
	}
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusFailed || done.Error == nil || !strings.Contains(done.Error.Message, "not WAV or MP3") {
		t.Fatalf("first job = %+v %+v", done, done.Error)
	}
	target, err := fx.rt.projects.AudioPath(fx.id, "music", fx.key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := fx.rt.store.ReadFile(ctx, path.Join(path.Dir(target), fx.key+".pending.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the pending file stayed: %v", err)
	}
	fx.seam.generate = good
	done = waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("second job = %+v %+v", done, done.Error)
	}
	if fx.calls.Load() != 2 {
		t.Fatalf("the provider was called %d times, want 2", fx.calls.Load())
	}
}

func TestStorymakerMusicFailedPendingWriteDoesNotPayTwice(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	ctx := context.Background()
	target, err := fx.rt.projects.AudioPath(fx.id, "music", fx.key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	pending := path.Join(path.Dir(target), fx.key+".pending.json")
	inner := fx.seam.generate
	// A folder at the pending path makes the write after the paid call fail.
	fx.seam.generate = func(c context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
		if err := fx.rt.store.MkdirAll(c, pending); err != nil {
			t.Error(err)
		}
		return inner(c, cfg, params)
	}
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusFailed {
		t.Fatalf("first job = %+v %+v", done, done.Error)
	}
	if err := fx.rt.store.Remove(ctx, pending); err != nil {
		t.Fatal(err)
	}
	done = waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("second job = %+v %+v", done, done.Error)
	}
	if fx.calls.Load() != 1 {
		t.Fatalf("the provider was called %d times, want 1", fx.calls.Load())
	}
}

func TestStorymakerMusicGeneratedScrubsTheProviderFailure(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	const sentinel = "sm-music-sentinel-0123456789abcdef"
	defer security.RegisterScopedSensitiveExact(sentinel)()
	// The sentinel crosses the 300-rune cut, so a scrub after the clip would no longer match it and would leave its head.
	fx.seam.generate = func(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
		return tools.MusicGenResult{Status: "error", Error: strings.Repeat("x", 280) + sentinel + " echoed by the provider"}
	}
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if msg := done.Error.Message; strings.Contains(msg, sentinel[:8]) || !strings.Contains(msg, "The music provider failed: ") || !strings.Contains(msg, "[redacted]") {
		t.Fatalf("message = %q", msg)
	}
}

func TestStorymakerMusicGeneratedRecordsItsCostOnce(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	tracker := musicTestTracker(t, 100, "warn")
	fx.s.BudgetTracker = tracker
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if w := postMusic(fx.s, fx.token, fx.id, `{"duration_ms":15000,"generate":true}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("a second call: %d %s", w.Code, w.Body.String())
	}
	// The stand-in reports 0.06 USD, and the second call queued nothing.
	if got := tracker.CategorySpendUSD("music_generation"); got != 0.06 || fx.calls.Load() != 1 {
		t.Fatalf("spend = %v after %d calls", got, fx.calls.Load())
	}
}

// musicToneWAV is musicRouteWAV with a 1 kHz square wave in it, so that the meter reads a level.
func musicToneWAV(seconds float64) []byte {
	data := musicRouteWAV(seconds)
	for i := 44; i+1 < len(data); i += 2 {
		sample := int16(8192)
		if (i-44)/2%8 >= 4 {
			sample = -8192
		}
		binary.LittleEndian.PutUint16(data[i:], uint16(sample))
	}
	return data
}

// The music job measures the stored copy for the preview, and the /narration answer carries the level.
func TestStorymakerMusicJobMeasuresTheStoredCopy(t *testing.T) {
	s, readToken, token := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = storymakerTestFFmpeg(t)
	p := createStorymakerProject(t, s, token)
	writeMusicSource(t, s, "Documents/theme.wav", musicToneWAV(2))
	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/theme.wav"}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	done := waitStorymakerMusicJob(t, rt, queueMusicJob(t, rt, p.Project.ID).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	answer := musicKeyAnswer(t, s, readToken, p.Project.ID)
	if answer.Music == nil || answer.Music.LoudnessI == nil || *answer.Music.LoudnessI > -3 || *answer.Music.LoudnessI < -40 {
		t.Fatalf("music = %+v, want a measured level", answer.Music)
	}
}

// The export measures for itself, so a missing FFmpeg costs the preview its level and never the job.
func TestStorymakerMusicJobSurvivesAMissingMeter(t *testing.T) {
	s, readToken, token := storymakerAPIFixture(t)
	s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-ffmpeg")
	p := createStorymakerProject(t, s, token)
	writeMusicSource(t, s, "Documents/theme.wav", musicToneWAV(2))
	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/theme.wav"}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	done := waitStorymakerMusicJob(t, rt, queueMusicJob(t, rt, p.Project.ID).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	answer := musicKeyAnswer(t, s, readToken, p.Project.ID)
	if answer.Music == nil || answer.Music.LoudnessI != nil {
		t.Fatalf("music = %+v, want stored music without a level", answer.Music)
	}
}
