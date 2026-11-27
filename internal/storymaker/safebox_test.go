package storymaker

import "testing"

func TestSafeBoxForPinsTheThreeFormats(t *testing.T) {
	for _, tc := range []struct {
		aspect string
		want   SafeBox
	}{
		{"16:9", SafeBox{96, 54, 1728, 803}},
		{"9:16", SafeBox{691, 125, 493, 581}},
		{"1:1", SafeBox{474, 54, 972, 803}},
		{"", SafeBox{96, 54, 1728, 803}},
		{"4:3", SafeBox{96, 54, 1728, 803}},
	} {
		if got := SafeBoxFor(tc.aspect); got != tc.want {
			t.Errorf("SafeBoxFor(%q) = %+v, want %+v", tc.aspect, got, tc.want)
		}
	}
}
