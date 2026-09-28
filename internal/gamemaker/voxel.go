package gamemaker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
)

const VoxelVersion = 1

// VoxelDefinition is authored game data, never a saved world or executable code.
type VoxelDefinition struct {
	Version int           `json:"version"`
	Mode    string        `json:"mode"`
	Seed    string        `json:"seed"`
	Size    [3]int        `json:"size"`
	Terrain string        `json:"terrain"`
	Blocks  []VoxelBlock  `json:"blocks"`
	Items   []VoxelItem   `json:"items"`
	Recipes []VoxelRecipe `json:"recipes"`
	Enemies []VoxelEnemy  `json:"enemies"`
	Goals   []VoxelGoal   `json:"goals,omitempty"`
}
type VoxelBlock struct {
	ID       int     `json:"id"`
	Key      string  `json:"key"`
	Name     string  `json:"name"`
	Material string  `json:"material"`
	Color    string  `json:"color"`
	Drop     string  `json:"drop"`
	Tier     int     `json:"tier"`
	Hardness float64 `json:"hardness"`
}
type VoxelItem struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Block  int     `json:"block"`
	Stack  int     `json:"stack"`
	Tier   int     `json:"tier"`
	Damage float64 `json:"damage"`
}
type VoxelRecipe struct {
	ID          string         `json:"id"`
	Ingredients map[string]int `json:"ingredients"`
	Item        string         `json:"item"`
	Count       int            `json:"count"`
}
type VoxelEnemy struct {
	ID       string  `json:"id"`
	Behavior string  `json:"behavior"`
	Count    int     `json:"count"`
	Health   float64 `json:"health"`
	Damage   float64 `json:"damage"`
	Drop     string  `json:"drop"`
}
type VoxelGoal struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Item  string `json:"item,omitempty"`
	Count int    `json:"count"`
}

var voxelID = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
var voxelColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (v *VoxelDefinition) UnmarshalJSON(data []byte) error {
	type plain VoxelDefinition
	var value plain
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return err
	}
	var shape struct {
		Size []int `json:"size"`
	}
	if json.Unmarshal(data, &shape) != nil || len(shape.Size) != 3 {
		return fmt.Errorf("voxel size must contain exactly three dimensions")
	}
	*v = VoxelDefinition(value)
	return nil
}

func DefaultVoxelDefinition() *VoxelDefinition {
	return &VoxelDefinition{
		Version: 1, Mode: "survival", Seed: "island-1", Size: [3]int{96, 48, 96}, Terrain: "hills",
		Blocks: []VoxelBlock{
			{1, "grass", "Grass", "grass", "#729c45", "dirt", 0, 0.3},
			{2, "dirt", "Dirt", "dirt", "#8b6242", "dirt", 0, 0.3},
			{3, "stone", "Stone", "stone", "#8c9299", "stone", 1, 0.6},
			{4, "wood", "Wood", "wood", "#95683d", "wood", 0, 0.4},
			{5, "leaves", "Leaves", "leaves", "#478344", "leaves", 0, 0.2},
			{6, "ore", "Metal ore", "ore", "#c4976f", "metal", 2, 0.8},
			{7, "sand", "Sand", "sand", "#ddcd94", "sand", 0, 0.3},
			{8, "planks", "Planks", "planks", "#bf945a", "planks", 0, 0.4},
			{9, "brick", "Brick", "brick", "#b97866", "brick", 1, 0.6},
		},
		Items: []VoxelItem{
			{"dirt", "Dirt", 2, 64, 0, 1}, {"wood", "Wood", 4, 64, 0, 1}, {"stone", "Stone", 3, 64, 0, 1},
			{"metal", "Metal", 0, 64, 0, 1}, {"leaves", "Leaves", 5, 64, 0, 1}, {"sand", "Sand", 7, 64, 0, 1},
			{"planks", "Planks", 8, 64, 0, 1}, {"brick", "Brick", 9, 64, 0, 1},
			{"wood_tool", "Wooden tool", 0, 1, 1, 4}, {"stone_tool", "Stone tool", 0, 1, 2, 8}, {"metal_tool", "Metal tool", 0, 1, 3, 14},
		},
		Recipes: []VoxelRecipe{
			{"planks", map[string]int{"wood": 1}, "planks", 4},
			{"wood_tool", map[string]int{"wood": 2}, "wood_tool", 1},
			{"stone_tool", map[string]int{"stone": 3, "wood": 1}, "stone_tool", 1},
			{"metal_tool", map[string]int{"metal": 3, "wood": 1}, "metal_tool", 1},
			{"brick", map[string]int{"stone": 2}, "brick", 2},
		},
		Enemies: []VoxelEnemy{{"crawler", "melee", 4, 20, 5, "wood"}, {"sentinel", "ranged", 2, 30, 8, "metal"}},
	}
}

