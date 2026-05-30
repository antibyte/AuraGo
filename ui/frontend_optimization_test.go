package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatTemplateExposesBuildVersionForLazyAssets(t *testing.T) {
	t.Parallel()

	html := readEmbeddedText(t, "index.html")
	for _, want := range []string{
		`"buildVersion":"{{.BuildVersion}}"`,
		`/js/shared/template-data.js?v={{.BuildVersion}}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("chat template missing build version marker %q", want)
		}
	}
}

func TestSharedLazyAssetsAPIIsEmbedded(t *testing.T) {
	t.Parallel()

	loader := readEmbeddedText(t, "js/shared/lazy-assets.js")
	for _, want := range []string{
		"window.AuraLazyAssets",
		"loadScript(src)",
		"loadStyle(href)",
		"loadAll(assets)",
		"window.BUILD_VERSION || 'dev'",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("lazy asset loader missing marker %q", want)
		}
	}

	desktopHTML := readEmbeddedText(t, "desktop.html")
	if !strings.Contains(desktopHTML, `/js/shared/lazy-assets.js?v={{.BuildVersion}}`) {
		t.Fatal("desktop.html must load shared lazy asset loader")
	}
	chatRuntime := readEmbeddedText(t, "js/chat/bundles/chat-runtime.bundle.js")
	if !strings.Contains(chatRuntime, `/* ui/js/shared/lazy-assets.js */`) {
		t.Fatal("chat runtime bundle must include shared lazy asset loader")
	}
}

func TestSharedChatCoreAPIIsEmbedded(t *testing.T) {
	t.Parallel()

	core := readEmbeddedText(t, "js/shared/chat-core.js")
	for _, want := range []string{
		"window.AuraChatCore",
		"personaIconUrl(key)",
		"personaAvatarMarkup(role)",
		"escapeHtml(value)",
		"escapeAttr(value)",
		"isSafeHref(url, allowRelative = true)",
		"sanitizeRenderedHTML(html)",
		"isVideoHref(url)",
		"decorateEmojiGlyphs(root)",
		"filenameFromPath(path, fallback = '')",
		"videoMimeTypeForPath(path)",
		"docFormatIcon(fmt)",
		"parseYouTubeTimeValue(raw)",
		"parseYouTubeVideoLink(raw)",
		"youtubePlayerDedupKey(data)",
		"safeYouTubeEmbedURL(raw, expectedVideoID, expectedStartSeconds)",
		"containsLeakedToolMarkup(text)",
		"stripLeakedToolMarkup(text)",
		"replaceRedactedMarkers(html, label = '[removed]')",
		"isDebugOnlyHistoryMessage(msg)",
		"prepareDisplayContent(text, isUser)",
		"prepareMarkdownContent(text)",
		"applyMarkdownLinkTargets(html)",
		"replaceThinkingPlaceholders(html, thinkingBlocks, renderBlock)",
		"removeSeenMarkdownImages(text, seenImages)",
		"normalizeTimestamp(timestamp)",
		"formatTimestamp(timestamp)",
		"createMarkdownRenderer(options)",
	} {
		if !strings.Contains(core, want) {
			t.Fatalf("shared chat core missing marker %q", want)
		}
	}

	chatHTML := readEmbeddedText(t, "index.html")
	if !strings.Contains(chatHTML, `/js/chat/bundles/chat-runtime.bundle.js?v={{.BuildVersion}}`) {
		t.Fatal("chat page must load bundled shared chat runtime")
	}
	chatRuntime := readEmbeddedText(t, "js/chat/bundles/chat-runtime.bundle.js")
	chatCoreIndex := strings.Index(chatRuntime, `/* ui/js/shared/chat-core.js */`)
	chatMessagesIndex := strings.Index(chatRuntime, `/* ui/js/chat/chat-messages.js */`)
	if chatCoreIndex < 0 {
		t.Fatal("chat runtime bundle must include shared chat core")
	}
	if chatCoreIndex > chatMessagesIndex {
		t.Fatal("chat runtime bundle must load shared chat core before chat message renderer")
	}

	desktopLoader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	desktopCoreIndex := strings.Index(desktopLoader, `/js/shared/chat-core.js`)
	desktopRendererIndex := strings.Index(desktopLoader, `/js/desktop/chat-renderer.js`)
	if desktopCoreIndex < 0 {
		t.Fatal("desktop agent chat must lazy-load shared chat core")
	}
	if desktopRendererIndex < 0 {
		t.Fatal("desktop agent chat missing chat renderer")
	}
	if desktopCoreIndex > desktopRendererIndex {
		t.Fatal("desktop agent chat must load shared chat core before desktop chat renderer")
	}
}

func TestSharedMonolithIsSplitForChatAndDesktop(t *testing.T) {
	t.Parallel()

	core := readEmbeddedText(t, "js/shared/shared-core.js")
	chat := readEmbeddedText(t, "js/shared/shared-chat.js")

	for _, want := range []string{
		"function t(k, p)",
		"function showModal(title, message, isConfirm = false, options = {})",
		"window.AuraAuth = window.AuraAuth || {};",
		"window.AuraSSE = (function ()",
		"function initShared()",
		"if (typeof initTheme === 'function')",
		"if (typeof ensure8BitChatThemeOption === 'function')",
		"if (typeof initThemeToggle === 'function')",
	} {
		if !strings.Contains(core, want) {
			t.Fatalf("shared core missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"const CHAT_THEME_DEFINITIONS",
		"window.AuraChatThemes",
		"function setChatTheme(theme)",
		"function ensure8BitChatThemeOption()",
	} {
		if strings.Contains(core, forbidden) {
			t.Fatalf("shared core must not contain chat-only marker %q", forbidden)
		}
	}
	for _, want := range []string{
		"const CHAT_THEME_DEFINITIONS",
		"window.AuraChatThemes = CHAT_THEME_DEFINITIONS",
		"function setChatTheme(theme)",
		"function ensure8BitChatThemeOption()",
		"function initThemeToggle()",
	} {
		if !strings.Contains(chat, want) {
			t.Fatalf("shared chat extension missing marker %q", want)
		}
	}

	chatHTML := readEmbeddedText(t, "index.html")
	for _, want := range []string{
		`/js/shared/shared-core.js?v={{.BuildVersion}}`,
		`/js/shared/shared-chat.js?v={{.BuildVersion}}`,
	} {
		if !strings.Contains(chatHTML, want) {
			t.Fatalf("chat page missing split shared asset %q", want)
		}
	}
	if strings.Contains(chatHTML, `/shared.js`) {
		t.Fatal("chat page must not load the shared.js monolith after the split")
	}
	if strings.Index(chatHTML, `/js/shared/shared-core.js`) > strings.Index(chatHTML, `/js/shared/shared-chat.js`) {
		t.Fatal("chat page must load shared core before shared chat extension")
	}

	desktopHTML := readEmbeddedText(t, "desktop.html")
	if !strings.Contains(desktopHTML, `/js/shared/shared-core.js?v={{.BuildVersion}}`) {
		t.Fatal("desktop page must load shared core")
	}
	for _, forbidden := range []string{
		`/shared.js`,
		`/js/shared/shared-chat.js`,
	} {
		if strings.Contains(desktopHTML, forbidden) {
			t.Fatalf("desktop page must not load chat-heavy shared asset %q", forbidden)
		}
	}
}

func TestAllTemplatesUseSplitSharedAssets(t *testing.T) {
	t.Parallel()

	pages := []string{
		"cheatsheets.html",
		"config.html",
		"containers.html",
		"dashboard.html",
		"gallery.html",
		"index.html",
		"invasion_control.html",
		"knowledge.html",
		"login.html",
		"media.html",
		"missions_v2.html",
		"plans.html",
		"setup.html",
		"skills.html",
		"truenas.html",
	}

	for _, page := range pages {
		page := page
		t.Run(page, func(t *testing.T) {
			t.Parallel()
			html := readEmbeddedText(t, page)
			if strings.Contains(html, `/shared.js`) {
				t.Fatalf("%s must load split shared assets instead of shared.js", page)
			}
			if !strings.Contains(html, `/js/shared/shared-core.js?v={{.BuildVersion}}`) {
				t.Fatalf("%s must load shared core with BuildVersion cache busting", page)
			}
			if strings.Contains(html, `id="theme-toggle"`) || strings.Contains(html, `id="chat-theme-picker"`) {
				if !strings.Contains(html, `/js/shared/shared-chat.js?v={{.BuildVersion}}`) {
					t.Fatalf("%s must load shared chat/theme extension for theme controls", page)
				}
				if strings.Index(html, `/js/shared/shared-core.js`) > strings.Index(html, `/js/shared/shared-chat.js`) {
					t.Fatalf("%s must load shared core before shared chat/theme extension", page)
				}
			}
		})
	}

	desktopHTML := readEmbeddedText(t, "desktop.html")
	if strings.Contains(desktopHTML, `/shared.js`) {
		t.Fatal("desktop.html must load split shared assets instead of shared.js")
	}
	if !strings.Contains(desktopHTML, `/js/shared/shared-core.js?v={{.BuildVersion}}`) {
		t.Fatal("desktop.html must load shared core with BuildVersion cache busting")
	}
	if strings.Contains(desktopHTML, `/js/shared/shared-chat.js`) {
		t.Fatal("desktop.html must not load the chat/theme shared extension")
	}
}

func TestChatRenderersDelegateToSharedChatCore(t *testing.T) {
	t.Parallel()

	chatJS := readEmbeddedText(t, "js/chat/chat-messages.js")
	for _, want := range []string{
		"window.AuraChatCore.personaIconUrl(key)",
		"window.AuraChatCore.personaAvatarMarkup(role)",
		"window.AuraChatCore.containsLeakedToolMarkup(text)",
		"window.AuraChatCore.stripLeakedToolMarkup(text)",
		"window.AuraChatCore.replaceRedactedMarkers(html, label)",
		"window.AuraChatCore.isDebugOnlyHistoryMessage(msg)",
		"window.AuraChatCore.prepareDisplayContent(text, isUser)",
		"window.AuraChatCore.prepareMarkdownContent(displayContent)",
		"window.AuraChatCore.applyMarkdownLinkTargets(finalHTML)",
		"window.AuraChatCore.replaceThinkingPlaceholders(finalHTML, thinkingBlocks,",
		"window.AuraChatCore.removeSeenMarkdownImages(displayContent, seenSSEImages)",
		"window.AuraChatCore.normalizeTimestamp(timestamp)",
		"window.AuraChatCore.formatTimestamp(timestamp)",
		"window.AuraChatCore.escapeHtml(str)",
		"window.AuraChatCore.escapeAttr(s)",
		"window.AuraChatCore.isSafeHref(url, allowRelative)",
		"window.AuraChatCore.sanitizeRenderedHTML(html)",
		"window.AuraChatCore.isVideoHref(url)",
		"window.AuraChatCore.decorateEmojiGlyphs(root)",
		"window.AuraChatCore.filenameFromPath(path)",
		"window.AuraChatCore.videoMimeTypeForPath(path)",
		"window.AuraChatCore.docFormatIcon(fmt)",
		"window.AuraChatCore.parseYouTubeTimeValue(raw)",
		"window.AuraChatCore.parseYouTubeVideoLink(raw)",
		"window.AuraChatCore.youtubePlayerDedupKey(data)",
		"window.AuraChatCore.safeYouTubeEmbedURL(raw, expectedVideoID, expectedStartSeconds)",
		"window.AuraChatCore.createMarkdownRenderer({",
	} {
		if !strings.Contains(chatJS, want) {
			t.Fatalf("chat renderer must delegate to AuraChatCore marker %q", want)
		}
	}

	desktopChatJS := readEmbeddedText(t, "js/desktop/chat-renderer.js")
	for _, want := range []string{
		"window.AuraChatCore.containsLeakedToolMarkup(text)",
		"window.AuraChatCore.stripLeakedToolMarkup(text)",
		"window.AuraChatCore.prepareDisplayContent(text, false)",
		"window.AuraChatCore.prepareMarkdownContent(displayContent)",
		"window.AuraChatCore.applyMarkdownLinkTargets(finalHTML)",
		"window.AuraChatCore.replaceThinkingPlaceholders(finalHTML, thinkingBlocks,",
		"window.AuraChatCore.removeSeenMarkdownImages(displayContent, this.seenSSEImages)",
		"window.AuraChatCore.escapeHtml(str)",
		"window.AuraChatCore.escapeAttr(s)",
		"window.AuraChatCore.normalizeTimestamp(timestamp)",
		"window.AuraChatCore.formatTimestamp(timestamp)",
		"window.AuraChatCore.createMarkdownRenderer()",
		"window.AuraChatCore.sanitizeRenderedHTML(html)",
		"window.AuraChatCore.videoMimeTypeForPath(videoData.path)",
	} {
		if !strings.Contains(desktopChatJS, want) {
			t.Fatalf("desktop chat renderer must delegate to AuraChatCore marker %q", want)
		}
	}
	chatCoreJS := readEmbeddedText(t, "js/shared/chat-core.js")
	for _, want := range []string{
		"const CHAT_SANITIZER_ALLOWED_TAGS = new Set([",
		"const CHAT_SANITIZER_ALLOWED_ATTRS = new Set([",
		"node.setAttribute('sandbox', 'allow-scripts allow-same-origin')",
		"keepBlobMedia",
	} {
		if !strings.Contains(chatCoreJS, want) {
			t.Fatalf("shared chat core sanitizer missing central security marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"return String(str)\n                .replace(/&/g, '&amp;')",
		"const allowed = new Set([",
		"const allowedAttrs = new Set([",
		"function decorateEmojiGlyphs(root) {\n    if (window.AuraChatCore",
	} {
		if strings.Contains(desktopChatJS, forbidden) {
			t.Fatalf("desktop chat renderer must not keep local fallback implementation marker %q", forbidden)
		}
	}
	for _, forbidden := range []string{
		"const emojiGlyphPattern =",
		"function decorateEmojiGlyphs(root) {\n    if (window.AuraChatCore",
	} {
		if strings.Contains(chatJS, forbidden) {
			t.Fatalf("chat message renderer must not keep local fallback implementation marker %q", forbidden)
		}
	}
}

func TestChatMediaHelpersDelegateToSharedChatCore(t *testing.T) {
	t.Parallel()

	stlViewerJS := readEmbeddedText(t, "js/chat/stl-viewer.js")
	for _, want := range []string{
		"window.AuraChatCore.filenameFromPath(path, 'model.stl')",
	} {
		if !strings.Contains(stlViewerJS, want) {
			t.Fatalf("stl viewer must delegate to AuraChatCore marker %q", want)
		}
	}
}

func TestChatInitialLoadDefersThemeEffectsAndThreeJS(t *testing.T) {
	t.Parallel()

	html := readEmbeddedText(t, "index.html")
	for _, forbidden := range []string{
		`src="/js/vendor/three.min.js"`,
		`src="/js/vendor/GLTFLoader.min.js`,
		`src="/js/vendor/DRACOLoader.min.js`,
		`src="/js/vendor/STLLoader.min.js"`,
		`src="/js/vendor/OrbitControls.min.js"`,
		`src="/js/chat/cyberwar-shader.js`,
		`src="/js/chat/dark-sun-shader.js`,
		`src="/js/chat/ocean-shader.js`,
		`src="/js/chat/sandstorm-particles.js`,
		`src="/js/chat/threedee-shader.js`,
		`src="/js/chat/threedee-fold.js`,
		`src="/js/chat/black-matrix-shader.js`,
		`src="/js/crt-persistence-shader.js`,
		`src="/js/crt-shader.js`,
		`src="/js/chat/8bit-pixelate.js`,
	} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("chat page should lazy-load heavy asset instead of loading %q upfront", forbidden)
		}
	}

	for _, want := range []string{
		`/js/chat/bundles/chat-runtime.bundle.js?v={{.BuildVersion}}`,
		`/css/chat.bundle.css?v={{.BuildVersion}}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("chat page missing optimized asset %q", want)
		}
	}
}

func TestChatInitialLoadUsesBundledRuntimeAssets(t *testing.T) {
	t.Parallel()

	html := readEmbeddedText(t, "index.html")
	for _, want := range []string{
		`/js/shared/template-data.js?v={{.BuildVersion}}`,
		`/js/chat/bundles/chat-vendor.bundle.js?v={{.BuildVersion}}`,
		`/js/chat/bundles/chat-runtime.bundle.js?v={{.BuildVersion}}`,
		`/css/chat.bundle.css?v={{.BuildVersion}}`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("chat page missing bundled initial-load asset %q", want)
		}
	}
	for _, forbidden := range []string{
		`/js/vendor/markdown-it.min.js`,
		`/js/vendor/highlight.min.js`,
		`/js/shared/render-markdown.js`,
		`/js/shared/lazy-assets.js`,
		`/js/shared/chat-core.js`,
		`/js/chat/theme-effects.js`,
		`/js/chat/main.js`,
		`/js/chat/modules/voice-recorder.js`,
		`/js/chat/modules/speech-to-text.js`,
		`/js/chat/modules/drag-drop.js`,
		`/js/chat/modules/mermaid-loader.js`,
	} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("chat page should use bundles instead of direct initial-load asset %q", forbidden)
		}
	}
}

func TestChatRuntimeBundleIsBuiltFromSplitMainSources(t *testing.T) {
	t.Parallel()

	buildScript := readEmbeddedRepoText(t, "../scripts/build-ui-bundles.js")
	bundle := readEmbeddedText(t, "js/chat/bundles/chat-runtime.bundle.js")
	compatMain := readEmbeddedText(t, "js/chat/main.js")
	for _, path := range []string{
		"ui/js/chat/main/state-dom.js",
		"ui/js/chat/main/i18n-ui-chrome.js",
		"ui/js/chat/main/feedback-audio-plan.js",
		"ui/js/chat/main/composer-uploads.js",
		"ui/js/chat/main/network-submit.js",
		"ui/js/chat/main/bootstrap.js",
	} {
		if !strings.Contains(buildScript, path) {
			t.Fatalf("UI build script must include split chat main source %q", path)
		}
		if !strings.Contains(bundle, "/* "+path+" */") {
			t.Fatalf("chat runtime bundle missing split source marker %q", path)
		}
		if !strings.Contains(compatMain, "/* "+path+" */") {
			t.Fatalf("chat compatibility main bundle missing split source marker %q", path)
		}
	}
}

func TestChatThemeEffectsRegistryLoadsHeavyAssets(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/chat/theme-effects.js")
	for _, want := range []string{
		"window.AuraChatThemeEffects",
		"ensure(theme)",
		"'threedee'",
		"/js/vendor/three.min.js",
		"/js/vendor/GLTFLoader.min.js",
		"/js/vendor/DRACOLoader.min.js",
		"/js/chat/threedee-shader.js",
		"/js/chat/threedee-fold.js",
		"'sandstorm'",
		"/js/chat/sandstorm-particles.js",
		"'retro-crt'",
		"/js/crt-shader.js",
		"'8bit'",
		"/js/chat/8bit-pixelate.js",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("chat theme effects registry missing marker %q", want)
		}
	}
}

func TestLegacyDuplicateChatModulesAreNotEmbedded(t *testing.T) {
	t.Parallel()

	for _, legacy := range []string{
		"js/chat/drag-drop.js",
		"js/chat/voice-recorder.js",
	} {
		if _, err := Content.ReadFile(legacy); err == nil {
			t.Fatalf("legacy duplicate chat module %s should be removed", legacy)
		}
	}
}

func TestLegacyChatThemeStylesheetsAreNotEmbedded(t *testing.T) {
	t.Parallel()

	for _, legacy := range []string{
		"css/chat-8bit.css",
		"css/chat-black-matrix.css",
		"css/chat-cyberwar.css",
		"css/chat-dark-sun.css",
		"css/chat-lollipop.css",
		"css/chat-ocean.css",
		"css/chat-papyrus.css",
		"css/chat-sandstorm.css",
		"css/chat-threedee.css",
	} {
		if _, err := Content.ReadFile(legacy); err == nil {
			t.Fatalf("legacy chat theme stylesheet %s should be consolidated into chat-themes.css", legacy)
		}
	}
}

func TestDesktopInitialLoadDefersAppAssets(t *testing.T) {
	t.Parallel()

	html := readEmbeddedText(t, "desktop.html")
	for _, forbidden := range []string{
		`src="/js/vendor/xterm.min.js"`,
		`src="/js/vendor/xterm-addon-fit.min.js"`,
		`src="/js/vendor/novnc.min.js"`,
		`src="/js/vendor/pdf.min.js"`,
		`src="/js/vendor/three.min.js"`,
		`src="/js/vendor/STLLoader.min.js"`,
		`src="/js/vendor/OrbitControls.min.js"`,
		`src="/js/vendor/quill.js"`,
		`src="/chart.min.js"`,
		`src="/js/desktop/apps/code-studio.js`,
		`src="/js/desktop/apps/writer.js`,
		`src="/js/desktop/apps/sheets.js`,
		`src="/js/desktop/file-manager.js`,
		`src="/js/desktop/chat-renderer.js`,
		`src="/js/desktop/apps/radio.js`,
		`src="/js/desktop/apps/looper.js`,
		`src="/js/desktop/apps/viewer.js`,
		`href="/css/radio.css`,
		`href="/css/camera.css`,
		`href="/css/zipper.css`,
		`href="/css/code-studio.css`,
		`href="/css/stl-viewer.css`,
		`href="/css/pixel.css`,
		`href="/css/galaxa-deluxe.css`,
		`href="/css/quill.snow.css`,
		`href="/css/xterm.css`,
	} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("desktop page should lazy-load app asset instead of loading %q upfront", forbidden)
		}
	}
}

