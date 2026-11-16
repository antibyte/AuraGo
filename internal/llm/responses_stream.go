package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
)

type responseToolState struct {
	id, name  string
	arguments strings.Builder
}

type responseCompletionStream struct {
	ctx              context.Context
	source           *openai.ResponseStream
	scope, id, model string
	created          int64
	tools            []*responseToolState
	indices          map[int]int
	items            map[int]json.RawMessage
	text, refusal    strings.Builder
	pending          []openai.ChatCompletionStreamResponse
	done             bool
	failure          error
	closeOnce        sync.Once
	closeError       error
	serviceTier      string
}

func (c openAIClientAdapter) createResponseCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (*responseCompletionStream, error) {
	scope := responsesScope(c.route, request.Model)
	native, err := responsesRequest(request, scope)
	if err != nil {
		return nil, err
	}
	stream, err := c.client.CreateResponseStream(ctx, native)
	if err != nil {
		return nil, err
	}
	return &responseCompletionStream{ctx: ctx, source: stream, scope: scope, model: request.Model, indices: make(map[int]int), items: make(map[int]json.RawMessage)}, nil
}

func (s *responseCompletionStream) chunk(delta openai.ChatCompletionStreamChoiceDelta) openai.ChatCompletionStreamResponse {
	return openai.ChatCompletionStreamResponse{ID: s.id, Model: s.model, Created: s.created, Object: "chat.completion.chunk",
		Choices: []openai.ChatCompletionStreamChoice{{Index: 0, Delta: delta}}}
}

func (s *responseCompletionStream) tool(item openai.ResponseOutputItem, outputIndex int, complete bool) error {
	if item.CallID == "" || item.Name == "" {
		return fmt.Errorf("Responses stream returned an invalid function call")
	}
	index, exists := s.indices[outputIndex]
	if !exists {
		index = len(s.tools)
		s.indices[outputIndex] = index
		s.tools = append(s.tools, &responseToolState{id: item.CallID, name: item.Name})
	} else if s.tools[index].id != item.CallID || s.tools[index].name != item.Name {
		return fmt.Errorf("Responses stream changed a function call identity")
	}
	state := s.tools[index]
	call := openai.ToolCall{Index: &index, Type: openai.ToolTypeFunction}
	if !exists {
		call.ID, call.Function.Name = item.CallID, item.Name
	}
	if complete || item.Arguments != "" {
		previous := state.arguments.String()
		if !strings.HasPrefix(item.Arguments, previous) {
			return fmt.Errorf("Responses stream changed completed function arguments")
		}
		call.Function.Arguments = strings.TrimPrefix(item.Arguments, previous)
		state.arguments.WriteString(call.Function.Arguments)
	}
	if !exists || call.Function.Arguments != "" {
		s.pending = append(s.pending, s.chunk(openai.ChatCompletionStreamChoiceDelta{ToolCalls: []openai.ToolCall{call}}))
	}
	return nil
}

