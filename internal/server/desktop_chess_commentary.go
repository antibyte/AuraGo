package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/phonon"
	"aurago/internal/tools"
)

const (
	desktopChessCommentaryMaxBodyBytes  = int64(2048)
	desktopChessCommentaryMaxExcludes   = 64
	desktopChessCommentarySlotWait      = 2500 * time.Millisecond
	desktopChessCommentarySynthTimeout  = 25 * time.Second
	desktopChessCommentaryMaxAudioBytes = 8 << 20
	desktopChessCommentaryCacheEntries  = 48
	desktopChessCommentaryCacheBytes    = 12 << 20
)

type desktopChessCommentarySynthFunc func(ctx context.Context, cfg tools.TTSConfig, text string) ([]byte, string, error)

type desktopChessCommentaryRequest struct {
	Category string `json:"category"`
	Piece    string `json:"piece,omitempty"`
	Square   string `json:"square,omitempty"`
	Lang     string `json:"lang,omitempty"`
	Exclude  []int  `json:"exclude,omitempty"`
	Seed     uint32 `json:"seed,omitempty"`
}

type desktopChessCommentaryResponse struct {
	Category         string `json:"category"`
	Variant          int    `json:"variant"`
	Language         string `json:"language"`
	Text             string `json:"text"`
	Audio            string `json:"audio,omitempty"`
	MIMEType         string `json:"mime_type,omitempty"`
	AudioUnavailable bool   `json:"audio_unavailable,omitempty"`
}

type chessCommentaryCacheItem struct {
	key  string
	data []byte
	mime string
}

type chessCommentaryAudioCache struct {
	mu    sync.Mutex
	items []chessCommentaryCacheItem
	bytes int
}

func (c *chessCommentaryAudioCache) get(key string) ([]byte, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for idx, item := range c.items {
		if item.key != key {
			continue
		}
		// Move to end (most recently used).
		if idx != len(c.items)-1 {
			copy(c.items[idx:], c.items[idx+1:])
			c.items[len(c.items)-1] = item
		}
		return item.data, item.mime, true
	}
	return nil, "", false
}

func (c *chessCommentaryAudioCache) put(key string, data []byte, mime string) {
	if len(data) == 0 || len(data) > desktopChessCommentaryMaxAudioBytes {
		return
	}
	owned := append([]byte(nil), data...)
	c.mu.Lock()
	defer c.mu.Unlock()
	for idx, item := range c.items {
		if item.key == key {
			c.bytes -= len(item.data)
			c.items = append(c.items[:idx], c.items[idx+1:]...)
			break
		}
	}
	for len(c.items) > 0 && (len(c.items) >= desktopChessCommentaryCacheEntries || c.bytes+len(owned) > desktopChessCommentaryCacheBytes) {
		c.bytes -= len(c.items[0].data)
		c.items = c.items[1:]
	}
	if c.bytes+len(owned) > desktopChessCommentaryCacheBytes {
		return
	}
	c.items = append(c.items, chessCommentaryCacheItem{key: key, data: owned, mime: mime})
	c.bytes += len(owned)
}

func handleDesktopChessCommentary(s *Server) http.HandlerFunc {
	return newDesktopChessCommentaryHandler(s, nil)
}

