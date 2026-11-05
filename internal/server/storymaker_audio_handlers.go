package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/phonon"
	"aurago/internal/realtimespeech"
	"aurago/internal/speechlab"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"
)

// storymakerVoiceListTimeout bounds one provider list, so a sidecar that hangs cannot hold GET /voices.
const storymakerVoiceListTimeout = 5 * time.Second

// storymakerPhononLanguages are the bases phonon.Language accepts (internal/phonon/catalog.go:63-70).
var storymakerPhononLanguages = []string{"de", "en", "es", "fr", "pt"}

// storymakerSanoLanguages are the languages sanotts.Voice has a voice for. hi, ne and zh fall back
// to English there (internal/sanotts/sanotts.go:46-65), so they are left out.
var storymakerSanoLanguages = []string{"ar", "cs", "de", "en", "es", "fr", "id", "it", "pt", "ro", "ru", "tr", "vi"}

// storymakerCodeVoiceUnknown answers a voice that is not on its provider's list or cannot run here.
const storymakerCodeVoiceUnknown = "voice_unknown"

// storymakerMaxVoiceAlternatives is the cap of contract line 60.
const storymakerMaxVoiceAlternatives = 5

// storymakerAudioFilePattern is the only file name the audio routes serve. By construction it leaves
// out env.json, meta.json and pending.json.
var storymakerAudioFilePattern = regexp.MustCompile(`^([0-9a-f]{64})\.(wav|mp3)$`)

// storymakerAudioTypes gives each served extension its fixed Content-Type.
var storymakerAudioTypes = map[string]string{"wav": "audio/wav", "mp3": "audio/mpeg"}

// handleStorymakerAudio serves GET and HEAD /projects/{id}/audio/{narration|music}/{name}. These are
// inert files: nosniff and a sandbox stop a browser that navigates to the URL from running anything.
// It goes through Store.Open, because Store.ReadFile stops at 50 MB and music can be larger.
func handleStorymakerAudio(s *Server, w http.ResponseWriter, r *http.Request, id, folder, file string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeStorymakerMethodError(w, "GET, HEAD")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
	if !ok {
		return
	}
	match := storymakerAudioFilePattern.FindStringSubmatch(file)
	if (folder != "narration" && folder != "music") || match == nil {
		writeStorymakerError(w, http.StatusNotFound, "audio_not_found", "Audio file not found.", nil)
		return
	}
	content, info, err := acc.rt.projects.OpenAudio(r.Context(), id, folder, file)
	if err != nil {
		mapStorymakerError(w, err, "audio_not_found")
		return
	}
	defer content.Close()
	h := w.Header()
	h.Set("Content-Type", storymakerAudioTypes[match[2]])
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, max-age=86400, immutable")
	http.ServeContent(w, r, file, info.ModTime, content)
}

// writeStorymakerErrorWith is writeStorymakerError with extra fields next to the fixed three.
func writeStorymakerErrorWith(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	body := make(map[string]any, len(extra)+3)
	for key, value := range extra {
		body[key] = value
	}
	body["error"], body["code"], body["message"] = code, code, message
	writeStorymakerJSON(w, status, body)
}

// handleStorymakerNarration answers GET /projects/{id}/narration. It works while the desktop is read-only.
func handleStorymakerNarration(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeStorymakerMethodError(w, "GET")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeRead, desktopRead, true)
	if !ok {
		return
	}
	_, answer, ok := s.storymakerStoryAudio(w, r, acc, id)
	if !ok {
		return
	}
	writeStorymakerJSON(w, http.StatusOK, answer)
}

// handleStorymakerNarrate answers POST /projects/{id}/narrate: 200 {job:null} when nothing is
// missing, the refusals in their fixed order, then 202 for a new job and 200 for the job of this
// project that is already queued or running.
func handleStorymakerNarrate(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	story, answer, ok := s.storymakerStoryAudio(w, r, acc, id)
	if !ok {
		return
	}
	if len(answer.Missing) == 0 {
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": nil})
		return
	}
	if storymakerNarrateRefused(r.Context(), w, s, acc, storymakerBlocksToSynthesize(story, answer), answer) {
		return
	}
	job, replayed, err := acc.rt.narrate.Enqueue(render.EnqueueRequest{
		ProjectID: id,
		Kind:      render.KindNarrate,
		Coalesce:  true,
		Payload:   storymakerNarratePayload{ProjectID: id},
	})
	switch {
	case errors.Is(err, render.ErrBusy):
		writeStorymakerError(w, http.StatusTooManyRequests, render.CodeBusy, "128 narration jobs are already waiting. Try again when one has finished.", nil)
	case err != nil:
		writeStorymakerError(w, http.StatusInternalServerError, "job_failed", "The narration job could not be queued.", nil)
	case replayed:
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": job})
	default:
		writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"job": job})
	}
}

// storymakerStoryAudio reads the story with its defaults applied, the way a render job sees it, and
// its audio answer. On a failure it has answered the request and returns false.
func (s *Server) storymakerStoryAudio(w http.ResponseWriter, r *http.Request, acc *storymakerAccess, id string) (*storymaker.Story, storymaker.AudioAnswer, bool) {
	ctx := r.Context()
	data, _, err := acc.rt.projects.LoadStory(ctx, id)
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return nil, storymaker.AudioAnswer{}, false
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return nil, storymaker.AudioAnswer{}, false
	}
	storymaker.ApplyDefaults(&story)
	answer, err := acc.rt.projects.AudioAnswer(ctx, id, &story, s.storymakerMusicKey)
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return nil, storymaker.AudioAnswer{}, false
	}
	return &story, answer, true
}

