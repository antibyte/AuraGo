package memory

import "testing"

func TestTopicTermsFiltersGenericWordsAndNormalizesUnicode(t *testing.T) {
	if got := TopicTerms("versuche das Problem erneut"); len(got) != 0 {
		t.Fatalf("generic retry terms = %#v, want none", got)
	}
	got := TopicTerms("Ｔａｉｌｓｃａｌｅ Status und API SSL GPU NAS PKW")
	want := []string{"tailscale", "api", "ssl", "gpu", "nas", "pkw"}
	if len(got) != len(want) {
		t.Fatalf("terms = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("terms = %#v, want %#v", got, want)
		}
	}
}

func TestTopicTermsReturnsAtMostEightUniqueTerms(t *testing.T) {
	got := TopicTerms("alpha beta gamma delta epsilon zeta theta kappa lambda alpha")
	if len(got) != 8 {
		t.Fatalf("len(terms) = %d, want 8: %#v", len(got), got)
	}
}
