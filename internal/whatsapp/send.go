package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
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

// tokenBucket paces the send queue. The caller holds the sender's queue slot, so take
// runs serially.
type tokenBucket struct {
	tokens float64
	last   time.Time
	// skew is the sleep time that the clock did not show; see take. It stays zero with
	// a real clock: time.Now is monotonic, so it always moves during a sleep.
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
		// The 1 ms floor keeps a token count just below 1 from rounding the wait to 0,
		// which would spin on a clock that has not ticked yet.
		wait := max(time.Duration((1-b.tokens)*float64(sendInterval)), time.Millisecond)
		if !b.sleep(ctx, wait) {
			return false
		}
		// A clock that did not move during a sleep (a frozen test clock) would never
		// refill the bucket. Count the sleep as elapsed instead of waiting forever.
		// With time.Now this never fires: the monotonic reading always advances during
		// a real sleep, so production pacing runs on the clock alone.
		if !b.now().Add(b.skew).After(now) {
			b.skew += wait
		}
	}
}

// sender is the Bot's one serialized outbound queue. Every text AuraGo sends goes through
// deliverText and every media message through deliverMedia: owner guard, formatting,
// scrubbing, chunking, self-chat prefix, pacing, echo-cache registration and delivery
// accounting.
type sender struct {
	// sem is the one queue slot, held for all chunks of one message so chunks stay
	// together. A channel instead of a mutex lets a waiting send give up with its ctx.
	sem    chan struct{}
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
	return &sender{sem: make(chan struct{}, 1), cfg: cfg, client: client, sent: sent, bucket: tokenBucket{now: now, sleep: sleep}, now: now, logger: logger}
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

// prepare runs the checks that precede every send and returns the client and the owner entry
// the message goes to. The broker discards the error of a reply, so each early refusal leaves
// a log line. It names the label only, never the content, and none of them counts as a
// delivery failure.
func (s *sender) prepare(ctx context.Context, to types.JID, what string) (waClient, types.JID, error) {
	if err := ctx.Err(); err != nil {
		return nil, types.EmptyJID, err // a cancelled send is never counted
	}
	client := s.client()
	if client == nil || !client.IsLoggedIn() {
		s.logger.Warn("Cannot send WhatsApp " + what + ": not connected")
		return nil, types.EmptyJID, ErrNotConnected
	}
	recipient, err := s.ownerRecipient(client, to)
	if err != nil {
		if !errors.Is(err, errNotOwnerRecipient) { // the guard has logged its own refusal
			s.logger.Warn("Cannot send WhatsApp " + what + ": no owner recipient")
		}
		return nil, types.EmptyJID, err
	}
	return client, recipient, nil
}

// acquire takes the queue slot. A send that never got it is not a delivery failure.
func (s *sender) acquire(ctx context.Context) (release func(), err error) {
	select {
	case s.sem <- struct{}{}:
		return func() { <-s.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// selfChatPrefix is what every outgoing text and caption starts with in self_chat mode.
func (s *sender) selfChatPrefix() string {
	if s.cfg.Mode == config.WhatsAppModeSelfChat {
		return s.cfg.SelfChatPrefix
	}
	return ""
}

// deliverText sends markdown to one owner as WhatsApp text. what names the message
// in log lines and errors ("reply", "test message"); it is never user content.
func (s *sender) deliverText(ctx context.Context, to types.JID, markdown, what string) error {
	_, err := s.deliverTextParts(ctx, to, markdown, what)
	return err
}

// deliverTextParts is deliverText that also reports whether at least one chunk reached WhatsApp.
// A long text goes out as several messages; when a later one fails, the earlier ones are
// delivered, and a caller that retries would send them twice.
func (s *sender) deliverTextParts(ctx context.Context, to types.JID, markdown, what string) (sent bool, err error) {
	client, recipient, err := s.prepare(ctx, to, what)
	if err != nil {
		return false, err
	}
	prefix := s.selfChatPrefix()
	// Scrub first: a registered value is matched as written, and the formatter would
	// break one that holds a pipe or a doubled asterisk.
	body := toWhatsApp(security.Scrub(markdown))
	chunks := chunkText(body, maxChunkRunes-utf8.RuneCountInString(prefix))

	release, err := s.acquire(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	for _, chunk := range chunks {
		if err := s.sendOne(ctx, client, recipient, &waE2E.Message{Conversation: proto.String(prefix + chunk)}, what); err != nil {
			return sent, err
		}
		sent = true
	}
	return sent, nil
}

// deliverMedia sends one media message to one owner through the same guard, slot, bucket,
// echo registration and failure accounting as deliverText. The caller has uploaded the file;
// msg carries its caption, already formatted by caption.
func (s *sender) deliverMedia(ctx context.Context, to types.JID, msg *waE2E.Message, what string) error {
	client, recipient, err := s.prepare(ctx, to, what)
	if err != nil {
		return err
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	return s.sendOne(ctx, client, recipient, msg, what)
}

// sendOne paces, registers the ID before the call, sends and accounts one message; the caller
// holds the queue slot.
func (s *sender) sendOne(ctx context.Context, client waClient, to types.JID, msg *waE2E.Message, what string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.bucket.take(ctx) {
		if err := ctx.Err(); err != nil {
			return err
		}
		return context.Canceled
	}
	// The ID is fixed and registered before the call: an echo of this message can reach
	// the inbound filter before SendMessage returns. A failed send leaves its ID in the
	// cache, which is harmless.
	id := client.GenerateMessageID()
	if s.sent != nil {
		s.sent.Add(string(id))
	}
	_, err := client.SendMessage(ctx, to, msg, id)
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
	integrationstatus.MarkWhatsAppOutbound(s.now())
	return nil
}

// caption formats markdown like deliverText: Scrub first, then toWhatsApp, then the self-chat
// prefix. It returns "" for a markdown that holds no text, which carries no prefix. cut=true
// trims the result to maxCaptionRunes with boundRunes; fits reports whether the result is
// within that limit, so it is always true after a cut. cut=false returns the whole text, and
// the caller sends a text that does not fit as a message of its own.
func (s *sender) caption(markdown string, cut bool) (text string, fits bool) {
	// Invalid UTF-8 would make the message unmarshalable, as in chunkText.
	body := trimBlankEdges(strings.ToValidUTF8(toWhatsApp(security.Scrub(markdown)), utf8Replacement))
	if body == "" {
		return "", true
	}
	text = s.selfChatPrefix() + body
	if utf8.RuneCountInString(text) <= maxCaptionRunes {
		return text, true
	}
	if cut {
		return boundRunes(text, maxCaptionRunes), true
	}
	return text, false
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
