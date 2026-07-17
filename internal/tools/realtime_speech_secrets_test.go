package tools

import "testing"

func TestRealtimeSpeechSecretsAreBlockedFromPython(t *testing.T) {
	for _, key := range []string{
		"realtime_speech_profile_main_api_key",
		"REALTIME_SPEECH_PROFILE_MAIN_API_KEY",
	} {
		if IsPythonAccessibleSecret(key) {
			t.Fatalf("realtime speech secret %q was exposed to Python", key)
		}
	}
}
