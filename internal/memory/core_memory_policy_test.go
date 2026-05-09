package memory

import "testing"

func TestCoreMemoryFactPolicyRejectsTransientOperationalJunk(t *testing.T) {
	cases := []string{
		`[recent_operational_details] Virtual desktop app path: Apps/space-invaders.html, app_id: space-invaders`,
		`2026-05-08: Created "Chaos Symphony XIII" - Duration: 3:55, uploaded to Koofr /aurago/music, Media Registry ID: 2320.`,
		`Image generation failed - MiniMax weekly usage limit reached (350/350 used). Resets at 2026-05-11T00:00:00Z.`,
		`Tool virtual_desktop failed during virtual_desktop_chat: no such file or directory`,
		`Local: /home/aurago/aurago/data/audio/music_12139354.mp3, Hochgeladen auf Koofr /aurago/music.`,
	}

	for _, fact := range cases {
		if err := ValidateCoreMemoryFact(fact); err == nil {
			t.Fatalf("ValidateCoreMemoryFact(%q) = nil, want rejection", fact)
		}
	}
}

func TestCoreMemoryFactPolicyAllowsDurableFacts(t *testing.T) {
	cases := []string{
		`Username is Andi`,
		`User prefers direct German answers`,
		`User's main AuraGo repository is c:\Users\Andi\Documents\repo\AuraGo`,
		`The Proxmox node name is pve01`,
	}

	for _, fact := range cases {
		if err := ValidateCoreMemoryFact(fact); err != nil {
			t.Fatalf("ValidateCoreMemoryFact(%q) = %v, want allowed", fact, err)
		}
	}
}
