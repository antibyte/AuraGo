package llm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

type usageErrorTransportFunc func(*http.Request) (*http.Response, error)

func (f usageErrorTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUsageOnBillableErrorPreservesProviderResponse(t *testing.T) {
	original := []byte("{\n  \"error\":{\"message\":\"fixture failure\"},\"usage\":{\"prompt_tokens\":23,\"completion_tokens\":8}}")
	ctx, capture := CaptureUsage(context.Background())
	request, err := http.NewRequestWithContext(ctx, "POST", "https://fixture.invalid/chat/completions", bytes.NewBufferString(`{"model":"fixture-model"}`))
	if err != nil {
		t.Fatal(err)
	}
	observer := usageObservationTransport{provider: "openai", base: usageErrorTransportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 400, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(original))}, nil
	})}
	response, err := observer.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	got, err := io.ReadAll(response.Body)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("provider error body was changed")
	}
	usage := capture.Snapshot()
	if len(usage) != 1 || usage[0].InputTokens == nil || *usage[0].InputTokens != 23 || usage[0].OutputTokens == nil || *usage[0].OutputTokens != 8 || usage[0].Status != 400 {
		t.Fatalf("billable error usage was not preserved: %d records", len(usage))
	}
}
