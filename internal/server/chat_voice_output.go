package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

var (
	chatVoiceOutputSynthesize       = tools.TTSSynthesize
	chatVoiceDoneTagPattern         = regexp.MustCompile(`(?i)<done\s*/?>`)
	chatVoiceFencePattern           = regexp.MustCompile("(?s)```.*?```")
	chatVoiceMarkdownLinkPattern    = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	chatVoiceLeadingListItemPattern = regexp.MustCompile(`^\s*(?:[-*+]\s+|\d+[.)]\s+)`)
)

const (
	chatVoiceDirectRuneLimit  = 220
	chatVoiceSummaryRuneLimit = 260
)

type chatVoiceOutputTrackingBroker struct {
	agent.FeedbackBroker

	mu       sync.Mutex
	ttsAudio bool
}

func newChatVoiceOutputTrackingBroker(base agent.FeedbackBroker) *chatVoiceOutputTrackingBroker {
	if base == nil {
		base = agent.NoopBroker{}
	}
	return &chatVoiceOutputTrackingBroker{FeedbackBroker: base}
}

func (b *chatVoiceOutputTrackingBroker) Send(event, message string) {
	if chatVoiceOutputIsTTSAudioEvent(event, message) {
		b.markTTSAudio()
	}
	b.FeedbackBroker.Send(event, message)
}

func (b *chatVoiceOutputTrackingBroker) SendJSON(jsonStr string) {
	if chatVoiceOutputJSONHasTTSAudio(jsonStr) {
		b.markTTSAudio()
	}
	b.FeedbackBroker.SendJSON(jsonStr)
}

func (b *chatVoiceOutputTrackingBroker) SendTyped(eventType string, payload interface{}) bool {
	if typed, ok := b.FeedbackBroker.(agent.TypedFeedbackBroker); ok {
		return typed.SendTyped(eventType, payload)
	}
	return false
}

func (b *chatVoiceOutputTrackingBroker) markTTSAudio() {
	b.mu.Lock()
	b.ttsAudio = true
	b.mu.Unlock()
}

func (b *chatVoiceOutputTrackingBroker) hasTTSAudio() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ttsAudio
}

func maybeEmitChatVoiceOutputFallback(cfg *config.Config, logger *slog.Logger, runCfg agent.RunConfig, broker *chatVoiceOutputTrackingBroker, content string) {
	if cfg == nil || broker == nil || broker.hasTTSAudio() {
		return
	}
	if !runCfg.VoiceOutputActive || runCfg.MessageSource != "web_chat" || !chatVoiceOutputTTSConfigured(cfg) {
		return
	}

	text := chatVoiceOutputText(content)
	if text == "" {
		return
	}

	ttsCfg := buildChatVoiceOutputTTSConfig(cfg, "")
	filename, err := chatVoiceOutputSynthesize(ttsCfg, text)
	if err != nil {
		if logger != nil {
			logger.Warn("Chat voice output fallback failed", "error", err)
		}
		return
	}
	if filename == "" {
		return
	}

	payload, err := json.Marshal(map[string]interface{}{
		"path":      "/tts/" + filename,
		"title":     "TTS Audio",
		"mime_type": chatVoiceAudioMIMEType(filename),
		"filename":  filename,
		"file_path": filepath.Join(cfg.Directories.DataDir, "tts", filename),
		"autoplay":  true,
	})
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to encode chat voice output fallback payload", "error", err)
		}
		return
	}
	broker.Send("audio", string(payload))
}

func chatVoiceOutputText(content string) string {
	text := chatVoiceCleanText(content)
	if text == "" {
		return ""
	}

	spoken := chatVoiceCollapseSpeech(text)
	if len([]rune(spoken)) <= chatVoiceDirectRuneLimit {
		return spoken
	}
	if summary := chatVoiceStructuredStatusSummary(text); summary != "" {
		return chatVoiceLimitRunes(summary, chatVoiceSummaryRuneLimit)
	}
	return chatVoiceLimitRunes(chatVoiceFallbackSummary(spoken), chatVoiceSummaryRuneLimit)
}

