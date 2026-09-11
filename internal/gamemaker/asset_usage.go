package gamemaker

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type TransformRule struct {
	Mode           string  `json:"mode"`
	ForwardRadians float64 `json:"forward_radians"`
	FlipX          bool    `json:"flip_x"`
	FlipY          bool    `json:"flip_y"`
}
type PackAsset struct {
	Model        *ModelAsset   `json:"model,omitempty"`
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Tags         []string      `json:"tags"`
	View         string        `json:"view"`
	Direction    string        `json:"direction"`
	Entity       string        `json:"entity"`
	Action       string        `json:"action"`
	Frames       []int         `json:"frames"`
	Origin       Point         `json:"origin"`
	AssemblyPart bool          `json:"assembly_part,omitempty"`
	Transform    TransformRule `json:"transform"`
}
type PackAnimation struct {
	ID        string `json:"id"`
	AssetID   string `json:"asset_id"`
	Frames    []int  `json:"frames"`
	FrameRate int    `json:"frame_rate"`
	Repeat    int    `json:"repeat"`
	Yoyo      bool   `json:"yoyo"`
	Entity    string `json:"entity"`
	Action    string `json:"action"`
	Direction string `json:"direction"`
}
type PackAssembly struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	View        string        `json:"view"`
	Direction   string        `json:"direction"`
	Transform   TransformRule `json:"transform"`
	Width       int           `json:"width"`
	Height      int           `json:"height"`
	Origin      Point         `json:"origin"`
	Parts       []struct {
		AssetID     string `json:"asset_id"`
		Frame       int    `json:"frame"`
		X           int    `json:"x"`
		Y           int    `json:"y"`
		AnimationID string `json:"animation_id,omitempty"`
	} `json:"parts"`
}
type AssetSearchResult struct {
	Kind           string   `json:"kind,omitempty"`
	Category       string   `json:"category,omitempty"`
	Entity         string   `json:"entity"`
	Actions        []string `json:"actions"`
	MissingActions []string `json:"missing_actions"`
	PackID         string   `json:"pack_id"`
	Version        string   `json:"version"`
	AssetID        string   `json:"asset_id,omitempty"`
	AssemblyID     string   `json:"assembly_id,omitempty"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	View           string   `json:"view"`
	Direction      string   `json:"direction"`
	Fragment       bool     `json:"fragment,omitempty"`
	score          int
}
type AssetDetail struct {
	Presentation *PresentationAsset `json:"presentation,omitempty"`
	// Usage must precede bulky model data so bounded tool summaries retain it.
	Example        string          `json:"example"`
	Model          *ModelAsset     `json:"model,omitempty"`
	PackID         string          `json:"pack_id"`
	Version        string          `json:"version"`
	View           string          `json:"view"`
	Asset          *PackAsset      `json:"asset,omitempty"`
	Assembly       *PackAssembly   `json:"assembly,omitempty"`
	Variants       []PackAsset     `json:"variants,omitempty"`
	Animations     []PackAnimation `json:"animations"`
	MissingActions []string        `json:"missing_actions"`
}

func readPackUsage(id string) (AssetPack, []PackAsset, []PackAssembly, []PackAnimation, error) {
	if id == ModelPackID {
		return readModelUsage()
	}
	data, err := bundledAssetPackFile(id, "sheet.json")
	if err != nil {
		return AssetPack{}, nil, nil, nil, err
	}
	var p AssetPack
	var assets []PackAsset
	var assemblies []PackAssembly
	var animations []PackAnimation
	if err = json.Unmarshal(data, &p); err == nil {
		err = json.Unmarshal(p.Assets, &assets)
	}
	if err == nil && len(p.Assemblies) > 0 {
		err = json.Unmarshal(p.Assemblies, &assemblies)
	}
	if err == nil {
		err = json.Unmarshal(p.Animations, &animations)
	}
	return p, assets, assemblies, animations, err
}

func (s *Service) SearchAssets(query, packID, view string, limit int, kinds ...string) ([]AssetSearchResult, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	if !s.policy.Enabled {
		return nil, ErrDisabled
	}
	if len(query) > 200 {
		return nil, fmt.Errorf("asset query exceeds 200 characters")
	}
	if view != "" && !slices.Contains([]string{"side", "top", "board", "3d"}, view) {
		return nil, fmt.Errorf("view must be side, top, board or 3d")
	}
	if limit <= 0 {
		limit = 6
	}
	if limit > 12 {
		limit = 12
	}
	kind := ""
	if len(kinds) > 0 {
		kind = kinds[0]
	}
	if presentationPack(packID) && kind == "" {
		if packID == EffectsPackID {
			kind = "effect"
		} else {
			kind = "audio"
		}
	}
	if kind == "effect" || kind == "audio" {
		return searchPresentation(query, packID, kind, view, limit)
	}
	if kind != "" && kind != "sprite2d" && kind != "model3d" {
		return nil, fmt.Errorf("unknown asset_kind")
	}
	data, err := assetPackFS.ReadFile("asset_packs/catalog.json")
	if err != nil {
		return nil, err
	}
	var packs []AssetPackSummary
	if err = json.Unmarshal(data, &packs); err != nil {
		return nil, err
	}
	if packID != "" && !slices.ContainsFunc(packs, func(p AssetPackSummary) bool { return p.ID == packID }) {
		return nil, ErrNotFound
	}
	terms := strings.Fields(strings.ToLower(query))
	out := []AssetSearchResult{}
	for _, summary := range packs {
		if presentationPack(summary.ID) || kind != "" && summary.Kind != kind {
			continue
		}
		if packID != "" && summary.ID != packID {
			continue
		}
		p, assets, assemblies, animations, err := readPackUsage(summary.ID)
		if err != nil {
			return nil, err
		}
		add := func(r AssetSearchResult, tags []string) {
			if view != "" && r.View != view {
				return
			}
			text := strings.ToLower(r.AssetID + " " + r.AssemblyID + " " + r.Name + " " + r.Description + " " + strings.Join(tags, " "))
			for _, term := range terms {
				if strings.Contains(text, term) {
					r.score += 2
				} else if packID == "" && strings.Contains(p.ID, term) {
					r.score++
				}
			}
			if len(terms) > 0 && r.score == 0 {
				return
			}
			r.PackID = p.ID
			r.Version = p.Version
			for _, a := range animations {
				if a.Entity == r.Entity && !slices.Contains(r.Actions, a.Action) {
					r.Actions = append(r.Actions, a.Action)
				}
			}
			for _, action := range []string{"idle", "move", "attack", "hurt", "death"} {
				if !slices.ContainsFunc(r.Actions, func(a string) bool { return a == action || action == "move" && (a == "walk" || a == "run") }) {
					r.MissingActions = append(r.MissingActions, action)
				}
			}
			out = append(out, r)
		}
		for _, a := range assets {
			result := AssetSearchResult{Entity: a.Entity, AssetID: a.ID, Name: a.Name, Description: a.Description, View: a.View, Direction: a.Direction, Fragment: a.AssemblyPart}
			if a.Model != nil {
				result.Kind = "model3d"
				result.Category = a.Model.Category
			}
			add(result, a.Tags)
		}
		for _, a := range assemblies {
			add(AssetSearchResult{Entity: a.ID, AssemblyID: a.ID, Name: a.Name, Description: a.Description, View: a.View, Direction: a.Direction}, nil)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Fragment != out[j].Fragment {
			return !out[i].Fragment
		}
		return out[i].score > out[j].score
	})
	seen := map[string]bool{}
	matches := []AssetSearchResult{}
	for _, result := range out {
		key := result.PackID + "/" + result.Entity
		if !seen[key] {
			matches = append(matches, result)
			seen[key] = true
		}
		if len(matches) == limit {
			break
		}
	}
	return matches, nil
}

func (s *Service) DescribeAsset(packID, assetID, assemblyID string) (AssetDetail, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	if !s.policy.Enabled {
		return AssetDetail{}, ErrDisabled
	}
	return s.describeAsset(packID, assetID, assemblyID)
}

func (s *Service) describeAsset(packID, assetID, assemblyID string) (AssetDetail, error) {
	if presentationPack(packID) {
		if assemblyID != "" {
			return AssetDetail{}, fmt.Errorf("presentation assets have no assemblies")
		}
		m, e := readPresentationPack(packID)
		if e != nil {
			return AssetDetail{}, e
		}
		a, e := presentationAsset(m, assetID)
		return AssetDetail{PackID: packID, Version: m.Version, Presentation: &a, Example: presentationExample(packID, assetID)}, e
	}
	if (assetID == "") == (assemblyID == "") {
		return AssetDetail{}, fmt.Errorf("provide exactly one asset_id or assembly_id")
	}
	p, assets, assemblies, animations, err := readPackUsage(packID)
	if err != nil {
		return AssetDetail{}, err
	}
	d := AssetDetail{PackID: p.ID, Version: p.Version, Animations: []PackAnimation{}, MissingActions: []string{}}
	if p.Kind == "model3d" {
		for _, a := range assets {
			if a.ID != assetID {
				continue
			}
			d.Model = a.Model
			d.View = "3d"
			d.Example = modelExample(p.Version, a.ID)
			for _, clip := range animations {
				if clip.AssetID == a.ID {
					d.Animations = append(d.Animations, clip)
				}
			}
			for _, action := range []string{"idle", "walk", "run", "attack", "hit", "death"} {
				if !slices.ContainsFunc(d.Animations, func(c PackAnimation) bool { return c.Action == action }) {
					d.MissingActions = append(d.MissingActions, action)
				}
			}
			return d, nil
		}
		return d, fmt.Errorf("3D asset not found in pack %s", p.ID)
	}
	ids := map[string]bool{}
	for _, a := range assets {
		if a.ID == assetID {
			a := a
			d.Asset = &a
			d.View = a.View
		}
	}
	for _, a := range assemblies {
		if a.ID == assemblyID {
			a := a
			d.Assembly = &a
			d.View = a.View
			for _, part := range a.Parts {
				ids[part.AssetID] = true
			}
		}
	}
	if d.Asset == nil && d.Assembly == nil {
		return d, fmt.Errorf("asset or assembly not found in pack %s", packID)
	}
	if d.Asset != nil {
		for _, a := range assets {
			if a.ID == d.Asset.ID || d.Asset.Entity != "" && a.Entity == d.Asset.Entity {
				ids[a.ID] = true
				d.Variants = append(d.Variants, a)
			}
		}
	}
	for _, a := range animations {
		if ids[a.AssetID] {
			d.Animations = append(d.Animations, a)
		}
	}
	for _, action := range []string{"idle", "move", "attack", "hurt", "death"} {
		if !slices.ContainsFunc(d.Animations, func(a PackAnimation) bool {
			return a.Action == action || action == "move" && (a.Action == "walk" || a.Action == "run")
		}) {
			d.MissingActions = append(d.MissingActions, action)
		}
	}
	factory := "createAsset"
	id := assetID
	if d.Assembly != nil {
		factory = "createAssembly"
		id = assemblyID
	}
	d.Example = fmt.Sprintf(`// Complete src/main.ts example. Merge these lifecycle methods into your game;
// retain the accepted rules and the installed common.ts lifecycle.
import { GameScene, start } from './common';
import { preloadPack, registerAnimations, %s, setFacing, playAction } from '../vendor/aurago-game-1.js';
import meta from '../assets/builtin/%s/%s/sheet.json';
class AssetGame extends GameScene {
  art: any;
  preload() { preloadPack(this, meta, 'assets/builtin/%s/%s/sheet.png'); }
  setup() {
    super.setup(); // Creates this.player, the dynamic collision/test object.
    this.player.setVisible(false);
    registerAnimations(this, meta);
    this.art = %s(this, meta, %q, this.player.x, this.player.y);
    const target = this.body(360,270,20,20,0xfacc15,true);
    this.physics.add.overlap(this.player,target,()=>{target.destroy();this.state.score++;this.state.hits++;});
  }
  step(delta: number) {
    super.step(delta);
    this.art.setPosition(this.player.x,this.player.y);
    // setFacing(this.art, dx, dy) respects the listed transform rules.
    // playAction(this.art, action) accepts only the listed actions.
  }
}
start(AssetGame);
`, factory, p.ID, p.Version, p.ID, p.Version, factory, id)
	return d, nil
}

func (d AssetDetail) allowsDirection(direction string) bool {
	if direction == "none" {
		return true
	}
	if d.Assembly != nil {
		return direction == d.Assembly.Direction || d.Assembly.Transform.Mode == "rotate" || d.Assembly.Transform.FlipX && (direction == "left" || direction == "right")
	}
	a := d.Asset
	if a == nil {
		return false
	}
	if direction == a.Direction || a.Transform.Mode == "rotate" {
		return true
	}
	if a.Transform.FlipX && (direction == "left" || direction == "right") {
		return true
	}
	return a.Transform.Mode == "directional" && slices.ContainsFunc(d.Variants, func(v PackAsset) bool { return v.Direction == direction })
}
