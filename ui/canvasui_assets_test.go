package ui

import (
	"encoding/json"
	"regexp"
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

func TestLoginCanvasUIUsesLocalModules(t *testing.T) {
	t.Parallel()

	dropletsJS := string(mustReadUIFile(t, "js/login/droplets.js"))
	for _, want := range []string{
		`from "/js/vendor/canvasui/droplets.js"`,
		"createDroplets",
		"supportsHtmlInCanvas",
		"login-droplets:webgl2-unavailable",
		"login-droplets:html-in-canvas-fallback",
		"login-droplets:theme-update",
		"login-droplets:destroy",
		"login-droplets:reduced-motion-skip",
		"aurago:themechange",
		"prefers-reduced-motion",
		"destroy()",
	} {
		if !strings.Contains(dropletsJS, want) {
			t.Fatalf("login droplets initializer missing marker %q", want)
		}
	}

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

	for _, text := range []string{dropletsJS, flameJS} {
		for _, forbidden := range []string{
			"cdn.jsdelivr",
			"unpkg.com",
			"canvasui.dev",
			"from \"react\"",
			"from 'react'",
			"https://github.com/DavidHDev/canvas-ui",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("login CanvasUI initializer must not reference remote/runtime asset %q", forbidden)
			}
		}
	}

	html := normalizeAssetText(mustReadUIFile(t, "login.html"))
	for _, want := range []string{
		`id="login-bg-host"`,
		`id="login-bg-content"`,
		`id="droplets-source"`,
		`id="droplets-output"`,
		`id="login-card-wrap"`,
		`id="flame-source"`,
		`id="flame-output"`,
		`id="main-content"`,
		`/js/login/droplets.js?v={{.BuildVersion}}`,
		`/js/login/flame-wrap.js?v={{.BuildVersion}}`,
		`type="module"`,
		`id="bg-canvas"`,
		`id="css-bg"`,
		`/js/vendor/three.min.js`,
		`/js/login/main.js`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("login.html missing CanvasUI integration marker %q", want)
		}
	}
	if strings.Contains(html, "canvasui.dev") || strings.Contains(html, "unpkg.com") || strings.Contains(html, "cdn.jsdelivr") {
		t.Fatal("login.html must not load CanvasUI from a remote host")
	}
	_ = regexp.MustCompile(`(?i)https?://`)
}

func TestLoginCanvasUICSSIsScopedAndDecorative(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/login.css"))
	prefix := `.pw-page.pw-entry-page[data-entry-page="login"]`
	for _, want := range []string{
		prefix + ` .login-bg-host`,
		prefix + ` .login-bg-content`,
		prefix + ` .login-droplets-source`,
		prefix + ` .login-droplets-output`,
		prefix + ` .login-card-wrap`,
		prefix + ` .login-flame-source`,
		prefix + ` .login-flame-output`,
		`pointer-events: none`,
		`is-native-capture`,
		`@media (prefers-reduced-motion: reduce)`,
		`.login-droplets-output`,
		`.login-flame-output`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("login.css missing CanvasUI style marker %q", want)
		}
	}
	if !strings.Contains(css, `display: none !important`) {
		t.Fatal("login CanvasUI reduced-motion hide contract missing")
	}
	reduced := css[strings.Index(css, `@media (prefers-reduced-motion: reduce)`):]
	if !strings.Contains(reduced, `.login-flame-output`) {
		t.Error("reduced-motion must hide login flame output")
	}
}
