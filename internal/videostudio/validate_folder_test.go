package videostudio

import (
	"strconv"
	"strings"
	"testing"
)

func folderTestProject() Project {
	return Project{
		Version: CurrentVersion, Name: "Folders", Width: 1280, Height: 720, FPS: FramesPerSecond,
		Assets:  []Asset{{ID: "00000000-0000-4000-8000-000000000001", Name: "photo.png", Folder: "Trips/2024", Kind: AssetImage, Width: 10, Height: 10}},
		Tracks:  []Track{},
		Folders: []string{"Empty/Folder"},
	}
}

func TestNormalizeFolderRules(t *testing.T) {
	long := strings.Repeat("x", 64)
	valid := map[string]string{
		"":               "",
		"  ":             "",
		"Trips":          "Trips",
		" Trips / 2024 ": "Trips/2024",
		"a/b/c/d":        "a/b/c/d",
		long:             long,
	}
	for raw, want := range valid {
		got, err := NormalizeFolder(raw)
		if err != nil || got != want {
			t.Fatalf("NormalizeFolder(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	tooLongPath := long + "/" + long + "/" + long + "/" + long
	invalid := []string{"a//b", "a/ /b", "/a", "a/", ".", "a/..", "a/b/c/d/e", strings.Repeat("x", 65), "a\x07b", "a\x7fb", tooLongPath}
	for _, raw := range invalid {
		if _, err := NormalizeFolder(raw); err == nil {
			t.Fatalf("NormalizeFolder(%q) accepted an invalid folder", raw)
		}
	}
}

func TestValidateAssetFieldsAndProjectFolders(t *testing.T) {
	if err := Validate(folderTestProject()); err != nil {
		t.Fatalf("valid folder project rejected: %v", err)
	}
	cases := map[string]func(*Project){
		"non-canonical asset folder": func(p *Project) { p.Assets[0].Folder = "Trips/" },
		"untrimmed asset name":       func(p *Project) { p.Assets[0].Name = " photo.png" },
		"duplicate project folder":   func(p *Project) { p.Folders = []string{"A", "A"} },
		"invalid project folder":     func(p *Project) { p.Folders = []string{"a//b"} },
		"too many project folders": func(p *Project) {
			p.Folders = nil
			for i := 0; i <= MaxProjectFolders; i++ {
				p.Folders = append(p.Folders, "f"+strconv.Itoa(i))
			}
		},
	}
	for name, mutate := range cases {
		project := folderTestProject()
		mutate(&project)
		if err := Validate(project); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}
