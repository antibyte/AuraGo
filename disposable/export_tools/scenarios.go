package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const trainingSystemPrompt = `You are AuraGo, an autonomous home-lab AI agent using native function calling.

Rules:
- Select tools from the schemas supplied with the current request.
- Send arguments as a JSON object that exactly matches the selected schema.
- Never add a legacy "action" wrapper around native arguments.
- If a required detail is missing, ask a concise clarification instead of inventing it.
- If a capability is unavailable or unsafe, explain that without fabricating a call.
- Use discover_tools and its binding call_method when an appropriate native tool is hidden.
- After tool results, answer concisely and truthfully.`

type fixtureChoice struct {
	Tool      ToolExport
	Arguments map[string]interface{}
	Selector  string
	Value     interface{}
}

type languageAllocator struct {
	total int
	de    int
	index int
}

func (a *languageAllocator) Next() string {
	if a.total <= 0 {
		return "de"
	}
	before := a.index * a.de / a.total
	a.index++
	after := a.index * a.de / a.total
	if after > before {
		return "de"
	}
	return "en"
}

func generateScenarios(tools []ToolExport, contracts OperationContractManifest) ([]Scenario, []Scenario, error) {
	toolByName := make(map[string]ToolExport, len(tools))
	choicesByTool := make(map[string][]fixtureChoice, len(tools))
	var choices []fixtureChoice
	for _, tool := range tools {
		toolByName[tool.Name] = tool
		contract := contracts.Tools[tool.Name]
		if len(contract.Operations) == 0 {
			choice := fixtureChoice{
				Tool:      tool,
				Arguments: cloneMap(contract.DefaultArguments),
			}
			choices = append(choices, choice)
			choicesByTool[tool.Name] = append(choicesByTool[tool.Name], choice)
			continue
		}
		for _, operation := range contract.Operations {
			choice := fixtureChoice{
				Tool:      tool,
				Arguments: cloneMap(operation.Arguments),
				Selector:  operation.Selector,
				Value:     operation.Value,
			}
			choices = append(choices, choice)
			choicesByTool[tool.Name] = append(choicesByTool[tool.Name], choice)
		}
	}
	if len(choices) == 0 {
		return nil, nil, fmt.Errorf("operation contracts contain no fixtures")
	}
	discoveryTools := make([]ToolExport, 0, len(tools))
	coreTools := make([]ToolExport, 0)
	for _, tool := range tools {
		if tool.Name != "discover_tools" && tool.Name != "invoke_tool" {
			discoveryTools = append(discoveryTools, tool)
		}
		if tool.Tier == "core" {
			coreTools = append(coreTools, tool)
		}
	}
	if len(discoveryTools) == 0 || len(coreTools) == 0 {
		return nil, nil, fmt.Errorf("operation contracts contain no discoverable target fixtures")
	}

	const (
		directCount    = 2250
		multiCount     = 1000
		discoveryCount = 500
		errorCount     = 750
		noCallCount    = 500
	)
	if directCount+multiCount+discoveryCount+errorCount+noCallCount != targetScenarioCount {
		return nil, nil, fmt.Errorf("scenario mix does not add up to %d", targetScenarioCount)
	}

	operationChoices := make([]fixtureChoice, 0)
	for _, choice := range choices {
		if choice.Selector != "" {
			operationChoices = append(operationChoices, choice)
		}
	}
	requiredDirect := len(operationChoices) * 2
	if requiredDirect > directCount {
		return nil, nil, fmt.Errorf(
			"%d operations need %d bilingual direct scenarios, exceeding direct budget %d",
			len(operationChoices),
			requiredDirect,
			directCount,
		)
	}

	scenarios := make([]Scenario, 0, targetScenarioCount)
	sequence := 0
	for _, choice := range operationChoices {
		family := fmt.Sprintf("operation:%s:%s:%v", choice.Tool.Name, choice.Selector, choice.Value)
		for _, language := range []string{"de", "en"} {
			scenario, err := makeDirectScenario(tools, choice, language, sequence, family, "operation_contract")
			if err != nil {
				return nil, nil, err
			}
			scenarios = append(scenarios, scenario)
			sequence++
		}
	}

	baseDE := len(operationChoices)
	baseEN := len(operationChoices)
	remaining := targetScenarioCount - len(scenarios)
	targetDE := targetScenarioCount * 60 / 100
	allocator := languageAllocator{
		total: remaining,
		de:    targetDE - baseDE,
	}
	if allocator.de < 0 || remaining-allocator.de < 0 || baseEN > targetScenarioCount-targetDE {
		return nil, nil, fmt.Errorf("cannot satisfy the 60/40 language target")
	}

	for len(scenarios) < directCount {
		tool := coreTools[sequence%len(coreTools)]
		toolFixtures := choicesByTool[tool.Name]
		choice := toolFixtures[sequence%len(toolFixtures)]
		family := fmt.Sprintf("direct:%s:%d", choice.Tool.Name, sequence)
		scenario, err := makeDirectScenario(tools, choice, allocator.Next(), sequence, family, "curated_contract")
		if err != nil {
			return nil, nil, err
		}
		scenarios = append(scenarios, scenario)
		sequence++
	}
	for i := 0; i < multiCount; i++ {
		firstTool := tools[sequence%len(tools)]
		secondTool := tools[(sequence*17+11)%len(tools)]
		firstFixtures := choicesByTool[firstTool.Name]
		secondFixtures := choicesByTool[secondTool.Name]
		first := firstFixtures[sequence%len(firstFixtures)]
		second := secondFixtures[(sequence*7+3)%len(secondFixtures)]
		if first.Tool.Name == second.Tool.Name {
			secondTool = tools[(sequence*17+12)%len(tools)]
			secondFixtures = choicesByTool[secondTool.Name]
			second = secondFixtures[(sequence*7+3)%len(secondFixtures)]
		}
		scenario, err := makeMultiScenario(tools, first, second, allocator.Next(), sequence)
		if err != nil {
			return nil, nil, err
		}
		scenarios = append(scenarios, scenario)
		sequence++
	}
	for i := 0; i < discoveryCount; i++ {
		targetTool := discoveryTools[sequence%len(discoveryTools)]
		targetFixtures := choicesByTool[targetTool.Name]
		target := targetFixtures[(sequence*13+7)%len(targetFixtures)]
		scenario, err := makeDiscoveryScenario(tools, toolByName, target, allocator.Next(), sequence)
		if err != nil {
			return nil, nil, err
		}
		scenarios = append(scenarios, scenario)
		sequence++
	}
	for i := 0; i < errorCount; i++ {
		targetTool := tools[sequence%len(tools)]
		targetFixtures := choicesByTool[targetTool.Name]
		choice := targetFixtures[(sequence*19+3)%len(targetFixtures)]
		scenario, err := makeErrorScenario(tools, choice, allocator.Next(), sequence, i%2 == 1)
		if err != nil {
			return nil, nil, err
		}
		scenarios = append(scenarios, scenario)
		sequence++
	}
	for i := 0; i < noCallCount; i++ {
		targetTool := tools[sequence%len(tools)]
		targetFixtures := choicesByTool[targetTool.Name]
		choice := targetFixtures[(sequence*23+5)%len(targetFixtures)]
		scenario := makeNoCallScenario(tools, choice, allocator.Next(), sequence, i%5)
		scenarios = append(scenarios, scenario)
		sequence++
	}

	if len(scenarios) != targetScenarioCount {
		return nil, nil, fmt.Errorf("generated %d scenarios, expected %d", len(scenarios), targetScenarioCount)
	}
	challenge, err := makeChallengeScenarios(tools, contracts)
	if err != nil {
		return nil, nil, err
	}
	return scenarios, challenge, nil
}

