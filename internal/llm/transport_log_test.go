package llm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type transportLogRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn transportLogRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestLoggingTransportExpectedDeadlineIsNotError(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	transport := newLoggingTransport(transportLogRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}), logger)
	ctx, cancel := context.WithTimeout(WithExpectedDeadline(context.Background()), 10*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "https://guardian.example/v1/chat/completions", nil).WithContext(ctx)

	_, err := transport.RoundTrip(req)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RoundTrip error = %v, want context deadline exceeded", err)
	}
	output := logs.String()
	if !strings.Contains(output, "roundtrip_done (expected deadline)") {
		t.Fatalf("expected degraded deadline log, got %s", output)
	}
	if strings.Contains(output, "level=ERROR") {
		t.Fatalf("expected deadline was logged as an error: %s", output)
	}
}

func TestLoggingTransportUnexpectedFailureRemainsError(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	transport := newLoggingTransport(transportLogRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("provider unavailable")
	}), logger)
	req := httptest.NewRequest(http.MethodPost, "https://provider.example/v1/chat/completions", nil)

	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Fatal("RoundTrip unexpectedly succeeded")
	}
	if output := logs.String(); !strings.Contains(output, "level=ERROR") || !strings.Contains(output, "provider unavailable") {
		t.Fatalf("unexpected failure lost error visibility: %s", output)
	}
}
