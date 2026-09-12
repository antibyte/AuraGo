package gamemaker

import "testing"

func TestSceneEveryGeneratorProducesBoundedScene(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		for _, kind := range []string{"grid", "rooms", "path", "scatter", "platforms", "zones"} {
			t.Run(dimension+"/"+kind, func(t *testing.T) {
				scene := generationFixture(dimension)
				out, err := GenerateSceneRegion(scene, GenerateSceneRegionRequest{RegionID: "generated", Kind: kind, Bounds: scene.WorldBounds, Density: 4, Seed: 9, MinDistance: .5})
				if err != nil {
					t.Fatal(err)
				}
				if len(out.Nodes)+len(out.Zones) == 0 {
					t.Fatal("empty generator")
				}
				if err := validateScene(out, AssetCatalog{}); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