func TestDesktopInitialLoadUsesShellCSSBundleOnly(t *testing.T) {
	t.Parallel()

	html := readEmbeddedText(t, "desktop.html")
	if !strings.Contains(html, `/css/desktop-shell.bundle.css?v={{.BuildVersion}}`) {
		t.Fatal("desktop page must load the generated shell CSS bundle")
	}
	for _, forbidden := range []string{
		`/css/desktop.css`,
		`desktop-apps.css`,
		`desktop-app-file-manager.css`,
		`desktop-app-office.css`,
		`desktop-app-chat.css`,
		`desktop-app-looper.css`,
	} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("desktop initial page must not load app CSS asset %q", forbidden)
		}
	}

	shellBundle := readEmbeddedText(t, "css/desktop-shell.bundle.css")
	for _, want := range []string{
		"/* ui/css/desktop-base.css */",
		"/* ui/css/desktop-taskbar.css */",
		"/* ui/css/desktop-shell-overrides.css */",
	} {
		if !strings.Contains(shellBundle, want) {
			t.Fatalf("desktop shell CSS bundle missing source marker %q", want)
		}
	}
	if strings.Contains(shellBundle, ".file-manager") || strings.Contains(shellBundle, ".vd-looper") {
		t.Fatal("desktop shell CSS bundle must not contain app-specific selectors")
	}
}

