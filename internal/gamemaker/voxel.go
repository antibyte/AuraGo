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
	"strings"
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
	var issues []string
	add := func(message string) {
		if len(issues) < 8 {
			runes := []rune(message)
			issues = append(issues, string(runes[:min(600, len(runes))]))
		}
	}
	if !slices.Contains([]string{"survival", "creative"}, v.Mode) {
		add("mode must be survival or creative")
	}
	if len(v.Seed) < 1 || len(v.Seed) > 128 {
		add("seed must contain 1–128 bytes")
	}
	for i, n := range v.Size {
		if limit := [3]int{128, 64, 128}[i]; n < 16 || n%16 != 0 || n > limit {
			add(fmt.Sprintf("size[%d] (%s) %d must be a multiple of 16 from 16 to %d", i, [3]string{"x", "height", "z"}[i], n, limit))
		}
	}
	if !slices.Contains([]string{"flat", "hills", "island"}, v.Terrain) {
		add("terrain must be flat, hills or island")
	}
	if len(v.Blocks) < 1 || len(v.Blocks) > 64 || len(v.Items) < 1 || len(v.Items) > 64 || len(v.Recipes) < 1 || len(v.Recipes) > 64 {
		return bad(fmt.Sprintf("1–64 blocks, items and recipes required (got %d, %d, %d)", len(v.Blocks), len(v.Items), len(v.Recipes)))
	}
	blocks, keys, materials := map[int]bool{}, map[string]bool{}, map[string]bool{}
	generated := map[int]bool{}
	// Terrain generation takes the first block per material in ID order.
	order := make([]int, len(v.Blocks))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return v.Blocks[a].ID - v.Blocks[b].ID })
	for _, i := range order {
		b := v.Blocks[i]
		if issue := voxelBlockIssue(b, blocks, keys); issue != "" {
			add(fmt.Sprintf("blocks[%d] %s", i, issue))
		}
		if !materials[b.Material] {
			generated[b.ID] = true
		}
		blocks[b.ID], keys[b.Key], materials[b.Material] = true, true, true
	}
	var missing []string
	for _, material := range strings.Split(voxelTerrainMaterials, ", ") {
		if !materials[material] {
			missing = append(missing, material)
		}
	}
	if len(missing) > 0 {
		add("blocks are missing terrain materials: " + strings.Join(missing, ", ") + ". Terrain requires " + voxelTerrainMaterials + " in both survival and creative modes. Restore matching palette blocks and drop items from inspect.design_example.voxel; keep their material identities")
	}
	items := map[string]VoxelItem{}
	for i, item := range v.Items {
		if issue := voxelItemIssue(item, items, blocks); issue != "" {
			add(fmt.Sprintf("items[%d] %q %s", i, item.ID, issue))
		}
		items[item.ID] = item
	}
	for i, b := range v.Blocks {
		if _, ok := items[b.Drop]; !ok {
			add(fmt.Sprintf("blocks[%d] drop %q references no declared item", i, b.Drop))
		}
	}
	recipes := map[string]bool{}
	for i, r := range v.Recipes {
		if issue := voxelRecipeIssue(r, recipes, items); issue != "" {
			add(fmt.Sprintf("recipes[%d] %q %s", i, r.ID, issue))
		}
		recipes[r.ID] = true
	}
	enemies, total := map[string]bool{}, 0
	for i, e := range v.Enemies {
		if issue := voxelEnemyIssue(e, enemies, items); issue != "" {
			add(fmt.Sprintf("enemies[%d] %q %s", i, e.ID, issue))
		}
		enemies[e.ID] = true
		total += e.Count
	}
	if total > 24 {
		add(fmt.Sprintf("at most 24 enemies in total (got %d)", total))
	}
	goals := map[string]bool{}
	if len(v.Goals) > 8 {
		add(fmt.Sprintf("at most 8 goals (got %d)", len(v.Goals)))
	}
	for i, g := range v.Goals {
		if issue := voxelGoalIssue(g, goals, items); issue != "" {
			add(fmt.Sprintf("goals[%d] %q %s", i, g.ID, issue))
		}
		goals[g.ID] = true
	}
	// Report independent corrections together, but only evaluate a valid rule graph.
	if len(issues) > 0 {
		return bad(strings.Join(issues, "; "))
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
		add("provide a craftable recipe reachable from terrain resources")
	}
	for _, b := range v.Blocks {
		if (b.Material == "stone" || b.Material == "ore") && v.Mode == "survival" && b.Tier > tier {
			add("stone and ore require a reachable tool progression")
			break
		}
	}
	for i, g := range v.Goals {
		var issue string
		switch {
		case g.Kind == "craft" && !crafted[g.Item]:
			issue = fmt.Sprintf("craft %q is unreachable: no recipe producing it can be completed from terrain resources", g.Item)
		case g.Kind == "collect" && !collectible[g.Item]:
			issue = fmt.Sprintf("collect %q is unreachable: it is neither a terrain block drop nor an enemy drop", g.Item)
		case g.Kind == "place" && items[g.Item].Block == 0:
			issue = fmt.Sprintf("place %q needs an item whose block is a declared block ID", g.Item)
		case g.Kind == "place" && !reachable[g.Item]:
			issue = fmt.Sprintf("place %q is unreachable: the player can never obtain it", g.Item)
		case g.Kind == "defeat" && g.Count > total:
			issue = fmt.Sprintf("defeat count %d exceeds the %d enemies declared", g.Count, total)
		}
		if issue != "" {
			add(fmt.Sprintf("goals[%d] %q %s", i, g.ID, issue))
		}
	}
	if len(issues) > 0 {
		return bad(strings.Join(issues, "; "))
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil || len(data) > 64*1024 {
		return bad("formatted definition exceeds 64 KiB")
	}
	return nil
}

