package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCYDCreatorDesignerFontsVendored checks the self-hosted preview fonts of the CYD Creator
// designer (Montserrat 500 and JetBrains Mono 400, latin + latin-ext), their licences and the
// third-party inventory. Inter is already vendored through fonts/fonts.css.
func TestCYDCreatorDesignerFontsVendored(t *testing.T) {
	for _, name := range []string{"montserrat-latin-500-normal.woff2", "montserrat-latin-ext-500-normal.woff2", "jetbrains-mono-latin-400-normal.woff2", "jetbrains-mono-latin-ext-400-normal.woff2"} {
		data, err := Content.ReadFile("fonts/" + name)
		if err != nil {
			t.Errorf("fonts/%s: %v", name, err)
			continue
		}
		if !bytes.HasPrefix(data, []byte("wOF2")) || len(data) < 4096 {
			t.Errorf("fonts/%s is not a WOFF2 font", name)
		}
	}
	for _, name := range []string{"Montserrat-LICENSE.txt", "JetBrainsMono-LICENSE.txt"} {
		data, err := Content.ReadFile("fonts/" + name)
		if err != nil || !strings.Contains(strings.ToLower(string(data)), "open font license") {
			t.Errorf("fonts/%s is missing or not the OFL text", name)
		}
	}
	if !strings.Contains(readDesktopAssetText(t, "fonts/fonts.css"), "font-family: 'Inter'") {
		t.Error("fonts/fonts.css no longer declares Inter")
	}
	inventory, err := os.ReadFile(filepath.Join("..", "THIRD_PARTY_LIBRARIES.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []string{"| Montserrat |", "| JetBrains Mono |"} {
		if !strings.Contains(string(inventory), row) {
			t.Errorf("THIRD_PARTY_LIBRARIES.md misses the %s row", row)
		}
	}
}
