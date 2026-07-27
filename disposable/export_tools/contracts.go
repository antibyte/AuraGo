package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func buildDefaultContracts(root string, tools []ToolExport) (OperationContractManifest, error) {
	digest, err := schemaDigest(tools)
	if err != nil {
		return OperationContractManifest{}, fmt.Errorf("hash native schemas: %w", err)
	}
	manifest := OperationContractManifest{
		Version:      contractVersion,
		SchemaSHA256: digest,
		Tools:        make(map[string]ToolContract, len(tools)),
	}
	for _, tool := range tools {
		manualRequirements, err := manualOperationRequirements(root, tool)
		if err != nil {
			return OperationContractManifest{}, err
		}
		defaultArgs, err := fixtureFromSchema(tool, "", nil)
		if err != nil {
			return OperationContractManifest{}, err
		}
		contract := ToolContract{DefaultArguments: defaultArgs}
		for _, operation := range extractOperations(tool.Parameters) {
			arguments, fixtureErr := fixtureFromSchema(tool, operation.Selector, operation.Value)
			if fixtureErr != nil {
				return OperationContractManifest{}, fixtureErr
			}
			requiredFields := operationRequiredFields(tool, operation, arguments)
			requiredFields = mergeFieldNames(requiredFields, manualRequirements[fmt.Sprint(operation.Value)])
			requiredFields = mergeFieldNames(requiredFields, conventionalOperationFields(tool, operation))
			for _, field := range requiredFields {
				if _, exists := arguments[field]; exists {
					continue
				}
				property, _ := tool.Properties[field].(map[string]interface{})
				arguments[field] = schemaExampleValue(field, property, 0)
			}
			if err := validateArguments(tool, arguments); err != nil {
				return OperationContractManifest{}, fmt.Errorf(
					"cannot bootstrap operation contract for %s %s=%v: %w",
					tool.Name,
					operation.Selector,
					operation.Value,
					err,
				)
			}
			contract.Operations = append(contract.Operations, OperationFixture{
				Selector:       operation.Selector,
				Value:          operation.Value,
				Arguments:      arguments,
				RequiredFields: requiredFields,
				ExcludedFields: operationExcludedFields(tool, operation),
			})
		}
		manifest.Tools[tool.Name] = contract
	}
	return manifest, nil
}

func operationExcludedFields(tool ToolExport, current OperationRef) []string {
	operations := extractOperations(tool.Parameters)
	excluded := make([]string, 0)
	for propertyName, raw := range tool.Properties {
		property, _ := raw.(map[string]interface{})
		description := strings.ToLower(fmt.Sprint(property["description"]))
		restrictionAt := -1
		for _, phrase := range []string{"only for", "only used for", "only valid for", "only allowed for"} {
			if index := strings.Index(description, phrase); index >= 0 &&
				(restrictionAt < 0 || index < restrictionAt) {
				restrictionAt = index
			}
		}
		if restrictionAt < 0 {
			continue
		}
		end := restrictionAt + 180
		if end > len(description) {
			end = len(description)
		}
		window := description[restrictionAt:end]
		mentionsOperation := false
		allowsCurrent := false
		for _, operation := range operations {
			name := strings.ToLower(fmt.Sprint(operation.Value))
			if operationMentioned(window, name) ||
				operationMentioned(window, strings.ReplaceAll(name, "_", " ")) {
				mentionsOperation = true
				if fmt.Sprint(operation.Value) == fmt.Sprint(current.Value) {
					allowsCurrent = true
				}
			}
		}
		if mentionsOperation && !allowsCurrent {
			excluded = append(excluded, propertyName)
		}
	}
	sort.Strings(excluded)
	return excluded
}

func conventionalOperationFields(tool ToolExport, operation OperationRef) []string {
	name := strings.ToLower(fmt.Sprint(operation.Value))
	if hasOperationPrefix(name, "list", "create", "add", "new", "search") {
		return nil
	}
	operationWords := tokenSet(name)
	toolWords := tokenSet(tool.Name)
	fields := make([]string, 0)
	for propertyName := range tool.Properties {
		if !strings.HasSuffix(propertyName, "_id") {
			continue
		}
		resource := strings.TrimSuffix(strings.ToLower(propertyName), "_id")
		singular := strings.TrimSuffix(resource, "s")
		if strings.HasPrefix(name, "send_") && singular == "message" {
			continue
		}
		if operationWords[resource] || operationWords[singular] ||
			toolWords[resource] || toolWords[singular] || toolWords[singular+"s"] {
			fields = append(fields, propertyName)
		}
	}
	return fields
}

