package storymaker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// VoiceOption is one voice of a provider.
type VoiceOption struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Language  string `json:"language,omitempty"`
	Gender    string `json:"gender,omitempty"`
	Installed *bool  `json:"installed,omitempty"`
	Note      string `json:"note,omitempty"`
}

// VoiceProvider is one entry of GET /voices.
type VoiceProvider struct {
	ID              string        `json:"id"`
	Active          bool          `json:"active"`
	Ready           bool          `json:"ready"`
	Voices          []VoiceOption `json:"voices,omitempty"`
	FreeText        bool          `json:"free_text,omitempty"`
	Languages       []string      `json:"languages"`        // "*" means not restricted
	TestedLanguages []string      `json:"tested_languages"` // never null; empty in phase 2
	NeedsInstall    bool          `json:"needs_install"`
	DefaultVoice    string        `json:"default_voice,omitempty"`
	ChunkLimit      int           `json:"chunk_limit"`
	Note            string        `json:"note,omitempty"`
}

// ErrNoVoiceForLanguage reports that no ready provider speaks the language.
var ErrNoVoiceForLanguage = errors.New("storymaker: no voice for this language")

// ResolveProjectVoice picks the voice of a new project. providers has the order of
// storymakerVoiceProviders: the active provider, then local ones, then cloud ones.
//  1. the active provider, if ready and it supports the language, with its configured voice
//     (DefaultVoice) when that voice speaks the language
//  2. else that provider's default voice for the language
//  3. else the first other ready provider in list order that supports the language
//
// When nothing fits it returns ErrNoVoiceForLanguage and the languages ready providers speak.
func ResolveProjectVoice(providers []VoiceProvider, language string, defaultVoiceFor func(provider, language string) string) (Voice, []string, error) {
	language = strings.TrimSpace(language)
	base := LanguageBase(language)
	for i := range providers {
		p := &providers[i]
		if !p.Active || !p.Ready || !providerSpeaks(p, base) {
			continue
		}
		id := NormalizeProvider(p.ID)
		voice := p.DefaultVoice
		if !voiceSpeaks(p, voice, base) {
			voice = defaultVoiceOf(p, id, language, base, defaultVoiceFor)
		}
		return Voice{Provider: id, Voice: voice, Language: language}, nil, nil
	}
	for i := range providers {
		p := &providers[i]
		if p.Active || !p.Ready || !providerSpeaks(p, base) {
			continue
		}
		id := NormalizeProvider(p.ID)
		return Voice{Provider: id, Voice: defaultVoiceOf(p, id, language, base, defaultVoiceFor), Language: language}, nil, nil
	}
	return Voice{}, readyLanguages(providers), ErrNoVoiceForLanguage
}

func providerSpeaks(p *VoiceProvider, base string) bool {
	for _, l := range p.Languages {
		if l == "*" || LanguageBase(l) == base {
			return true
		}
	}
	return false
}

// voiceSpeaks says whether a voice id speaks the language. A provider without a list has nothing
// to check against, so its id counts. A listed voice without a language counts too.
func voiceSpeaks(p *VoiceProvider, voice, base string) bool {
	if voice == "" {
		return false
	}
	for _, o := range p.Voices {
		if o.ID == voice {
			return o.Language == "" || LanguageBase(o.Language) == base
		}
	}
	return len(p.Voices) == 0
}

// defaultVoiceOf asks defaultVoiceFor first. Without an answer it takes the provider's own voice,
// then its first installed voice of that language, so a list provider does not run with an empty
// voice that would mean the configured voice of another language.
func defaultVoiceOf(p *VoiceProvider, id, language, base string, defaultVoiceFor func(provider, language string) string) string {
	if defaultVoiceFor != nil {
		if v := defaultVoiceFor(id, language); v != "" {
			return v
		}
	}
	if voiceSpeaks(p, p.DefaultVoice, base) {
		return p.DefaultVoice
	}
	for _, o := range p.Voices {
		if LanguageBase(o.Language) == base && (o.Installed == nil || *o.Installed) {
			return o.ID
		}
	}
	return ""
}

