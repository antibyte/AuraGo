package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/discord"
	"aurago/internal/i18n"
	"aurago/internal/persondetect"
	"aurago/internal/security"
	"aurago/internal/telegram"
	"aurago/internal/telnyx"
	"aurago/internal/tools"
)

const (
	// personAlertMaxInFlight bounds the person detection messages that have not returned,
	// per channel, like flowNotifyMaxInFlight for flow failure notifications.
	personAlertMaxInFlight = 4
	// personAlertPhotoName is the file name of the alert photo in Telegram and Discord.
	personAlertPhotoName = "person-detection.jpg"
)

// personAlertChannels are the channels a camera's message action may name (C5).
var personAlertChannels = map[string]bool{
	"ntfy": true, "pushover": true, "telegram": true, "discord": true,
	"push": true, "telnyx": true, "cyd": true, "all": true,
}

// The senders of person detection messages; tests replace them.
var (
	personAlertSendNotification = tools.SendNotification
	personAlertTelegramPhoto    = telegram.SendPhoto
	personAlertDiscordImage     = discord.SendDiscordImage
)

// personAlertURL matches a URL in a channel's error detail (see personAlertDetail).
var personAlertURL = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s"'<>]+`)

// personAlertSlots holds the send slots of person detection messages per channel. The zero
// value is ready to use.
type personAlertSlots struct {
	mu       sync.Mutex
	inFlight map[string]int
}

// acquire takes one of the personAlertMaxInFlight slots of channel; false when all are busy.
func (p *personAlertSlots) acquire(channel string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inFlight[channel] >= personAlertMaxInFlight {
		return false
	}
	if p.inFlight == nil {
		p.inFlight = map[string]int{}
	}
	p.inFlight[channel]++
	return true
}

// release gives back a slot of channel that acquire took.
func (p *personAlertSlots) release(channel string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inFlight[channel] <= 1 {
		delete(p.inFlight, channel)
		return
	}
	p.inFlight[channel]--
}

// busy returns how many sends of channel hold a slot.
func (p *personAlertSlots) busy(channel string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.inFlight[channel]
}

func (s *Server) personAlertLogger() *slog.Logger {
	if s != nil && s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// sendPersonAlertMessage is the Message sink of the person detection dispatcher. It reads
// the configuration snapshot when it sends, so a dispatcher built before Start follows
// every reload. The send runs on a goroutine of its own that holds one of the channel's
// slots until the send returns, even after ctx ended: tools.SendNotification takes no
// context and its web push channel can stall for good (see sendFlowNotification), so a
// channel whose slots are all busy drops further messages ("busy") instead of piling up
// goroutines.
func (s *Server) sendPersonAlertMessage(ctx context.Context, channel, title, body string, photo []byte) error {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return persondetect.Skipped("channel unavailable")
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "all"
	}
	if !personAlertChannels[channel] {
		return persondetect.Skipped("channel unavailable")
	}
	logger := s.personAlertLogger()
	if !s.personAlertSlots.acquire(channel) {
		logger.Warn("Person detection message dropped; earlier messages are still being sent",
			"channel", channel, "in_flight", personAlertMaxInFlight)
		return errors.New("busy")
	}
	done := make(chan error, 1)
	go func() {
		defer s.personAlertSlots.release(channel)
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Person detection message send panicked", "channel", channel, "panic", fmt.Sprint(r))
				done <- errors.New("internal error")
			}
		}()
		done <- s.deliverPersonAlert(ctx, cfg, logger, channel, title, body, photo)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// deliverPersonAlert sends one message. telegram and discord send the photo with title and
