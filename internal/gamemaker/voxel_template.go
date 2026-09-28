package gamemaker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func voxelTemplateSources(plan GamePlan) (map[string][]byte, error) {
	if err := plan.Voxel.Validate(); err != nil {
		return nil, err
	}
	definition, err := json.MarshalIndent(plan.Voxel, "", "  ")
	if err != nil {
		return nil, err
	}
	common, err := gameTemplates.ReadFile("templates/voxel-common.ts")
	if err != nil {
		return nil, err
	}
	presentation, err := presentationConfig(plan.Presentation)
	if err != nil {
		return nil, err
	}
	main := "import { startVoxelGame } from './common';\nimport definition from './voxel.json';\nimport presentation from './presentation.json';\n\nstartVoxelGame(definition, { presentation, objective: " + mustJSONString(plan.Objective) + " });\n"
	return map[string][]byte{"main.ts": []byte(main), "common.ts": common, "voxel.json": definition, "presentation.json": []byte(presentation)}, nil
}

func checkVoxelBuildGraph(metafile string) error {
	var graph struct {
		Inputs  map[string]json.RawMessage `json:"inputs"`
		Outputs map[string]struct {
			Imports []struct {
				Path string `json:"path"`
			} `json:"imports"`
			Inputs map[string]struct {
				Bytes int `json:"bytesInOutput"`
			} `json:"inputs"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(metafile), &graph); err != nil {
		return err
	}
	definition, runtime := false, false
	for _, output := range graph.Outputs {
		for path, input := range output.Inputs {
			if filepath.ToSlash(path) == "src/voxel.json" && input.Bytes > 0 {
				definition = true
			}
		}
		for _, dependency := range output.Imports {
			if strings.HasSuffix(dependency.Path, "/vendor/aurago-voxel-1.js") {
				runtime = true
			}
		}
	}
	if !definition || !runtime {
		return fmt.Errorf("voxel implementation must import src/voxel.json and use the installed startVoxelGame runtime")
	}
	return nil
}

func checkVoxelProject(dir string) error {
	var manifest gameManifest
	if data, err := os.ReadFile(filepath.Join(dir, "game.json")); err == nil {
		if err = json.Unmarshal(data, &manifest); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(gamePlanPath)))
	if os.IsNotExist(err) {
		if manifest.Variant == "voxel" {
			return fmt.Errorf("voxel project requires an accepted voxel plan")
		}
		return nil
	}
	if err != nil {
		return err
	}
	var plan GamePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return err
	}
	if plan.Template != "voxel" {
		if manifest.Variant == "voxel" {
			return fmt.Errorf("voxel project requires an accepted voxel plan")
		}
		return nil
	}
	if manifest.Variant != "voxel" || manifest.Dimension != "3d" {
		return fmt.Errorf("voxel project variant and dimension cannot change")
	}
	data, err = os.ReadFile(filepath.Join(dir, "src", "voxel.json"))
	if err != nil {
		return fmt.Errorf("read voxel definition: %w", err)
	}
	_, err = ParseVoxelDefinition(data)
	return err
}
