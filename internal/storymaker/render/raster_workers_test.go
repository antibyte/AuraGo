package render

import (
	"aurago/internal/storymaker"
	"testing"
)

func TestBoundRasterWorkers(t *testing.T) {
	files := []storymaker.CatalogFile{{Type: "image/png", Bytes: 128 << 20}, {Type: "audio/wav", Bytes: 128 << 20}}
	for _, tc := range []struct {
		free int64
		want int
	}{{0, 1}, {256, 1}, {1024, 1}, {1792, 1}, {3584, 2}, {4096, 2}, {8192, 5}, {16384, 6}} {
		if got := BoundRasterWorkers(6, tc.free, files); got != tc.want {
			t.Errorf("free %d: got %d, want %d", tc.free, got, tc.want)
		}
	}
}
