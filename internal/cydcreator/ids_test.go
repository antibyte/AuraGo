package cydcreator

import (
	"regexp"
	"strings"
	"testing"
)

func TestRandomIDsShape(t *testing.T) {
	re := regexp.MustCompile(`^ccp_[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := RandomIDs{}.NewID("ccp")
		if !re.MatchString(id) {
			t.Fatalf("id %q does not match %s", id, re)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q after %d draws", id, i)
		}
		seen[id] = true
		if !validElementID(id) {
			t.Fatalf("random id %q is not a valid element id", id)
		}
	}
}

func TestSeqIDs(t *testing.T) {
	var ids SeqIDs // zero value must work
	got := []string{ids.NewID("w"), ids.NewID("w"), ids.NewID("s"), ids.NewID("w")}
	want := []string{"w_0001", "w_0002", "s_0001", "w_0003"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NewID #%d = %q, want %q", i, got[i], want[i])
		}
	}
	var src IDSource = &ids
	if id := src.NewID("a"); id != "a_0001" {
		t.Fatalf("via interface = %q", id)
	}
}

func TestValidElementID(t *testing.T) {
	cases := map[string]bool{
		"w_0001": true, "s_1": true, "a_sun": true, "W-7f3a": true,
		"": false, "_w": false, "w 1": false, "w/1": false, "w.1": false,
		strings.Repeat("x", 64): true, strings.Repeat("x", 65): false,
	}
	for id, want := range cases {
		if got := validElementID(id); got != want {
			t.Errorf("validElementID(%q) = %v, want %v", id, got, want)
		}
	}
}