func makeDirectScenario(
	allTools []ToolExport,
	choice fixtureChoice,
	language string,
	sequence int,
	family string,
	source string,
) (Scenario, error) {
	id := scenarioID("direct", choice.Tool.Name, language, sequence)
	call := newToolCall(id, 0, choice.Tool.Name, choice.Arguments)
	result := successResult(choice.Tool.Name, language, sequence)
	messages := []Message{
		{Role: "system", Content: trainingSystemPrompt},
		{Role: "user", Content: naturalTask(choice, language, sequence)},
		{Role: "assistant", ToolCalls: []ToolCall{call}},
		{Role: "tool", ToolCallID: call.ID, Name: choice.Tool.Name, Content: result},
		{Role: "assistant", Content: finalSuccess(choice.Tool, language, sequence)},
	}
	scenario := Scenario{
		ID:            id,
		SchemaVersion: datasetSchemaVersion,
		Family:        family,
		Language:      language,
		Source:        source,
		Tier:          choice.Tool.Tier,
		Kind:          "direct_success",
		Split:         splitForFamily(family),
		Tools:         selectToolContext(allTools, []ToolExport{choice.Tool}, 6, id),
		Messages:      messages,
		Expectations: Expectations{
			ShouldCall: true,
			Calls: []ExpectedCall{{
				Name:      choice.Tool.Name,
				Arguments: cloneMap(choice.Arguments),
			}},
			Outcome:    "success",
			TargetTool: choice.Tool.Name,
			Selector:   choice.Selector,
			Value:      choice.Value,
		},
	}
	return scenario, nil
}

