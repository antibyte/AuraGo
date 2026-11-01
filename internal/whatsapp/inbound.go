package whatsapp

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/integrationstatus"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Fixed drop reason codes. Status counts them; logs never carry message content.
const (
	dropStatusBroadcast   = "status"
	dropBroadcast         = "broadcast"
	dropNewsletter        = "newsletter"
	dropGroup             = "group"
	dropEmpty             = "empty"
	dropEdit              = "edit" // edits, pins and revokes (Info.Edit)
	dropProtocol          = "protocol"
	dropReaction          = "reaction"
	dropPoll              = "poll"
	dropEcho              = "echo"
	dropDuplicate         = "duplicate"
	dropStale             = "stale"
	dropFromMe            = "from_me"
	dropUnsupportedSender = "unsupported_sender"
	dropNotSelfChat       = "not_self_chat"
	dropNonOwner          = integrationstatus.WhatsAppDropNonOwner
	dropUnresolvedLID     = integrationstatus.WhatsAppDropUnresolvedLID
)

// inboundMessage is an accepted owner message.
type inboundMessage struct {
	Chat      types.JID // chat the owner wrote in (receipts, typing)
	Sender    types.JID // whatsmeow sender (receipts)
	Owner     types.JID // owner phone JID: the reply target, always a member of ownerRecipients
	ID        types.MessageID
	Timestamp time.Time
	Message   *waE2E.Message // unwrapped content
}

// inboundFilter decides which inbound messages reach the agent. It holds no
// message content and logs only masked JIDs.
type inboundFilter struct {
	mode      string
	owners    map[string]bool // owner numbers as JID users (E.164 without "+")
	sent      *idCache        // shared with the sender: the echo guard
	seen      *idCache        // dedupe of accepted IDs
	logger    *slog.Logger
	mu        sync.Mutex
	ownerLIDs map[string]types.JID // LID user -> owner phone JID, learned on acceptance
}

func newInboundFilter(cfg config.WhatsAppConfig, sent *idCache, logger *slog.Logger) *inboundFilter {
	if logger == nil {
		logger = slog.Default()
	}
	if sent == nil {
		sent = newIDCache(sentIDCacheSize, sentIDCacheTTL, nil)
	}
	owners := make(map[string]bool, len(cfg.OwnerNumbers))
	for _, raw := range cfg.OwnerNumbers {
		number, err := config.NormalizeE164(raw)
		if err != nil {
			continue
		}
		owners[strings.TrimPrefix(number, "+")] = true
	}
	return &inboundFilter{
		mode:      cfg.Mode,
		owners:    owners,
		sent:      sent,
		seen:      newIDCache(dedupeCacheSize, 0, nil),
		logger:    logger,
		ownerLIDs: make(map[string]types.JID),
	}
}

// accept runs design steps 2-8. Step 1 (only *events.Message; HistorySync ignored) is the
// caller's type switch. cutoff is zero outside the offline sync (Task 8). The reason of a
// dropped message is one of the fixed drop codes; an accepted message has an empty reason.
func (f *inboundFilter) accept(ctx context.Context, client waClient, evt *events.Message, cutoff time.Time) (inboundMessage, string) {
	if evt == nil {
		return inboundMessage{}, dropEmpty
	}
	info := evt.Info
	chat := info.Chat.ToNonAD()
	switch { // 2. chats AuraGo never serves; status and broadcast lists share the broadcast server
	case chat.Server == types.BroadcastServer && chat.User == types.StatusBroadcastJID.User:
		return inboundMessage{}, dropStatusBroadcast
	case chat.Server == types.BroadcastServer:
		return inboundMessage{}, dropBroadcast
	case chat.Server == types.NewsletterServer || info.Sender.Server == types.NewsletterServer:
		return inboundMessage{}, dropNewsletter
	case info.IsGroup || chat.Server == types.GroupServer:
		return inboundMessage{}, dropGroup
	}
	if evt.Message == nil && evt.RawMessage != nil { // 3. whatsmeow unwraps already (UnwrapRaw)
		evt.UnwrapRaw()
	}
	if reason := kindDropReason(evt); reason != "" {
		return inboundMessage{}, reason
	}
	id := string(info.ID)
	if f.sent.Contains(id) { // 4.
		return inboundMessage{}, dropEcho
	}
	if f.seen.Contains(id) { // 5. recorded only on acceptance
		return inboundMessage{}, dropDuplicate
	}
	if !cutoff.IsZero() && info.Timestamp.Before(cutoff) { // 6.
		return inboundMessage{}, dropStale
	}
	var owner types.JID
	var reason string
	if f.mode == config.WhatsAppModeSelfChat { // 8. before any content is read
		owner, reason = selfChatOwner(info, client.OwnJID(), client.OwnLID())
	} else { // 7.
		owner, reason = f.dedicatedOwner(ctx, client, info)
	}
	if reason != "" {
		return inboundMessage{}, reason
	}
	f.seen.Add(id)
	return inboundMessage{Chat: chat, Sender: info.Sender, Owner: owner, ID: info.ID, Timestamp: info.Timestamp, Message: evt.Message}, ""
}

