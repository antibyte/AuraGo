package budget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

type TokenCounter func(context.Context, string, string) int

var ErrCompletionBudgetBlocked = errors.New("auxiliary completion budget exhausted")
var ErrCompletionBudgetUnavailable = errors.New("auxiliary completion context budget unavailable")

// CompleteChat owns admission and response accounting for one auxiliary call.
// It installs a fresh capture so a parent's measurements cannot be booked twice.
// Callers retain response usage for telemetry, but must not book it again.
func CompleteChat(ctx context.Context, client llm.ChatClient, request openai.ChatCompletionRequest, route llm.ModelRoute, category string, maxOutput int, counters ...TokenCounter) (openai.ChatCompletionResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	if client == nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("auxiliary completion client unavailable")
	}
	tracker, inheritedCategory := Tracking(ctx)
	if category == "" {
		category = inheritedCategory
	}
	if tracker != nil && tracker.IsBlocked(category) {
		return openai.ChatCompletionResponse{}, ErrCompletionBudgetBlocked
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if route.Model == "" {
		route.Model = request.Model
	}
	if route.ProviderType == "" {
		if provider, ok := client.(llm.RequestRouteProvider); ok {
			routes := provider.CandidateRoutes(request)
			for _, candidate := range routes {
				if candidate.Primary || candidate.Model == request.Model {
					route = candidate
					break
				}
			}
		}
	}
	if maxOutput <= 0 {
		maxOutput = 2048
	}
	requested := request.MaxTokens
	if request.MaxCompletionTokens > 0 {
		requested = request.MaxCompletionTokens
	}
	if requested <= 0 || requested > maxOutput {
		requested = maxOutput
	}
	count := TokenCounter(func(_ context.Context, text, _ string) int { return len(text) })
	if len(counters) > 0 && counters[0] != nil {
		count = counters[0]
	}
	inputEstimate := completionInputEstimate(ctx, request, count)
	limits := llm.ResolveModelLimitsCached(route, 0)
	// Callers supply the complete output allowance, including reasoning.
	// Never silently expand an explicitly bounded application request.
	limits.Reasoning = false
	outputLimit, err := llm.JSONCompletionOutputBudget(limits, requested, inputEstimate)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("%w: %v", ErrCompletionBudgetUnavailable, err)
	}
	request.MaxTokens, request.MaxCompletionTokens = outputLimit, 0
	if errors.Is(openai.NewReasoningValidator().Validate(request), openai.ErrReasoningModelMaxTokensDeprecated) {
		request.MaxTokens, request.MaxCompletionTokens, request.Temperature = 0, outputLimit, 0
	}
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	callCtx, capture := llm.CaptureUsage(ctx)
	response, callErr := client.CreateChatCompletion(callCtx, request)
	input, output, observed := completionResponseUsage(ctx, request, response, inputEstimate, count)
	if tracker != nil {
		model := response.Model
		if model == "" {
			model = request.Model
		}
		tracker.RecordResponseUsage(category, route.ProviderType, model, capture, input, output, observed)
	}
	return response, callErr
}

func completionInputEstimate(ctx context.Context, request openai.ChatCompletionRequest, count TokenCounter) int {
	total := 3
	for _, message := range request.Messages {
		total += 4 + count(ctx, message.Content+message.ReasoningContent, request.Model)
		for _, part := range message.MultiContent {
			total += count(ctx, part.Text, request.Model)
			if part.ImageURL != nil {
				total += 85
			} // Minimum image charge; measured provider usage remains authoritative.
		}
		for _, call := range message.ToolCalls {
			total += count(ctx, call.Function.Name+call.Function.Arguments, request.Model)
		}
		if call := message.FunctionCall; call != nil {
			total += count(ctx, call.Name+call.Arguments, request.Model)
		}
	}
	if len(request.Tools) > 0 {
		if encoded, err := json.Marshal(request.Tools); err == nil {
			total += count(ctx, string(encoded), request.Model)
		}
	}
	if len(request.Functions) > 0 {
		if encoded, err := json.Marshal(request.Functions); err == nil {
			total += count(ctx, string(encoded), request.Model)
		}
	}
	return total
}

func completionResponseUsage(ctx context.Context, request openai.ChatCompletionRequest, response openai.ChatCompletionResponse, inputEstimate int, count TokenCounter) (int, int, bool) {
	input, output, total := max(0, response.Usage.PromptTokens), max(0, response.Usage.CompletionTokens), max(0, response.Usage.TotalTokens)
	var text strings.Builder
	observed := input > 0 || output > 0 || total > 0
	if response.Usage.PromptTokensDetails != nil && response.Usage.PromptTokensDetails.CachedTokens > 0 {
		observed = true
	}
	for _, choice := range response.Choices {
		text.WriteString(choice.Message.Content)
		text.WriteString(choice.Message.ReasoningContent)
		for _, call := range choice.Message.ToolCalls {
			text.WriteString(call.Function.Name)
			text.WriteString(call.Function.Arguments)
		}
		if call := choice.Message.FunctionCall; call != nil {
			text.WriteString(call.Name)
			text.WriteString(call.Arguments)
		}
	}
	if text.Len() > 0 {
		observed = true
	}
	if !observed {
		return 0, 0, false
	}
	if total > 0 {
		if input == 0 && output > 0 {
			input = max(0, total-output)
		} else if output == 0 && input > 0 {
			output = max(0, total-input)
		} else if input == 0 && output == 0 {
			input = min(total, inputEstimate)
			output = total - input
		}
	} else {
		if input == 0 {
			input = inputEstimate
		}
		if output == 0 && text.Len() > 0 {
			model := response.Model
			if model == "" {
				model = request.Model
			}
			output = count(ctx, text.String(), model)
		}
	}
	return input, output, true
}
