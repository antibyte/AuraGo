package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

const canvasUIPinnedCommit = "81b65e159c63bde7167b9b4b458a775838e4cd39"

func TestCanvasUIDropletsAssetsAreEmbedded(t *testing.T) {
	t.Parallel()

	module, err := Content.ReadFile("js/vendor/canvasui/droplets.js")
	if err != nil {
		t.Fatalf("CanvasUI droplets ESM missing from embedded UI: %v", err)
	}
	if len(module) < 8000 {
		t.Fatalf("CanvasUI droplets ESM is unexpectedly small: %d bytes", len(module))
	}
	moduleText := string(module)
	for _, want := range []string{
		"export function createDroplets",
		"export function supportsHtmlInCanvas",
		"uHasContent",
		canvasUIPinnedCommit,
		"react-source-to-vanilla-esm",
		"https://github.com/DavidHDev/canvas-ui",
	} {
		if !strings.Contains(moduleText, want) {
			t.Fatalf("CanvasUI droplets module missing marker %q", want)
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
			t.Fatalf("CanvasUI droplets module must not contain remote/runtime import %q", forbidden)
		}
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
		Name         string   `json:"name"`
		Component    string   `json:"component"`
		Components   []string `json:"components"`
		Source       string   `json:"source"`
		Commit       string   `json:"commit"`
		License      string   `json:"license"`
		LicenseFile  string   `json:"license_file"`
		Module       string   `json:"module"`
		Adaptation   string   `json:"adaptation"`
		UpstreamPath string   `json:"upstream_path"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("parse CanvasUI manifest: %v", err)
	}
	if manifest.Name != "canvasui" || manifest.Component != "droplets" || manifest.Module != "droplets.js" {
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
	foundComponent := false
	for _, component := range manifest.Components {
		if component == "droplets" {
			foundComponent = true
			break
		}
	}
	if !foundComponent {
		t.Fatalf("CanvasUI manifest components missing droplets: %+v", manifest.Components)
	}
}

func TestLoginDropletsUsesLocalCanvasUI(t *testing.T) {
	t.Parallel()

	loginJS := string(mustReadUIFile(t, "js/login/droplets.js"))
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
		if !strings.Contains(loginJS, want) {
			t.Fatalf("login droplets initializer missing marker %q", want)
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
		if strings.Contains(loginJS, forbidden) {
			t.Fatalf("login droplets initializer must not reference remote/runtime asset %q", forbidden)
		}
	}

	html := normalizeAssetText(mustReadUIFile(t, "login.html"))
	for _, want := range []string{
		`id="login-bg-host"`,
		`id="login-bg-content"`,
		`id="droplets-source"`,
		`id="droplets-output"`,
		`/js/login/droplets.js?v={{.BuildVersion}}`,
		`type="module"`,
		`id="bg-canvas"`,
		`id="css-bg"`,
		`/js/vendor/three.min.js`,
		`/js/login/main.js`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("login.html missing droplets integration marker %q", want)
		}
	}
	if regexp.MustCompile(`(?i)https?://`).MatchString(html) {
		// Allow only non-runtime meta/CDN-free relative assets; block remote script hosts.
		if strings.Contains(html, "canvasui.dev") || strings.Contains(html, "unpkg.com") || strings.Contains(html, "cdn.jsdelivr") {
			t.Fatal("login.html must not load CanvasUI from a remote host")
		}
	}
}

func TestLoginDropletsCSSIsScopedAndDecorative(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/login.css"))
	prefix := `.pw-page.pw-entry-page[data-entry-page="login"]`
	for _, want := range []string{
		prefix + ` .login-bg-host`,
		prefix + ` .login-bg-content`,
		prefix + ` .login-droplets-source`,
		prefix + ` .login-droplets-output`,
		`pointer-events: none`,
		`is-native-capture`,
		`@media (prefers-reduced-motion: reduce)`,
		`.login-droplets-output`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("login.css missing droplets style marker %q", want)
		}
	}
	// Output must stay non-interactive and reduced-motion must hide it.
	if !strings.Contains(css, `.login-droplets-output`) || !strings.Contains(css, `display: none !important`) {
		t.Fatal("login droplets reduced-motion hide contract missing")
	}
}
