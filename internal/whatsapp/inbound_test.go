package whatsapp

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// Fake identities only. The owner is +4917012345678, the stranger +4917099998888.
const (
	inboundOwnerUser    = "4917012345678"
	inboundStrangerUser = "4917099998888"
	inboundOwnerLID     = "111122223333444"
	inboundStoreLID     = "555566667777888"
	inboundAltLID       = "999900001111222"
	inboundStrangerLID  = "111122223334444"
	inboundOwnLID       = "777788889999000"
	inboundSecretText   = "secret-message-text"
)

var (
	inboundOwnerJID    = types.NewJID(inboundOwnerUser, types.DefaultUserServer)
	inboundStrangerJID = types.NewJID(inboundStrangerUser, types.DefaultUserServer)
)

// msgEvent builds an event the way whatsmeow does: unwrapped from the raw message.
func msgEvent(info types.MessageInfo, raw *waE2E.Message) *events.Message {
	return (&events.Message{Info: info, RawMessage: raw}).UnwrapRaw()
}

func textMsg(text string) *waE2E.Message { return &waE2E.Message{Conversation: proto.String(text)} }

// dmInfo is a direct message from sender in sender's own chat.
func dmInfo(id string, sender types.JID) types.MessageInfo {
	return types.MessageInfo{
		MessageSource: types.MessageSource{Chat: sender, Sender: sender},
		ID:            types.MessageID(id),
		Timestamp:     fixedNow(),
	}
}

// newTestFilter returns a filter over a logger that writes to the returned buffer.
func newTestFilter(mode string, owners ...string) (*inboundFilter, *idCache, *bytes.Buffer) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	sent := newIDCache(sentIDCacheSize, sentIDCacheTTL, nil)
	cfg := config.WhatsAppConfig{Mode: mode, OwnerNumbers: owners}
	return newInboundFilter(cfg, sent, logger), sent, &logs
}

func lidJID(user string) types.JID { return types.NewJID(user, types.HiddenUserServer) }

func TestInboundResolvesOwnerByPhoneAltStoreAndMemory(t *testing.T) {
	f, _, _ := newTestFilter(config.WhatsAppModeDedicated, "+"+inboundOwnerUser)
	client := &fakeClient{own: testOwn, lidPN: map[types.JID]types.JID{lidJID(inboundStoreLID): inboundOwnerJID}}
	ctx := context.Background()

	withAlt := dmInfo("m2", lidJID(inboundOwnerLID))
	withAlt.SenderAlt = inboundOwnerJID
	viaStore := dmInfo("m3", lidJID(inboundStoreLID))
	phoneWithLIDAlt := dmInfo("m4", inboundOwnerJID)
	phoneWithLIDAlt.SenderAlt = lidJID(inboundAltLID) // whatsmeow fills this from its LID store
	learned := dmInfo("m5", lidJID(inboundOwnerLID))  // no alt, and the store has no mapping for it

	for _, c := range []struct {
		name string
		info types.MessageInfo
	}{
		{"phone sender", dmInfo("m1", inboundOwnerJID)},
		{"lid sender with a phone alt", withAlt},
		{"lid sender through the store", viaStore},
		{"phone sender with a lid alt", phoneWithLIDAlt},
		{"learned lid", learned},
	} {
		in, reason := f.accept(ctx, client, msgEvent(c.info, textMsg(inboundSecretText)), time.Time{})
		if reason != "" || in.Owner != inboundOwnerJID {
			t.Fatalf("%s: owner = %v, reason = %q, want %v accepted", c.name, in.Owner, reason, inboundOwnerJID)
		}
		if in.Message.GetConversation() != inboundSecretText || string(in.ID) != string(c.info.ID) {
			t.Fatalf("%s: accepted message lost its content or ID: %+v", c.name, in)
		}
	}
	// The phone-sender case taught its LID alt as well.
	if _, reason := f.accept(ctx, client, msgEvent(dmInfo("m6", lidJID(inboundAltLID)), textMsg("x")), time.Time{}); reason != "" {
		t.Fatalf("the LID alt of an accepted owner message was not remembered: %q", reason)
	}
}

