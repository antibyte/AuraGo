package ui

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGodsEyeLogosAreValidSVG(t *testing.T) {
	for _, path := range []string{
		"img/desktop/store/gods-eye-view.svg",
		"img/papirus/icons/gods-eye-view.svg",
		"img/whitesur/icons/gods-eye-view.svg",
	} {
		t.Run(path, func(t *testing.T) {
			data, err := Content.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var svg struct {
				XMLName xml.Name `xml:"http://www.w3.org/2000/svg svg"`
			}
			if err := xml.Unmarshal(data, &svg); err != nil {
				t.Fatalf("logo is not a valid SVG document: %v", err)
			}
		})
	}
}

func TestGodsEyeDesktopTranslations(t *testing.T) {
	files, err := filepath.Glob("lang/desktop/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 16 {
		t.Fatalf("expected 16 desktop locales, got %d", len(files))
	}
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var words map[string]string
		if err := json.Unmarshal(data, &words); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"gev_description", "gev_configure", "gev_intro", "gev_key_hint", "gev_client_keys", "gev_origins", "gev_origins_hint", "gev_lan_warning", "gev_pending", "gev_key_set", "gev_key_empty"} {
			if strings.TrimSpace(words["desktop.store."+key]) == "" {
				t.Fatalf("%s missing %s", name, key)
			}
		}
	}
	for _, path := range []string{"img/desktop/store/gods-eye-view.svg", "img/desktop/store/gods-eye-view.LICENSE"} {
		if _, err := Content.ReadFile(path); err != nil {
			t.Fatal(err)
		}
	}
}
