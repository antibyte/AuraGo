package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func voiceTestDefault(provider, language string) string {
	return provider + "-" + LanguageBase(language)
}

// The lists have the order of storymakerVoiceProviders: active first, then local, then cloud.
func TestResolveProjectVoiceFollowsTheListOrder(t *testing.T) {
	piper := VoiceProvider{
		ID: "piper", Active: true, Ready: true, Languages: []string{"de", "en"}, DefaultVoice: "de_DE-thorsten-high",
		Voices: []VoiceOption{{ID: "de_DE-thorsten-high", Language: "de_DE"}, {ID: "en_US-amy-low", Language: "en_US"}},
	}
	phonon := VoiceProvider{ID: "phonon", Ready: true, Languages: []string{"de", "en", "es", "fr", "pt"}}
	sanotts := VoiceProvider{ID: "sanotts", Ready: true, Languages: []string{"ar", "cs", "de", "en", "es", "fr", "id", "it", "pt", "ro", "ru", "tr", "vi"}}
	openai := VoiceProvider{ID: "openai", Ready: true, Languages: []string{"*"}}
	offline := VoiceProvider{ID: "elevenlabs", Active: true, Ready: false, Languages: []string{"*"}}
	for _, tc := range []struct {
		name      string
		providers []VoiceProvider
		language  string
		want      Voice
	}{
		{"step 1: the configured voice speaks the language", []VoiceProvider{piper, phonon, openai}, "de-DE",
			Voice{Provider: "piper", Voice: "de_DE-thorsten-high", Language: "de-DE"}},
		{"step 2: the active provider's default voice", []VoiceProvider{piper, phonon, openai}, "en",
			Voice{Provider: "piper", Voice: "piper-en", Language: "en"}},
		{"step 3: the active provider is not ready, a free one wins", []VoiceProvider{offline, phonon, sanotts, openai}, "de",
			Voice{Provider: "phonon", Voice: "phonon-de", Language: "de"}},
		{"step 3: the active provider lacks the language, a free one wins", []VoiceProvider{piper, phonon, sanotts, openai}, "ar",
			Voice{Provider: "sanotts", Voice: "sanotts-ar", Language: "ar"}},
		{"step 3: a paid provider only after every free one", []VoiceProvider{offline, phonon, sanotts, openai}, "hi",
			Voice{Provider: "openai", Voice: "openai-hi", Language: "hi"}},
	} {
		got, languages, err := ResolveProjectVoice(tc.providers, tc.language, voiceTestDefault)
		if err != nil || got != tc.want || languages != nil {
			t.Errorf("%s: %+v, %v, %v; want %+v", tc.name, got, languages, err, tc.want)
		}
	}
}

func TestResolveProjectVoiceRefusesWhatNoProviderSpeaks(t *testing.T) {
	providers := []VoiceProvider{
		{ID: "piper", Active: true, Ready: false, Languages: []string{"*"}},
		{ID: "phonon", Ready: true, Languages: []string{"de", "en", "es", "fr", "pt"}},
		{ID: "sanotts", Ready: true, Languages: []string{"ar", "cs", "de", "en", "es", "fr", "id", "it", "pt", "ro", "ru", "tr", "vi"}},
	}
	got, languages, err := ResolveProjectVoice(providers, "hi", voiceTestDefault)
	if !errors.Is(err, ErrNoVoiceForLanguage) || got != (Voice{}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	// The phonon list is inside the sanotts list, so the union is the sanotts list. "*" is left out.
	want := []string{"ar", "cs", "de", "en", "es", "fr", "id", "it", "pt", "ro", "ru", "tr", "vi"}
	if !reflect.DeepEqual(languages, want) {
		t.Fatalf("languages = %v, want %v", languages, want)
	}
}

func TestLanguageBaseAndLocalVoiceProvider(t *testing.T) {
	for tag, want := range map[string]string{"de": "de", " DE_de ": "de", "pt-BR": "pt", "ZH-Hans-CN": "zh"} {
		if got := LanguageBase(tag); got != want {
			t.Errorf("LanguageBase(%q) = %q, want %q", tag, got, want)
		}
	}
	for _, id := range []string{"Phonon", "piper", "sanotts", "supertonic", " speech_lab "} {
		if !LocalVoiceProvider(id) {
			t.Errorf("LocalVoiceProvider(%q) = false", id)
		}
	}
	for _, id := range []string{"openai", "google", ""} {
		if LocalVoiceProvider(id) {
			t.Errorf("LocalVoiceProvider(%q) = true", id)
		}
	}
}

func TestApplyCaptionOverride(t *testing.T) {
	on, off := true, false
	s := Story{Style: Style{Captions: true}}
	ApplyCaptionOverride(&s, nil)
	if !s.Style.Captions {
		t.Fatal("a nil override changed the style")
	}
	ApplyCaptionOverride(&s, &off)
	if s.Style.Captions {
		t.Fatal("false did not switch captions off")
	}
	ApplyCaptionOverride(&s, &on)
	if !s.Style.Captions {
		t.Fatal("true did not switch captions on")
	}
}

func TestCreateWithVoiceStoresTheVoiceAndCreateStaysNull(t *testing.T) {
	ps, _, _ := newTestStore(t)
	ctx := context.Background()
	storedVoice := func(id string) json.RawMessage {
		t.Helper()
		data, _, err := ps.LoadStory(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		return doc["voice"]
	}
	plain, story, _, err := ps.Create(ctx, "Ohne Stimme", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if story.Voice != nil || string(storedVoice(plain.ID)) != "null" {
		t.Fatalf("Create must leave voice:null: %+v, %s", story.Voice, storedVoice(plain.ID))
	}
	want := Voice{Provider: "sanotts", Language: "de"}
	voiced, story, _, err := ps.CreateWithVoice(ctx, "Mit Stimme", "", 0, &want)
	if err != nil {
		t.Fatal(err)
	}
	var stored Voice
	if err := json.Unmarshal(storedVoice(voiced.ID), &stored); err != nil || stored != want || story.Voice == nil || *story.Voice != want {
		t.Fatalf("stored %+v, answer %+v, %v", stored, story.Voice, err)
	}
}
