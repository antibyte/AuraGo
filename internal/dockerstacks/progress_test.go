package dockerstacks

import (
	"strings"
	"testing"
)

func TestParseProgressLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		want Progress
		ok   bool
	}{
		{"compose item", `{"id":"Container web-1","status":"Working","text":"Starting"}`,
			Progress{ID: "Container web-1", Status: "Working", Text: "Starting"}, true},
		{"parent_id is the parent", `{"id":"layer1","parent_id":"Image redis","status":"Working","text":"Downloading","current":512,"total":2048,"percent":25}`,
			Progress{ID: "layer1", Parent: "Image redis", Status: "Working", Text: "Downloading", Current: 512, Total: 2048, Percent: 25}, true},
		{"parent also works", `{"id":"layer1","parent":"Image redis","status":"Done"}`,
			Progress{ID: "layer1", Parent: "Image redis", Status: "Done"}, true},
		{"numbers as strings and a fractional percent", `{"id":"x","current":"10","total":"40","percent":"25.5"}`,
			Progress{ID: "x", Current: 10, Total: 40, Percent: 25.5}, true},
		{"percent is derived from current and total", `{"id":"x","current":1,"total":3}`,
			Progress{ID: "x", Current: 1, Total: 3, Percent: 33.3}, true},
		{"unknown fields and wrong types are ignored", `{"id":"x","dry-run":true,"tail":false,"details":{"a":1},"status":7}`,
			Progress{ID: "x"}, true},
		{"leading blanks", `   {"id":"x","text":"t"}`, Progress{ID: "x", Text: "t"}, true},
		{"plain text", `Network web_default  Created`, Progress{}, false},
		{"broken json", `{"id":"x","status":`, Progress{}, false},
		{"json that is not an object", `["a"]`, Progress{}, false},
		{"an object without id, status and text", `{"dry-run":true}`, Progress{}, false},
		{"empty", ``, Progress{}, false},
	}
	for _, c := range cases {
		got, ok := parseProgressLine(c.line)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: parseProgressLine = (%+v, %v), want (%+v, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestProgressTextAndByteTicks(t *testing.T) {
	if got := progressText(Progress{ID: "Container web-1", Status: "Done", Text: "Started"}); got != "Container web-1 Done Started" {
		t.Fatalf("progressText = %q", got)
	}
	if got := progressText(Progress{ID: "x", Text: "Pulled"}); got != "x Pulled" {
		t.Fatalf("progressText without a status = %q", got)
	}
	if !isByteTick(Progress{Current: 1, Total: 10}) || !isByteTick(Progress{Current: 10, Total: 10}) || isByteTick(Progress{ID: "x", Status: "Done"}) {
		t.Fatal("isByteTick must be true exactly for items that carry a byte count")
	}
}

func TestLineWriterSplitsLinesAcrossWrites(t *testing.T) {
	var lines []string
	w := newLineWriter(func(l string) { lines = append(lines, l) })
	_, _ = w.Write([]byte("first\nsec"))
	_, _ = w.Write([]byte("ond\r\n\n   \nthird"))
	if strings.Join(lines, "|") != "first|second" {
		t.Fatalf("lines before Flush = %q, want the two complete ones (blank lines skipped)", lines)
	}
	w.Flush()
	if strings.Join(lines, "|") != "first|second|third" {
		t.Fatalf("lines after Flush = %q", lines)
	}
	w.Flush()
	if len(lines) != 3 {
		t.Fatal("a second Flush must add nothing")
	}

	long := strings.Repeat("x", maxLineBytes+10)
	lines = nil
	w = newLineWriter(func(l string) { lines = append(lines, l) })
	_, _ = w.Write([]byte(long))
	if len(lines) != 1 || len(lines[0]) != maxLineBytes+10 {
		t.Fatalf("an endless line must be handed on once it passes %d bytes, got %d lines", maxLineBytes, len(lines))
	}
}

func TestErrorScanPicksTheLineThatExplainsTheFailure(t *testing.T) {
	scan := func(lines ...string) string {
		var s errorScan
		for _, l := range lines {
			if p, ok := parseProgressLine(l); ok {
				s.observe(l, &p)
			} else {
				s.observe(l, nil)
			}
		}
		return s.message()
	}
	if got := scan(`{"id":"Container web-1","status":"Working","text":"Starting"}`, "yaml: line 12: did not find expected key"); got != "yaml: line 12: did not find expected key" {
		t.Fatalf("a plain error line = %q", got)
	}
	if got := scan("Network x Created", "Error response from daemon: Ports are not available", "exit status 1"); got != "Error response from daemon: Ports are not available" {
		t.Fatalf("the last plain line naming an error must win over a later bare line, got %q", got)
	}
	if got := scan("first error: a", "second error: b"); got != "second error: b" {
		t.Fatalf("the last error line wins, got %q", got)
	}
	if got := scan(`{"id":"Container db-1","status":"Error","text":"Error response from daemon: no such image"}`); got != "Container db-1: Error response from daemon: no such image" {
		t.Fatalf("an error item = %q", got)
	}
	if got := scan(`{"id":"Container db-1","status":"Error","text":"Error"}`); got != "Container db-1 Error Error" {
		t.Fatalf("an error item without details = %q", got)
	}
	if got := scan("\x1b[31mError\x1b[0m: bad thing\x1b[0m"); got != "Error: bad thing" {
		t.Fatalf("ANSI must be stripped, got %q", got)
	}
	if got := scan("something happened"); got != "something happened" {
		t.Fatalf("with no error word the last plain line is the message, got %q", got)
	}
	if got := scan(); got != "" {
		t.Fatalf("no output = %q", got)
	}
}

func TestErrorLineOf(t *testing.T) {
	cases := map[string]int{
		"yaml: line 12: did not find expected key":           12,
		"yaml: line 3: mapping values are not allowed":       3,
		"validating /opt/stacks/web/compose.yaml: Line 7: x": 7,
		"no line number here":                                0,
		"timeline 5 is not a line":                           0,
		"line 0: nothing":                                    0,
		"line 99999999999999999999: overflow":                0,
	}
	for msg, want := range cases {
		if got := errorLineOf(msg); got != want {
			t.Errorf("errorLineOf(%q) = %d, want %d", msg, got, want)
		}
	}
}
