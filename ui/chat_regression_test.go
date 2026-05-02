package ui

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/fnv"
	"image/png"
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
		"(action|tool|tool_call|tool_name)",
		"\"parameters\"",
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
		"trimmed.includes('\"tool\"')",
		"trimmed.includes('\"parameters\"')",
		"data.event === 'video'",
		"appendVideoMessage(videoData)",
		"data.event === 'youtube_video'",
		"appendYouTubeMessage(youtubeData)",
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
		"let seenSSEYouTubeVideos = new Set()",
		"function appendVideoMessage(videoData)",
		"function appendYouTubeMessage(youtubeData)",
		"function renderYouTubeLinksAsPlayers(html)",
		"function safeYouTubeEmbedURL",
		"function youtubePlayerDedupKey",
		"className = 'chat-youtube-player'",
		"https://www.youtube-nocookie.com/embed/",
		"start_seconds",
		"className = 'chat-video-player'",
		"renderVideoLinksAsPlayers(finalHTML)",
		"renderYouTubeLinksAsPlayers(finalHTML)",
		"data.event === 'video'",
		"data.event === 'youtube_video'",
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

func TestChatFrontend_IntegrationsDrawerRemainsWired(t *testing.T) {
	t.Parallel()

	indexContent, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	drawerContent, err := os.ReadFile(filepath.Join("js", "chat", "modules", "integrations-drawer.js"))
	if err != nil {
		t.Fatalf("read integrations drawer module: %v", err)
	}

	indexHTML := string(indexContent)
	for _, marker := range []string{
		`id="integrations-toggle-btn"`,
		`class="integrations-edge-tab"`,
		`id="integrations-drawer"`,
		`/css/integrations-drawer.css`,
		`/js/chat/modules/integrations-drawer.js`,
	} {
		if !strings.Contains(indexHTML, marker) {
			t.Fatalf("index.html missing integrations drawer marker %q", marker)
		}
	}
	if strings.Contains(indexHTML, `btn-integrations-toggle`) {
		t.Fatal("integrations toggle must be rendered as the right-edge tab, not a header icon button")
	}

	drawerJS := string(drawerContent)
	for _, marker := range []string{
		"/api/integrations/webhosts",
		"window.open(url, '_blank', 'noopener,noreferrer')",
	} {
		if !strings.Contains(drawerJS, marker) {
			t.Fatalf("integrations drawer JS missing marker %q", marker)
		}
	}
	if strings.Contains(drawerJS, "alert(") {
		t.Fatal("integrations drawer must not introduce alert()")
	}
}

func TestChatFrontend_MobileHeaderControlsRemainTappable(t *testing.T) {
	t.Parallel()

	sharedContent, err := os.ReadFile("shared-components.css")
	if err != nil {
		t.Fatalf("read shared-components.css: %v", err)
	}
	sharedJSContent, err := os.ReadFile("shared.js")
	if err != nil {
		t.Fatalf("read shared.js: %v", err)
	}
	mainContent, err := os.ReadFile(filepath.Join("js", "chat", "main.js"))
	if err != nil {
		t.Fatalf("read chat main JS: %v", err)
	}

	sharedCSS := string(sharedContent)
	for _, marker := range []string{
		".app-header::before,\n.app-header::after,\n.cfg-header::before,\n.cfg-header::after",
		"pointer-events: none;",
		".radial-menu.open",
		".radial-trigger",
		"pointer-events: auto;",
		"touch-action: pan-x;",
		"touch-action: manipulation;",
		"overflow-x: auto;",
	} {
		if !strings.Contains(sharedCSS, marker) {
			t.Fatalf("shared header CSS missing mobile tap safety marker %q", marker)
		}
	}

	sharedJS := string(sharedJSContent)
	for _, marker := range []string{
		"function initHeaderTouchActivation()",
		".app-header button, .app-header a, .cfg-header button, .cfg-header a",
		"const tapSlop = 14",
		"let allowProgrammaticClick = false",
		"suppressTrustedClickUntil",
		"if (allowProgrammaticClick) return;",
		"allowProgrammaticClick = true;",
		"control.click()",
		"stopImmediatePropagation()",
		"initHeaderTouchActivation()",
	} {
		if !strings.Contains(sharedJS, marker) {
			t.Fatalf("shared JS missing mobile header touch activation marker %q", marker)
		}
	}

	mainJS := string(mainContent)
	for _, marker := range []string{
		"function bindHeaderActivation(el, handler)",
		"el.dataset.headerTouchBound = 'true';",
		"el.addEventListener('pointerup', activate);",
		"el.addEventListener('touchend', activate, { passive: false });",
		"bindHeaderActivation(document.getElementById('speaker-toggle'), toggleSpeakerMode);",
		"bindHeaderActivation(btn, toggleChatThemeDropdown);",
		"bindHeaderActivation(opt, () => {",
		"bindHeaderActivation(mobilePersonalityBtn, togglePersonalityDropdown);",
		"bindHeaderActivation(document.getElementById('moodToggle'), toggleMoodPanel);",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("chat main JS missing direct mobile header activation marker %q", marker)
		}
	}
}

func TestChatFrontend_MobilePersonalityButtonKeepsDropdownOpen(t *testing.T) {
	t.Parallel()

	mainContent, err := os.ReadFile(filepath.Join("js", "chat", "main.js"))
	if err != nil {
		t.Fatalf("read chat main JS: %v", err)
	}

	mainJS := string(mainContent)
	for _, marker := range []string{
		"function togglePersonalityDropdown(e) {",
		"e.stopPropagation();",
		"bindHeaderActivation(btn, togglePersonalityDropdown);",
		"bindHeaderActivation(mobilePersonalityBtn, togglePersonalityDropdown);",
		"const clickedMobilePersonality = mobilePersonalityBtn && mobilePersonalityBtn.contains(e.target);",
		"!clickedMobilePersonality",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("chat main JS missing mobile personality dropdown marker %q", marker)
		}
	}
}

func TestChatFrontend_PersonalityPreviewFollowsWholeOption(t *testing.T) {
	t.Parallel()

	historyContent, err := os.ReadFile(filepath.Join("js", "chat", "chat-history.js"))
	if err != nil {
		t.Fatalf("read chat history JS: %v", err)
	}

	historyJS := string(historyContent)
	for _, marker := range []string{
		"function showPersonaPreviewFromOption(event) {",
		"event.target.closest('.personality-option')",
		"dropdown.addEventListener('mouseover', showPersonaPreviewFromOption);",
		"dropdown.addEventListener('mousemove', showPersonaPreviewFromOption);",
		"dropdown.addEventListener('focusin', showPersonaPreviewFromOption);",
	} {
		if !strings.Contains(historyJS, marker) {
			t.Fatalf("chat history JS missing full-option persona preview marker %q", marker)
		}
	}
}

func TestChatFrontend_BlackMatrixEdgeTabsStayAnchoredOnHover(t *testing.T) {
	t.Parallel()

	blackMatrixContent, err := os.ReadFile(filepath.Join("css", "chat-black-matrix.css"))
	if err != nil {
		t.Fatalf("read black matrix CSS: %v", err)
	}

	blackMatrixCSS := string(blackMatrixContent)
	for _, marker := range []string{
		`[data-theme="black-matrix"] .session-edge-tab:hover`,
		`[data-theme="black-matrix"] .session-edge-tab:focus-visible`,
		`[data-theme="black-matrix"] .integrations-edge-tab:hover`,
		`[data-theme="black-matrix"] .integrations-edge-tab:focus-visible`,
		`transform: translateY(-50%);`,
	} {
		if !strings.Contains(blackMatrixCSS, marker) {
			t.Fatalf("black matrix CSS missing anchored edge tab hover marker %q", marker)
		}
	}
}

