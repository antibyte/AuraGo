package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const EffectsPackID = "aurago-effects"
const SoundsPackID = "aurago-sounds"
const PresentationVersion = "1.0.0"

const PresentationGuide = `Presentation: search_assets(asset_kind="effect" or "audio") and describe_asset return exact local presets. In set_design add optional presentation:{environment:"forest-rain",effects:["blood-spray","blood-pool"],sounds:[{event:"step",sound:"step-grass"},{event:"shot",sound:"rifle"},{event:"hit",sound:"impact-flesh"}],quality:"auto"}. The server resolves versions/dependencies and imports after acceptance. Guided templates already own weather, a single mixer and loop update/disposal. Do not read vendor code or implement another animation/audio loop.
Keep src/presentation.json connected to createPresentation and keep feedback/update/dispose hooks in common.ts. Never set the config/controller to null or remove requested effects/sounds to fix an unrelated gameplay error; repair only the reported fault. A playable game with imported but disconnected presentation assets fails the accepted plan.
Complete combinations: forest FPS uses forest-rain, blood-spray/blood-pool/muzzle-flash, step-grass/rifle/impact-flesh; rainy 2D adventure uses forest-rain, water-ripple/pickup-glow, step-water/pickup; coastal flight uses coast, engine-trail/water-splash, engine/pickup; space uses space, explosion/metal-sparks, laser/impact-metal/explosion-small. Bind sounds to known events (step,jump,land,shot,reload,hit,pickup,win,lose,splash,interact,engine,ui,ambient).
Custom integration: import config from './presentation.json'; import {createPresentation,createThreeAdapter} from '../vendor/aurago-effects-3d-1.js'; const presentation=createPresentation({config,root,adapter:createThreeAdapter({scene,camera,renderer,sun,ambient})}); registerSurface(ground,{kind:'ground'}), registerSurface(roof,{kind:'roof'}); call presentation.update(dt) and presentation.render() in the existing loop instead of renderer.render; event('hit',hit.point.toArray(),worldNormal.toArray(),'metal' or 'stone' or 'flesh') only on actual contact. For Phaser import createPhaserAdapter from aurago-effects-2d-1.js and pass {scene:this,view:'top' or 'side'}; call update(dt) in inherited GameScene lifecycle; no explicit render. In guided 2D use this.feedback(event,object,material). Controller: set(id,parameters), emit(id,{position:[x,y,z],normal:[0,1,0]}), applyObject(object,'hologram'|'dissolve'|'hit-flash'), setEnvironment(importedAtmosphereID), setPaused(bool), setActive(bool), reset(), dispose(). Never invent parameters; use describe_asset defaults. Audio unlocks on real interaction; presentation.audio.play(id,{position:[x,y,z]}), setRoom('outside'|'small-room'|'hall'), setVolume(0..1), setMuted(bool). Imported generated music can connect to audio.context via audio.connectMusic(node). Preview and ZIP use identical local files. Preserve working old games without presentation; when adding it to an old game integrate these lifecycle hooks explicitly.`

// Presentation contains semantic choices only. The service resolves local files.
type Presentation struct {
	Environment string         `json:"environment,omitempty"`
	Effects     []string       `json:"effects,omitempty"`
	Sounds      []SoundBinding `json:"sounds,omitempty"`
	Quality     string         `json:"quality,omitempty"`
	Version     string         `json:"version,omitempty"`
}
type SoundBinding struct {
	Event string `json:"event"`
	Sound string `json:"sound"`
}
type PresentationAsset struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	Dimensions  []string        `json:"dimensions"`
	Defaults    map[string]any  `json:"defaults,omitempty"`
	Effects     []string        `json:"effects,omitempty"`
	Sounds      []string        `json:"sounds,omitempty"`
	Loop        bool            `json:"loop,omitempty"`
	Duration    float64         `json:"duration,omitempty"`
	Gain        float64         `json:"gain,omitempty"`
	Files       []ModelFile     `json:"files"`
	Provenance  json.RawMessage `json:"provenance,omitempty"`
}
type PresentationManifest struct {
	AssetPackSummary
	SchemaVersion int                 `json:"schema_version"`
	Assets        []PresentationAsset `json:"assets"`
}

