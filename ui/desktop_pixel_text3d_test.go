package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const (
	pixelText3DLibScript = "js/desktop/apps/pixel-lib-text3d.js"
	pixelText3DGLScript  = "js/desktop/apps/pixel-text3d-gl.js"
	pixelText3DScript    = "js/desktop/apps/pixel-text3d.js"
	pixelText3DUIScript  = "js/desktop/apps/pixel-text3d-ui.js"
	pixelText3DStyles    = "css/pixel-text3d.css"
)

func TestPixelText3DModulesWiringAndLineBudget(t *testing.T) {
	t.Parallel()

	lib := normalizePixelAsset(readDesktopAssetText(t, pixelText3DLibScript))
	for _, want := range []string{
		"Lib.registerOp('text3d.render'",
		"function computeSDF(",
		"function generateTexture(",
		"function rotationMatrix(",
		"function inverseWarp(",
		"function projectBox(",
		"function renderCPU(",
		"Lib.text3d = {",
	} {
		if !strings.Contains(lib, want) {
			t.Fatalf("%s missing %q", pixelText3DLibScript, want)
		}
	}
	for _, forbidden := range []string{"document.", "window.", "localStorage", "createElement", "Math.random"} {
		if strings.Contains(lib, forbidden) {
			t.Fatalf("%s must stay DOM-free and deterministic, found %q", pixelText3DLibScript, forbidden)
		}
	}

	gl := normalizePixelAsset(readDesktopAssetText(t, pixelText3DGLScript))
	for _, want := range []string{"Pixel.text3dGL = { createRenderer }", "webgl2", "webglcontextlost", "inverseWarp", "shadeRay"} {
		if !strings.Contains(gl, want) {
			t.Fatalf("%s missing %q", pixelText3DGLScript, want)
		}
	}

	mod := normalizePixelAsset(readDesktopAssetText(t, pixelText3DScript))
	for _, want := range []string{
		"Pixel.registerModule('text3d'",
		"id: 'text-3d'",
		"group: 'type'",
		"shortcut: 'Shift+T'",
		"Pixel.registerMenuItems('generate'",
		"Pixel.registerCommand({",
		"commitLayerPixels(layer, out",
		"pushHistory('text-3d')",
		"stripTransientFromHistory",
		"r.disposeCore =",
	} {
		if !strings.Contains(mod, want) {
			t.Fatalf("%s missing %q", pixelText3DScript, want)
		}
	}

	panel := normalizePixelAsset(readDesktopAssetText(t, pixelText3DUIScript))
	for _, want := range []string{
		"Pixel.registerPanel({",
		"id: 'text3d'",
		"dock: 'inspector'",
		"data-t3d-trackball",
		"data-t3d-warp",
		"data-t3d-tex",
		"data-t3d-font",
		"data-t3d-apply",
	} {
		if !strings.Contains(panel, want) {
			t.Fatalf("%s missing %q", pixelText3DUIScript, want)
		}
	}

	for _, script := range []string{pixelText3DLibScript, pixelText3DGLScript, pixelText3DScript, pixelText3DUIScript} {
		src := normalizePixelAsset(readDesktopAssetText(t, script))
		if lines := strings.Count(src, "\n"); lines >= 1100 {
			t.Fatalf("%s has %d lines, want < 1100", script, lines)
		}
		for _, forbidden := range []string{"http://", "https://", "eval(", "new Function("} {
			if strings.Contains(src, forbidden) {
				t.Fatalf("%s must not contain %q", script, forbidden)
			}
		}
		if !slicesContains(pixelAppScriptPaths, script) {
			t.Fatalf("%s is missing from pixelAppScriptPaths", script)
		}
	}

	requirePixelBundleKeys(t, pixelText3DScript, mod, 8)
	requirePixelBundleKeys(t, pixelText3DUIScript, panel, 20)

	worker := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-worker.js"))
	if !strings.Contains(worker, "'pixel-lib-text3d.js'") {
		t.Fatal("pixel-worker.js must import pixel-lib-text3d.js")
	}
}

