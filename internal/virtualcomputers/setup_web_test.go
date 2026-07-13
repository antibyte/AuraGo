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
		"runtime/node-v${NODE_VERSION}-linux-${NODE_ARCH}",
		"export PATH=\"${NODE_BIN}:${PATH}\"",
		"\"${NODE_BIN}/npm\" ci",
		"\"${NODE_BIN}/npm\" run build -w web",
		"RELEASE_ID=\"${BORING_WEB_REVISION}-$(date -u +%Y%m%dT%H%M%SZ)-$$\"",
		"releases/${RELEASE_ID}",
		"ln -sfnT",
		".aurago-revision",
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
	for _, forbidden := range []string{"/usr/local/bin/node", "/usr/local/bin/npm", "/usr/local/bin/npx", "rm -rf \"${RELEASE_DIR}\""} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("management install script must not contain %q", forbidden)
		}
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

func TestManagementInstallScriptPinsReviewedSourceHashes(t *testing.T) {
	script := managementInstallScript(SetupInstallOptions{})
	for _, want := range []string{
		"50cddca87651ac11e4b525f13fc60572b63b29e65ba2aa60c6b84d1e610e8832",
		"1cc5f7e2f766f36c07a86a1d5fb8ad0aa22f80a1bf968b7ac87bea5fee0ae6ba",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("management install script missing reviewed source hash %q", want)
		}
	}
}
