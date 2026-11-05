package flows

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestVisualGroupsExportImportVersionFence(t *testing.T) {
	doc, err := ParseFlow([]byte(groupsLegacyDocument))
	if err != nil {
		t.Fatal(err)
	}
	groups := []VisualGroup{{ID: "g_abcdefgh", Title: "Section", Color: "blue", Members: []string{doc.Nodes[0].ID}}}
	doc.Groups = &groups
	exported, err := ExportFlow(doc)
	if err != nil || exported.Schema != 2 || doc.Schema != SchemaVersion {
		t.Fatalf("group export mutated source or lacks marker: %+v %v", exported, err)
	}
	wire, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFlow(wire); !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("ordinary parser accepted export-only schema: %v", err)
	}
	imported, err := ParseFlowImport(wire)
	if err != nil || imported.Schema != SchemaVersion || imported.Groups == nil || (*imported.Groups)[0].Title != "Section" {
		t.Fatalf("group export roundtrip: %+v %v", imported, err)
	}
	groups[0].Members = []string{NewNodeID()}
	bad, _ := ExportFlow(doc)
	wire, _ = json.Marshal(bad)
	if _, err := ParseFlowImport(wire); err == nil {
		t.Fatal("invalid group import accepted")
	}
	for _, raw := range []string{`{"schema":2}`, `{"schema":3}`, `{"schema":2,"groups":[]}`} {
		if _, err := ParseFlowImport([]byte(raw)); err == nil {
			t.Fatalf("invalid import accepted: %s", raw)
		}
	}
	clear := []VisualGroup{}
	doc.Groups = &clear
	exported, err = ExportFlow(doc)
	if err != nil || exported.Schema != SchemaVersion {
		t.Fatalf("empty groups need no version marker: %+v %v", exported, err)
	}
	if _, err := ParseFlowImport([]byte(groupsLegacyDocument)); err != nil {
		t.Fatalf("legacy import failed: %v", err)
	}
}

func TestVisualGroupsExportScrubExpansionStaysImportable(t *testing.T) {
	doc, err := ParseFlow([]byte(groupsLegacyDocument))
	if err != nil {
		t.Fatal(err)
	}
	secret := "g8Secret"
	security.RegisterSensitive(secret)
	groups := []VisualGroup{{ID: "g_abcdefgh", Title: strings.Repeat("x", 72) + secret, Note: strings.Repeat("y", 2040) + secret, Color: "blue", Members: []string{doc.Nodes[0].ID}}}
	doc.Groups = &groups
	if len([]rune(security.ScrubSecrets(groups[0].Title))) <= 80 {
		t.Fatal("fixture did not expand when scrubbed")
	}
	exported, err := ExportFlow(doc)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := exported.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), secret) || !strings.Contains((*exported.Groups)[0].Title, "…") || !strings.Contains((*exported.Groups)[0].Note, "…") {
		t.Fatal("export failed to scrub and bound group text")
	}
	if _, err := ParseFlowImport(wire); err != nil {
		t.Fatalf("bounded scrubbed export cannot be imported: %v", err)
	}
	if !strings.Contains(groups[0].Title, secret) {
		t.Fatal("export mutated source")
	}
}