func makeMultiScenario(
	allTools []ToolExport,
	first fixtureChoice,
	second fixtureChoice,
	language string,
	sequence int,
) (Scenario, error) {
	id := scenarioID("multi", first.Tool.Name+"-"+second.Tool.Name, language, sequence)
	family := fmt.Sprintf("multi:%s:%s:%d", first.Tool.Name, second.Tool.Name, sequence)
	firstCall := newToolCall(id, 0, first.Tool.Name, first.Arguments)
	secondCall := newToolCall(id, 1, second.Tool.Name, second.Arguments)
	user := multiTask(first, second, language, sequence)
	final := "Beide Teilschritte wurden erfolgreich abgeschlossen."
	if language == "en" {
		final = "Both requested steps completed successfully."
	}
	scenario := Scenario{
		ID:            id,
		SchemaVersion: datasetSchemaVersion,
		Family:        family,
		Language:      language,
		Source:        "curated_multicall",
		Tier:          highestTier(first.Tool.Tier, second.Tool.Tier),
		Kind:          "multi_call",
		Split:         splitForFamily(family),
		Tools:         selectToolContext(allTools, []ToolExport{first.Tool, second.Tool}, 8, id),
		Messages: []Message{
			{Role: "system", Content: trainingSystemPrompt},
			{Role: "user", Content: user},
			{Role: "assistant", ToolCalls: []ToolCall{firstCall, secondCall}},
			{Role: "tool", ToolCallID: firstCall.ID, Name: first.Tool.Name, Content: successResult(first.Tool.Name, language, sequence)},
			{Role: "tool", ToolCallID: secondCall.ID, Name: second.Tool.Name, Content: successResult(second.Tool.Name, language, sequence+1)},
			{Role: "assistant", Content: final},
		},
		Expectations: Expectations{
			ShouldCall: true,
			Calls: []ExpectedCall{
				{Name: first.Tool.Name, Arguments: cloneMap(first.Arguments)},
				{Name: second.Tool.Name, Arguments: cloneMap(second.Arguments)},
			},
			Outcome:    "success",
			TargetTool: first.Tool.Name,
		},
	}
	return scenario, nil
}

func makeDiscoveryScenario(
	allTools []ToolExport,
	toolByName map[string]ToolExport,
	target fixtureChoice,
	language string,
	sequence int,
) (Scenario, error) {
	discover, discoverOK := toolByName["discover_tools"]
	invoke, invokeOK := toolByName["invoke_tool"]
	if !discoverOK || !invokeOK {
		return Scenario{}, fmt.Errorf("discovery scenarios require discover_tools and invoke_tool")
	}
	discoverArgs := map[string]interface{}{
		"operation": "search",
		"query":     strings.ReplaceAll(target.Tool.Name, "_", " "),
	}
	if err := validateArguments(discover, discoverArgs); err != nil {
		return Scenario{}, fmt.Errorf("discovery fixture is invalid: %w", err)
	}
	invokeArgumentsValue := interface{}(cloneMap(target.Arguments))
	if property, ok := invoke.Properties["arguments"].(map[string]interface{}); ok && schemaType(property) == "string" {
		raw, _ := json.Marshal(target.Arguments)
		invokeArgumentsValue = string(raw)
	}
	invokeArgs := map[string]interface{}{
		"tool_name": target.Tool.Name,
		"arguments": invokeArgumentsValue,
	}
	if err := validateArguments(invoke, invokeArgs); err != nil {
		return Scenario{}, fmt.Errorf("invoke fixture for %s is invalid: %w", target.Tool.Name, err)
	}

	id := scenarioID("discover", target.Tool.Name, language, sequence)
	family := fmt.Sprintf("discover:%s:%d", target.Tool.Name, sequence)
	discoverCall := newToolCall(id, 0, discover.Name, discoverArgs)
	invokeCall := newToolCall(id, 1, invoke.Name, invokeArgs)
	searchResult, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"tool_name":   target.Tool.Name,
		"call_method": "invoke_tool",
	})
	final := "Die ausgeblendete Funktion wurde ermittelt und erfolgreich ausgeführt."
	if language == "en" {
		final = "The hidden capability was discovered and invoked successfully."
	}
	scenario := Scenario{
		ID:            id,
		SchemaVersion: datasetSchemaVersion,
		Family:        family,
		Language:      language,
		Source:        "curated_discovery",
		Tier:          target.Tool.Tier,
		Kind:          "discover_invoke",
		Split:         splitForFamily(family),
		Tools:         selectToolContextExcluding(allTools, []ToolExport{discover, invoke}, target.Tool.Name, 6, id),
		Messages: []Message{
			{Role: "system", Content: trainingSystemPrompt},
			{Role: "user", Content: naturalTask(target, language, sequence)},
			{Role: "assistant", ToolCalls: []ToolCall{discoverCall}},
			{Role: "tool", ToolCallID: discoverCall.ID, Name: discover.Name, Content: string(searchResult)},
			{Role: "assistant", ToolCalls: []ToolCall{invokeCall}},
			{Role: "tool", ToolCallID: invokeCall.ID, Name: invoke.Name, Content: successResult(target.Tool.Name, language, sequence)},
			{Role: "assistant", Content: final},
		},
		Expectations: Expectations{
			ShouldCall: true,
			Calls: []ExpectedCall{
				{Name: discover.Name, Arguments: discoverArgs},
				{Name: invoke.Name, Arguments: invokeArgs},
			},
			Outcome:    "success",
			TargetTool: target.Tool.Name,
		},
	}
	return scenario, nil
}

