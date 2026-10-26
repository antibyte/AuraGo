package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

var networkCamerasDetectionFiles = []string{
	"js/desktop/apps/network-cameras-detection-views.js",
	"js/desktop/apps/network-cameras-detection-zones.js",
	"js/desktop/apps/network-cameras-detection.js",
}

// Keys built as prefix + value in the detection modules.
var networkCamerasDetectionDynamicKeys = map[string][]string{
	"arm_":     {"off", "on", "schedule"},
	"phase_":   {"downloading_model", "downloading_runtime", "preparing_gpu_model", "starting_worker", "self_test"},
	"camera_":  {"unreachable", "waiting", "error"},
	"action_":  {"message", "agent", "desktop", "mission"},
	"alert_":   {"sent", "failed", "skipped"},
	"actor_":   {"user", "agent", "schedule"},
	"model_":   {"standard", "accurate"},
	"backend_": {"auto", "gpu", "cpu"},
}

// German strings that may equal English.
var networkCamerasDetectionSameInGerman = map[string]bool{
	"desktop.network_cameras.detection.action_agent":      true,
	"desktop.network_cameras.detection.zone_default_name": true,
}

func networkCamerasDetectionUsedKeys(t *testing.T) []string {
	t.Helper()
	var parts []string
	for _, file := range networkCamerasDetectionFiles {
		parts = append(parts, rawDesktopAssetText(t, file))
	}
	source := strings.Join(parts, "\n")
	keys := map[string]bool{}
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`\bT\((?:ctx|inst\.ctx), '([a-z0-9_]+)'`),
		regexp.MustCompile(`\bT\((?:ctx|inst\.ctx), [^,()]+\? '([a-z0-9_]+)' : '([a-z0-9_]+)'`),
		regexp.MustCompile(`\bkey: '([a-z0-9_]+)'`),
		regexp.MustCompile(`NS \+ '([a-z0-9_]+)'`),
	} {
		for _, match := range pattern.FindAllStringSubmatch(source, -1) {
			for _, group := range match[1:] {
				if group != "" && !strings.HasSuffix(group, "_") {
					keys["desktop.network_cameras.detection."+group] = true
				}
			}
		}
	}
	for prefix, values := range networkCamerasDetectionDynamicKeys {
		if !strings.Contains(source, "'"+prefix+"' +") {
			t.Errorf("the detection modules no longer build %s* keys; update networkCamerasDetectionDynamicKeys", prefix)
		}
		for _, value := range values {
			keys["desktop.network_cameras.detection."+prefix+value] = true
		}
	}
	keys["desktop.network_cameras.detection.mode"] = true
	keys["desktop.network_cameras.detection.tab"] = true
	keys["desktop.network_cameras.tab_general"] = true
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func TestDesktopNetworkCamerasDetectionTranslations(t *testing.T) {
	keys := networkCamerasDetectionUsedKeys(t)
	if len(keys) < 160 {
		t.Fatalf("found %d detection keys in the sources, want at least 160 (extraction broken?)", len(keys))
	}
	english := readMergedLangMap(t, "en")
	placeholder := regexp.MustCompile(`\{\{[a-z_]+\}\}`)
	placeholders := func(value string) string {
		found := placeholder.FindAllString(value, -1)
		sort.Strings(found)
		return strings.Join(found, ",")
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readMergedLangMap(t, lang)
		for _, key := range keys {
			value := strings.TrimSpace(values[key])
			if value == "" || strings.ContainsRune(value, '�') {
				t.Errorf("%s: %s missing or broken", lang, key)
				continue
			}
			if placeholders(value) != placeholders(english[key]) {
				t.Errorf("%s: %s placeholders %q, want %q", lang, key, placeholders(value), placeholders(english[key]))
			}
			if lang != "de" {
				continue
			}
			if value == english[key] && !networkCamerasDetectionSameInGerman[key] {
				t.Errorf("de: %s is untranslated", key)
			}
			for _, formal := range []string{" Sie ", " Ihre ", " Ihnen ", " Ihr "} {
				if strings.Contains(" "+value+" ", formal) {
					t.Errorf("de: %s uses formal address: %q", key, value)
				}
			}
		}
	}
}

