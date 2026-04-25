package ui

import (
	"encoding/binary"
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

func TestChatPapyrusThemeUsesRefinedManuscriptPalette(t *testing.T) {
	t.Parallel()

	papyrusPath := filepath.Join("css", "chat-papyrus.css")
	papyrusContent, err := os.ReadFile(papyrusPath)
	if err != nil {
		t.Fatalf("read %s: %v", papyrusPath, err)
	}

	papyrusCSS := string(papyrusContent)
	requiredMarkers := []string{
		"--papyrus-ink-blue: #1e3f66;",
		"--papyrus-verdigris: #2f7f73;",
		"--papyrus-wax: #9f3f35;",
		"--papyrus-font-body: 'Inter', system-ui, sans-serif;",
		"linear-gradient(135deg, rgba(30, 63, 102, 0.34) 0%, rgba(47, 127, 115, 0.2) 38%, rgba(159, 63, 53, 0.16) 72%, rgba(20, 35, 51, 0.98) 100%)",
		"linear-gradient(135deg, rgba(30, 63, 102, 0.96), rgba(47, 127, 115, 0.92))",
		"opacity: 0.38;",
	}
	for _, marker := range requiredMarkers {
		if !strings.Contains(papyrusCSS, marker) {
			t.Fatalf("%s is missing refined papyrus marker %q", papyrusPath, marker)
		}
	}

	if strings.Contains(papyrusCSS, "[data-theme=\"papyrus\"] body {\n    background: url('../wood.jpg');") {
		t.Fatalf("%s still uses the old wood-only body background", papyrusPath)
	}
}

func TestChatToolIconPngSpriteCatalogRemainsWired(t *testing.T) {
	t.Parallel()

	iconsPath := filepath.Join("js", "chat", "tool-icons.js")
	spritePath := filepath.Join("img", "tool-icons-sprite.png")
	streamingPath := filepath.Join("js", "chat", "chat-streaming.js")
	cssPath := filepath.Join("css", "chat.css")
	indexPath := "index.html"

	iconsContent, err := os.ReadFile(iconsPath)
	if err != nil {
		t.Fatalf("read %s: %v", iconsPath, err)
	}
	spriteContent, err := os.ReadFile(spritePath)
	if err != nil {
		t.Fatalf("read %s: %v", spritePath, err)
	}
	streamingContent, err := os.ReadFile(streamingPath)
	if err != nil {
		t.Fatalf("read %s: %v", streamingPath, err)
	}
	cssContent, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("read %s: %v", cssPath, err)
	}
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read %s: %v", indexPath, err)
	}

	iconsJS := string(iconsContent)
	if got := strings.Count(iconsJS, "slot: "); got != 100 {
		t.Fatalf("%s has %d sprite slots, want 100", iconsPath, got)
	}
	requiredIconMarkers := []string{
		"key: 'execute_shell'",
		"key: 'docker'",
		"key: 'proxmox'",
		"key: 'home_assistant'",
		"key: 'github'",
		"key: 'cloudflare_tunnel'",
		"key: 'truenas'",
		"key: 'generic_tool'",
		"window.AuraToolIcons",
		"createIcon(toolName",
	}
	for _, marker := range requiredIconMarkers {
		if !strings.Contains(iconsJS, marker) {
			t.Fatalf("%s is missing tool icon marker %q", iconsPath, marker)
		}
	}

	const pngHeaderLen = 26
	if len(spriteContent) < pngHeaderLen || string(spriteContent[:8]) != "\x89PNG\r\n\x1a\n" || string(spriteContent[12:16]) != "IHDR" {
		t.Fatalf("%s is not a valid PNG sprite header", spritePath)
	}
	width := binary.BigEndian.Uint32(spriteContent[16:20])
	height := binary.BigEndian.Uint32(spriteContent[20:24])
	colorType := spriteContent[25]
	if width != 1280 || height != 1280 {
		t.Fatalf("%s is %dx%d, want 1280x1280", spritePath, width, height)
	}
	if colorType != 6 {
		t.Fatalf("%s uses PNG color type %d, want 6 for RGBA alpha", spritePath, colorType)
	}

	streamingJS := string(streamingContent)
	for _, marker := range []string{"AuraToolIcons.createIcon", "setStatusToolIcon(data.detail)", "setStatusToolIcon('thinking')"} {
		if !strings.Contains(streamingJS, marker) {
			t.Fatalf("%s is missing icon wiring marker %q", streamingPath, marker)
		}
	}
	if strings.Contains(streamingJS, "const TOOL_ICONS = {") {
		t.Fatalf("%s still contains the old emoji tool icon map", streamingPath)
	}

	css := string(cssContent)
	for _, marker := range []string{".tool-icon-sprite", "background-image: url('/img/tool-icons-sprite.png", ".status-tool-icon"} {
		if !strings.Contains(css, marker) {
			t.Fatalf("%s is missing icon CSS marker %q", cssPath, marker)
		}
	}

	if !strings.Contains(string(indexContent), `/js/chat/tool-icons.js`) {
		t.Fatalf("%s does not load the tool icon catalog", indexPath)
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
