package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

const canvasUIPinnedCommit = "81b65e159c63bde7167b9b4b458a775838e4cd39"

func TestCanvasUIAssetsAreEmbedded(t *testing.T) {
	t.Parallel()

	for _, modulePath := range []string{
		"js/vendor/canvasui/droplets.js",
		"js/vendor/canvasui/flame-wrap.js",
	} {
		module, err := Content.ReadFile(modulePath)
		if err != nil {
			t.Fatalf("CanvasUI module %s missing from embedded UI: %v", modulePath, err)
		}
		if len(module) < 8000 {
			t.Fatalf("CanvasUI module %s is unexpectedly small: %d bytes", modulePath, len(module))
		}
		moduleText := string(module)
		for _, want := range []string{
			"export function supportsHtmlInCanvas",
			"uHasContent",
			canvasUIPinnedCommit,
			"react-source-to-vanilla-esm",
			"https://github.com/DavidHDev/canvas-ui",
		} {
			if !strings.Contains(moduleText, want) {
				t.Fatalf("CanvasUI module %s missing marker %q", modulePath, want)
			}
		}
		for _, forbidden := range []string{
			"from \"react\"",
			"from 'react'",
			"cdn.jsdelivr",
			"unpkg.com",
			"canvasui.dev",
			"https://raw.githubusercontent.com",
		} {
			if strings.Contains(moduleText, forbidden) {
				t.Fatalf("CanvasUI module %s must not contain remote/runtime import %q", modulePath, forbidden)
			}
		}
	}

	droplets := string(mustReadUIFile(t, "js/vendor/canvasui/droplets.js"))
	if !strings.Contains(droplets, "export function createDroplets") {
		t.Fatal("droplets module missing createDroplets export")
	}
	for _, want := range []string{
		"elements.bitmap",
		"drawImageCover",
		"setBitmap",
		"allowBitmap",
		"hasContent()",
	} {
		if !strings.Contains(droplets, want) {
			t.Fatalf("droplets module missing bitmap-content marker %q", want)
		}
	}
	flame := string(mustReadUIFile(t, "js/vendor/canvasui/flame-wrap.js"))
	if !strings.Contains(flame, "export function createFlameWrap") {
		t.Fatal("flame-wrap module missing createFlameWrap export")
	}

	license, err := Content.ReadFile("js/vendor/canvasui/LICENSE.txt")
	if err != nil {
		t.Fatalf("CanvasUI license missing from embedded UI: %v", err)
	}
	licenseText := string(license)
	for _, want := range []string{
		"MIT + Commons Clause License Condition v1.0",
		"Copyright (c) 2026 David Haz",
		"Commons Clause Restriction",
	} {
		if !strings.Contains(licenseText, want) {
			t.Fatalf("CanvasUI license missing %q", want)
		}
	}

	manifestBytes, err := Content.ReadFile("js/vendor/canvasui/manifest.json")
	if err != nil {
		t.Fatalf("CanvasUI manifest missing from embedded UI: %v", err)
	}
	var manifest struct {
		Name          string            `json:"name"`
		Components    []string          `json:"components"`
		Source        string            `json:"source"`
		Commit        string            `json:"commit"`
		License       string            `json:"license"`
		LicenseFile   string            `json:"license_file"`
		Modules       map[string]string `json:"modules"`
		Adaptation    string            `json:"adaptation"`
		UpstreamPaths map[string]string `json:"upstream_paths"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("parse CanvasUI manifest: %v", err)
	}
	if manifest.Name != "canvasui" {
		t.Fatalf("unexpected CanvasUI manifest identity: %+v", manifest)
	}
	if manifest.Source != "https://github.com/DavidHDev/canvas-ui" {
		t.Fatalf("unexpected CanvasUI source %q", manifest.Source)
	}
	if manifest.Commit != canvasUIPinnedCommit {
		t.Fatalf("unexpected CanvasUI commit %q", manifest.Commit)
	}
	if !strings.Contains(manifest.License, "Commons Clause") {
		t.Fatalf("unexpected CanvasUI license %q", manifest.License)
	}
	if manifest.LicenseFile != "LICENSE.txt" || manifest.Adaptation != "react-source-to-vanilla-esm" {
		t.Fatalf("unexpected CanvasUI adaptation metadata: %+v", manifest)
	}
	for _, component := range []string{"droplets", "flame-wrap"} {
		found := false
		for _, entry := range manifest.Components {
			if entry == component {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("CanvasUI manifest components missing %s: %+v", component, manifest.Components)
		}
		if manifest.Modules[component] == "" {
			t.Fatalf("CanvasUI manifest modules missing %s", component)
		}
		if manifest.UpstreamPaths[component] == "" {
			t.Fatalf("CanvasUI manifest upstream_paths missing %s", component)
		}
	}
}

func TestLoginFlameWrapUsesLocalCanvasUI(t *testing.T) {
	t.Parallel()

	flameJS := string(mustReadUIFile(t, "js/login/flame-wrap.js"))
	for _, want := range []string{
		`from "/js/vendor/canvasui/flame-wrap.js"`,
		"createFlameWrap",
		"supportsHtmlInCanvas",
		"login-flame:webgl2-unavailable",
		"login-flame:html-in-canvas-fallback",
		"login-flame:theme-update",
		"login-flame:destroy",
		"login-flame:reduced-motion-skip",
		"aurago:themechange",
		"prefers-reduced-motion",
		"destroy()",
		"applyOutputGeometry(wrap, card, output)",
		"getBoundingClientRect()",
		"Must be measurable before createFlameWrap",
	} {
		if !strings.Contains(flameJS, want) {
			t.Fatalf("login flame-wrap initializer missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"cdn.jsdelivr",
		"unpkg.com",
		"canvasui.dev",
		"from \"react\"",
		"from 'react'",
		"https://github.com/DavidHDev/canvas-ui",
	} {
		if strings.Contains(flameJS, forbidden) {
			t.Fatalf("login flame-wrap initializer must not reference remote/runtime asset %q", forbidden)
		}
	}

	html := normalizeAssetText(mustReadUIFile(t, "login.html"))
	for _, want := range []string{
		`id="login-card-wrap"`,
		`id="flame-source"`,
		`id="flame-output"`,
		`id="main-content"`,
		`/js/login/flame-wrap.js?v={{.BuildVersion}}`,
		`type="module"`,
		`id="bg-canvas"`,
		`id="css-bg"`,
		`/js/vendor/three.min.js`,
		`/js/login/main.js`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("login.html missing flame-wrap integration marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"droplets.js",
		"droplets-source",
		"droplets-output",
		"login-droplets",
		"canvasui.dev",
		"unpkg.com",
		"cdn.jsdelivr",
	} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("login.html must not contain %q", forbidden)
		}
	}
}

func TestLoginFlameCSSIsScopedAndDecorative(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/login.css"))
	prefix := `.pw-page.pw-entry-page[data-entry-page="login"]`
	for _, want := range []string{
		prefix + ` .login-card-wrap`,
		prefix + ` .login-flame-source`,
		prefix + ` .login-flame-output`,
		`pointer-events: none`,
		`is-native-capture`,
		`@media (prefers-reduced-motion: reduce)`,
		`.login-flame-output`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("login.css missing flame style marker %q", want)
		}
	}
	for _, forbidden := range []string{
		`.login-droplets-output`,
		`.login-droplets-source`,
	} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("login.css must not retain droplets selector %q", forbidden)
		}
	}
	if !strings.Contains(css, `display: none !important`) {
		t.Fatal("login flame reduced-motion hide contract missing")
	}
}

func TestDesktopCityRainDropletsStayOnWallpaper(t *testing.T) {
	t.Parallel()

	desktopJS := string(mustReadUIFile(t, "js/desktop/city-rain-droplets.js"))
	for _, want := range []string{
		`from "/js/vendor/canvasui/droplets.js"`,
		"createDroplets",
		`city_rain`,
		`/img/wallpapers/city_rain.jpg`,
		"bitmap",
		"wallpaper-bitmap",
		"data-wallpaper",
		"vd-wallpaper-fx",
		"desktop-droplets:webgl2-unavailable",
		"desktop-droplets:reduced-motion-skip",
		"desktop-droplets:active",
		"desktop-droplets:waiting-layout",
		"interactive: false",
		"MutationObserver",
		"ResizeObserver",
		"preloadWallpaperImage",
		"scheduleBootstrapRetries",
		"pageshow",
		"prefers-reduced-motion",
		"destroy()",
		"tintStrength: 0",
	} {
		if !strings.Contains(desktopJS, want) {
			t.Fatalf("desktop city-rain droplets missing marker %q", want)
		}
	}
	// Must not capture desktop HTML into the effect (keeps windows/icons clean).
	if strings.Contains(desktopJS, "layoutsubtree=\"true\"") || strings.Contains(desktopJS, "drawElementImage") {
		t.Fatal("desktop droplets must not capture desktop HTML; wallpaper bitmap only")
	}

	html := normalizeAssetText(mustReadUIFile(t, "desktop.html"))
	for _, want := range []string{
		`id="vd-wallpaper-fx"`,
		`id="vd-droplets-source"`,
		`id="vd-droplets-output"`,
		`id="vd-workspace"`,
		`id="vd-icons"`,
		`id="vd-window-layer"`,
		`/js/desktop/city-rain-droplets.js?v={{.BuildVersion}}`,
		`type="module"`,
		`rel="preload"`,
		`/img/wallpapers/city_rain.jpg`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("desktop.html missing city-rain droplets marker %q", want)
		}
	}
	// Wallpaper FX must appear before foreground layers in DOM order.
	fxIdx := strings.Index(html, `id="vd-wallpaper-fx"`)
	iconsIdx := strings.Index(html, `id="vd-icons"`)
	windowsIdx := strings.Index(html, `id="vd-window-layer"`)
	if fxIdx < 0 || iconsIdx < 0 || windowsIdx < 0 || !(fxIdx < iconsIdx && iconsIdx < windowsIdx) {
		t.Fatal("vd-wallpaper-fx must precede icons and window layer in desktop.html")
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/desktop-base.css"))
	bundle := normalizeAssetText(mustReadUIFile(t, "css/desktop-shell.bundle.css"))
	for _, source := range []string{css, bundle} {
		for _, want := range []string{
			`.vd-wallpaper-fx`,
			`z-index: -1`,
			`pointer-events: none`,
			`data-wallpaper="city_rain"`,
			`.vd-droplets-output`,
			`mix-blend-mode: normal`,
			`@media (prefers-reduced-motion: reduce)`,
		} {
			if !strings.Contains(source, want) {
				t.Fatalf("desktop CSS missing city-rain droplets marker %q", want)
			}
		}
		// CSS wallpaper must remain as fallback while WebGL settles.
		if strings.Contains(source, `:has(.vd-wallpaper-fx.is-active)`) &&
			strings.Contains(source, `background-image: none`) {
			t.Fatal("city_rain must not hide the CSS wallpaper while droplets activate")
		}
		if strings.Contains(source, `.vd-droplets-output.is-active`) &&
			strings.Contains(source[strings.Index(source, `.vd-droplets-output.is-active`):], `mix-blend-mode: screen`) {
			t.Fatal("desktop droplets must not use mix-blend-mode: screen on the output canvas")
		}
	}
}
