package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestContextGuardUsesCompactedTokensWhenCompressionSkips(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "run the inspection"},
		nativeToolCallMessage("call-large", "execute_shell", `{"command":"inspect"}`),
		{
			Role:       openai.ChatMessageRoleTool,
			ToolCallID: "call-large",
			Content:    "Tool Output: " + strings.Repeat("large retained output ", 5000),
		},
	}
	cache := newTokenCountCache(32)
	preCompactionTokens := countMessageTokens(messages, "test", cache)

	compacted, compaction := CompactHistoryToolRounds(messages, HistoryCompactionOptions{KeepRecentToolRoundsFull: 0})
	if !compaction.Compacted {
		t.Fatal("expected tool-round compaction")
	}
	if countNativeToolResult(compacted, "call-large") != 0 {
		t.Fatalf("compaction retained old tool result: %#v", compacted)
	}
	postCompactionTokens := countMessageTokens(compacted, "test", cache)
	if postCompactionTokens >= preCompactionTokens {
		t.Fatalf("compaction did not reduce tokens: before=%d after=%d", preCompactionTokens, postCompactionTokens)
	}

	_, _, compression := CompressHistory(context.Background(), compacted, postCompactionTokens, "test", nil, 0, nil)
	if compression.TotalTokens != 0 {
		t.Fatalf("CompressHistory TotalTokens = %d, want zero for its too-few-messages skip", compression.TotalTokens)
	}

	maxHistoryTokens := postCompactionTokens
	guardTokens := contextGuardMessageTokens(preCompactionTokens, compacted, "test", cache, compression, compaction.Compacted, 0)
	if guardTokens != postCompactionTokens {
		t.Fatalf("context guard tokens = %d, want post-compaction %d", guardTokens, postCompactionTokens)
	}
	if guardTokens > maxHistoryTokens {
		t.Fatalf("context guard would hard-trim fitting compacted history: tokens=%d limit=%d", guardTokens, maxHistoryTokens)
	}
}
