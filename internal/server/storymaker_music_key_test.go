package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/storymaker/storymakertest"
	"aurago/internal/tools"
)

func musicKeyPutStory(t *testing.T, s *Server, token, id, etag, body string) string {
	t.Helper()
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+id+"/story", strings.NewReader(body), map[string]string{"If-Match": etag})
	if w.Code != http.StatusOK {
		t.Fatalf("save story: %d %s", w.Code, w.Body.String())
	}
	var saved struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &saved)
	return saved.ETag
}

func musicKeyAnswer(t *testing.T, s *Server, token, id string) storymaker.AudioAnswer {
	t.Helper()
	w := storymakerRequest(s, token, http.MethodGet, "/projects/"+id+"/narration", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("narration: %d %s", w.Code, w.Body.String())
	}
	var answer storymaker.AudioAnswer
	decodeBody(t, w, &answer)
	return answer
}

func TestStorymakerMusicKeyRule(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	key := func(m *storymaker.Music) string {
		t.Helper()
		k, err := s.storymakerMusicKey(context.Background(), m)
		if err != nil {
			t.Fatalf("key of %+v: %v", m, err)
		}
		return k
	}
	calm := key(&storymaker.Music{Mode: "generated", Mood: "calm-ambient"})
	lofi := key(&storymaker.Music{Mode: "generated", Mood: "lo-fi"})
	if len(calm) != 64 || calm == lofi {
		t.Fatalf("keys: calm %q, lo-fi %q", calm, lofi)
	}
	zero := 0.0
	if got := key(&storymaker.Music{Mode: "generated", Mood: "calm-ambient", Volume: &zero}); got != "" {
		t.Errorf("volume 0 gave key %q", got)
	}
	if got := key(&storymaker.Music{Mode: "none"}); got != "" {
		t.Errorf("mode none gave key %q", got)
	}
	if got := key(nil); got != "" {
		t.Errorf("no music gave key %q", got)
	}
	if got := key(&storymaker.Music{Mode: "file"}); got != "" {
		t.Errorf("mode file without a source gave key %q", got)
	}
}

