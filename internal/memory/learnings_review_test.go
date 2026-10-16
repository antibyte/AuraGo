package memory

import "testing"

func TestLearningEvidenceCannotPromoteDifferentDraft(t *testing.T) {
	for _, change := range []string{"statement", "rationale", "kind", "alternatives", "fingerprint"} {
		t.Run(change, func(t *testing.T) {
			s := newTestSTM(t)
			d := LearningDraft{Statement: "Use Go", Rationale: "Portable", Kind: "preference", Confidence: .5}
			original, err := s.ObserveLearning(t.Context(), d, LearningScope{}, learningSource("first", "Use Go Portable"), "old")
			if err != nil {
				t.Fatal(err)
			}
			d.Confidence = .99
			fingerprint := "old"
			switch change {
			case "statement":
				d.Statement = "use go"
			case "rationale":
				d.Rationale = "New reason"
			case "kind":
				d.Kind = "decision"
			case "alternatives":
				d.Alternatives = []string{"Rust"}
			case "fingerprint":
				fingerprint = "new"
			}
			for _, session := range []string{"second", "third"} {
				_, _ = s.ObserveLearning(t.Context(), d, LearningScope{}, learningSource(session, "Use Go Portable New reason Rust"), fingerprint)
			}
			got, err := s.Learning(t.Context(), original.ID, original.Scope)
			if err != nil || got.Status != "proposed" || got.Revision != original.Revision || len(got.Sources) != 1 {
				t.Fatalf("different evidence changed stored draft: %+v, %v", got, err)
			}
		})
	}
}

func TestLearningReplacementDoesNotRetainRejectedAlternative(t *testing.T) {
	s := newTestSTM(t)
	l, err := s.ObserveLearning(t.Context(), LearningDraft{Statement: "Use Go", Kind: "decision", Confidence: .99, Alternatives: []string{"Rust"}}, LearningScope{}, learningSource("first", "Use Go instead of Rust"), "")
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.ReviewLearning(t.Context(), l.ID, l.Revision, l.Scope, "replace", "Use Rust", "Requirements changed")
	if err != nil || len(next.Alternatives) != 0 {
		t.Fatalf("replacement carries superseded alternatives: %+v, %v", next, err)
	}
	history, err := s.LearningHistory(t.Context(), l.ID, l.Scope)
	if err != nil || len(history) != 2 {
		t.Fatal(history, err)
	}
	old, err := s.Learning(t.Context(), l.ID, l.Scope)
	if err != nil || len(old.Alternatives) != 1 || old.Status != "superseded" {
		t.Fatal(old, err)
	}
}
