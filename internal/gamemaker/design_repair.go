package gamemaker

import (
	"encoding/json"
	"strconv"
	"strings"
)

// repairDesignDraft normalizes the mistakes weak models repeat in set_design
// before strict validation sees them. Every repair is reported back as a warning;
// semantic errors (unknown catalog IDs, conflicting helpers, invalid scenes) stay
// hard rejections.
func (s *Service) repairDesignDraft(jobID string, project Project, draft map[string]json.RawMessage) {
	s.repairDesignAssets(jobID, draft)
}

func (s *Service) repairDesignAssets(jobID string, draft map[string]json.RawMessage) {
	raw, ok := draft["assets"]
	if !ok {
		return
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return
	}
	items := make([]DesignAsset, 0, len(entries))
	changed := false
	for _, entry := range entries {
		role := jsonStringField(entry["role"])
		packID := jsonStringField(entry["pack_id"])
		assetID := jsonStringField(entry["asset_id"])
		assemblyID := jsonStringField(entry["assembly_id"])
		fallback := jsonStringField(entry["fallback"])
		packs := jsonStringList(entry["pack_ids"])
		assets := jsonStringList(entry["asset_ids"])
		assemblies := jsonStringList(entry["assembly_ids"])
		for key := range entry {
			switch key {
			case "role", "pack_id", "asset_id", "assembly_id", "fallback",
				"pack_ids", "asset_ids", "assembly_ids":
			default:
				// Unknown asset fields (for example the read-only catalog "view")
				// are metadata, not a design conflict.
				changed = true
			}
		}
		if len(packs) > 0 {
			changed = true
			if packID == "" {
				packID = packs[0]
			}
		}
		if len(assets) > 0 || len(assemblies) > 0 {
			changed = true
		}
		if strings.TrimSpace(role) == "" && (assetID != "" || assemblyID != "") {
			role = designRoleFromIDs(assetID, assemblyID)
			changed = true
		}
		variants := max(1, max(len(assets), len(assemblies)))
		for i := 0; i < variants; i++ {
			item := DesignAsset{Role: role, PackID: packID, Fallback: fallback}
			if i < len(assets) {
				item.AssetID = assets[i]
			} else if i == 0 {
				item.AssetID = assetID
			}
			if i < len(assemblies) {
				item.AssemblyID = assemblies[i]
			} else if i == 0 {
				item.AssemblyID = assemblyID
			}
			if i > 0 {
				item.Role = role + "_" + strconv.Itoa(i+1)
			}
			items = append(items, item)
		}
	}
	seen := map[string]int{}
	for i := range items {
		base := strings.TrimSpace(items[i].Role)
		if base == "" {
			// An empty role without any ID cannot be derived; strict validation
			// reports the missing role instead of inventing one.
			continue
		}
		seen[base]++
		if seen[base] > 1 {
			items[i].Role = base + "_" + strconv.Itoa(seen[base])
			changed = true
		} else {
			items[i].Role = base
		}
	}
	if !changed {
		return
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return
	}
	draft["assets"] = encoded
	s.noteDesignWarning(jobID, "design.assets: normalized the asset list automatically (plural IDs split into variants, duplicate roles suffixed, unknown fields dropped)")
}

// defaultDesignAssets gives a 2D plan concrete procedural roles when the model
// omits assets, so the shared example's instruction prose never becomes part of
// an accepted plan. Catalog IDs stay optional; these entries are fallback art.
func defaultDesignAssets(base string) []DesignAsset {
	roles := []string{"player", "enemy", "item", "goal", "obstacle"}
	switch base {
	case "shooter":
		roles = []string{"player", "enemy", "projectile", "goal", "obstacle"}
	case "board":
		roles = []string{"cell_hidden", "cell_shown", "token"}
	}
	fallbacks := map[string]string{
		"player":      "procedural hero capsule",
		"enemy":       "procedural hostile shape",
		"item":        "procedural pickup diamond",
		"projectile":  "procedural shot capsule",
		"goal":        "procedural goal ring",
		"obstacle":    "procedural obstacle block",
		"cell_hidden": "procedural closed cell",
		"cell_shown":  "procedural revealed cell",
		"token":       "procedural player marker",
	}
	out := make([]DesignAsset, 0, len(roles))
	for _, role := range roles {
		fallback := fallbacks[role]
		if fallback == "" {
			fallback = "procedural " + role + " shape"
		}
		out = append(out, DesignAsset{Role: role, Fallback: fallback})
	}
	return out
}

func jsonStringField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func jsonStringList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		out := make([]string, 0, len(list))
		for _, value := range list {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	if single := jsonStringField(raw); single != "" {
		return []string{single}
	}
	return nil
}

func designRoleFromIDs(assetID, assemblyID string) string {
	value := strings.TrimSpace(assetID)
	if value == "" {
		value = strings.TrimSpace(assemblyID)
	}
	if value == "" {
		return "role"
	}
	var builder strings.Builder
	for _, c := range strings.ToLower(value) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			builder.WriteRune(c)
			continue
		}
		builder.WriteByte('_')
	}
	role := strings.Trim(builder.String(), "_")
	if role == "" {
		return "role"
	}
	if len(role) > 48 {
		role = role[:48]
	}
	return role
}