func presentationPack(id string) bool { return id == EffectsPackID || id == SoundsPackID }
func readPresentationPack(id string) (PresentationManifest, error) {
	var m PresentationManifest
	if !presentationPack(id) {
		return m, ErrNotFound
	}
	data, err := assetPackFS.ReadFile("asset_packs/" + id + "/manifest.json")
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("decode presentation catalog: %w", err)
	}
	if m.ID != id || m.Version != PresentationVersion {
		return m, fmt.Errorf("invalid presentation catalog version")
	}
	return m, nil
}
func presentationAsset(m PresentationManifest, id string) (PresentationAsset, error) {
	for _, a := range m.Assets {
		if a.ID == id {
			return a, nil
		}
	}
	return PresentationAsset{}, fmt.Errorf("unknown %s asset %q; use search_assets", m.Kind, id)
}
func bundledPresentationFile(id, filename string) ([]byte, error) {
	m, err := readPresentationPack(id)
	if err != nil {
		return nil, err
	}
	allowed := filename == "manifest.json" || filename == "LICENSE.txt"
	for _, a := range m.Assets {
		for _, f := range a.Files {
			allowed = allowed || f.File == filename
		}
	}
	if !allowed {
		return nil, ErrNotFound
	}
	return assetPackFS.ReadFile("asset_packs/" + id + "/" + filename)
}
func presentationExample(id, asset string) string {
	if id == SoundsPackID {
		return fmt.Sprintf(`// In set_design.presentation.sounds: [{"event":"hit","sound":%q}].
// Guided templates load and play this on real hits. Custom games: presentation.audio.play(%q,{position:[x,y,z]}).
// Use one presentation controller per game and its existing update/dispose hooks.`, asset, asset)
	}
	if m, err := readPresentationPack(id); err == nil {
		if a, err := presentationAsset(m, asset); err == nil {
			if a.Category == "environment" {
				return fmt.Sprintf(`// set_design: presentation:{environment:%q,quality:"auto"}. The server imports its effects and sounds.
// To switch to this imported atmosphere in an existing controller:
presentation.setEnvironment(%q);`, asset, asset)
			}
			if slices.Contains([]string{"hologram", "dissolve", "hit-flash"}, asset) {
				return fmt.Sprintf(`const release = presentation.applyObject(object, %q); // release() restores the original material.`, asset)
			}
			if a.Category == "impact" || a.Category == "particles" || asset == "water-splash" || asset == "water-ripple" {
				return fmt.Sprintf(`presentation.emit(%q, {position:[x,y,z],normal:[0,1,0]}); // Actual world contact. For continuous fire/smoke/embers/engine-trail use set(id,{position:[x,y,z]}).`, asset)
			}
			defaults, _ := json.Marshal(a.Defaults)
			return fmt.Sprintf(`presentation.set(%q, %s); // Imported preset; update(dt) remains in the existing game loop.`, asset, defaults)
		}
	}
	return "Use describe_asset with an exact catalog ID."
}
func searchPresentation(query, pack, kind, view string, limit int) ([]AssetSearchResult, error) {
	if kind != "effect" && kind != "audio" {
		return nil, fmt.Errorf("asset_kind must be effect or audio")
	}
	if pack == "" {
		if kind == "effect" {
			pack = EffectsPackID
		} else {
			pack = SoundsPackID
		}
	}
	m, err := readPresentationPack(pack)
	if err != nil {
		return nil, err
	}
	if m.Kind != kind {
		return nil, fmt.Errorf("asset_kind does not match pack")
	}
	out := []AssetSearchResult{}
	terms := strings.Fields(strings.ToLower(query))
	for _, a := range m.Assets {
		score := 0
		text := strings.ToLower(a.ID + " " + a.Name + " " + a.Description + " " + strings.Join(a.Tags, " "))
		for _, term := range terms {
			if strings.Contains(text, term) {
				score++
			}
		}
		if len(terms) > 0 && score == 0 {
			continue
		}
		out = append(out, AssetSearchResult{Kind: m.Kind, Category: a.Category, Entity: a.ID, AssetID: a.ID, Name: a.Name, Description: a.Description, PackID: m.ID, Version: m.Version, View: view, score: score})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	return out[:min(limit, len(out))], nil
}
func resolvePresentation(p *Presentation) ([]PresentationAsset, []PresentationAsset, error) {
	if p == nil {
		return nil, nil, nil
	}
	if p.Version != "" && p.Version != PresentationVersion {
		return nil, nil, fmt.Errorf("presentation.version: unsupported version")
	}
	if p.Quality != "" && !slices.Contains([]string{"auto", "low", "medium", "high"}, p.Quality) {
		return nil, nil, fmt.Errorf("presentation.quality: auto, low, medium or high")
	}
	if len(p.Effects) > 32 || len(p.Sounds) > 40 {
		return nil, nil, fmt.Errorf("presentation: at most 32 effects and 40 sound bindings")
	}
	em, err := readPresentationPack(EffectsPackID)
	if err != nil {
		return nil, nil, err
	}
	sm, err := readPresentationPack(SoundsPackID)
	if err != nil {
		return nil, nil, err
	}
	effects := []PresentationAsset{}
	sounds := []PresentationAsset{}
	seenE := map[string]bool{}
	seenS := map[string]bool{}
	addSound := func(id string) error {
		a, e := presentationAsset(sm, id)
		if e == nil && !seenS[id] {
			seenS[id] = true
			sounds = append(sounds, a)
		}
		return e
	}
	ids := append([]string{}, p.Effects...)
	if p.Environment != "" {
		a, e := presentationAsset(em, p.Environment)
		if e != nil {
			return nil, nil, e
		}
		if a.Category != "environment" {
			return nil, nil, fmt.Errorf("presentation.environment: choose an atmosphere")
		}
		effects = append(effects, a)
		seenE[a.ID] = true
		ids = append(a.Effects, ids...)
		for _, id := range a.Sounds {
			if e := addSound(id); e != nil {
				return nil, nil, e
			}
		}
	}
	for _, id := range ids {
		a, e := presentationAsset(em, id)
		if e != nil {
			return nil, nil, e
		}
		if a.Category == "environment" {
			return nil, nil, fmt.Errorf("presentation.effects: put atmosphere in environment")
		}
		if !seenE[id] {
			seenE[id] = true
			effects = append(effects, a)
		}
	}
	events := map[string]bool{}
	for _, b := range p.Sounds {
		if !slices.Contains([]string{"step", "jump", "land", "shot", "reload", "hit", "pickup", "win", "lose", "splash", "interact", "engine", "ui", "ambient"}, b.Event) || events[b.Event+":"+b.Sound] {
			return nil, nil, fmt.Errorf("presentation.sounds: use unique supported event names")
		}
		events[b.Event+":"+b.Sound] = true
		if e := addSound(b.Sound); e != nil {
			return nil, nil, e
		}
	}
	return effects, sounds, nil
}
func presentationConfig(p *Presentation) (string, error) {
	effects, sounds, err := resolvePresentation(p)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "null", nil
	}
	config := map[string]any{"environment": p.Environment, "effects": effects, "sounds": sounds, "bindings": p.Sounds, "quality": p.Quality, "version": PresentationVersion, "base": "assets/builtin/" + SoundsPackID + "/" + PresentationVersion + "/"}
	b, err := json.Marshal(config)
	return string(b), err
}

