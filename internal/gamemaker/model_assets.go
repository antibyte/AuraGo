package gamemaker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const ModelPackID = "aurago-low-poly"

type ModelFile struct {
	File      string `json:"file"`
	Bytes     int64  `json:"bytes"`
	SHA256    string `json:"sha256"`
	Triangles int    `json:"triangles,omitempty"`
	Level     int    `json:"level"`
}

type ModelAnimation struct {
	ID       string                `json:"id"`
	Duration float64               `json:"duration"`
	Loop     bool                  `json:"loop"`
	Speed    float64               `json:"speed"`
	Events   []ModelAnimationEvent `json:"events,omitempty"`
}

type ModelAnimationEvent struct {
	Time float64 `json:"time"`
	Name string  `json:"name"`
}

type ModelAsset struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	Tags        []string        `json:"tags"`
	Kind        string          `json:"kind"`
	View        string          `json:"view"`
	Entity      string          `json:"entity"`
	Forward     string          `json:"forward"`
	Up          string          `json:"up"`
	Units       string          `json:"units"`
	Origin      string          `json:"origin"`
	Views       []string        `json:"views,omitempty"`
	FPSBinding  json.RawMessage `json:"fps_binding,omitempty"`
	PreviewFile *ModelFile      `json:"preview_file,omitempty"`
	Connections json.RawMessage `json:"connections,omitempty"`
	Bounds      struct {
		Min [3]float64 `json:"min"`
		Max [3]float64 `json:"max"`
	} `json:"bounds"`
	Collider json.RawMessage `json:"collider"`
	Sockets  []struct {
		ID   string `json:"id"`
		Node string `json:"node"`
	} `json:"sockets"`
	MovingParts []struct {
		Node string     `json:"node"`
		Kind string     `json:"kind"`
		Axis [3]float64 `json:"axis"`
	} `json:"moving_parts"`
	Rig              string           `json:"rig"`
	LODs             []ModelFile      `json:"lods"`
	AnimationLibrary *ModelFile       `json:"animation_library,omitempty"`
	Animations       []ModelAnimation `json:"animations"`
	Preview          string           `json:"preview"`
}

type ModelManifest struct {
	AssetPackSummary
	SchemaVersion int             `json:"schema_version"`
	License       string          `json:"license"`
	Categories    json.RawMessage `json:"categories,omitempty"`
	Assets        []ModelAsset    `json:"assets"`
}

func readModelManifest() (ModelManifest, error) {
	var manifest ModelManifest
	data, err := assetPackFS.ReadFile("asset_packs/" + ModelPackID + "/manifest.json")
	if err != nil {
		return manifest, fmt.Errorf("read 3D catalog: %w", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("decode 3D catalog: %w", err)
	}
	if manifest.ID != ModelPackID || manifest.Kind != "model3d" || !safeModelComponent(manifest.Version) {
		return manifest, fmt.Errorf("invalid 3D catalog identity")
	}
	return manifest, nil
}

func safeModelComponent(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, "/\\\x00")
}

func modelFiles(asset ModelAsset) []ModelFile {
	files := append([]ModelFile(nil), asset.LODs...)
	if asset.AnimationLibrary != nil {
		files = append(files, *asset.AnimationLibrary)
	}
	return files
}

func readModelUsage() (AssetPack, []PackAsset, []PackAssembly, []PackAnimation, error) {
	m, err := readModelManifest()
	if err != nil {
		return AssetPack{}, nil, nil, nil, err
	}
	p := AssetPack{AssetPackSummary: m.AssetPackSummary, SchemaVersion: m.SchemaVersion, Categories: m.Categories}
	var assets []PackAsset
	var animations []PackAnimation
	for _, model := range m.Assets {
		model := model
		assets = append(assets, PackAsset{ID: model.ID, Name: model.Name, Description: model.Description, Tags: model.Tags, View: "3d", Direction: "none", Entity: model.ID, Model: &model})
		for _, clip := range model.Animations {
			animations = append(animations, PackAnimation{ID: clip.ID, AssetID: model.ID, Entity: model.ID, Action: clip.ID, Direction: "none"})
		}
	}
	return p, assets, nil, animations, nil
}

