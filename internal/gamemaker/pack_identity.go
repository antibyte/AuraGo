package gamemaker

import (
	"encoding/json"
	"fmt"
)

// Resolve kinds from the bundled catalog, never from a path supplied by a model.
func catalogPackKind(id string) string {
	if !safeModelComponent(id) {
		return ""
	}
	data, err := assetPackFS.ReadFile("asset_packs/catalog.json")
	if err != nil {
		return ""
	}
	var packs []AssetPackSummary
	if json.Unmarshal(data, &packs) != nil {
		return ""
	}
	for _, pack := range packs {
		if pack.ID == id {
			return pack.Kind
		}
	}
	return ""
}

func modelPack(id string) bool { return catalogPackKind(id) == "model3d" }

// Omitted pack IDs retain the original public and test helper contracts.
func modelPackArgument(ids []string) string {
	if len(ids) == 0 {
		return ModelPackID
	}
	if len(ids) != 1 {
		return ""
	}
	return ids[0]
}

func (s *Service) validateAssetSelections(dimension string, values []AssetSelection) ([]AssetSelection, error) {
	if len(values) > 64 {
		return nil, fmt.Errorf("select at most 64 concrete assets")
	}
	seen := map[AssetSelection]bool{}
	out := []AssetSelection{}
	for _, v := range values {
		if seen[v] {
			continue
		}
		d, err := s.describeAsset(v.PackID, v.AssetID, "")
		if err != nil {
			return nil, err
		}
		if (dimension == "3d") != (d.Model != nil) || d.Presentation != nil {
			return nil, fmt.Errorf("selected asset %s/%s is incompatible with %s", v.PackID, v.AssetID, dimension)
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, nil
}
