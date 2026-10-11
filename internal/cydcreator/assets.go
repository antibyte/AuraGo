package cydcreator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// MaxAssetUploadBytes caps one uploaded PNG/JPEG original (2 MiB).
const MaxAssetUploadBytes = 2 << 20

const maxAssetBaseBytes = 58 // leaves room for "_999" within the 63-character rule

// assetExtension detects PNG and JPEG by magic bytes.
func assetExtension(data []byte) (ext, contentType string, ok bool) {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return ".png", "image/png", true
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg", "image/jpeg", true
	}
	return "", "", false
}

func assetContentType(file string) string {
	if strings.HasSuffix(file, ".png") {
		return "image/png"
	}
	return "image/jpeg"
}

// assetFileError refuses a layout asset entry whose file breaks the asset file
// rule (assetFileRegex). PutLayout already rejects such entries; a hand-edited
// or imported layout may still carry one, so every reader and remover of an
// asset file checks the rule again right before it touches the file.
func assetFileError(assetID string) error {
	return layoutError("asset file must match assets/<name>.png, .jpg or .jpeg", map[string]any{"asset": clipEcho(assetID)})
}

// checkUploadHeader reads only the image header and refuses a picture larger
// than the largest display of any board before InspectAsset decodes its
// pixels (a small PNG can decode to tens of MiB). Every display is smaller
// than assetHeader's 2048-pixel limit, so a phone photo is "too large"
// (dimensions), not a bad request. The display of the project's board is
// checked exactly after the decode.
func checkUploadHeader(data []byte) error {
	long, short, err := maxBoardDisplay()
	if err != nil {
		return err
	}
	if cfg, format, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && (format == "png" || format == "jpeg") {
		w, h := cfg.Width, cfg.Height
		if max(w, h) > long || min(w, h) > short {
			return NewError(CodeAssetTooLarge, fmt.Sprintf("the image (%dx%d) is larger than every supported display (%dx%d)", w, h, long, short),
				map[string]any{"reason": "dimensions", "w": w, "h": h, "max_w": long, "max_h": short})
		}
	}
	_, err = assetHeader(data)
	return err
}

// layoutListsAsset reports whether the stored layout lists asset a.
func layoutListsAsset(root *os.Root, a Asset) bool {
	p, err := readLayout(root)
	if err != nil {
		return false
	}
	for _, listed := range p.Assets {
		if listed.ID == a.ID && listed.File == a.File {
			return true
		}
	}
	return false
}

// sanitizeAssetBase turns an upload file name into the base of
// assets/[a-z0-9][a-z0-9_-]{0,62}.(png|jpg|jpeg): folded to ASCII, other
// characters become "_", leading "_"/"-" are removed, empty names become
// "image".
func sanitizeAssetBase(name string) string {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	var b strings.Builder
	underscore := false
	for _, r := range asciiFold(base) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
			underscore = false
		} else if !underscore {
			b.WriteByte('_')
			underscore = true
		}
	}
	out := strings.TrimLeft(b.String(), "_-")
	if len(out) > maxAssetBaseBytes {
		out = out[:maxAssetBaseBytes]
	}
	out = strings.TrimRight(out, "_-")
	if out == "" {
		out = "image"
	}
	return out
}

// uniqueAssetFile returns assets/<base>[_n]<ext> that neither the layout nor
// the disk uses yet.
func uniqueAssetFile(root *os.Root, p Project, base, ext string) (string, error) {
	used := map[string]bool{}
	for _, a := range p.Assets {
		used[a.File] = true
	}
	for n := 1; n <= 999; n++ {
		name := base
		if n > 1 {
			name = fmt.Sprintf("%s_%d", base, n)
		}
		file := "assets/" + name + ext
		if used[file] {
			continue
		}
		if _, err := root.Stat(filepath.FromSlash(file)); errors.Is(err, fs.ErrNotExist) {
			return file, nil
		}
	}
	return "", NewError(CodeBusy, "no free asset file name", map[string]any{"name": base})
}

// PutAsset stores an uploaded PNG/JPEG under assets/, adds it to the layout
// and returns the asset and the new layout version. Limits: 2 MiB per file,
// image within the display, decoded total within AssetBudget.
func (s *Service) PutAsset(ctx context.Context, id string, name string, data []byte) (Asset, int64, error) {
	if len(data) == 0 {
		return Asset{}, 0, NewError(CodeBadRequest, "the image is empty", nil)
	}
	if len(data) > MaxAssetUploadBytes {
		return Asset{}, 0, NewError(CodeAssetTooLarge, fmt.Sprintf("an image file may have at most %d bytes", MaxAssetUploadBytes),
			map[string]any{"reason": "file_size", "max": MaxAssetUploadBytes})
	}
	ext, _, ok := assetExtension(data)
	if !ok {
		return Asset{}, 0, NewError(CodeBadRequest, "only PNG and JPEG images are supported", nil)
	}
	if err := checkUploadHeader(data); err != nil {
		return Asset{}, 0, s.internalError("store asset", err)
	}
	w, h, alpha, decoded, err := InspectAsset(data)
	if err != nil {
		return Asset{}, 0, err
	}
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return Asset{}, 0, s.internalError("store asset", err)
	}
	if err := checkLock(row, OriginUser); err != nil {
		return Asset{}, 0, err
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return Asset{}, 0, s.internalError("store asset", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return Asset{}, 0, s.internalError("store asset", err)
	}
	board, err := BoardByID(p.Board)
	if err != nil {
		return Asset{}, 0, err
	}
	long, short := max(board.Display.Width, board.Display.Height), min(board.Display.Width, board.Display.Height)
	if max(w, h) > long || min(w, h) > short {
		return Asset{}, 0, NewError(CodeAssetTooLarge, fmt.Sprintf("the image (%dx%d) is larger than the %dx%d display", w, h, long, short),
			map[string]any{"reason": "dimensions", "w": w, "h": h, "max_w": long, "max_h": short})
	}
	var used int64
	for _, size := range layoutAssetSizes(p) {
		used += size
	}
	if budget := AssetBudget(board.FlashMB); used+decoded > budget {
		return Asset{}, 0, NewError(CodeAssetTooLarge, fmt.Sprintf("the images would need %d bytes, the budget is %d", used+decoded, budget),
			map[string]any{"reason": "budget", "budget": budget, "used": used, "needed": decoded})
	}
	file, err := uniqueAssetFile(root, p, sanitizeAssetBase(name), ext)
	if err != nil {
		return Asset{}, 0, err
	}
	assetID, err := freshID(s.ids, "a", p.usedIDs())
	if err != nil {
		return Asset{}, 0, s.internalError("store asset", err)
	}
	asset := Asset{ID: assetID, File: file, W: w, H: h, Alpha: alpha}
	if err := rootWriteFile(ctx, root, file, data); err != nil {
		return Asset{}, 0, s.internalError("store asset", err)
	}
	p.Assets = append(p.Assets, asset)
	version, err := s.saveLayoutLocked(ctx, root, row, p)
	if err != nil {
		// saveLayoutLocked can fail after it published the layout (version
		// bump): then the stored layout lists the image and it must stay.
		if !layoutListsAsset(root, asset) {
			_ = root.Remove(filepath.FromSlash(file))
		}
		return Asset{}, 0, s.internalError("store asset", err)
	}
	s.Emit(id, EventProjectChanged, "", "", projectChange(version, OriginUser, "assets"))
	return asset, version, nil
}

