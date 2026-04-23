package server

import "fmt"

// normalizeConfigYAMLValue converts YAML-decoded legacy map shapes into a
// structure that can always be marshaled back to YAML after JSON patch merges.
// Older or hand-edited configs may contain nested map[interface{}]interface{}
// values or non-scalar keys that yaml.Unmarshal accepts, but yaml.Marshal
// rejects once the setup/config UI touches the document.
func normalizeConfigYAMLValue(v interface{}) interface{} {
	switch typed := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = normalizeConfigYAMLValue(value)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[fmt.Sprint(normalizeConfigYAMLValue(key))] = normalizeConfigYAMLValue(value)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = normalizeConfigYAMLValue(value)
		}
		return out
	default:
		return v
	}
}

func normalizeConfigYAMLMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	normalized, ok := normalizeConfigYAMLValue(m).(map[string]interface{})
	if !ok {
		return m
	}
	return normalized
}