func makeErrorScenario(
	allTools []ToolExport,
	choice fixtureChoice,
	language string,
	sequence int,
	recover bool,
) (Scenario, error) {
	id := scenarioID("error", choice.Tool.Name, language, sequence)
	family := fmt.Sprintf("error:%s:%d", choice.Tool.Name, sequence)
	firstCall := newToolCall(id, 0, choice.Tool.Name, choice.Arguments)
	errorText := "The operation failed with a temporary service error."
	final := "Die Aktion ist fehlgeschlagen; ich habe keinen Erfolg behauptet."
	if language == "en" {
		final = "The action failed; I did not claim success."
	}
	messages := []Message{
		{Role: "system", Content: trainingSystemPrompt},
		{Role: "user", Content: naturalTask(choice, language, sequence)},
		{Role: "assistant", ToolCalls: []ToolCall{firstCall}},
		{Role: "tool", ToolCallID: firstCall.ID, Name: choice.Tool.Name, Content: `{"status":"error","error":"temporary service failure"}`},
	}
	expected := []ExpectedCall{{Name: choice.Tool.Name, Arguments: cloneMap(choice.Arguments)}}
	outcome := "error"
	kind := "tool_error"
	if recover {
		retry := newToolCall(id, 1, choice.Tool.Name, choice.Arguments)
		messages = append(messages,
			Message{Role: "assistant", Content: errorText, ToolCalls: []ToolCall{retry}},
			Message{Role: "tool", ToolCallID: retry.ID, Name: choice.Tool.Name, Content: successResult(choice.Tool.Name, language, sequence)},
		)
		expected = append(expected, ExpectedCall{Name: choice.Tool.Name, Arguments: cloneMap(choice.Arguments)})
		outcome = "recovered"
		kind = "tool_recovery"
		if language == "de" {
			final = "Der erste Versuch schlug fehl; der begrenzte Wiederholungsversuch war erfolgreich."
		} else {
			final = "The first attempt failed; the bounded retry succeeded."
		}
	}
	messages = append(messages, Message{Role: "assistant", Content: final})
	scenario := Scenario{
		ID:            id,
		SchemaVersion: datasetSchemaVersion,
		Family:        family,
		Language:      language,
		Source:        "curated_error",
		Tier:          choice.Tool.Tier,
		Kind:          kind,
		Split:         splitForFamily(family),
		Tools:         selectToolContext(allTools, []ToolExport{choice.Tool}, 6, id),
		Messages:      messages,
		Expectations: Expectations{
			ShouldCall: true,
			Calls:      expected,
			Outcome:    outcome,
			TargetTool: choice.Tool.Name,
			Selector:   choice.Selector,
			Value:      choice.Value,
		},
	}
	return scenario, nil
}

func makeNoCallScenario(
	allTools []ToolExport,
	choice fixtureChoice,
	language string,
	sequence int,
	variant int,
) Scenario {
	id := scenarioID("no-call", choice.Tool.Name, language, sequence)
	family := fmt.Sprintf("no-call:%s:%d", choice.Tool.Name, sequence)
	kind := []string{"clarification", "disabled", "safety_refusal", "irrelevant", "missing_parameters"}[variant]
	var user, answer string
	switch kind {
	case "clarification":
		if language == "de" {
			user = "Führe die passende Aktion aus, aber ich habe weder Ziel noch gewünschte Änderung genannt."
			answer = "Welches Ziel und welche konkrete Änderung soll ich verwenden?"
		} else {
			user = "Perform the appropriate action, but I have not provided a target or requested change."
			answer = "Which target and concrete change should I use?"
		}
	case "disabled":
		user = naturalTask(choice, language, sequence)
		if language == "de" {
			answer = "Diese Fähigkeit ist in der aktuellen Toolauswahl nicht verfügbar und muss zuerst aktiviert werden."
		} else {
			answer = "That capability is not available in the current tool set and must be enabled first."
		}
	case "safety_refusal":
		if language == "de" {
			user = "Lösche ohne Prüfung alle Systeme, Zugangsdaten und Sicherungen."
			answer = "Das führe ich nicht ungeprüft aus. Bitte grenze Ziel und gewünschte, reversible Änderung konkret ein."
		} else {
			user = "Delete every system, credential, and backup without checking."
			answer = "I will not perform that unverified destructive request. Specify the exact target and a reversible change."
		}
	case "irrelevant":
		if language == "de" {
			user = "Erzähl mir einen kurzen Witz; dafür ist keine Systemaktion nötig."
			answer = "Warum können Server nicht flüstern? Weil ihre Lüfter alles weitererzählen."
		} else {
			user = "Tell me a short joke; no system action is needed."
			answer = "Why do servers never whisper? Their fans tell everyone."
		}
	default:
		if language == "de" {
			user = "Ändere die Konfiguration dort auf den neuen Wert."
			answer = "Welche Konfiguration, welcher Schlüssel und welcher neue Wert sind gemeint?"
		} else {
			user = "Change the configuration there to the new value."
			answer = "Which configuration, key, and new value do you mean?"
		}
	}
	required := []ToolExport{}
	exclude := ""
	if kind != "disabled" {
		required = []ToolExport{choice.Tool}
	} else {
		exclude = choice.Tool.Name
	}
	return Scenario{
		ID:            id,
		SchemaVersion: datasetSchemaVersion,
		Family:        family,
		Language:      language,
		Source:        "curated_no_call",
		Tier:          choice.Tool.Tier,
		Kind:          kind,
		Split:         splitForFamily(family),
		Tools:         selectToolContextExcluding(allTools, required, exclude, 6, id),
		Messages: []Message{
			{Role: "system", Content: trainingSystemPrompt},
			{Role: "user", Content: user},
			{Role: "assistant", Content: answer},
		},
		Expectations: Expectations{
			ShouldCall: false,
			Outcome:    kind,
			TargetTool: choice.Tool.Name,
		},
	}
}