func TestStorymakerMusicKeyFollowsATrack(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	ctx := context.Background()
	db, err := tools.InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s.MediaRegistryDB = db
	audioDir := filepath.Join(s.Cfg.Directories.DataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o700); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(audioDir, "song.wav")
	outside := filepath.Join(t.TempDir(), "song.wav")
	for _, p := range []string{inside, outside} {
		if err := os.WriteFile(p, []byte("one"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	register := func(mediaType, path string) string {
		t.Helper()
		id, _, err := tools.RegisterMedia(db, tools.MediaItem{MediaType: mediaType, SourceTool: "manual", Filename: filepath.Base(path), FilePath: path, Format: "wav"})
		if err != nil {
			t.Fatal(err)
		}
		return strconv.FormatInt(id, 10)
	}

	good := register("music", inside)
	first, err := s.storymakerMusicKey(ctx, &storymaker.Music{Mode: "file", Track: good})
	if err != nil || len(first) != 64 {
		t.Fatalf("track key = %q, %v", first, err)
	}
	// The size is part of the key.
	if err := os.WriteFile(inside, []byte("a longer file"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := s.storymakerMusicKey(ctx, &storymaker.Music{Mode: "file", Track: good})
	if err != nil || second == first {
		t.Fatalf("key after the file changed = %q, %v", second, err)
	}

	for _, bad := range []string{register("music", outside), register("image", inside), "999"} {
		key, err := s.storymakerMusicKey(ctx, &storymaker.Music{Mode: "file", Track: bad})
		if key != "" || !errors.Is(err, storymaker.ErrMusicSourceMissing) {
			t.Errorf("track %s: key %q, err %v", bad, key, err)
		}
	}
}

func TestStorymakerNarrationMusicAnswerReadsTheMeta(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	id := p.Project.ID
	ctx := context.Background()
	const story = `{"schema":"storymaker/1","title":"Musik","music":{"mode":"generated","mood":"calm-ambient"},"scenes":[{"id":"s1","duration":%SECONDS%}]}`
	etag := musicKeyPutStory(t, s, writeToken, id, p.ETag, strings.ReplaceAll(story, "%SECONDS%", "3"))
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	base := render.ProjectsRoot + "/" + id + "/audio/music/" + key

	if answer := musicKeyAnswer(t, s, readToken, id); answer.Music != nil {
		t.Fatalf("nothing stored: music = %+v", answer.Music)
	}
	// The audio bytes are not audio, so a decode would fail. Only the meta can supply the duration.
	if err := rt.projects.WriteMusic(ctx, id, key, "wav", []byte("this is not audio")); err != nil {
		t.Fatal(err)
	}
	if answer := musicKeyAnswer(t, s, readToken, id); answer.Music != nil {
		t.Fatalf("a file without meta: music = %+v", answer.Music)
	}
	if _, err := rt.store.WriteFile(ctx, base+".meta.json", []byte(`{"v":9}`), ""); err != nil {
		t.Fatal(err)
	}
	if answer := musicKeyAnswer(t, s, readToken, id); answer.Music != nil {
		t.Fatalf("an invalid meta: music = %+v", answer.Music)
	}
	if err := rt.projects.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: "wav", DurationMS: 61000}); err != nil {
		t.Fatal(err)
	}
	answer := musicKeyAnswer(t, s, readToken, id)
	if answer.Music == nil || answer.Music.Key != key || answer.Music.Ext != "wav" || answer.Music.DurationMS != 61000 {
		t.Fatalf("music = %+v, want the meta's values", answer.Music)
	}
	// A longer film keeps the key.
	musicKeyPutStory(t, s, writeToken, id, etag, strings.ReplaceAll(story, "%SECONDS%", "9"))
	if after := musicKeyAnswer(t, s, readToken, id); after.Music == nil || after.Music.Key != key {
		t.Fatalf("after a length change: music = %+v, want key %s", after.Music, key)
	}
	// A meta without its file counts as missing too.
	if err := rt.store.Remove(ctx, base+".wav"); err != nil {
		t.Fatal(err)
	}
	if answer := musicKeyAnswer(t, s, readToken, id); answer.Music != nil {
		t.Fatalf("a meta without a file: music = %+v", answer.Music)
	}
}

func TestStorymakerNarrationReportsAMovedMusicSource(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	id := p.Project.ID
	ctx := context.Background()
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	const source = "Documents/Music/theme.wav"
	if _, err := rt.store.WriteFile(ctx, source, []byte("source bytes"), ""); err != nil {
		t.Fatal(err)
	}
	body := `{"schema":"storymaker/1","title":"Musik","voice":{"provider":"phonon","voice":"thorsten","language":"de"},` +
		`"music":{"mode":"file","file":"` + source + `"},"scenes":[{"id":"s1","narration":"Das ist ein ganzer Satz zur Firewall."}]}`
	musicKeyPutStory(t, s, writeToken, id, p.ETag, body)
	var story storymaker.Story
	if err := json.Unmarshal([]byte(body), &story); err != nil {
		t.Fatal(err)
	}
	if err := storymakertest.SeedNarration(ctx, rt.projects, id, &story, 1); err != nil {
		t.Fatal(err)
	}
	answer := musicKeyAnswer(t, s, readToken, id)
	if answer.MusicIssue != "" || answer.Music != nil || len(answer.Missing) != 0 {
		t.Fatalf("source in place: %+v", answer)
	}
	if err := rt.store.Remove(ctx, source); err != nil {
		t.Fatal(err)
	}
	answer = musicKeyAnswer(t, s, readToken, id)
	if answer.MusicIssue != "source_missing" || answer.Music != nil {
		t.Fatalf("source moved: issue %q, music %+v", answer.MusicIssue, answer.Music)
	}
	if len(answer.Narration["s1"].Blocks) != 1 || len(answer.Missing) != 0 {
		t.Fatalf("the narration was lost: %+v, missing %+v", answer.Narration, answer.Missing)
	}
}

// Choosing the mode "File or track" saves a story with no source yet. That is an unfinished choice, not a source that is
// gone: the save reports it, and the audio answer stays quiet.
func TestStorymakerNarrationHasNoIssueBeforeASourceIsChosen(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	const body = `{"schema":"storymaker/1","title":"Musik","music":{"mode":"file"},"scenes":[{"id":"s1"}]}`
	w := storymakerRequest(s, writeToken, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(body), map[string]string{"If-Match": p.ETag})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/music/file") {
		t.Fatalf("save story: %d %s", w.Code, w.Body.String())
	}
	answer := musicKeyAnswer(t, s, readToken, p.Project.ID)
	if answer.Music != nil || answer.MusicIssue != "" {
		t.Fatalf("no source chosen: music %+v, issue %q", answer.Music, answer.MusicIssue)
	}
}

func TestStorymakerRenderPlanHoldsTheMusicRef(t *testing.T) {
	plans := make(chan *render.Plan, 1)
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		plan, _ := payload.(*render.Plan)
		plans <- plan
		return render.Output{}, nil
	})
	id := p.Project.ID
	etag := musicKeyPutStory(t, s, token, id, p.ETag, `{"schema":"storymaker/1","title":"Musik","music":{"mode":"generated","mood":"calm-ambient"},"scenes":[{"id":"s1"}]}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	if err := storymakertest.SeedMusic(context.Background(), rt.projects, id, key, 2); err != nil {
		t.Fatal(err)
	}
	w := storymakerRequest(s, token, http.MethodPost, "/projects/"+id+"/render", nil, map[string]string{"If-Match": etag})
	if w.Code != http.StatusAccepted {
		t.Fatalf("render = %d %s", w.Code, w.Body.String())
	}
	select {
	case plan := <-plans:
		if plan == nil || plan.Music == nil || plan.Music.Key != key || plan.Music.Ext != "wav" || plan.Music.DurationMS != 2000 {
			t.Fatalf("plan = %+v", plan)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not start")
	}
}
