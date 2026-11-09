package storymaker

import (
	"strings"
	"testing"
)

// The guide and the catalog must keep agreeing: every environment has a line, no line names a missing environment.
func TestEnvironmentGuideCoversTheCatalog(t *testing.T) {
	cat := loadReal(t)
	guide := EnvironmentGuide()
	if len(guide) == 0 {
		t.Fatal("the embedded guide did not decode")
	}
	for _, id := range cat.Names("environment") {
		g, ok := guide[id]
		if !ok || strings.TrimSpace(g.About) == "" {
			t.Errorf("the catalog environment %s has no guide line", id)
		}
		if id != plainStageID && len(g.Tags) == 0 {
			t.Errorf("%s has no tags", id)
		}
	}
	for id := range guide {
		if _, ok := cat.Lookup(id); !ok {
			t.Errorf("the guide names %s, which the catalog lacks", id)
		}
	}
}
