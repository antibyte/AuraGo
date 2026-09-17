package gamemaker

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// threeObservationPlugin adds missing read-only ray metadata to known older
// copies of the template observer. It does not rewrite the user's source or
// replace their shooting, movement, camera, or other game rules.
func threeObservationPlugin(projectDir string) api.Plugin {
	common, _ := filepath.Abs(filepath.Join(projectDir, "src", "common.ts"))
	return api.Plugin{Name: "three-target-observation", Setup: func(build api.PluginBuild) {
		build.OnLoad(api.OnLoadOptions{Filter: `[/\\]common\.ts$`}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			if filepath.Clean(args.Path) != common {
				return api.OnLoadResult{}, nil
			}
			data, err := os.ReadFile(args.Path)
			if err != nil {
				return api.OnLoadResult{}, fmt.Errorf("read three observer: %w", err)
			}
			source := string(data)
			upgraded := upgradeThreeTargetObservation(source)
			if source == upgraded {
				return api.OnLoadResult{}, nil
			}
			return api.OnLoadResult{Contents: &upgraded, Loader: api.LoaderTS, ResolveDir: filepath.Dir(args.Path)}, nil
		})
	}}
}

func upgradeThreeTargetObservation(source string) string {
	start := strings.Index(source, "function observeTargets(){")
	if start < 0 {
		return source
	}
	end := strings.Index(source[start:], "function draw(")
	if end < 0 {
		return source
	}
	observer := source[start : start+end]
	// These are the two known ray setups: the original +Z space gun and the
	// camera-directed variant found in the failed space-game reproduction.
	legacy := "if(config.mode==='fps')look.setFromCamera({x:0,y:0},camera);else look.set(player.position,new T.Vector3(0,0,1));"
	camera := "if(config.mode==='fps'||config.mode==='space'){camera.updateMatrixWorld();look.setFromCamera({x:0,y:0},camera)}else look.set(player.position,new T.Vector3(0,0,1));"
	if strings.Count(observer, legacy)+strings.Count(observer, camera) != 1 {
		return source
	}
	canonical := strings.Replace(observer, legacy, "", 1)
	canonical = strings.Replace(canonical, camera, "", 1)
	// Match the whole known observer, excluding only its ray setup. Unknown
	// custom observers are left intact. Fixture: testdata/legacy-three-observer.ts.
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(strings.Fields(canonical), " "))))
	switch digest {
	case "82478bdfb0d9b479477505d21acc82bdb354381d0c6b4c8ae8d0547c2c7f00c0": // Original targets.
	case "47961033ece734de80abbe8deaeeb2c9be1074784f800538e9a4b9519dc6be5c": // Also observes ASTEROID_ROLES.
	default:
		return source
	}
	metadata := "aim_ray:{origin:{x:look.ray.origin.x,y:look.ray.origin.z,z:look.ray.origin.y},direction:{x:look.ray.direction.x,y:look.ray.direction.z,z:look.ray.direction.y}},"
	upgraded := strings.Replace(observer, "return {kind:'3d',", "return {"+metadata+"kind:'3d',", 1)
	return source[:start] + upgraded + source[start+end:]
}