func TestChatFrontend_HeaderControlsRemainNormalizedAcrossThemes(t *testing.T) {
	t.Parallel()

	indexContent, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	controlsContent, err := os.ReadFile(filepath.Join("css", "chat-header-controls.css"))
	if err != nil {
		t.Fatalf("read header controls CSS: %v", err)
	}

	indexHTML := string(indexContent)
	if !strings.Contains(indexHTML, `/css/chat-header-controls.css`) {
		t.Fatal("index.html must load final chat header controls CSS after theme styles")
	}
	if strings.Index(indexHTML, `/css/chat-header-controls.css`) < strings.Index(indexHTML, `/css/chat-8bit.css`) {
		t.Fatal("chat header controls CSS must load after all chat theme styles")
	}

	themeCSSFiles, err := filepath.Glob(filepath.Join("css", "chat*.css"))
	if err != nil {
		t.Fatalf("glob chat CSS files: %v", err)
	}
	for _, cssFile := range themeCSSFiles {
		cssContent, err := os.ReadFile(cssFile)
		if err != nil {
			t.Fatalf("read %s: %v", cssFile, err)
		}
		cssText := string(cssContent)
		for _, broken := range []string{
			".session-edge-tab\n[data-theme=",
			".session-edge-tab\r\n[data-theme=",
		} {
			if strings.Contains(cssText, broken) {
				t.Fatalf("%s still contains broken header selector fragment %q", cssFile, broken)
			}
		}
	}

	controlsCSS := string(controlsContent)
	for _, marker := range []string{
		".app-header .header-actions :where(.chat-theme-btn, .btn-speaker, .btn-warnings)",
		"width: var(--chat-header-control-size) !important;",
		"border-radius: var(--chat-header-control-radius) !important;",
		".app-header .header-actions .personality-mobile-btn",
		"background: var(--bg-glass);",
		"border: 1px solid var(--border-subtle);",
		"display: inline-grid !important;",
		".app-header .header-actions .mood-panel:not(.open)",
		"pointer-events: none !important;",
		"@media (max-width: 1100px)",
		".app-header .header-actions :where(.chat-theme-dropdown, .personality-dropdown, .mood-panel)",
		"position: fixed !important;",
		"top: 64px !important;",
		"@media (hover: none), (pointer: coarse)",
		"[data-theme=\"threedee\"] .app-header .header-actions :where(.chat-theme-btn, .btn-speaker, .btn-warnings, .select-personality, .mood-toggle, .btn-header, .btn-header-link, #logout-btn, .personality-mobile-btn)",
		"transform-style: flat !important;",
		"transform: none !important;",
		"pointer-events: auto !important;",
		"z-index: 4;",
		".chat-theme-dropdown .chat-theme-option[data-theme]",
		"font-family: inherit !important;",
	} {
		if !strings.Contains(controlsCSS, marker) {
			t.Fatalf("header controls CSS missing normalization marker %q", marker)
		}
	}
}

func TestChatFrontend_ThreeDeeFoldStaysReadable(t *testing.T) {
	t.Parallel()

	foldContent, err := os.ReadFile(filepath.Join("js", "chat", "threedee-fold.js"))
	if err != nil {
		t.Fatalf("read threedee fold JS: %v", err)
	}
	cssContent, err := os.ReadFile(filepath.Join("css", "chat-threedee.css"))
	if err != nil {
		t.Fatalf("read threedee CSS: %v", err)
	}

	foldJS := string(foldContent)
	for _, marker := range []string{
		"clamp(2, 6, aboveBy / 22)",
		"progress * 4",
		"scale(0.995)",
	} {
		if !strings.Contains(foldJS, marker) {
			t.Fatalf("threedee fold JS missing readable fold marker %q", marker)
		}
	}

	threedeeCSS := string(cssContent)
	for _, marker := range []string{
		`[data-theme="threedee"] .msg-row.folding .bubble`,
		`transform: translateZ(1px) rotateX(0.001deg);`,
		`backface-visibility: visible;`,
	} {
		if !strings.Contains(threedeeCSS, marker) {
			t.Fatalf("threedee CSS missing readable fold marker %q", marker)
		}
	}
}

func TestChatFrontend_IntegrationsDrawerI18nKeysExist(t *testing.T) {
	t.Parallel()

	keys := []string{
		"chat.integrations_title",
		"chat.integrations_empty",
		"chat.integrations_open",
		"chat.integrations_loading",
		"chat.integrations_error",
		"chat.aria_integrations",
	}
	files, err := filepath.Glob(filepath.Join("lang", "chat", "*.json"))
	if err != nil {
		t.Fatalf("glob chat lang files: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("expected all chat language files, got %d", len(files))
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var lang map[string]interface{}
		if err := json.Unmarshal(raw, &lang); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if _, ok := lang[key]; !ok {
				t.Fatalf("%s missing i18n key %s", path, key)
			}
		}
	}
}

