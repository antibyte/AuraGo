package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSpritePackContent(t *testing.T) {
	s := newTestService(t)
	packs, err := s.ListAssetPacks()
	packs = slices.DeleteFunc(packs, func(p AssetPackSummary) bool { return p.Kind == "model3d" || presentationPack(p.ID) })
	if err != nil || len(packs) != 18 {
		t.Fatalf("catalog: %d packs, %v", len(packs), err)
	}
	for _, summary := range packs {
		t.Run(summary.ID, func(t *testing.T) {
			pack, err := s.DescribeAssetPack(summary.ID)
			if err != nil {
				t.Fatal(err)
			}
			if pack.SchemaVersion != 1 || pack.Version != "2" || pack.Columns != 10 || pack.Rows != 10 || pack.FrameWidth != 64 || pack.FrameHeight != 64 || pack.Image != "sheet.png" {
				t.Fatal("invalid grid contract")
			}
			data, err := s.AssetPackFile(summary.ID, "sheet.png")
			if err != nil {
				t.Fatal(err)
			}
			// PNG color type 6 is RGBA, not an opaque RGB image painted like alpha.
			if len(data) < 26 || data[25] != 6 {
				t.Fatal("sheet must be RGBA PNG")
			}
			im, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if im.Bounds().Dx() != 640 || im.Bounds().Dy() != 640 {
				t.Fatal("sheet must be 640x640")
			}
			var frames []struct {
				Index, X, Y, W, H int
				AssetID           string `json:"asset_id"`
			}
			var assets []struct {
				ID, Name, Description, View, Direction string
				Tags                                   []string
				Frames                                 []int
				Tile                                   bool
				AssemblyPart                           bool `json:"assembly_part"`
				Origin                                 map[string]float64
			}
			var animations []struct {
				ID        string
				AssetID   string `json:"asset_id"`
				Frames    []int
				FrameRate int `json:"frame_rate"`
				Repeat    int
			}
			for _, pair := range []struct {
				data   []byte
				target any
			}{{pack.Frames, &frames}, {pack.Assets, &assets}, {pack.Animations, &animations}} {
				if err := json.Unmarshal(pair.data, pair.target); err != nil {
					t.Fatal(err)
				}
			}
			if len(frames) != 100 {
				t.Fatalf("frame count = %d", len(frames))
			}
			assetIDs, used := map[string]bool{}, map[int]bool{}
			for _, a := range assets {
				if a.ID == "" || assetIDs[a.ID] || a.Name == "" || a.Description == "" || a.View == "" || a.Direction == "" || len(a.Tags) == 0 || len(a.Frames) == 0 || len(a.Origin) != 2 {
					t.Fatalf("incomplete/duplicate asset: %+v", a)
				}
				assetIDs[a.ID] = true
				for _, index := range a.Frames {
					if index < 0 || index >= 100 || used[index] {
						t.Fatalf("duplicate/invalid asset frame %d", index)
					}
					used[index] = true
					f := frames[index]
					if f.Index != index || f.AssetID != a.ID || f.X != index%10*64 || f.Y != index/10*64 || f.W != 64 || f.H != 64 {
						t.Fatalf("invalid frame %+v", f)
					}
					opaque, transparent := 0, 0
					for y := 0; y < 64; y++ {
						for x := 0; x < 64; x++ {
							_, _, _, alpha := im.At(f.X+x, f.Y+y).RGBA()
							if alpha == 0 {
								transparent++
							} else {
								opaque++
								if !a.Tile && !a.AssemblyPart && (x == 0 || y == 0 || x == 63 || y == 63) {
									t.Fatalf("asset touches cell edge: %s frame %d", a.ID, index)
								}
							}
						}
					}
					if opaque == 0 || (!a.Tile && !a.AssemblyPart && transparent == 0) {
						t.Fatalf("empty/opaque asset: %s frame %d", a.ID, index)
					}
				}
			}
			if len(used) != 100 {
				t.Fatal("not every cell is described")
			}
			animationIDs := map[string]bool{}
			for _, a := range animations {
				if a.ID == "" || animationIDs[a.ID] || !assetIDs[a.AssetID] || len(a.Frames) == 0 || a.FrameRate <= 0 || a.Repeat < -1 {
					t.Fatalf("invalid animation %+v", a)
				}
				animationIDs[a.ID] = true
				for _, f := range a.Frames {
					if !used[f] || frames[f].AssetID != a.AssetID {
						t.Fatalf("invalid animation frame %d", f)
					}
				}
			}
		})
	}
}

func TestSpriteGuardIsBundledOnlyForPhaser(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			root := t.TempDir()
			if err := WriteScaffold(root, Project{Name: "Guard", Dimension: dimension}); err != nil {
				t.Fatal(err)
			}
			if result := buildDirectory(context.Background(), root, 100, 64*1024*1024); !result.OK {
				t.Fatalf("build: %+v", result)
			}
			bundle, err := os.ReadFile(filepath.Join(root, "dist", "game.js"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(bundle), phaserSpriteGuard) != (dimension == "2d") {
				t.Fatalf("incorrect sprite guard for %s", dimension)
			}
		})
	}
}

