package gamemaker

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Production source artwork is retained in git but never shipped in the binary.
//
//go:embed asset_packs/catalog.json asset_packs/*/sheet.png asset_packs/*/sheet.json
var assetPackFS embed.FS

type AssetPackSummary struct {
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type AssetPack struct {
	AssetPackSummary
	SchemaVersion int             `json:"schema_version"`
	Image         string          `json:"image"`
	Columns       int             `json:"columns"`
	Rows          int             `json:"rows"`
	FrameWidth    int             `json:"frame_width"`
	FrameHeight   int             `json:"frame_height"`
	Frames        json.RawMessage `json:"frames"`
	Assets        json.RawMessage `json:"assets"`
	Animations    json.RawMessage `json:"animations"`
	Provenance    json.RawMessage `json:"provenance"`
}

type ImportedAssetPack struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Image    string `json:"image"`
	Metadata string `json:"metadata"`
}

func (s *Service) ListAssetPacks() ([]AssetPackSummary, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	if !s.policy.Enabled {
		return nil, ErrDisabled
	}
	data, err := assetPackFS.ReadFile("asset_packs/catalog.json")
	if err != nil {
		return nil, fmt.Errorf("list bundled sprite packs: %w", err)
	}
	var packs []AssetPackSummary
	if err := json.Unmarshal(data, &packs); err != nil {
		return nil, fmt.Errorf("decode sprite catalog: %w", err)
	}
	return packs, nil
}

func (s *Service) AssetPackFile(id, filename string) ([]byte, error) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	if !s.policy.Enabled {
		return nil, ErrDisabled
	}
	return bundledAssetPackFile(id, filename)
}

func bundledAssetPackFile(id, filename string) ([]byte, error) {
	if id == "" || strings.ContainsAny(id, "/\\.\x00") || (filename != "sheet.png" && filename != "sheet.json") {
		return nil, ErrNotFound
	}
	data, err := assetPackFS.ReadFile("asset_packs/" + id + "/" + filename)
	if err != nil {
		return nil, ErrNotFound
	}
	return data, nil
}

func (s *Service) DescribeAssetPack(id string) (AssetPack, error) {
	data, err := s.AssetPackFile(id, "sheet.json")
	if err != nil {
		return AssetPack{}, err
	}
	var pack AssetPack
	if err := json.Unmarshal(data, &pack); err != nil {
		return AssetPack{}, fmt.Errorf("decode bundled sprite metadata: %w", err)
	}
	return pack, nil
}

