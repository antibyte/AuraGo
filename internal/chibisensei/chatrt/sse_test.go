package chatrt

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func collect(t *testing.T, input string) (string, streamResult, error) {
	t.Helper()
	var text strings.Builder
	res, err := parseStream(strings.NewReader(input), func(s string) { text.WriteString(s) })
	return text.String(), res, err
}

func TestParseStream(t *testing.T) {
	t.Run("basic stream of the real binary", func(t *testing.T) {
		text, res, err := collect(t, readTestdata(t, "stream_basic.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if text != "Hello there, how are you?" {
			t.Fatalf("text %q", text)
		}
		if res.FinishReason != "stop" || !res.Done || res.PromptTokens != 18 || res.CompletionTokens != 8 || res.PerSecond <= 0 {
			t.Fatalf("result %+v", res)
		}
	})

	t.Run("tolerant framing", func(t *testing.T) {
		input := strings.ReplaceAll(readTestdata(t, "stream_tolerant.txt"), "\n", "\r\n")
		text, res, err := collect(t, input)
		if err != nil {
			t.Fatal(err)
		}
		if text != "Hello there" {
			t.Fatalf("text %q (reasoning must not reach the callback)", text)
		}
		if res.FinishReason != "length" || res.PromptTokens != 12 || res.CompletionTokens != 3 || res.PerSecond != 4.5 || !res.Done {
			t.Fatalf("result %+v", res)
		}
	})

	t.Run("error chunk with a context message", func(t *testing.T) {
		_, _, err := collect(t, `data: {"error":{"code":400,"message":"prompt is too long for the context window","type":"invalid_request_error"}}`+"\n")
		if !errors.Is(err, ErrContext) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("error chunk of another kind", func(t *testing.T) {
		_, _, err := collect(t, `data: {"error":{"code":500,"message":"kaboom","type":"server_error"}}`+"\n")
		if err == nil || errors.Is(err, ErrContext) || !errors.Is(err, errUpstream) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("cut stream", func(t *testing.T) {
		_, _, err := collect(t, `data: {"choices":[{"delta":{"content":"Hi"}}]}`+"\n")
		if !errors.Is(err, errStreamCut) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("finish reason without DONE is fine", func(t *testing.T) {
		text, res, err := collect(t, `data: {"choices":[{"delta":{"content":"Hi"},"finish_reason":"stop"}]}`+"\n")
		if err != nil || text != "Hi" || res.FinishReason != "stop" || res.Done {
			t.Fatalf("%q %+v %v", text, res, err)
		}
	})

	t.Run("only DONE", func(t *testing.T) {
		_, res, err := collect(t, "data: [DONE]\n")
		if err != nil || !res.Done || res.FinishReason != "" {
			t.Fatalf("%+v %v", res, err)
		}
	})

	t.Run("overlong line", func(t *testing.T) {
		_, _, err := collect(t, "data: "+strings.Repeat("x", maxStreamLine+10)+"\n")
		if err == nil {
			t.Fatal("overlong line accepted")
		}
	})

	t.Run("too many bad lines", func(t *testing.T) {
		_, _, err := collect(t, strings.Repeat("data: {broken\n", maxBadLines+1))
		if err == nil || errors.Is(err, errStreamCut) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("stream above the byte cap", func(t *testing.T) {
		line := "data: " + strings.Repeat("x", 900<<10) + "\n"
		_, _, err := collect(t, strings.Repeat(": "+strings.Repeat("c", 900<<10)+"\n", 10)+line)
		if err == nil {
			t.Fatal("byte cap not enforced")
		}
	})
}

func TestClassifyUpstream(t *testing.T) {
	cases := []struct {
		name    string
		e       upstreamError
		context bool
	}{
		{"real answer of b9994", upstreamError{Code: 400, Type: "exceed_context_size_error",
			Message: "request (3012 tokens) exceeds the available context size (1024 tokens), try increasing it"}, true},
		{"message only", upstreamError{Message: "the prompt exceeds the context size"}, true},
		{"too long", upstreamError{Message: "input too long for context"}, true},
		{"unrelated", upstreamError{Code: 500, Type: "server_error", Message: "out of memory"}, false},
		{"empty", upstreamError{}, false},
	}
	for _, tc := range cases {
		err := classifyUpstream(400, tc.e)
		if errors.Is(err, ErrContext) != tc.context {
			t.Errorf("%s: got %v", tc.name, err)
		}
	}
}
