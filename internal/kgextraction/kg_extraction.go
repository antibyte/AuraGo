// Package kgextraction provides source-agnostic knowledge graph entity extraction
// via LLM. It is intentionally decoupled from both the agent and services packages
// to avoid import cycles.
package kgextraction

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

// ExtractKGFromText performs source-agnostic entity extraction from arbitrary text.
// It builds the LLM prompt, executes the extraction call, parses the response, and returns
// nodes and edges. This function does not interact with the knowledge graph directly.
func ExtractKGFromText(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, inputText string, existingNodesString string) ([]memory.Node, []memory.Edge, error) {
	if len(inputText) < 50 {
		return nil, nil, fmt.Errorf("input text too short for extraction")
	}

	prompt := fmt.Sprintf(`Extract entities and relationships from this conversation.
Return ONLY valid JSON with this exact structure:
{
  "nodes": [{"id": "lowercase_id", "label": "Display Label", "properties": {"type": "person|device|service|software|location|project|concept|event"}}],
  "edges": [{"source": "node_id", "target": "node_id", "relation": "relationship_type"}]
}

Rules:
- IDs must be lowercase with underscores (e.g. "john_doe", "home_server").
- REUSE existing node IDs if the entity matches an existing one.
- Extract only clear, factual entities.
- Vocabulary for types: person, device, service, software, location, project, concept, event.
- Vocabulary for relationships: runs_on, owns, manages, uses, depends_on, connected_to, related_to, part_of, deployed_on, located_in.
- Limit to highly relevant facts. Maximum 15 nodes and 20 edges.

Example:
Excerpt: "I installed adguard on my truenas server at 192.168.1.5"
JSON:
{
  "nodes": [
    {"id": "adguard", "label": "AdGuard", "properties": {"type": "software"}},
    {"id": "truenas", "label": "TrueNAS Server", "properties": {"type": "device", "ip": "192.168.1.5"}}
  ],
  "edges": [
    {"source": "adguard", "target": "truenas", "relation": "runs_on"}
  ]
}

Inputs:
%s%s`, existingNodesString, inputText)

	kgClient, kgModel := resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
	if kgClient == nil || kgModel == "" {
		return nil, nil, fmt.Errorf("no helper/main LLM available")
	}

	kgCtx, kgCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer kgCancel()

	resp, err := llm.ExecuteWithRetry(
		kgCtx,
		kgClient,
		openai.ChatCompletionRequest{
			Model: kgModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: "You are an entity extraction engine. Output ONLY valid JSON, no markdown fences."},
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			MaxTokens: 1500,
		},
		logger,
		nil,
	)
	if err != nil || len(resp.Choices) == 0 {
		return nil, nil, fmt.Errorf("LLM call failed: %w", err)
	}

	rawJSON := trimJSONResponse(resp.Choices[0].Message.Content)

	var extracted struct {
		Nodes []memory.Node `json:"nodes"`
		Edges []memory.Edge `json:"edges"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &extracted); err != nil {
		return nil, nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	return extracted.Nodes, extracted.Edges, nil
}

func resolveHelperBackedLLM(cfg *config.Config, fallbackClient llm.ChatClient, fallbackModel string) (llm.ChatClient, string) {
	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		manager := getOrCreateHelperLLMManager(cfg, nil)
		if manager != nil && manager.client != nil {
			return manager.client, helperCfg.Model
		}
		helperClient := llm.NewClientFromProvider(helperCfg.ProviderType, helperCfg.BaseURL, helperCfg.APIKey)
		if helperClient != nil {
			return helperClient, helperCfg.Model
		}
	}
	return fallbackClient, strings.TrimSpace(fallbackModel)
}

func getOrCreateHelperLLMManager(cfg *config.Config, logger *slog.Logger) *helperLLMManager {
	// This is a minimal stub to avoid pulling the entire agent package.
	// In practice, the helper LLM manager is stateful; this function
	// creates a lightweight instance on demand.
	if cfg == nil {
		return nil
	}
	helperCfg := llm.ResolveHelperLLM(cfg)
	if !helperCfg.Enabled || helperCfg.Model == "" {
		return nil
	}
	client := llm.NewClientFromProvider(helperCfg.ProviderType, helperCfg.BaseURL, helperCfg.APIKey)
	if client == nil {
		return nil
	}
	return &helperLLMManager{client: client, cfg: cfg, logger: logger}
}

type helperLLMManager struct {
	client llm.ChatClient
	cfg    *config.Config
	logger *slog.Logger
}

func (m *helperLLMManager) ObserveFallback(_, _ string) {}

func trimJSONResponse(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSpace(content)
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSpace(content)
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}
	return content
}