func manualOperationRequirements(root string, tool ToolExport) (map[string][]string, error) {
	requirements := make(map[string][]string)
	if tool.ManualPath == "" {
		return requirements, nil
	}
	path := filepath.Join(root, filepath.FromSlash(tool.ManualPath))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manual %s: %w", tool.ManualPath, err)
	}
	operations := extractOperations(tool.Parameters)
	knownOperations := make(map[string]bool, len(operations))
	for _, operation := range operations {
		knownOperations[fmt.Sprint(operation.Value)] = true
	}
	var parameterColumn, requiredColumn int
	inRequiredTable := false
	currentOperation := ""
	headingPattern := regexp.MustCompile(`^#{2,6}\s+` + "`?" + `([A-Za-z0-9_-]+)` + "`?")
	requiredBulletPattern := regexp.MustCompile(`(?i)^-\s+\*\*([A-Za-z0-9_]+)\*\*\s*\(required\)`)
	for _, line := range strings.Split(string(data), "\n") {
		trimmedLine := strings.TrimSpace(line)
		if match := headingPattern.FindStringSubmatch(trimmedLine); len(match) == 2 {
			if knownOperations[match[1]] {
				currentOperation = match[1]
			} else {
				currentOperation = ""
			}
		}
		if currentOperation != "" {
			if match := requiredBulletPattern.FindStringSubmatch(trimmedLine); len(match) == 2 {
				if _, known := tool.Properties[match[1]]; known {
					requirements[currentOperation] = mergeFieldNames(
						requirements[currentOperation],
						[]string{match[1]},
					)
				}
			}
		}
		cells := markdownTableCells(line)
		if len(cells) == 0 {
			inRequiredTable = false
			continue
		}
		if parameterIndex, requiredIndex, ok := requiredTableHeader(cells); ok {
			parameterColumn = parameterIndex
			requiredColumn = requiredIndex
			inRequiredTable = true
			continue
		}
		if !inRequiredTable || parameterColumn >= len(cells) || requiredColumn >= len(cells) {
			continue
		}
		field := strings.Trim(strings.TrimSpace(cells[parameterColumn]), "`* ")
		if field == "" || strings.Trim(field, "-: ") == "" {
			continue
		}
		if _, known := tool.Properties[field]; !known {
			continue
		}
		requirement := strings.Trim(strings.TrimSpace(cells[requiredColumn]), "`* ")
		for _, operation := range operations {
			operationName := fmt.Sprint(operation.Value)
			if manualRequirementApplies(requirement, operationName) {
				requirements[operationName] = mergeFieldNames(requirements[operationName], []string{field})
			}
		}
	}
	return requirements, nil
}

func markdownTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return nil
	}
	parts := strings.Split(strings.Trim(trimmed, "|"), "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func requiredTableHeader(cells []string) (int, int, bool) {
	parameterIndex := -1
	requiredIndex := -1
	for index, cell := range cells {
		normalized := strings.ToLower(strings.Trim(strings.TrimSpace(cell), "`* "))
		switch {
		case normalized == "parameter" || normalized == "field":
			parameterIndex = index
		case strings.Contains(normalized, "required"):
			requiredIndex = index
		}
	}
	return parameterIndex, requiredIndex, parameterIndex >= 0 && requiredIndex >= 0
}

func manualRequirementApplies(requirement, operation string) bool {
	normalized := strings.ToLower(strings.TrimSpace(requirement))
	switch normalized {
	case "", "-", "—", "no", "none", "optional":
		return false
	case "yes", "required", "always":
		return true
	}
	if optionalAt := strings.Index(normalized, "optional"); optionalAt >= 0 {
		normalized = strings.TrimSpace(strings.TrimRight(normalized[:optionalAt], ";, "))
	}
	if operationMentioned(normalized, strings.ToLower(operation)) {
		return true
	}
	return strings.Contains(normalized, "item operation") && strings.Contains(strings.ToLower(operation), "item")
}

