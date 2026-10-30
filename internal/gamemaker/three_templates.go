package gamemaker

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

func threeTemplateSources(plan GamePlan) (map[string][]byte, error) {
	var imports, roles []string
	for i, a := range plan.Assets {
		if a.PackID == "" && plan.Scene == nil {
			return nil, fmt.Errorf("guided 3D role %s requires a catalog model; use template three for procedural-only games", a.Role)
		}
		if a.PackID == "" {
			continue
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
	mechanics, err := json.Marshal(plan.Mechanics)
	if err != nil {
		return nil, fmt.Errorf("encode mechanics: %w", err)
	}
	if plan.Scene == nil {
		// Preserve the established role-based starter until an agent explicitly
		// creates a scene. The null document is the legacy disabled-scene marker.
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
					} else if i > 0 && (plan.Template == "fps" || plan.Template == "space") {
						// Staggered across the clearing instead of a firing line.
						x = float64((i*7)%5-2) * 3.5
						z = 8 + float64(i*6+(i%2)*2)
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
		main := guidedEntrySource(plan, settings, objects, starterLevels(plan, settings), false)
		return map[string][]byte{"main.ts": []byte(main), "common.ts": []byte(common), "presentation.json": []byte(presentation), "mechanics.json": mechanics, "scene.json": []byte("null\n")}, nil
	}
	sceneData, err := MarshalSceneJSON(*plan.Scene)
	if err != nil {
		return nil, err
	}
	main := guidedEntrySource(plan, settings, nil, nil, true)
	return map[string][]byte{"main.ts": []byte(main), "common.ts": []byte(common), "presentation.json": []byte(presentation), "mechanics.json": mechanics, "scene.json": sceneData}, nil
}

// guidedEntrySource writes src/main.ts as short lines. Each editable fact stays
// under the adapt-mode snippet bound so a small model can replace one line
// instead of rewriting the file. The startGame argument is the same config the
// helper already reads: mode, objective, pacing, objects and, for a role-based
// starter, levels.
func guidedEntrySource(plan GamePlan, settings GameSettings, objects []map[string]any, levels []map[string]any, scene bool) string {
	var b strings.Builder
	b.WriteString("import { startGame } from './common';\n")
	if scene {
		b.WriteString("// Tune mode and pacing here. Geometry and behavior live in scene.json.\n")
		b.WriteString("// Custom hooks: step(dt, api), action(api), reset(api); read held keys with api.input.isDown(\"s\").\n")
	} else {
		b.WriteString("// Edit rules and legacy level objects here. Keep the shared lifecycle in common.ts.\n")
		b.WriteString("// Hooks: step(dt, api), action(api), reset(api). Read held keys with api.input.isDown(\"s\"); duration:0 disables countdown.\n")
	}
	fmt.Fprintf(&b, "const mode = %s;\n", mustJSONString(plan.Template))
	fmt.Fprintf(&b, "const objective = %s;\n", mustJSONString(plan.Objective))
	fmt.Fprintf(&b, "const goal = %d;\n", settings.Goal)
	fmt.Fprintf(&b, "const speed = %s;\n", formatPlainNumber(settings.Speed))
	fmt.Fprintf(&b, "const duration = %d;\n", settings.Duration)
	writeJSONArray(&b, "objects", objects)
	if !scene {
		writeLevelArray(&b, levels)
		b.WriteString("startGame({mode, objective, goal, speed, duration, objects, levels});\n")
	} else {
		b.WriteString("startGame({mode, objective, goal, speed, duration, objects});\n")
	}
	return b.String()
}

func formatPlainNumber(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", value), "0"), ".")
}

func writeJSONArray(b *strings.Builder, name string, objects []map[string]any) {
	if len(objects) == 0 {
		b.WriteString("const " + name + " = [];\n")
		return
	}
	// The comma starts the next line, so a matched object line is one complete
	// JSON value and replacing it cannot drop the separator.
	b.WriteString("const " + name + " = [\n")
	for i, object := range objects {
		if i > 0 {
			b.WriteString(",")
		}
		encoded, _ := json.Marshal(object)
		b.Write(encoded)
		b.WriteString("\n")
	}
	b.WriteString("];\n")
}

func writeLevelArray(b *strings.Builder, levels []map[string]any) {
	names := make([]string, len(levels))
	for i, level := range levels {
		name := fmt.Sprintf("stage%d", i+1)
		names[i] = name
		objects, _ := level["objects"].([]map[string]any)
		meta := make(map[string]any, len(level))
		for key, value := range level {
			if key != "objects" {
				meta[key] = value
			}
		}
		encoded, _ := json.Marshal(meta)
		fmt.Fprintf(b, "const %s = %s;\n", name, encoded)
		if len(objects) > 0 {
			writeJSONArray(b, name+"Objects", objects)
		}
	}
	b.WriteString("const levels = [\n")
	for i, name := range names {
		if i > 0 {
			b.WriteString(",")
		}
		objects, _ := levels[i]["objects"].([]map[string]any)
		if len(objects) == 0 {
			b.WriteString(name + "\n")
			continue
		}
		fmt.Fprintf(b, "{...%s, objects: %sObjects}\n", name, name)
	}
	b.WriteString("];\n")
}

var starterLevelTitles = map[string][3]string{
	"exploration": {"Crystal meadow", "Wolf forest", "Summit ruins"},
	"fps":         {"Forest edge", "Supply camp", "Deep bunker"},
	"transport":   {"Depot run", "Warehouse maze", "Night delivery"},
	"flight":      {"Harbor gates", "Island slalom", "Storm ring"},
	"space":       {"Asteroid belt", "Debris field", "Planet ring"},
}

// starterLevels gives new role-based guided 3D games three stages. The first keeps
// the objective along the starting heading; later stages scatter it on a winding
// route with guards beside it, cover in front of guards and clustered scenery, so
// agents start from a designed layout instead of a line of objects.
func starterLevels(plan GamePlan, settings GameSettings) []map[string]any {
	titles, ok := starterLevelTitles[plan.Template]
	if !ok {
		return nil
	}
	levels := []map[string]any{{"id": "stage-1", "title": titles[0]}}
	for level := 1; level <= 2; level++ {
		objects, goal := designedStageObjects(plan, settings, level)
		levels = append(levels, map[string]any{"id": fmt.Sprintf("stage-%d", level+1), "title": titles[level], "objects": objects, "goal": goal})
	}
	return levels
}

func designedStageObjects(plan GamePlan, settings GameSettings, level int) ([]map[string]any, int) {
	has := map[string]bool{}
	for _, a := range plan.Assets {
		has[a.Role] = true
	}
	round := func(v float64) float64 { return math.Round(v*10) / 10 }
	objects := []map[string]any{}
	add := func(role string, x, y, z float64) {
		if has[role] {
			objects = append(objects, map[string]any{"role": role, "at": []float64{round(x), round(y), round(z)}})
		}
	}
	count := min(24, max(settings.Goal, 3)+level*2)
	amplitude, spacing := 10+float64(level)*4, 9+float64(level)*2
	route := func(i int) (float64, float64) {
		return math.Sin(float64(i)*1.3+float64(level)) * amplitude, 10 + float64(i)*spacing
	}
	goal := count
	switch plan.Template {
	case "exploration":
		for i := 0; i < count; i++ {
			x, z := route(i)
			add("item", x, 0, z)
			if i%2 == 1 {
				add("enemy", x+5, 0, z+3)
			}
		}
	case "fps", "space":
		for i := 0; i < count; i++ {
			x, z := route(i)
			y := 0.0
			if plan.Template == "space" {
				y = 2 + float64(i%3)*2
			}
			add("enemy", x, y, z)
			add("tree", x+2.5, 0, z-3)
		}
	case "flight":
		for i := 0; i < count; i++ {
			x, z := route(i)
			add("goal", x, 4+float64((i+level)%3)*3, z)
		}
	case "transport":
		goal = 1
		side := float64(1 - 2*(level%2))
		add("cargo", side*(14+float64(level)*4), 0, 25+float64(level)*15)
		add("goal", -side*(18+float64(level)*5), 0, 60+float64(level)*25)
		for i := 0; i < 2+level*2; i++ {
			x, z := route(i)
			add("building", x*1.4, 0, z+6)
		}
	}
	// Scenery clusters beside the route, never on it.
	for c := 0; c < 4+level*2; c++ {
		x, z := route(c * 2)
		side := float64(1 - 2*(c%2))
		for t := 0; t < 3; t++ {
			add("tree", x+side*(9+float64(t)*2.5), 0, z+float64(t)*2-2)
		}
	}
	add("planet", -24, 8, 55+float64(level)*20)
	add("terrain", 22, -2, 30+float64(level)*25)
	return objects, goal
}
