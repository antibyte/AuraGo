package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/agent"

	openai "github.com/sashabaranov/go-openai"
)

func validateScenario(toolByName map[string]ToolExport, scenario Scenario) error {
	if scenario.ID == "" || scenario.SchemaVersion != datasetSchemaVersion {
		return fmt.Errorf("scenario has invalid identity or schema version")
	}
	available := make(map[string]bool, len(scenario.Tools))
	schemaTokens := 0
	for _, definition := range scenario.Tools {
		name := definition.Function.Name
		if name == "" || available[name] {
			return fmt.Errorf("scenario %s has an empty or duplicate tool definition %q", scenario.ID, name)
		}
		available[name] = true
		raw, _ := json.Marshal(definition)
		schemaTokens += (len(raw) + 3) / 4
	}
	if len(available) > maxToolsPerScenario {
		return fmt.Errorf("scenario %s has %d tools, max %d", scenario.ID, len(available), maxToolsPerScenario)
	}
	if schemaTokens > maxSchemaTokens {
		return fmt.Errorf("scenario %s has approximately %d schema tokens, max %d", scenario.ID, schemaTokens, maxSchemaTokens)
	}

	var calls []ExpectedCall
	seenCallIDs := make(map[string]bool)
	pending := make(map[string]string)
	openAIMessages := make([]openai.ChatCompletionMessage, 0, len(scenario.Messages))
	for _, message := range scenario.Messages {
		converted := openai.ChatCompletionMessage{
			Role:       message.Role,
			Content:    message.Content,
			ToolCallID: message.ToolCallID,
			Name:       message.Name,
		}
		for _, call := range message.ToolCalls {
			if message.Role != openai.ChatMessageRoleAssistant {
				return fmt.Errorf("scenario %s has tool_calls on role %s", scenario.ID, message.Role)
			}
			if call.ID == "" || seenCallIDs[call.ID] {
				return fmt.Errorf("scenario %s has an empty or duplicate tool-call ID %q", scenario.ID, call.ID)
			}
			seenCallIDs[call.ID] = true
			if !available[call.Function.Name] {
				return fmt.Errorf("scenario %s calls unavailable tool %s", scenario.ID, call.Function.Name)
			}
			if _, ok := toolByName[call.Function.Name]; !ok {
				return fmt.Errorf("scenario %s calls unknown tool %s", scenario.ID, call.Function.Name)
			}
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return fmt.Errorf("scenario %s call %s has invalid argument JSON: %w", scenario.ID, call.ID, err)
			}
			pending[call.ID] = call.Function.Name
			calls = append(calls, ExpectedCall{Name: call.Function.Name, Arguments: args})
			converted.ToolCalls = append(converted.ToolCalls, openai.ToolCall{
				ID:   call.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
			})
		}
		if message.Role == openai.ChatMessageRoleTool {
			name, ok := pending[message.ToolCallID]
			if !ok {
				return fmt.Errorf("scenario %s has orphan tool result %s", scenario.ID, message.ToolCallID)
			}
			if message.Name != "" && message.Name != name {
				return fmt.Errorf("scenario %s tool result %s names %s, expected %s", scenario.ID, message.ToolCallID, message.Name, name)
			}
			delete(pending, message.ToolCallID)
		}
		openAIMessages = append(openAIMessages, converted)
	}
	if len(pending) != 0 {
		return fmt.Errorf("scenario %s has %d tool calls without results", scenario.ID, len(pending))
	}
	sanitized, dropped := agent.SanitizeToolMessages(openAIMessages)
	if dropped != 0 || len(sanitized) != len(openAIMessages) {
		return fmt.Errorf("scenario %s is not accepted by AuraGo message integrity checks", scenario.ID)
	}
	if scenario.Expectations.ShouldCall != (len(calls) > 0) {
		return fmt.Errorf("scenario %s should_call does not match its messages", scenario.ID)
	}
	if len(calls) != len(scenario.Expectations.Calls) {
		return fmt.Errorf("scenario %s has %d calls, expectations list %d", scenario.ID, len(calls), len(scenario.Expectations.Calls))
	}
	for i := range calls {
		if calls[i].Name != scenario.Expectations.Calls[i].Name ||
			!jsonEqual(calls[i].Arguments, scenario.Expectations.Calls[i].Arguments) {
			return fmt.Errorf("scenario %s expected call %d does not match messages", scenario.ID, i)
		}
	}
	return nil
}

