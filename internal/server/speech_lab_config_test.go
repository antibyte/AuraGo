package server

import "testing"

func TestRemoveDeprecatedSpeechLabVoice(t *testing.T) {
	raw := map[string]interface{}{
		"speech_lab": map[string]interface{}{
			"voice":                "M1",
			"chat_llm_provider_id": "fast",
		},
	}
	removeDeprecatedSpeechLabVoice(raw)
	section := raw["speech_lab"].(map[string]interface{})
	if _, exists := section["voice"]; exists {
		t.Fatal("legacy voice remained in the configuration payload")
	}
	if section["chat_llm_provider_id"] != "fast" {
		t.Fatal("canonical Speech Lab settings were changed")
	}
}