// storymakerBlocksToSynthesize keeps the blocks with missing pieces. Only they call a provider, so
// a provider that went away cannot block a story whose other blocks need it no more.
func storymakerBlocksToSynthesize(story *storymaker.Story, answer storymaker.AudioAnswer) []storymaker.NarrationBlock {
	missing := make(map[string]bool, len(answer.Missing))
	for _, m := range answer.Missing {
		missing[m.BlockID] = true
	}
	var out []storymaker.NarrationBlock
	for _, b := range storymaker.Blocks(story) {
		if missing[b.ID] {
			out = append(out, b)
		}
	}
	return out
}

// storymakerNarrateRefused answers the first refusal in the order voice_required,
// narration_unavailable, voice_unknown, budget_exceeded, and reports whether it answered.
func storymakerNarrateRefused(ctx context.Context, w http.ResponseWriter, s *Server, acc *storymakerAccess, blocks []storymaker.NarrationBlock, answer storymaker.AudioAnswer) bool {
	for _, b := range blocks {
		if b.Voice.Provider == "" {
			writeStorymakerError(w, http.StatusConflict, render.CodeVoiceRequired, fmt.Sprintf("Block %s has no voice. Choose a voice for the story first.", b.ID), nil)
			return true
		}
	}
	rows := make(map[string]storymaker.VoiceProvider)
	for _, row := range s.storymakerVoiceProviders(ctx) {
		rows[row.ID] = row
	}
	for _, b := range blocks {
		row, offered := rows[b.Voice.Provider]
		if !offered {
			writeStorymakerError(w, http.StatusConflict, render.CodeNarrationUnavailable, fmt.Sprintf("The voice provider %s is not offered here. Block %s needs it.", b.Voice.Provider, b.ID), nil)
			return true
		}
		if !row.Ready {
			writeStorymakerError(w, http.StatusConflict, render.CodeNarrationUnavailable, fmt.Sprintf("The voice provider %s is not ready. Block %s needs it.", b.Voice.Provider, b.ID), nil)
			return true
		}
	}
	for _, b := range blocks {
		row := rows[b.Voice.Provider]
		if !storymakerVoiceOnList(row, b.Voice.Voice) {
			writeStorymakerErrorWith(w, http.StatusConflict, storymakerCodeVoiceUnknown,
				fmt.Sprintf("The voice %s of %s is not available here. Block %s uses it.", b.Voice.Voice, b.Voice.Provider, b.ID),
				map[string]any{"alternatives": storymakerVoiceAlternatives(row, b.Voice.Language)})
			return true
		}
	}
	cloud := make(map[string]bool, len(blocks))
	for _, b := range blocks {
		cloud[b.ID] = !storymaker.LocalVoiceProvider(b.Voice.Provider)
	}
	chars := 0
	for _, m := range answer.Missing {
		if cloud[m.BlockID] {
			chars += m.Chars
		}
	}
	if chars == 0 {
		return false
	}
	caps := acc.cfg.Storymaker.Narration
	// today turns a damaged file into a full count, so the route refuses as the job would. A read
	// error counts as 0 here, because the job loop holds the real guard.
	used, err := acc.rt.usage.today(caps.MaxDailyChars)
	if err != nil {
		used = 0
	}
	var limit int
	switch {
	case chars > caps.MaxJobChars:
		limit = caps.MaxJobChars
	case used+chars > caps.MaxDailyChars:
		limit = caps.MaxDailyChars
	default:
		return false
	}
	writeStorymakerErrorWith(w, http.StatusConflict, render.CodeBudgetExceeded,
		fmt.Sprintf("This narration needs %d characters from a paid provider and the limit is %d.", chars, limit),
		map[string]any{"limit": limit, "missing": chars})
	return true
}

// storymakerVoiceOnList is the list check of POST /narrate. An empty id means the provider's
// default, and a free-text provider or one without a list has nothing to check against. A listed
// voice that cannot run counts as off the list, so the refusal comes before the job.
func storymakerVoiceOnList(row storymaker.VoiceProvider, voice string) bool {
	if voice == "" || row.FreeText || len(row.Voices) == 0 {
		return true
	}
	for _, option := range row.Voices {
		if option.ID == voice {
			return storymakerVoiceUsable(row, option)
		}
	}
	return false
}

// storymakerVoiceUsable says whether a listed voice can run. A Phonon voice that is not installed
// fails in the job, because nothing downloads it on the way (phonon.VoicePath). Settings offers every
// Piper voice (ui/cfg/tts.js:593-601), so that flag decides nothing.
func storymakerVoiceUsable(row storymaker.VoiceProvider, option storymaker.VoiceOption) bool {
	return row.ID != "phonon" || option.Installed == nil || *option.Installed
}

// storymakerVoiceAlternatives names at most five voices of the provider, those of the language first.
func storymakerVoiceAlternatives(row storymaker.VoiceProvider, language string) []string {
	base := storymaker.LanguageBase(language)
	out := make([]string, 0, storymakerMaxVoiceAlternatives)
	for _, sameLanguage := range []bool{true, false} {
		for _, option := range row.Voices {
			if len(out) == storymakerMaxVoiceAlternatives {
				return out
			}
			if storymakerVoiceUsable(row, option) && (storymaker.LanguageBase(option.Language) == base) == sameLanguage {
				out = append(out, option.ID)
			}
		}
	}
	return out
}

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
