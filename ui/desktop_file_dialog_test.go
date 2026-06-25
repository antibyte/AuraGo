package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopFileDialogRuntimeIsEmbeddedAndWired(t *testing.T) {
	t.Parallel()

	mainJS := readDesktopAssetText(t, "js/desktop/main.js")
	if !strings.Contains(mainJS, "ui/js/desktop/core/file-dialog-runtime.js") {
		t.Fatal("desktop main bundle does not include file-dialog-runtime.js")
	}

	runtime := readDesktopFileDialogAsset(t, "js/desktop/core/file-dialog-runtime.js")
	for _, marker := range []string{
		"window.AuraDesktopFileDialogs",
		"function openDesktopFileDialog",
		"function saveDesktopFileDialog",
		"function importHostFiles",
		"function exportWorkspaceFile",
		"data-file-dialog-filter",
		"data-file-dialog-filename",
		"data-file-dialog-multi-select",
		"defaultExtension",
		"confirmOverwrite",
		"readonly",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("file dialog runtime missing marker %q", marker)
		}
	}
}

func TestDesktopSDKExposesFileDialogsWithPermissions(t *testing.T) {
	t.Parallel()

	sdk := readDesktopFileDialogAsset(t, "js/desktop/aura-desktop-sdk.js")
	for _, marker := range []string{
		"dialogs.openFile",
		"dialogs.saveFile",
		"dialogs.importFiles",
		"dialogs.exportFile",
		"dialogs,",
	} {
		if !strings.Contains(sdk, marker) {
			t.Fatalf("SDK missing file dialog marker %q", marker)
		}
	}

	bridge := readDesktopFileDialogAsset(t, "js/desktop/core/sdk-events-bootstrap.js")
	for _, marker := range []string{
		"dialog:open-file",
		"dialog:save-file",
		"dialog:import-files",
		"dialog:export-file",
		"requirePermission(client, ['files:read', 'filesystem:read'])",
		"requirePermission(client, ['files:write', 'filesystem:write'])",
	} {
		if !strings.Contains(bridge, marker) {
			t.Fatalf("SDK bridge missing dialog permission marker %q", marker)
		}
	}
}

func TestDesktopFirstPartyAppsReceiveFileDialogCallbacks(t *testing.T) {
	t.Parallel()

	routing := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"openFileDialog",
		"saveFileDialog",
		"importFilesFromHost",
		"exportDesktopFile",
	} {
		if !strings.Contains(routing, marker) {
			t.Fatalf("desktop app routing missing file dialog callback %q", marker)
		}
	}

	for _, path := range []string{
		"js/desktop/file-manager/actions-operations.js",
		"js/desktop/apps/writer.js",
		"js/desktop/apps/sheets.js",
		"js/desktop/apps/viewer.js",
		"js/desktop/apps/viewer-3d.js",
		"js/desktop/apps/code-studio/core.js",
	} {
		source := readDesktopFileDialogAsset(t, path)
		if !strings.Contains(source, "exportDesktopFile") && !strings.Contains(source, "importFilesFromHost") &&
			!strings.Contains(source, "openFileDialog") && !strings.Contains(source, "saveFileDialog") {
			t.Fatalf("%s is not wired to the central desktop file dialog callbacks", path)
		}
	}
}

func TestDesktopFileDialogTranslations(t *testing.T) {
	t.Parallel()

	required := []string{
		"desktop.file_dialog_open",
		"desktop.file_dialog_save",
		"desktop.file_dialog_import",
		"desktop.file_dialog_export",
		"desktop.file_dialog_filter",
		"desktop.file_dialog_overwrite_title",
		"desktop.file_dialog_readonly",
		"desktop.file_dialog_empty",
	}
	entries, err := os.ReadDir(filepath.Join("lang", "desktop"))
	if err != nil {
		t.Fatalf("read desktop languages: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join("lang", "desktop", entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing translation key %q", path, key)
			}
		}
	}
}

func readDesktopFileDialogAsset(t *testing.T, path string) string {
	t.Helper()
	data, err := Content.ReadFile(filepath.ToSlash(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
