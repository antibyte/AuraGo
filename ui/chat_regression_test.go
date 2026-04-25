package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatFrontend_ToolLeakSanitizerPatternsRemainPresent(t *testing.T) {
	t.Parallel()

	messagesPath := filepath.Join("js", "chat", "chat-messages.js")
	streamingPath := filepath.Join("js", "chat", "chat-streaming.js")

	messagesContent, err := os.ReadFile(messagesPath)
	if err != nil {
		t.Fatalf("read %s: %v", messagesPath, err)
	}
	streamingContent, err := os.ReadFile(streamingPath)
	if err != nil {
		t.Fatalf("read %s: %v", streamingPath, err)
	}

	msg := string(messagesContent)
	stream := string(streamingContent)

	requiredMessageMarkers := []string{
		"function stripLeakedToolMarkup",
		"minimax:tool_call",
		"<invoke\\b",
		"<parameter\\b",
		"\\[Suggested next step\\]",
		"containsLeakedToolMarkup(text)",
	}
	for _, marker := range requiredMessageMarkers {
		if !strings.Contains(msg, marker) {
			t.Fatalf("%s is missing expected regression marker %q", messagesPath, marker)
		}
	}

	requiredStreamingMarkers := []string{
		"stripLeakedToolMarkup(payload.content)",
		"stripLeakedToolMarkup(thinkingText)",
		"data.event === 'video'",
		"appendVideoMessage(videoData)",
	}
	for _, marker := range requiredStreamingMarkers {
		if !strings.Contains(stream, marker) {
			t.Fatalf("%s is missing expected regression marker %q", streamingPath, marker)
		}
	}
}

func TestChatFrontend_VideoPlayerFlowRemainsPresent(t *testing.T) {
	t.Parallel()

	mainPath := filepath.Join("js", "chat", "main.js")
	messagesPath := filepath.Join("js", "chat", "chat-messages.js")
	streamingPath := filepath.Join("js", "chat", "chat-streaming.js")

	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainPath, err)
	}
	messagesContent, err := os.ReadFile(messagesPath)
	if err != nil {
		t.Fatalf("read %s: %v", messagesPath, err)
	}
	streamingContent, err := os.ReadFile(streamingPath)
	if err != nil {
		t.Fatalf("read %s: %v", streamingPath, err)
	}

	all := string(mainContent) + "\n" + string(messagesContent) + "\n" + string(streamingContent)
	requiredMarkers := []string{
		"let seenSSEVideos = new Set()",
		"function appendVideoMessage(videoData)",
		"className = 'chat-video-player'",
		"renderVideoLinksAsPlayers(finalHTML)",
		"data.event === 'video'",
	}
	for _, marker := range requiredMarkers {
		if !strings.Contains(all, marker) {
			t.Fatalf("chat frontend is missing expected video player marker %q", marker)
		}
	}
}

func TestChatFrontend_PasteAttachmentFlowRemainsPresent(t *testing.T) {
	t.Parallel()

	mainPath := filepath.Join("js", "chat", "main.js")

	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainPath, err)
	}

	mainJS := string(mainContent)
	requiredMarkers := []string{
		"userInput.addEventListener('paste'",
		"item.kind === 'file'",
		"queueAttachmentUploads(files)",
		"_normalizedAttachmentName(file)",
		"formData.append('file', file, _normalizedAttachmentName(file))",
	}
	for _, marker := range requiredMarkers {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("%s is missing expected paste-upload regression marker %q", mainPath, marker)
		}
	}
}

func TestChatRobotGreetingStartsAboveGreetingText(t *testing.T) {
	t.Parallel()

	robotPath := filepath.Join("js", "chat", "robot-mascot.js")
	robotContent, err := os.ReadFile(robotPath)
	if err != nil {
		t.Fatalf("read %s: %v", robotPath, err)
	}

	robotJS := string(robotContent)
	requiredMarkers := []string{
		"const verticalLift = window.innerWidth <= 767 ? 48 : 56;",
		"top: rect.top + ((rect.height - size) / 2) - verticalLift",
	}
	for _, marker := range requiredMarkers {
		if !strings.Contains(robotJS, marker) {
			t.Fatalf("%s is missing expected elevated greeting placement marker %q", robotPath, marker)
		}
	}
}

func TestMediaFrontend_VideoTabFlowRemainsPresent(t *testing.T) {
	t.Parallel()

	mediaHTMLPath := "media.html"
	mediaJSPath := filepath.Join("js", "media", "main.js")
	mediaCSSPath := filepath.Join("css", "media.css")

	mediaHTML, err := os.ReadFile(mediaHTMLPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaHTMLPath, err)
	}
	mediaJS, err := os.ReadFile(mediaJSPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaJSPath, err)
	}
	mediaCSS, err := os.ReadFile(mediaCSSPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaCSSPath, err)
	}

	combined := string(mediaHTML) + "\n" + string(mediaJS) + "\n" + string(mediaCSS)
	requiredMarkers := []string{
		`id="tab-videos"`,
		`id="panel-videos"`,
		`MEDIA_TABS_ORDER = ['images', 'audio', 'videos', 'documents']`,
		`type: 'video'`,
		`function loadVideos()`,
		`className = 'media-video-player'`,
		`.media-video-grid`,
	}
	for _, marker := range requiredMarkers {
		if !strings.Contains(combined, marker) {
			t.Fatalf("media frontend is missing expected video tab marker %q", marker)
		}
	}
}
