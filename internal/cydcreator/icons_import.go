package cydcreator

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
)

type IconImport struct {
	Asset      Asset    `json:"asset"`
	Assets     []Asset  `json:"assets"`
	Frames     []string `json:"frames"`
	Motion     string   `json:"motion"`
	Version    int64    `json:"version"`
	AddedBytes int64    `json:"added_bytes"`
	TotalBytes int64    `json:"total_bytes"`
}

// ImportIcon validates the whole selection before publishing any layout change.
// All frames participate in the ordinary asset, revision and ZIP lifecycles.
func (s *Service) ImportIcon(ctx context.Context, id string, ifVersion int64, request IconRequest, library *IconLibrary) (IconImport, error) {
	var result IconImport
	motif, err := request.motif()
	if err != nil {
		return result, err
	}
	if library == nil {
		return result, NewError(CodeNotFound, "icon library is unavailable", nil)
	}
	count := 1
	result.Motion = motif.Motion
	if request.Style == "sober" && result.Motion != "frames" {
		result.Motion = "pulse"
	}
	if request.Animated && motif.Motion == "frames" {
		count = 6
	}
	raw := make(map[string][]byte, count)
	for frame := 0; frame < count; frame++ {
		data, asset, err := library.read(request, frame)
		if err != nil {
			return result, err
		}
		raw[asset.ID] = data
		result.Assets = append(result.Assets, asset)
		result.Frames = append(result.Frames, asset.ID)
	}
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return result, s.internalError("import icon", err)
	}
	if err := checkLock(row, OriginUser); err != nil {
		return result, err
	}
	if ifVersion != row.Version {
		return result, NewError(CodeProjectConflict, "the layout was changed in the meantime", map[string]any{"version": row.Version})
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return result, s.internalError("import icon", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return result, err
	}
	board, err := BoardByID(p.Board)
	if err != nil {
		return result, err
	}
	storedBytes, err := storedAssetBytes(root, p)
	if err != nil {
		return result, s.internalError("import icon", err)
	}
	for _, size := range layoutAssetSizes(p) {
		result.TotalBytes += size
	}
	var pending []Asset
	staged := map[string]bool{}
	for i, asset := range result.Assets {
		found := false
		for _, old := range p.Assets {
			if old.ID != asset.ID && old.File != asset.File {
				continue
			}
			if old.ID != asset.ID || old.File != asset.File || old.W != asset.W || old.H != asset.H || old.Encoding != asset.Encoding || !old.Alpha {
				return result, NewError(CodeFileConflict, "icon asset conflicts with an existing image", nil)
			}
			if !staged[asset.ID] {
				data, err := rootReadFile(root, old.File, MaxAssetUploadBytes)
				if err != nil || !bytes.Equal(data, raw[asset.ID]) {
					return result, NewError(CodeFileConflict, "existing icon data has changed", nil)
				}
			}
			result.Assets[i] = old
			found = true
			break
		}
		if found {
			continue
		}
		// An undo may have removed the entry but retained its immutable file.
		if data, err := rootReadFile(root, asset.File, MaxAssetUploadBytes); err == nil {
			if !bytes.Equal(data, raw[asset.ID]) {
				return result, NewError(CodeFileConflict, "icon file already exists with different data", nil)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return result, s.internalError("import icon", err)
		}
		pending = append(pending, asset)
		staged[asset.ID] = true
		p.Assets = append(p.Assets, asset)
		result.AddedBytes += assetDataSize(asset, true)
		storedBytes += int64(len(raw[asset.ID]))
	}
	result.TotalBytes += result.AddedBytes
	if result.TotalBytes > AssetBudget(board.FlashMB) {
		return result, NewError(CodeAssetTooLarge, "the selected icon exceeds the project image budget", map[string]any{"reason": "budget", "needed": result.TotalBytes, "budget": AssetBudget(board.FlashMB)})
	}
	if storedBytes > MaxAssetTotalBytes {
		return result, assetTotalError()
	}
	if _, err := MarshalProject(p); err != nil {
		return result, err
	}
	var created []Asset
	defer func() {
		for _, a := range created {
			if !layoutListsAsset(root, a) {
				_ = root.Remove(filepath.FromSlash(a.File))
			}
		}
	}()
	for _, a := range pending {
		if _, err := root.Stat(filepath.FromSlash(a.File)); errors.Is(err, fs.ErrNotExist) {
			if err := rootWriteFile(ctx, root, a.File, raw[a.ID]); err != nil {
				return result, s.internalError("import icon", err)
			}
			created = append(created, a)
		} else if err != nil {
			return result, s.internalError("import icon", err)
		}
	}
	result.Asset = result.Assets[0]
	result.Version = row.Version
	if len(pending) > 0 {
		result.Version, err = s.saveLayoutLocked(ctx, root, row, p)
		if err != nil {
			return result, s.internalError("import icon", err)
		}
		s.Emit(id, EventProjectChanged, "", "", projectChange(result.Version, OriginUser, "assets"))
	}
	return result, nil
}
