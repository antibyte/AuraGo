package detective

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// provisionalSession starts a running session on an existing case. A non-zero base
// revision marks it as a continue, deepen or update run.
func provisionalSession(t *testing.T, s *Service, key string, base int) *Session {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.getLocked(key)
	if err != nil {
		t.Fatal(err)
	}
	c.Run = Run{ID: id("run_"), Status: "running", Phase: "research", Profile: Profiles()["quick"], BaseRevision: base}
	if err = s.saveLocked(&c); err != nil {
		t.Fatal(err)
	}
	return &Session{service: s, CaseID: key, cancel: func() {}}
}

// A deepen or continue that ends without a full report keeps its findings but must not
// replace the last complete report as the default revision (audit 4.14).
func TestFailedDeepenKeepsCompleteAsDefault(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "Provisional", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	baseline := provisionalSession(t, s, c.ID, 0)
	finding := updateEvidence(t, baseline, 1)
	if err = baseline.Submit(updateReport(finding)); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestRevision != 1 || got.LatestCompleteRevision != 1 {
		t.Fatalf("baseline: latest=%d complete=%d", got.LatestRevision, got.LatestCompleteRevision)
	}

	deepen := provisionalSession(t, s, c.ID, 1)
	updateEvidence(t, deepen, 2)
	if err = deepen.complete(errors.New("provider failed")); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestRevision != 2 || got.LatestCompleteRevision != 1 {
		t.Fatalf("provisional: latest=%d complete=%d", got.LatestRevision, got.LatestCompleteRevision)
	}
	newest := got.Reports[len(got.Reports)-1]
	if !newest.Provisional || !newest.Partial || newest.Revision != 2 {
		t.Fatalf("newest report: %+v", newest)
	}
	live, err := s.Live(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live.LatestRevision != 2 || live.LatestCompleteRevision != 1 {
		t.Fatalf("live: latest=%d complete=%d", live.LatestRevision, live.LatestCompleteRevision)
	}

	// The last complete report is still the auto-update baseline.
	if _, err = s.SetAutoUpdate(c.ID, AutoUpdateSettings{Enabled: true, IntervalHours: 24, ReportStyle: "append", Effort: "quick"}); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AutoUpdate.Enabled || got.AutoUpdate.NextRunAt.IsZero() {
		t.Fatalf("auto-update not scheduled: %+v", got.AutoUpdate)
	}
}

func TestModelChangeKeepsResearchOnContinue(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "Provider switch", Effort: "quick", ProviderID: "old", Model: "old-model"})
	if err != nil {
		t.Fatal(err)
	}
	x := provisionalSession(t, s, c.ID, 0)
	if err = s.SetModel(c.ID, "replacement", ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("running case accepted a model change: %v", err)
	}
	if err = x.Iterate(); err != nil {
		t.Fatal(err)
	}
	sampleEvidence(t, x)
	if err = x.Checkpoint(Continuation{Provider: "old", Model: "old-model", Messages: json.RawMessage(`[{"role":"user","content":"test context"}]`)}); err != nil {
		t.Fatal(err)
	}
	if err = x.complete(errors.New("provider failed")); err != nil {
		t.Fatal(err)
	}
	before, err := s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetModel(c.ID, "replacement", ""); err != nil {
		t.Fatal(err)
	}
	s.SetRunner(testRunner(func(context.Context, *Session) error { return nil }))
	resumed, err := s.Start(c.ID, "continue", "maximum", "resume")
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Request.ProviderID != "replacement" || resumed.Request.Model != "" || resumed.Run.ID != before.Run.ID || resumed.Run.Profile != before.Run.Profile || resumed.Run.Usage != before.Run.Usage {
		t.Fatalf("continue lost the new selection or existing run state: %+v %+v", resumed.Request, resumed.Run)
	}
	if len(resumed.Sources) != 1 || len(resumed.Findings) != 1 || resumed.LatestRevision != before.LatestRevision {
		t.Fatalf("continue lost case evidence: sources=%d findings=%d revision=%d", len(resumed.Sources), len(resumed.Findings), resumed.LatestRevision)
	}
	checkpoint, err := s.Continuation(c.ID)
	if err != nil || checkpoint.Provider != "old" || checkpoint.Model != "old-model" {
		t.Fatalf("checkpoint identity changed: %+v, %v", checkpoint, err)
	}
}

// An update run bases its append revision on the last complete report, never on a
// provisional one.
func TestAppendUpdateBasesOnLatestCompleteRevision(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "Append base", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	baseline := provisionalSession(t, s, c.ID, 0)
	if err = baseline.Submit(updateReport(updateEvidence(t, baseline, 1))); err != nil {
		t.Fatal(err)
	}
	deepen := provisionalSession(t, s, c.ID, 1)
	updateEvidence(t, deepen, 2)
	if err = deepen.complete(errors.New("provider failed")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetAutoUpdate(c.ID, AutoUpdateSettings{Enabled: true, IntervalHours: 24, ReportStyle: "append", Effort: "quick"}); err != nil {
		t.Fatal(err)
	}
	s.SetRunner(testRunner(func(ctx context.Context, job *Session) error { return nil }))
	started, err := s.startLocked(c.ID, "update", "", "append-base")
	if err != nil {
		t.Fatal(err)
	}
	if started.Run.BaseRevision != 1 {
		t.Fatalf("append base=%d, want the last complete revision 1", started.Run.BaseRevision)
	}
}

// A provisional revision stays addressable: its export is immutable and independent of
// the complete revision's export.
func TestExportsUseTheSelectedProvisionalRevision(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "Provisional export", Effort: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	baseline := provisionalSession(t, s, c.ID, 0)
	if err = baseline.Submit(updateReport(updateEvidence(t, baseline, 1))); err != nil {
		t.Fatal(err)
	}
	deepen := provisionalSession(t, s, c.ID, 1)
	updateEvidence(t, deepen, 2)
	if err = deepen.complete(errors.New("provider failed")); err != nil {
		t.Fatal(err)
	}
	full, err := s.ExportRevision(context.Background(), c.ID, 1, "md")
	if err != nil {
		t.Fatal(err)
	}
	provisional, err := s.ExportRevision(context.Background(), c.ID, 2, "md")
	if err != nil {
		t.Fatal(err)
	}
	if string(full.Data) == string(provisional.Data) {
		t.Fatal("provisional export duplicated the complete revision")
	}
	again, err := s.ExportRevision(context.Background(), c.ID, 2, "md")
	if err != nil {
		t.Fatal(err)
	}
	if string(again.Data) != string(provisional.Data) || again.SHA256 != provisional.SHA256 {
		t.Fatal("a cached export changed")
	}
}
