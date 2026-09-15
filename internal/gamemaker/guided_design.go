package gamemaker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
)

// A small public design describes choices; the server owns technical plan fields.
type GameDesign struct {
	Presentation *Presentation `json:"presentation,omitempty"`
	// Scene is optional and validated separately from source code; omitting it keeps the classic flow.
	Scene *Scene `json:"scene,omitempty"`
	// Mechanics is a bounded optional helper declaration; arbitrary gameplay remains source-editable.
	Mechanics map[string]any `json:"mechanics,omitempty"`
	Base      string         `json:"base"`
	Objective string         `json:"objective"`
	Features  []string       `json:"features"`
	Assets    []DesignAsset  `json:"assets"`
	Settings  *GameSettings  `json:"settings,omitempty"`
	Preserve  []string       `json:"preserve,omitempty"`
	Scenarios []GameScenario `json:"scenarios,omitempty"`
}
type DesignAsset struct {
	Role       string `json:"role"`
	PackID     string `json:"pack_id,omitempty"`
	AssetID    string `json:"asset_id,omitempty"`
	AssemblyID string `json:"assembly_id,omitempty"`
	Fallback   string `json:"fallback,omitempty"`
}
type GameSettings struct {
	Goal     int     `json:"goal"`
	Speed    float64 `json:"speed"`
	Duration int     `json:"duration"`
}

func (g GameSettings) validate() error {
	if g.Goal < 1 || g.Goal > 24 || !finite(g.Speed) || g.Speed < 1 || g.Speed > 40 || (g.Duration != 0 && g.Duration < 15) || g.Duration > 600 {
		return fmt.Errorf("design.settings: goal 1–24, speed 1–40, duration 0 (no countdown) or 15–600 seconds required")
	}
	return nil
}
func is3DTemplate(name string) bool {
	return slices.Contains([]string{"three", "fps", "exploration", "transport", "flight", "space"}, name)
}
func guided3D(name string) bool { return name != "three" && is3DTemplate(name) }

func ExampleGameDesign(project Project) GameDesign {
	base := "topdown"
	if project.Dimension == "3d" {
		base = "exploration"
	}
	d := GameDesign{Base: base, Objective: "Collect the crystals before time runs out", Features: []string{"Collectibles and obstacles", "Score, timer and restart"}, Assets: []DesignAsset{}, Settings: &GameSettings{Goal: 5, Speed: 5, Duration: 120}}
	if project.Dimension != "3d" {
		d.Settings = nil
	}
	return d
}

// Only explicitly supplied top-level fields replace the failed draft. Arrays are
// replaced whole. Unknown fields/invalid JSON never poison the next correction.
func (s *Service) expandDesign(ctx context.Context, jobID string, project Project, data []byte) ([]byte, error) {
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(data, &patch); err != nil {
		return nil, fmt.Errorf("design: %w. Use inspect.design_example fields; level objects belong in scene.nodes, and generated platforms use scene_generate after plan acceptance. Free mechanics belong in features and source code", err)
	}
	s.mu.RLock()
	draft := map[string]json.RawMessage{}
	for k, v := range s.designDrafts[jobID] {
		draft[k] = v
	}
	s.mu.RUnlock()
	if err := normalizeDesignMechanics(patch, draft); err != nil {
		return nil, err
	}
	for k, v := range patch {
		if !bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			draft[k] = v
		}
	}
	// The correction for unsupported settings is to omit/null them. Do not
	// resurrect them from a failed draft when the effective base cannot use them.
	// Guided bases still retain omitted/null values, including a disabled timer.
	var base string
	if json.Unmarshal(draft["base"], &base) == nil && slices.Contains(templateNames(), base) && !guided3D(base) {
		if raw := patch["settings"]; len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			delete(draft, "settings")
		}
	}
	merged, err := json.Marshal(draft)
	if err != nil {
		return nil, err
	}
	if len(merged) > 24000 {
		return nil, fmt.Errorf("design exceeds 24000 bytes")
	}
	var design GameDesign
	dec := json.NewDecoder(bytes.NewReader(merged))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&design); err != nil {
		return nil, fmt.Errorf("design: %w. Use inspect.design_example fields; level objects belong in scene.nodes, and generated platforms use scene_generate after plan acceptance. Free mechanics belong in features and source code", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("design: submit one object")
	}
	s.mu.Lock()
	if s.designDrafts == nil {
		s.designDrafts = map[string]map[string]json.RawMessage{}
	}
	s.designDrafts[jobID] = draft
	s.mu.Unlock()
	plan, err := s.planFromDesign(ctx, jobID, project, design)
	if err != nil {
		return nil, err
	}
	return json.Marshal(plan)
}

