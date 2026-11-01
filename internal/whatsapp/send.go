package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"
	"aurago/internal/security"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const (
	sendBurst    = 5
	sendInterval = 3 * time.Second // 20 messages per minute
)

// tokenBucket paces the send queue. The caller holds sender.mu, so take runs serially.
type tokenBucket struct {
	tokens float64
	last   time.Time
	// skew is the sleep time that a frozen clock did not show; see take.
	skew  time.Duration
	now   func() time.Time
	sleep func(ctx context.Context, d time.Duration) bool
}

// take waits until a token is available and uses it. It returns false when the
// sleep ends early because ctx is done.
func (b *tokenBucket) take(ctx context.Context) bool {
	for {
		now := b.now().Add(b.skew)
		if b.last.IsZero() {
			b.tokens, b.last = sendBurst, now
		}
		elapsed := now.Sub(b.last)
		if elapsed < 0 {
			elapsed = 0
		}
		b.tokens = math.Min(sendBurst, b.tokens+float64(elapsed)/float64(sendInterval))
		b.last = now
		if b.tokens >= 1 {
			b.tokens--
			return true
		}
		wait := max(time.Duration((1-b.tokens)*float64(sendInterval)), time.Millisecond)
		if !b.sleep(ctx, wait) {
			return false
		}
		// A clock that did not move during a sleep (a frozen test clock) would never
		// refill the bucket. Count the sleep as elapsed instead of waiting forever.
		if !b.now().Add(b.skew).After(now) {
			b.skew += wait
		}
	}
}

// sender is the Bot's one serialized outbound queue. Every text AuraGo sends goes through
// deliverText: owner guard, formatting, scrubbing, chunking, self-chat prefix, pacing,
// echo-cache registration and delivery accounting.
type sender struct {
	mu     sync.Mutex // held for all chunks of one message, so chunks stay together
	cfg    config.WhatsAppConfig
	client func() waClient
	sent   *idCache
	bucket tokenBucket
	now    func() time.Time
	logger *slog.Logger
}

// newSender builds the queue for one Bot generation. client is read on every send,
// so it follows the connection; sent receives the IDs of the messages that left.
func newSender(cfg config.WhatsAppConfig, client func() waClient, sent *idCache, deps runtimeDeps, logger *slog.Logger) *sender {
	if logger == nil {
		logger = slog.Default()
	}
	now, sleep := deps.now, deps.sleep
	if now == nil {
		now = time.Now
	}
	if sleep == nil {
		sleep = sleepContext
	}
	return &sender{cfg: cfg, client: client, sent: sent, bucket: tokenBucket{now: now, sleep: sleep}, now: now, logger: logger}
}

// ownerRecipient returns the owner entry that to stands for, or errNotOwnerRecipient.
// The entry, not the caller's JID, is what gets the message.
func (s *sender) ownerRecipient(client waClient, to types.JID) (types.JID, error) {
	owners, err := ownerRecipients(s.cfg, client.OwnJID())
	if err != nil {
		return types.EmptyJID, err
	}
	for _, owner := range owners {
		if owner.User == to.User && owner.Server == to.Server {
			return owner, nil
		}
	}
	s.logger.Warn("WhatsApp refused a recipient that is not an owner", "to", maskJID(to))
	return types.EmptyJID, errNotOwnerRecipient
}

// deliverText sends markdown to one owner as WhatsApp text. what names the message
// in log lines and errors ("reply", "test message"); it is never user content.
func (s *sender) deliverText(ctx context.Context, to types.JID, markdown, what string) error {
	if err := ctx.Err(); err != nil {
		return err // a cancelled send is never counted
	}
	client := s.client()
	if client == nil || !client.IsLoggedIn() {
		return ErrNotConnected
	}
	recipient, err := s.ownerRecipient(client, to)
	if err != nil {
		return err
	}
	prefix := ""
	if s.cfg.Mode == config.WhatsAppModeSelfChat {
		prefix = s.cfg.SelfChatPrefix
	}
	body := security.Scrub(toWhatsApp(markdown))
	chunks := chunkText(body, maxChunkRunes-utf8.RuneCountInString(prefix))

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !s.bucket.take(ctx) {
			if err := ctx.Err(); err != nil {
				return err
			}
			return context.Canceled
		}
		resp, err := client.SendMessage(ctx, recipient, &waE2E.Message{Conversation: proto.String(prefix + chunk)})
		if err != nil {
			if ctx.Err() != nil {
				return err
			}
			integrationstatus.MarkWhatsAppDeliveryFailure()
			s.logger.Warn("Failed to send WhatsApp "+what, "error", maskDigits(security.Scrub(err.Error())))
			if errors.Is(err, whatsmeow.ErrRecipientNotOnWhatsApp) {
				return fmt.Errorf("%w: %w", ErrRecipientNotOnWhatsApp, err)
			}
			return fmt.Errorf("send whatsapp %s: %w", what, err)
		}
		if s.sent != nil {
			s.sent.Add(string(resp.ID))
		}
		integrationstatus.MarkWhatsAppOutbound(s.now())
	}
	return nil
}

// deliverToOwners sends markdown to every owner recipient. It continues after a
// failure and returns the first error.
func (s *sender) deliverToOwners(ctx context.Context, markdown, what string) error {
	client := s.client()
	if client == nil || !client.IsLoggedIn() {
		return ErrNotConnected
	}
	owners, err := ownerRecipients(s.cfg, client.OwnJID())
	if err != nil {
		return err
	}
	var first error
	for _, to := range owners {
		if err := s.deliverText(ctx, to, markdown, what); err != nil && first == nil {
			first = err
		}
	}
	return first
}
