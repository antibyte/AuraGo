package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrueNASFrontendUsesVaultSecretsEndpoint(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("js", "truenas.js"))
	if err != nil {
		t.Fatalf("read truenas.js: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, "fetch('/api/vault/secrets'") {
		t.Fatal("truenas settings must save API keys through /api/vault/secrets")
	}
	if strings.Contains(js, "fetch('/api/vault',") {
		t.Fatal("truenas settings must not POST API keys to legacy /api/vault")
	}
	for _, marker := range []string{
		"const vaultResponse = await fetch('/api/vault/secrets'",
		"if (!vaultResponse.ok)",
		"throw new Error(data.error || data.message || `HTTP ${vaultResponse.status}`)",
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("truenas settings must surface vault save failures, missing marker %q", marker)
		}
	}
}

func TestTrueNASFrontendConnectionTestRequiresOnlineStatus(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("js", "truenas.js"))
	if err != nil {
		t.Fatalf("read truenas.js: %v", err)
	}
	js := string(content)

	for _, marker := range []string{
		"return data;",
		"const status = await this.checkStatus();",
		"status.status !== 'online'",
		"throw new Error(status.error || t('truenas.connection_failed'))",
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("truenas connection test missing marker %q", marker)
		}
	}
}

func TestTrueNASFrontendSnapshotAgeAndNFSShareFlowRemainPresent(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("js", "truenas.js"))
	if err != nil {
		t.Fatalf("read truenas.js: %v", err)
	}
	js := string(content)

	for _, marker := range []string{
		"snapshotAge(snap)",
		"Number.isFinite",
		"/shares/nfs",
		"createNFSShare",
		"deleteNFSShare",
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("truenas frontend missing marker %q", marker)
		}
	}
}
