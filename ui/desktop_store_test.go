package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSoftwareStoreDisablesMutatingActionsWhenBackendDisallowsThem(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"let mutationsAllowed = true;",
		"body.mutations_allowed !== false",
		"mutationDisabledText()",
		"if (isMutatingAction(action) && !mutationsAllowed)",
		"const actionDisabled = operation ? statusLabel(status, operation) : mutationDisabled;",
		"actionDisabled ? `disabled title=\"${esc(actionDisabled)}\"` : ''",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing mutation guard marker %q", want)
		}
	}
}

func TestSoftwareStoreLogoFallbackRespectsHiddenAttribute(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "css/desktop-apps.css")
	for _, want := range []string{
		".vd-store-logo-fallback[hidden]",
		"display: none",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store logo fallback CSS missing marker %q", want)
		}
	}
}

func TestSoftwareStoreShowsInstallingAndDisablesActionsDuringInstallOperation(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"const actionDisabled = operation ? statusLabel(status, operation) : mutationDisabled;",
		"actionDisabled ? `disabled title=\"${esc(actionDisabled)}\"` : ''",
		"if (operation && operation.type === 'install')",
		"return t('desktop.store.status_installing', 'Installing');",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing install operation UI marker %q", want)
		}
	}
}

func TestSoftwareStoreResumesAndRefreshesActiveOperations(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"const operation = busy.get(entry.id) || activeOperationForApp(app);",
		"function activeOperationForApp(app)",
		"app.last_operation_state === 'pending' || app.last_operation_state === 'running'",
		"const pollingOperations = new Set();",
		"resumeActiveOperationPolling();",
		"if (pollingOperations.has(operationId)) return;",
		"instance.onDesktopEvent = payload =>",
		"payload.operation === 'desktop_store_changed'",
		"window.AuraSSE.on('virtual_desktop_event', instance.onDesktopEvent)",
		"window.AuraSSE.off('virtual_desktop_event', instance.onDesktopEvent)",
		"window.SoftwareStoreApp = { render, dispose };",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing operation resume/refresh marker %q", want)
		}
	}
}

func TestSoftwareStoreSupportsExpandedAppCapabilities(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"function hostAccessWarning(entry)",
		"entry.host_binds",
		"entry.companions",
		"function extraPortButtons(entry, app, actionDisabled)",
		"data-action=\"open-port\"",
		"function openStorePort(appId, portId)",
		"port_id=",
		"function openCredentialsModal(appId)",
		"/credentials",
		"function openBeszelAgentModal(appId)",
		"/companions/agent/config",
		"desktop.store.host_access_warning",
		"desktop.store.credentials",
		"desktop.store.configure_agent",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing expanded app marker %q", want)
		}
	}
}

func TestSoftwareStoreExpandedCapabilityTranslations(t *testing.T) {
	t.Parallel()

	required := []string{
		"desktop.store.host_access_warning",
		"desktop.store.credentials",
		"desktop.store.configure_agent",
		"desktop.store.no_credentials",
		"desktop.store.beszel_agent_copy",
		"desktop.store.beszel_key",
		"desktop.store.beszel_token",
		"desktop.store.agent_configured",
	}
	entries, err := Content.ReadDir("lang/desktop")
	if err != nil {
		t.Fatalf("read desktop translations: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := "lang/desktop/" + entry.Name()
		raw, err := Content.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
