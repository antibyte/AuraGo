package storymaker

import (
	"reflect"
	"testing"

	"aurago/internal/phonon"
)

func TestPhononVoicesMatchLanguageAndCatalogGender(t *testing.T) {
	catalog := loadReal(t)
	for _, language := range []string{"de-DE", "en_US", "fr", "es", "pt-BR"} {
		t.Run(language, func(t *testing.T) {
			story := Story{Voice: &Voice{Provider: "openai", Voice: "alloy", Language: language}, Cast: []CastMember{
				{ID: "a", Character: "char.hoodie"}, {ID: "b", Character: "char.professor"},
				{ID: "c", Character: "char.punk"}, {ID: "d", Character: "char.office-woman-1"},
				{ID: "e", Character: "char.senior-woman-1"},
			}}
			if !ApplyPhononVoices(&story, catalog) || ApplyPhononVoices(&story, catalog) {
				t.Fatal("voice resolution must change the old provider exactly once")
			}
			for i, gender := range []string{"male", "male", "female", "female", "male"} {
				got := story.Cast[i].Voice
				v, ok := phonon.Lookup(got.Voice)
				if !ok || got.Provider != "phonon" || got.Language != language || v.Language != LanguageBase(language) || v.Gender != gender || !v.Bundled {
					t.Fatalf("cast %d: %+v, catalog %+v", i, got, v)
				}
			}
			story.Scenes = []Scene{{ID: "s1", Speaker: "a", Narration: "Hallo.", Beats: []Beat{{Do: "say", Who: "c", Text: "Guten Tag."}}}}
			blocks := Blocks(&story)
			if len(blocks) != 2 || blocks[0].Voice != *story.Cast[0].Voice || blocks[1].Voice != *story.Cast[2].Voice {
				t.Fatalf("narration/dialogue lost their speaker voices: %+v", blocks)
			}
			if PieceKey(blocks[0].Voice, "same text") == PieceKey(blocks[1].Voice, "same text") {
				t.Fatal("different speaker voices must not share cached audio")
			}
		})
	}
}

func TestPhononVoicesPreserveSpeakerLanguageAndCorrectChangedCharacter(t *testing.T) {
	catalog := loadReal(t)
	voice := Voice{Provider: "phonon", Voice: phonon.DefaultVoiceForGender("fr", "female"), Language: "fr"}
	story := Story{Voice: &Voice{Provider: "phonon", Voice: phonon.DefaultVoiceForGender("de", "male"), Language: "de"}, Cast: []CastMember{{ID: "a", Character: "char.punk", Voice: &voice}}}
	before := *story.Voice
	ApplyPhononVoices(&story, catalog)
	if !reflect.DeepEqual(*story.Cast[0].Voice, voice) || *story.Voice != before {
		t.Fatal("valid narrator or multilingual character voice was lost")
	}
	story.Cast[0].Character = "char.professor"
	ApplyPhononVoices(&story, catalog)
	if got := *story.Cast[0].Voice; got.Language != "fr" || got.Voice != phonon.DefaultVoiceForGender("fr", "male") {
		t.Fatal(got)
	}
	story.Voice = &Voice{Provider: "openai", Voice: "alloy", Language: "ja"}
	ApplyPhononVoices(&story, catalog)
	if story.Voice.Provider != "phonon" || story.Voice.Language != "ja" || story.Voice.Voice != "" {
		t.Fatal("unsupported language silently fell back", story.Voice)
	}
	story.Voice = nil
	if ApplyPhononVoices(&story, catalog) || story.Voice != nil {
		t.Fatal("explicitly unvoiced story changed")
	}
	story.Cast[0].Voice.Provider = "openai"
	ApplyPhononVoices(&story, catalog)
	if story.Voice != nil || story.Cast[0].Voice.Provider != "phonon" {
		t.Fatal("explicit cast voice bypassed Phonon when narrator is unset")
	}
}