func mergeFieldNames(groups ...[]string) []string {
	seen := make(map[string]bool)
	for _, group := range groups {
		for _, field := range group {
			if field != "" {
				seen[field] = true
			}
		}
	}
	fields := make([]string, 0, len(seen))
	for field := range seen {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func validateContracts(tools []ToolExport, manifest OperationContractManifest) (int, error) {
	if manifest.Version != contractVersion {
		return 0, fmt.Errorf("unsupported operation contract version %d", manifest.Version)
	}
	digest, err := schemaDigest(tools)
	if err != nil {
		return 0, err
	}
	if manifest.SchemaSHA256 != digest {
		return 0, fmt.Errorf(
			"operation_contracts.json targets schema %s, current schema is %s; run --bootstrap-contracts and review the diff",
			manifest.SchemaSHA256,
			digest,
		)
	}
	known := make(map[string]ToolExport, len(tools))
	for _, tool := range tools {
		known[tool.Name] = tool
	}
	for name := range manifest.Tools {
		if _, ok := known[name]; !ok {
			return 0, fmt.Errorf("operation_contracts.json references unknown tool %s", name)
		}
	}
	total := 0
	for _, tool := range tools {
		contract, ok := manifest.Tools[tool.Name]
		if !ok {
			return 0, fmt.Errorf("tool %s is missing from operation_contracts.json", tool.Name)
		}
		if err := validateArguments(tool, contract.DefaultArguments); err != nil {
			return 0, fmt.Errorf("default fixture for %s is invalid: %w", tool.Name, err)
		}
		expected := extractOperations(tool.Parameters)
		if len(contract.Operations) != len(expected) {
			return 0, fmt.Errorf(
				"tool %s has %d operation fixtures, expected %d",
				tool.Name,
				len(contract.Operations),
				len(expected),
			)
		}
		for i, fixture := range contract.Operations {
			want := expected[i]
			if fixture.Selector != want.Selector || fmt.Sprint(fixture.Value) != fmt.Sprint(want.Value) {
				return 0, fmt.Errorf(
					"tool %s operation fixture %d is %s=%v, expected %s=%v",
					tool.Name,
					i,
					fixture.Selector,
					fixture.Value,
					want.Selector,
					want.Value,
				)
			}
			if got, ok := fixture.Arguments[fixture.Selector]; !ok || fmt.Sprint(got) != fmt.Sprint(fixture.Value) {
				return 0, fmt.Errorf(
					"tool %s fixture for %s=%v does not contain the selector value",
					tool.Name,
					fixture.Selector,
					fixture.Value,
				)
			}
			if err := validateArguments(tool, fixture.Arguments); err != nil {
				return 0, fmt.Errorf(
					"operation fixture for %s %s=%v is invalid: %w",
					tool.Name,
					fixture.Selector,
					fixture.Value,
					err,
				)
			}
			if err := validateOperationFields(tool, fixture); err != nil {
				return 0, fmt.Errorf(
					"operation contract for %s %s=%v is invalid: %w",
					tool.Name,
					fixture.Selector,
					fixture.Value,
					err,
				)
			}
			total++
		}
	}
	return total, nil
}

func operationRequiredFields(
	tool ToolExport,
	operation OperationRef,
	arguments map[string]interface{},
) []string {
	required := make(map[string]bool, len(arguments))
	for name := range arguments {
		required[name] = true
	}
	operationNames := []string{
		strings.ToLower(fmt.Sprint(operation.Value)),
		strings.ReplaceAll(strings.ToLower(fmt.Sprint(operation.Value)), "_", " "),
	}
	for name, raw := range tool.Properties {
		property, _ := raw.(map[string]interface{})
		description := strings.ToLower(fmt.Sprint(property["description"]))
		requiredAt := strings.Index(description, "required")
		for requiredAt >= 0 {
			end := requiredAt + 180
			if end > len(description) {
				end = len(description)
			}
			window := description[requiredAt:end]
			for _, operationName := range operationNames {
				if operationMentioned(window, operationName) {
					required[name] = true
					break
				}
			}
			if strings.Contains(window, "item operation") &&
				strings.Contains(strings.ToLower(fmt.Sprint(operation.Value)), "item") {
				required[name] = true
			}
			next := strings.Index(description[requiredAt+len("required"):], "required")
			if next < 0 {
				break
			}
			requiredAt += len("required") + next
		}
	}
	fields := make([]string, 0, len(required))
	for name := range required {
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func operationMentioned(description, operation string) bool {
	if operation == "" {
		return false
	}
	pattern := `(^|[^a-z0-9_])` + regexp.QuoteMeta(operation) + `([^a-z0-9_]|$)`
	matched, err := regexp.MatchString(pattern, description)
	return err == nil && matched
}

func validateOperationFields(tool ToolExport, fixture OperationFixture) error {
	required := make(map[string]bool, len(fixture.RequiredFields))
	for _, field := range fixture.RequiredFields {
		if required[field] {
			return fmt.Errorf("required_fields contains duplicate %q", field)
		}
		required[field] = true
		if _, known := tool.Properties[field]; !known {
			return fmt.Errorf("required_fields contains unknown property %q", field)
		}
		if _, present := fixture.Arguments[field]; !present {
			return fmt.Errorf("required field %q is missing from arguments", field)
		}
	}
	excluded := make(map[string]bool, len(fixture.ExcludedFields))
	for _, field := range fixture.ExcludedFields {
		if excluded[field] {
			return fmt.Errorf("excluded_fields contains duplicate %q", field)
		}
		excluded[field] = true
		if _, known := tool.Properties[field]; !known {
			return fmt.Errorf("excluded_fields contains unknown property %q", field)
		}
		if required[field] {
			return fmt.Errorf("field %q is both required and excluded", field)
		}
		if _, present := fixture.Arguments[field]; present {
			return fmt.Errorf("excluded field %q is present in arguments", field)
		}
	}
	if !required[fixture.Selector] {
		return fmt.Errorf("selector %q must be listed in required_fields", fixture.Selector)
	}
	return nil
}

func fixtureFromSchema(tool ToolExport, selector string, operation interface{}) (map[string]interface{}, error) {
	args := make(map[string]interface{})
	required := make(map[string]bool, len(tool.Required))
	for _, name := range tool.Required {
		required[name] = true
	}
	if selector != "" {
		required[selector] = true
	}
	keys := make([]string, 0, len(required))
	for name := range required {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		property, _ := tool.Properties[name].(map[string]interface{})
		if name == selector {
			args[name] = operation
			continue
		}
		args[name] = schemaExampleValue(name, property, 0)
	}
	if len(args) == 0 && len(tool.Properties) > 0 {
		propertyNames := make([]string, 0, len(tool.Properties))
		for name := range tool.Properties {
			if strings.HasPrefix(name, "_") {
				continue
			}
			propertyNames = append(propertyNames, name)
		}
		sort.Strings(propertyNames)
		if len(propertyNames) > 0 {
			name := propertyNames[0]
			property, _ := tool.Properties[name].(map[string]interface{})
			args[name] = schemaExampleValue(name, property, 0)
		}
	}
	if err := validateArguments(tool, args); err != nil {
		return nil, fmt.Errorf(
			"cannot bootstrap valid fixture for %s %s=%v: %w",
			tool.Name,
			selector,
			operation,
			err,
		)
	}
	return args, nil
}

func schemaExampleValue(name string, schema map[string]interface{}, depth int) interface{} {
	if schema == nil || depth > 8 {
		return "example"
	}
	if value, ok := schema["const"]; ok {
		return value
	}
	if values := enumValues(schema["enum"]); len(values) > 0 {
		return values[0]
	}
	if value, ok := schema["default"]; ok {
		return value
	}
	if value, ok := schema["example"]; ok {
		return value
	}
	for _, combinator := range []string{"oneOf", "anyOf", "allOf"} {
		if branches, ok := schema[combinator].([]interface{}); ok && len(branches) > 0 {
			if branch, ok := branches[0].(map[string]interface{}); ok {
				return schemaExampleValue(name, branch, depth+1)
			}
		}
	}

	typ := schemaType(schema)
	switch typ {
	case "boolean":
		return false
	case "integer":
		return boundedNumber(schema, true)
	case "number":
		return boundedNumber(schema, false)
	case "array":
		items, _ := schema["items"].(map[string]interface{})
		minItems := int(numberValue(schema["minItems"], 0))
		if minItems <= 0 {
			return []interface{}{}
		}
		values := make([]interface{}, minItems)
		for i := range values {
			values[i] = schemaExampleValue(strings.TrimSuffix(name, "s"), items, depth+1)
		}
		return values
	case "object":
		properties, _ := schema["properties"].(map[string]interface{})
		out := make(map[string]interface{})
		required := stringSlice(schema["required"])
		sort.Strings(required)
		for _, childName := range required {
			child, _ := properties[childName].(map[string]interface{})
			out[childName] = schemaExampleValue(childName, child, depth+1)
		}
		return out
	case "null":
		return nil
	default:
		return stringExample(name, schema)
	}
}

func schemaType(schema map[string]interface{}) string {
	switch value := schema["type"].(type) {
	case string:
		return value
	case []interface{}:
		for _, candidate := range value {
			if text, ok := candidate.(string); ok && text != "null" {
				return text
			}
		}
	case []string:
		for _, candidate := range value {
			if candidate != "null" {
				return candidate
			}
		}
	}
	if _, ok := schema["properties"]; ok {
		return "object"
	}
	if _, ok := schema["items"]; ok {
		return "array"
	}
	return "string"
}

func boundedNumber(schema map[string]interface{}, integer bool) interface{} {
	minimum := numberValue(schema["minimum"], 1)
	if exclusive := numberValue(schema["exclusiveMinimum"], math.Inf(-1)); !math.IsInf(exclusive, -1) {
		minimum = exclusive + 1
	}
	maximum := numberValue(schema["maximum"], minimum)
	if minimum > maximum {
		minimum = maximum
	}
	if integer {
		return int64(math.Ceil(minimum))
	}
	return minimum
}

func numberValue(value interface{}, fallback float64) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	case json.Number:
		parsed, err := number.Float64()
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func stringExample(name string, schema map[string]interface{}) string {
	lower := strings.ToLower(name)
	var value string
	switch {
	case strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token"):
		value = "[redacted]"
	case lower == "status":
		value = "open"
	case lower == "priority":
		value = "medium"
	case lower == "title":
		value = "Example task"
	case strings.Contains(lower, "email"):
		value = "user@example.com"
	case lower == "url" || strings.HasSuffix(lower, "_url") || strings.Contains(lower, "endpoint"):
		value = "https://example.com/resource"
	case strings.Contains(lower, "cidr"):
		value = "192.0.2.0/24"
	case lower == "ip" || strings.HasSuffix(lower, "_ip"):
		value = "192.0.2.10"
	case strings.Contains(lower, "host"):
		value = "device.example.com"
	case strings.Contains(lower, "path") || strings.Contains(lower, "file") || strings.Contains(lower, "source"):
		value = "workspace/example.txt"
	case strings.Contains(lower, "destination") || lower == "dest":
		value = "workspace/output.txt"
	case strings.Contains(lower, "command"):
		value = "printf hello"
	case strings.Contains(lower, "code"):
		value = "print('hello')"
	case strings.Contains(lower, "query") || lower == "q" || strings.Contains(lower, "search"):
		value = "service status"
	case strings.Contains(lower, "prompt"):
		value = "A calm landscape at sunset"
	case strings.Contains(lower, "content") || strings.Contains(lower, "body") || strings.Contains(lower, "message") || lower == "text":
		value = "Example content"
	case strings.HasSuffix(lower, "_id") || lower == "id":
		value = "example-id"
	case strings.Contains(lower, "name"):
		value = "example-name"
	case strings.Contains(lower, "format"):
		value = "json"
	case strings.Contains(lower, "language"):
		value = "de"
	default:
		value = "example"
	}
	if pattern, ok := schema["pattern"].(string); ok && pattern != "" {
		if compiled, err := regexp.Compile(pattern); err == nil && !compiled.MatchString(value) {
			for _, candidate := range []string{"example", "example-id", "abc123", "1", "a", "https://example.com"} {
				if compiled.MatchString(candidate) {
					value = candidate
					break
				}
			}
		}
	}
	minLength := int(numberValue(schema["minLength"], 0))
	for len(value) < minLength {
		value += "x"
	}
	if maxLength := int(numberValue(schema["maxLength"], 0)); maxLength > 0 && len(value) > maxLength {
		value = value[:maxLength]
	}
	return value
}
