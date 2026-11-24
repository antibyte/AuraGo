package homedash

import "testing"

func TestBuildAllowedSet(t *testing.T) {
	doc := validDoc()
	set := BuildAllowedSet(doc, Catalog())
	for _, c := range []Capability{CapToggle, CapTurnOn, CapTurnOff} {
		if !set.Allows("ha:switch.lamp", c) {
			t.Errorf("ha:switch.lamp must allow %s", c)
		}
	}
	if set.Allows("ha:switch.lamp", CapUnlock) {
		t.Error("a command outside the widget's catalog list was allowed")
	}
	if !set.Has("system:cpu") || set.Allows("system:cpu", CapToggle) || len(set.Commands["system:cpu"]) != 0 {
		t.Errorf("read-only binding: %+v", set.Commands["system:cpu"])
	}
	if set.Has("ha:switch.other") {
		t.Error("an unbound entity is in the set")
	}
	var nilSet *AllowedSet
	if nilSet.Has("ha:switch.lamp") || nilSet.Allows("ha:switch.lamp", CapToggle) {
		t.Error("a nil set allows something")
	}

	odd := validDoc()
	odd.Pages[0].Widgets[0].Type = "toaster" // unknown type contributes nothing
	odd.Pages[0].Widgets[1].Bindings = map[string]string{"nope": "ha:sensor.x", "primary": "nocolon"}
	got := BuildAllowedSet(odd, Catalog())
	if len(got.Commands) != 0 {
		t.Errorf("unknown type/binding or bad id contributed: %+v", got.Commands)
	}
	badVariant := validDoc()
	badVariant.Pages[0].Widgets[0].Variant = "flip"
	if got := BuildAllowedSet(badVariant, Catalog()); got.Has("ha:switch.lamp") {
		t.Error("unknown variant contributed")
	}
}