func TestChatFrontend_8BitThemeRemainsWired(t *testing.T) {
	t.Parallel()

	indexContent, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	chatCSSContent, err := os.ReadFile(filepath.Join("css", "chat.css"))
	if err != nil {
		t.Fatalf("read chat.css: %v", err)
	}
	sharedContent, err := os.ReadFile("shared.js")
	if err != nil {
		t.Fatalf("read shared.js: %v", err)
	}
	mainContent, err := os.ReadFile(filepath.Join("js", "chat", "main.js"))
	if err != nil {
		t.Fatalf("read chat main.js: %v", err)
	}
	iconsContent, err := os.ReadFile(filepath.Join("js", "chat", "ui-icons.js"))
	if err != nil {
		t.Fatalf("read ui-icons.js: %v", err)
	}

	indexHTML := string(indexContent)
	for _, marker := range []string{
		`/css/chat-8bit.css`,
		`data-theme="8bit"`,
		`data-chat-icon="theme-8bit"`,
		`chat.theme_8bit`,
	} {
		if !strings.Contains(indexHTML, marker) {
			t.Fatalf("index.html missing 8Bit theme marker %q", marker)
		}
	}

	for path, content := range map[string]string{
		"shared.js":           string(sharedContent),
		"js/chat/main.js":     string(mainContent),
		"js/chat/ui-icons.js": string(iconsContent),
	} {
		for _, marker := range []string{"8bit", "theme-8bit"} {
			if !strings.Contains(content, marker) {
				t.Fatalf("%s missing 8Bit theme marker %q", path, marker)
			}
		}
	}

	chatCSS := string(chatCSSContent)
	for _, marker := range []string{
		`max-height: min(calc(100dvh - 5rem), 34rem);`,
	} {
		if !strings.Contains(chatCSS, marker) {
			t.Fatalf("css/chat.css missing 8Bit theme picker marker %q", marker)
		}
	}
	for _, marker := range []string{
		`.chat-theme-option[data-theme="8bit"] .chat-theme-option-label`,
		`font-family: 'Press Start 2P'`,
	} {
		if strings.Contains(chatCSS, marker) {
			t.Fatalf("css/chat.css still gives the 8Bit picker option a custom font marker %q", marker)
		}
	}
	for _, marker := range []string{
		`.personality-select-wrapper {`,
		`display: contents;`,
		`#connectionPill`,
	} {
		if !strings.Contains(chatCSS, marker) {
			t.Fatalf("css/chat.css missing compact mobile header marker %q", marker)
		}
	}
	papyrusContent, err := os.ReadFile(filepath.Join("css", "chat-papyrus.css"))
	if err != nil {
		t.Fatalf("read chat-papyrus.css: %v", err)
	}
	if !strings.Contains(string(papyrusContent), `max-height: min(calc(100dvh - 5rem), 34rem);`) {
		t.Fatalf("css/chat-papyrus.css still caps the theme dropdown before viewport space is exhausted")
	}

	for _, assetPath := range []string{
		filepath.Join("css", "chat-8bit.css"),
		filepath.Join("fonts", "press-start-2p-latin-400-normal.woff2"),
		filepath.Join("img", "chat-ui-icons", "theme-8bit.png"),
	} {
		info, err := os.Stat(assetPath)
		if err != nil {
			t.Fatalf("missing 8Bit theme asset %s: %v", assetPath, err)
		}
		if info.Size() == 0 {
			t.Fatalf("8Bit theme asset %s is empty", assetPath)
		}
	}

	bitCSS, err := os.ReadFile(filepath.Join("css", "chat-8bit.css"))
	if err != nil {
		t.Fatalf("read chat-8bit.css: %v", err)
	}
	bitCSSString := string(bitCSS)
	for _, marker := range []string{
		`--bg-logo-size: cover;`,
		`[data-theme="8bit"] #chat-box::after`,
		`background-size: var(--bg-logo-size);`,
		`image-rendering: pixelated;`,
		`[data-theme="8bit"] .app-header #logout-btn`,
		`align-items: center;`,
		`min-width: max-content;`,
		`[data-theme="8bit"] .radial-trigger,`,
		`[data-theme="8bit"] .radial-item-label`,
		`font-size: 0.58rem;`,
		`.chat-theme-dropdown .chat-theme-option[data-theme="8bit"] .chat-theme-option-label`,
		`font-family: inherit !important;`,
	} {
		if !strings.Contains(bitCSSString, marker) {
			t.Fatalf("css/chat-8bit.css missing 8Bit background pixelation marker %q", marker)
		}
	}
	if strings.Contains(bitCSSString, "192px auto") {
		t.Fatal("css/chat-8bit.css still shrinks the 8Bit background image to 192px")
	}
	if strings.Contains(bitCSSString, "`n") {
		t.Fatal("css/chat-8bit.css contains a stray PowerShell newline escape")
	}
}

func TestChatThemeDrawerSelectorsRemainWired(t *testing.T) {
	t.Parallel()

	papyrusPath := filepath.Join("css", "chat-papyrus.css")
	papyrusContent, err := os.ReadFile(papyrusPath)
	if err != nil {
		t.Fatalf("read %s: %v", papyrusPath, err)
	}
	papyrusCSS := string(papyrusContent)
	for _, marker := range []string{
		`[data-theme="papyrus"] .session-drawer,`,
		`[data-theme="papyrus"] .integrations-drawer,`,
		`[data-theme="papyrus"] .integrations-drawer { position: fixed; top: 0; right: 0;`,
		`[data-theme="papyrus"] .integrations-drawer::before`,
		`[data-theme="papyrus"] .integrations-drawer > *`,
		`[data-theme="papyrus"] .integrations-drawer ::-webkit-scrollbar`,
	} {
		if !strings.Contains(papyrusCSS, marker) {
			t.Fatalf("%s missing drawer selector marker %q", papyrusPath, marker)
		}
	}
	for _, broken := range []string{
		"[data-theme=\"papyrus\"] .session-drawer\n[data-theme=\"papyrus\"] .modal-card",
		"[data-theme=\"papyrus\"] .integrations-drawer,\n\n/* Parchment fiber overlay */",
		"[data-theme=\"papyrus\"] .session-drawer ::-webkit-scrollbar {\n[data-theme=\"papyrus\"] .integrations-drawer,",
	} {
		if strings.Contains(papyrusCSS, broken) {
			t.Fatalf("%s still contains broken drawer selector fragment %q", papyrusPath, broken)
		}
	}

	matrixPath := filepath.Join("css", "chat-black-matrix.css")
	matrixContent, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("read %s: %v", matrixPath, err)
	}
	matrixCSS := string(matrixContent)
	for _, marker := range []string{
		`[data-theme="black-matrix"] .session-drawer,`,
		`[data-theme="black-matrix"] .integrations-drawer,`,
		`repeating-linear-gradient(135deg,`,
		`[data-theme="black-matrix"] .integrations-drawer ::-webkit-scrollbar`,
		`[data-theme="black-matrix"] .integrations-edge-tab:hover`,
	} {
		if !strings.Contains(matrixCSS, marker) {
			t.Fatalf("%s missing drawer selector marker %q", matrixPath, marker)
		}
	}
	for _, broken := range []string{
		"[data-theme=\"black-matrix\"] .session-drawer\n[data-theme=\"black-matrix\"] .integrations-drawer",
		"[data-theme=\"black-matrix\"] .session-edge-tab\n[data-theme=\"black-matrix\"] .integrations-edge-tab",
		"[data-theme=\"black-matrix\"] .session-drawer ::-webkit-scrollbar {\n[data-theme=\"black-matrix\"] .integrations-drawer,",
	} {
		if strings.Contains(matrixCSS, broken) {
			t.Fatalf("%s still contains broken drawer selector fragment %q", matrixPath, broken)
		}
	}
}

func TestChatSmartScrollerIgnoresEmptyNonScrollableState(t *testing.T) {
	t.Parallel()

	scrollerPath := filepath.Join("js", "chat", "modules", "smart-scroller.js")
	scrollerContent, err := os.ReadFile(scrollerPath)
	if err != nil {
		t.Fatalf("read %s: %v", scrollerPath, err)
	}

	scrollerJS := string(scrollerContent)
	requiredMarkers := []string{
		"hasScrollableOverflow()",
		"hasRenderedMessages()",
		"const hasOverflow = this.hasScrollableOverflow();",
		"const hasMessages = this.hasRenderedMessages();",
		"this.isUserScrolledUp = hasOverflow && hasMessages && distanceFromBottom > this.scrollThreshold;",
		"if (!this.isUserScrolledUp || !this.hasScrollableOverflow() || !this.hasRenderedMessages())",
		"this.scrollButton.disabled = true;",
	}
	for _, marker := range requiredMarkers {
		if !strings.Contains(scrollerJS, marker) {
			t.Fatalf("%s is missing empty-state scroll guard marker %q", scrollerPath, marker)
		}
	}
}

