package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/newspaper"
	"aurago/internal/tools"
	"aurago/internal/whatsapp"
)

// newspaperWhatsAppEdition is the edition shape of the Telegram link-preview fixture: a calm
// headline renders as a PDF, a Hebrew one has no PDF glyph coverage and falls back to text.
func newspaperWhatsAppEdition(headline string) newspaper.Edition {
	const link = "https://attacker.example/r?d=summary-of-the-mails"
	return newspaper.Edition{
		ID: "ed-1", LocalDate: "2026-10-28", Revision: 1, Title: "Test Daily",
		Stories: []newspaper.Story{{
			ID: "s1", Section: "News", Headline: headline, Deck: "See " + link,
			Paragraphs: []newspaper.Paragraph{{Text: "Details at " + link, SourceIDs: []string{"src1"}}},
			SourceIDs:  []string{"src1"},
		}},
		Sources: []newspaper.Source{{ID: "src1", URL: link, Publisher: "Example", Title: "Report"}},
	}
}

// newspaperWhatsAppConfig is a configuration in which an edition may go to WhatsApp.
func newspaperWhatsAppConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Newspaper.Enabled = true
	cfg.Newspaper.AllowWhatsApp = true
	cfg.WhatsApp.Enabled = true
	cfg.WhatsApp.Mode = config.WhatsAppModeDedicated
	cfg.WhatsApp.OwnerNumbers = []string{"+4917012345678"}
	return cfg
}

func newspaperDeliveryIsSafe(err error) bool {
	var safe *newspaper.SafeDeliveryError
	return errors.As(err, &safe)
}

func TestNewspaperWhatsAppSendsThePDFWithTheDigestAsCaption(t *testing.T) {
	fake := &fakeWhatsAppOwner{ready: true}
	useFakeWhatsAppOwner(t, fake)
	e := newspaperWhatsAppEdition("Calm headline")

	id, err := (&Server{}).sendNewspaperWhatsApp(context.Background(), e)
	if err != nil || id != "" {
		t.Fatalf("sendNewspaperWhatsApp = %q, %v; want an empty provider ID and no error", id, err)
	}
	prefix := "newspaper-2026-10-28-r1.pdf|" + newspaperDigest(e) + "|"
	if len(fake.attachments) != 1 || !strings.HasPrefix(fake.attachments[0], prefix) {
		t.Fatalf("attachments = %q, want one starting with %q", fake.attachments, prefix)
	}
	if size := strings.TrimPrefix(fake.attachments[0], prefix); size == "" || size == "0" {
		t.Fatalf("the PDF is empty: %q", fake.attachments[0])
	}
	if len(fake.messages) != 0 {
		t.Fatalf("messages = %q, want none: the digest is the caption", fake.messages)
	}
}

func TestNewspaperWhatsAppFallsBackToTextChunks(t *testing.T) {
	fake := &fakeWhatsAppOwner{ready: true}
	useFakeWhatsAppOwner(t, fake)
	e := newspaperWhatsAppEdition("שלום")

	if _, err := (&Server{}).sendNewspaperWhatsApp(context.Background(), e); err != nil {
		t.Fatalf("sendNewspaperWhatsApp: %v", err)
	}
	if len(fake.attachments) != 0 {
		t.Fatalf("attachments = %q, want none for an edition without PDF glyphs", fake.attachments)
	}
	if len(fake.messages) < 2 || fake.messages[0] != newspaperDigest(e) {
		t.Fatalf("messages = %q, want the digest first and at least one text chunk", fake.messages)
	}
	for i, message := range fake.messages {
		if n := utf8.RuneCountInString(message); n > newspaperDigestRunes {
			t.Errorf("message %d has %d runes, want at most %d", i, n, newspaperDigestRunes)
		}
	}
}

// cancelAfterFirstMessage cancels the delivery once the first text message went out.
type cancelAfterFirstMessage struct {
	*fakeWhatsAppOwner
	cancel context.CancelFunc
}

func (c cancelAfterFirstMessage) SendOwnerMessage(ctx context.Context, text, filePath string) (tools.WhatsAppSendResult, error) {
	res, err := c.fakeWhatsAppOwner.SendOwnerMessage(ctx, text, filePath)
	c.cancel()
	return res, err
}