func validateBuildResult(result BuildResult) error {
	if len(result.Scenarios) != targetScenarioCount {
		return fmt.Errorf("training scenario count is %d, expected %d", len(result.Scenarios), targetScenarioCount)
	}
	if len(result.Tagged) != len(result.Scenarios) {
		return fmt.Errorf("native/tagged scenario count mismatch")
	}
	if len(result.Challenge) != targetChallengeCount {
		return fmt.Errorf("challenge scenario count is %d, expected %d", len(result.Challenge), targetChallengeCount)
	}

	toolByName := scoper(result.Tools)
	ids := make(map[string]bool, len(result.Scenarios)+len(result.Challenge))
	languages := map[string]int{}
	kinds := map[string]int{}
	splits := map[string]int{}
	targetCounts := map[string]int{}
	operationCoverage := map[string]map[string]bool{}
	conversations := make(map[string]string)
	callIDs := make(map[string]string)

	validateCollection := func(scenarios []Scenario, challenge bool) error {
		for _, scenario := range scenarios {
			if ids[scenario.ID] {
				return fmt.Errorf("duplicate scenario ID %s", scenario.ID)
			}
			ids[scenario.ID] = true
			if err := validateScenario(toolByName, scenario); err != nil {
				return err
			}
			for _, message := range scenario.Messages {
				for _, call := range message.ToolCalls {
					if previous := callIDs[call.ID]; previous != "" {
						return fmt.Errorf(
							"scenarios %s and %s reuse tool-call ID %s",
							previous,
							scenario.ID,
							call.ID,
						)
					}
					callIDs[call.ID] = scenario.ID
				}
			}
			if !challenge {
				languages[scenario.Language]++
				kinds[scenario.Kind]++
				splits[scenario.Split]++
				if scenario.Expectations.TargetTool != "" {
					targetCounts[scenario.Expectations.TargetTool]++
				}
				if scenario.Kind == "direct_success" &&
					scenario.Expectations.Selector != "" &&
					(scenario.Language == "de" || scenario.Language == "en") {
					key := fmt.Sprintf(
						"%s:%s:%v",
						scenario.Expectations.TargetTool,
						scenario.Expectations.Selector,
						scenario.Expectations.Value,
					)
					if operationCoverage[key] == nil {
						operationCoverage[key] = map[string]bool{}
					}
					operationCoverage[key][scenario.Language] = true
				}
			}
			raw, _ := json.Marshal(struct {
				Tools    []ToolDefinition `json:"tools"`
				Messages []Message        `json:"messages"`
			}{
				Tools:    scenario.Tools,
				Messages: scenario.Messages,
			})
			sum := sha256.Sum256(raw)
			digest := hex.EncodeToString(sum[:])
			if previous := conversations[digest]; previous != "" {
				return fmt.Errorf("scenarios %s and %s have identical conversations", previous, scenario.ID)
			}
			conversations[digest] = scenario.ID
		}
		return nil
	}
	if err := validateCollection(result.Scenarios, false); err != nil {
		return err
	}
	if err := validateCollection(result.Challenge, true); err != nil {
		return err
	}

	if languages["de"] != targetScenarioCount*60/100 || languages["en"] != targetScenarioCount*40/100 {
		return fmt.Errorf("language mix is de=%d en=%d, expected 3000/2000", languages["de"], languages["en"])
	}
	expectedKinds := map[string]int{
		"direct_success":  2250,
		"multi_call":      1000,
		"discover_invoke": 500,
		"tool_error":      375,
		"tool_recovery":   375,
	}
	for kind, want := range expectedKinds {
		if kinds[kind] != want {
			return fmt.Errorf("scenario kind %s count is %d, expected %d", kind, kinds[kind], want)
		}
	}
	noCall := kinds["clarification"] + kinds["disabled"] + kinds["safety_refusal"] + kinds["irrelevant"] + kinds["missing_parameters"]
	if noCall != 500 {
		return fmt.Errorf("no-call scenario count is %d, expected 500", noCall)
	}
	for split, minimum := range map[string]int{"train": 3750, "validation": 350, "test": 350} {
		if splits[split] < minimum {
			return fmt.Errorf("split %s has only %d rows", split, splits[split])
		}
	}
	for _, tool := range result.Tools {
		minimum := map[string]int{"core": 20, "extended": 10, "rare": 6}[tool.Tier]
		if targetCounts[tool.Name] < minimum {
			return fmt.Errorf("tool %s tier %s has %d scenarios, minimum %d", tool.Name, tool.Tier, targetCounts[tool.Name], minimum)
		}
		for _, operation := range tool.Operations {
			key := fmt.Sprintf("%s:%s:%v", tool.Name, operation.Selector, operation.Value)
			if !operationCoverage[key]["de"] || !operationCoverage[key]["en"] {
				return fmt.Errorf("operation %s lacks bilingual direct coverage", key)
			}
		}
	}
	challengeCounts := make(map[string]int)
	for _, scenario := range result.Challenge {
		challengeCounts[scenario.Expectations.TargetTool]++
	}
	for _, tool := range result.Tools {
		if challengeCounts[tool.Name] != 2 {
			return fmt.Errorf("challenge set has %d rows for %s, expected 2", challengeCounts[tool.Name], tool.Name)
		}
	}
	return validateTaggedRoundTrip(result.Scenarios, result.Tagged)
}

