package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// secondOwner is the stranger fixture used as a second owner number where a test needs two.
const secondOwner = "+4917099998888"

var jpegBytes = append([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10}, []byte("JFIF fake photo")...)

// ownerAPI is a started Bot over the fake client, with a workspace and a data directory that are
// siblings below root (the config file lives in root, as the layout of a real install).
type ownerAPI struct {
	bot            *Bot
	fake           *fakeClient
	root, ws, data string
	live           atomic.Pointer[config.Config] // what the agent snapshot returns; starts as the Bot's own config
}

// ownerAPIOptions vary the fixture: noSnapshot starts the Bot without a snapshot function, wrap
// puts a client around the fake.
type ownerAPIOptions struct {
	noSnapshot bool
	wrap       func(*fakeClient) waClient
}

// clientStore is a linked device store whose client is c.
type clientStore struct {
	*fakeStore
	c waClient
}

func (s clientStore) NewClient() waClient { return s.c }

type storeBackend struct{ st deviceStore }

func (b storeBackend) open(context.Context, string) (deviceStore, error) { return b.st, nil }

func newOwnerAPI(t *testing.T, mutate func(*config.Config), owners ...string) *ownerAPI {
	t.Helper()
	return newOwnerAPIWith(t, ownerAPIOptions{}, mutate, owners...)
}

func newOwnerAPIWith(t *testing.T, opts ownerAPIOptions, mutate func(*config.Config), owners ...string) *ownerAPI {
	t.Helper()
	if len(owners) == 0 {
		owners = []string{"+4917012345678"}
	}
	a := &ownerAPI{root: t.TempDir()}
	a.ws, a.data = filepath.Join(a.root, "workspace"), filepath.Join(a.root, "data")
	for _, dir := range []string{a.ws, a.data} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	cfg := testBotConfig(config.WhatsAppModeDedicated, owners...)
	cfg.Directories.WorkspaceDir, cfg.Directories.DataDir = a.ws, a.data
	cfg.ConfigPath = filepath.Join(a.root, "config.yaml")
	if mutate != nil {
		mutate(cfg)
	}
	a.live.Store(cfg.Clone())
	a.fake = &fakeClient{own: testOwn, loggedIn: true}
	st := &fakeStore{linked: true, own: testOwn, client: a.fake}
	var backend backend = &fakeBackend{store: st}
	if opts.wrap != nil {
		backend = storeBackend{clientStore{st, opts.wrap(a.fake)}}
	}
	deps := runtimeDeps{backend: backend, sleep: (&recordedSleeps{}).sleep, now: fixedNow}
	if !opts.noSnapshot {
		deps.agent.Snapshot = func() (*config.Config, llm.ChatClient) { return a.live.Load(), nil }
	}
	a.bot = start(context.Background(), cfg, quietLogger(), deps)
	if a.bot == nil {
		t.Fatal("start returned no Bot")
	}
	t.Cleanup(a.bot.Stop)
	waitFor(t, "client", func() bool { return a.bot.currentClient() != nil })
	return a
}

func (a *ownerAPI) writeWorkspace(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(a.ws, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (a *ownerAPI) nothingSent(t *testing.T) {
	t.Helper()
	if up, sent := uploadsOf(a.fake), sentTexts(a.fake); len(up) != 0 || len(sent) != 0 {
		t.Fatalf("uploads = %d, sends = %d; want neither", len(up), len(sent))
	}
}

func TestSendOwnerMessageRefusesInReadOnlyMode(t *testing.T) {
	ctx := context.Background()
	check := func(t *testing.T, a *ownerAPI) {
		t.Helper()
		path := a.writeWorkspace(t, "report.pdf", []byte("%PDF-1.4 fake"))
		if _, err := a.bot.SendOwnerMessage(ctx, "hi", ""); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("text send = %v, want ErrReadOnly", err)
		}
		if _, err := a.bot.SendOwnerMessage(ctx, "hi", path); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("file send = %v, want ErrReadOnly", err)
		}
		if _, err := a.bot.SendOwnerAttachment(ctx, "hi", OwnerAttachment{Name: "p.jpg", Data: jpegBytes}); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("attachment send = %v, want ErrReadOnly", err)
		}
		a.nothingSent(t)
	}

	t.Run("started read-only", func(t *testing.T) {
		check(t, newOwnerAPI(t, func(c *config.Config) { c.WhatsApp.ReadOnly = true }))
	})

	t.Run("switched on in the live config", func(t *testing.T) {
		a := newOwnerAPI(t, nil)
		live := a.bot.pipeline.base.Clone()
		live.WhatsApp.ReadOnly = true
		a.live.Store(live)
		check(t, a)
	})

	t.Run("sends once the live config is writable again", func(t *testing.T) {
		a := newOwnerAPI(t, nil)
		a.live.Store(a.bot.pipeline.base.Clone())
		if _, err := a.bot.SendOwnerMessage(ctx, "hi", ""); err != nil {
			t.Fatalf("send with an unchanged live config: %v", err)
		}
		if got := sentBodies(a.fake); !slices.Equal(got, []string{"hi"}) {
			t.Fatalf("sent %q", got)
		}
	})
}