func chatVoiceCleanText(content string) string {
	text := security.StripThinkingTags(security.Scrub(content))
	text = chatVoiceDoneTagPattern.ReplaceAllString(text, "")
	text = chatVoiceFencePattern.ReplaceAllString(text, " ")
	text = chatVoiceMarkdownLinkPattern.ReplaceAllString(text, "$1")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	replacements := []struct {
		old string
		new string
	}{
		{"**", ""},
		{"__", ""},
		{"`", ""},
		{"#", ""},
		{">", ""},
		{"*", ""},
	}
	for _, r := range replacements {
		text = strings.ReplaceAll(text, r.old, r.new)
	}
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(chatVoiceLeadingListItemPattern.ReplaceAllString(line, ""))
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func chatVoiceCollapseSpeech(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func chatVoiceStructuredStatusSummary(text string) string {
	spoken := chatVoiceCollapseSpeech(text)
	lower := strings.ToLower(spoken)
	if !chatVoiceLooksLikeStatus(lower) {
		return ""
	}

	parts := make([]string, 0, 3)
	if chatVoiceContainsAny(lower, "erledigt", "fertig", "completed", "done") {
		parts = append(parts, "Ein Teil ist erledigt")
	}
	if chatVoiceBuildStillRunning(lower) {
		parts = append(parts, "der Build l\u00e4uft noch und dauert einen Moment")
	} else if chatVoiceContainsAny(lower, "blockiert", "blocked") {
		parts = append(parts, "ein Punkt ist noch blockiert")
	} else if chatVoiceContainsAny(lower, "in arbeit", "l\u00e4uft", "laeuft", "running", "in progress") {
		parts = append(parts, "es l\u00e4uft noch etwas")
	}
	if len(parts) == 0 {
		return ""
	}

	prefix := ""
	if chatVoiceContainsAny(lower, "stand jetzt", "status", "stand:") {
		prefix = "Stand jetzt: "
	}
	return prefix + strings.Join(parts, "; ") + ". " + chatVoiceDetailsSuffix(lower)
}

func chatVoiceLooksLikeStatus(lower string) bool {
	return chatVoiceContainsAny(lower,
		"stand jetzt", "status", "erledigt", "fertig", "completed", "done",
		"in arbeit", "blockiert", "blocked", "build", "gebaut", "running", "in progress",
	)
}

func chatVoiceBuildStillRunning(lower string) bool {
	return chatVoiceContainsAny(lower, "build", "gebaut", "docker-image") &&
		chatVoiceContainsAny(lower, "l\u00e4uft", "laeuft", "gerade", "hintergrund", "in arbeit", "running", "still")
}

func chatVoiceFallbackSummary(spoken string) string {
	lower := strings.ToLower(spoken)
	cut := len(spoken)
	for _, sep := range []string{". ", "! ", "? "} {
		if idx := strings.Index(spoken, sep); idx >= 0 && idx+1 < cut {
			cut = idx + 1
		}
	}
	first := strings.TrimSpace(spoken[:cut])
	if len([]rune(first)) > 180 {
		first = chatVoiceLimitRunes(first, 180)
	}
	if first == "" {
		return chatVoiceDetailsSuffix(lower)
	}
	return strings.TrimRight(first, ".!?") + ". " + chatVoiceDetailsSuffix(lower)
}

func chatVoiceDetailsSuffix(lower string) string {
	if chatVoiceLooksGerman(lower) {
		return "Details stehen im Chat."
	}
	return "Details are in the chat."
}

func chatVoiceLooksGerman(lower string) bool {
	return chatVoiceContainsAny(lower,
		"erledigt", "fertig", "in arbeit", "blockiert", "l\u00e4uft", "laeuft",
		"gepr\u00fcft", "geprueft", "n\u00e4chstes", "naechstes", "sobald", "wenn du",
	)
}

func chatVoiceContainsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func chatVoiceLimitRunes(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if limit <= 0 || len(runes) <= limit {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:limit]))
}

