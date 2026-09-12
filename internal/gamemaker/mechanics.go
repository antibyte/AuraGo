package gamemaker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

type mechanicBlock struct {
	ID      string  `json:"id"`
	Kind    string  `json:"kind"`
	Role    string  `json:"role,omitempty"`
	Target  string  `json:"target,omitempty"`
	Value   float64 `json:"value,omitempty"`
	Params  string  `json:"params,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

type mechanicEvent struct {
	Event  string `json:"event"`
	Effect string `json:"effect,omitempty"`
	Sound  string `json:"sound,omitempty"`
}

func validateMechanicBlocks(mechanics map[string]any) error {
	kinds := []string{"movement", "camera", "health", "collect", "destroy", "reach", "survive", "checkpoint", "patrol", "chase", "keepdistance", "damage", "projectile", "waves", "inventory", "dialogue", "unlock"}
	for key := range mechanics {
		if !slices.Contains([]string{"outcomes", "lives", "blocks", "events"}, key) {
			return fmt.Errorf("plan.mechanics.%s: unknown helper field; use source code for custom data", key)
		}
	}
	data, err := json.Marshal(mechanics["blocks"])
	if err != nil {
		return err
	}
	var blocks []mechanicBlock
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&blocks); err != nil {
		return fmt.Errorf("plan.mechanics.blocks: %w", err)
	}
	if len(blocks) > 32 {
		return fmt.Errorf("plan.mechanics.blocks: at most 32 helpers")
	}
	seen := map[string]bool{}
	for i, block := range blocks {
		if !sceneID(block.ID) || seen[block.ID] || !slices.Contains(kinds, block.Kind) || !finite(block.Value) {
			return fmt.Errorf("plan.mechanics.blocks[%d]: unique id, supported kind and finite value required", i)
		}
		seen[block.ID] = true
		if len(block.Role) > 64 || len(block.Target) > 96 || len(block.Params) > 4000 {
			return fmt.Errorf("plan.mechanics.blocks[%d]: oversized binding or parameters", i)
		}
		if block.Params != "" {
			var params map[string]any
			if err := json.Unmarshal([]byte(block.Params), &params); err != nil || params == nil {
				return fmt.Errorf("plan.mechanics.blocks[%d].params: expected a JSON object string", i)
			}
			if err := safeMechanicKeys(params); err != nil {
				return err
			}
			for _, key := range []string{"speed", "gravity", "jump_speed", "interval", "ttl", "lifetime", "cooldown", "duration", "seconds", "count", "amount", "health", "zoom", "fov", "smoothing"} {
				if value, exists := params[key]; exists {
					n, ok := value.(float64)
					if !ok || !finite(n) || n < 0 || n > 1e6 {
						return fmt.Errorf("mechanic %s: %s must be a finite nonnegative number at most 1000000", block.ID, key)
					}
				}
			}
			for _, key := range []string{"offset", "direction", "position"} {
				if value, exists := params[key]; exists {
					v, ok := value.([]any)
					if !ok || len(v) < 2 || len(v) > 3 {
						return fmt.Errorf("mechanic %s: %s requires 2 or 3 finite coordinates", block.ID, key)
					}
					for _, c := range v {
						n, ok := c.(float64)
						if !ok || !finite(n) || n < -1e6 || n > 1e6 {
							return fmt.Errorf("mechanic %s: invalid %s coordinate", block.ID, key)
						}
					}
				}
			}
		}
	}
	data, err = json.Marshal(mechanics["events"])
	if err != nil {
		return err
	}
	var events []mechanicEvent
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&events); err != nil {
		return fmt.Errorf("plan.mechanics.events: %w", err)
	}
	if len(events) > 32 {
		return fmt.Errorf("plan.mechanics.events: at most 32 bindings")
	}
	seen = map[string]bool{}
	for i, event := range events {
		if !sceneID(event.Event) || len(event.Event) > 64 || seen[event.Event] || len(event.Effect) > 64 || len(event.Sound) > 64 {
			return fmt.Errorf("plan.mechanics.events[%d]: unique event and bounded catalog IDs required", i)
		}
		seen[event.Event] = true
	}
	return nil
}

func safeMechanicKeys(value any) error {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if key == "__proto__" || key == "prototype" || key == "constructor" {
				return fmt.Errorf("mechanic parameters contain forbidden key %s", key)
			}
			if err := safeMechanicKeys(item); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := safeMechanicKeys(item); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMechanicBindings(plan GamePlan) error {
	var blocks []mechanicBlock
	raw, _ := json.Marshal(plan.Mechanics["blocks"])
	_ = json.Unmarshal(raw, &blocks)
	ids, roles := map[string]bool{}, map[string]bool{}
	for _, asset := range plan.Assets {
		roles[asset.Role] = true
	}
	if plan.Scene != nil {
		for _, node := range plan.Scene.Nodes {
			ids[node.ID] = true
		}
		for _, placement := range plan.Scene.Placements {
			roles[placement.AssetRole] = true
		}
	}
	for _, block := range blocks {
		if block.Enabled != nil && !*block.Enabled {
			continue
		}
		if plan.Scene != nil && block.Kind != "camera" && block.Target == "" && block.Role == "" {
			return fmt.Errorf("mechanic %s requires an explicit target node or asset role", block.ID)
		}
		if plan.Scene != nil && (block.Target != "" && !ids[block.Target] || block.Role != "" && !roles[block.Role]) {
			return fmt.Errorf("mechanic %s references an unknown target or asset role", block.ID)
		}
	}
	var events []mechanicEvent
	raw, _ = json.Marshal(plan.Mechanics["events"])
	_ = json.Unmarshal(raw, &events)
	if len(events) == 0 {
		return nil
	}
	effects, sounds, err := resolvePresentation(plan.Presentation)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.Effect != "" && !slices.ContainsFunc(effects, func(a PresentationAsset) bool { return a.ID == event.Effect }) {
			return fmt.Errorf("mechanic event %s: effect %s is absent from presentation selection", event.Event, event.Effect)
		}
		if event.Sound != "" && !slices.ContainsFunc(sounds, func(a PresentationAsset) bool { return a.ID == event.Sound }) {
			return fmt.Errorf("mechanic event %s: sound %s is absent from presentation selection", event.Event, event.Sound)
		}
	}
	return nil
}

func checkMechanicsSource(dir, content string) error {
	if err := validateMechanicsSource(content); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(gamePlanPath)))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var plan GamePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(content), &plan.Mechanics); err != nil {
		return err
	}
	if data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(SceneFilePath))); err == nil && !bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		scene, err := DecodeSceneJSON(data)
		if err != nil {
			return err
		}
		plan.Scene = &scene
	}
	return validateMechanicBindings(plan)
}

func validateMechanicsSource(content string) error {
	var mechanics map[string]any
	if err := json.Unmarshal([]byte(content), &mechanics); err != nil {
		return fmt.Errorf("mechanics JSON: %w", err)
	}
	return validateGameMechanics(mechanics)
}