// The manifest is the allowlist; no arbitrary directory/file serving is allowed.
func bundledModelFile(filename string) ([]byte, error) {
	clean, err := safeRelativePath(filename, false)
	if err != nil || clean != filename {
		return nil, ErrNotFound
	}
	manifest, err := readModelManifest()
	if err != nil {
		return nil, err
	}
	allowed := filename == "manifest.json" || filename == "LICENSE.txt"
	for _, asset := range manifest.Assets {
		allowed = allowed || filename == asset.Preview
		for _, file := range modelFiles(asset) {
			allowed = allowed || filename == file.File
		}
	}
	if !allowed {
		return nil, ErrNotFound
	}
	data, err := assetPackFS.ReadFile("asset_packs/" + ModelPackID + "/" + filename)
	if err != nil {
		return nil, ErrNotFound
	}
	return data, nil
}

func validateModelSelection(ids []string) ([]ModelAsset, ModelManifest, error) {
	manifest, err := readModelManifest()
	if err != nil {
		return nil, manifest, err
	}
	if len(ids) < 1 || len(ids) > 64 {
		return nil, manifest, fmt.Errorf("model3d import requires 1–64 explicit asset_ids")
	}
	selected := []ModelAsset{}
	seen := map[string]bool{}
	for _, id := range ids {
		i := slices.IndexFunc(manifest.Assets, func(a ModelAsset) bool { return a.ID == id })
		if i < 0 || !safeModelComponent(id) {
			return nil, manifest, fmt.Errorf("unknown 3D asset %q", id)
		}
		if !seen[id] {
			selected = append(selected, manifest.Assets[i])
			seen[id] = true
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	return selected, manifest, nil
}

func modelExample(version, id string) string {
	return fmt.Sprintf(`// Merge into the existing Three.js game and its single update/dispose lifecycle.
import { loadAsset, createInstance, playAction, updateInstance, disposeInstance, releaseAsset } from '../vendor/aurago-three-assets-1.js';
import meta from '../assets/builtin/%s/%s/assets/%s.json';
const asset = await loadAsset(meta, %q, 'assets/builtin/%s/%s/');
const instance = createInstance(asset);
scene.add(instance.root); // metres, +Y up, +Z forward; use documented bounds/collider.
// playAction(instance, 'idle'); // Only use actions listed by describe_asset.
// In the existing game loop: updateInstance(instance, deltaSeconds, camera);
// On teardown: disposeInstance(instance); releaseAsset(asset);
`, ModelPackID, version, id, id, ModelPackID, version)
}

// Add a verified selection under the existing build lock. Each immutable file is
// published once; a failed transaction removes only files added by this import.
func (s *Service) importModels(ctx context.Context, jobID string, ids []string) (ImportedAssetPack, error) {
	project, _, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	if project.Dimension != "3d" {
		return ImportedAssetPack{}, fmt.Errorf("3D models require a 3D project")
	}
	selected, manifest, err := validateModelSelection(ids)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	rel := "assets/builtin/" + ModelPackID + "/" + manifest.Version
	result := ImportedAssetPack{ID: ModelPackID, Version: manifest.Version, Kind: "model3d", Manifests: map[string]string{}}
	wanted := map[string][]byte{}
	license, err := bundledModelFile("LICENSE.txt")
	if err != nil {
		return result, err
	}
	wanted["LICENSE.txt"] = license
	for _, asset := range selected {
		for _, file := range modelFiles(asset) {
			if _, exists := wanted[file.File]; exists {
				continue
			}
			data, err := bundledModelFile(file.File)
			if err != nil {
				return result, fmt.Errorf("read model dependency: %w", err)
			}
			if int64(len(data)) != file.Bytes || sha256Bytes(data) != file.SHA256 {
				return result, fmt.Errorf("3D asset integrity mismatch: %s", file.File)
			}
			if int64(len(data)) > s.opts.MaxAssetBytes {
				return result, fmt.Errorf("3D asset exceeds configured asset limit")
			}
			wanted[file.File] = data
		}
		copy := manifest
		copy.Categories = nil
		copy.Assets = []ModelAsset{asset}
		data, err := json.MarshalIndent(copy, "", "  ")
		if err != nil {
			return result, err
		}
		if int64(len(data)+1) > s.opts.MaxFileBytes {
			return result, fmt.Errorf("3D manifest exceeds configured file limit")
		}
		name := "assets/" + asset.ID + ".json"
		wanted[name] = append(data, '\n')
		result.AssetIDs = append(result.AssetIDs, asset.ID)
		result.Manifests[asset.ID] = rel + "/" + name
	}
	result.Metadata = result.Manifests[result.AssetIDs[0]]
	result.ThreeExample = modelExample(manifest.Version, result.AssetIDs[0])
	added := map[string][]byte{}
	var extraBytes int64
	for name, data := range wanted {
		path, _, err := secureJoin(stage, rel+"/"+name, false)
		if err != nil {
			return result, err
		}
		actual, err := os.ReadFile(path)
		if err == nil {
			if !bytes.Equal(data, actual) {
				return result, fmt.Errorf("3D pack copy is modified; preserve %s", name)
			}
		} else if os.IsNotExist(err) {
			added[name] = data
			extraBytes += int64(len(data))
		} else {
			return result, fmt.Errorf("inspect 3D project copy: %w", err)
		}
	}
	if len(added) == 0 {
		return result, nil
	}
	if err := validateTreeLimits(stage, s.opts.MaxFilesPerProject-len(added), s.opts.MaxProjectBytes-extraBytes); err != nil {
		return result, fmt.Errorf("3D selection exceeds project limits: %w", err)
	}
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return result, err
	}
	tmp, err := os.MkdirTemp(s.stagingDir, ".gm-models-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(tmp)
	for name, data := range added {
		path := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return result, err
		}
		if err := os.WriteFile(path, data, 0o640); err != nil {
			return result, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var published []string
	committed := false
	defer func() {
		if !committed {
			for _, path := range published {
				relative, _ := filepath.Rel(filepath.Join(stage, filepath.FromSlash(rel)), path)
				expected := added[filepath.ToSlash(relative)]
				if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, expected) {
					_ = os.Remove(path)
				}
			}
		}
	}()
	names := make([]string, 0, len(added))
	for name := range added {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		// Publish discovery metadata after its complete dependency closure.
		a, b := strings.HasPrefix(names[i], "assets/"), strings.HasPrefix(names[j], "assets/")
		if a != b {
			return !a
		}
		return names[i] < names[j]
	})
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		path, _, err := secureJoin(stage, rel+"/"+name, false)
		if err != nil {
			return result, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return result, err
		}
		// Link publishes complete bytes without replacing a concurrently created file.
		// Both paths are on the job staging volume; removing tmp drops only its link.
		if err := os.Link(filepath.Join(tmp, filepath.FromSlash(name)), path); err != nil {
			return result, fmt.Errorf("publish 3D asset: %w", err)
		}
		published = append(published, path)
		_, err = tx.ExecContext(ctx, `INSERT INTO gm_assets(project_id,job_id,path,kind,generator,provenance,content_hash,created_at) VALUES(?,?,?,?,?,?,?,?)`, job.ProjectID, jobID, rel+"/"+name, "model_pack", "builtin", ModelPackID+"@"+manifest.Version, sha256Bytes(added[name]), time.Now().UTC())
		if err != nil {
			return result, fmt.Errorf("record 3D asset: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit 3D asset import: %w", err)
	}
	committed = true
	_, _ = s.emit(ctx, job.ProjectID, jobID, "asset_changed", map[string]any{"path": rel, "kind": "model_pack", "generator": "builtin"})
	return result, nil
}