// Some models flatten the documented mechanics object. Relocate only its four
// exact fields; normal plan validation still checks their values and limits.
func normalizeDesignMechanics(patch, draft map[string]json.RawMessage) error {
	var mechanics map[string]any
	changed := false
	raw := patch["mechanics"]
	explicit := len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
	if !explicit {
		raw = draft["mechanics"]
	}
	for _, key := range []string{"outcomes", "lives", "blocks", "events"} {
		value, exists := patch[key]
		if !exists {
			continue
		}
		delete(patch, key)
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			continue
		}
		if mechanics == nil {
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &mechanics); err != nil {
					return fmt.Errorf("design.mechanics: expected an object containing outcomes, lives, blocks and events: %w", err)
				}
			}
			if mechanics == nil {
				mechanics = map[string]any{}
			}
		}
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			return fmt.Errorf("design.mechanics.%s: %w", key, err)
		}
		if nested, exists := mechanics[key]; explicit && exists && nested != nil && !reflect.DeepEqual(nested, decoded) {
			return fmt.Errorf("design.mechanics.%s: conflicts with design.%s; keep one value inside mechanics", key, key)
		}
		mechanics[key] = decoded
		changed = true
	}
	if changed {
		encoded, err := json.Marshal(mechanics)
		if err != nil {
			return fmt.Errorf("design.mechanics: %w", err)
		}
		patch["mechanics"] = encoded
	}
	return nil
}

