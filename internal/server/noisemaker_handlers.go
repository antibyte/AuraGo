package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// Noisemaker is the virtual desktop music studio ("Suno-style"). These handlers
// expose the existing music/image generation backends plus one-shot LLM helpers
// to the desktop app. All endpoints require desktop scopes.

const (
	noisemakerEnhanceBodyLimit   = int64(64 * 1024)
	noisemakerGenerateBodyLimit  = int64(64 * 1024)
	noisemakerEnhanceTimeout     = 45 * time.Second
	noisemakerMaxIdeaLength      = 2000
	noisemakerMaxStyleLength     = 500
	noisemakerMaxLyricsLength    = 8000
	noisemakerMaxTitleLength     = 200
	noisemakerMaxContextLength   = 4000
	noisemakerTracksDefaultLimit = 60
	noisemakerTracksMaxLimit     = 200
	noisemakerPatchBodyLimit     = int64(4 * 1024)
)

// noisemakerFavoriteTag marks a song as favorite in the media registry tags.
const noisemakerFavoriteTag = "favorite"

func noisemakerHasTag(item tools.MediaItem, tag string) bool {
	for _, t := range item.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// noisemakerTrackJSON serializes a registry item for the library list, the
// PATCH response and the generate response so every surface sees the same shape.
func noisemakerTrackJSON(item tools.MediaItem) map[string]interface{} {
	title := strings.TrimSpace(item.Description)
	if title == "" {
		title = limitNoisemakerText(item.Prompt, 100)
	}
	tags := item.Tags
	if tags == nil {
		tags = []string{}
	}
	return map[string]interface{}{
		"id":           item.ID,
		"title":        title,
		"prompt":       item.Prompt,
		"style":        item.Style,
		"lyrics":       item.Lyrics,
		"tags":         tags,
		"favorite":     noisemakerHasTag(item, noisemakerFavoriteTag),
		"instrumental": noisemakerHasTag(item, "instrumental"),
		"duration_ms":  item.DurationMs,
		"file_size":    item.FileSize,
		"format":       item.Format,
		"web_path":     item.WebPath,
		"cover_url":    item.SourceImage,
		"provider":     item.Provider,
		"model":        item.Model,
		"created_at":   item.CreatedAt,
	}
}

type noisemakerEnhanceRequest struct {
	Kind    string `json:"kind"`    // idea | style | lyrics | title
	Text    string `json:"text"`    // current field content (may be empty for random/from-scratch)
	Context string `json:"context"` // supporting context (idea/style of the other field)
	Lang    string `json:"lang"`    // UI language hint for from-scratch generation
}

type noisemakerTrackPatchRequest struct {
	Favorite *bool `json:"favorite"`
}

type noisemakerGenerateRequest struct {
	Prompt          string  `json:"prompt"`
	Style           string  `json:"style"`
	Lyrics          string  `json:"lyrics"`
	Instrumental    bool    `json:"instrumental"`
	Title           string  `json:"title"`
	Cover           bool    `json:"cover"`
	Lang            string  `json:"lang"`
	DurationSeconds float64 `json:"duration_seconds"`
	BPM             int     `json:"bpm"`
	VocalLanguage   string  `json:"vocal_language"`
	Seed            *int64  `json:"seed"`
}

// handleNoisemakerState returns GET /api/desktop/noisemaker/state — capabilities
// and quota so the app can render onboarding, hide AI helpers, or show the cover option.
func handleNoisemakerState(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		s.CfgMu.RLock()
		mg := s.Cfg.MusicGeneration
		ig := s.Cfg.ImageGeneration
		configured := s.Cfg.MusicConfigured()
		local := s.Cfg.UsesLocalMusic()
		s.CfgMu.RUnlock()
		var localStatus any
		if local && s.LocalMusic != nil {
			localStatus = s.LocalMusic.Status()
		}

		providerType := strings.ToLower(strings.TrimSpace(mg.ProviderType))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":            "ok",
			"enabled":           configured,
			"local":             localStatus,
			"supports_controls": local,
			"configured":        mg.Enabled,
			"provider_type":     providerType,
			"model":             mg.ResolvedModel,
			"supports_lyrics":   providerType == "minimax" || local,
			"daily_used":        tools.MusicCounterGet(),
			"daily_max":         mg.MaxDaily,
			"llm_available":     s.LLMClient != nil && !local,
			"covers_enabled":    ig.Enabled && strings.TrimSpace(ig.APIKey) != "",
			"cover_provider":    strings.ToLower(strings.TrimSpace(ig.ProviderType)),
			"registry_enabled":  s.MediaRegistryDB != nil,
		})
	}
}

