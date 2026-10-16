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