func TestPixelText3DCSSIsPrefixedAndFontsExist(t *testing.T) {
	t.Parallel()

	css := normalizePixelAsset(readDesktopAssetText(t, pixelText3DStyles))
	if !slicesContains(pixelAppStylePaths, pixelText3DStyles) {
		t.Fatalf("%s is missing from pixelAppStylePaths", pixelText3DStyles)
	}
	if !strings.Contains(css, "--vd-mat-") {
		t.Fatalf("%s must adopt --vd-mat-* desktop material tokens", pixelText3DStyles)
	}
	if !pixelCSSRuleContains(css, ".pixel-t3d-panel [hidden]", "display: none") {
		t.Fatalf("%s must define explicit [hidden] display:none override", pixelText3DStyles)
	}
	for _, selector := range []string{".pixel-t3d-panel", ".pixel-t3d-trackball", ".pixel-t3d-warp-grid", ".pixel-t3d-tex-grid", ".pixel-t3d-bar"} {
		if !strings.Contains(css, selector) {
			t.Fatalf("%s missing style for %q", pixelText3DStyles, selector)
		}
	}
	stripped := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	stripped = regexp.MustCompile(`(?s)@font-face\s*\{[^}]*\}`).ReplaceAllString(stripped, "")
	stripped = regexp.MustCompile(`(?s)\{[^}]*\}`).ReplaceAllString(stripped, "{}")
	classRe := regexp.MustCompile(`\.([A-Za-z_][\w-]*)`)
	for _, m := range classRe.FindAllStringSubmatch(stripped, -1) {
		name := m[1]
		if strings.HasPrefix(name, "cc-") || strings.HasPrefix(name, "ad-") {
			t.Fatalf("%s uses blocked class prefix %q", pixelText3DStyles, name)
		}
		if !strings.HasPrefix(name, "pixel-") {
			t.Fatalf("%s class %q must carry the pixel- prefix", pixelText3DStyles, name)
		}
	}
	urlRe := regexp.MustCompile(`url\(['"]?(/fonts/[^'")]+)['"]?\)`)
	fontMatches := urlRe.FindAllStringSubmatch(css, -1)
	if len(fontMatches) < 5 {
		t.Fatalf("%s should declare local @font-face entries for bundled fonts", pixelText3DStyles)
	}
	for _, m := range fontMatches {
		asset := strings.TrimPrefix(m[1], "/")
		if _, err := Content.ReadFile(asset); err != nil {
			t.Fatalf("%s references missing local font asset %q: %v", pixelText3DStyles, asset, err)
		}
	}
}

func TestPixelText3DLocalesCompleteInAll16Languages(t *testing.T) {
	t.Parallel()

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	en := map[string]string{}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &en); err != nil {
		t.Fatalf("parse en.json: %v", err)
	}
	var keys []string
	for k := range en {
		if strings.HasPrefix(k, "pixel.text3d_") {
			keys = append(keys, k)
		}
	}
	if len(keys) < 90 {
		t.Fatalf("expected >= 90 pixel.text3d_* keys in en.json, got %d", len(keys))
	}
	for _, lang := range langs {
		values := map[string]string{}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
			t.Fatalf("parse %s.json: %v", lang, err)
		}
		sameAsEn := 0
		for _, k := range keys {
			v := strings.TrimSpace(values[k])
			if v == "" {
				t.Fatalf("%s.json missing %q", lang, k)
			}
			if lang != "en" && v == strings.TrimSpace(en[k]) {
				sameAsEn++
			}
		}
		// Allow short international terms (Auto, Gold, Chrome, Neon, Position X/Y) while ensuring genuine translation.
		if lang != "en" && sameAsEn > 18 {
			t.Fatalf("%s.json has %d untranslated pixel.text3d_* strings matching English", lang, sameAsEn)
		}
		if lang == "de" {
			if !strings.Contains(values["pixel.text3d_warp_bulge"], "ö") || !strings.Contains(values["pixel.text3d_warp_arch"], "ö") {
				t.Fatalf("German pixel.text3d_* labels must use real umlauts, got %q / %q", values["pixel.text3d_warp_bulge"], values["pixel.text3d_warp_arch"])
			}
			if strings.Contains(values["pixel.text3d_empty"], "Sie ") || strings.Contains(values["pixel.text3d_readonly"], "Sie ") {
				t.Fatalf("German pixel.text3d_* messages must use informal Du, not Sie")
			}
		}
	}
}