// handleNoisemakerEnhance returns POST /api/desktop/noisemaker/enhance — one-shot
// LLM helper that expands a song idea, refines a style, writes lyrics, or suggests a title.
func handleNoisemakerEnhance(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.LLMClient == nil {
			jsonError(w, "LLM is not available", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		local := s.Cfg.UsesLocalMusic()
		s.CfgMu.RUnlock()
		if local {
			jsonError(w, "Local music does not use cloud enhancement. Supply lyrics or use the local music language model.", http.StatusBadRequest)
			return
		}
		var body noisemakerEnhanceRequest
		if err := decodeDesktopJSON(w, r, &body, noisemakerEnhanceBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		body.Kind = strings.ToLower(strings.TrimSpace(body.Kind))
		body.Text = limitNoisemakerText(body.Text, noisemakerMaxLyricsLength)
		body.Context = limitNoisemakerText(body.Context, noisemakerMaxContextLength)
		body.Lang = limitNoisemakerText(body.Lang, 16)

		systemPrompt, userPrompt, maxTokens, err := noisemakerEnhancePrompts(body)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		model := ""
		if s.Cfg != nil {
			s.CfgMu.RLock()
			model = s.Cfg.LLM.Model
			s.CfgMu.RUnlock()
		}
		ctx, cancel := context.WithTimeout(r.Context(), noisemakerEnhanceTimeout)
		defer cancel()

		text, err := noisemakerRunEnhance(ctx, s.LLMClient, model, systemPrompt, userPrompt, maxTokens)
		if err != nil {
			if s.Logger != nil && !llm.IsContextError(err) {
				s.Logger.Warn("Noisemaker enhance failed", "kind", body.Kind, "error", err)
			}
			jsonError(w, "AI enhancement failed", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "text": text})
	}
}

// noisemakerRunEnhance executes the one-shot LLM call shared by the enhance
// endpoint and the automatic lyrics writer in the generate flow.
func noisemakerRunEnhance(ctx context.Context, client llm.ChatClient, model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.8,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", err
	}
	text := ""
	if len(resp.Choices) > 0 {
		text = strings.TrimSpace(resp.Choices[0].Message.Content)
	}
	if text == "" {
		return "", fmt.Errorf("llm returned an empty result")
	}
	return text, nil
}

// noisemakerEnhancePrompts builds system/user prompts per enhancement kind.
func noisemakerEnhancePrompts(body noisemakerEnhanceRequest) (string, string, int, error) {
	langRule := ""
	if strings.TrimSpace(body.Lang) != "" {
		langRule = fmt.Sprintf(" Write in the language with ISO code %q.", strings.TrimSpace(body.Lang))
	}
	switch body.Kind {
	case "idea":
		sys := "You are a creative music producer helping a user describe a song for an AI music generator. " +
			"Expand the user's rough idea into a vivid, concrete song description of 2-4 sentences: subject, mood, " +
			"story, tempo and instrumentation hints. Return ONLY the description, no explanations, no quotes, no markdown. " +
			"Keep the user's original language." + langRule
		usr := strings.TrimSpace(body.Text)
		if usr == "" {
			usr = "Invent one surprising, specific song idea on your own (pick an unusual theme or genre combination)."
			if body.Context != "" {
				usr += " It must fit this style: " + body.Context
			}
		} else if body.Context != "" {
			usr += "\n\nStyle direction: " + body.Context
		}
		return sys, usr, 320, nil
	case "style":
		sys := "You are a music expert. Turn the user's input into a concise, comma-separated list of music style tags " +
			"for an AI music generator (genre, subgenre, mood, era, instrumentation, vocals, tempo, production style). " +
			"Return ONLY the tag list, at most 12 tags, no explanations, no markdown. Answer in English unless the user input is clearly not English." + langRule
		usr := strings.TrimSpace(body.Text)
		if usr == "" {
			return "", "", 0, fmt.Errorf("text is required for style enhancement")
		}
		if body.Context != "" {
			usr += "\n\nSong idea for context: " + body.Context
		}
		return sys, usr, 200, nil
	case "lyrics":
		sys := "You are a professional songwriter. Write complete song lyrics with clear structure using bracketed " +
			"section markers like [Verse 1], [Chorus], [Bridge], [Outro]. Match the mood and story of the given idea " +
			"and style. Return ONLY the lyrics, no explanations. Keep the language of the user's input." + langRule
		usr := ""
		if strings.TrimSpace(body.Text) != "" {
			usr = "Continue, improve and complete these lyrics:\n" + strings.TrimSpace(body.Text)
		} else {
			usr = "Write full lyrics from scratch."
		}
		if body.Context != "" {
			usr += "\n\nSong idea / style:\n" + body.Context
		}
		return sys, usr, 1400, nil
	case "title":
		sys := "You are a music branding expert. Suggest a single short, memorable song title (1-5 words) fitting the " +
			"given song idea and style. Return ONLY the title, no quotes, no explanations. Keep the language of the user's input." + langRule
		usr := strings.TrimSpace(body.Context)
		if strings.TrimSpace(body.Text) != "" {
			usr = "Current title draft: " + strings.TrimSpace(body.Text) + "\n\n" + usr
		}
		if usr == "" {
			return "", "", 0, fmt.Errorf("context is required for title suggestions")
		}
		return sys, usr, 60, nil
	default:
		return "", "", 0, fmt.Errorf("unknown kind %q (expected idea, style, lyrics or title)", body.Kind)
	}
}

// handleNoisemakerGenerate returns POST /api/desktop/noisemaker/generate — runs
// music generation synchronously and optionally renders an AI cover afterwards.
func handleNoisemakerGenerate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var body noisemakerGenerateRequest
		if err := decodeDesktopJSON(w, r, &body, noisemakerGenerateBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		body.Prompt = limitNoisemakerText(body.Prompt, noisemakerMaxIdeaLength)
		body.Style = limitNoisemakerText(body.Style, noisemakerMaxStyleLength)
		body.Lyrics = limitNoisemakerText(body.Lyrics, noisemakerMaxLyricsLength)
		body.Title = limitNoisemakerText(body.Title, noisemakerMaxTitleLength)
		if body.Prompt == "" && body.Style == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "prompt or style is required"})
			return
		}

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		if !cfg.MusicConfigured() {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Music generation is not enabled. Enable it in Settings > Music Generation."})
			return
		}
		if !cfg.UsesLocalMusic() && s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("music_generation") {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Music generation blocked: daily budget exceeded."})
			return
		}

		composed := body.Prompt
		if body.Style != "" {
			if composed != "" {
				composed = body.Style + " — " + composed
			} else {
				composed = body.Style
			}
		}

		// MiniMax requires lyrics for non-instrumental songs (API error 2013).
		// Suno-style: write them automatically with the main LLM when missing.
		lyrics := body.Lyrics
		autoLyrics := false
		providerType := strings.ToLower(strings.TrimSpace(cfg.MusicGeneration.ProviderType))
		if !body.Instrumental && lyrics == "" && providerType == "minimax" {
			if s.LLMClient == nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "code": "lyrics_required", "message": "The music provider requires lyrics for songs with vocals. Add lyrics or enable instrumental."})
				return
			}
			model := ""
			s.CfgMu.RLock()
			model = s.Cfg.LLM.Model
			s.CfgMu.RUnlock()
			sys, usr, maxTokens, perr := noisemakerEnhancePrompts(noisemakerEnhanceRequest{
				Kind:    "lyrics",
				Context: strings.TrimSpace(body.Prompt + "\n" + body.Style),
				Lang:    body.Lang,
			})
			if perr == nil {
				llmCtx, cancel := context.WithTimeout(r.Context(), noisemakerEnhanceTimeout)
				generated, lerr := noisemakerRunEnhance(llmCtx, s.LLMClient, model, sys, usr, maxTokens)
				cancel()
				if lerr != nil {
					if s.Logger != nil && !llm.IsContextError(lerr) {
						s.Logger.Warn("Noisemaker auto-lyrics failed", "error", lerr)
					}
					w.WriteHeader(http.StatusBadGateway)
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "code": "lyrics_required", "message": "Could not write lyrics automatically. Add lyrics yourself or enable instrumental."})
					return
				}
				lyrics = limitNoisemakerText(generated, noisemakerMaxLyricsLength)
				autoLyrics = true
			}
		}

		if s.Logger != nil {
			s.Logger.Info("Noisemaker generation requested", "prompt_len", len(composed), "instrumental", body.Instrumental, "cover", body.Cover, "auto_lyrics", autoLyrics)
		}
		result := tools.GenerateMusicResult(r.Context(), cfg, s.MediaRegistryDB, s.Logger, tools.MusicGenParams{
			Prompt:          composed,
			Lyrics:          lyrics,
			Instrumental:    body.Instrumental,
			Title:           body.Title,
			DurationSeconds: body.DurationSeconds,
			BPM:             body.BPM,
			VocalLanguage:   body.VocalLanguage,
			Seed:            body.Seed,
		})
		if result.Status != "ok" {
			status := http.StatusBadGateway
			if result.Error == "acestep_busy" {
				status = http.StatusConflict
			}
			if result.Error == "lyrics_required" || strings.HasPrefix(result.Error, "invalid_") {
				status = http.StatusBadRequest
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "code": result.Error, "message": result.Error})
			return
		}
		if s.BudgetTracker != nil && result.CostEstimate > 0 {
			s.BudgetTracker.RecordCostForCategory("music_generation", result.CostEstimate)
		}

		coverURL := ""
		coverError := ""
		if body.Cover && cfg.ImageGeneration.Enabled && strings.TrimSpace(cfg.ImageGeneration.APIKey) != "" {
			title := result.Title
			if title == "" {
				title = body.Title
			}
			coverURL, coverError = s.noisemakerGenerateCover(cfg, title, body.Style, body.Prompt, result.MediaID)
			if coverError != "" && s.Logger != nil {
				s.Logger.Warn("Noisemaker cover generation failed", "error", coverError)
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "ok",
			"title":         result.Title,
			"filename":      result.Filename,
			"web_path":      result.WebPath,
			"duration_ms":   result.DurationMs,
			"provider":      result.Provider,
			"model":         result.Model,
			"format":        result.Format,
			"file_size":     result.FileSize,
			"media_id":      result.MediaID,
			"cost_estimate": result.CostEstimate,
			"daily_used":    tools.MusicCounterGet(),
			"lyrics":        lyrics,
			"auto_lyrics":   autoLyrics,
			"cover_url":     coverURL,
			"cover_error":   coverError,
		})
	}
}

