package storymaker

import (
	"strings"

	"aurago/internal/phonon"
)

// ApplyPhononVoices resolves effective speech for editing, generation and rendering.
// A nil project voice stays nil; explicit cast voices still follow the policy.
func ApplyPhononVoices(story *Story, catalog *Catalog) bool {
	if story == nil {
		return false
	}
	changed := false
	projectLanguage := ""
	if story.Voice != nil {
		voice := phononVoice(story.Voice.Language, "", story.Voice)
		if *story.Voice != voice {
			story.Voice = &voice
			changed = true
		}
		projectLanguage = story.Voice.Language
	}
	for i := range story.Cast {
		member := &story.Cast[i]
		if story.Voice == nil && member.Voice == nil {
			continue
		}
		language := projectLanguage
		if member.Voice != nil && strings.TrimSpace(member.Voice.Language) != "" {
			language = member.Voice.Language
		}
		gender := ""
		if catalog != nil {
			if entry, ok := catalog.Lookup(member.Character); ok {
				for _, tag := range entry.Tags {
					switch tag {
					case "female", "woman", "girl":
						gender = "female"
					case "male", "man", "boy":
						gender = "male"
					}
				}
			}
		}
		voice := phononVoice(language, gender, member.Voice)
		if member.Voice == nil || *member.Voice != voice {
			member.Voice = &voice
			changed = true
		}
	}
	return changed
}

func phononVoice(language, gender string, preferred *Voice) Voice {
	language = strings.TrimSpace(language)
	id := phonon.DefaultVoiceForGender(language, gender)
	if preferred != nil && NormalizeProvider(preferred.Provider) == "phonon" {
		if candidate, ok := phonon.Lookup(preferred.Voice); ok && candidate.Language == phonon.Language(language) && (gender == "" || candidate.Gender == gender) {
			id = candidate.ID
		}
	}
	return Voice{Provider: "phonon", Voice: id, Language: language}
}