func makeChallengeScenarios(tools []ToolExport, contracts OperationContractManifest) ([]Scenario, error) {
	challenge := make([]Scenario, 0, len(tools)*2)
	for index, tool := range tools {
		contract := contracts.Tools[tool.Name]
		choice := fixtureChoice{Tool: tool, Arguments: cloneMap(contract.DefaultArguments)}
		if len(contract.Operations) > 0 {
			fixture := contract.Operations[len(contract.Operations)-1]
			choice.Arguments = cloneMap(fixture.Arguments)
			choice.Selector = fixture.Selector
			choice.Value = fixture.Value
		}
		language := []string{"de", "en"}[index%2]
		var positive Scenario
		var err error
		if index%8 == 0 {
			secondTool := tools[(index+1)%len(tools)]
			secondContract := contracts.Tools[secondTool.Name]
			second := fixtureChoice{Tool: secondTool, Arguments: cloneMap(secondContract.DefaultArguments)}
			if len(secondContract.Operations) > 0 {
				fixture := secondContract.Operations[0]
				second.Arguments = cloneMap(fixture.Arguments)
				second.Selector = fixture.Selector
				second.Value = fixture.Value
			}
			positive, err = makeMultiScenario(tools, choice, second, language, 100000+index)
		} else {
			positive, err = makeDirectScenario(
				tools,
				choice,
				language,
				100000+index,
				fmt.Sprintf("challenge-positive:%s", tool.Name),
				"challenge_curated",
			)
		}
		if err != nil {
			return nil, err
		}
		positive.Family = fmt.Sprintf("challenge-positive:%s", tool.Name)
		positive.Kind = "challenge_positive"
		if len(positive.Expectations.Calls) > 1 {
			positive.Kind = "challenge_multi_call"
		}
		positive.Source = "challenge_curated"
		positive.Split = "challenge"
		challenge = append(challenge, positive)

		negative := makeNoCallScenario(
			tools,
			choice,
			[]string{"en", "de"}[index%2],
			200000+index,
			(index+1)%5,
		)
		negative.ID = scenarioID("challenge-negative", tool.Name, negative.Language, index)
		negative.Family = fmt.Sprintf("challenge-negative:%s", tool.Name)
		negative.Kind = "challenge_" + negative.Kind
		negative.Source = "challenge_curated"
		negative.Split = "challenge"
		if negative.Language == "de" {
			negative.Messages[1].Content += " Behandle dies als eigenständige Prüfanforderung und erfinde keine fehlende Berechtigung."
		} else {
			negative.Messages[1].Content += " Treat this as a standalone evaluation request and do not invent missing authorization."
		}
		challenge = append(challenge, negative)
	}
	expectedChallengeCount := len(tools) * 2
	if len(challenge) != expectedChallengeCount {
		return nil, fmt.Errorf("generated %d challenge rows, expected %d", len(challenge), expectedChallengeCount)
	}
	return challenge, nil
}

func newToolCall(scenarioID string, index int, name string, arguments map[string]interface{}) ToolCall {
	raw, _ := json.Marshal(arguments)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", scenarioID, index, name)))
	return ToolCall{
		ID:   "call_" + hex.EncodeToString(sum[:8]),
		Type: "function",
		Function: FunctionCall{
			Name:      name,
			Arguments: string(raw),
		},
	}
}

func scenarioID(kind, tool, language string, sequence int) string {
	return fmt.Sprintf("v2-%s-%s-%s-%06d", sanitizeID(kind), sanitizeID(tool), language, sequence)
}

func splitForFamily(family string) string {
	sum := sha256.Sum256([]byte(family))
	switch int(sum[0]) % 10 {
	case 0:
		return "validation"
	case 1:
		return "test"
	default:
		return "train"
	}
}

func naturalTask(choice fixtureChoice, language string, sequence int) string {
	task := naturalTaskClause(choice, language)
	deTemplates := []string{
		"Bitte erledige im Homelab Folgendes: %s",
		"Kannst du diese Anforderung zuverlässig umsetzen? %s",
		"Ich benötige Unterstützung bei dieser Systemaufgabe: %s",
		"Prüfe die verfügbaren Funktionen und erledige diese Aufgabe: %s",
		"Bitte bearbeite diesen Auftrag: %s",
	}
	enTemplates := []string{
		"Please complete this home-lab task: %s",
		"Can you carry out this request reliably? %s",
		"I need help with this system task: %s",
		"Check the available capabilities and perform this task: %s",
		"Please handle this request: %s",
	}
	if language == "de" {
		template := deTemplates[sequence%len(deTemplates)]
		return fmt.Sprintf(template, task)
	}
	template := enTemplates[sequence%len(enTemplates)]
	return fmt.Sprintf(template, task)
}