func TestSendOwnerMessageRefusesWithoutMessageOrConnection(t *testing.T) {
	ctx := context.Background()
	a := newOwnerAPI(t, nil)
	if !a.bot.OwnerSendReady() {
		t.Fatal("a running generation with a client must be ready")
	}
	if _, err := a.bot.SendOwnerMessage(ctx, " \n ", ""); err == nil || !strings.Contains(err.Error(), "message is required") {
		t.Fatalf("empty send = %v, want message is required", err)
	}
	if _, err := a.bot.SendOwnerAttachment(ctx, "hi", OwnerAttachment{Name: "a.jpg"}); err == nil {
		t.Fatal("an empty attachment was accepted")
	}

	a.fake.with(func(f *fakeClient) { f.loggedIn = false })
	if _, err := a.bot.SendOwnerMessage(ctx, "hi", ""); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("send while logged out = %v, want ErrNotConnected", err)
	}
	path := a.writeWorkspace(t, "report.pdf", []byte("%PDF-1.4 fake"))
	if _, err := a.bot.SendOwnerMessage(ctx, "hi", path); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("file send while logged out = %v, want ErrNotConnected", err)
	}
	a.nothingSent(t)

	a.bot.Stop()
	if a.bot.OwnerSendReady() {
		t.Fatal("a stopped generation reports ready")
	}
	if _, err := a.bot.SendOwnerMessage(ctx, "hi", ""); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("send after Stop = %v, want ErrNotConnected", err)
	}
	var nilBot *Bot
	if nilBot.OwnerSendReady() {
		t.Fatal("a nil Bot reports ready")
	}
	if _, err := nilBot.SendOwnerMessage(ctx, "hi", ""); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("send on a nil Bot = %v, want ErrNotConnected", err)
	}
}