func TestNewspaperWhatsAppFailuresAreSafeOnlyBeforeSending(t *testing.T) {
	boom := errors.New("boom")
	// Like the Bot's errors: wrapped, so only errors.Is can find the sentinel.
	readOnly := fmt.Errorf("send owner attachment: %w", whatsapp.ErrReadOnly)
	tooLarge := fmt.Errorf("send owner attachment: %w", whatsapp.ErrMediaTooLarge)
	pdf, text := newspaperWhatsAppEdition("Calm headline"), newspaperWhatsAppEdition("שלום")

	for _, tc := range []struct {
		name        string
		fake        *fakeWhatsAppOwner
		edition     newspaper.Edition
		wantSafe    bool
		wantUnknown bool
	}{
		{"not ready", &fakeWhatsAppOwner{ready: false}, pdf, true, false},
		{"pdf, read-only refusal", &fakeWhatsAppOwner{ready: true, attachErr: readOnly}, pdf, true, false},
		{"pdf, file over 50 MB", &fakeWhatsAppOwner{ready: true, attachErr: tooLarge}, pdf, true, false},
		{"pdf, unknown failure", &fakeWhatsAppOwner{ready: true, attachErr: boom}, pdf, false, true},
		{"pdf, cancelled by the API", &fakeWhatsAppOwner{ready: true, attachErr: context.Canceled}, pdf, false, true},
		{"text, failure after the first message", &fakeWhatsAppOwner{ready: true, msgErrs: []error{nil, boom}}, text, false, true},
		{"text, read-only refusal of the first message", &fakeWhatsAppOwner{ready: true, msgErrs: []error{whatsapp.ErrReadOnly}}, text, true, false},
		{"text, read-only refusal after the first message", &fakeWhatsAppOwner{ready: true, msgErrs: []error{nil, whatsapp.ErrReadOnly}}, text, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useFakeWhatsAppOwner(t, tc.fake)
			_, err := (&Server{}).sendNewspaperWhatsApp(context.Background(), tc.edition)
			if err == nil {
				t.Fatal("the delivery reported success")
			}
			if got := newspaperDeliveryIsSafe(err); got != tc.wantSafe {
				t.Fatalf("safe = %v, want %v (err: %v)", got, tc.wantSafe, err)
			}
			if got := strings.Contains(err.Error(), "outcome unknown"); got != tc.wantUnknown {
				t.Fatalf("outcome-unknown text = %v, want %v (err: %v)", got, tc.wantUnknown, err)
			}
		})
	}

	t.Run("a context that ended before the first send is safe", func(t *testing.T) {
		fake := &fakeWhatsAppOwner{ready: true}
		useFakeWhatsAppOwner(t, fake)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for _, e := range []newspaper.Edition{pdf, text} {
			_, err := (&Server{}).sendNewspaperWhatsApp(ctx, e)
			if !newspaperDeliveryIsSafe(err) || !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want a safe cancellation", err)
			}
		}
		if len(fake.attachments)+len(fake.messages) != 0 {
			t.Fatalf("something was sent: %q %q", fake.attachments, fake.messages)
		}
	})
	t.Run("a context that ended after the first message is uncertain", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		fake := &fakeWhatsAppOwner{ready: true}
		previous := serverWhatsAppOwner
		serverWhatsAppOwner = func(*Server) whatsAppOwnerAPI { return cancelAfterFirstMessage{fake, cancel} }
		t.Cleanup(func() { serverWhatsAppOwner = previous })
		_, err := (&Server{}).sendNewspaperWhatsApp(ctx, text)
		if err == nil || newspaperDeliveryIsSafe(err) || !strings.Contains(err.Error(), "outcome unknown") || len(fake.messages) != 1 {
			t.Fatalf("err = %v, messages = %d; want an uncertain failure after one message", err, len(fake.messages))
		}
	})
}

