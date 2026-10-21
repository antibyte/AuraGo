package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

type scriptedCompletionStream struct {
	responses []openai.ChatCompletionStreamResponse
	errors    []error
	closeErr  error
}

func TestObservedStreamAttemptDeadlineCountsButParentCancelIsNeutral(t *testing.T) {
	for _, parentCancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("parent_cancelled=%t", parentCancelled), func(t *testing.T) {
			parent, cancelParent := context.WithCancel(context.Background())
			defer cancelParent()
			attempt, cancel := context.WithDeadline(parent, time.Now().Add(-time.Second))
			defer cancel()
			if parentCancelled {
				cancelParent()
			}
			failures := 0
			stream := observeCompletionStream(withAttemptParent(attempt, parent), &scriptedCompletionStream{errors: []error{context.DeadlineExceeded, io.EOF}}, openai.ChatCompletionRequest{}, nil, func(err error) {
				failures++
				if ClassifyError(err) != ErrCategoryTemporaryTransport {
					t.Fatalf("attempt outcome category=%s", ClassifyError(err))
				}
			})
			_, _ = stream.Recv()
			_, _ = stream.Recv()
			_ = stream.Close()
			want := 1
			if parentCancelled {
				want = 0
			}
			if failures != want {
				t.Fatalf("failures=%d want %d", failures, want)
			}
		})
	}
}

func (s *scriptedCompletionStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	if len(s.responses) > 0 {
		response := s.responses[0]
		s.responses = s.responses[1:]
		return response, nil
	}
	if len(s.errors) > 0 {
		err := s.errors[0]
		s.errors = s.errors[1:]
		return openai.ChatCompletionStreamResponse{}, err
	}
	return openai.ChatCompletionStreamResponse{}, io.EOF
}

func (s *scriptedCompletionStream) Close() error { return s.closeErr }

func TestObservedStreamCountsInterruptedEOFOnlyOnce(t *testing.T) {
	var successes, failures int
	var observed error
	stream := observeCompletionStream(context.Background(), &scriptedCompletionStream{
		errors: []error{io.EOF, errors.New("later error")},
	}, openai.ChatCompletionRequest{}, func() { successes++ }, func(err error) {
		failures++
		observed = err
	})
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("Recv error = %v, want EOF", err)
	}
	_, _ = stream.Recv()
	_ = stream.Close()
	if successes != 0 || failures != 1 || !errors.Is(observed, io.ErrUnexpectedEOF) {
		t.Fatalf("outcomes success=%d failures=%d observed=%v", successes, failures, observed)
	}
}

func TestObservedStreamPrematureCloseAndCancellationAreNeutral(t *testing.T) {
	var successes, failures int
	raw := &scriptedCompletionStream{errors: []error{context.Canceled}}
	stream := observeCompletionStream(context.Background(), raw, openai.ChatCompletionRequest{},
		func() { successes++ }, func(error) { failures++ })
	_ = stream.Close()
	_, _ = stream.Recv()
	if successes != 0 || failures != 0 {
		t.Fatalf("premature close outcomes success=%d failures=%d", successes, failures)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream = observeCompletionStream(ctx, &scriptedCompletionStream{errors: []error{errors.New("connection reset")}}, openai.ChatCompletionRequest{},
		func() { successes++ }, func(error) { failures++ })
	_, _ = stream.Recv()
	if successes != 0 || failures != 0 {
		t.Fatalf("canceled stream outcomes success=%d failures=%d", successes, failures)
	}
}

func TestObservedStreamWaitsForAllRequestedFinishMarkers(t *testing.T) {
	var successes, failures int
	raw := &scriptedCompletionStream{responses: []openai.ChatCompletionStreamResponse{
		{Choices: []openai.ChatCompletionStreamChoice{{Index: 0, FinishReason: "stop"}}},
		{Choices: []openai.ChatCompletionStreamChoice{{Index: 1, FinishReason: "stop"}}},
	}}
	stream := observeCompletionStream(context.Background(), raw, openai.ChatCompletionRequest{N: 2},
		func() { successes++ }, func(error) { failures++ })
	_, _ = stream.Recv()
	if successes != 0 {
		t.Fatal("first finish marker completed a two-choice stream")
	}
	_, _ = stream.Recv()
	if successes != 1 || failures != 0 {
		t.Fatalf("outcomes success=%d failures=%d after all choices finished", successes, failures)
	}
}