const voxelTerrainMaterials = "grass, dirt, stone, wood, leaves, ore, sand"

var voxelMaterials = strings.Split(voxelTerrainMaterials+", planks, brick", ", ")

const voxelIDRule = "must be lowercase snake_case (a-z first, then a-z, 0-9 or _, at most 40 characters)"

// The issue helpers name the first invalid field of one element together with
// its accepted values, so a model can correct it without guessing.
func voxelBlockIssue(b VoxelBlock, blocks map[int]bool, keys map[string]bool) string {
	switch {
	case b.ID < 1 || b.ID > 64:
		return fmt.Sprintf("id %d must be 1–64", b.ID)
	case blocks[b.ID]:
		return fmt.Sprintf("duplicate id %d", b.ID)
	case !voxelID.MatchString(b.Key):
		return fmt.Sprintf("key %q %s", b.Key, voxelIDRule)
	case keys[b.Key]:
		return fmt.Sprintf("duplicate key %q", b.Key)
	case len(b.Name) < 1 || len(b.Name) > 80:
		return "name must contain 1–80 bytes"
	case !slices.Contains(voxelMaterials, b.Material):
		return fmt.Sprintf("material %q must be one of %s", b.Material, strings.Join(voxelMaterials, ", "))
	case !voxelColor.MatchString(b.Color):
		return fmt.Sprintf("color %q must be #RRGGBB", b.Color)
	case b.Tier < 0 || b.Tier > 3:
		return fmt.Sprintf("tier %d must be 0–3", b.Tier)
	case !finite(b.Hardness) || b.Hardness < 0.1 || b.Hardness > 4:
		return fmt.Sprintf("hardness %v must be 0.1–4 seconds", b.Hardness)
	}
	return ""
}

