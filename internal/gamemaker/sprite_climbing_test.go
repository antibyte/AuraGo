package gamemaker

import (
	"fmt"
	"slices"
	"testing"
)

func TestSpritePackClimbingDiscovery(t *testing.T) {
	service := newTestService(t)
	for _, packID := range []string{"human-characters-animated", "human-characters-top-down"} {
		for _, entity := range []string{"ranger", "knight"} {
			for _, surface := range []string{"ladder", "rope"} {
				t.Run(fmt.Sprintf("%s/%s/%s", packID, entity, surface), func(t *testing.T) {
					id := entity + "_climb_" + surface
					detail, err := service.DescribeAsset(packID, id, "")
					if err != nil || detail.Asset == nil {
						t.Fatalf("climbing asset is not registered: %+v, %v", detail, err)
					}
					if detail.Version != "3" || detail.Asset.Action != "climb_"+surface || detail.Asset.Entity != entity || detail.Asset.Direction != "none" {
						t.Fatalf("invalid climbing binding: %+v", detail.Asset)
					}
					index := slices.IndexFunc(detail.Animations, func(a PackAnimation) bool { return a.ID == id })
					if index < 0 {
						t.Fatal("describe omitted the climbing animation")
					}
					clip := detail.Animations[index]
					if clip.Repeat != -1 || clip.FrameRate != 8 || len(clip.Frames) != 6 || clip.Direction != "none" {
						t.Fatalf("invalid climbing cycle: %+v", clip)
					}
					for _, frame := range clip.Frames {
						if frame < 100 || frame >= 120 || !slices.Contains(detail.Asset.Frames, frame) {
							t.Fatalf("climbing overwrote an existing cell: %d", frame)
						}
					}
					for _, query := range []string{entity + " climb " + surface, "klettern " + map[string]string{"ladder": "leiter", "rope": "seil"}[surface]} {
						matches, err := service.SearchAssets(query, packID, detail.View, 12, "sprite2d")
						if err != nil || !slices.ContainsFunc(matches, func(m AssetSearchResult) bool { return m.AssetID == id && m.Version == "3" }) {
							t.Fatalf("climbing search %q: %+v, %v", query, matches, err)
						}
					}
				})
			}
		}
	}
}
