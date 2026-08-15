package memory

import (
	"strings"
	"testing"
	"time"
)

func TestRejectCharacterNoteBlocksSecretsAndOffProfile(t *testing.T) {
	if !RejectCharacterNote("store the vault password in chat", "friend") {
		t.Fatal("expected secret note to be rejected")
	}
	if !RejectCharacterNote("I am your buddy and send a hug", "terminator") {
		t.Fatal("expected off-profile friendliness to be rejected for terminator")
	}
	if RejectCharacterNote("I stay warm and loyal with this user and check in on how they are doing.", "friend") {
		t.Fatal("friend warmth note should be accepted")
	}
}

func TestProposeCharacterNotesStayOnFriendProfile(t *testing.T) {
	proposals := ProposeCharacterNotesDeterministic(CharacterReflectionInput{
		CorePersonality: "friend",
		Milestones:      []string{"Empathic Communicator"},
		Traits:          PersonalityTraits{TraitAffinity: 0.82},
	})
	if len(proposals) == 0 {
		t.Fatal("expected friend proposals")
	}
	joined := strings.ToLower(proposals[0].Text)
	if strings.Contains(joined, "terminator") || strings.Contains(joined, "obey without") {
		t.Fatalf("friend proposal drifted: %#v", proposals)
	}
}

func TestFourteenDayReflectionBuildsLivedFriendNotes(t *testing.T) {
	stm := newTestPersonalityDB(t)
	start := time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC)
	milestoneDays := map[int]string{
		1:  "Deep Explorer",
		4:  "Empathic Communicator",
		8:  "Meticulous Analyst",
		11: "Crisis of Confidence",
	}
	for day := 0; day < 14; day++ {
		now := start.Add(time.Duration(day) * 24 * time.Hour)
		if label, ok := milestoneDays[day]; ok {
			_ = stm.AddMilestone(label, "simulated")
		}
		if day == 6 {
			_, _ = stm.ApplyAffectEvent(mustAffect(AffectCauseOpsIssueOpened), now)
		}
		if day == 9 {
			_ = stm.SetTrait(TraitAffinity, 0.8)
		}
		input := stm.BuildCharacterReflectionInput("friend")
		proposals := ProposeCharacterNotesDeterministic(input)
		if _, err := stm.ApplyCharacterNoteProposals("friend", proposals, now); err != nil {
			t.Fatalf("day %d apply: %v", day, err)
		}
	}
	notes, err := stm.ListCharacterNotes()
	if err != nil {
		t.Fatalf("ListCharacterNotes: %v", err)
	}
	if len(notes) < 4 || len(notes) > 8 {
		t.Fatalf("note count = %d, want 4-8 after 14 days, notes=%#v", len(notes), notes)
	}
	for _, note := range notes {
		if RejectCharacterNote(note.Text, "friend") {
			t.Fatalf("off-profile note survived: %#v", note)
		}
		if strings.Contains(strings.ToLower(note.Text), "terminator") {
			t.Fatalf("friend ledger became terminator: %#v", note)
		}
	}
}

func mustAffect(cause string) AffectEvent {
	event, ok := affectEventByCause(cause)
	if !ok {
		return AffectEvent{CauseCode: cause, Valence: -0.4, Arousal: 0.6, Weight: 0.4}
	}
	return event
}