func TestInboundDropsUnresolvedAndNonOwnerSendersWithoutContent(t *testing.T) {
	f, _, logs := newTestFilter(config.WhatsAppModeDedicated, "+"+inboundOwnerUser)
	client := &fakeClient{own: testOwn}
	ctx := context.Background()
	text := textMsg(inboundSecretText)

	// An unmapped LID.
	if _, reason := f.accept(ctx, client, msgEvent(dmInfo("u1", lidJID(inboundStrangerLID)), text), time.Time{}); reason != dropUnresolvedLID {
		t.Fatalf("unmapped LID: reason = %q, want %q", reason, dropUnresolvedLID)
	}
	if out := logs.String(); !strings.Contains(out, "***4444") || strings.Contains(out, inboundStrangerLID) || strings.Contains(out, inboundSecretText) {
		t.Fatalf("unresolved LID log is not masked or leaks content: %q", out)
	}

	// A stranger's phone.
	logs.Reset()
	if _, reason := f.accept(ctx, client, msgEvent(dmInfo("n1", inboundStrangerJID), text), time.Time{}); reason != dropNonOwner {
		t.Fatalf("stranger: reason = %q, want %q", reason, dropNonOwner)
	}
	if out := logs.String(); !strings.Contains(out, "***8888") || strings.Contains(out, inboundStrangerUser) || strings.Contains(out, inboundSecretText) {
		t.Fatalf("non-owner log is not masked or leaks content: %q", out)
	}

	// A stranger cannot teach a LID: it stays unresolved after its phone message carried it as alt.
	taught := dmInfo("n2", inboundStrangerJID)
	taught.SenderAlt = lidJID(inboundStrangerLID)
	if _, reason := f.accept(ctx, client, msgEvent(taught, text), time.Time{}); reason != dropNonOwner {
		t.Fatalf("stranger with a LID alt: reason = %q, want %q", reason, dropNonOwner)
	}
	if _, reason := f.accept(ctx, client, msgEvent(dmInfo("u2", lidJID(inboundStrangerLID)), text), time.Time{}); reason != dropUnresolvedLID {
		t.Fatalf("a stranger's LID was remembered: reason = %q", reason)
	}

	// Own messages and unsupported servers.
	fromMe := dmInfo("f1", inboundOwnerJID)
	fromMe.IsFromMe = true
	if _, reason := f.accept(ctx, client, msgEvent(fromMe, text), time.Time{}); reason != dropFromMe {
		t.Fatalf("IsFromMe in dedicated mode: reason = %q, want %q", reason, dropFromMe)
	}
	if _, reason := f.accept(ctx, client, msgEvent(dmInfo("b1", types.NewJID("123456789", types.BotServer)), text), time.Time{}); reason != dropUnsupportedSender {
		t.Fatalf("bot sender: reason = %q, want %q", reason, dropUnsupportedSender)
	}
}

func TestInboundDropsChatsAuraGoNeverServes(t *testing.T) {
	f, _, _ := newTestFilter(config.WhatsAppModeDedicated, "+"+inboundOwnerUser)
	client := &fakeClient{own: testOwn}
	group := dmInfo("g1", inboundOwnerJID)
	group.Chat, group.IsGroup = types.NewJID("120363000000000001", types.GroupServer), true
	list := dmInfo("b1", inboundOwnerJID)
	list.Chat, list.IsGroup = types.NewJID("1730000000", types.BroadcastServer), true
	status := dmInfo("s1", inboundOwnerJID)
	status.Chat, status.IsGroup = types.StatusBroadcastJID, true
	newsletter := dmInfo("n1", inboundOwnerJID)
	newsletter.Chat = types.NewJID("120363111111111111", types.NewsletterServer)
	newsletter.Sender = newsletter.Chat
	for _, c := range []struct {
		info types.MessageInfo
		want string
	}{
		{group, dropGroup},
		{list, dropBroadcast},
		{status, dropStatusBroadcast},
		{newsletter, dropNewsletter},
	} {
		if _, reason := f.accept(context.Background(), client, msgEvent(c.info, textMsg("x")), time.Time{}); reason != c.want {
			t.Errorf("chat %s: reason = %q, want %q", c.info.Chat.Server, reason, c.want)
		}
	}
}

func TestInboundDropsNonConversationKinds(t *testing.T) {
	f, _, _ := newTestFilter(config.WhatsAppModeDedicated, "+"+inboundOwnerUser)
	client := &fakeClient{own: testOwn}
	revoke := dmInfo("r1", inboundOwnerJID)
	revoke.Edit = types.EditAttributeSenderRevoke
	for _, c := range []struct {
		name string
		info types.MessageInfo
		raw  *waE2E.Message
		want string
	}{
		{"protocol", dmInfo("k1", inboundOwnerJID), &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{}}, dropProtocol},
		{"reaction", dmInfo("k2", inboundOwnerJID), &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{}}, dropReaction},
		{"encrypted reaction", dmInfo("k3", inboundOwnerJID), &waE2E.Message{EncReactionMessage: &waE2E.EncReactionMessage{}}, dropReaction},
		{"edited wrapper", dmInfo("k4", inboundOwnerJID), &waE2E.Message{EditedMessage: &waE2E.FutureProofMessage{Message: textMsg("edited")}}, dropEdit},
		{"revoke attribute", revoke, textMsg("x"), dropEdit},
		{"poll creation v3", dmInfo("k5", inboundOwnerJID), &waE2E.Message{PollCreationMessageV3: &waE2E.PollCreationMessage{}}, dropPoll},
		{"poll update", dmInfo("k6", inboundOwnerJID), &waE2E.Message{PollUpdateMessage: &waE2E.PollUpdateMessage{}}, dropPoll},
		{"no content", dmInfo("k7", inboundOwnerJID), nil, dropEmpty},
	} {
		if _, reason := f.accept(context.Background(), client, msgEvent(c.info, c.raw), time.Time{}); reason != c.want {
			t.Errorf("%s: reason = %q, want %q", c.name, reason, c.want)
		}
	}
}

