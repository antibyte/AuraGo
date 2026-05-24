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

func TestSoftwareStoreKeepsReadOnlyActionsAvailableWhileOperationIsBusy(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"if (isMutatingAction(action) && busy.has(appId)) return;",
		"if (action === 'open') return openApp('store-' + appId);",
		"if (action === 'credentials') return openCredentialsModal(appId);",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing non-mutating busy action marker %q", want)
		}
	}
}

func TestSoftwareStorePrefersSpecificMutationDisabledReason(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	switchIndex := strings.Index(source, "switch (mutationDisabledReason)")
	dockerFallbackIndex := strings.Index(source, "if (!dockerAvailable)")
	if switchIndex < 0 || dockerFallbackIndex < 0 {
		t.Fatalf("software store mutation disabled text missing expected branches")
	}
	if dockerFallbackIndex < switchIndex {
		t.Fatal("software store must check specific mutation_disabled_reason before generic docker availability fallback")
	}
	for _, want := range []string{
		"case 'docker_disabled':",
		"case 'docker_readonly':",
		"case 'docker_unavailable':",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store mutation disabled text missing marker %q", want)
		}
	}
}

func TestSoftwareStoreIgnoresStaleInstallMarkersForRunningApps(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"const operationType = app.last_operation_type || app.status || 'install';",
		"if (!storeAppStatusAllowsActiveOperation(app, operationType)) return null;",
		"function storeAppStatusAllowsActiveOperation(app, operationType)",
		"if (operationType === 'install') return app.status === 'installing';",
		"if (operationType === 'update') return app.status === 'updating';",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing stale install marker guard %q", want)
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

func TestSoftwareStoreOpensExternalStoreAppsInBrowserTab(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"function shouldOpenStoreEntryExternally(entry)",
		"entry.metadata.open_external === 'true'",
		"const entry = catalog.find(item => item.id === appId);",
		"if (action === 'open' && shouldOpenStoreEntryExternally(entry)) return openStorePort(appId, '');",
		"const pendingWindow = window.open('about:blank', '_blank');",
		"pendingWindow.opener = null;",
		"pendingWindow.location.replace(body.url);",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing external open marker %q", want)
		}
	}
}

func TestSoftwareStoreFiltersRetiredEmulatorJSEntries(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/software-store.js")
	for _, want := range []string{
		"const retiredStoreAppIDs = new Set(['emulatorjs']);",
		"function activeStoreCatalogEntries(items)",
		"function activeInstalledStoreApps(items)",
		"catalog = activeStoreCatalogEntries(body.catalog || []);",
		"installed = activeInstalledStoreApps(body.installed || []);",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store missing retired app filter marker %q", want)
		}
	}
}

func TestDesktopAPIClientBypassesBrowserCache(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"const requestOptions = Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {});",
		"const resp = await fetch(url, requestOptions);",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("desktop API client missing no-store marker %q", want)
		}
	}
}

func TestSoftwareStoreActionButtonsUseStableGridLayout(t *testing.T) {
	t.Parallel()

	source := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-apps.css"), "\r\n", "\n")
	for _, want := range []string{
		".vd-store-card {\n    display: grid;",
		"grid-template-rows: auto minmax(64px, auto) auto auto 1fr;",
		".vd-store-actions {\n    display: grid;",
		"grid-template-columns: repeat(auto-fit, minmax(128px, 1fr));",
		"align-items: stretch;",
		".vd-store-btn {\n    width: 100%;",
		".vd-store-btn span {\n    min-width: 0;",
		"text-overflow: ellipsis;",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("software store action layout missing marker %q", want)
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
