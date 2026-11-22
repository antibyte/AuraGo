package storymaker

import "testing"

// The 9:16 zone is the union of the platform overlays; every other aspect keeps the stage's 5 percent margin.
func TestSafeZone(t *testing.T) {
	if got, want := SafeZone("9:16"), (LayoutZone{Top: 0.115, Right: 0.130, Bottom: 0.219, Left: 0.056}); got != want {
		t.Errorf("9:16 zone = %+v, want %+v", got, want)
	}
	margin := LayoutZone{Top: 0.05, Right: 0.05, Bottom: 0.05, Left: 0.05}
	for _, aspect := range []string{"16:9", "1:1", "", "4:5"} {
		if got := SafeZone(aspect); got != margin {
			t.Errorf("%q zone = %+v, want %+v", aspect, got, margin)
		}
	}
}

// The check hands the story's aspect to the layout rules.
func TestStoryLayoutOptionsCarryTheAspect(t *testing.T) {
	var s Story
	s.Format.Aspect = "9:16"
	if got := StoryLayoutOptions(&s).Aspect; got != "9:16" {
		t.Errorf("Aspect = %q, want 9:16", got)
	}
}