func TestConfigFrontendVideoDownloadSectionRemainsWired(t *testing.T) {
	t.Parallel()

	mainPath := filepath.Join("js", "config", "main.js")
	modulePath := filepath.Join("cfg", "video_download.js")
	sectionLangPath := filepath.Join("lang", "config", "sections", "en.json")
	moduleLangPath := filepath.Join("lang", "config", "video_download", "en.json")

	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainPath, err)
	}
	moduleContent, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatalf("read %s: %v", modulePath, err)
	}
	sectionLangContent, err := os.ReadFile(sectionLangPath)
	if err != nil {
		t.Fatalf("read %s: %v", sectionLangPath, err)
	}
	moduleLangContent, err := os.ReadFile(moduleLangPath)
	if err != nil {
		t.Fatalf("read %s: %v", moduleLangPath, err)
	}

	checks := map[string]string{
		mainPath:        string(mainContent),
		modulePath:      string(moduleContent),
		sectionLangPath: string(sectionLangContent),
		moduleLangPath:  string(moduleLangContent),
	}
	requiredMarkers := map[string][]string{
		mainPath: {
			"{ key: 'video_download'",
			"video_download: { m: 'video_download', fn: 'renderVideoDownloadSection' }",
			"'send_youtube_video'",
		},
		modulePath: {
			"tools.video_download.mode",
			"tools.video_download.allow_download",
			"tools.video_download.allow_transcribe",
			"tools.send_youtube_video.enabled",
		},
		sectionLangPath: {
			"config.section.video_download.label",
			"config.section.video_download.desc",
		},
		moduleLangPath: {
			"config.video_download.mode_docker",
			"help.video_download.allow_transcribe",
		},
	}
	for path, markers := range requiredMarkers {
		for _, marker := range markers {
			if !strings.Contains(checks[path], marker) {
				t.Fatalf("%s is missing video download config marker %q", path, marker)
			}
		}
	}
}

func TestConfigFrontendSpaceAgentSectionRemainsWired(t *testing.T) {
	t.Parallel()

	mainPath := filepath.Join("js", "config", "main.js")
	modulePath := filepath.Join("cfg", "space_agent.js")

	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainPath, err)
	}
	moduleContent, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatalf("read %s: %v", modulePath, err)
	}

	mainJS := string(mainContent)
	for _, marker := range []string{
		"{ key: 'space_agent'",
		"space_agent: { m: 'space_agent', fn: 'renderSpaceAgentSection' }",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("%s missing Space Agent config marker %q", mainPath, marker)
		}
	}

	moduleJS := string(moduleContent)
	for _, marker := range []string{
		"function renderSpaceAgentSection",
		"space_agent.enabled",
		"space_agent.public_url",
		"space_agent.port",
		"space_agent.https_enabled",
		"space_agent.https_port",
		"space_agent.admin_password",
		"/api/space-agent/status",
		"/api/space-agent/recreate",
	} {
		if !strings.Contains(moduleJS, marker) {
			t.Fatalf("%s missing Space Agent module marker %q", modulePath, marker)
		}
	}
	if strings.Contains(moduleJS, "alert(") {
		t.Fatal("Space Agent config module must not introduce alert()")
	}
}

func TestConfigFrontendSpaceAgentI18nKeysExist(t *testing.T) {
	t.Parallel()

	keys := []string{
		"config.section.space_agent.label",
		"config.section.space_agent.desc",
		"config.space_agent.enabled_label",
		"config.space_agent.public_url_label",
		"config.space_agent.https_enabled_label",
		"config.space_agent.https_port_label",
		"config.space_agent.admin_password_label",
		"config.space_agent.recreate_button",
		"help.space_agent.enabled",
		"help.space_agent.public_url",
		"help.space_agent.https_enabled",
		"help.space_agent.https_port",
		"help.space_agent.admin_password",
	}
	files, err := filepath.Glob(filepath.Join("lang", "config", "sections", "*.json"))
	if err != nil {
		t.Fatalf("glob config section lang files: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("expected all config section language files, got %d", len(files))
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var lang map[string]interface{}
		if err := json.Unmarshal(raw, &lang); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if _, ok := lang[key]; !ok {
				t.Fatalf("%s missing i18n key %s", path, key)
			}
		}
	}
}

func TestConfigFrontendTailscaleSpaceAgentKeysExist(t *testing.T) {
	t.Parallel()

	modulePath := filepath.Join("cfg", "tailscale.js")
	moduleContent, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatalf("read %s: %v", modulePath, err)
	}
	moduleJS := string(moduleContent)
	for _, marker := range []string{
		"tailscale.tsnet.expose_space_agent",
		"tailscale.tsnet.space_agent_hostname",
		"config.tailscale.tsnet_expose_space_agent_label",
		"config.tailscale.tsnet_space_agent_url_label",
	} {
		if !strings.Contains(moduleJS, marker) {
			t.Fatalf("%s missing Tailscale Space Agent marker %q", modulePath, marker)
		}
	}

	keys := []string{
		"config.tailscale.tsnet_expose_space_agent_label",
		"config.tailscale.tsnet_expose_space_agent_hint",
		"config.tailscale.tsnet_space_agent_requires_enabled",
		"config.tailscale.tsnet_space_agent_url_label",
		"config.tailscale.tsnet_space_agent_pending_hint",
		"config.tailscale.tsnet_space_agent_hostname_label",
		"config.tailscale.tsnet_space_agent_hostname_hint",
	}
	files, err := filepath.Glob(filepath.Join("lang", "config", "tailscale", "*.json"))
	if err != nil {
		t.Fatalf("glob tailscale lang files: %v", err)
	}
	if len(files) < 15 {
		t.Fatalf("expected all tailscale language files, got %d", len(files))
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var lang map[string]interface{}
		if err := json.Unmarshal(raw, &lang); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if _, ok := lang[key]; !ok {
				t.Fatalf("%s missing i18n key %s", path, key)
			}
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
		"'send_youtube_video'",
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
	spriteImage, err := png.Decode(bytes.NewReader(spriteContent))
	if err != nil {
		t.Fatalf("decode %s: %v", spritePath, err)
	}
	if got := spriteImage.Bounds().Dx(); got != 1280 {
		t.Fatalf("%s decoded width is %d, want 1280", spritePath, got)
	}
	if got := spriteImage.Bounds().Dy(); got != 1280 {
		t.Fatalf("%s decoded height is %d, want 1280", spritePath, got)
	}
	seenCellHashes := make(map[uint64]int, 100)
	const cellSize = 128
	for slot := 0; slot < 100; slot++ {
		cellX := (slot % 10) * cellSize
		cellY := (slot / 10) * cellSize
		paintedPixels := 0
		hash := fnv.New64a()
		for y := 0; y < cellSize; y++ {
			for x := 0; x < cellSize; x++ {
				r, g, b, a := spriteImage.At(cellX+x, cellY+y).RGBA()
				if a > 0x0fff {
					paintedPixels++
				}
				hash.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)})
			}
		}
		coverage := float64(paintedPixels) / float64(cellSize*cellSize)
		if coverage < 0.04 || coverage > 0.61 {
			t.Fatalf("%s slot %d alpha coverage is %.2f, want freestanding icon coverage between 0.04 and 0.61", spritePath, slot, coverage)
		}
		sum := hash.Sum64()
		if previous, ok := seenCellHashes[sum]; ok {
			t.Fatalf("%s slots %d and %d are exact duplicate icons", spritePath, previous, slot)
		}
		seenCellHashes[sum] = slot
		for _, point := range [][2]int{{0, 0}, {cellSize - 1, 0}, {0, cellSize - 1}, {cellSize - 1, cellSize - 1}} {
			_, _, _, a := spriteImage.At(cellX+point[0], cellY+point[1]).RGBA()
			if a != 0 {
				t.Fatalf("%s slot %d has non-transparent corner pixel, alpha=%d", spritePath, slot, a)
			}
		}
	}

	streamingJS := string(streamingContent)
	for _, marker := range []string{
		"AuraToolIcons.createIcon",
		"setStatusToolIcon(data.detail)",
		"setStatusToolIcon('thinking')",
		"--tool-bubble-drift",
		"--tool-bubble-tilt",
		"const toolIconStack = document.getElementById('tool-icon-stack')",
		"const TOOL_STACK_IDLE_MS = 10000",
		"function pushToolStackIcon(toolName)",
		"toolIconStack.replaceChildren(icon)",
	} {
		if !strings.Contains(streamingJS, marker) {
			t.Fatalf("%s is missing icon wiring marker %q", streamingPath, marker)
		}
	}
	if strings.Contains(streamingJS, "TOOL_STACK_MAX_ICONS") || strings.Contains(streamingJS, "function updateToolStackDepth") {
		t.Fatalf("%s still keeps multiple right-side activity icons instead of replacing them with the latest icon", streamingPath)
	}
	if strings.Contains(streamingJS, "const TOOL_ICONS = {") {
		t.Fatalf("%s still contains the old emoji tool icon map", streamingPath)
	}

	css := string(cssContent)
	for _, marker := range []string{
		".tool-icon-sprite",
		"background-image: url('/img/tool-icons-sprite.png",
		".status-tool-icon",
		"animation: toolBubbleFloat var(--chat-robot-icon-duration)",
		".floating-icon::before",
		".floating-icon::after",
		"@keyframes toolBubbleFloat",
		"@keyframes toolBubbleShell",
		"@keyframes toolBubblePop",
		"scale(1.72)",
		"--chat-robot-icon-duration: 3.2s;",
		".tool-icon-stack",
		".tool-stack-icon",
		".tool-icon-stack.is-fading",
		"--tool-stack-icon-size: clamp(44px, 4.6vw, 56px);",
		"--tool-stack-icon-top-crop: 2px;",
		"clip-path: inset(var(--tool-stack-icon-top-crop) 0 0 0);",
		"width: var(--tool-stack-icon-size);",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("%s is missing icon CSS marker %q", cssPath, marker)
		}
	}
	if strings.Contains(css, "@keyframes floatUp") {
		t.Fatalf("%s still contains the old simple floatUp icon animation", cssPath)
	}

	indexHTML := string(indexContent)
	if !strings.Contains(indexHTML, `/js/chat/tool-icons.js`) {
		t.Fatalf("%s does not load the tool icon catalog", indexPath)
	}
	if !strings.Contains(indexHTML, `id="tool-icon-stack"`) {
		t.Fatalf("%s does not include the right-side tool icon stack", indexPath)
	}
}

