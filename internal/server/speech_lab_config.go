package server

// removeDeprecatedSpeechLabVoice keeps the legacy YAML key load-compatible
// while preventing it from being exposed or persisted by the configuration UI.
func removeDeprecatedSpeechLabVoice(raw map[string]interface{}) {
	section, ok := raw["speech_lab"].(map[string]interface{})
	if !ok {
		return
	}
	delete(section, "voice")
}
