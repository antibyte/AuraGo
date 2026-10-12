package ui

import (
	"strings"
	"testing"
)

func TestGo2RTCConfigPointsToPersonDetection(t *testing.T) {
	module := string(mustReadUIFile(t, "cfg/go2rtc.js"))
	for _, required := range []string{
		`id="go2rtc-detection-hint"`,
		`t('config.go2rtc.detection_hint')`,
		`<a href="/desktop?app=network-cameras" target="_blank" rel="noopener">`,
	} {
		if !strings.Contains(module, required) {
			t.Fatalf("go2rtc config module missing %q", required)
		}
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		if strings.TrimSpace(readMergedLangMap(t, locale)["config.go2rtc.detection_hint"]) == "" {
			t.Fatalf("%s lacks config.go2rtc.detection_hint", locale)
		}
	}
}
