package memory

import "testing"

func TestMemoryCorrectionDoesNotChangeAnotherDeviceWithSameOS(t *testing.T) {
	c := &MemoryCorrectionService{Memory: automaticTestStore(t), Graph: newTestKG(t), Access: CorrectionAccess{true, true, true}}
	for _, id := range []string{"server-a", "server-b"} {
		if err := c.Graph.AddNode(id, id, map[string]string{"type": "server", "os": "Windows Server 2025"}); err != nil {
			t.Fatal(err)
		}
	}
	p, err := c.Preview(t.Context(), "chat", "turn", "Observed new OS on server-a", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "kg_node", ID: "server-a", Field: "os"}, Operation: "replace", Value: "Ubuntu 24.04 LTS"}}, []CorrectionEvidence{{Reference: "tool:server-a-sysinfo", Kind: "tool", Quote: "Ubuntu 24.04 LTS", ContentHash: "runtime-test-hash"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 1 || len(p.Review) != 1 || p.Review[0].ID != "server-b" {
		t.Fatalf("equal property value must remain a review candidate: %+v", p)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal(err)
	}
	other, err := c.Graph.GetNode("server-b")
	if err != nil {
		t.Fatal(err)
	}
	if other.Properties["os"] != "Windows Server 2025" {
		t.Errorf("unrelated device was automatically corrected using evidence for another device: os=%q; targets=%d", other.Properties["os"], len(p.Items))
	}
}

func TestMemoryCorrectionProfileDeletionAllowsFreshCorrection(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(map[bool]string{false: "delete_entry", true: "reset_profile"}[reset], func(t *testing.T) {
			c, ref, oldID := correctionFixture(t)
			p, err := c.Preview(t.Context(), "chat", "turn", "User correction", correctionFixtureChanges(ref, oldID, "replace", correctionNew), correctionTestEvidence())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
				t.Fatal(err)
			}
			if reset {
				err = c.Memory.ResetUserProfile()
			} else {
				err = c.Memory.DeleteProfileEntry("prefs", "drink")
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = c.Memory.UpsertProfileEntry("prefs", "drink", "User prefers sparkling water", "analysis"); err != nil {
				t.Fatal(err)
			}
			entries, err := c.Memory.GetProfileEntries("prefs")
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatal("delayed analysis restored a corrected profile field")
			}
			next, err := c.Preview(t.Context(), "chat", "fresh-turn", "Fresh user evidence", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "profile", Category: "prefs", Key: "drink"}, Operation: "replace", Value: "User prefers sparkling water"}}, correctionTestEvidence())
			if err != nil {
				t.Fatal("fresh correction could not restore deleted field", err)
			}
			if _, err = c.Apply(t.Context(), next.ID, "chat"); err != nil {
				t.Fatal(err)
			}
			entries, err = c.Memory.GetProfileEntries("prefs")
			if err != nil || len(entries) != 1 || entries[0].Value != normalizeProfileValue("User prefers sparkling water") {
				t.Fatal("fresh correction did not become active", entries, err)
			}
		})
	}
}

func TestMemoryCorrectionGraphRejectsObsoletePreviewAfterRevert(t *testing.T) {
	c := &MemoryCorrectionService{Memory: automaticTestStore(t), Graph: newTestKG(t), Access: CorrectionAccess{true, true, true}}
	if err := c.Graph.AddNode("host", "host", map[string]string{"type": "server", "version": "1"}); err != nil {
		t.Fatal(err)
	}
	preview := func(run, value string) *MemoryCorrection {
		t.Helper()
		p, err := c.Preview(t.Context(), "chat", run, "Current evidence", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "kg_node", ID: "host", Field: "version"}, Operation: "replace", Value: value}}, correctionTestEvidence())
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	old := preview("old-turn", "9")
	for _, v := range []string{"2", "1"} {
		p := preview("new-turn-"+v, v)
		if _, err := c.Apply(t.Context(), p.ID, "chat"); err != nil {
			t.Fatal(err)
		}
	}
	if p, err := c.Apply(t.Context(), old.ID, "chat"); err == nil {
		t.Errorf("obsolete preview applied after two newer corrections: status=%s", p.Status)
	}
}