func TestSpritePackImportWritePreservesSource(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(testRunner{service: s, mutate: func(ctx context.Context, run JobRun) error {
		original, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		pack := "../assets/builtin/blocks-and-balls/2/sheet.json"
		if err := s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", "import meta from '"+pack+"';\n"+original); err == nil || !strings.Contains(err.Error(), "No complete packs") {
			return fmt.Errorf("unimported pack accepted: %v", err)
		}
		if _, err := s.ImportAssetPack(ctx, run.Job.ID, "blocks-and-balls"); err != nil {
			return err
		}
		original, err = s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		s.mu.RLock()
		preview := s.previewCheck
		s.mu.RUnlock()
		for _, statement := range []string{
			"import meta from '../assets/builtin/tile-bits/1/sheet.json';",
			"import meta from '../../assets/builtin/tile-bits/1/sheet.json';",
			"import meta from '../../assets/builtin/blocks-and-balls/2/sheet.json';",
			"import meta from '../assets/builtin/blocks-and-balls/1/sheet.json';",
			"export {default as meta} from '../assets/builtin/tile-bits/1/sheet.json';",
			"const meta = require('../assets/builtin/tile-bits/1/sheet.json');",
			"void import('../assets/builtin/tile-bits/1/sheet.json');",
			`import meta from '../assets/\u0062uiltin/tile-bits/1/sheet.json';`,
		} {
			if err := s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", statement+"\n"+original); err == nil || !strings.Contains(err.Error(), pack) || !strings.Contains(err.Error(), "File unchanged") {
				return fmt.Errorf("missing concrete import correction for %s: %v", statement, err)
			}
			if current, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts"); err != nil || current != original {
				return fmt.Errorf("rejected write changed source: %v", err)
			}
		}
		s.mu.RLock()
		unchanged := s.previewCheck == preview && s.validationFailures[run.Job.ID] == 0
		s.mu.RUnlock()
		if !unchanged {
			return fmt.Errorf("rejected write invalidated preview or consumed a repair")
		}
		// Resolve relative to each source file, including nested modules.
		for _, file := range []string{"src/scene.ts", "src/nested/scene.mjs", "src/scene.tsx", "scene.js"} {
			prefix := "../"
			if strings.Contains(file, "nested") {
				prefix = "../../"
			} else if file == "scene.js" {
				prefix = "./"
			}
			code := "import meta from '" + prefix + "assets/builtin/blocks-and-balls/2/sheet.json'; export default meta;"
			if err := s.WriteJobFile(ctx, run.Job.ID, file, code); err != nil {
				return err
			}
		}
		stage, _ := s.JobDirectory(run.Job.ID)
		image := filepath.Join(stage, "assets/builtin/blocks-and-balls/2/sheet.png")
		png, _ := os.ReadFile(image)
		if err := os.Remove(image); err != nil {
			return err
		}
		if err := s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", "import meta from '"+pack+"';\n"+original); err == nil {
			return fmt.Errorf("incomplete pack accepted")
		}
		if err := os.WriteFile(image, png, 0o640); err != nil {
			return err
		}
		// Legacy versions are project-owned; the catalog's current version must
		// not make their existing imports invalid.
		legacy := filepath.Join(stage, "assets/builtin/blocks-and-balls/1")
		if err := os.MkdirAll(legacy, 0o750); err != nil {
			return err
		}
		for _, file := range []string{"sheet.json", "sheet.png"} {
			data, err := os.ReadFile(filepath.Join(filepath.Dir(image), file))
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(legacy, file), data, 0o640); err != nil {
				return err
			}
		}
		if err := s.WriteJobFile(ctx, run.Job.ID, "src/legacy.ts", "export {default} from '../assets/builtin/blocks-and-balls/1/sheet.json';"); err != nil {
			return err
		}
		// Text mentioning a path is not an import.
		return s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", "// import meta from '../assets/builtin/tile-bits/1/sheet.json';\nconst example = \"import '../assets/builtin/tile-bits/1/sheet.json'\";\n"+original)
	}})
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "ready" {
		t.Fatalf("job failed: %+v", done)
	}
}

