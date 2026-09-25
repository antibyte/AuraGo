package newspaper

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testDraft(now time.Time) Draft {
	quote := "The city council approved a new public library on Tuesday."
	source := Source{ID: "src-1", URL: "https://example.org/library", Publisher: "Example News", Title: "New public library", RetrievedAt: now, Excerpt: quote + " The project is scheduled to begin next year after a public design process."}
	story := Story{ID: "story-1", Section: "culture", Headline: "Council approves new library", Deck: "A public library is planned.", SourceIDs: []string{source.ID}, SingleSource: true, Paragraphs: []Paragraph{{Text: "The council approved a new library.", SourceIDs: []string{source.ID}, EvidenceQuote: quote}}}
	return Draft{Stories: []Story{story}, Sources: []Source{source}}
}

func TestProfileAndDraftValidation(t *testing.T) {
	p := DefaultProfile()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	p.Name = "News\r\nBcc: victim@example.org"
	if p.Validate() == nil {
		t.Fatal("header newline accepted in publication name")
	}
	p = DefaultProfile()
	p.Sections = []string{"regional"}
	p.City, p.Region = "", ""
	if p.Validate() == nil {
		t.Fatal("regional coverage without place accepted")
	}
	p = DefaultProfile()
	d := testDraft(time.Now().UTC())
	p.RSSFeeds = []RSSFeed{{URL: "https://example.org/feed.xml", Section: "culture"}}
	if err := p.Validate(); err != nil {
		t.Fatalf("valid RSS feed: %v", err)
	}
	p.RSSFeeds[0].Section = "regional"
	if p.Validate() == nil {
		t.Fatal("RSS feed for an unselected section accepted")
	}
	p.RSSFeeds = nil
	if err := ValidateDraft(d, p, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	d.Stories[0].Paragraphs[0].EvidenceQuote = "This passage was never retrieved from the source"
	if ValidateDraft(d, p, time.Now().UTC()) == nil {
		t.Fatal("invented passage accepted")
	}
	d = testDraft(time.Now().UTC())
	d.Stories[0].SourceIDs = nil
	if ValidateDraft(d, p, time.Now().UTC()) == nil {
		t.Fatal("omitted story citation accepted")
	}
	d = testDraft(time.Now().UTC())
	d.Sources[0].URL = "javascript:alert(1)"
	if ValidateDraft(d, p, time.Now().UTC()) == nil {
		t.Fatal("unsafe source link accepted")
	}
}

func TestStoreRevisionPublicationAndDeliveryIdempotency(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "newspaper.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Profile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "The Test Edition"
	if _, err = s.SaveProfile(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveProfile(ctx, p); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale profile: %v", err)
	}
	now := time.Now().UTC()
	run, err := s.Start(ctx, "2026-09-25", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Start(ctx, "2026-09-25", false, now); !errors.Is(err, ErrBusy) {
		t.Fatalf("overlap: %v", err)
	}
	draft := testDraft(now)
	e := Edition{ID: run.ID, LocalDate: run.LocalDate, Revision: run.Revision, Title: p.Name, CreatedAt: now, CutoffAt: now, Stories: draft.Stories, Sources: draft.Sources}
	if err = e.Seal(); err != nil {
		t.Fatal(err)
	}
	if err = s.Publish(ctx, run, e); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Start(ctx, "2026-09-25", false, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("implicit revision: %v", err)
	}
	got, err := s.Get(ctx, e.ID)
	if err != nil || got.Hash != e.Hash || got.Title != "The Test Edition" {
		t.Fatalf("published snapshot: %#v, %v", got, err)
	}
	target := strings.Repeat("a", 64)
	d, claimed, err := s.ClaimDelivery(ctx, e, "email", "manual", target, "request-1234567890")
	if err != nil || !claimed {
		t.Fatalf("first claim: %#v %v %v", d, claimed, err)
	}
	if err = s.FinishDelivery(ctx, d, "sent", "", "provider-42"); err != nil {
		t.Fatal(err)
	}
	replay, claimed, err := s.ClaimDelivery(ctx, e, "email", "manual", target, "request-1234567890")
	if err != nil || claimed || replay.ID != d.ID || replay.Status != "sent" || replay.ProviderID != "provider-42" {
		t.Fatalf("replay: %#v %v %v", replay, claimed, err)
	}
	pending, claimed, err := s.ClaimDelivery(ctx, e, "telegram", "manual", target, "request-1234567891")
	if err != nil || !claimed || pending.Status != "sending" {
		t.Fatalf("pending claim: %#v %v %v", pending, claimed, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err = s.Get(ctx, e.ID)
	if err != nil || got.Hash != e.Hash {
		t.Fatalf("reopen: %#v %v", got, err)
	}
	deliveries, err := s.Deliveries(ctx, e.ID)
	if err != nil || len(deliveries) != 2 || deliveries[0].Status != "uncertain" {
		t.Fatalf("reconciled delivery: %#v %v", deliveries, err)
	}
	if _, err = s.Start(ctx, "2026-09-25", true, now); err != nil {
		t.Fatalf("explicit revision: %v", err)
	}
}

func TestInterruptedRunAndDST(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "newspaper.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.Start(ctx, "2026-09-25", false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordRunSource(ctx, run.ID, testDraft(time.Now().UTC()).Sources[0]); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	recovered, err := s.Run(ctx, run.ID)
	if err != nil || recovered.Status != "interrupted" {
		t.Fatalf("restart: %#v %v", recovered, err)
	}
	sources, err := s.RunSources(ctx, run.ID)
	if err != nil || len(sources) != 1 || sources[0].ID != "src-1" {
		t.Fatalf("retained evidence: %#v %v", sources, err)
	}
	loc, _ := time.LoadLocation("Europe/Berlin")
	gap, err := readyInstant("2026-03-29", "02:30", loc)
	if err != nil || gap.In(loc).Format("15:04") != "03:00" {
		t.Fatalf("DST gap: %s %v", gap.In(loc), err)
	}
	repeated, err := readyInstant("2026-10-25", "02:30", loc)
	if err != nil || repeated.In(loc).Format("15:04 -0700") != "02:30 +0200" {
		t.Fatalf("DST repeated minute: %s %v", repeated.In(loc), err)
	}
}

func TestEditionRenderersPreserveSourcesAndRejectUnsupportedPDFGlyphs(t *testing.T) {
	draft := testDraft(time.Now().UTC())
	e := Edition{Title: "Der Morgen", Language: "de", LocalDate: "2026-09-25", Place: "Berlin", Revision: 1, Stories: draft.Stories, Sources: draft.Sources}
	plain := Text(e)
	rich := HTML(e)
	if !strings.Contains(plain, draft.Sources[0].URL) || !strings.Contains(rich, draft.Sources[0].URL) || !strings.Contains(rich, "Bericht aus einer Quelle") {
		t.Fatal("a delivery format omitted source context")
	}
	pdf, err := PDF(e)
	if err != nil || !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("local PDF rendering failed: %v", err)
	}
	e.Title = "每日新闻"
	if _, err := PDF(e); err == nil {
		t.Fatal("unsupported PDF glyphs were rendered without a fallback signal")
	}
}

func TestDeliveryFailureReceiptDistinguishesSafeAndUncertain(t *testing.T) {
	ctx := context.Background()
	var sendErr error
	calls := 0
	s, err := New(Options{
		Path:        filepath.Join(t.TempDir(), "newspaper.db"),
		Policy:      func() Policy { return Policy{Enabled: true, Telegram: true} },
		Research:    func(context.Context, Profile, time.Time, func(Progress)) (Draft, error) { return Draft{}, nil },
		Destination: func(context.Context, Profile, string) (string, error) { return "chat-1", nil },
		Deliver:     func(context.Context, Edition, Profile, string) (string, error) { calls++; return "", sendErr },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	run, err := s.Store().Start(ctx, "2026-09-25", false, now)
	if err != nil {
		t.Fatal(err)
	}
	draft := testDraft(now)
	e := Edition{ID: run.ID, LocalDate: run.LocalDate, Revision: run.Revision, Title: "News", CreatedAt: now, CutoffAt: now, Stories: draft.Stories, Sources: draft.Sources}
	if err := e.Seal(); err != nil {
		t.Fatal(err)
	}
	if err := s.Store().Publish(ctx, run, e); err != nil {
		t.Fatal(err)
	}
	sendErr = SafeDelivery(errors.New("not sent"))
	failed, err := s.Deliver(ctx, e.ID, "telegram", "manual", "safe-request-12345")
	if err == nil || failed.Status != "failed" {
		t.Fatalf("safe failure: %+v %v", failed, err)
	}
	sendErr = errors.New("transport outcome unknown")
	unknown, err := s.Deliver(ctx, e.ID, "telegram", "manual", "unknown-request-1")
	if err == nil || unknown.Status != "uncertain" {
		t.Fatalf("uncertain failure: %+v %v", unknown, err)
	}
	_, err = s.Deliver(ctx, e.ID, "telegram", "manual", "unknown-request-1")
	if err != nil || calls != 2 {
		t.Fatalf("idempotent replay sent again: calls=%d err=%v", calls, err)
	}
}

func TestCorrectionCreatesLabeledImmutableRevision(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	s, err := New(Options{
		Path:   filepath.Join(t.TempDir(), "newspaper.db"),
		Policy: func() Policy { return Policy{Enabled: true} },
		Now:    func() time.Time { return now },
		Research: func(context.Context, Profile, time.Time, func(Progress)) (Draft, error) {
			return testDraft(now), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	date := now.Format("2006-01-02")
	if _, err := s.Store().StartCorrected(ctx, date, false, "Incorrect date", now); err == nil {
		t.Fatal("correction without a published edition was accepted")
	}
	first, err := s.Store().Start(ctx, date, false, now)
	if err != nil {
		t.Fatal(err)
	}
	s.execute(ctx, first, DefaultProfile())
	note := "The earlier headline stated the wrong opening date."
	revised, err := s.Store().StartCorrected(ctx, date, true, note, now)
	if err != nil {
		t.Fatal(err)
	}
	s.execute(ctx, revised, DefaultProfile())
	previous, err := s.Get(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	corrected, err := s.Get(ctx, revised.ID)
	if err != nil {
		t.Fatal(err)
	}
	if revised.CorrectionNote != note || corrected.Revision != 2 || len(corrected.Corrections) != 1 || corrected.Corrections[0] != note || len(previous.Corrections) != 0 || previous.Hash == corrected.Hash {
		t.Fatalf("correction revision: run=%+v previous=%+v corrected=%+v", revised, previous.Corrections, corrected.Corrections)
	}
}

func TestRSSProfileRequiresSelectedSectionAndPlainURL(t *testing.T) {
	p := DefaultProfile()
	p.RSSFeeds = []RSSFeed{{URL: "https://news.example/feed.xml", Section: "science"}}
	if err := p.Validate(); err != nil {
		t.Fatalf("valid RSS feed rejected: %v", err)
	}
	for _, feeds := range [][]RSSFeed{
		{{URL: "https://news.example/feed.xml#entry", Section: "science"}},
		{{URL: "https://user:pass@news.example/feed.xml", Section: "science"}},
		{{URL: "https://news.example/feed.xml", Section: "regional"}},
		{{URL: "https://news.example/feed.xml", Section: "science"}, {URL: "https://NEWS.example/feed.xml", Section: "science"}},
	} {
		invalid := DefaultProfile()
		invalid.RSSFeeds = feeds
		if err := invalid.Validate(); err == nil {
			t.Fatalf("invalid RSS configuration accepted: %+v", feeds)
		}
	}
}
