package storymaker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func validateMusic(t *testing.T, m *Music) []Issue {
	t.Helper()
	s := storyOf(t, `{"id":"s1"}`)
	s.Music = m
	return Validate(&s, loadMini(t), DefaultLimits())
}

func TestValidateMusicFields(t *testing.T) {
	cases := []struct {
		name       string
		m          *Music
		code, path string
	}{
		{"file and track", &Music{Mode: "file", File: "Documents/Music/a.mp3", Track: "5"}, CodeInvalidValue, "/music/track"},
		{"track is text", &Music{Mode: "file", Track: "abc"}, CodeInvalidValue, "/music/track"},
		{"track is zero", &Music{Mode: "file", Track: "0"}, CodeInvalidValue, "/music/track"},
		{"track has a sign", &Music{Mode: "file", Track: "+5"}, CodeInvalidValue, "/music/track"},
		{"generated without mood or prompt", &Music{Mode: "generated"}, CodeMissingField, "/music/mood"},
		{"mood outside the presets", &Music{Mode: "generated", Mood: "sad-trombone"}, CodeInvalidValue, "/music/mood"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectIssue(t, validateMusic(t, tc.m), tc.code, SeverityError, tc.path)
		})
	}
	for _, good := range []*Music{
		{Mode: "none"},
		{Mode: "file", Track: "42"},
		{Mode: "file", File: "Documents/Music/a.mp3"},
		{Mode: "generated", Mood: "lo-fi"},
		{Mode: "generated", Prompt: "soft piano"},
	} {
		if issues := validateMusic(t, good); len(issues) != 0 {
			t.Errorf("%+v:%s", good, dump(issues))
		}
	}
}

func TestMusicKeysExcludeLength(t *testing.T) {
	hash := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	// 1700000000 s plus 123 ns is the decimal UnixNano 1700000000000000123. The hash input has no length field.
	at := time.Unix(1700000000, 123)
	if got := MusicKeyGenerated("lo-fi", "soft"); got != hash("generated|lo-fi|soft") {
		t.Errorf("generated key %s", got)
	}
	if got := MusicKeyFile("Documents/a.mp3", 7, at); got != hash("file|Documents/a.mp3|7|1700000000000000123") {
		t.Errorf("file key %s", got)
	}
	if got := MusicKeyTrack("42", 7, at); got != hash("track|42|7|1700000000000000123") {
		t.Errorf("track key %s", got)
	}
}

func TestMusicPromptAndVolume(t *testing.T) {
	template := "Lo-fi background music, mellow electric piano, soft vinyl texture, relaxed slow beat"
	if got := MusicPrompt("lo-fi", ""); got != template {
		t.Errorf("mood only: %q", got)
	}
	if got := MusicPrompt("lo-fi", "with rain"); got != template+". with rain" {
		t.Errorf("mood and prompt: %q", got)
	}
	if got := MusicPrompt("", " soft piano "); got != " soft piano " {
		t.Errorf("a prompt without a mood stays as written: %q", got)
	}
	for _, mood := range MusicMoods {
		if MusicPrompt(mood, "") == "" {
			t.Errorf("mood %s has no template", mood)
		}
	}
	half := 0.5
	if MusicVolume(nil) != 0.25 || MusicVolume(&Music{}) != 0.25 || MusicVolume(&Music{Volume: &half}) != 0.5 {
		t.Error("MusicVolume")
	}
}

func TestMusicStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	mem := NewMemStore()
	ps := NewProjectStore(mem, "Documents/Storymaker")
	proj, _, _, err := ps.Create(ctx, "T", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	key := MusicKeyGenerated("lo-fi", "")

	if meta, ext, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta.V != 0 || ext != "" {
		t.Fatalf("empty: %+v %q %v", meta, ext, err)
	}
	if err := ps.WriteMusic(ctx, proj.ID, key, "mp3", []byte("ID3")); err != nil {
		t.Fatal(err)
	}
	if err := ps.WriteMusic(ctx, proj.ID, key, "mp3", []byte("ID3")); err != nil {
		t.Fatalf("a hash name that exists is not an error: %v", err)
	}
	if meta, ext, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta.V != 0 || ext != "mp3" {
		t.Fatalf("audio without meta: %+v %q %v", meta, ext, err)
	}
	want := MusicMeta{V: 1, Ext: "mp3", DurationMS: 61000}
	if err := ps.WriteMusicMeta(ctx, proj.ID, key, want); err != nil {
		t.Fatal(err)
	}
	if meta, ext, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta != want || ext != "mp3" {
		t.Fatalf("round trip: %+v %q %v", meta, ext, err)
	}

	metaPath := ps.musicFile(proj.ID, key+".meta.json")
	for _, bad := range []string{
		`{`,
		`{"v":2,"ext":"mp3","duration_ms":1000}`,
		`{"v":1,"ext":"ogg","duration_ms":1000}`,
		`{"v":1,"ext":"mp3","duration_ms":0}`,
		`{"v":1,"ext":"mp3","duration_ms":3600001}`,
		`{"v":1,"ext":"wav","duration_ms":1000}`,
	} {
		if _, err := mem.WriteFile(ctx, metaPath, []byte(bad), ""); err != nil {
			t.Fatal(err)
		}
		if meta, ext, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta.V != 0 || ext != "mp3" {
			t.Errorf("meta %s: %+v %q %v", bad, meta, ext, err)
		}
	}

	if err := ps.WriteMusic(ctx, proj.ID, "../x", "mp3", nil); err == nil {
		t.Error("a bad key was written")
	}
	if err := ps.WriteMusic(ctx, proj.ID, key, "ogg", nil); err == nil {
		t.Error("a bad extension was written")
	}
	if err := ps.WriteMusicMeta(ctx, proj.ID, key, MusicMeta{V: 1, Ext: "mp3"}); err == nil {
		t.Error("a meta without a duration was written")
	}
}

func TestStyleAutoSFXIsAccepted(t *testing.T) {
	doc := func(style string) string {
		return `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},` +
			`"style":{"palette":"sunny","font":"default","captions":true` + style + `},"cast":[],"scenes":[{"id":"s1"}]}`
	}
	s := mustDecode(t, doc(`,"auto_sfx":true`))
	if !s.Style.AutoSFX {
		t.Fatal("auto_sfx was not read")
	}
	if issues := Validate(&s, loadMini(t), DefaultLimits()); len(issues) != 0 {
		t.Fatalf("issues:%s", dump(issues))
	}
	data, err := CanonicalJSON(s)
	if err != nil || !strings.Contains(string(data), `"auto_sfx": true`) {
		t.Fatalf("canonical with auto_sfx: %s %v", data, err)
	}
	s.Style.AutoSFX = false
	data, err = CanonicalJSON(s)
	if err != nil || strings.Contains(string(data), "auto_sfx") {
		t.Fatalf("Go never writes the default: %s %v", data, err)
	}
	_, issues, err := DecodeStory([]byte(doc(`,"sfx":true`)), DefaultLimits())
	if err == nil || len(issues) != 1 || issues[0].Code != CodeUnknownField || !strings.Contains(issues[0].Fix, "auto_sfx") {
		t.Fatalf("hint: %+v %v", issues, err)
	}
}

func TestValidateAcceptsSFXBeat(t *testing.T) {
	if issues := validateScenes(t, `{"id":"s1","beats":[{"do":"sfx","sound":"sfx.pop"}]}`); len(issues) != 0 {
		t.Fatalf("an sfx beat is played now:%s", dump(issues))
	}
}

func TestMusicMetaKeepsTheMeasuredLevel(t *testing.T) {
	ctx := context.Background()
	mem := NewMemStore()
	ps := NewProjectStore(mem, "Documents/Storymaker")
	proj, _, _, err := ps.Create(ctx, "T", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	key := MusicKeyGenerated("lo-fi", "")
	if err := ps.WriteMusic(ctx, proj.ID, key, "mp3", []byte("ID3")); err != nil {
		t.Fatal(err)
	}
	level := -14.25
	if err := ps.WriteMusicMeta(ctx, proj.ID, key, MusicMeta{V: 1, Ext: "mp3", DurationMS: 61000, LoudnessI: &level}); err != nil {
		t.Fatal(err)
	}
	resolve := func(context.Context, *Music) (string, error) { return key, nil }
	answer, err := ps.AudioAnswer(ctx, proj.ID, &Story{}, resolve)
	if err != nil || answer.Music == nil || answer.Music.LoudnessI == nil || *answer.Music.LoudnessI != level {
		t.Fatalf("answer = %+v, %v", answer.Music, err)
	}
	if _, err := mem.WriteFile(ctx, ps.musicFile(proj.ID, key+".meta.json"), []byte(`{"v":1,"ext":"mp3","duration_ms":1000,"loudness_i":999}`), ""); err != nil {
		t.Fatal(err)
	}
	if meta, _, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta.V != 0 {
		t.Fatalf("a level of 999: %+v, %v", meta, err)
	}
}