func TestDesktopShellBundleSizesGeneratedAndStoreAppFrames(t *testing.T) {
	t.Parallel()

	shellBundle := readEmbeddedText(t, "css/desktop-shell.bundle.css")
	for _, want := range []string{
		".vd-generated-frame",
		".vd-store-app-frame",
		"width: 100%;",
		"height: 100%;",
		"border: 0;",
	} {
		if !strings.Contains(shellBundle, want) {
			t.Fatalf("desktop shell CSS bundle must size generated/store app frames, missing %q", want)
		}
	}
}

func TestDesktopModuleLoaderUsesBuiltBundlesWithoutEval(t *testing.T) {
	t.Parallel()

	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	for _, forbidden := range []string{
		"(0, eval)",
		"response.text()",
		"Promise.all(parts.map(fetchScriptPart))",
	} {
		if strings.Contains(loader, forbidden) {
			t.Fatalf("desktop module loader must not use eval bundle path marker %q", forbidden)
		}
	}
	for _, want := range []string{
		"DESKTOP_APP_ASSETS",
		"loadAppScript(appId)",
		"loadAppAssets(appId)",
		"loadScript(src)",
		"loadStyle(href)",
		"loadBundle(label, src)",
		"/js/desktop/bundles/main.bundle.js",
		"/js/desktop/bundles/file-manager.bundle.js",
		"/js/desktop/bundles/code-studio.bundle.js",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("desktop module loader missing no-eval marker %q", want)
		}
	}
}