func voxelItemIssue(item VoxelItem, items map[string]VoxelItem, blocks map[int]bool) string {
	_, duplicate := items[item.ID]
	switch {
	case duplicate:
		return "is a duplicate item id"
	case !voxelID.MatchString(item.ID):
		return "id " + voxelIDRule
	case len(item.Name) < 1 || len(item.Name) > 80:
		return "name must contain 1–80 bytes"
	case item.Block != 0 && !blocks[item.Block]:
		return fmt.Sprintf("block %d references no declared block id; use 0 for items that cannot be placed", item.Block)
	case item.Stack < 1 || item.Stack > 999:
		return fmt.Sprintf("stack %d must be 1–999", item.Stack)
	case item.Tier < 0 || item.Tier > 3:
		return fmt.Sprintf("tier %d must be 0–3", item.Tier)
	case !finite(item.Damage) || item.Damage < 0 || item.Damage > 100:
		return fmt.Sprintf("damage %v must be 0–100", item.Damage)
	}
	return ""
}

func voxelRecipeIssue(r VoxelRecipe, recipes map[string]bool, items map[string]VoxelItem) string {
	output, ok := items[r.Item]
	switch {
	case !voxelID.MatchString(r.ID):
		return "id " + voxelIDRule
	case recipes[r.ID]:
		return "is a duplicate recipe id"
	case !ok:
		return fmt.Sprintf("item %q is not a declared item", r.Item)
	case r.Count < 1 || r.Count > output.Stack:
		return fmt.Sprintf("count %d must be 1–%d (the output item's stack)", r.Count, output.Stack)
	case len(r.Ingredients) < 1 || len(r.Ingredients) > 8:
		return fmt.Sprintf("needs 1–8 ingredients (got %d)", len(r.Ingredients))
	}
	ids := make([]string, 0, len(r.Ingredients))
	for id := range r.Ingredients {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		item, ok := items[id]
		if !ok {
			return fmt.Sprintf("ingredient %q is not a declared item", id)
		}
		if n := r.Ingredients[id]; n < 1 || n > item.Stack*36 {
			return fmt.Sprintf("ingredient %q amount %d must be 1–%d", id, n, item.Stack*36)
		}
	}
	return ""
}

func voxelEnemyIssue(e VoxelEnemy, enemies map[string]bool, items map[string]VoxelItem) string {
	_, drop := items[e.Drop]
	switch {
	case !voxelID.MatchString(e.ID):
		return "id " + voxelIDRule
	case enemies[e.ID]:
		return "is a duplicate enemy id"
	case !slices.Contains([]string{"melee", "ranged"}, e.Behavior):
		return fmt.Sprintf("behavior %q must be melee or ranged", e.Behavior)
	case e.Count < 1 || e.Count > 24:
		return fmt.Sprintf("count %d must be 1–24", e.Count)
	case !finite(e.Health) || e.Health < 1 || e.Health > 1000:
		return fmt.Sprintf("health %v must be 1–1000", e.Health)
	case !finite(e.Damage) || e.Damage < 1 || e.Damage > 100:
		return fmt.Sprintf("damage %v must be 1–100", e.Damage)
	case !drop:
		return fmt.Sprintf("drop %q is not a declared item", e.Drop)
	}
	return ""
}

func voxelGoalIssue(g VoxelGoal, goals map[string]bool, items map[string]VoxelItem) string {
	_, hasItem := items[g.Item]
	switch {
	case !voxelID.MatchString(g.ID):
		return "id " + voxelIDRule
	case goals[g.ID]:
		return "is a duplicate goal id"
	case g.Count < 1 || g.Count > 10000:
		return fmt.Sprintf("count %d must be 1–10000", g.Count)
	case !slices.Contains([]string{"collect", "craft", "place", "defeat"}, g.Kind):
		return fmt.Sprintf("kind %q must be one of collect, craft, place, defeat", g.Kind)
	case g.Kind == "defeat" && g.Item != "":
		return "defeat goals count defeated enemies; omit item"
	case g.Kind != "defeat" && !hasItem:
		return fmt.Sprintf("item %q is not a declared item", g.Item)
	}
	return ""
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
