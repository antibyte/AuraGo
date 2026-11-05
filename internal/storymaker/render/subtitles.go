package render

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"aurago/internal/storymaker"
)

// SubRip defines no entities, so many players would show "&amp;" as written, and they do read tags. So & stays as it is,
// and < and > become the close look-alikes U+2039 and U+203A, which also breaks -->.
var srtEscape = strings.NewReplacer("<", "\xe2\x80\xb9", ">", "\xe2\x80\xba")

// WebVTT defines entities, and its cue text needs them for & < >. That also breaks -->.
var vttEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// WriteSRT writes the report's captions as SubRip. A caption with nothing left after the cleaning is dropped.
func WriteSRT(caps []reportCaption) []byte {
	var b bytes.Buffer
	n := 0
	for _, c := range caps {
		lines := subtitleLines(c.Lines, srtEscape)
		if len(lines) == 0 {
			continue
		}
		n++
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n, subtitleTime(c.StartMS, ','), subtitleTime(c.EndMS, ','), strings.Join(lines, "\n"))
	}
	return b.Bytes()
}

// WriteVTT writes the report's captions as WebVTT.
func WriteVTT(caps []reportCaption) []byte {
	var b bytes.Buffer
	b.WriteString("WEBVTT\n\n")
	for _, c := range caps {
		lines := subtitleLines(c.Lines, vttEscape)
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n", subtitleTime(c.StartMS, '.'), subtitleTime(c.EndMS, '.'), strings.Join(lines, "\n"))
	}
	return b.Bytes()
}

func subtitleLines(raw []string, escape *strings.Replacer) []string {
	var out []string
	for _, line := range raw {
		if clean := subtitleLine(line, escape); clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

// subtitleLine strips controls, collapses runs of JS whitespace to one space and applies the format's escape.
func subtitleLine(s string, escape *strings.Replacer) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case storymaker.IsJSSpace(r):
			space = b.Len() > 0
			continue
		case unicode.IsControl(r):
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return escape.Replace(b.String())
}

func subtitleTime(ms int64, sep byte) string {
	if ms < 0 {
		ms = 0
	}
	return fmt.Sprintf("%02d:%02d:%02d%c%03d", ms/3600000, ms/60000%60, ms/1000%60, sep, ms%1000)
}