// Check the compiled dependency graph, not comments mentioning unused assets.
// Runtime/visual checks still own effect timing and actual event delivery.
func checkPresentationBuild(dir, dimension, metafile string) error {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(gamePlanPath)))
	if os.IsNotExist(err) {
		return nil // Legacy projects and exported games need no presentation plan.
	}
	var plan GamePlan
	if err != nil {
		return fmt.Errorf("read presentation plan: %w", err)
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		return fmt.Errorf("decode presentation plan: %w", err)
	}
	if plan.Presentation == nil {
		return nil
	}
	effects, sounds, err := resolvePresentation(plan.Presentation)
	if err != nil {
		return err
	}
	if len(effects)+len(sounds) == 0 {
		return nil
	}
	var config struct {
		Environment     string
		Effects, Sounds []PresentationAsset
		Bindings        []SoundBinding
	}
	data, err = os.ReadFile(filepath.Join(dir, "src", "presentation.json"))
	if err != nil || json.Unmarshal(data, &config) != nil || config.Environment != plan.Presentation.Environment {
		return fmt.Errorf("Restore src/presentation.json with the accepted presentation plan; do not disable the requested atmosphere, effects or sounds")
	}
	for _, group := range []struct{ want, got []PresentationAsset }{{effects, config.Effects}, {sounds, config.Sounds}} {
		for _, asset := range group.want {
			if !slices.ContainsFunc(group.got, func(a PresentationAsset) bool { return a.ID == asset.ID }) {
				return fmt.Errorf("Restore planned presentation asset %q in src/presentation.json; do not remove requested effects or sounds to fix gameplay", asset.ID)
			}
		}
	}
	for _, binding := range plan.Presentation.Sounds {
		if !slices.Contains(config.Bindings, binding) {
			return fmt.Errorf("Restore planned sound binding %s -> %s", binding.Event, binding.Sound)
		}
	}
	var meta struct {
		Inputs  map[string]json.RawMessage
		Outputs map[string]struct{ Imports []struct{ Path string } }
	}
	if err := json.Unmarshal([]byte(metafile), &meta); err != nil {
		return fmt.Errorf("decode presentation build graph: %w", err)
	}
	helper := "../vendor/aurago-effects-" + dimension + "-1.js"
	for _, output := range meta.Outputs {
		for _, dependency := range output.Imports {
			if dependency.Path == helper && meta.Inputs["src/presentation.json"] != nil {
				return nil
			}
		}
	}
	return fmt.Errorf("Planned presentation is disconnected: import src/presentation.json and %s, retain createPresentation, feedback and update/dispose hooks in common.ts; imported files alone do not enable effects or sounds", helper)
}

