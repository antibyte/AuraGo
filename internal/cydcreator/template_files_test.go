package cydcreator

import (
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"testing"
)

// templateFiles is the complete embedded template tree. A new runtime file
// must be added here, so the generator (plan 1c) and the builder image never
// miss one.
var templateFiles = []string{
	"lib/cyd_template/config/cyd_strings.h",
	"lib/cyd_template/config/cyd_wm_strings.h",
	"lib/cyd_template/config/lv_conf.h",
	"lib/cyd_template/library.json",
	"lib/cyd_template/src/cyd.h",
	"lib/cyd_template/src/cyd_board.cpp",
	"lib/cyd_template/src/cyd_build_info.cpp",
	"lib/cyd_template/src/cyd_config.h",
	"lib/cyd_template/src/cyd_display.cpp",
	"lib/cyd_template/src/cyd_log.cpp",
	"lib/cyd_template/src/cyd_portal.cpp",
	"lib/cyd_template/src/cyd_private.h",
	"lib/cyd_template/src/cyd_screens.cpp",
	"lib/cyd_template/src/cyd_storage.cpp",
	"lib/cyd_template/src/cyd_touchcal.cpp",
	"lib/cyd_template/src/cyd_wifi.cpp",
	"lib/cyd_template/src/cyd_wifi_ui.cpp",
	"strings.json",
}

func TestTemplateFSContainsExpectedFiles(t *testing.T) {
	var got []string
	err := fs.WalkDir(TemplateFS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			got = append(got, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := slices.Clone(templateFiles)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("embedded template tree mismatch\n got: %q\nwant: %q", got, want)
	}
	for _, p := range got {
		data, err := fs.ReadFile(TemplateFS(), p)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", p)
		}
		if strings.Contains(string(data), "\r") {
			t.Errorf("%s has CR line endings", p)
		}
	}
}

func TestTemplateVersionMatchesLibraryJSON(t *testing.T) {
	data, err := fs.ReadFile(TemplateFS(), "lib/cyd_template/library.json")
	if err != nil {
		t.Fatal(err)
	}
	var lib struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Build   struct {
			LibArchive *bool `json:"libArchive"`
		} `json:"build"`
		Dependencies any `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &lib); err != nil {
		t.Fatal(err)
	}
	if lib.Name != "cyd_template" || lib.Version != TemplateVersion {
		t.Fatalf("library.json = %s %s, want cyd_template %s", lib.Name, lib.Version, TemplateVersion)
	}
	// libArchive=false links every object directly, so the strong
	// verifyRollbackLater() and setup()/loop() always win over weak defaults.
	if lib.Build.LibArchive == nil || *lib.Build.LibArchive {
		t.Fatal("library.json must set build.libArchive to false")
	}
	// Builds are offline: a dependency here would make PlatformIO try the registry.
	if lib.Dependencies != nil {
		t.Fatal("library.json must not declare dependencies (platformio.ini lib_deps pins them)")
	}
}