// noisemakerGenerateCover renders a square album cover and links it to the track's
// media registry entry (source_image). Failures are non-fatal for the generation.
func (s *Server) noisemakerGenerateCover(cfg *config.Config, title, style, idea string, mediaID int64) (string, string) {
	ig := cfg.ImageGeneration
	coverTitle := strings.TrimSpace(title)
	if coverTitle == "" {
		coverTitle = "Untitled"
	}
	coverStyle := strings.TrimSpace(style)
	if coverStyle == "" {
		coverStyle = "music"
	}
	prompt := fmt.Sprintf("Square album cover art for a %s song titled %q. Bold, high-quality digital artwork, atmospheric, no text, no letters, no words, no typography.", coverStyle, coverTitle)
	if mood := limitNoisemakerText(idea, 200); mood != "" {
		prompt += " Mood: " + mood
	}

	genCfg := tools.ImageGenConfig{
		ProviderType: ig.ProviderType,
		BaseURL:      ig.BaseURL,
		APIKey:       ig.APIKey,
		Model:        ig.ResolvedModel,
		DataDir:      cfg.Directories.DataDir,
	}
	opts := tools.ImageGenOptions{
		Size:    ig.DefaultSize,
		Quality: ig.DefaultQuality,
		Style:   ig.DefaultStyle,
	}
	img, err := tools.GenerateImage(genCfg, prompt, opts)
	if err != nil {
		return "", err.Error()
	}
	if _, err := tools.SaveGeneratedImage(s.ImageGalleryDB, img); err != nil && s.Logger != nil {
		s.Logger.Warn("Noisemaker: failed to save cover to gallery", "error", err)
	}

	// Register the cover in the media registry (visible in the media browser)
	// and link it to the track via source_image.
	if s.MediaRegistryDB != nil {
		coverPath := filepath.Join(cfg.Directories.DataDir, "generated_images", img.Filename)
		fileSize := int64(0)
		if info, statErr := os.Stat(coverPath); statErr == nil {
			fileSize = info.Size()
		}
		fileHash, _ := tools.ComputeMediaFileHash(coverPath)
		if _, _, regErr := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{
			MediaType:  "image",
			SourceTool: "generate_image",
			Filename:   img.Filename,
			FilePath:   coverPath,
			WebPath:    img.WebPath,
			FileSize:   fileSize,
			Format:     strings.TrimPrefix(filepath.Ext(img.Filename), "."),
			Provider:   img.Provider,
			Model:      img.Model,
			Prompt:     prompt,
			Tags:       []string{"auto-generated", "cover", "noisemaker"},
			Hash:       fileHash,
		}); regErr != nil && s.Logger != nil {
			s.Logger.Warn("Noisemaker: failed to register cover in media registry", "error", regErr)
		}
		if mediaID > 0 {
			if _, err := s.MediaRegistryDB.Exec("UPDATE media_items SET source_image = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", img.WebPath, mediaID); err != nil && s.Logger != nil {
				s.Logger.Warn("Noisemaker: failed to link cover to track", "media_id", mediaID, "error", err)
			}
		}
	}
	return img.WebPath, ""
}