func TestDesktopNetworkCamerasDetectionStyles(t *testing.T) {
	t.Parallel()

	styles := rawDesktopAssetText(t, "css/desktop-app-network-cameras-detection.css")
	for _, marker := range []string{
		".ncd-root",
		"container-type: inline-size",
		"--ncd-danger: color-mix(in srgb, #e11d48 55%, var(--vd-text, #e5e7eb))",
		".ncd-zone-stage:focus-visible",
		"touch-action: none",
		".nc-modal [data-tab-panel][hidden] { display: none !important; }",
		"@media (prefers-reduced-motion: reduce)",
		// Batch 4A/4B: the server time zone label, and test boxes never take the
		// pointer from the zone editor (vertex drags and new points go to the stage).
		".ncd-server-zone",
		".ncd-zone-status.is-error",
		".ncd-zone-stage .ncd-box,\n.ncd-zone-stage .ncd-box-label { pointer-events: none; }",
	} {
		if !strings.Contains(strings.ReplaceAll(styles, "\r\n", "\n"), marker) {
			t.Errorf("desktop-app-network-cameras-detection.css lacks %q", marker)
		}
	}
}

func TestDesktopNetworkCamerasDetectionRegistration(t *testing.T) {
	t.Parallel()

	loader := strings.ReplaceAll(rawDesktopAssetText(t, "js/desktop/core/module-loader.js"), "\r\n", "\n")
	for _, marker := range []string{
		"styles: appStyles('/css/desktop-app-network-cameras.css', '/css/desktop-app-network-cameras-detection.css', '/css/go2rtc-tuya.css')",
		"'/js/desktop/apps/network-cameras-detection-views.js',\n                '/js/desktop/apps/network-cameras-detection-zones.js',\n                '/js/desktop/apps/network-cameras-detection.js',\n                '/js/desktop/apps/network-cameras-media.js',\n                '/js/desktop/apps/network-cameras.js'",
	} {
		if !strings.Contains(loader, marker) {
			t.Errorf("module-loader.js lacks %q", marker)
		}
	}
	app := rawDesktopAssetText(t, "js/desktop/apps/network-cameras.js")
	for _, marker := range []string{
		"function coerceMode(value)",
		"mode: coerceMode(value.mode)",
		"state.mode = coerceMode(button.dataset.mode)",
		`data-mode="detections"`,
		"function syncDetection(state)",
		"module.unmount(state.windowId, { keepState: true })",
		"if (detection) detection.unmount(windowId)",
		"openCameraSettings: streamID => openManageModal(state, streamID, 'detection')",
		"detection.validateCameraSettings(modal.detection)",
		"modal.detectionDirty && modal.detection ? { detection: modal.detection } : {}",
		"function launchContext(context)",
		"mode: launchContext(ctx) ? (launchContext(ctx).event_id ? 'detections' : 'snapshots') : preferences.mode",
		"window.NetworkCamerasApp = { render, dispose, open }",
		"function notify(state, message, type)",
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("network-cameras.js lacks %q", marker)
		}
	}
	hook := "window.NetworkCamerasApp?.open?.(existing.id, context);"
	if !strings.Contains(rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js"), hook) {
		t.Error("window-shell-runtime.js openApp does not forward launch contexts to an open Network Cameras window")
	}
	if !strings.Contains(rawDesktopAssetText(t, "js/desktop/bundles/main.bundle.js"), hook) {
		t.Error("main.bundle.js lacks the Network Cameras launch hook (run npm run build:ui)")
	}
	for _, file := range networkCamerasDetectionFiles {
		if lines := strings.Count(rawDesktopAssetText(t, file), "\n"); lines >= 1100 {
			t.Errorf("%s has %d lines; desktop JS files stay below 1100", file, lines)
		}
	}
}
