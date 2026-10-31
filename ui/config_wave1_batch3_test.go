package ui

import (
	"strings"
	"testing"
)

// Wave 1, batch 3: result lines, draft tests and dock secrets of the modules.
func TestConfigWave1Batch3ModuleContracts(t *testing.T) {
	draftTests := map[string]string{
		"browser_automation": "ba-test-btn", "document_creator": "dc-test-btn", "email": "ea-test-btn",
		"image_generation": "imggen-test-btn", "ldap": "ldap-test-btn", "media_conversion": "mc-test-btn",
		"music_generation": "music-test-btn", "paperless": "paperless-test-btn", "three_d_printers": "three-d-printer-test-elegoo-",
		"video_generation": "video-test-btn", "yepapi": "yepapi-test-btn",
	}
	for module, button := range draftTests {
		src := string(mustReadUIFile(t, "cfg/"+module+".js"))
		if !strings.Contains(src, "cfgActionResult(") {
			t.Errorf("%s does not use cfgActionResult", module)
		}
		if i := strings.Index(src, button); i < 0 || !strings.Contains(src[i:i+400], `data-config-action-draft="true"`) {
			t.Errorf("%s: test button %s is not marked as a draft action", module, button)
		}
	}
	for module, path := range map[string]string{"ldap": "ldap.bind_password", "paperless": "paperless_ngx.api_token"} {
		src := string(mustReadUIFile(t, "cfg/"+module+".js"))
		if !strings.Contains(src, `data-path="`+path+`"`) {
			t.Errorf("%s: secret %s is not in the save dock", module, path)
		}
		if strings.Contains(src, "/api/vault/secrets") {
			t.Errorf("%s still posts to the vault itself", module)
		}
	}
	space := string(mustReadUIFile(t, "cfg/space_agent.js"))
	if strings.Count(space, `data-config-action-draft="false"`) != 2 {
		t.Error("space_agent status and recreate must be runtime-only actions")
	}
}
