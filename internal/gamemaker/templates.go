package gamemaker

import (
	"context"
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

// ponytail: exact starter detection; broader plan compliance still needs gameplay/visual checks.
func (s *Service) unchangedGameStarter(ctx context.Context, jobID, dimension string) (bool, error) {
	normalize := func(text string) string {
		text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
		// The build injects this prelude; it is not an agent implementation.
		return strings.TrimSpace(strings.TrimPrefix(text, strings.TrimSpace(diagnosticsPrelude)))
	}
	main, err := s.ReadJobFile(ctx, jobID, "src/main.ts")
	if err != nil {
		return false, err
	}
	if normalize(main) == normalize(threeScaffold) || normalize(main) == normalize(phaserScaffold) {
		return true, nil
	}
	if dimension != "2d" {
		plan, err := s.GetPlan(ctx, jobID)
		if err != nil {
			return false, err
		}
		if plan != nil && guided3D(plan.Template) {
			files, err := threeTemplateSources(*plan)
			if err != nil {
				return false, err
			}
			common, err := s.ReadJobFile(ctx, jobID, "src/common.ts")
			return err == nil && normalize(main) == normalize(string(files["main.ts"])) && normalize(common) == normalize(string(files["common.ts"])), err
		}
		return false, nil
	}
	plan, err := s.GetPlan(ctx, jobID)
	if err != nil || plan == nil {
		return false, err
	}
	files, err := gameTemplateSources(*plan)
	if err != nil || normalize(main) != normalize(string(files["main.ts"])) {
		return false, err
	}
	common, err := s.ReadJobFile(ctx, jobID, "src/common.ts")
	return err == nil && normalize(common) == normalize(string(files["common.ts"])), err
}

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

// Use the same plan-bound sources for installation and unchanged-template checks.
func gameTemplateSources(plan GamePlan) (map[string][]byte, error) {
	if guided3D(plan.Template) {
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
		name, ok := packNames[key]
		if !ok {
			name = fmt.Sprintf("pack%d", len(packNames))
			packNames[key] = name
			path, _ := json.Marshal("../assets/builtin/" + a.PackID + "/" + a.Version + "/sheet.json")
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
		}
		files[target] = data
	}
	presentation, err := presentationConfig(plan.Presentation)
	if err != nil {
		return nil, err
	}
	files["presentation.json"] = []byte(presentation)
	return files, nil
}

func mustJSONString(value string) string { data, _ := json.Marshal(value); return string(data) }
