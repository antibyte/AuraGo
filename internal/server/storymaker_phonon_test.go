package server

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"aurago/internal/config"
	"aurago/internal/phonon"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"
)

// Ready metadata for job tests whose synthesis is stubbed; no model files are required.
func readyStorymakerPhononVoices(context.Context) []storymaker.VoiceProvider {
	row := storymaker.VoiceProvider{ID: "phonon", Active: true, Ready: true, Languages: storymakerPhononLanguages, DefaultVoice: phonon.DefaultVoiceForGender("de", "")}
	for _, language := range storymakerPhononLanguages {
		for _, gender := range []string{"female", "male"} {
			row.Voices = append(row.Voices, storymaker.VoiceOption{ID: phonon.DefaultVoiceForGender(language, gender), Language: language, Gender: gender})
		}
	}
	return []storymaker.VoiceProvider{row}
}

func TestStorymakerPhononUsesSpeakerGenderAndIgnoresCloudBudget(t *testing.T) {
	env := newNarrateUsageEnv(t)
	narrateJobSetConfig(env.s, func(c *config.Config) {
		c.TTS.Provider = "openai"
		c.Storymaker.Narration.MaxJobChars = 100
		c.Storymaker.Narration.MaxDailyChars = 1000
	})
	env.mustSave(t, `{"schema":"storymaker/1","title":"Cast","voice":{"provider":"openai","voice":"alloy","language":"de"},"cast":[{"id":"h","name":"Hoodie","character":"char.hoodie"},{"id":"p","name":"Punk","character":"char.punk"}],"scenes":[{"id":"s1","speaker":"h","narration":"Ich erkläre Dir heute die Technik.","beats":[{"do":"say","who":"p","text":"Und ich zeige Dir ein Beispiel."}]}]}`)
	if err := os.WriteFile(env.rt.usage.path, []byte("old damaged cloud usage"), 0o600); err != nil {
		t.Fatal(err)
	}
	var voices []string
	env.s.storymakerSynthOverride = func(_ context.Context, cfg tools.TTSConfig, _ string) ([]byte, string, error) {
		if cfg.Provider != "phonon" || cfg.Language != "de" || !cfg.NoProviderFallback {
			t.Errorf("unexpected synthesis route: provider=%s language=%s fallback=%v", cfg.Provider, cfg.Language, !cfg.NoProviderFallback)
		}
		voices = append(voices, cfg.Phonon.VoiceID)
		return narrateJobWAV(), "wav", nil
	}
	job := env.run(t)
	if job.Status != render.StatusSucceeded || len(voices) != 2 || voices[0] != phonon.DefaultVoiceForGender("de", "male") || voices[1] != phonon.DefaultVoiceForGender("de", "female") {
		t.Fatalf("job=%s voices=%v error=%+v", job.Status, voices, job.Error)
	}
	if data, err := os.ReadFile(env.rt.usage.path); err != nil || string(data) != "old damaged cloud usage" {
		t.Fatalf("local speech touched cloud usage: %v", err)
	}
}

func TestStorymakerAgentRevisionChangesInheritedVoiceLanguage(t *testing.T) {
	env := newNarrateJobEnv(t)
	env.mustSave(t, `{"schema":"storymaker/1","title":"Cast","voice":{"provider":"phonon","language":"de"},"cast":[{"id":"h","name":"Hoodie","character":"char.hoodie","voice":{"provider":"phonon","language":"de"}},{"id":"p","name":"Punk","character":"char.punk","voice":{"provider":"phonon","language":"fr"}}],"scenes":[{"id":"s1"}]}`)
	j := &storymakerAgentJob{s: env.s, rt: env.rt, job: render.Job{ProjectID: env.id}, route: &storymakerAgentRoute{cfg: env.s.ConfigSnapshot()}, payload: storymakerAgentPayload{Kind: storymaker.AgentKindRevise, Language: "en"}}
	language, ready, err := j.prepareVoice(context.Background())
	if err != nil || !ready || language != "en" {
		t.Fatalf("prepare: %s %v %v", language, ready, err)
	}
	raw, _, err := env.rt.projects.LoadStory(context.Background(), env.id)
	var story storymaker.Story
	if err != nil || json.Unmarshal(raw, &story) != nil {
		t.Fatalf("read saved story: %v", err)
	}
	if story.Voice.Language != "en" || story.Cast[0].Voice.Voice != phonon.DefaultVoiceForGender("en", "male") || story.Cast[1].Voice.Voice != phonon.DefaultVoiceForGender("fr", "female") {
		t.Fatalf("wrong voices: %+v %+v %+v", story.Voice, story.Cast[0].Voice, story.Cast[1].Voice)
	}
}

func TestStorymakerSynthNeverCallsAnotherProvider(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	called := false
	s.storymakerSynthOverride = func(context.Context, tools.TTSConfig, string) ([]byte, string, error) {
		called = true
		return nil, "", nil
	}
	_, err := s.storymakerSynth(context.Background(), s.ConfigSnapshot(), storymaker.Voice{Provider: "openai", Voice: "alloy", Language: "de"}, "Hallo.")
	if err == nil || called {
		t.Fatal("non-Phonon speech reached synthesis")
	}
}
