package llm

import (
	"context"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func TestStreamAttemptTimeoutIsRequestLocalAndParentBounded(t *testing.T) {
	parent, stop := context.WithTimeout(context.Background(), 10*time.Minute)
	defer stop()
	for _, explicit := range []time.Duration{0, 5 * time.Minute, 20 * time.Minute} {
		client := &capturingStreamContextClient{}
		_, cancel, err := ExecuteStreamWithRetry(WithStreamAttemptTimeout(parent, explicit), client, openai.ChatCompletionRequest{}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		deadline, ok := client.captured.Deadline()
		expected := explicit
		if explicit == 0 {
			expected = perAttemptTimeout()
		}
		expected = min(expected, 10*time.Minute)
		if !ok || time.Until(deadline) > expected || time.Until(deadline) < expected-time.Second {
			t.Fatalf("unexpected stream deadline %v for %v", time.Until(deadline), explicit)
		}
		cancel()
		if client.captured.Err() == nil {
			t.Fatal("stream cleanup did not cancel request")
		}
	}
	if streamAttemptTimeout(context.Background()) != perAttemptTimeout() {
		t.Fatal("workflow changed concurrent chat timeout")
	}
}
