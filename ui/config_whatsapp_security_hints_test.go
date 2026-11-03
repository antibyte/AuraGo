package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two WhatsApp security hints carry no server data, so both title and description are
// translated (security.hint.<id>.*). The Danger Zone text names the WhatsApp device keys.
func TestWhatsAppSecurityHintsAreLocalized(t *testing.T) {
	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"security.hint.whatsapp_self_chat_personal_account.title",
		"security.hint.whatsapp_self_chat_personal_account.description",
		"security.hint.whatsapp_dedicated_no_owner.title",
		"security.hint.whatsapp_dedicated_no_owner.description",
	}
	const dangerKey = "config.danger.unsafe_host_execution.desc"

	en, err := readJSONFileMap(filepath.Join("lang", "config", "security", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range langs {
		security, err := readJSONFileMap(filepath.Join("lang", "config", "security", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range keys {
			value, _ := security[key].(string)
			if strings.TrimSpace(value) == "" {
				t.Errorf("%s: %s is missing", lang, key)
				continue
			}
			if lang != "en" && value == en[key] {
				t.Errorf("%s: %s is still the English text", lang, key)
			}
		}

		danger, err := readJSONFileMap(filepath.Join("lang", "config", "danger", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if value, _ := danger[dangerKey].(string); !strings.Contains(value, "WhatsApp") {
			t.Errorf("%s: %s does not name the WhatsApp device keys", lang, dangerKey)
		}
	}

	source, err := os.ReadFile(filepath.Join("..", "internal", "server", "security_check.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"whatsapp_self_chat_personal_account", "whatsapp_dedicated_no_owner"} {
		if !strings.Contains(string(source), `ID: "`+id+`"`) {
			t.Errorf("security_check.go lacks the hint ID %s", id)
		}
	}
}
