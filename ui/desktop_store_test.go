package ui

import (
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
