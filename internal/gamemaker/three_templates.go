package gamemaker

import (
	"encoding/json"
	"fmt"
	"strings"
)

func threeTemplateSources(plan GamePlan) (map[string][]byte, error) {
	var imports, roles []string
	for i, a := range plan.Assets {
		if a.PackID == "" {
			return nil, fmt.Errorf("guided 3D role %s requires a catalog model; use template three for procedural-only games", a.Role)
		}
		name := fmt.Sprintf("model%d", i)
		path := fmt.Sprintf("../assets/builtin/%s/%s/assets/%s.json", a.PackID, a.Version, a.AssetID)
		imports = append(imports, "import "+name+" from "+mustJSONString(path)+";")
		roles = append(roles, fmt.Sprintf("[%s]: {manifest:%s,id:%s,scale:%g,base:%s}", mustJSONString(a.Role), name, mustJSONString(a.AssetID), a.Scale, mustJSONString("assets/builtin/"+a.PackID+"/"+a.Version+"/")))
	}
	data, err := gameTemplates.ReadFile("templates/three-common.ts")
	if err != nil {
		return nil, err
	}
	common := strings.Replace(string(data), "// PLAN_MODEL_IMPORTS", strings.Join(imports, "\n"), 1)
	presentation, err := presentationConfig(plan.Presentation)
	if err != nil {
		return nil, err
	}
	common = strings.Replace(common, "const roles: any = {};", "const roles: any = {"+strings.Join(roles, ",\n")+"};", 1)
	settings := GameSettings{Goal: 5, Speed: 5, Duration: 120}
	if plan.Gameplay != nil {
		settings = *plan.Gameplay
	}
	config := map[string]any{"mode": plan.Template, "objective": plan.Objective, "goal": settings.Goal, "speed": settings.Speed, "duration": settings.Duration, "objects": []any{}}
	// Put editable level composition in the small entry file, not the renderer.
	objects := []map[string]any{}
	for _, a := range plan.Assets {
		count := 1
		if a.Role == "tree" {
			count = 16
		}
		if a.Role == "enemy" && plan.Template != "exploration" || a.Role == "item" || a.Role == "goal" && plan.Template == "flight" {
			count = settings.Goal
		}
		if a.Role == "player" || a.Role == "arms" || a.Role == "weapon" {
			continue
		}
		for i := 0; i < count; i++ {
			x, y, z := float64(0), float64(0), float64(3+i*7)
			switch a.Role {
			case "tree":
				x = float64(i%2*2-1) * float64(7+(i%3)*3)
				z = float64(i/2*6 - 8)
			case "building":
				x = 12
				z = 24
			case "enemy":
				if plan.Template == "exploration" {
					x = 6
					z = 15
				} else {
					z = 8 + float64(i*7)
					if i > 0 {
						x = float64(i%3-1) * 3
					}
				}
			case "cargo":
				z = 3
			case "goal":
				z = 15
				if plan.Template == "flight" {
					y = 5
					z = 7 + float64(i*10)
				}
			case "planet":
				x = -24
				y = 8
				z = 55
			case "terrain":
				x = 22
				y = -2
				z = 30
			default:
				if a.Role != "item" {
					x = 10 + float64(i)*3
				}
			}
			if plan.Template == "space" && a.Role == "enemy" {
				y = 4
			}
			objects = append(objects, map[string]any{"role": a.Role, "at": []float64{x, y, z}})
		}
	}
	// One object per line keeps the complete editable entry inside one read.
	lines := make([]string, 0, len(objects))
	for _, object := range objects {
		data, _ := json.Marshal(object)
		lines = append(lines, "    "+string(data))
	}
	encoded, _ := json.MarshalIndent(config, "", "  ")
	body := strings.Replace(string(encoded), `"objects": []`, `"objects": [`+"\n"+strings.Join(lines, ",\n")+"\n  ]", 1)
	main := "import { startGame } from './common';\n// Edit rules and level objects here. Keep the shared lifecycle in common.ts.\nstartGame(" + body + ");\n"
	return map[string][]byte{"main.ts": []byte(main), "common.ts": []byte(common), "presentation.json": []byte(presentation)}, nil
}
