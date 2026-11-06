package storymaker

import (
	"slices"
	"testing"
)

func TestChangedScenesListsChangedUnchangedRemoved(t *testing.T) {
	before := &Story{Scenes: []Scene{{ID: "s1", Narration: "a"}, {ID: "s2", Narration: "b"}, {ID: "s3", Narration: "c"}}}
	after := &Story{Scenes: []Scene{{ID: "s2", Narration: "b"}, {ID: "s1", Narration: "a, changed"}, {ID: "s4", Narration: "d"}}}
	changed, unchanged, removed := ChangedScenes(before, after)
	if !slices.Equal(changed, []string{"s1", "s4"}) || !slices.Equal(unchanged, []string{"s2"}) || !slices.Equal(removed, []string{"s3"}) {
		t.Fatalf("changed %v, unchanged %v, removed %v", changed, unchanged, removed)
	}
}