// handleNoisemakerTracks returns GET /api/desktop/noisemaker/tracks — the generated
// song library from the media registry, newest first (created_at DESC), paginated
// via ?limit=&offset= and searchable via ?q=.
func handleNoisemakerTracks(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if s.MediaRegistryDB == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "items": []interface{}{}, "total": 0, "registry_enabled": false})
			return
		}

		limit := noisemakerTracksDefaultLimit
		if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= noisemakerTracksMaxLimit {
			limit = v
		}
		offset := 0
		if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
			offset = v
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		var tags []string
		if fav := r.URL.Query().Get("favorites"); fav == "1" || fav == "true" {
			tags = []string{noisemakerFavoriteTag}
		}
		items, err := searchAllMediaForServerWithTags(s.MediaRegistryDB, query, "music", tags)
		if err != nil {
			s.Logger.Error("Noisemaker: failed to list tracks", "error", err)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to load tracks"})
			return
		}

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		items, total := filterDisplayableMediaItems(dataDir, items, limit, offset)

		tracks := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			tracks = append(tracks, noisemakerTrackJSON(item))
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":           "ok",
			"items":            tracks,
			"total":            total,
			"limit":            limit,
			"offset":           offset,
			"registry_enabled": true,
			"daily_used":       tools.MusicCounterGet(),
		})
	}
}

