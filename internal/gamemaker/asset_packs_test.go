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
	"testing"
)

func TestSpritePackContent(t *testing.T) {
	s := newTestService(t)
	packs, err := s.ListAssetPacks()
	if err != nil || len(packs) != 10 {
		t.Fatalf("catalog: %d packs, %v", len(packs), err)
	}
	for _, summary := range packs {
		t.Run(summary.ID, func(t *testing.T) {
			pack, err := s.DescribeAssetPack(summary.ID)
			if err != nil {
				t.Fatal(err)
			}
			if pack.SchemaVersion != 1 || pack.Version != "1" || pack.Columns != 10 || pack.Rows != 10 || pack.FrameWidth != 64 || pack.FrameHeight != 64 || pack.Image != "sheet.png" {
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
								if !a.Tile && (x == 0 || y == 0 || x == 63 || y == 63) {
									t.Fatalf("asset touches cell edge: %s frame %d", a.ID, index)
								}
							}
						}
					}
					if opaque == 0 || (!a.Tile && transparent == 0) {
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

func TestSpritePackSelectionImportAndOfflineExport(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	packs, _ := s.ListAssetPacks()
	ids := []string{}
	for _, p := range packs {
		ids = append(ids, p.ID)
	}
	s.SetRunner(testRunner{service: s, mutate: func(ctx context.Context, run JobRun) error {
		if len(run.AssetPacks) != 10 {
			return fmt.Errorf("agent saw %d imports", len(run.AssetPacks))
		}
		for _, pack := range run.AssetPacks {
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
		if count != 20 {
			return fmt.Errorf("idempotent import recorded %d files", count)
		}
		return nil
	}})
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{AssetPackIDs: append(ids, ids[0])})
	if err == nil {
		t.Fatal("more than ten selections accepted")
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
			path := "assets/builtin/" + id + "/1/" + filename
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
				target := filepath.Join(stage, "assets", "builtin", "space-shooter", "1")
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