func newDesktopChessCommentaryHandler(s *Server, synthOverride desktopChessCommentarySynthFunc) http.HandlerFunc {
	slot := make(chan struct{}, 1)
	cache := &chessCommentaryAudioCache{}
	synth := tools.TTSSynthesizeInMemoryContext
	if synthOverride != nil {
		synth = synthOverride
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !requireDesktopOperation(s, w, r, desktopScopeWrite, desktopRead) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, desktopChessCommentaryMaxBodyBytes)
		var req desktopChessCommentaryRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if dec.More() {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := validateDesktopChessCommentaryRequest(&req); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var cfg *config.Config
		if s != nil {
			cfg = s.ConfigSnapshot()
			if cfg == nil && s.Cfg != nil {
				s.CfgMu.RLock()
				cp := *s.Cfg
				cfg = &cp
				s.CfgMu.RUnlock()
			}
		}

		speechLang := desktopChessCommentaryLanguage(cfg, req.Lang)
		variant, text := pickDesktopChessCommentaryLine(speechLang, req.Category, req.Piece, req.Square, req.Exclude, req.Seed)
		ttsCfg := desktopChessPhononTTSConfig(cfg, speechLang)
		cacheKey := speechLang + "|" + ttsCfg.Phonon.VoiceID + "|" + text

		resp := desktopChessCommentaryResponse{
			Category: req.Category,
			Variant:  variant,
			Language: speechLang,
			Text:     text,
		}

		if cachedData, cachedMIME, ok := cache.get(cacheKey); ok {
			resp.Audio = base64.StdEncoding.EncodeToString(cachedData)
			resp.MIMEType = cachedMIME
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		waitTimer := time.NewTimer(desktopChessCommentarySlotWait)
		select {
		case slot <- struct{}{}:
			waitTimer.Stop()
			defer func() { <-slot }()
		case <-r.Context().Done():
			waitTimer.Stop()
			return
		case <-waitTimer.C:
			jsonError(w, "Chess commentary busy", http.StatusTooManyRequests)
			return
		}

		if cachedData, cachedMIME, ok := cache.get(cacheKey); ok {
			resp.Audio = base64.StdEncoding.EncodeToString(cachedData)
			resp.MIMEType = cachedMIME
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), desktopChessCommentarySynthTimeout)
		defer cancel()
		data, ext, err := synth(ctx, ttsCfg, text)
		if r.Context().Err() != nil {
			return
		}
		if err != nil || len(data) == 0 || len(data) > desktopChessCommentaryMaxAudioBytes {
			resp.AudioUnavailable = true
		} else {
			mime := chatVoiceAudioMIMEType("commentary." + strings.TrimPrefix(ext, "."))
			cache.put(cacheKey, data, mime)
			resp.Audio = base64.StdEncoding.EncodeToString(data)
			resp.MIMEType = mime
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func validateDesktopChessCommentaryRequest(req *desktopChessCommentaryRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	req.Category = strings.ToLower(strings.TrimSpace(req.Category))
	if !isDesktopChessCommentaryCategory(req.Category) {
		return fmt.Errorf("invalid commentary category")
	}
	req.Piece = strings.ToLower(strings.TrimSpace(req.Piece))
	switch req.Piece {
	case "", "p", "n", "b", "r", "q", "k":
	default:
		return fmt.Errorf("invalid piece code")
	}
	req.Square = strings.ToLower(strings.TrimSpace(req.Square))
	if req.Square != "" {
		if len(req.Square) != 2 || req.Square[0] < 'a' || req.Square[0] > 'h' || req.Square[1] < '1' || req.Square[1] > '8' {
			return fmt.Errorf("invalid board square")
		}
	}
	req.Lang = strings.ToLower(limitDesktopChessText(strings.TrimSpace(req.Lang), 16))
	if len(req.Exclude) > desktopChessCommentaryMaxExcludes {
		return fmt.Errorf("exclude has too many entries")
	}
	for _, idx := range req.Exclude {
		if idx < 0 || idx > 256 {
			return fmt.Errorf("invalid exclude index")
		}
	}
	return nil
}

func desktopChessCommentaryLanguage(cfg *config.Config, requested string) string {
	candidates := []string{requested}
	if cfg != nil {
		// Respect an explicit user-configured TTS language first, otherwise prefer UI/request language.
		if explicit := strings.TrimSpace(cfg.TTS.Language); explicit != "" && !strings.EqualFold(explicit, "auto") {
			candidates = append([]string{explicit}, candidates...)
		}
		candidates = append(candidates, cfg.Server.UILanguage, cfg.Agent.SystemLanguage)
	}
	for _, cand := range candidates {
		if lang := phonon.Language(cand); lang != "" {
			return lang
		}
	}
	return "en"
}

func desktopChessPhononTTSConfig(cfg *config.Config, speechLang string) tools.TTSConfig {
	var persona, dataDir, configuredVoice string
	if cfg != nil {
		persona = cfg.Personality.CorePersonality
		dataDir = cfg.Directories.DataDir
		configuredVoice = strings.TrimSpace(cfg.TTS.Phonon.VoiceID)
	}
	voiceID := ""
	if configuredVoice != "" {
		if v, ok := phonon.Lookup(configuredVoice); ok && v.Language == speechLang {
			voiceID = v.ID
		}
	}
	if voiceID == "" {
		voiceID = phonon.DefaultVoice(speechLang, persona)
	}
	out := tools.TTSConfig{
		Provider: "phonon",
		Language: speechLang,
		DataDir:  dataDir,
	}
	out.Phonon.Root = phonon.Root()
	out.Phonon.VoiceID = voiceID
	out.Phonon.Persona = persona
	return out
}

func pickDesktopChessCommentaryLine(lang, category, piece, square string, exclude []int, seed uint32) (int, string) {
	lines := desktopChessCommentaryLines(lang, category)
	if len(lines) == 0 {
		return 0, ""
	}
	excluded := make(map[int]struct{}, len(exclude))
	for _, idx := range exclude {
		excluded[idx] = struct{}{}
	}
	available := make([]int, 0, len(lines))
	for idx := range lines {
		if _, skip := excluded[idx]; !skip {
			available = append(available, idx)
		}
	}
	if len(available) == 0 {
		last := -1
		if len(exclude) > 0 && len(lines) > 1 {
			last = exclude[len(exclude)-1]
		}
		for idx := range lines {
			if idx != last {
				available = append(available, idx)
			}
		}
		if len(available) == 0 {
			available = append(available, 0)
		}
	}
	var pick int
	if seed != 0 {
		pick = available[int(seed%uint32(len(available)))]
	} else {
		pick = available[rand.IntN(len(available))]
	}
	sq := square
	if sq == "" {
		sq = "d4"
	}
	pieceName := desktopChessPieceName(lang, piece)
	r := strings.NewReplacer("{square}", sq, "{piece}", pieceName)
	return pick, r.Replace(lines[pick])
}