func TestDesktopAppAssetsRegistryCoversHeavyApps(t *testing.T) {
	t.Parallel()

	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'agent-chat'",
		"'files'",
		"'code-studio'",
		"'writer'",
		"'sheets'",
		"'radio'",
		"'looper'",
		"'viewer'",
		"'camera'",
		"'zipper'",
		"'pixel'",
		"'galaxa-deluxe'",
		"'viewer-3d'",
		"'system-info'",
		"/js/vendor/xterm.min.js",
		"/js/vendor/quill.js",
		"/js/vendor/pdf.min.js",
		"/js/vendor/three.min.js",
		"/chart.min.js",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("desktop app asset registry missing marker %q", want)
		}
	}
}

func TestDesktopAppAssetsRegistryCoversLazyCSSForAllBuiltinApps(t *testing.T) {
	t.Parallel()

	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'/css/desktop-app-common.css'",
		"'/css/desktop-app-file-manager.css'",
		"'/css/desktop-app-office.css'",
		"'/css/desktop-app-settings.css'",
		"'/css/desktop-app-calculator.css'",
		"'/css/desktop-app-planning.css'",
		"'/css/desktop-app-gallery.css'",
		"'/css/desktop-app-chat.css'",
		"'/css/desktop-app-quick-connect.css'",
		"'/css/desktop-app-launchpad.css'",
		"'/css/desktop-app-system-info.css'",
		"'/css/desktop-app-looper.css'",
		"'/css/desktop-app-viewer.css'",
		"'/css/desktop-app-software-store.css'",
		"'settings'",
		"'calculator'",
		"'todo'",
		"'calendar'",
		"'gallery'",
		"'music-player'",
		"'launchpad'",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("desktop app asset registry missing lazy CSS marker %q", want)
		}
	}
}