func TestChatUIEmojiIconsAreImageAssets(t *testing.T) {
	t.Parallel()

	iconsPath := filepath.Join("js", "chat", "ui-icons.js")
	spritePath := filepath.Join("img", "chat-ui-icons-sprite.png")
	iconDir := filepath.Join("img", "chat-ui-icons")
	cssPath := filepath.Join("css", "chat.css")
	indexPath := "index.html"

	iconsContent, err := os.ReadFile(iconsPath)
	if err != nil {
		t.Fatalf("read %s: %v", iconsPath, err)
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
	requiredMarkers := []string{
		"const CHAT_UI_ICON_DEFINITIONS = [",
		"const CHAT_UI_ICON_STYLE_PRESET = 'ai-generated-activity-3d';",
		"window.AuraChatIcons",
		"stylePreset: CHAT_UI_ICON_STYLE_PRESET",
		"chatUiIconMarkup",
		"hydrate(root = document)",
		"shape: 'send'",
		"shape: 'close'",
		"shape: 'paperclip'",
		"shape: 'microphone'",
		"shape: 'speaker-muted'",
	}
	for _, marker := range requiredMarkers {
		if !strings.Contains(iconsJS, marker) {
			t.Fatalf("%s is missing chat UI icon marker %q", iconsPath, marker)
		}
	}
	if strings.Contains(iconsJS, "sourceSlot: ") {
		t.Fatalf("%s still maps chat UI icons to activity sprite source slots", iconsPath)
	}
	if got := strings.Count(iconsJS, "shape: "); got != 102 {
		t.Fatalf("%s has %d explicit icon shapes, want 102", iconsPath, got)
	}

	requiredIconKeys := []string{
		"robot", "user", "bot", "conversation", "speaker", "speaker-muted", "credit-card",
		"theme-dark", "theme-light", "theme-retro-crt", "theme-cyberwar", "theme-lollipop",
		"theme-dark-sun", "theme-ocean", "theme-sandstorm", "theme-papyrus", "theme-threedee", "theme-black-matrix", "theme-8bit",
		"mood-brain", "mood-curious", "mood-focused", "mood-creative", "mood-analytical",
		"mood-cautious", "mood-playful", "warning", "close", "new-chat", "voice", "clear",
		"attach", "clipboard", "bell", "feedback", "stop", "send", "more", "positive",
		"negative", "angry", "laughing", "crying", "amazed", "document", "edit-document",
		"spreadsheet", "presentation", "csv", "markdown", "text-file", "json", "xml", "web",
		"image", "video", "audio", "pdf", "archive", "pending", "upload", "complete", "error",
		"folder", "retry", "play", "pause", "download", "expand", "target", "in-progress",
		"blocked", "skipped", "info", "chevron-down", "scroll-down",
	}
	for _, key := range requiredIconKeys {
		if !strings.Contains(iconsJS, "key: '"+key+"'") {
			t.Fatalf("%s is missing chat UI icon key %q", iconsPath, key)
		}
		iconPath := filepath.Join(iconDir, key+".png")
		assertPNGIcon(t, iconPath, 128, 128)
	}
	iconFiles, err := filepath.Glob(filepath.Join(iconDir, "*.png"))
	if err != nil {
		t.Fatalf("list chat UI icon files: %v", err)
	}
	if len(iconFiles) != 102 {
		t.Fatalf("%s has %d generated PNG icons, want 102", iconDir, len(iconFiles))
	}
	for _, iconPath := range iconFiles {
		assertPNGIcon(t, iconPath, 128, 128)
	}

	assertPNGIcon(t, spritePath, 1280, 1280)
	assertChatUISpriteCellsHaveVisibleIcons(t, spritePath)

	css := string(cssContent)
	for _, marker := range []string{
		".chat-ui-icon",
		"--chat-ui-icon-url",
		".chat-ui-icon.is-large",
		".chat-theme-option-icon .chat-ui-icon",
		".mood-btn .chat-ui-icon",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("%s is missing chat UI icon CSS marker %q", cssPath, marker)
		}
	}

	indexHTML := string(indexContent)
	iconVersion := extractJSStringConst(t, iconsJS, "ICON_VERSION")
	if !strings.Contains(indexHTML, `/js/chat/ui-icons.js?v=`+iconVersion) {
		t.Fatalf("%s loads ui-icons.js without the current icon cache-bust version %q", indexPath, iconVersion)
	}
	for _, marker := range []string{
		`/js/chat/ui-icons.js`,
		`data-chat-icon="robot"`,
		`data-chat-icon="voice"`,
		`data-chat-icon="send"`,
		`data-chat-icon="warning"`,
		`data-chat-icon="positive"`,
		`data-chat-icon="attach"`,
	} {
		if !strings.Contains(indexHTML, marker) {
			t.Fatalf("%s is missing chat UI icon wiring marker %q", indexPath, marker)
		}
	}

	disallowedStaticGlyphs := []string{"🤖", "💬", "🔇", "💳", "🧠", "🛡️", "🎤", "📎", "📋", "🔔", "🙂", "➤", "⋯", "👍", "👎", "😡", "😂", "😢", "😲"}
	for _, glyph := range disallowedStaticGlyphs {
		if strings.Contains(indexHTML, glyph) {
			t.Fatalf("%s still contains static emoji/icon glyph %q", indexPath, glyph)
		}
	}
}