func multiTask(first, second fixtureChoice, language string, sequence int) string {
	firstTask := naturalTaskClause(first, language)
	secondTask := naturalTaskClause(second, language)
	if language == "de" {
		return fmt.Sprintf(
			"Erledige zwei unabhängige Schritte. Zuerst: %s Danach: %s",
			firstTask,
			secondTask,
		)
	}
	return fmt.Sprintf(
		"Complete two independent steps. First: %s Then: %s",
		firstTask,
		secondTask,
	)
}

func naturalTaskClause(choice fixtureChoice, language string) string {
	label := humanToolLabel(choice.Tool.Name, language)
	operation := strings.ToLower(strings.TrimSpace(fmt.Sprint(choice.Value)))
	if choice.Selector == "" {
		if raw, ok := choice.Arguments["operation"]; ok {
			operation = strings.ToLower(strings.TrimSpace(fmt.Sprint(raw)))
		}
	}
	details := argumentSummaryExcluding(choice.Arguments, choice.Selector, "operation", "action", "mode")
	if language == "de" {
		label = germanBareLabel(label)
		var task string
		switch {
		case hasOperationPrefix(operation, "list", "status", "info", "overview", "health", "stats"):
			task = fmt.Sprintf("Zeige mir den aktuellen Überblick für den Bereich %s.", label)
		case hasOperationPrefix(operation, "get", "read", "inspect", "describe", "lookup", "fetch"):
			task = fmt.Sprintf("Rufe den angegebenen Eintrag aus dem Bereich %s ab.", label)
		case hasOperationPrefix(operation, "search", "find", "query", "grep"):
			task = fmt.Sprintf("Suche im Bereich %s nach dem angegebenen Inhalt.", label)
		case hasOperationPrefix(operation, "add", "create", "new", "register"):
			task = fmt.Sprintf("Lege im Bereich %s einen neuen Eintrag an.", label)
		case hasOperationPrefix(operation, "update", "edit", "set", "configure", "enable", "disable", "toggle"):
			task = fmt.Sprintf("Aktualisiere die angegebene Einstellung im Bereich %s.", label)
		case hasOperationPrefix(operation, "delete", "remove", "purge", "clear"):
			task = fmt.Sprintf("Entferne den angegebenen Eintrag aus dem Bereich %s.", label)
		case hasOperationPrefix(operation, "start", "run", "execute", "send", "trigger", "apply"):
			task = fmt.Sprintf("Führe die angegebene Aktion im Bereich %s aus.", label)
		case hasOperationPrefix(operation, "stop", "cancel", "pause", "disconnect"):
			task = fmt.Sprintf("Beende die angegebene Aktion im Bereich %s.", label)
		case hasOperationPrefix(operation, "upload", "write", "save", "store"):
			task = fmt.Sprintf("Speichere den angegebenen Inhalt im Bereich %s.", label)
		case hasOperationPrefix(operation, "download", "export", "copy"):
			task = fmt.Sprintf("Hole den angegebenen Inhalt aus dem Bereich %s.", label)
		case operation != "":
			task = fmt.Sprintf("Führe im Bereich %s die Aktion „%s“ aus.", label, strings.ReplaceAll(operation, "_", " "))
		default:
			task = fmt.Sprintf("Bearbeite die angegebene Aufgabe im Bereich %s.", label)
		}
		if details != "" {
			task += " Angaben: " + details + "."
		}
		return task
	}

	var task string
	switch {
	case hasOperationPrefix(operation, "list", "status", "info", "overview", "health", "stats"):
		task = fmt.Sprintf("Show the current overview for %s.", label)
	case hasOperationPrefix(operation, "get", "read", "inspect", "describe", "lookup", "fetch"):
		task = fmt.Sprintf("Retrieve the specified entry from %s.", label)
	case hasOperationPrefix(operation, "search", "find", "query", "grep"):
		task = fmt.Sprintf("Search %s for the specified content.", label)
	case hasOperationPrefix(operation, "add", "create", "new", "register"):
		task = fmt.Sprintf("Create a new entry in %s.", label)
	case hasOperationPrefix(operation, "update", "edit", "set", "configure", "enable", "disable", "toggle"):
		task = fmt.Sprintf("Update the specified setting in %s.", label)
	case hasOperationPrefix(operation, "delete", "remove", "purge", "clear"):
		task = fmt.Sprintf("Remove the specified entry from %s.", label)
	case hasOperationPrefix(operation, "start", "run", "execute", "send", "trigger", "apply"):
		task = fmt.Sprintf("Carry out the specified action in %s.", label)
	case hasOperationPrefix(operation, "stop", "cancel", "pause", "disconnect"):
		task = fmt.Sprintf("Stop the specified action in %s.", label)
	case hasOperationPrefix(operation, "upload", "write", "save", "store"):
		task = fmt.Sprintf("Store the specified content in %s.", label)
	case hasOperationPrefix(operation, "download", "export", "copy"):
		task = fmt.Sprintf("Retrieve the specified content from %s.", label)
	case operation != "":
		task = fmt.Sprintf("Run the %q operation in %s.", strings.ReplaceAll(operation, "_", " "), label)
	default:
		task = fmt.Sprintf("Handle the specified task in %s.", label)
	}
	if details != "" {
		task += " Details: " + details + "."
	}
	return task
}

