package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

func testThroughputPolicy() ThroughputPolicy {
	return ThroughputPolicy{Base: 200 * time.Millisecond, ReserveTokens: 1000, MinTokensPerSecond: 50, MaxFactor: 4, MinSample: 20 * time.Millisecond}
}

// streamSteadily reports output at a fixed rate until the context ends.
func streamSteadily(ctx context.Context, charsPerTick int, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ObserveStreamOutput(ctx, charsPerTick)
		}
	}
}

func TestThroughputDeadlineKeepsBaseWithoutOutput(t *testing.T) {
	ctx, cancel := NewThroughputDeadline(context.Background(), testThroughputPolicy())
	defer cancel()
	start := time.Now()
	<-ctx.Done()
	if elapsed := time.Since(start); elapsed < 180*time.Millisecond || elapsed > 400*time.Millisecond {
		t.Fatalf("expired after %v, want the 200ms base", elapsed)
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", ctx.Err())
	}
}

// A model too slow to finish its reserve within the base keeps streaming
// until it could, bounded by MaxFactor; children observe a real deadline.
func TestThroughputDeadlineExtendsForSlowSteadyOutput(t *testing.T) {
	ctx, cancel := NewThroughputDeadline(context.Background(), testThroughputPolicy())
	defer cancel()
	child, stopChild := context.WithCancel(ctx)
	defer stopChild()
	go streamSteadily(ctx, 40, 10*time.Millisecond) // ~1000 tokens/s, needs ~1.1s for 1000 tokens
	select {
	case <-ctx.Done():
		t.Fatalf("slow but steady output expired at the base: %v", ctx.Err())
	case <-time.After(450 * time.Millisecond):
	}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) <= 0 {
		t.Fatalf("extended deadline not reported: %v %v", deadline, ok)
	}
	select {
	case <-child.Done():
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("extension ignored the MaxFactor cap")
	}
	if !errors.Is(child.Err(), context.DeadlineExceeded) || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("errors = %v / %v, want deadline exceeded", ctx.Err(), child.Err())
	}
}

func TestThroughputDeadlineIgnoresOutputBelowMinimumRate(t *testing.T) {
	policy := testThroughputPolicy()
	policy.MinTokensPerSecond = 1e7
	ctx, cancel := NewThroughputDeadline(context.Background(), policy)
	defer cancel()
	go streamSteadily(ctx, 40, 10*time.Millisecond)
	start := time.Now()
	<-ctx.Done()
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Fatalf("output below the minimum rate extended the deadline to %v", elapsed)
	}
}

func TestThroughputDeadlineFollowsParentAndCancel(t *testing.T) {
	parent, stopParent := context.WithCancel(context.Background())
	ctx, cancel := NewThroughputDeadline(parent, testThroughputPolicy())
	defer cancel()
	stopParent()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("parent cancellation did not propagate")
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("err = %v, want canceled", ctx.Err())
	}
	own, stop := NewThroughputDeadline(context.Background(), testThroughputPolicy())
	stop()
	if !errors.Is(own.Err(), context.Canceled) {
		t.Fatalf("cancel func err = %v", own.Err())
	}
	// Reporting on a plain context is a no-op.
	ObserveStreamOutput(context.Background(), 100)
}

// The per-attempt stream timeout must not cut a call whose duration the
// throughput deadline governs.
func TestStreamAttemptDefersToThroughputDeadline(t *testing.T) {
	policy := testThroughputPolicy()
	policy.Base = 10 * time.Minute
	ctx, cancel := NewThroughputDeadline(context.Background(), policy)
	defer cancel()
	client := &capturingStreamContextClient{}
	_, stop, err := ExecuteStreamWithRetry(WithStreamAttemptTimeout(ctx, time.Second), client, openai.ChatCompletionRequest{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	deadline, ok := client.captured.Deadline()
	if !ok || time.Until(deadline) < 9*time.Minute {
		t.Fatalf("stream attempt deadline %v, want the throughput deadline", time.Until(deadline))
	}
}
