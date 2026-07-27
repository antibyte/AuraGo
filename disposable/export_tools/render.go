package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type catalogDocument struct {
	SchemaVersion  string       `json:"schema_version"`
	Source         string       `json:"source"`
	ToolCount      int          `json:"tool_count"`
	OperationCount int          `json:"operation_count"`
	CallFormat     string       `json:"call_format"`
	Tools          []ToolExport `json:"tools"`
}

type artifactManifest struct {
	SchemaVersion  string            `json:"schema_version"`
	ScenarioCount  int               `json:"scenario_count"`
	ChallengeCount int               `json:"challenge_count"`
	ToolCount      int               `json:"tool_count"`
	OperationCount int               `json:"operation_count"`
	LanguageCounts map[string]int    `json:"language_counts"`
	KindCounts     map[string]int    `json:"kind_counts"`
	SplitCounts    map[string]int    `json:"split_counts"`
	SHA256         map[string]string `json:"sha256"`
}

type shareGPTMessage struct {
	From  string `json:"from"`
	Value string `json:"value"`
}

type shareGPTRow struct {
	ID            string            `json:"id"`
	Conversations []shareGPTMessage `json:"conversations"`
	Tools         []ToolDefinition  `json:"tools,omitempty"`
	Source        string            `json:"source"`
	Split         string            `json:"split"`
}

type alpacaRow struct {
	ID          string `json:"id"`
	Instruction string `json:"instruction"`
	Input       string `json:"input"`
	Output      string `json:"output"`
	Source      string `json:"source"`
	Split       string `json:"split"`
}

func buildTaggedScenarios(scenarios []Scenario) ([]TaggedScenario, error) {
	tagged := make([]TaggedScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		messages := make([]Message, 0, len(scenario.Messages))
		for _, message := range scenario.Messages {
			converted := message
			if len(message.ToolCalls) > 0 {
				var builder strings.Builder
				if strings.TrimSpace(message.Content) != "" {
					builder.WriteString(strings.TrimSpace(message.Content))
					builder.WriteString("\n\n")
				}
				for index, call := range message.ToolCalls {
					if index > 0 {
						builder.WriteByte('\n')
					}
					var arguments map[string]interface{}
					if err := json.Unmarshal([]byte(call.Function.Arguments), &arguments); err != nil {
						return nil, fmt.Errorf("scenario %s has invalid call arguments: %w", scenario.ID, err)
					}
					payload := map[string]interface{}{
						"id":        call.ID,
						"name":      call.Function.Name,
						"arguments": arguments,
					}
					raw, _ := json.Marshal(payload)
					builder.WriteString("<tool_call>\n")
					builder.Write(raw)
					builder.WriteString("\n</tool_call>")
				}
				converted.Content = builder.String()
				converted.ToolCalls = nil
			}
			messages = append(messages, converted)
		}
		tagged = append(tagged, TaggedScenario{
			ID:            scenario.ID,
			SchemaVersion: scenario.SchemaVersion,
			Family:        scenario.Family,
			Language:      scenario.Language,
			Source:        scenario.Source,
			Tier:          scenario.Tier,
			Kind:          scenario.Kind,
			Split:         scenario.Split,
			Tools:         scenario.Tools,
			Messages:      messages,
			Expectations:  scenario.Expectations,
		})
	}
	return tagged, nil
}

func writeGeneratedArtifacts(outDir string, result BuildResult) error {
	catalog := catalogDocument{
		SchemaVersion:  datasetSchemaVersion,
		Source:         "aurago internal/agent.BuildNativeToolSchemaSnapshot.StrictSchemas (all feature flags enabled)",
		ToolCount:      len(result.Tools),
		OperationCount: result.OperationCount,
		CallFormat:     "OpenAI-compatible native function calling with assistant tool_calls and role=tool results linked by tool_call_id.",
		Tools:          result.Tools,
	}
	if err := writeJSON(filepath.Join(outDir, "tools_catalog.json"), catalog); err != nil {
		return err
	}
	if err := writeMarkdownCatalog(filepath.Join(outDir, "tools_catalog.md"), result.Tools, result.OperationCount); err != nil {
		return err
	}
	if err := writeCompactTools(filepath.Join(outDir, "tools_list_compact.json"), result.Tools); err != nil {
		return err
	}

	if err := writeJSONL(filepath.Join(outDir, "dataset_native_fc.jsonl"), result.Scenarios); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(outDir, "dataset_tagged_fc.jsonl"), result.Tagged); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(outDir, "dataset_challenge_native_fc.jsonl"), result.Challenge); err != nil {
		return err
	}
	for _, split := range []string{"train", "validation", "test"} {
		if err := writeJSONL(
			filepath.Join(outDir, "dataset_native_fc_"+split+".jsonl"),
			filterScenarios(result.Scenarios, split),
		); err != nil {
			return err
		}
		if err := writeJSONL(
			filepath.Join(outDir, "dataset_tagged_fc_"+split+".jsonl"),
			filterTagged(result.Tagged, split),
		); err != nil {
			return err
		}
	}

	if err := writeCompatibilityArtifacts(outDir, result); err != nil {
		return err
	}
	return writeArtifactManifest(outDir, result)
}

