package virtualcomputers

import (
	"strings"
	"testing"
)

func TestManagementInstallScriptContainsManagedDeploymentContract(t *testing.T) {
	script := managementInstallScript(SetupInstallOptions{
		InstallDir: "/opt/boring-test",
		BoringdURL: "http://127.0.0.1:18080",
		Token:      "test-token-value",
	})

	for _, want := range []string{
		PinnedUpstreamRevision,
		"nodejs",
		"npm ci",
		"npm run build -w web",
		"releases/${BORING_WEB_REVISION}",
		"ln -sfn",
		"/etc/boring/boring-web.env",
		"chmod 0600 /etc/boring/boring-web.env",
		"BORING_URL=${BORING_WEB_BORING_URL}",
		"BORING_TOKEN=${BORING_TOKEN_VALUE}",
		"boring-web.service",
		"127.0.0.1:18081",
		"ProtectSystem=strict",
		"NoNewPrivileges=true",
		"systemctl enable boring-web.service",
		"/boring-computers/",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("management install script missing %q", want)
		}
	}

	if strings.Contains(script, "PUBLIC_BORING_URL=http://127.0.0.1:18080") {
		t.Fatal("management build must not expose the private boringd URL to browser assets")
	}
}

func TestManagementInstallScriptPinsUpstreamInsteadOfFollowingMain(t *testing.T) {
	script := managementInstallScript(SetupInstallOptions{})
	if strings.Contains(script, "reset --hard origin/main") || strings.Contains(script, "--branch main") {
		t.Fatal("management deployment must use AuraGo's reviewed upstream revision")
	}
	if !strings.Contains(script, "checkout --detach ${BORING_WEB_REVISION}") {
		t.Fatal("management deployment must detach at the reviewed revision")
	}
}