func TestNewspaperWhatsAppGatesAndDestination(t *testing.T) {
	e := newspaperWhatsAppEdition("Calm headline")
	for _, tc := range []struct {
		name  string
		tweak func(*config.Config)
		ready bool // the fake reports a running Bot
		sends bool
	}{
		{"all gates open", func(*config.Config) {}, true, true},
		{"allow switch off", func(c *config.Config) { c.Newspaper.AllowWhatsApp = false }, true, false},
		{"WhatsApp read-only", func(c *config.Config) { c.WhatsApp.ReadOnly = true }, true, false},
		{"WhatsApp disabled", func(c *config.Config) { c.WhatsApp.Enabled = false }, true, false},
		{"Newspaper read-only", func(c *config.Config) { c.Newspaper.ReadOnly = true }, true, false},
		{"Newspaper disabled", func(c *config.Config) { c.Newspaper.Enabled = false }, true, false},
		{"no running generation", func(*config.Config) {}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeWhatsAppOwner{ready: tc.ready}
			useFakeWhatsAppOwner(t, fake)
			cfg := newspaperWhatsAppConfig()
			tc.tweak(cfg)
			_, err := (&Server{Cfg: cfg}).newspaperDelivery(context.Background(), e, newspaper.Profile{}, "whatsapp")
			if tc.sends {
				if err != nil || len(fake.attachments) != 1 {
					t.Fatalf("err = %v, attachments = %q; want one PDF", err, fake.attachments)
				}
				return
			}
			if !errors.Is(err, newspaper.ErrDisabled) || !newspaperDeliveryIsSafe(err) {
				t.Fatalf("err = %v, want a safe ErrDisabled", err)
			}
			if len(fake.attachments)+len(fake.messages) != 0 {
				t.Fatalf("a refused delivery sent %q %q", fake.attachments, fake.messages)
			}
		})
	}

	t.Run("destination", func(t *testing.T) {
		dedicated := newspaperWhatsAppConfig()
		dedicated.WhatsApp.OwnerNumbers = []string{"+4917012345678", "+4917099998888", "+4917012345678", "not a number"}
		got, err := newspaperWhatsAppDestination(dedicated)
		if want := "dedicated\x00" + "5678,8888"; err != nil || got != want {
			t.Fatalf("dedicated destination = %q, %v; want %q", got, err, want)
		}
		for _, full := range []string{"4917012345678", "4917099998888", "17012345678", "7012345678"} {
			if strings.Contains(got, full) {
				t.Errorf("the destination carries the number %q", full)
			}
		}
		self := newspaperWhatsAppConfig()
		self.WhatsApp.Mode = config.WhatsAppModeSelfChat
		self.WhatsApp.OwnerNumbers = []string{"+4917012345678"}
		if got, err := newspaperWhatsAppDestination(self); err != nil || got != "self_chat" {
			t.Fatalf("self-chat destination = %q, %v; want self_chat", got, err)
		}
		none := newspaperWhatsAppConfig()
		none.WhatsApp.OwnerNumbers = []string{"12", ""}
		if got, err := newspaperWhatsAppDestination(none); err == nil {
			t.Fatalf("a destination without a valid owner = %q, want an error", got)
		}
	})
}