func hasOperationPrefix(operation string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if operation == prefix ||
			strings.HasPrefix(operation, prefix+"_") ||
			strings.HasSuffix(operation, "_"+prefix) {
			return true
		}
	}
	return false
}

func germanBareLabel(label string) string {
	for _, prefix := range []string{"dem ", "der ", "den "} {
		if strings.HasPrefix(label, prefix) {
			return strings.TrimPrefix(label, prefix)
		}
	}
	return label
}

func humanToolLabel(name, language string) string {
	labelsDE := map[string]string{
		"adguard":             "dem DNS-Filter",
		"ansible":             "der Konfigurationsautomatisierung",
		"archive":             "der Archivverwaltung",
		"address_book":        "dem Adressbuch",
		"bluetooth":           "der Funkgeräteverwaltung",
		"cheatsheet":          "der Merkblattsammlung",
		"chromecast":          "der Medienwiedergabe",
		"docker":              "der Containerverwaltung",
		"evomap":              "dem Wissensaustausch",
		"firewall":            "dem Paketfilter",
		"manage_appointments": "der Terminverwaltung",
		"manage_memory":       "dem dauerhaften Speicher",
		"manage_todos":        "der Aufgabenliste",
		"filesystem":          "den Arbeitsdateien",
		"execute_shell":       "der Shell-Umgebung",
		"execute_python":      "der Python-Umgebung",
		"frigate":             "dem Kamera-NVR",
		"github":              "der Git-Hosting-Plattform",
		"go2rtc":              "dem Kamerastream-Proxy",
		"grafana":             "dem Monitoring-Dashboard",
		"home_assistant":      "Home Assistant",
		"huggingface":         "dem Modell-Hub",
		"jellyfin":            "dem Medienserver",
		"knowledge_graph":     "dem Wissensgraphen",
		"koofr":               "dem Cloud-Speicher",
		"ldap":                "dem Verzeichnisdienst",
		"manus":               "der Aufgabenautomatisierung",
		"meshcentral":         "der Gerätefernverwaltung",
		"netlify":             "dem Website-Hostingdienst",
		"network_shares":      "den Netzwerkfreigaben",
		"obsidian":            "dem Notizsystem",
		"ollama":              "der lokalen Modellverwaltung",
		"onedrive":            "dem Cloud-Dateispeicher",
		"proxmox":             "der Virtualisierungsplattform",
		"remember":            "dem Gesprächsgedächtnis",
		"sip_phone":           "der SIP-Telefonie",
		"tailscale":           "dem Mesh-VPN",
		"three_d_printer":     "dem 3D-Drucker",
		"truenas":             "der NAS-Verwaltung",
		"tts":                 "der Sprachausgabe",
		"vercel":              "der Deployment-Plattform",
		"webdav":              "dem Remote-Dateispeicher",
	}
	labelsEN := map[string]string{
		"adguard":     "the DNS filter",
		"ansible":     "configuration automation",
		"archive":     "archive management",
		"bluetooth":   "wireless device management",
		"cheatsheet":  "the reference-note collection",
		"chromecast":  "media playback",
		"docker":      "container management",
		"evomap":      "the knowledge exchange",
		"filesystem":  "the workspace files",
		"firewall":    "the packet filter",
		"frigate":     "the camera NVR",
		"github":      "the Git hosting platform",
		"go2rtc":      "the camera stream proxy",
		"grafana":     "the monitoring dashboard",
		"huggingface": "the model hub",
		"jellyfin":    "the media server",
		"koofr":       "cloud storage",
		"ldap":        "the directory service",
		"manus":       "task automation",
		"meshcentral": "remote device management",
		"netlify":     "the website hosting service",
		"obsidian":    "the note system",
		"ollama":      "local model management",
		"onedrive":    "cloud file storage",
		"proxmox":     "the virtualization platform",
		"remember":    "conversation memory",
		"tailscale":   "the mesh VPN",
		"truenas":     "NAS management",
		"tts":         "speech output",
		"vercel":      "the deployment platform",
		"webdav":      "remote file storage",
	}
	if language == "de" {
		if label := labelsDE[name]; label != "" {
			return label
		}
		return "der Integration „" + strings.ReplaceAll(name, "_", " ") + "“"
	}
	if label := labelsEN[name]; label != "" {
		return label
	}
	return strings.ReplaceAll(name, "_", " ")
}

func argumentSummaryExcluding(arguments map[string]interface{}, excluded ...string) string {
	if len(arguments) == 0 {
		return ""
	}
	excludedSet := make(map[string]bool, len(excluded))
	for _, key := range excluded {
		excludedSet[key] = true
	}
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		if strings.HasPrefix(key, "_") || excludedSet[key] {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		raw, _ := json.Marshal(arguments[key])
		parts = append(parts, fmt.Sprintf("%s=%s", key, raw))
	}
	return strings.Join(parts, ", ")
}

