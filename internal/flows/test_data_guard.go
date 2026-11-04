package flows

import (
	"encoding/json"
	"fmt"
)

// rejectRedactedTestData treats exact server-produced markers as incomplete input.
func rejectRedactedTestData(data map[string]any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode test data: %w", err)
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return fmt.Errorf("decode test data: %w", err)
	}
	stack := []any{value}
	for len(stack) != 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch x := v.(type) {
		case string:
			if x == redactedText {
				return ErrTestDataRedacted
			}
		case []any:
			stack = append(stack, x...)
		case map[string]any:
			preview, hasPreview := x["_preview"].(string)
			if hasPreview && (len(x) == 1 || len(x) == 2 && (preview == "" && x["_omitted"] == storedOmittedReason || x["_truncated"] == true)) {
				return ErrTestDataRedacted
			}
			for key, item := range x {
				if key == redactedText {
					return ErrTestDataRedacted
				}
				stack = append(stack, item)
			}
		}
	}
	return nil
}
