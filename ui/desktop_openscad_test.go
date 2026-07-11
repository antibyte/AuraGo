package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopOpenSCADLazyAssetsRoutingAndWindowRuntime(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'openscad'",
		"'/css/desktop-app-openscad.css'",
		"'/css/stl-viewer.css'",
		"'/js/vendor/three.min.js'",
		"'/js/vendor/STLLoader.min.js'",
		"'/js/vendor/OrbitControls.min.js'",
		"'/js/desktop/apps/openscad-defines.js'",
		"'/js/desktop/apps/openscad-editor.js'",
		"'/js/desktop/apps/openscad.js'",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("desktop app asset registry missing OpenSCAD marker %q", want)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'openscad'",
		"window.OpenSCADApp",
		"window.OpenSCADApp.render",
		"updateWindowContext",
		"readonly: desktopReadonly()",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("desktop routing missing OpenSCAD marker %q", want)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"openscad: 'openscad'",
		"scad: 'openscad'",
		"openscad: 'OS'",
		"err.body = body",
		"OpenSCADApp",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop foundation missing OpenSCAD marker %q", want)
		}
	}

	windows := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"openscad: { width: 1120, height: 720 }",
		"openscad: true",
	} {
		if !strings.Contains(windows, want) {
			t.Fatalf("desktop window runtime missing OpenSCAD marker %q", want)
		}
	}
}

func TestDesktopOpenSCADAppMarkers(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	for _, want := range []string{
		"window.OpenSCADApp = { render, dispose }",
		"/api/openscad/status",
		"/api/openscad/render",
		"/api/desktop/chat/stream",
		"source: 'openscad'",
		"openscad_render",
		"window.AuraSSE.on('virtual_desktop_event'",
		"isOpenSCADResultPayload(data)",
		"isOpenSCADReadOnly",
		"aurago.desktop.openscad.draft",
		"openSCADDraftStorageKey",
		"readOpenSCADDraft",
		"scheduleOpenSCADDraftSave",
		"persistOpenSCADDraft",
		"draftSaveTimer",
		"targetWindowId",
		"window_id: state.windowId",
		"parseDefinesText",
		"OpenSCADDefines.parse",
		"data-oscad-defines",
		"setReadOnly",
		"!state.cancelRequested && renderSerial === state.renderSerial",
		"ensureShell",
		"preview3D",
		"previewStlURL",
		"new AbortController()",
		"payload.source_scad",
		"mesh.rotation.x = -Math.PI / 2",
		"body.status === 'error'",
		"err.body",
		"renderRequestTimeoutMS(state)",
		"setOpenSCADBusy(state, true,",
		"setOpenSCADBusy(state, false)",
		"if (state.renderAbort)",
		"THREE.STLLoader",
		"requestFullscreen",
		"cleanupPreview(state)",
		"cancelAnimationFrame",
		"renderer.dispose()",
		"preview_url",
		"data-oscad-preview-img",
		"bindPreviewLoadError(state, panel",
		"data-oscad-panel",
		"data-oscad-workbench",
		"data-oscad-agent-panel",
		"data-oscad-preview-panel",
		"data-oscad-inspector",
		"data-oscad-source",
		"data-oscad-agent",
		"data-oscad-render",
		"data-oscad-cancel",
		"cancelCurrentOpenSCADWork",
		"data-oscad-fit",
		"data-oscad-background",
		"data-oscad-axes",
		"resetPreviewView(state)",
		"setWindowMenus(state)",
		"clearWindowMenus(state)",
		"state.sourceDirty = true",
		"data-oscad-file-download",
		"saved_path",
		"desktop.openscad.download_hint",
		"parseOpenSCADErrors",
		"mountDefinesPanel",
		"state.editor",
		"state.sourceEditorReady",
		"window.OpenSCADDefines",
		"window.OpenSCADEditor",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("OpenSCAD app missing implementation marker %q", want)
		}
	}
	for _, forbidden := range []string{
		`src="${esc(file.download_url)}"`,
		`data="${esc(file.download_url)}"`,
		`renderSTL(state, panel.querySelector('[data-stl-viewer]'), file.download_url)`,
	} {
		if strings.Contains(app, forbidden) {
			t.Fatalf("OpenSCAD app should use preview_url for inline preview, found %q", forbidden)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-openscad.css")
	for _, want := range []string{
		".openscad-app",
		".oscad-workbench",
		".oscad-agent-panel",
		"flex-direction: column",
		".oscad-defines-panel",
		".oscad-more-exports[open]",
		".oscad-preview-zone",
		".oscad-inspector",
		"grid-template-columns: minmax(260px, 320px) minmax(420px, 1fr) minmax(320px, 380px);",
		".oscad-source",
		".oscad-define-row",
		".oscad-define-slider",
		".oscad-error-line",
		".oscad-warning-line",
		".tok-keyword",
		"white-space: normal",
		"flex-wrap: wrap",
		".oscad-viewport-toolbar",
		".oscad-panel",
		".oscad-file-actions",
		".oscad-dirty",
		".openscad-app.light-preview",
		".openscad-app.busy .oscad-panel::after",
		"@keyframes oscad-spin",
		".oscad-stl",
		".oscad-file-list",
		".oscad-footer",
		"@media (max-width: 860px)",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("OpenSCAD CSS missing marker %q", want)
		}
	}
}