// DeleteAsset removes an asset from the layout and deletes its file. Widgets
// that still reference it get an ASSET_MISSING issue.
func (s *Service) DeleteAsset(ctx context.Context, id, assetID string) (int64, error) {
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return 0, s.internalError("delete asset", err)
	}
	if err := checkLock(row, OriginUser); err != nil {
		return 0, err
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return 0, s.internalError("delete asset", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return 0, s.internalError("delete asset", err)
	}
	idx := -1
	for i, a := range p.Assets {
		if a.ID == assetID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0, NewError(CodeNotFound, "asset not found", map[string]any{"asset_id": assetID})
	}
	file := p.Assets[idx].File
	p.Assets = append(p.Assets[:idx], p.Assets[idx+1:]...)
	version, err := s.saveLayoutLocked(ctx, root, row, p)
	if err != nil {
		return 0, s.internalError("delete asset", err)
	}
	s.removeAssetFile(root, id, file, p.Assets)
	s.Emit(id, EventProjectChanged, "", "", projectChange(version, OriginUser, "assets"))
	return version, nil
}

// removeAssetFile deletes the image of a removed layout entry. A file outside
// the asset file rule (the layout, an app source, a path with "..") or one
// that a remaining entry still lists is left alone: the layout may have been
// edited by hand or imported.
func (s *Service) removeAssetFile(root *os.Root, projectID, file string, remaining []Asset) {
	if !assetFileRegex.MatchString(file) {
		s.logger.Warn("CYD Creator asset entry with an invalid file name removed; the file was left alone",
			"project_id", projectID, "file", clipEcho(file))
		return
	}
	for _, a := range remaining {
		if a.File == file {
			return
		}
	}
	if err := root.Remove(filepath.FromSlash(file)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.logger.Warn("CYD Creator asset file could not be removed", "project_id", projectID, "file", file, "error", err)
	}
}

// memoryAsset serves an asset read into memory; Close is a no-op.
type memoryAsset struct{ *bytes.Reader }

func (memoryAsset) Close() error { return nil }

// OpenAsset returns the original image of an asset for the designer preview.
// The file is read into memory (at most MaxAssetUploadBytes, else
// CYD_ASSET_TOO_LARGE reason file_size) under the project lock, so no file
// handle stays open inside the project directory while the response streams:
// on Windows an open handle would block the directory swap of a publication
// or restore. The caller closes the reader.
func (s *Service) OpenAsset(ctx context.Context, id, assetID string) (io.ReadSeekCloser, Asset, string, error) {
	unlock := s.lockProject(id)
	defer unlock()
	row, err := s.loadProjectRow(ctx, id)
	if err != nil {
		return nil, Asset{}, "", s.internalError("read asset", err)
	}
	root, err := s.openProjectRoot(row)
	if err != nil {
		return nil, Asset{}, "", s.internalError("read asset", err)
	}
	defer root.Close()
	p, err := readLayout(root)
	if err != nil {
		return nil, Asset{}, "", s.internalError("read asset", err)
	}
	for _, a := range p.Assets {
		if a.ID != assetID {
			continue
		}
		if !assetFileRegex.MatchString(a.File) {
			return nil, Asset{}, "", assetFileError(a.ID)
		}
		data, err := rootReadFile(root, a.File, MaxAssetUploadBytes)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errNotRegularFile) {
			break
		}
		if errors.Is(err, errFileTooLarge) {
			return nil, Asset{}, "", NewError(CodeAssetTooLarge, fmt.Sprintf("asset %s: the image file is larger than %d bytes", a.ID, MaxAssetUploadBytes),
				map[string]any{"asset": a.ID, "reason": "file_size", "max": MaxAssetUploadBytes})
		}
		if err != nil {
			return nil, Asset{}, "", s.internalError("read asset", err)
		}
		return memoryAsset{bytes.NewReader(data)}, a, assetContentType(a.File), nil
	}
	return nil, Asset{}, "", NewError(CodeNotFound, "asset not found", map[string]any{"asset_id": assetID})
}
