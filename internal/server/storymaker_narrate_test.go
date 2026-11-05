package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"
)

// Two sentences that stay separate pieces: widths 32 and 36 are at least 20 and their 6 words are
// over 3 (SplitPieces rule 3), and each fits the chunk limit of 2000 of sanotts.
const (
	narrateJobFirst  = "Der erste Satz steht ganz vorne."
	narrateJobSecond = "Der zweite Satz folgt gleich danach."
	narrateJobSano   = `{"provider":"sanotts","voice":"","language":"de"}`
)

func narrateJobStory(voice, narration string) string {
	return `{"schema":"storymaker/1","title":"Stimme","voice":` + voice + `,"scenes":[{"id":"s1","narration":"` + narration + `"}]}`
}

// narrateJobWAV is a valid piece: 1 s of 24 kHz mono s16 with a 440 Hz tone at amplitude 0.1.
func narrateJobWAV() []byte {
	const rate = 24000
	samples := make([]int16, rate)
	for i := range samples {
		samples[i] = int16(3276 * math.Sin(2*math.Pi*440*float64(i)/rate))
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+2*len(samples)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(2 * rate), uint16(2), uint16(16)} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(2*len(samples)))
	_ = binary.Write(&b, binary.LittleEndian, samples)
	return b.Bytes()
}

// narrateJobRecorder is a synth stub that keeps the texts it was asked for and answers a valid piece.
func narrateJobRecorder(calls *[]string) func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
	return func(_ context.Context, _ tools.TTSConfig, text string) ([]byte, string, error) {
		*calls = append(*calls, text)
		return narrateJobWAV(), "wav", nil
	}
}

func narrateJobSetConfig(s *Server, edit func(*config.Config)) {
	s.CfgMu.Lock()
	defer s.CfgMu.Unlock()
	next := *s.ConfigSnapshot()
	edit(&next)
	s.replaceConfigSnapshot(&next)
}

type narrateJobEnv struct {
	s     *Server
	rt    *storymakerRuntime
	token string
	id    string
	etag  string
}

func newNarrateJobEnv(t *testing.T) *narrateJobEnv {
	t.Helper()
	s, _, token := storymakerAPIFixture(t)
	// Plenty of room, so a nearly full test disk cannot change a result.
	s.storymakerFreeDiskOverride = func(string) (int64, error) { return 1 << 40, nil }
	p := createStorymakerProject(t, s, token)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	return &narrateJobEnv{s: s, rt: rt, token: token, id: p.Project.ID, etag: p.ETag}
}