func (s *responseCompletionStream) Recv() (chunk openai.ChatCompletionStreamResponse, err error) {
	defer func() {
		if err != nil && err != io.EOF {
			s.done = true
		}
	}()
	for {
		if err := s.ctx.Err(); err != nil {
			return openai.ChatCompletionStreamResponse{}, err
		}
		if len(s.pending) > 0 {
			chunk := s.pending[0]
			s.pending = s.pending[1:]
			return chunk, nil
		}
		if s.done {
			if s.failure != nil {
				err := s.failure
				s.failure = nil
				return openai.ChatCompletionStreamResponse{}, err
			}
			return openai.ChatCompletionStreamResponse{}, io.EOF
		}
		event, err := s.source.Recv()
		if err != nil {
			s.done = true
			if err == io.EOF {
				err = fmt.Errorf("Responses stream ended before a terminal event: %w", io.ErrUnexpectedEOF)
			}
			return openai.ChatCompletionStreamResponse{}, err
		}
		if event.Response != nil {
			if event.Response.ID != "" {
				s.id = event.Response.ID
			}
			if event.Response.Model != "" {
				s.model = event.Response.Model
			}
			s.created = event.Response.Created
			s.serviceTier = event.Response.ServiceTier
		}
		switch string(event.Type) {
		case "":
			return openai.ChatCompletionStreamResponse{}, fmt.Errorf("Responses stream omitted its event type")
		case "response.created":
			if event.Response == nil {
				return openai.ChatCompletionStreamResponse{}, fmt.Errorf("Responses stream omitted the created response")
			}
			return s.chunk(openai.ChatCompletionStreamChoiceDelta{Role: "assistant"}), nil
		case "response.output_text.delta":
			s.text.WriteString(event.Delta)
			return s.chunk(openai.ChatCompletionStreamChoiceDelta{Content: event.Delta}), nil
		case "response.refusal.delta":
			s.refusal.WriteString(event.Delta)
			return s.chunk(openai.ChatCompletionStreamChoiceDelta{Refusal: event.Delta}), nil
		case "response.output_item.added", "response.output_item.done":
			if event.Item == nil {
				return openai.ChatCompletionStreamResponse{}, fmt.Errorf("Responses stream omitted an output item")
			}
			complete := string(event.Type) == "response.output_item.done"
			if complete {
				var raw struct {
					Item json.RawMessage `json:"item"`
				}
				if err := json.Unmarshal(event.Raw, &raw); err != nil {
					return openai.ChatCompletionStreamResponse{}, fmt.Errorf("decode Responses item: %w", err)
				}
				s.items[event.OutputIndex] = raw.Item
			}
			if event.Item.Type == "function_call" {
				if err := s.tool(*event.Item, event.OutputIndex, complete); err != nil {
					return openai.ChatCompletionStreamResponse{}, err
				}
			}
		case "response.function_call_arguments.delta":
			index, exists := s.indices[event.OutputIndex]
			if !exists {
				return openai.ChatCompletionStreamResponse{}, fmt.Errorf("Responses stream returned arguments before their function call")
			}
			s.tools[index].arguments.WriteString(event.Delta)
			return s.chunk(openai.ChatCompletionStreamChoiceDelta{ToolCalls: []openai.ToolCall{{Index: &index, Function: openai.FunctionCall{Arguments: event.Delta}}}}), nil
		case "response.completed", "response.incomplete", "response.failed":
			if event.Response == nil {
				return openai.ChatCompletionStreamResponse{}, fmt.Errorf("Responses stream omitted its terminal response")
			}
			response := *event.Response
			if len(response.Output) == 0 && len(s.items) > 0 {
				indices := make([]int, 0, len(s.items))
				for index := range s.items {
					indices = append(indices, index)
				}
				sort.Ints(indices)
				for _, index := range indices {
					if raw := s.items[index]; len(raw) > 0 {
						response.Output = append(response.Output, raw)
					}
				}
			}
			completed, err := responseCompletion(response, s.scope)
			if string(response.Status) != strings.TrimPrefix(string(event.Type), "response.") {
				err = fmt.Errorf("Responses stream terminal event disagrees with its response status")
			}
			s.done = true
			if err == nil {
				err = func() error {
					message := completed.Choices[0].Message
					if !strings.HasPrefix(message.Content, s.text.String()) {
						return fmt.Errorf("Responses stream changed its final text")
					}
					if suffix := strings.TrimPrefix(message.Content, s.text.String()); suffix != "" {
						s.pending = append(s.pending, s.chunk(openai.ChatCompletionStreamChoiceDelta{Content: suffix}))
					}
					if !strings.HasPrefix(message.Refusal, s.refusal.String()) {
						return fmt.Errorf("Responses stream changed its final refusal")
					}
					if suffix := strings.TrimPrefix(message.Refusal, s.refusal.String()); suffix != "" {
						s.pending = append(s.pending, s.chunk(openai.ChatCompletionStreamChoiceDelta{Refusal: suffix}))
					}
					for _, tool := range s.tools {
						found := false
						for _, call := range message.ToolCalls {
							found = found || call.ID == tool.id
						}
						if !found {
							return fmt.Errorf("Responses stream omitted a previously emitted function call")
						}
					}
					for index, call := range message.ToolCalls {
						outputIndex := -index - 1
						for key, slot := range s.indices {
							if s.tools[slot].id == call.ID {
								outputIndex = key
								break
							}
						}
						if err := s.tool(openai.ResponseOutputItem{CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments}, outputIndex, true); err != nil {
							return err
						}
					}
					if message.ReasoningContent != "" {
						s.pending = append(s.pending, s.chunk(openai.ChatCompletionStreamChoiceDelta{ReasoningContent: message.ReasoningContent}))
					}
					last := s.chunk(openai.ChatCompletionStreamChoiceDelta{})
					last.Choices[0].FinishReason = completed.Choices[0].FinishReason
					if response.Usage != nil {
						last.Usage = &completed.Usage
					}
					s.pending = append(s.pending, last)
					return nil
				}()
			}
			s.failure = err
			if err != nil && response.Usage != nil {
				s.pending = append(s.pending, openai.ChatCompletionStreamResponse{ID: s.id, Model: s.model, Usage: &completed.Usage})
			}
		case "error":
			s.done = true
			if event.Error != nil {
				s.failure = responseFailure(event.Error.Code, event.Error.Message)
			} else {
				s.failure = responseFailure(event.Code, event.Message)
			}
		}
	}
}

func (s *responseCompletionStream) Close() error {
	s.closeOnce.Do(func() { s.closeError = s.source.Close() })
	return s.closeError
}
