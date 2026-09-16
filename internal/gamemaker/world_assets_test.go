package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"aurago/internal/webassets"
)

// Review packs are exercised before admission to the public production catalog.
type worldReviewFS struct {
	fs.FS
	catalog []byte
}

func (f worldReviewFS) Open(name string) (fs.File, error) {
	if name == "asset_packs/catalog.json" {
		return fstest.MapFS{name: &fstest.MapFile{Data: f.catalog}}.Open(name)
	}
	return f.FS.Open(name)
}
func enableWorldReviewPacks(t *testing.T) {
	t.Helper()
	old := assetPackFS
	data, err := os.ReadFile("asset_packs/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var packs []AssetPackSummary
	if json.Unmarshal(data, &packs) != nil {
		t.Fatal("catalog")
	}
	for _, id := range []string{"aurago-pirates-3d", "aurago-pirates-topdown", "aurago-pirates-side", "aurago-isometric"} {
		data, err := os.ReadFile("asset_packs/" + id + "/manifest.json")
		if err != nil {
			t.Fatal(err)
		}
		var pack AssetPackSummary
		if err := json.Unmarshal(data, &pack); err != nil {
			t.Fatal(err)
		}
		pack.SelectionMode = "assets"
		if pack.Kind == "sprite2d" {
			pack.ManifestSchema = 2
		}
		if !slices.ContainsFunc(packs, func(p AssetPackSummary) bool { return p.ID == id }) {
			packs = append(packs, pack)
		}
	}
	data, _ = json.Marshal(packs)
	assetPackFS = webassets.Files{FS: worldReviewFS{FS: os.DirFS("."), catalog: data}}
	t.Cleanup(func() { assetPackFS = old })
}

func TestWorldPackIdentityAndAtlasClosure(t *testing.T) {
	enableWorldReviewPacks(t)
	s := newTestService(t)
	for _, id := range []string{"../aurago-pirates-3d", "aurago-pirates-side/..", "aurago-pirates-side", "unknown"} {
		if _, err := readModelManifest(id); err == nil {
			t.Fatalf("non-model identity accepted: %s", id)
		}
	}
	m, err := readModelManifest("aurago-pirates-3d")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Assets) < 3 {
		t.Fatal("vertical slice missing")
	}
	d, err := s.DescribeAsset(m.ID, "ships-sloop", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Example, m.ID+"/1.0.0/") || strings.Contains(d.Example, ModelPackID) {
		t.Fatal(d.Example)
	}
	if _, err := s.AssetPackFile(m.ID, "../../config.yaml"); err == nil {
		t.Fatal("traversal")
	}
	for _, id := range []string{"aurago-pirates-topdown", "aurago-pirates-side"} {
		m, err := readAtlasManifest(id)
		if err != nil {
			t.Fatal(err)
		}
		selected, err := selectAtlasAssets(m, []string{"people-diver-brass"})
		if err != nil {
			t.Fatal(err)
		}
		if len(selected.Assets) != 1 || len(selected.Atlases) == 0 {
			t.Fatal("selection")
		}
		for _, page := range selected.Atlases {
			data, err := s.AssetPackFile(id, page.File)
			if err != nil {
				t.Fatal(err)
			}
			if int64(len(data)) != page.Bytes || sha256Bytes(data) != page.SHA256 {
				t.Fatal("atlas checksum", page.File)
			}
		}
		for _, ids := range [][]string{nil, {"unknown"}, {"people-diver-brass", "unknown"}} {
			if _, err := selectAtlasAssets(m, ids); err == nil {
				t.Fatal("invalid selection accepted")
			}
		}
		if _, err := s.AssetPackFile(id, "sheet.png"); err == nil {
			t.Fatal("unlisted file served")
		}
	}
}

func TestWorldAtlasImportIsAtomicAndPreservesEdits(t *testing.T) {
	enableWorldReviewPacks(t)
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "visual" {
			return nil
		}
		if run.Stage == "planning" {
			p := ExampleGamePlan(run.Project)
			return s.SetPlan(ctx, run.Job.ID, p)
		}
		pack, err := s.ImportAssetPack(ctx, run.Job.ID, "aurago-pirates-side", "people-diver-brass")
		if err != nil {
			return err
		}
		before, err := s.ListJobFiles(ctx, run.Job.ID)
		if err != nil {
			return err
		}
		if _, err := s.ImportAssetPack(ctx, run.Job.ID, "aurago-pirates-side", "people-diver-brass", "unknown"); err == nil {
			return fmt.Errorf("invalid import succeeded")
		}
		after, _ := s.ListJobFiles(ctx, run.Job.ID)
		if !slices.Equal(before, after) {
			return fmt.Errorf("failed import left files")
		}
		if _, err := s.ImportAssetPack(ctx, run.Job.ID, "aurago-pirates-side", "people-diver-brass"); err != nil {
			return err
		}
		inventory, err := s.importedJobPacks(ctx, run.Job.ID)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(inventory, func(p ImportedAssetPack) bool { return p.ID == pack.ID && len(p.Manifests) == 1 }) {
			return fmt.Errorf("atlas missing from job inventory")
		}
		stage, _ := s.JobDirectory(run.Job.ID)
		path := filepath.Join(stage, filepath.FromSlash(pack.Metadata))
		original, _ := os.ReadFile(path)
		if err := os.WriteFile(path, []byte("edited atlas"), 0600); err != nil {
			return err
		}
		_, err = s.ImportAssetPack(ctx, run.Job.ID, pack.ID, "people-diver-brass")
		if err == nil || !strings.Contains(err.Error(), "modified") {
			return fmt.Errorf("edited copy replaced: %v", err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != "edited atlas" {
			return fmt.Errorf("edit lost")
		}
		if err := os.WriteFile(path, original, 0600); err != nil {
			return err
		}
		content, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		return s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", "import meta from '../"+pack.Metadata+"';\nconsole.info(meta.id);\n"+content)
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "ready" {
		t.Fatalf("%+v", done)
	}
}

func TestWorldAtlasDescriptionIsBounded(t *testing.T) {
	enableWorldReviewPacks(t)
	s := newTestService(t)
	full, err := s.describeAsset("aurago-pirates-topdown", "people-diver-brass", "")
	if err != nil {
		t.Fatal(err)
	}
	compact, err := s.DescribeAsset("aurago-pirates-topdown", "people-diver-brass", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Animations) != 8*len(compact.Animations) || len(compact.Asset.Directions) != 8 {
		t.Fatal("direction coverage lost")
	}
	data, _ := json.Marshal(compact)
	if len(data) > 12500 {
		t.Fatalf("oversized agent description: %d", len(data))
	}
	for _, c := range compact.Animations {
		if len(c.Frames) != 0 || c.Direction != compact.Asset.Direction {
			t.Fatal("full frame data leaked into compact description")
		}
	}
}
