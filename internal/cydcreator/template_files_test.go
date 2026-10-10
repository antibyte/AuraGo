package cydcreator

import (
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
