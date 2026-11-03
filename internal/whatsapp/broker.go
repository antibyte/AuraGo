package whatsapp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/telegram"

	"go.mau.fi/whatsmeow/types"
)

// statusCoalesceWindow is the shortest gap between two status messages of one turn.
const statusCoalesceWindow = 10 * time.Second

// maxCapturedMedia bounds the audio files and the YouTube videos a broker keeps for one run, each
// list on its own. The model decides how often it calls the tools; the extra ones are dropped.
const maxCapturedMedia = 5

// Status prefixes, the same as TelegramBroker writes (internal/telegram/bot.go).
const (
	statusPrefix   = "⚙️ "
	progressPrefix = "⏳ "
	budgetWarning  = "⚠️ "
	budgetBlocked  = "🚫 "
)

var _ agent.FeedbackBroker = (*whatsAppBroker)(nil)

// whatsAppBroker implements agent.FeedbackBroker for one owner turn (TelegramBroker parity).
// It sends text events through the sender, whose owner guard also covers to: the pipeline
// takes it from ownerRecipients. It captures audio and YouTube events for the pipeline to
// deliver after the run. It logs nothing, so no message content reaches a log.
type whatsAppBroker struct {
	ctx        context.Context
	sender     *sender
	to         types.JID
	now        func() time.Time
	mu         sync.Mutex
	lastStatus time.Time
	audio      []telegram.CapturedAudio
	videos     []telegram.CapturedYouTubeVideo
}

// Send delivers the events a chat user needs and captures what the pipeline sends after
// the run.
//
//   - question_user, budget_warning and budget_blocked are delivered at once.
//   - tool_start, error_recovery, api_retry and progress are status updates: at most
//     one per statusCoalesceWindow, the others are dropped.
//   - audio (with a file_path, which only the tts action sets) and youtube_video (with a
//     url) are captured, at most maxCapturedMedia of each per run, as TelegramBroker does
//     (internal/telegram/bot.go:948-978). Nothing is sent during the run. send_audio and generate_music events carry web paths only and
//     are ignored.
//   - every other event is ignored.
func (b *whatsAppBroker) Send(event, message string) {
	switch event {
	case "question_user":
		b.deliver(message, "question")
	case "audio":
		var audio struct {
			FilePath string `json:"file_path"`
			Title    string `json:"title"`
			MimeType string `json:"mime_type"`
			Filename string `json:"filename"`
		}
		if json.Unmarshal([]byte(message), &audio) == nil && audio.FilePath != "" {
			b.mu.Lock()
			if len(b.audio) < maxCapturedMedia {
				b.audio = append(b.audio, telegram.CapturedAudio{
					FilePath: audio.FilePath, Title: audio.Title, MimeType: audio.MimeType, Filename: audio.Filename,
				})
			}
			b.mu.Unlock()
		}
	case "youtube_video":
		var video struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			VideoID string `json:"video_id"`
		}
		if json.Unmarshal([]byte(message), &video) == nil && video.URL != "" {
			b.mu.Lock()
			if len(b.videos) < maxCapturedMedia {
				b.videos = append(b.videos, telegram.CapturedYouTubeVideo{URL: video.URL, Title: video.Title, VideoID: video.VideoID})
			}
			b.mu.Unlock()
		}
	case "tool_start", "error_recovery", "api_retry", "progress":
		text := statusPrefix + strings.ToUpper(event) + ": " + message
		if event == "progress" {
			text = progressPrefix + message
		}
		if b.takeStatusSlot() {
			b.deliver(text, "status update")
		}
	case "budget_warning":
		b.deliver(budgetWarning+message, "budget notice")
	case "budget_blocked":
		b.deliver(budgetBlocked+message, "budget notice")
	}
}

// captured returns copies of the audio files and YouTube videos the run produced.
func (b *whatsAppBroker) captured() ([]telegram.CapturedAudio, []telegram.CapturedYouTubeVideo) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.audio), slices.Clone(b.videos)
}

// takeStatusSlot reports whether a status update may go out now, and claims the slot.
func (b *whatsAppBroker) takeStatusSlot() bool {
	now := time.Now
	if b.now != nil {
		now = b.now
	}
	t := now()
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.lastStatus.IsZero() && t.Sub(b.lastStatus) < statusCoalesceWindow {
		return false
	}
	b.lastStatus = t
	return true
}

// deliver hands text to the sender, which counts and logs a failed delivery.
func (b *whatsAppBroker) deliver(text, label string) {
	_ = b.sender.deliverText(b.ctx, b.to, text, label)
}

func (b *whatsAppBroker) SendJSON(string)                                             {}
func (b *whatsAppBroker) SendLLMStreamDelta(string, string, string, int, string)      {}
func (b *whatsAppBroker) SendLLMStreamDone(string)                                    {}
func (b *whatsAppBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}
func (b *whatsAppBroker) SendThinkingBlock(string, string, string)                    {}
