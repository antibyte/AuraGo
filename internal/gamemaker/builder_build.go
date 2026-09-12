package gamemaker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Resolve scene bindings from the accepted plan, not guessed asset names. This
// is used by both direct file writes and builds of already stored projects.
func sceneProjectDiagnostics(dir string, data []byte) []SceneDiagnostic {
	var plan GamePlan
	planData, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(gamePlanPath)))
	if err == nil {
		if err = json.Unmarshal(planData, &plan); err != nil {
			return []SceneDiagnostic{sceneError(gamePlanPath, "invalid accepted plan")}
		}
	} else if !os.IsNotExist(err) {
		return []SceneDiagnostic{sceneError(gamePlanPath, "cannot read accepted plan")}
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		if plan.Scene != nil {
			return []SceneDiagnostic{sceneError(SceneFilePath, "restore the accepted scene; null is only the legacy disabled scene")}
		}
		return nil
	}
	scene, err := DecodeSceneJSON(data)
	if err != nil {
		return []SceneDiagnostic{sceneError(SceneFilePath, err.Error())}
	}
	if err := loadSceneMechanics(dir, &scene); err != nil {
		return []SceneDiagnostic{sceneError("mechanics", err.Error())}
	}
	var manifest gameManifest
	if raw, err := os.ReadFile(filepath.Join(dir, "game.json")); err == nil && json.Unmarshal(raw, &manifest) == nil && manifest.Dimension != scene.Dimension {
		return []SceneDiagnostic{sceneError("dimension", "scene must match the game's dimension")}
	}
	roles := map[string]PlanAsset{}
	for _, a := range plan.Assets {
		roles[a.Role] = a
	}
	catalog, err := sceneCatalogForPlan(plan)
	if err != nil {
		return []SceneDiagnostic{sceneError("assets", err.Error())}
	}
	var out []SceneDiagnostic
	for i := range scene.Placements {
		p := &scene.Placements[i]
		a, ok := roles[p.AssetRole]
		if !ok {
			if p.AssetID != "" {
				out = append(out, sceneError(fmt.Sprintf("placements[%d].asset_role", i), "asset role is not in the accepted plan"))
			}
			continue // Explicitly unbound procedural objects remain available.
		}
		id := a.AssetID
		if a.AssemblyID != "" {
			id = a.AssemblyID
		}
		if p.AssetID != "" && p.AssetID != id {
			out = append(out, sceneError(fmt.Sprintf("placements[%d].asset_id", i), "must match the accepted asset for role "+p.AssetRole))
			continue
		}
		p.AssetID = id

	}
	// Validate procedural bindings alongside real ones without disabling catalog
	// validation just because one object intentionally has no catalog asset.
	for _, p := range scene.Placements {
		if p.AssetID == "" {
			a := catalog.Assets[""]
			a.Roles = append(a.Roles, p.AssetRole)
			catalog.Assets[""] = a
		}
	}
	return append(out, ValidateScene(scene, catalog)...)
}

// Use the actual editable helper configuration for static movement estimates.
func loadSceneMechanics(dir string, scene *Scene) error {
	raw, err := os.ReadFile(filepath.Join(dir, "src", "mechanics.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var mechanics map[string]any
	if err := json.Unmarshal(raw, &mechanics); err != nil {
		return err
	}
	metadata := make(map[string]any, len(scene.Metadata)+1)
	for key, value := range scene.Metadata {
		metadata[key] = value
	}
	metadata["mechanics"] = mechanics
	scene.Metadata = metadata
	return nil
}

// Resolve the same local dimensions for spatial validation and generation.
func sceneCatalogForPlan(plan GamePlan) (AssetCatalog, error) {
	catalog := AssetCatalog{Assets: map[string]SceneAsset{}}
	var models *ModelManifest
	for _, a := range plan.Assets {
		id := a.AssetID
		if a.AssemblyID != "" {
			id = a.AssemblyID
		}
		asset := catalog.Assets[id]
		asset.ID = id
		asset.Roles = append(asset.Roles, a.Role)
		if a.PackID == ModelPackID {
			if models == nil {
				m, err := readModelManifest()
				if err != nil {
					return catalog, err
				}
				models = &m
			}
			for _, m := range models.Assets {
				if m.ID == id {
					scale := a.Scale
					if scale <= 0 {
						scale = 1
					}
					for axis := 0; axis < 3; axis++ {
						asset.Bounds.Min[axis] = min(asset.Bounds.Min[axis], m.Bounds.Min[axis]*scale)
						asset.Bounds.Max[axis] = max(asset.Bounds.Max[axis], m.Bounds.Max[axis]*scale)
					}
					for _, socket := range m.Sockets {
						asset.Sockets = append(asset.Sockets, socket.ID)
					}
					break
				}
			}
		} else if a.PackID != "" && a.DisplayHeight > 0 {
			pack, _, assemblies, _, err := readPackUsage(a.PackID)
			if err != nil {
				return catalog, err
			}
			width, height := float64(pack.FrameWidth), float64(pack.FrameHeight)
			for _, assembly := range assemblies {
				if assembly.ID == a.AssemblyID {
					width, height = float64(assembly.Width), float64(assembly.Height)
					break
				}
			}
			if height > 0 {
				width = width * a.DisplayHeight / height
				height = a.DisplayHeight
				asset.Bounds = SceneBounds{Min: Vec3{-width / 2, -height / 2, 0}, Max: Vec3{width / 2, height / 2, 0}}
			}
		}
		catalog.Assets[id] = asset
	}
	return catalog, nil
}

func validateBuilderSource(dir, path, content string) error {
	if filepath.ToSlash(path) == "src/mechanics.json" {
		return checkMechanicsSource(dir, content)
	}
	if filepath.ToSlash(path) != SceneFilePath {
		return nil
	}
	for _, d := range sceneProjectDiagnostics(dir, []byte(content)) {
		if d.Severity == "error" {
			return fmt.Errorf("scene %s: %s", d.Path, d.Message)
		}
	}
	oldData, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(SceneFilePath)))
	if err == nil && !bytes.Equal(bytes.TrimSpace(oldData), []byte("null")) {
		old, oldErr := DecodeSceneJSON(oldData)
		next, nextErr := DecodeSceneJSON([]byte(content))
		if oldErr == nil && nextErr == nil {
			return protectPinnedScene(old, next)
		}
	}
	return nil
}

