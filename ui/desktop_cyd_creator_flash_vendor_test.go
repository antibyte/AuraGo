package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCYDCreatorEsptoolVendored checks the vendored esptool-js entry of plan 5: the manifest lists the
// package and the bundle hash, the bundle is a self-contained ES module exporting ESPLoader and
// Transport, and licence, README and the third-party inventory are in place.
func TestCYDCreatorEsptoolVendored(t *testing.T) {
	var manifest struct {
		Packages map[string]struct {
			Version string `json:"version"`
			License string `json:"license"`
		} `json:"packages"`
		Files map[string]struct {
			Bytes  int    `json:"bytes"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	data, err := os.ReadFile(filepath.Join("js", "vendor", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if pkg := manifest.Packages["esptool-js"]; pkg.Version != "0.7.0" || pkg.License != "Apache-2.0" {
		t.Fatalf("manifest esptool-js = %+v", pkg)
	}
	bundle, err := os.ReadFile(filepath.Join("js", "vendor", "esptool-js", "esptool-js.min.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bundle)
	if entry := manifest.Files["ui/js/vendor/esptool-js/esptool-js.min.mjs"]; entry.SHA256 != hex.EncodeToString(sum[:]) || entry.Bytes != len(bundle) {
		t.Fatalf("manifest entry %+v does not match the bundle", entry)
	}
	if !regexp.MustCompile(`export\{[^}]*\bESPLoader\b[^}]*\}`).Match(bundle) || !regexp.MustCompile(`export\{[^}]*\bTransport\b[^}]*\}`).Match(bundle) {
		t.Fatal("the bundle must export ESPLoader and Transport")
	}
	if strings.Contains(string(bundle), "import(") {
		t.Fatal("the bundle must be self-contained (no dynamic imports)")
	}
	license, err := os.ReadFile(filepath.Join("js", "vendor", "esptool-js", "LICENSE.txt"))
	if err != nil || !strings.Contains(string(license), "Apache License") {
		t.Fatalf("LICENSE.txt: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join("js", "vendor", "esptool-js", "README.md"))
	if err != nil || !strings.Contains(string(readme), "esptool-js) 0.7.0") {
		t.Fatalf("README.md: %v %s", err, readme)
	}
	inventory, err := os.ReadFile(filepath.Join("..", "THIRD_PARTY_LIBRARIES.md"))
	if err != nil || !strings.Contains(string(inventory), "| esptool-js 0.7.0 | `ui/js/vendor/esptool-js/esptool-js.min.mjs`") {
		t.Fatal("THIRD_PARTY_LIBRARIES.md misses the esptool-js row")
	}
}
