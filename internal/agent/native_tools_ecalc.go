package agent

import (
	"aurago/internal/ecalc"

	openai "github.com/sashabaranov/go-openai"
)

// ecalcSchema describes the ecalc tool: electronics calculators, reference tables, component codes and connector pinouts
// from the embedded ECalc catalog. It is always present (pure computation, no switch); only the open operation needs the Virtual
// Desktop. The schema stays small because the manual (prompts/tools_manuals/ecalc.md) carries the details. The variable
// values are called inputs: ToolCall already owns a "values" field for spreadsheets that only decodes arrays of arrays,
// so an array of objects under that name would mark every call malformed. The other new parameter names (code, value,
// offset, limit) are ToolCall fields of the same type; filter, lookup, kind, bands, tolerance, tempco, voltage, class, pin
// and variant are not ToolCall fields at all.
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
		"Electronics calculators, tables, component codes and pinouts: search, describe, calculate, read tables or pinouts, decode/encode codes, or open in the ECalc app.",
		schema(map[string]interface{}{
			"operation": operationProperty("search finds items; describe lists variables, columns, code forms or variants; table reads rows or one lookup; pinout reads pins; decode/encode convert codes; open shows it in the app.", []string{"search", "describe", "calculate", "open", "table", "pinout", "decode", "encode"}),
			"query":     prop("string", "search: topic, part or formula, English or German. Empty lists the categories."),
			"id":        prop("string", "describe, calculate, table, pinout, open: id from search."),
			"inputs":    pairs("calculate, open: known values as {name, value}, for example 4k7, 12 V or 25 °C. A bare number is in the variable's default unit.", "value", "Value as typed."),
			"choices":   pairs("calculate, open: choice selections as {name, option}.", "option", "Option id from describe."),
			"eseries": map[string]interface{}{
				"type":        "string",
				"description": "calculate: E-series for standard-value suggestions (default E24).",
				"enum":        append([]string(nil), ecalc.ESeriesIDs...),
			},
			"limit":   prop("integer", "search 1-25 (10), table 1-50 (20), pinout 1-50 (50)."),
			"offset":  prop("integer", "table, pinout: first row."),
			"filter":  prop("string", "table, pinout: rows containing this text."),
			"pin":     prop("string", "pinout: pin number or name."),
			"variant": prop("string", "pinout: variant id."),
			"lookup": map[string]interface{}{
				"type":        "object",
				"description": "table: exact finds the row, ge the row with the smallest cell >= value, interp interpolates output.",
				"properties": map[string]interface{}{
					"column": prop("string", "Column id."),
					"value":  prop("string", "As typed; a bare number is in the column's unit."),
					"mode":   map[string]interface{}{"type": "string", "enum": []string{"exact", "ge", "interp"}},
					"output": prop("string", "Column id to return."),
				},
				"required":             []string{"value"},
				"additionalProperties": false,
			},
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "decode, encode: code kind.",
				"enum":        append([]string(nil), ecalc.CodeKindIDs()...),
			},
			"code":      prop("string", "decode: code as printed or band colours."),
			"value":     prop("string", "encode: value, e.g. 4k7 or 100n."),
			"bands":     prop("integer", "encode: number of bands."),
			"tolerance": prop("string", "encode: e.g. 5% or K."),
			"tempco":    prop("string", "encode: ppm/K."),
			"voltage":   prop("string", "encode: rated V."),
			"class":     prop("string", "encode: ceramic class 1 or 2."),
		}, "operation"),
	)
}
