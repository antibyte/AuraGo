package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"aurago/internal/buildinfo"

	"github.com/sashabaranov/go-openai"
)

type promptLogRecovery struct {
	ConsecutiveErrors int `json:"consecutive_errors"`
	TotalErrors       int `json:"total_errors"`
	DuplicateTools    int `json:"duplicate_tool_count"`
	Retry422Count     int `json:"retry_422_count"`
	ToolCallCount     int `json:"tool_call_count"`
}

type promptLogEntry struct {
	Time            string                         `json:"time"`
	Provider        string                         `json:"provider"`
	Model           string                         `json:"model"`
	BuildID         string                         `json:"build_id"`
	VCSRevision     string                         `json:"vcs_revision"`
	VCSModified     bool                           `json:"vcs_modified"`
	PromptRevision  string                         `json:"prompt_revision"`
	ToolsCount      int                            `json:"tools_count"`
	ActiveTools     []string                       `json:"active_tools"`
	ToolCatalogHash string                         `json:"tool_catalog_hash"`
	Recovery        promptLogRecovery              `json:"recovery"`
	Messages        []openai.ChatCompletionMessage `json:"messages"`
}

func newPromptLogEntry(req openai.ChatCompletionRequest, provider string, recovery toolRecoveryState, retry422Count, toolCallCount int) promptLogEntry {
	build := buildinfo.Current()
	activeTools := make([]string, 0, len(req.Tools))
	toolsByName := append([]openai.Tool(nil), req.Tools...)
	sort.SliceStable(toolsByName, func(i, j int) bool {
		return nativeToolSortName(toolsByName[i]) < nativeToolSortName(toolsByName[j])
	})
	for _, tool := range toolsByName {
		if tool.Function != nil && strings.TrimSpace(tool.Function.Name) != "" {
			activeTools = append(activeTools, strings.TrimSpace(tool.Function.Name))
		}
	}
	return promptLogEntry{
		Time: time.Now().UTC().Format(time.RFC3339), Provider: provider, Model: req.Model,
		BuildID: build.BuildID, VCSRevision: build.VCSRevision, VCSModified: build.VCSModified,
		PromptRevision: promptMessagesRevision(req.Messages), ToolsCount: len(req.Tools),
		ActiveTools: activeTools, ToolCatalogHash: promptToolCatalogHash(toolsByName),
		Recovery: promptLogRecovery{
			ConsecutiveErrors: recovery.ConsecutiveErrorCount,
			TotalErrors:       recovery.TotalErrorCount,
			DuplicateTools:    recovery.DuplicateToolCount,
			Retry422Count:     retry422Count,
			ToolCallCount:     toolCallCount,
		},
		Messages: req.Messages,
	}
}

func promptMessagesRevision(messages []openai.ChatCompletionMessage) string {
	hash := sha256.New()
	for _, message := range messages {
		if message.Role != openai.ChatMessageRoleSystem {
			continue
		}
		_, _ = hash.Write([]byte(message.Content))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)[:12])
}

func promptToolCatalogHash(tools []openai.Tool) string {
	encoded, _ := json.Marshal(tools)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:12])
}
