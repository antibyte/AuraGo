package main

import "encoding/json"

// strictArguments projects sparse curated inputs onto the wire contract. The
// original required list prevents null from standing in for a missing value.
func strictArguments(tool ToolExport, arguments map[string]interface{}) map[string]interface{} {
	args := cloneMap(arguments)
	completeStrictArguments(args, tool.ArgumentSchema)
	return args
}

func completeStrictArguments(value interface{}, original map[string]interface{}) {
	switch value := value.(type) {
	case map[string]interface{}:
		properties, _ := original["properties"].(map[string]interface{})
		required := make(map[string]bool)
		for _, name := range stringSlice(original["required"]) {
			required[name] = true
		}
		for name, raw := range properties {
			child, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if arg, present := value[name]; present {
				completeStrictArguments(arg, child)
			} else if !required[name] {
				value[name] = nil
			}
		}
	case []interface{}:
		items, _ := original["items"].(map[string]interface{})
		for _, item := range value {
			completeStrictArguments(item, items)
		}
	}
}

// Project only model calls and their expected wire arguments. Human tasks,
// operation selectors and required/excluded rules keep the curated semantics.
func completeScenarioArguments(scenarios []Scenario, tools map[string]ToolExport) error {
	for i := range scenarios {
		for j := range scenarios[i].Messages {
			for k := range scenarios[i].Messages[j].ToolCalls {
				call := &scenarios[i].Messages[j].ToolCalls[k]
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
					return err
				}
				raw, err := json.Marshal(strictArguments(tools[call.Function.Name], args))
				if err != nil {
					return err
				}
				call.Function.Arguments = string(raw)
			}
		}
		for j := range scenarios[i].Expectations.Calls {
			call := &scenarios[i].Expectations.Calls[j]
			call.Arguments = strictArguments(tools[call.Name], call.Arguments)
		}
	}
	return nil
}