// Pins also apply to direct file edits. Unpinning is an explicit separate edit;
// combining an unpin with moved/replaced content is rejected.
func sceneEntries(scene Scene) map[string]map[string]any {
	data, _ := json.Marshal(scene)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(data, &fields)
	out := map[string]map[string]any{}
	for _, kind := range []string{"nodes", "regions", "placements", "colliders", "attachments", "zones", "routes"} {
		var entries []map[string]any
		_ = json.Unmarshal(fields[kind], &entries)
		for _, entry := range entries {
			if id, ok := entry["id"].(string); ok {
				out[kind+"/"+id] = entry
			}
		}
	}
	return out
}

func protectPinnedScene(previous, next Scene) error {
	before, after := sceneEntries(previous), sceneEntries(next)
	for key, entry := range before {
		region, _ := entry["region_id"].(string)
		node, _ := entry["node_id"].(string)
		if entry["pinned"] != true && before["regions/"+region]["pinned"] != true && before["nodes/"+node]["pinned"] != true {
			continue
		}
		candidate := after[key]
		if candidate == nil {
			return fmt.Errorf("scene %s is pinned; unpin it explicitly before removing it", key)
		}
		if entry["pinned"] == true && candidate["pinned"] != true {
			copyEntry := make(map[string]any, len(entry))
			for k, v := range entry {
				if k != "pinned" {
					copyEntry[k] = v
				}
			}
			if reflect.DeepEqual(copyEntry, candidate) {
				continue
			}
		}
		if !reflect.DeepEqual(entry, candidate) {
			return fmt.Errorf("scene %s is pinned; unpin it explicitly before modifying it", key)
		}
	}
	return nil
}

func sceneChanges(previous, next Scene) *SceneChanges {
	before, after := sceneEntries(previous), sceneEntries(next)
	result := &SceneChanges{Counts: map[string]int{"added": 0, "modified": 0, "removed": 0}}
	keys := make([]string, 0, len(before)+len(after))
	for key := range before {
		keys = append(keys, key)
	}
	for key := range after {
		if before[key] == nil {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	add := func(kind, id string, list *[]string) {
		result.Counts[kind]++
		if len(*list) < 32 {
			*list = append(*list, id)
		}
	}
	for _, key := range keys {
		switch {
		case before[key] == nil:
			add("added", key, &result.Added)
		case after[key] == nil:
			add("removed", key, &result.Removed)
		case !reflect.DeepEqual(before[key], after[key]):
			add("modified", key, &result.Modified)
		}
	}
	if previous.WorldBounds != next.WorldBounds {
		add("modified", "world_bounds", &result.Modified)
	}
	if !reflect.DeepEqual(previous.CameraBounds, next.CameraBounds) {
		add("modified", "camera_bounds", &result.Modified)
	}
	if !reflect.DeepEqual(previous.Levels, next.Levels) {
		add("modified", "levels", &result.Modified)
	}
	if previous.Seed != next.Seed {
		add("modified", "seed", &result.Modified)
	}
	return result
}

func checkBuilderScene(dir string) []Diagnostic {
	if mechanics, err := os.ReadFile(filepath.Join(dir, "src", "mechanics.json")); err == nil {
		if err := checkMechanicsSource(dir, string(mechanics)); err != nil {
			return []Diagnostic{{Level: "error", File: "src/mechanics.json", Message: err.Error()}}
		}
	} else if !os.IsNotExist(err) {
		return []Diagnostic{{Level: "error", File: "src/mechanics.json", Message: "Cannot read mechanics"}}
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(SceneFilePath)))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []Diagnostic{{Level: "error", File: SceneFilePath, Message: "Cannot read scene"}}
	}
	var out []Diagnostic
	diagnostics := sceneProjectDiagnostics(dir, data)
	for _, severity := range []string{"error", "warning"} {
		for _, d := range diagnostics {
			if d.Severity != severity {
				continue
			}
			out = append(out, Diagnostic{Level: severity, File: SceneFilePath, Message: d.Path + ": " + d.Message})
			if len(out) == 24 {
				return out
			}
		}
	}
	return out
}

// Import presence is necessary, not proof that a game implements its rules.
func checkBuilderBuildGraph(dir, metafile string) error {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(SceneFilePath)))
	if os.IsNotExist(err) || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	if err != nil {
		return err
	}
	scene, err := DecodeSceneJSON(data)
	if err != nil {
		return err
	}
	if len(scene.Nodes)+len(scene.Zones) == 0 {
		return nil
	}
	var meta struct {
		Inputs  map[string]json.RawMessage
		Outputs map[string]struct{ Imports []struct{ Path string } }
	}
	if err := json.Unmarshal([]byte(metafile), &meta); err != nil {
		return err
	}
	for _, output := range meta.Outputs {
		for _, dependency := range output.Imports {
			if strings.HasSuffix(dependency.Path, "/scene-builder.js") && meta.Inputs[SceneFilePath] != nil {
				return nil
			}
		}
	}
	return fmt.Errorf("scene is disconnected: import src/scene.json and the local scene-builder.js through the existing common.ts lifecycle, or use the documented custom scene adapter")
}
