package llm

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/sashabaranov/go-openai"
)

type streamRouteSnapshot struct {
	generation     int
	onFallback     bool
	fallbackClient bool
	client         *openai.Client
	model          string
	valid          bool
}

type observedCompletionStream struct {
	stream          CompletionStream
	ctx             context.Context
	expectedChoices int
	finishedChoices map[int]struct{}
	once            sync.Once
	onSuccess       func()
	onError         func(error)
}

func (fm *FailoverManager) activeStreamRoute() streamRouteSnapshot {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if fm.isOnFallback && fm.fallback != nil {
		return streamRouteSnapshot{
			generation:     fm.generation,
			onFallback:     true,
			fallbackClient: true,
			client:         fm.fallback,
			model:          fm.fallbackModel,
			valid:          true,
		}
	}
	return streamRouteSnapshot{
		generation: fm.generation,
		onFallback: fm.isOnFallback,
		client:     fm.primary,
		model:      fm.primaryModel,
		valid:      fm.primary != nil,
	}
}

func (fm *FailoverManager) primaryStreamRoute(current streamRouteSnapshot) streamRouteSnapshot {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if !current.valid || fm.generation != current.generation || fm.isOnFallback != current.onFallback || fm.primary == nil {
		return streamRouteSnapshot{}
	}
	current.client = fm.primary
	current.model = fm.primaryModel
	current.fallbackClient = false
	return current
}

func (fm *FailoverManager) fallbackStreamRoute(generation int, client *openai.Client) streamRouteSnapshot {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if client == nil || fm.generation != generation || !fm.isOnFallback || fm.fallback != client {
		return streamRouteSnapshot{}
	}
	return streamRouteSnapshot{
		generation:     generation,
		onFallback:     true,
		fallbackClient: true,
		client:         client,
		model:          fm.fallbackModel,
		valid:          true,
	}
}

func (fm *FailoverManager) streamRouteCurrentLocked(route streamRouteSnapshot) bool {
	if !route.valid || fm.generation != route.generation || fm.isOnFallback != route.onFallback {
		return false
	}
	if route.fallbackClient {
		return fm.fallback == route.client
	}
	return fm.primary == route.client
}

func (fm *FailoverManager) observeStream(ctx context.Context, stream CompletionStream, request openai.ChatCompletionRequest, route streamRouteSnapshot) CompletionStream {
	return observeCompletionStream(ctx, stream, request,
		func() { fm.recordStreamSuccess(route) },
		func(err error) { fm.recordStreamError(route, err) },
	)
}

func (fm *FailoverManager) recordStreamSuccess(route streamRouteSnapshot) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if !fm.streamRouteCurrentLocked(route) {
		return
	}
	fm.errorCount = 0
	fm.fallbackErrorCount = 0
}

func (fm *FailoverManager) recordStreamError(route streamRouteSnapshot, err error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if !fm.streamRouteCurrentLocked(route) {
		return
	}
	fm.recordErrorLocked(err)
}

func observeCompletionStream(
	ctx context.Context,
	stream CompletionStream,
	request openai.ChatCompletionRequest,
	onSuccess func(),
	onError func(error),
) CompletionStream {
	if stream == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	expected := request.N
	if expected <= 0 {
		expected = 1
	}
	return &observedCompletionStream{
		stream:          stream,
		ctx:             ctx,
		expectedChoices: expected,
		finishedChoices: make(map[int]struct{}),
		onSuccess:       onSuccess,
		onError:         onError,
	}
}

func (s *observedCompletionStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	response, err := s.stream.Recv()
	if err != nil {
		if s.ctx.Err() != nil || IsContextError(err) {
			s.finishNeutral()
		} else if errors.Is(err, io.EOF) {
			s.finishError(io.ErrUnexpectedEOF)
		} else {
			s.finishError(err)
		}
		return response, err
	}

	for _, choice := range response.Choices {
		if choice.FinishReason != "" {
			s.finishedChoices[choice.Index] = struct{}{}
		}
	}
	if len(s.finishedChoices) >= s.expectedChoices {
		s.finishSuccess()
	}
	return response, nil
}

func (s *observedCompletionStream) Close() error {
	// Closing before every finish marker is observed is neutral. In particular,
	// callers are allowed to stop reading after their own cancellation or limit.
	s.finishNeutral()
	return s.stream.Close()
}

func (s *observedCompletionStream) finishSuccess() {
	s.once.Do(func() {
		if s.ctx.Err() == nil && s.onSuccess != nil {
			s.onSuccess()
		}
	})
}

func (s *observedCompletionStream) finishError(err error) {
	s.once.Do(func() {
		if s.ctx.Err() == nil && s.onError != nil {
			s.onError(err)
		}
	})
}

func (s *observedCompletionStream) finishNeutral() {
	s.once.Do(func() {})
}