func (v *VoxelDefinition) Validate() error {
	bad := func(message string) error { return fmt.Errorf("voxel: %s", message) }
	if v == nil || v.Version != VoxelVersion {
		return bad("version must be 1")
	}
	if !slices.Contains([]string{"survival", "creative"}, v.Mode) {
		return bad("mode must be survival or creative")
	}
	if len(v.Seed) < 1 || len(v.Seed) > 128 {
		return bad("seed must contain 1–128 bytes")
	}
	for i, n := range v.Size {
		if n < 16 || n%16 != 0 || n > [3]int{128, 64, 128}[i] {
			return bad("size must use 16-block multiples, up to 128×64×128")
		}
	}
	if !slices.Contains([]string{"flat", "hills", "island"}, v.Terrain) {
		return bad("terrain must be flat, hills or island")
	}
	if len(v.Blocks) < 1 || len(v.Blocks) > 64 || len(v.Items) < 1 || len(v.Items) > 64 || len(v.Recipes) < 1 || len(v.Recipes) > 64 {
		return bad("1–64 blocks, items and recipes required")
	}
	blocks, keys, materials := map[int]bool{}, map[string]bool{}, map[string]bool{}
	generated := map[int]bool{}
	orderedBlocks := slices.Clone(v.Blocks)
	slices.SortFunc(orderedBlocks, func(a, b VoxelBlock) int { return a.ID - b.ID })
	for _, b := range orderedBlocks {
		if b.ID < 1 || b.ID > 64 || blocks[b.ID] || !voxelID.MatchString(b.Key) || keys[b.Key] || len(b.Name) < 1 || len(b.Name) > 80 {
			return bad("blocks need unique IDs (1–64), keys and bounded names")
		}
		if !slices.Contains([]string{"grass", "dirt", "stone", "wood", "leaves", "ore", "sand", "planks", "brick"}, b.Material) || !voxelColor.MatchString(b.Color) || b.Tier < 0 || b.Tier > 3 || !finite(b.Hardness) || b.Hardness < 0.1 || b.Hardness > 4 {
			return bad("invalid block material, color, tier or hardness (0.1–4 seconds)")
		}
		if !materials[b.Material] {
			generated[b.ID] = true
		}
		blocks[b.ID], keys[b.Key], materials[b.Material] = true, true, true
	}
	for _, material := range []string{"grass", "dirt", "stone", "wood", "leaves", "ore", "sand"} {
		if !materials[material] {
			return bad("terrain requires material " + material)
		}
	}
	items := map[string]VoxelItem{}
	for _, item := range v.Items {
		if _, ok := items[item.ID]; ok || !voxelID.MatchString(item.ID) || len(item.Name) < 1 || len(item.Name) > 80 || (item.Block != 0 && !blocks[item.Block]) || item.Stack < 1 || item.Stack > 999 || item.Tier < 0 || item.Tier > 3 || !finite(item.Damage) || item.Damage < 0 || item.Damage > 100 {
			return bad("invalid or duplicate item")
		}
		items[item.ID] = item
	}
	for _, b := range v.Blocks {
		if _, ok := items[b.Drop]; !ok {
			return bad("block drop references unknown item " + b.Drop)
		}
	}
	recipes := map[string]bool{}
	for _, r := range v.Recipes {
		item, ok := items[r.Item]
		if !voxelID.MatchString(r.ID) || recipes[r.ID] || !ok || r.Count < 1 || r.Count > item.Stack || len(r.Ingredients) < 1 || len(r.Ingredients) > 8 {
			return bad("invalid recipe identity, output or ingredients")
		}
		recipes[r.ID] = true
		for id, n := range r.Ingredients {
			item, ok := items[id]
			if !ok || n < 1 || n > item.Stack*36 {
				return bad("invalid recipe ingredient")
			}
		}
	}
	enemies, total := map[string]bool{}, 0
	for _, e := range v.Enemies {
		_, drop := items[e.Drop]
		if !voxelID.MatchString(e.ID) || enemies[e.ID] || !slices.Contains([]string{"melee", "ranged"}, e.Behavior) || e.Count < 1 || e.Count > 24 || !finite(e.Health) || e.Health < 1 || e.Health > 1000 || !finite(e.Damage) || e.Damage < 1 || e.Damage > 100 || !drop {
			return bad("invalid enemy")
		}
		enemies[e.ID] = true
		total += e.Count
	}
	if total > 24 {
		return bad("at most 24 enemies")
	}
	// Reject circular progressions (for example stone requiring a stone tool).
	reachable, crafted, collectible := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, e := range v.Enemies {
		reachable[e.Drop], collectible[e.Drop] = true, true
	}
	if v.Mode == "creative" {
		for id := range items {
			reachable[id] = true
		}
	}
	tier := 0
	for round := 0; round < 65; round++ {
		changed := false
		for id := range reachable {
			if items[id].Tier > tier {
				tier = items[id].Tier
				changed = true
			}
		}
		for _, b := range v.Blocks {
			if (v.Mode == "creative" || b.Tier <= tier) && generated[b.ID] && b.Material != "planks" && b.Material != "brick" {
				collectible[b.Drop] = true
				if reachable[b.Drop] {
					continue
				}
				reachable[b.Drop] = true
				changed = true
			}
		}
		for _, r := range v.Recipes {
			possible := true
			for id := range r.Ingredients {
				possible = possible && (v.Mode == "creative" || reachable[id])
			}
			if possible {
				crafted[r.Item] = true
				if !reachable[r.Item] {
					reachable[r.Item] = true
					changed = true
				}
				if items[r.Item].Tier > tier {
					tier = items[r.Item].Tier
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	if len(crafted) == 0 {
		return bad("provide a craftable recipe reachable from terrain resources")
	}
	for _, b := range v.Blocks {
		if (b.Material == "stone" || b.Material == "ore") && v.Mode == "survival" && b.Tier > tier {
			return bad("stone and ore require a reachable tool progression")
		}
	}
	goals := map[string]bool{}
	if len(v.Goals) > 8 {
		return bad("at most 8 goals")
	}
	for _, g := range v.Goals {
		_, hasItem := items[g.Item]
		if !voxelID.MatchString(g.ID) || goals[g.ID] || g.Count < 1 || g.Count > 10000 || !slices.Contains([]string{"collect", "craft", "place", "defeat"}, g.Kind) || (g.Kind != "defeat" && !hasItem) || (g.Kind == "defeat" && g.Item != "") {
			return bad("invalid goal")
		}
		goals[g.ID] = true
		if g.Kind == "craft" && !crafted[g.Item] || g.Kind == "collect" && !collectible[g.Item] || g.Kind == "place" && (!reachable[g.Item] || items[g.Item].Block == 0) || g.Kind == "defeat" && g.Count > total {
			return bad("goal cannot be reached with the declared world and rules")
		}
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil || len(data) > 64*1024 {
		return bad("formatted definition exceeds 64 KiB")
	}
	return nil
}

func ParseVoxelDefinition(data []byte) (*VoxelDefinition, error) {
	if len(data) > 64*1024 {
		return nil, fmt.Errorf("voxel definition exceeds 64 KiB")
	}
	var v VoxelDefinition
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&v); err != nil {
		return nil, fmt.Errorf("voxel definition: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("voxel definition must contain one object")
	}
	var shape struct {
		Size []int `json:"size"`
	}
	if json.Unmarshal(data, &shape) != nil || len(shape.Size) != 3 {
		return nil, fmt.Errorf("voxel size must contain exactly three dimensions")
	}
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return &v, nil
}

// Compatibility deliberately excludes labels, recipes, goals and combat tuning.
// Stable numeric block IDs and item identities define the saved world's meaning.
func (v VoxelDefinition) Compatibility() string {
	blocks := slices.Clone(v.Blocks)
	slices.SortFunc(blocks, func(a, b VoxelBlock) int { return a.ID - b.ID })
	items := slices.Clone(v.Items)
	slices.SortFunc(items, func(a, b VoxelItem) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	identities := []any{}
	for _, b := range blocks {
		identities = append(identities, []any{b.ID, b.Key, b.Material})
	}
	for _, i := range items {
		identities = append(identities, []any{i.ID, i.Block, i.Stack})
	}
	enemies := slices.Clone(v.Enemies)
	slices.SortFunc(enemies, func(a, b VoxelEnemy) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	for _, e := range enemies {
		identities = append(identities, []any{"enemy", e.ID, e.Behavior, e.Count})
	}
	data, _ := json.Marshal([]any{VoxelVersion, "generator-1", "save-1", v.Mode, v.Seed, v.Size, v.Terrain, identities})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
