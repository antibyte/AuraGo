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