func TestDesktopOpenSCADEventResultClearsBusyBeforeDrawing(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	start := strings.Index(app, "function applyOpenSCADResultEvent")
	if start < 0 {
		t.Fatal("OpenSCAD app missing applyOpenSCADResultEvent")
	}
	body := app[start:]
	drawIndex := strings.Index(body, "draw(state)")
	if drawIndex < 0 {
		t.Fatal("applyOpenSCADResultEvent must redraw after receiving a result")
	}
	clearBusyIndex := strings.Index(body, "setOpenSCADBusy(state, false)")
	if clearBusyIndex < 0 || clearBusyIndex > drawIndex {
		t.Fatalf("applyOpenSCADResultEvent must clear busy before draw; clearBusyIndex=%d drawIndex=%d", clearBusyIndex, drawIndex)
	}
}

func TestDesktopOpenSCADTypedSSEPayloadIsAccepted(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	for _, want := range []string{
		"function isOpenSCADResultPayload",
		"isOpenSCADResultPayload(data)",
		"payload = data",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("OpenSCAD result handler must accept raw typed SSE payload; missing %q", want)
		}
	}
}

func TestDesktopOpenSCADPartialErrorKeepsPreviewFiles(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	for _, want := range []string{
		"function hasOpenSCADResultFiles",
		"hasOpenSCADResultFiles(state.result)",
		"state.activeTab = hasFiles ? 'files'",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("OpenSCAD direct render must keep partial preview files; missing %q", want)
		}
	}
}

func TestDesktopOpenSCADMultiExportRequestsPNGPreviewFirst(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	for _, want := range []string{
		"function renderOpenSCADRequest",
		"const previewFirst = exports.includes('png') && exports.length > 1;",
		"const previewExports = previewFirst ? ['png'] : exports;",
		"const remainingExports = previewFirst ? exports.filter(format => format !== 'png') : [];",
		"await renderOpenSCADRequest(state, previewExports, controller.signal)",
		"renderRemainingOpenSCADExports(state, remainingExports",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("OpenSCAD render must request PNG preview before slow exports; missing %q", want)
		}
	}
	renderStart := strings.Index(app, "async function renderSource")
	if renderStart < 0 {
		t.Fatal("OpenSCAD app missing renderSource")
	}
	renderBody := app[renderStart:]
	backgroundIndex := strings.Index(renderBody, "renderRemainingOpenSCADExports(state, remainingExports")
	if backgroundIndex < 0 {
		t.Fatal("renderSource must continue remaining exports after preview")
	}
	clearBusyIndex := strings.LastIndex(renderBody[:backgroundIndex], "setOpenSCADBusy(state, false)")
	if clearBusyIndex < 0 {
		t.Fatal("renderSource must clear busy before remaining exports can continue")
	}
}

func TestDesktopOpenSCADLogsRenderDiagnostics(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	for _, want := range []string{
		"function logOpenSCAD",
		"function warnOpenSCAD",
		"console.info('[OpenSCAD]'",
		"console.warn('[OpenSCAD]'",
		"logOpenSCAD(state, 'render requested'",
		"logOpenSCAD(state, 'preview render completed'",
		"logOpenSCAD(state, 'background exports started'",
		"logOpenSCAD(state, 'background exports completed'",
		"warnOpenSCAD(state, 'render failed'",
		"warnOpenSCAD(state, 'background exports failed'",
		"openSCADResultSummary",
		"elapsed_ms",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("OpenSCAD app missing render diagnostic marker %q", want)
		}
	}
}

func TestDesktopOpenSCADSaveAllIncludesMergedPreviewAndExportJobs(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	for _, want := range []string{
		"function openSCADResultJobIDs",
		"function openSCADJobIDFromURL",
		"openSCADJobIDFromURL(file.preview_url)",
		"for (const jobID of openSCADResultJobIDs(state.result))",
		"savedResults.reduce((merged, result) => mergeOpenSCADResults(merged, result), null)",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("OpenSCAD save all must save every merged render job; missing %q", want)
		}
	}
}

func TestDesktopOpenSCADIconsExistInBothThemes(t *testing.T) {
	t.Parallel()

	for _, theme := range []string{"papirus", "whitesur"} {
		manifest := readDesktopAssetText(t, "img/"+theme+"/manifest.json")
		for _, want := range []string{`"openscad"`, `"open_scad"`} {
			if !strings.Contains(manifest, want) {
				t.Fatalf("%s theme manifest missing OpenSCAD icon marker %q", theme, want)
			}
		}
		rawDesktopAssetText(t, "img/"+theme+"/icons/openscad.svg")
	}
}

