package dockerutil

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// probeDeadlineTransport answers the version probe and reports how much time
// the probe's context had left; every other request answers 204.
type probeDeadlineTransport struct {
	remaining chan time.Duration
}

func (p probeDeadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/version" {
		left := time.Duration(-1)
		if deadline, ok := req.Context().Deadline(); ok {
			left = time.Until(deadline)
		}
		p.remaining <- left
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: req,
			Body: io.NopCloser(strings.NewReader(`{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`))}, nil
	}
	return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Request: req, Body: http.NoBody}, nil
}

func probeBudget(t *testing.T, ctx context.Context, transport *VersionTransport, base probeDeadlineTransport) time.Duration {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/"+APIVersion+"/_ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	select {
	case left := <-base.remaining:
		return left
	default:
		t.Fatal("no /version probe was sent")
		return 0
	}
}

func TestNewVersionTransportKeepsFiveSecondProbeBudget(t *testing.T) {
	base := probeDeadlineTransport{remaining: make(chan time.Duration, 1)}
	if left := probeBudget(t, context.Background(), NewVersionTransport(base), base); left <= 4*time.Second || left > 5*time.Second {
		t.Fatalf("probe budget = %s, want 5s", left)
	}
}

func TestNewVersionTransportWithProbeTimeoutExtendsTheBudget(t *testing.T) {
	base := probeDeadlineTransport{remaining: make(chan time.Duration, 1)}
	if left := probeBudget(t, context.Background(), NewVersionTransportWithProbeTimeout(base, 30*time.Second), base); left <= 29*time.Second || left > 30*time.Second {
		t.Fatalf("probe budget = %s, want 30s", left)
	}
}

func TestNewVersionTransportWithProbeTimeoutKeepsDefaultForZero(t *testing.T) {
	base := probeDeadlineTransport{remaining: make(chan time.Duration, 1)}
	if left := probeBudget(t, context.Background(), NewVersionTransportWithProbeTimeout(base, 0), base); left <= 4*time.Second || left > 5*time.Second {
		t.Fatalf("probe budget = %s, want the 5s default", left)
	}
}

func TestNewVersionTransportWithProbeTimeoutKeepsShorterCallerDeadline(t *testing.T) {
	base := probeDeadlineTransport{remaining: make(chan time.Duration, 1)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if left := probeBudget(t, ctx, NewVersionTransportWithProbeTimeout(base, 30*time.Second), base); left > 2*time.Second {
		t.Fatalf("probe budget = %s, want the caller's 2s deadline", left)
	}
}
