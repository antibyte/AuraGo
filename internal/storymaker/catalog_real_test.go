package storymaker

import (
	"os"
	"sync"
	"testing"
)

// The real starter catalog (ui/img/storymaker, produced by scripts/build-storymaker-catalog.mjs) and the fixture stories must
// keep agreeing: this guards the Go core against drifting away from the catalog (names, spots, files).

var (
	realOnce sync.Once
	realCat  *Catalog
	realErr  error
)

func loadReal(t testing.TB) *Catalog {
	t.Helper()
	realOnce.Do(func() {
		realCat, realErr = LoadCatalog(os.DirFS("../../ui"), "img/storymaker/catalog.json")
	})
	if realErr != nil {
		t.Fatalf("LoadCatalog(ui/img/storymaker): %v", realErr)
	}
	return realCat
}

func TestRealCatalogLoadsAndItsFilesExist(t *testing.T) {
	cat := loadReal(t)
	if err := cat.VerifyFiles(os.DirFS("../../ui"), "img/storymaker"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"char.office-woman-1", "char.office-man-1", "char.it-tech-1", "char.senior-man-1", "env.plain-stage", "env.office",
		"env.home-office", "env.abstract-tech", "prop.lock", "prop.shield", "pose.typing", "gesture.point", "expr.surprised", "cycle.walk", "text.caption", "trans.wipe"} {
		if _, ok := cat.Lookup(id); !ok {
			t.Errorf("the starter catalog has no %s", id)
		}
	}
	for _, kind := range []string{"pose", "gesture", "expression"} {
		if len(cat.Names(kind)) == 0 {
			t.Errorf("no %s names", kind)
		}
	}
}

func TestRealCatalogValidatesTheFixtureStories(t *testing.T) {
	cat := loadReal(t)
	for _, name := range []string{"stories/minimal", "stories/firewall", "player/castsheet"} {
		data, err := os.ReadFile("testdata/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		s := mustDecode(t, string(data))
		if issues := Validate(&s, cat, DefaultLimits()); len(issues) != 0 {
			t.Errorf("%s: %s", name, dump(issues))
		}
		ApplyDefaults(&s)
		if issues := Validate(&s, cat, DefaultLimits()); len(issues) != 0 {
			t.Errorf("%s after defaults: %s", name, dump(issues))
		}
	}
}
