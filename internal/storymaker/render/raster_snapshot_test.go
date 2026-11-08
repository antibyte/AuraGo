package render

import (
	"bytes"
	"io/fs"
	"os"
	"strings"
	"testing"

	"aurago/internal/storymaker"
)

type changedRasterFS struct {
	fs.FS
	target string
	data   []byte
}

func (f changedRasterFS) Open(name string) (fs.File, error) {
	if name == f.target {
		return &memFile{name: name, r: bytes.NewReader(f.data), size: int64(len(f.data))}, nil
	}
	return f.FS.Open(name)
}

func TestRasterSnapshotRefusesFilesChangedAfterCatalogVerification(t *testing.T) {
	ui := os.DirFS(uiDirForTests)
	cat, err := storymaker.LoadCatalog(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var target string
	for _, file := range cat.DeliveryFiles() {
		if file.Type == "image/png" {
			target = "img/storymaker/" + file.Path
			break
		}
	}
	data, err := fs.ReadFile(ui, target)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"changed", "truncated", "extended"} {
		t.Run(kind, func(t *testing.T) {
			altered := bytes.Clone(data)
			switch kind {
			case "changed":
				altered[len(altered)-1] ^= 1
			case "truncated":
				altered = altered[:len(altered)-1]
			case "extended":
				altered = append(altered, make([]byte, 1024)...)
			}
			run := &renderRun{r: NewRunner(Deps{UI: changedRasterFS{ui, target, altered}, Catalog: cat}), plan: &Plan{StoryJSON: []byte("{}")}}
			if _, err := run.snapshot(); err == nil || !strings.Contains(err.Error(), "changed before the render snapshot") {
				t.Fatalf("snapshot accepted %s image: %v", kind, err)
			}
		})
	}
}

func TestRasterSnapshotKeepsTheCheckedManifest(t *testing.T) {
	ui := os.DirFS(uiDirForTests)
	cat, err := storymaker.LoadCatalog(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	original, err := fs.ReadFile(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	copy := cat.ManifestJSON()
	copy[0] ^= 1
	if bytes.Equal(copy, cat.ManifestJSON()) {
		t.Fatal("the returned manifest aliases the shared catalog")
	}
	run := &renderRun{r: NewRunner(Deps{UI: changedRasterFS{ui, "img/storymaker/catalog.json", []byte("changed after startup")}, Catalog: cat}), plan: &Plan{StoryJSON: []byte("{}")}}
	snap, err := run.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(snap.CatalogFiles["catalog.json"], original) {
		t.Fatal("the snapshot replaced its checked manifest")
	}
}
