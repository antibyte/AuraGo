package storymaker

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadCatalogReadsIcons(t *testing.T) {
	mini := loadMini(t)
	if got := len(mini.IconIDs()); got != 6 {
		t.Fatalf("mini icons = %d, want 6", got)
	}
	cpu, ok := mini.Icon("icon.cpu")
	if !ok || len(cpu.Paths) == 0 || len(cpu.Tags) == 0 {
		t.Fatalf("Icon(icon.cpu) = %+v, %v", cpu, ok)
	}
	if _, ok := mini.Icon("icon.nope"); ok {
		t.Fatal("an unknown icon was found")
	}
	if mini.IconsFile() != "icons.json" {
		t.Fatalf("IconsFile() = %q", mini.IconsFile())
	}
	ids := mini.IconIDs()
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("IconIDs not sorted: %v", ids)
		}
	}

	rc := loadReal(t)
	if n := len(rc.IconIDs()); n < 490 || n > 530 {
		t.Fatalf("real icons = %d, want 490..530", n)
	}
	if rc.IconsFile() != "icons/lucide.json" {
		t.Fatalf("real IconsFile() = %q", rc.IconsFile())
	}

	manifest, err := os.ReadFile("testdata/catalog-mini/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	icons, err := os.ReadFile("testdata/catalog-mini/icons.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, from, to, want string }{
		{"duplicate id", `"id":"icon.database"`, `"id":"icon.cpu"`, "icon.cpu"},
		{"unsorted", `"id":"icon.cpu"`, `"id":"icon.zzz"`, "sorted"},
		{"bad id", `"id":"icon.mail"`, `"id":"icon.Mail"`, "icon.Mail"},
		{"bad schema", `storymaker-icons/1`, `storymaker-icons/9`, "schema"},
		{"bad path", `"M12 20v2"`, `"M12 20 <script>"`, "path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Replace(string(icons), tc.from, tc.to, 1)
			if text == string(icons) {
				t.Fatal("the edit changed nothing")
			}
			fsys := fstest.MapFS{"catalog.json": {Data: manifest}, "icons.json": {Data: []byte(text)}}
			_, err := LoadCatalog(fsys, "catalog.json")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadCatalog error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