func TestChatPersonaPreviewAssetsRemainWired(t *testing.T) {
	t.Parallel()

	mainPath := filepath.Join("js", "chat", "chat-history.js")
	cssPath := filepath.Join("css", "chat.css")
	indexPath := "index.html"
	spritePath := filepath.Join("img", "personas", "persona-spritesheet.png")
	personaDir := filepath.Join("img", "personas")

	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainPath, err)
	}
	cssContent, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("read %s: %v", cssPath, err)
	}
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read %s: %v", indexPath, err)
	}

	mainJS := string(mainContent)
	for _, marker := range []string{
		"const PERSONA_PREVIEW_FALLBACK = 'custom';",
		"function personaPreviewKey(name, isCore)",
		"personality-preview-image",
		"/img/personas/${key}.png",
		"personaPreview.hidden = false;",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("%s is missing persona preview JS marker %q", mainPath, marker)
		}
	}

	css := string(cssContent)
	for _, marker := range []string{
		".personality-preview-panel",
		"right: calc(100% + 48px);",
		"width: 256px;",
		"height: 256px;",
		".personality-preview-image",
		"transform: scale(1.185);",
		".personality-preview-panel[hidden]",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("%s is missing persona preview CSS marker %q", cssPath, marker)
		}
	}

	if !strings.Contains(string(indexContent), `id="personality-preview"`) {
		t.Fatalf("%s is missing personality preview container", indexPath)
	}

	assertPNGImageSize(t, spritePath, 1024, 1024)
	for _, name := range []string{
		"evil", "friend", "mcp", "mistress", "neutral", "professional", "psycho",
		"punk", "secretary", "servant", "terminator", "thinker", "custom",
	} {
		assertPNGImageSize(t, filepath.Join(personaDir, name+".png"), 256, 256)
	}
}

func TestChatPersonaIconAssetsRemainWired(t *testing.T) {
	t.Parallel()

	messagesPath := filepath.Join("js", "chat", "chat-messages.js")
	historyPath := filepath.Join("js", "chat", "chat-history.js")
	streamingPath := filepath.Join("js", "chat", "chat-streaming.js")
	cssPath := filepath.Join("css", "chat.css")
	indexPath := "index.html"
	spritePath := filepath.Join("img", "persona-icons", "persona-icons-spritesheet.png")
	iconDir := filepath.Join("img", "persona-icons")

	messagesContent, err := os.ReadFile(messagesPath)
	if err != nil {
		t.Fatalf("read %s: %v", messagesPath, err)
	}
	historyContent, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("read %s: %v", historyPath, err)
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

	messagesJS := string(messagesContent)
	for _, marker := range []string{
		"function personaAvatarMarkup(role)",
		"window._activePersonaIconKey",
		"/img/persona-icons/user.png",
		"/img/persona-icons/${key}.png",
		"persona-avatar-img",
	} {
		if !strings.Contains(messagesJS, marker) {
			t.Fatalf("%s is missing persona bubble avatar JS marker %q", messagesPath, marker)
		}
	}

	streamingJS := string(streamingContent)
	if !strings.Contains(streamingJS, "personaAvatarMarkup('bot')") {
		t.Fatalf("%s should use persona avatars for streamed assistant bubbles", streamingPath)
	}
	if strings.Contains(streamingJS, `\ud83e\udd16`) {
		t.Fatalf("%s still uses the static robot glyph for streamed assistant bubbles", streamingPath)
	}

	historyJS := string(historyContent)
	for _, marker := range []string{
		"function setActivePersonaIconKey(previewKey)",
		"window._activePersonaIconKey = key;",
		"personality-current-icon",
		"persona-option-avatar",
		"/img/persona-icons/${previewKey}.png",
	} {
		if !strings.Contains(historyJS, marker) {
			t.Fatalf("%s is missing persona dropdown icon JS marker %q", historyPath, marker)
		}
	}

	css := string(cssContent)
	for _, marker := range []string{
		".personality-current-icon",
		".persona-option-avatar",
		".persona-avatar-img",
		"width: 24px;",
		"height: 24px;",
		"flex: 0 0 24px;",
		"max-width: 24px;",
		"max-height: 24px;",
		"width: 32px;",
		"height: 32px;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("%s is missing persona icon CSS marker %q", cssPath, marker)
		}
	}
	if !strings.Contains(string(indexContent), `id="personality-current-icon"`) {
		t.Fatalf("%s is missing selected persona icon in the dropdown button", indexPath)
	}

	assertPNGIcon(t, spritePath, 128, 128)
	for _, name := range []string{
		"evil", "friend", "mcp", "mistress", "neutral", "professional", "psycho",
		"punk", "secretary", "servant", "terminator", "thinker", "custom", "user",
	} {
		assertPNGIcon(t, filepath.Join(iconDir, name+".png"), 32, 32)
	}
}

func TestChatLogoIconIsNotCapturedByWordmarkCSS(t *testing.T) {
	t.Parallel()

	cssFiles, err := filepath.Glob(filepath.Join("css", "chat*.css"))
	if err != nil {
		t.Fatalf("list chat css files: %v", err)
	}
	if len(cssFiles) == 0 {
		t.Fatal("expected chat css files")
	}

	for _, cssPath := range cssFiles {
		content, err := os.ReadFile(cssPath)
		if err != nil {
			t.Fatalf("read %s: %v", cssPath, err)
		}
		css := string(content)
		if strings.Contains(css, ".logo span:first-of-type") {
			t.Fatalf("%s still styles the first span in .logo; this captures the logo icon span and hides its image", cssPath)
		}
	}

	chatCSS, err := os.ReadFile(filepath.Join("css", "chat.css"))
	if err != nil {
		t.Fatalf("read chat.css: %v", err)
	}
	if !strings.Contains(string(chatCSS), ".logo-wordmark-accent") {
		t.Fatal("chat.css should style the AURA wordmark via .logo-wordmark-accent")
	}
}

func TestChatComposerToolIconsKeepExplicitImageBox(t *testing.T) {
	t.Parallel()

	cssPath := filepath.Join("css", "chat.css")
	indexPath := "index.html"

	cssContent, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("read %s: %v", cssPath, err)
	}
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read %s: %v", indexPath, err)
	}

	indexHTML := string(indexContent)
	for _, marker := range []string{
		`id="clear-btn"`,
		`data-chat-icon="clear"`,
		`id="upload-btn"`,
		`data-chat-icon="attach"`,
		`id="cheatsheet-picker-btn"`,
		`data-chat-icon="clipboard"`,
		`id="push-btn"`,
		`data-chat-icon="bell"`,
		`id="stop-btn"`,
		`data-chat-icon="stop"`,
	} {
		if !strings.Contains(indexHTML, marker) {
			t.Fatalf("%s is missing composer icon wiring marker %q", indexPath, marker)
		}
	}

	css := string(cssContent)
	if strings.Contains(css, ".composer-tool-btn .tool-icon {\n                width: auto;") ||
		strings.Contains(css, ".composer-panel .composer-tool-btn .tool-icon {\n                font-size: 0.95rem;\n                width: auto;") {
		t.Fatalf("%s lets composer image icons collapse to zero width with width:auto", cssPath)
	}
	for _, marker := range []string{
		".composer-tool-btn .tool-icon {\n            font-size: 0.95rem;\n            width: var(--chat-ui-icon-size);",
		".composer-tool-btn .tool-icon {\n                font-size: 1rem;\n                width: var(--chat-ui-icon-size);",
		".composer-panel .composer-tool-btn .tool-icon {\n                font-size: 0.95rem;\n                width: var(--chat-ui-icon-size);",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("%s is missing explicit composer icon box marker %q", cssPath, marker)
		}
	}
}