func TestInboundAcceptsEphemeralAndViewOnceContent(t *testing.T) {
	f, _, _ := newTestFilter(config.WhatsAppModeDedicated, "+"+inboundOwnerUser)
	client := &fakeClient{own: testOwn}
	inner := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hi")}}
	for name, raw := range map[string]*waE2E.Message{
		"ephemeral":    {EphemeralMessage: &waE2E.FutureProofMessage{Message: inner}},
		"view once v2": {ViewOnceMessageV2: &waE2E.FutureProofMessage{Message: inner}},
	} {
		in, reason := f.accept(context.Background(), client, msgEvent(dmInfo("w-"+name, inboundOwnerJID), raw), time.Time{})
		if reason != "" || in.Message.GetExtendedTextMessage().GetText() != "hi" {
			t.Errorf("%s: reason = %q, text = %q, want an accepted \"hi\"", name, reason, in.Message.GetExtendedTextMessage().GetText())
		}
	}
}

func TestInboundEchoDedupeAndOfflineCutoff(t *testing.T) {
	f, sent, _ := newTestFilter(config.WhatsAppModeDedicated, "+"+inboundOwnerUser)
	client := &fakeClient{own: testOwn}
	ctx := context.Background()
	accept := func(info types.MessageInfo, cutoff time.Time) string {
		_, reason := f.accept(ctx, client, msgEvent(info, textMsg("x")), cutoff)
		return reason
	}

	sent.Add("echo-1")
	if reason := accept(dmInfo("echo-1", inboundOwnerJID), time.Time{}); reason != dropEcho {
		t.Fatalf("an ID we sent: reason = %q, want %q", reason, dropEcho)
	}

	if reason := accept(dmInfo("d1", inboundOwnerJID), time.Time{}); reason != "" {
		t.Fatalf("first delivery: reason = %q", reason)
	}
	if reason := accept(dmInfo("d1", inboundOwnerJID), time.Time{}); reason != dropDuplicate {
		t.Fatalf("second delivery: reason = %q, want %q", reason, dropDuplicate)
	}

	// A rejected ID is not remembered: the owner's message with the same ID still passes.
	if reason := accept(dmInfo("r1", inboundStrangerJID), time.Time{}); reason != dropNonOwner {
		t.Fatalf("stranger: reason = %q", reason)
	}
	if reason := accept(dmInfo("r1", inboundOwnerJID), time.Time{}); reason != "" {
		t.Fatalf("a rejected ID entered the dedupe cache: reason = %q", reason)
	}

	cutoff := fixedNow()
	old := dmInfo("c1", inboundOwnerJID)
	old.Timestamp = cutoff.Add(-time.Second)
	if reason := accept(old, cutoff); reason != dropStale {
		t.Fatalf("a message before the cutoff: reason = %q, want %q", reason, dropStale)
	}
	if reason := accept(old, time.Time{}); reason != "" {
		t.Fatalf("a zero cutoff must accept an old timestamp (and a stale drop must not be remembered): reason = %q", reason)
	}
	onTime := dmInfo("c2", inboundOwnerJID)
	onTime.Timestamp = cutoff
	if reason := accept(onTime, cutoff); reason != "" {
		t.Fatalf("a message at the cutoff: reason = %q", reason)
	}
}

func TestInboundSelfChatAcceptsOnlyTheOwnChat(t *testing.T) {
	f, _, _ := newTestFilter(config.WhatsAppModeSelfChat)
	client := &fakeClient{own: testOwn, ownLID: lidJID(inboundOwnLID)}
	ctx := context.Background()
	own := func(id string, chat types.JID, fromMe bool) types.MessageInfo {
		info := dmInfo(id, testOwn)
		info.Chat, info.IsFromMe = chat, fromMe
		return info
	}

	in, reason := f.accept(ctx, client, msgEvent(own("s1", testOwn, true), textMsg("note")), time.Time{})
	if reason != "" || in.Owner != testOwn {
		t.Fatalf("own phone chat: owner = %v, reason = %q", in.Owner, reason)
	}
	if _, reason := f.accept(ctx, client, msgEvent(own("s2", lidJID(inboundOwnLID), true), textMsg("note")), time.Time{}); reason != "" {
		t.Fatalf("own LID chat: reason = %q", reason)
	}
	if _, reason := f.accept(ctx, client, msgEvent(own("s3", inboundOwnerJID, true), textMsg("note")), time.Time{}); reason != dropNotSelfChat {
		t.Fatalf("a contact's chat: reason = %q, want %q", reason, dropNotSelfChat)
	}
	if _, reason := f.accept(ctx, client, msgEvent(own("s4", testOwn, false), textMsg("note")), time.Time{}); reason != dropNotSelfChat {
		t.Fatalf("not from me in the own chat: reason = %q, want %q", reason, dropNotSelfChat)
	}
	// Without a known own LID, a LID chat is never the own chat.
	noLID := &fakeClient{own: testOwn}
	if _, reason := f.accept(ctx, noLID, msgEvent(own("s5", lidJID(inboundOwnLID), true), textMsg("note")), time.Time{}); reason != dropNotSelfChat {
		t.Fatalf("LID chat without an own LID: reason = %q, want %q", reason, dropNotSelfChat)
	}
}