// body as caption and fall back to text when the photo fails (refused for its size or
// format, for example); all sends the text to every enabled channel and the photo,
// captioned with the title, to Telegram and Discord where they are configured; the other
// channels are text only. Once ctx has ended no further send starts: the dispatcher has
// reported the action as a timeout, and a late message would contradict that.
func (s *Server) deliverPersonAlert(ctx context.Context, cfg *config.Config, logger *slog.Logger, channel, title, body string, photo []byte) error {
	caption := title + "\n" + body
	switch channel {
	case "telegram":
		if !personAlertTelegramReady(cfg) {
			return persondetect.Skipped("channel unavailable")
		}
		if len(photo) > 0 {
			err := personAlertTelegramPhoto(ctx, cfg, personAlertPhotoName, photo, caption)
			if err == nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			logger.Warn("Person detection photo could not be sent to Telegram; sending text", "error", err)
		}
		return personAlertOutcome(personAlertSendNotification(cfg, logger, "telegram", title, body, "high", nil))
	case "discord":
		if !personAlertDiscordReady(cfg) {
			return persondetect.Skipped("channel unavailable")
		}
		if len(photo) > 0 {
			err := sendPersonAlertDiscordPhoto(cfg, logger, photo, caption)
			if err == nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			logger.Warn("Person detection photo could not be sent to Discord; sending text", "error", err)
		}
		return personAlertOutcome(personAlertSendNotification(cfg, logger, "discord", title, body, "high", personAlertDiscordSender(cfg, logger)))
	case "all":
		err := personAlertOutcome(personAlertSendNotification(cfg, logger, "all", title, body, "high",
			personAlertDiscordSender(cfg, logger), personAlertTelnyxSender(ctx, cfg, logger)))
		if len(photo) > 0 {
			if personAlertTelegramReady(cfg) && ctx.Err() == nil {
				if perr := personAlertTelegramPhoto(ctx, cfg, personAlertPhotoName, photo, title); perr != nil {
					logger.Warn("Person detection photo could not be sent to Telegram", "error", perr)
				}
			}
			if personAlertDiscordReady(cfg) && ctx.Err() == nil {
				if perr := sendPersonAlertDiscordPhoto(cfg, logger, photo, title); perr != nil {
					logger.Warn("Person detection photo could not be sent to Discord", "error", perr)
				}
			}
		}
		return err
	default:
		return personAlertOutcome(personAlertSendNotification(cfg, logger, channel, title, body, "high", nil, personAlertTelnyxSender(ctx, cfg, logger)))
	}
}

