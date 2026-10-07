package newspaper

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSchedulerMigrationBacksUpLegacyCopy(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "legacy.db")
	store, err := Open(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`DROP TABLE newspaper_scheduler_attempts; PRAGMA user_version=1`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(root, "migration-copy.db")
	if err := os.WriteFile(copyPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	migrated, err := Open(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	backups, err := filepath.Glob(copyPath + ".pre-v2-*.db")
	if err != nil || len(backups) != 1 {
		t.Fatalf("migration backups = %v, %v", backups, err)
	}
	for path, want := range map[string]int{legacy: 1, backups[0]: 1, copyPath: 2} {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		var version int
		err = db.QueryRow("PRAGMA user_version").Scan(&version)
		_ = db.Close()
		if err != nil || version != want {
			t.Fatalf("%s version = %d, %v; want %d", path, version, err, want)
		}
	}
}

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

func TestValidateDraftAllowsThirtyOneStories(t *testing.T) {
	now := time.Now().UTC()
	draft := testDraft(now)
	for i := 2; i <= 31; i++ {
		story := draft.Stories[0]
		story.ID = "story-" + strconv.Itoa(i)
		story.Headline = "Council approves library " + strconv.Itoa(i)
		draft.Stories = append(draft.Stories, story)
	}
	if err := ValidateDraft(draft, DefaultProfile(), now); err != nil {
		t.Fatalf("31-story edition rejected: %v", err)
	}
	draft.Stories = append(draft.Stories, draft.Stories[0])
	if err := ValidateDraft(draft, DefaultProfile(), now); err == nil {
		t.Fatal("32-story edition was accepted")
	}
}

func TestPublishReloadAndRenderThirtyOneSourcedStories(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	p := DefaultProfile()
	p.Language = "en"
	draft := Draft{Stories: make([]Story, 0, 31), Sources: make([]Source, 0, 31)}
	for i := 1; i <= 31; i++ {
		suffix := strconv.Itoa(i)
		id := "source-" + suffix
		quote := "The verified report " + suffix + " records a confirmed public update."
		draft.Sources = append(draft.Sources, Source{
			ID: id, URL: "https://publisher-" + suffix + ".example/story/" + suffix,
			Publisher: "Publisher " + suffix, Title: "Verified update " + suffix,
			RetrievedAt: now, Excerpt: quote + " Additional verified reporting provides useful context for readers.",
		})
		draft.Stories = append(draft.Stories, Story{
			ID: "story-" + suffix, Section: "culture", Headline: "Verified update " + suffix,
			Deck: "A sourced update for the daily edition.", SourceIDs: []string{id}, SingleSource: true,
			Paragraphs: []Paragraph{{Text: quote, SourceIDs: []string{id}, EvidenceQuote: quote}},
		})
	}
	if err := ValidateDraft(draft, p, now); err != nil {
		t.Fatalf("validate 31 stories: %v", err)
	}
	store, err := Open(filepath.Join(t.TempDir(), "newspaper.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	run, err := store.Start(ctx, now.Format("2006-01-02"), false, now)
	if err != nil {
		t.Fatal(err)
	}
	edition := Edition{ID: run.ID, LocalDate: run.LocalDate, Revision: run.Revision, Title: p.Name, Language: p.Language, CreatedAt: now, CutoffAt: now, Stories: draft.Stories, Sources: draft.Sources}
	if err := edition.Seal(); err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(ctx, run, edition); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(ctx, edition.ID)
	if err != nil || len(loaded.Stories) != 31 || len(loaded.Sources) != 31 {
		t.Fatalf("reload: stories=%d sources=%d err=%v", len(loaded.Stories), len(loaded.Sources), err)
	}
	if got := strings.Count(HTML(loaded), "<article "); got != 31 {
		t.Fatalf("HTML rendered %d stories", got)
	}
	if text := Text(loaded); !strings.Contains(text, "Verified update 31") {
		t.Fatal("plain text omitted the last story")
	}
	pdf, err := PDF(loaded)
	if err != nil || len(pdf) < 5 || string(pdf[:5]) != "%PDF-" {
		t.Fatalf("PDF render: %v", err)
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

func TestFailedManualRunDoesNotBlockOneScheduledAttempt(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 25, 6, 30, 0, 0, time.UTC)
	s, err := New(Options{
		Path:   filepath.Join(t.TempDir(), "newspaper.db"),
		Policy: func() Policy { return Policy{Enabled: true} },
		Research: func(context.Context, Profile, time.Time, func(Progress)) (Draft, error) {
			return Draft{}, errors.New("test research failure")
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.Profile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.Daily = true
	p.ReadyTime = "08:30"
	date := "2026-09-25"
	first, err := s.Store().Start(ctx, date, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Store().RecordRunSource(ctx, first.ID, testDraft(now).Sources[0], maxResearchSources); err != nil {
		t.Fatal(err)
	}
	first.Status = "failed"
	if err = s.Store().UpdateRun(ctx, first, "Research failed"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Store().SaveProfile(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.tick()
	latest, err := s.LatestRun(ctx)
	if err != nil || latest.ID == first.ID || latest.Revision != 2 {
		t.Fatalf("daily scheduler did not start after failed manual run: %+v, %v", latest, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		latest, err = s.LatestRun(ctx)
		if err == nil && latest.Status == "failed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if latest.Status != "failed" {
		t.Fatalf("scheduled run did not finish: %+v, %v", latest, err)
	}
	s.tick()
	afterTick, err := s.LatestRun(ctx)
	if err != nil || afterTick.ID != latest.ID {
		t.Fatalf("daily scheduler repeated its attempt: %+v, %v", afterTick, err)
	}
	var retry Run
	for time.Now().Before(deadline) {
		retry, err = s.Start(ctx, false)
		if err != ErrBusy {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || retry.Revision != 3 || retry.ID == first.ID {
		t.Fatalf("manual retry: %+v, %v", retry, err)
	}
	previous, err := s.Run(ctx, first.ID)
	if err != nil || previous.Status != "failed" {
		t.Fatalf("previous run: %+v, %v", previous, err)
	}
	sources, err := s.RunSources(ctx, first.ID)
	if err != nil || len(sources) != 1 || sources[0].ID != "src-1" {
		t.Fatalf("previous run evidence: %+v, %v", sources, err)
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
	if _, _, err := s.RecordRunSource(ctx, run.ID, testDraft(time.Now().UTC()).Sources[0], maxResearchSources); err != nil {
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
	retry, err := s.Start(ctx, "2026-09-25", false, time.Now())
	if err != nil || retry.Revision != 2 {
		t.Fatalf("interrupted run retry: %+v %v", retry, err)
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
	s.execute(ctx, ctx, first, DefaultProfile())
	note := "The earlier headline stated the wrong opening date."
	revised, err := s.Store().StartCorrected(ctx, date, true, note, now)
	if err != nil {
		t.Fatal(err)
	}
	s.execute(ctx, ctx, revised, DefaultProfile())
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
