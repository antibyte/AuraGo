package server

import (
	"context"
	"errors"
	"fmt"
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
