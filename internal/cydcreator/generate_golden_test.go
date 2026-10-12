package cydcreator

import (
	"bytes"
	"flag"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite internal/cydcreator/testdata/golden from the generator")

// goldenDir holds one directory per golden case: the input
// cyd.project.json and the expected generator output next to it.
const goldenDir = "testdata/golden"

// loadGoldenProject reads a golden input; with -update it rewrites the file
// in MarshalProject's canonical form first.
func loadGoldenProject(t *testing.T, dir string) Project {
	t.Helper()
	path := filepath.Join(dir, "cyd.project.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := MarshalProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, raw) {
		if !*updateGolden {
			t.Fatalf("%s is not canonical; run go test ./internal/cydcreator/ -run TestGenerateGolden -update", path)
		}
		if err := os.WriteFile(path, canonical, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func generateGolden(t *testing.T, p Project) GenerateOutput {
	t.Helper()
	out, err := Generate(GenerateInput{Project: p, ProjectID: testProjectID, AppFiles: ScaffoldAppFiles(p), Assets: goldenAssets(t)})
	if err != nil {
		t.Fatalf("Generate: %v\n%s", err, fmtIssues(out.Issues))
	}
	return out
}

// TestGenerateGolden compares the generator output of every project below
// testdata/golden/<case>/cyd.project.json with the files next to it.
// Regenerate after an intended change with:
//
//	go test ./internal/cydcreator/ -run TestGenerateGolden -update
func TestGenerateGolden(t *testing.T) {
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	var cases []string
	for _, e := range entries {
		if e.IsDir() {
			cases = append(cases, e.Name())
		}
	}
	wantCases := []string{"all_widgets", "all_widgets_portrait", "theme_clear", "theme_dark_sun", "theme_high_contrast",
		"theme_night", "theme_sandstorm", "theme_synthwave", "theme_terminal"}
	if !slices.Equal(cases, wantCases) {
		t.Fatalf("golden cases %v, want %v", cases, wantCases)
	}
	// Every theme of themes.json has its theme_<id> case and no case names a
	// theme that is gone, so a new theme cannot ship without a golden.
	themes, err := Themes()
	if err != nil {
		t.Fatal(err)
	}
	var themeCases, wantThemeCases []string
	for _, name := range cases {
		if strings.HasPrefix(name, "theme_") {
			themeCases = append(themeCases, name)
		}
	}
	for _, th := range themes {
		wantThemeCases = append(wantThemeCases, "theme_"+th.ID)
	}
	slices.Sort(themeCases)
	slices.Sort(wantThemeCases)
	if !slices.Equal(themeCases, wantThemeCases) {
		t.Fatalf("theme golden cases %v, want one per Themes() entry %v", themeCases, wantThemeCases)
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(goldenDir, name)
			p := loadGoldenProject(t, dir)
			out := generateGolden(t, p)
			for _, is := range out.Issues {
				t.Errorf("golden project has issue %s %s at %s: %s", is.Severity, is.Code, is.Path, is.Message)
			}
			for _, rel := range genOutputFiles {
				path := filepath.Join(dir, filepath.FromSlash(rel))
				if *updateGolden {
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, out.Files[rel], 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("%v (run with -update to create the golden files)", err)
				}
				if !bytes.Equal(out.Files[rel], want) {
					t.Errorf("%s differs from the generator output; inspect, then run -update:\n%s", rel, firstDiff(string(want), string(out.Files[rel])))
				}
			}
			var got []string
			err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(dir, path)
				got = append(got, filepath.ToSlash(rel))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			want := append([]string{"cyd.project.json"}, genOutputFiles...)
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("golden directory holds %v, want %v", got, want)
			}
		})
	}
}

// TestGenerateDeterministic generates the all-widgets project repeatedly from
// freshly parsed layouts (new maps each time, so Go's randomised map order
// is exercised) and requires identical trees.
func TestGenerateDeterministic(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(goldenDir, "all_widgets", "cyd.project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var first map[string][]byte
	for i := 0; i < 20; i++ {
		p, err := ParseProject(raw)
		if err != nil {
			t.Fatal(err)
		}
		out := generateGolden(t, p)
		if first == nil {
			first = out.Files
			continue
		}
		if !maps.EqualFunc(first, out.Files, bytes.Equal) {
			for k := range first {
				if !bytes.Equal(first[k], out.Files[k]) {
					t.Fatalf("run %d: %s differs:\n%s", i, k, firstDiff(string(first[k]), string(out.Files[k])))
				}
			}
			t.Fatalf("run %d: file sets differ", i)
		}
	}
}

// TestAllWidgetsGoldenCoversCatalog keeps the all-widgets golden complete:
// every catalog type, event and action kind appears in it.
func TestAllWidgetsGoldenCoversCatalog(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(goldenDir, "all_widgets", "cyd.project.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	types, events, actions := map[string]bool{}, map[string]bool{}, map[string]bool{}
	named, unnamed, hidden, tabbed := 0, 0, 0, 0
	p.WalkWidgets(func(_ *Screen, parent *Widget, w *Widget) bool {
		types[w.Type] = true
		if w.Name != "" {
			named++
		} else {
			unnamed++
		}
		if w.Hidden {
			hidden++
		}
		if parent != nil && parent.Type == "tabview" {
			tabbed++
		}
		for _, ev := range w.Events {
			events[ev.On] = true
			actions[ev.Action.Type] = true
		}
		return true
	})
	for _, spec := range WidgetCatalog() {
		if !types[spec.Type] {
			t.Errorf("all_widgets lacks a %s widget", spec.Type)
		}
	}
	for _, on := range []string{"clicked", "long_pressed", "value_changed", "ready"} {
		if !events[on] {
			t.Errorf("all_widgets lacks a %s event", on)
		}
	}
	for _, a := range actionTypes {
		if !actions[a] {
			t.Errorf("all_widgets lacks a %s action", a)
		}
	}
	if named == 0 || unnamed == 0 || hidden == 0 || tabbed == 0 || len(p.Assets) != 2 {
		t.Errorf("all_widgets must mix named/unnamed/hidden/tabbed widgets and two assets: %d/%d/%d/%d/%d",
			named, unnamed, hidden, tabbed, len(p.Assets))
	}
}
