package kgreasoner

import "testing"

func TestInferSuggestsTransitiveRelations(t *testing.T) {
	got := Infer([]EdgeFact{
		{Source: "service-a", Relation: "depends_on", Target: "service-b"},
		{Source: "service-b", Relation: "depends_on", Target: "database"},
	}, DefaultRules(), 10)

	if !hasInference(got, "service-a", "depends_on", "database", "transitive_relation") {
		t.Fatalf("missing transitive inference: %+v", got)
	}
}

func TestInferSuggestsInverseRelations(t *testing.T) {
	got := Infer([]EdgeFact{
		{Source: "proxmox", Relation: "hosts", Target: "homeassistant"},
	}, DefaultRules(), 10)

	if !hasInference(got, "homeassistant", "hosted_on", "proxmox", "inverse_relation") {
		t.Fatalf("missing inverse inference: %+v", got)
	}
}

func TestInferSkipsExistingAndSelfLoopRelations(t *testing.T) {
	got := Infer([]EdgeFact{
		{Source: "a", Relation: "depends_on", Target: "b"},
		{Source: "b", Relation: "depends_on", Target: "a"},
		{Source: "b", Relation: "dependency_of", Target: "a"},
	}, DefaultRules(), 10)

	if hasInference(got, "b", "dependency_of", "a", "inverse_relation") {
		t.Fatalf("should not suggest existing inverse relation: %+v", got)
	}
	if hasInference(got, "a", "depends_on", "a", "transitive_relation") {
		t.Fatalf("should not suggest transitive self-loop: %+v", got)
	}
}

func TestInferHonorsLimitDeterministically(t *testing.T) {
	got := Infer([]EdgeFact{
		{Source: "a", Relation: "hosts", Target: "b"},
		{Source: "c", Relation: "hosts", Target: "d"},
	}, DefaultRules(), 1)

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1: %+v", len(got), got)
	}
	if got[0].Source != "b" || got[0].Relation != "hosted_on" || got[0].Target != "a" {
		t.Fatalf("unexpected deterministic first result: %+v", got[0])
	}
}

func hasInference(inferences []InferredFact, source, relation, target, reason string) bool {
	for _, inf := range inferences {
		if inf.Source == source && inf.Relation == relation && inf.Target == target && inf.Reason == reason {
			return true
		}
	}
	return false
}
