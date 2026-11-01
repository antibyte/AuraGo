package whatsapp

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

type sentMessage struct {
	to   types.JID
	text string
}

type readCall struct {
	ids          []types.MessageID
	chat, sender types.JID
}

type chatStateCall struct {
	chat  types.JID
	state types.ChatPresence
}

// fakeClient records every call and lets a test emit whatsmeow events.
type fakeClient struct {
	mu          sync.Mutex
	handlers    []func(evt any)
	connectErrs []error
	connects    int
	disconnects int
	connected   bool // set by a successful ConnectContext, cleared by Disconnect
	loggedIn    bool
	own         types.JID
	qr          chan whatsmeow.QRChannelItem
	pairCode    string
	pairPhones  []string
	logoutErr   error
	logouts     int
	sendErr     error
	sent        []sentMessage
	presences   []types.Presence
	presenceErr error

	ownLID      types.JID
	lidPN       map[types.JID]types.JID // keyed by the non-AD LID
	lidErr      error
	lastConnect time.Time
	reads       []readCall
	chatStates  []chatStateCall
	sendErrs    []error // SendMessage consumes one per call before it falls back to sendErr
}

func (f *fakeClient) with(fn func(f *fakeClient)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeClient) AddEventHandler(handler func(evt any)) uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers = append(f.handlers, handler)
	return uint32(len(f.handlers))
}

func (f *fakeClient) ConnectContext(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connects++
	if len(f.connectErrs) > 0 {
		err := f.connectErrs[0]
		f.connectErrs = f.connectErrs[1:]
		return err
	}
	f.connected = true
	return nil
}

func (f *fakeClient) Disconnect() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disconnects++
	f.connected = false
}

func (f *fakeClient) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

func (f *fakeClient) IsLoggedIn() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loggedIn
}

func (f *fakeClient) GetQRChannel(context.Context) (<-chan whatsmeow.QRChannelItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.qr == nil {
		return nil, errors.New("fake: no qr channel")
	}
	return f.qr, nil
}

func (f *fakeClient) PairPhone(_ context.Context, phone string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pairPhones = append(f.pairPhones, phone)
	return f.pairCode, nil
}

func (f *fakeClient) Logout(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logouts++
	return f.logoutErr
}

func (f *fakeClient) SendMessage(_ context.Context, to types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sendErrs) > 0 {
		err := f.sendErrs[0]
		f.sendErrs = f.sendErrs[1:]
		if err != nil {
			return whatsmeow.SendResponse{}, err
		}
	} else if f.sendErr != nil {
		return whatsmeow.SendResponse{}, f.sendErr
	}
	f.sent = append(f.sent, sentMessage{to: to, text: msg.GetConversation()})
	return whatsmeow.SendResponse{ID: types.MessageID("fake-" + strconv.Itoa(len(f.sent)))}, nil
}

func (f *fakeClient) SendPresence(_ context.Context, state types.Presence) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presences = append(f.presences, state)
	return f.presenceErr
}

func (f *fakeClient) OwnJID() types.JID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.own
}

func (f *fakeClient) MarkRead(_ context.Context, ids []types.MessageID, _ time.Time, chat, sender types.JID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, readCall{ids: slices.Clone(ids), chat: chat, sender: sender})
	return nil
}

func (f *fakeClient) SendChatPresence(_ context.Context, chat types.JID, state types.ChatPresence, _ types.ChatPresenceMedia) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chatStates = append(f.chatStates, chatStateCall{chat: chat, state: state})
	return nil
}

func (f *fakeClient) OwnLID() types.JID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ownLID
}

// PNForLID answers from lidPN; an unknown LID is EmptyJID without an error, like sqlstore.
func (f *fakeClient) PNForLID(_ context.Context, lid types.JID) (types.JID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lidErr != nil {
		return types.EmptyJID, f.lidErr
	}
	if pn, ok := f.lidPN[lid.ToNonAD()]; ok {
		return pn, nil
	}
	return types.EmptyJID, nil
}

func (f *fakeClient) LastSuccessfulConnect() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastConnect
}

// emit delivers evt to every handler, like whatsmeow's dispatcher.
func (f *fakeClient) emit(evt any) {
	f.mu.Lock()
	handlers := slices.Clone(f.handlers)
	f.mu.Unlock()
	for _, handler := range handlers {
		handler(evt)
	}
}

// fakeStore is one opened device store backed by a fakeClient.
type fakeStore struct {
	mu      sync.Mutex
	linked  bool
	own     types.JID
	client  *fakeClient
	deleted int
	closed  int
}

func (s *fakeStore) Linked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.linked
}

func (s *fakeStore) OwnJID() types.JID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.own
}

func (s *fakeStore) NewClient() waClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

func (s *fakeStore) DeleteLocal(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted++
	s.linked = false
	s.own = types.EmptyJID
	return nil
}

func (s *fakeStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed++
	return nil
}

type fakeBackend struct {
	store    *fakeStore
	err      error
	mu       sync.Mutex
	openErrs []error // consumed one per open call, before store is returned
}

func (b *fakeBackend) open(context.Context, string) (deviceStore, error) {
	if b.err != nil {
		return nil, b.err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.openErrs) > 0 {
		err := b.openErrs[0]
		b.openErrs = b.openErrs[1:]
		return nil, err
	}
	return b.store, nil
}

// waitFor polls cond until it holds or five seconds pass.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
