package gamemaker

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestModelPackCatalogIntegrity(t *testing.T) {
	s := newTestService(t)
	packs, err := s.ListAssetPacks()
	if err != nil || len(packs) != 19 {
		t.Fatalf("catalog: %d, %v", len(packs), err)
	}
	m, err := readModelManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Assets) != 220 || m.Version != "1.0.0" {
		t.Fatal("incomplete 3D catalog")
	}
	want := map[string]int{"road": 18, "aircraft": 8, "space": 22, "architecture": 42, "landscape": 28, "vegetation": 22, "props": 36, "humans": 12, "animals": 12, "fps": 20}
	counts := map[string]int{}
	checked := map[string]bool{}
	var total int64
	for _, a := range m.Assets {
		counts[a.Category]++
		if a.Up != "+Y" || a.Forward != "+Z" || a.Units != "metres" || len(a.LODs) == 0 {
			t.Fatalf("invalid model contract: %s", a.ID)
		}
		if a.Category == "humans" && (a.Rig != "humanoid-v1" || len(a.Animations) != 34 || a.AnimationLibrary == nil) {
			t.Fatal(a.ID, "human rig contract")
		}
		for _, f := range modelFiles(a) {
			if checked[f.File] {
				continue
			}
			checked[f.File] = true
			data, err := s.AssetPackFile(m.ID, f.File)
			if err != nil {
				t.Fatal(err)
			}
			total += int64(len(data))
			if int64(len(data)) != f.Bytes || sha256Bytes(data) != f.SHA256 {
				t.Fatal("checksum", f.File)
			}
			if len(data) < 20 || binary.LittleEndian.Uint32(data) != 0x46546c67 || binary.LittleEndian.Uint32(data[4:]) != 2 {
				t.Fatal("GLB", f.File)
			}
			size := int(binary.LittleEndian.Uint32(data[12:]))
			if size < 2 || 20+size > len(data) {
				t.Fatal("GLB JSON size", f.File)
			}
			var doc struct {
				Nodes      []struct{ Name string }
				Animations []struct{ Name string }
				Buffers    []struct{ URI string }
				Images     []struct{ URI string }
			}
			if err := json.Unmarshal(data[20:20+size], &doc); err != nil {
				t.Fatal(err)
			}
			for _, b := range doc.Buffers {
				if b.URI != "" {
					t.Fatal("external buffer")
				}
			}
			for _, b := range doc.Images {
				if b.URI != "" {
					t.Fatal("external image")
				}
			}
			if f.File == a.LODs[0].File && a.AnimationLibrary == nil {
				for _, c := range a.Animations {
					if !slices.ContainsFunc(doc.Animations, func(v struct{ Name string }) bool { return v.Name == c.ID }) {
						t.Fatal(a.ID, c.ID)
					}
				}
			}
		}
		preview, err := s.AssetPackFile(m.ID, a.Preview)
		if err != nil || len(preview) < 12 || string(preview[8:12]) != "WEBP" {
			t.Fatalf("preview %s: %v", a.ID, err)
		}
		total += int64(len(preview))
	}
	for k, n := range want {
		if counts[k] != n {
			t.Errorf("%s: got %d want %d", k, counts[k], n)
		}
	}
	if total > 100*1024*1024 {
		t.Fatalf("runtime budget exceeded: %d", total)
	}
	for _, path := range []string{"../catalog.json", "models/unknown.glb", "production/road.blend", "animations/humanoid-v1.json", "models/../../runtime/three.core.min.js"} {
		if _, err := s.AssetPackFile(m.ID, path); err == nil {
			t.Errorf("uncatalogued path accepted: %s", path)
		}
	}
	for _, file := range []string{"aurago-three-assets-1.js", "three-0.185.1.module.min.js", "three.core.min.js"} {
		if data, err := s.AssetPackFile("runtime", file); err != nil || len(data) == 0 {
			t.Fatalf("runtime %s: %v", file, err)
		}
	}
	matches, err := s.SearchAssets("sedan", m.ID, "3d", 6)
	if err != nil || len(matches) == 0 || matches[0].AssetID != "road-sedan" {
		t.Fatalf("model search: %+v, %v", matches, err)
	}
	detail, err := s.DescribeAsset(m.ID, "humans-civilian-a", "")
	if err != nil || detail.Model == nil || len(detail.Animations) != 34 || !strings.Contains(detail.Example, "loadAsset") {
		t.Fatalf("model detail: %+v, %v", detail, err)
	}
}

func TestModelPlanV2(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "3d")
	p := ExampleGamePlan(project)
	if err := s.checkPlan(project, p); err != nil {
		t.Fatal(err)
	}
	p.Assets = []PlanAsset{{Role: "driver", PackID: ModelPackID, Version: "1.0.0", AssetID: "humans-pilot-a", Scale: 1, Collider: "catalog", Animations: []string{"idle", "walk"}}}
	if err := s.checkPlan(project, p); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*GamePlan){
		func(p *GamePlan) { p.SchemaVersion = 1 },
		func(p *GamePlan) { p.Units = "pixels" },
		func(p *GamePlan) { p.Assets[0].Scale = 0 },
		func(p *GamePlan) { p.Assets[0].Animations = []string{"invented"} },
		func(p *GamePlan) { p.Assets[0].Collider = "rectangle" },
	} {
		bad := p
		bad.Assets = append([]PlanAsset(nil), p.Assets...)
		change(&bad)
		if s.checkPlan(project, bad) == nil {
			t.Fatal("invalid model plan accepted")
		}
	}
	p.Assets = nil
	for i := 0; i < 64; i++ {
		p.Assets = append(p.Assets, PlanAsset{Role: fmt.Sprint(i), Scale: 1, Collider: "box", Fallback: "2 metre procedural cube"})
	}
	if err := s.checkPlan(project, p); err != nil {
		t.Fatal(err)
	}
	p.Assets = append(p.Assets, PlanAsset{Role: "extra", Scale: 1, Collider: "box", Fallback: "cube"})
	if s.checkPlan(project, p) == nil {
		t.Fatal("65 assets accepted")
	}
}

