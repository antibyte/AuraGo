package server

import "strings"

// storymakerFFmpegVersion shortens the first line of "ffmpeg -version" to the version token:
// "ffmpeg version 8.1.1-full_build-www.gyan.dev Copyright (c) ..." becomes "8.1.1-full_build-www.gyan.dev".
// Any other line is kept trimmed and capped.
func storymakerFFmpegVersion(line string) string {
	line = strings.TrimSpace(line)
	if rest, ok := strings.CutPrefix(line, "ffmpeg version "); ok {
		if fields := strings.Fields(rest); len(fields) > 0 {
			return fields[0]
		}
	}
	if len(line) > 60 {
		line = line[:60]
	}
	return line
}
