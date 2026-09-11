package server

import (
	"context"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/desktop"
	"aurago/internal/speechlab"
	"aurago/internal/tools"
)

var systemWorldSentence = regexp.MustCompile(`[^.!?。！？\n]+[.!?。！？]?`)

// One transient phrase per request, with the same effective TTS selection as chat.
// No chat/agent run, file cache, access-count update or publication through SSE.
func handleSystemWorldVoice(s *Server) http.HandlerFunc {
	var mu sync.Mutex
	var next time.Time
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// Scoped desktop readers must not gain access to the owner's global memory/chat.
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		cfg := s.ConfigSnapshot()
		if !cfg.VirtualDesktop.Enabled || !chatVoiceOutputTTSConfigured(cfg) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if !mu.TryLock() {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		defer mu.Unlock()
		if time.Now().Before(next) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next = time.Now().Add(12 * time.Second)
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		text := systemWorldVoiceText(ctx, s)
		if ctx.Err() != nil {
			return
		}
		if text == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		tts := buildChatVoiceOutputTTSConfig(cfg, "", s.SpeechLab)
		if isSpeechLabTTSProvider(tts.Provider) {
			client := tts.SpeechLab.Client
			if client == nil {
				var err error
				client, err = speechlab.NewClient(cfg.SpeechLab)
				if err != nil {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
			}
			ready, err := client.Require(ctx, false, true)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			tts.SpeechLab.Client, tts.SpeechLab.ExpectedTTSID, tts.SpeechLab.Voice = client, ready.TTSID, ready.Voice
		}
		data, ext, err := tools.TTSSynthesizeInMemoryContext(ctx, tts, text)
		if ctx.Err() != nil {
			return
		}
		if err != nil || len(data) > 8<<20 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", chatVoiceAudioMIMEType("voice."+strings.TrimPrefix(ext, ".")))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
	}
}

func systemWorldVoiceText(ctx context.Context, s *Server) string {
	excerpts := systemWorldSampleExcerpts(ctx, s, 1, 180)
	if len(excerpts) == 0 {
		return ""
	}
	return excerpts[0]
}

// systemWorldSampleExcerpts returns up to limit distinct short excerpts from the same
// read-only memory sources as the tower voice. The result is ambience, never a search.
func systemWorldSampleExcerpts(ctx context.Context, s *Server, limit, maxRunes int) []string {
	var out []string
	seen := map[string]bool{}
	// Shuffle independent sources, not relevance: unrelated fragments are the desired ambience.
	for _, source := range rand.Perm(5) {
		if ctx.Err() != nil || len(out) >= limit {
			break
		}
		var texts []string
		if source == 3 {
			if svc, _, err := s.getDesktopService(ctx); err == nil {
				result, err := svc.SearchNotes(ctx, desktop.NotesQuery{Limit: 1})
				if err == nil && result.Total > 0 {
					result, err = svc.SearchNotes(ctx, desktop.NotesQuery{Limit: 1, Offset: rand.IntN(min(result.Total, 100001))})
					if err == nil && len(result.Notes) > 0 {
						if note, err := svc.ReadNote(ctx, result.Notes[0].Path); err == nil {
							texts = append(texts, note.Content)
						}
					}
				}
			}
		} else if s.ShortTermMem != nil {
			switch source {
			case 0:
				facts, _ := s.ShortTermMem.GetCoreMemoryFacts()
				for _, fact := range facts {
					texts = append(texts, fact.Fact)
				}
			case 1:
				if s.LongTermMem != nil {
					count, _ := s.ShortTermMem.GetAllMemoryMetaCount()
					if count > 0 {
						metas, _ := s.ShortTermMem.GetAllMemoryMeta(1, rand.IntN(count))
						if len(metas) > 0 && metas[0].ArchivedAt == "" {
							// File-index collections and tool guides are not personal memories.
							text, _ := s.LongTermMem.GetByIDFromCollection(metas[0].DocID, "aurago_memories")
							texts = append(texts, text)
						}
					}
				}
			case 2:
				notes, _ := s.ShortTermMem.ListNotes("", -1)
				for _, note := range notes {
					texts = append(texts, note.Content)
				}
			case 4:
				texts, _ = s.ShortTermMem.SampleVisibleChatText(ctx)
			}
		}
		for _, i := range rand.Perm(len(texts)) {
			if len(out) >= limit {
				break
			}
			if text := systemWorldExcerpt(texts[i], maxRunes); text != "" && !seen[text] {
				seen[text] = true
				out = append(out, text)
			}
		}
	}
	return out
}

func systemWorldVoiceExcerpt(content string) string {
	return systemWorldExcerpt(content, 180)
}

func systemWorldExcerpt(content string, maxRunes int) string {
	if len(content) > 128<<10 || !utf8.ValidString(content) {
		return ""
	}
	// Existing chat cleanup scrubs registered secrets, thinking blocks and fenced code.
	text := chatVoiceCleanText(stripAgodeskAttachmentBlock(content))
	parts := systemWorldSentence.FindAllString(text, -1)
	for _, i := range rand.Perm(len(parts)) {
		phrase := strings.Join(strings.Fields(parts[i]), " ")
		if utf8.RuneCountInString(phrase) < 12 || strings.Contains(strings.ToLower(phrase), "[redacted") {
			continue
		}
		return chatVoiceLimitRunes(phrase, maxRunes)
	}
	return ""
}
