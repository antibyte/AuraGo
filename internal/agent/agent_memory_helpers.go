package agent

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

// Agent memory and telemetry helper functions for ExecuteAgentLoop.

const coreMemCacheTTL = 5 * time.Minute

func retrievalLatencyBucket(elapsed time.Duration) string {
	switch {
	case elapsed < 50*time.Millisecond:
		return "lt_50ms"
	case elapsed < 150*time.Millisecond:
		return "50_149ms"
	case elapsed < 500*time.Millisecond:
		return "150_499ms"
	case elapsed < time.Second:
		return "500_999ms"
	default:
		return "ge_1000ms"
	}
}

func retrievalPromptTokenBucket(tokens int) string {
	switch {
	case tokens <= 0:
		return "0"
	case tokens <= 128:
		return "1_128"
	case tokens <= 384:
		return "129_384"
	case tokens <= 768:
		return "385_768"
	default:
		return "ge_769"
	}
}

func retrievalPromptShareBucket(tokens, budget int) string {
	if tokens <= 0 || budget <= 0 {
		return "0_pct"
	}
	share := (float64(tokens) / float64(budget)) * 100
	switch {
	case share <= 10:
		return "1_10_pct"
	case share <= 25:
		return "11_25_pct"
	case share <= 40:
		return "26_40_pct"
	default:
		return "gt_40_pct"
	}
}

func recordRetrievalPromptTelemetry(scope AgentTelemetryScope, retrievalTokens, tokenBudget int) {
	RecordRetrievalEventForScope(scope, "memory_prompt_tokens:"+retrievalPromptTokenBucket(retrievalTokens))
	RecordRetrievalEventForScope(scope, "memory_prompt_share:"+retrievalPromptShareBucket(retrievalTokens, tokenBudget))
	if retrievalTokens > 0 {
		share := 0
		if tokenBudget > 0 {
			share = int(math.Round((float64(retrievalTokens) / float64(tokenBudget)) * 100))
		}
		RecordRetrievalEventForScope(scope, fmt.Sprintf("memory_prompt_share_value:%d", share))
	}
}

const ragRefreshAfterToolIterations = 2

func shouldRefreshRAG(query, lastQuery string, toolIterations int, lastResponseWasTool bool) bool {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return false
	}
	if trimmedQuery != strings.TrimSpace(lastQuery) {
		return true
	}
	if lastResponseWasTool {
		return false
	}
	return toolIterations >= ragRefreshAfterToolIterations
}

func buildTrimmedContextRecap(messages []openai.ChatCompletionMessage, tokenBudget int) string {
	if len(messages) == 0 || tokenBudget <= 0 {
		return ""
	}
	start := 0
	if len(messages) > 6 {
		start = len(messages) - 6
	}
	var builder strings.Builder
	builder.WriteString("[TRIMMED_CONTEXT_RECAP]: Older conversation content was condensed to stay within the model context window. Use this only as supporting context and do not quote it verbatim.\n")
	if start > 0 {
		builder.WriteString(fmt.Sprintf("Earlier omitted messages before this recap: %d\n", start))
	}
	for _, msg := range messages[start:] {
		content := strings.Join(strings.Fields(messageText(msg)), " ")
		if content == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(msg.Role)
		builder.WriteString(": ")
		builder.WriteString(Truncate(content, 220))
		builder.WriteString("\n")
	}
	recap := strings.TrimSpace(builder.String())
	if recap == "" {
		return ""
	}
	// Estimate target char length from token budget (approx 4 chars per token)
	// then verify with a single CountTokens call.
	estChars := tokenBudget * 4
	if len(recap) > estChars {
		for estChars > 0 && estChars < len(recap) && !utf8.RuneStart(recap[estChars]) {
			estChars--
		}
		recap = strings.TrimSpace(recap[:estChars])
	}
	if prompts.CountTokens(recap) > tokenBudget {
		if len(recap) > 160 {
			recap = strings.TrimSpace(Truncate(recap, len(recap)/2))
		}
		for recap != "" && prompts.CountTokens(recap) > tokenBudget {
			if len(recap) <= 160 {
				return ""
			}
			recap = strings.TrimSpace(Truncate(recap, len(recap)-(len(recap)/4)))
		}
	}
	return recap
}
