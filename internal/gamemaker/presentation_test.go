package gamemaker

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPresentationCatalog(t *testing.T) {
	s := newTestService(t)
	var total int64
	for _, id := range []string{EffectsPackID, SoundsPackID} {
		m, err := readPresentationPack(id)
		if err != nil {
			t.Fatal(err)
		}
		want := 52
		if id == SoundsPackID {
			want = 40
		}
		if len(m.Assets) != want {
			t.Fatal(id, len(m.Assets))
		}
		for _, a := range m.Assets {
			if _, err := s.describeAsset(id, a.ID, ""); err != nil {
				t.Fatal(err)
			}
			for _, f := range a.Files {
				b, err := bundledPresentationFile(id, f.File)
				if err != nil || int64(len(b)) != f.Bytes || sha256Bytes(b) != f.SHA256 {
					t.Fatal(a.ID, "integrity", err)
				}
				if !bytes.Equal(b[:4], []byte("RIFF")) || string(b[8:12]) != "WAVE" || binary.LittleEndian.Uint32(b[24:]) != 48000 || binary.LittleEndian.Uint16(b[34:]) != 16 {
					t.Fatal(a.ID, "PCM 16/48 kHz required")
				}
				channels := binary.LittleEndian.Uint16(b[22:])
				if a.Loop && channels != 2 || !a.Loop && channels != 1 {
					t.Fatal(a.ID, "channel layout")
				}
				var peak int
				for i := 44; i+1 < len(b); i += 2 {
					v := int(int16(binary.LittleEndian.Uint16(b[i:])))
					if v < 0 {
						v = -v
					}
					if v > peak {
						peak = v
					}
				}
				if peak < 100 || peak > 23200 {
					t.Fatal(a.ID, "silent or clipping", peak)
				}
			}
		}
		err = fs.WalkDir(assetPackFS, "asset_packs/"+id, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				b, e := assetPackFS.ReadFile(path)
				if e != nil {
					return e
				}
				total += int64(len(b))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"aurago-effects-2d-1.js", "aurago-effects-3d-1.js"} {
		b, e := bundledAssetPackFile("runtime", id)
		if e != nil {
			t.Fatal(e)
		}
		total += int64(len(b))
	}
	if total > 64*1024*1024 {
		t.Fatal("runtime exceeds 64 MiB", total)
	}
	t.Logf("additional runtime %.2f MiB", float64(total)/1024/1024)
	for _, query := range []struct{ kind, term string }{{"effect", "rain"}, {"audio", "footstep"}} {
		results, err := s.SearchAssets(query.term, "", "", 6, query.kind)
		if err != nil || len(results) == 0 {
			t.Fatal(query, err)
		}
		for _, a := range results {
			if a.Kind != query.kind {
				t.Fatal("search kind mixed", a)
			}
		}
	}
	for _, path := range []string{"../sources.json", "sounds/../manifest.json", "sounds/unknown.wav", "sources/forest.mp3"} {
		if _, err := bundledPresentationFile(SoundsPackID, path); err == nil {
			t.Fatal("uncatalogued route", path)
		}
	}
	for _, p := range []*Presentation{{Quality: "ultra"}, {Version: "2"}, {Environment: "rain-heavy"}, {Effects: []string{"forest-rain"}}, {Sounds: []SoundBinding{{"write-file", "rifle"}}}, {Sounds: []SoundBinding{{"shot", "missing"}}}} {
		if _, _, err := resolvePresentation(p); err == nil {
			t.Fatal("invalid plan accepted", p)
		}
	}
}

func TestPresentationPlanVersions(t *testing.T) {
	s := newTestService(t)
	for _, dim := range []string{"2d", "3d"} {
		project := Project{Dimension: dim}
		plan := ExampleGamePlan(project)
		if plan.SchemaVersion != 3 {
			t.Fatal("new plan is not schema 3")
		}
		if err := s.checkPlan(project, plan); err != nil {
			t.Fatal(err)
		}
		plan.SchemaVersion = 1
		if dim == "3d" {
			plan.SchemaVersion = 2
		}
		if err := s.checkPlan(project, plan); err != nil {
			t.Fatal("old schema rejected", err)
		}
		plan.Presentation = &Presentation{Environment: "forest-rain"}
		if err := s.checkPlan(project, plan); err == nil {
			t.Fatal("presentation accepted without schema 3")
		}
		plan.SchemaVersion = 3
		if err := s.checkPlan(project, plan); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPresentationBuildRetainsPlannedDependencies(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			dir := t.TempDir()
			write := func(path, content string) {
				t.Helper()
				target := filepath.Join(dir, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, []byte(content), 0640); err != nil {
					t.Fatal(err)
				}
			}
			write("game.json", `{"name":"presentation","dimension":"`+dimension+`"}`)
			write("index.html", scaffoldHTML(dimension, "presentation"))
			plan := GamePlan{SchemaVersion: 3, Presentation: &Presentation{Environment: "coast", Effects: []string{"stone-debris"}, Sounds: []SoundBinding{{"hit", "impact-stone"}}}}
			data, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			write(gamePlanPath, string(data))
			config, err := presentationConfig(plan.Presentation)
			if err != nil {
				t.Fatal(err)
			}
			main := `import config from './presentation.json'; import {createPresentation} from '../vendor/aurago-effects-` + dimension + `-1.js'; console.log(createPresentation, config);`
			for _, test := range []struct {
				name, config, source string
				want                 bool
			}{
				{"connected", config, main, true},
				{"disabled", "null", main, false},
				{"effect-removed", strings.Replace(config, `"id":"stone-debris"`, `"id":"removed"`, 1), main, false},
				{"sound-removed", strings.Replace(config, `"id":"impact-stone"`, `"id":"removed"`, 1), main, false},
				{"binding-removed", strings.Replace(config, `"event":"hit"`, `"event":"shot"`, 1), main, false},
				{"runtime-disconnected", config, `import config from './presentation.json'; console.log(config); // createPresentation in ../vendor/aurago-effects-` + dimension + `-1.js`, false},
				{"config-disconnected", config, `import {createPresentation} from '../vendor/aurago-effects-` + dimension + `-1.js'; console.log(createPresentation);`, false},
			} {
				t.Run(test.name, func(t *testing.T) {
					write("src/presentation.json", test.config)
					write("src/main.ts", test.source)
					result := buildDirectory(context.Background(), dir, 100, 64<<20)
					if result.OK != test.want {
						t.Fatalf("OK=%t, want %t: %+v", result.OK, test.want, result.Diagnostics)
					}
				})
			}
			write(gamePlanPath, `{"schema_version":2}`)
			write("src/presentation.json", "null")
			write("src/main.ts", `console.log('legacy game');`)
			if result := buildDirectory(context.Background(), dir, 100, 64<<20); !result.OK {
				t.Fatal("legacy game rejected", result.Diagnostics)
			}
		})
	}
}