// kindDropReason names why an unwrapped message is not a plain conversation
// message, or returns "". Only the message kind is read, never its text.
func kindDropReason(evt *events.Message) string {
	m := evt.Message
	switch {
	case m == nil:
		return dropEmpty
	case evt.IsEdit || evt.Info.Edit != types.EditAttributeEmpty:
		return dropEdit
	case m.GetProtocolMessage() != nil:
		return dropProtocol
	case m.GetReactionMessage() != nil || m.GetEncReactionMessage() != nil:
		return dropReaction
	case isPollMessage(m):
		return dropPoll
	}
	return ""
}

func isPollMessage(m *waE2E.Message) bool {
	return m.GetPollCreationMessage() != nil || m.GetPollCreationMessageV2() != nil ||
		m.GetPollCreationMessageV3() != nil || m.GetPollCreationMessageV4() != nil ||
		m.GetPollCreationMessageV5() != nil || m.GetPollCreationMessageV6() != nil ||
		m.GetPollUpdateMessage() != nil ||
		m.GetPollResultSnapshotMessage() != nil || m.GetPollResultSnapshotMessageV3() != nil
}

// dedicatedOwner is step 7: the sender must be one of the configured owner
// phones. A LID sender is mapped to its phone first.
func (f *inboundFilter) dedicatedOwner(ctx context.Context, client waClient, info types.MessageInfo) (types.JID, string) {
	if info.IsFromMe {
		return types.EmptyJID, dropFromMe
	}
	pn, reason := f.senderPhone(ctx, client, info)
	switch reason {
	case "":
	case dropUnresolvedLID:
		f.logger.Warn("WhatsApp sender has no phone mapping; message dropped", "sender", maskJID(info.Sender))
		return types.EmptyJID, reason
	default:
		return types.EmptyJID, reason
	}
	if !f.owners[pn.User] {
		f.logger.Info("WhatsApp message from a non-owner dropped", "sender", maskJID(pn))
		return types.EmptyJID, dropNonOwner
	}
	f.rememberOwnerLID(info, pn)
	return pn, ""
}

// senderPhone resolves the sender's phone JID. It always works on the sender
// without its device part. A LID resolves through the message's own phone alt,
// the device store, and finally the LIDs learned from accepted owner messages.
func (f *inboundFilter) senderPhone(ctx context.Context, client waClient, info types.MessageInfo) (types.JID, string) {
	sender := info.Sender.ToNonAD()
	switch sender.Server {
	case types.DefaultUserServer:
		return types.NewJID(sender.User, types.DefaultUserServer), ""
	case types.HiddenUserServer:
		// SenderAlt counts only as a phone JID: whatsmeow also fills it with a LID.
		if alt := info.SenderAlt.ToNonAD(); alt.Server == types.DefaultUserServer && alt.User != "" {
			return types.NewJID(alt.User, types.DefaultUserServer), ""
		}
		if pn, err := client.PNForLID(ctx, sender); err == nil && pn.Server == types.DefaultUserServer && pn.User != "" {
			return types.NewJID(pn.User, types.DefaultUserServer), ""
		}
		f.mu.Lock()
		pn, ok := f.ownerLIDs[sender.User]
		f.mu.Unlock()
		if ok {
			return pn, ""
		}
		return types.EmptyJID, dropUnresolvedLID
	}
	return types.EmptyJID, dropUnsupportedSender
}

// rememberOwnerLID stores the LID of an accepted owner message, so a later
// message from that LID resolves even when whatsmeow has no mapping for it.
// Only accepted owners are remembered, so a stranger cannot teach a LID.
func (f *inboundFilter) rememberOwnerLID(info types.MessageInfo, pn types.JID) {
	lid := info.Sender.ToNonAD()
	if lid.Server != types.HiddenUserServer {
		lid = info.SenderAlt.ToNonAD()
	}
	if lid.Server != types.HiddenUserServer || lid.User == "" {
		return
	}
	f.mu.Lock()
	f.ownerLIDs[lid.User] = pn
	f.mu.Unlock()
}

// selfChatOwner is step 8: in self_chat mode only the account's own messages
// in its own chat count, and the owner is the linked account itself.
func selfChatOwner(info types.MessageInfo, own, ownLID types.JID) (types.JID, string) {
	chat := info.Chat.ToNonAD()
	switch {
	case !info.IsFromMe || own.IsEmpty():
		return types.EmptyJID, dropNotSelfChat
	case chat.Server == types.DefaultUserServer && chat.User == own.User,
		chat.Server == types.HiddenUserServer && !ownLID.IsEmpty() && chat.User == ownLID.User:
		return own.ToNonAD(), ""
	}
	return types.EmptyJID, dropNotSelfChat
}
