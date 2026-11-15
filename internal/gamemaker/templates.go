package gamemaker

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed templates/*.ts
var gameTemplates embed.FS

// unchangedStarterDataFiles preserves legacy scaffold compatibility while
// treating an existing changed scene or mechanics document as implementation.

func installGameTemplate(stage string, plan GamePlan) error {
	files, err := gameTemplateSources(plan)
	if err != nil {
		return err
	}
	for target, data := range files {
		if err := os.WriteFile(filepath.Join(stage, "src", target), data, 0o640); err != nil {
			return fmt.Errorf("install game template: %w", err)
		}
	}
	return nil
}

// Generate the optional template files.
func gameTemplateSources(plan GamePlan) (map[string][]byte, error) {
	if plan.Template == "voxel" {
		return voxelTemplateSources(plan)
	}
	if guided3D(plan.Template) || (plan.Template == "three" && plan.Scene != nil) {
		return threeTemplateSources(plan)
	}
	if !slices.Contains(templateNames()[:6], plan.Template) {
		return nil, fmt.Errorf("unknown 2D template %q", plan.Template)
	}
	var imports, entries []string
	packNames := map[string]string{}
	for _, a := range plan.Assets {
		if a.PackID == "" {
			continue
		}
		key := a.PackID + "@" + a.Version
		metadata := "sheet.json"
		if atlasPack(a.PackID) {
			metadata = "assets/" + a.AssetID + ".json"
			key += "/" + a.AssetID
		}
		name, ok := packNames[key]
		if !ok {
			name = fmt.Sprintf("pack%d", len(packNames))
			packNames[key] = name
			path, _ := json.Marshal("../assets/builtin/" + a.PackID + "/" + a.Version + "/" + metadata)
			imports = append(imports, fmt.Sprintf("import %s from %s;", name, path))
		}
		role, _ := json.Marshal(a.Role)
		id := a.AssetID
		if a.AssemblyID != "" {
			id = a.AssemblyID
		}
		assetID, _ := json.Marshal(id)
		entries = append(entries, fmt.Sprintf("[%s]: {meta:%s, id:%s, assembly:%t, direction:%s}", role, name, assetID, a.AssemblyID != "", mustJSONString(a.Direction)))
	}
	files := map[string][]byte{}
	for source, target := range map[string]string{plan.Template + ".ts": "main.ts", "common.ts": "common.ts"} {
		data, err := gameTemplates.ReadFile("templates/" + source)
		if err != nil {
			return nil, fmt.Errorf("read game template: %w", err)
		}
		if source == "common.ts" {

			data = []byte(strings.Replace(string(data), "width: 960, height: 540", fmt.Sprintf("width: %d, height: %d", plan.Width, plan.Height), 1))
			data = []byte(strings.Replace(string(data), "// PLAN_ASSET_IMPORTS", strings.Join(imports, "\n"), 1))
			data = []byte(strings.Replace(string(data), "const plannedAssets: any = {};", "const plannedAssets: any = {"+strings.Join(entries, ",\n")+"};", 1))
			data = []byte(strings.Replace(string(data), "{mode:'minimal',objective:''}; // PLAN_PLAYER_UI", "{mode:"+mustJSONString(plan.Template)+",objective:"+mustJSONString(plan.Objective)+"};", 1))
		}
		files[target] = data
	}
	presentation, err := presentationConfig(plan.Presentation)
	if err != nil {
		return nil, err
	}
	files["presentation.json"] = []byte(presentation)
	mechanics, err := json.Marshal(plan.Mechanics)
	if err != nil {
		return nil, fmt.Errorf("encode mechanics: %w", err)
	}
	files["mechanics.json"] = mechanics
	if plan.Scene == nil {
		files["scene.json"] = []byte("null\n")
	} else {
		scene, err := MarshalSceneJSON(*plan.Scene)
		if err != nil {
			return nil, err
		}
		files["scene.json"] = scene
	}
	return files, nil
}

func mustJSONString(value string) string { data, _ := json.Marshal(value); return string(data) }
