package gamemaker

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
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
	if normalize(main) == normalize(phaserScaffold) {
		return true, nil
	}
	if normalize(main) == normalize(threeScaffold) {
		if dimension == "2d" {
			return true, nil
		}
		// A scene_set/scene_patch is an implementation even when the legacy
		// Three starter source remains unchanged. Missing legacy data files keep
		// the old starter result; any actual data change clears it.
		var mechanics []byte = []byte("null")
		if plan, planErr := s.GetPlan(ctx, jobID); planErr == nil && plan != nil && plan.Mechanics != nil {
			if encoded, encodeErr := json.Marshal(plan.Mechanics); encodeErr == nil {
				mechanics = encoded
			}
		}
		return s.unchangedStarterDataFiles(ctx, jobID, map[string][]byte{
			"scene.json":     []byte("null\n"),
			"mechanics.json": mechanics,
		})
	}
	if dimension != "2d" {
		plan, err := s.GetPlan(ctx, jobID)
		if err != nil {
			return false, err
		}
		if plan != nil && (guided3D(plan.Template) || (plan.Template == "three" && plan.Scene != nil)) {
			files, err := threeTemplateSources(*plan)
			if err != nil {
				return false, err
			}
			return s.unchangedManagedTemplateFiles(ctx, jobID, files, normalize)
		}
		return false, nil
	}
	plan, err := s.GetPlan(ctx, jobID)
	if err != nil || plan == nil {
		return false, err
	}
	files, err := gameTemplateSources(*plan)
	if err != nil {
		return false, err
	}
	return s.unchangedManagedTemplateFiles(ctx, jobID, files, normalize)
}

// unchangedStarterDataFiles preserves legacy scaffold compatibility while
// treating an existing changed scene or mechanics document as implementation.
func (s *Service) unchangedStarterDataFiles(ctx context.Context, jobID string, expected map[string][]byte) (bool, error) {
	for path, want := range expected {
		got, err := s.ReadJobFile(ctx, jobID, "src/"+path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, err
		}
		if strings.TrimSpace(got) != strings.TrimSpace(string(want)) {
			return false, nil
		}
	}
	return true, nil
}

func (s *Service) unchangedManagedTemplateFiles(ctx context.Context, jobID string, expected map[string][]byte, normalize func(string) string) (bool, error) {
	for path, want := range expected {
		got, err := s.ReadJobFile(ctx, jobID, "src/"+path)
		if err != nil {
			return false, err
		}
		if normalize(got) != normalize(string(want)) {
			return false, nil
		}
	}
	return true, nil
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
