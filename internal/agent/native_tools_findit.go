package agent

import openai "github.com/sashabaranov/go-openai"

func appendFindItToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if !ff.FindItEnabled {
		return tools
	}
	filter := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"field":    map[string]interface{}{"type": "string", "pattern": "^[a-z][a-z0-9_]{0,63}$", "description": "Property key from the selected FindIt template."},
			"operator": map[string]interface{}{"type": "string", "enum": []string{"eq", "ne", "contains", "gt", "gte", "lt", "lte"}, "description": "Comparison supported by the property type."},
			"value": map[string]interface{}{
				"anyOf":       []interface{}{map[string]interface{}{"type": "string"}, map[string]interface{}{"type": "number"}, map[string]interface{}{"type": "boolean"}},
				"description": "Typed scalar value to compare against the property.",
			},
		},
		"required":             []string{"field", "operator", "value"},
		"additionalProperties": false,
	}
	return append(tools, tool(
		"findit",
		"Read the physical homelab inventory. Search or get items, locations, projects, and templates; inspect item availability including reserved and free quantities; or list project shortages. Search templates first to discover custom property fields before filtering items. This tool never changes inventory.",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"search", "get", "availability", "shortages"},
				"description": "Read operation: search records, get one by id, calculate item availability, or list a project's BOM shortages.",
			},
			"entity": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"items", "locations", "projects", "templates"},
				"description": "Record type. availability requires items; shortages requires projects.",
			},
			"id":             map[string]interface{}{"type": "string", "maxLength": 128, "pattern": "^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$", "description": "Record id for get and availability; project id for shortages."},
			"query":          map[string]interface{}{"type": "string", "maxLength": 512, "description": "Words or exact identifiers to search."},
			"location_id":    prop("string", "Limit item search to a location."),
			"template_id":    prop("string", "Limit item search to a template. Search templates to discover fields."),
			"category":       prop("string", "Limit item search to a catalog category."),
			"package":        prop("string", "Limit item search to a package family, such as SOIC or 0603."),
			"available_only": prop("boolean", "For item search, keep records with free stock."),
			"low_stock":      prop("boolean", "For item search, keep records below minimum stock."),
			"filters": map[string]interface{}{
				"type":        "array",
				"description": "Typed filters for template properties. Use a template's key and type returned by search templates.",
				"maxItems":    8,
				"items":       filter,
			},
			"limit":  map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 200, "description": "Maximum number of search results (default 50, maximum 200)."},
			"offset": map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 1000000, "description": "Search pagination offset."},
		}, "operation", "entity"),
	))
}
