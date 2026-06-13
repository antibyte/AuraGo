package memory

import "testing"

func TestCoreMemoryFactPolicyRejectsTransientOperationalJunk(t *testing.T) {
	cases := []string{
		`[recent_operational_details] Virtual desktop app path: Apps/space-invaders.html, app_id: space-invaders`,
		`2026-05-08: Created "Chaos Symphony XIII" - Duration: 3:55, uploaded to Koofr /aurago/music, Media Registry ID: 2320.`,
		`Image generation failed - MiniMax weekly usage limit reached (350/350 used). Resets at 2026-05-11T00:00:00Z.`,
		`Tool virtual_desktop failed during virtual_desktop_chat: no such file or directory`,
		`Local: /home/aurago/aurago/data/audio/music_12139354.mp3, Hochgeladen auf Koofr /aurago/music.`,
		`KI News Mission (mission_1778830830260898251) erfolgreich mit korrektem Port 11434 konfiguriert. Letzter Lauf 2026-05-25 06:00 Uhr: success. Port-Problem ist behoben.`,
		`KI News Seite (ki-news) aktualisiert am 2026-05-30: 10 Artikel mit Quellen-Links, Build erfolgreich, Netlify-Deploy state: ready. Site-ID: c018062e-c3d6-44b6-aca5-8979387dfeae.`,
		`Ollama läuft auf Port 11434, Modell phi3:latest (3.8B, Q4_0) ist geladen. Stand 26.05.2026 18:02.`,
		`Ollama Health-Check 18:02 Uhr 26.05.: API auf Port 11434 antwortet, phi3:latest geladen.`,
		`Telegram Bot "SecretariyBot" antwortet nicht (HTTP 404), zuletzt gesehen Heartbeat 15:47 Uhr. Bitte Bot-Token prüfen.`,
		`Google Home Mini im Arbeitszimmer hat IP 192.168.6.130, Port 8009. Erreichbar via Chromecast/TTS.`,
		`Mission abgeschlossen: 192.168.6.2 per Port-Scan bestätigt erreichbar (SMB/Windows), Google Home Mini TTS-Begrüßung erfolgreich gesendet.`,
		`KI-News-Aktualisierung: 7 Artikel hinzugefügt und die lokale KI-News-Seite im Caddy-Container neu geladen.`,
		`Docker-Leichen Cleanup: 5 beendete Container entfernt.`,
		`ntfy/send_notification ist deaktiviert; Ntfy ist aktuell nicht verfügbar.`,
		`Tool-Verfügbarkeitsclaim: ddg_search ist immer aktiv und braucht keine Discovery.`,
	}

	for _, fact := range cases {
		if err := ValidateCoreMemoryFact(fact); err == nil {
			t.Fatalf("ValidateCoreMemoryFact(%q) = nil, want rejection", fact)
		}
	}
}

func TestReviewCoreMemoryFactsReportsCleanupCandidatesWithoutMutation(t *testing.T) {
	facts := []string{
		`User prefers direct German answers`,
		`KI-News-Seite im Caddy-Container aktualisiert; Caddy reload erfolgreich.`,
		`ntfy disabled in Core Memory but Ntfy reachable in chat channel context.`,
	}

	issues := ReviewCoreMemoryFacts(facts)
	if len(issues) != 2 {
		t.Fatalf("ReviewCoreMemoryFacts issues len = %d, want 2: %+v", len(issues), issues)
	}
	if facts[1] != `KI-News-Seite im Caddy-Container aktualisiert; Caddy reload erfolgreich.` {
		t.Fatalf("review must not mutate input facts: %#v", facts)
	}
	if issues[0].Index != 1 || issues[1].Index != 2 {
		t.Fatalf("review issues preserve source indices, got %+v", issues)
	}
	for _, issue := range issues {
		if issue.Fact == "" || issue.Reason == "" {
			t.Fatalf("review issue must include fact and reason: %+v", issue)
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