func (s *Service) planFromDesign(ctx context.Context, jobID string, project Project, d GameDesign) (GamePlan, error) {
	p := ExampleGamePlan(project)
	p.Template = d.Base
	p.Objective = d.Objective
	p.Scope = d.Features
	if !slices.Contains(templateNames(), d.Base) || is3DTemplate(d.Base) != (project.Dimension == "3d") {
		return p, fmt.Errorf("design.base: choose a base matching project dimension (%s)", project.Dimension)
	}
	if d.Settings != nil && !guided3D(d.Base) {
		return p, fmt.Errorf("design.settings: goal/speed/duration apply only to fps/exploration/transport/flight/space, not base %q. Keep base %q and resubmit set_design with {\"settings\":null}, or omit settings in the correction. Other design fields are retained. Put custom tuning in features and source, or supported movement values in mechanics.blocks[].params", d.Base, d.Base)
	}
	p.CoreLoop = "Use the displayed controls to pursue: " + d.Objective
	p.Rules = map[string]string{"progress": d.Objective, "failure": "Health or time exhausted", "completion": "Reach the objective; show result and allow restart"}
	if d.Base == "minimal" || d.Base == "three" {
		p.Rules = map[string]string{
			"progress":   d.Objective,
			"failure":    "No loss condition is required unless requested by the design",
			"completion": "No win condition is required unless requested by the design",
		}
	}
	p.Fallback = "Use the explicitly named procedural role where no library asset is selected"
	p.Gameplay = d.Settings
	seen := map[string]bool{}
	for _, a := range d.Assets {
		if strings.TrimSpace(a.Role) == "" || seen[a.Role] {
			return p, fmt.Errorf("design.assets: use a unique nonempty role: %q. Each role selects one visual asset; use distinct roles (for example cover_wall and cover_barrier) for different models and reuse a role in multiple scene placements. Put effects and sounds in presentation, not assets. Resubmit only the corrected assets array", a.Role)
		}
		seen[a.Role] = true
	}
	if guided3D(d.Base) && p.Gameplay == nil {
		p.Gameplay = &GameSettings{Goal: 5, Speed: 5, Duration: 120}
	}
	p.Presentation = d.Presentation
	p.Scene = d.Scene
	p.Mechanics = d.Mechanics
	p.Preserve = d.Preserve
	p.Scenarios = d.Scenarios
	if project.CurrentRevision > 0 {
		old, err := s.GetPlan(ctx, jobID)
		if err != nil {
			return p, err
		}
		if len(p.Preserve) == 0 {
			p.Preserve = []string{"Keep existing working controls, visuals and gameplay outside the requested edit"}
		}
		if old != nil {
			if d.Presentation == nil {
				p.Presentation = old.Presentation
			}
			if len(d.Assets) == 0 {
				p.Assets = old.Assets
			}
			if guided3D(d.Base) && d.Settings == nil && old.Gameplay != nil {
				p.Gameplay = old.Gameplay
			}
			if d.Scene == nil && old.Scene != nil {
				p.Scene = old.Scene
			}
			if d.Scenarios == nil {
				p.Scenarios = old.Scenarios
			}
			if d.Mechanics == nil && old.Mechanics != nil {
				p.Mechanics = old.Mechanics
			}
		}
	}
	if d.Base == "platformer" {
		p.Perspective = "side"
	}
	if d.Base == "board" {
		p.Perspective = "board"
	}
	if project.Dimension == "3d" {
		p.Controls = map[string]string{"move": "WASD", "primary": "Space / click", "restart": "R", "aim": "Mouse drag or arrows", "reload": "F", "altitude": "Q/E"}
	}
	if project.CurrentRevision == 0 && guided3D(d.Base) {
		chosen := d.Assets
		d.Assets = defaultModelRoles(d.Base)
		for _, a := range chosen {
			index := slices.IndexFunc(d.Assets, func(existing DesignAsset) bool { return existing.Role == a.Role })
			if index >= 0 {
				d.Assets[index] = a
			} else {
				d.Assets = append(d.Assets, a)
			}
		}
	}
	if len(d.Assets) > 0 {
		p.Assets = nil
		var viewErrors []string
		for i, a := range d.Assets {
			spec := PlanAsset{Role: a.Role, PackID: a.PackID, AssetID: a.AssetID, AssemblyID: a.AssemblyID, Direction: "none", DisplayHeight: 48, Origin: Point{.5, .5}, Collider: "rectangle", Scale: 1, Fallback: a.Fallback}
			if project.Dimension == "3d" {
				spec.Direction = "+Z"
				spec.Collider = "box"
			}
			if a.PackID != "" {
				detail, err := s.describeAsset(a.PackID, a.AssetID, a.AssemblyID)
				if err != nil {
					return p, fmt.Errorf("design.assets[%d]: %w", i, err)
				}
				spec.Version = detail.Version
				if project.Dimension == "2d" && !detail.supportsPerspective(p.Perspective) {
					message := fmt.Sprintf("design.assets[%d] role %q: %s/%s%s is %s-view art; base %q needs %s-view art", i, a.Role, a.PackID, a.AssetID, a.AssemblyID, detail.View, d.Base, p.Perspective)
					query := a.Role
					if detail.Asset != nil {
						query = strings.ReplaceAll(detail.Asset.Entity, "_", " ")
					}
					if detail.Assembly != nil {
						query = detail.Assembly.Name
					}
					matches, err := searchAssets(query, "", p.Perspective, 2, "sprite2d")
					if err != nil {
						return p, fmt.Errorf("find compatible art: %w", err)
					}
					alternatives := []DesignAsset{}
					for _, match := range matches {
						if !match.Fragment {
							alternatives = append(alternatives, DesignAsset{Role: a.Role, PackID: match.PackID, AssetID: match.AssetID, AssemblyID: match.AssemblyID})
						}
					}
					if len(alternatives) > 0 {
						encoded, _ := json.Marshal(alternatives)
						message += "; compatible alternatives (choose one): " + string(encoded)
					}
					viewErrors = append(viewErrors, message)
				}
				if detail.Model != nil {
					spec.Collider = "catalog"
				} else if detail.Asset != nil {
					spec.Direction = detail.Asset.Direction
					spec.Origin = detail.Asset.Origin
				}
				if detail.Assembly != nil {
					spec.Origin = detail.Assembly.Origin
				}
			} else if strings.TrimSpace(a.Fallback) == "" {
				return p, fmt.Errorf("design.assets[%d]: select catalog IDs or describe fallback graphics", i)
			}
			p.Assets = append(p.Assets, spec)
		}
		if len(viewErrors) > 0 {
			return p, fmt.Errorf("%s. Keep base %q. Resubmit set_design with the complete corrected assets array, retaining compatible roles. Search other artwork with search_assets(view=%q); do not set plan.perspective or asset.view in design", strings.Join(viewErrors, "; "), d.Base, p.Perspective)
		}
	}
	if p.Presentation != nil {
		p.SchemaVersion = 3
		p.Presentation.Version = PresentationVersion
	}
	if p.Scene != nil || p.Mechanics != nil {
		p.SchemaVersion = 4
	}
	return p, nil
}

// IDs are verified against the shipped catalog when accepting every design.
func defaultModelRoles(base string) []DesignAsset {
	roles := [][2]string{{"player", "humans-explorer-a"}, {"item", "props-crystal"}, {"tree", "vegetation-pine"}, {"enemy", "animals-wolf"}}
	switch base {
	case "fps":
		roles = [][2]string{{"arms", "fps-arms-modern"}, {"weapon", "fps-rifle"}, {"enemy", "humans-trooper-b"}, {"tree", "vegetation-pine"}}
	case "transport":
		roles = [][2]string{{"player", "road-pickup"}, {"cargo", "props-crate-wood"}, {"goal", "props-checkpoint"}, {"building", "architecture-warehouse"}}
	case "flight":
		roles = [][2]string{{"player", "aircraft-prop-plane"}, {"goal", "props-checkpoint"}, {"terrain", "landscape-island"}}
	case "space":
		roles = [][2]string{{"player", "space-scout"}, {"enemy", "space-asteroid-round"}, {"planet", "space-planet-earth"}}
	}
	out := []DesignAsset{}
	for _, pair := range roles {
		out = append(out, DesignAsset{Role: pair[0], PackID: ModelPackID, AssetID: pair[1]})
	}
	return out
}