// readyLanguages is the sorted union of what ready providers speak, without "*".
func readyLanguages(providers []VoiceProvider) []string {
	seen := map[string]bool{}
	out := []string{}
	for i := range providers {
		if !providers[i].Ready {
			continue
		}
		for _, l := range providers[i].Languages {
			base := LanguageBase(l)
			if l == "*" || base == "" || seen[base] {
				continue
			}
			seen[base] = true
			out = append(out, base)
		}
	}
	sort.Strings(out)
	return out
}

// LanguageBase is the first segment of the lower-cased tag: "pt-BR" and "pt_BR" give "pt".
func LanguageBase(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_"); i >= 0 {
		return tag[:i]
	}
	return tag
}

// LocalVoiceProvider says whether a provider runs on this box. Local providers are exempt from the
// character caps. It normalises its argument.
func LocalVoiceProvider(id string) bool {
	switch NormalizeProvider(id) {
	case "phonon", "piper", "sanotts", "supertonic", "speech_lab":
		return true
	}
	return false
}

// ApplyCaptionOverride sets style.captions when burnIn is not nil.
func ApplyCaptionOverride(s *Story, burnIn *bool) {
	if burnIn != nil {
		s.Style.Captions = *burnIn
	}
}

// CreateWithVoice is Create with story.voice set to a copy of voice. A nil voice leaves voice:null.
func (p *ProjectStore) CreateWithVoice(ctx context.Context, title, aspect string, fps int, voice *Voice) (Project, Story, string, error) {
	return p.CreateWithOptions(ctx, title, aspect, fps, voice, CreateOptions{})
}

// CreateOptions are the choices of a new project that the editor does not make. Compose is the story's style.compose: the
// agent sets "v2", the editor leaves it empty, so existing and hand-made films keep their look.
type CreateOptions struct {
	Compose string
}

// CreateWithOptions is CreateWithVoice with options.
func (p *ProjectStore) CreateWithOptions(ctx context.Context, title, aspect string, fps int, voice *Voice, opts CreateOptions) (Project, Story, string, error) {
	title = strings.TrimSpace(title)
	if title == "" || runeLen(title) > 120 {
		return Project{}, Story{}, "", fmt.Errorf("%w: the title must have 1 to 120 characters", ErrInvalidArgument)
	}
	if aspect == "" {
		aspect = DefaultAspect
	}
	if aspect != "16:9" && aspect != "9:16" && aspect != "1:1" {
		return Project{}, Story{}, "", fmt.Errorf("%w: aspect must be 16:9, 9:16 or 1:1", ErrInvalidArgument)
	}
	if fps == 0 {
		fps = DefaultFPS
	}
	if fps != 24 && fps != 25 && fps != 30 {
		return Project{}, Story{}, "", fmt.Errorf("%w: fps must be 24, 25 or 30", ErrInvalidArgument)
	}

	id := p.newID()
	now := p.now()
	story := Story{
		Schema: Schema, Title: title,
		Format: Format{Aspect: aspect, FPS: fps},
		Style:  Style{Palette: DefaultPalette, Font: DefaultFont, Captions: true, Compose: opts.Compose},
		Cast:   []CastMember{},
		Scenes: []Scene{{ID: "s1"}},
	}
	if voice != nil {
		v := *voice
		story.Voice = &v
	}
	ApplyDefaults(&story)
	storyBytes, err := CanonicalJSON(story)
	if err != nil {
		return Project{}, Story{}, "", err
	}
	proj := Project{ID: id, Title: title, Aspect: aspect, FPS: fps, CreatedAt: now, UpdatedAt: now}
	projBytes, err := encodeProject(proj)
	if err != nil {
		return Project{}, Story{}, "", err
	}

	for _, sub := range []string{"", "assets", "audio/narration", "audio/music", "exports"} {
		if err := p.st.MkdirAll(ctx, p.file(id, sub)); err != nil {
			p.cleanup(id)
			return Project{}, Story{}, "", fmt.Errorf("storymaker: create project folders: %w", err)
		}
	}
	etag, err := p.st.WriteFile(ctx, p.file(id, "story.json"), storyBytes, MustNotExist)
	if err == nil {
		_, err = p.st.WriteFile(ctx, p.file(id, "project.json"), projBytes, MustNotExist)
	}
	if err != nil {
		p.cleanup(id)
		return Project{}, Story{}, "", fmt.Errorf("storymaker: create project: %w", err)
	}
	return proj, story, etag, nil
}
