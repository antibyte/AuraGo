package server

import (
	"strings"
	"testing"
)

func TestStorymakerFFmpegVersionToken(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 100)
	for _, tc := range []struct{ name, line, want string }{
		{"windows build", "ffmpeg version 8.1.1-full_build-www.gyan.dev Copyright (c) 2000-2026 the FFmpeg developers", "8.1.1-full_build-www.gyan.dev"},
		{"plain release", "ffmpeg version 6.1.1 Copyright (c) 2000-2023 the FFmpeg developers", "6.1.1"},
		{"git build", "ffmpeg version N-117000-gabcdef123 Copyright (c) 2000-2024", "N-117000-gabcdef123"},
		{"token only", "ffmpeg version 7.0", "7.0"},
		{"other banner", "  custom encoder 1.2  ", "custom encoder 1.2"},
		{"other banner is capped", long, long[:60]},
		{"version without token", "ffmpeg version", "ffmpeg version"},
		{"empty", "", ""},
	} {
		if got := storymakerFFmpegVersion(tc.line); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
