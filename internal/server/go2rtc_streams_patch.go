package server

import (
	"fmt"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// validateGo2RTCStreamsPatch rejects a go2rtc.streams value that is not a
// complete stream list. A config save replaces the list wholesale
// (mergeConfigArrayValue), so a sparse or partial list would delete every
// stream it leaves out. An explicit empty list still removes all streams.
func validateGo2RTCStreamsPatch(patch map[string]interface{}) error {
	section, ok := patch["go2rtc"].(map[string]interface{})
	if !ok {
		return nil
	}
	raw, present := section["streams"]
	if !present {
		return nil
	}
	var items []interface{}
	switch streams := raw.(type) {
	case []interface{}:
		items = streams
	case map[string]interface{}:
		converted, dense := numericKeyedMapToSlice(streams)
		if !dense {
			return fmt.Errorf("go2rtc.streams must be a list of streams")
		}
		items = converted
	default:
		return fmt.Errorf("go2rtc.streams must be a list of streams")
	}
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		stream, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("go2rtc.streams must list every stream: entry %d is empty", index+1)
		}
		id, ok := stream["id"].(string)
		if !ok || strings.TrimSpace(id) == "" {
			return fmt.Errorf("go2rtc.streams must list every stream: entry %d has no id", index+1)
		}
		if err := tools.ValidateGo2RTCStreamID(id); err != nil {
			return fmt.Errorf("go2rtc.streams entry %d: %w", index+1, err)
		}
		key := config.Go2RTCStreamSourceVaultKey(id)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("go2rtc.streams: stream id %q is duplicated", strings.TrimSpace(id))
		}
		seen[key] = struct{}{}
	}
	return nil
}
