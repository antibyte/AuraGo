package server

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/phonon"
	"aurago/internal/realtimespeech"
	"aurago/internal/speechlab"
	"aurago/internal/storymaker"
	"aurago/internal/tools"
)

// storymakerVoiceListTimeout bounds one provider list, so a sidecar that hangs cannot hold GET /voices.
const storymakerVoiceListTimeout = 5 * time.Second

// storymakerPhononLanguages are the bases phonon.Language accepts (internal/phonon/catalog.go:63-70).
var storymakerPhononLanguages = []string{"de", "en", "es", "fr", "pt"}

// storymakerSanoLanguages are the languages sanotts.Voice has a voice for. hi, ne and zh fall back
// to English there (internal/sanotts/sanotts.go:46-65), so they are left out.
var storymakerSanoLanguages = []string{"ar", "cs", "de", "en", "es", "fr", "id", "it", "pt", "ro", "ru", "tr", "vi"}

func handleStorymakerVoices(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	if _, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, false); !ok {
		return
	}
	writeStorymakerJSON(w, http.StatusOK, map[string]any{"providers": s.storymakerVoiceProviders(r.Context())})
}

// storymakerCloudVoiceIDs are the cloud ids, sorted. google joins only when it is the active provider.
var storymakerCloudVoiceIDs = []string{"elevenlabs", "google", "minimax", "mistral", "openai", "resemble", "stepfun", "xai"}

// storymakerXAIVoiceTTL is how long a fetched xAI list stays valid.
const storymakerXAIVoiceTTL = 5 * time.Minute

// storymakerVoiceCache holds the xAI voice list, the only list that costs a network call to a
// provider. A failed fetch is cached too, so the voice panel cannot hammer the endpoint.
type storymakerVoiceCache struct {
	mu      sync.Mutex
	client  *realtimespeech.Client
	now     func() time.Time
	key     string
	fetched time.Time
	voices  []realtimespeech.Voice
	err     error
}

func newStorymakerVoiceCache() *storymakerVoiceCache {
	return &storymakerVoiceCache{client: realtimespeech.NewClient(), now: time.Now}
}

// xai returns the list for this API key. A caller that went away does not fill the cache, because
// its cancelled request says nothing about xAI.
func (c *storymakerVoiceCache) xai(ctx context.Context, apiKey string) ([]realtimespeech.Voice, error) {
	if c == nil {
		return nil, errors.New("the voice cache is not available")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key == apiKey && !c.fetched.IsZero() && c.now().Sub(c.fetched) < storymakerXAIVoiceTTL {
		return c.voices, c.err
	}
	fetchCtx, cancel := context.WithTimeout(ctx, storymakerVoiceListTimeout)
	defer cancel()
	voices, err := c.client.FetchXAIVoices(fetchCtx, apiKey)
	if err != nil && ctx.Err() != nil {
		return nil, err
	}
	c.key, c.fetched, c.voices, c.err = apiKey, c.now(), voices, err
	return voices, err
}

// storymakerConfiguredVoice is the configured voice, or the provider's built-in default.
func storymakerConfiguredVoice(configured, fallback string) string {
	if voice := strings.TrimSpace(configured); voice != "" {
		return voice
	}
	return fallback
}

// storymakerCloudVoiceRow builds one cloud row. The fallbacks are the defaults of the synthesis code
// in internal/tools (tts.go:478, tts.go:793, tts_cloud.go:21-24, tts_cloud.go:37).
func storymakerCloudVoiceRow(ctx context.Context, cfg *config.Config, id string, cache *storymakerVoiceCache) storymaker.VoiceProvider {
	row := storymakerVoiceRow(cfg, id)
	switch id {
	case "elevenlabs":
		row.FreeText, row.DefaultVoice = true, storymakerConfiguredVoice(cfg.TTS.ElevenLabs.VoiceID, "21m00Tcm4TlvDq8ikWAM")
	case "minimax":
		row.FreeText, row.DefaultVoice = true, storymakerConfiguredVoice(cfg.TTS.MiniMax.VoiceID, "English_expressive_narrator")
	case "mistral":
		row.FreeText, row.DefaultVoice = true, storymakerConfiguredVoice(cfg.TTS.Mistral.VoiceID, "")
		if !row.Ready {
			row.Note = "Needs an API key and a voice id."
		}
	case "resemble":
		row.FreeText, row.DefaultVoice = true, storymakerConfiguredVoice(cfg.TTS.Resemble.VoiceID, "")
		if !row.Ready {
			row.Note = "Needs an API key and a voice id."
		}
	case "stepfun":
		row.FreeText, row.DefaultVoice = true, storymakerConfiguredVoice(cfg.TTS.StepFun.VoiceID, "jingdiannvsheng")
	case "openai":
		row.DefaultVoice = storymakerConfiguredVoice(cfg.TTS.OpenAI.VoiceID, "coral")
		for _, voice := range tools.OpenAITTSVoices() {
			row.Voices = append(row.Voices, storymaker.VoiceOption{ID: voice, Name: voice})
		}
		row.Note = "The default model gpt-4o-mini-tts shuts down on 2027-01-06."
	case "xai":
		row.DefaultVoice = storymakerConfiguredVoice(cfg.TTS.XAI.VoiceID, "eve")
		if !row.Ready {
			break
		}
		voices, err := cache.xai(ctx, strings.TrimSpace(cfg.TTS.XAI.APIKey))
		if err != nil {
			row.Ready = false
			row.Note = "The xAI voice list could not be read."
			break
		}
		for _, voice := range voices {
			row.Voices = append(row.Voices, storymaker.VoiceOption{ID: voice.ID, Name: voice.Label})
		}
	case "google":
		row.Note = "Language only, with no voice choice. It uses an unofficial endpoint."
	}
	if !row.Ready && row.Note == "" {
		row.Note = "Needs an API key."
	}
	return row
}

// storymakerActiveVoiceProvider is the provider the tool route would use, as a list id.
func storymakerActiveVoiceProvider(cfg *config.Config) string {
	id := storymaker.NormalizeProvider(tools.BuildTTSConfig(cfg, tools.TTSRouteTool, "").Provider)
	if id == "s2s" || id == "s2s-lab" {
		return "speech_lab"
	}
	return id
}

// storymakerVoiceProviders is the one provider list behind GET /voices, POST /projects and
// POST /narrate: the active provider first, then the local ids, then the cloud ids sorted. Keeping
// local ahead of cloud means voice resolution never picks a paid provider ahead of a free one.
func (s *Server) storymakerVoiceProviders(ctx context.Context) []storymaker.VoiceProvider {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return []storymaker.VoiceProvider{}
	}
	var cache *storymakerVoiceCache
	if rt, err := s.storymakerRuntime(); err == nil {
		cache = rt.voices
	}
	active := storymakerActiveVoiceProvider(cfg)
	rows := s.storymakerLocalVoiceRows(ctx, cfg)
	for _, id := range storymakerCloudVoiceIDs {
		if id == "google" && active != "google" {
			continue
		}
		rows = append(rows, storymakerCloudVoiceRow(ctx, cfg, id, cache))
	}
	out := make([]storymaker.VoiceProvider, 0, len(rows))
	for _, row := range rows {
		if row.ID == active {
			row.Active = true
			out = append(out, row)
		}
	}
	for _, row := range rows {
		if row.ID != active {
			out = append(out, row)
		}
	}
	return out
}

