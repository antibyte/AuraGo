package llm

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"
)

const responsesReasoningPrefix = "aurago:responses-reasoning:v1:"

// IsResponsesContinuation identifies opaque native history, never display text.
func IsResponsesContinuation(value string) bool {
	return strings.HasPrefix(value, responsesReasoningPrefix)
}

type responsesContinuation struct {
	Scope string            `json:"scope"`
	Items []json.RawMessage `json:"items"`
}

func responsesScope(route ModelRoute, model string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(route.ProviderType+"|"+route.ProviderID+"|"+route.BaseURL+"|"+model)))
}

func responsesRequest(request openai.ChatCompletionRequest, scope string) (openai.CreateResponseRequest, error) {
	if request.N > 1 || len(request.Stop) > 0 || request.PresencePenalty != 0 || request.FrequencyPenalty != 0 ||
		request.Seed != nil || len(request.LogitBias) > 0 || request.Prediction != nil || len(request.ChatTemplateKwargs) > 0 || len(request.GuidedChoice) > 0 || request.Store {
		return openai.CreateResponseRequest{}, completionConfigError("request contains options unsupported by the stateless Responses bridge")
	}
	store := false
	out := openai.CreateResponseRequest{Model: request.Model, MaxOutputTokens: max(request.MaxTokens, request.MaxCompletionTokens), Store: &store,
		User: request.User, ServiceTier: string(request.ServiceTier), SafetyIdentifier: request.SafetyIdentifier}
	out.Include = append(out.Include, "reasoning.encrypted_content")
	if request.Temperature != 0 {
		out.Temperature = &request.Temperature
	}
	if request.TopP != 0 {
		out.TopP = &request.TopP
	}
	if request.ReasoningEffort != "" {
		out.Reasoning = &openai.ResponseReasoning{Effort: request.ReasoningEffort}
	}
	if request.LogProbs {
		out.Include = append(out.Include, "message.output_text.logprobs")
		out.TopLogprobs = request.TopLogProbs
	}
	if request.ParallelToolCalls != nil {
		parallel, ok := request.ParallelToolCalls.(bool)
		if !ok {
			return out, completionConfigError("parallel_tool_calls must be a boolean")
		}
		out.ParallelToolCalls = &parallel
	}
	for key, value := range request.Metadata {
		if out.Metadata == nil {
			out.Metadata = make(map[string]any)
		}
		out.Metadata[key] = value
	}
	for _, tool := range request.Tools {
		if tool.Type != openai.ToolTypeFunction || tool.Function == nil {
			return out, completionConfigError("Responses bridge supports function tools only")
		}
		definition := *tool.Function
		if definition.Parameters == nil {
			definition.Parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		function := openai.NewResponseFunctionTool(definition)
		function.Parameters["strict"] = tool.Function.Strict
		out.Tools = append(out.Tools, function)
	}
	for _, function := range request.Functions {
		if function.Parameters == nil {
			function.Parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tool := openai.NewResponseFunctionTool(function)
		tool.Parameters["strict"] = function.Strict
		out.Tools = append(out.Tools, tool)
	}
	choice := request.ToolChoice
	if choice == nil {
		choice = request.FunctionCall
	}
	if choice != nil {
		data, err := json.Marshal(choice)
		if err != nil {
			return out, completionConfigError("invalid Responses tool choice: %s", err)
		}
		var mode string
		if json.Unmarshal(data, &mode) == nil {
			if mode != "auto" && mode != "none" && mode != "required" {
				return out, completionConfigError("unsupported Responses tool choice %q", mode)
			}
			out.ToolChoice = mode
		} else {
			var named struct {
				Type     string `json:"type"`
				Name     string `json:"name"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if json.Unmarshal(data, &named) != nil || named.Type != "" && named.Type != "function" {
				return out, completionConfigError("unsupported Responses tool choice")
			}
			if named.Function.Name != "" {
				named.Name = named.Function.Name
			}
			if named.Name == "" {
				return out, completionConfigError("function tool choice requires a name")
			}
			out.ToolChoice = map[string]string{"type": "function", "name": named.Name}
		}
	}
	if format := request.ResponseFormat; format != nil {
		out.Text = &openai.ResponseTextConfig{Format: &openai.ResponseTextFormat{Type: string(format.Type)}}
		if format.Type == openai.ChatCompletionResponseFormatTypeJSONSchema {
			if format.JSONSchema == nil {
				return out, completionConfigError("json_schema requires a schema")
			}
			out.Text.Format.Name, out.Text.Format.Description = format.JSONSchema.Name, format.JSONSchema.Description
			out.Text.Format.Schema, out.Text.Format.Strict = format.JSONSchema.Schema, format.JSONSchema.Strict
		} else if format.Type != openai.ChatCompletionResponseFormatTypeText && format.Type != openai.ChatCompletionResponseFormatTypeJSONObject {
			return out, completionConfigError("unsupported Responses response format")
		}
	}
	if request.Verbosity != "" {
		if out.Text == nil {
			out.Text = &openai.ResponseTextConfig{}
		}
		out.Text.Verbosity = request.Verbosity
	}
	var input []any
	for _, message := range request.Messages {
		if message.Role == openai.ChatMessageRoleTool {
			if message.ToolCallID == "" || len(message.MultiContent) > 0 {
				return out, completionConfigError("tool result requires a call ID and text content")
			}
			input = append(input, openai.ResponseFunctionCallOutput{Type: "function_call_output", CallID: message.ToolCallID, Output: message.Content})
			continue
		}
		if message.Role != "system" && message.Role != "developer" && message.Role != "user" && message.Role != "assistant" || message.FunctionCall != nil || message.Name != "" {
			return out, completionConfigError("unsupported Responses message role or legacy function message")
		}
		if message.Content != "" && len(message.MultiContent) > 0 {
			return out, completionConfigError("message contains both text and multipart content")
		}
		if strings.HasPrefix(message.ReasoningContent, responsesReasoningPrefix) {
			data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(message.ReasoningContent, responsesReasoningPrefix))
			var continuation responsesContinuation
			if err != nil || json.Unmarshal(data, &continuation) != nil {
				return out, completionConfigError("invalid Responses continuation envelope")
			}
			if continuation.Scope == scope {
				var output []any
				for _, item := range continuation.Items {
					output = append(output, item)
				}
				replayed, err := responseCompletion(openai.CreateResponseResponse{Status: openai.ResponseStatusCompleted, Output: output}, scope)
				if err != nil {
					return out, completionConfigError("invalid Responses continuation: %s", err)
				}
				original := replayed.Choices[0].Message
				matches := message.Role == "assistant" && len(message.MultiContent) == 0 && message.Content == original.Content && message.Refusal == original.Refusal && len(message.ToolCalls) == len(original.ToolCalls)
				for i, call := range original.ToolCalls {
					if !matches {
						break
					}
					matches = call.ID == message.ToolCalls[i].ID && call.Function == message.ToolCalls[i].Function
				}
				if matches {
					input = append(input, output...)
					continue
				}
			}
		}
		if message.Content != "" || len(message.MultiContent) > 0 || message.Refusal != "" || len(message.ToolCalls) == 0 {
			var content any = message.Content
			if len(message.MultiContent) > 0 || message.Refusal != "" {
				parts := []any{}
				if message.Content != "" {
					parts = append(parts, map[string]string{"type": "output_text", "text": message.Content})
				}
				for _, part := range message.MultiContent {
					switch part.Type {
					case openai.ChatMessagePartTypeText:
						kind := "input_text"
						if message.Role == "assistant" {
							kind = "output_text"
						}
						parts = append(parts, map[string]string{"type": kind, "text": part.Text})
					case openai.ChatMessagePartTypeImageURL:
						if message.Role != "user" || part.ImageURL == nil {
							return out, completionConfigError("invalid Responses image part")
						}
						parts = append(parts, openai.ResponseInputImage{Type: "input_image", ImageURL: part.ImageURL.URL, Detail: string(part.ImageURL.Detail)})
					default:
						return out, completionConfigError("unsupported Responses content part")
					}
				}
				if message.Refusal != "" {
					parts = append(parts, map[string]string{"type": "refusal", "refusal": message.Refusal})
				}
				content = parts
			}
			input = append(input, openai.ResponseInputMessage{Role: message.Role, Content: content})
		}
		for _, call := range message.ToolCalls {
			if call.ID == "" || call.Function.Name == "" || call.Type != "" && call.Type != openai.ToolTypeFunction {
				return out, completionConfigError("invalid Responses function call")
			}
			input = append(input, map[string]string{"type": "function_call", "call_id": call.ID, "name": call.Function.Name, "arguments": call.Function.Arguments})
		}
	}
	out.Input = input
	return out, nil
}

func responseCompletion(response openai.CreateResponseResponse, scope string) (openai.ChatCompletionResponse, error) {
	out := openai.ChatCompletionResponse{ID: response.ID, Object: "chat.completion", Created: response.Created, Model: response.Model, ServiceTier: openai.ServiceTier(response.ServiceTier)}
	if usage := response.Usage; usage != nil {
		out.Usage = openai.Usage{PromptTokens: usage.InputTokens, CompletionTokens: usage.OutputTokens, TotalTokens: usage.TotalTokens}
		if usage.InputTokensDetails != nil {
			out.Usage.PromptTokensDetails = &openai.PromptTokensDetails{CachedTokens: usage.InputTokensDetails.CachedTokens}
		}
		if usage.OutputTokensDetails != nil {
			out.Usage.CompletionTokensDetails = &openai.CompletionTokensDetails{ReasoningTokens: usage.OutputTokensDetails.ReasoningTokens}
		}
	}
	if response.Error != nil {
		return out, responseFailure(response.Error.Code, response.Error.Message)
	}
	finish := openai.FinishReasonStop
	if response.Status == openai.ResponseStatusIncomplete && response.IncompleteDetails != nil {
		switch response.IncompleteDetails.Reason {
		case "max_output_tokens":
			finish = openai.FinishReasonLength
		case "content_filter":
			finish = openai.FinishReasonContentFilter
		default:
			return out, fmt.Errorf("responses incomplete: %s", response.IncompleteDetails.Reason)
		}
	} else if response.Status != openai.ResponseStatusCompleted {
		return out, fmt.Errorf("responses returned non-terminal status %q", response.Status)
	}
	message := openai.ChatCompletionMessage{Role: "assistant"}
	var nativeItems []json.RawMessage
	hasReasoning := false
	callIDs := make(map[string]bool)
	for _, item := range response.Output {
		raw, err := json.Marshal(item)
		if err != nil {
			return out, fmt.Errorf("encode Responses output: %w", err)
		}
		var output struct {
			Type      string `json:"type"`
			Role      string `json:"role"`
			Status    string `json:"status"`
			CallID    string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Content   []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			} `json:"content"`
		}
		if err := json.Unmarshal(raw, &output); err != nil {
			return out, fmt.Errorf("decode Responses output: %w", err)
		}
		nativeItems = append(nativeItems, raw)
		switch output.Type {
		case "reasoning":
			hasReasoning = true
		case "function_call":
			if response.Status == openai.ResponseStatusCompleted && output.Status != "" && output.Status != "completed" {
				return out, fmt.Errorf("responses returned an unfinished function call")
			}
			var arguments map[string]json.RawMessage
			if output.CallID == "" || callIDs[output.CallID] || output.Name == "" || json.Unmarshal([]byte(output.Arguments), &arguments) != nil || arguments == nil {
				return out, fmt.Errorf("responses returned an invalid function call")
			}
			callIDs[output.CallID] = true
			message.ToolCalls = append(message.ToolCalls, openai.ToolCall{ID: output.CallID, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: output.Name, Arguments: output.Arguments}})
		case "message":
			if output.Role != "" && output.Role != "assistant" || response.Status == openai.ResponseStatusCompleted && output.Status != "" && output.Status != "completed" {
				return out, fmt.Errorf("responses returned an invalid assistant message")
			}
			for _, content := range output.Content {
				switch content.Type {
				case "output_text":
					message.Content += content.Text
				case "refusal":
					message.Refusal += content.Refusal
				default:
					return out, fmt.Errorf("responses returned unsupported message content %q", content.Type)
				}
			}
		default:
			return out, fmt.Errorf("responses returned unsupported output item %q", output.Type)
		}
	}
	if message.Content == "" {
		message.Content = response.OutputText
	}
	if hasReasoning {
		raw, err := json.Marshal(responsesContinuation{Scope: scope, Items: nativeItems})
		if err != nil {
			return out, fmt.Errorf("encode Responses continuation: %w", err)
		}
		message.ReasoningContent = responsesReasoningPrefix + base64.StdEncoding.EncodeToString(raw)
	}
	if finish == openai.FinishReasonStop {
		if message.Refusal != "" {
			finish = openai.FinishReasonContentFilter
		} else if len(message.ToolCalls) > 0 {
			finish = openai.FinishReasonToolCalls
		}
	}
	out.Choices = []openai.ChatCompletionChoice{{Index: 0, Message: message, FinishReason: finish}}
	return out, nil
}

func responseFailure(code, message string) error {
	category := ErrCategoryTemporaryTransport
	switch code {
	case "invalid_api_key", "authentication_error":
		category = ErrCategoryAuthError
	case "rate_limit_exceeded":
		category = ErrCategoryRateLimit
	case "insufficient_quota", "billing_hard_limit_reached":
		category = ErrCategoryQuotaExceeded
	case "invalid_request_error", "invalid_prompt", "unsupported_parameter", "model_not_found":
		category = ErrCategoryNonRetryableConfig
	}
	return &LLMError{Category: category, Message: "Responses API: " + message}
}

func (c openAIClientAdapter) createResponseCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if c.route.ProviderType == "copilot" {
		stream, err := c.createResponseCompletionStream(ctx, request)
		if err != nil {
			return openai.ChatCompletionResponse{}, err
		}
		defer stream.Close()
		var out openai.ChatCompletionResponse
		message := openai.ChatCompletionMessage{Role: "assistant"}
		calls := map[int]*openai.ToolCall{}
		var finish openai.FinishReason
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				out.Choices = nil
				return out, err
			}
			out.ID, out.Model, out.Created, out.Object = chunk.ID, chunk.Model, chunk.Created, "chat.completion"
			if chunk.Usage != nil {
				out.Usage = *chunk.Usage
			}
			for _, choice := range chunk.Choices {
				message.Content += choice.Delta.Content
				message.Refusal += choice.Delta.Refusal
				message.ReasoningContent += choice.Delta.ReasoningContent
				for _, delta := range choice.Delta.ToolCalls {
					if delta.Index == nil {
						return out, fmt.Errorf("Responses function delta omitted its index")
					}
					call := calls[*delta.Index]
					if call == nil {
						call = &openai.ToolCall{Type: openai.ToolTypeFunction}
						calls[*delta.Index] = call
					}
					if delta.ID != "" {
						call.ID = delta.ID
					}
					call.Function.Name += delta.Function.Name
					call.Function.Arguments += delta.Function.Arguments
				}
				if choice.FinishReason != "" {
					finish = choice.FinishReason
				}
			}
		}
		for index := 0; index < len(calls); index++ {
			if call := calls[index]; call != nil {
				message.ToolCalls = append(message.ToolCalls, *call)
			}
		}
		out.Choices = []openai.ChatCompletionChoice{{Index: 0, Message: message, FinishReason: finish}}
		out.ServiceTier = openai.ServiceTier(stream.serviceTier)
		return out, nil
	}
	native, err := responsesRequest(request, responsesScope(c.route, request.Model))
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	response, err := c.client.CreateResponse(ctx, native)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	return responseCompletion(response, responsesScope(c.route, request.Model))
}
