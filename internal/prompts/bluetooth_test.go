package prompts

import (
	"strings"
	"testing"
)

func TestBluetoothPromptConditionAndOverview(t *testing.T) {
	flags := &ContextFlags{BluetoothEnabled: true}
	if !matchPromptCondition("bluetooth_enabled", flags) {
		t.Fatal("bluetooth_enabled prompt condition did not match")
	}
	if !strings.Contains(buildEnabledToolsOverview(flags), "bluetooth") {
		t.Fatal("enabled tools overview does not include bluetooth")
	}
	if matchPromptCondition("bluetooth_enabled", &ContextFlags{}) {
		t.Fatal("disabled Bluetooth unexpectedly matched prompt condition")
	}
}