// handleNoisemakerTrackDelete handles DELETE /api/desktop/noisemaker/tracks/{id}.
// Only media items of type "music" can be removed through this route.
func handleNoisemakerTrackDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		s.CfgMu.RLock()
		readonly := s.Cfg.VirtualDesktop.ReadOnly
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()
		if readonly {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "read-only mode"})
			return
		}

		id, ok := noisemakerTrackIDFromPath(w, r)
		if !ok {
			return
		}
		if s.MediaRegistryDB == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "media registry is not available"})
			return
		}
		item, err := tools.GetMedia(s.MediaRegistryDB, id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "track not found"})
			return
		}
		if item.MediaType != "music" {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "only music tracks can be deleted here"})
			return
		}
		if err := s.deleteMediaItemByID(id, dataDir); err != nil {
			s.Logger.Error("Noisemaker: failed to delete track", "track_id", id, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to delete track"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Track deleted"})
	}
}

// noisemakerTrackIDFromPath parses the {id} segment of /api/desktop/noisemaker/tracks/{id}
// and writes a 400 response when it is missing or not a positive integer.
func noisemakerTrackIDFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	idStr := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/noisemaker/tracks/"), "/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "invalid track ID"})
		return 0, false
	}
	return id, true
}

// handleNoisemakerTrackItem routes /api/desktop/noisemaker/tracks/{id}:
// DELETE removes a song, PATCH updates its favorite flag.
func handleNoisemakerTrackItem(s *Server) http.HandlerFunc {
	del := handleNoisemakerTrackDelete(s)
	patch := handleNoisemakerTrackPatch(s)
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			del(w, r)
		case http.MethodPatch:
			patch(w, r)
		default:
			if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
				return
			}
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleNoisemakerTrackPatch handles PATCH /api/desktop/noisemaker/tracks/{id}
// with body {"favorite": true|false}. Only music items can be updated.
func handleNoisemakerTrackPatch(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPatch {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		s.CfgMu.RLock()
		readonly := s.Cfg.VirtualDesktop.ReadOnly
		s.CfgMu.RUnlock()
		if readonly {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "read-only mode"})
			return
		}
		id, ok := noisemakerTrackIDFromPath(w, r)
		if !ok {
			return
		}
		if s.MediaRegistryDB == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "media registry is not available"})
			return
		}
		var body noisemakerTrackPatchRequest
		if err := decodeDesktopJSON(w, r, &body, noisemakerPatchBodyLimit); err != nil || body.Favorite == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "favorite (true or false) is required"})
			return
		}
		item, err := tools.GetMedia(s.MediaRegistryDB, id)
		if err != nil || item.MediaType != "music" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "track not found"})
			return
		}
		mode := "remove"
		if *body.Favorite {
			mode = "add"
		}
		if err := tools.TagMedia(s.MediaRegistryDB, id, []string{noisemakerFavoriteTag}, mode); err != nil {
			s.Logger.Error("Noisemaker: failed to update favorite", "track_id", id, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to update favorite"})
			return
		}
		updated, err := tools.GetMedia(s.MediaRegistryDB, id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to reload track"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "track": noisemakerTrackJSON(*updated)})
	}
}

// limitNoisemakerText trims and caps user-supplied text.
func limitNoisemakerText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max > 0 && len(value) > max {
		value = strings.TrimSpace(value[:max])
	}
	return value
}
