package tools

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

	return nil
}