func TestMemoryCorrectionCurationCannotPublishPreparedReplacement(t *testing.T) {
	c, ref, oldID := correctionFixture(t)
	p, err := c.Preview(t.Context(), "chat", "turn", "User correction", correctionFixtureChanges(ref, oldID, "replace", correctionNew), correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Memory.db.Exec(`CREATE TRIGGER review_fail_core BEFORE UPDATE ON core_memory BEGIN SELECT RAISE(ABORT,'review fault'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err == nil {
		t.Fatal("fault injection failed")
	}
	var replacement string
	for _, item := range p.Items {
		if item.Reference.Store == "ltm" {
			replacement = item.ReplacementID
		}
	}
	if replacement == "" {
		t.Fatal("no staged vector")
	}
	if _, err = c.Vector.GetByIDFromCollection(replacement, "aurago_memories"); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"confirm", "unverify"} {
		if err = c.Memory.ApplyMemoryCurationAction(MemoryCurationAction{DocID: replacement, Action: action, Reason: "manual review"}, "admin", false); err == nil {
			t.Errorf("curation %s published an incomplete replacement", action)
		}
	}
	for _, statement := range []string{
		`UPDATE memory_meta SET verification_status='confirmed',archived_at=NULL WHERE doc_id=?`,
		`DELETE FROM memory_meta WHERE doc_id=?`,
		`UPDATE memory_meta SET doc_id='renamed' WHERE doc_id=?`,
		`INSERT OR REPLACE INTO memory_meta(doc_id,verification_status) VALUES(?,'confirmed')`,
	} {
		if _, err = c.Memory.db.Exec(statement, replacement); err == nil {
			t.Errorf("legacy writer bypassed publication guard: %s", statement)
		}
	}
	meta, err := c.Memory.GetMemoryMeta(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if !IsMemoryArchived(meta) {
		t.Error("uncommitted replacement was published by legacy curation while original remains active")
	}
	if _, err = c.Memory.db.Exec(`DROP TRIGGER review_fail_core`); err != nil {
		t.Fatal(err)
	}
	if p, err = c.Apply(t.Context(), p.ID, "chat"); err != nil || p.Status != "applied" {
		t.Fatal("publication guard prevented authorized recovery", p, err)
	}
}

func TestMemoryCorrectionPublicationRollsBackWithOriginalRetirement(t *testing.T) {
	c, _, oldID := correctionFixture(t)
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", []MemoryCorrectionChange{{Reference: MemoryReference{Store: "ltm", ID: oldID}, Operation: "replace", Value: correctionNew}}, correctionTestEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Memory.db.Exec(`CREATE TRIGGER fail_retirement BEFORE UPDATE ON memory_meta
		WHEN NEW.verification_status='archived' AND OLD.verification_status<>'archived'
		BEGIN SELECT RAISE(ABORT,'injected retirement failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err == nil {
		t.Fatal("retirement fault was not injected")
	}
	var published bool
	if err = c.Memory.db.QueryRow(`SELECT published FROM memory_correction_vectors WHERE doc_id=?`, p.Items[0].ReplacementID).Scan(&published); err != nil || published {
		t.Fatal("failed retirement left a publication receipt", published, err)
	}
	for id, archived := range map[string]bool{oldID: false, p.Items[0].ReplacementID: true} {
		meta, err := c.Memory.GetMemoryMeta(id)
		if err != nil || IsMemoryArchived(meta) != archived {
			t.Fatal("publication and retirement did not roll back together", meta, err)
		}
	}
	if _, err = c.Memory.db.Exec(`DROP TRIGGER fail_retirement`); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err != nil {
		t.Fatal("rolled-back publication could not resume", err)
	}
}
