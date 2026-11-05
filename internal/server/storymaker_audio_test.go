package server

import (
	"context"
	"errors"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/phonon"
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
