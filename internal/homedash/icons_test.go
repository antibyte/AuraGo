package homedash_test

import (
	"testing"

	"aurago/internal/homedash"
)

func TestIconNamesAreValidUniqueAndComplete(t *testing.T) {
	names := homedash.IconNames()
	seen := map[string]bool{}
	for _, n := range names {
		if !homedash.ValidIcon(n) || seen[n] {
			t.Fatalf("bad or duplicate icon name %q", n)
		}
		seen[n] = true
	}
	for _, n := range []string{"cpu", "memory", "disk", "download", "upload", "timer", "bulb", "power", "outlet", "scene",
		"script", "button", "list", "number", "thermometer", "droplet", "gauge", "lock", "brightness", "color-temp", "color", "effect"} {
		if !homedash.KnownIcon(n) {
			t.Errorf("%q missing from the contract list", n)
		}
	}
	if homedash.KnownIcon("mdi:lightbulb") || homedash.KnownIcon("") {
		t.Fatal("foreign icon accepted")
	}
	names[0] = "changed"
	if homedash.IconNames()[0] == "changed" {
		t.Fatal("IconNames must return a copy")
	}
}
