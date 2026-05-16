package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestIsAnnouncementOnlyResponseBeforeAnyToolCall(t *testing.T) {
	tc := ToolCall{}
	if !isAnnouncementOnlyResponse("Ich prüfe jetzt die Logs und suche den Fehler.", tc, false, false, "finde den Fehler") {
		t.Fatal("expected pre-tool announcement to trigger recovery")
	}
}

func TestClaimsToolUnavailableWithoutDiscovery(t *testing.T) {
	cases := []string{
		"Tool 'yepapi' not found. It may be disabled in config.",
		"Ich sehe das YepAPI-Tool einfach nicht in meiner aktiven Tool-Liste.",
		"Das Tool ist nicht verfügbar.",
	}
	for _, content := range cases {
		if !claimsToolUnavailableWithoutDiscovery(content) {
			t.Fatalf("expected availability claim to trigger recovery: %q", content)
		}
	}
}

func TestClaimsToolUnavailableWithoutDiscoveryIgnoresDiscoveryInstruction(t *testing.T) {
	content := "Use discover_tools to check whether the tool is not available."
	if claimsToolUnavailableWithoutDiscovery(content) {
		t.Fatalf("did not expect discovery instruction to trigger recovery: %q", content)
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

func TestIsAnnouncementOnlyResponseAfterToolCallWithBugReportCompletionDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	content := "Erledigt! Der Bug-Report wurde erstellt und direkt in den Chat gesendet."
	if isAnnouncementOnlyResponse(content, tc, false, true, "analysiere das bild") {
		t.Fatal("did not expect bug report completion summary to trigger recovery")
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

func TestIsAnnouncementOnlyResponseConditionalApprovalDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	content := `Alles klar. Das ist ein System-/Tool-Aufruffehler, nicht dein Inhalt.

Fehlerbericht für den Coding Agent:

Betroffenes Tool: api_request
Fehlermeldung: invalid function arguments JSON (Call wurde verworfen)

Wenn du willst, mach ich direkt weiter mit deinem eigentlichen Auftrag ("KI-News komplett neu aufsetzen") ab dem letzten stabilen Punkt.`
	if !asksUserForInput(content) {
		t.Fatal("expected conditional German approval phrasing to be detected as user input")
	}
	if isAnnouncementOnlyResponse(content, tc, true, false, "KI-News komplett neu aufsetzen") {
		t.Fatal("did not expect conditional approval prompt to trigger announcement recovery")
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

func TestAnnouncementDetector_CatchesPlaybackActionPromise(t *testing.T) {
	tc := ToolCall{}
	content := `Spiele "Überall zuhause" nochmal auf dem Google Home Mini ab.`
	if !isAnnouncementOnlyResponse(content, tc, false, false, "spiel es nochmal ab") {
		t.Fatal("expected playback action promise without tool call to trigger announcement detection")
	}
}

func TestAnnouncementDetector_CatchesGermanBuildPromiseWithDoneSignal(t *testing.T) {
	tc := ToolCall{}
	lastUserMsg := "<external_data>The user is chatting from AuraGo Virtual Desktop.</external_data>\nkannst du musik in space invaders einbauen?"

	cases := []string{
		"Klar — kann ich machen. Ich bau dir Musik in Space Invaders ein. Einen kurzen Moment.",
		"Ja — kann ich.\nIch bau dir jetzt echte **Retro-Arcade-Hintergrundmusik** in Space Invaders ein (loopend, mit sauberem Start nach User-Input).",
	}
	for _, content := range cases {
		parsed := ParsedToolResponse{
			SanitizedContent: content,
			IsFinished:       true,
		}
		if !isAnnouncementOnlyResponse(parsed.SanitizedContent, tc, false, false, lastUserMsg) {
			t.Fatalf("expected colloquial German build promise to trigger announcement detection: %q", content)
		}
		if !shouldRecoverAnnouncementOnlyResponse(parsed, tc, false, false, lastUserMsg) {
			t.Fatalf("expected action promise with <done/> to still trigger recovery: %q", content)
		}
	}
}

func TestAnnouncementDetector_CatchesFabricatedOperationalSuccessBeforeToolCall(t *testing.T) {
	tc := ToolCall{}
	content := "**Test-Ergebnis: POSITIV** ✅\n\nMiniMax TTS funktioniert einwandfrei."
	if !isAnnouncementOnlyResponse(content, tc, false, false, "teste tts") {
		t.Fatal("expected fabricated operational success claim before tool call to trigger recovery")
	}
}

func TestAnnouncementDetector_AllowsInformationalSuccessAnswerWithoutExecutionRequest(t *testing.T) {
	tc := ToolCall{}
	content := "**Test-Ergebnis: POSITIV** ✅\n\nMiniMax TTS funktioniert einwandfrei."
	if isAnnouncementOnlyResponse(content, tc, false, false, "war der test erfolgreich?") {
		t.Fatal("did not expect informational success answer to trigger recovery")
	}
}

func TestDesktopAnnouncementRecoveryRejectsDoneWithoutToolAfterPromise(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AnnouncementDetector.Enabled = true
	cfg.Agent.AnnouncementDetector.MaxRetries = 2
	logger := slog.New(slog.NewTextHandler(testDiscardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	s := &agentLoopState{
		ctx:                context.Background(),
		broker:             NoopBroker{},
		currentLogger:      logger,
		useNativeFunctions: true,
		announcementCount:  1,
		lastUserMsg:        "ERROR: Your last response was text-only - use the native function-calling mechanism NOW.",
		recoverySession:    NewRecoverySessionState(logger, NoopBroker{}, cfg),
		runCfg: RunConfig{
			Config:        cfg,
			SessionID:     "virtual-desktop",
			MessageSource: "virtual_desktop_chat",
			ShortTermMem:  stm,
		},
	}

	content := "Erwischt - mein Fehler. Schick mir den Auftrag nochmal in einem Satz, dann mach ich's sofort wirklich. <done/>"
	parsed := ParsedToolResponse{
		Content:          content,
		SanitizedContent: strings.ReplaceAll(content, " <done/>", ""),
		IsFinished:       true,
	}

	_, _, shouldContinue, _ := handleAgentLoopRecoveries(s, content, ToolCall{}, parsed, true, emotionBehaviorPolicy{})
	if !shouldContinue {
		t.Fatal("expected desktop recovery to reject <done/> when no desktop tool ran after an action promise")
	}
	if s.announcementCount != 2 {
		t.Fatalf("announcementCount = %d, want 2", s.announcementCount)
	}
	if len(s.req.Messages) == 0 || !strings.Contains(s.req.Messages[len(s.req.Messages)-1].Content, "virtual_desktop") {
		t.Fatalf("expected recovery feedback to require a desktop tool call, got %#v", s.req.Messages)
	}
}

func TestDesktopEmptyResponseAfterToolRequiresRecovery(t *testing.T) {
	runCfg := RunConfig{MessageSource: "virtual_desktop_chat"}

	if !shouldAbortDesktopEmptyAfterTool(runCfg, "", true) {
		t.Fatal("expected empty desktop response after a tool call to require recovery")
	}
	if !shouldAbortDesktopEmptyAfterTool(runCfg, "<think>still thinking", true) {
		t.Fatal("expected reasoning-only desktop response after a tool call to require recovery")
	}
	if shouldAbortDesktopEmptyAfterTool(runCfg, "done", true) {
		t.Fatal("did not expect visible content to require empty-response recovery")
	}
	if shouldAbortDesktopEmptyAfterTool(runCfg, "", false) {
		t.Fatal("did not expect pre-tool empty response to use the desktop post-tool guard")
	}
	if shouldAbortDesktopEmptyAfterTool(RunConfig{MessageSource: "web_chat"}, "", true) {
		t.Fatal("did not expect non-desktop chat to use the desktop post-tool guard")
	}
}

func TestAsksUserForInputDetectsMidTaskQuestions(t *testing.T) {
	content := strings.Repeat("Die Webseite ist gebaut und der Deploy braucht eine Entscheidung. ", 8) +
		"Soll ich die bestehende Netlify-Seite überschreiben?"
	if !asksUserForInput(content) {
		t.Fatal("expected German mid-task question to be detected")
	}
}

func TestAsksUserForInputIgnoresCompletionSummary(t *testing.T) {
	content := "Build completed successfully. Netlify deploy failed with HTTP 500, so I kept the local dist output unchanged."
	if asksUserForInput(content) {
		t.Fatal("did not expect completion summary to be treated as a user question")
	}
}

type testDiscardWriter struct{}

func (testDiscardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
