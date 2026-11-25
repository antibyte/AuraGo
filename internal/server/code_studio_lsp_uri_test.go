package server

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCodeStudioLSPURIRewriterMapsBothWays(t *testing.T) {
	rw := newCodeStudioLSPURIRewriter("/home/dev/my proj", "/srv/dev/my proj/")

	toServer := []struct{ in, want string }{
		{ // initialize: rootUri, rootPath and workspaceFolders; key order and numbers stay as sent
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":"file:///workspace","rootPath":"/workspace","workspaceFolders":[{"uri":"file:///workspace","name":"workspace"}]}}`,
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":"file:///home/dev/my%20proj","rootPath":"/home/dev/my proj","workspaceFolders":[{"uri":"file:///home/dev/my%20proj","name":"workspace"}]}}`,
		},
		{ // percent-encoded segments are decoded and encoded again for the remote root
			`{"params":{"textDocument":{"uri":"file:///workspace/src/it%27s%20%231.go","text":"package main // file:///workspace/x"}}}`,
			`{"params":{"textDocument":{"uri":"file:///home/dev/my%20proj/src/it%27s%20%231.go","text":"package main // file:///workspace/x"}}}`,
		},
		{`{"uri":"file:///workspacex/a.go"}`, `{"uri":"file:///workspacex/a.go"}`}, // not a path boundary
	}
	for _, tc := range toServer {
		if got := string(rw.toServer([]byte(tc.in))); got != tc.want {
			t.Errorf("toServer(%s)\n got %s\nwant %s", tc.in, got, tc.want)
		}
	}

	toPage := []struct{ in, want string }{
		{ // WorkspaceEdit: changes is keyed by URI, documentChanges carries URIs as values; the pwd -P root maps too
			`{"jsonrpc":"2.0","id":7,"result":{"changes":{"file:///home/dev/my%20proj/a.go":[{"newText":"x"}]},"documentChanges":[{"textDocument":{"uri":"file:///srv/dev/my%20proj/b.go","version":3},"edits":[]}]}}`,
			`{"jsonrpc":"2.0","id":7,"result":{"changes":{"file:///workspace/a.go":[{"newText":"x"}]},"documentChanges":[{"textDocument":{"uri":"file:///workspace/b.go","version":3},"edits":[]}]}}`,
		},
		{`{"uri":"file:///usr/local/go/src/fmt/print.go"}`, `{"uri":"file:///usr/local/go/src/fmt/print.go"}`}, // outside the folder
		{`{"uri":"file:///home/dev/my%20projX/a.go"}`, `{"uri":"file:///home/dev/my%20projX/a.go"}`},
		{`{"uri":"file:\/\/\/home\/dev\/my%20proj\/c.go"}`, `{"uri":"file:///workspace/c.go"}`}, // JSON-escaped literal
		{`{"uri":"FILE://localhost/home/dev/my%20proj/%C3%A4.go#L3"}`, `{"uri":"file:///workspace/%C3%A4.go#L3"}`},
		{`{"uri":"file:///home/dev/my%20proj/a.go`, `{"uri":"file:///home/dev/my%20proj/a.go`}, // malformed: unchanged
	}
	for _, tc := range toPage {
		if got := string(rw.toPage([]byte(tc.in))); got != tc.want {
			t.Errorf("toPage(%s)\n got %s\nwant %s", tc.in, got, tc.want)
		}
	}

	var none *codeStudioLSPURIRewriter // the container session has no rewriter
	if got := string(none.toServer([]byte(`{"a":"file:///workspace"}`))); got != `{"a":"file:///workspace"}` {
		t.Fatalf("nil rewriter changed a frame: %s", got)
	}
}

func TestCodeStudioLSPCloseReasonFitsControlFrame(t *testing.T) {
	if got := codeStudioLSPCloseReason("gopls is not installed"); got != "gopls is not installed" {
		t.Fatalf("short reason changed: %q", got)
	}
	long := strings.Repeat("ä", 100) // 200 bytes
	got := codeStudioLSPCloseReason(long)
	if len(got) > 123 || !utf8.ValidString(got) {
		t.Fatalf("reason = %d bytes, valid %v", len(got), utf8.ValidString(got))
	}
}