func TestPresentationImportTransaction(t *testing.T) {
	for _, failure := range []string{"", "modified", "limit", "ledger", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			s.SetRunner(testRunner{service: s, mutate: func(ctx context.Context, run JobRun) error {
				stage, _ := s.JobDirectory(run.Job.ID)
				target := filepath.Join(stage, "assets", "builtin")
				sample := filepath.Join(target, SoundsPackID, PresentationVersion, "sounds", "rain-heavy.wav")
				if failure == "modified" {
					os.MkdirAll(filepath.Dir(sample), 0750)
					os.WriteFile(sample, []byte("user version"), 0640)
				}
				if failure == "limit" {
					s.opts.MaxAssetBytes = 1
				}
				if failure == "ledger" {
					s.db.Exec(`CREATE TRIGGER fail_presentation BEFORE INSERT ON gm_assets BEGIN SELECT RAISE(ABORT,'fixture'); END`)
				}
				if failure == "cancel" {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				result, err := s.ImportAssetPack(ctx, run.Job.ID, EffectsPackID, []string{"forest-rain"}...)
				if failure != "" {
					if err == nil {
						return errors.New("failure import succeeded")
					}
					files := 0
					filepath.WalkDir(target, func(_ string, d fs.DirEntry, e error) error {
						if e == nil && !d.IsDir() {
							files++
						}
						return nil
					})
					want := 0
					if failure == "modified" {
						want = 1
						b, _ := os.ReadFile(sample)
						if string(b) != "user version" {
							return errors.New("modified sample overwritten")
						}
					}
					if files != want {
						return fmt.Errorf("partial import: %d files", files)
					}
				} else {
					if err != nil {
						return err
					}
					if _, err = os.Stat(sample); err != nil {
						return errors.New("atmosphere dependency missing")
					}
					if len(result.Manifests) < 6 {
						return errors.New("missing dependency metadata")
					}
					if _, err = s.ImportAssetPack(ctx, run.Job.ID, EffectsPackID, "forest-rain"); err != nil {
						return err
					}
					inventory, err := s.importedJobPacks(ctx, run.Job.ID)
					if err != nil || len(inventory) != 2 {
						return fmt.Errorf("bounded inventory: %d, %v", len(inventory), err)
					}
				}
				return errors.New("presentation fixture finished")
			}})
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			done := waitJob(t, s, job.ID)
			if !strings.Contains(done.Error, "presentation fixture finished") {
				t.Fatal(done.Error)
			}
		})
	}
}
