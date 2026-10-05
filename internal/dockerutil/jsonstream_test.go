package dockerutil

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestDrainJSONMessages(t *testing.T) {
	cases := []struct {
		name        string
		stream      string
		wantMessage string // non-empty: expect *JSONMessageError with this message
		wantCut     bool   // expect a stream error wrapping io.ErrUnexpectedEOF
	}{
		{name: "clean pull", stream: `{"status":"Pulling from library/caddy"}` + "\r\n" + `{"status":"Status: Downloaded newer image for caddy:2"}` + "\r\n"},
		{name: "final message without newline", stream: `{"status":"Downloading"}` + "\n" + `{}`},
		{name: "blank lines", stream: "\n\r\n" + `{"status":"Extracting"}` + "\n\n"},
		{name: "non JSON line skipped", stream: "plain text from a proxy\n" + `{"status":"Pull complete"}` + "\n"},
		{name: "error detail before error", stream: `{"status":"Pulling fs layer"}` + "\n" + `{"errorDetail":{"message":"no matching manifest for linux/arm64"},"error":"short text"}` + "\n", wantMessage: "no matching manifest for linux/arm64"},
		{name: "error field only", stream: `{"error":"toomanyrequests: rate limit"}` + "\n", wantMessage: "toomanyrequests: rate limit"},
		{name: "error detail only", stream: `{"errorDetail":{"message":"registry denied"}}` + "\n", wantMessage: "registry denied"},
		{name: "first error wins", stream: `{"error":"first"}` + "\n" + `{"error":"second"}` + "\n", wantMessage: "first"},
		{name: "stream ends inside a message", stream: `{"status":"Downloading"}` + "\n" + `{"status":"Downlo`, wantCut: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := DrainJSONMessages(strings.NewReader(tc.stream))
			var event *JSONMessageError
			switch {
			case tc.wantMessage != "":
				if !errors.As(err, &event) || event.Message != tc.wantMessage {
					t.Fatalf("DrainJSONMessages() = %v, want event %q", err, tc.wantMessage)
				}
			case tc.wantCut:
				if err == nil || errors.As(err, &event) || !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("DrainJSONMessages() = %v, want a cut-stream error wrapping io.ErrUnexpectedEOF", err)
				}
			default:
				if err != nil {
					t.Fatalf("DrainJSONMessages() = %v, want nil", err)
				}
			}
		})
	}
}

func TestDrainJSONMessagesReturnsReadError(t *testing.T) {
	cut := errors.New("connection reset by peer")
	err := DrainJSONMessages(io.MultiReader(strings.NewReader(`{"status":"Downloading"}`+"\n"), iotest.ErrReader(cut)))
	if !errors.Is(err, cut) {
		t.Fatalf("DrainJSONMessages() = %v, want the read error", err)
	}
}

func TestDrainJSONMessagesBoundsLineLength(t *testing.T) {
	long := `{"stream":"` + strings.Repeat("x", 512<<10) + `"}` + "\n"
	if err := DrainJSONMessages(strings.NewReader(long)); err != nil {
		t.Fatalf("512 KiB line: %v, want nil (build output lines below 1 MiB stay accepted)", err)
	}
	tooLong := `{"stream":"` + strings.Repeat("x", MaxJSONMessageLine) + `"}` + "\n"
	if err := DrainJSONMessages(strings.NewReader(tooLong)); !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("over-long line: %v, want bufio.ErrTooLong", err)
	}
}

func TestDrainJSONMessagesRejectsMissingReader(t *testing.T) {
	if err := DrainJSONMessages(nil); err == nil {
		t.Fatal("DrainJSONMessages(nil) = nil, want an error")
	}
}

func TestJSONMessageErrorText(t *testing.T) {
	if got := (&JSONMessageError{Message: "manifest unknown"}).Error(); got != "manifest unknown" {
		t.Fatalf("Error() = %q, want the Engine message", got)
	}
}

func TestReadErrorBodyCapsAtMaxErrorBody(t *testing.T) {
	if got := len(ReadErrorBody(strings.NewReader(strings.Repeat("e", MaxErrorBody+4096)))); got != MaxErrorBody {
		t.Fatalf("ReadErrorBody() length = %d, want %d", got, MaxErrorBody)
	}
	const short = `{"message":"No such image: ghcr.io/example/app:latest"}`
	if got := string(ReadErrorBody(strings.NewReader(short))); got != short {
		t.Fatalf("ReadErrorBody() = %q, want the whole short body", got)
	}
	if ReadErrorBody(nil) != nil {
		t.Fatal("ReadErrorBody(nil) must return nil")
	}
}