func TestModelSelectionImportPreservationAndExport(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "3d")
	ids := []string{"road-sedan", "humans-civilian-a", "architecture-floor"}
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "visual" {
			return nil
		}
		if run.Stage == "planning" {
			if !slices.Equal(run.ModelAssetIDs, ids) {
				return fmt.Errorf("preselection lost")
			}
			files, err := s.ListJobFiles(ctx, run.Job.ID)
			if err != nil {
				return err
			}
			for _, f := range files {
				if strings.Contains(f, ModelPackID) {
					return fmt.Errorf("models imported before accepted plan")
				}
			}
			p := ExampleGamePlan(run.Project)
			p.Assets = nil
			for _, id := range ids {
				p.Assets = append(p.Assets, PlanAsset{Role: id, PackID: ModelPackID, Version: "1.0.0", AssetID: id, Scale: 1, Collider: "catalog"})
			}
			return s.SetPlan(ctx, run.Job.ID, p)
		}
		pack, err := s.ImportAssetPack(ctx, run.Job.ID, ModelPackID, ids...)
		if err != nil {
			return err
		}
		if len(pack.AssetIDs) != 3 || len(pack.Manifests) != 3 || !strings.Contains(pack.ThreeExample, "disposeInstance") {
			return fmt.Errorf("missing model import contract")
		}
		before, err := s.ListJobFiles(ctx, run.Job.ID)
		if err != nil {
			return err
		}
		if _, err := s.ImportAssetPack(ctx, run.Job.ID, ModelPackID, "vegetation-oak", "unknown"); err == nil {
			return fmt.Errorf("invalid import accepted")
		}
		after, _ := s.ListJobFiles(ctx, run.Job.ID)
		if !slices.Equal(before, after) {
			return fmt.Errorf("failed import left files")
		}
		if _, err := s.db.Exec(`CREATE TRIGGER model_import_failure BEFORE INSERT ON gm_assets WHEN NEW.path LIKE '%vegetation-oak.json' BEGIN SELECT RAISE(ABORT,'forced model import failure'); END`); err != nil {
			return err
		}
		_, failed := s.ImportAssetPack(ctx, run.Job.ID, ModelPackID, "vegetation-oak")
		if _, err := s.db.Exec("DROP TRIGGER model_import_failure"); err != nil {
			return err
		}
		if failed == nil {
			return fmt.Errorf("injected model import failure was ignored")
		}
		after, _ = s.ListJobFiles(ctx, run.Job.ID)
		if !slices.Equal(before, after) {
			return fmt.Errorf("late import failure left published dependencies")
		}
		stage, _ := s.JobDirectory(run.Job.ID)
		path := filepath.Join(stage, filepath.FromSlash(pack.Manifests[ids[0]]))
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("edited project copy"), 0600); err != nil {
			return err
		}
		_, err = s.ImportAssetPack(ctx, run.Job.ID, ModelPackID, ids...)
		if err == nil || !strings.Contains(err.Error(), "modified") {
			return fmt.Errorf("modified model overwritten: %v", err)
		}
		actual, _ := os.ReadFile(path)
		if string(actual) != "edited project copy" {
			return fmt.Errorf("project copy changed")
		}
		if err := os.WriteFile(path, original, 0600); err != nil {
			return err
		}
		content, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		if err := s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", "import meta from '../"+pack.Metadata+"';\nconsole.info(meta.id);\n"+content); err != nil {
			return err
		}
		inventory, err := s.importedJobPacks(ctx, run.Job.ID)
		if err != nil || len(inventory) != 3 {
			return fmt.Errorf("import inventory: %d, %v", len(inventory), err)
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{ModelAssetIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "ready" {
		t.Fatalf("job: %+v", done)
	}
	var buffer bytes.Buffer
	if _, err := s.WriteExport(context.Background(), project.ID, &buffer); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, f := range archive.File {
		r, _ := f.Open()
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = data
	}
	if len(files["vendor/aurago-three-assets-1.js"]) == 0 {
		t.Fatal("offline helper absent")
	}
	for _, id := range ids {
		d, err := s.DescribeAsset(ModelPackID, id, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range modelFiles(*d.Model) {
			path := "assets/builtin/" + ModelPackID + "/1.0.0/" + f.File
			if sha256Bytes(files[path]) != f.SHA256 {
				t.Fatal("offline dependency missing", path)
			}
		}
	}
	for path := range files {
		if strings.Contains(path, "vegetation-oak") {
			t.Fatal("unselected model exported")
		}
	}
}