func TestDesktopOpenSCADEditorModule(t *testing.T) {
	t.Parallel()

	editor := readDesktopAssetText(t, "js/desktop/apps/openscad-editor.js")
	for _, want := range []string{
		"window.OpenSCADEditor",
		"OpenSCADEditor = { create: create",
		"codemirror-bundle.esm.js",
		"EditorView",
		"EditorState",
		"Compartment",
		"EditorView.editable",
		"setReadOnly",
		".cm-content .cm-line",
		"oscad-error-line",
		"oscad-warning-line",
		"oscad-error-gutter",
		"createFallback",
		"setErrors",
		"clearErrors",
		"dispose",
		"getValue",
		"setValue",
		"parseOpenSCADErrors",
	} {
		if !strings.Contains(editor, want) {
			t.Fatalf("OpenSCAD editor module missing marker %q", want)
		}
	}
}

func TestDesktopOpenSCADDefinesModule(t *testing.T) {
	t.Parallel()

	defines := readDesktopAssetText(t, "js/desktop/apps/openscad-defines.js")
	for _, want := range []string{
		"window.OpenSCADDefines",
		"OpenSCADDefines = { parse: parse, render: render, toText: toText }",
		"parse",
		"render",
		"toText",
		"oscad-defines-panel",
		"oscad-defines-mode-toggle",
		"desktop.openscad.no_defines",
		"desktop.openscad.editor_text_mode",
		"desktop.openscad.editor_slider_mode",
		"desktop.openscad.define_range_hint",
		"disabled",
		"oscad-define-row",
		"oscad-define-slider",
		"oscad-define-number",
		"oscad-define-text",
		"type=\"range\"",
		"sliderRange",
	} {
		if !strings.Contains(defines, want) {
			t.Fatalf("OpenSCAD defines module missing marker %q", want)
		}
	}
}

func TestDesktopOpenSCADTranslations(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.app_openscad",
		"desktop.store.badge_native",
		"desktop.openscad.title",
		"desktop.openscad.subtitle",
		"desktop.openscad.prompt",
		"desktop.openscad.prompt_placeholder",
		"desktop.openscad.ask_agent",
		"desktop.openscad.render",
		"desktop.openscad.generate_render",
		"desktop.openscad.cancel",
		"desktop.openscad.cancelled",
		"desktop.openscad.exports",
		"desktop.openscad.more_exports",
		"desktop.openscad.defines",
		"desktop.openscad.defines_placeholder",
		"desktop.openscad.no_defines",
		"desktop.openscad.editor_text_mode",
		"desktop.openscad.editor_slider_mode",
		"desktop.openscad.editor_loading",
		"desktop.openscad.error_line",
		"desktop.openscad.error_gutter",
		"desktop.openscad.define_range_hint",
		"desktop.openscad.mode",
		"desktop.openscad.mode_render",
		"desktop.openscad.mode_preview",
		"desktop.openscad.timeout",
		"desktop.openscad.source",
		"desktop.openscad.tab_preview",
		"desktop.openscad.tab_source",
		"desktop.openscad.tab_files",
		"desktop.openscad.tab_log",
		"desktop.openscad.refresh",
		"desktop.openscad.ready",
		"desktop.openscad.download",
		"desktop.openscad.primary_download",
		"desktop.openscad.save_desktop",
		"desktop.openscad.save_all_desktop",
		"desktop.openscad.fullscreen",
		"desktop.openscad.viewport_fit",
		"desktop.openscad.viewport_background",
		"desktop.openscad.viewport_axes",
		"desktop.openscad.render_complete",
		"desktop.openscad.render_required",
		"desktop.openscad.render_failed",
		"desktop.openscad.job",
		"desktop.openscad.duration",
		"desktop.openscad.status_running",
		"desktop.openscad.rendering",
		"desktop.openscad.agent_working",
		"desktop.openscad.saving",
		"desktop.openscad.saved",
		"desktop.openscad.no_files",
		"desktop.openscad.no_log",
		"desktop.openscad.no_preview",
		"desktop.openscad.render_timeout",
		"desktop.openscad.download_hint",
		"desktop.openscad.file_download",
		"desktop.openscad.saved_path",
		"desktop.openscad.open_saved",
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			data, err := Content.ReadFile(filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
			if err != nil {
				t.Fatalf("read %s desktop translations: %v", lang, err)
			}
			var values map[string]string
			if err := json.Unmarshal(data, &values); err != nil {
				t.Fatalf("parse %s desktop translations: %v", lang, err)
			}
			for _, key := range keys {
				if strings.TrimSpace(values[key]) == "" {
					t.Fatalf("%s missing non-empty translation for %s", lang, key)
				}
			}
		})
	}
}
