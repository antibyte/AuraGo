package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type AtlasPage struct {
	ModelFile
	Width  int `json:"width"`
	Height int `json:"height"`
}
type AtlasFrame struct {
	ID     int    `json:"id"`
	Atlas  string `json:"atlas"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type SpriteDirection struct {
	ID      string  `json:"id"`
	Radians float64 `json:"radians"`
}
type SpriteEvent struct {
	Frame int    `json:"frame"`
	Name  string `json:"name"`
}
type SpriteLayer struct {
	ID          string         `json:"id"`
	Frames      map[string]int `json:"frames"`
	DepthOffset float64        `json:"depth_offset"`
}
type SpriteSocket struct {
	ID         string           `json:"id"`
	Pose       string           `json:"pose"`
	Directions map[string]Point `json:"directions"`
}
type AtlasManifest struct {
	AssetPackSummary
	SchemaVersion int             `json:"schema_version"`
	License       string          `json:"license"`
	Atlases       []AtlasPage     `json:"atlases"`
	Frames        []AtlasFrame    `json:"frames"`
	Assets        []PackAsset     `json:"assets"`
	Animations    []PackAnimation `json:"animations"`
	Categories    json.RawMessage `json:"categories,omitempty"`
}

func atlasPack(id string) bool {
	if catalogPackKind(id) != "sprite2d" {
		return false
	}
	data, err := assetPackFS.ReadFile("asset_packs/" + id + "/manifest.json")
	var head struct {
		SchemaVersion int `json:"schema_version"`
	}
	return err == nil && json.Unmarshal(data, &head) == nil && head.SchemaVersion == 2
}

func readAtlasManifest(id string) (AtlasManifest, error) {
	var m AtlasManifest
	if !atlasPack(id) {
		return m, ErrNotFound
	}
	data, err := assetPackFS.ReadFile("asset_packs/" + id + "/manifest.json")
	if err != nil {
		return m, err
	}
	if err = json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if m.ID != id || m.Kind != "sprite2d" || !safeModelComponent(m.Version) || m.License != "MIT" {
		return m, fmt.Errorf("invalid atlas identity")
	}
	pages := map[string]AtlasPage{}
	for _, p := range m.Atlases {
		clean, e := safeRelativePath(p.File, false)
		if e != nil || clean != p.File || !strings.HasSuffix(p.File, ".png") || p.Width < 1 || p.Height < 1 || p.Width > 2048 || p.Height > 2048 || p.Bytes < 1 || len(p.SHA256) != 64 {
			return m, fmt.Errorf("invalid atlas page %q", p.File)
		}
		if _, exists := pages[p.File]; exists {
			return m, fmt.Errorf("duplicate atlas page")
		}
		pages[p.File] = p
	}
	frames := map[int]AtlasFrame{}
	for _, f := range m.Frames {
		p, ok := pages[f.Atlas]
		if _, exists := frames[f.ID]; exists || !ok || f.ID < 0 || f.X < 0 || f.Y < 0 || f.Width < 1 || f.Height < 1 || f.X+f.Width > p.Width || f.Y+f.Height > p.Height {
			return m, fmt.Errorf("invalid atlas frame %d", f.ID)
		}
		frames[f.ID] = f
	}
	assets := map[string]PackAsset{}
	for _, a := range m.Assets {
		if a.PreviewFile != nil && (a.Preview != a.PreviewFile.File || !strings.HasPrefix(a.Preview, "previews/") || !strings.HasSuffix(a.Preview, ".png") || strings.Contains(a.Preview, "..") || len(a.PreviewFile.SHA256) != 64) {
			return m, fmt.Errorf("invalid atlas preview")
		}
		if !safeModelComponent(a.ID) || assets[a.ID].ID != "" || !validOrigin(a.Origin) || len(a.Frames) == 0 || a.Width < 1 || a.Height < 1 || a.Width > 2048 || a.Height > 2048 {
			return m, fmt.Errorf("invalid atlas asset %q", a.ID)
		}
		assets[a.ID] = a
		for _, id := range a.Frames {
			if _, ok := frames[id]; !ok {
				return m, fmt.Errorf("missing asset frame")
			}
		}
		directions := map[string]bool{}
		for _, d := range a.Directions {
			if !safeModelComponent(d.ID) || !finite(d.Radians) || directions[d.ID] {
				return m, fmt.Errorf("invalid sprite direction")
			}
			directions[d.ID] = true
		}
		if !directions[a.Direction] {
			return m, fmt.Errorf("missing default direction for %s", a.ID)
		}
		sockets := map[string]bool{}
		for _, socket := range a.Sockets {
			if !safeModelComponent(socket.ID) || sockets[socket.ID] || socket.Pose != "rest" || len(socket.Directions) != len(directions) {
				return m, fmt.Errorf("invalid sprite socket")
			}
			sockets[socket.ID] = true
			for dir, point := range socket.Directions {
				if !directions[dir] || !finite(point.X) || !finite(point.Y) {
					return m, fmt.Errorf("invalid sprite socket position")
				}
			}
		}
		layers := map[string]bool{}
		for _, layer := range a.Layers {
			if !safeModelComponent(layer.ID) || layers[layer.ID] || !finite(layer.DepthOffset) || len(layer.Frames) != len(directions) {
				return m, fmt.Errorf("invalid sprite layer")
			}
			layers[layer.ID] = true
			for dir, frame := range layer.Frames {
				f, ok := frames[frame]
				if !ok || !directions[dir] || f.Width != a.Width || f.Height != a.Height {
					return m, fmt.Errorf("invalid layer frame")
				}
			}
		}
	}
	clips := map[string]bool{}
	for _, a := range m.Animations {
		owner := assets[a.AssetID]
		if clips[a.ID] || owner.ID == "" || !safeModelComponent(a.ID) || !safeModelComponent(a.Action) || !slices.ContainsFunc(owner.Directions, func(d SpriteDirection) bool { return d.ID == a.Direction }) || len(a.Frames) == 0 || a.FrameRate < 1 || a.FrameRate > 60 || a.Repeat < -1 {
			return m, fmt.Errorf("invalid atlas animation %q", a.ID)
		}
		clips[a.ID] = true
		for _, id := range a.Frames {
			if _, ok := frames[id]; !ok {
				return m, fmt.Errorf("missing animation frame")
			}
			if f := frames[id]; f.Width != owner.Width || f.Height != owner.Height {
				return m, fmt.Errorf("animation frame size mismatch")
			}
		}
		for _, event := range a.Events {
			if event.Frame < 0 || event.Frame >= len(a.Frames) || !safeModelComponent(event.Name) {
				return m, fmt.Errorf("invalid animation event")
			}
		}
	}
	return m, nil
}

func bundledAtlasFile(id, filename string) ([]byte, error) {
	m, err := readAtlasManifest(id)
	if err != nil {
		return nil, err
	}
	allowed := filename == "manifest.json" || filename == "LICENSE.txt"
	for _, page := range m.Atlases {
		allowed = allowed || page.File == filename
	}
	for _, asset := range m.Assets {
		allowed = allowed || asset.PreviewFile != nil && asset.PreviewFile.File == filename
	}
	if !allowed {
		return nil, ErrNotFound
	}
	data, err := assetPackFS.ReadFile("asset_packs/" + id + "/" + filename)
	if err != nil {
		return nil, ErrNotFound
	}
	return data, nil
}

// Keep only selected motifs and the exact pages needed by their animation frames.
func selectAtlasAssets(m AtlasManifest, ids []string) (AtlasManifest, error) {
	if len(ids) < 1 || len(ids) > 64 {
		return m, fmt.Errorf("atlas import requires 1–64 explicit asset_ids")
	}
	selected := m
	selected.Assets = nil
	selected.Animations = nil
	selected.Frames = nil
	selected.Atlases = nil
	selected.Categories = nil
	needed, frameIDs, pageIDs := map[string]bool{}, map[int]bool{}, map[string]bool{}
	for _, id := range ids {
		if needed[id] {
			continue
		}
		needed[id] = true
		index := slices.IndexFunc(m.Assets, func(a PackAsset) bool { return a.ID == id })
		if index < 0 {
			return selected, fmt.Errorf("unknown atlas asset %q", id)
		}
		a := m.Assets[index]
		selected.Assets = append(selected.Assets, a)
		for _, frame := range a.Frames {
			frameIDs[frame] = true
		}
		for _, layer := range a.Layers {
			for _, frame := range layer.Frames {
				frameIDs[frame] = true
			}
		}
	}
	for _, a := range m.Animations {
		if needed[a.AssetID] {
			selected.Animations = append(selected.Animations, a)
			for _, f := range a.Frames {
				frameIDs[f] = true
			}
		}
	}
	for _, f := range m.Frames {
		if frameIDs[f.ID] {
			selected.Frames = append(selected.Frames, f)
			pageIDs[f.Atlas] = true
		}
	}
	for _, page := range m.Atlases {
		if pageIDs[page.File] {
			selected.Atlases = append(selected.Atlases, page)
		}
	}
	return selected, nil
}

func atlasExample(pack, version, id string) string {
	return fmt.Sprintf(`// A complete scene; use its hooks in your existing game, never add a second loop.
import {preloadPack, registerAnimations, createAsset, setFacing, playAction} from '../vendor/aurago-game-1.js';
import meta from '../assets/builtin/%s/%s/assets/%s.json';
class AssetDemo extends Phaser.Scene {
  preload() { preloadPack(this, meta, 'assets/builtin/%s/%s/'); }
  create() {
    registerAnimations(this, meta);
    this.art = createAsset(this, meta, %q, 320, 270);
    setFacing(this.art, 1, 0);
    const clip = meta.animations.find(c => c.asset_id === meta.assets[0].id);
    if (clip) playAction(this.art, clip.action);
  }
}
// Pass AssetDemo to your single Phaser.Game scene list. Scene shutdown owns art.
// art.on('asset-event', ({name}) => ...): animation cues are not proof of a hit.
// GameScene.body(..., plannedRole) manages its own art; do not add a second sprite.
`, pack, version, id, pack, version, id)
}

func (s *Service) importAtlas(ctx context.Context, jobID, id string, ids []string) (ImportedAssetPack, error) {
	project, _, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	if project.Dimension != "2d" {
		return ImportedAssetPack{}, fmt.Errorf("sprite atlases require a 2D project")
	}
	m, err := readAtlasManifest(id)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	selection, err := selectAtlasAssets(m, ids)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	rel := "assets/builtin/" + id + "/" + m.Version
	result := ImportedAssetPack{ID: id, Kind: "sprite2d", Version: m.Version, Manifests: map[string]string{}}
	wanted := map[string][]byte{}
	for _, p := range selection.Atlases {
		data, e := bundledAtlasFile(id, p.File)
		if e != nil {
			return result, e
		}
		if int64(len(data)) != p.Bytes || sha256Bytes(data) != p.SHA256 {
			return result, fmt.Errorf("atlas integrity mismatch: %s", p.File)
		}
		if int64(len(data)) > s.opts.MaxAssetBytes {
			return result, fmt.Errorf("atlas exceeds configured asset limit")
		}
		wanted[p.File] = data
	}
	license, err := bundledAtlasFile(id, "LICENSE.txt")
	if err != nil {
		return result, err
	}
	wanted["LICENSE.txt"] = license
	for _, a := range selection.Assets {
		if a.PreviewFile != nil {
			data, e := bundledAtlasFile(id, a.PreviewFile.File)
			if e != nil {
				return result, e
			}
			if int64(len(data)) != a.PreviewFile.Bytes || sha256Bytes(data) != a.PreviewFile.SHA256 {
				return result, fmt.Errorf("preview integrity mismatch: %s", a.ID)
			}
			wanted[a.PreviewFile.File] = data
		}
		single, e := selectAtlasAssets(m, []string{a.ID})
		if e != nil {
			return result, e
		}
		data, e := json.Marshal(single)
		if e != nil {
			return result, e
		}
		if int64(len(data)) > s.opts.MaxFileBytes {
			return result, fmt.Errorf("atlas metadata exceeds configured file limit: %s", a.ID)
		}
		name := "assets/" + a.ID + ".json"
		wanted[name] = data
		result.AssetIDs = append(result.AssetIDs, a.ID)
		result.Manifests[a.ID] = rel + "/" + name
	}
	result.Metadata = result.Manifests[result.AssetIDs[0]]
	result.PhaserExample = atlasExample(id, m.Version, result.AssetIDs[0])
	return s.publishAssetSelection(ctx, jobID, stage, rel, wanted, result)
}