func TestSpritePackSelectionImportAndOfflineExport(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	packs, _ := s.ListAssetPacks()
	packs = slices.DeleteFunc(packs, func(p AssetPackSummary) bool { return p.Kind == "model3d" || presentationPack(p.ID) })
	ids := []string{}
	for _, p := range packs {
		ids = append(ids, p.ID)
	}
	s.SetRunner(testRunner{service: s, mutate: func(ctx context.Context, run JobRun) error {
		if len(run.AssetPacks) != 18 {
			return fmt.Errorf("agent saw %d imports", len(run.AssetPacks))
		}
		for _, pack := range run.AssetPacks {
			metadata, err := s.DescribeAssetPack(pack.ID)
			if err != nil {
				return err
			}
			guidance := []string{"aurago-game-1.js", "preloadPack(this, meta,", "registerAnimations(this, meta)", "../" + pack.Metadata, pack.Image}
			if len(metadata.Assemblies) > 0 {
				guidance = append(guidance, "createAssembly(this, meta,")
				if strings.Contains(pack.PhaserExample, "createAsset(this, meta,") {
					return fmt.Errorf("import %s renders an isolated assembly fragment", pack.ID)
				}
			} else {
				guidance = append(guidance, "createAsset(this, meta,")
			}
			for _, required := range guidance {
				if !strings.Contains(pack.PhaserExample, required) {
					return fmt.Errorf("import %s omitted usage guidance %q", pack.ID, required)
				}
			}
			for _, path := range []string{pack.Image, pack.Metadata} {
				stage, err := s.JobDirectory(run.Job.ID)
				if err != nil {
					return err
				}
				if _, err := os.Stat(filepath.Join(stage, filepath.FromSlash(path))); err != nil {
					return err
				}
			}
			if _, err := s.ImportAssetPack(ctx, run.Job.ID, pack.ID); err != nil {
				return err
			}
		}
		var count int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM gm_assets WHERE job_id=?`, run.Job.ID).Scan(&count); err != nil {
			return err
		}
		if count != 2*len(packs) {
			return fmt.Errorf("idempotent import recorded %d files", count)
		}
		inventory, err := s.importedJobPacks(ctx, run.Job.ID)
		if err != nil || len(inventory) != len(packs) {
			return fmt.Errorf("project import inventory incomplete: %d, %v", len(inventory), err)
		}
		for _, copy := range inventory {
			if copy.Version != "2" || copy.Image == "" || copy.Metadata == "" {
				return fmt.Errorf("invalid imported context: %+v", copy)
			}
		}
		return nil
	}})
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{AssetPackIDs: append(ids, ids[0])})
	if err == nil {
		t.Fatal("more selections than available packs accepted")
	}
	job, err = s.StartJob(context.Background(), project.ID, StartJobRequest{AssetPackIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "ready" {
		t.Fatalf("job failed: %+v", done)
	}
	if _, err := s.RestoreRevision(context.Background(), project.ID, 1); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if _, err := s.WriteExport(context.Background(), project.ID, &output); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = data
	}
	for _, id := range ids {
		for _, filename := range []string{"sheet.png", "sheet.json"} {
			want, _ := s.AssetPackFile(id, filename)
			path := "assets/builtin/" + id + "/2/" + filename
			if !bytes.Equal(files[path], want) {
				t.Fatalf("exported project copy differs: %s", path)
			}
		}
	}
}

func TestSpritePackImportFailuresLeaveNoPartialPair(t *testing.T) {
	for _, kind := range []string{"unknown", "disabled", "readonly", "edit_disabled", "asset_limit", "file_limit", "count_limit", "project_limit", "incomplete", "modified", "cancelled", "ledger_failure"} {
		t.Run(kind, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			s.SetRunner(testRunner{service: s, mutate: func(ctx context.Context, run JobRun) error {
				stage, _ := s.JobDirectory(run.Job.ID)
				target := filepath.Join(stage, "assets", "builtin", "space-shooter", "2")
				id := "space-shooter"
				switch kind {
				case "unknown":
					id = "../space-shooter"
				case "disabled":
					s.UpdatePolicy(Policy{})
				case "readonly":
					s.UpdatePolicy(Policy{Enabled: true, ReadOnly: true, AllowEdit: true})
				case "edit_disabled":
					s.UpdatePolicy(Policy{Enabled: true})
				case "asset_limit":
					s.opts.MaxAssetBytes = 1
				case "file_limit":
					s.opts.MaxFileBytes = 1
				case "count_limit":
					s.opts.MaxFilesPerProject = 1
				case "project_limit":
					s.opts.MaxProjectBytes = 1
				case "cancelled":
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				case "ledger_failure":
					if _, err := s.db.Exec(`CREATE TRIGGER reject_sprite BEFORE INSERT ON gm_assets BEGIN SELECT RAISE(ABORT,'test failure'); END`); err != nil {
						return err
					}
				case "incomplete", "modified":
					if err := os.MkdirAll(target, 0750); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(target, "sheet.png"), []byte("keep"), 0640); err != nil {
						return err
					}
					if kind == "modified" {
						if err := os.WriteFile(filepath.Join(target, "sheet.json"), []byte("keep"), 0640); err != nil {
							return err
						}
					}
				}
				_, err := s.ImportAssetPack(ctx, run.Job.ID, id)
				if err == nil {
					return fmt.Errorf("%s import unexpectedly succeeded", kind)
				}
				if kind == "incomplete" || kind == "modified" {
					data, _ := os.ReadFile(filepath.Join(target, "sheet.png"))
					if string(data) != "keep" {
						return errors.New("existing file was overwritten")
					}
				} else if _, err := os.Stat(target); !os.IsNotExist(err) {
					return fmt.Errorf("partial destination remains: %v", err)
				}
				return errors.New("fixture finished")
			}})
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if done := waitJob(t, s, job.ID); done.Error != "fixture finished" {
				t.Fatalf("failure fixture: %+v", done)
			}
		})
	}
}
