package agent

import "testing"

func TestIsAnnouncementOnlyResponseBeforeAnyToolCall(t *testing.T) {
	tc := ToolCall{}
	if !isAnnouncementOnlyResponse("Ich prüfe jetzt die Logs und suche den Fehler.", tc, false, false, "finde den Fehler") {
		t.Fatal("expected pre-tool announcement to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseAfterToolCallWithForwardCue(t *testing.T) {
	tc := ToolCall{}
	content := "Dateien aktualisiert! Jetzt baue ich das Projekt und deploye es:"
	if !isAnnouncementOnlyResponse(content, tc, false, true, "fahre fort") {
		t.Fatal("expected post-tool forward-looking announcement to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseAfterToolCallWithCompletionOnly(t *testing.T) {
	tc := ToolCall{}
	content := "Die Dateien wurden aktualisiert und die Bilder sind eingebaut."
	if isAnnouncementOnlyResponse(content, tc, false, true, "fahre fort") {
		t.Fatal("did not expect plain completion message to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseAfterToolCallWithForwardCueButNoAction(t *testing.T) {
	tc := ToolCall{}
	content := "Jetzt ist alles aktualisiert."
	if isAnnouncementOnlyResponse(content, tc, false, true, "fahre fort") {
		t.Fatal("did not expect status-only update to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseQuestionDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	content := "Soll ich jetzt auch direkt deployen?"
	if isAnnouncementOnlyResponse(content, tc, false, true, "fahre fort") {
		t.Fatal("did not expect question to trigger announcement recovery")
	}
}

func TestIsAnnouncementOnlyResponseAfterToolCallWithPlanStructure(t *testing.T) {
	tc := ToolCall{}
	content := "1. Build production bundle\n2. Deploy to Netlify\n3. Verify homepage"
	if !isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
		t.Fatal("expected structured action plan after tool call to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseBeforeToolCallWithPathAndActionIntent(t *testing.T) {
	tc := ToolCall{}
	content := "Next I will update src/App.tsx and then run the build."
	if !isAnnouncementOnlyResponse(content, tc, false, false, "continue") {
		t.Fatal("expected generic path/action intent announcement to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseAfterToolCallWithCompletionEvidenceDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	content := "Build completed successfully. Netlify deploy is live at https://ki-news.netlify.app"
	if isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
		t.Fatal("did not expect completion evidence to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseAfterToolCallWithFailureEvidenceDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	content := "Build failed with exit code 1 in src/App.tsx"
	if isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
		t.Fatal("did not expect failure evidence to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseMixedCompletionAndNextActionStillTriggers(t *testing.T) {
	tc := ToolCall{}
	content := "Dateien aktualisiert und gespeichert. Jetzt deploye ich die Seite zu Netlify."
	if !isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
		t.Fatal("expected unfinished next action to trigger recovery even with completion evidence present")
	}
}

func TestIsAnnouncementOnlyResponseStallGuardCompletionSummaryDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	// Simulates agent giving a completion summary after a STALL GUARD fake user message
	// (lastResponseWasTool=false because STALL GUARD injected a user turn, not a tool result).
	// The bullet list + "deployed" used to trigger hasPlanStructure+hasActionIntent → false positive loop.
	content := "✅ TASK COMPLETE — Die psychedelische Tunnel-Version ist erfolgreich deployed:\n- ✅ Sound-System aktiv\n- ✅ Tunnel-Effekt aktiv"
	if isAnnouncementOnlyResponse(content, tc, false, false, "Bitte bestätige ob alles fertig ist") {
		t.Fatal("did not expect completion summary in pre-tool path to trigger recovery (stall-guard scenario)")
	}
}

func TestIsAnnouncementOnlyResponsePreToolCompletionWithNextActionStillTriggers(t *testing.T) {
	tc := ToolCall{}
	// Even in the pre-tool path, completion evidence + explicit forward cue + action cue must still trigger.
	content := "Files updated successfully! Now I will deploy to Netlify."
	if !isAnnouncementOnlyResponse(content, tc, false, false, "continue") {
		t.Fatal("expected mixed completion+next-action in pre-tool path to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponsePublishLocalCompletionWithURLDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	// Regression test: after publish_local succeeds, the agent reports the local URL.
	// "jetzt" (current state) + URL used to falsely trigger hasActionIntent+containsForwardCue
	// overriding completion evidence → false-positive ERROR loop.
	content := "Fertig! 🚀 Die Psychedelic Tunnel WebGL Demo läuft jetzt lokal auf deinem Server:\n\n**→ http://192.168.6.238:8080**\n\nDie Demo zeigt einen hypnotisierenden Tunnel mit psychedelischen Farben, Ringmustern und Pulsieren-Effekt. Probier's aus! ✨"
	if isAnnouncementOnlyResponse(content, tc, false, true, "fahre fort") {
		t.Fatal("did not expect post-publish completion message with URL and 'jetzt' to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseStatusWithURLAndJetztPreToolDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	// Same scenario but in pre-tool path (e.g. stall guard fired after publish_local).
	content := "Fertig! Die Demo läuft jetzt lokal unter http://192.168.6.238:8080 — viel Spaß!"
	if isAnnouncementOnlyResponse(content, tc, false, false, "check status") {
		t.Fatal("did not expect status-with-URL completion in pre-tool path to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseLetMeWithoutOperationalTermTriggers(t *testing.T) {
	tc := ToolCall{}
	// After a tool error the agent says "I'll do it myself! Let me look at the code
	// structure first." — no file path, no operational term, but "lass mich" is a
	// clear announcement phrase. Must trigger recovery.
	content := "Ich mach das selbst! Lass mich zuerst die existierende Code-Struktur des COSMIC SURGE Spiels anschauen."
	if !isAnnouncementOnlyResponse(content, tc, false, true, "power-ups") {
		t.Fatal("expected post-tool 'lass mich' announcement without operational term to trigger recovery")
	}
}

func TestIsAnnouncementOnlyResponseLetMeWithCompletionEvidenceDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	// "Fertig! Ich werde dir jetzt die Ergebnisse zeigen." — completion evidence guards
	// against false-positive even though "ich werde" is an announcement phrase.
	content := "Fertig! Ich werde dir jetzt die Ergebnisse zeigen."
	if isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
		t.Fatal("did not expect completion evidence to allow announcement phrase to trigger recovery")
	}
}

// TestIsAnnouncementOnlyResponseCheckmarkCompletionSummaryDoesNotTrigger ensures that a
// response starting with ✅ and listing completed features (with a URL) is not mistaken
// for an announcement. This was the real-world false-positive after a successful
// filesystem write that caused the agent to keep running after the chat already showed
// the completion message.
func TestIsAnnouncementOnlyResponseCheckmarkCompletionSummaryDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	// Mirrors the actual post-tool response that triggered the false positive:
	// think-block stripped, leaving only the completion summary with emoji, feature
	// bullet list, and a localhost URL.
	content := "✅ **Synthwave-Musik hinzugefügt!** 🎸\n\n**Features:**\n- 🎵 Synthwave Bassline (60–180 Hz)\n- 🥁 Drum Machine (Kick, Snare, Hi-Hat)\n- 🎹 Chord-Pads\n\nDas Spiel läuft unter http://localhost:8080/phaser-demo/"
	if isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
		t.Fatal("✅ completion summary with feature list and URL must NOT trigger announcement recovery")
	}
}

// TestIsAnnouncementOnlyResponseGermanPastParticipleDoesNotTrigger ensures German
// past participles like "hinzugefügt", "eingebaut", "ergänzt" are treated as
// completion evidence and prevent false-positive announcement detection.
func TestIsAnnouncementOnlyResponseGermanPastParticipleDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	cases := []string{
		"Ich habe die Musik hinzugefügt. Das Feature ist aktiviert.",
		"Die Komponente wurde eingebaut. Schau mal: http://localhost:3000",
		"Erfolgreich ergänzt! Hier sind die Änderungen:\n- Punkt 1\n- Punkt 2",
	}
	for _, content := range cases {
		if isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
			t.Fatalf("German past-participle completion summary must NOT trigger: %q", content)
		}
	}
}

// ── Word-boundary regression tests ──

func TestContainsAnyWordPhraseEndBoundary(t *testing.T) {
	// "i will" must NOT match inside "i willing" — end-boundary check.
	if containsAnyWordPhrase("i willing to help", []string{"i will"}) {
		t.Fatal("end-boundary bug: 'i will' should NOT match 'i willing'")
	}
	// But must match when followed by a space.
	if !containsAnyWordPhrase("i will deploy now", []string{"i will"}) {
		t.Fatal("expected 'i will' to match with space separator")
	}
}

func TestContainsAnyWordPhraseStartBoundary(t *testing.T) {
	// "read" must NOT match inside "already".
	if containsAnyWordPhrase("i already did that", []string{"read"}) {
		t.Fatal("start-boundary bug: 'read' should NOT match 'already'")
	}
}

func TestContainsAnyWordPhraseBothBoundaries(t *testing.T) {
	// "list" must NOT match inside "listen".
	if containsAnyWordPhrase("listen to this", []string{"list"}) {
		t.Fatal("boundary bug: 'list' should NOT match 'listen'")
	}
	// But must match at the start of the string.
	if !containsAnyWordPhrase("list the files", []string{"list"}) {
		t.Fatal("expected 'list' to match at start of string")
	}
	// And at end of string.
	if !containsAnyWordPhrase("show me the list", []string{"list"}) {
		t.Fatal("expected 'list' to match at end of string")
	}
}

func TestOperationalTermsNoFileExtensionFalsePositive(t *testing.T) {
	tc := ToolCall{}
	// Mentioning a filename like "config.yaml" should not trigger as an announcement.
	content := "The config.yaml file looks correct to me."
	if isAnnouncementOnlyResponse(content, tc, false, false, "check config") {
		t.Fatal("file extension in plain text should not trigger announcement recovery")
	}
}

func TestNochmalDoesNotTriggerPreTool(t *testing.T) {
	tc := ToolCall{}
	// "Erkläre das nochmal" is a user-like statement; "nochmal" was de-escalated from
	// announcementPhrases to postToolForwardCues, so it should NOT trigger pre-tool.
	content := "Erkläre das nochmal bitte."
	if isAnnouncementOnlyResponse(content, tc, false, false, "erkläre") {
		t.Fatal("'nochmal' in conversational pre-tool text should not trigger announcement")
	}
}

// TestIsAnnouncementOnlyResponsePureThinkBlockDoesNotTrigger ensures that a response
// consisting entirely of <think>...</think> reasoning (SanitizedContent == "") never
// triggers the announcement detector.  The caller must NOT fall back to raw content.
func TestIsAnnouncementOnlyResponsePureThinkBlockDoesNotTrigger(t *testing.T) {
	// The actual announcement detector receives the SanitizedContent (think-stripped) string,
	// which is "" for pure think-block responses.  Passing "" should return false.
	tc := ToolCall{}
	// Simulate what the agent loop does after stripping think tags — passes SanitizedContent.
	if isAnnouncementOnlyResponse("", tc, true, false, "absolute stille") {
		t.Fatal("empty sanitized content (pure think-block) must NOT trigger announcement recovery")
	}
	if isAnnouncementOnlyResponse("", tc, false, true, "weiter") {
		t.Fatal("empty sanitized content (pure think-block) post-tool must NOT trigger announcement recovery")
	}
}