func validateAssetPackIDs(ids []string) ([]string, error) {
	if len(ids) > 10 {
		return nil, fmt.Errorf("at most ten sprite packs may be selected")
	}
	var result []string
	seen := map[string]bool{}
	for _, id := range ids {
		if _, err := bundledAssetPackFile(id, "sheet.json"); err != nil {
			return nil, fmt.Errorf("unknown sprite pack %q: %w", id, err)
		}
		if !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	return result, nil
}

// ImportAssetPack publishes the PNG/JSON pair by renaming one temporary directory.
// Existing project copies are immutable: matching copies are reused, not replaced.
func (s *Service) ImportAssetPack(ctx context.Context, jobID, id string) (ImportedAssetPack, error) {
	s.policyMu.RLock()
	policy := s.policy
	if !policy.Enabled || policy.ReadOnly || !policy.AllowEdit {
		s.policyMu.RUnlock()
		if !policy.Enabled {
			return ImportedAssetPack{}, ErrDisabled
		}
		return ImportedAssetPack{}, ErrReadOnly
	}
	result, err := s.importAssetPack(ctx, jobID, id)
	s.policyMu.RUnlock()
	if err == nil {
		_ = s.BuildJob(ctx, jobID)
	}
	return result, err
}

func (s *Service) importAssetPack(ctx context.Context, jobID, id string) (ImportedAssetPack, error) {
	var result ImportedAssetPack
	metadata, err := bundledAssetPackFile(id, "sheet.json")
	if err != nil {
		return result, err
	}
	png, err := bundledAssetPackFile(id, "sheet.png")
	if err != nil {
		return result, err
	}
	var pack AssetPack
	if err := json.Unmarshal(metadata, &pack); err != nil {
		return result, fmt.Errorf("decode sprite pack: %w", err)
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return result, err
	}
	// Serialize against builds and other imports so limit checks see one snapshot.
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	rel := "assets/builtin/" + id + "/" + pack.Version
	target, _, err := secureJoin(stage, rel, false)
	if err != nil {
		return result, err
	}
	result = ImportedAssetPack{ID: id, Version: pack.Version, Image: rel + "/sheet.png", Metadata: rel + "/sheet.json"}
	if info, statErr := os.Lstat(target); statErr == nil {
		if !info.IsDir() {
			return ImportedAssetPack{}, fmt.Errorf("sprite pack destination already exists")
		}
		entries, err := os.ReadDir(target)
		if err != nil || len(entries) != 2 {
			return ImportedAssetPack{}, fmt.Errorf("sprite pack copy is incomplete or modified")
		}
		for filename, expected := range map[string][]byte{"sheet.png": png, "sheet.json": metadata} {
			path, _, err := secureJoin(stage, rel+"/"+filename, false)
			if err != nil {
				return ImportedAssetPack{}, err
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, expected) {
				return ImportedAssetPack{}, fmt.Errorf("sprite pack copy is incomplete or modified")
			}
		}
		return result, nil
	} else if !os.IsNotExist(statErr) {
		return ImportedAssetPack{}, fmt.Errorf("inspect sprite pack destination: %w", statErr)
	}
	if int64(len(png)) > s.opts.MaxAssetBytes || int64(len(metadata)) > s.opts.MaxFileBytes {
		return ImportedAssetPack{}, fmt.Errorf("sprite pack exceeds configured file or asset limit")
	}
	if err := validateTreeLimits(stage, s.opts.MaxFilesPerProject-2, s.opts.MaxProjectBytes-int64(len(png)+len(metadata))); err != nil {
		return ImportedAssetPack{}, fmt.Errorf("sprite pack exceeds configured project limit: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return ImportedAssetPack{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return ImportedAssetPack{}, fmt.Errorf("create sprite pack parent: %w", err)
	}
	tmp, err := os.MkdirTemp(filepath.Dir(target), ".gm-pack-*")
	if err != nil {
		return ImportedAssetPack{}, fmt.Errorf("stage sprite pack: %w", err)
	}
	defer os.RemoveAll(tmp)
	for filename, data := range map[string][]byte{"sheet.png": png, "sheet.json": metadata} {
		if err := os.WriteFile(filepath.Join(tmp, filename), data, 0o640); err != nil {
			return ImportedAssetPack{}, fmt.Errorf("write staged sprite pack: %w", err)
		}
	}
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return ImportedAssetPack{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportedAssetPack{}, fmt.Errorf("record sprite pack: %w", err)
	}
	defer tx.Rollback()
	for filename, data := range map[string][]byte{"sheet.png": png, "sheet.json": metadata} {
		_, err = tx.ExecContext(ctx, `INSERT INTO gm_assets(project_id,job_id,path,kind,generator,provenance,content_hash,created_at) VALUES(?,?,?,?,?,?,?,?)`,
			job.ProjectID, jobID, rel+"/"+filename, "sprite_pack", "builtin", id+"@"+pack.Version, sha256Bytes(data), time.Now().UTC())
		if err != nil {
			return ImportedAssetPack{}, fmt.Errorf("record sprite pack file: %w", err)
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		return ImportedAssetPack{}, fmt.Errorf("publish sprite pack: %w", err)
	}
	if err := tx.Commit(); err != nil {
		if rollbackErr := os.Rename(target, tmp); rollbackErr != nil {
			return ImportedAssetPack{}, fmt.Errorf("record sprite pack: %w; undo import: %v", err, rollbackErr)
		}
		return ImportedAssetPack{}, fmt.Errorf("record sprite pack: %w", err)
	}
	_, _ = s.emit(ctx, job.ProjectID, jobID, "asset_changed", map[string]any{"path": rel, "kind": "sprite_pack", "generator": "builtin"})
	return result, nil
}