// storymakerVoiceRow is the part of a row that every provider shares. Readiness runs on a copy of
// the config with this provider selected, on the tool route, because the chat route forces
// speech_lab (internal/tools/tts_config.go:35-37).
func storymakerVoiceRow(cfg *config.Config, id string) storymaker.VoiceProvider {
	cp := *cfg
	cp.TTS.Provider = id
	return storymaker.VoiceProvider{
		ID:              id,
		Ready:           tools.TTSReady(&cp, tools.TTSRouteTool),
		Languages:       []string{"*"},
		TestedLanguages: []string{},
		ChunkLimit:      storymaker.ChunkLimit(id),
	}
}

// storymakerBoundedList runs one provider list with the shared time limit. The list calls take no
// context, so a late answer is dropped and its goroutine ends with the call's own timeout.
func storymakerBoundedList[T any](ctx context.Context, list func() ([]T, error)) ([]T, error) {
	type answer struct {
		items []T
		err   error
	}
	done := make(chan answer, 1)
	go func() {
		items, err := list()
		done <- answer{items, err}
	}()
	timer := time.NewTimer(storymakerVoiceListTimeout)
	defer timer.Stop()
	select {
	case a := <-done:
		return a.items, a.err
	case <-timer.C:
		return nil, errors.New("the voice list timed out")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// storymakerPhononRule is the phonon readiness rule. TTSReady says yes for phonon with no files at
// all (internal/tools/tts_config.go:166-167), so the runtime and the voice must be checked as well.
func storymakerPhononRule(ttsReady bool, readyErr, voiceErr error) (ready, needsInstall bool) {
	needsInstall = readyErr != nil || voiceErr != nil
	return ttsReady && !needsInstall, needsInstall
}

func storymakerPhononRow(cfg *config.Config) storymaker.VoiceProvider {
	row := storymakerVoiceRow(cfg, "phonon")
	row.Languages = storymakerPhononLanguages
	root, dataDir := phonon.Root(), cfg.Directories.DataDir
	voice := strings.TrimSpace(cfg.TTS.Phonon.VoiceID)
	if voice == "" {
		voice = phonon.DefaultVoice(cfg.SanoTTSLanguage(""), cfg.Personality.CorePersonality)
	}
	if voice == "" {
		// The configured language is one phonon does not speak. English stands in, so the row
		// still tells whether phonon can speak at all.
		voice = phonon.DefaultVoice("en", cfg.Personality.CorePersonality)
	}
	row.DefaultVoice = voice
	_, voiceErr := phonon.VoicePath(root, dataDir, voice)
	row.Ready, row.NeedsInstall = storymakerPhononRule(row.Ready, phonon.Ready(root), voiceErr)
	if row.NeedsInstall {
		row.Note = "The Phonon runtime or the chosen voice is not installed."
	}
	for _, v := range phonon.Voices(root, dataDir) {
		installed := v.Installed
		row.Voices = append(row.Voices, storymaker.VoiceOption{ID: v.ID, Name: v.Name, Language: v.Language, Gender: v.Gender, Installed: &installed})
	}
	return row
}

// storymakerPiperLanguages lists the bases of the voice locales, sorted and without repeats.
func storymakerPiperLanguages(voices []tools.WyomingVoice) []string {
	seen := make(map[string]bool)
	out := []string{}
	for _, v := range voices {
		for _, locale := range v.Languages {
			base := storymaker.LanguageBase(locale)
			if base != "" && !seen[base] {
				seen[base] = true
				out = append(out, base)
			}
		}
	}
	sort.Strings(out)
	return out
}

func storymakerPiperRow(ctx context.Context, cfg *config.Config) storymaker.VoiceProvider {
	row := storymakerVoiceRow(cfg, "piper")
	// "piper" is the TTS system name, not a voice (internal/tools/tts.go:655-659).
	if voice := strings.TrimSpace(cfg.TTS.Piper.Voice); voice != "piper" {
		row.DefaultVoice = voice
	}
	if !row.Ready {
		return row
	}
	voices, err := storymakerBoundedList(ctx, func() ([]tools.WyomingVoice, error) {
		return tools.PiperListVoices(cfg.TTS.Piper.ContainerPort)
	})
	if err != nil {
		row.Ready = false
		row.Note = "The Piper voice list could not be read."
		return row
	}
	for _, v := range voices {
		installed := v.Installed
		option := storymaker.VoiceOption{ID: v.Name, Name: v.Name, Installed: &installed}
		if len(v.Languages) > 0 {
			option.Language = storymaker.LanguageBase(v.Languages[0])
		}
		row.Voices = append(row.Voices, option)
	}
	if languages := storymakerPiperLanguages(voices); len(languages) > 0 {
		row.Languages = languages
	}
	return row
}

func storymakerSanoRow(cfg *config.Config) storymaker.VoiceProvider {
	row := storymakerVoiceRow(cfg, "sanotts")
	row.Languages = storymakerSanoLanguages
	row.Note = "The voice follows the language. hi, ne and zh have no voice here, so they are not offered."
	return row
}

func storymakerSupertonicRow(ctx context.Context, cfg *config.Config) storymaker.VoiceProvider {
	row := storymakerVoiceRow(cfg, "supertonic")
	row.DefaultVoice = strings.TrimSpace(cfg.TTS.Supertonic.Voice)
	if row.DefaultVoice == "" {
		row.DefaultVoice = "M1" // the default of internal/tools/tts.go:679-682
	}
	if !row.Ready {
		return row
	}
	styles, err := storymakerBoundedList(ctx, func() ([]tools.SupertonicStyle, error) {
		return tools.SupertonicListStyles(cfg.TTS.Supertonic.URL)
	})
	if err != nil {
		row.Ready = false
		row.Note = "The Supertonic style list could not be read."
		return row
	}
	for _, style := range styles {
		row.Voices = append(row.Voices, storymaker.VoiceOption{ID: style.Name, Name: style.Name})
	}
	return row
}

// storymakerSpeechLabRow lists the one voice of the running stack. Speech Lab has no per-call voice,
// so the /ready snapshot is the whole list.
func (s *Server) storymakerSpeechLabRow(ctx context.Context, cfg *config.Config) storymaker.VoiceProvider {
	row := storymakerVoiceRow(cfg, "speech_lab")
	if base := storymaker.LanguageBase(cfg.SpeechLab.Language); base != "" && base != "auto" {
		row.Languages = []string{base}
	}
	if !row.Ready {
		return row
	}
	client := s.SpeechLab
	if client == nil {
		created, err := speechlab.NewClient(cfg.SpeechLab)
		if err != nil {
			row.Ready = false
			row.Note = "The Speech Lab configuration is not valid."
			return row
		}
		client = created
	}
	ctx, cancel := context.WithTimeout(ctx, storymakerVoiceListTimeout)
	defer cancel()
	ready, err := client.Require(ctx, false, true)
	if err != nil {
		row.Ready = false
		row.Note = "Speech Lab reports no ready voice."
		return row
	}
	row.DefaultVoice = ready.Voice
	row.Voices = []storymaker.VoiceOption{{ID: ready.Voice, Name: ready.Voice}}
	row.Note = "The voice comes from the running Speech Lab stack."
	return row
}

// storymakerLocalVoiceRows returns the local providers in the order of the provider list.
func (s *Server) storymakerLocalVoiceRows(ctx context.Context, cfg *config.Config) []storymaker.VoiceProvider {
	return []storymaker.VoiceProvider{
		storymakerPhononRow(cfg),
		storymakerPiperRow(ctx, cfg),
		storymakerSanoRow(cfg),
		storymakerSupertonicRow(ctx, cfg),
		s.storymakerSpeechLabRow(ctx, cfg),
	}
}