func jsonEqual(left, right interface{}) bool {
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)
	return string(leftRaw) == string(rightRaw)
}

func validateTaggedRoundTrip(native []Scenario, tagged []TaggedScenario) error {
	if len(native) != len(tagged) {
		return fmt.Errorf("tagged roundtrip count mismatch")
	}
	for i := range native {
		if native[i].ID != tagged[i].ID {
			return fmt.Errorf("tagged row %d ID mismatch", i)
		}
		var parsed []ToolCall
		for _, message := range tagged[i].Messages {
			if message.Role != openai.ChatMessageRoleAssistant {
				continue
			}
			calls, err := parseTaggedCalls(message.Content)
			if err != nil {
				return fmt.Errorf("tagged row %s: %w", tagged[i].ID, err)
			}
			parsed = append(parsed, calls...)
		}
		var expected []ToolCall
		for _, message := range native[i].Messages {
			expected = append(expected, message.ToolCalls...)
		}
		if len(parsed) != len(expected) {
			return fmt.Errorf("tagged row %s has %d calls, expected %d", tagged[i].ID, len(parsed), len(expected))
		}
		for index := range expected {
			if parsed[index].ID != expected[index].ID ||
				parsed[index].Function.Name != expected[index].Function.Name ||
				parsed[index].Function.Arguments != expected[index].Function.Arguments {
				return fmt.Errorf("tagged row %s call %d does not roundtrip", tagged[i].ID, index)
			}
		}
	}
	return nil
}

func parseTaggedCalls(content string) ([]ToolCall, error) {
	var calls []ToolCall
	remaining := content
	for {
		start := strings.Index(remaining, "<tool_call>")
		if start < 0 {
			break
		}
		remaining = remaining[start+len("<tool_call>"):]
		end := strings.Index(remaining, "</tool_call>")
		if end < 0 {
			return nil, fmt.Errorf("unclosed <tool_call> tag")
		}
		var payload struct {
			ID        string                 `json:"id"`
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(remaining[:end])), &payload); err != nil {
			return nil, fmt.Errorf("invalid tagged tool call: %w", err)
		}
		raw, _ := json.Marshal(payload.Arguments)
		calls = append(calls, ToolCall{
			ID:   payload.ID,
			Type: "function",
			Function: FunctionCall{
				Name:      payload.Name,
				Arguments: string(raw),
			},
		})
		remaining = remaining[end+len("</tool_call>"):]
	}
	return calls, nil
}