func TestVisualGroupsValidationMayReenterSaveWithoutDraftLock(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	b := newFlow("Reentrant validation")
	b.node("start", TypeTriggerManual, nil)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	var nestedErr error
	called := 0
	if err := s.reg.Register(&NodeDef{Type: "test.reentrant", Version: 1, Validate: func(_ *Node, _ ValidateContext) []Issue {
		called++
		nested, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		_, _, nestedErr = s.SaveDraft(nested, rec.ID, rec.Draft, rec.DraftRevision)
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	doc, err := rec.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Nodes = append(doc.Nodes, Node{ID: NewNodeID(), Key: "reentrant", Type: "test.reentrant", TypeVersion: 1, Params: map[string]any{}})
	_, _, err = s.SaveDraft(ctx, rec.ID, doc, rec.DraftRevision)
	if called != 1 || nestedErr != nil || !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("reentrant save: called=%d nested=%v outer=%v", called, nestedErr, err)
	}
}

func TestVisualGroupsDraftCompatibilityAndRestore(t *testing.T) {
	for _, kind := range []Kind{KindFlow, KindBlock} {
		t.Run(string(kind), func(t *testing.T) {
			ctx := context.Background()
			s, _ := newServiceFixture(t, nil)
			var rec *FlowRecord
			if kind == KindBlock {
				rec = publishedGreetingBlock(t, s)
			} else {
				b := newFlow("Legacy save")
				b.node("start", TypeTriggerManual, nil)
				var err error
				rec, err = s.CreateFlow(ctx, CreateRequest{Import: b.build()})
				if err != nil {
					t.Fatal(err)
				}
				rec, _, err = s.Publish(ctx, rec.ID, rec.DraftRevision)
				if err != nil {
					t.Fatal(err)
				}
			}
			save := func(doc *Flow, base int) (int, []Issue, error) {
				if kind == KindBlock {
					return s.SaveBlockDraft(ctx, rec.ID, doc, base)
				}
				return s.SaveDraft(ctx, rec.ID, doc, base)
			}
			get := func() *FlowRecord {
				var current *FlowRecord
				var err error
				if kind == KindBlock {
					current, err = s.GetBlock(ctx, rec.ID)
				} else {
					current, err = s.GetFlow(ctx, rec.ID)
				}
				if err != nil {
					t.Fatal(err)
				}
				return current
			}
			doc, _ := rec.Draft.Clone()
			groups := []VisualGroup{{ID: "g_abcdefgh", Title: "Preparation", Color: "blue", Members: []string{doc.Nodes[0].ID}}}
			doc.Groups = &groups
			rev, _, err := save(doc, rec.DraftRevision)
			if err != nil {
				t.Fatal(err)
			}
			legacy, _ := doc.Clone()
			legacy.Groups = nil
			rev, _, err = save(legacy, rev)
			if err != nil || get().Draft.Groups == nil {
				t.Fatalf("legacy save lost groups: %v", err)
			}
			if _, _, err := save(legacy, rev-1); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("stale save: %v", err)
			}
			dangling, _ := legacy.Clone()
			dangling.Nodes = dangling.Nodes[1:]
			_, issues, err := save(dangling, rev)
			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("dangling group accepted: %+v %v", issues, err)
			}
			cleared := []VisualGroup{}
			legacy.Groups = &cleared
			rev, _, err = save(legacy, rev)
			if err != nil || get().Draft.Groups == nil || len(*get().Draft.Groups) != 0 {
				t.Fatalf("explicit clear failed: %v", err)
			}
			legacy.Groups = &groups
			rev, _, err = save(legacy, rev)
			if err != nil {
				t.Fatal(err)
			}
			var restored *FlowRecord
			if kind == KindBlock {
				restored, _, err = s.RestoreBlockVersion(ctx, rec.ID, rec.LiveRevision, rev)
			} else {
				restored, _, err = s.RestoreVersion(ctx, rec.ID, rec.LiveRevision, rev)
			}
			if err != nil || restored.Draft.Groups != nil {
				t.Fatalf("restore did not replace groups: %v", err)
			}
		})
	}
}

func TestVisualGroupsValidateBoundariesAndExport(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	doc := newFlow("Groups").build()
	doc.Nodes = []Node{{ID: NewNodeID(), Key: "start", Type: TypeTriggerManual, TypeVersion: 1}}
	group := VisualGroup{ID: "g_abcdefgh", Title: "Section", Color: "blue", Members: []string{doc.Nodes[0].ID}}
	groups := []VisualGroup{group}
	doc.Groups = &groups
	assertGroupIssue := func(want bool) {
		t.Helper()
		found := false
		for _, issue := range s.Validate(doc, ModeDraft) {
			found = found || issue.Code == IssueGroupInvalid
		}
		if found != want {
			t.Fatalf("GROUP_INVALID=%v, want %v", found, want)
		}
	}
	assertGroupIssue(false)
	groups[0].Title = strings.Repeat("x", 81)
	assertGroupIssue(true)
	groups[0].Title = "bad\u0085title"
	assertGroupIssue(true)
	groups[0] = group
	groups[0].Note = "allowed\nline\twith\rwhitespace"
	assertGroupIssue(false)
	groups[0].Note = "bad\x00note"
	assertGroupIssue(true)
	groups[0] = group
	groups[0].Members = []string{doc.Nodes[0].ID, doc.Nodes[0].ID}
	assertGroupIssue(true)
	groups[0] = group
	groups = append(groups, group)
	assertGroupIssue(true)
	groups = groups[:1]
	groups[0] = group
	secret := "group-export-secret-8721"
	security.RegisterSensitive(secret)
	groups[0].Title = "Heading " + secret
	groups[0].Note = "Note " + secret
	copy, err := RedactCredentials(doc)
	if err != nil || strings.Contains((*copy.Groups)[0].Title, secret) || strings.Contains((*copy.Groups)[0].Note, secret) || !strings.Contains(groups[0].Title, secret) {
		t.Fatalf("group export redaction failed: %v", err)
	}
}

func TestVisualGroupsPreservePublishedBlockPins(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Pinned group parent")
	if _, issues, err := s.Publish(ctx, parent.ID, parent.DraftRevision); err != nil {
		t.Fatalf("publish parent: %v %+v", err, issues)
	}
	updated, err := block.Draft.Clone()
	if err != nil {
		t.Fatal(err)
	}
	groups := []VisualGroup{{ID: "g_abcdefgh", Title: "Greeting", Color: "green", Members: []string{updated.Nodes[0].ID}}}
	updated.Groups = &groups
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, updated, block.DraftRevision)
	if err != nil {
		t.Fatalf("save grouped block: %v %+v", err, issues)
	}
	if _, issues, err := s.PublishBlock(ctx, block.ID, rev); err != nil {
		t.Fatalf("publish grouped block: %v %+v", err, issues)
	}
	result, err := s.Simulate(ctx, parent.ID, SimulationRequest{BaseRevision: parent.DraftRevision})
	if err != nil || result.Status != RunSuccess || result.Steps[len(result.Steps)-1].Output["greeting"] != "Hello Ada" {
		t.Fatalf("old pinned version no longer executes: %+v %v", result, err)
	}
}
