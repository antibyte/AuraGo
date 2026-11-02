package agent

import (
	"aurago/internal/ecalc"

	openai "github.com/sashabaranov/go-openai"
)

// ecalcSchema describes the ecalc tool: electronics calculators from the embedded ECalc catalog. It is always
// present (pure computation, no switch); only the open operation needs the Virtual Desktop. The schema stays small
// because the manual (prompts/tools_manuals/ecalc.md) carries the details. The variable values are called inputs:
// ToolCall already owns a "values" field for spreadsheets that only decodes arrays of arrays, so an array of
// objects under that name would mark every call malformed.
func ecalcSchema() openai.Tool {
	pairs := func(description, valueKey, valueHelp string) map[string]interface{} {
		return map[string]interface{}{
			"type":        "array",
			"description": description,
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":   prop("string", "Id from describe."),
					valueKey: prop("string", valueHelp),
				},
				"required":             []string{"name", valueKey},
				"additionalProperties": false,
			},
		}
	}
	return tool("ecalc",
		"Electronics calculators from the ECalc catalog (Ohm's law, dividers, filters, op-amps, E-series): search them, describe one, calculate with units, or open one pre-filled in the ECalc app.",
		schema(map[string]interface{}{
			"operation": operationProperty("search finds calculators, describe lists variables and solvable inputs, calculate solves, open shows one in the ECalc desktop app.", []string{"search", "describe", "calculate", "open"}),
			"query":     prop("string", "search: topic, part or formula, English or German. Empty lists the categories."),
			"id":        prop("string", "describe, calculate, open: calculator id from search."),
			"inputs":    pairs("calculate, open: known values as {name, value}, for example 4k7, 12 V or 25 °C. A bare number is in the variable's default unit.", "value", "Value as typed."),
			"choices":   pairs("calculate, open: choice selections as {name, option}.", "option", "Option id from describe."),
			"eseries": map[string]interface{}{
				"type":        "string",
				"description": "calculate: E-series for standard-value suggestions (default E24).",
				"enum":        append([]string(nil), ecalc.ESeriesIDs...),
			},
			"limit": prop("integer", "search: 1-25, default 10."),
		}, "operation"),
	)
}
