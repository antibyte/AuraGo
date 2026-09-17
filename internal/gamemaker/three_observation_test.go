package gamemaker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evanw/esbuild/pkg/api"
)

func TestUpgradeThreeTargetObservation(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy-three-observer.ts")
	if err != nil {
		t.Fatal(err)
	}
	legacy := string(data) + "  function draw(now:number){}\n"
	camera := strings.Replace(legacy, "if(config.mode==='fps')look.setFromCamera({x:0,y:0},camera);", "if(config.mode==='fps'||config.mode==='space'){camera.updateMatrixWorld();look.setFromCamera({x:0,y:0},camera)}", 1)
	asteroids := strings.Replace(camera, "o.role==='enemy'||o.node?", "o.role==='enemy'||(!builder&&ASTEROID_ROLES.includes(o.role))||o.node?", 1)
	for _, tc := range []struct {
		name, source string
		upgrade      bool
	}{
		{"legacy", legacy, true}, {"camera", camera, true}, {"asteroids", asteroids, true},
		{"windows", strings.ReplaceAll(camera, "\n", "\r\n"), true},
		{"custom_observer", strings.Replace(camera, "targets,aimed", "targets:customTargets,aimed", 1), false},
		{"custom_ray", strings.Replace(camera, "new T.Vector3(0,0,1)", "customDirection", 1), false},
		{"unrelated", "export const source = 42;", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Game rules around the observer must remain byte-for-byte intact.
			source := "function fire(){customShotAndDamage()}\n" + tc.source + "function customRules(){tick()}\n"
			got := upgradeThreeTargetObservation(source)
			if (got != source) != tc.upgrade {
				t.Fatalf("unexpected upgrade=%v", got != source)
			}
			if tc.upgrade {
				if !strings.Contains(got, "aim_ray:{origin:{x:look.ray.origin.x,y:look.ray.origin.z,z:look.ray.origin.y}") {
					t.Fatal("missing mapped shot origin")
				}
				begin, end := strings.Index(got, "aim_ray:"), strings.Index(got, "kind:'3d',")
				if got[:begin]+got[end:] != source {
					t.Fatal("upgrade altered more than the read-only metadata")
				}
			}
			if upgradeThreeTargetObservation(got) != got {
				t.Fatal("upgrade is not idempotent")
			}
		})
	}
}

func TestThreeObservationBuildPreservesSource(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy-three-observer.ts")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	source := string(data) + "function draw(){}\nexport {observeTargets};\n"
	path := filepath.Join(root, "src", "common.ts")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	build := api.Build(api.BuildOptions{AbsWorkingDir: root, EntryPoints: []string{"src/common.ts"}, Bundle: true, Format: api.FormatESModule, Plugins: []api.Plugin{threeObservationPlugin(root)}})
	if len(build.Errors) > 0 {
		t.Fatal(build.Errors)
	}
	if len(build.OutputFiles) != 1 || !strings.Contains(string(build.OutputFiles[0].Contents), "aim_ray:") {
		t.Fatal("compiled legacy observer has no ray metadata")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != source {
		t.Fatal("building rewrote the original source", err)
	}
}