func TestNewspaperWhatsAppTestSharesTheWhatsAppTestMinute(t *testing.T) {
	whatsAppTestLast = time.Time{}
	t.Cleanup(func() { whatsAppTestLast = time.Time{} })
	fake := &fakeWhatsAppOwner{ready: true}
	useFakeWhatsAppOwner(t, fake)
	s := &Server{}
	ctx := context.Background()
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	// A refused request must not use up the minute.
	for name, tweak := range map[string]func(*config.Config){
		"Newspaper read-only": func(c *config.Config) { c.Newspaper.ReadOnly = true },
		"desktop read-only":   func(c *config.Config) { c.VirtualDesktop.ReadOnly = true },
		"WhatsApp read-only":  func(c *config.Config) { c.WhatsApp.ReadOnly = true },
		"allow switch off":    func(c *config.Config) { c.Newspaper.AllowWhatsApp = false },
	} {
		cfg := newspaperWhatsAppConfig()
		tweak(cfg)
		if err := s.sendNewspaperWhatsAppTest(ctx, cfg, now); !errors.Is(err, newspaper.ErrDisabled) {
			t.Fatalf("%s: err = %v, want ErrDisabled", name, err)
		}
		if !whatsAppTestLast.IsZero() || len(fake.messages) != 0 {
			t.Fatalf("%s: a refused test claimed the minute or sent: %v %q", name, whatsAppTestLast, fake.messages)
		}
	}

	cfg := newspaperWhatsAppConfig()
	if err := s.sendNewspaperWhatsAppTest(ctx, cfg, now); err != nil || len(fake.messages) != 1 || fake.messages[0] != newspaperWhatsAppTestText {
		t.Fatalf("first test: err = %v, messages = %q; want the fixed test text", err, fake.messages)
	}
	if err := s.sendNewspaperWhatsAppTest(ctx, cfg, now.Add(30*time.Second)); !errors.Is(err, errNewspaperWhatsAppTestRateLimited) || len(fake.messages) != 1 {
		t.Fatalf("second test within a minute: err = %v, messages = %d; want the rate limit and no send", err, len(fake.messages))
	}
	if err := s.sendNewspaperWhatsAppTest(ctx, cfg, now.Add(time.Minute)); err != nil || len(fake.messages) != 2 {
		t.Fatalf("test after a minute: err = %v, messages = %d; want a second send", err, len(fake.messages))
	}
	// /api/whatsapp/test claims the same minute.
	if takeWhatsAppTestMinute(now.Add(time.Minute + time.Second)) {
		t.Fatal("the WhatsApp test endpoint did not share the newspaper's minute")
	}
}

func TestNewspaperWhatsAppRoutes(t *testing.T) {
	whatsAppTestLast = time.Time{}
	t.Cleanup(func() { whatsAppTestLast = time.Time{} })
	fake := &fakeWhatsAppOwner{ready: true}
	useFakeWhatsAppOwner(t, fake)
	s, _, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.Newspaper = newspaperWhatsAppConfig().Newspaper
	s.Cfg.WhatsApp = newspaperWhatsAppConfig().WhatsApp
	service, err := newspaper.New(newspaper.Options{
		Path:   filepath.Join(t.TempDir(), "newspaper.db"),
		Policy: func() newspaper.Policy { return newspaper.Policy{Enabled: true} },
		Research: func(context.Context, newspaper.Profile, time.Time, func(newspaper.Progress)) (newspaper.Draft, error) {
			return newspaper.Draft{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	s.Newspaper = service
	admin, _, err := s.TokenManager.Create("newspaper admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/desktop/newspaper/"+path, strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer "+admin)
		w := httptest.NewRecorder()
		s.handleNewspaper(w, r)
		return w
	}
	capability := func() (allowed, ready bool) {
		t.Helper()
		w := call(http.MethodGet, "capabilities")
		var caps map[string]any
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &caps) != nil {
			t.Fatalf("capabilities: %d %s", w.Code, w.Body.String())
		}
		allowed, _ = caps["whatsapp_allowed"].(bool)
		ready, _ = caps["whatsapp_ready"].(bool)
		return allowed, ready
	}

	if allowed, ready := capability(); !allowed || !ready {
		t.Fatalf("capabilities: whatsapp_allowed=%v whatsapp_ready=%v, want both true", allowed, ready)
	}
	fake.mu.Lock()
	fake.ready = false
	fake.mu.Unlock()
	if allowed, ready := capability(); !allowed || ready {
		t.Fatalf("without a running Bot: whatsapp_allowed=%v whatsapp_ready=%v, want true and false", allowed, ready)
	}
	if w := call(http.MethodPost, "whatsapp/test"); w.Code != http.StatusForbidden || len(fake.messages) != 0 {
		t.Fatalf("test send without a running Bot: %d %s", w.Code, w.Body.String())
	}
	fake.mu.Lock()
	fake.ready = true
	fake.mu.Unlock()
	if w := call(http.MethodPost, "whatsapp/test"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"sent"`) || len(fake.messages) != 1 {
		t.Fatalf("first test send: %d %s (messages %d)", w.Code, w.Body.String(), len(fake.messages))
	}
	if w := call(http.MethodPost, "whatsapp/test"); w.Code != http.StatusTooManyRequests || len(fake.messages) != 1 {
		t.Fatalf("second test send: %d %s (messages %d), want 429 and no second send", w.Code, w.Body.String(), len(fake.messages))
	}
}