func buildChatVoiceOutputTTSConfig(cfg *config.Config, language string) tools.TTSConfig {
	provider := cfg.TTS.Provider
	if provider == "" && cfg.TTS.Piper.Enabled {
		provider = "piper"
	}

	ttsCfg := tools.TTSConfig{
		Provider:            provider,
		Language:            language,
		DataDir:             cfg.Directories.DataDir,
		CacheRetentionHours: cfg.TTS.CacheRetentionHours,
		CacheMaxFiles:       cfg.TTS.CacheMaxFiles,
	}
	if ttsCfg.Language == "" {
		ttsCfg.Language = cfg.TTS.Language
	}
	ttsCfg.ElevenLabs.APIKey = cfg.TTS.ElevenLabs.APIKey
	ttsCfg.ElevenLabs.VoiceID = cfg.TTS.ElevenLabs.VoiceID
	ttsCfg.ElevenLabs.ModelID = cfg.TTS.ElevenLabs.ModelID
	ttsCfg.MiniMax.APIKey = cfg.TTS.MiniMax.APIKey
	ttsCfg.MiniMax.VoiceID = cfg.TTS.MiniMax.VoiceID
	ttsCfg.MiniMax.ModelID = cfg.TTS.MiniMax.ModelID
	ttsCfg.MiniMax.Speed = cfg.TTS.MiniMax.Speed
	ttsCfg.Piper.Port = cfg.TTS.Piper.ContainerPort
	ttsCfg.Piper.Voice = cfg.TTS.Piper.Voice
	ttsCfg.Piper.SpeakerID = cfg.TTS.Piper.SpeakerID
	ttsCfg.Supertonic.URL = cfg.TTS.Supertonic.URL
	ttsCfg.Supertonic.Model = cfg.TTS.Supertonic.Model
	ttsCfg.Supertonic.Voice = cfg.TTS.Supertonic.Voice
	ttsCfg.Supertonic.Speed = cfg.TTS.Supertonic.Speed
	ttsCfg.Supertonic.Steps = cfg.TTS.Supertonic.Steps
	ttsCfg.Supertonic.ResponseFormat = cfg.TTS.Supertonic.ResponseFormat
	return ttsCfg
}

func chatVoiceOutputTTSConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.TTS.Provider))
	if provider == "" && cfg.TTS.Piper.Enabled {
		provider = "piper"
	}

	switch provider {
	case "google":
		return true
	case "elevenlabs":
		return strings.TrimSpace(cfg.TTS.ElevenLabs.APIKey) != ""
	case "minimax":
		return strings.TrimSpace(cfg.TTS.MiniMax.APIKey) != ""
	case "piper":
		return cfg.TTS.Piper.Enabled
	case "supertonic":
		return strings.TrimSpace(cfg.TTS.Supertonic.URL) != ""
	default:
		return false
	}
}

func chatVoiceOutputIsTTSAudioEvent(event, message string) bool {
	if event != "audio" {
		return false
	}
	var payload struct {
		Path     string `json:"path"`
		Filename string `json:"filename"`
		Title    string `json:"title"`
	}
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return false
	}
	return strings.HasPrefix(payload.Path, "/tts/") ||
		strings.HasPrefix(payload.Filename, "tts/") ||
		strings.Contains(strings.ToLower(payload.Title), "tts")
}

func chatVoiceOutputJSONHasTTSAudio(jsonStr string) bool {
	var envelope struct {
		Event  string `json:"event"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		return false
	}
	return chatVoiceOutputIsTTSAudioEvent(envelope.Event, envelope.Detail)
}

func chatVoiceAudioMIMEType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	default:
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
		if ext == "" {
			return "application/octet-stream"
		}
		return fmt.Sprintf("audio/%s", ext)
	}
}