func TestConfigFormBuilderAPIIsEmbeddedAndLoadedBeforeConfigMain(t *testing.T) {
	t.Parallel()

	html := readEmbeddedText(t, "config.html")
	builderRef := `/cfg/form-builder.js?v=20`
	mainRef := `/js/config/main.js?v=20`
	if !strings.Contains(html, builderRef) {
		t.Fatal("config page must load the shared config form builder")
	}
	if strings.Index(html, builderRef) > strings.Index(html, mainRef) {
		t.Fatal("config form builder must load before config main.js")
	}

	builder := readEmbeddedText(t, "cfg/form-builder.js")
	for _, want := range []string{
		"window.AuraConfigForm",
		"section(",
		"field(",
		"toggle(",
		"select(",
		"textarea(",
		"password(",
		"number(",
		"note(",
		"actions(",
		"renderSpec(",
	} {
		if !strings.Contains(builder, want) {
			t.Fatalf("config form builder missing public API marker %q", want)
		}
	}

	grafana := readEmbeddedText(t, "cfg/grafana.js")
	if !strings.Contains(grafana, "window.AuraConfigForm.renderSpec") {
		t.Fatal("at least one field-list config section must use AuraConfigForm.renderSpec")
	}
}

func TestChatAndDesktopAvoidExecutableInlineScriptsForCSPPrep(t *testing.T) {
	t.Parallel()

	for _, page := range []string{"index.html", "desktop.html"} {
		html := readEmbeddedText(t, page)
		for _, forbidden := range []string{
			"<script>",
			"onclick=",
			"ontouchend=",
		} {
			if strings.Contains(html, forbidden) {
				t.Fatalf("%s must not contain executable inline marker %q", page, forbidden)
			}
		}
		if !strings.Contains(html, `type="application/json" id="aurago-template-data"`) {
			t.Fatalf("%s must expose template data through a non-executable JSON script", page)
		}
	}

	sharedCore := readEmbeddedText(t, "js/shared/shared-core.js")
	for _, forbidden := range []string{
		`onclick="performLogout()`,
		`ontouchend="event.preventDefault(); performLogout()`,
	} {
		if strings.Contains(sharedCore, forbidden) {
			t.Fatalf("shared radial logout markup must not contain inline handler %q", forbidden)
		}
	}
}

func readEmbeddedRepoText(t *testing.T, rel string) string {
	t.Helper()
	if strings.HasPrefix(rel, "../") {
		rel = strings.TrimPrefix(rel, "../")
	}
	data, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read repo file %s: %v", rel, err)
	}
	return string(data)
}