// personAlertOutcome reads the JSON answer of tools.SendNotification: sent when at least one
// channel sent, skipped "channel unavailable" when no channel is enabled, failed otherwise
// with the channels' details (personAlertDetail).
func personAlertOutcome(raw string) error {
	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Results []struct {
			Channel string `json:"channel"`
			Status  string `json:"status"`
			Detail  string `json:"detail"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return errors.New("unexpected notification result")
	}
	if out.Status != "success" {
		if out.Message == "no notification channels are enabled" {
			return persondetect.Skipped("channel unavailable")
		}
		if out.Message == "" {
			out.Message = "notification failed"
		}
		return errors.New(personAlertDetail(out.Message))
	}
	failures := make([]string, 0, len(out.Results))
	for _, r := range out.Results {
		if r.Status == "sent" {
			return nil
		}
		failures = append(failures, r.Channel+": "+personAlertDetail(r.Detail))
	}
	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

// personAlertDetail cleans a channel's error for the event's alert detail, which the
// Network Cameras app and the agent read: URLs (an ntfy topic URL is a secret, a request
// URL may carry a token) become "[url]", and registered secrets are scrubbed.
func personAlertDetail(detail string) string {
	return security.Scrub(personAlertURL.ReplaceAllString(strings.TrimSpace(detail), "[url]"))
}

// sendPersonAlertDiscordPhoto writes photo to a private temporary directory, because
// discord.SendDiscordImage uploads a file, and removes it after the send.
func sendPersonAlertDiscordPhoto(cfg *config.Config, logger *slog.Logger, photo []byte, caption string) error {
	dir, err := os.MkdirTemp("", "aurago-person-alert-*")
	if err != nil {
		return fmt.Errorf("create temporary photo directory: %w", err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, personAlertPhotoName)
	if err := os.WriteFile(path, photo, 0o600); err != nil {
		return fmt.Errorf("write temporary photo: %w", err)
	}
	return personAlertDiscordImage(cfg.Discord.DefaultChannelID, path, caption, logger)
}

// personAlertTelegramReady mirrors the Telegram condition of tools.SendNotification.
func personAlertTelegramReady(cfg *config.Config) bool {
	return strings.TrimSpace(cfg.Telegram.BotToken) != "" && cfg.Telegram.UserID != 0
}

// personAlertDiscordReady mirrors the Discord condition of tools.SendNotification.
func personAlertDiscordReady(cfg *config.Config) bool {
	return cfg.Discord.Enabled && !cfg.Discord.ReadOnly && strings.TrimSpace(cfg.Discord.DefaultChannelID) != ""
}

// personAlertDiscordSender is the Discord text sender of SendNotification, built like the
// agent's send_notification tool does (dispatch_messaging.go).
func personAlertDiscordSender(cfg *config.Config, logger *slog.Logger) tools.DiscordSendFunc {
	if !cfg.Discord.Enabled {
		return nil
	}
	return func(channelID, content string) error { return tools.DiscordSend(channelID, content, logger) }
}

// personAlertTelnyxSender is the Telnyx SMS sender of SendNotification, built like
// telnyxNotificationSender in the agent package.
func personAlertTelnyxSender(ctx context.Context, cfg *config.Config, logger *slog.Logger) tools.TelnyxSendFunc {
	if !cfg.Telnyx.Enabled || cfg.Telnyx.PhoneNumber == "" {
		return nil
	}
	return func(to, message string) error {
		_, err := telnyx.NewConfiguredClient(cfg, logger).SendSMS(ctx, cfg.Telnyx.PhoneNumber, to, message, cfg.Telnyx.MessagingProfileID)
		return err
	}
}

// showPersonAlertToast is the Desktop sink: a notification for the desktop notification
// center, shaped like the flow failure notifications; its context lets a click open the
// event in Network Cameras (shell-chrome-runtime.js keeps it). The hub is read when the
// toast is shown, because the dispatcher is built before the desktop starts.
func (s *Server) showPersonAlertToast(ev persondetect.AlertEvent, title, message string) error {
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		return persondetect.Skipped("desktop unavailable")
	}
	s.DesktopMu.Lock()
	hub := s.DesktopHub
	s.DesktopMu.Unlock()
	broadcastDesktopEvent(s, hub, desktop.Event{Type: "notification", Payload: map[string]any{
		"title":   title,
		"message": message,
		"type":    "info",
		"appId":   "network-cameras",
		"context": map[string]any{"event_id": ev.EventID, "camera_id": ev.CameraID},
	}, CreatedAt: time.Now().UTC()})
	return nil
}

// notifyPersonDetectedMission is the Mission sink: every event start, armed or not, goes to
// Mission Control, whose missions and flows apply their own filters and min intervals.
// Handing the event over counts as sent; without a mission manager the action is skipped.
func (s *Server) notifyPersonDetectedMission(ev persondetect.AlertEvent) error {
	mm := s.MissionManagerV2
	if mm == nil {
		return persondetect.Skipped("missions unavailable")
	}
	mm.NotifyPersonDetected(tools.PersonDetectedEvent{
		EventID:    ev.EventID,
		CameraID:   ev.CameraID,
		CameraName: ev.CameraName,
		Zones:      ev.Zones,
		MaxScore:   ev.Score,
		Persons:    ev.Persons,
		Armed:      ev.Armed,
		StartedAt:  ev.StartedAt,
		ImagePath:  ev.CoverPath,
	})
	return nil
}

// newPersonAlertDispatcher builds the dispatcher that person detection reports event starts
// to (persondetect.Deps.Alerts): messages, agent wake-ups, desktop toasts and the
// person_detected trigger. It is built in newServerFromOptions, before Start: every sink
// reads the server's configuration, desktop hub and mission manager when it runs, and the
// agent queue holds wake-ups until Start starts its consumer.
func (s *Server) newPersonAlertDispatcher() *persondetect.Dispatcher {
	return persondetect.NewAlertDispatcher(persondetect.AlertSinks{
		Message:   s.sendPersonAlertMessage,
		Agent:     s.personAlertAgents.enqueue,
		AgentGate: s.personWakeGate,
		Desktop:   s.showPersonAlertToast,
		Mission:   s.notifyPersonDetectedMission,
	}, personAlertText, s.personAlertLogger())
}

// personAlertText is i18n.T without parameters: the dispatcher fills the placeholders
// itself, in one pass, and treats a returned key as "no text".
func personAlertText(lang, key string) string { return i18n.T(lang, key) }
