package gamemaker

import (
	"encoding/json"
	"testing"
)

func TestSpritePackAssemblies(t *testing.T) {
	s := newTestService(t)
	for id, count := range map[string]int{"buildings-structures": 16, "buildings-structures-top-down": 16, "vehicles-planes": 8, "vehicles-planes-top-down": 8} {
		t.Run(id, func(t *testing.T) {
			pack, err := s.DescribeAssetPack(id)
			if err != nil {
				t.Fatal(err)
			}
			var assemblies []struct {
				ID, Name, Description string
				Width, Height         int
				Origin                map[string]float64
				Parts                 []struct {
					AssetID     string `json:"asset_id"`
					Frame, X, Y int
					AnimationID string `json:"animation_id"`
				}
			}
			var assets []struct {
				ID           string
				Frames       []int
				AssemblyPart bool `json:"assembly_part"`
				Origin       map[string]float64
			}
			var animations []struct {
				ID     string
				Frames []int
			}
			for _, item := range []struct {
				data []byte
				dst  any
			}{{pack.Assemblies, &assemblies}, {pack.Assets, &assets}, {pack.Animations, &animations}} {
				if err := json.Unmarshal(item.data, item.dst); err != nil {
					t.Fatal(err)
				}
			}
			if len(assemblies) != count {
				t.Fatalf("assembly count %d, want %d", len(assemblies), count)
			}
			seen, used := map[string]bool{}, map[string]bool{}
			for _, a := range assemblies {
				if a.ID == "" || seen[a.ID] || a.Name == "" || a.Description == "" || a.Width <= 0 || a.Height <= 0 || a.Width%64 != 0 || a.Height%64 != 0 || len(a.Origin) != 2 || len(a.Parts) == 0 {
					t.Fatalf("invalid assembly %+v", a)
				}
				seen[a.ID] = true
				positions := map[[2]int]bool{}
				for _, p := range a.Parts {
					pos := [2]int{p.X, p.Y}
					if p.X < 0 || p.Y < 0 || p.X+64 > a.Width || p.Y+64 > a.Height || p.X%64 != 0 || p.Y%64 != 0 || positions[pos] {
						t.Fatalf("invalid part position %+v", p)
					}
					positions[pos] = true
					found := false
					for _, asset := range assets {
						if asset.ID == p.AssetID {
							found = asset.AssemblyPart && asset.Frames[0] == p.Frame && asset.Origin["x"] == 0 && asset.Origin["y"] == 0
						}
					}
					if !found || used[p.AssetID] {
						t.Fatalf("unknown/reused assembly part %+v", p)
					}
					used[p.AssetID] = true
					if p.AnimationID != "" {
						found = false
						for _, animation := range animations {
							if animation.ID == p.AnimationID {
								found = len(animation.Frames) > 1 && animation.Frames[0] == p.Frame
							}
						}
						if !found {
							t.Fatalf("unknown part animation %s", p.AnimationID)
						}
					}
				}
			}
			for _, a := range assets {
				if a.AssemblyPart && !used[a.ID] {
					t.Fatalf("orphan assembly part %s", a.ID)
				}
			}
		})
	}
}