func TestDesktopPixelText3DBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	var styleLinks strings.Builder
	styleLinks.WriteString(`<link rel="stylesheet" href="/fonts/fonts.css">`)
	for _, css := range append([]string{"shared-variables.css", "shared-utilities.css", "shared-components.css", "css/desktop-shell.bundle.css", "css/desktop-app-common.css"}, pixelAppStylePaths...) {
		styleLinks.WriteString(`<link rel="stylesheet" href="/` + css + `">`)
	}
	var scriptTags strings.Builder
	for _, js := range pixelAppScriptPaths {
		scriptTags.WriteString(`<script src="/` + js + `"></script>`)
	}
	deStrings := readDesktopAssetText(t, "lang/desktop/de.json")

	html := `<!doctype html><html lang="de"><head><meta charset="utf-8">` + styleLinks.String() + `
	<style>html,body{margin:0;padding:0;width:100%;height:100%;overflow:hidden;}#host{position:absolute;inset:0;}</style>
	</head><body class="desktop-body" data-theme="standard"><div id="host"></div>` + scriptTags.String() + `
	<script>
	window.__errors = [];
	window.addEventListener('error', e => window.__errors.push('err: ' + (e.message || e)));
	window.addEventListener('unhandledrejection', e => window.__errors.push('rej: ' + (e.reason && e.reason.message || e.reason)));
	window.__strings = ` + deStrings + `;
	window.__rt = null;
	AuraPixelApp.registerModule('browser-capture', r => { window.__rt = r; });
	PixelApp.render(document.getElementById('host'), 'p3d', {
		t: (k, v) => {
			let s = window.__strings[k] || k;
			if (v && typeof v === 'object') Object.keys(v).forEach(n => { s = s.split('{{'+n+'}}').join(v[n]).split('{'+n+'}').join(v[n]); });
			return s;
		},
		esc: v => String(v).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])),
		api: async () => ({}),
		iconMarkup: () => '',
		notify: () => {},
		setWindowMenus: () => {},
		readonly: false
	});
	// Paint a rich scenic sunset photo onto the background layer so 3D text placement over an image is genuinely tested.
	window.__rt.newBlankCanvas(840, 520);
	const bgCtx = window.__rt.layers[0].canvas.getContext('2d');
	const grad = bgCtx.createLinearGradient(0, 0, 840, 520);
	grad.addColorStop(0, '#0f2044');
	grad.addColorStop(0.52, '#2d4b7a');
	grad.addColorStop(0.82, '#c86b3c');
	grad.addColorStop(1, '#261a2e');
	bgCtx.fillStyle = grad;
	bgCtx.fillRect(0, 0, 840, 520);
	bgCtx.fillStyle = 'rgba(255, 224, 150, 0.25)';
	bgCtx.beginPath();
	bgCtx.arc(640, 150, 82, 0, Math.PI * 2);
	bgCtx.fill();
	window.__rt.markLayerDirty(window.__rt.layers[0]);
	window.__rt.compositeLayers();
	window.__rt.pushHistory('photo');
	window.__rt.zoomFit();
	</script></body></html>`

	mux := http.NewServeMux()
	mux.HandleFunc("/pixel-test.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})
	mux.Handle("/", http.FileServer(http.FS(Content)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	browser := newSmokeBrowser(t)
	page := browser.MustPage(srv.URL + "/pixel-test.html")
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1360, 860, 1, false)
	page.Timeout(15 * time.Second).MustWait(`() => !!window.__rt && !!document.querySelector('[data-panel="text3d"]')`)

	// Open 3D Text tool & panel, render textured warped rotated 3D text with German umlauts.
	res := page.MustEval(`async () => {
		await document.fonts.ready;
		const rt = window.__rt;
		rt.openText3D({
			text: 'AURAGO\n3D ÄÖÜ',
			font: 'oxanium',
			fontSize: 112,
			texture: 'gold',
			warp: 'cylinder',
			warpStrength: 42,
			pitch: -14,
			yaw: 26,
			roll: -4,
			depth: 52,
			bevel: 42,
			bevelStyle: 'chamfer',
			shadow: true
		});
		const st = rt.text3DState();
		AuraPixelApp.text3d.scheduleRender(rt, st, { immediate: true, highQuality: true });
		const pv = st.previewCanvas;
		const d = pv.getContext('2d').getImageData(0, 0, pv.width, pv.height).data;
		let opaque = 0, shadow = 0, clear = 0;
		const colors = new Set();
		for (let i = 0; i < d.length; i += 4) {
			const a = d[i + 3];
			if (a > 240) {
				opaque++;
				colors.add((d[i] >> 4) + ',' + (d[i+1] >> 4) + ',' + (d[i+2] >> 4));
			} else if (a > 8) shadow++;
			else clear++;
		}
		const panel = document.querySelector('[data-section="text3d"]');
		return {
			webgl: !!(st.gl && st.gl.isUsable()),
			webgl2: !!(st.gl && st.gl.isWebGL2),
			fonts: document.querySelectorAll('[data-t3d-font] option').length,
			textures: document.querySelectorAll('[data-t3d-tex]').length,
			warps: document.querySelectorAll('[data-t3d-warp]').length,
			opaque, shadow, clear, colorBuckets: colors.size,
			overflowX: panel ? (panel.scrollWidth - panel.clientWidth) : -1
		};
	}`)

	if res.Get("fonts").Int() < 16 || res.Get("textures").Int() < 16 || res.Get("warps").Int() < 12 {
		t.Fatalf("unexpected catalog counts: %s", res.Raw())
	}
	if res.Get("opaque").Int() < 2000 || res.Get("clear").Int() < 50000 || res.Get("colorBuckets").Int() < 15 {
		t.Fatalf("expected rich shaded 3D text pixels with transparent surroundings, got %s", res.Raw())
	}
	if res.Get("overflowX").Int() > 2 {
		t.Fatalf("3D Text inspector panel overflows horizontally by %dpx", res.Get("overflowX").Int())
	}

	artifactDir := strings.TrimSpace(os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"))
	if artifactDir != "" {
		_ = os.MkdirAll(artifactDir, 0o755)
		if data, err := page.Screenshot(false, nil); err == nil {
			_ = os.WriteFile(filepath.Join(artifactDir, "pixel-text3d-gold-cylinder.png"), data, 0o644)
		}
	}

	// Switch to Molten Lava + 3D Twist + different angle, verify pixels change distinctly, then Place & Undo/Redo.
	res2 := page.MustEval(`() => {
		const rt = window.__rt;
		const st = rt.text3DState();
		const before = new Uint8ClampedArray(st.previewCanvas.getContext('2d').getImageData(0, 0, st.previewCanvas.width, st.previewCanvas.height).data);
		document.querySelector('[data-t3d-tex="lava"]').click();
		document.querySelector('[data-t3d-warp="twist"]').click();
		AuraPixelApp.text3d.updateParams(rt, st, { yaw: -32, pitch: 18, warpStrength: 55 }, { immediate: true, highQuality: true });
		const after = st.previewCanvas.getContext('2d').getImageData(0, 0, st.previewCanvas.width, st.previewCanvas.height).data;
		let diff = 0;
		for (let i = 0; i < before.length; i += 16) diff += Math.abs(before[i] - after[i]) + Math.abs(before[i+1] - after[i+1]) + Math.abs(before[i+2] - after[i+2]);
		return { diff };
	}`)
	if res2.Get("diff").Int() < 50000 {
		t.Fatalf("switching texture/warp/angle should produce major pixel change, got diff=%d", res2.Get("diff").Int())
	}

	if artifactDir != "" {
		if data, err := page.Screenshot(false, nil); err == nil {
			_ = os.WriteFile(filepath.Join(artifactDir, "pixel-text3d-lava-twist.png"), data, 0o644)
		}
		page.MustEval(`() => {
			const dock = document.querySelector('[data-section="text3d"]');
			if (dock && dock.parentElement) dock.parentElement.scrollTop = 760;
		}`)
		if data, err := page.Screenshot(false, nil); err == nil {
			_ = os.WriteFile(filepath.Join(artifactDir, "pixel-text3d-panel-textures.png"), data, 0o644)
		}
	}

	// Place onto the image as a real raster layer, verify undo/redo and Fruity theme layout.
	res3 := page.MustEval(`() => {
		const rt = window.__rt;
		document.querySelector('[data-t3d-apply]').click();
		const layerCountAfterApply = rt.layers.length;
		const layerName = rt.layers[rt.layers.length - 1].name;
		const d = rt.layers[rt.layers.length - 1].canvas.getContext('2d').getImageData(0, 0, rt.imgWidth, rt.imgHeight).data;
		let placedOpaque = 0;
		for (let i = 3; i < d.length; i += 4) if (d[i] > 200) placedOpaque++;
		rt.undo();
		const afterUndo = rt.layers.length;
		rt.redo();
		const afterRedo = rt.layers.length;
		document.body.dataset.theme = 'fruity';
		const panel = document.querySelector('[data-section="text3d"]');
		return {
			layerCountAfterApply, layerName, placedOpaque, afterUndo, afterRedo,
			fruityOverflowX: panel ? (panel.scrollWidth - panel.clientWidth) : -1,
			errors: window.__errors
		};
	}`)
	if res3.Get("layerCountAfterApply").Int() != 2 || res3.Get("afterUndo").Int() != 1 || res3.Get("afterRedo").Int() != 2 {
		t.Fatalf("unexpected apply/undo/redo layer counts: %s", res3.Raw())
	}
	if res3.Get("placedOpaque").Int() < 2000 {
		t.Fatalf("committed 3D text layer has too few opaque pixels: %s", res3.Raw())
	}
	if res3.Get("fruityOverflowX").Int() > 2 {
		t.Fatalf("3D Text panel overflows horizontally in Fruity theme: %dpx", res3.Get("fruityOverflowX").Int())
	}
	if len(res3.Get("errors").Arr()) > 0 {
		t.Fatalf("browser errors during 3D Text flow: %s", res3.Get("errors").Raw())
	}

	if artifactDir != "" {
		if data, err := page.Screenshot(false, nil); err == nil {
			_ = os.WriteFile(filepath.Join(artifactDir, "pixel-text3d-placed-fruity.png"), data, 0o644)
		}
	}
}
