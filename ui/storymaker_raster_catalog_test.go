package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"aurago/internal/storymaker"
)

type smRasterFile struct {
	Path          string
	Type          string
	Bytes         int64
	SHA256        string
	Width, Height int
}

func TestStorymakerCatalogLocalizedSearchFindsNewArtwork(t *testing.T) {
	cat, err := storymaker.LoadCatalog(os.DirFS("."), "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	for query, id := range map[string]string{"Professor": "char.professor", "Punk": "char.punk", "Klassenzimmer": "env.classroom", "Wasserflasche": "prop.water-bottle", "車": "prop.car"} {
		found := false
		for _, entry := range cat.Search(query, "", 12) {
			if entry.ID == id {
				found = true
			}
		}
		if !found {
			t.Errorf("catalog search %q missed %s", query, id)
		}
	}
}

type smRasterImage struct {
	File string
	Rect []int
	Box  []float64
}
type smRasterSlot struct {
	Name, Bone, Kind string
	Box              []float64
	Image            smRasterImage
	Variants         map[string]smRasterImage
}

func smRasterFiles(t *testing.T) map[string]smRasterFile {
	t.Helper()
	var manifest struct{ Files []smRasterFile }
	smReadJSON(t, "catalog.json", &manifest)
	result := map[string]smRasterFile{}
	for _, f := range manifest.Files {
		if _, ok := result[f.Path]; ok {
			t.Fatalf("duplicate file %s", f.Path)
		}
		result[f.Path] = f
	}
	return result
}
func smRasterRegion(t *testing.T, files map[string]smRasterFile, image smRasterImage) {
	t.Helper()
	meta, ok := files[image.File]
	if !ok || meta.Type != "image/png" {
		t.Fatalf("unlisted atlas %q", image.File)
	}
	r := image.Rect
	if len(r) == 0 {
		return
	}
	if len(r) != 4 || r[0] < 0 || r[1] < 0 || r[2] < 1 || r[3] < 1 || r[0]+r[2] > meta.Width || r[1]+r[3] > meta.Height {
		t.Fatalf("invalid region %s %v", image.File, r)
	}
}
func smCheckRasterRig(t *testing.T) {
	c, _ := smLoad(t)
	files := smRasterFiles(t)
	var rig struct {
		ID          string
		UnitsHeight float64 `json:"units_height"`
		Bones       []struct {
			Name   string
			Parent *string
			Rest   map[string]struct{ X, Y, Angle float64 }
		}
		Bodies map[string]any
	}
	smReadJSON(t, "rig/human.json", &rig)
	if rig.ID != "rig.human/1" || rig.UnitsHeight != 1000 || len(rig.Bones) != 13 {
		t.Fatalf("invalid shared skeleton: %+v", rig)
	}
	bones := map[string]bool{}
	for i, b := range rig.Bones {
		if b.Name != storymakerBones[i] || b.Parent != nil && !bones[*b.Parent] {
			t.Errorf("invalid bone order: %s", b.Name)
		}
		bones[b.Name] = true
		for _, view := range storymakerViews {
			r, ok := b.Rest[view]
			if !ok || math.IsNaN(r.X) || math.IsNaN(r.Y) || math.IsNaN(r.Angle) {
				t.Errorf("invalid rest frame %s/%s", b.Name, view)
			}
		}
	}
	count := 0
	for _, entry := range c.Entries {
		switch smStr(entry, "kind") {
		case "character":
			count++
			id := smStr(entry, "id")
			var d struct{ Schema, Model, Motions string }
			smReadJSON(t, smStr(entry, "file"), &d)
			if d.Schema != "storymaker-character-3d/1" || files[d.Model].Type != "model/gltf-binary" || files[d.Motions].Type != "application/json" {
				t.Fatalf("invalid model character %s", id)
			}
			resources := map[string]bool{}
			for _, resource := range entry["resources"].([]any) {
				resources[resource.(string)] = true
			}
			if !resources[d.Model] || !resources[d.Motions] {
				t.Errorf("missing model dependencies: %s", id)
			}
			var motions struct {
				Schema string
				Poses  map[string]any
				Clips  []any
			}
			smReadJSON(t, d.Motions, &motions)
			if motions.Schema != "storymaker-model-motions/1" || len(motions.Poses) != 15 || len(motions.Clips) < 2 {
				t.Errorf("incomplete model motion pack: %s", id)
			}
			for _, pose := range storymakerRequired["pose"] {
				if motions.Poses[strings.TrimPrefix(pose, "pose.")] == nil {
					t.Errorf("missing model pose: %s/%s", id, pose)
				}
			}
		case "pose":
			smCheckBones(t, smStr(entry, "id"), smObj(entry, "bones"))
		}
	}
	if count != 3 {
		t.Errorf("got %d fixed characters, want 3", count)
	}
}
func smCheckRasterPropsAndEnvironments(t *testing.T) {
	c, _ := smLoad(t)
	files := smRasterFiles(t)
	props, envs := 0, 0
	for _, e := range c.Entries {
		switch smStr(e, "kind") {
		case "prop":
			props++
			var p struct {
				Viewbox []float64
				Image   smRasterImage
			}
			smReadJSON(t, smStr(e, "file"), &p)
			smRasterRegion(t, files, p.Image)
			if len(p.Viewbox) != 4 || p.Viewbox[2] <= 0 || p.Viewbox[3] <= 0 {
				t.Errorf("invalid prop extent: %s", smStr(e, "id"))
			}
			if h := smNum(smObj(e, "size"), "h"); h <= 0 || h >= 1 {
				t.Errorf("invalid prop size: %s", smStr(e, "id"))
			}
		case "environment":
			envs++
			var d struct {
				Variants map[string][]struct {
					Image      smRasterImage
					Box        []float64
					Parallax   float64
					Foreground bool
				}
				Tokens map[string]map[string]string
			}
			smReadJSON(t, smStr(e, "file"), &d)
			drawn := smStr(e, "drawn")
			for _, variant := range []string{"day", "night"} {
				layers := d.Variants[variant]
				if drawn == "board" {
					// a drawn environment has no bitmap layers: the player paints it
					if len(layers) != 0 || d.Tokens[variant]["text"] == "" || smStr(e, "id") != "env.board" {
						t.Errorf("invalid drawn environment: %s/%s", smStr(e, "id"), variant)
					}
					continue
				}
				if len(layers) < 2 || d.Tokens[variant]["text"] == "" {
					t.Errorf("missing depth or text tokens: %s/%s", smStr(e, "id"), variant)
				}
				for _, layer := range layers {
					smRasterRegion(t, files, layer.Image)
					if len(layer.Box) != 4 || layer.Box[2] <= 0 || layer.Box[3] <= 0 || layer.Parallax <= 0 {
						t.Errorf("invalid environment layer: %s", smStr(e, "id"))
					}
				}
			}
			for _, name := range []string{"left", "center", "right"} {
				if smObj(e, "spots")[name] == nil {
					t.Errorf("missing spot %s/%s", smStr(e, "id"), name)
				}
			}
			floor := smObj(e, "floor")
			if smNum(floor, "y_far") >= smNum(floor, "y_near") || smNum(floor, "scale_far") <= 0 {
				t.Errorf("invalid floor %s", smStr(e, "id"))
			}
			for _, aspect := range []string{"9:16", "1:1"} {
				a := smObj(smObj(e, "aspects"), aspect)
				if a == nil {
					t.Errorf("missing crop %s/%s", smStr(e, "id"), aspect)
				}
			}
		}
	}
	if props != 80 || envs != 13 {
		t.Errorf("got %d props and %d environments", props, envs)
	}
}
func smCheckRasterInventory(t *testing.T) {
	c, _ := smLoad(t)
	files := smRasterFiles(t)
	var paths []string
	var total int64
	err := filepath.WalkDir(storymakerCatalogDir, func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(storymakerCatalogDir, file)
		paths = append(paths, filepath.ToSlash(rel))
		info, err := d.Info()
		if err == nil {
			total += info.Size()
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(files)+1 || len(paths) > 2000 || total > 256<<20 {
		t.Fatalf("catalog delivery budget/inventory: %d files %d bytes", len(paths), total)
	}
	sort.Strings(paths)
	version := sha256.New()
	for _, file := range paths {
		if file == "catalog.json" {
			continue
		}
		meta, ok := files[file]
		if !ok {
			t.Fatalf("unlisted file %s", file)
		}
		data, err := os.ReadFile(filepath.Join(storymakerCatalogDir, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if meta.Bytes != int64(len(data)) || meta.Bytes > 8<<20 || meta.SHA256 != hex.EncodeToString(sum[:]) {
			t.Errorf("incorrect file integrity: %s", file)
		}
		switch meta.Type {
		case "image/png":
			cfg, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil || cfg.Width != meta.Width || cfg.Height != meta.Height || cfg.Width > 3840 || cfg.Height > 2160 {
				t.Errorf("invalid PNG dimensions: %s %v", file, err)
			}
			if strings.HasPrefix(file, "thumbs/") && (cfg.Width > 256 || cfg.Height > 256) {
				t.Errorf("oversized thumbnail %s", file)
			}
		case "application/json":
			if !json.Valid(data) {
				t.Errorf("invalid JSON %s", file)
			}
		case "model/gltf-binary":
			if len(data) < 28 || string(data[:4]) != "glTF" {
				t.Errorf("invalid model %s", file)
			}
		case "text/plain":
			if meta.Width != 0 || meta.Height != 0 {
				t.Errorf("text has image dimensions: %s", file)
			}
		case "audio/wav":
			if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
				t.Errorf("invalid WAV %s", file)
			}
		default:
			t.Errorf("unsupported catalog type %s", meta.Type)
		}
		if filepath.Ext(file) == ".json" {
			version.Write(smLF(data))
		} else {
			version.Write(data)
		}
	}
	if got := hex.EncodeToString(version.Sum(nil))[:12]; got != c.Version {
		t.Errorf("catalog version %s, files %s", c.Version, got)
	}
	for _, entry := range c.Entries {
		if smStr(entry, "kind") != "sfx" {
			thumb := smStr(entry, "thumb")
			if files[thumb].Type != "image/png" {
				t.Errorf("missing PNG thumbnail %s", smStr(entry, "id"))
			}
		}
	}
}