func TestSendOwnerMessageOpensFilesOnlyThroughOutgoingAttachments(t *testing.T) {
	ctx := context.Background()
	a := newOwnerAPI(t, nil)
	a.writeWorkspace(t, "report.pdf", []byte("%PDF-1.4 fake"))

	res, err := a.bot.SendOwnerMessage(ctx, "hi", "report.pdf")
	if err != nil {
		t.Fatalf("send a workspace file: %v", err)
	}
	if res.Kind != "document" || res.File != "report.pdf" || res.TextSent {
		t.Fatalf("result = %+v", res)
	}
	sent := sentTexts(a.fake)
	doc := sent[0].msg.GetDocumentMessage()
	if len(sent) != 1 || doc == nil || doc.GetFileName() != "report.pdf" || doc.GetCaption() != "hi" || sent[0].to != ownerJID {
		t.Fatalf("sent = %+v, want one document report.pdf with caption hi for the owner", sent)
	}
	if up := uploadsOf(a.fake); len(up) != 1 || up[0].mediaType != whatsmeow.MediaDocument || string(up[0].data) != "%PDF-1.4 fake" {
		t.Fatalf("uploads = %+v", up)
	}

	// Anything outside the workspace and the documents folder never reaches the upload.
	if err := os.WriteFile(filepath.Join(a.root, "config.yaml"), []byte("secret: value"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(a.data, "whatsapp"), 0o700); err != nil {
		t.Fatal(err)
	}
	deviceDB := filepath.Join(a.data, "whatsapp", "device.db")
	if err := os.WriteFile(deviceDB, []byte("SQLite format 3"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../config.yaml", filepath.Join(a.root, "config.yaml"), deviceDB, "missing.pdf"} {
		if _, err := a.bot.SendOwnerMessage(ctx, "hi", path); err == nil {
			t.Fatalf("SendOwnerMessage(%q) succeeded", path)
		}
	}
	if up, sent := uploadsOf(a.fake), sentTexts(a.fake); len(up) != 1 || len(sent) != 1 {
		t.Fatalf("a refused path reached the network: uploads = %d, sends = %d (want 1 and 1 from the first send)", len(up), len(sent))
	}
}

func TestSendOwnerMessageRefusesFilesOverFiftyMB(t *testing.T) {
	a := newOwnerAPI(t, nil)
	path := a.writeWorkspace(t, "big.bin", nil)
	if err := os.Truncate(path, maxMediaBytes+1); err != nil {
		t.Skipf("cannot make a sparse file: %v", err)
	}
	_, err := a.bot.SendOwnerMessage(context.Background(), "hi", "big.bin")
	if !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("send of a file over 50 MB = %v, want ErrMediaTooLarge", err)
	}
	a.nothingSent(t)
}

func TestSendOwnerMessageReachesEveryOwnerAndNobodyElse(t *testing.T) {
	a := newOwnerAPI(t, nil, "+4917012345678", secondOwner)
	a.writeWorkspace(t, "photo.jpg", jpegBytes)
	owners, err := ownerRecipients(a.bot.initial, testOwn)
	if err != nil || len(owners) != 2 {
		t.Fatalf("owner recipients = %v, %v", owners, err)
	}

	if _, err := a.bot.SendOwnerMessage(context.Background(), "look", "photo.jpg"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if up := uploadsOf(a.fake); len(up) != 1 || up[0].mediaType != whatsmeow.MediaImage {
		t.Fatalf("uploads = %+v, want the file uploaded once", up)
	}
	var to []types.JID
	for _, m := range sentTexts(a.fake) {
		if m.msg.GetImageMessage() == nil {
			t.Fatalf("a message that is not the image was sent: %+v", m.msg)
		}
		to = append(to, m.to)
	}
	if len(to) != 2 || !slices.Contains(to, owners[0]) || !slices.Contains(to, owners[1]) || to[0] == to[1] {
		t.Fatalf("recipients = %v, want exactly the two owners %v", to, owners)
	}
	if got := sentTexts(a.fake); got[0].id == got[1].id {
		t.Fatal("both owners received the same message ID")
	}

	// A text goes to both as well, and a failure for the first does not stop the second.
	before := len(sentTexts(a.fake))
	a.fake.with(func(f *fakeClient) { f.sendErrs = []error{errors.New("boom")} })
	res, err := a.bot.SendOwnerMessage(context.Background(), "hello", "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("text send with a failing first owner = %v, want the first error", err)
	}
	if got := sentTexts(a.fake); len(got) != before+1 || got[before].to != owners[1] || !res.TextSent {
		t.Fatalf("sends after the failure = %+v, result = %+v; want the second owner served and TextSent set", got[before:], res)
	}
}

func TestSendOwnerMessageSendsLongTextBeforeTheFile(t *testing.T) {
	ctx := context.Background()
	long := strings.Repeat("word ", 300) // 1500 runes: more than a caption holds
	a := newOwnerAPI(t, nil)
	a.writeWorkspace(t, "photo.jpg", jpegBytes)
	a.writeWorkspace(t, "note.mp3", []byte("ID3 fake mp3"))

	res, err := a.bot.SendOwnerMessage(ctx, long, "photo.jpg")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	sent := sentTexts(a.fake)
	if len(sent) != 2 || sent[0].text == "" || sent[1].msg.GetImageMessage() == nil || sent[1].msg.GetImageMessage().Caption != nil {
		t.Fatalf("sent = %+v, want the text and then an image without a caption", sent)
	}
	if !res.TextSent || res.Kind != "image" || res.File != "photo.jpg" {
		t.Fatalf("result = %+v", res)
	}

	// Audio has no caption: the text always goes first, however short.
	res, err = a.bot.SendOwnerMessage(ctx, "listen", "note.mp3")
	if err != nil {
		t.Fatalf("send audio: %v", err)
	}
	sent = sentTexts(a.fake)[2:]
	if len(sent) != 2 || sent[0].text != "listen" || sent[1].msg.GetAudioMessage() == nil || !res.TextSent || res.Kind != "audio" {
		t.Fatalf("sent = %+v, result = %+v; want the text and then the audio file", sent, res)
	}

	// A short text rides along as the caption.
	res, err = a.bot.SendOwnerMessage(ctx, "look", "photo.jpg")
	if err != nil {
		t.Fatalf("send with caption: %v", err)
	}
	sent = sentTexts(a.fake)[4:]
	if len(sent) != 1 || sent[0].msg.GetImageMessage().GetCaption() != "look" || res.TextSent {
		t.Fatalf("sent = %+v, result = %+v; want one image captioned look", sent, res)
	}

	// A file that fails after its text keeps TextSent, so the tool can say so.
	a.fake.with(func(f *fakeClient) { f.sendErrs = []error{nil, errors.New("boom")} })
	res, err = a.bot.SendOwnerMessage(ctx, long, "photo.jpg")
	if err == nil || !res.TextSent {
		t.Fatalf("result = %+v, error = %v; want TextSent with the file's error", res, err)
	}
}

func TestSendOwnerAttachmentSendsBytes(t *testing.T) {
	secret := "attachment-secret-value"
	defer security.RegisterScopedSensitiveExact(secret)()
	a := newOwnerAPI(t, nil)

	res, err := a.bot.SendOwnerAttachment(context.Background(), "**front door** "+secret, OwnerAttachment{Name: "door.jpg", Data: jpegBytes})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if res.Kind != "image" || res.File != "door.jpg" || res.TextSent {
		t.Fatalf("result = %+v", res)
	}
	sent := sentTexts(a.fake)
	img := sent[0].msg.GetImageMessage()
	if len(sent) != 1 || img == nil || img.GetCaption() != "*front door* [redacted]" || sent[0].to != ownerJID {
		t.Fatalf("sent = %+v, want one image with the formatted, scrubbed caption", sent)
	}
	if up := uploadsOf(a.fake); len(up) != 1 || !bytes.Equal(up[0].data, jpegBytes) {
		t.Fatalf("uploads = %+v", up)
	}

	// A caption that does not fit goes first as a message of its own.
	res, err = a.bot.SendOwnerAttachment(context.Background(), strings.Repeat("x", 1500), OwnerAttachment{Name: "door.jpg", Data: jpegBytes})
	if err != nil || !res.TextSent {
		t.Fatalf("long caption: result = %+v, error = %v", res, err)
	}
	last := sentTexts(a.fake)
	if text := last[len(last)-2]; utf8.RuneCountInString(text.text) != 1500 || last[len(last)-1].msg.GetImageMessage().Caption != nil {
		t.Fatalf("sent = %+v, want the long text and then a bare image", last[1:])
	}

	if _, err := a.bot.SendOwnerAttachment(context.Background(), "x", OwnerAttachment{Name: "big.bin", Data: make([]byte, maxMediaBytes+1)}); !errors.Is(err, ErrMediaTooLarge) {
		t.Fatalf("attachment over 50 MB = %v, want ErrMediaTooLarge", err)
	}
}

func TestOwnerSendErrorsAreMasked(t *testing.T) {
	a := newOwnerAPI(t, nil)
	a.writeWorkspace(t, "report.pdf", []byte("%PDF-1.4 fake"))
	cause := fmt.Errorf("%w: no LID found for 4917012345678@s.whatsapp.net", whatsmeow.ErrRecipientNotOnWhatsApp)

	for name, send := range map[string]func() error{
		"text": func() error { _, err := a.bot.SendOwnerMessage(context.Background(), "hi", ""); return err },
		"file": func() error { _, err := a.bot.SendOwnerMessage(context.Background(), "hi", "report.pdf"); return err },
		"attachment": func() error {
			_, err := a.bot.SendOwnerAttachment(context.Background(), "hi", OwnerAttachment{Name: "p.jpg", Data: jpegBytes})
			return err
		},
	} {
		a.fake.with(func(f *fakeClient) { f.sendErrs = []error{cause} })
		err := send()
		if err == nil {
			t.Fatalf("%s: send succeeded", name)
		}
		if text := err.Error(); !strings.Contains(text, "***5678") || strings.Contains(text, "4917012345678") {
			t.Fatalf("%s: error text %q must hold the masked number only", name, text)
		}
		if !errors.Is(err, ErrRecipientNotOnWhatsApp) {
			t.Fatalf("%s: error %v lost errors.Is(ErrRecipientNotOnWhatsApp)", name, err)
		}
	}

	// A refusal keeps its sentinel too.
	a.fake.with(func(f *fakeClient) { f.loggedIn = false })
	if _, err := a.bot.SendOwnerMessage(context.Background(), "hi", ""); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("masked refusal = %v, want errors.Is(ErrNotConnected)", err)
	}
}

// The owner API is the one place an agent path meets a file system. It opens that path through
// tools.OpenOutgoingAttachment and no other way.
func TestOwnerAPIOpensNoPathItself(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "outbound_api.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	banned := map[string]bool{"Open": true, "OpenFile": true, "ReadFile": true, "OpenRoot": true}
	outgoing := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch {
		case pkg.Name == "os" && banned[sel.Sel.Name]:
			t.Errorf("outbound_api.go calls os.%s; paths open through tools.OpenOutgoingAttachment only", sel.Sel.Name)
		case pkg.Name == "tools" && sel.Sel.Name == "OpenOutgoingAttachment":
			outgoing++
		}
		return true
	})
	if outgoing != 1 {
		t.Fatalf("outbound_api.go calls tools.OpenOutgoingAttachment %d times, want exactly 1", outgoing)
	}
}

