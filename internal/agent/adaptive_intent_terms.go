package agent

import "strings"

// Term tables of the intent helpers that add tools to the adaptive selection.
// Every table matches whole words (see toolIntentText); a stem covers the
// plural and the usual verb forms, and the spelling is folded ("ae", "oe",
// "ue", "ss").
var (
	resourceUsageTerms = newToolIntentTerms([]string{
		"ram", "cpu", "memory usage", "mem usage", "resource usage", "resources", "ressourcen",
		"wieviel speicher", "wie viel speicher", "arbeitsspeicher", "verbrauch", "verbraucht",
	}, []string{
		// Verbrauchs- and Speicherverbrauch, but not Verbraucher (consumer)
		// or Stromverbrauch (power).
		"^verbrauchs", "$verbrauch:speicher,ressourcen,cpu,ram,system,prozessor,arbeitsspeicher",
		"auslastung",
	})

	containerTerms = newToolIntentTerms([]string{"docker", "dockerfile", "container", "compose"}, nil)

	mcpTerms = newToolIntentTerms([]string{"mcp", "model context protocol"}, nil)

	composioTerms = newToolIntentTerms([]string{
		"composio", "integration", "gmail", "google mail", "googlemail", "g mail",
	}, nil)

	manusTerms = newToolIntentTerms([]string{"manus"}, nil)

	pdfTerms = newToolIntentTerms([]string{"pdf"}, nil)

	// networkCameraTerms name the camera tools outright; the other camera
	// cues need a second word (networkCameraActionTerms, arming).
	networkCameraTerms = newToolIntentTerms([]string{
		"go2rtc", "onvif", "netzwerkkamera", "netzwerk kamera", "ip kamera", "ip camera", "network camera",
		"personenerkennung", "personen erkennung", "person detection", "people detection",
	}, nil)
	networkCameraActionTerms = newToolIntentTerms([]string{
		"snapshot", "live stream", "livebild", "stream anzeigen",
	}, nil)
	networkCameraAlarmTerms = newToolIntentTerms([]string{"alarmanlage", "alarm system"}, nil)
)

func isResourceUsageIntent(text toolIntentText) bool { return resourceUsageTerms.matches(text) }

func isContainerIntent(text toolIntentText) bool { return containerTerms.matches(text) }

func isMCPIntent(text toolIntentText) bool { return mcpTerms.matches(text) }

func isComposioIntent(text toolIntentText) bool { return composioTerms.matches(text) }

func isManusIntent(text toolIntentText) bool { return manusTerms.matches(text) }

func isPDFIntent(text toolIntentText) bool { return pdfTerms.matches(text) }

// isNetworkCameraIntent reports a request for the go2rtc camera tool: a term
// that names it outright, a camera with a live-view action, or a camera or
// alarm system with an arming word.
func isNetworkCameraIntent(text toolIntentText) bool {
	if len(text.words) == 0 {
		return false
	}
	if networkCameraTerms.matches(text) {
		return true
	}
	cameraMentioned := networkCameraWord(text)
	if cameraMentioned && networkCameraActionTerms.matches(text) {
		return true
	}
	// Arming person detection ("scharf schalten", "unscharf", "arm", "disarm") needs a camera
	// or an alarm system in the same request.
	armingContext := cameraMentioned || networkCameraAlarmTerms.matches(text)
	return armingContext && networkCameraArmingWord(text)
}

// networkCameraWord reports a word naming a camera, compounds such as "gartenkamera" or
// "kamerabild" included; "kamerad" (comrade) and its compounds do not count.
func networkCameraWord(text toolIntentText) bool {
	for _, word := range text.words {
		if strings.Contains(word, "camera") || (strings.Contains(word, "kamera") && !strings.Contains(word, "kamerad")) {
			return true
		}
	}
	return false
}

// networkCameraArmingWord reports an arming word as a whole word, so "alarm" or "warm" do
// not count as "arm".
func networkCameraArmingWord(text toolIntentText) bool {
	for _, word := range text.words {
		switch word {
		case "scharf", "unscharf", "scharfschalten", "scharfgeschaltet", "unscharfschalten",
			"arm", "disarm", "armed", "disarmed", "arming", "disarming":
			return true
		}
	}
	return false
}
