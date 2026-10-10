package cydcreator

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var templateStringKeyPattern = regexp.MustCompile(`^[A-Z0-9_]+$`)

func TestNewProjectStampsTemplateVersion(t *testing.T) {
	p, err := NewProject("Test", "esp32-2432s028r", "night", "de", "CET-1CEST,M3.5.0,M10.5.0/3", &SeqIDs{})
	if err != nil {
		t.Fatal(err)
	}
	if p.TemplateVersion != TemplateVersion {
		t.Fatalf("NewProject stamped %q, want %q", p.TemplateVersion, TemplateVersion)
	}
}

func TestDeviceLanguage(t *testing.T) {
	for _, l := range Languages {
		got := DeviceLanguage(l)
		switch l {
		case "el", "hi", "ja", "zh":
			if got != "en" {
				t.Errorf("DeviceLanguage(%q) = %q, want en", l, got)
			}
		default:
			if got != l {
				t.Errorf("DeviceLanguage(%q) = %q, want %q", l, got, l)
			}
		}
	}
}

func TestTemplateStringsComplete(t *testing.T) {
	f, err := loadTemplateStrings()
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Keys) == 0 {
		t.Fatal("no keys")
	}
	seen := make(map[string]bool, len(f.Keys))
	for _, k := range f.Keys {
		if seen[k] {
			t.Errorf("duplicate key %q", k)
		}
		seen[k] = true
		if !templateStringKeyPattern.MatchString(k) {
			t.Errorf("key %q does not match %s", k, templateStringKeyPattern)
		}
	}
	if len(f.Strings) != len(DeviceLanguages) {
		t.Fatalf("strings.json has %d locales, want %d", len(f.Strings), len(DeviceLanguages))
	}
	for _, lang := range DeviceLanguages {
		table, ok := f.Strings[lang]
		if !ok {
			t.Errorf("locale %s missing", lang)
			continue
		}
		if len(table) != len(f.Keys) {
			t.Errorf("%s has %d strings, want %d", lang, len(table), len(f.Keys))
		}
		for _, k := range f.Keys {
			v := strings.TrimSpace(table[k])
			if v == "" {
				t.Errorf("%s.%s is empty", lang, k)
			}
			// Portal strings are embedded in HTML and every string in C source.
			if strings.ContainsAny(v, "<>&\"\\") {
				t.Errorf("%s.%s contains one of < > & \" \\", lang, k)
			}
		}
	}
	got, err := TemplateStrings("ja")
	if err != nil || len(got) != len(f.Keys) || got[0] != f.Strings["en"][f.Keys[0]] {
		t.Fatalf("TemplateStrings(ja) = %q, %v; want the English table", got, err)
	}
	if !strings.Contains(f.Strings["de"]["OTA_CERT_CHANGED"], "bitte per USB neu flashen") {
		t.Fatal("German certificate message must match spec §9")
	}
}

func TestCStringLiteral(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"quote", "\"", `"\""`},
		{"backslash", `\`, `"\\"`},
		{"newline", "\n", `"\n"`},
		{"non-ASCII stays raw", "é…", `"é…"`},
		{"trigraph", "??=", `"?\?="`},
		{"single question mark", "a?b", `"a?b"`},
		{"question mark run", "???", `"?\?\?"`},
		{"carriage return", "\r", `"\015"`},
		{"nul", "\x00", `"\000"`},
		{"del", "\x7f", `"\177"`},
	}
	for _, tc := range cases {
		if got := cStringLiteral(tc.in); got != tc.want {
			t.Errorf("%s: cStringLiteral(%q) = %s, want %s", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestTemplateStringsHeaderUpToDate(t *testing.T) {
	f, err := loadTemplateStrings()
	if err != nil {
		t.Fatal(err)
	}
	want := renderStringsHeader(f)
	const path = "template/lib/cyd_template/config/cyd_strings.h"
	if os.Getenv("CYD_UPDATE_STRINGS") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s is stale; run CYD_UPDATE_STRINGS=1 go test ./internal/cydcreator/ -run TestTemplateStringsHeaderUpToDate", path)
	}
}
