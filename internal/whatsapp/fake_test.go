package whatsapp

import (
	"context"
	"errors"
	"io"
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
	id   types.MessageID // the ID the sender passed
	msg  *waE2E.Message  // the whole message, for media tests
}

// uploadCall is one Upload the fake received.
type uploadCall struct {
	mediaType whatsmeow.MediaType
	data      []byte
}

// fakeDownloadChunk is the size of the writes DownloadToFile makes into the file.
const fakeDownloadChunk = 32 << 10

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
	idSeq       int     // GenerateMessageID hands out fake-1, fake-2, ...
	// beforeSend runs at the start of every SendMessage, outside the lock, with the ID the
	// sender passed.
	beforeSend func(id types.MessageID)

	media          map[string][]byte // attachment bytes keyed by direct path
	downloadErr    error
	downloads      []string // direct paths DownloadToFile was called with
	downloadCtxErr []error  // ctx.Err() when each DownloadToFile returned
	uploads        []uploadCall
	uploadErr      error
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

func (f *fakeClient) GenerateMessageID() types.MessageID {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.idSeq++
	return types.MessageID("fake-" + strconv.Itoa(f.idSeq))
}

// SendMessage records the call and answers with the ID it was given, like whatsmeow.
func (f *fakeClient) SendMessage(_ context.Context, to types.JID, msg *waE2E.Message, id types.MessageID) (whatsmeow.SendResponse, error) {
	f.mu.Lock()
	hook := f.beforeSend
	f.mu.Unlock()
	if hook != nil {
		hook(id)
	}
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
	f.sent = append(f.sent, sentMessage{to: to, text: msg.GetConversation(), id: id, msg: msg})
	return whatsmeow.SendResponse{ID: id}, nil
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

// DownloadToFile records the direct path and writes the stored bytes through file.Write in
// 32 KiB chunks, checking ctx before each one, like a streaming download. An unknown path
// answers 404 (expired media).
func (f *fakeClient) DownloadToFile(ctx context.Context, msg whatsmeow.DownloadableMessage, file whatsmeow.File) (err error) {
	f.mu.Lock()
	path := msg.GetDirectPath()
	f.downloads = append(f.downloads, path)
	data, ok := f.media[path]
	downloadErr := f.downloadErr
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.downloadCtxErr = append(f.downloadCtxErr, ctx.Err())
		f.mu.Unlock()
	}()
	if downloadErr != nil {
		return downloadErr
	}
	if !ok {
		return whatsmeow.ErrMediaDownloadFailedWith404
	}
	for len(data) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := min(len(data), fakeDownloadChunk)
		if _, err := file.Write(data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// Upload reads r completely and answers with a canned response, plus a streaming sidecar for
// video.
func (f *fakeClient) Upload(ctx context.Context, r io.Reader, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	if err := ctx.Err(); err != nil {
		return whatsmeow.UploadResponse{}, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return whatsmeow.UploadResponse{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploads = append(f.uploads, uploadCall{mediaType: mediaType, data: data})
	if f.uploadErr != nil {
		return whatsmeow.UploadResponse{}, f.uploadErr
	}
	up := whatsmeow.UploadResponse{
		URL:           "https://mmg.example/u",
		DirectPath:    "/v/fake",
		MediaKey:      []byte{1},
		FileSHA256:    []byte{2},
		FileEncSHA256: []byte{3},
		FileLength:    uint64(len(data)),
	}
	if mediaType == whatsmeow.MediaVideo {
		up.StreamingSidecar = []byte{4}
	}
	return up, nil
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
