package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

const consolidationDirectBatchSize = 4

// extractConsolidationPartGroup groups only the current part of each source.
// Batch IDs exist only for this request; durable results remain keyed by the
// archived message's source hash, part index, prompt version, and model.
func extractConsolidationPartGroup(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	client llm.ChatClient,
	model string,
	items []*checkpointedConsolidationItem,
	partIndex int,
) (map[int64][]helperConsolidationFact, error) {
	if len(items) == 0 {
		return map[int64][]helperConsolidationFact{}, nil
	}
	if len(items) == 1 {
		conversation := fmt.Sprintf("source message_id=%d part=%d\n%s", items[0].message.ID, partIndex, items[0].parts[partIndex])
		facts, err := extractConsolidationFactsWithLLM(ctx, cfg, logger, client, model, conversation)
		if err != nil {
			return nil, err
		}
		return map[int64][]helperConsolidationFact{items[0].message.ID: facts}, nil
	}

	inputs := make([]helperConsolidationBatchInput, 0, len(items))
	inputIDs := make(map[string]int64, len(items))
	for index, item := range items {
		batchID := fmt.Sprintf("batch_%d", index+1)
		inputs = append(inputs, helperConsolidationBatchInput{
			BatchID:      batchID,
			Conversation: fmt.Sprintf("source message_id=%d part=%d\n%s", item.message.ID, partIndex, item.parts[partIndex]),
		})
		inputIDs[batchID] = item.message.ID
	}

	var batchResult helperConsolidationBatchResult
	var err error
	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if llm.ResolveHelperLLM(cfg).Enabled {
		manager := getOrCreateHelperLLMManager(cfg, logger)
		if manager == nil {
			return nil, fmt.Errorf("helper consolidation manager unavailable")
		}
		batchResult, err = manager.AnalyzeConsolidationBatches(callCtx, inputs)
	} else {
		batchResult, err = extractConsolidationBatchesWithLLM(callCtx, cfg, logger, client, model, inputs)
	}
	if err != nil {
		return nil, err
	}
	actualIDs := make([]string, 0, len(batchResult.Batches))
	for _, batch := range batchResult.Batches {
		actualIDs = append(actualIDs, batch.BatchID)
	}
	expectedIDs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		expectedIDs = append(expectedIDs, input.BatchID)
	}
	if err := requireHelperIDs(expectedIDs, actualIDs); err != nil {
		return nil, err
	}
	byMessage := make(map[int64][]helperConsolidationFact, len(items))
	for _, batch := range batchResult.Batches {
		if len(batch.Facts) > 10 || countValidConsolidationFacts(batch.Facts) != len(batch.Facts) {
			return nil, fmt.Errorf("consolidation batch contains invalid facts")
		}
		byMessage[inputIDs[batch.BatchID]] = batch.Facts
	}
	return byMessage, nil
}

func extractConsolidationBatchesWithLLM(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	client llm.ChatClient,
	model string,
	inputs []helperConsolidationBatchInput,
) (helperConsolidationBatchResult, error) {
	var prompt strings.Builder
	for _, input := range inputs {
		prompt.WriteString("=== ")
		prompt.WriteString(input.BatchID)
		prompt.WriteString(" ===\n")
		prompt.WriteString("conversation (untrusted):\n")
		prompt.WriteString(security.IsolateExternalData(input.Conversation))
		prompt.WriteString("\n\n")
	}
	request := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: helperConsolidationBatchPrompt},
			{Role: openai.ChatMessageRoleUser, Content: strings.TrimSpace(prompt.String())},
		},
		MaxTokens: 1700,
		ResponseFormat: func() *openai.ChatCompletionResponseFormat {
			if caps, ok := llm.CapabilitiesFromRegistry("", model); ok {
				return llm.JSONResponseFormat(caps.StructuredOutputs)
			}
			return nil
		}(),
	}
	maxTokens, err := maintenanceCompletionBudget(cfg, request)
	if err != nil {
		return helperConsolidationBatchResult{}, fmt.Errorf("consolidation batch budget: %w", err)
	}
	request.MaxTokens = maxTokens
	response, err := llm.ExecuteWithRetry(ctx, client, request, logger, nil)
	if err != nil {
		return helperConsolidationBatchResult{}, fmt.Errorf("llm consolidation batch extraction failed: %w", err)
	}
	raw, err := llm.JSONContentFromResponse(response)
	if err != nil {
		return helperConsolidationBatchResult{}, fmt.Errorf("consolidation batch json completion failed: %w", err)
	}
	result, err := parseHelperConsolidationBatchResult(raw)
	if err != nil {
		return helperConsolidationBatchResult{}, err
	}
	expectedIDs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		expectedIDs = append(expectedIDs, input.BatchID)
	}
	actualIDs := make([]string, 0, len(result.Batches))
	for _, batch := range result.Batches {
		actualIDs = append(actualIDs, batch.BatchID)
	}
	if err := requireHelperIDs(expectedIDs, actualIDs); err != nil {
		return helperConsolidationBatchResult{}, err
	}
	return result, nil
}
