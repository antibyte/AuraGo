package flows

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"aurago/internal/security"
)

// noteFamilyEmoji is a ZWJ sequence (woman, ZWJ, girl). It must stay valid.
var noteFamilyEmoji = string([]rune{0x1F469, 0x200D, 0x1F467})

// noteBidiText holds U+202E (RIGHT-TO-LEFT OVERRIDE), a format character the rule refuses.
var noteBidiText = "pay " + string(rune(0x202E)) + "evil"

func hasIssue(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func stickyNote(id, text string) StickyNote {
	return StickyNote{ID: id, Position: Point{X: 10, Y: 20}, Width: 200, Height: 120, Text: text, Color: "amber"}
}

func TestStickyNotesAreBoundedOnSaveAndImport(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	b := newFlow("Notes")
	b.node("start", TypeTriggerManual, nil)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	many := make([]StickyNote, 101)
	for i := range many {
		many[i] = stickyNote(fmt.Sprintf("s_%03d", i), "")
	}
	with := func(edit func(*StickyNote)) []StickyNote {
		note := stickyNote("s_edit", "x")
		edit(&note)
		return []StickyNote{note}
	}
	cases := map[string][]StickyNote{
		"2049 runes":     {stickyNote("s_long", strings.Repeat("ä", 2049))},
		"101st note":     many,
		"bidi override":  {stickyNote("s_bidi", noteBidiText)},
		"width 79":       with(func(n *StickyNote) { n.Width = 79 }),
		"NaN position":   with(func(n *StickyNote) { n.Position.X = math.NaN() }),
		"duplicate id":   {stickyNote("s_twin", "a"), stickyNote("s_twin", "b")},
		"unknown colour": with(func(n *StickyNote) { n.Color = "red" }),
	}
	for name, notes := range cases {
		t.Run(name, func(t *testing.T) {
			doc, err := rec.Draft.Clone()
			if err != nil {
				t.Fatal(err)
			}
			doc.Notes = notes
			issues := s.Validate(doc, ModeDraft)
			if !hasIssue(issues, IssueNoteInvalid) {
				t.Fatalf("validation did not refuse the notes: %+v", issues)
			}
			for _, issue := range issues {
				if issue.Code == IssueNoteInvalid && issue.Message != "sticky notes are invalid" {
					t.Fatalf("the issue must not echo note text: %q", issue.Message)
				}
			}
			if name == "NaN position" {
				return // JSON cannot carry NaN, so neither a save body nor an import can hold one.
			}
			var validation *ValidationError
			if _, _, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision); !errors.As(err, &validation) || !hasIssue(validation.Issues, IssueNoteInvalid) {
				t.Fatalf("save: %v", err)
			}
			wire, err := doc.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseFlowImport(wire); !errors.As(err, &validation) || !hasIssue(validation.Issues, IssueNoteInvalid) {
				t.Fatalf("import: %v", err)
			}
		})
	}
	doc, _ := rec.Draft.Clone()
	doc.Notes = []StickyNote{stickyNote("s_ok", "Familie "+noteFamilyEmoji+"\nnext line\twith tab")}
	if _, issues, err := s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision); err != nil {
		t.Fatalf("a note with a ZWJ emoji was refused: %v %+v", err, issues)
	}
}

func TestGroupTextRefusesFormatCharactersButKeepsJoiners(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	doc := newFlow("Groups").build()
	doc.Nodes = []Node{{ID: NewNodeID(), Key: "start", Type: TypeTriggerManual, TypeVersion: 1}}
	groups := []VisualGroup{{ID: "g_abcdefgh", Title: noteBidiText, Color: "blue", Members: []string{doc.Nodes[0].ID}}}
	doc.Groups = &groups
	if !hasIssue(s.Validate(doc, ModeDraft), IssueGroupInvalid) {
		t.Fatal("a group title with U+202E was accepted")
	}
	groups[0].Title = "Familie " + noteFamilyEmoji
	if hasIssue(s.Validate(doc, ModeDraft), IssueGroupInvalid) {
		t.Fatal("a group title with a ZWJ emoji was refused")
	}
}

// Load never validates, and annotations never mark a live flow. This test pins the skip
// list: it fails once noteChecks exists if NOTE_INVALID or GROUP_INVALID is missing from liveIssueSkipped.
func TestStoredBadNotesLoadAndDoNotFlagLiveFlows(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	b := newFlow("Stored notes")
	b.node("start", TypeTriggerManual, nil)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	if rec, _, err = s.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatal(err)
	}
	doc, _ := rec.Draft.Clone()
	long := strings.Repeat("n", 3000)
	doc.Notes = []StickyNote{stickyNote("s_long", long)}
	groups := []VisualGroup{{ID: "g_abcdefgh", Title: noteBidiText, Color: "blue", Members: []string{doc.Nodes[0].ID}}}
	doc.Groups = &groups
	wire, err := doc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.ExecContext(ctx, `UPDATE flows SET draft_json=?, live_json=? WHERE id=?`, string(wire), string(wire), rec.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetFlow(ctx, rec.ID)
	if err != nil || len(got.Draft.Notes) != 1 || got.Draft.Notes[0].Text != long {
		t.Fatalf("a stored draft with a long note must load as stored: %v", err)
	}
	reports, err := s.CheckLiveRevisions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, report := range reports {
		if report.FlowID == rec.ID {
			t.Fatalf("a live flow was flagged for its notes or groups: %+v", report.Issues)
		}
	}
}

// A registered secret in a sticky note or a step note is scrubbed in the export, the local
// pack and the run-view document (Run uses RedactCredentials, service_runs.go:356). The rest
// of the note stays.
func TestNotesAreScrubbedInExportPackAndRunView(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	const secret = "note-secret-fixture-5531"
	security.RegisterSensitive(secret)
	b := newFlow("Scrubbed notes")
	start := b.node("start", TypeTriggerManual, map[string]any{"data": map[string]any{"name": "sample"}})
	result := b.node("result", TypeSet, map[string]any{"keep_input": false, "fields": []any{
		map[string]any{"name": "greeting", "type": "text", "value": "Hello {{trigger.data.name}}"},
	}})
	b.edge(start, PortOut, result)
	doc := b.build()
	doc.Notes = []StickyNote{stickyNote("s_secret", "Sticky keep-this "+secret)}
	doc.Nodes[1].Settings.Notes = "Step keep-that " + secret
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: doc})
	if err != nil {
		t.Fatal(err)
	}
	check := func(where string, f *Flow) {
		t.Helper()
		wire, err := f.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		text := string(wire)
		if strings.Contains(text, secret) || !strings.Contains(text, "Sticky keep-this") || !strings.Contains(text, "Step keep-that") {
			t.Fatalf("%s: notes must stay with the secret scrubbed: %s", where, text)
		}
	}
	exported, err := ExportFlow(rec.Draft)
	if err != nil {
		t.Fatal(err)
	}
	check("export", exported)
	pack, err := s.ExportLocalPack(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	check("local pack", pack.Parent.Flow)
	runView, err := RedactCredentials(rec.Draft)
	if err != nil {
		t.Fatal(err)
	}
	check("run view", runView)
	if !strings.Contains(rec.Draft.Notes[0].Text, secret) {
		t.Fatal("redaction changed the stored draft")
	}
}