func TestChatCheatsheetPickerRequestsOnlyUserSheets(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("js", "chat", "main.js"))
	if err != nil {
		t.Fatalf("read chat main.js: %v", err)
	}
	if !strings.Contains(string(content), "/api/cheatsheets?active=true&created_by=user") {
		t.Fatal("chat cheatsheet picker should request only active user-created cheatsheets")
	}
}

func TestMissionCheatsheetPickerRequestsOnlyUserSheets(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("js", "missions", "main.js"))
	if err != nil {
		t.Fatalf("read missions main.js: %v", err)
	}
	if !strings.Contains(string(content), "/api/cheatsheets?active=true&created_by=user") {
		t.Fatal("mission cheatsheet picker should request only active user-created cheatsheets")
	}
}

func TestGlobalSafeAreaRulesPreserveHeaderFooterSpacing(t *testing.T) {
	t.Parallel()

	enhancementsContent, err := os.ReadFile(filepath.Join("css", "enhancements.css"))
	if err != nil {
		t.Fatalf("read enhancements.css: %v", err)
	}
	sharedContent, err := os.ReadFile("shared-components.css")
	if err != nil {
		t.Fatalf("read shared-components.css: %v", err)
	}
	chatContent, err := os.ReadFile(filepath.Join("css", "chat.css"))
	if err != nil {
		t.Fatalf("read chat.css: %v", err)
	}
	configContent, err := os.ReadFile(filepath.Join("css", "config.css"))
	if err != nil {
		t.Fatalf("read config.css: %v", err)
	}

	enhancementsCSS := string(enhancementsContent)
	for _, staleRule := range []string{
		"padding-top: var(--safe-area-top);",
		"padding-bottom: var(--safe-area-bottom);",
	} {
		if strings.Contains(enhancementsCSS, staleRule) {
			t.Fatalf("enhancements.css still replaces base spacing with raw safe-area rule %q", staleRule)
		}
	}
	for _, marker := range []string{
		".app-header,\n.cfg-header",
		"padding-top: calc(var(--safe-area-header-padding-top, 0.75rem) + var(--safe-area-top));",
		"padding-bottom: calc(var(--safe-area-footer-padding-bottom, 0.75rem) + var(--safe-area-bottom));",
	} {
		if !strings.Contains(enhancementsCSS, marker) {
			t.Fatalf("enhancements.css is missing safe-area spacing marker %q", marker)
		}
	}

	sharedCSS := string(sharedContent)
	for _, marker := range []string{
		"--safe-area-header-padding-top: 0.75rem;",
		"--safe-area-header-padding-top: 0.7rem;",
		"--safe-area-header-padding-top: 0.6rem;",
	} {
		if !strings.Contains(sharedCSS, marker) {
			t.Fatalf("shared-components.css is missing header spacing marker %q", marker)
		}
	}

	chatCSS := string(chatContent)
	for _, marker := range []string{
		"--safe-area-footer-padding-bottom: 0.35rem;",
		"--safe-area-footer-padding-bottom: 0.34rem;",
	} {
		if !strings.Contains(chatCSS, marker) {
			t.Fatalf("chat.css is missing footer spacing marker %q", marker)
		}
	}

	configCSS := string(configContent)
	for _, marker := range []string{
		"--safe-area-footer-padding-bottom: 0.7rem;",
		"--safe-area-footer-padding-bottom: 0.6rem;",
		"--safe-area-footer-padding-bottom: 0.5rem;",
	} {
		if !strings.Contains(configCSS, marker) {
			t.Fatalf("config.css is missing save-bar spacing marker %q", marker)
		}
	}
}

func TestSharedSSEAuthFailureRedirectsImmediately(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile("shared.js")
	if err != nil {
		t.Fatalf("read shared.js: %v", err)
	}
	sharedJS := string(content)

	for _, staleMarker := range []string{
		"_authErrorCount",
		"Only redirect after multiple consecutive auth errors",
		"if (_authErrorCount < 3) return;",
	} {
		if strings.Contains(sharedJS, staleMarker) {
			t.Fatalf("shared.js still delays login redirect on SSE auth failure via marker %q", staleMarker)
		}
	}
	for _, marker := range []string{
		"function _checkAuthAfterSSEError()",
		"fetch('/api/auth/status', { credentials: 'same-origin', cache: 'no-store' })",
		"if (r.status === 401) _redirectToLogin();",
		"_typed['_error'].push(function () {",
		"_checkAuthAfterSSEError();",
	} {
		if !strings.Contains(sharedJS, marker) {
			t.Fatalf("shared.js is missing immediate SSE auth redirect marker %q", marker)
		}
	}
}

func extractJSStringConst(t *testing.T, js, name string) string {
	t.Helper()

	marker := "const " + name + " = '"
	start := strings.Index(js, marker)
	if start < 0 {
		t.Fatalf("missing JS const %s", name)
	}
	start += len(marker)
	end := strings.Index(js[start:], "'")
	if end < 0 {
		t.Fatalf("unterminated JS const %s", name)
	}
	return js[start : start+end]
}

func assertPNGIcon(t *testing.T, path string, wantWidth, wantHeight int) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	const pngHeaderLen = 26
	if len(content) < pngHeaderLen || string(content[:8]) != "\x89PNG\r\n\x1a\n" || string(content[12:16]) != "IHDR" {
		t.Fatalf("%s is not a valid PNG header", path)
	}
	width := binary.BigEndian.Uint32(content[16:20])
	height := binary.BigEndian.Uint32(content[20:24])
	colorType := content[25]
	if int(width) != wantWidth || int(height) != wantHeight {
		t.Fatalf("%s is %dx%d, want %dx%d", path, width, height, wantWidth, wantHeight)
	}
	if colorType != 6 {
		t.Fatalf("%s uses PNG color type %d, want 6 for RGBA alpha", path, colorType)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if got := img.Bounds().Dx(); got != wantWidth {
		t.Fatalf("%s decoded width is %d, want %d", path, got, wantWidth)
	}
	if got := img.Bounds().Dy(); got != wantHeight {
		t.Fatalf("%s decoded height is %d, want %d", path, got, wantHeight)
	}
	for _, point := range [][2]int{{0, 0}, {wantWidth - 1, 0}, {0, wantHeight - 1}, {wantWidth - 1, wantHeight - 1}} {
		_, _, _, a := img.At(point[0], point[1]).RGBA()
		if a != 0 {
			t.Fatalf("%s has non-transparent corner pixel at %v, alpha=%d", path, point, a)
		}
	}
}