// Import both packs in one existing file/ledger transaction, including atmosphere dependencies.
func (s *Service) importPresentation(ctx context.Context, jobID, id string, ids []string) (ImportedAssetPack, error) {
	if len(ids) < 1 || len(ids) > 64 {
		return ImportedAssetPack{}, fmt.Errorf("presentation import requires 1–64 exact asset_ids")
	}
	m, err := readPresentationPack(id)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	effects, sounds := []PresentationAsset{}, []PresentationAsset{}
	for _, aid := range ids {
		a, err := presentationAsset(m, aid)
		if err != nil {
			return ImportedAssetPack{}, err
		}
		if id == SoundsPackID {
			sounds = append(sounds, a)
			continue
		}
		p := &Presentation{Effects: []string{aid}}
		if a.Category == "environment" {
			p.Effects = nil
			p.Environment = aid
		}
		e, v, err := resolvePresentation(p)
		if err != nil {
			return ImportedAssetPack{}, err
		}
		effects = append(effects, e...)
		sounds = append(sounds, v...)
	}
	return s.publishPresentation(ctx, jobID, id, effects, sounds)
}
func (s *Service) publishPresentation(ctx context.Context, jobID, id string, effects, sounds []PresentationAsset) (ImportedAssetPack, error) {
	result := ImportedAssetPack{ID: id, Kind: "effect", Version: PresentationVersion, Manifests: map[string]string{}}
	if id == SoundsPackID {
		result.Kind = "audio"
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return result, err
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	wanted := map[string][]byte{}
	for _, group := range []struct {
		id     string
		assets []PresentationAsset
	}{{EffectsPackID, effects}, {SoundsPackID, sounds}} {
		if len(group.assets) == 0 {
			continue
		}
		m, err := readPresentationPack(group.id)
		if err != nil {
			return result, err
		}
		base := group.id + "/" + PresentationVersion + "/"
		wanted[base+"LICENSE.txt"], err = bundledPresentationFile(group.id, "LICENSE.txt")
		if err != nil {
			return result, err
		}
		for _, a := range group.assets {
			name := base + "assets/" + a.ID + ".json"
			if _, ok := wanted[name]; ok {
				continue
			}
			for _, f := range a.Files {
				data, err := bundledPresentationFile(group.id, f.File)
				if err != nil {
					return result, err
				}
				if int64(len(data)) != f.Bytes || sha256Bytes(data) != f.SHA256 {
					return result, fmt.Errorf("presentation integrity mismatch: %s", f.File)
				}
				if int64(len(data)) > s.opts.MaxAssetBytes {
					return result, fmt.Errorf("presentation asset exceeds configured limit")
				}
				wanted[base+f.File] = data
			}
			one := m
			one.Assets = []PresentationAsset{a}
			data, err := json.Marshal(one)
			if err != nil {
				return result, err
			}
			if int64(len(data)) > s.opts.MaxFileBytes {
				return result, fmt.Errorf("presentation metadata exceeds configured limit")
			}
			wanted[name] = data
			key := a.ID
			if group.id != id {
				key = group.id + ":" + a.ID
			}
			result.AssetIDs = append(result.AssetIDs, key)
			result.Manifests[key] = "assets/builtin/" + name
		}
	}
	if len(result.AssetIDs) == 0 {
		return result, nil
	}
	result.Metadata = result.Manifests[result.AssetIDs[0]]
	result.Example = presentationExample(id, result.AssetIDs[0])
	return s.publishAssetSelection(ctx, jobID, stage, "assets/builtin", wanted, result)
}
func (s *Service) importPlannedPresentation(ctx context.Context, jobID string, p *Presentation) ([]ImportedAssetPack, error) {
	effects, sounds, err := resolvePresentation(p)
	if err != nil {
		return nil, err
	}
	if len(effects)+len(sounds) == 0 {
		return nil, nil
	}
	id := EffectsPackID
	if len(effects) == 0 {
		id = SoundsPackID
	}
	result, err := s.publishPresentation(ctx, jobID, id, effects, sounds)
	if err != nil {
		return nil, err
	}
	return []ImportedAssetPack{result}, nil
}