func filterScenarios(rows []Scenario, split string) []Scenario {
	out := make([]Scenario, 0)
	for _, row := range rows {
		if row.Split == split {
			out = append(out, row)
		}
	}
	return out
}

func filterTagged(rows []TaggedScenario, split string) []TaggedScenario {
	out := make([]TaggedScenario, 0)
	for _, row := range rows {
		if row.Split == split {
			out = append(out, row)
		}
	}
	return out
}

func writeCompatibilityArtifacts(outDir string, result BuildResult) error {
	shareRows := make([]shareGPTRow, 0, len(result.Tagged))
	alpacaRows := make([]alpacaRow, 0, len(result.Tagged))
	for _, row := range result.Tagged {
		share := shareGPTRow{
			ID:     row.ID,
			Tools:  row.Tools,
			Source: "derived:" + row.Source,
			Split:  row.Split,
		}
		var firstUser string
		var assistantParts []string
		for _, message := range row.Messages {
			from := map[string]string{
				"system":    "system",
				"user":      "human",
				"assistant": "gpt",
				"tool":      "tool",
			}[message.Role]
			if from == "" {
				continue
			}
			share.Conversations = append(share.Conversations, shareGPTMessage{From: from, Value: message.Content})
			if message.Role == "user" && firstUser == "" {
				firstUser = message.Content
			}
			if message.Role == "assistant" {
				assistantParts = append(assistantParts, message.Content)
			}
		}
		shareRows = append(shareRows, share)
		alpacaRows = append(alpacaRows, alpacaRow{
			ID:          row.ID,
			Instruction: "Respond as AuraGo. This is a derived compatibility row; use the native dataset for function-calling training.",
			Input:       firstUser,
			Output:      strings.Join(assistantParts, "\n\n"),
			Source:      "derived:" + row.Source,
			Split:       row.Split,
		})
	}
	if err := writeJSONL(filepath.Join(outDir, "dataset_sharegpt.jsonl"), shareRows); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(outDir, "dataset_chatml.jsonl"), result.Scenarios); err != nil {
		return err
	}
	return writeJSONL(filepath.Join(outDir, "dataset_alpaca.jsonl"), alpacaRows)
}