// save stores a story. It returns an error instead of failing, because stubs call it on the job goroutine.
func (e *narrateJobEnv) save(story string) error {
	w := storymakerRequest(e.s, e.token, http.MethodPut, "/projects/"+e.id+"/story", strings.NewReader(story), map[string]string{"If-Match": e.etag})
	if w.Code != http.StatusOK {
		return fmt.Errorf("save = %d %s", w.Code, w.Body.String())
	}
	var saved struct {
		ETag string `json:"etag"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		return err
	}
	e.etag = saved.ETag
	return nil
}

func (e *narrateJobEnv) mustSave(t *testing.T, story string) {
	t.Helper()
	if err := e.save(story); err != nil {
		t.Fatal(err)
	}
}

// run enqueues a narrate job and waits for its end.
func (e *narrateJobEnv) run(t *testing.T) render.Job {
	t.Helper()
	job, _, err := e.rt.narrate.Enqueue(render.EnqueueRequest{ProjectID: e.id, Kind: render.KindNarrate, Coalesce: true, Payload: storymakerNarratePayload{ProjectID: e.id}})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if done, ok := e.rt.narrate.Get(job.ID); ok && done.Terminal() {
			return done
		}
		time.Sleep(5 * time.Millisecond)
	}
	last, _ := e.rt.narrate.Get(job.ID)
	t.Fatalf("the narrate job did not end: %+v", last)
	return render.Job{}
}

func (e *narrateJobEnv) missing(t *testing.T) []storymaker.MissingBlock {
	t.Helper()
	ctx := context.Background()
	data, _, err := e.rt.projects.LoadStory(ctx, e.id)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := storymaker.DecodeStory(data, storymaker.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	answer, err := e.rt.projects.AudioAnswer(ctx, e.id, &story, nil)
	if err != nil {
		t.Fatal(err)
	}
	return answer.Missing
}

func requireNarrateFailure(t *testing.T, job render.Job, code string) {
	t.Helper()
	if job.Status != render.StatusFailed || job.Error == nil || job.Error.Code != code {
		t.Fatalf("job = %+v, want failed with %s", job, code)
	}
}

func TestStorymakerTTSConfigFollowsTheVoice(t *testing.T) {
	cfg := &config.Config{}
	cfg.TTS.Provider = "google"
	cfg.TTS.Language = "en"
	for _, tc := range []struct {
		provider string
		get      func(tools.TTSConfig) string
	}{
		{"phonon", func(c tools.TTSConfig) string { return c.Phonon.VoiceID }},
		{"piper", func(c tools.TTSConfig) string { return c.Piper.Voice }},
		{"supertonic", func(c tools.TTSConfig) string { return c.Supertonic.Voice }},
		{"xai", func(c tools.TTSConfig) string { return c.XAI.VoiceID }},
		{"openai", func(c tools.TTSConfig) string { return c.OpenAI.VoiceID }},
		{"stepfun", func(c tools.TTSConfig) string { return c.StepFun.VoiceID }},
		{"elevenlabs", func(c tools.TTSConfig) string { return c.ElevenLabs.VoiceID }},
		{"minimax", func(c tools.TTSConfig) string { return c.MiniMax.VoiceID }},
		{"mistral", func(c tools.TTSConfig) string { return c.Mistral.VoiceID }},
		{"resemble", func(c tools.TTSConfig) string { return c.Resemble.VoiceID }},
	} {
		got := storymakerTTSConfig(cfg, storymaker.Voice{Provider: tc.provider, Voice: "voice-1", Language: "de"})
		if got.Provider != tc.provider || got.Language != "de" || !got.NoProviderFallback || tc.get(got) != "voice-1" {
			t.Errorf("%s: %+v", tc.provider, got)
		}
	}
	if got := storymakerTTSConfig(cfg, storymaker.Voice{Provider: "supertonic", Language: "de"}); got.Supertonic.ResponseFormat != "wav" {
		t.Errorf("supertonic format = %q", got.Supertonic.ResponseFormat)
	}
	// BuildTTSConfig answers cfg.TTS.Language ("en") for sanotts (tts_config.go:62-68), so "de" proves the second set.
	if got := storymakerTTSConfig(cfg, storymaker.Voice{Provider: "sanotts", Language: "de"}); got.Language != "de" {
		t.Errorf("sanotts language = %q", got.Language)
	}
	if cfg.TTS.Provider != "google" {
		t.Errorf("the caller's config changed: %q", cfg.TTS.Provider)
	}
}

func TestStorymakerNarrateSkipsASentenceDeletedDuringTheRun(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst+" "+narrateJobSecond))
	var calls []string
	env.s.storymakerSynthOverride = func(_ context.Context, _ tools.TTSConfig, text string) ([]byte, string, error) {
		calls = append(calls, text)
		if err := env.save(narrateJobStory(narrateJobSano, narrateJobFirst)); err != nil {
			t.Error(err)
		}
		return narrateJobWAV(), "wav", nil
	}
	job := env.run(t)
	if job.Status != render.StatusSucceeded || !slices.Equal(calls, []string{narrateJobFirst}) {
		t.Fatalf("job = %+v, calls = %q", job, calls)
	}
}

func TestStorymakerNarrateReadOnlyStopsABlockedProviderCall(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst))
	entered := make(chan struct{})
	env.s.storymakerSynthOverride = func(ctx context.Context, _ tools.TTSConfig, _ string) ([]byte, string, error) {
		close(entered)
		<-ctx.Done()
		return nil, "", ctx.Err()
	}
	go func() {
		<-entered
		narrateJobSetConfig(env.s, func(c *config.Config) { c.VirtualDesktop.ReadOnly = true })
	}()
	job := env.run(t)
	requireNarrateFailure(t, job, render.CodeNarrationUnavailable)
	if want := "The desktop became read-only or its configuration changed, so the narration stopped."; job.Error.Message != want {
		t.Errorf("message = %q, want %q", job.Error.Message, want)
	}
	files, err := env.rt.projects.PieceFiles(context.Background(), env.id)
	if err != nil || len(files) != 0 {
		t.Fatalf("stored files = %v, %v", files, err)
	}
}

func TestStorymakerNarrateStopsWhenStorymakerIsSwitchedOff(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst+" "+narrateJobSecond))
	calls := 0
	env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		calls++
		narrateJobSetConfig(env.s, func(c *config.Config) { c.Storymaker.Enabled = false })
		return narrateJobWAV(), "wav", nil
	}
	requireNarrateFailure(t, env.run(t), render.CodeDisabled)
	if calls != 1 {
		t.Fatalf("%d provider calls, want 1", calls)
	}
}

func TestStorymakerNarrateVoiceSetToNullMidRun(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst+" "+narrateJobSecond))
	calls := 0
	env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		calls++
		if err := env.save(narrateJobStory("null", narrateJobFirst+" "+narrateJobSecond)); err != nil {
			t.Error(err)
		}
		return narrateJobWAV(), "wav", nil
	}
	requireNarrateFailure(t, env.run(t), render.CodeVoiceRequired)
	if calls != 1 {
		t.Fatalf("%d provider calls, want 1", calls)
	}
}

func TestStorymakerNarrateUndecodableAudioNamesTheProvider(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst))
	env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		return []byte("this is not audio"), "wav", nil
	}
	job := env.run(t)
	requireNarrateFailure(t, job, render.CodeNarrationFailed)
	if !strings.Contains(job.Error.Message, "sanotts") {
		t.Errorf("message = %q", job.Error.Message)
	}
	if files, err := env.rt.projects.PieceFiles(context.Background(), env.id); err != nil || len(files) != 0 {
		t.Fatalf("stored files = %v, %v", files, err)
	}
}

func TestStorymakerNarrateRefusesAnOversizedProviderAnswer(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst))
	oversized := make([]byte, storymakerMaxPieceBytes+1)
	env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		return oversized, "wav", nil
	}
	job := env.run(t)
	requireNarrateFailure(t, job, render.CodeNarrationFailed)
	// These zero bytes would fail DecodeAudio with "could not be decoded", so this text shows it never ran.
	if !strings.Contains(job.Error.Message, "256 MiB") {
		t.Errorf("message = %q", job.Error.Message)
	}
	if files, err := env.rt.projects.PieceFiles(context.Background(), env.id); err != nil || len(files) != 0 {
		t.Fatalf("stored files = %v, %v", files, err)
	}
	// The refusal keeps the bytes: the provider answered, so it has billed.
	data, err := env.s.storymakerSynth(context.Background(), env.s.ConfigSnapshot(), storymaker.Voice{Provider: "sanotts", Language: "de"}, narrateJobFirst)
	if !errors.Is(err, errStorymakerPieceTooLarge) || len(data) == 0 {
		t.Fatalf("direct call = %d bytes, %v", len(data), err)
	}
}

func TestStorymakerNarrateLowDiskNamesTheFreeSize(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst))
	workspace := env.s.ConfigSnapshot().VirtualDesktop.WorkspaceDir
	var probed string
	env.s.storymakerFreeDiskOverride = func(path string) (int64, error) {
		probed = path
		return 12 << 20, nil
	}
	var calls []string
	env.s.storymakerSynthOverride = narrateJobRecorder(&calls)
	job := env.run(t)
	requireNarrateFailure(t, job, render.CodeNarrationFailed)
	if !strings.Contains(job.Error.Message, "12 MiB") || len(calls) != 0 || probed != workspace {
		t.Fatalf("message = %q, calls = %q, probed = %q", job.Error.Message, calls, probed)
	}
}

func TestStorymakerNarrateReplacesAStoredPieceThatNoLongerDecodes(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst))
	ctx := context.Background()
	hash := storymaker.PieceKey(storymaker.Voice{Provider: "sanotts", Language: "de"}, narrateJobFirst)
	path, err := env.rt.projects.AudioPath(env.id, "narration", hash+".wav")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.rt.store.WriteBytes(ctx, path, []byte("damaged")); err != nil {
		t.Fatal(err)
	}
	var calls []string
	env.s.storymakerSynthOverride = narrateJobRecorder(&calls)
	job := env.run(t)
	if job.Status != render.StatusSucceeded || !slices.Equal(calls, []string{narrateJobFirst}) {
		t.Fatalf("job = %+v, calls = %q", job, calls)
	}
	rc, _, err := env.rt.projects.OpenAudio(ctx, env.id, "narration", hash+".wav")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := storymaker.DecodeAudio(data); err != nil {
		t.Fatalf("the stored piece still does not decode: %v", err)
	}
}

func TestStorymakerNarrateRerunSkipsStoredPieces(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobSano, narrateJobFirst+" "+narrateJobSecond))
	env.s.storymakerSynthOverride = func(_ context.Context, _ tools.TTSConfig, text string) ([]byte, string, error) {
		if text == narrateJobSecond {
			return nil, "", errors.New("provider down")
		}
		return narrateJobWAV(), "wav", nil
	}
	first := env.run(t)
	requireNarrateFailure(t, first, render.CodeNarrationFailed)
	if !strings.Contains(first.Error.Message, "sanotts") {
		t.Errorf("message = %q", first.Error.Message)
	}

	// Without its envelope the stored piece counts as missing, and the rerun rebuilds it with no provider call.
	ctx := context.Background()
	hash := storymaker.PieceKey(storymaker.Voice{Provider: "sanotts", Language: "de"}, narrateJobFirst)
	audioPath, err := env.rt.projects.AudioPath(env.id, "narration", hash+".wav")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.rt.store.Remove(ctx, strings.TrimSuffix(audioPath, ".wav")+".env.json"); err != nil {
		t.Fatal(err)
	}

	var calls []string
	env.s.storymakerSynthOverride = narrateJobRecorder(&calls)
	second := env.run(t)
	if second.Status != render.StatusSucceeded || !slices.Equal(calls, []string{narrateJobSecond}) {
		t.Fatalf("rerun = %+v, calls = %q", second, calls)
	}
	// Two pieces, both stored at the end.
	if second.PiecesDone != 2 || second.PiecesTotal != 2 {
		t.Fatalf("pieces = %d of %d", second.PiecesDone, second.PiecesTotal)
	}
	if missing := env.missing(t); len(missing) != 0 {
		t.Fatalf("still missing: %+v", missing)
	}
}

const narrateJobOpenAI = `{"provider":"openai","voice":"alloy","language":"de"}`

// A fixed noon, so a test run across midnight cannot change the date.
var narrateUsageDay = time.Date(2026, 11, 4, 12, 0, 0, 0, time.Local)

func newNarrateUsageEnv(t *testing.T) *narrateJobEnv {
	t.Helper()
	env := newNarrateJobEnv(t)
	env.rt.usage.now = func() time.Time { return narrateUsageDay }
	return env
}

// narrateUsageChars reads the count on disk, -1 when the file is unusable. Stubs call it on the job goroutine.
func narrateUsageChars(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	var cur storymakerUsageFile
	if json.Unmarshal(data, &cur) != nil {
		return -1
	}
	return cur.Chars
}

func TestStorymakerNarrateCloudCapsRefuseBeforeTheCall(t *testing.T) {
	// 135 code points (26 x 5 + 5) in one sentence: over the job cap of 100, under the chunk limit 400 of openai.
	long := strings.Repeat("wort ", 26) + "ende."
	t.Run("job cap", func(t *testing.T) {
		env := newNarrateUsageEnv(t)
		narrateJobSetConfig(env.s, func(c *config.Config) { c.Storymaker.Narration.MaxJobChars = 100 })
		env.mustSave(t, narrateJobStory(narrateJobOpenAI, long))
		var calls []string
		env.s.storymakerSynthOverride = narrateJobRecorder(&calls)
		requireNarrateFailure(t, env.run(t), render.CodeBudgetExceeded)
		if len(calls) != 0 {
			t.Fatalf("provider calls = %q", calls)
		}
		if _, err := os.Stat(env.rt.usage.path); !os.IsNotExist(err) {
			t.Fatalf("a refused piece touched usage.json: %v", err)
		}
	})
	t.Run("daily cap", func(t *testing.T) {
		env := newNarrateUsageEnv(t)
		narrateJobSetConfig(env.s, func(c *config.Config) { c.Storymaker.Narration.MaxDailyChars = 1000 })
		env.mustSave(t, narrateJobStory(narrateJobOpenAI, narrateJobFirst))
		// 995 counted: 995 + 32 code points of the sentence is over the cap of 1000.
		if err := os.WriteFile(env.rt.usage.path, []byte(`{"v":1,"date":"2026-11-04","chars":995}`), 0o600); err != nil {
			t.Fatal(err)
		}
		var calls []string
		env.s.storymakerSynthOverride = narrateJobRecorder(&calls)
		requireNarrateFailure(t, env.run(t), render.CodeBudgetExceeded)
		if len(calls) != 0 || narrateUsageChars(env.rt.usage.path) != 995 {
			t.Fatalf("calls = %q, chars = %d", calls, narrateUsageChars(env.rt.usage.path))
		}
	})
}

func TestStorymakerNarrateLocalVoiceIsExemptInAnyCase(t *testing.T) {
	env := newNarrateUsageEnv(t)
	narrateJobSetConfig(env.s, func(c *config.Config) {
		c.Storymaker.Narration.MaxJobChars = 100
		c.Storymaker.Narration.MaxDailyChars = 1000
	})
	// 135 code points, over the job cap, on a provider spelled with a capital.
	long := strings.Repeat("wort ", 26) + "ende."
	env.mustSave(t, narrateJobStory(`{"provider":"Phonon","voice":"","language":"de"}`, long))
	var calls []string
	env.s.storymakerSynthOverride = narrateJobRecorder(&calls)
	job := env.run(t)
	if job.Status != render.StatusSucceeded || !slices.Equal(calls, []string{long}) {
		t.Fatalf("job = %+v, calls = %q", job, calls)
	}
	if _, err := os.Stat(env.rt.usage.path); !os.IsNotExist(err) {
		t.Fatalf("a local voice touched usage.json: %v", err)
	}
}

func TestStorymakerNarrateUsageIsWrittenBeforeTheCallAndKeptOnceAnswered(t *testing.T) {
	env := newNarrateUsageEnv(t)
	env.mustSave(t, narrateJobStory(narrateJobOpenAI, narrateJobFirst))
	path := env.rt.usage.path
	// The one piece has 32 code points.
	const charge = 32

	during := -1
	env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		during = narrateUsageChars(path)
		return nil, "", errors.New("provider down")
	}
	requireNarrateFailure(t, env.run(t), render.CodeNarrationFailed)
	if during != charge || narrateUsageChars(path) != 0 {
		t.Fatalf("during the call %d, after it %d, want %d and 0", during, narrateUsageChars(path), charge)
	}

	// The provider answered, so it has billed: bytes that do not decode keep the reservation.
	env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		return []byte("this is not audio"), "mp3", nil
	}
	requireNarrateFailure(t, env.run(t), render.CodeNarrationFailed)
	if got := narrateUsageChars(path); got != charge {
		t.Fatalf("after undecodable audio %d, want %d", got, charge)
	}

	// An error that still delivered bytes keeps it too.
	env.s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		return []byte("partial"), "mp3", errors.New("cut off")
	}
	requireNarrateFailure(t, env.run(t), render.CodeNarrationFailed)
	if got := narrateUsageChars(path); got != 2*charge {
		t.Fatalf("after an error with bytes %d, want %d", got, 2*charge)
	}
}

func TestStorymakerNarrateJobCapCountsCharactersAlreadySent(t *testing.T) {
	env := newNarrateUsageEnv(t)
	narrateJobSetConfig(env.s, func(c *config.Config) { c.Storymaker.Narration.MaxJobChars = 100 })
	// Two sentences of 65 code points (12 x 5 + 5): each fits the cap of 100, together they do not.
	one := strings.Repeat("wort ", 12) + "ende."
	two := strings.Repeat("Satz ", 12) + "ende."
	env.mustSave(t, narrateJobStory(narrateJobOpenAI, one+" "+two))
	var calls []string
	env.s.storymakerSynthOverride = narrateJobRecorder(&calls)
	requireNarrateFailure(t, env.run(t), render.CodeBudgetExceeded)
	if !slices.Equal(calls, []string{one}) || narrateUsageChars(env.rt.usage.path) != 65 {
		t.Fatalf("calls = %q, chars = %d", calls, narrateUsageChars(env.rt.usage.path))
	}
}

func TestStorymakerNarrateUnreadableUsageStopsCloudCalls(t *testing.T) {
	env := newNarrateUsageEnv(t)
	narrateJobSetConfig(env.s, func(c *config.Config) { c.Storymaker.Narration.MaxDailyChars = 1000 })
	env.mustSave(t, narrateJobStory(narrateJobOpenAI, narrateJobFirst))
	path := env.rt.usage.path
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls []string
	env.s.storymakerSynthOverride = narrateJobRecorder(&calls)
	job := env.run(t)
	requireNarrateFailure(t, job, render.CodeBudgetExceeded)
	if !strings.Contains(job.Error.Message, "usage.bad.json") || len(calls) != 0 {
		t.Fatalf("message = %q, calls = %q", job.Error.Message, calls)
	}
	bad, err := os.ReadFile(strings.TrimSuffix(path, "usage.json") + "usage.bad.json")
	if err != nil || string(bad) != "not json" {
		t.Fatalf("usage.bad.json = %q, %v", bad, err)
	}
	// The replacement holds the whole cap, so the guard does not fail open.
	if got := narrateUsageChars(path); got != 1000 {
		t.Fatalf("usage.json holds %d, want 1000", got)
	}
}

func TestStorymakerUsageResetsOnANewLocalDate(t *testing.T) {
	u := newStorymakerNarrationUsage(t.TempDir())
	day := time.Date(2026, 11, 4, 23, 59, 0, 0, time.Local)
	u.now = func() time.Time { return day }
	if err := u.reserve(900, 1000); err != nil {
		t.Fatal(err)
	}
	// 900 + 200 is over the cap of 1000.
	var refused *render.JobError
	if err := u.reserve(200, 1000); !errors.As(err, &refused) || refused.Code != render.CodeBudgetExceeded {
		t.Fatalf("second reserve = %v", err)
	}
	day = day.Add(2 * time.Minute) // 00:01 on the next local date
	if err := u.reserve(200, 1000); err != nil {
		t.Fatalf("reserve on the new date = %v", err)
	}
	used, err := u.today(1000)
	if err != nil || used != 200 {
		t.Fatalf("today = %d, %v", used, err)
	}
	data, err := os.ReadFile(u.path)
	if err != nil {
		t.Fatal(err)
	}
	var cur storymakerUsageFile
	if err := json.Unmarshal(data, &cur); err != nil {
		t.Fatal(err)
	}
	if cur.V != 1 || cur.Date != "2026-11-05" || cur.Chars != 200 {
		t.Fatalf("usage.json = %+v", cur)
	}
}
