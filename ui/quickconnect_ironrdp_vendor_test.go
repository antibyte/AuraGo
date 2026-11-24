package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIronRDPVendoredLocally guards the Quick Connect RDP client: the backend module must load its wasm from a
// file next to it (the CSP has no data: in connect-src) and the wasm, the component and the licence must ship.
func TestIronRDPVendoredLocally(t *testing.T) {
	dir := filepath.Join("js", "vendor", "ironrdp")
	module, err := os.ReadFile(filepath.Join(dir, "iron-remote-desktop-rdp.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(module), `new URL("./ironrdp_web_bg.wasm", import.meta.url)`) {
		t.Fatal("the rdp module must load ./ironrdp_web_bg.wasm relative to itself")
	}
	if strings.Contains(string(module), "data:application/wasm") {
		t.Fatal("the rdp module must not embed the wasm as a data: URL")
	}
	wasm, err := os.ReadFile(filepath.Join(dir, "ironrdp_web_bg.wasm"))
	if err != nil || !bytes.HasPrefix(wasm, []byte("\x00asm")) {
		t.Fatalf("ironrdp_web_bg.wasm is missing or not a wasm module: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "iron-remote-desktop.js")); err != nil {
		t.Fatal(err)
	}
	license, err := os.ReadFile(filepath.Join(dir, "LICENSE.txt"))
	if err != nil || !strings.Contains(string(license), "MIT OR Apache-2.0") {
		t.Fatalf("LICENSE.txt: %v", err)
	}
}