func writeArtifactManifest(outDir string, result BuildResult) error {
	manifest := artifactManifest{
		SchemaVersion:  datasetSchemaVersion,
		ScenarioCount:  len(result.Scenarios),
		ChallengeCount: len(result.Challenge),
		ToolCount:      len(result.Tools),
		OperationCount: result.OperationCount,
		LanguageCounts: map[string]int{},
		KindCounts:     map[string]int{},
		SplitCounts:    map[string]int{},
		SHA256:         map[string]string{},
	}
	for _, scenario := range result.Scenarios {
		manifest.LanguageCounts[scenario.Language]++
		manifest.KindCounts[scenario.Kind]++
		manifest.SplitCounts[scenario.Split]++
	}
	for _, name := range generatedArtifactNamesWithoutManifest() {
		digest, err := sha256File(filepath.Join(outDir, name))
		if err != nil {
			return fmt.Errorf("hash generated artifact %s: %w", name, err)
		}
		manifest.SHA256[name] = digest
	}
	return writeJSON(filepath.Join(outDir, "dataset_manifest.json"), manifest)
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func generatedArtifactNamesWithoutManifest() []string {
	return []string{
		"tools_catalog.json",
		"tools_catalog.md",
		"tools_list_compact.json",
		"dataset_native_fc.jsonl",
		"dataset_native_fc_train.jsonl",
		"dataset_native_fc_validation.jsonl",
		"dataset_native_fc_test.jsonl",
		"dataset_tagged_fc.jsonl",
		"dataset_tagged_fc_train.jsonl",
		"dataset_tagged_fc_validation.jsonl",
		"dataset_tagged_fc_test.jsonl",
		"dataset_challenge_native_fc.jsonl",
		"dataset_sharegpt.jsonl",
		"dataset_chatml.jsonl",
		"dataset_alpaca.jsonl",
	}
}

func generatedArtifactNames() []string {
	return append(generatedArtifactNamesWithoutManifest(), "dataset_manifest.json")
}

func writeJSON(path string, value interface{}) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeJSONL(path string, rows interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encode := func(value interface{}) error {
		if err := encoder.Encode(value); err != nil {
			return fmt.Errorf("encode %s: %w", path, err)
		}
		return nil
	}
	switch typed := rows.(type) {
	case []Scenario:
		for _, row := range typed {
			if err := encode(row); err != nil {
				_ = file.Close()
				return err
			}
		}
	case []TaggedScenario:
		for _, row := range typed {
			if err := encode(row); err != nil {
				_ = file.Close()
				return err
			}
		}
	case []shareGPTRow:
		for _, row := range typed {
			if err := encode(row); err != nil {
				_ = file.Close()
				return err
			}
		}
	case []alpacaRow:
		for _, row := range typed {
			if err := encode(row); err != nil {
				_ = file.Close()
				return err
			}
		}
	default:
		_ = file.Close()
		return fmt.Errorf("unsupported JSONL row type %T", rows)
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		return fmt.Errorf("flush %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

func writeCompactTools(path string, tools []ToolExport) error {
	type compactTool struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Required    []string       `json:"required,omitempty"`
		Tier        string         `json:"tier"`
		Operations  []OperationRef `json:"operations,omitempty"`
	}
	rows := make([]compactTool, 0, len(tools))
	for _, tool := range tools {
		rows = append(rows, compactTool{
			Name:        tool.Name,
			Description: tool.Description,
			Required:    tool.Required,
			Tier:        tool.Tier,
			Operations:  tool.Operations,
		})
	}
	return writeJSON(path, rows)
}

func writeMarkdownCatalog(path string, tools []ToolExport, operationCount int) error {
	var builder strings.Builder
	builder.WriteString("# AuraGo Native Tools Catalog\n\n")
	builder.WriteString("Generated deterministically from `BuildNativeToolSchemaSnapshot(...).StrictSchemas()` with all feature flags enabled.\n\n")
	builder.WriteString(fmt.Sprintf("- Tools: **%d**\n", len(tools)))
	builder.WriteString(fmt.Sprintf("- Enumerated operations: **%d**\n", operationCount))
	builder.WriteString("- Native format: assistant `tool_calls` followed by adjacent `role=tool` messages with matching `tool_call_id`.\n")
	builder.WriteString("- Hidden format: `discover_tools`, then the returned binding `call_method` such as `invoke_tool`.\n\n")
	builder.WriteString("The checked-in `operation_contracts.json` contains the validated training fixture for every operation. Manual code blocks are documentation and are not silently imported as training calls.\n\n")
	for _, tool := range tools {
		builder.WriteString(fmt.Sprintf("## `%s`\n\n", tool.Name))
		builder.WriteString(tool.Description + "\n\n")
		builder.WriteString(fmt.Sprintf("- Tier: `%s`\n", tool.Tier))
		if len(tool.Required) > 0 {
			builder.WriteString("- Required: `" + strings.Join(tool.Required, "`, `") + "`\n")
		}
		if len(tool.Operations) > 0 {
			builder.WriteString(fmt.Sprintf("- Operations: %d\n", len(tool.Operations)))
		}
		if tool.ManualPath != "" {
			builder.WriteString(fmt.Sprintf("- Manual: `%s`\n", tool.ManualPath))
		}
		builder.WriteString("\n")
		if len(tool.Properties) > 0 {
			builder.WriteString("| Parameter | Type | Description |\n|---|---|---|\n")
			names := make([]string, 0, len(tool.Properties))
			for name := range tool.Properties {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				property, _ := tool.Properties[name].(map[string]interface{})
				typ := schemaType(property)
				description, _ := property["description"].(string)
				description = strings.ReplaceAll(description, "\n", " ")
				builder.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", name, typ, description))
			}
			builder.WriteString("\n")
		}
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