func assertPNGImageSize(t *testing.T, path string, wantWidth, wantHeight int) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	const pngHeaderLen = 26
	if len(content) < pngHeaderLen || string(content[:8]) != "\x89PNG\r\n\x1a\n" || string(content[12:16]) != "IHDR" {
		t.Fatalf("%s is not a valid PNG header", path)
	}
	width := binary.BigEndian.Uint32(content[16:20])
	height := binary.BigEndian.Uint32(content[20:24])
	if int(width) != wantWidth || int(height) != wantHeight {
		t.Fatalf("%s is %dx%d, want %dx%d", path, width, height, wantWidth, wantHeight)
	}
	if _, err := png.Decode(bytes.NewReader(content)); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertChatUISpriteCellsHaveVisibleIcons(t *testing.T, path string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}

	const cellSize = 128
	const minVisiblePixels = 400
	for slot := 0; slot < 100; slot++ {
		cellX := (slot % 10) * cellSize
		cellY := (slot / 10) * cellSize
		visiblePixels := 0
		for y := 0; y < cellSize; y++ {
			for x := 0; x < cellSize; x++ {
				_, _, _, a16 := img.At(cellX+x, cellY+y).RGBA()
				a := int(a16 >> 8)
				if a <= 8 {
					continue
				}
				visiblePixels++
			}
		}
		if visiblePixels < minVisiblePixels {
			t.Fatalf("%s slot %d has only %d visible pixels, want at least %d for a generated icon", path, slot, visiblePixels, minVisiblePixels)
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

func TestMediaFrontend_ImageDeleteFlowUsesSharedConfirm(t *testing.T) {
	t.Parallel()

	mediaHTMLPath := "media.html"
	mediaJSPath := filepath.Join("js", "gallery", "main.js")

	mediaHTML, err := os.ReadFile(mediaHTMLPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaHTMLPath, err)
	}
	galleryJS, err := os.ReadFile(mediaJSPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaJSPath, err)
	}

	for _, marker := range []string{
		`<script src="/shared.js`,
		`<script src="/js/gallery/main.js"></script>`,
	} {
		if !strings.Contains(string(mediaHTML), marker) {
			t.Fatalf("%s is missing image delete dependency marker %q", mediaHTMLPath, marker)
		}
	}

	for _, marker := range []string{
		`const confirmed = await showConfirm(t('common.confirm_title'), t('gallery.confirm_delete'))`,
		`let currentLightboxSource = '';`,
		`onclick="handleGalleryCardClick(event, this.dataset.mediaId, this.dataset.source)"`,
		`function handleGalleryCardClick(event, id, source = '')`,
		`function findGalleryImage(id, source)`,
		`async function deleteGalleryImage(id, source = '')`,
		`await deleteGalleryImage(id, source)`,
		`source_db`,
	} {
		if !strings.Contains(string(galleryJS), marker) {
			t.Fatalf("%s is missing shared confirm delete marker %q", mediaJSPath, marker)
		}
	}
}

func TestMissionsFrontend_DeleteDialogUsesExistingConfirmTitleKey(t *testing.T) {
	t.Parallel()

	missionsJSPath := filepath.Join("js", "missions", "main.js")
	missionsJS, err := os.ReadFile(missionsJSPath)
	if err != nil {
		t.Fatalf("read %s: %v", missionsJSPath, err)
	}
	content := string(missionsJS)

	if !strings.Contains(content, `showConfirm(t('common.confirm_title'), t('missions.confirm_delete'`) {
		t.Fatalf("%s should use the existing confirm title translation key for mission delete", missionsJSPath)
	}
	if strings.Contains(content, `t('common.confirm')`) {
		t.Fatalf("%s still references missing translation key common.confirm", missionsJSPath)
	}
}

func TestMediaFrontend_AudioPlayerIconsRemainWired(t *testing.T) {
	t.Parallel()

	mediaHTMLPath := "media.html"
	audioPlayerJSPath := filepath.Join("js", "chat", "audio-player.js")
	iconsJSPath := filepath.Join("js", "chat", "ui-icons.js")
	mediaCSSPath := filepath.Join("css", "media.css")

	mediaHTMLBytes, err := os.ReadFile(mediaHTMLPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaHTMLPath, err)
	}
	audioPlayerJSBytes, err := os.ReadFile(audioPlayerJSPath)
	if err != nil {
		t.Fatalf("read %s: %v", audioPlayerJSPath, err)
	}
	iconsJSBytes, err := os.ReadFile(iconsJSPath)
	if err != nil {
		t.Fatalf("read %s: %v", iconsJSPath, err)
	}
	mediaCSSBytes, err := os.ReadFile(mediaCSSPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaCSSPath, err)
	}

	mediaHTML := string(mediaHTMLBytes)
	iconVersion := extractJSStringConst(t, string(iconsJSBytes), "ICON_VERSION")
	iconScript := `/js/chat/ui-icons.js?v=` + iconVersion
	audioPlayerScript := `/js/chat/audio-player.js`
	iconScriptIndex := strings.Index(mediaHTML, iconScript)
	if iconScriptIndex < 0 {
		t.Fatalf("%s is missing %s before the shared audio player", mediaHTMLPath, iconScript)
	}
	audioPlayerScriptIndex := strings.Index(mediaHTML, audioPlayerScript)
	if audioPlayerScriptIndex < 0 {
		t.Fatalf("%s is missing %s", mediaHTMLPath, audioPlayerScript)
	}
	if iconScriptIndex > audioPlayerScriptIndex {
		t.Fatalf("%s loads %s after %s; icon registry must be available first", mediaHTMLPath, iconScript, audioPlayerScript)
	}

	audioPlayerJS := string(audioPlayerJSBytes)
	for _, marker := range []string{
		`<span class="audio-emoji-icon play-icon" aria-hidden="true">`,
		`<span class="audio-emoji-icon pause-icon is-hidden" aria-hidden="true">`,
		`window.chatUiIconMarkup('download')`,
	} {
		if !strings.Contains(audioPlayerJS, marker) {
			t.Fatalf("%s is missing audio icon marker %q", audioPlayerJSPath, marker)
		}
	}

	mediaCSS := string(mediaCSSBytes)
	for _, marker := range []string{
		`.chat-ui-icon`,
		`--chat-ui-icon-url`,
		`background-image: var(--chat-ui-icon-url)`,
		`.audio-emoji-icon`,
	} {
		if !strings.Contains(mediaCSS, marker) {
			t.Fatalf("%s is missing audio player icon CSS marker %q", mediaCSSPath, marker)
		}
	}
}

func TestMediaFrontend_BulkDeleteSelectionFlowRemainsPresent(t *testing.T) {
	t.Parallel()

	mediaHTMLPath := "media.html"
	mediaJSPath := filepath.Join("js", "media", "main.js")
	galleryJSPath := filepath.Join("js", "gallery", "main.js")
	mediaCSSPath := filepath.Join("css", "media.css")

	mediaHTML, err := os.ReadFile(mediaHTMLPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaHTMLPath, err)
	}
	mediaJS, err := os.ReadFile(mediaJSPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaJSPath, err)
	}
	galleryJS, err := os.ReadFile(galleryJSPath)
	if err != nil {
		t.Fatalf("read %s: %v", galleryJSPath, err)
	}
	mediaCSS, err := os.ReadFile(mediaCSSPath)
	if err != nil {
		t.Fatalf("read %s: %v", mediaCSSPath, err)
	}

	combined := string(mediaHTML) + "\n" + string(mediaJS) + "\n" + string(galleryJS) + "\n" + string(mediaCSS)
	for _, marker := range []string{
		`media-bulk-toolbar`,
		`function toggleMediaSelectionMode()`,
		`function selectVisibleMediaItems()`,
		`async function deleteSelectedMediaItems()`,
		`/api/media/bulk-delete`,
		`/api/image-gallery/bulk-delete`,
		`media-select-check`,
		`function handleMediaGalleryCardClick(event, id, source)`,
	} {
		if !strings.Contains(combined, marker) {
			t.Fatalf("media bulk delete frontend is missing marker %q", marker)
		}
	}
}
