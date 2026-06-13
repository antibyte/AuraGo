package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFrontendDograhSectionAndI18nExist(t *testing.T) {
	t.Parallel()

	mainPath := filepath.Join("js", "config", "main.js")
	modulePath := filepath.Join("cfg", "dograh.js")
	dashboardPath := filepath.Join("js", "dashboard", "dashboard-widgets.js")

	mainContent, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read %s: %v", mainPath, err)
	}
	moduleContent, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatalf("read %s: %v", modulePath, err)
	}
	dashboardContent, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("read %s: %v", dashboardPath, err)
	}

	mainJS := string(mainContent)
	for _, marker := range []string{
		"{ key: 'dograh'",
		"dograh: { m: 'dograh', fn: 'renderDograhSection' }",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("%s missing Dograh marker %q", mainPath, marker)
		}
	}

	moduleJS := string(moduleContent)
	for _, marker := range []string{
		"function renderDograhSection",
		"/api/dograh/status",
		"/api/dograh/test",
		"/api/dograh/start",
		"/api/dograh/stop",
		"/api/dograh/recreate",
		"/api/dograh/provision-webhook",
		"/api/dograh/register-aurago-mcp-tool",
		"dograh.api_key",
		"class=\"field-help\"",
		"class=\"field-group\"",
		"dograh-test-result",
		"adg-test-result",
		"dograhSetBanner",
	} {
		if !strings.Contains(moduleJS, marker) {
			t.Fatalf("%s missing Dograh module marker %q", modulePath, marker)
		}
	}
	if strings.Contains(moduleJS, "alert(") {
		t.Fatal("Dograh config module must not introduce alert()")
	}
	dashboardJS := string(dashboardContent)
	if !strings.Contains(dashboardJS, "dograh:") || !strings.Contains(dashboardJS, "dashboard.integration_dograh") {
		t.Fatalf("%s missing Dograh dashboard integration markers", dashboardPath)
	}

	configKeys := []string{
		"config.section.dograh.label",
		"config.section.dograh.desc",
		"config.dograh.enabled_label",
		"config.dograh.mode_label",
		"config.dograh.api_key_label",
		"config.dograh.start_button",
		"config.dograh.stop_button",
		"config.dograh.recreate_button",
		"config.dograh.test_button",
		"config.dograh.webhook_button",
		"config.dograh.mcp_register_button",
		"config.dograh.vault_section_title",
		"config.dograh.mcp_section_title",
		"config.dograh.testing",
		"config.dograh.starting",
		"config.dograh.stopping",
		"config.dograh.recreating",
		"help.dograh.enabled",
		"help.dograh.api_key",
		"help.dograh.api_url",
		"help.dograh.ui_url",
		"help.dograh.mode",
		"help.dograh.readonly",
		"help.dograh.auto_start",
		"help.dograh.telemetry_enabled",
		"help.dograh.mcp_client_enabled",
		"help.dograh.mcp_server_tool_enabled",
		"help.dograh.credential_uuid",
		"help.dograh.allowed_tools",
		"help.dograh.webhook_slug",
		"help.dograh.host",
		"help.dograh.api_host_port",
		"help.dograh.ui_host_port",
	}
	assertLangKeys(t, filepath.Join("lang", "config"), configKeys)
	assertDograhHelpDescriptions(t, filepath.Join("lang", "config"))
	assertLangKeys(t, filepath.Join("lang", "dashboard"), []string{"dashboard.integration_dograh"})
}

func assertLangKeys(t *testing.T, dir string, keys []string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("%s has no language files", dir)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", path, err)
		}
		for _, key := range keys {
			value, ok := data[key]
			if !ok {
				t.Fatalf("%s missing i18n key %s", path, key)
			}
			if text, ok := value.(string); !ok || strings.TrimSpace(text) == "" {
				t.Fatalf("%s key %s must be a non-empty string", path, key)
			}
		}
	}
}

func assertDograhHelpDescriptions(t *testing.T, dir string) {
	t.Helper()
	labelByHelpKey := map[string]string{
		"help.dograh.enabled":                 "config.dograh.enabled_label",
		"help.dograh.api_key":                 "config.dograh.api_key_label",
		"help.dograh.api_url":                 "config.dograh.api_url_label",
		"help.dograh.ui_url":                  "config.dograh.ui_url_label",
		"help.dograh.mode":                    "config.dograh.mode_label",
		"help.dograh.readonly":                "config.dograh.readonly_label",
		"help.dograh.auto_start":              "config.dograh.auto_start_label",
		"help.dograh.telemetry_enabled":       "config.dograh.telemetry_label",
		"help.dograh.mcp_client_enabled":      "config.dograh.mcp_client_label",
		"help.dograh.mcp_server_tool_enabled": "config.dograh.mcp_server_tool_label",
		"help.dograh.credential_uuid":         "config.dograh.credential_uuid_label",
		"help.dograh.allowed_tools":           "config.dograh.allowed_tools_label",
		"help.dograh.webhook_slug":            "config.dograh.webhook_slug_label",
		"help.dograh.host":                    "config.dograh.host_label",
		"help.dograh.api_host_port":           "config.dograh.api_host_port_label",
		"help.dograh.ui_host_port":            "config.dograh.ui_host_port_label",
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", path, err)
		}
		for helpKey, labelKey := range labelByHelpKey {
			helpText := strings.TrimSpace(data[helpKey].(string))
			labelText := strings.TrimSpace(data[labelKey].(string))
			if helpText == labelText {
				t.Fatalf("%s help key %s repeats label %s", path, helpKey, labelKey)
			}
			if len([]rune(helpText)) < 18 {
				t.Fatalf("%s help key %s is too short to explain the field: %q", path, helpKey, helpText)
			}
		}
	}
}
