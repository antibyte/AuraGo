package tools

// stringArgWithFallback extracts a string value from args, falling back to "query"
// if the primary key is empty. This makes tool calls more robust when the LLM
// uses "query" as a universal parameter even though the schema expects a
// specific key (e.g. "username", "url", "asin", etc.).
func stringArgWithFallback(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	if v, ok := args["query"].(string); ok && v != "" {
		return v
	}
	return ""
}

// stringSliceFromArgs extracts a string slice from map arguments.
// Handles both []interface{} (from JSON unmarshalling) and []string.
func stringSliceFromArgs(args map[string]interface{}, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}

	if ss, ok := raw.([]string); ok {
		return ss
	}

	if arr, ok := raw.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	// Fallback: if the value is a single string, wrap it in a slice
	if s, ok := raw.(string); ok && s != "" {
		return []string{s}
	}

	return nil
}

func addPositiveIntArg(payload map[string]interface{}, args map[string]interface{}, argKey, payloadKey string) {
	if n, ok := args[argKey].(float64); ok && n > 0 {
		payload[payloadKey] = int(n)
	}
}

func addOptionalStringArg(payload map[string]interface{}, args map[string]interface{}, argKey, payloadKey string) {
	if v, ok := args[argKey].(string); ok && v != "" {
		payload[payloadKey] = v
	}
}

func addCountryArg(payload map[string]interface{}, args map[string]interface{}) {
	if country, ok := args["country"].(string); ok && country != "" {
		payload["country"] = country
		return
	}
	payload["country"] = "US"
}