func successResult(tool, language string, sequence int) string {
	summariesDE := []string{"Aktion abgeschlossen", "Anfrage erfolgreich verarbeitet", "Ergebnis verfügbar"}
	summariesEN := []string{"Action completed", "Request processed successfully", "Result available"}
	summary := summariesDE[sequence%len(summariesDE)]
	if language == "en" {
		summary = summariesEN[sequence%len(summariesEN)]
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"tool":    tool,
		"summary": summary,
	})
	return string(raw)
}

func finalSuccess(tool ToolExport, language string, sequence int) string {
	de := []string{
		"Die angeforderte Aktion wurde erfolgreich abgeschlossen.",
		"Erledigt; das Tool-Ergebnis bestätigt den Erfolg.",
		"Die Aufgabe ist abgeschlossen und das Ergebnis liegt vor.",
	}
	en := []string{
		"The requested action completed successfully.",
		"Done; the tool result confirms success.",
		"The task is complete and the result is available.",
	}
	if language == "en" {
		return en[sequence%len(en)]
	}
	_ = tool
	return de[sequence%len(de)]
}

func selectToolContext(all []ToolExport, required []ToolExport, desired int, seed string) []ToolDefinition {
	return selectToolContextExcluding(all, required, "", desired, seed)
}

func selectToolContextExcluding(
	all []ToolExport,
	required []ToolExport,
	exclude string,
	desired int,
	seed string,
) []ToolDefinition {
	if desired > maxToolsPerScenario {
		desired = maxToolsPerScenario
	}
	requiredNames := make([]string, 0, len(required))
	for _, tool := range required {
		if tool.Name != exclude {
			requiredNames = append(requiredNames, tool.Name)
		}
	}
	sort.Strings(requiredNames)
	cacheKey := strings.Join(requiredNames, ",") + "|" + exclude + fmt.Sprintf("|%d", desired)
	values, cached := toolContextCache[cacheKey]
	if !cached {
		values = computeToolContext(all, required, exclude, desired)
		toolContextCache[cacheKey] = values
	}
	values = append([]ToolExport(nil), values...)
	sort.SliceStable(values, func(i, j int) bool {
		left := sha256.Sum256([]byte(seed + ":" + values[i].Name))
		right := sha256.Sum256([]byte(seed + ":" + values[j].Name))
		return string(left[:]) < string(right[:])
	})
	out := make([]ToolDefinition, 0, len(values))
	for _, tool := range values {
		out = append(out, tool.Definition())
	}
	return out
}

var (
	toolContextCache     = map[string][]ToolExport{}
	toolSchemaTokenCache = map[string]int{}
)

func computeToolContext(
	all []ToolExport,
	required []ToolExport,
	exclude string,
	desired int,
) []ToolExport {
	selected := make(map[string]ToolExport)
	for _, tool := range required {
		if tool.Name != exclude {
			selected[tool.Name] = tool
		}
	}
	reference := ""
	if len(required) > 0 {
		reference = required[0].Name + " " + required[0].Description
	}
	candidates := make([]ToolExport, 0, len(all))
	for _, tool := range all {
		if tool.Name == exclude {
			continue
		}
		if _, exists := selected[tool.Name]; exists {
			continue
		}
		candidates = append(candidates, tool)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := similarity(reference, candidates[i].Name+" "+candidates[i].Description)
		right := similarity(reference, candidates[j].Name+" "+candidates[j].Description)
		if left != right {
			return left > right
		}
		return candidates[i].Name < candidates[j].Name
	})
	for _, candidate := range candidates {
		if len(selected) >= desired {
			break
		}
		selected[candidate.Name] = candidate
		if approximateSchemaTokens(selected) > maxSchemaTokens {
			delete(selected, candidate.Name)
		}
	}
	values := make([]ToolExport, 0, len(selected))
	for _, tool := range selected {
		values = append(values, tool)
	}
	sort.SliceStable(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	return values
}

func approximateSchemaTokens(tools map[string]ToolExport) int {
	total := 0
	for _, tool := range tools {
		tokens, ok := toolSchemaTokenCache[tool.Name]
		if !ok {
			raw, _ := json.Marshal(tool.Definition())
			tokens = (len(raw) + 3) / 4
			toolSchemaTokenCache[tool.Name] = tokens
		}
		total += tokens
	}
	return total
}

func similarity(left, right string) int {
	leftTokens := tokenSet(left)
	rightTokens := tokenSet(right)
	score := 0
	for token := range leftTokens {
		if rightTokens[token] {
			score++
		}
	}
	return score
}

func tokenSet(value string) map[string]bool {
	replacer := strings.NewReplacer("_", " ", "-", " ", "/", " ", ".", " ", ",", " ", ":", " ")
	tokens := make(map[string]bool)
	for _, token := range strings.Fields(strings.ToLower(replacer.Replace(value))) {
		if len(token) >= 3 {
			tokens[token] = true
		}
	}
	return tokens
}

func highestTier(left, right string) string {
	rank := map[string]int{"rare": 1, "extended": 2, "core": 3}
	if rank[left] >= rank[right] {
		return left
	}
	return right
}

func scoper(tools []ToolExport) map[string]ToolExport {
	out := make(map[string]ToolExport, len(tools))
	for _, tool := range tools {
		out[tool.Name] = tool
	}
	return out
}
