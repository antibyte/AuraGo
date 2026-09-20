package virtualcomputers

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestWorkspaceFingerprintIgnoresTestsAndCheckoutLineEndings(t *testing.T) {
	assets := fstest.MapFS{
		"patches/one.patch":                  {Data: []byte("patch\n")},
		"guest_workspace_agent/main.go":      {Data: []byte("package main\nfunc main() {}\n")},
		"guest_workspace_agent/main_test.go": {Data: []byte("old tests\n")},
	}
	want := workspaceRuntimeAssetFingerprint(assets)
	assets["guest_workspace_agent/main_test.go"].Data = []byte("different tests\n")
	assets["guest_workspace_agent/new_test.go"] = &fstest.MapFile{Data: []byte("new tests")}
	for _, file := range assets {
		file.Data = []byte(strings.ReplaceAll(string(file.Data), "\n", "\r\n"))
	}
	if got := workspaceRuntimeAssetFingerprint(assets); got != want {
		t.Fatalf("non-runtime change altered fingerprint: %s != %s", got, want)
	}
	assets["guest_workspace_agent/main.go"].Data = []byte("different runtime\n")
	if workspaceRuntimeAssetFingerprint(assets) == want {
		t.Fatal("runtime change did not invalidate fingerprint")
	}
}

func TestWorkspaceLegacyFingerprintCompatibilityIsBoundToExactRuntime(t *testing.T) {
	current := WorkspaceAssetFingerprint()
	for _, legacy := range []string{"ffb4211a9f999e7c97ee34ff9e89f348c2e5a6417e087d0dd7f9dbab868ad018", "e98a3f8d79bd236c0830883d671b68adfd8b56500d161a4f1be48c594be15786"} {
		if !compatibleWorkspaceAssetFingerprint(current, legacy) {
			t.Fatal("verified test-only migration rejected")
		}
		if compatibleWorkspaceAssetFingerprint("future-runtime", legacy) {
			t.Fatal("legacy compatibility escaped its runtime binding")
		}
	}
	if compatibleWorkspaceAssetFingerprint(current, "unknown") {
		t.Fatal("unknown guest accepted")
	}
}
