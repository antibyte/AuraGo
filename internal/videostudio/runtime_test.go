package videostudio

import "testing"

func TestHasRequiredRenderFiltersIncludesTPad(t *testing.T) {
	filters := " T.. xfade Cross fade input videos\n T.. amix Audio mixing\n T.. afade Fade in/out audio\n"
	if hasRequiredRenderFilters(filters) {
		t.Fatal("runtime accepted FFmpeg without the tpad filter")
	}
	filters += " T.. tpad Temporarily pad video frames\n"
	if hasRequiredRenderFilters(filters) {
		t.Fatal("runtime accepted FFmpeg without the premultiply filters")
	}
	filters += " TS premultiply PreMultiply first stream\n TS unpremultiply UnPreMultiply first stream\n"
	if !hasRequiredRenderFilters(filters) {
		t.Fatal("runtime rejected FFmpeg with all required render filters")
	}
}