// Without a live configuration the gate cannot tell whether read-only was switched on since the
// generation started, so it refuses instead of trusting the generation's own copy.
func TestSendOwnerFailsClosedWithoutALiveConfig(t *testing.T) {
	ctx := context.Background()
	check := func(t *testing.T, a *ownerAPI) {
		t.Helper()
		if _, err := a.bot.SendOwnerMessage(ctx, "hi", ""); !errors.Is(err, ErrNotConnected) {
			t.Fatalf("text send = %v, want ErrNotConnected", err)
		}
		if _, err := a.bot.SendOwnerAttachment(ctx, "hi", OwnerAttachment{Name: "p.jpg", Data: jpegBytes}); !errors.Is(err, ErrNotConnected) {
			t.Fatalf("attachment send = %v, want ErrNotConnected", err)
		}
		a.nothingSent(t)
	}
	t.Run("no snapshot function", func(t *testing.T) {
		check(t, newOwnerAPIWith(t, ownerAPIOptions{noSnapshot: true}, nil))
	})
	t.Run("a snapshot without a config", func(t *testing.T) {
		a := newOwnerAPI(t, nil)
		a.live.Store(nil)
		check(t, a)
	})
}

// ctxHoldingClient holds every SendMessage until its context ends.
type ctxHoldingClient struct {
	*fakeClient
	started chan struct{}
	once    sync.Once
}

func (c *ctxHoldingClient) SendMessage(ctx context.Context, _ types.JID, _ *waE2E.Message, _ types.MessageID) (whatsmeow.SendResponse, error) {
	c.once.Do(func() { close(c.started) })
	<-ctx.Done()
	return whatsmeow.SendResponse{}, ctx.Err()
}

// A send that passed the gate belongs to its generation: when the Bot stops, the send stops,
// whatever context the caller gave it.
func TestOwnerSendEndsWithTheGeneration(t *testing.T) {
	var holder *ctxHoldingClient
	a := newOwnerAPIWith(t, ownerAPIOptions{wrap: func(f *fakeClient) waClient {
		holder = &ctxHoldingClient{fakeClient: f, started: make(chan struct{})}
		return holder
	}}, nil)

	done := make(chan error, 1)
	go func() {
		_, err := a.bot.SendOwnerMessage(context.Background(), "hi", "")
		done <- err
	}()
	select {
	case <-holder.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the send never reached the client")
	}
	a.bot.Stop()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("send after Stop = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a send outlived its generation")
	}
}
