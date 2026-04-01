package tools

import "encoding/json"

func marshalToolJSON(payload map[string]interface{}) string {
	data, _ := json.Marshal(payload)
	return string(data)
}

func marshalPrefixedToolJSON(payload map[string]interface{}) string {
	return "Tool Output: " + marshalToolJSON(payload)
}

func jsonRawOrString(data []byte) interface{} {
	if len(data) == 0 {
		return nil
	}
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err == nil {
		return parsed
	}
	return string(data)
}
