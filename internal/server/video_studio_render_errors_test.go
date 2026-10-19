package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

func TestVideoStudioRenderFailureCodes(t *testing.T) {
	ctx := context.Background()
	segment := &videostudio.SegmentError{Index: 2, Start: 20, End: 30, Err: errors.New("ffmpeg exploded at /secret/path")}
	status, code, message := videoStudioJobFailure("render", fmt.Errorf("render: %w", segment), ctx)
	if status != "failed" || code != "render_segment_failed" || message != "Rendering failed in video segment 3 (0:20–0:30)." {
		t.Fatalf("segment failure = %s %s %q", status, code, message)
	}
	if result := videoStudioFailureResult(segment); result["failed_from_seconds"] != 20.0 || result["failed_to_seconds"] != 30.0 {
		t.Fatalf("failure result = %v", result)
	}
	if videoStudioFailureResult(errors.New("plain")) != nil {
		t.Fatal("a plain error produced a failure result")
	}
	if _, code, _ := videoStudioJobFailure("render", fmt.Errorf("x: %w", videostudio.ErrInsufficientDiskSpace), ctx); code != "insufficient_disk_space" {
		t.Fatalf("disk space code = %s", code)
	}
	if _, code, _ := videoStudioJobFailure("render", fmt.Errorf("x: %w", videostudio.ErrTooManyInputs), ctx); code != "render_too_many_inputs" {
		t.Fatalf("too many inputs code = %s", code)
	}
	// F4 (plan 2f Task 6): the LUT cap has its own code and message. ErrTooManyLUTs wraps ErrTooManyInputs, so it is
	// checked first; a segment error around it, a range render and a still keep the code.
	lutCap := fmt.Errorf("%w: 256 lut effects between frames 0 and 30, at most 64", videostudio.ErrTooManyLUTs)
	for _, tt := range []struct {
		kind string
		err  error
	}{
		{"render", fmt.Errorf("render: %w", lutCap)},
		{"render", fmt.Errorf("render: %w", &videostudio.SegmentError{Index: 0, Start: 0, End: 10, Err: lutCap})},
		{"range_render", lutCap},
		{"still", lutCap},
	} {
		status, code, message := videoStudioJobFailure(tt.kind, tt.err, ctx)
		if status != "failed" || code != "render_too_many_luts" || message != "Too many LUT effects play at the same time in one part of the timeline." {
			t.Fatalf("%s with the LUT cap (%v) = %s %s %q", tt.kind, tt.err, status, code, message)
		}
	}
	// A full export names the limit that stopped it: the project quota when it was the smaller one.
	limitErr := fmt.Errorf("render: %w", videostudio.ErrOutputLimit)
	for _, tt := range []struct {
		exportLimit, remaining int64
		code                   string
	}{{100 << 20, 10 << 20, "project_size_limit"}, {100 << 20, 500 << 20, "export_size_limit"}, {100 << 20, 100 << 20, "export_size_limit"}} {
		if _, code, _ := videoStudioJobFailure("render", videoStudioRenderLimitError(limitErr, tt.exportLimit, tt.remaining), ctx); code != tt.code {
			t.Fatalf("output limit with export %d / remaining %d = %s, want %s", tt.exportLimit, tt.remaining, code, tt.code)
		}
	}
	if err := videoStudioRenderLimitError(segment, 1, 2); err != segment {
		t.Fatalf("a segment error was rewritten to %v", err)
	}
}

// A render that needs a filter or xfade transition the server's FFmpeg lacks fails with feature_unavailable and names
// the feature. The check runs before the kind switch, so the range_render kind (Task 16) maps the same way.
func TestVideoStudioFeatureUnavailableFailureCode(t *testing.T) {
	ctx := context.Background()
	missing := &videostudio.CapabilityError{Feature: "transition:black", Missing: "xfade:fadeblack"}
	for _, kind := range []string{"render", "range_render"} {
		status, code, message := videoStudioJobFailure(kind, fmt.Errorf("render: %w", missing), ctx)
		if status != "failed" || code != "feature_unavailable" || !strings.Contains(message, "transition:black") {
			t.Fatalf("%s with a missing FFmpeg feature = %s %s %q", kind, status, code, message)
		}
	}
	if _, code, message := videoStudioJobFailure("render", videostudio.ErrFeatureUnavailable, ctx); code != "feature_unavailable" || strings.Contains(message, "(") {
		t.Fatalf("bare ErrFeatureUnavailable = %s %q", code, message)
	}
}

// A video beyond the aspect rule (long side more than 16 times the short side) fails an import, a preview, an export or a
// range render with source_aspect_extreme. The check runs before the kind switch, like feature_unavailable.
func TestVideoStudioSourceAspectFailureCode(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"probe", "preview", "render", "range_render", "reverse", "still", "generate"} {
		status, code, message := videoStudioJobFailure(kind, fmt.Errorf("probe staged media: %w: 16384x16", videostudio.ErrSourceAspect), ctx)
		if status != "failed" || code != "source_aspect_extreme" || !strings.Contains(message, "16") {
			t.Fatalf("%s with an extreme aspect = %s %s %q", kind, status, code, message)
		}
	}
}
